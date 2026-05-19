// Package i18n declares internal/plugins' hardcoded-content
// abstraction per CONST-046 (round-234 §11.4 anti-bluff sweep,
// 2026-05-19).
//
// internal/plugins is the plugin loader/registry/sandbox executor
// (manifest parsing, dependency wiring, exec.Command launch). Most
// of its surface is structural — Go struct fields, YAML manifest
// keys, exec argv tokens, dependency identifier strings, sandbox
// directory paths — but the package emits one operator-visible
// error message through exec.go's plugin sandbox path: the "plugin
// sandbox: <name> entrypoint not found at <path>" warning surfaced
// when ExecutePlugin cannot find the requested plugin entrypoint.
// Hardcoded English silently breaks the product for non-English
// operators who see this message in their plugin-launch logs.
//
// Mirrors rounds 221 (approval) / 222 (clarification) / 223
// (quality NO-OP infra) / 224 (render) / 225 (secrets) / 226
// (voice) / 228 (approvalwire) / 233 (plantree) of the same CONST-046
// Phase-4 sweep. Pattern: consumer-defined Translator interface +
// NoopTranslator default + SetTranslator wiring + package-level tr()
// helper. The wire-in path at boot: the consuming binary constructs
// an *i18n.Localizer (loaded with active.en.yaml from
// internal/plugins/i18n/bundles), wraps it in
// *i18nadapter.Translator, and stores it via plugins.SetTranslator.
// The package-level tr() helper resolves message IDs through this
// interface and falls back to NoopTranslator{} when no real
// translator has been wired — loud message-ID echo rather than
// silent swallow (a silent swallow would be a §11.4 PASS-bluff at
// the i18n layer).
package i18n

import "context"

// Translator is the contract internal/plugins uses for every
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
