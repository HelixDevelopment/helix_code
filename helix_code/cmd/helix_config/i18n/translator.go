// Package i18n declares helix_config's hardcoded-content abstraction
// per CONST-046 (round-108 §11.4 anti-bluff sweep, 2026-05-18).
//
// helix_config is a CLI tool living inside the dev.helix.code module
// (NOT an externally-consumable submodule), so it COULD import
// dev.helix.code/pkg/i18nadapter directly. We nonetheless follow the
// "consumer defines its own Translator interface" pattern established
// by rounds 93/94/95/96 (Lazy / SelfImprove / HelixLLM / harmony_os)
// for three reasons: (1) uniform pattern across the codebase keeps
// mental overhead low, (2) the Translator interface is the natural
// seam for stubbing in tests without dragging in pkg/i18n's bundle-
// loading, (3) future extraction of helix_config into its own
// submodule would not require restructuring the i18n surface.
//
// The wire-in path at boot is: helix_config constructs an
// *i18n.Localizer (loaded with the active.en.yaml bundle from
// ./bundles), wraps it in *i18nadapter.Translator, and stores it via
// SetTranslator. Cobra command handlers reach the translator via the
// package-level tr() helper because cobra's RunE signature
// (func(*cobra.Command, []string) error) does not support injection
// of additional parameters without restructuring the command tree.
package i18n

import "context"

// Translator is the contract helix_config uses for every CONST-046-
// migrated user-facing string. The package-level tr() helper resolves
// IDs through this interface and falls back to NoopTranslator{} when
// no real translator has been wired — loud message-ID echo rather
// than silent swallow (a silent swallow would be a §11.4 PASS-bluff
// at the i18n layer).
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
