// bundle.go — boot-time Translator constructor for the aurora_os app.
//
// The CONST-046 migration created this package's Translator interface +
// NoopTranslator{} loud-echo default + bundles/active.en.yaml, AND
// translator.go documents the intended wire-in path ("helix_code constructs
// an *i18n.Localizer ... wraps it in *i18nadapter.Translator, and passes it to
// the aurora_os app via SetTranslator"). But that boot-time constructor was
// never written for the standalone aurora_os binary (GUI or `nogui`), so every
// run fell back to NoopTranslator{} and the CLI leaked raw message-ID keys
// (e.g. `aurora_os_cli_version_banner`, `aurora_os_cli_status_header`) instead
// of resolved text — a §11.4 / CONST-046 PASS-bluff visible on
// `version`/`help`/`status`. A second symptom rode along: Printf call sites
// that pass args to a format-string echoed as a bare key (no `%` verbs) emit
// Go's `%!(EXTRA string=go1.26.2, ...)` noise. NewTranslator closes both gaps,
// mirroring applications/desktop/i18n/bundle.go and
// applications/terminal_ui/i18n/bundle.go.
//
// The bundle is go:embed'd so a deployed binary needs no on-disk YAML — the
// message catalogue ships inside the executable.
package i18n

import (
	"embed"
	"fmt"

	"dev.helix.code/pkg/i18n"
	"dev.helix.code/pkg/i18nadapter"
	"golang.org/x/text/language"
)

// activeBundleFS embeds the shipped en locale catalogue so the translator is
// self-contained in the compiled binary.
//
//go:embed bundles/active.en.yaml
var activeBundleFS embed.FS

// activeBundlePath is the embedded path of the en message file. It MUST follow
// go-i18n's "active.<lang>.yaml" naming convention so the language tag (en) is
// inferred from the filename when the file is parsed.
const activeBundlePath = "bundles/active.en.yaml"

// NewTranslator builds a real Translator backed by the embedded active.en.yaml
// catalogue. It is the boot-time replacement for the NoopTranslator{} default:
// the aurora_os binary calls this once and injects the result via SetTranslator.
//
// langs follows go-i18n accept-language semantics (ordered preference list,
// e.g. "sr-RS", "en"); an empty list falls back to en (the bundle's default +
// only currently-shipped locale). Returns an error (never a NoopTranslator) if
// the embedded bundle fails to load, so a misconfiguration surfaces loudly
// instead of silently degrading to raw-key echo — a §11.4 PASS-bluff at the
// i18n layer.
func NewTranslator(langs ...string) (Translator, error) {
	bundle := i18n.NewBundle(language.English)
	if err := bundle.LoadMessageFileFS(activeBundleFS, activeBundlePath); err != nil {
		return nil, fmt.Errorf("aurora_os i18n: load embedded %q: %w", activeBundlePath, err)
	}
	if len(langs) == 0 {
		langs = []string{language.English.String()}
	}
	loc := i18n.NewLocalizer(bundle, langs...)
	// *i18nadapter.Translator's method set (T(ctx,id,data),
	// TPlural(ctx,id,count,data)) structurally satisfies this package's local
	// Translator interface — no project-aware coupling required.
	return i18nadapter.New(loc), nil
}
