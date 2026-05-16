package tools

import (
	"testing"

	"dev.helix.code/internal/approval"
	"dev.helix.code/internal/tools/browser"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestScreenshotToolV2_Name(t *testing.T) {
	require.Equal(t, "browser_screenshot", NewBrowserScreenshotTool(nil, browser.BrowserOptions{}).Name())
}

func TestScreenshotToolV2_RequiresApproval_LevelReadOnly(t *testing.T) {
	require.Equal(t, approval.LevelReadOnly, NewBrowserScreenshotTool(nil, browser.BrowserOptions{}).RequiresApproval())
}

func TestScreenshotToolV2_Category_Browser(t *testing.T) {
	require.Equal(t, CategoryBrowser, NewBrowserScreenshotTool(nil, browser.BrowserOptions{}).Category())
}

func TestScreenshotToolV2_Validate_FullPage(t *testing.T) {
	tool := NewBrowserScreenshotTool(nil, browser.BrowserOptions{})
	require.NoError(t, tool.Validate(map[string]interface{}{}))
	require.NoError(t, tool.Validate(map[string]interface{}{"full_page": true}))
	require.NoError(t, tool.Validate(map[string]interface{}{"full_page": false}))
	require.Error(t, tool.Validate(map[string]interface{}{"full_page": "yes"}))
	require.Error(t, tool.Validate(map[string]interface{}{"full_page": 1}))
}

func TestScreenshotToolV2_Schema_OptionalFullPage(t *testing.T) {
	sch := NewBrowserScreenshotTool(nil, browser.BrowserOptions{}).Schema()
	require.Equal(t, "object", sch.Type)
	require.Contains(t, sch.Properties, "full_page")
	require.Empty(t, sch.Required)
}

func TestScreenshotToolV2_Execute_NoSession_Err(t *testing.T) {
	mgr := browser.NewBrowserManager(nil, zap.NewNop())
	tool := NewBrowserScreenshotTool(mgr, browser.BrowserOptions{ScreenshotMaxBytes: browser.MaxScreenshotBytes})
	_, err := tool.Execute(t.Context(), map[string]interface{}{})
	require.ErrorIs(t, err, browser.ErrNoActiveSession)
}

func TestScreenshotToolV2_PNGMagic_Bytes(t *testing.T) {
	require.Equal(t, []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}, pngMagic)
	require.Equal(t, 8, len(pngMagic))
}

func TestScreenshotToolV2_MinSize_Pin(t *testing.T) {
	require.Equal(t, int64(1024), minScreenshotBytes)
}

func TestScreenshotToolV2_Execute_NilManager_Err(t *testing.T) {
	tool := NewBrowserScreenshotTool(nil, browser.BrowserOptions{})
	_, err := tool.Execute(t.Context(), map[string]interface{}{})
	require.Error(t, err)
}
