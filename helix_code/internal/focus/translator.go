// translator.go — CONST-046 message-ID resolver seam for
// internal/focus.
//
// Round-157 §11.4 anti-bluff sweep (2026-05-18). Mirrors the
// "consumer defines its own Translator + tr() helper" pattern used
// by every other CONST-046-migrated package in this codebase
// (rounds 93/94/95/96/108/131/134/136-156, most recently
// internal/event 156, internal/editor 155, internal/discovery 154).
package focus

import (
	stdctx "context"
	"sync"

	focusi18n "dev.helix.code/internal/focus/i18n"
)

// translator resolves CONST-046 message IDs for every user-facing
// string emitted by this package. Defaults to i18n.NoopTranslator{}
// (loud message-ID echo) so unit tests + ad-hoc invocations remain
// obvious. helix_code wires a real *i18nadapter.Translator at boot
// via SetTranslator.
//
// A package-level variable is the chosen DI seam to keep the
// migration minimally invasive — Manager/Chain/Focus methods are
// called from many sites and threading a struct-scoped translator
// would inflate the diff without behavioural benefit.
//
// HXC-014b §11.4.85 concurrency fix: both reads (tr) and writes
// (SetTranslator) of this package-level seam MUST be guarded by
// translatorMu — the package emits strings from many goroutines while
// SetTranslator may be re-invoked, so the unguarded access is a data
// race (caught by `go test -race`), a §11.4.85(B) state-corruption defect.
var (
	translatorMu sync.RWMutex
	translator   focusi18n.Translator = focusi18n.NoopTranslator{}
)

// SetTranslator wires a CONST-046-compliant Translator. Passing nil
// resets to i18n.NoopTranslator{} (loud echo) — never silently
// disables translation lookup (which would be a §11.4 PASS-bluff at
// the i18n injection layer).
func SetTranslator(tr focusi18n.Translator) {
	translatorMu.Lock()
	defer translatorMu.Unlock()
	if tr == nil {
		translator = focusi18n.NoopTranslator{}
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
// HXC-014b §11.4.85 panic isolation: a panicking Translator (buggy or
// hostile injected implementation) MUST NOT crash the emitting
// goroutine — the recover() below degrades to the message ID, matching
// the error/empty fallback behaviour.
func tr(ctx stdctx.Context, msgID string, data map[string]any) (result string) {
	translatorMu.RLock()
	active := translator
	translatorMu.RUnlock()
	if active == nil {
		active = focusi18n.NoopTranslator{}
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
