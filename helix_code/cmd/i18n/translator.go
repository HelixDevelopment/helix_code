// Package i18n declares the helix_code/cmd package's hardcoded-content
// abstraction per CONST-046 (round-315 §11.4 anti-bluff sweep,
// 2026-05-20, CONST-046 Phase 4).
//
// helix_code/cmd hosts the cobra command tree (root.go,
// main_commands.go, other_commands.go, local_llm.go,
// local_llm_advanced.go). Its user-facing strings reach the end user
// verbatim through every `helix <subcommand>` invocation. Every
// formatted output line, error message, and cobra help string is a
// CONST-046 candidate.
//
// We follow the "consumer defines its own Translator interface"
// pattern established by rounds 93/94/95/96/108/131/134/136-314 for
// three reasons: (1) uniform pattern across the codebase keeps mental
// overhead low, (2) the Translator interface is the natural seam for
// stubbing in tests without dragging in pkg/i18n's bundle-loading
// machinery, (3) the cmd package is leaf-level for the cobra tree —
// independence from pkg/i18nadapter preserves the no-cycle guarantee.
//
// The wire-in path at boot is: the consuming binary constructs an
// *i18n.Localizer (loaded with the active.en.yaml bundle from
// helix_code/cmd/i18n/bundles), wraps it in *i18nadapter.Translator,
// and stores it via cmd.SetTranslator. The package-level tr() helper
// in helix_code/cmd/i18n_seam.go resolves message IDs through this
// interface and falls back to NoopTranslator{} when no real
// translator has been wired — loud message-ID echo rather than silent
// swallow (a silent swallow would be a §11.4 PASS-bluff at the i18n
// layer).
//
// Message-ID naming convention: prefix every ID with `cmd_` to avoid
// collision with other submodules' bundles when loaded into the same
// go-i18n.Bundle.
package i18n

import "context"

// Translator is the contract helix_code/cmd uses for every
// CONST-046-migrated user-facing string. The package-level tr()
// helper resolves IDs through this interface and falls back to
// NoopTranslator{} when no real translator has been wired — loud
// message-ID echo rather than silent swallow (a silent swallow would
// be a §11.4 PASS-bluff at the i18n layer).
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
