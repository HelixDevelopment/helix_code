// Unit tests for the examples/ i18n seam: Translator interface,
// NoopTranslator default, the Tr() resolver, SetTranslator DI, the
// active.en.yaml bundle integrity, and CONST-046 paired-mutation
// assertions over the migrated example sources. Mocks ALLOWED per
// CONST-050(A) (unit tests only).
//
// Round-347 §11.4 anti-bluff sweep (2026-05-20).
package i18n

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── Translator / NoopTranslator ─────────────────────────────────

func TestNoopTranslator_T_ReturnsID(t *testing.T) {
	got, err := NoopTranslator{}.T(context.Background(), "examples_basic_header", nil)
	if err != nil {
		t.Fatalf("NoopTranslator.T returned error: %v", err)
	}
	if got != "examples_basic_header" {
		t.Fatalf("NoopTranslator.T returned %q, want loud echo of message ID", got)
	}
}

func TestNoopTranslator_TPlural_ReturnsID(t *testing.T) {
	got, err := NoopTranslator{}.TPlural(context.Background(), "examples_basic_statistics_header", 3, nil)
	if err != nil {
		t.Fatalf("NoopTranslator.TPlural returned error: %v", err)
	}
	if got != "examples_basic_statistics_header" {
		t.Fatalf("NoopTranslator.TPlural returned %q, want loud echo of message ID", got)
	}
}

func TestNoopTranslator_T_IgnoresTemplateData(t *testing.T) {
	// Anti-bluff: NoopTranslator returns the raw ID even when
	// templateData is provided, so a call-site test can detect a
	// non-i18n call site by the literal remaining unchanged.
	got, err := NoopTranslator{}.T(context.Background(), "examples_qa_integration_waiting", map[string]any{"X": "y"})
	if err != nil {
		t.Fatalf("NoopTranslator.T returned error: %v", err)
	}
	if got != "examples_qa_integration_waiting" {
		t.Fatalf("NoopTranslator.T returned %q, want raw message ID (ignoring templateData)", got)
	}
}

// fakeTranslator wraps the message ID in a sentinel so call-site
// tests can assert the lookup actually went through Translator.T.
type fakeTranslator struct {
	failOnID string
}

func (f fakeTranslator) T(_ context.Context, id string, _ map[string]any) (string, error) {
	if f.failOnID != "" && id == f.failOnID {
		return "", errors.New("fakeTranslator: deliberate failure for " + id)
	}
	return "<TR:" + id + ">", nil
}

func (f fakeTranslator) TPlural(_ context.Context, id string, _ int, _ map[string]any) (string, error) {
	if f.failOnID != "" && id == f.failOnID {
		return "", errors.New("fakeTranslator: deliberate failure for " + id)
	}
	return "<TR:" + id + ">", nil
}

// ── Tr() resolver + SetTranslator DI ────────────────────────────

func TestTr_DefaultsToNoopEcho(t *testing.T) {
	SetTranslator(nil) // reset to NoopTranslator
	t.Cleanup(func() { SetTranslator(nil) })
	if got := Tr(context.Background(), "examples_debugging_header", nil); got != "examples_debugging_header" {
		t.Fatalf("Tr default returned %q, want loud message-ID echo", got)
	}
}

func TestTr_RoutesThroughWiredTranslator(t *testing.T) {
	SetTranslator(fakeTranslator{})
	t.Cleanup(func() { SetTranslator(nil) })
	got := Tr(context.Background(), "examples_templates_header", nil)
	if got != "<TR:examples_templates_header>" {
		t.Fatalf("Tr with wired translator returned %q, want sentinel-wrapped ID", got)
	}
}

func TestTr_TranslationFailureDegradesToID(t *testing.T) {
	// Anti-bluff: a translator error must degrade to the loud
	// message ID, never an empty string (a silent swallow would be
	// a §11.4 PASS-bluff at the i18n layer).
	SetTranslator(fakeTranslator{failOnID: "examples_basic_header"})
	t.Cleanup(func() { SetTranslator(nil) })
	if got := Tr(context.Background(), "examples_basic_header", nil); got != "examples_basic_header" {
		t.Fatalf("Tr on translator error returned %q, want loud message-ID fallback", got)
	}
}

func TestSetTranslator_NilResetsToNoop(t *testing.T) {
	SetTranslator(fakeTranslator{})
	SetTranslator(nil)
	t.Cleanup(func() { SetTranslator(nil) })
	if got := Tr(context.Background(), "examples_multi_session_header", nil); got != "examples_multi_session_header" {
		t.Fatalf("after SetTranslator(nil), Tr returned %q, want NoopTranslator echo", got)
	}
}

// ── Bundle integrity ────────────────────────────────────────────

