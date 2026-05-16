// worktree_integration.go — P1-F15-T06.
//
// WorktreeIntegration adapts the F04 worktree system (internal/tools/worktree)
// for F15 subagent dispatch. The SubagentManager (T05) calls Setup before
// dispatching a worktree-isolated subagent so that:
//
//  1. A real git worktree exists under <repoRoot>/.helix-worktrees/<name>/.
//  2. The subprocess spawner can use that path as cmd.Dir for the child
//     process — giving the subagent its own filesystem view.
//  3. The parent agent's view (worktree.Manager.currentWorktree) is NOT
//     mutated. This is the anti-bluff invariant: dispatching a subagent must
//     never silently relocate the parent.
//
// The companion CaptureDiff helper is invoked after the subagent terminates
// (success, failure, timeout, or kill) so SubagentResult.WorktreeDiff carries
// real evidence of what the subagent actually changed. The diff is captured
// BEFORE cleanup runs, otherwise the worktree directory would be gone.
//
// Spec: docs/superpowers/specs/2026-05-06-p1-f15-subagent-team-design.md §4.5
// Plan: docs/superpowers/plans/2026-05-06-p1-f15-subagent-team.md T06
package subagent

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
)

// WorktreeProvider is the subset of *worktree.Manager that the subagent
// integration depends on. Defining the interface here (rather than importing
// the concrete type at every call site) keeps the integration testable with a
// fake provider that records call args.
type WorktreeProvider interface {
	// CreateWorktreeForSubagent creates a new isolated git worktree without
	// mutating the provider's parent-agent state. Returns the absolute path
	// to the worktree and a cleanup closure that the caller MUST invoke
	// when the subagent terminates.
	CreateWorktreeForSubagent(ctx context.Context, name, baseBranch string) (string, func() error, error)
}

// WorktreeIntegration wires F04's WorktreeManager into the F15 subagent
// dispatch path. It is a thin adapter — no caching, no goroutines, no shared
// state — so callers can construct one per dispatch or share a single
// instance across the manager lifetime; either is safe.
type WorktreeIntegration struct {
	// Provider is the F04 worktree manager (or a test fake). Construction
	// via NewWorktreeIntegration ensures Provider is non-nil; direct struct
	// construction with a nil Provider will panic on Setup.
	Provider WorktreeProvider
}

// NewWorktreeIntegration constructs the integration with a real F04 manager
// (or any WorktreeProvider implementation). Returns nil when provider is nil
// — callers MUST check the return value before using it. This matches the
// documented contract enforced by TestWorktreeIntegration_NilProviderConstructor.
func NewWorktreeIntegration(provider WorktreeProvider) *WorktreeIntegration {
	if provider == nil {
		return nil
	}
	return &WorktreeIntegration{Provider: provider}
}

// subagentWorktreeNamePrefix is prepended to task IDs to form the on-disk
// worktree name. The fixed prefix lets operators identify subagent worktrees
// at a glance and lets `git worktree prune` selectively target them via grep.
const subagentWorktreeNamePrefix = "helixcode-subagent-"

// Setup creates a worktree for the given task. Returns the worktree's
// absolute path and a cleanup closure that the caller MUST invoke when the
// subagent terminates (success or failure). Typical usage:
//
//	workDir, cleanup, err := wi.Setup(ctx, task)
//	if err != nil { return err }
//	defer func() { _ = cleanup() }()
//	// dispatch the subprocess spawner with WorkDir = workDir...
//
// Errors:
//   - returns an error if task.ID is empty (the worktree name would not be
//     unique).
//   - propagates any error from the underlying provider verbatim.
func (w *WorktreeIntegration) Setup(ctx context.Context, task SubagentTask) (string, func() error, error) {
	if task.ID == "" {
		return "", nil, errors.New("subagent: WorktreeIntegration.Setup: task.ID is empty (cannot derive worktree name)")
	}
	name := subagentWorktreeNamePrefix + task.ID
	path, cleanup, err := w.Provider.CreateWorktreeForSubagent(ctx, name, task.BaseBranch)
	if err != nil {
		return "", nil, fmt.Errorf("subagent: WorktreeIntegration.Setup: provider create failed: %w", err)
	}
	return path, cleanup, nil
}

// CaptureDiff runs `git diff HEAD` inside the given worktree and returns the
// captured diff text. Used by the SubagentManager to populate
// SubagentResult.WorktreeDiff with real evidence of subagent changes.
//
// Contract: returns ("", error) when the diff command fails (e.g. workDir is
// not a git checkout, git is not on PATH). Callers SHOULD treat the error as
// best-effort and continue completing the SubagentResult; they MUST NOT block
// subagent completion on diff capture failure. The diff command captures both
// staged and unstaged changes via `git diff HEAD`.
func (w *WorktreeIntegration) CaptureDiff(ctx context.Context, workDir string) (string, error) {
	if workDir == "" {
		return "", errors.New("subagent: WorktreeIntegration.CaptureDiff: workDir is empty")
	}
	cmd := exec.CommandContext(ctx, "git", "diff", "HEAD")
	cmd.Dir = workDir
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("subagent: CaptureDiff: git diff HEAD failed in %s: %w (stderr: %s)",
			workDir, err, stderr.String())
	}
	return stdout.String(), nil
}
