// i18n_internal_wire_test.go — §11.4.115 RED-polarity guard for HXC-099
// Goal A on the cmd/server entry path (2026-06-15).
//
// Proves that the server binary wires the importable internal/* CONST-046
// packages at boot via i18nwiring.WireAll() (called from this package's
// init() in i18n_internal_wire.go). The observable probe is
// askuser.FormatQuestion, which renders the "askuser_prompt_enter_choice_no_default"
// message through the internal/tools/askuser package translator — a package
// WireAll wires and whose default is the loud NoopTranslator{} raw-key echo.
//
//	RED_MODE=1 — reproduce-and-assert-defect: reset askuser to its
//	             NoopTranslator{} default (the pre-wiring state every
//	             non-cli entry path shipped) and assert FormatQuestion echoes
//	             the raw message-ID key.
//	RED_MODE=0 — standing GREEN guard: rely on this package's init() having
//	             run WireAll(), and assert FormatQuestion renders resolved
//	             prose (NOT the raw key) — i.e. the server entry path really
//	             wires the internal packages.
package main

import (
	"os"
	"strings"
	"testing"

	"dev.helix.code/internal/i18nwiring"
	"dev.helix.code/internal/tools/askuser"
	askuseri18n "dev.helix.code/internal/tools/askuser/i18n"
)

const (
	hxc099RawKey = "askuser_prompt_enter_choice_no_default"
)

func hxc099Question() askuser.Question {
	return askuser.Question{
		Question: "pick",
		Choices:  []askuser.Choice{{Label: "x"}},
	}
}

func TestHXC099_GoalA_ServerEntryPathWiresInternalPackages(t *testing.T) {
	if os.Getenv("RED_MODE") == "1" {
		// Reproduce the pre-wiring defect: force the NoopTranslator{} default.
		askuser.SetTranslator(askuseri18n.NoopTranslator{})
		got := askuser.FormatQuestion(hxc099Question())
		if !strings.Contains(got, hxc099RawKey) {
			t.Fatalf("RED_MODE: FormatQuestion = %q, expected raw-key %q to leak "+
				"(RED must reproduce HXC-099 Goal A on the Noop default)", got, hxc099RawKey)
		}
		t.Logf("RED_MODE reproduced HXC-099 Goal A: raw key %q leaked", hxc099RawKey)
		// Restore the wired state for any later test in this binary.
		_ = i18nwiring.WireAll()
		return
	}

	// GREEN guard: init() (i18n_internal_wire.go) ran WireAll() at process
	// start. The askuser package must now render resolved prose, not the raw
	// key — proving the server entry path wires the internal packages.
	got := askuser.FormatQuestion(hxc099Question())
	if strings.Contains(got, hxc099RawKey) {
		t.Fatalf("HXC-099 GoalA REGRESSION: FormatQuestion = %q still leaks raw key %q "+
			"(server entry path did not call i18nwiring.WireAll at boot)", got, hxc099RawKey)
	}
	if !strings.Contains(got, "Enter choice [1-1]:") {
		t.Fatalf("HXC-099 GoalA: FormatQuestion = %q, expected resolved prose "+
			"'Enter choice [1-1]:' from the wired askuser translator", got)
	}
}