// migratedIDs is the closed set of message IDs introduced by the
// round-347 examples/ CONST-046 migration. Every one MUST resolve in
// the bundle; the bundle MUST NOT carry extras (drift detection).
var migratedIDs = []string{
	"examples_basic_header",
	"examples_basic_no_previous_state",
	"examples_basic_state_restored",
	"examples_basic_state_saved",
	"examples_basic_statistics_header",
	"examples_feature_dev_header",
	"examples_feature_dev_phase_planning",
	"examples_feature_dev_phase_implementation",
	"examples_feature_dev_phase_testing",
	"examples_feature_dev_summary_header",
	"examples_code_review_header",
	"examples_code_review_summary_header",
	"examples_code_review_exporting",
	"examples_debugging_header",
	"examples_templates_header",
	"examples_multi_session_header",
	"examples_multi_session_starting_auth",
	"examples_multi_session_switch_payments",
	"examples_multi_session_payments_done",
	"examples_multi_session_resuming_auth",
	"examples_multi_session_summary_header",
	"examples_qa_integration_token_required",
	"examples_qa_integration_starting_session",
	"examples_qa_integration_waiting",
	"examples_qa_integration_fetching_report",
	"examples_qa_integration_listing_sessions",
}

func readBundle(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("bundles", "active.en.yaml"))
	if err != nil {
		t.Fatalf("read active.en.yaml: %v", err)
	}
	return string(data)
}

func TestBundle_ContainsEveryMigratedID(t *testing.T) {
	bundle := readBundle(t)
	for _, id := range migratedIDs {
		if !strings.Contains(bundle, id+":") {
			t.Errorf("bundle active.en.yaml missing migrated message ID %q", id)
		}
	}
}

func TestBundle_EntriesHaveNonEmptyValues(t *testing.T) {
	// Anti-bluff: an empty bundle value would make Tr() degrade to
	// the raw message ID — an i18n bluff. Every entry must carry a
	// real localized string.
	bundle := readBundle(t)
	for _, line := range strings.Split(bundle, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		idx := strings.Index(trimmed, ":")
		if idx < 0 {
			continue
		}
		val := strings.TrimSpace(trimmed[idx+1:])
		if val == "" || val == `""` {
			t.Errorf("bundle entry %q has empty value", strings.TrimSpace(trimmed[:idx]))
		}
	}
}

// ── CONST-046 paired-mutation: literals absent from sources ──────

// migratedLiterals maps each migrated example source file (relative
// to the examples/ root) to the English literals that MUST NO LONGER
// appear as bare string literals in it. A regression that reverts a
// tr() call back to a hardcoded literal trips this test.
var migratedLiterals = map[string][]string{
	"phase3/basic/main.go": {
		`"=== HelixCode Phase 3 Basic Example ==="`,
		`"No previous state found, starting fresh"`,
		`"Restored previous state successfully"`,
		`"State saved successfully"`,
		`"=== Statistics ==="`,
	},
	"phase3/feature_dev/main.go": {
		`"=== Feature Development Workflow ==="`,
		`"📋 Phase 1: Planning"`,
		`"🔨 Phase 2: Implementation"`,
		`"🧪 Phase 3: Testing"`,
		`"=== Feature Development Summary ==="`,
	},
	"phase3/code_review/main.go": {
		`"=== Code Review Workflow ==="`,
		`"=== Review Summary ==="`,
		`"\nExporting review conversation..."`,
	},
	"phase3/debugging/main.go": {
		`"=== Debugging Workflow ==="`,
	},
	"phase3/templates/main.go": {
		`"=== Template Library Example ==="`,
	},
	"phase3/multi_session/main.go": {
		`"=== Multi-Session Workflow ==="`,
		`"Starting auth work..."`,
		`"Pausing auth, switching to payments..."`,
		`"Completed payments work"`,
		`"Resuming auth work..."`,
		`"\n=== Sessions Summary ==="`,
	},
	"qa_integration/main.go": {
		`"HELIXCODE_TOKEN environment variable is required"`,
		`"Starting QA session..."`,
		`"Waiting for session to complete..."`,
		`"Fetching report..."`,
		`"Listing all sessions..."`,
	},
}

func TestMigratedSources_NoHardcodedLiterals(t *testing.T) {
	for rel, literals := range migratedLiterals {
		// examples/i18n is one level below examples/; sources sit
		// at ../<rel>.
		path := filepath.Join("..", rel)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read migrated source %s: %v", path, err)
		}
		src := string(data)
		for _, lit := range literals {
			if strings.Contains(src, lit) {
				t.Errorf("CONST-046 regression: %s still contains hardcoded literal %s", rel, lit)
			}
		}
		// Positive assertion: the file must route through the i18n
		// seam (otherwise a deletion would falsely pass).
		if !strings.Contains(src, "examples/i18n") || !strings.Contains(src, "i18n.Tr(") {
			t.Errorf("%s does not import/use the examples/i18n seam — migration incomplete", rel)
		}
	}
}

func TestMigratedSources_EveryMessageIDReferenced(t *testing.T) {
	// Anti-bluff: every bundle ID must be referenced by at least one
	// migrated source — a dangling bundle entry is dead i18n.
	allSrc := &strings.Builder{}
	for rel := range migratedLiterals {
		data, err := os.ReadFile(filepath.Join("..", rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		allSrc.Write(data)
	}
	joined := allSrc.String()
	for _, id := range migratedIDs {
		if !strings.Contains(joined, `"`+id+`"`) {
			t.Errorf("message ID %q is in the bundle but referenced by no migrated source", id)
		}
	}
}
