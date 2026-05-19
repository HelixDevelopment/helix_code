package tools

import (
	"context"
	"errors"
	"fmt"
	"time"

	"dev.helix.code/internal/approval"
	"dev.helix.code/internal/tools/browser"
	"github.com/chromedp/chromedp"
)

// browserSelectorTimeout is the per-call sub-timeout for click/type's
// NodeVisible. Disambiguates missing-selector from slow-navigation
// per spec §5.2 Bluff #3.
const browserSelectorTimeout = 5 * time.Second

// BrowserClickTool is the F23 browser_click tool. Uses chromedp.Click
// with NodeVisible (waits until selector becomes visible) gated by a
// 5 s sub-timeout that disambiguates missing-selector from slow-nav.
// After the click, sleeps ClickWaitDuration (default 500 ms) so async
// DOM updates settle before the caller's next snapshot.
type BrowserClickTool struct {
	mgr  *browser.BrowserManager
	opts browser.BrowserOptions
}

func NewBrowserClickTool(mgr *browser.BrowserManager, opts browser.BrowserOptions) *BrowserClickTool {
	return &BrowserClickTool{mgr: mgr, opts: opts}
}

func (t *BrowserClickTool) Name() string                                    { return "browser_click" }
func (t *BrowserClickTool) Description() string {
	return tr(context.Background(), "internal_tools_browser_click_description", nil)
}
func (t *BrowserClickTool) Category() ToolCategory                          { return CategoryBrowser }
func (t *BrowserClickTool) RequiresApproval() approval.ApprovalLevel        { return approval.LevelEdit }
func (t *BrowserClickTool) Schema() ToolSchema {
	return ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"selector": map[string]interface{}{
				"type":        "string",
				"description": "CSS selector of the element to click",
			},
		},
		Required:    []string{"selector"},
		Description: "Click an element by CSS selector. Waits for visibility (5s) then settles.",
	}
}
func (t *BrowserClickTool) Validate(params map[string]interface{}) error {
	sel, ok := params["selector"].(string)
	if !ok || sel == "" {
		return fmt.Errorf("selector is required (non-empty string)")
	}
	return nil
}
func (t *BrowserClickTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	if err := t.Validate(params); err != nil {
		return nil, err
	}
	sel := params["selector"].(string)
	if t.mgr == nil {
		return nil, fmt.Errorf("browser_click: nil manager")
	}
	s, err := t.mgr.RequireSession()
	if err != nil {
		return nil, err
	}
	cctx, cancel := context.WithTimeout(s.Ctx(), browserSelectorTimeout)
	defer cancel()
	if err := s.RunWithCtx(cctx,
		chromedp.Click(sel, chromedp.NodeVisible, chromedp.ByQuery),
		chromedp.Sleep(t.opts.ClickWaitDuration),
	); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, browser.ErrSelectorNotFound
		}
		return nil, fmt.Errorf("browser_click: %w", err)
	}
	return map[string]interface{}{
		"clicked":  true,
		"selector": sel,
	}, nil
}

// BrowserTypeTool is the F23 browser_type tool. Uses chromedp.SendKeys
// with NodeVisible to type into an input/textarea/contenteditable
// element. Same 5 s selector sub-timeout as click.
type BrowserTypeTool struct {
	mgr  *browser.BrowserManager
	opts browser.BrowserOptions
}

func NewBrowserTypeTool(mgr *browser.BrowserManager, opts browser.BrowserOptions) *BrowserTypeTool {
	return &BrowserTypeTool{mgr: mgr, opts: opts}
}

func (t *BrowserTypeTool) Name() string                                    { return "browser_type" }
func (t *BrowserTypeTool) Description() string {
	return tr(context.Background(), "internal_tools_browser_type_description", nil)
}
func (t *BrowserTypeTool) Category() ToolCategory                          { return CategoryBrowser }
func (t *BrowserTypeTool) RequiresApproval() approval.ApprovalLevel        { return approval.LevelEdit }
func (t *BrowserTypeTool) Schema() ToolSchema {
	return ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"selector": map[string]interface{}{
				"type":        "string",
				"description": "CSS selector of the input element",
			},
			"text": map[string]interface{}{
				"type":        "string",
				"description": "Text to type into the element",
			},
		},
		Required:    []string{"selector", "text"},
		Description: "Type text into an input element by CSS selector.",
	}
}
func (t *BrowserTypeTool) Validate(params map[string]interface{}) error {
	sel, ok := params["selector"].(string)
	if !ok || sel == "" {
		return fmt.Errorf("selector is required (non-empty string)")
	}
	txt, ok := params["text"].(string)
	if !ok {
		return fmt.Errorf("text is required (string)")
	}
	_ = txt // empty string is allowed
	return nil
}
func (t *BrowserTypeTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	if err := t.Validate(params); err != nil {
		return nil, err
	}
	sel := params["selector"].(string)
	txt := params["text"].(string)
	if t.mgr == nil {
		return nil, fmt.Errorf("browser_type: nil manager")
	}
	s, err := t.mgr.RequireSession()
	if err != nil {
		return nil, err
	}
	cctx, cancel := context.WithTimeout(s.Ctx(), browserSelectorTimeout)
	defer cancel()
	if err := s.RunWithCtx(cctx,
		chromedp.SendKeys(sel, txt, chromedp.NodeVisible, chromedp.ByQuery),
	); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, browser.ErrSelectorNotFound
		}
		return nil, fmt.Errorf("browser_type: %w", err)
	}
	// CONST-042: do NOT include `txt` in returned map (may be a secret).
	return map[string]interface{}{
		"typed":    true,
		"selector": sel,
	}, nil
}
