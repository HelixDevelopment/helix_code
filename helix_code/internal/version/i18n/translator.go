// Package i18n declares the HelixCode internal/version package's
// hardcoded-content abstraction per CONST-046 (round-183 §11.4
// anti-bluff sweep, 2026-05-19, CONST-046 Phase 4 round 76).
//
// internal/version exposes build metadata (Version, GitCommit,
// BuildDate, GoVersion) to every CLI surface, HTTP health endpoint,
// and admin/log line in the HelixCode binary. Its formatted output
// strings (GetVersion, GetFullVersion) reach the end user verbatim
// through `helixcode version`, `--version`, JSON health responses,
// and any log line that prints the build banner. Every user-facing
// composition is therefore a CONST-046 candidate.
//
// We follow the "consumer defines its own Translator interface"
// pattern established by rounds 93/94/95/96/108/131/134/136-177
// (Lazy / SelfImprove / HelixLLM / harmony_os / helix_config /
// helixllm / cli / cmd_server / desktop / terminal_ui / ios /
// android / aurora_os / config_test / security_test / security_fix /
// performance_optimization / security_fix_standalone / auth /
// internal/server / internal/security / ...) for three reasons:
// (1) uniform pattern across the codebase keeps mental overhead low,
// (2) the Translator interface is the natural seam for stubbing in
// tests without dragging in pkg/i18n's bundle-loading machinery,
// (3) internal/version is leaf-level (zero dev.helix.code internal
// imports beyond runtime) — independence from pkg/i18nadapter
// preserves the no-cycle guarantee future refactors depend on.
//
// The wire-in path at boot is: the consuming binary (cmd/server,
// cmd/cli, applications/desktop, etc.) constructs an *i18n.Localizer
// (loaded with the active.en.yaml bundle from
// internal/version/i18n/bundles), wraps it in *i18nadapter.Translator,
// and stores it via version.SetTranslator. The package-level tr()
// helper in helix_code/internal/version/i18n_seam.go resolves message
// IDs through this interface and falls back to NoopTranslator{} when
// no real translator has been wired — loud message-ID echo rather
// than silent swallow (a silent swallow would be a §11.4 PASS-bluff
// at the i18n layer).
//
// Message-ID naming convention: prefix every ID with
// `internal_version_` to avoid collision with other submodules'
// bundles when loaded into the same go-i18n.Bundle (e.g. cmd/server
// `server_*` IDs and internal/server `internal_server_*` IDs).
package i18n

import "context"

// Translator is the contract internal/version uses for every
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
