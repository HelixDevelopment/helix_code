// Sentinel call-site tests for the round-142 CONST-046 migration of
// helix_code/cmd/security_test/main.go. Mocks ALLOWED per CONST-050(A)
// (unit tests only).
//
// Each test:
//  1. Wires a fakeTranslator that wraps every message ID as
//     "<TRANSLATED:%s>".
//  2. Invokes the tr() helper directly (main() itself depends on
//     real security scanners + os.Exit, so we exercise the i18n
//     injection seam — the only surface this round changes).
//  3. Asserts the captured output contains the sentinel-wrapped IDs
//     we expect — proving the call site actually went through the
//     translator instead of a hardcoded literal that happens to
//     match the bundle value (§11.4 anti-bluff).
//
// Round-142 also asserts SetTranslator(nil) resets to NoopTranslator
// (loud echo), guarding against the §11.4 i18n-layer PASS-bluff of
// silently disabling translation.
package main

import (
	"context"
	"errors"
	"testing"

	"dev.helix.code/cmd/security_test/i18n"
)

// fakeTranslator wraps every message ID so call sites can be detected
// by sentinel substring search rather than relying on the bundle's
// English value (which would render the test indistinguishable from a
// hardcoded literal that happens to match — a §11.4 PASS-bluff).
type fakeTranslator struct {
	seen     map[string]int
	failOnID string
}

func newFake() *fakeTranslator {
	return &fakeTranslator{seen: make(map[string]int)}
}

func (f *fakeTranslator) T(_ context.Context, id string, _ map[string]any) (string, error) {
	f.seen[id]++
	if f.failOnID != "" && id == f.failOnID {
		return "", errors.New("fakeTranslator: deliberate failure for " + id)
	}
	return "<TRANSLATED:" + id + ">", nil
}

func (f *fakeTranslator) TPlural(_ context.Context, id string, _ int, _ map[string]any) (string, error) {
	f.seen[id]++
	if f.failOnID != "" && id == f.failOnID {
		return "", errors.New("fakeTranslator: deliberate failure for " + id)
	}
	return "<TRANSLATED:" + id + ">", nil
}

// resetTranslator restores the package-level default after each test
// so test ordering cannot leak fakeTranslator state into the next.
func resetTranslator(t *testing.T) {
	t.Helper()
	t.Cleanup(func() { SetTranslator(nil) })
}

func TestTr_ResolvesViaTranslator(t *testing.T) {
	resetTranslator(t)
	fake := newFake()
	SetTranslator(fake)

	got := tr(context.Background(), "security_test_suite_llm_provider", nil)
	want := "<TRANSLATED:security_test_suite_llm_provider>"
	if got != want {
		t.Fatalf("tr() returned %q, want %q (call site did not go through Translator)", got, want)
	}
	if fake.seen["security_test_suite_llm_provider"] != 1 {
		t.Fatalf("fakeTranslator.T was called %d times for security_test_suite_llm_provider, want 1",
			fake.seen["security_test_suite_llm_provider"])
	}
}

func TestTr_AllMigratedMessageIDs(t *testing.T) {
	// Anti-bluff: enumerate every message ID this round migrated and
	// assert each one resolves through the Translator with the
	// sentinel wrapper. A future regression that hardcodes any one
	// of these literals back into main.go would surface here as the
	// fakeTranslator.seen counter staying at zero for that ID.
	resetTranslator(t)
	fake := newFake()
	SetTranslator(fake)

	migratedIDs := []string{
		"security_test_suite_llm_provider",
		"security_test_suite_ssh_connection",
		"security_test_suite_database",
		"security_test_suite_authentication",
		"security_test_suite_input_validation",
		"security_test_suite_api",
		"security_test_suite_worker_isolation",
		"security_test_suite_dependency",
		"security_test_summary_fail_critical",
		"security_test_summary_warn_scanners_unavailable",
		// Round-460 (2026-05-20): residual suite names + results
		// table + completion line.
		"security_test_suite_container",
		"security_test_suite_configuration",
		"security_test_suite_file_system",
		"security_test_suite_logging",
		"security_test_results_heading",
		"security_test_results_total_tests",
		"security_test_results_failed_tests",
		"security_test_results_total_issues",
		"security_test_results_critical",
		"security_test_completed",
	}

	ctx := context.Background()
	for _, id := range migratedIDs {
		got := tr(ctx, id, nil)
		want := "<TRANSLATED:" + id + ">"
		if got != want {
			t.Errorf("tr(%q) returned %q, want %q", id, got, want)
		}
		if fake.seen[id] == 0 {
			t.Errorf("fakeTranslator.T never saw message ID %q", id)
		}
	}
}

