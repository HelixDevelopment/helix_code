// Package i18n declares the HelixCode internal/workflow package's
// hardcoded-content abstraction per CONST-046 (round-185 §11.4
// anti-bluff sweep, 2026-05-19, CONST-046 Phase 4 round 78).
//
// internal/workflow is the autonomy + plan-mode + background-runner
// layer of dev.helix.code. User-facing strings here surface to every
// CLI / TUI / desktop / mobile consumer that renders autonomy-mode
// labels, plan-execution progress, or workflow status — and so must
// adapt to the caller's locale per CONST-046.
//
// We follow the "consumer defines its own Translator interface"
// pattern established by rounds 93/94/95/96/108/131/134/177
// (Lazy / SelfImprove / HelixLLM / harmony_os / helix_config /
// cli / cmd_server / internal/server) for three reasons: (1) uniform
// pattern across the codebase keeps mental overhead low, (2) the
// Translator interface is the natural seam for stubbing in tests
// without dragging in pkg/i18n's bundle-loading, (3) future
// extraction of internal/workflow into its own submodule would not
// require restructuring the i18n surface.
//
// Wire-in path at boot: cmd/server (or any embedding host)
// constructs an *i18n.Localizer (loaded with the active.en.yaml
// bundle from internal/workflow/i18n/bundles), wraps it in
// *i18nadapter.Translator, and stores it via the appropriate
// SetTranslator helper. Modes / planmode reach the translator via
// the package-level tr() seams in
// helix_code/internal/workflow/autonomy/i18n_seam.go and
// helix_code/internal/workflow/planmode/i18n_seam.go.
//
// Message-ID naming convention: prefix every ID with
// `internal_workflow_` to avoid collision with other submodules'
// bundles when loaded into the same go-i18n.Bundle.
package i18n

import "context"

// Translator is the contract internal/workflow uses for every
// CONST-046-migrated user-facing string. The package-level tr()
// helpers resolve IDs through this interface and fall back to
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
