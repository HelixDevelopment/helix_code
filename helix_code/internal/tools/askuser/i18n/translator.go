// Package i18n declares internal/tools/askuser's hardcoded-content
// abstraction per CONST-046 (round-440 §11.4 anti-bluff sweep,
// 2026-05-20). Mirrors the "consumer defines its own Translator
// interface" pattern of rounds 93..436 (most recently
// internal/render 224, internal/quality 223, internal/clarification
// 222, internal/approval 221, internal/logging 162, internal/hardware
// 158, internal/discovery 154, internal/deployment 153,
// internal/database 152, internal/context 151, internal/config 150,
// internal/commands 149/436).
//
// Wire-in path at boot: the consuming binary builds an
// *i18n.Localizer (loaded with active.en.yaml from
// internal/tools/askuser/i18n/bundles), wraps it in an
// *i18nadapter.Translator, and stores it via askuser.SetTranslator.
// The package-level tr() helper falls back to NoopTranslator{} when
// not wired — loud message-ID echo, never silent swallow (which
// would be a §11.4 PASS-bluff at the i18n layer).
//
// SCOPE (round-440): internal/tools/askuser/stdin_prompter.go emits
// genuine (C) user-facing CLI prompt narrative — the numbered-choice
// menu footer ("Enter choice [1-N, default X]:"), the choice
// preview-line label prefix, and the invalid-input retry hint
// ("Please enter a number 1-N."). These strings are written
// directly to the operator's terminal via the F18 renderer and are
// therefore IN scope for CONST-046 migration. The package's tool
// Schema() parameter descriptions in ask_user_tool.go are LLM-facing
// (A) and the errors.New sentinels in types.go are compared via
// errors.Is (technical-token precedent, round-158) — both OUT of
// scope and intentionally NOT migrated here.
package i18n

import "context"

// Translator is the contract internal/tools/askuser uses for every
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
