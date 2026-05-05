package shell

import (
	"context"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func skipIfWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("SKIP-OK: shell test relies on POSIX bash; CI runs on Linux only for now")
	}
}

// newShellToolInstance returns a ShellExecutor configured for unit tests
// using the permissive config so sandbox/security doesn't block echo/sleep/exit.
func newShellToolInstance() *ShellExecutor {
	return NewShellExecutor(PermissiveConfig())
}

// LineSink mirrors the tools.LineSink type locally to avoid an import cycle
// (dev.helix.code/internal/tools imports dev.helix.code/internal/tools/shell,
// so the shell package must not import tools).
type LineSink func(line string)

func TestShellBackgroundAware_StreamsLines(t *testing.T) {
	skipIfWindows(t)

	tool := newShellToolInstance()

	var mu sync.Mutex
	var got []string
	sink := LineSink(func(line string) {
		mu.Lock()
		got = append(got, line)
		mu.Unlock()
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := tool.ExecuteWithProgress(ctx, map[string]interface{}{
		"command": "echo a; echo b; echo c",
	}, sink)
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, []string{"a", "b", "c"}, got)
}

func TestShellBackgroundAware_ContextCancelKillsProcess(t *testing.T) {
	skipIfWindows(t)

	tool := newShellToolInstance()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := tool.ExecuteWithProgress(ctx, map[string]interface{}{
			"command": "sleep 30",
		}, func(string) {})
		done <- err
	}()
	time.Sleep(100 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		require.Error(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("subprocess did not exit within 3s after ctx cancel")
	}
}

func TestShellBackgroundAware_ExitNonZeroIsErrorButCompletes(t *testing.T) {
	skipIfWindows(t)

	tool := newShellToolInstance()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err := tool.ExecuteWithProgress(ctx, map[string]interface{}{
		"command": "exit 7",
	}, func(string) {})
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "exit")
}
