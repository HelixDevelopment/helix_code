// Package i18n declares internal/provider's hardcoded-content
// abstraction per CONST-046 (round-171 §11.4 anti-bluff sweep,
// 2026-05-19).
//
// internal/provider is the unified LLM Provider interface +
// ProviderType enumeration package. As of round-171 the package
// contains zero user-facing hardcoded strings — provider.go is
// pure interface + protocol-identifier constants (the
// ProviderType literals like "openai", "anthropic" are
// wire-protocol identifiers, NOT user-facing per CONST-046 §3),
// doc.go is Go doc-comment reference material (compile-time
// godoc, not runtime user output), and provider_test.go is
// developer-diagnostic t.Errorf strings (test-only, exempt).
//
// This translator + bundle infrastructure is provisioned now so
// any future user-facing literals introduced into this package
// (error messages, validation feedback, health-check status
// labels surfaced to operators) land in established
// CONST-046-compliant structure without needing scaffolding
// retrofit. The pattern mirrors rounds 93/94/95/96/108/131/134
// /136-146/161 (Lazy / SelfImprove / HelixLLM / harmony_os /
// helix_config / helixllm / cli / server / desktop / terminal_ui
// / ios / android / aurora_os / config_test / security_test /
// security_fix / performance_optimization /
// security_fix_standalone / auth / llm).
//
// The wire-in path at boot is: the consuming binary constructs
// an *i18n.Localizer (loaded with the active.en.yaml bundle from
// internal/provider/i18n/bundles), wraps it in
// *i18nadapter.Translator, and stores it via
// provider.SetTranslator. The package-level tr() helper resolves
// message IDs through this interface and falls back to
// NoopTranslator{} when no real translator has been wired —
// loud message-ID echo rather than silent swallow (a silent
// swallow would be a §11.4 PASS-bluff at the i18n layer).
package i18n

import "context"

// Translator is the contract internal/provider uses for every
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

// NoopTranslator returns the messageID verbatim. SAFETY default
// for unit tests within this package + backward-compat for
// callers who have not yet wired a real Translator. Production
// paths MUST inject a real Translator (helix_code wires
// *i18nadapter.Translator at boot).
type NoopTranslator struct{}

// T returns id unchanged (loud echo). Never returns an error.
func (NoopTranslator) T(_ context.Context, id string, _ map[string]any) (string, error) {
	return id, nil
}

// TPlural returns id unchanged (loud echo). Never returns an error.
func (NoopTranslator) TPlural(_ context.Context, id string, _ int, _ map[string]any) (string, error) {
	return id, nil
}
