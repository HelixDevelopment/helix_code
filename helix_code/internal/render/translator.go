// translator.go — CONST-046 message-ID resolver seam for
// internal/render.
//
// Round-224 §11.4 anti-bluff sweep (2026-05-19). Mirrors the
// "consumer defines its own Translator + tr() helper" pattern used
// by every other CONST-046-migrated package in this codebase
// (rounds 93/94/95/96/108/131/134/136-162/...//223, most recently
// internal/quality 223, internal/clarification 222, internal/approval
// 221, internal/logging 162, internal/hardware 158, internal/discovery
// 154, internal/deployment 153, internal/database 152).
//
// NO-OP INFRA: the audit gate reports zero CONST-046 violations in
// internal/render/ at HEAD. The package's user-visible literals are
// RenderMode parse-domain tokens ("fancy"/"plain"/"auto"), env-var
// key, BlockID anchors, ANSI escape sequences, and sentinel errors
// — explicitly out of CONST-046 scope per round-158's hardware
// identifier-token precedent. This seam still lands so any FUTURE
// user-facing string added to internal/render/ inherits the
// standard migration pattern without further infra work.
package render

import (
	stdctx "context"
	"sync"

	renderi18n "dev.helix.code/internal/render/i18n"
)

// translator resolves CONST-046 message IDs for every user-facing
// string emitted by this package. Defaults to i18n.NoopTranslator{}
// (loud message-ID echo) so unit tests + ad-hoc invocations remain
// obvious. helix_code wires a real *i18nadapter.Translator at boot
// via SetTranslator.
//
// A package-level variable is the chosen DI seam to keep future
// migrations minimally invasive — renderer constructors / frame
// emitters are called from many sites and threading a struct-scoped
// translator would inflate the diff without behavioural benefit.
//
// HXC-014b §11.4.85 fix: SetTranslator (a write) may run concurrently
// with tr() (a read), so both accesses MUST be guarded by translatorMu
// — otherwise the concurrent read/write is a data race (caught by
// `go test -race`), a §11.4.85(B) state-corruption defect.
var (
	translatorMu sync.RWMutex
	translator   renderi18n.Translator = renderi18n.NoopTranslator{}
)

// SetTranslator wires a CONST-046-compliant Translator. Passing nil
// resets to i18n.NoopTranslator{} (loud echo) — never silently
// disables translation lookup (which would be a §11.4 PASS-bluff at
// the i18n injection layer).
func SetTranslator(tr renderi18n.Translator) {
	translatorMu.Lock()
	defer translatorMu.Unlock()
	if tr == nil {
		translator = renderi18n.NoopTranslator{}
		return
	}
	translator = tr
}

// tr is the internal CONST-046 resolver reserved for future
// user-facing string emissions in this package. It NEVER returns an
// error to the caller — translation failures degrade to the message
// ID itself (matching NoopTranslator behaviour) so production output
// remains loud + obvious instead of silently empty.
//
// HXC-014b §11.4.85(B): a panicking Translator MUST NOT crash the
// emitting goroutine — the recover() below isolates such a panic and
// degrades to the message ID.
//
//nolint:unused // reserved for future CONST-046 migrations; see translator_test.go.
func tr(ctx stdctx.Context, msgID string, data map[string]any) (result string) {
	translatorMu.RLock()
	active := translator
	translatorMu.RUnlock()
	if active == nil {
		active = renderi18n.NoopTranslator{}
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
