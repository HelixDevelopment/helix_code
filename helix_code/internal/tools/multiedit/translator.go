// translator.go — CONST-046 message-ID resolver seam for
// internal/tools/multiedit.
//
// Round-428 §11.4 anti-bluff sweep (2026-05-20). Mirrors the
// "consumer defines its own Translator + trc() helper" pattern used
// by every other CONST-046-migrated package in this codebase
// (most recently internal/tools/confirmation, round-382).
//
// The migrated strings here are produced by PreviewFormatter, whose
// Format() method takes no request-scoped context.Context — it is a
// pure render accessor over a PreviewResult. The trc() resolver
// therefore resolves against context.Background(), exactly like the
// internal/commands/builtin trc() seam for context-free metadata
// accessors. The package-level translator is wired via
// SetTranslator before the multiedit tool is registered, so trc()
// sees a real Translator in production and i18n.NoopTranslator{}
// (loud message-ID echo) in unit tests that build a formatter
// without wiring one.
package multiedit

import (
	"context"
	"fmt"
	"os"

	"dev.helix.code/internal/tools/multiedit/i18n"
)

// translator resolves CONST-046 message IDs for every user-facing
// string emitted by this package (the PreviewFormatter transaction
// preview report rendered in plain / markdown / HTML form).
// Defaults to i18n.NoopTranslator{} (loud message-ID echo) so unit
// tests + ad-hoc invocations remain obvious. helix_code wires a real
// *i18nadapter.Translator at boot via SetTranslator.
//
// A package-level variable is the chosen DI seam to keep the
// migration minimally invasive — PreviewFormatter.Format() and its
// formatPlain/formatMarkdown/formatHTML helpers carry no context
// parameter, and NewPreviewFormatter is a context-free constructor.
// Threading a struct-scoped translator would inflate the diff
// without behavioural benefit and matches the seam already used in
// internal/commands/builtin.
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
			"internal/tools/multiedit i18n: embedded-bundle translator load failed; "+
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

// tr is the internal CONST-046 resolver. It NEVER returns an error
// to the caller — translation failures degrade to the message ID
// itself (matching NoopTranslator behaviour) so production output
// remains loud + obvious instead of silently empty.
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
// context-free PreviewFormatter render accessors. It resolves
// against context.Background() through the same package-level
// translator the runtime tr() helper uses, so the transaction
// preview report is just as locale-aware as any context-scoped
// output.
func trc(msgID string, data map[string]any) string {
	return tr(context.Background(), msgID, data)
}
