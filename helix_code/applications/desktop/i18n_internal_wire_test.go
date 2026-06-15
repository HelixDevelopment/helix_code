// i18n_internal_wire_test.go — §11.4.115 RED-polarity guard for HXC-099
// Goal A on the desktop entry path (2026-06-15).
//
// Proves the desktop binary wires the importable internal/* CONST-046
// packages at boot via i18nwiring.WireAll() (from this package's init() in
// i18n_internal_wire.go). Probe: askuser.FormatQuestion renders the
// "askuser_prompt_enter_choice_no_default" message through the
// internal/tools/askuser package translator (WireAll-wired; Noop default).
// No build tag — runs on both the GUI and nogui variants.
//
//	RED_MODE=1 — force the NoopTranslator{} default and assert the raw key leaks.
//	RED_MODE=0 — rely on init() having run WireAll(); assert resolved prose.
package main

import (
	"os"
	"strings"
	"testing"

	"dev.helix.code/internal/i18nwiring"
	"dev.helix.code/internal/tools/askuser"
	askuseri18n "dev.helix.code/internal/tools/askuser/i18n"
)

func TestHXC099_GoalA_DesktopEntryPathWiresInternalPackages(t *testing.T) {
	const rawKey = "askuser_prompt_enter_choice_no_default"
	q := askuser.Question{Question: "pick", Choices: []askuser.Choice{{Label: "x"}}}

	if os.Getenv("RED_MODE") == "1" {
		askuser.SetTranslator(askuseri18n.NoopTranslator{})
		got := askuser.FormatQuestion(q)
		if !strings.Contains(got, rawKey) {
			t.Fatalf("RED_MODE: FormatQuestion = %q, expected raw-key %q to leak", got, rawKey)
		}
		t.Logf("RED_MODE reproduced HXC-099 Goal A: raw key %q leaked", rawKey)
		_ = i18nwiring.WireAll()
		return
	}

	got := askuser.FormatQuestion(q)
	if strings.Contains(got, rawKey) {
		t.Fatalf("HXC-099 GoalA REGRESSION: FormatQuestion = %q still leaks raw key %q "+
			"(desktop entry path did not call i18nwiring.WireAll at boot)", got, rawKey)
	}
	if !strings.Contains(got, "Enter choice [1-1]:") {
		t.Fatalf("HXC-099 GoalA: FormatQuestion = %q, expected resolved prose 'Enter choice [1-1]:'", got)
	}
}
