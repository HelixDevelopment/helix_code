// Package i18n declares internal/tools's hardcoded-content
// abstraction per CONST-046 (round-181 §11.4 anti-bluff sweep,
// 2026-05-19). Mirrors the "consumer defines its own Translator
// interface" pattern of rounds 93..178 (most recently
// internal/session 178). Wire-in path at boot: consuming binary
// builds an *i18n.Localizer (loaded with active.en.yaml from
// internal/tools/i18n/bundles), wraps it in
// *i18nadapter.Translator, stores via tools.SetTranslator. The
// package-level tr() helper falls back to NoopTranslator{} when not
// wired — loud message-ID echo, never silent swallow (which would
// be a §11.4 PASS-bluff at the i18n layer).
package i18n

import "context"

// Translator is the contract internal/tools uses for every
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
