// i18n_boot_wire_test.go — HXC-036 Phase 3 runtime proof: the init() in
// i18n_boot_wire.go must have wired a REAL translator (from the embedded
// active.en.yaml bundle) before any test runs, so trc() resolves message
// IDs to real text instead of the NoopTranslator{} raw-key echo.
package main

import "testing"

func TestI18nBootWire_ResolvesRealText(t *testing.T) {
	// cli_commands_root_short is a CONST-046-migrated key shipped in
	// cmd/cli/i18n/bundles/active.en.yaml. With the boot wiring active, trc
	// MUST return the resolved English text, NOT the raw key (which is what
	// NoopTranslator{} would echo — the HXC-036 defect).
	const key = "cli_commands_root_short"
	const want = "Inspect or run user-defined Markdown slash commands"
	got := trc(key, nil)
	if got == key {
		t.Fatalf("HXC-036 regression: trc(%q) returned the raw key — boot wiring not active (NoopTranslator echo)", key)
	}
	if got != want {
		t.Fatalf("trc(%q) = %q; want resolved %q", key, got, want)
	}
}
