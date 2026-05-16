package tools

import (
	"context"
	"errors"
	"fmt"

	"dev.helix.code/internal/approval"
	"dev.helix.code/internal/tools/browser"
	"github.com/chromedp/chromedp"
)

// BrowserNavigateToolV2 is the F23 cline-style browser_navigate tool.
// Lazy-creates a browser.BrowserSession on first call (no separate
// launch tool per cline's single-session model), navigates with
// WaitReady("body") + a configurable per-call timeout (default 30 s
// from BrowserOptions), and returns the resolved URL + page title.
//
// RequiresApproval: LevelEdit (mutates browser state, may visit
// arbitrary URLs).
type BrowserNavigateToolV2 struct {
	mgr  *browser.BrowserManager
	opts browser.BrowserOptions
}

// NewBrowserNavigateTool wires the F23 navigate tool.
func NewBrowserNavigateTool(mgr *browser.BrowserManager, opts browser.BrowserOptions) *BrowserNavigateToolV2 {
	return &BrowserNavigateToolV2{mgr: mgr, opts: opts}
}

// Name returns the tool name.
func (t *BrowserNavigateToolV2) Name() string { return "browser_navigate" }

// Description returns a brief tool description.
func (t *BrowserNavigateToolV2) Description() string {
	return "Navigate the active browser session to a URL (lazy-creates session if none)."
}

// Category returns the tool category.
func (t *BrowserNavigateToolV2) Category() ToolCategory { return CategoryBrowser }

// RequiresApproval — LevelEdit per spec §3.6.
func (t *BrowserNavigateToolV2) RequiresApproval() approval.ApprovalLevel {
	return approval.LevelEdit
}

// Schema returns the tool's parameter schema.
func (t *BrowserNavigateToolV2) Schema() ToolSchema {
	return ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"url": map[string]interface{}{
				"type":        "string",
				"description": "URL to navigate to",
			},
		},
		Required:    []string{"url"},
		Description: "Navigate the active browser session (lazy-creates if none) to the given URL.",
	}
}

// Validate checks the params before Execute.
func (t *BrowserNavigateToolV2) Validate(params map[string]interface{}) error {
	u, ok := params["url"].(string)
	if !ok || u == "" {
		return fmt.Errorf("url is required (non-empty string)")
	}
	return nil
}

// Execute navigates the browser. Returns map[string]interface{} with
// keys "url" (resolved URL after navigation) and "title".
func (t *BrowserNavigateToolV2) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	if err := t.Validate(params); err != nil {
		return nil, err
	}
	url := params["url"].(string)
	if t.mgr == nil {
		return nil, fmt.Errorf("browser_navigate: nil manager")
	}
	s, err := t.mgr.EnsureSession(ctx)
	if err != nil {
		return nil, err
	}
	cctx, cancel := context.WithTimeout(s.Ctx(), t.opts.NavigateTimeout)
	defer cancel()
	var resolvedURL, title string
	if err := s.RunWithCtx(cctx,
		chromedp.Navigate(url),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Title(&title),
		chromedp.Location(&resolvedURL),
	); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, browser.ErrNavigationTimeout
		}
		return nil, fmt.Errorf("browser_navigate: %w", err)
	}
	return map[string]interface{}{
		"url":   resolvedURL,
		"title": title,
	}, nil
}
