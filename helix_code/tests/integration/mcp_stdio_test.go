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
	"dev.helix.code/internal/tools"
)

func buildEchoBinaryForIT(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "echo")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	// Use module-qualified path so this resolves correctly regardless of cwd.
	pkg := "dev.helix.code/internal/mcp/testhelper_echo_server"
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
	assert.Equal(t, 1, st[0].ToolCount, "echo server must report 1 tool after handshake")

	// Echo server now returns a real tool for tools/list; exercise CallTool path.
	res, err := mgr.CallTool(ctx, "echo", "echo", map[string]any{"text": "hello"})
	require.NoError(t, err)
	require.NotNil(t, res)
	b, _ := json.Marshal(res.Raw)
	t.Logf("response: %s", b)
	_ = os.WriteFile(filepath.Join(cfgDir, "evidence.json"), b, 0644)
}

// TestMCP_Stdio_ToolRegistryAdapter exercises the full agent path:
// Manager → tools.ToolRegistry.RegisterMCPManager → mcpTool.Execute → CallTool.
func TestMCP_Stdio_ToolRegistryAdapter(t *testing.T) {
	bin := buildEchoBinaryForIT(t)
	cfg := &mcp.Config{
		Servers: []mcp.ServerSpec{
			{Name: "echo", Transport: mcp.TransportStdio, Command: []string{bin}, AlwaysLoad: true},
		},
	}
	mgr := mcp.NewManager()
	mgr.SetConfig(cfg)
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
		time.Sleep(50 * time.Millisecond)
	}
	st := mgr.Status()
	require.Len(t, st, 1)
	require.Equal(t, mcp.StateReady, st[0].State)
	require.Equal(t, 1, st[0].ToolCount, "echo server must report 1 tool")

	// Register tools into the ToolRegistry
	reg, err := tools.NewToolRegistry(tools.DefaultRegistryConfig())
	require.NoError(t, err)
	reg.RegisterMCPManager(mgr)

	// Confirm the echo tool was registered under its OpenAI/DeepSeek-compatible
	// registry key. HXC-113 (commit 8f203793) deliberately changed MCP tool
	// names from "server:name" to "server__name" so they match the LLM
	// function-name grammar ^[A-Za-z0-9_-]+$ (a colon causes a 400 from
	// OpenAI-compatible providers). The registry key for the ("echo","echo")
	// pair is therefore "echo__echo", not the old "echo:echo".
	const echoToolKey = "echo__echo"
	tool, err := reg.Get(echoToolKey)
	require.NoError(t, err)
	require.NotNil(t, tool)
	assert.Equal(t, echoToolKey, tool.Name())

	// Execute through the adapter — this exercises mcpTool.Execute → CallTool
	result, execErr := tool.Execute(ctx, map[string]any{"text": "hello"})
	require.NoError(t, execErr)
	require.NotNil(t, result)
	b, _ := json.Marshal(result)
	t.Logf("mcpTool.Execute result: %s", b)
}
