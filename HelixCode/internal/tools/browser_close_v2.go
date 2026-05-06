package tools

import (
	"context"
	"fmt"

	"dev.helix.code/internal/approval"
	"dev.helix.code/internal/tools/browser"
)

// BrowserCloseToolV2 is the F23 browser_close tool. Tears down the
// active session via mgr.CloseSession (idempotent — sync.Once on the
// underlying BrowserSession; second close is a no-op). After this
// call, RequireSession returns ErrNoActiveSession (positive teardown
// evidence per spec §5.2 Bluff #5).
//
// RequiresApproval LevelEdit (drops in-flight page state — user
// prompted before tear-down per spec §11 #12).
type BrowserCloseToolV2 struct {
	mgr *browser.BrowserManager
}

func NewBrowserCloseTool(mgr *browser.BrowserManager) *BrowserCloseToolV2 {
	return &BrowserCloseToolV2{mgr: mgr}
}

func (t *BrowserCloseToolV2) Name() string                                    { return "browser_close" }
func (t *BrowserCloseToolV2) Description() string                             { return "Close the active browser session and terminate the chromium subprocess." }
func (t *BrowserCloseToolV2) Category() ToolCategory                          { return CategoryBrowser }
func (t *BrowserCloseToolV2) RequiresApproval() approval.ApprovalLevel        { return approval.LevelEdit }
func (t *BrowserCloseToolV2) Schema() ToolSchema {
	return ToolSchema{
		Type:        "object",
		Properties:  map[string]interface{}{},
		Required:    []string{},
		Description: "Close the active browser session and terminate the chromium subprocess.",
	}
}
func (t *BrowserCloseToolV2) Validate(params map[string]interface{}) error { return nil }
func (t *BrowserCloseToolV2) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	if t.mgr == nil {
		return nil, fmt.Errorf("browser_close: nil manager")
	}
	if err := t.mgr.CloseSession(); err != nil {
		return nil, fmt.Errorf("browser_close: %w", err)
	}
	return map[string]interface{}{"closed": true}, nil
}
