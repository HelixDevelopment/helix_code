// hxc097_guard_test.go — §11.4.115 RED-polarity regression guard for
// HXC-097 (i18n raw-key emission, 2026-06-15).
//
// Root cause (FACT): internal/database emitted the raw message-ID key
// "internal_database_ping_failed" because the package translator defaulted
// to i18n.NoopTranslator{} and the central i18nwiring.WireAll() runs too
// late / not on every path. Fix: default the package translator to the
// real embedded-bundle translator at init().
//
// Polarity switch per §11.4.115: RED_MODE (default "0") —
//
//	RED_MODE=1 — reproduce-and-assert-defect: force the NoopTranslator{}
//	             default and assert the raw key IS emitted.
//	RED_MODE=0 — standing GREEN regression guard: assert the as-shipped
//	             default resolves the key to bundle PROSE.
package database

import (
	stdctx "context"
	"os"
	"testing"

	databasei18n "dev.helix.code/internal/database/i18n"
)

func TestHXC097_Guard_PingFailedKeyResolvesToProse(t *testing.T) {
	const (
		key  = "internal_database_ping_failed"
		want = "failed to ping database: boom"
	)
	data := map[string]any{"Err": "boom"}

	if os.Getenv("RED_MODE") == "1" {
		resetTranslator(t)
		SetTranslator(databasei18n.NoopTranslator{})
		defer resetTranslator(t)
		got := tr(stdctx.Background(), key, data)
		if got != key {
			t.Fatalf("RED_MODE: tr(%q) = %q, expected raw-key leak %q "+
				"(RED must reproduce HXC-097 on the Noop default)", key, got, key)
		}
		t.Logf("RED_MODE reproduced HXC-097: %q -> %q (raw key leaked)", key, got)
		return
	}

	resetTranslator(t)
	got := tr(stdctx.Background(), key, data)
	if got != want {
		t.Fatalf("HXC-097 GREEN guard: tr(%q) = %q, want resolved prose %q "+
			"(default must be the real embedded-bundle translator)", key, got, want)
	}
	if got == key {
		t.Fatalf("HXC-097 REGRESSION: raw message-ID key %q leaked to user "+
			"(package fell back to NoopTranslator)", key)
	}
}
