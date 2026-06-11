// i18n_boot_wire.go — wires the terminal_ui binary's CONST-046 translator at
// boot. `package main` cannot be wired by a shared internal package (Go forbids
// importing main), and the TUI translator is per-instance (a *TerminalUI
// field), so the wiring happens on the instance in main() BEFORE Initialize()
// (setupUI resolves sidebar/status-bar strings via tui.t(...)).
//
// On embedded-bundle load failure the loud NoopTranslator{} echo remains (raw
// keys) — never a silent swallow, which would be a §11.4 PASS-bluff at the
// i18n layer.
package main

import (
	"log"

	tuii18n "dev.helix.code/applications/terminal_ui/i18n"
)

// wireTranslator installs the real translator (embedded active.en.yaml bundle)
// onto tui, replacing the NoopTranslator{} message-ID-echo default. Returns
// true when a real translator was installed (test-observable wiring proof).
func wireTranslator(tui *TerminalUI) bool {
	tr, err := tuii18n.NewTranslator()
	if err != nil {
		log.Printf("⚠️  i18n: falling back to message-ID echo (bundle load failed): %v", err)
		return false
	}
	tui.SetTranslator(tr)
	return true
}
