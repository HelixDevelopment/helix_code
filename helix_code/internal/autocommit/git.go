// Package autocommit — git.go (P2-F22-T03).
//
// Thin wrapper around the system `git` binary. The wrapper exists so the
// rest of the package can take a *Git pointer and exercise it against real
// repos in tests; it does NOT abstract over different git implementations.
//
// Why shell-out to git rather than go-git? The user's existing tooling
// (~/.gitconfig, hooks, commit signing, aliases, credential helpers, etc.)
// only works through the real binary. go-git would silently bypass all of
// that. Per spec §11 #15.
//
// Every method takes a context.Context to be cancellable; every method
// returns the wrapped error from `git` verbatim so callers can grep
// stderr if something goes wrong.
package autocommit

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"go.uber.org/zap"
)

// Git is a thin wrapper around `exec.CommandContext("git", ...)` rooted at
// a single working directory. Construct with NewGit. Safe for concurrent
// use only if callers serialise mutations (Add/Commit) — read-only methods
// (IsRepo / StatusPorcelain / DiffStaged / DiffUnstaged / HeadSHA) may be
// called concurrently with each other but not with mutations.
type Git struct {
	workingDir string
	log        *zap.Logger
}

// NewGit constructs a Git rooted at the given workingDir. workingDir MUST
// exist on the filesystem; the constructor does not check (errors surface
// at first method call). nil logger is upgraded to zap.NewNop.
func NewGit(workingDir string, log *zap.Logger) *Git {
	if log == nil {
		log = zap.NewNop()
	}
	return &Git{workingDir: workingDir, log: log}
}

// run is the single shell-out helper. Methods are one-line wrappers around
// run() so the cmd-construction logic lives in exactly one place.
func (g *Git) run(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = g.workingDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("git %s: %w (output: %s)",
			strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

// IsRepo returns true iff the working directory is inside a git work tree.
// Errors are returned as (false, nil) so callers can treat "not a repo" as a
// non-error signal — git emits a non-zero exit when outside a repo.
func (g *Git) IsRepo(ctx context.Context) (bool, error) {
	out, err := g.run(ctx, "rev-parse", "--is-inside-work-tree")
	if err != nil {
		// Not-a-repo is the common non-error case. Suppress.
		return false, nil
	}
	return strings.TrimSpace(out) == "true", nil
}

// StatusPorcelain returns the verbatim output of `git status --porcelain`.
// The empty string (after TrimSpace) means "clean working tree".
func (g *Git) StatusPorcelain(ctx context.Context) (string, error) {
	return g.run(ctx, "status", "--porcelain")
}

// DiffStaged returns the verbatim output of `git diff --staged`. The empty
// string means nothing is staged.
func (g *Git) DiffStaged(ctx context.Context) (string, error) {
	return g.run(ctx, "diff", "--staged")
}

// DiffUnstaged returns the verbatim output of `git diff` (the unstaged
// working-tree diff). Empty string means working tree matches index.
func (g *Git) DiffUnstaged(ctx context.Context) (string, error) {
	return g.run(ctx, "diff")
}

// Add stages the listed paths via `git add --`. Paths are taken verbatim;
// callers SHOULD pass repository-relative paths (the wrapper does NOT
// translate absolute paths). Empty paths slice is a no-op (no shell-out).
func (g *Git) Add(ctx context.Context, paths ...string) error {
	if len(paths) == 0 {
		return nil
	}
	args := append([]string{"add", "--"}, paths...)
	_, err := g.run(ctx, args...)
	return err
}

// Commit creates a new commit with the given message. Message MAY include a
// blank-line-separated body and trailers; git's `-m` flag preserves them
// when invoked once. Returns the full 40-char SHA of the new commit (via a
// follow-up rev-parse HEAD). Returns an error if no changes are staged.
func (g *Git) Commit(ctx context.Context, message string) (string, error) {
	// We use `--no-verify` is NOT set — pre-commit hooks DO run by design.
	// Honouring the user's hooks is the whole reason we shell out to git.
	if _, err := g.run(ctx, "commit", "-m", message); err != nil {
		return "", err
	}
	return g.HeadSHA(ctx)
}

// HeadSHA returns the full 40-char SHA at HEAD via `git rev-parse HEAD`.
// Errors out on an empty (unborn-HEAD) repository.
func (g *Git) HeadSHA(ctx context.Context) (string, error) {
	out, err := g.run(ctx, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}
