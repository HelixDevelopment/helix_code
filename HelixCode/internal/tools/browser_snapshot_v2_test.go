package tools

import (
	"testing"

	"dev.helix.code/internal/approval"
	"dev.helix.code/internal/tools/browser"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestSnapshotTool_Name(t *testing.T) {
	require.Equal(t, "browser_snapshot", NewBrowserSnapshotTool(nil, browser.BrowserOptions{}).Name())
}

func TestSnapshotTool_RequiresApproval_LevelReadOnly(t *testing.T) {
	require.Equal(t, approval.LevelReadOnly, NewBrowserSnapshotTool(nil, browser.BrowserOptions{}).RequiresApproval())
}

func TestSnapshotTool_Category_Browser(t *testing.T) {
	require.Equal(t, CategoryBrowser, NewBrowserSnapshotTool(nil, browser.BrowserOptions{}).Category())
}

func TestSnapshotTool_Validate_Mode(t *testing.T) {
	tool := NewBrowserSnapshotTool(nil, browser.BrowserOptions{})
	require.NoError(t, tool.Validate(map[string]interface{}{}))
	require.NoError(t, tool.Validate(map[string]interface{}{"mode": "html"}))
	require.NoError(t, tool.Validate(map[string]interface{}{"mode": "text"}))
	require.NoError(t, tool.Validate(map[string]interface{}{"mode": ""}))
	require.Error(t, tool.Validate(map[string]interface{}{"mode": "json"}))
	require.Error(t, tool.Validate(map[string]interface{}{"mode": 42}))
}

func TestSnapshotTool_Schema_OptionalMode(t *testing.T) {
	sch := NewBrowserSnapshotTool(nil, browser.BrowserOptions{}).Schema()
	require.Equal(t, "object", sch.Type)
	require.Contains(t, sch.Properties, "mode")
	require.Empty(t, sch.Required)
}

func TestSnapshotTool_Execute_NoSession_Err(t *testing.T) {
	mgr := browser.NewBrowserManager(nil, zap.NewNop())
	tool := NewBrowserSnapshotTool(mgr, browser.BrowserOptions{})
	_, err := tool.Execute(t.Context(), map[string]interface{}{})
	require.ErrorIs(t, err, browser.ErrNoActiveSession)
}

func TestSnapshotTool_Execute_NilManager_Err(t *testing.T) {
	tool := NewBrowserSnapshotTool(nil, browser.BrowserOptions{})
	_, err := tool.Execute(t.Context(), map[string]interface{}{})
	require.Error(t, err)
}
