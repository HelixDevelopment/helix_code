//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/mcp"
)

func buildEchoBinaryForIT(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "echo")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	pkg := "../../internal/mcp/testhelper_echo_server"
	cmd := exec.Command("go", "build", "-o", bin, pkg)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
	return bin
}

// TestMCP_Stdio_FullHandshake exercises Connect → tools/list → tools/call
// against a real subprocess. No mocks. Asserts ready state + non-nil result.
func TestMCP_Stdio_FullHandshake(t *testing.T) {
	bin := buildEchoBinaryForIT(t)
	cfgDir := t.TempDir()
	cfgPath := filepath.Join(cfgDir, "mcp.yml")
	cfg := &mcp.Config{
		Servers: []mcp.ServerSpec{
			{Name: "echo", Transport: mcp.TransportStdio, Command: []string{bin}, AlwaysLoad: true},
		},
	}
	require.NoError(t, mcp.SaveConfig(cfgPath, cfg))

	loaded, err := mcp.LoadConfig(cfgPath)
	require.NoError(t, err)

	mgr := mcp.NewManager()
	mgr.SetConfig(loaded)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	require.NoError(t, mgr.Start(ctx))
	defer mgr.Close() //nolint:errcheck

	// Wait for ready
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		st := mgr.Status()
		if len(st) > 0 && st[0].State == mcp.StateReady {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	st := mgr.Status()
	require.Len(t, st, 1)
	assert.Equal(t, mcp.StateReady, st[0].State, "echo server did not reach ready state")

	// Echo server replies {} to every request — exercise CallTool path.
	res, err := mgr.CallTool(ctx, "echo", "any", map[string]any{"x": 1})
	require.NoError(t, err)
	require.NotNil(t, res)
	b, _ := json.Marshal(res.Raw)
	t.Logf("response: %s", b)
	_ = os.WriteFile(filepath.Join(cfgDir, "evidence.json"), b, 0644)
}
