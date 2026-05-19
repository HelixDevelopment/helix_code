// Package i18n declares internal/verifier's hardcoded-content
// abstraction per CONST-046 (round-182 §11.4 anti-bluff sweep,
// 2026-05-19). Mirrors the "consumer defines its own Translator
// interface" pattern of rounds 93..162.
//
// Round-182 status: NO-OP INFRA — the audit gate
// (scripts/audit-const046-hardcoded-content.sh) reports only two
// findings in internal/verifier/ at HEAD, both for the model
// DisplayName/Name string "Grok-3 Fast Beta" in fallback_models.go.
// These are model IDENTITY tokens consumed downstream as equality
// keys by provider routing, the LLMsVerifier scoring pipeline,
// CONST-036/037 single-source-of-truth lookup, and end-user UI
// (where the model name is itself the user-recognised brand —
// "Grok-3 Fast Beta" is xAI's product name, not a translatable
// descriptive phrase). Per round-158's internal/hardware
// model-size-identifier precedent and round-162's internal/logging
// log-level-identifier precedent, identity tokens are explicitly
// NOT in scope for CONST-046 migration — translating them would
// silently rewrite identity keys used by model-routing pipelines
// and break the §CONST-037 promise that LLMsVerifier is the single
// source of truth for model metadata.
//
// The scaffold lands now so any FUTURE user-facing string added to
// internal/verifier/ inherits the standard migration pattern without
// further infra work.
package i18n

import "context"

// Translator is the contract internal/verifier uses for every
// CONST-046-migrated user-facing string.
type Translator interface {
	T(ctx context.Context, messageID string, templateData map[string]any) (string, error)
	TPlural(ctx context.Context, messageID string, count int, templateData map[string]any) (string, error)
}

// NoopTranslator returns the messageID verbatim. SAFETY default for
// unit tests + backward-compat for callers who have not yet wired a
// real Translator. Production paths MUST inject a real Translator
// (helix_code wires *i18nadapter.Translator at boot).
type NoopTranslator struct{}

// T returns id unchanged (loud echo). Never returns an error.
func (NoopTranslator) T(_ context.Context, id string, _ map[string]any) (string, error) {
	return id, nil
}

// TPlural returns id unchanged (loud echo). Never returns an error.
func (NoopTranslator) TPlural(_ context.Context, id string, _ int, _ map[string]any) (string, error) {
	return id, nil
}
