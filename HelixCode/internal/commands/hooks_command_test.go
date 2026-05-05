package commands

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/hooks"
)

func setupHooksManager(t *testing.T) *hooks.Manager {
	t.Helper()
	tmp := t.TempDir()
	scriptPath := filepath.Join(tmp, "hook.sh")
	require.NoError(t, os.WriteFile(scriptPath, []byte("#!/bin/sh\nexit 0\n"), 0o755))
	mgr := hooks.NewManager()
	h := hooks.NewHook("hk", hooks.HookTypeBeforeToolCall, hooks.NewShellRunner(scriptPath, 0))
	require.NoError(t, mgr.Register(h))
	return mgr
}

func TestHooksCommand_NameAliases(t *testing.T) {
	cmd := NewHooksCommand(setupHooksManager(t))
	assert.Equal(t, "hooks", cmd.Name())
	assert.Contains(t, cmd.Aliases(), "hk")
}

func TestHooksCommand_ListSubaction(t *testing.T) {
	mgr := setupHooksManager(t)
	cmd := NewHooksCommand(mgr)
	res, err := cmd.Execute(context.Background(), &CommandContext{
		Args:     []string{},
		RawInput: "/hooks",
	})
	require.NoError(t, err)
	assert.Contains(t, res.Output, "hk", "list output must contain registered hook id")
}

func TestHooksCommand_TestSubaction(t *testing.T) {
	mgr := setupHooksManager(t)
	cmd := NewHooksCommand(mgr)
	res, err := cmd.Execute(context.Background(), &CommandContext{
		Args:     []string{"test", "before_tool_call"},
		RawInput: "/hooks test before_tool_call",
	})
	require.NoError(t, err)
	assert.Contains(t, res.Output, "hk", "hook id must appear in test output")
}

func TestHooksCommand_RejectsUnknownSubaction(t *testing.T) {
	cmd := NewHooksCommand(setupHooksManager(t))
	_, err := cmd.Execute(context.Background(), &CommandContext{
		Args:     []string{"frobnicate"},
		RawInput: "/hooks frobnicate",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown")
}

func TestHooksCommand_TestRequiresEvent(t *testing.T) {
	cmd := NewHooksCommand(setupHooksManager(t))
	_, err := cmd.Execute(context.Background(), &CommandContext{
		Args:     []string{"test"},
		RawInput: "/hooks test",
	})
	require.Error(t, err)
}
