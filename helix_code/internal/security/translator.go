// translator.go — CONST-046 message-ID resolver seam for
// internal/security.
//
// Round-176 §11.4 anti-bluff sweep (2026-05-19). Mirrors the
// "consumer defines its own Translator + tr() helper" pattern used
// by every other CONST-046-migrated package in this codebase
// (rounds 93/94/95/96/108/131/134/136-169, most recently
// internal/persistence 169, internal/helixqa 168, internal/focus 167,
// internal/hardware 158, internal/event 157, internal/editor 155,
// internal/discovery 154, internal/deployment 153, internal/database
// 152).
//
// Scope note: this seam covers helix_code/internal/security/ ONLY —
// the root-level security/ submodule (round 130) has its own
// translator surface and bundle prefix.
package security

import (
	stdctx "context"
	"fmt"
	"os"
	"sync"

	securityi18n "dev.helix.code/internal/security/i18n"
)

// translator resolves CONST-046 message IDs for every user-facing
// string emitted by this package. Defaults to i18n.NoopTranslator{}
// (loud message-ID echo) so unit tests + ad-hoc invocations remain
// obvious. helix_code wires a real *i18nadapter.Translator at boot
// via SetTranslator.
//
// A package-level variable is the chosen DI seam to keep the
// migration minimally invasive — SecurityManager methods are called
// from many sites (and from background goroutines for concurrent
// scans) and threading a struct-scoped translator would inflate the
// diff without behavioural benefit. The package-level seam also
// matches the pattern established by rounds 93..169.
//
// Because the same SecurityManager emits user-facing strings from
// background goroutines (concurrent scans) while SetTranslator may be
// re-invoked, both reads (tr) and writes (SetTranslator) of this
// package-level pointer MUST be guarded by translatorMu — otherwise
// the concurrent read/write is a data race (caught by `go test
// -race`), which is a §11.4.85(B) state-corruption defect.
var (
	translatorMu sync.RWMutex
	// defaultTranslator is the package default installed by init() — the
	// real embedded-bundle translator (resolved prose) when the embed
	// loads, or securityi18n.NoopTranslator{} (loud message-ID echo) if it
	// does not. SetTranslator(nil) restores THIS — "nil = restore default"
	// means "restore correct prose", not "revert to raw-key echo".
	defaultTranslator securityi18n.Translator = securityi18n.NoopTranslator{}
	translator        securityi18n.Translator = securityi18n.NoopTranslator{}
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
	tr, err := securityi18n.NewTranslator()
	if err != nil {
		fmt.Fprintf(os.Stderr,
			"internal/security i18n: embedded-bundle translator load failed; "+
				"degrading to raw message-ID echo (NoopTranslator): %v\n", err)
		return
	}
	defaultTranslator = tr
	translator = tr
}

// SetTranslator wires a CONST-046-compliant Translator. Passing nil
// resets to i18n.NoopTranslator{} (loud echo) — never silently
// disables translation lookup (which would be a §11.4 PASS-bluff at
// the i18n injection layer).
func SetTranslator(tr securityi18n.Translator) {
	translatorMu.Lock()
	defer translatorMu.Unlock()
	if tr == nil {
		translator = defaultTranslator
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
// A panicking Translator (buggy or hostile injected implementation)
// MUST NOT crash the caller — the package emits strings from many
// goroutines, including security-scan workers, so an unrecovered
// translator panic would take down the scan. The recover() below
// isolates such a panic and degrades to the message ID, matching the
// error/empty fallback behaviour (§11.4.85(B) callback-panic isolation).
func tr(ctx stdctx.Context, msgID string, data map[string]any) (result string) {
	translatorMu.RLock()
	active := translator
	translatorMu.RUnlock()
	if active == nil {
		active = securityi18n.NoopTranslator{}
	}

	defer func() {
		if r := recover(); r != nil {
			// Translator panicked — degrade loudly to the message ID
			// rather than propagating the panic to the emitting goroutine.
			result = msgID
		}
	}()

	out, err := active.T(ctx, msgID, data)
	if err != nil || out == "" {
		return msgID
	}
	return out
}
