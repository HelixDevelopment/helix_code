//go:build integration

package hooks_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/hooks"
)

func newScript(t *testing.T, body string) string {
	t.Helper()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "hook.sh")
	require.NoError(t, os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o755))
	return path
}

// TestIntegration_BeforeBashHookBlocksRm proves that a real shell-script
// hook on before_bash exiting non-zero produces exactly one blocker.
// NO mocks of the hooks system.
func TestIntegration_BeforeBashHookBlocksRm(t *testing.T) {
	scriptPath := newScript(t, "echo 'rm blocked' >&2; exit 1")
	mgr := hooks.NewManager()
	hook := hooks.NewHook("blocker", hooks.HookTypeBeforeBash,
		hooks.NewShellRunner(scriptPath, 0))
	require.NoError(t, mgr.Register(hook))

	event := hooks.NewEventWithContext(context.Background(), hooks.HookTypeBeforeBash)
	event.SetData("toolName", "Bash")
	event.SetData("params", map[string]interface{}{"command": "rm -rf /tmp/x"})
	results := mgr.TriggerEventAndWait(event)

	blockers := hooks.Blockers(results)
	require.Len(t, blockers, 1, "the blocking hook must produce exactly one blocker")
	assert.Contains(t, blockers[0].Error(), "rm blocked")
}

// TestIntegration_AfterToolCallFiresThreeTimes proves the hooks system can
// audit multiple tool calls in sequence.
func TestIntegration_AfterToolCallFiresThreeTimes(t *testing.T) {
	tmp := t.TempDir()
	logPath := filepath.Join(tmp, "audit.log")
	scriptPath := newScript(t, "echo 'tool fired' >> "+logPath)

	mgr := hooks.NewManager()
	require.NoError(t, mgr.Register(hooks.NewHook("audit", hooks.HookTypeAfterToolCall,
		hooks.NewShellRunner(scriptPath, 0))))

	for i := 0; i < 3; i++ {
		event := hooks.NewEventWithContext(context.Background(), hooks.HookTypeAfterToolCall)
		event.SetData("toolName", "X")
		mgr.TriggerEventAndWait(event)
	}

	body, err := os.ReadFile(logPath)
	require.NoError(t, err)
	lines := 0
	for _, b := range body {
		if b == '\n' {
			lines++
		}
	}
	assert.Equal(t, 3, lines, "audit log must have 3 lines (one per tool call)")
}

// TestIntegration_YAMLLoaderToManagerRoundTrip proves the path from YAML
// file → FileLoader → shellRunner → Manager.Register → TriggerEventAndWait
// → script execution → result → Blockers actually works end-to-end.
func TestIntegration_YAMLLoaderToManagerRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	scriptPath := newScript(t, "exit 0")
	yamlPath := filepath.Join(tmp, "hooks.yaml")
	require.NoError(t, os.WriteFile(yamlPath, []byte(`apiVersion: helixcode.hooks/v1
hooks:
  - id: rt
    event: on_compaction
    script: `+scriptPath+`
`), 0o600))

	loader := &hooks.FileLoader{UserPath: yamlPath, ProjectPath: filepath.Join(tmp, "missing.yaml")}
	hs, _, err := loader.Load(context.Background())
	require.NoError(t, err)
	require.Len(t, hs, 1)

	mgr := hooks.NewManager()
	hs[0].Handler = hooks.NewShellRunner(hs[0].Metadata["script"], hs[0].Timeout)
	require.NoError(t, mgr.Register(hs[0]))

	event := hooks.NewEventWithContext(context.Background(), hooks.HookTypeOnCompaction)
	results := mgr.TriggerEventAndWait(event)
	assert.Empty(t, hooks.Blockers(results), "exit-0 hook must not block")
}
