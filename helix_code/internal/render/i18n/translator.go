// Package i18n declares internal/render's hardcoded-content
// abstraction per CONST-046 (round-224 §11.4 anti-bluff sweep,
// 2026-05-19). Mirrors the "consumer defines its own Translator
// interface" pattern of rounds 93..223 (most recently
// internal/quality 223, internal/clarification 222,
// internal/approval 221, internal/logging 162, internal/hardware 158,
// internal/discovery 154, internal/deployment 153, internal/database
// 152, internal/context 151, internal/config 150, internal/commands
// 149, internal/cognee 148, internal/agent 147, internal/auth 146).
//
// Wire-in path at boot: consuming binary builds an *i18n.Localizer
// (loaded with active.en.yaml from internal/render/i18n/bundles),
// wraps it in *i18nadapter.Translator, stores via
// render.SetTranslator. The package-level tr() helper falls back
// to NoopTranslator{} when not wired — loud message-ID echo, never
// silent swallow (which would be a §11.4 PASS-bluff at the i18n
// layer).
//
// Round-224 status: NO-OP INFRA — the audit gate
// (scripts/audit-const046-hardcoded-content.sh) reports zero
// hardcoded-content violations in internal/render/ at HEAD. The
// package is a low-level terminal-rendering library; its surface is
// entirely technical: RenderMode constants ("fancy"/"plain"/"auto"
// — env-var values consumed by os.LookupEnv parsing, NOT user
// narrative), env-var key (HELIXCODE_RENDER — config token), BlockID
// tokens ("oneshot-<n>", "_auto", "lsp-diagnostics",
// "smart-edit-diff" — programmatic diff anchors consumed by
// successive RenderFrame calls), ANSI escape sequences
// (\x1b[...m / CSI fragments — terminal control bytes, not text),
// and sentinel errors ("invalid render mode", "renderer is closed",
// "frame block ID required" — wrapped via errors.New + compared via
// errors.Is by callers, not surfaced as user-facing labels). Per
// round-158's hardware identifier-token precedent, tokens consumed
// by downstream parsers / equality checks are explicitly NOT in
// scope for CONST-046 migration — translating them would silently
// rewrite programmatic contracts.
//
// The scaffold lands now so any FUTURE user-facing string added to
// internal/render/ (e.g. a renderer-mode help banner shown when an
// invalid HELIXCODE_RENDER value is detected, an error-toast
// message surfaced via the renderer rather than via the underlying
// errors.Is chain, a CLI "no terminal detected" notice) inherits
// the standard migration pattern without further infra work.
package i18n

import "context"

// Translator is the contract internal/render uses for every
// CONST-046-migrated user-facing string.
type Translator interface {
	// T resolves messageID against the active locale. templateData
	// supplies named placeholders for go-i18n style interpolation;
	// pass nil when the message has no placeholders.
	T(ctx context.Context, messageID string, templateData map[string]any) (string, error)

	// TPlural resolves messageID with plural-form selection driven
	// by count. templateData carries any non-count placeholders.
	TPlural(ctx context.Context, messageID string, count int, templateData map[string]any) (string, error)
}

// NoopTranslator returns the messageID verbatim. SAFETY default for
// unit tests within this package + backward-compat for callers who
// have not yet wired a real Translator. Production paths MUST inject
// a real Translator (helix_code wires *i18nadapter.Translator at
// boot).
type NoopTranslator struct{}

// T returns id unchanged (loud echo). Never returns an error.
func (NoopTranslator) T(_ context.Context, id string, _ map[string]any) (string, error) {
	return id, nil
}

// TPlural returns id unchanged (loud echo). Never returns an error.
func (NoopTranslator) TPlural(_ context.Context, id string, _ int, _ map[string]any) (string, error) {
	return id, nil
}
