// Unit tests for the internal/security package-level translator +
// tr() helper (CONST-046 round-176 §11.4 anti-bluff sweep,
// 2026-05-19).
//
// Paired-mutation test per §11.4: planted/unplanted Translator yields
// distinguishable output at every migrated call site. Mocks ALLOWED
// per CONST-050(A) (unit tests only).
//
// Scope note: covers helix_code/internal/security/ ONLY. The
// root-level security/ submodule (round 130) has its own tests.
package security

import (
	stdctx "context"
	"errors"
	"strings"
	"testing"

	securityi18n "dev.helix.code/internal/security/i18n"
)

// sentinelTranslator returns "<TR:" + id + ">" so call-site tests can
// assert tr() actually went through Translator.T rather than returning
// a hardcoded literal that happened to match the bundle value.
type sentinelTranslator struct{}

func (sentinelTranslator) T(_ stdctx.Context, id string, _ map[string]any) (string, error) {
	return "<TR:" + id + ">", nil
}
func (sentinelTranslator) TPlural(_ stdctx.Context, id string, _ int, _ map[string]any) (string, error) {
	return "<TR:" + id + ">", nil
}

type errTranslator struct{}

func (errTranslator) T(_ stdctx.Context, _ string, _ map[string]any) (string, error) {
	return "", errors.New("intentional translator failure")
}
func (errTranslator) TPlural(_ stdctx.Context, _ string, _ int, _ map[string]any) (string, error) {
	return "", errors.New("intentional translator failure")
}

// resetTranslator restores the package-level translator after each
// test so cross-test pollution can't mask a regression.
func resetTranslator(t *testing.T) {
	t.Helper()
	SetTranslator(nil)
}

func TestTr_DefaultsToNoopTranslator(t *testing.T) {
	resetTranslator(t)
	got := tr(stdctx.Background(), "internal_security_global_manager_initialized", nil)
	if got == "internal_security_global_manager_initialized" || got == "" {
		t.Fatalf("HXC-097 §11.4.120: default/nil path must resolve to bundle prose, got %q (raw key or empty)", got)
	}
}

func TestTr_UsesInjectedTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	got := tr(stdctx.Background(), "internal_security_scan_starting", nil)
	if got != "<TR:internal_security_scan_starting>" {
		t.Fatalf("tr = %q, want sentinel-wrapped ID — call site bypassed Translator", got)
	}
}

func TestTr_TranslatorErrorReturnsMessageID(t *testing.T) {
	// Anti-bluff: an erroring Translator MUST NOT silently return an
	// empty string (that would be a §11.4 PASS-bluff at the i18n
	// layer — user sees blank output). Implementation MUST degrade to
	// the message ID.
	resetTranslator(t)
	SetTranslator(errTranslator{})
	defer resetTranslator(t)

	got := tr(stdctx.Background(), "internal_security_scanner_error", nil)
	if got != "internal_security_scanner_error" {
		t.Fatalf("tr on err = %q, want raw message ID (no silent swallow)", got)
	}
}

func TestSetTranslator_NilResetsToNoop(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	SetTranslator(nil) // explicit reset
	defer resetTranslator(t)

	got := tr(stdctx.Background(), "internal_security_no_scanners_available", nil)
	if got == "internal_security_no_scanners_available" || got == "" {
		t.Fatalf("HXC-097 §11.4.120: default/nil path must resolve to bundle prose, got %q (raw key or empty)", got)
	}
}

func TestSetTranslator_AcceptsNoopExplicit(t *testing.T) {
	resetTranslator(t)
	defer resetTranslator(t)

	SetTranslator(securityi18n.NoopTranslator{})
	got := tr(stdctx.Background(), "internal_security_metrics_updated", nil)
	if got != "internal_security_metrics_updated" {
		t.Fatalf("tr with explicit NoopTranslator = %q, want raw ID", got)
	}
}

