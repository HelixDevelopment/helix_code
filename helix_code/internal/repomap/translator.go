// translator.go — CONST-046 message-ID resolver seam for
// internal/repomap (round-198 §11.4 anti-bluff sweep, 2026-05-19;
// recovery from round-174 stall in which the scaffolding was
// dispatched but never written).
//
// Scope: ONE real migration target landed — the
// RepoMapTool.Description() string emitted to end users via the tool
// registry. The other strings in internal/repomap/ are either log
// framing (debug/warn output per round-162 logging precedent), tool
// category identifiers (structural lookup keys per round-158/162
// identifier precedent), or doc.go comments (godoc-only, never
// runtime-emitted). See i18n/translator.go for the full scope
// rationale.
package repomap

import (
	stdctx "context"
	"fmt"
	"os"
	"sync"

	repomapi18n "dev.helix.code/internal/repomap/i18n"
)

// translator resolves CONST-046 message IDs for every user-facing
// string emitted by this package. Defaults to i18n.NoopTranslator{}
// (loud message-ID echo). helix_code wires a real
// *i18nadapter.Translator at boot via SetTranslator.
//
// HXC-014b §11.4.85 fix: SetTranslator (a write) may run concurrently
// with tr() (a read), so both accesses MUST be guarded by translatorMu
// — otherwise the concurrent read/write is a data race (caught by
// `go test -race`), a §11.4.85(B) state-corruption defect.
var (
	translatorMu sync.RWMutex
	// defaultTranslator is the package default installed by init() — the
	// real embedded-bundle translator (resolved prose) when the embed
	// loads, or repomapi18n.NoopTranslator{} (loud message-ID echo) if it
	// does not. SetTranslator(nil) restores THIS — "nil = restore default"
	// means "restore correct prose", not "revert to raw-key echo".
	defaultTranslator repomapi18n.Translator = repomapi18n.NoopTranslator{}
	translator        repomapi18n.Translator = repomapi18n.NoopTranslator{}
)

// init installs the real embedded-bundle translator as the package
// default so user-facing strings resolve to prose on every entry path,
// not just the ones that reach i18nwiring.WireAll() (HXC-097 §11.4
// anti-bluff, 2026-06-15: library code emits raw message-ID keys when
// the package runs on NoopTranslator{} because WireAll only runs on the
// interactive-CLI path). On embed-load failure it degrades loudly to the
// NoopTranslator{} already assigned above and warns on stderr (never a
// silent swallow / empty string — that would be a §11.4 PASS-bluff).
func init() {
	tr, err := repomapi18n.NewTranslator()
	if err != nil {
		fmt.Fprintf(os.Stderr,
			"internal/repomap i18n: embedded-bundle translator load failed; "+
				"degrading to raw message-ID echo (NoopTranslator): %v\n", err)
		return
	}
	defaultTranslator = tr
	translator = tr
}

// SetTranslator wires a CONST-046-compliant Translator. Passing nil
// resets to i18n.NoopTranslator{} (loud echo) — never silently
// disables translation lookup (silent disable would be a §11.4
// PASS-bluff at the i18n layer).
func SetTranslator(tr repomapi18n.Translator) {
	translatorMu.Lock()
	defer translatorMu.Unlock()
	if tr == nil {
		translator = defaultTranslator
		return
	}
	translator = tr
}

// tr is the internal CONST-046 resolver. It NEVER returns an error
// to the caller — translation failures degrade to the message ID
// itself so production output remains loud + obvious instead of
// silently empty.
//
// HXC-014b §11.4.85(B): a panicking Translator MUST NOT crash the
// emitting goroutine — the recover() below isolates such a panic and
// degrades to the message ID.
func tr(ctx stdctx.Context, msgID string, data map[string]any) (result string) {
	translatorMu.RLock()
	active := translator
	translatorMu.RUnlock()
	if active == nil {
		active = repomapi18n.NoopTranslator{}
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
