// Package i18n declares the examples/ tree's hardcoded-content
// abstraction per CONST-046 (round-347 §11.4 anti-bluff sweep,
// 2026-05-20).
//
// The example programs under helix_code/examples/ are separate
// `package main` binaries (one per directory) sharing the
// dev.helix.code module. Rather than each example carrying its own
// i18n seam, they import this single shared package — the developer-
// facing console narration they emit is identical in shape across
// every demo, so a shared bundle keeps the migration uniform and the
// mental overhead low.
//
// We follow the "consumer defines its own Translator interface"
// pattern established by every prior CONST-046-migrated package in
// this codebase (most recently the cmd/* rounds). The Translator
// interface is the natural seam for stubbing in tests without
// dragging in pkg/i18n's bundle-loading machinery.
//
// The wire-in path at boot is: a consuming binary constructs an
// *i18n.Localizer (loaded with active.en.yaml from ./bundles), wraps
// it in *i18nadapter.Translator, and stores it via SetTranslator. The
// package-level Tr() helper resolves message IDs through this
// interface and falls back to NoopTranslator{} when no real
// translator has been wired — loud message-ID echo rather than silent
// swallow (a silent swallow would be a §11.4 PASS-bluff at the i18n
// layer). The example mains run with NoopTranslator{} by default,
// which echoes the message ID verbatim — honest and obvious for a
// developer running `go run main.go`.
package i18n

import "context"

// Translator is the contract the examples/ tree uses for every
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

// translator resolves CONST-046 message IDs for every user-facing
// string emitted by the example programs. Defaults to
// NoopTranslator{} (loud message-ID echo) so test runs and ad-hoc
// `go run` invocations remain obvious.
var translator Translator = NoopTranslator{}

// SetTranslator wires a CONST-046-compliant Translator. Passing nil
// resets to NoopTranslator{} (loud echo) — never silently disables
// translation lookup (which would be a §11.4 PASS-bluff at the i18n
// injection layer).
func SetTranslator(tr Translator) {
	if tr == nil {
		translator = NoopTranslator{}
		return
	}
	translator = tr
}

// Tr is the shared CONST-046 resolver used by every user-facing
// string emission in the examples/ tree. It NEVER returns an error
// to the caller — translation failures degrade to the message ID
// itself (matching NoopTranslator behaviour) so example output
// remains loud + obvious instead of silently empty.
func Tr(ctx context.Context, msgID string, data map[string]any) string {
	if translator == nil {
		translator = NoopTranslator{}
	}
	out, err := translator.T(ctx, msgID, data)
	if err != nil || out == "" {
		return msgID
	}
	return out
}
