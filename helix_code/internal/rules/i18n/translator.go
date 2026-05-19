// Package i18n declares internal/rules's hardcoded-content
// abstraction per CONST-046 (round-175 §11.4 anti-bluff sweep,
// 2026-05-19). Mirrors the "consumer defines its own Translator
// interface" pattern of rounds 93..174 (most recently
// internal/commands 149, internal/agent 147, internal/cognee 148,
// internal/auth 146). Wire-in path at boot: the consuming binary
// builds an *i18n.Localizer (loaded with active.en.yaml from
// internal/rules/i18n/bundles), wraps it in *i18nadapter.Translator,
// stores via rules.SetTranslator. The package-level tr() helper
// falls back to NoopTranslator{} when not wired — loud message-ID
// echo, never silent swallow (which would be a §11.4 PASS-bluff at
// the i18n layer).
//
// internal/rules hosts the workspace/project/file-scoped rule
// management surface emitting user-facing validation errors
// (rule.go's Validate), parser errors (parser.go), load-failure
// warnings (manager.go), and prompt-builder text (manager.go's
// FormatRulesForFile). Every literal user-facing string is therefore
// a CONST-046 candidate; round 175 migrates 10 most-visible ones.
package i18n

import "context"

// Translator is the contract internal/rules uses for every
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
