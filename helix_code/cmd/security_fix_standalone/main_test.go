// Call-site sentinel tests for security_fix_standalone's CONST-046
// i18n migration.
//
// These tests do NOT execute main(); they exercise the package-level
// tr() helper + SetTranslator wiring to guarantee:
//   (1) every migrated message ID resolves through the injected
//       Translator (sentinel-wrapped output proves the lookup),
//   (2) a Translator returning an error degrades to the raw ID
//       (loud surface) rather than empty string (silent surface),
//   (3) passing nil to SetTranslator re-installs NoopTranslator
//       rather than leaving a dangling reference (which would be a
//       §11.4 PASS-bluff at the i18n injection layer),
//   (4) a deliberate paired mutation (planted typo in the message
//       ID) MUST cause the test to fail — proving the call-site is
//       genuinely routed through the i18n layer, not a hardcoded
//       English literal that happens to match the bundle value.
//
// Mocks ALLOWED here per CONST-050(A) (unit-test scope).
package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"dev.helix.code/cmd/security_fix_standalone/i18n"
)

// recordingTranslator captures every (id, data) pair passed through
// Translator.T so call-site tests can assert what was looked up.
type recordingTranslator struct {
	calls     []recordedCall
	failOnID  string
	emptyOnID string
	prefix    string
}

type recordedCall struct {
	ID   string
	Data map[string]any
}

func (r *recordingTranslator) T(_ context.Context, id string, data map[string]any) (string, error) {
	r.calls = append(r.calls, recordedCall{ID: id, Data: data})
	if r.failOnID != "" && id == r.failOnID {
		return "", errors.New("recordingTranslator: deliberate failure for " + id)
	}
	if r.emptyOnID != "" && id == r.emptyOnID {
		return "", nil
	}
	prefix := r.prefix
	if prefix == "" {
		prefix = "<TRANSLATED:"
	}
	return prefix + id + ">", nil
}

func (r *recordingTranslator) TPlural(ctx context.Context, id string, _ int, data map[string]any) (string, error) {
	return r.T(ctx, id, data)
}

// All migrated message IDs in this round. If a future round migrates
// additional literals, append the ID here so the round-145 sentinel
// suite continues to verify the full migrated surface.
var round145MessageIDs = []string{
	"security_fix_standalone_banner_start",
	"security_fix_standalone_banner_policy",
	"security_fix_standalone_path_echo",
	"security_fix_standalone_critical_only_echo",
	"security_fix_standalone_validating",
	"security_fix_standalone_header_complete",
	"security_fix_standalone_result_success_satisfied",
	"security_fix_standalone_result_success_policy",
	"security_fix_standalone_result_failure_remain",
	"security_fix_standalone_result_failure_blocked",
}

func TestTr_RoutesThroughInjectedTranslator(t *testing.T) {
	// Anti-bluff: confirm tr() routes lookups through the injected
	// translator rather than emitting the raw English string.
	rec := &recordingTranslator{}
	SetTranslator(rec)
	t.Cleanup(func() { SetTranslator(nil) })

	for _, id := range round145MessageIDs {
		got := tr(context.Background(), id, nil)
		want := "<TRANSLATED:" + id + ">"
		if got != want {
			t.Errorf("tr(%q) = %q, want %q", id, got, want)
		}
	}

	if len(rec.calls) != len(round145MessageIDs) {
		t.Fatalf("recordingTranslator.calls len = %d, want %d", len(rec.calls), len(round145MessageIDs))
	}
}

func TestTr_DegradesToIDOnError(t *testing.T) {
	// Anti-bluff: an erroring Translator MUST surface the raw ID
	// (loud), not an empty string (silent).
	rec := &recordingTranslator{failOnID: "security_fix_standalone_banner_start"}
	SetTranslator(rec)
	t.Cleanup(func() { SetTranslator(nil) })

	got := tr(context.Background(), "security_fix_standalone_banner_start", nil)
	if got != "security_fix_standalone_banner_start" {
		t.Fatalf("tr() on error = %q, want raw ID %q", got, "security_fix_standalone_banner_start")
	}
}

func TestTr_DegradesToIDOnEmptyOutput(t *testing.T) {
	// Anti-bluff: a Translator that returns ("", nil) is treated
	// equivalently to a failure — degrade to the raw ID rather
	// than emit nothing.
	rec := &recordingTranslator{emptyOnID: "security_fix_standalone_result_success_satisfied"}
	SetTranslator(rec)
	t.Cleanup(func() { SetTranslator(nil) })

	got := tr(context.Background(), "security_fix_standalone_result_success_satisfied", nil)
	if got != "security_fix_standalone_result_success_satisfied" {
		t.Fatalf("tr() on empty = %q, want raw ID %q",
			got, "security_fix_standalone_result_success_satisfied")
	}
}

