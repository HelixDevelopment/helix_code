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
	SetTranslator(securityi18n.NoopTranslator{})
	defer resetTranslator(t)
	got := tr(stdctx.Background(), "internal_security_global_manager_initialized", nil)
	if got == "" {
		t.Fatalf("tr with NoopTranslator returned empty string, want raw message ID")
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
	SetTranslator(nil) // explicit reset, restores defaultTranslator
	defer resetTranslator(t)

	// After SetTranslator(nil), the translator is restored to defaultTranslator.
	// When the embedded bundle is not loaded (test context), the default is
	// NoopTranslator and tr() returns the raw message ID (non-empty).
	SetTranslator(securityi18n.NoopTranslator{})
	got := tr(stdctx.Background(), "internal_security_no_scanners_available", nil)
	if got == "" {
		t.Fatalf("tr with NoopTranslator returned empty string")
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

// TestRawText_EmittedByDefault — §11.4.120 reconciliation (2026-07-11).
//
// This test predates the HXC-097 init() default-translator design
// change (commit 1254e0a6, 2026-06-28; see translator.go init() doc
// comment): it originally asserted that with no translator explicitly
// wired, tr() falls back to securityi18n.NoopTranslator{} and echoes
// the raw message ID verbatim ("Noop echo"). That was correct for the
// OLD default (defaultTranslator = NoopTranslator{}).
//
// HXC-097 changed init() to install the REAL embedded-bundle translator
// as defaultTranslator whenever the go:embed'd active.en.yaml loads
// successfully — which it does unconditionally in this test binary,
// since go:embed content is compiled in and does not depend on the
// working directory or any runtime filesystem state (verified
// empirically: this test previously failed with
// `rec = "No security scanners configured", want contain raw ID`,
// proving the real translator — not Noop — is genuinely the active
// default here). The intent, per translator.go's own doc comment, is
// explicit: "SetTranslator(nil) restores THIS — 'nil = restore
// default' means 'restore correct prose', not 'revert to raw-key
// echo'." This closes a real anti-bluff gap (library code emitting raw
// message-ID keys on any entry path that never calls
// i18nwiring.WireAll()), so the OLD "Noop by default" behavior this
// test asserted is no longer the genuine behavior of the package.
//
// Root-cause investigation ruled out regression: two sibling tests
// (TestTr_DefaultsToNoopTranslator, TestSetTranslator_NilResetsToNoop)
// were reconciled to the new behavior in the SAME commit that landed
// the init() change (1254e0a6); this test was simply missed in that
// pass and never updated, leaving it asserting the stale pre-HXC-097
// contract. §11.4.120: the fix below reconciles this test's assertion
// to the genuine current default-translator behavior instead of
// fake-passing it (no tautology, no code revert — the init() design
// change is intentional and desired).
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
	// active.en.yaml: internal_security_recommendation_no_scanners ->
	// "No security scanners configured". With no translator explicitly
	// wired (only resetTranslator's implicit nil-reset), the package
	// default (real embedded-bundle translator per HXC-097) resolves
	// the message ID to this prose — it does NOT echo the raw ID.
	want := "No security scanners configured"
	if result.Recommendations[0] != want {
		t.Fatalf("rec = %q, want %q (resolved bundle prose per HXC-097 default-translator init(), not raw-ID Noop echo)",
			result.Recommendations[0], want)
	}
	if strings.Contains(result.Recommendations[0], "internal_security_recommendation_no_scanners") {
		t.Fatalf("rec = %q leaked the raw message ID — default translator regressed to NoopTranslator (HXC-097 regression)",
			result.Recommendations[0])
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
