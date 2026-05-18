// Package i18n declares the android application's hardcoded-content
// abstraction per CONST-046 (round-139 §11.4 anti-bluff sweep,
// 2026-05-18).
//
// IMPORTANT — language exemption: the android application's
// user-visible code is native Kotlin/Java (see
// app/src/main/java/dev/helix/code/*.kt + app/src/main/res/layout/
// *.xml). Per CONST-052 §11.4.29 the language-mandated naming +
// resource conventions of the Android platform are honoured INSIDE
// the language root — concretely, all Kotlin/Java strings should
// migrate to Android's own `res/values/strings.xml` /
// `res/values-<locale>/strings.xml` resource system, not to a Go
// i18n bundle. Android's resource system is the platform-correct
// translation surface for native Android code; injecting a Go
// translator into Kotlin would itself be a CONST-051(B) decoupling
// violation (project-specific context bleeding into a platform
// surface).
//
// This Go-side package is therefore the INFRASTRUCTURE for the
// future Go-bridge consumer surface. The android app's MobileCore.kt
// already binds to a generated `HelixCoreMobileCore` symbol
// (gomobile / equivalent FFI), and that bridge's Go-side
// implementation lives in the helix_code module. Strings produced by
// the Go bridge (error messages, dashboard JSON labels, theme
// metadata) MUST be migrated to this i18n bundle so that:
//
//  1. Bridge-produced text adapts to the user's locale via the same
//     Translator seam every other consumer of pkg/i18nadapter uses
//     (uniform pattern across rounds 93/94/95/96/108/131/134/136/
//     137/138/139).
//  2. The Translator interface gives a stubbable seam for the
//     bridge's unit tests without dragging in pkg/i18n's
//     bundle-loading.
//  3. Future extraction of the android Go-bridge into its own
//     submodule would not require restructuring the i18n surface.
//
// Wire-in path at boot: helix_code constructs an *i18n.Localizer
// (loaded with active.en.yaml bundle from ./bundles), wraps it in
// *i18nadapter.Translator, and passes it to the android Go bridge
// via SetTranslator (structural match — no shared package required).
package i18n

import "context"

// Translator is the contract the android Go bridge uses for every
// CONST-046-migrated user-facing string. Constructors accept a
// Translator and fall back to NoopTranslator{} when nil (loud
// message-ID echo — never silently swallow translation requests,
// which would be a §11.4 PASS-bluff at the i18n layer).
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