// TestScanFeature_NoScannersRecommendationGoesThroughTranslator covers
// the no-scanners-configured recommendation path. With sentinel wired,
// the Recommendations slice MUST contain the sentinel-wrapped message
// ID — proving the literal was NOT hardcoded on the path.
func TestScanFeature_NoScannersRecommendationGoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	// NewSecurityManager creates scanners that are unavailable when
	// SONARQUBE_URL/SONARQUBE_TOKEN/SNYK_TOKEN env vars are unset —
	// driving the no-scanners-configured branch.
	sm := NewSecurityManagerWithScanners()
	result, err := sm.ScanFeature("trigger-no-scanners")
	if err != nil {
		t.Fatalf("ScanFeature returned err = %v", err)
	}
	if result == nil {
		t.Fatal("ScanFeature returned nil result")
	}
	if len(result.Recommendations) != 1 {
		t.Fatalf("Recommendations len = %d, want 1", len(result.Recommendations))
	}
	want := "<TR:internal_security_recommendation_no_scanners>"
	if result.Recommendations[0] != want {
		t.Fatalf("Recommendations[0] = %q, want %q — ScanFeature bypassed tr()",
			result.Recommendations[0], want)
	}
}

// TestScanFeature_ReviewIssuesRecommendationGoesThroughTranslator
// covers the post-scan review recommendation path. A stub scanner that
// reports availability + returns one issue drives the review-issues
// branch; the recommendation MUST be sentinel-wrapped.
func TestScanFeature_ReviewIssuesRecommendationGoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	sm := NewSecurityManagerWithScanners(&stubAvailableScanner{
		issues: []SecurityIssue{
			{Severity: "MAJOR", Title: "x", Description: "y"},
		},
	})
	result, err := sm.ScanFeature("trigger-review-rec")
	if err != nil {
		t.Fatalf("ScanFeature returned err = %v", err)
	}
	if result == nil {
		t.Fatal("ScanFeature returned nil result")
	}
	if len(result.Recommendations) != 1 {
		t.Fatalf("Recommendations len = %d, want 1", len(result.Recommendations))
	}
	want := "<TR:internal_security_recommendation_review_issues>"
	if result.Recommendations[0] != want {
		t.Fatalf("Recommendations[0] = %q, want %q — ScanFeature bypassed tr()",
			result.Recommendations[0], want)
	}
}

// TestRawText_EmittedByDefault asserts that with no translator wired
// (NoopTranslator), the no-scanners path emits the bundle message ID
// verbatim — confirming the migration didn't accidentally pass an
// empty string or a different literal.
func TestRawText_EmittedByDefault(t *testing.T) {
	resetTranslator(t)

	sm := NewSecurityManagerWithScanners()
	result, err := sm.ScanFeature("noop-emit")
	if err != nil {
		t.Fatalf("ScanFeature returned err = %v", err)
	}
	if result == nil || len(result.Recommendations) != 1 {
		t.Fatalf("unexpected result = %#v", result)
	}
	if !strings.Contains(result.Recommendations[0], "internal_security_recommendation_no_scanners") {
		t.Fatalf("rec = %q, want contain raw ID (Noop echo)", result.Recommendations[0])
	}
}

// stubAvailableScanner is a unit-test-only Scanner used to drive the
// "any scanner succeeded" branch of ScanFeatureContext. Mocks/stubs
// PERMITTED here per CONST-050(A) (unit-test file).
type stubAvailableScanner struct {
	issues []SecurityIssue
}

func (s *stubAvailableScanner) Name() string                      { return "stub-available" }
func (s *stubAvailableScanner) IsAvailable(_ stdctx.Context) bool { return true }
func (s *stubAvailableScanner) Close() error                      { return nil }
func (s *stubAvailableScanner) Scan(_ stdctx.Context, _ string) (*ScanResult, error) {
	return &ScanResult{
		ScannerName: s.Name(),
		Issues:      s.issues,
		Success:     true,
	}, nil
}
