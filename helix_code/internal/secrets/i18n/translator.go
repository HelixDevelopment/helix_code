// Package i18n declares internal/secrets' hardcoded-content
// abstraction per CONST-046 (round-225 §11.4 anti-bluff sweep,
// 2026-05-19).
//
// internal/secrets is the API-key loader (P1.5-WP4). Its surface is
// intentionally tiny — LoadAPIKeys reads $HOME/api_keys.sh first, or
// falls back to a walked-up .env file — but the one user-facing
// literal it does emit (the "no api_keys.sh or .env found" error
// message returned when neither source is present) reaches users
// directly through the boot path of every server, CLI, and
// application binary that calls LoadAPIKeys at startup. That makes
// it a CONST-046 candidate: hardcoded English silently breaks the
// product for non-English operators who see "no api_keys.sh or
// .env found" in their boot log.
//
// CONST-042 anti-leak guarantee: the seam introduced here is
// strictly for error / message-text translation. It NEVER touches
// secret values themselves. loader.go does not log or emit any
// loaded variable's value (CONST-042 §12.1), and this i18n surface
// preserves that posture — no message ID, no bundle entry, and no
// templateData field in this package ever carries a secret. The
// pattern is identical to the auth / mcp / config_test
// CONST-046 seams (rounds 146/188/etc.) where the consumer defines
// its own Translator interface, defaults to NoopTranslator{} (loud
// echo of the message ID), and exposes SetTranslator for the
// consuming binary to wire a real *i18nadapter.Translator at boot.
//
// Three reasons for the "consumer defines its own Translator"
// pattern instead of importing a shared types package: (1) uniform
// pattern across the codebase keeps mental overhead low, (2) the
// Translator interface is the natural seam for stubbing in tests
// without dragging in pkg/i18n's bundle-loading dependency, (3)
// internal/secrets could plausibly be extracted into its own
// standalone module for reuse without restructuring the i18n
// surface.
//
// The wire-in path at boot is: the consuming binary constructs an
// *i18n.Localizer (loaded with the active.en.yaml bundle from
// internal/secrets/i18n/bundles), wraps it in
// *i18nadapter.Translator, and stores it via secrets.SetTranslator.
// The package-level tr() helper resolves message IDs through this
// interface and falls back to NoopTranslator{} when no real
// translator has been wired — loud message-ID echo rather than
// silent swallow (a silent swallow would be a §11.4 PASS-bluff at
// the i18n layer).
package i18n

import "context"

// Translator is the contract internal/secrets uses for every
// CONST-046-migrated user-facing string.
type Translator interface {
	// T resolves messageID against the active locale. templateData
	// supplies named placeholders for go-i18n style interpolation;
	// pass nil when the message has no placeholders. CONST-042
	// guarantee: templateData MUST NEVER carry secret material in
	// the internal/secrets package — values are not logged or
	// surfaced through error paths.
	T(ctx context.Context, messageID string, templateData map[string]any) (string, error)

	// TPlural resolves messageID with plural-form selection driven
	// by count. templateData carries any non-count placeholders.
	// Same CONST-042 secret-handling guarantee as T.
	TPlural(ctx context.Context, messageID string, count int, templateData map[string]any) (string, error)
}

// NoopTranslator returns the messageID verbatim. SAFETY default for
// unit tests within this package + backward-compat for callers who
// have not yet wired a real Translator. Production paths MUST inject
// a real Translator (helix_code wires *i18nadapter.Translator at
// boot). Loud echo (rather than silent empty) preserves
// debuggability when wiring is missing — operators see the raw
// message ID in error output and know exactly which lookup failed.
type NoopTranslator struct{}

// T returns id unchanged (loud echo). Never returns an error.
func (NoopTranslator) T(_ context.Context, id string, _ map[string]any) (string, error) {
	return id, nil
}

// TPlural returns id unchanged (loud echo). Never returns an error.
func (NoopTranslator) TPlural(_ context.Context, id string, _ int, _ map[string]any) (string, error) {
	return id, nil
}
