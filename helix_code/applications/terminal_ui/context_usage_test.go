package main

import (
	"strings"
	"testing"
)

// context_usage_test.go — HXC-077 regression guard (§11.4.135) for the TUI
// context-window USED-% indicator. These are pure-function tests so they need
// no tview event loop or live provider.
//
// Anti-bluff (CONST-035): the indicator MUST be omitted (empty string) when the
// model's real context window is unknown — never a fabricated denominator — and
// MUST reflect the REAL used/window numbers when the window is known.

// TestFormatContextUsage_HonestOmitAndRealPercent proves formatContextUsage:
//   - returns "" when window <= 0 (honest omit — no fake denominator),
//   - clamps negative used to 0,
//   - renders the real used/window and integer percent (language-neutral body),
//   - never re-introduces a hardcoded "context:" label (that lives in i18n now).
func TestFormatContextUsage_HonestOmitAndRealPercent(t *testing.T) {
	cases := []struct {
		name        string
		used, window int
		want        string
	}{
		{"unknown_window_omits", 1234, 0, ""},
		{"negative_window_omits", 100, -1, ""},
		{"zero_used", 0, 8192, "0/8192 (0%)"},
		{"normal", 4096, 8192, "4096/8192 (50%)"},
		{"negative_used_clamped", -50, 8192, "0/8192 (0%)"},
		{"over_100_not_capped", 9000, 8192, "9000/8192 (109%)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := formatContextUsage(tc.used, tc.window)
			if got != tc.want {
				t.Fatalf("formatContextUsage(%d,%d) = %q, want %q", tc.used, tc.window, got, tc.want)
			}
			if strings.Contains(strings.ToLower(got), "context") {
				t.Fatalf("formatContextUsage must NOT hardcode a 'context' label (i18n owns it): %q", got)
			}
		})
	}
}

// TestContextUsageStatus_OmitsWhenWindowUnknown proves the TUI status fragment
// is empty when no real window is resolvable (nil provider here), so the caller
// never appends a fabricated indicator (CONST-035).
func TestContextUsageStatus_OmitsWhenWindowUnknown(t *testing.T) {
	tui := &TerminalUI{sessionUsedTokens: 500} // no provider => window unknown
	if got := tui.contextUsageStatus(); got != "" {
		t.Fatalf("contextUsageStatus with unknown window must be empty, got %q", got)
	}
}

// TestContextUsageStatus_RealWindow_LocalisedLabel proves that with a real
// context window the fragment carries the localised label from the i18n bundle
// (the terminal_ui_chat_context_usage key) wrapping the real numeric body — and
// is NOT the raw i18n key (which would mean the bundle key is missing).
func TestContextUsageStatus_RealWindow_LocalisedLabel(t *testing.T) {
	// fakeStreamProvider (chat_stream_test.go) reports GetContextWindow()==8192;
	// with selectedModel="" contextWindowForModel falls back to it — a real,
	// known window, no fabricated denominator.
	tui := &TerminalUI{llmProvider: &fakeStreamProvider{}}
	// Install the REAL embedded-bundle translator exactly as production does
	// (i18n_boot_wire.go), so this exercises genuine bundle resolution of the
	// terminal_ui_chat_context_usage key — not the NoopTranslator key-echo.
	wireTranslator(tui)
	tui.sessionUsedTokens = 2048
	got := tui.contextUsageStatus()
	if got == "" {
		t.Fatalf("contextUsageStatus must render an indicator when the window is known")
	}
	if strings.Contains(got, "terminal_ui_chat_context_usage") {
		t.Fatalf("contextUsageStatus rendered the raw i18n key (bundle key missing): %q", got)
	}
	if !strings.Contains(got, "2048/8192 (25%)") {
		t.Fatalf("contextUsageStatus must contain the real numeric body 2048/8192 (25%%), got %q", got)
	}
}
