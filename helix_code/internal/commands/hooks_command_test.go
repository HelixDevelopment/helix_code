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
	// CONST-046 round-416: the per-hook result line routes through
	// tr() with the hook ID as a named placeholder. Wire a real
	// localiser via the sentinel so the placeholder interpolation is
	// observable; the registered hook ID "hk" must surface in the
	// rendered line.
	resetTranslator(t)
	SetTranslator(interpolatingTranslator{})
	defer resetTranslator(t)

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

// --- CONST-046 round-416 paired-mutation tests ---
//
// With sentinelTranslator wired, every migrated /hooks user-facing
// literal MUST surface the sentinel-wrapped message ID. Re-inlining
// any literal fails these assertions (§11.4 anti-bluff).

func TestHooksCommand_DescriptionUsage_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	cmd := NewHooksCommand(setupHooksManager(t))
	assert.Equal(t, "<TR:internal_commands_hooks_description>", cmd.Description())
	assert.Equal(t, "<TR:internal_commands_hooks_usage>", cmd.Usage())
}

func TestHooksCommand_ListHeaders_GoThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	cmd := NewHooksCommand(setupHooksManager(t))
	res, err := cmd.Execute(context.Background(), &CommandContext{Args: []string{"list"}})
	require.NoError(t, err)
	for _, id := range []string{
		"<TR:internal_commands_hooks_col_id>",
		"<TR:internal_commands_hooks_col_event>",
		"<TR:internal_commands_hooks_col_priority>",
		"<TR:internal_commands_hooks_col_async>",
		"<TR:internal_commands_hooks_col_enabled>",
	} {
		assert.Contains(t, res.Output, id)
	}
}

func TestHooksCommand_TestResult_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	cmd := NewHooksCommand(setupHooksManager(t))
	res, err := cmd.Execute(context.Background(), &CommandContext{
		Args: []string{"test", "before_tool_call"},
	})
	require.NoError(t, err)
	assert.Contains(t, res.Output, "<TR:internal_commands_hooks_test_result>")
}

func TestHooksCommand_UnknownSubaction_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	cmd := NewHooksCommand(setupHooksManager(t))
	_, err := cmd.Execute(context.Background(), &CommandContext{Args: []string{"frobnicate"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "<TR:internal_commands_hooks_unknown_subaction>")
}
