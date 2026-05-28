// translator.go — CONST-046 message-ID resolver seam for internal/project.
//
// Round-170 §11.4 anti-bluff sweep (2026-05-19). Mirrors the
// "consumer defines its own Translator + tr() helper" pattern used
// by every other CONST-046-migrated package in this codebase
// (rounds 93..169, most recently database-152, context-151,
// config-150, commands-149, cognee-148, agent-147, auth-146).
package project

import (
	stdctx "context"
	"sync"

	"dev.helix.code/internal/project/i18n"
)

// translator resolves CONST-046 message IDs for every user-facing
// string emitted by this package. Defaults to i18n.NoopTranslator{}
// (loud message-ID echo) so unit tests + ad-hoc invocations remain
// obvious. helix_code wires a real *i18nadapter.Translator at boot
// via SetTranslator.
//
// A package-level variable is the chosen DI seam to keep the
// migration minimally invasive — both Manager and DatabaseManager
// methods are called from many sites and the project store is
// process-wide; threading a struct-scoped translator would inflate
// the diff without behavioural benefit.
//
// HXC-014b §11.4.85: both reads (tr) and writes (SetTranslator) of
// this package-level seam MUST be guarded by translatorMu — the
// project store is process-wide and methods run from many concurrent
// sites while SetTranslator may be re-invoked at boot, so an
// unguarded access is a data race (caught by `go test -race`).
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
// HXC-014b §11.4.85(B): a panicking Translator (buggy or hostile
// injected implementation) MUST NOT crash the emitting goroutine —
// the recover() below isolates the panic and degrades to the message
// ID, matching the error/empty fallback behaviour.
func tr(ctx stdctx.Context, msgID string, data map[string]any) (result string) {
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
