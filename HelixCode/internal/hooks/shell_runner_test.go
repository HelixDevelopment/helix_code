package hooks

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeScript(t *testing.T, body string) string {
	t.Helper()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "hook.sh")
	require.NoError(t, os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o755))
	return path
}

func TestShellRunner_ExitZeroIsSuccess(t *testing.T) {
	script := writeScript(t, "exit 0")
	runner := NewShellRunner(script, 0)
	event := NewEvent(HookTypeBeforeToolCall)
	event.SetData("toolName", "Bash")
	err := runner(context.Background(), event)
	assert.NoError(t, err)
}

func TestShellRunner_NonZeroExitIsBlock(t *testing.T) {
	script := writeScript(t, "echo 'blocked!' >&2; exit 1")
	runner := NewShellRunner(script, 0)
	event := NewEvent(HookTypeBeforeBash)
	err := runner(context.Background(), event)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "blocked!", "stderr must surface in error")
}

func TestShellRunner_TimeoutAborts(t *testing.T) {
	script := writeScript(t, "sleep 5")
	runner := NewShellRunner(script, 100*time.Millisecond)
	event := NewEvent(HookTypeOnError)
	start := time.Now()
	err := runner(context.Background(), event)
	elapsed := time.Since(start)
	require.Error(t, err)
	assert.Less(t, elapsed, 2*time.Second, "timeout must fire well before 5s sleep")
}

func TestShellRunner_MissingScriptIsBlock(t *testing.T) {
	runner := NewShellRunner("/nonexistent/script.sh", 0)
	event := NewEvent(HookTypeOnCompaction)
	err := runner(context.Background(), event)
	require.Error(t, err)
}

func TestShellRunner_StdinReceivesEventJSON(t *testing.T) {
	tmp := t.TempDir()
	stdinCapture := filepath.Join(tmp, "captured.json")
	script := writeScript(t, "cat > "+stdinCapture)
	runner := NewShellRunner(script, 0)
	event := NewEvent(HookTypeBeforeToolCall)
	event.Source = "tool_registry"
	event.SetData("toolName", "Bash")
	event.SetData("params", map[string]interface{}{"command": "ls"})
	require.NoError(t, runner(context.Background(), event))

	body, err := os.ReadFile(stdinCapture)
	require.NoError(t, err)
	assert.Contains(t, string(body), `"type":"before_tool_call"`)
	assert.Contains(t, string(body), `"toolName":"Bash"`)
	assert.Contains(t, string(body), `"command":"ls"`)
}

func TestShellRunner_StdoutModifyPayloadMergedIntoEvent(t *testing.T) {
	script := writeScript(t, `echo '{"data":{"injected":"value"}}'`)
	runner := NewShellRunner(script, 0)
	event := NewEvent(HookTypeBeforeToolCall)
	event.SetData("original", "x")
	require.NoError(t, runner(context.Background(), event))
	assert.Equal(t, "x", event.Data["original"], "original keys preserved")
	assert.Equal(t, "value", event.Data["injected"], "stdout JSON merged into event.Data")
}

func TestShellRunner_MalformedStdoutIsLoggedNotBlock(t *testing.T) {
	script := writeScript(t, `echo 'this is not json'; exit 0`)
	runner := NewShellRunner(script, 0)
	event := NewEvent(HookTypeBeforeToolCall)
	err := runner(context.Background(), event)
	assert.NoError(t, err, "malformed stdout JSON must NOT block; only logged")
}

func TestShellRunner_RespectsCallerContextCancel(t *testing.T) {
	script := writeScript(t, "sleep 5")
	runner := NewShellRunner(script, 10*time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()
	event := NewEvent(HookTypeOnError)
	start := time.Now()
	err := runner(ctx, event)
	elapsed := time.Since(start)
	require.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled) || elapsed < 2*time.Second,
		"caller cancel must abort run; got err=%v elapsed=%s", err, elapsed)
}
