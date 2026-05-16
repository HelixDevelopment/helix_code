package tools

import (
	"testing"

	"dev.helix.code/internal/approval"
	"dev.helix.code/internal/tools/browser"
	"github.com/stretchr/testify/require"
)

func TestNavigateToolV2_Name(t *testing.T) {
	require.Equal(t, "browser_navigate", NewBrowserNavigateTool(nil, browser.BrowserOptions{}).Name())
}

func TestNavigateToolV2_RequiresApproval_LevelEdit(t *testing.T) {
	require.Equal(t, approval.LevelEdit, NewBrowserNavigateTool(nil, browser.BrowserOptions{}).RequiresApproval())
}

func TestNavigateToolV2_Category_Browser(t *testing.T) {
	require.Equal(t, CategoryBrowser, NewBrowserNavigateTool(nil, browser.BrowserOptions{}).Category())
}

func TestNavigateToolV2_Validate_RequiresURL(t *testing.T) {
	tool := NewBrowserNavigateTool(nil, browser.BrowserOptions{})
	require.Error(t, tool.Validate(map[string]interface{}{}))
	require.Error(t, tool.Validate(map[string]interface{}{"url": ""}))
	require.Error(t, tool.Validate(map[string]interface{}{"url": 42}))
	require.NoError(t, tool.Validate(map[string]interface{}{"url": "https://example.com"}))
}

func TestNavigateToolV2_Schema_RequiresURL(t *testing.T) {
	sch := NewBrowserNavigateTool(nil, browser.BrowserOptions{}).Schema()
	require.Contains(t, sch.Required, "url")
	require.Equal(t, "object", sch.Type)
	require.Contains(t, sch.Properties, "url")
}

func TestNavigateToolV2_Description_NonEmpty(t *testing.T) {
	require.NotEmpty(t, NewBrowserNavigateTool(nil, browser.BrowserOptions{}).Description())
}

func TestNavigateToolV2_Execute_NilManager_Err(t *testing.T) {
	tool := NewBrowserNavigateTool(nil, browser.BrowserOptions{})
	_, err := tool.Execute(t.Context(), map[string]interface{}{"url": "https://example.com"})
	require.Error(t, err)
}

func TestNavigateToolV2_Execute_InvalidURL_ValidateRejects(t *testing.T) {
	tool := NewBrowserNavigateTool(nil, browser.BrowserOptions{})
	_, err := tool.Execute(t.Context(), map[string]interface{}{})
	require.Error(t, err)
}
