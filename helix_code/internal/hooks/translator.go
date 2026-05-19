// translator.go — CONST-046 message-ID resolver seam for
// internal/hooks.
//
// Round-160 §11.4 anti-bluff sweep (2026-05-19). Mirrors the
// "consumer defines its own Translator + tr() helper" pattern used
// by every other CONST-046-migrated package in this codebase
// (rounds 93/94/95/96/108/131/134/136-159, most recently
// internal/helixqa 159, internal/hardware 158, internal/focus 157,
// internal/event 156, internal/editor 155, internal/discovery 154).
package hooks

import (
	stdctx "context"

	hooksi18n "dev.helix.code/internal/hooks/i18n"
)

// translator resolves CONST-046 message IDs for every user-facing
// string emitted by this package. Defaults to i18n.NoopTranslator{}
// (loud message-ID echo) so unit tests + ad-hoc invocations remain
// obvious. helix_code wires a real *i18nadapter.Translator at boot
// via SetTranslator.
//
// A package-level variable is the chosen DI seam to keep the
// migration minimally invasive — Manager methods are called from
// many sites (HTTP handlers, CLI, background goroutines) and
// threading a struct-scoped translator would inflate the diff
// without behavioural benefit. Hook.Validate is also called from
// non-Manager paths (NewHookBuilder, RegisterMany), so a package
// seam keeps the migration uniform.
var translator hooksi18n.Translator = hooksi18n.NoopTranslator{}

// SetTranslator wires a CONST-046-compliant Translator. Passing nil
// resets to i18n.NoopTranslator{} (loud echo) — never silently
// disables translation lookup (which would be a §11.4 PASS-bluff at
// the i18n injection layer).
func SetTranslator(tr hooksi18n.Translator) {
	if tr == nil {
		translator = hooksi18n.NoopTranslator{}
		return
	}
	translator = tr
}

// tr is the internal CONST-046 resolver used by every user-facing
// string emission in this package. It NEVER returns an error to the
// caller — translation failures degrade to the message ID itself
// (matching NoopTranslator behaviour) so production output remains
// loud + obvious instead of silently empty.
func tr(ctx stdctx.Context, msgID string, data map[string]any) string {
	if translator == nil {
		translator = hooksi18n.NoopTranslator{}
	}
	out, err := translator.T(ctx, msgID, data)
	if err != nil || out == "" {
		return msgID
	}
	return out
}
