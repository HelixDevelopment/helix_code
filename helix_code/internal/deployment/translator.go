// translator.go — CONST-046 message-ID resolver seam for
// internal/deployment.
//
// Round-153 §11.4 anti-bluff sweep (2026-05-18). Mirrors the
// "consumer defines its own Translator + tr() helper" pattern used
// by every other CONST-046-migrated package in this codebase
// (rounds 93/94/95/96/108/131/134/136-152).
package deployment

import (
	"context"
	"sync"

	deploymenti18n "dev.helix.code/internal/deployment/i18n"
)

// translator resolves CONST-046 message IDs for every user-facing
// string emitted by this package. Defaults to i18n.NoopTranslator{}
// (loud message-ID echo) so unit tests + ad-hoc invocations remain
// obvious. helix_code wires a real *i18nadapter.Translator at boot
// via SetTranslator.
//
// A package-level variable is the chosen DI seam to keep the
// migration minimally invasive — ProductionDeployer methods are
// called from many sites and threading a struct-scoped translator
// would inflate the diff without behavioural benefit.
//
// HXC-014 §11.4.85 data-race fix: SetTranslator (a write) can be called
// from boot/reconfiguration while tr() (a read) runs concurrently from
// deployment phases on other goroutines. The package-level var was
// previously read+written with no synchronisation, producing a data race
// detected under -race (translator.go:34 write vs :46 read). Guard every
// access with a RWMutex so the DI seam is concurrency-safe.
var (
	translatorMu sync.RWMutex
	translator   deploymenti18n.Translator = deploymenti18n.NoopTranslator{}
)

// SetTranslator wires a CONST-046-compliant Translator. Passing nil
// resets to i18n.NoopTranslator{} (loud echo) — never silently
// disables translation lookup (which would be a §11.4 PASS-bluff at
// the i18n injection layer).
func SetTranslator(tr deploymenti18n.Translator) {
	translatorMu.Lock()
	defer translatorMu.Unlock()
	if tr == nil {
		translator = deploymenti18n.NoopTranslator{}
		return
	}
	translator = tr
}

// tr is the internal CONST-046 resolver used by every user-facing
// string emission in this package. It NEVER returns an error to the
// caller — translation failures degrade to the message ID itself
// (matching NoopTranslator behaviour) so production output remains
// loud + obvious instead of silently empty.
func tr(ctx context.Context, msgID string, data map[string]any) string {
	translatorMu.RLock()
	cur := translator
	translatorMu.RUnlock()
	if cur == nil {
		// Repair the nil seam under the write lock, then re-read.
		translatorMu.Lock()
		if translator == nil {
			translator = deploymenti18n.NoopTranslator{}
		}
		cur = translator
		translatorMu.Unlock()
	}
	out, err := cur.T(ctx, msgID, data)
	if err != nil || out == "" {
		return msgID
	}
	return out
}
