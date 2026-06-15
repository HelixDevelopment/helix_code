// Package autocommit — git_security_test.go (§11.4.118 discovery; §11.4.135 standing guard).
//
// Standing regression guard for a git ARGUMENT-INJECTION → arbitrary-file-write
// defect (CWE-88) in the read-only ref-taking wrappers DiffSinceRef / ShowCommit.
//
// Defect: both wrappers passed the caller-supplied `ref` VERBATIM as the first
// positional argument to `git diff <ref>` / `git show <ref>`. A `ref` that
// begins with `-` (e.g. `--output=/tmp/pwned`) is parsed by git as an OPTION,
// not a revision. `--output=<path>` makes `git diff`/`git show` write the patch
// to an attacker-chosen host path — arbitrary file write. The `ref` is operator/
// agent-supplied (the `/diff <ref>` REPL command → cmd/cli/main.go handleDiff →
// DiffSinceRef), so this is reachable, not theoretical.
//
// Fix: insert git's `--end-of-options` sentinel (git ≥ 2.24) BEFORE the ref so
// everything after it is forced to be an operand (revision), never an option.
// This blocks the `--output` (and every other `--flag`) injection while
// preserving normal ref semantics (a real SHA/tag/HEAD~N still resolves).
//
// §11.4.115 polarity switch via RED_MODE env:
//   - RED_MODE=0 (default — the STANDING GREEN guard): drive the REAL fixed
//     DiffSinceRef / ShowCommit and assert the injection is BLOCKED (no
//     out-of-tree file written) AND that legitimate refs still resolve. This is
//     the default so a regression in git.go (e.g. dropping --end-of-options) is
//     caught by a bare `go test ./internal/autocommit/`.
//   - RED_MODE=1: reproduce the defect on the UNGUARDED pre-fix construction
//     (raw `git diff <malicious-ref>`) inside a real t.TempDir() git repo,
//     asserting the injection SUCCEEDS (the out-of-tree file is written). This
//     proves the guard targets a genuine defect.
package autocommit

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// redMode reports whether the RED reproduction polarity is active. Default
// (RED_MODE unset or != "1") is the standing GREEN guard against the real fixed
// code; set RED_MODE=1 for the reproduction-on-broken-construction per §11.4.115.
func redMode() bool {
	return strings.TrimSpace(os.Getenv("RED_MODE")) == "1"
}

// seedTwoCommitRepo creates a real two-commit repo so `git diff <ref>` and
// `git show <ref>` have real content to emit (and thus a real patch the
// injected --output= would write out).
func seedTwoCommitRepo(t *testing.T) string {
	t.Helper()
	dir := setupRealGitRepo(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "x.txt"), []byte("v1\n"), 0644))
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v: %s", args, out)
	}
	run("add", "x.txt")
	run("commit", "-qm", "base")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "x.txt"), []byte("v2\n"), 0644))
	run("add", "x.txt")
	run("commit", "-qm", "change")
	return dir
}

// TestGit_DiffSinceRef_NoArgumentInjection is the standing guard for the
// DiffSinceRef arg-injection → arbitrary-file-write defect.
func TestGit_DiffSinceRef_NoArgumentInjection(t *testing.T) {
	dir := seedTwoCommitRepo(t)
	// Out-of-tree target the injection tries to write to. Placed in a sibling
	// TempDir so a successful injection is unambiguous and self-cleaning.
	pwned := filepath.Join(t.TempDir(), "pwned_diff.txt")
	require.NoFileExists(t, pwned)

	maliciousRef := "--output=" + pwned

	if redMode() {
		// RED_MODE=1: reproduce the defect on the UNGUARDED pre-fix
		// construction — raw `git diff <malicious-ref>`, exactly what the old
		// DiffSinceRef did (g.run(ctx, "diff", ref)). The injection MUST
		// succeed, writing the patch to the out-of-tree path.
		cmd := exec.CommandContext(context.Background(), "git", "diff", maliciousRef)
		cmd.Dir = dir
		_, _ = cmd.CombinedOutput() // exit code is 0 on success; ignore
		require.FileExists(t, pwned,
			"RED: unguarded `git diff <ref>` must write the injected --output= file (defect reproduced)")
		return
	}

	// RED_MODE=0: drive the REAL fixed code. The injection MUST be blocked —
	// no out-of-tree file is written. git rejects the option after
	// --end-of-options, surfaced as an error to the caller.
	g := NewGit(dir, zap.NewNop())
	_, err := g.DiffSinceRef(context.Background(), maliciousRef)
	require.Error(t, err, "GREEN: a `--output=`-style ref must be rejected, not executed as an option")
	require.NoFileExists(t, pwned,
		"GREEN: fixed DiffSinceRef must NOT write the injected out-of-tree file")

	// And a legitimate ref still resolves correctly (no over-blocking).
	out, err := g.DiffSinceRef(context.Background(), "HEAD~1")
	require.NoError(t, err)
	require.Contains(t, out, "+v2", "GREEN: legitimate refs must still produce the real diff")
}

// TestGit_ShowCommit_NoArgumentInjection is the standing guard for the
// ShowCommit arg-injection → arbitrary-file-write defect (the `/diff <commit>`
// substrate per the wrapper's doc comment).
func TestGit_ShowCommit_NoArgumentInjection(t *testing.T) {
	dir := seedTwoCommitRepo(t)
	pwned := filepath.Join(t.TempDir(), "pwned_show.txt")
	require.NoFileExists(t, pwned)

	maliciousRef := "--output=" + pwned

	if redMode() {
		// RED_MODE=1: reproduce on the UNGUARDED pre-fix construction —
		// raw `git show <malicious-ref>`, exactly what the old ShowCommit did.
		cmd := exec.CommandContext(context.Background(), "git", "show", maliciousRef)
		cmd.Dir = dir
		_, _ = cmd.CombinedOutput()
		require.FileExists(t, pwned,
			"RED: unguarded `git show <ref>` must write the injected --output= file (defect reproduced)")
		return
	}

	// RED_MODE=0: drive the REAL fixed code — injection blocked, real ref works.
	g := NewGit(dir, zap.NewNop())
	_, err := g.ShowCommit(context.Background(), maliciousRef)
	require.Error(t, err, "GREEN: a `--output=`-style ref must be rejected, not executed as an option")
	require.NoFileExists(t, pwned,
		"GREEN: fixed ShowCommit must NOT write the injected out-of-tree file")

	out, err := g.ShowCommit(context.Background(), "HEAD")
	require.NoError(t, err)
	require.Contains(t, out, "+v2", "GREEN: legitimate refs must still produce the real patch")
}
