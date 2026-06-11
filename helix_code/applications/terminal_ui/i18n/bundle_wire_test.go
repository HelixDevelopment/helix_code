// Verifies the boot-time translator constructor (bundle.go) actually resolves
// the message IDs that were leaking raw on the TUI landing screen. Without
// NewTranslator the terminal_ui binary ran on NoopTranslator{} and users saw
// `terminal_ui_sidebar_title` / `terminal_ui_status_bar_default` verbatim.
//
// Anti-bluff (CONST-035 / CONST-046): a real translator MUST return a non-empty
// translation that DIFFERS from the message ID — returning the ID unchanged is
// exactly the NoopTranslator echo this fix removes.
package i18n

import (
	"context"
	"testing"
)

// landingScreenKeys are the user-facing message IDs rendered on the TUI's first
// (Dashboard) screen that were observed leaking raw during the boot-screenshot
// task.
var landingScreenKeys = []string{
	"terminal_ui_sidebar_title",
	"terminal_ui_status_bar_default",
}

func TestNewTranslator_ResolvesLandingScreenKeys(t *testing.T) {
	tr, err := NewTranslator()
	if err != nil {
		t.Fatalf("NewTranslator failed (embedded bundle should always load): %v", err)
	}
	ctx := context.Background()
	for _, id := range landingScreenKeys {
		got, err := tr.T(ctx, id, nil)
		if err != nil {
			t.Errorf("T(%q) errored: %v", id, err)
			continue
		}
		if got == "" {
			t.Errorf("T(%q) returned empty string (silent-swallow bluff)", id)
		}
		if got == id {
			t.Errorf("T(%q) returned the message ID unchanged — translator is echoing like NoopTranslator (the bug)", id)
		}
	}
}

// TestNewTranslator_NotNoop guards against a regression that wires the Noop
// translator: a real translator must resolve a known key to real text.
func TestNewTranslator_NotNoop(t *testing.T) {
	tr, err := NewTranslator()
	if err != nil {
		t.Fatalf("NewTranslator failed: %v", err)
	}
	got, _ := tr.T(context.Background(), "terminal_ui_sidebar_title", nil)
	if got == "terminal_ui_sidebar_title" {
		t.Fatalf("real translator must not echo the ID; got %q", got)
	}
	t.Logf("terminal_ui_sidebar_title => %q", got)
}
