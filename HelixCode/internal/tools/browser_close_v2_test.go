package tools

import (
	"context"
	"testing"
	"time"

	"dev.helix.code/internal/approval"
	"dev.helix.code/internal/tools/browser"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestCloseToolV2_Name(t *testing.T) {
	require.Equal(t, "browser_close", NewBrowserCloseTool(nil).Name())
}

func TestCloseToolV2_RequiresApproval_LevelEdit(t *testing.T) {
	require.Equal(t, approval.LevelEdit, NewBrowserCloseTool(nil).RequiresApproval())
}

func TestCloseToolV2_Category_Browser(t *testing.T) {
	require.Equal(t, CategoryBrowser, NewBrowserCloseTool(nil).Category())
}

func TestCloseToolV2_Validate_NoArgs(t *testing.T) {
	require.NoError(t, NewBrowserCloseTool(nil).Validate(map[string]interface{}{}))
}

func TestCloseToolV2_Schema_NoRequired(t *testing.T) {
	sch := NewBrowserCloseTool(nil).Schema()
	require.Empty(t, sch.Required)
}

func TestCloseToolV2_Execute_NoActiveSession_NoOpSuccess(t *testing.T) {
	mgr := browser.NewBrowserManager(nil, zap.NewNop())
	tool := NewBrowserCloseTool(mgr)
	res, err := tool.Execute(t.Context(), map[string]interface{}{})
	require.NoError(t, err)
	resMap := res.(map[string]interface{})
	require.True(t, resMap["closed"].(bool))
}

func TestCloseToolV2_Execute_NilManager_Err(t *testing.T) {
	tool := NewBrowserCloseTool(nil)
	_, err := tool.Execute(t.Context(), map[string]interface{}{})
	require.Error(t, err)
}

func TestCloseToolV2_Execute_RequireFailsAfter(t *testing.T) {
	// Use a stub manager (avoid real chromium).
	mgr := browser.NewBrowserManager(nil, zap.NewNop())
	mgr.SetSessionFactory(func(_ context.Context, _ *browser.BrowserManager, opts browser.BrowserOptions) (*browser.BrowserSession, error) {
		// Construct a stub BrowserSession via the package's own helper.
		// We can't directly init unexported fields; the manager.go
		// stub-factory test pattern in browser pkg uses public-test
		// helpers — we mirror that by ensuring CloseSession works
		// even with a minimal-but-valid session.
		return browser.NewStubBrowserSessionForTest(t.TempDir(), "/fake/chromium", time.Now()), nil
	})
	_, err := mgr.EnsureSession(t.Context())
	require.NoError(t, err)
	tool := NewBrowserCloseTool(mgr)
	_, err = tool.Execute(t.Context(), map[string]interface{}{})
	require.NoError(t, err)
	_, reqErr := mgr.RequireSession()
	require.ErrorIs(t, reqErr, browser.ErrNoActiveSession)
}

func TestCloseToolV2_Execute_Idempotent(t *testing.T) {
	mgr := browser.NewBrowserManager(nil, zap.NewNop())
	tool := NewBrowserCloseTool(mgr)
	for i := 0; i < 3; i++ {
		_, err := tool.Execute(t.Context(), map[string]interface{}{})
		require.NoError(t, err)
	}
}
