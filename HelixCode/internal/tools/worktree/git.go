package worktree

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// gitRevParseToplevel returns the absolute path of the git repository
// containing cwd. Errors if cwd is not inside a git repo.
func gitRevParseToplevel(ctx context.Context, cwd string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--show-toplevel")
	cmd.Dir = cwd
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git rev-parse --show-toplevel: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

// gitWorktreeAdd attaches an existing branch to a new worktree at path.
// Returns the combined git output and any error. The output is preserved
// even on failure so the caller can decide whether to retry with -b.
func gitWorktreeAdd(ctx context.Context, repoRoot, branch, path string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", "worktree", "add", path, branch)
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("git worktree add %s %s: %w", path, branch, err)
	}
	return out, nil
}

// gitWorktreeAddNewBranch creates a new branch and a worktree in one shot
// (git worktree add -b <branch> <path>). Used as the fallback when the
// branch doesn't already exist.
func gitWorktreeAddNewBranch(ctx context.Context, repoRoot, branch, path string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", "worktree", "add", "-b", branch, path)
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("git worktree add -b %s %s: %w", branch, path, err)
	}
	return out, nil
}

// gitWorktreeRemove removes the worktree at path. If force is true, passes
// the -f flag to allow removal of dirty / locked worktrees.
func gitWorktreeRemove(ctx context.Context, repoRoot, path string, force bool) ([]byte, error) {
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "-f")
	}
	args = append(args, path)
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("git worktree remove (force=%v) %s: %w", force, path, err)
	}
	return out, nil
}

// gitWorktreeList runs `git worktree list --porcelain` and returns the raw
// output. The Manager parses this when populating ListWorktrees.
func gitWorktreeList(ctx context.Context, repoRoot string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", "worktree", "list", "--porcelain")
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("git worktree list: %w", err)
	}
	return out, nil
}

// gitStatusPorcelain runs `git status --porcelain` in dir and returns the raw
// output. Empty output means clean.
func gitStatusPorcelain(ctx context.Context, dir string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", "status", "--porcelain")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("git status --porcelain: %w", err)
	}
	return out, nil
}
