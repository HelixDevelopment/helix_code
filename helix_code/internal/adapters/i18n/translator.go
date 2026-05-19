// Package i18n declares internal/adapters' hardcoded-content
// abstraction per CONST-046 (round-239 §11.4 anti-bluff sweep,
// 2026-05-19). Mirrors the "consumer defines its own Translator
// interface" pattern of rounds 93..226 (most recently
// internal/voice 226, internal/secrets 225, internal/render 224,
// internal/quality 223, internal/clarification 222,
// internal/approval 221, internal/logging 162, internal/hardware
// 158, internal/discovery 154, internal/deployment 153,
// internal/database 152).
//
// Wire-in path at boot: consuming binary builds an *i18n.Localizer
// (loaded with active.en.yaml from internal/adapters/i18n/bundles),
// wraps it in *i18nadapter.Translator, stores via
// adapters.SetTranslator. The package-level tr() helper falls back
// to NoopTranslator{} when not wired — loud message-ID echo, never
// silent swallow (which would be a §11.4 PASS-bluff at the i18n
// layer).
//
// Round-239 status: NO-OP INFRA — the audit gate
// (scripts/audit-const046-hardcoded-content.sh) exits 0 on
// internal/adapters/ at HEAD. The package surface is entirely
// structural:
//
//   - speckit_debate_adapter/adapter.go: 8 hits in
//     formatDebateResponse — debate-transcript format markers
//     (FOR:, AGAINST:, SYNTHESIS:, CONCLUSION:, "## Rounds
//     conducted:", "## Quality score:", "## Metrics:") explicitly
//     engineered to satisfy HelixSpecifier's heuristic scorer per
//     round-70's commit message (see lines 281-309 of adapter.go).
//     Translating these markers would silently rewrite the scoring
//     contract — analogous to round-158's hardware identifier-token
//     precedent and round-223's struct-tag identity-key precedent.
//
//   - speckit_debate_adapter/adapter.go also carries adapter-context
//     string (line 248: "Invoked via HelixCode speckit_debate_adapter
//     (round-70 wiring).") — diagnostic metadata surfaced to the
//     orchestrator's Context field, not end-user UI narrative.
//
//   - containers/adapter.go: zero hits. Only wrapped-error technical
//     strings ("compose up cancelled: %w", "no container runtime
//     detected (docker/podman required): %w", "health check failed
//     for %s: %s") — developer-log surface, never user-facing UI,
//     explicitly out of CONST-046 scope per the established
//     wrapped-error precedent.
//
// Per round-158's hardware identifier-token precedent and round-223's
// struct-tag identity-key precedent, identity / format / scoring-marker
// keys consumed by downstream parsers / heuristic scorers / equality
// checks are explicitly NOT in scope for CONST-046 migration —
// translating them would silently rewrite programmatic contracts.
//
// The scaffold lands now so any FUTURE user-facing string added to
// internal/adapters/ (e.g. an adapter-status banner surfaced to a
// developer dashboard, a CLI summary line describing adapter health,
// a wire-evidence-capture message displayed to end users) inherits the
// standard migration pattern without further infra work.
package i18n

import "context"

// Translator is the contract internal/adapters uses for every
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
