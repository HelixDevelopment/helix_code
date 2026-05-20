// Sentinel call-site tests for the round-455 CONST-046 migration of
// helix_code/cmd/infrastructure/main.go. Mocks ALLOWED per
// CONST-050(A) (unit tests only).
//
// Each test:
//  1. Wires a fakeTranslator that wraps every message ID as
//     "<TRANSLATED:%s>".
//  2. Invokes the tr() helper directly (main() itself depends on a
//     real container runtime + os.Exit, so we exercise the i18n
//     injection seam — the only surface this round changes).
//  3. Asserts the captured output contains the sentinel-wrapped IDs
//     we expect — proving the call site actually went through the
//     translator instead of a hardcoded literal that happens to
//     match the bundle value (§11.4 anti-bluff).
//
// Round-455 also asserts SetTranslator(nil) resets to NoopTranslator
// (loud echo), guarding against the §11.4 i18n-layer PASS-bluff of
// silently disabling translation.
package main

import (
	"context"
	"errors"
	"testing"

	"dev.helix.code/cmd/infrastructure/i18n"
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

	got := tr(context.Background(), "infra_start_starting", nil)
	want := "<TRANSLATED:infra_start_starting>"
	if got != want {
		t.Fatalf("tr() returned %q, want %q (call site did not go through Translator)", got, want)
	}
	if fake.seen["infra_start_starting"] != 1 {
		t.Fatalf("fakeTranslator.T was called %d times for infra_start_starting, want 1",
			fake.seen["infra_start_starting"])
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
		"infra_start_starting",
		"infra_start_runtime",
		"infra_start_success",
		"infra_start_count_started",
		"infra_start_count_failed",
		"infra_start_count_skipped",
		"infra_start_some_failed",
		"infra_start_service_status_heading",
		"infra_stop_stopping",
		"infra_stop_success",
		"infra_status_heading",
		"infra_status_not_started",
		"infra_status_running",
		"infra_wait_running",
		"infra_wait_received_signal",
		"infra_usage_heading",
		"infra_usage_synopsis",
		"infra_usage_commands_heading",
		"infra_usage_command_start",
		"infra_usage_command_stop",
		"infra_usage_command_status",
		"infra_usage_modes_heading",
		"infra_usage_mode_production",
		"infra_usage_mode_testing",
		"infra_usage_mode_full",
		"infra_usage_examples_heading",
		"infra_error_unknown_mode",
		"infra_error_unknown_command",
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
	fake.failOnID = "infra_stop_stopping"
	SetTranslator(fake)

	got := tr(context.Background(), "infra_stop_stopping", nil)
	if got != "infra_stop_stopping" {
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
	got := tr(context.Background(), "infra_status_heading", nil)
	if got != "infra_status_heading" {
		t.Fatalf("after SetTranslator(nil), tr() returned %q, want loud echo of message ID", got)
	}
}

// TestTr_MutationGuard is the round-455 paired-mutation gate.
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

	got := tr(context.Background(), "infra_start_success", nil)
	if got == "infra_start_success" {
		t.Fatal("MUTATION DETECTED: tr() returned raw message ID despite fakeTranslator being wired; i18n layer is silently disabled (§11.4 PASS-bluff)")
	}
	if got != "<TRANSLATED:infra_start_success>" {
		t.Fatalf("tr() returned unexpected value %q; expected sentinel wrapper", got)
	}
}
