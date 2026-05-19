// CONST-046 i18n seam for the internal/fix package. Boot wiring in
// cmd/security_fix (and any consumer that triggers automated
// security-fix runs) is expected to call SetTranslator with a real
// *i18nadapter.Translator that has loaded
// internal/fix/i18n/bundles/active.en.yaml (plus any locale
// overlays). Tests and any caller that has not yet wired a real
// translator fall through to NoopTranslator, which echoes the
// message ID verbatim — loud failure mode rather than silent swallow
// (round-187 §11.4 anti-bluff sweep, 2026-05-19, CONST-046 Phase 4
// round 80).
//
// fix.go's exported entry points (FixAllCriticalSecurityIssues) do
// not currently take a context.Context — keeping the public API
// stable is a §11.4.17 concern. The migrated call sites therefore
// pass context.Background() to tr(); when the caller wires a real
// Translator at boot, the active locale is read from a process-wide
// configuration source (Viper / env) rather than per-call ctx. A
// future round can plumb ctx through if per-request locale
// overrides become a product requirement.
package fix

import (
	"context"
	"sync"

	fixi18n "dev.helix.code/internal/fix/i18n"
)

var (
	trMu         sync.RWMutex
	trTranslator fixi18n.Translator = fixi18n.NoopTranslator{}
)

// SetTranslator wires a CONST-046-compliant Translator. Passing nil
// resets to NoopTranslator (loud echo of message IDs). Thread-safe.
func SetTranslator(t fixi18n.Translator) {
	trMu.Lock()
	defer trMu.Unlock()
	if t == nil {
		trTranslator = fixi18n.NoopTranslator{}
		return
	}
	trTranslator = t
}

// tr resolves msgID against the currently-wired Translator. If the
// translator returns an error, tr falls back to msgID itself (loud
// echo) so the caller always gets a non-empty string. This is the
// canonical accessor used by every CONST-046-migrated emission in
// internal/fix.
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
