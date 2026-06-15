// i18n_internal_wire.go — HXC-099 Goal A: wire the CONST-046 boot-time
// translators for every importable internal/* package on the terminal_ui
// entry path.
//
// `applications/terminal_ui/i18n_boot_wire.go` only wires the per-instance
// TUI translator (sidebar/status-bar strings via tui.t). It does NOT wire
// the ~60 internal/* packages (internal/agent, internal/llm,
// internal/provider, internal/tools/askuser, …) that the TUI session
// transitively emits user-facing strings through — those each default to
// the loud NoopTranslator{} raw-key echo and are wired centrally by
// i18nwiring.WireAll().
//
// Before this file, ONLY cmd/cli called i18nwiring.WireAll(); the TUI
// binary never did, so those internal packages ran on NoopTranslator{} and
// real users saw raw message-ID keys instead of resolved + interpolated
// prose — a CONST-046 / §11.4 regression on the terminal_ui entry path.
//
// WireAll runs from init() at process start, BEFORE main() builds any
// subsystem that emits a user-facing string, on every build variant (no
// build tag). It is idempotent. A failed translator build is logged but
// non-fatal — the loud message-ID echo is a degraded-but-honest fallback,
// never a silent swallow (a §11.4 PASS-bluff at the i18n layer).
package main

import (
	"log"

	"dev.helix.code/internal/i18nwiring"
)

func init() {
	if err := i18nwiring.WireAll(); err != nil {
		log.Printf("i18n: boot-time internal-package translator wiring failed (prompts degrade to message-ID echo): %v", err)
	}
}
