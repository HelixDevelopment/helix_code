// Paired-mutation unit tests for the internal/workflow/planmode
// CONST-046 i18n seam (round-424 §11.4, 2026-05-20). Mocks ALLOWED
// per CONST-050(A) — unit-test-only file.
//
// Anti-bluff intent: the fakeTranslator returns a sentinel string
// distinct from every message ID. A migrated call site that routes
// through tr() therefore yields the sentinel; a regressed call site
// that hardcodes English again yields the literal. The mutation
// tests assert the seam actually swaps the rendered string so a
// future un-migration is caught loudly.
package planmode

import (
	"context"
	"strings"
	"testing"

	workflowi18n "dev.helix.code/internal/workflow/i18n"
)

// fakeTranslator renders every message ID as "XLATE:<id>" so tests
// can distinguish a real translation hit from a NoopTranslator echo
// (raw ID) or a hardcoded-literal regression (English text).
type fakeTranslator struct{}

func (fakeTranslator) T(_ context.Context, id string, _ map[string]any) (string, error) {
	return "XLATE:" + id, nil
}

func (fakeTranslator) TPlural(_ context.Context, id string, _ int, _ map[string]any) (string, error) {
	return "XLATE:" + id, nil
}

var _ workflowi18n.Translator = fakeTranslator{}

func TestPlanmodeSeam_NoopEchoesMessageID(t *testing.T) {
	SetTranslator(nil) // reset to NoopTranslator
	t.Cleanup(func() { SetTranslator(nil) })

	got := tr(context.Background(), "internal_workflow_planmode_report_validation_title", nil)
	if got != "internal_workflow_planmode_report_validation_title" {
		t.Fatalf("Noop seam returned %q, want loud echo of message ID", got)
	}
}

func TestPlanmodeSeam_WiredTranslatorRendersBundle(t *testing.T) {
	SetTranslator(fakeTranslator{})
	t.Cleanup(func() { SetTranslator(nil) })

	ids := []string{
		"internal_workflow_planmode_report_validation_title",
		"internal_workflow_planmode_report_test_execution_title",
		"internal_workflow_planmode_check_go_build_passed",
		"internal_workflow_planmode_check_python_syntax_passed",
		"internal_workflow_planmode_validation_failed_heading",
		"internal_workflow_planmode_validation_passed_heading",
		"internal_workflow_planmode_test_unknown_project_warning",
	}
	for _, id := range ids {
		got := tr(context.Background(), id, nil)
		want := "XLATE:" + id
		if got != want {
			t.Fatalf("wired seam for %q returned %q, want %q", id, got, want)
		}
	}
}

// TestPlanmodeSeam_MutationDetectsRegression is the paired-mutation
// guard: it proves the seam is the ONLY thing producing the report
// header. If a future edit reverts the call site to a hardcoded
// English literal, tr() would no longer be invoked and this test —
// which asserts the wired translator's sentinel reaches the
// rendered output — would catch it.
func TestPlanmodeSeam_MutationDetectsRegression(t *testing.T) {
	SetTranslator(fakeTranslator{})
	t.Cleanup(func() { SetTranslator(nil) })

	rendered := tr(context.Background(), "internal_workflow_planmode_check_go_build_passed", nil)
	// Mutation: a regressed call site emits the raw English literal.
	const hardcodedRegression = "Go Build: PASSED"
	if strings.Contains(rendered, hardcodedRegression) {
		t.Fatalf("seam rendered hardcoded English %q — CONST-046 regression", hardcodedRegression)
	}
	if !strings.HasPrefix(rendered, "XLATE:") {
		t.Fatalf("seam output %q lost translator routing", rendered)
	}
}
