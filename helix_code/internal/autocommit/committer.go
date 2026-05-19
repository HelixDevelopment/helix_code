// Package autocommit — committer.go (P2-F22-T05).
//
// AutoCommitter is the public entry point. The registry calls
// MaybeCommit(ctx, CommitContext) after every successful edit-class tool
// invocation. The committer:
//   1. Checks the per-call SkipRequested flag (and SkipParamKey in Args).
//   2. Checks the atomic-bool enabled flag (set by env at startup;
//      mutated by /git_auto_commit on/off at runtime).
//   3. Verifies the working dir is a git repo.
//   4. Checks `git status --porcelain` — clean tree → no-op.
//   5. Stages MutatedPaths via `git add --` (or all dirty paths if list
//      is empty).
//   6. Reads `git diff --staged` for the LLM summariser.
//   7. Calls MessageSummariser.Summarise to get a 1-line subject.
//   8. Runs the secret filter over the subject.
//   9. Builds the full message: subject + "\n\n" + CoAuthorTrailer.
//  10. Runs `git commit -m <message>` and reads back the new SHA.
//  11. Returns CommitResult{SHA, Subject, Files, Skipped:false}.
//
// All failures are wrapped in ErrCommitFailed. The registry's hook
// swallows any error and logs at WARN — auto-commit failure NEVER
// propagates to the calling tool.
//
// Atomicity: enabled is read once at the top of MaybeCommit via
// atomic.Bool.Load, so /git_auto_commit on/off swaps are honoured on
// the very next MaybeCommit (not the in-flight one).
package autocommit

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// AutoCommitter is the per-CLI-instance committer. Construct via
// NewAutoCommitter. Safe for concurrent use — the enabled flag is
// atomic.Bool, the rest of the state is set at construct-time and
// read-only thereafter (the *Git wrapper itself serialises mutations
// implicitly because git's index lock is per-repo).
type AutoCommitter struct {
	enabled    atomic.Bool
	git        *Git
	summariser MessageSummariser
	filter     *SecretFilter
	workingDir string
	log        *zap.Logger
	now        func() time.Time
}

// NewAutoCommitter constructs an AutoCommitter from Options. nil
// Options.Logger is upgraded to zap.NewNop; nil Options.NowFunc to
// time.Now. Provider may be nil — DeterministicFallback is used.
func NewAutoCommitter(o Options) *AutoCommitter {
	log := o.Logger
	if log == nil {
		log = zap.NewNop()
	}
	now := o.NowFunc
	if now == nil {
		now = time.Now
	}
	c := &AutoCommitter{
		git:        NewGit(o.WorkingDir, log),
		summariser: NewSummariser(o.Provider),
		filter:     NewSecretFilter(),
		workingDir: o.WorkingDir,
		log:        log,
		now:        now,
	}
	c.enabled.Store(o.Enabled)
	return c
}

// Enabled reports the current enabled state. Lock-free read.
func (c *AutoCommitter) Enabled() bool {
	return c.enabled.Load()
}

// SetEnabled atomically swaps the enabled flag. The next MaybeCommit
// call observes the new value.
func (c *AutoCommitter) SetEnabled(v bool) {
	c.enabled.Store(v)
}

// IsGitRepo is a convenience for /git_auto_commit status.
func (c *AutoCommitter) IsGitRepo() bool {
	if c == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	ok, _ := c.git.IsRepo(ctx)
	return ok
}

