package worktree

import "regexp"

// Constants control validation, on-disk location, and invariants.
const (
	// WorktreeNameRegex constrains worktree names to alphanumerics + . _ - .
	// Matches claude-code's own validation pattern.
	WorktreeNameRegex = `^[a-zA-Z0-9._-]+$`

	// WorktreeNameMaxLength is the maximum allowed name length in bytes.
	WorktreeNameMaxLength = 64

	// WorktreeDir is the relative path under repoRoot for worktree checkouts.
	WorktreeDir = ".helix-worktrees"
)

// worktreeNamePattern is the compiled regex used by ValidateName.
var worktreeNamePattern = regexp.MustCompile(WorktreeNameRegex)

// Worktree describes a single isolated checkout.
//
// Path is the absolute path to the worktree on disk (under
// <repoRoot>/.helix-worktrees/<name>). Branch is best-effort and may be
// empty if the worktree has a detached HEAD.
type Worktree struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Branch string `json:"branch,omitempty"`
}
