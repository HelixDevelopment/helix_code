package commands

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"dev.helix.code/internal/tools/browser"
	"github.com/chromedp/chromedp"
)

// BrowserCommand is the F23 /browser slash command. Supports three
// subcommands:
//
//   /browser status              — print active session info
//   /browser navigate <url>      — lazy-create + navigate
//   /browser close               — tear down session (idempotent)
//
// Mirrors the per-tool RequiresApproval contract: navigate/close map
// to LevelEdit (gated by F21 approval), status is LevelReadOnly.
type BrowserCommand struct {
	mgr *browser.BrowserManager
}

func NewBrowserCommand(mgr *browser.BrowserManager) *BrowserCommand {
	return &BrowserCommand{mgr: mgr}
}

func (c *BrowserCommand) Name() string      { return "browser" }
func (c *BrowserCommand) Aliases() []string { return []string{} }
func (c *BrowserCommand) Description() string {
	return tr(context.Background(), "internal_commands_browser_description", nil)
}
func (c *BrowserCommand) Usage() string {
	return tr(context.Background(), "internal_commands_browser_usage", nil)
}

func (c *BrowserCommand) Execute(ctx context.Context, cmdCtx *CommandContext) (*CommandResult, error) {
	if c.mgr == nil {
		return nil, fmt.Errorf("/browser: nil manager")
	}
	args := []string{}
	if cmdCtx != nil {
		args = cmdCtx.Args
	}
	if len(args) == 0 {
		return c.handleStatus(ctx)
	}
	sub := strings.ToLower(args[0])
	switch sub {
	case "status":
		return c.handleStatus(ctx)
	case "navigate":
		if len(args) < 2 || strings.TrimSpace(args[1]) == "" {
			return nil, fmt.Errorf("/browser navigate: url is required")
		}
		return c.handleNavigate(ctx, args[1])
	case "close":
		return c.handleClose(ctx)
	default:
		return nil, fmt.Errorf("/browser: unknown subcommand %q (want: status|navigate|close)", sub)
	}
}

func (c *BrowserCommand) handleStatus(ctx context.Context) (*CommandResult, error) {
	st := c.mgr.Status()
	out := fmt.Sprintf("/browser status: active=%t headed=%t chromium=%q screenshot_dir=%q created_at=%s",
		st.Active, st.Headed, st.ChromiumPath, st.ScreenshotDir, st.CreatedAt.Format("2006-01-02T15:04:05Z07:00"))
	return &CommandResult{
		Success:     true,
		Output:      out,
		ShouldReply: true,
		Data: map[string]interface{}{
			"active":         st.Active,
			"headed":         st.Headed,
			"chromium_path":  st.ChromiumPath,
			"screenshot_dir": st.ScreenshotDir,
		},
	}, nil
}

func (c *BrowserCommand) handleNavigate(ctx context.Context, url string) (*CommandResult, error) {
	s, err := c.mgr.EnsureSession(ctx)
	if err != nil {
		return nil, fmt.Errorf("/browser navigate: ensure session: %w", err)
	}
	cctx, cancel := context.WithTimeout(s.Ctx(), 30_000_000_000) // 30 s
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
		return nil, fmt.Errorf("/browser navigate: %w", err)
	}
	return &CommandResult{
		Success:     true,
		Output:      fmt.Sprintf("navigated to %s (title=%q)", resolvedURL, title),
		ShouldReply: true,
		Data:        map[string]interface{}{"url": resolvedURL, "title": title},
	}, nil
}

func (c *BrowserCommand) handleClose(ctx context.Context) (*CommandResult, error) {
	if err := c.mgr.CloseSession(); err != nil {
		return nil, fmt.Errorf("/browser close: %w", err)
	}
	return &CommandResult{
		Success:     true,
		Output:      "closed",
		ShouldReply: true,
	}, nil
}
