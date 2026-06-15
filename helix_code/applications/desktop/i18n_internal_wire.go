// i18n_internal_wire.go — HXC-099 Goal A: wire the CONST-046 boot-time
// translators for every importable internal/* package on the desktop
// (Fyne GUI + nogui) entry path.
//
// The desktop binary self-wires its own client/UI translator but never
// called i18nwiring.WireAll(), so the ~60 internal/* packages it
// transitively emits user-facing strings through (internal/agent,
// internal/llm, internal/provider, internal/tools/askuser, …) ran on their
// loud NoopTranslator{} raw-key-echo default and real users saw raw
// message-ID keys instead of resolved + interpolated prose — a CONST-046 /
// §11.4 regression on the desktop entry path.
//
// WireAll runs from init() at process start, BEFORE main(), on BOTH the
// GUI (!nogui) and nogui build variants (this file carries no build tag).
// It is idempotent. A failed translator build is logged but non-fatal —
// the loud message-ID echo is a degraded-but-honest fallback, never a
// silent swallow (a §11.4 PASS-bluff at the i18n layer).
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
