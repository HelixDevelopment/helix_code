// Package i18n declares the iOS application's hardcoded-content
// abstraction per CONST-046 (round-138 §11.4 anti-bluff sweep,
// 2026-05-18).
//
// SCOPE NOTE — iOS is a gomobile-bridged Swift application:
//
//	helix_code/applications/ios/HelixCode/{AppDelegate,MobileCore,
//	ViewController}.swift consume the Go core through the
//	HelixCoreMobileCore class produced by `gomobile bind`. There is
//	currently NO Go source inside the iOS application directory
//	itself — every Swift-side literal lives in the platform-native
//	Localizable.strings / .stringsdict bundles that ship with the
//	iOS app target (Apple-platform convention preserved per
//	CONST-052 "language-mandated case / convention" exception).
//
// This package therefore provides FORWARD-LOOKING Go-side
// translator infrastructure for the moment that:
//
//   - Notification text emitted from Go (via MobileCore.sendNotification)
//     needs to bubble across the gomobile bridge as a localized string
//     resolved against the user's iOS locale.
//   - Error messages surfaced by the Go core (MobileCore.connect,
//     MobileCore.getDashboardData, etc.) need to be localizable
//     instead of raw English literals.
//   - Server-pushed copy (theme labels, task status strings, dashboard
//     headlines) needs to round-trip through go-i18n on the Go side
//     before crossing the bridge.
//
// The Translator contract here mirrors the pattern established by
// rounds 93/94/95 (Lazy / SelfImprove / HelixLLM) and the platform-
// sibling rounds 96 (harmony_os), 136 (desktop), 137 (terminal-ui).
// Uniformity across the codebase keeps mental overhead low and
// makes the eventual Go-side bridge consumer integration trivial.
//
// Wire-in plan (when first Go-side user-facing string lands):
//
//  1. Construct an *i18n.Localizer (loaded with the active.en.yaml
//     bundle from ./bundles, plus locale overrides loaded from the
//     iOS app's preferredLocalizations list passed across the
//     bridge).
//  2. Wrap it in *i18nadapter.Translator (dev.helix.code/pkg/i18nadapter).
//  3. Pass it to the iOS bridge constructor as a Translator
//     (structural match — no shared package required).
//
// Until then, the package compiles standalone and is exercised by
// translator_test.go below; any future call site that imports this
// package picks up the NoopTranslator default (loud message-ID echo)
// when no real Translator is injected — which is the §11.4 anti-
// bluff posture: NEVER silently swallow a translation request.
package i18n

import "context"

// Translator is the contract iOS-bridge Go code uses for every
// CONST-046-migrated user-facing string emitted from the Go side.
// Constructors accept a Translator and fall back to NoopTranslator{}
// when nil (loud message-ID echo — never silently swallow translation
// requests, which would be a §11.4 PASS-bluff at the i18n layer).
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
// a real Translator (iOS bridge wires *i18nadapter.Translator at
// boot once Go-side user-facing strings cross the bridge).
type NoopTranslator struct{}

// T returns id unchanged (loud echo). Never returns an error.
func (NoopTranslator) T(_ context.Context, id string, _ map[string]any) (string, error) {
	return id, nil
}

// TPlural returns id unchanged (loud echo). Never returns an error.
func (NoopTranslator) TPlural(_ context.Context, id string, _ int, _ map[string]any) (string, error) {
	return id, nil
}
