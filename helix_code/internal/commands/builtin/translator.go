// translator.go — CONST-046 message-ID resolver seam for
// internal/commands/builtin.
//
// Round-353 §11.4 anti-bluff sweep (2026-05-19). Mirrors the
// "consumer defines its own Translator + trc() helper" pattern used
// by every other CONST-046-migrated package in this codebase
// (rounds 93..315; most directly the cmd/cli round-311 trc() seam).
//
// builtin's Description() / Usage() methods are context-free (no
// request-scoped context.Context is threaded into them — they are
// metadata accessors). The trc() resolver therefore resolves against
// context.Background(), exactly like cmd/cli's trc() for cobra
// command-construction-time strings. The package-level translator is
// wired via SetTranslator before the command registry is built, so
// trc() sees a real Translator in production and i18n.NoopTranslator{}
// (loud message-ID echo) in unit tests that build commands without
// wiring one.
package builtin

import (
	"context"
	"fmt"
	"os"

	"dev.helix.code/internal/commands/builtin/i18n"
)

// translator resolves CONST-046 message IDs for every user-facing
// string emitted by this package (Description() / Usage() output).
// Defaults to i18n.NoopTranslator{} (loud message-ID echo) so unit
// tests + ad-hoc invocations remain obvious. helix_code wires a real
// *i18nadapter.Translator at boot via SetTranslator.
//
// A package-level variable is the chosen DI seam because the command
// Description()/Usage() method signatures take no parameters at all
// — threading a translator would require restructuring the
// commands.Command interface across every implementer. Global
// injection keeps the migration minimally invasive and matches the
// identical seam already in internal/commands and cmd/cli.
var (
	// defaultTranslator is the package default installed by init() — the
	// real embedded-bundle translator (resolved prose) when the embed
	// loads, or i18n.NoopTranslator{} (loud message-ID echo) if it
	// does not. SetTranslator(nil) restores THIS — "nil = restore default"
	// means "restore correct prose", not "revert to raw-key echo".
	defaultTranslator i18n.Translator = i18n.NoopTranslator{}
	translator        i18n.Translator = i18n.NoopTranslator{}
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
	tr, err := i18n.NewTranslator()
	if err != nil {
		fmt.Fprintf(os.Stderr,
			"internal/commands/builtin i18n: embedded-bundle translator load failed; "+
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
func SetTranslator(t i18n.Translator) {
	if t == nil {
		translator = defaultTranslator
		return
	}
	translator = t
}

// tr is the internal CONST-046 resolver. It NEVER returns an error to
// the caller — translation failures degrade to the message ID itself
// (matching NoopTranslator behaviour) so production output remains
// loud + obvious instead of silently empty.
func tr(ctx context.Context, msgID string, data map[string]any) string {
	if translator == nil {
		translator = i18n.NoopTranslator{}
	}
	out, err := translator.T(ctx, msgID, data)
	if err != nil || out == "" {
		return msgID
	}
	return out
}

// trc is the CONST-046 resolver for strings produced by the
// context-free Description() / Usage() command-metadata accessors. It
// resolves against context.Background() through the same package-level
// translator the runtime tr() helper uses, so command metadata is just
// as locale-aware as runtime output.
func trc(msgID string, data map[string]any) string {
	return tr(context.Background(), msgID, data)
}
