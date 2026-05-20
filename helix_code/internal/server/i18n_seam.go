// CONST-046 i18n seam for the internal/server package. Boot wiring
// in cmd/server is expected to call SetTranslator with a real
// *i18nadapter.Translator that has loaded
// internal/server/i18n/bundles/active.en.yaml (plus any locale
// overlays). Tests and any caller that has not yet wired a real
// translator fall through to NoopTranslator, which echoes the
// message ID verbatim — loud failure mode rather than silent
// swallow (round-177 §11.4 anti-bluff sweep, 2026-05-19, CONST-046
// Phase 4 round 70).
package server

import (
	"context"
	"sync"

	serveri18n "dev.helix.code/internal/server/i18n"
	"github.com/gin-gonic/gin"
)

var (
	trMu         sync.RWMutex
	trTranslator serveri18n.Translator = serveri18n.NoopTranslator{}
)

// SetTranslator wires a CONST-046-compliant Translator. Passing nil
// resets to NoopTranslator (loud echo of message IDs). Thread-safe.
func SetTranslator(t serveri18n.Translator) {
	trMu.Lock()
	defer trMu.Unlock()
	if t == nil {
		trTranslator = serveri18n.NoopTranslator{}
		return
	}
	trTranslator = t
}

// reqCtx returns the request-scoped context for a Gin context,
// degrading to context.Background() when c or c.Request is nil.
// gin.CreateTestContext (used pervasively in *_test.go) leaves
// c.Request nil, so calling c.Request.Context() directly panics —
// the round-350 §11.4 anti-bluff sweep widened the CONST-046 tr()
// migration across handlers.go and this helper is the single
// nil-safe accessor every migrated callsite uses. It also repairs
// the pre-existing latent panic in qa_handlers.go (round-70 wiring
// that assumed a non-nil c.Request).
func reqCtx(c *gin.Context) context.Context {
	if c == nil || c.Request == nil {
		return context.Background()
	}
	return c.Request.Context()
}

// tr resolves msgID against the currently-wired Translator. If the
// translator returns an error, tr falls back to msgID itself (loud
// echo) so the caller always gets a non-empty string. This is the
// canonical handler-layer accessor used by every CONST-046-migrated
// response in internal/server.
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
