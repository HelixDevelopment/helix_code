package browser

import (
	"context"
	"time"
)

// NewStubBrowserSessionForTest constructs a minimal BrowserSession
// suitable for unit tests in dependent packages that need to inject
// sessions via SessionFactory but cannot reach the unexported fields.
//
// Test helper only — not intended for production use. Lives in a
// non-_test.go file so external test packages can reference it.
func NewStubBrowserSessionForTest(screenshotDir, chromiumPath string, createdAt time.Time) *BrowserSession {
	return &BrowserSession{
		ctx:           context.Background(),
		cancel:        func() {},
		screenshotDir: screenshotDir,
		chromiumPath:  chromiumPath,
		createdAt:     createdAt,
	}
}
