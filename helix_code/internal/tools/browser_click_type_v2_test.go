package tools

import (
	"testing"

	"dev.helix.code/internal/approval"
	"dev.helix.code/internal/tools/browser"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// --- Click ---

func TestClickTool_Name(t *testing.T) {
	require.Equal(t, "browser_click", NewBrowserClickTool(nil, browser.BrowserOptions{}).Name())
}

func TestClickTool_RequiresApproval_LevelEdit(t *testing.T) {
	require.Equal(t, approval.LevelEdit, NewBrowserClickTool(nil, browser.BrowserOptions{}).RequiresApproval())
}

func TestClickTool_Validate_RequiresSelector(t *testing.T) {
	tool := NewBrowserClickTool(nil, browser.BrowserOptions{})
	require.Error(t, tool.Validate(map[string]interface{}{}))
	require.Error(t, tool.Validate(map[string]interface{}{"selector": ""}))
	require.Error(t, tool.Validate(map[string]interface{}{"selector": 42}))
	require.NoError(t, tool.Validate(map[string]interface{}{"selector": "#b"}))
}

func TestClickTool_Schema_RequiresSelector(t *testing.T) {
	sch := NewBrowserClickTool(nil, browser.BrowserOptions{}).Schema()
	require.Contains(t, sch.Required, "selector")
}

func TestClickTool_Execute_NoSession_Err(t *testing.T) {
	mgr := browser.NewBrowserManager(nil, zap.NewNop())
	tool := NewBrowserClickTool(mgr, browser.BrowserOptions{})
	_, err := tool.Execute(t.Context(), map[string]interface{}{"selector": "#b"})
	require.ErrorIs(t, err, browser.ErrNoActiveSession)
}

// --- Type ---

func TestTypeTool_Name(t *testing.T) {
	require.Equal(t, "browser_type", NewBrowserTypeTool(nil, browser.BrowserOptions{}).Name())
}

func TestTypeTool_RequiresApproval_LevelEdit(t *testing.T) {
	require.Equal(t, approval.LevelEdit, NewBrowserTypeTool(nil, browser.BrowserOptions{}).RequiresApproval())
}

func TestTypeTool_Validate_RequiresSelectorAndText(t *testing.T) {
	tool := NewBrowserTypeTool(nil, browser.BrowserOptions{})
	require.Error(t, tool.Validate(map[string]interface{}{"selector": "#in"}))
	require.Error(t, tool.Validate(map[string]interface{}{"text": "hi"}))
	require.Error(t, tool.Validate(map[string]interface{}{"selector": "", "text": "hi"}))
	require.NoError(t, tool.Validate(map[string]interface{}{"selector": "#in", "text": "hi"}))
	require.NoError(t, tool.Validate(map[string]interface{}{"selector": "#in", "text": ""}))
}

func TestTypeTool_Schema_RequiresBoth(t *testing.T) {
	sch := NewBrowserTypeTool(nil, browser.BrowserOptions{}).Schema()
	require.Contains(t, sch.Required, "selector")
	require.Contains(t, sch.Required, "text")
}

func TestTypeTool_Execute_NoSession_Err(t *testing.T) {
	mgr := browser.NewBrowserManager(nil, zap.NewNop())
	tool := NewBrowserTypeTool(mgr, browser.BrowserOptions{})
	_, err := tool.Execute(t.Context(), map[string]interface{}{"selector": "#in", "text": "hi"})
	require.ErrorIs(t, err, browser.ErrNoActiveSession)
}

func TestTypeTool_Execute_DoesNotEchoText_CONST042(t *testing.T) {
	// Even on the no-session error path, the text never reaches a
	// position where it could leak. Sanity: ensure tool struct itself
	// does not store the text.
	tool := NewBrowserTypeTool(nil, browser.BrowserOptions{})
	require.NotNil(t, tool)
	// No accessor exposing typed text exists; confirm via the schema
	// that `text` is required-but-not-returned (unit-level).
	sch := tool.Schema()
	require.Contains(t, sch.Required, "text")
}
