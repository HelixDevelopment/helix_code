// translator.go — CONST-046 message-ID resolver seam for
// internal/provider.
//
// Round-171 §11.4 anti-bluff sweep (2026-05-19). Mirrors the
// "consumer defines its own Translator + tr() helper" pattern
// used by every other CONST-046-migrated package in this
// codebase (rounds 93/94/95/96/108/131/134/136-146/161).
//
// internal/provider currently emits zero user-facing strings —
// the package is pure interface + ProviderType protocol-
// identifier constants. The seam is provisioned now so future
// user-facing additions (error messages, operator-visible health
// status labels) land in established CONST-046-compliant
// structure without scaffolding retrofit.
package provider

import (
	"context"

	"dev.helix.code/internal/provider/i18n"
)

// translator resolves CONST-046 message IDs for every
// user-facing string emitted by this package. Defaults to
// i18n.NoopTranslator{} (loud message-ID echo) so unit tests +
// ad-hoc invocations remain obvious. helix_code wires a real
// *i18nadapter.Translator at boot via SetTranslator.
//
// A package-level variable is the chosen DI seam to keep future
// migrations minimally invasive — the Provider interface methods
// are implemented by many concrete types in sibling packages,
// and threading a translator field through every implementation
// would inflate diffs without behavioural benefit. Free
// functions / package-scoped helpers added later can resolve
// strings via tr() directly.
var translator i18n.Translator = i18n.NoopTranslator{}

// SetTranslator wires a CONST-046-compliant Translator. Passing
// nil resets to i18n.NoopTranslator{} (loud echo) — never
// silently disables translation lookup (which would be a §11.4
// PASS-bluff at the i18n injection layer).
func SetTranslator(tr i18n.Translator) {
	if tr == nil {
		translator = i18n.NoopTranslator{}
		return
	}
	translator = tr
}

// tr is the internal CONST-046 resolver used by every
// user-facing string emission in this package. It NEVER returns
// an error to the caller — translation failures degrade to the
// message ID itself (matching NoopTranslator behaviour) so
// production output remains loud + obvious instead of silently
// empty.
//
// Currently unused: internal/provider emits zero user-facing
// strings as of round-171. Retained as the ready seam for
// future migrations. The build-tag-free presence keeps the
// helper compile-checked against the i18n contract on every
// build.
func tr(ctx context.Context, msgID string, data map[string]any) string { //nolint:unused // CONST-046 seam ready for future migrations
	if translator == nil {
		translator = i18n.NoopTranslator{}
	}
	out, err := translator.T(ctx, msgID, data)
	if err != nil || out == "" {
		return msgID
	}
	return out
}
