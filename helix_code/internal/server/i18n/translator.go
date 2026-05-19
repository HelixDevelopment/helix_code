// Package i18n declares the HelixCode internal/server package's
// hardcoded-content abstraction per CONST-046 (round-177 §11.4
// anti-bluff sweep, 2026-05-19, CONST-046 Phase 4 round 70).
//
// internal/server is the HTTP handler layer of dev.helix.code,
// distinct from cmd/server (boot-time wiring, migrated in round 134).
// It COULD import dev.helix.code/pkg/i18nadapter directly. We
// nonetheless follow the "consumer defines its own Translator
// interface" pattern established by rounds 93/94/95/96/108/131/134
// (Lazy / SelfImprove / HelixLLM / harmony_os / helix_config /
// cli / cmd_server) for three reasons: (1) uniform pattern across
// the codebase keeps mental overhead low, (2) the Translator
// interface is the natural seam for stubbing in tests without
// dragging in pkg/i18n's bundle-loading, (3) future extraction of
// internal/server into its own submodule would not require
// restructuring the i18n surface.
//
// The wire-in path at boot is: cmd/server constructs an
// *i18n.Localizer (loaded with the active.en.yaml bundle from
// internal/server/i18n/bundles), wraps it in
// *i18nadapter.Translator, and stores it via SetTranslator. Handlers
// reach the translator via the package-level tr() helper in
// helix_code/internal/server/i18n_seam.go because every handler is a
// method on *Server but the user-facing string set is global to the
// process — global injection matches Gin's own use of package-level
// state and keeps the migration minimally invasive.
//
// Message-ID naming convention: prefix every ID with
// `internal_server_` to avoid collision with other submodules'
// bundles when loaded into the same go-i18n.Bundle (e.g. cmd/server's
// own `server_*` IDs).
package i18n

import "context"

// Translator is the contract internal/server uses for every
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
