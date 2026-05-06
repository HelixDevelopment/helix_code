package tools

import (
	"fmt"

	"dev.helix.code/internal/tools/browser"
)

// RegisterBrowserToolsV2 registers the F23 cline-style browser tool
// suite (browser_navigate, browser_snapshot, browser_click,
// browser_type, browser_screenshot, browser_close) on the given
// registry. Names collide with the legacy tool names — those are
// renamed to browser_legacy_* in browser_tools.go (P2-F23-T04).
func RegisterBrowserToolsV2(reg *ToolRegistry, mgr *browser.BrowserManager) error {
	if reg == nil {
		return fmt.Errorf("RegisterBrowserToolsV2: nil registry")
	}
	if mgr == nil {
		return fmt.Errorf("RegisterBrowserToolsV2: nil manager")
	}
	opts := browser.OptionsFromEnv()
	items := []Tool{
		NewBrowserNavigateTool(mgr, opts),
		NewBrowserSnapshotTool(mgr, opts),
		NewBrowserClickTool(mgr, opts),
		NewBrowserTypeTool(mgr, opts),
		NewBrowserScreenshotTool(mgr, opts),
		NewBrowserCloseTool(mgr),
	}
	for _, it := range items {
		reg.Register(it)
	}
	return nil
}
