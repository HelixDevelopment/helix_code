// translator.go — CONST-046 message-ID resolver seam for internal/config.
//
// Round-150 §11.4 anti-bluff sweep (2026-05-18). Mirrors the
// "consumer defines its own Translator + tr() helper" pattern used
// by every other CONST-046-migrated package in this codebase
// (rounds 93/94/95/96/108/131/134/136-149).
package config

import (
	"context"
	"fmt"
	"os"
	"sync"

	"dev.helix.code/internal/config/i18n"
)

// translator resolves CONST-046 message IDs for every user-facing
// string emitted by this package.
//
// HXC-097 (§11.4 anti-bluff, 2026-06-15): the default is now the REAL
// embedded-bundle translator (i18n.NewTranslator, backed by the
// go:embed'd bundles/active.en.yaml), installed at package init() —
// NOT i18n.NoopTranslator{}. Root cause of HXC-097: config.LoadConfig
// emits "internal_config_info_using_config_file" at process startup,
// BEFORE the central i18nwiring.WireAll() runs (WireAll is gated behind
// the interactive-REPL-only subsystem cluster in cmd/cli, and other
// binaries never call it at all), so the package ran on NoopTranslator{}
// and users saw the raw message-ID key instead of resolved + interpolated
// prose. Defaulting to the real bundle makes the package emit correct
// prose out-of-the-box on EVERY entry path. A consumer-injected
// Translator (i18nwiring.WireAll / direct SetTranslator) still takes
// precedence — it simply re-injects an equivalent (or locale-specific)
// translator over this default.
//
// If the embedded bundle fails to load (corrupt/missing embed — should
// be impossible for a correctly-built binary), init() degrades LOUDLY to
// i18n.NoopTranslator{} (raw message-ID echo) and warns on stderr,
// never silently swallowing — a silent empty string would be a §11.4
// PASS-bluff at the i18n layer.
//
// A package-level variable is the chosen DI seam to keep the
// migration minimally invasive — config Load/Validate are free
// functions called from many sites; threading a struct-scoped
// translator would inflate the diff without behavioural benefit.
//
// HXC-014b §11.4.85 concurrency fix: both reads (tr) and writes
// (SetTranslator) of this package-level seam MUST be guarded by
// translatorMu — the package emits strings from many goroutines while
// SetTranslator may be re-invoked, so the unguarded access is a data
// race (caught by `go test -race`), a §11.4.85(B) state-corruption defect.
var (
	translatorMu sync.RWMutex
	// defaultTranslator is the package default installed by init() — the
	// real embedded-bundle translator (resolved prose) when the embed
	// loads, or i18n.NoopTranslator{} (loud message-ID echo) if it does
	// not. SetTranslator(nil) restores THIS, so "nil = restore default"
	// means "restore correct prose", not "revert to raw-key echo".
	defaultTranslator i18n.Translator = i18n.NoopTranslator{}
	translator        i18n.Translator = i18n.NoopTranslator{}
)

// init installs the real embedded-bundle translator as the package
// default so user-facing strings resolve to prose on every entry path,
// not just the ones that reach i18nwiring.WireAll(). On embed-load
// failure it degrades loudly to the NoopTranslator{} already assigned
// above and warns on stderr (never a silent swallow / empty string).
func init() {
	tr, err := i18n.NewTranslator()
	if err != nil {
		fmt.Fprintf(os.Stderr,
			"internal/config i18n: embedded-bundle translator load failed; "+
				"degrading to raw message-ID echo (NoopTranslator): %v\n", err)
		return
	}
	defaultTranslator = tr
	translator = tr
}

// SetTranslator wires a CONST-046-compliant Translator. Passing nil
// restores the package DEFAULT (the real embedded-bundle translator
// installed at init(), or i18n.NoopTranslator{} loud echo if the embed
// failed to load) — never silently disables translation lookup (which
// would be a §11.4 PASS-bluff at the i18n injection layer), and never
// reverts a correctly-loaded binary to raw-key echo.
func SetTranslator(tr i18n.Translator) {
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
// HXC-014b §11.4.85 panic isolation: a panicking Translator (buggy or
// hostile injected implementation) MUST NOT crash the emitting
// goroutine — the recover() below degrades to the message ID, matching
// the error/empty fallback behaviour.
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
