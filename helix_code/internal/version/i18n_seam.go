// CONST-046 i18n seam for the internal/version package. Boot wiring
// in the consuming binary (cmd/server, cmd/cli, applications/*)
// is expected to call SetTranslator with a real
// *i18nadapter.Translator that has loaded
// internal/version/i18n/bundles/active.en.yaml (plus any locale
// overlays). Tests and any caller that has not yet wired a real
// translator fall through to NoopTranslator, which echoes the
// message ID verbatim — loud failure mode rather than silent swallow
// (round-183 §11.4 anti-bluff sweep, 2026-05-19, CONST-046 Phase 4
// round 76).
package version

import (
	"context"
	"sync"

	versioni18n "dev.helix.code/internal/version/i18n"
)

var (
	trMu         sync.RWMutex
	trTranslator versioni18n.Translator = versioni18n.NoopTranslator{}
)

// SetTranslator wires a CONST-046-compliant Translator. Passing nil
// resets to NoopTranslator (loud echo of message IDs). Thread-safe
// because GetVersion / GetFullVersion are called from arbitrary
// goroutines (every HTTP handler, every CLI invocation, every health
// probe).
func SetTranslator(t versioni18n.Translator) {
	trMu.Lock()
	defer trMu.Unlock()
	if t == nil {
		trTranslator = versioni18n.NoopTranslator{}
		return
	}
	trTranslator = t
}

// tr resolves msgID against the currently-wired Translator. If the
// translator returns an error, tr falls back to msgID itself (loud
// echo) so the caller always gets a non-empty string. This is the
// canonical accessor used by every CONST-046-migrated emission in
// internal/version. ctx is accepted (and forwarded) so future
// locale-aware callers may inject the request's locale via
// context.Value without changing this signature.
func tr(ctx context.Context, msgID string, data map[string]any) string {
	trMu.RLock()
	t := trTranslator
	trMu.RUnlock()
	out, err := t.T(ctx, msgID, data)
	if err != nil || out == "" {
		return msgID
	}
	return out
}
