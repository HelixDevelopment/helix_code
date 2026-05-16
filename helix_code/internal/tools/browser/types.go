// Package browser — F23 cline-style single-session façade types.
//
// This file defines value types and sentinel errors used across the F23
// Tool implementations (browser_navigate, browser_snapshot, browser_click,
// browser_type, browser_screenshot, browser_close) and the BrowserManager.
// It coexists with the pre-existing multi-browser tools in this package
// (Controller / browser.go / actions.go / etc.) which retain their
// browser_legacy_* names per F23 T09 collision-resolution.
package browser

import (
	"errors"
	"time"
)

// EnvVarHeadedMode is the env var name (case-insensitive literal "true"
// only) that opts in to headed-mode chromium. Anything else (including
// unset, "True123", "yes", "1") is headless.
const EnvVarHeadedMode = "HELIXCODE_BROWSER_HEADED"

// MaxSnapshotBytes is the hard cap on snapshot Content length returned
// to the tool caller; longer content is truncated and Truncated set true.
const MaxSnapshotBytes = 64 * 1024

// MaxScreenshotBytes is the soft cap on PNG screenshot byte size; the
// screenshot tool falls back to viewport-only when full-page exceeds.
const MaxScreenshotBytes int64 = 5 * 1024 * 1024

// Snapshot is the result of a browser_snapshot tool call. Mode is
// either "html" (OuterHTML of the document) or "text" (visible body
// text). Truncated is true when Content was clipped to MaxSnapshotBytes.
type Snapshot struct {
	URL       string `json:"url"`
	Title     string `json:"title"`
	Mode      string `json:"mode"`
	Content   string `json:"content"`
	Truncated bool   `json:"truncated"`
}

// ScreenshotResult is the result of a browser_screenshot tool call.
// Path is the absolute on-disk path of the written PNG file; the file
// is mode 0600 under the per-session tempdir. Bytes/Width/Height carry
// positive runtime evidence that the bytes parsed as a real PNG.
type ScreenshotResult struct {
	Path   string `json:"path"`
	Bytes  int64  `json:"bytes"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// ManagerStatus is the result of a /browser status slash invocation
// and BrowserManager.Status() — Active is false when no session has
// been lazily-created yet.
type ManagerStatus struct {
	Active        bool      `json:"active"`
	ChromiumPath  string    `json:"chromium_path,omitempty"`
	ScreenshotDir string    `json:"screenshot_dir,omitempty"`
	Headed        bool      `json:"headed"`
	CreatedAt     time.Time `json:"created_at,omitempty"`
}

// Sentinel errors returned by F23 browser tools and the BrowserManager.
// All wrap-able via fmt.Errorf("%w", ...) and assertable via errors.Is.
var (
	// ErrNoActiveSession indicates the manager has no active session.
	// Returned by RequireSession() and by every tool except navigate
	// and close. Navigate lazy-creates; close is idempotent.
	ErrNoActiveSession = errors.New("browser: no active session")

	// ErrChromiumNotFound indicates ChromeDiscovery.FindChrome returned
	// no path. The integration test and Challenge gate on this and emit
	// SKIP-OK: #P2-F23 chromium not available.
	ErrChromiumNotFound = errors.New("browser: chromium not found on PATH")

	// ErrNavigationTimeout indicates the navigate tool's per-call
	// timeout fired (default 30 s). Wraps context.DeadlineExceeded.
	ErrNavigationTimeout = errors.New("browser: navigation timeout")

	// ErrSelectorNotFound indicates click/type's 5 s sub-timeout fired
	// before NodeVisible resolved. Disambiguates missing-selector from
	// slow-navigation per spec §5.2 Bluff #3.
	ErrSelectorNotFound = errors.New("browser: selector not found")

	// ErrScreenshotTooLarge indicates the captured PNG exceeds the
	// configured ScreenshotMaxBytes ceiling even after viewport-only
	// fallback.
	ErrScreenshotTooLarge = errors.New("browser: screenshot too large")
)
