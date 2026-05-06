// Package autocommit implements aider-style per-edit git auto-commit for the
// HelixCode CLI agent (P2-F22).
//
// The package adds a single AutoCommitter that fires after each successful
// edit-class tool execution, stages the mutated paths, summarises the diff via
// the configured llm.Provider (with a deterministic fallback when the LLM is
// unavailable), commits with a fixed Co-Authored-By trailer, and best-effort
// strips common credential patterns from the commit message.
//
// Design summary (Q1-Q5 = A,A,A,A,A):
//   - Q1=A: ONE commit per accepted edit (aider default).
//   - Q2=A: LLM-summarised commit message; deterministic fallback on LLM error.
//   - Q3=A: Co-Authored-By: HelixCode <noreply@helixcode.dev> on every commit.
//   - Q4=A: Default ON. Opt-out via env HELIXCODE_GIT_AUTO_COMMIT=off,
//     runtime /git_auto_commit off, or per-edit _helix_skip_git_commit:true.
//   - Q5=A: /git_auto_commit slash command (status/on/off/show); no cobra.
//
// This file is the data foundation: types, constants, sentinel errors. No
// behaviour. The git wrapper, summariser, secret filter, and committer live
// in sibling files (git.go, summariser.go, secret_filter.go, committer.go).
//
// References:
//   - Spec: docs/superpowers/specs/2026-05-06-p2-f22-aider-git-auto-commit-design.md (commit 8be7fba) §3.3
//   - Plan: docs/superpowers/plans/2026-05-06-p2-f22-aider-git-auto-commit.md T02
//   - CONST-035 (Zero-Bluff): every commit MUST be backed by a real
//     working-tree change; no metadata-only commits.
//   - CONST-042 (No-Secret-Leak): credential patterns are stripped from
//     the commit message before exec; see secret_filter.go.
//   - CONST-043 (No-Force-Push): this package NEVER calls git push.
package autocommit

import (
	"context"
	"errors"
	"time"

	"go.uber.org/zap"

	"dev.helix.code/internal/llm"
)

// EnvVarName is the environment variable consulted at startup to choose the
// initial enabled state of the auto-committer. Default-on; the literal value
// "off" disables auto-commit. Anything else (including unset) leaves it on.
// Typos default to safe-on per Q4=A.
const EnvVarName = "HELIXCODE_GIT_AUTO_COMMIT"

// CoAuthorTrailer is appended verbatim to every auto-commit message body
// (after a blank line). Q3=A is unambiguous — no per-commit suppression. The
// trailer is byte-pinned by tests to prevent silent drift.
const CoAuthorTrailer = "Co-Authored-By: HelixCode <noreply@helixcode.dev>"

// SkipParamKey is the per-tool-call parameter key that callers may set to
// true to suppress auto-commit for that single invocation. The committer
// honours it before the enabled-flag check so an opted-out call is a clean
// no-op even when the global flag is on.
const SkipParamKey = "_helix_skip_git_commit"

// CommitContext carries per-call data from the registry's post-Execute hook
// to the committer. The registry derives MutatedPaths via a per-tool table
// (see internal/tools/registry.go::derivePaths); SkipRequested mirrors the
// SkipParamKey value.
type CommitContext struct {
	// ToolName is the name of the tool that just ran successfully.
	ToolName string

	// Args is the (possibly-rewritten) parameter map passed to the tool.
	// The committer reads SkipParamKey from this map as a backup to
	// SkipRequested; both must agree (set by the registry).
	Args map[string]interface{}

	// MutatedPaths is the list of repository-relative paths the tool is
	// known to have written. Empty list means "let the porcelain
	// discovery decide" — useful for tools whose mutation set isn't
	// statically derivable from Args.
	MutatedPaths []string

	// SkipRequested is true when the registry observed SkipParamKey:true
	// in Args. Mirrored here for explicit visibility.
	SkipRequested bool
}

// CommitResult is returned from MaybeCommit. The Skipped field encodes
// "no commit fired" without surfacing it as an error; callers that want to
// know why consult Reason. SHA is empty when Skipped is true.
type CommitResult struct {
	// SHA is the full 40-char hex SHA of the new commit. Empty when
	// Skipped is true.
	SHA string

	// Subject is the first line of the commit message (≤ 72 chars).
	// Empty when Skipped is true.
	Subject string

	// Files is the list of paths that were actually staged + committed.
	// Empty when Skipped is true.
	Files []string

	// Skipped is true when the committer decided not to commit
	// (disabled, not-a-repo, no-changes, per-edit-skip, ...). Not an
	// error condition.
	Skipped bool

	// Reason is a short human-readable explanation when Skipped is
	// true. Examples: "auto-commit disabled", "not a git repo",
	// "no changes", "per-edit skip".
	Reason string
}

// Options is the constructor input for AutoCommitter. WorkingDir MUST be the
// repository root or any path inside it; the git wrapper resolves the actual
// repo via "git rev-parse --is-inside-work-tree". Logger MAY be nil (zap.NewNop
// is used). Provider MAY be nil — the deterministic fallback summariser is used
// when the LLM is unavailable. NowFunc is a test seam for deterministic
// timestamps; nil → time.Now.
type Options struct {
	// Enabled is the initial value of the atomic-bool enabled flag.
	// Resolve from env at startup: env != "off" → true.
	Enabled bool

	// Provider is the llm.Provider used by the LLMSummariser to
	// generate commit messages. nil falls back to DeterministicFallback.
	Provider llm.Provider

	// WorkingDir is the directory inside the git work tree to operate
	// on. All git invocations run with this as cwd.
	WorkingDir string

	// Logger receives best-effort INFO/WARN/DEBUG records. nil →
	// zap.NewNop. The committer NEVER logs the diff body or commit
	// message at INFO; only paths, SHA, and lengths.
	Logger *zap.Logger

	// NowFunc is the time-source seam. nil → time.Now.
	NowFunc func() time.Time
}

// Sentinel errors. Wrapped via fmt.Errorf("...: %w", ...) at every callsite
// so callers can errors.Is-classify failures.
var (
	// ErrNotGitRepo is returned when the configured WorkingDir is not
	// inside a git work tree. Callers SHOULD treat this as a non-error
	// "no auto-commit possible" signal.
	ErrNotGitRepo = errors.New("not a git repo")

	// ErrCommitFailed wraps any failure during the commit pipeline
	// (stage, summarise, exec). The committer never propagates these
	// to the calling tool; the registry's hook logs at WARN and
	// continues.
	ErrCommitFailed = errors.New("commit failed")

	// ErrLLMUnavailable indicates the configured llm.Provider returned
	// an error or empty response. The committer recovers by falling
	// through to the deterministic-fallback summariser; this error is
	// surfaced for observability only.
	ErrLLMUnavailable = errors.New("llm unavailable")
)

// _ = context.Background is here so the package-level imports list reflects
// the runtime dependency graph even before the sibling files land. Removing
// this once committer.go is in place is harmless.
var _ = context.Background
