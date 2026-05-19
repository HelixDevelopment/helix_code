// CONST-046 i18n seam for the internal/workflow/autonomy package.
// Boot wiring (cmd/server or any embedding host) is expected to call
// SetTranslator with a real *i18nadapter.Translator that has loaded
// internal/workflow/i18n/bundles/active.en.yaml (plus any locale
// overlays). Tests and any caller that has not yet wired a real
// translator fall through to NoopTranslator, which echoes the
// message ID verbatim — loud failure mode rather than silent swallow
// (round-185 §11.4 anti-bluff sweep, 2026-05-19, CONST-046 Phase 4
// round 78).
package autonomy

import (
	"context"
	"sync"

	workflowi18n "dev.helix.code/internal/workflow/i18n"
)

var (
	trMu         sync.RWMutex
	trTranslator workflowi18n.Translator = workflowi18n.NoopTranslator{}
)

// SetTranslator wires a CONST-046-compliant Translator. Passing nil
// resets to NoopTranslator (loud echo of message IDs). Thread-safe.
func SetTranslator(t workflowi18n.Translator) {
	trMu.Lock()
	defer trMu.Unlock()
	if t == nil {
		trTranslator = workflowi18n.NoopTranslator{}
		return
	}
	trTranslator = t
}

// tr resolves msgID against the currently-wired Translator. If the
// translator returns an error, tr falls back to msgID itself (loud
// echo) so the caller always gets a non-empty string. This is the
// canonical accessor used by every CONST-046-migrated label /
// status string in internal/workflow/autonomy.
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
