// translator.go — CONST-046 message-ID resolver seam for
// internal/tools/askuser.
//
// Round-440 §11.4 anti-bluff sweep (2026-05-20). Mirrors the
// "consumer defines its own Translator + tr() helper" pattern used
// by every other CONST-046-migrated package in this codebase
// (rounds 93..436, most recently internal/render 224,
// internal/quality 223, internal/clarification 222, internal/approval
// 221, internal/commands 149/436, internal/logging 162,
// internal/hardware 158, internal/discovery 154).
//
// SCOPE: the genuine (C) user-facing CLI prompt narrative emitted by
// stdin_prompter.go (numbered-choice menu footer, choice preview
// label, invalid-input retry hint) is routed through this seam. Tool
// Schema() descriptions (LLM-facing, A) and types.go errors.New
// sentinels (errors.Is-compared technical tokens) are intentionally
// NOT migrated — see i18n/translator.go package doc for the rationale.
package askuser

import (
	stdctx "context"

	askuseri18n "dev.helix.code/internal/tools/askuser/i18n"
)

// translator resolves CONST-046 message IDs for every user-facing
// string emitted by this package. Defaults to i18n.NoopTranslator{}
// (loud message-ID echo) so unit tests + ad-hoc invocations remain
// obvious. helix_code wires a real *i18nadapter.Translator at boot
// via SetTranslator.
//
// A package-level variable is the chosen DI seam to keep the
// migration minimally invasive — FormatQuestion is a pure exported
// helper called from both the prompter and (in future) the /ask
// slash command, and threading a struct-scoped translator through
// every call site would inflate the diff without behavioural benefit.
var translator askuseri18n.Translator = askuseri18n.NoopTranslator{}

// SetTranslator wires a CONST-046-compliant Translator. Passing nil
// resets to i18n.NoopTranslator{} (loud echo) — never silently
// disables translation lookup (which would be a §11.4 PASS-bluff at
// the i18n injection layer).
func SetTranslator(tr askuseri18n.Translator) {
	if tr == nil {
		translator = askuseri18n.NoopTranslator{}
		return
	}
	translator = tr
}

// tr is the internal CONST-046 resolver for user-facing string
// emissions in this package. It NEVER returns an error to the
// caller — translation failures degrade to the message ID itself
// (matching NoopTranslator behaviour) so production output remains
// loud + obvious instead of silently empty.
func tr(ctx stdctx.Context, msgID string, data map[string]any) string {
	if translator == nil {
		translator = askuseri18n.NoopTranslator{}
	}
	out, err := translator.T(ctx, msgID, data)
	if err != nil || out == "" {
		return msgID
	}
	return out
}
