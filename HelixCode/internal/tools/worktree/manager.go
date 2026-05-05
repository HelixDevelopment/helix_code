package worktree

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Manager owns worktree state for a repository.
//
// repoRoot is the absolute path to the main worktree (git rev-parse
// --show-toplevel). currentWorktree is the absolute path of the active
// isolated worktree, or "" when the agent is in the main worktree.
type Manager struct {
	repoRoot        string
	currentWorktree string
	mu              sync.RWMutex
}

// NewManager creates a Manager rooted at repoRoot. Performs no I/O.
func NewManager(repoRoot string) *Manager {
	return &Manager{repoRoot: repoRoot}
}

// ValidateName rejects empty / too-long / non-conforming worktree names.
// The pattern matches claude-code's own validation (`^[a-zA-Z0-9._-]+$`).
func (m *Manager) ValidateName(name string) error {
	if name == "" {
		return fmt.Errorf("worktree name cannot be empty")
	}
	if len(name) > WorktreeNameMaxLength {
		return fmt.Errorf("worktree name exceeds %d characters", WorktreeNameMaxLength)
	}
	// Reject directory traversal attempts
	if name == ".." || name == "." {
		return fmt.Errorf("worktree name %q does not match pattern %s", name, WorktreeNameRegex)
	}
	if !worktreeNamePattern.MatchString(name) {
		return fmt.Errorf("worktree name %q does not match pattern %s", name, WorktreeNameRegex)
	}
	return nil
}

// GetCurrentDirectory returns the effective working directory: the active
// worktree's path if isolated, otherwise the main repo root.
func (m *Manager) GetCurrentDirectory() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.currentWorktree != "" {
		return m.currentWorktree
	}
	return m.repoRoot
}

// IsIsolated reports whether the Manager is currently inside a worktree
// (set by EnterWorktree, cleared by ExitWorktree).
func (m *Manager) IsIsolated() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentWorktree != ""
}

// EnterWorktree switches into a named worktree, creating it if necessary.
// If baseBranch is empty, the worktree's branch defaults to name.
//
// Behaviour:
//   - Validates name via ValidateName.
//   - If <repoRoot>/.helix-worktrees/<name>/ exists:
//   - Verifies clean (git status --porcelain empty); rejects with error
//     if dirty.
//   - Updates currentWorktree and returns the path.
//   - Otherwise:
//   - Creates the parent dir.
//   - Tries `git worktree add <path> <branch>` (existing branch).
//   - On failure, falls back to `git worktree add -b <branch> <path>`
//     (new branch).
//   - On second failure, returns a composite error with both outputs.
func (m *Manager) EnterWorktree(ctx context.Context, name, baseBranch string) (string, error) {
	if err := m.ValidateName(name); err != nil {
		return "", err
	}

	branch := baseBranch
	if branch == "" {
		branch = name
	}

	path := filepath.Join(m.repoRoot, WorktreeDir, name)

	m.mu.Lock()
	defer m.mu.Unlock()

	// Reuse existing worktree if present and clean.
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		out, sErr := gitStatusPorcelain(ctx, path)
		if sErr != nil {
			return "", fmt.Errorf("checking worktree status: %w", sErr)
		}
		if strings.TrimSpace(string(out)) != "" {
			return "", fmt.Errorf("worktree %q has uncommitted changes — clean or remove first", name)
		}
		m.currentWorktree = path
		return path, nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("creating worktree parent dir: %w", err)
	}

	// Try existing branch first.
	if out, err := gitWorktreeAdd(ctx, m.repoRoot, branch, path); err != nil {
		// Fall back to creating a new branch.
		if out2, err2 := gitWorktreeAddNewBranch(ctx, m.repoRoot, branch, path); err2 != nil {
			return "", fmt.Errorf(
				"creating worktree (existing-branch attempt: %s) (new-branch attempt: %s): %w",
				strings.TrimSpace(string(out)), strings.TrimSpace(string(out2)), err2,
			)
		}
	}

	m.currentWorktree = path
	return path, nil
}
