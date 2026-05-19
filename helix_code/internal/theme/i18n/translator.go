// Package i18n declares internal/theme's hardcoded-content
// abstraction per CONST-046 (round-238 §11.4 anti-bluff sweep,
// 2026-05-19). Mirrors the "consumer defines its own Translator
// interface" pattern of rounds 93..237 (most recently
// internal/persistence, internal/redis, internal/security, etc.).
//
// Scope note: this is helix_code/internal/theme/ — a pure
// theme/color/styler package. The current public surface emits only
// sentinel errors (ErrInvalidColorDepth, ErrInvalidThemeName,
// ErrThemeNotFound, ErrInvalidYAML) wrapped via fmt.Errorf for
// caller context. Sentinel-error identifiers are technical labels
// (caller-side errors.Is targets), not user-facing prose; per the
// round-223 NO-OP precedent they remain in-place pending a future
// CLI/UI layer that surfaces user-visible theme load/list messages.
// This scaffold therefore lands the Translator + NoopTranslator
// abstraction so a follow-up migration round can adopt it without
// re-doing infra.
//
// Wire-in path at boot (future): consuming binary builds an
// *i18n.Localizer (loaded with active.en.yaml from
// internal/theme/i18n/bundles), wraps it in *i18nadapter.Translator,
// stores via a theme.SetTranslator helper introduced when the first
// real user-facing message ID lands. The package-level tr() helper
// (introduced at first migration) falls back to NoopTranslator{}
// when not wired — loud message-ID echo, never silent swallow
// (which would be a §11.4 PASS-bluff at the i18n layer).
package i18n

import "context"

// Translator is the contract internal/theme uses for every
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
