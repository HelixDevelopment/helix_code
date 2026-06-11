package main

import (
	"testing"
)

// TestWireTranslator_RemovesRawIDEcho is the end-to-end guard for the TUI i18n
// bluff fix. It asserts the RED→GREEN transition on the actual resolution path
// the landing screen uses (tui.t):
//
//	RED  : a fresh TerminalUI echoes the raw message ID (NoopTranslator default)
//	GREEN: after wireTranslator(tui), the same ID resolves to real, different text
//
// If wireTranslator is ever removed or regressed to install the Noop translator,
// this test FAILs (the GREEN assertion reverts to the RED echo).
func TestWireTranslator_RemovesRawIDEcho(t *testing.T) {
	const id = "terminal_ui_status_bar_default"

	tui := NewTerminalUI()

	// RED baseline: default is NoopTranslator → raw-ID echo (the observed bug).
	if got := tui.t(id); got != id {
		t.Fatalf("expected raw-ID echo from the NoopTranslator default, got %q", got)
	}

	// GREEN: wire the real translator (the fix).
	if !wireTranslator(tui) {
		t.Fatalf("wireTranslator returned false — embedded bundle should always load")
	}

	got := tui.t(id)
	if got == "" {
		t.Fatalf("resolved string is empty (silent-swallow bluff)")
	}
	if got == id {
		t.Fatalf("after wiring, %q still echoes the raw ID — translator not installed", id)
	}
	t.Logf("GREEN: %q resolves to %q after wiring", id, got)
}
