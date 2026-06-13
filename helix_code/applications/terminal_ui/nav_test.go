package main

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func runeKey(r rune) *tcell.EventKey { return tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone) }

// TestMenuHotkeyTarget_RoutesWhenNotInInput is the regression guard for the
// keyboard-navigation dead-end (the LLM chat was unreachable because focus left
// the sidebar and no key could route back). When focus is on a non-text
// primitive (e.g. the dashboard's "New Task" button), every sidebar hotkey must
// resolve to its page.
func TestMenuHotkeyTarget_RoutesWhenNotInInput(t *testing.T) {
	btn := tview.NewButton("x")
	cases := map[rune]string{
		'd': "dashboard", 't': "tasks", 'w': "workers", 'p': "projects",
		's': "sessions", 'l': "llm", 'q': "qa", 'c': "settings",
	}
	for r, want := range cases {
		got, ok := menuHotkeyTarget(btn, runeKey(r))
		if !ok || got != want {
			t.Fatalf("rune %q: got (%q,%v), want (%q,true)", r, got, ok, want)
		}
	}
}

// TestMenuHotkeyTarget_PassThroughWhenInputFocused is the §1.1 guard: a
// menu-letter typed into the chat input / picker / form MUST pass through (so
// prompts can be typed and pickers used). Removing the input-aware early-return
// in menuHotkeyTarget makes this FAIL.
func TestMenuHotkeyTarget_PassThroughWhenInputFocused(t *testing.T) {
	for _, focus := range []tview.Primitive{
		tview.NewInputField(), tview.NewList(), tview.NewForm(), tview.NewTextArea(),
	} {
		if got, ok := menuHotkeyTarget(focus, runeKey('l')); ok {
			t.Fatalf("focus %T: 'l' must pass through (false), got (%q,true)", focus, got)
		}
	}
}

// TestMenuHotkeyTarget_IgnoresNonHotkeyAndNonRune covers boundaries.
func TestMenuHotkeyTarget_IgnoresNonHotkeyAndNonRune(t *testing.T) {
	btn := tview.NewButton("x")
	if _, ok := menuHotkeyTarget(btn, runeKey('z')); ok {
		t.Fatal("non-hotkey rune 'z' must not route")
	}
	if _, ok := menuHotkeyTarget(btn, tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)); ok {
		t.Fatal("Enter key must not route")
	}
	if _, ok := menuHotkeyTarget(btn, nil); ok {
		t.Fatal("nil event must not route")
	}
}
