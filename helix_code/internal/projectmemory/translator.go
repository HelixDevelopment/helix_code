// translator.go — CONST-046 message-ID resolver seam for
// internal/projectmemory.
//
// Round-235 §11.4 anti-bluff sweep (2026-05-19). Mirrors the
// "consumer defines its own Translator + tr() helper" pattern used
// by every other CONST-046-migrated package in this codebase
// (rounds 93/94/95/96/108/131/134/136-162/...//226, most recently
// internal/voice 226, internal/secrets 225, internal/render 224,
// internal/quality 223, internal/clarification 222, internal/approval
// 221).
//
// NO-OP INFRA: the audit gate reports zero CONST-046 violations in
// internal/projectmemory/ at HEAD. The package's user-visible
// literals are filesystem discovery filenames, path-suffix tokens,
// the Memory.Render delimiter (test-pinned protocol marker),
// sentinel-error fragments, wrapped-error technical strings, and
// zap-logger WARN messages — explicitly out of CONST-046 scope per
// round-158's hardware-identifier-token precedent. This seam still
// lands so any FUTURE user-facing string added to
// internal/projectmemory/ inherits the standard migration pattern
// without further infra work.
package projectmemory

import (
	stdctx "context"
	"sync"

	pmi18n "dev.helix.code/internal/projectmemory/i18n"
)

// translator resolves CONST-046 message IDs for every user-facing
// string emitted by this package. Defaults to i18n.NoopTranslator{}
// (loud message-ID echo) so unit tests + ad-hoc invocations remain
// obvious. helix_code wires a real *i18nadapter.Translator at boot
// via SetTranslator.
//
// A package-level variable is the chosen DI seam to keep future
// migrations minimally invasive — MemoryLoader.Discover is called
// per LLM invocation (hot path) and MemoryWatcher debounce-fires on
// fsnotify events; threading a struct-scoped translator would inflate
// every call without behavioural benefit.
//
// HXC-014b §11.4.85: both reads (tr) and writes (SetTranslator) of
// this package-level seam MUST be guarded by translatorMu — Discover
// runs on the hot LLM path while MemoryWatcher debounce-fires on
// fsnotify goroutines and SetTranslator may be re-invoked, so an
// unguarded access is a data race (caught by `go test -race`).
var (
	translatorMu sync.RWMutex
	translator   pmi18n.Translator = pmi18n.NoopTranslator{}
)

// SetTranslator wires a CONST-046-compliant Translator. Passing nil
// resets to i18n.NoopTranslator{} (loud echo) — never silently
// disables translation lookup (which would be a §11.4 PASS-bluff at
// the i18n injection layer).
func SetTranslator(tr pmi18n.Translator) {
	translatorMu.Lock()
	defer translatorMu.Unlock()
	if tr == nil {
		translator = pmi18n.NoopTranslator{}
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
// HXC-014b §11.4.85(B): a panicking Translator (buggy or hostile
// injected implementation) MUST NOT crash the emitting goroutine —
// the recover() below isolates the panic and degrades to the message
// ID, matching the error/empty fallback behaviour.
//
//nolint:unused // reserved for future CONST-046 migrations; see translator_test.go.
func tr(ctx stdctx.Context, msgID string, data map[string]any) (result string) {
	translatorMu.RLock()
	active := translator
	translatorMu.RUnlock()
	if active == nil {
		active = pmi18n.NoopTranslator{}
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
