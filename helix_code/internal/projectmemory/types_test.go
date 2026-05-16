// Package projectmemory — types_test.go (P2-F24-T02).
//
// TDD pin tests for the Memory value type, MemorySource enum, sentinels,
// and constants. Every assertion here is byte-for-byte; changing a value
// is a breaking change for downstream consumers (BaseAgent's prompt
// prepend, /memory status formatting, MemoryWatcher's debounce window).
package projectmemory

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMaxMemoryBytes_Pin(t *testing.T) {
	require.Equal(t, 64*1024, MaxMemoryBytes)
}

func TestDebounceWindow_Pin(t *testing.T) {
	require.Equal(t, 200*time.Millisecond, DebounceWindow)
}

func TestDiscoveryFilenames_OrderPin(t *testing.T) {
	// Order is load-bearing: helixcode.md first (project brand), codex.md
	// second (codex shim), AGENTS.md third (cross-tool generic).
	require.Equal(t, []string{"helixcode.md", "codex.md", "AGENTS.md"}, DiscoveryFilenames)
}

func TestMemorySource_Constants(t *testing.T) {
	require.Equal(t, MemorySource("project"), SourceProject)
	require.Equal(t, MemorySource("user"), SourceUser)
}

func TestErrorSentinels_DistinctErrorsIs(t *testing.T) {
	for _, e := range []error{ErrNoMemoryFile, ErrMemoryFileTooLarge} {
		wrapped := fmt.Errorf("wrapped: %w", e)
		require.ErrorIs(t, wrapped, e)
	}
	// Distinct sentinels.
	require.False(t, errors.Is(ErrNoMemoryFile, ErrMemoryFileTooLarge))
}

func TestMemoryRender_BothEmpty_ReturnsEmpty(t *testing.T) {
	require.Equal(t, "", Memory{}.Render())
}

func TestMemoryRender_OnlyProject(t *testing.T) {
	require.Equal(t, "p", Memory{Project: "p"}.Render())
}

func TestMemoryRender_OnlyUser(t *testing.T) {
	require.Equal(t, "u", Memory{User: "u"}.Render())
}

func TestMemoryRender_BothPresent_ProjectBeforeUser(t *testing.T) {
	out := Memory{Project: "PROJECT_BODY", User: "USER_BODY"}.Render()
	require.Contains(t, out, "PROJECT_BODY")
	require.Contains(t, out, "USER_BODY")
	// Project bytes must precede user bytes.
	require.Less(t, strings.Index(out, "PROJECT_BODY"), strings.Index(out, "USER_BODY"))
	// Delimiter must be present and pinned exactly.
	require.Contains(t, out, "\n\n--- USER MEMORY OVERLAY ---\n\n")
}

func TestMemoryRender_DelimiterExactString(t *testing.T) {
	out := Memory{Project: "P", User: "U"}.Render()
	require.Equal(t, "P\n\n--- USER MEMORY OVERLAY ---\n\nU", out)
}

func TestMemory_FieldsZeroValueSafe(t *testing.T) {
	// A zero-value Memory must be safe to .Render() and yield empty string.
	var m Memory
	require.Equal(t, "", m.Render())
	require.Equal(t, "", m.Project)
	require.Equal(t, "", m.User)
	require.Equal(t, "", m.ProjectPath)
	require.Equal(t, "", m.UserPath)
	require.False(t, m.TruncatedProject)
	require.False(t, m.TruncatedUser)
	require.True(t, m.LoadedAt.IsZero())
}
