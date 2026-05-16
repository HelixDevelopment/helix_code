package browser

import (
	"os"
	"strings"
	"time"
)

// BrowserOptions is the per-session configuration applied at session
// create. Read from env via OptionsFromEnv().
type BrowserOptions struct {
	Headless           bool
	ViewportWidth      int
	ViewportHeight     int
	NavigateTimeout    time.Duration
	ClickWaitDuration  time.Duration
	ScreenshotMaxBytes int64
}

// OptionsFromEnv reads HELIXCODE_BROWSER_HEADED (case-insensitive
// literal "true" only) and returns a BrowserOptions with default
// viewport (1280x720), 30 s navigate timeout, 500 ms click-settle,
// and the package-level ScreenshotMaxBytes ceiling.
//
// Any value other than the literal "true" (case-folded) — including
// the empty string, "yes", "1", "True123", "headless", "0" — leaves
// Headless=true. Typos default safe (headless).
func OptionsFromEnv() BrowserOptions {
	headless := true
	if v := os.Getenv(EnvVarHeadedMode); strings.EqualFold(v, "true") {
		headless = false
	}
	return BrowserOptions{
		Headless:           headless,
		ViewportWidth:      1280,
		ViewportHeight:     720,
		NavigateTimeout:    30 * time.Second,
		ClickWaitDuration:  500 * time.Millisecond,
		ScreenshotMaxBytes: MaxScreenshotBytes,
	}
}
