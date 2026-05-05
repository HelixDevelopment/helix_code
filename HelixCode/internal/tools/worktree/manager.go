package worktree

import (
	"fmt"
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
