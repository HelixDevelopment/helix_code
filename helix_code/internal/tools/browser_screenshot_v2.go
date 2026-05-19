package tools

import (
	"bytes"
	"context"
	"fmt"
	"image/png"
	"os"
	"path/filepath"

	"dev.helix.code/internal/approval"
	"dev.helix.code/internal/tools/browser"
	"github.com/chromedp/chromedp"
)

// pngMagic is the 8-byte PNG file signature. Verified against the
// in-memory chromedp buffer BEFORE writing to disk so the file
// system never sees a chromedp-empty-buf bluff (spec §5.2 Bluff #4).
var pngMagic = []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}

// minScreenshotBytes is the lower-bound size assertion. A 0-byte
// or near-empty PNG is a bluff: chromedp returned nil-buf but the
// implementation wrote it anyway. We refuse anything ≤1024 bytes.
const minScreenshotBytes = int64(1024)

// BrowserScreenshotToolV2 is the F23 browser_screenshot tool.
// Captures the current page as PNG (viewport by default; full_page
// optional), verifies PNG magic + image/png.DecodeConfig + size>1024
// before writing, and writes to the per-session tempdir at mode 0600.
// Returns an absolute file path (NOT base64) per spec §3.4.
//
// RequiresApproval LevelReadOnly (pure read of in-memory buffer →
// disk; no browser-state mutation).
type BrowserScreenshotToolV2 struct {
	mgr  *browser.BrowserManager
	opts browser.BrowserOptions
}

func NewBrowserScreenshotTool(mgr *browser.BrowserManager, opts browser.BrowserOptions) *BrowserScreenshotToolV2 {
	return &BrowserScreenshotToolV2{mgr: mgr, opts: opts}
}

func (t *BrowserScreenshotToolV2) Name() string                                    { return "browser_screenshot" }
func (t *BrowserScreenshotToolV2) Description() string {
	return tr(context.Background(), "internal_tools_browser_screenshot_v2_description", nil)
}
func (t *BrowserScreenshotToolV2) Category() ToolCategory                          { return CategoryBrowser }
func (t *BrowserScreenshotToolV2) RequiresApproval() approval.ApprovalLevel        { return approval.LevelReadOnly }
func (t *BrowserScreenshotToolV2) Schema() ToolSchema {
	return ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"full_page": map[string]interface{}{
				"type":        "boolean",
				"description": "Capture the full scrollable page (default false → viewport-only).",
			},
		},
		Required:    []string{},
		Description: "Capture a screenshot of the current page as a PNG file (returns absolute path).",
	}
}
func (t *BrowserScreenshotToolV2) Validate(params map[string]interface{}) error {
	if v, ok := params["full_page"]; ok {
		if _, isBool := v.(bool); !isBool {
			return fmt.Errorf("full_page must be a boolean, got %T", v)
		}
	}
	return nil
}
func (t *BrowserScreenshotToolV2) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	if err := t.Validate(params); err != nil {
		return nil, err
	}
	fullPage := false
	if v, ok := params["full_page"].(bool); ok {
		fullPage = v
	}
	if t.mgr == nil {
		return nil, fmt.Errorf("browser_screenshot: nil manager")
	}
	s, err := t.mgr.RequireSession()
	if err != nil {
		return nil, err
	}
	maxBytes := t.opts.ScreenshotMaxBytes
	if maxBytes <= 0 {
		maxBytes = browser.MaxScreenshotBytes
	}
	var buf []byte
	var action chromedp.Action
	if fullPage {
		action = chromedp.FullScreenshot(&buf, 90)
	} else {
		action = chromedp.CaptureScreenshot(&buf)
	}
	if err := s.Run(ctx, action); err != nil {
		return nil, fmt.Errorf("browser_screenshot: %w", err)
	}
	if int64(len(buf)) > maxBytes {
		// Fall back to viewport-only.
		if !fullPage {
			return nil, browser.ErrScreenshotTooLarge
		}
		buf = buf[:0]
		if err := s.Run(ctx, chromedp.CaptureScreenshot(&buf)); err != nil {
			return nil, fmt.Errorf("browser_screenshot: viewport fallback: %w", err)
		}
		if int64(len(buf)) > maxBytes {
			return nil, browser.ErrScreenshotTooLarge
		}
	}
	// PNG-magic verification on the IN-MEMORY buf (spec §5.2 Bluff #4).
	if len(buf) <= 8 || !bytes.Equal(buf[:8], pngMagic) {
		return nil, fmt.Errorf("browser_screenshot: chromedp returned non-PNG bytes (len=%d)", len(buf))
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(buf))
	if err != nil {
		return nil, fmt.Errorf("browser_screenshot: invalid PNG: %w", err)
	}
	path := s.NextScreenshotPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("browser_screenshot: mkdir: %w", err)
	}
	if err := os.WriteFile(path, buf, 0o600); err != nil {
		return nil, fmt.Errorf("browser_screenshot: write: %w", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("browser_screenshot: stat: %w", err)
	}
	if info.Size() <= minScreenshotBytes {
		return nil, fmt.Errorf("browser_screenshot: file too small (%d bytes ≤ %d)", info.Size(), minScreenshotBytes)
	}
	return browser.ScreenshotResult{
		Path:   path,
		Bytes:  info.Size(),
		Width:  cfg.Width,
		Height: cfg.Height,
	}, nil
}
