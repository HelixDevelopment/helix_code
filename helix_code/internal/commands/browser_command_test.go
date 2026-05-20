package commands

import (
	"context"
	"testing"

	"dev.helix.code/internal/tools/browser"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestBrowserCommand_Name(t *testing.T) {
	require.Equal(t, "browser", NewBrowserCommand(nil).Name())
}

func TestBrowserCommand_Description_NonEmpty(t *testing.T) {
	require.NotEmpty(t, NewBrowserCommand(nil).Description())
}

func TestBrowserCommand_Usage_NonEmpty(t *testing.T) {
	require.NotEmpty(t, NewBrowserCommand(nil).Usage())
}

func TestBrowserCommand_Status_Inactive(t *testing.T) {
	mgr := browser.NewBrowserManager(nil, zap.NewNop())
	cmd := NewBrowserCommand(mgr)
	res, err := cmd.Execute(context.Background(), &CommandContext{Args: []string{"status"}})
	require.NoError(t, err)
	require.True(t, res.Success)
	// CONST-046 round-416: status line routes through tr(); the
	// NoopTranslator (test default) returns the message ID verbatim.
	require.Contains(t, res.Output, "internal_commands_browser_status_line")
}

func TestBrowserCommand_Status_NoArgs_DefaultsToStatus(t *testing.T) {
	mgr := browser.NewBrowserManager(nil, zap.NewNop())
	cmd := NewBrowserCommand(mgr)
	res, err := cmd.Execute(context.Background(), &CommandContext{Args: []string{}})
	require.NoError(t, err)
	require.Contains(t, res.Output, "internal_commands_browser_status_line")
}

func TestBrowserCommand_Close_NoActiveSession_OK(t *testing.T) {
	mgr := browser.NewBrowserManager(nil, zap.NewNop())
	cmd := NewBrowserCommand(mgr)
	res, err := cmd.Execute(context.Background(), &CommandContext{Args: []string{"close"}})
	require.NoError(t, err)
	require.True(t, res.Success)
	// CONST-046 round-416: "closed" routes through tr().
	require.Contains(t, res.Output, "internal_commands_browser_closed")
}

func TestBrowserCommand_Navigate_RequiresURL(t *testing.T) {
	mgr := browser.NewBrowserManager(nil, zap.NewNop())
	cmd := NewBrowserCommand(mgr)
	_, err := cmd.Execute(context.Background(), &CommandContext{Args: []string{"navigate"}})
	require.Error(t, err)
	_, err = cmd.Execute(context.Background(), &CommandContext{Args: []string{"navigate", ""}})
	require.Error(t, err)
}

func TestBrowserCommand_UnknownSubcommand_Err(t *testing.T) {
	mgr := browser.NewBrowserManager(nil, zap.NewNop())
	cmd := NewBrowserCommand(mgr)
	_, err := cmd.Execute(context.Background(), &CommandContext{Args: []string{"nope"}})
	require.Error(t, err)
}

func TestBrowserCommand_NilMgr_Err(t *testing.T) {
	cmd := NewBrowserCommand(nil)
	_, err := cmd.Execute(context.Background(), &CommandContext{Args: []string{"status"}})
	require.Error(t, err)
}

func TestBrowserCommand_CaseInsensitiveSubcommand(t *testing.T) {
	mgr := browser.NewBrowserManager(nil, zap.NewNop())
	cmd := NewBrowserCommand(mgr)
	res, err := cmd.Execute(context.Background(), &CommandContext{Args: []string{"STATUS"}})
	require.NoError(t, err)
	require.Contains(t, res.Output, "internal_commands_browser_status_line")
}

// --- CONST-046 round-416 paired-mutation tests ---
//
// With sentinelTranslator wired, every migrated /browser user-facing
// literal MUST surface the sentinel-wrapped message ID. If a future
// change re-inlines any literal, these assertions fail — proving the
// migration is real, not a bluff (§11.4 anti-bluff).

func TestBrowserCommand_Status_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	mgr := browser.NewBrowserManager(nil, zap.NewNop())
	cmd := NewBrowserCommand(mgr)
	res, err := cmd.Execute(context.Background(), &CommandContext{Args: []string{"status"}})
	require.NoError(t, err)
	require.Contains(t, res.Output, "<TR:internal_commands_browser_status_line>")
}

func TestBrowserCommand_Close_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	mgr := browser.NewBrowserManager(nil, zap.NewNop())
	cmd := NewBrowserCommand(mgr)
	res, err := cmd.Execute(context.Background(), &CommandContext{Args: []string{"close"}})
	require.NoError(t, err)
	require.Contains(t, res.Output, "<TR:internal_commands_browser_closed>")
}

func TestBrowserCommand_NavigateRequiresURL_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	mgr := browser.NewBrowserManager(nil, zap.NewNop())
	cmd := NewBrowserCommand(mgr)
	_, err := cmd.Execute(context.Background(), &CommandContext{Args: []string{"navigate"}})
	require.Error(t, err)
	require.Contains(t, err.Error(), "<TR:internal_commands_browser_navigate_url_required>")
}

func TestBrowserCommand_UnknownSubcommand_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	mgr := browser.NewBrowserManager(nil, zap.NewNop())
	cmd := NewBrowserCommand(mgr)
	_, err := cmd.Execute(context.Background(), &CommandContext{Args: []string{"nope"}})
	require.Error(t, err)
	require.Contains(t, err.Error(), "<TR:internal_commands_browser_unknown_subcommand>")
}