// MaybeCommit is the single entry point. Returns CommitResult{Skipped:true}
// for any non-error skip case (disabled, not-a-repo, no-changes,
// per-edit skip). Returns ErrCommitFailed wrapped error for any
// pipeline failure; the caller is expected to log + continue.
func (c *AutoCommitter) MaybeCommit(ctx context.Context, cctx CommitContext) (CommitResult, error) {
	// Step 1: per-call skip request takes precedence over everything.
	// CONST-046: Reason is user-facing (returned to /git_auto_commit
	// observers + zap log fields); resolved via tr().
	if cctx.SkipRequested {
		return CommitResult{Skipped: true, Reason: tr(ctx, "internal_autocommit_skipped_per_edit_skip_requested", nil)}, nil
	}
	// Backup: check Args for SkipParamKey:true.
	if cctx.Args != nil {
		if v, ok := cctx.Args[SkipParamKey].(bool); ok && v {
			return CommitResult{Skipped: true, Reason: tr(ctx, "internal_autocommit_skipped_per_edit_skip_via_param", map[string]any{"ParamKey": SkipParamKey})}, nil
		}
	}

	// Step 2: enabled check — atomic.Bool.Load is the single
	// re-entry point that observes /git_auto_commit on/off swaps.
	if !c.enabled.Load() {
		return CommitResult{Skipped: true, Reason: tr(ctx, "internal_autocommit_skipped_disabled", nil)}, nil
	}

	// Step 3: must be a git repo.
	isRepo, _ := c.git.IsRepo(ctx)
	if !isRepo {
		return CommitResult{Skipped: true, Reason: tr(ctx, "internal_autocommit_skipped_not_a_git_repo", nil)}, nil
	}

	// Step 4: check porcelain — clean tree means nothing to do.
	porcelain, err := c.git.StatusPorcelain(ctx)
	if err != nil {
		return CommitResult{}, fmt.Errorf("%w: status: %v", ErrCommitFailed, err)
	}
	if strings.TrimSpace(porcelain) == "" {
		return CommitResult{Skipped: true, Reason: tr(ctx, "internal_autocommit_skipped_no_changes_to_commit", nil)}, nil
	}

	// Step 5: stage paths. If MutatedPaths is non-empty, add only
	// those; else `git add -A` to stage everything dirty (matches the
	// porcelain-discovery fallback for tools without an explicit path
	// derivation).
	if len(cctx.MutatedPaths) > 0 {
		if err := c.git.Add(ctx, cctx.MutatedPaths...); err != nil {
			return CommitResult{}, fmt.Errorf("%w: add: %v", ErrCommitFailed, err)
		}
	} else {
		if _, err := c.git.run(ctx, "add", "-A"); err != nil {
			return CommitResult{}, fmt.Errorf("%w: add -A: %v", ErrCommitFailed, err)
		}
	}

	// Step 6: read staged diff for the summariser.
	diff, err := c.git.DiffStaged(ctx)
	if err != nil {
		return CommitResult{}, fmt.Errorf("%w: diff --staged: %v", ErrCommitFailed, err)
	}
	if strings.TrimSpace(diff) == "" {
		// Race: porcelain showed dirt but diff is empty. Bail
		// without committing. Possible on race with concurrent
		// editor; rare in practice.
		return CommitResult{Skipped: true, Reason: tr(ctx, "internal_autocommit_skipped_no_staged_changes_after_add", nil)}, nil
	}

	// Step 7: summarise.
	subject := c.summariser.Summarise(ctx, diff, cctx.ToolName, cctx.MutatedPaths)
	// Step 8: secret filter — over the subject only (the body is the
	// trailer + nothing else).
	subject = c.filter.Filter(subject)
	if len(subject) > maxSubjectChars {
		subject = subject[:maxSubjectChars]
	}

	// Step 9: assemble message — subject + blank line + co-author
	// trailer (Q3=A unconditional).
	message := subject + "\n\n" + CoAuthorTrailer + "\n"

	// Step 10: commit via the git wrapper. SHA comes back via
	// rev-parse HEAD inside Commit().
	sha, err := c.git.Commit(ctx, message)
	if err != nil {
		return CommitResult{}, fmt.Errorf("%w: commit: %v", ErrCommitFailed, err)
	}

	// Step 11: log only the safe fields (path + SHA + length;
	// NEVER the diff body or commit message body).
	c.log.Info("auto-commit",
		zap.String("sha", sha),
		zap.Strings("paths", cctx.MutatedPaths),
		zap.Int("subject_len", len(subject)),
	)

	return CommitResult{
		SHA:     sha,
		Subject: subject,
		Files:   cctx.MutatedPaths,
		Skipped: false,
	}, nil
}
