// Package i18n declares internal/fix's hardcoded-content abstraction
// per CONST-046 (round-187 §11.4 anti-bluff sweep, 2026-05-19,
// CONST-046 Phase 4 round 80).
//
// internal/fix is the security-issue resolution package: it emits
// fmt.Errorf-wrapped errors that surface in API responses + CLI
// output, plus logger.Error/Warn lines that end up in operator logs
// and remediation guidance shown to the developer. Every literal
// error string and every operator-actionable log message is a
// CONST-046 candidate — a hardcoded English "Hardcoded credential
// found in ... - requires manual review" silently breaks for
// non-English operators reading the security-remediation report
// (CONST-046 verbatim mandate: "Hardcoded English strings silently
// break the product for non-English users.").
//
// We follow the consumer-defined Translator interface pattern
// established by rounds 93/94/95/96/108/131/134/136-177 for three
// reasons: (1) uniform pattern across the codebase keeps mental
// overhead low, (2) the Translator interface is the natural seam
// for stubbing in tests without dragging in pkg/i18n's
// bundle-loading, (3) internal/fix could plausibly be extracted into
// its own submodule for reuse by other Helix security tooling
// without restructuring the i18n surface.
//
// The wire-in path at boot is: the consuming binary constructs an
// *i18n.Localizer (loaded with the active.en.yaml bundle from
// internal/fix/i18n/bundles), wraps it in *i18nadapter.Translator,
// and stores it via fix.SetTranslator. The package-level tr() helper
// resolves message IDs through this interface and falls back to
// NoopTranslator{} when no real translator has been wired — loud
// message-ID echo rather than silent swallow (a silent swallow
// would be a §11.4 PASS-bluff at the i18n layer).
package i18n

import "context"

// Translator is the contract internal/fix uses for every
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
