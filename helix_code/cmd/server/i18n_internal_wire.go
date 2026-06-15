// i18n_internal_wire.go — HXC-099 Goal A: wire the CONST-046 boot-time
// translators for every importable internal/* package on the cmd/server
// entry path.
//
// `cmd/server/i18n_boot_wire.go` only self-wires THIS main package's own
// translator (the server-startup banners). It does NOT wire the ~60
// internal/* packages (internal/server's HTTP-handler strings,
// internal/config, internal/database, …) — those each ship a per-package
// SetTranslator DI seam defaulting to the loud NoopTranslator{} raw-key
// echo, and are wired centrally by i18nwiring.WireAll().
//
// Before this file, ONLY cmd/cli called i18nwiring.WireAll(); the server
// binary never did, so internal/server (and every other internal package
// it transitively emits strings through) ran on NoopTranslator{} and real
// users saw raw message-ID keys instead of resolved + interpolated prose —
// a CONST-046 / §11.4 regression on the server entry path.
//
// WireAll is invoked from init() so it runs once at process start, BEFORE
// main() constructs any subsystem that emits a user-facing string, on BOTH
// the GUI and nogui build variants (this file carries no build tag). It is
// idempotent (each SetTranslator simply re-injects an equivalent
// translator). A failed translator build is logged but non-fatal: the loud
// message-ID echo is a degraded-but-honest fallback, never a silent swallow
// (a silent-empty would itself be a §11.4 PASS-bluff at the i18n layer).
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
