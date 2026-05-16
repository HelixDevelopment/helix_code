package tools

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestShellTool_BackgroundAwareInterface verifies that *ShellTool obtained from
// the ToolRegistry satisfies the BackgroundAware interface at runtime (the
// compile-time assertion in shell_tools.go guarantees this won't regress
// silently at build time).
func TestShellTool_BackgroundAwareInterface(t *testing.T) {
	// Compile-time assertion is at package scope; this verifies at runtime via
	// type assertion through the Tool interface.
	var tool Tool
	var err error
	tool, err = newShellToolForBackgroundTest(t)
	require.NoError(t, err)

	_, ok := tool.(BackgroundAware)
	require.True(t, ok, "*ShellTool must satisfy BackgroundAware via type assertion")
}

func TestShellTool_ExecuteWithProgressStreams(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("SKIP-OK: shell test relies on POSIX bash; CI runs on Linux only for now")
	}

	tool, err := newShellToolForBackgroundTest(t)
	require.NoError(t, err)
	ba, ok := tool.(BackgroundAware)
	require.True(t, ok)

	var mu sync.Mutex
	var got []string
	sink := LineSink(func(line string) {
		mu.Lock()
		got = append(got, line)
		mu.Unlock()
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = ba.ExecuteWithProgress(ctx, map[string]interface{}{
		"command": "echo a; echo b; echo c",
	}, sink)
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, []string{"a", "b", "c"}, got)
}

// newShellToolForBackgroundTest constructs a *ShellTool via the ToolRegistry,
// retrieving it by its registered name "shell".
func newShellToolForBackgroundTest(t *testing.T) (Tool, error) {
	t.Helper()
	reg, err := NewToolRegistry(DefaultRegistryConfig())
	if err != nil {
		return nil, fmt.Errorf("construct tool registry: %w", err)
	}
	// ShellTool registers itself under the name "shell" (see Name() method and
	// registerAllTools in registry.go).
	tool, gerr := reg.Get("shell")
	if gerr != nil {
		return nil, fmt.Errorf("could not locate shell tool in registry: %w", gerr)
	}
	return tool, nil
}
