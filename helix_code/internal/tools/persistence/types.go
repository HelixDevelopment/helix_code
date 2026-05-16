package persistence

import "time"

// Constants control persistence threshold, on-disk location, and cleanup window.
const (
	// PersistThreshold is the byte count above which outputs are persisted.
	// A result with len([]byte(output)) > PersistThreshold is persisted.
	// A result with len(...) == PersistThreshold stays inline (boundary is
	// strictly greater than).
	PersistThreshold = 50_000

	// PersistDir is the relative path under projectRoot for persisted outputs.
	PersistDir = ".helix/tool-results"

	// DefaultMaxAge is the default cleanup window for CleanupOld.
	DefaultMaxAge = 7 * 24 * time.Hour
)

// PersistedResult represents a tool result, either inline or persisted-to-disk.
//
// Output and PersistedOutputPath are mutually exclusive. WasPersisted is
// the canonical boolean — providers serialise this struct to a wire format
// by branching on WasPersisted.
type PersistedResult struct {
	Output              string `json:"output,omitempty"`              // empty if persisted
	PersistedOutputPath string `json:"persistedOutputPath,omitempty"` // absolute path on disk
	PersistedOutputSize int    `json:"persistedOutputSize,omitempty"` // byte count of the original content
	WasPersisted        bool   `json:"wasPersisted"`
	ToolName            string `json:"toolName"`
	ToolCallID          string `json:"toolCallID,omitempty"`
}
