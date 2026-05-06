// Package projectmemory — types.go (P2-F24-T02).
//
// Defines the immutable Memory value type, the MemorySource enum, sentinel
// errors, and tunable constants. All values pinned by tests in types_test.go;
// changing them is a breaking change for downstream code (BaseAgent's
// system-prompt prepend, /memory slash output formatting, MemoryWatcher's
// debounce window).
package projectmemory

import (
	"errors"
	"time"
)

// MaxMemoryBytes caps the size of either the project memory file OR the
// user overlay file. A file larger than this is read-truncated to exactly
// MaxMemoryBytes bytes and the corresponding Truncated* flag is set so the
// user can detect (via /memory status) that truncation happened. 64 KB
// matches F23's MaxSnapshotBytes for codebase symmetry; ~10–15 K tokens
// is a sane upper bound for an LLM's context budget.
const MaxMemoryBytes = 64 * 1024

// DebounceWindow is the fsnotify-event coalescing window for MemoryWatcher.
// Atomic-write editors (vim's :w, emacs's save-buffer) emit 3-5 events in
// ~50 ms; 200 ms coalesces them into one Reload. Lower would over-fire;
// higher would feel laggy to the user.
const DebounceWindow = 200 * time.Millisecond

// DiscoveryFilenames is the ordered list of project-memory file basenames
// that MemoryLoader.Discover searches for during the parent-walk. First
// match wins. Order is significant: helixcode.md is the project's own
// brand; codex.md is the codex compatibility shim; AGENTS.md is the
// cross-tool generic. Tests pin this exact slice.
var DiscoveryFilenames = []string{"helixcode.md", "codex.md", "AGENTS.md"}

// MemorySource identifies which source a piece of memory came from. Used
// in logs and /memory status output to distinguish project-level vs user-
// level overrides.
type MemorySource string

const (
	SourceProject MemorySource = "project"
	SourceUser    MemorySource = "user"
)

// Sentinel errors. Wrapped via fmt.Errorf("...: %w", err) by callers.
var (
	// ErrNoMemoryFile signals that no project memory file was found during
	// the parent-walk. NOTE: MemoryLoader.Discover does NOT return this
	// error — missing files are normal and yield an empty Memory{}. The
	// sentinel exists for callers that want to assert "no memory" as a
	// distinct condition (e.g. /memory edit creating a fresh file).
	ErrNoMemoryFile = errors.New("projectmemory: no memory file found")

	// ErrMemoryFileTooLarge signals that a memory file exceeded
	// MaxMemoryBytes. NOTE: MemoryLoader.Discover does NOT return this
	// error either — it truncates and sets the Truncated* flag instead.
	// The sentinel exists for stricter callers that want to refuse rather
	// than silently truncate.
	ErrMemoryFileTooLarge = errors.New("projectmemory: memory file exceeds MaxMemoryBytes")
)

// Memory is the immutable result of MemoryLoader.Discover. All fields are
// snapshot-at-load-time; a subsequent Reload returns a new Memory (the
// registry's atomic-pointer swaps in the new value). Empty Project / User
// strings are valid and signal "no file present" (NOT an error).
type Memory struct {
	// Project is the raw bytes of the resolved project memory file. Empty
	// when no file was found during the parent-walk. Capped at
	// MaxMemoryBytes; TruncatedProject is true when capping happened.
	Project string

	// User is the raw bytes of the user overlay file at
	// $XDG_CONFIG_HOME/helixcode/memory.md. Empty when no file was found.
	// Capped at MaxMemoryBytes; TruncatedUser is true when capping happened.
	User string

	// ProjectPath is the absolute path of the discovered project memory
	// file. Empty when not found.
	ProjectPath string

	// UserPath is the absolute path of the user overlay file. Empty when
	// not found.
	UserPath string

	// LoadedAt is wall-clock time when MemoryLoader.Discover completed.
	// Used by /memory status to show the last-loaded timestamp.
	LoadedAt time.Time

	// TruncatedProject is true when the project memory file was larger
	// than MaxMemoryBytes and Project was truncated to the cap. Tests
	// require this flag to be SET whenever truncation happens — silent
	// truncation is a documented anti-bluff pattern (spec §5.2 Bluff #4).
	TruncatedProject bool

	// TruncatedUser is true when the user overlay file was truncated.
	TruncatedUser bool
}

// Render returns the canonical concatenation of Project + User content
// suitable for prepending to the agent's system prompt. Order is
// load-bearing: Project first, User second, separated by an ALL-CAPS
// delimiter so the LLM can distinguish source. When BOTH fields are
// empty, Render returns the empty string (the caller should NOT prepend
// anything to the system prompt in that case). The delimiter is
// "\n\n--- USER MEMORY OVERLAY ---\n\n"; tests pin this exact string.
//
// Project-before-user order is a security invariant: project memory is
// the project's contract; user memory is the user's preferences. Reverse
// order would let a user-level "disable approval gates" instruction
// silently override a project-level "always require approval" mandate.
func (m Memory) Render() string {
	if m.Project == "" && m.User == "" {
		return ""
	}
	if m.User == "" {
		return m.Project
	}
	if m.Project == "" {
		return m.User
	}
	return m.Project + "\n\n--- USER MEMORY OVERLAY ---\n\n" + m.User
}
