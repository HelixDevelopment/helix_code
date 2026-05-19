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

	repomapi18n "dev.helix.code/internal/repomap/i18n"
)

// translator resolves CONST-046 message IDs for every user-facing
// string emitted by this package. Defaults to i18n.NoopTranslator{}
// (loud message-ID echo). helix_code wires a real
// *i18nadapter.Translator at boot via SetTranslator.
var translator repomapi18n.Translator = repomapi18n.NoopTranslator{}

// SetTranslator wires a CONST-046-compliant Translator. Passing nil
// resets to i18n.NoopTranslator{} (loud echo) — never silently
// disables translation lookup (silent disable would be a §11.4
// PASS-bluff at the i18n layer).
func SetTranslator(tr repomapi18n.Translator) {
	if tr == nil {
		translator = repomapi18n.NoopTranslator{}
		return
	}
	translator = tr
}

// tr is the internal CONST-046 resolver. It NEVER returns an error
// to the caller — translation failures degrade to the message ID
// itself so production output remains loud + obvious instead of
// silently empty.
func tr(ctx stdctx.Context, msgID string, data map[string]any) string {
	if translator == nil {
		translator = repomapi18n.NoopTranslator{}
	}
	out, err := translator.T(ctx, msgID, data)
	if err != nil || out == "" {
		return msgID
	}
	return out
}
