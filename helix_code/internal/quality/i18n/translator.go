// Package i18n declares internal/quality's hardcoded-content
// abstraction per CONST-046 (round-223 §11.4 anti-bluff sweep,
// 2026-05-19). Mirrors the "consumer defines its own Translator
// interface" pattern of rounds 93..222 (most recently
// internal/logging 162, internal/hardware 158, internal/discovery
// 154, internal/deployment 153, internal/database 152,
// internal/context 151, internal/config 150, internal/commands 149,
// internal/cognee 148, internal/agent 147, internal/auth 146).
//
// Wire-in path at boot: consuming binary builds an *i18n.Localizer
// (loaded with active.en.yaml from internal/quality/i18n/bundles),
// wraps it in *i18nadapter.Translator, stores via
// quality.SetTranslator. The package-level tr() helper falls back
// to NoopTranslator{} when not wired — loud message-ID echo, never
// silent swallow (which would be a §11.4 PASS-bluff at the i18n
// layer).
//
// Round-223 status: NO-OP INFRA — the audit gate
// (scripts/audit-const046-hardcoded-content.sh) reports zero
// hardcoded-content violations in internal/quality/ at HEAD. The
// package's surface is entirely structural: numeric thresholds
// (MinScore, LintScore floor 80.0, overall score 70.0/90.0),
// boolean gate flags (RequireBuild, RequireTests, RequireLint),
// struct-tag keys (json/yaml metadata consumed by serializers),
// internal Details map keys ("build_error", "test_error",
// "test_output") used for programmatic lookup by consumers, and
// wrapped-error technical strings ("create temp dir: %w", "write
// output: %w") that surface to logs not end-users. Per round-158's
// hardware identifier-token precedent, identity keys consumed by
// downstream parsers / equality checks are explicitly NOT in scope
// for CONST-046 migration — translating them would silently rewrite
// programmatic contracts.
//
// The scaffold lands now so any FUTURE user-facing string added to
// internal/quality/ (e.g. a gate-failure explanation surfaced to a
// developer UI, a quality-history report banner, a CLI summary
// line) inherits the standard migration pattern without further
// infra work.
package i18n

import "context"

// Translator is the contract internal/quality uses for every
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