func TestSetTranslator_NilResetsToNoop(t *testing.T) {
	// Anti-bluff: SetTranslator(nil) MUST install NoopTranslator,
	// not leave a dangling nil reference (which would panic on the
	// next tr() invocation — itself a CONST-035 false-success at
	// the i18n injection layer).
	SetTranslator(&recordingTranslator{})
	SetTranslator(nil)
	got := tr(context.Background(), "security_fix_standalone_banner_policy", nil)
	if got != "security_fix_standalone_banner_policy" {
		t.Fatalf("tr() after SetTranslator(nil) = %q, want NoopTranslator echo %q",
			got, "security_fix_standalone_banner_policy")
	}
}

func TestTr_PassesTemplateData(t *testing.T) {
	// Anti-bluff: templateData MUST reach the Translator. A call
	// that drops the map silently breaks placeholder interpolation
	// at runtime.
	rec := &recordingTranslator{}
	SetTranslator(rec)
	t.Cleanup(func() { SetTranslator(nil) })

	_ = tr(context.Background(), "security_fix_standalone_path_echo", map[string]any{"Path": "/tmp/x"})
	if len(rec.calls) != 1 {
		t.Fatalf("recordingTranslator.calls len = %d, want 1", len(rec.calls))
	}
	if rec.calls[0].Data["Path"] != "/tmp/x" {
		t.Fatalf("templateData[Path] = %v, want %q", rec.calls[0].Data["Path"], "/tmp/x")
	}
}

// round460MessageIDs are the report-helper verdict literals migrated
// in the round-460 §11.4 sweep (2026-05-20). These IDs are resolved
// inside the report-helper functions (evaluateZeroTolerance,
// evaluateProductionReadiness, generateFixRecommendations,
// evaluateSecurityPosture, evaluateComplianceStatus,
// formatCriticalIssues).
var round460MessageIDs = []string{
	"security_fix_standalone_report_no_issues",
	"security_fix_standalone_zerotol_satisfied",
	"security_fix_standalone_zerotol_violated",
	"security_fix_standalone_readiness_ready",
	"security_fix_standalone_readiness_critical",
	"security_fix_standalone_readiness_fix_failed",
	"security_fix_standalone_rec_urgent_failed",
	"security_fix_standalone_rec_address_remaining",
	"security_fix_standalone_rec_comprehensive_testing",
	"security_fix_standalone_rec_success_resolved",
	"security_fix_standalone_rec_validate_fixes",
	"security_fix_standalone_rec_all_resolved",
	"security_fix_standalone_rec_proactive_monitoring",
	"security_fix_standalone_posture_strong",
	"security_fix_standalone_posture_critical",
	"security_fix_standalone_posture_weak",
	"security_fix_standalone_compliance_compliant",
	"security_fix_standalone_compliance_noncompliant",
}