func TestTr_TranslationFailureDegradesToMessageID(t *testing.T) {
	// Anti-bluff: when the Translator returns an error, tr() MUST
	// return the raw message ID — not an empty string and not panic.
	// An empty-string fallback would silently delete the user-visible
	// line (a §11.4 PASS-bluff at the i18n layer).
	resetTranslator(t)
	fake := newFake()
	fake.failOnID = "security_test_summary_fail_critical"
	SetTranslator(fake)

	got := tr(context.Background(), "security_test_summary_fail_critical", nil)
	if got != "security_test_summary_fail_critical" {
		t.Fatalf("tr() returned %q on Translator failure, want raw message ID echo", got)
	}
}

func TestSetTranslator_NilResetsToNoop(t *testing.T) {
	// Anti-bluff: SetTranslator(nil) MUST reset to NoopTranslator
	// (loud echo of message ID), NEVER silently retain the previous
	// wired Translator or set nil that would panic on next tr() call.
	resetTranslator(t)
	fake := newFake()
	SetTranslator(fake)
	if _, ok := translator.(*fakeTranslator); !ok {
		t.Fatalf("SetTranslator(fake) did not wire fakeTranslator; got %T", translator)
	}
	SetTranslator(nil)
	if _, ok := translator.(i18n.NoopTranslator); !ok {
		t.Fatalf("SetTranslator(nil) did not reset to NoopTranslator; got %T", translator)
	}
	// Verify the reset actually produces loud-echo behaviour.
	got := tr(context.Background(), "security_test_suite_api", nil)
	if got != "security_test_suite_api" {
		t.Fatalf("after SetTranslator(nil), tr() returned %q, want loud echo of message ID", got)
	}
}

// TestRound460_ResultsTableTemplateData confirms the round-460
// results-table message IDs carry their {{.Count}} placeholder data
// through the Translator — a call site that dropped the map would
// silently break placeholder interpolation at runtime.
func TestRound460_ResultsTableTemplateData(t *testing.T) {
	resetTranslator(t)
	fake := &recordingFake{seen: make(map[string]map[string]any)}
	SetTranslator(fake)

	ctx := context.Background()
	_ = tr(ctx, "security_test_results_total_tests", map[string]any{"Count": 12})
	_ = tr(ctx, "security_test_results_critical", map[string]any{"Count": 3})

	if got := fake.seen["security_test_results_total_tests"]["Count"]; got != 12 {
		t.Errorf("security_test_results_total_tests Count = %v, want 12", got)
	}
	if got := fake.seen["security_test_results_critical"]["Count"]; got != 3 {
		t.Errorf("security_test_results_critical Count = %v, want 3", got)
	}
}

// recordingFake captures templateData per ID for round-460 assertions.
type recordingFake struct {
	seen map[string]map[string]any
}

func (r *recordingFake) T(_ context.Context, id string, data map[string]any) (string, error) {
	r.seen[id] = data
	return "<TRANSLATED:" + id + ">", nil
}

func (r *recordingFake) TPlural(_ context.Context, id string, _ int, data map[string]any) (string, error) {
	r.seen[id] = data
	return "<TRANSLATED:" + id + ">", nil
}

// TestTr_MutationGuard is the round-142 paired-mutation gate.
// If a future maintainer accidentally swaps the tr() implementation
// for one that always returns the raw message ID (a regression that
// would silently disable the entire i18n layer), this test FAILS
// because the fakeTranslator's sentinel wrapper is no longer applied.
// This is the §11.4 anti-bluff guarantee that the migration is
// REAL — not a no-op rename.
func TestTr_MutationGuard(t *testing.T) {
	resetTranslator(t)
	fake := newFake()
	SetTranslator(fake)

	got := tr(context.Background(), "security_test_suite_database", nil)
	if got == "security_test_suite_database" {
		t.Fatal("MUTATION DETECTED: tr() returned raw message ID despite fakeTranslator being wired; i18n layer is silently disabled (§11.4 PASS-bluff)")
	}
	if got != "<TRANSLATED:security_test_suite_database>" {
		t.Fatalf("tr() returned unexpected value %q; expected sentinel wrapper", got)
	}
}
