package tools

import (
	"context"
	"fmt"

	"dev.helix.code/internal/approval"
	"dev.helix.code/internal/tools/browser"
	"github.com/chromedp/chromedp"
)

// BrowserSnapshotTool is the F23 browser_snapshot tool. Returns
// either OuterHTML of the document (mode="html") or visible body
// text (mode="text"). Truncates Content to MaxSnapshotBytes (64 KB)
// and sets Truncated=true when clipped. Pure read — RequiresApproval
// LevelReadOnly.
type BrowserSnapshotTool struct {
	mgr  *browser.BrowserManager
	opts browser.BrowserOptions
}

// NewBrowserSnapshotTool wires the F23 snapshot tool.
func NewBrowserSnapshotTool(mgr *browser.BrowserManager, opts browser.BrowserOptions) *BrowserSnapshotTool {
	return &BrowserSnapshotTool{mgr: mgr, opts: opts}
}

func (t *BrowserSnapshotTool) Name() string { return "browser_snapshot" }
func (t *BrowserSnapshotTool) Description() string {
	return "Snapshot the current page DOM as html or text."
}
func (t *BrowserSnapshotTool) Category() ToolCategory { return CategoryBrowser }
func (t *BrowserSnapshotTool) RequiresApproval() approval.ApprovalLevel {
	return approval.LevelReadOnly
}
func (t *BrowserSnapshotTool) Schema() ToolSchema {
	return ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"mode": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"html", "text"},
				"description": "Snapshot mode: 'html' returns OuterHTML; 'text' returns visible body text. Default 'html'.",
			},
		},
		Required:    []string{},
		Description: "Snapshot the current browser page DOM as html or text.",
	}
}
func (t *BrowserSnapshotTool) Validate(params map[string]interface{}) error {
	if v, ok := params["mode"]; ok {
		m, isStr := v.(string)
		if !isStr {
			return fmt.Errorf("mode must be a string, got %T", v)
		}
		if m != "" && m != "html" && m != "text" {
			return fmt.Errorf("mode must be 'html' or 'text', got %q", m)
		}
	}
	return nil
}
func (t *BrowserSnapshotTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	if err := t.Validate(params); err != nil {
		return nil, err
	}
	mode := "html"
	if m, ok := params["mode"].(string); ok && m != "" {
		mode = m
	}
	if t.mgr == nil {
		return nil, fmt.Errorf("browser_snapshot: nil manager")
	}
	s, err := t.mgr.RequireSession()
	if err != nil {
		return nil, err
	}
	var content, pageURL, pageTitle string
	var actions []chromedp.Action
	switch mode {
	case "html":
		actions = []chromedp.Action{
			chromedp.OuterHTML("html", &content, chromedp.ByQuery),
			chromedp.Location(&pageURL),
			chromedp.Title(&pageTitle),
		}
	case "text":
		actions = []chromedp.Action{
			chromedp.Text("body", &content, chromedp.NodeVisible, chromedp.ByQuery),
			chromedp.Location(&pageURL),
			chromedp.Title(&pageTitle),
		}
	}
	if err := s.Run(ctx, actions...); err != nil {
		return nil, fmt.Errorf("browser_snapshot: %w", err)
	}
	truncated := false
	if len(content) > browser.MaxSnapshotBytes {
		content = content[:browser.MaxSnapshotBytes]
		truncated = true
	}
	return browser.Snapshot{
		URL:       pageURL,
		Title:     pageTitle,
		Mode:      mode,
		Content:   content,
		Truncated: truncated,
	}, nil
}