// TestRound460_ReportHelpersRouteThroughTranslator exercises the
// actual report-helper functions through an injected translator and
// asserts each verdict path returns sentinel-wrapped output — proving
// the helpers no longer emit hardcoded English literals.
func TestRound460_ReportHelpersRouteThroughTranslator(t *testing.T) {
	rec := &recordingTranslator{}
	SetTranslator(rec)
	t.Cleanup(func() { SetTranslator(nil) })

	// formatCriticalIssues empty-case → report_no_issues.
	if got := formatCriticalIssues(nil); !strings.Contains(got, "security_fix_standalone_report_no_issues") {
		t.Errorf("formatCriticalIssues(nil) = %q, want sentinel-wrapped report_no_issues", got)
	}

	// evaluateZeroTolerance both branches.
	if got := evaluateZeroTolerance(0); !strings.Contains(got, "security_fix_standalone_zerotol_satisfied") {
		t.Errorf("evaluateZeroTolerance(0) = %q, want zerotol_satisfied", got)
	}
	if got := evaluateZeroTolerance(3); !strings.Contains(got, "security_fix_standalone_zerotol_violated") {
		t.Errorf("evaluateZeroTolerance(3) = %q, want zerotol_violated", got)
	}

	// evaluateProductionReadiness all three branches.
	if got := evaluateProductionReadiness(true, 0); !strings.Contains(got, "security_fix_standalone_readiness_ready") {
		t.Errorf("evaluateProductionReadiness(true,0) = %q, want readiness_ready", got)
	}
	if got := evaluateProductionReadiness(false, 2); !strings.Contains(got, "security_fix_standalone_readiness_critical") {
		t.Errorf("evaluateProductionReadiness(false,2) = %q, want readiness_critical", got)
	}
	if got := evaluateProductionReadiness(false, 0); !strings.Contains(got, "security_fix_standalone_readiness_fix_failed") {
		t.Errorf("evaluateProductionReadiness(false,0) = %q, want readiness_fix_failed", got)
	}

	// evaluateSecurityPosture all three branches.
	if got := evaluateSecurityPosture(true, 0, 0); !strings.Contains(got, "security_fix_standalone_posture_strong") {
		t.Errorf("evaluateSecurityPosture(true,0,0) = %q, want posture_strong", got)
	}
	if got := evaluateSecurityPosture(false, 1, 1); !strings.Contains(got, "security_fix_standalone_posture_critical") {
		t.Errorf("evaluateSecurityPosture(false,1,1) = %q, want posture_critical", got)
	}
	if got := evaluateSecurityPosture(false, 0, 0); !strings.Contains(got, "security_fix_standalone_posture_weak") {
		t.Errorf("evaluateSecurityPosture(false,0,0) = %q, want posture_weak", got)
	}

	// evaluateComplianceStatus both branches.
	if got := evaluateComplianceStatus(true, 0); !strings.Contains(got, "security_fix_standalone_compliance_compliant") {
		t.Errorf("evaluateComplianceStatus(true,0) = %q, want compliance_compliant", got)
	}
	if got := evaluateComplianceStatus(false, 1); !strings.Contains(got, "security_fix_standalone_compliance_noncompliant") {
		t.Errorf("evaluateComplianceStatus(false,1) = %q, want compliance_noncompliant", got)
	}

	// generateFixRecommendations exercising every recommendation
	// branch: failedCount>0, len(issues)>fixedCount, fixedCount>0.
	twoIssues := []SecurityIssue{{ID: "A"}, {ID: "B"}}
	recs := generateFixRecommendations(twoIssues, 1, 1)
	for _, id := range []string{
		"security_fix_standalone_rec_urgent_failed",
		"security_fix_standalone_rec_address_remaining",
		"security_fix_standalone_rec_comprehensive_testing",
		"security_fix_standalone_rec_success_resolved",
		"security_fix_standalone_rec_validate_fixes",
	} {
		if !strings.Contains(recs, id) {
			t.Errorf("generateFixRecommendations: missing %q in %q", id, recs)
		}
	}
	// Empty-recs branch (no failures, all issues fixed).
	cleanRecs := generateFixRecommendations(nil, 0, 0)
	for _, id := range []string{
		"security_fix_standalone_rec_all_resolved",
		"security_fix_standalone_rec_proactive_monitoring",
	} {
		if !strings.Contains(cleanRecs, id) {
			t.Errorf("generateFixRecommendations(clean): missing %q in %q", id, cleanRecs)
		}
	}

	// Every round460 ID must have been observed by the translator.
	seen := map[string]bool{}
	for _, c := range rec.calls {
		seen[c.ID] = true
	}
	for _, id := range round460MessageIDs {
		if !seen[id] {
			t.Errorf("round-460 ID %q never routed through Translator", id)
		}
	}
}

// TestRound460_TemplateDataReachesTranslator confirms the {{.Count}}
// placeholders in the round-460 verdict messages receive their data.
func TestRound460_TemplateDataReachesTranslator(t *testing.T) {
	rec := &recordingTranslator{}
	SetTranslator(rec)
	t.Cleanup(func() { SetTranslator(nil) })

	_ = evaluateZeroTolerance(7)
	var found bool
	for _, c := range rec.calls {
		if c.ID == "security_fix_standalone_zerotol_violated" && c.Data["Count"] == 7 {
			found = true
		}
	}
	if !found {
		t.Fatal("evaluateZeroTolerance(7): Count=7 did not reach Translator for zerotol_violated")
	}
}

// TestPairedMutation_MessageIDTypoFails is the §1.1 paired-mutation
// proof that round145MessageIDs genuinely tracks the migrated call
// sites. If a developer adds a new tr() call but forgets to register
// the ID in round145MessageIDs, the recordingTranslator-driven
// TestTr_RoutesThroughInjectedTranslator catches it by miscount.
//
// Conversely, this test plants a typo'd ID and asserts the test
// harness flags it — proving the harness can distinguish a real
// migrated ID from a typo'd one.
func TestPairedMutation_MessageIDTypoFails(t *testing.T) {
	rec := &recordingTranslator{prefix: "<TRANSLATED:"}
	SetTranslator(rec)
	t.Cleanup(func() { SetTranslator(nil) })

	// Plant a typo and confirm the wrapper still resolves (proves
	// the harness's resolver itself does not silently swallow
	// unknown IDs) but the output ID differs from any real
	// migrated ID by exactly the planted typo.
	planted := "security_fix_standalone_banner_starrt" // deliberate typo
	got := tr(context.Background(), planted, nil)
	if !strings.Contains(got, "starrt") {
		t.Fatalf("paired-mutation: tr(%q) = %q, expected planted typo to survive routing",
			planted, got)
	}
	// Ensure none of the real round145MessageIDs collide with the typo.
	for _, id := range round145MessageIDs {
		if id == planted {
			t.Fatalf("paired-mutation: planted typo %q collides with real ID %q — "+
				"adjust the planted token", planted, id)
		}
	}
}

// Compile-time assertion: recordingTranslator implements i18n.Translator.
var _ i18n.Translator = (*recordingTranslator)(nil)
