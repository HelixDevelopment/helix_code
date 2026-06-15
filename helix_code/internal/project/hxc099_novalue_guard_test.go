// hxc099_novalue_guard_test.go — §11.4.115 RED-polarity regression guard for
// the HXC-099 Goal-B '<no value>' templating bug (2026-06-15).
//
// Root cause (FACT): the internal/project bundle messages for the DB/IO
// failure errors (internal_project_create_failed, _get_failed,
// _list_query_failed, _list_scan_failed, _list_iter_failed, _update_failed,
// _update_metadata_failed in manager_db.go; _detect_type_failed in
// manager.go) each ended in a "{{.Err}}" template placeholder, but EVERY
// call site invokes tr(ctx, KEY, nil) — i.e. with a nil data map — and
// appends the real error itself via fmt.Errorf("%s: %w", tr(...), err).
// With nil data the "{{.Err}}" placeholder has no binding, so Go
// text/template renders it as the literal "<no value>" — a dropped
// parameter leaking into the user-visible error string. The "%w"-appended
// err was already the correct error tail, so the placeholder was both
// redundant AND broken.
//
// Fix: drop the redundant "{{.Err}}" tail from the 8 bundle messages so the
// real translator renders clean prose (the err is still appended by the
// call site's %w wrap). This is independent of the package's Noop-by-default
// decision — it is purely a real-translator-path render bug.
//
// Polarity switch per §11.4.115: RED_MODE (default "0") flips this single
// source between two roles —
//
//	RED_MODE=1 — reproduce-and-assert-defect: render each offending key
//	             through a REAL translator whose bundle STILL carries the
//	             "{{.Err}}" tail (the pre-fix bundle text, reconstructed
//	             in-test) and assert the output DOES contain "<no value>".
//	RED_MODE=0 — standing GREEN regression guard: render each offending key
//	             through the as-shipped embedded bundle and assert the output
//	             does NOT contain "<no value>" and DOES contain the prose stem.
package project

import (
	stdctx "context"
	"os"
	"strings"
	"testing"

	projecti18n "dev.helix.code/internal/project/i18n"
	pkgi18n "dev.helix.code/pkg/i18n"
	"dev.helix.code/pkg/i18nadapter"
	"golang.org/x/text/language"
)

// novalueStems maps each offending message ID to a stable prose stem that
// MUST survive in the rendered output (proves we resolved real prose, not
// just stripped everything).
var novalueStems = map[string]string{
	"internal_project_create_failed":          "failed to create project in database",
	"internal_project_get_failed":             "failed to get project from database",
	"internal_project_list_query_failed":      "failed to query projects",
	"internal_project_list_scan_failed":       "failed to scan project row",
	"internal_project_list_iter_failed":       "error iterating project rows",
	"internal_project_update_failed":          "failed to update project",
	"internal_project_update_metadata_failed": "failed to update project metadata",
	"internal_project_detect_type_failed":     "failed to detect project type",
}

// prefixBundleText is the PRE-FIX message text (with the broken "{{.Err}}"
// tail) used only by RED_MODE to reproduce the defect on a pre-fix-equivalent
// state without mutating the shipped active.en.yaml.
var prefixBundleText = map[string]string{
	"internal_project_create_failed":          "failed to create project in database: {{.Err}}",
	"internal_project_get_failed":             "failed to get project from database: {{.Err}}",
	"internal_project_list_query_failed":      "failed to query projects: {{.Err}}",
	"internal_project_list_scan_failed":       "failed to scan project row: {{.Err}}",
	"internal_project_list_iter_failed":       "error iterating project rows: {{.Err}}",
	"internal_project_update_failed":          "failed to update project: {{.Err}}",
	"internal_project_update_metadata_failed": "failed to update project metadata: {{.Err}}",
	"internal_project_detect_type_failed":     "failed to detect project type: {{.Err}}",
}

// realTranslatorFromYAML builds a real *i18nadapter.Translator over an
// in-memory YAML bundle (the pre-fix text), so RED_MODE reproduces the defect
// without touching the shipped file.
func realTranslatorFromYAML(t *testing.T, id, msg string) *i18nadapter.Translator {
	t.Helper()
	body := id + ":\n  other: \"" + msg + "\"\n"
	bundle := pkgi18n.NewBundle(language.English)
	bundle.MustParseMessageFileBytes([]byte(body), "active.en.yaml")
	loc := pkgi18n.NewLocalizer(bundle, language.English.String())
	return i18nadapter.New(loc)
}

func TestHXC099_GoalB_NoValuePlaceholderDoesNotLeak(t *testing.T) {
	ctx := stdctx.Background()

	if os.Getenv("RED_MODE") == "1" {
		for id, msg := range prefixBundleText {
			real := realTranslatorFromYAML(t, id, msg)
			resetTranslator(t)
			SetTranslator(real)
			got := tr(ctx, id, nil) // exact call-site path: nil data map
			resetTranslator(t)
			if !strings.Contains(got, "<no value>") {
				t.Fatalf("RED_MODE: tr(%q, nil) = %q, expected '<no value>' leak "+
					"(RED must reproduce HXC-099 Goal B on the pre-fix bundle)", id, got)
			}
			t.Logf("RED_MODE reproduced HXC-099 Goal B: %q -> %q", id, got)
		}
		return
	}

	// GREEN guard: render through the AS-SHIPPED embedded bundle (exactly what
	// i18nwiring.WireAll installs) and assert no '<no value>' leaks.
	real, err := projecti18n.NewTranslator()
	if err != nil {
		t.Fatalf("projecti18n.NewTranslator (embedded bundle) failed: %v", err)
	}
	resetTranslator(t)
	SetTranslator(real)
	defer resetTranslator(t)

	for id, stem := range novalueStems {
		got := tr(ctx, id, nil)
		if strings.Contains(got, "<no value>") {
			t.Fatalf("HXC-099 GoalB REGRESSION: tr(%q, nil) = %q leaks '<no value>' "+
				"(dropped template param — bundle still carries an unbound {{.}} placeholder)", id, got)
		}
		if got == id {
			t.Fatalf("HXC-099 GoalB: tr(%q, nil) = %q echoed the raw key "+
				"(real translator not active — bundle key missing?)", id, got)
		}
		if !strings.Contains(got, stem) {
			t.Fatalf("HXC-099 GoalB: tr(%q, nil) = %q, expected to contain prose stem %q", id, got, stem)
		}
	}
}
