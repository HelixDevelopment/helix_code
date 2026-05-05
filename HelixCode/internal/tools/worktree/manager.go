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

// CreateWorktreeForSubagent creates a new git worktree for a subagent without
// mutating the manager's currentWorktree state. Returns the absolute path to
// the new worktree and a cleanup closure that removes the worktree directory
// (and unregisters its git metadata). Used by the F15 subagent system, where
// the parent agent's location must NOT change when a subagent is dispatched.
//
// Behaviour mirrors EnterWorktree's worktree-creation path minus the state
// assignment:
//   - Validates name via ValidateName.
//   - If <repoRoot>/.helix-worktrees/<name>/ already exists and is clean,
//     returns it (the caller's cleanup will still remove it).
//   - Otherwise creates the parent dir, tries `git worktree add <path>
//     <branch>`, falling back to `git worktree add -b <branch> <path>` on
//     failure.
//   - baseBranch == "" defaults the branch to name (matches EnterWorktree).
//
// The returned cleanup func is idempotent and safe to call from a deferred
// statement: it runs `git worktree remove -f <path>` (force) so dirty changes
// inside the worktree do not block teardown.
//
// Cross-references: P1-F15-T06.
func (m *Manager) CreateWorktreeForSubagent(ctx context.Context, name, baseBranch string) (string, func() error, error) {
	if err := m.ValidateName(name); err != nil {
		return "", nil, err
	}

	branch := baseBranch
	if branch == "" {
		branch = name
	}

	path := filepath.Join(m.repoRoot, WorktreeDir, name)

	// We do NOT touch m.currentWorktree here, so we only need the lock to
	// serialize concurrent creates against the same path. Use the same lock
	// as EnterWorktree for parity.
	m.mu.Lock()
	defer m.mu.Unlock()

	// Reuse existing clean worktree if present.
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		out, sErr := gitStatusPorcelain(ctx, path)
		if sErr != nil {
			return "", nil, fmt.Errorf("checking worktree status: %w", sErr)
		}
		if strings.TrimSpace(string(out)) != "" {
			return "", nil, fmt.Errorf("worktree %q has uncommitted changes — clean or remove first", name)
		}
		return path, m.subagentCleanupFunc(path), nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", nil, fmt.Errorf("creating worktree parent dir: %w", err)
	}

	// Try existing branch first.
	if out, err := gitWorktreeAdd(ctx, m.repoRoot, branch, path); err != nil {
		// Fall back to creating a new branch.
		if out2, err2 := gitWorktreeAddNewBranch(ctx, m.repoRoot, branch, path); err2 != nil {
			return "", nil, fmt.Errorf(
				"creating subagent worktree (existing-branch attempt: %s) (new-branch attempt: %s): %w",
				strings.TrimSpace(string(out)), strings.TrimSpace(string(out2)), err2,
			)
		}
	}

	return path, m.subagentCleanupFunc(path), nil
}

// subagentCleanupFunc returns an idempotent closure that removes the worktree
// at path via `git worktree remove -f`. The closure does NOT touch
// m.currentWorktree (subagent worktrees were never registered there).
func (m *Manager) subagentCleanupFunc(path string) func() error {
	var done bool
	return func() error {
		if done {
			return nil
		}
		done = true
		// Use a background ctx because cleanup may run after the original
		// dispatch ctx has been canceled (e.g. timeout / kill); we still
		// want the directory gone.
		ctx := context.Background()
		if _, err := gitWorktreeRemove(ctx, m.repoRoot, path, true); err != nil {
			// Force-remove failed; fall back to raw rmdir so we don't leak
			// the directory. Git metadata may linger until the next
			// `git worktree prune`, which is acceptable.
			if rmErr := os.RemoveAll(path); rmErr != nil {
				return fmt.Errorf("subagent worktree cleanup: git remove failed (%v) and rmdir failed: %w", err, rmErr)
			}
		}
		return nil
	}
}

// ExitWorktree clears the active-worktree state and returns the agent to
// the main worktree. Idempotent: calling on a non-isolated Manager is a
// no-op.
func (m *Manager) ExitWorktree() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.currentWorktree = ""
}

// ListWorktrees returns all helix-managed worktrees (the directory entries
// under <repoRoot>/.helix-worktrees/). Files in the WorktreeDir are
// ignored — only subdirectories count.
//
// The Branch field is best-effort: it parses `git worktree list --porcelain`
// output to associate paths with branches. If parsing fails for any entry,
// Branch is left empty for that entry.
func (m *Manager) ListWorktrees(ctx context.Context) ([]Worktree, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	dir := filepath.Join(m.repoRoot, WorktreeDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading %s: %w", dir, err)
	}

	branchByPath := parseWorktreeBranches(ctx, m.repoRoot)

	var out []Worktree
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		full := filepath.Join(dir, entry.Name())
		out = append(out, Worktree{
			Name:   entry.Name(),
			Path:   full,
			Branch: branchByPath[full],
		})
	}
	return out, nil
}

// parseWorktreeBranches returns a map from worktree absolute path to its
// current branch name, derived from `git worktree list --porcelain`. On
// any parse error, returns whatever was successfully parsed (best-effort).
func parseWorktreeBranches(ctx context.Context, repoRoot string) map[string]string {
	out, err := gitWorktreeList(ctx, repoRoot)
	if err != nil {
		return nil
	}
	branches := map[string]string{}
	var curPath, curBranch string
	for _, line := range strings.Split(string(out), "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			if curPath != "" {
				branches[curPath] = curBranch
			}
			curPath = strings.TrimPrefix(line, "worktree ")
			curBranch = ""
		case strings.HasPrefix(line, "branch "):
			curBranch = strings.TrimPrefix(strings.TrimPrefix(line, "branch "), "refs/heads/")
		}
	}
	if curPath != "" {
		branches[curPath] = curBranch
	}
	return branches
}

// RemoveWorktree deletes the worktree directory and unregisters its git
// metadata. Refuses to remove the currently-active worktree (call
// ExitWorktree first). On `git worktree remove` failure, retries with -f
// before returning a composite error.
func (m *Manager) RemoveWorktree(ctx context.Context, name string) error {
	if err := m.ValidateName(name); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	path := filepath.Join(m.repoRoot, WorktreeDir, name)
	if path == m.currentWorktree {
		return fmt.Errorf("cannot remove the current worktree; ExitWorktree first")
	}

	if out, err := gitWorktreeRemove(ctx, m.repoRoot, path, false); err != nil {
		// Retry with -f.
		if out2, err2 := gitWorktreeRemove(ctx, m.repoRoot, path, true); err2 != nil {
			return fmt.Errorf(
				"removing worktree (without -f: %s) (with -f: %s): %w",
				strings.TrimSpace(string(out)), strings.TrimSpace(string(out2)), err2,
			)
		}
	}
	return nil
}
