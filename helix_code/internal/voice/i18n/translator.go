// Package i18n declares internal/voice's hardcoded-content
// abstraction per CONST-046 (round-226 §11.4 anti-bluff sweep,
// 2026-05-19).
//
// internal/voice is the speech-to-text capture and transcription
// layer (P2-F27 aider voice port). Its user-visible surface is three
// tool descriptions (voice_start / voice_stop / voice_transcribe)
// returned through the tools.ToolSchema.Description() method —
// rendered verbatim in CLI help output, prompt suggestions, and any
// future tool-registry UI. Hardcoded English silently breaks the
// product for non-English operators per CONST-046.
//
// Wire-in path at boot: consuming binary builds an *i18n.Localizer
// (loaded with active.en.yaml from internal/voice/i18n/bundles),
// wraps it in *i18nadapter.Translator, stores via
// voice.SetTranslator. The package-level tr() helper falls back to
// NoopTranslator{} when not wired — loud message-ID echo, never
// silent swallow (which would be a §11.4 PASS-bluff at the i18n
// layer).
//
// Three reasons for the "consumer defines its own Translator"
// pattern instead of importing a shared types package: (1) uniform
// pattern across the codebase keeps mental overhead low (rounds
// 93/94/95/96/108/131/134/136-225), (2) the Translator interface is
// the natural seam for stubbing in tests without dragging in
// pkg/i18n's bundle-loading dependency, (3) internal/voice could
// plausibly be extracted into its own standalone module for reuse
// without restructuring the i18n surface.
package i18n

import "context"

// Translator is the contract internal/voice uses for every
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
// boot). Loud echo (rather than silent empty) preserves
// debuggability when wiring is missing — operators see the raw
// message ID in tool-description output and know exactly which
// lookup failed.
type NoopTranslator struct{}

// T returns id unchanged (loud echo). Never returns an error.
func (NoopTranslator) T(_ context.Context, id string, _ map[string]any) (string, error) {
	return id, nil
}

// TPlural returns id unchanged (loud echo). Never returns an error.
func (NoopTranslator) TPlural(_ context.Context, id string, _ int, _ map[string]any) (string, error) {
	return id, nil
}
