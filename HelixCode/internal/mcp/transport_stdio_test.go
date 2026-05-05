package mcp

import (
	"context"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func buildEchoServer(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "echo-mcp-server")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", bin, "./testhelper_echo_server")
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
	return bin
}

func TestStdioTransport_RoundTrip(t *testing.T) {
	bin := buildEchoServer(t)
	tr := NewStdioTransport(StdioConfig{
		Command: []string{bin},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, tr.Open(ctx))
	defer tr.Close()

	require.NoError(t, tr.Send(ctx, &MCPMessage{
		JSONRPC: "2.0",
		ID:      "1",
		Method:  "ping",
	}))
	resp, err := tr.Recv(ctx)
	require.NoError(t, err)
	assert.Equal(t, "1", resp.ID)
	assert.Equal(t, TransportStdio, tr.Type())
}

func TestStdioTransport_StderrCapture(t *testing.T) {
	bin := buildEchoServer(t)
	tr := NewStdioTransport(StdioConfig{Command: []string{bin}})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, tr.Open(ctx))
	time.Sleep(200 * time.Millisecond)
	stderr := tr.Stderr()
	assert.Contains(t, string(stderr), "echo-mcp-server: ready")
	require.NoError(t, tr.Close())
}

func TestStdioTransport_CloseKillsProcess(t *testing.T) {
	bin := buildEchoServer(t)
	tr := NewStdioTransport(StdioConfig{Command: []string{bin}})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, tr.Open(ctx))
	pid := tr.PID()
	require.NotZero(t, pid)
	require.NoError(t, tr.Close())
	err := tr.Send(ctx, &MCPMessage{JSONRPC: "2.0", ID: "x", Method: "ping"})
	assert.Error(t, err)
}
