// translator.go — CONST-046 message-ID resolver seam for
// internal/template.
//
// Round-180 §11.4 anti-bluff sweep (2026-05-19, Phase 4 round 73).
// Mirrors the "consumer defines its own Translator + tr() helper"
// pattern used by every other CONST-046-migrated package in this
// codebase (rounds 93/94/95/96/108/131/134/136-179, most recently
// internal/hooks 160, internal/helixqa 159, internal/hardware 158,
// internal/focus 157, internal/event 156, internal/editor 155,
// internal/discovery 154).
package template

import (
	stdctx "context"
	"sync"

	templatei18n "dev.helix.code/internal/template/i18n"
)

// translator resolves CONST-046 message IDs for every user-facing
// string emitted by this package. Defaults to i18n.NoopTranslator{}
// (loud message-ID echo) so unit tests + ad-hoc invocations remain
// obvious. helix_code wires a real *i18nadapter.Translator at boot
// via SetTranslator.
//
// A package-level variable is the chosen DI seam to keep the
// migration minimally invasive — Manager methods + Template methods
// are called from many sites (CLI, HTTP handlers, background
// rendering goroutines) and threading a struct-scoped translator
// would inflate the diff without behavioural benefit. Template
// validation also runs from non-Manager paths (NewTemplate +
// Register, ParseTemplate, ad-hoc Validate), so a package seam
// keeps the migration uniform.
//
// HXC-014b §11.4.85 fix: SetTranslator (a write) may run concurrently
// with tr() (a read) from background rendering goroutines, so both
// accesses MUST be guarded by translatorMu — otherwise the concurrent
// read/write is a data race (caught by `go test -race`), a §11.4.85(B)
// state-corruption defect.
var (
	translatorMu sync.RWMutex
	translator   templatei18n.Translator = templatei18n.NoopTranslator{}
)

// SetTranslator wires a CONST-046-compliant Translator. Passing nil
// resets to i18n.NoopTranslator{} (loud echo) — never silently
// disables translation lookup (which would be a §11.4 PASS-bluff at
// the i18n injection layer).
func SetTranslator(tr templatei18n.Translator) {
	translatorMu.Lock()
	defer translatorMu.Unlock()
	if tr == nil {
		translator = templatei18n.NoopTranslator{}
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
// HXC-014b §11.4.85(B): a panicking Translator MUST NOT crash the
// emitting goroutine — the recover() below isolates such a panic and
// degrades to the message ID.
func tr(ctx stdctx.Context, msgID string, data map[string]any) (result string) {
	translatorMu.RLock()
	active := translator
	translatorMu.RUnlock()
	if active == nil {
		active = templatei18n.NoopTranslator{}
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
