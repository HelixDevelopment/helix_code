package browser

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/chromedp/chromedp"
	"go.uber.org/zap"
)

// BrowserSession is the per-process single chromedp browser session.
// Constructed lazily on first browser_navigate and torn down on
// browser_close. Holds a chromedp Context (which itself owns the
// chromium subprocess via chromedp.NewExecAllocator), a per-session
// screenshot tempdir, a monotonic counter for screenshot file names,
// and a sync.Once-guarded Close() so double-close is a no-op.
type BrowserSession struct {
	ctx             context.Context
	cancel          context.CancelFunc
	allocCancel     context.CancelFunc
	screenshotDir   string
	screenshotCount atomic.Uint64
	chromiumPath    string
	headed          bool
	createdAt       time.Time
	closeOnce       sync.Once
	log             *zap.Logger
}

// Run executes one or more chromedp Actions against the session ctx.
// Returns ErrNoActiveSession if the session was zero-valued (defensive
// guard against passing a nil session into a tool).
func (s *BrowserSession) Run(ctx context.Context, actions ...chromedp.Action) error {
	if s == nil || s.ctx == nil {
		return ErrNoActiveSession
	}
	// Use the session's own ctx (which carries the chromedp browser
	// allocator) but allow callers to layer a timeout via ctx.
	// chromedp.Run takes the chromedp context only.
	_ = ctx
	return chromedp.Run(s.ctx, actions...)
}

// RunWithCtx executes actions allowing the caller to provide a
// derived chromedp context (e.g. with a timeout). The provided ctx
// MUST be derived from s.ctx (e.g. via context.WithTimeout(s.ctx, ...)).
func (s *BrowserSession) RunWithCtx(cctx context.Context, actions ...chromedp.Action) error {
	if s == nil || s.ctx == nil {
		return ErrNoActiveSession
	}
	return chromedp.Run(cctx, actions...)
}

// Ctx returns the session's chromedp context (so callers can derive
// timeouts via context.WithTimeout(s.Ctx(), ...)).
func (s *BrowserSession) Ctx() context.Context {
	if s == nil {
		return nil
	}
	return s.ctx
}

// NextScreenshotPath atomically increments the per-session counter
// and returns an absolute file path inside the screenshot tempdir.
func (s *BrowserSession) NextScreenshotPath() string {
	n := s.screenshotCount.Add(1)
	return filepath.Join(s.screenshotDir, fmt.Sprintf("%d.png", n))
}

// ScreenshotDir returns the per-session screenshot directory.
func (s *BrowserSession) ScreenshotDir() string { return s.screenshotDir }

// ChromiumPath returns the resolved chromium binary path.
func (s *BrowserSession) ChromiumPath() string { return s.chromiumPath }

// Headed reports whether the session was launched in headed mode.
func (s *BrowserSession) Headed() bool { return s.headed }

// CreatedAt returns the session creation timestamp.
func (s *BrowserSession) CreatedAt() time.Time { return s.createdAt }

// Close cancels the chromedp context (which terminates the chromium
// subprocess) and removes the per-session screenshot tempdir.
// Idempotent: subsequent calls are no-ops via sync.Once.
func (s *BrowserSession) Close() error {
	if s == nil {
		return nil
	}
	var rmErr error
	s.closeOnce.Do(func() {
		if s.cancel != nil {
			s.cancel()
		}
		if s.allocCancel != nil {
			s.allocCancel()
		}
		if s.screenshotDir != "" {
			rmErr = os.RemoveAll(s.screenshotDir)
		}
	})
	return rmErr
}
