// Package i18n declares performance_optimization's hardcoded-content
// abstraction per CONST-046 (round-144 §11.4 anti-bluff sweep,
// 2026-05-18).
//
// performance_optimization is a tiny CLI living inside the
// dev.helix.code module (NOT an externally-consumable submodule), so
// it COULD import dev.helix.code/pkg/i18nadapter directly. We
// nonetheless follow the "consumer defines its own Translator
// interface" pattern established by rounds 93/94/95/96/108/131/134/
// 136-141/142/143 (Lazy / SelfImprove / HelixLLM / harmony_os /
// helix_config / helixllm / cli / server / desktop / terminal_ui /
// ios / android / aurora_os / config_test / security_test /
// security_fix) for three reasons: (1) uniform pattern across the
// codebase keeps mental overhead low, (2) the Translator interface is
// the natural seam for stubbing in tests without dragging in
// pkg/i18n's bundle-loading, (3) future extraction of
// performance_optimization into its own submodule would not require
// restructuring the i18n surface.
//
// The wire-in path at boot is: performance_optimization constructs an
// *i18n.Localizer (loaded with the active.en.yaml bundle from
// ./bundles), wraps it in *i18nadapter.Translator, and stores it via
// SetTranslator. The package-level tr() helper resolves message IDs
// through this interface and falls back to NoopTranslator{} when no
// real translator has been wired — loud message-ID echo rather than
// silent swallow (a silent swallow would be a §11.4 PASS-bluff at
// the i18n layer).
package i18n

import "context"

// Translator is the contract performance_optimization uses for every
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
