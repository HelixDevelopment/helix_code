// bundle.go — boot-time Translator constructor for internal/render
// (HXC-036 Phase 2, 2026-05-29).
//
// The CONST-046 migration created this package's Translator interface +
// NoopTranslator{} loud-echo default + bundles/active.en.yaml, but the
// boot-time wiring that builds a REAL translator from the shipped bundle
// was never written — every caller ran with NoopTranslator{}, so users
// saw raw message-ID keys instead of resolved + interpolated text.
// NewTranslator closes that gap: it embeds the active.en.yaml bundle,
// loads it into a pkg/i18n *Bundle, constructs an *i18n.Localizer, and
// wraps it in an *i18nadapter.Translator that structurally satisfies
// this package's local Translator interface.
//
// The bundle is go:embed'd so a deployed binary needs no on-disk YAML —
// the message catalogue ships inside the executable.
package i18n

import (
	"embed"
	"fmt"

	"dev.helix.code/pkg/i18n"
	"dev.helix.code/pkg/i18nadapter"
	"golang.org/x/text/language"
)

// activeBundleFS embeds the shipped en locale catalogue so the
// translator is self-contained in the compiled binary.
//
//go:embed bundles/active.en.yaml
var activeBundleFS embed.FS

// activeBundlePath is the embedded path of the en message file. It MUST
// follow go-i18n's "active.<lang>.yaml" naming convention so the language
// tag (en) is inferred from the filename when the file is parsed.
const activeBundlePath = "bundles/active.en.yaml"

// NewTranslator builds a real Translator backed by the embedded
// active.en.yaml catalogue. It is the boot-time replacement for the
// NoopTranslator{} default: the consuming binary calls this once and
// injects the result via the package's SetTranslator.
//
// langs follows go-i18n accept-language semantics (ordered preference
// list, e.g. "sr-RS", "en"); an empty list falls back to en (the
// bundle's default + only currently-shipped locale). Returns an error
// (never a NoopTranslator) if the embedded bundle fails to load, so a
// misconfiguration surfaces loudly instead of silently degrading to
// raw-key echo — a §11.4 PASS-bluff at the i18n layer.
func NewTranslator(langs ...string) (Translator, error) {
	bundle := i18n.NewBundle(language.English)
	if err := bundle.LoadMessageFileFS(activeBundleFS, activeBundlePath); err != nil {
		return nil, fmt.Errorf("internal/render i18n: load embedded %q: %w", activeBundlePath, err)
	}
	if len(langs) == 0 {
		langs = []string{language.English.String()}
	}
	loc := i18n.NewLocalizer(bundle, langs...)
	// *i18nadapter.Translator's method set (T(ctx,id,data),
	// TPlural(ctx,id,count,data)) structurally satisfies this package's
	// local Translator interface — no project-aware coupling required.
	return i18nadapter.New(loc), nil
}
