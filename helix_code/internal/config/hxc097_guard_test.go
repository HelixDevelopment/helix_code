// hxc097_guard_test.go — §11.4.115 RED-polarity regression guard for
// HXC-097 (i18n raw-key emission, 2026-06-15).
//
// Root cause (FACT): internal/config emitted the raw message-ID key
// "internal_config_info_using_config_file" at config.LoadConfig time
// because the package translator defaulted to i18n.NoopTranslator{} and
// the central i18nwiring.WireAll() runs too late / not on every path.
// Fix: default the package translator to the real embedded-bundle
// translator at init().
//
// Polarity switch per §11.4.115: RED_MODE (default "0") flips this single
// source between two roles —
//
//	RED_MODE=1 — reproduce-and-assert-defect: force the NoopTranslator{}
//	             default the bug shipped with and assert the raw key IS
//	             emitted (proves the test genuinely reproduces HXC-097 on a
//	             pre-fix-equivalent state).
//	RED_MODE=0 — standing GREEN regression guard: assert the as-shipped
//	             default resolves the key to bundle PROSE (the user-visible
//	             behaviour). A revert to Noop (or a broken embed) FAILs here.
package config

import (
	"context"
	"os"
	"testing"

	configi18n "dev.helix.code/internal/config/i18n"
)

func TestHXC097_Guard_UsingConfigFileKeyResolvesToProse(t *testing.T) {
	const (
		key  = "internal_config_info_using_config_file"
		want = "INFO: Using config file: /etc/helix/config.yaml"
	)
	data := map[string]any{"Path": "/etc/helix/config.yaml"}

	if os.Getenv("RED_MODE") == "1" {
		// Reproduce the defect on the pre-fix-equivalent state: force the
		// NoopTranslator{} default and assert the raw key leaks.
		resetTranslator(t)
		SetTranslator(configi18n.NoopTranslator{})
		defer resetTranslator(t)
		got := tr(context.Background(), key, data)
		if got != key {
			t.Fatalf("RED_MODE: tr(%q) = %q, expected raw-key leak %q "+
				"(RED must reproduce HXC-097 on the Noop default)", key, got, key)
		}
		t.Logf("RED_MODE reproduced HXC-097: %q -> %q (raw key leaked)", key, got)
		return
	}

	// GREEN guard: as-shipped default (init-installed real bundle).
	resetTranslator(t)
	got := tr(context.Background(), key, data)
	if got != want {
		t.Fatalf("HXC-097 GREEN guard: tr(%q) = %q, want resolved prose %q "+
			"(default must be the real embedded-bundle translator)", key, got, want)
	}
	if got == key {
		t.Fatalf("HXC-097 REGRESSION: raw message-ID key %q leaked to user "+
			"(package fell back to NoopTranslator)", key)
	}
}
