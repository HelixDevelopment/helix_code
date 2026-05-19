// translator.go — CONST-046 message-ID resolver seam for
// internal/plantree.
//
// Round-233 §11.4 anti-bluff sweep (2026-05-19). Mirrors the
// "consumer defines its own Translator + tr() helper" pattern used
// by every other CONST-046-migrated package in this codebase
// (rounds 93/94/95/96/108/131/134/136-232, most recently
// internal/voice 226, internal/secrets 225, internal/render 224,
// internal/quality 223).
//
// The seam routes the six tool-description literals emitted by
// plan_tools.go (plan_create / plan_branch / plan_merge /
// plan_list / plan_show / plan_delete Description() returns)
// through a locale-aware resolver. Without this migration, every
// non-English operator sees the English description text verbatim
// — silently breaking the CLI help surface for them per CONST-046.
package plantree

import (
	"context"

	plantreei18n "dev.helix.code/internal/plantree/i18n"
)

// translator resolves CONST-046 message IDs for every user-facing
// string emitted by this package. Defaults to i18n.NoopTranslator{}
// (loud message-ID echo) so unit tests + ad-hoc invocations remain
// obvious. helix_code wires a real *i18nadapter.Translator at boot
// via SetTranslator.
//
// A package-level variable is the chosen DI seam to keep the
// migration minimally invasive — the Tool implementations
// (PlanCreateTool / PlanBranchTool / ... / PlanDeleteTool) carry
// no translator field today and threading one through the
// NewPlan*Tool constructors would break every existing call site
// that already wires the tools at boot. The package-level seam
// matches the established pattern from auth/mcp/cognee/logging/
// secrets/voice/etc.
var translator plantreei18n.Translator = plantreei18n.NoopTranslator{}

// SetTranslator wires a CONST-046-compliant Translator. Passing nil
// resets to i18n.NoopTranslator{} (loud echo) — never silently
// disables translation lookup (which would be a §11.4 PASS-bluff at
// the i18n injection layer).
func SetTranslator(tr plantreei18n.Translator) {
	if tr == nil {
		translator = plantreei18n.NoopTranslator{}
		return
	}
	translator = tr
}

// tr is the internal CONST-046 resolver used by every user-facing
// string emission in this package. It NEVER returns an error to the
// caller — translation failures degrade to the message ID itself
// (matching NoopTranslator behaviour) so production output remains
// loud + obvious instead of silently empty.
func tr(ctx context.Context, msgID string, data map[string]any) string {
	if translator == nil {
		translator = plantreei18n.NoopTranslator{}
	}
	out, err := translator.T(ctx, msgID, data)
	if err != nil || out == "" {
		return msgID
	}
	return out
}
