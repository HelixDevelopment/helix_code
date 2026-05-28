// translator.go — CONST-046 message-ID resolver seam for
// internal/secrets.
//
// Round-225 §11.4 anti-bluff sweep (2026-05-19). Mirrors the
// "consumer defines its own Translator + tr() helper" pattern used
// by every other CONST-046-migrated package in this codebase
// (rounds 93/94/95/96/108/131/134/136-146/etc.).
//
// CONST-042 anti-leak guarantee: the seam introduced here is
// strictly for error-text translation. loader.go does not log or
// emit any loaded credential value (CONST-042 §12.1); this seam
// preserves that posture — no message ID, no bundle entry, and no
// tr() call site in this package ever carries a secret through
// templateData.
package secrets

import (
	"context"
	"sync"

	"dev.helix.code/internal/secrets/i18n"
)

// translator resolves CONST-046 message IDs for every user-facing
// string emitted by this package. Defaults to i18n.NoopTranslator{}
// (loud message-ID echo) so unit tests + ad-hoc invocations remain
// obvious. helix_code wires a real *i18nadapter.Translator at boot
// via SetTranslator.
//
// A package-level variable is the chosen DI seam to keep the
// migration minimally invasive — LoadAPIKeys is a package-level
// function (not a method on a struct), and threading a translator
// parameter through its signature would break every existing
// caller. The package-level seam matches the established pattern
// from auth/mcp/cognee/etc.
//
// HXC-014b §11.4.85 fix: SetTranslator (a write) may run concurrently
// with tr() (a read), so both accesses MUST be guarded by translatorMu
// — otherwise the concurrent read/write is a data race (caught by
// `go test -race`), a §11.4.85(B) state-corruption defect.
var (
	translatorMu sync.RWMutex
	translator   i18n.Translator = i18n.NoopTranslator{}
)

// SetTranslator wires a CONST-046-compliant Translator. Passing nil
// resets to i18n.NoopTranslator{} (loud echo) — never silently
// disables translation lookup (which would be a §11.4 PASS-bluff at
// the i18n injection layer).
func SetTranslator(tr i18n.Translator) {
	translatorMu.Lock()
	defer translatorMu.Unlock()
	if tr == nil {
		translator = i18n.NoopTranslator{}
		return
	}
	translator = tr
}

// tr is the internal CONST-046 resolver used by every user-facing
// string emission in this package. It NEVER returns an error to the
// caller — translation failures degrade to the message ID itself
// (matching NoopTranslator behaviour) so production output remains
// loud + obvious instead of silently empty.
//
// CONST-042 §12.1: callers in this package MUST NOT pass secret
// material via the data map. The only current call site (the
// no-source-found error in LoadAPIKeys) carries no data at all,
// which is the safest stance — there is literally no channel for
// a secret to leak through this seam.
//
// HXC-014b §11.4.85(B): a panicking Translator MUST NOT crash the
// emitting goroutine — the recover() below isolates such a panic and
// degrades to the message ID.
func tr(ctx context.Context, msgID string, data map[string]any) (result string) {
	translatorMu.RLock()
	active := translator
	translatorMu.RUnlock()
	if active == nil {
		active = i18n.NoopTranslator{}
	}

	defer func() {
		if r := recover(); r != nil {
			result = msgID
		}
	}()

	out, err := active.T(ctx, msgID, data)
	if err != nil || out == "" {
		return msgID
	}
	return out
}
