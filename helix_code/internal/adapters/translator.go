// translator.go — CONST-046 message-ID resolver seam for
// internal/adapters.
//
// Round-239 §11.4 anti-bluff sweep (2026-05-19). Mirrors the
// "consumer defines its own Translator + tr() helper" pattern used
// by every other CONST-046-migrated package in this codebase
// (rounds 93/94/95/96/108/131/134/136-162/...//226, most recently
// internal/voice 226, internal/secrets 225, internal/render 224,
// internal/quality 223, internal/clarification 222,
// internal/approval 221, internal/logging 162).
//
// NO-OP INFRA: the audit gate exits 0 on internal/adapters/ at HEAD.
// The package's user-visible literals are debate-transcript format
// markers (speckit_debate_adapter/adapter.go) consumed by HelixSpecifier's
// heuristic scorer + adapter-context diagnostic metadata + wrapped-error
// technical strings (containers/adapter.go) — explicitly out of CONST-046
// scope per round-158's hardware-identifier-token precedent and round-223's
// struct-tag identity-key precedent. This seam still lands so any FUTURE
// user-facing string added to internal/adapters/ inherits the standard
// migration pattern without further infra work.
package adapters

import (
	stdctx "context"

	adaptersi18n "dev.helix.code/internal/adapters/i18n"
)

// translator resolves CONST-046 message IDs for every user-facing
// string emitted by this package. Defaults to i18n.NoopTranslator{}
// (loud message-ID echo) so unit tests + ad-hoc invocations remain
// obvious. helix_code wires a real *i18nadapter.Translator at boot
// via SetTranslator.
//
// A package-level variable is the chosen DI seam to keep future
// migrations minimally invasive — sub-package adapters are called
// from many sites and threading a struct-scoped translator would
// inflate the diff without behavioural benefit.
var translator adaptersi18n.Translator = adaptersi18n.NoopTranslator{}

// SetTranslator wires a CONST-046-compliant Translator. Passing nil
// resets to i18n.NoopTranslator{} (loud echo) — never silently
// disables translation lookup (which would be a §11.4 PASS-bluff at
// the i18n injection layer).
func SetTranslator(tr adaptersi18n.Translator) {
	if tr == nil {
		translator = adaptersi18n.NoopTranslator{}
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
//nolint:unused // reserved for future CONST-046 migrations; see translator_test.go.
func tr(ctx stdctx.Context, msgID string, data map[string]any) string {
	if translator == nil {
		translator = adaptersi18n.NoopTranslator{}
	}
	out, err := translator.T(ctx, msgID, data)
	if err != nil || out == "" {
		return msgID
	}
	return out
}
