// i18n_boot_wire.go — HXC-036 Phase 3: self-wire this main package's
// CONST-046 translator at process init. `package main` cannot be wired by
// internal/i18nwiring (Go forbids importing main), so each binary wires its
// OWN user-facing-string translator here. On embedded-bundle load failure
// the loud NoopTranslator{} echo remains (raw keys, never a silent swallow
// — a §11.4 PASS-bluff at the i18n layer would be silent-empty output).
package main

import helix_configi18n "dev.helix.code/cmd/helix_config/i18n"

func init() {
	if tr, err := helix_configi18n.NewTranslator(); err == nil {
		SetTranslator(tr)
	}
}
