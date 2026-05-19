// Package i18n declares internal/clarification's hardcoded-content
// abstraction per CONST-046 (round-222 §11.4 anti-bluff sweep,
// 2026-05-19).
//
// internal/clarification hosts the ambiguity-detection Engine and
// QuestionGenerator that emit user-facing prompt fragments + resolved
// clarification summaries that surface to end users via CLI output,
// HTTP API responses, and desktop/terminal UI. Three CONST-046
// candidates were detected by scripts/audit-const046-hardcoded-content.sh
// at the start of round-222: the LLM system-prompt template (instructs
// the model how to generate clarification questions — must be locale-
// aware so non-English operators get questions in their own language),
// the per-turn "User request: %s" wrapper sent to the model, and the
// "Clarifications received:\n" header prefixed to the resolved-context
// string returned by Engine.Resolve.
//
// We follow the "consumer defines its own Translator interface"
// pattern established by every prior CONST-046 round (rounds
// 93/94/95/96/108/131/134/136-221) for three reasons: (1) uniform
// pattern across the codebase keeps mental overhead low, (2) the
// Translator interface is the natural seam for stubbing in tests
// without dragging in pkg/i18n's bundle-loading, (3)
// internal/clarification could plausibly be lifted into its own
// submodule for reuse by other Helix agents without restructuring
// the i18n surface.
//
// The wire-in path at boot is: the consuming binary constructs an
// *i18n.Localizer (loaded with the active.en.yaml bundle from
// internal/clarification/i18n/bundles), wraps it in
// *i18nadapter.Translator, and stores it via clarification.SetTranslator.
// The package-level tr() helper resolves message IDs through this
// interface and falls back to NoopTranslator{} when no real translator
// has been wired — loud message-ID echo rather than silent swallow
// (a silent swallow would be a §11.4 PASS-bluff at the i18n layer).
package i18n

import "context"

// Translator is the contract internal/clarification uses for every
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
