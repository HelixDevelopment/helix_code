package main

import (
	"os"
	"strings"
	"testing"
)

// withColorForced runs fn with the TTY check and NO_COLOR state forced to
// the given values, restoring both afterwards. This lets the tests drive
// both color-enabled and color-disabled branches deterministically without
// a real terminal attached.
func withColorForced(t *testing.T, tty bool, noColor bool, fn func()) {
	t.Helper()
	prevTTY := colorStdoutIsTTY
	colorStdoutIsTTY = func() bool { return tty }
	defer func() { colorStdoutIsTTY = prevTTY }()

	if noColor {
		t.Setenv("NO_COLOR", "1")
	} else {
		// Ensure NO_COLOR is unset for the color-enabled branch.
		_ = os.Unsetenv("NO_COLOR")
	}
	fn()
}

// TestBrandBanner_ContainsWordmarkAndLimeWhenColorEnabled asserts the banner
// always contains the "HelixCode" wordmark, and that the lime PRIMARY ANSI
// 24-bit code is emitted when color is enabled (TTY + NO_COLOR unset).
func TestBrandBanner_ContainsWordmarkAndLimeWhenColorEnabled(t *testing.T) {
	withColorForced(t, true, false, func() {
		got := brandBanner()
		if !strings.Contains(got, "HelixCode") {
			t.Fatalf("brandBanner() must contain the wordmark %q; got:\n%s", "HelixCode", got)
		}
		// Lime PRIMARY #A8DD22 → 168;221;34 as an ANSI 24-bit fg escape.
		wantLime := "\x1b[" + brandPrimary + "m"
		if !strings.Contains(got, wantLime) {
			t.Fatalf("brandBanner() must contain the lime ANSI code %q when color enabled; got:\n%q", wantLime, got)
		}
		if brandPrimary != "38;2;168;221;34" {
			t.Fatalf("brandPrimary must encode lime #A8DD22 (168;221;34); got %q", brandPrimary)
		}
	})
}

// TestBrandBanner_NoEscapesWhenColorDisabled asserts the banner still contains
// the wordmark but emits zero ANSI escape bytes when color is disabled (either
// NO_COLOR set or non-TTY).
func TestBrandBanner_NoEscapesWhenColorDisabled(t *testing.T) {
	// NO_COLOR set, even on a TTY.
	withColorForced(t, true, true, func() {
		got := brandBanner()
		if !strings.Contains(got, "HelixCode") {
			t.Fatalf("brandBanner() must still contain the wordmark when color disabled; got:\n%s", got)
		}
		if strings.Contains(got, "\x1b[") {
			t.Fatalf("brandBanner() must emit no ANSI escape when NO_COLOR set; got:\n%q", got)
		}
	})

	// Non-TTY, NO_COLOR unset.
	withColorForced(t, false, false, func() {
		got := brandBanner()
		if strings.Contains(got, "\x1b[") {
			t.Fatalf("brandBanner() must emit no ANSI escape on a non-TTY; got:\n%q", got)
		}
	})
}

// TestBrandColorize_RawWhenNoColor asserts colorize returns the input string
// unchanged (no escape bytes at all) when NO_COLOR is set.
func TestBrandColorize_RawWhenNoColor(t *testing.T) {
	withColorForced(t, true, true, func() {
		const s = "deploy succeeded"
		if got := colorize(brandPrimary, s); got != s {
			t.Fatalf("colorize() must return raw string when NO_COLOR set; want %q got %q", s, got)
		}
		if got := brandSuccess(s); got != s {
			t.Fatalf("brandSuccess() must return raw string when NO_COLOR set; want %q got %q", s, got)
		}
	})
}

// TestBrandColorize_WrapsWhenColorEnabled asserts colorize wraps the string in
// the requested ANSI 24-bit escape + reset when color is enabled.
func TestBrandColorize_WrapsWhenColorEnabled(t *testing.T) {
	withColorForced(t, true, false, func() {
		const s = "error: boom"
		got := brandErrorText(s)
		want := "\x1b[" + brandError + "m" + s + ansiReset
		if got != want {
			t.Fatalf("brandErrorText() color-enabled mismatch; want %q got %q", want, got)
		}
		if !strings.HasSuffix(got, ansiReset) {
			t.Fatalf("colorize() must reset after the payload; got %q", got)
		}
	})
}

// TestBrandColorize_NonTTYRaw asserts that on a non-TTY (e.g. piped output)
// colorize returns the raw string even when NO_COLOR is unset.
func TestBrandColorize_NonTTYRaw(t *testing.T) {
	withColorForced(t, false, false, func() {
		const s = "info"
		if got := brandInfo(s); got != s {
			t.Fatalf("brandInfo() must return raw string on non-TTY; want %q got %q", s, got)
		}
	})
}
