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
	require.Contains(t, res.Output, "active=false")
}

func TestBrowserCommand_Status_NoArgs_DefaultsToStatus(t *testing.T) {
	mgr := browser.NewBrowserManager(nil, zap.NewNop())
	cmd := NewBrowserCommand(mgr)
	res, err := cmd.Execute(context.Background(), &CommandContext{Args: []string{}})
	require.NoError(t, err)
	require.Contains(t, res.Output, "active=false")
}

func TestBrowserCommand_Close_NoActiveSession_OK(t *testing.T) {
	mgr := browser.NewBrowserManager(nil, zap.NewNop())
	cmd := NewBrowserCommand(mgr)
	res, err := cmd.Execute(context.Background(), &CommandContext{Args: []string{"close"}})
	require.NoError(t, err)
	require.True(t, res.Success)
	require.Contains(t, res.Output, "closed")
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
	require.Contains(t, res.Output, "active=false")
}
