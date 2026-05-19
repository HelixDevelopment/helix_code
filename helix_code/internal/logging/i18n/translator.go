// Package i18n declares internal/logging's hardcoded-content
// abstraction per CONST-046 (round-162 §11.4 anti-bluff sweep,
// 2026-05-19). Mirrors the "consumer defines its own Translator
// interface" pattern of rounds 93..158 (most recently
// internal/hardware 158, internal/discovery 154, internal/deployment
// 153, internal/database 152, internal/context 151, internal/config
// 150, internal/commands 149, internal/cognee 148, internal/agent 147,
// internal/auth 146).
//
// Wire-in path at boot: consuming binary builds an *i18n.Localizer
// (loaded with active.en.yaml from internal/logging/i18n/bundles),
// wraps it in *i18nadapter.Translator, stores via
// logging.SetTranslator. The package-level tr() helper falls back
// to NoopTranslator{} when not wired — loud message-ID echo, never
// silent swallow (which would be a §11.4 PASS-bluff at the i18n
// layer).
//
// Round-162 status: NO-OP INFRA — the audit gate
// (scripts/audit-const046-hardcoded-content.sh) reports zero
// hardcoded-content violations in internal/logging/ at HEAD. The
// package's user-visible literals are stable structural log tokens
// ("DEBUG", "INFO", "WARN", "ERROR", "FATAL", "UNKNOWN" — log-level
// identifiers consumed by log parsers and filters, not display
// content) plus go-style log format strings ("[%s] %s") that carry
// no human-readable narrative. Per round-158's hardware
// model-size-identifier precedent, identity tokens consumed by
// downstream parsers / filters / equality checks are explicitly NOT
// in scope for CONST-046 migration — translating them would silently
// rewrite identity keys used by log-aggregation pipelines.
//
// The scaffold lands now so any FUTURE user-facing string added to
// internal/logging/ (e.g. a wizard prompt, an error message destined
// for end-user UI, a help text) inherits the standard migration
// pattern without further infra work.
package i18n

import "context"

// Translator is the contract internal/logging uses for every
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
