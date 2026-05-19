// translator.go — CONST-046 message-ID resolver seam for
// internal/verifier (round-182 §11.4 anti-bluff sweep, 2026-05-19).
//
// NO-OP INFRA: the audit gate reports only two CONST-046 findings in
// internal/verifier/ at HEAD, both for the model DisplayName/Name
// string "Grok-3 Fast Beta" in fallback_models.go. These are model
// IDENTITY tokens consumed downstream as equality keys by provider
// routing and the LLMsVerifier scoring pipeline (CONST-036/037
// single source of truth) — explicitly out of CONST-046 scope per
// round-158's hardware-identifier-token precedent and round-162's
// logging log-level-identifier precedent. Translating brand names
// like "Grok-3 Fast Beta" would silently rewrite identity keys used
// by model-routing pipelines and break the §CONST-037 promise. This
// seam still lands so any FUTURE user-facing string added to
// internal/verifier/ inherits the standard migration pattern without
// further infra work.
package verifier

import (
	stdctx "context"

	verifieri18n "dev.helix.code/internal/verifier/i18n"
)

// translator resolves CONST-046 message IDs for every user-facing
// string emitted by this package. Defaults to i18n.NoopTranslator{}
// (loud message-ID echo). helix_code wires a real
// *i18nadapter.Translator at boot via SetTranslator.
var translator verifieri18n.Translator = verifieri18n.NoopTranslator{}

// SetTranslator wires a CONST-046-compliant Translator. Passing nil
// resets to i18n.NoopTranslator{} (loud echo) — never silently
// disables translation lookup.
func SetTranslator(tr verifieri18n.Translator) {
	if tr == nil {
		translator = verifieri18n.NoopTranslator{}
		return
	}
	translator = tr
}

// tr is the internal CONST-046 resolver. It NEVER returns an error
// to the caller — translation failures degrade to the message ID
// itself so production output remains loud + obvious instead of
// silently empty.
//
//nolint:unused // reserved for future CONST-046 migrations; see translator_test.go.
func tr(ctx stdctx.Context, msgID string, data map[string]any) string {
	if translator == nil {
		translator = verifieri18n.NoopTranslator{}
	}
	out, err := translator.T(ctx, msgID, data)
	if err != nil || out == "" {
		return msgID
	}
	return out
}
