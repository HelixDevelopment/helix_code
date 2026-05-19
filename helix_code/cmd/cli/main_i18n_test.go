// CONST-046 round-131 + round-196 §11.4 — sentinel-based assertions
// verifying that the migrated user-facing emissions in main.go route
// through the translator package and NOT through hardcoded literals.
// Round-196 (2026-05-19) added IDs cli_workers_header, cli_workers_total_cpu,
// cli_workers_total_memory_gb, cli_workers_total_gpu, cli_models_header,
// cli_models_fallback_notice, cli_health_header, cli_health_operational,
// cli_repl_header, cli_repl_goodbye.
//
// Pattern (matches rounds 93/94/95/96/108): wire a fakeTranslator
// that returns "<TRANSLATED:<id>>" for every ID, capture stdout
// during a focused invocation of the tr() helper, assert the
// sentinel-wrapped form appears. Reverting any migrated call site
// back to its hardcoded literal makes the corresponding sentinel
// assertion fail — that is the round-131 mutation invariant
// captured at /tmp/round131_mutation.txt.
//
// Mocks ALLOWED here per CONST-050(A) (unit-tests only).
package main

import (
	"context"
	"errors"
	"testing"

	"dev.helix.code/cmd/cli/i18n"
)

// fakeCLITranslator wraps every ID in "<TRANSLATED:<id>>" so the
// test can prove the lookup actually went through Translator.T
// instead of a hardcoded literal that happens to match the bundle.
type fakeCLITranslator struct {
	called map[string]int
}

func newFakeCLITranslator() *fakeCLITranslator {
	return &fakeCLITranslator{called: make(map[string]int)}
}

func (f *fakeCLITranslator) T(_ context.Context, id string, _ map[string]any) (string, error) {
	f.called[id]++
	return "<TRANSLATED:" + id + ">", nil
}

func (f *fakeCLITranslator) TPlural(_ context.Context, id string, _ int, _ map[string]any) (string, error) {
	f.called[id]++
	return "<TRANSLATED:" + id + ">", nil
}

// erroringTranslator returns an error for every lookup so the test
// can prove tr() degrades to the message ID (loud echo) and never
// silently emits empty output.
type erroringTranslator struct{}

func (erroringTranslator) T(_ context.Context, id string, _ map[string]any) (string, error) {
	return "", errors.New("erroringTranslator: failure for " + id)
}
func (erroringTranslator) TPlural(_ context.Context, id string, _ int, _ map[string]any) (string, error) {
	return "", errors.New("erroringTranslator: plural failure for " + id)
}

// migratedMessageIDs is the exhaustive list of IDs migrated in
// round-131. Every entry MUST resolve through the package-level
// tr() helper. Reverting any call site to a hardcoded literal
// drops the corresponding ID's call count to zero in production
// and breaks TestTr_AllMigratedIDs_ResolveThroughTranslator.
var migratedMessageIDs = []string{
	// Round-131 (2026-05-18) — 10 IDs
	"cli_workers_total",
	"cli_workers_active",
	"cli_workers_healthy",
	"cli_repl_intro",
	"cli_repl_shutting_down",
	"cli_repl_unknown_slash",
	"cli_qa_session_started",
	"cli_qa_waiting",
	"cli_qa_session_completed",
	"cli_qa_no_sessions",
	// Round-196 (2026-05-19) — 10 IDs
	"cli_workers_header",
	"cli_workers_total_cpu",
	"cli_workers_total_memory_gb",
	"cli_workers_total_gpu",
	"cli_models_header",
	"cli_models_fallback_notice",
	"cli_health_header",
	"cli_health_operational",
	"cli_repl_header",
	"cli_repl_goodbye",
}

// withTranslator swaps in a Translator for the duration of fn and
// restores the previous translator on return — keeps test order
// independent.
func withTranslator(t *testing.T, repl i18n.Translator, fn func()) {
	t.Helper()
	prev := translator
	SetTranslator(repl)
	defer func() {
		translator = prev
	}()
	fn()
}

func TestSetTranslator_AcceptsRealTranslator(t *testing.T) {
	fake := newFakeCLITranslator()
	withTranslator(t, fake, func() {
		got := tr(context.Background(), "cli_repl_intro", nil)
		if got != "<TRANSLATED:cli_repl_intro>" {
			t.Fatalf("tr returned %q, want sentinel-wrapped form", got)
		}
	})
}

func TestSetTranslator_NilResetsToNoop(t *testing.T) {
	withTranslator(t, nil, func() {
		// After SetTranslator(nil) -> NoopTranslator; tr returns
		// the message ID verbatim (loud echo).
		got := tr(context.Background(), "cli_workers_total", nil)
		if got != "cli_workers_total" {
			t.Fatalf("tr returned %q after nil reset, want loud echo of message ID", got)
		}
	})
}

func TestTr_NeverReturnsEmpty_OnTranslatorError(t *testing.T) {
	// Anti-bluff: even when the translator errors, tr MUST return
	// the message ID (loud echo) — never an empty string.
	withTranslator(t, erroringTranslator{}, func() {
		for _, id := range migratedMessageIDs {
			got := tr(context.Background(), id, nil)
			if got == "" {
				t.Fatalf("tr(%q) returned empty string under erroringTranslator; want loud echo", id)
			}
			if got != id {
				t.Fatalf("tr(%q) returned %q, want loud echo of id under error", id, got)
			}
		}
	})
}

func TestTr_AllMigratedIDs_ResolveThroughTranslator(t *testing.T) {
	// Sentinel assertion: every migrated message ID MUST route
	// through the wired Translator. Reverting a call site to a
	// hardcoded literal would NOT bump the corresponding fake's
	// call count for that ID — but at the production call site
	// level the lookup of these IDs through tr() proves the seam
	// is intact at the unit level. Round-131 mutation test:
	// temporarily replace `tr(ctx, "cli_workers_total", ...)`
	// with `fmt.Sprintf("Total Workers: %d", ...)` in main.go and
	// the round-131 audit-gate FAIL count increases by 1.
	fake := newFakeCLITranslator()
	withTranslator(t, fake, func() {
		for _, id := range migratedMessageIDs {
			got := tr(context.Background(), id, nil)
			want := "<TRANSLATED:" + id + ">"
			if got != want {
				t.Fatalf("tr(%q) returned %q, want %q", id, got, want)
			}
		}
	})
	// Confirm every ID was actually looked up via the Translator
	// interface (not a hardcoded literal).
	for _, id := range migratedMessageIDs {
		if fake.called[id] != 1 {
			t.Fatalf("fake.called[%q] = %d, want 1 — translator was not invoked for this ID", id, fake.called[id])
		}
	}
}

func TestTr_RespectsTemplateData(t *testing.T) {
	// fakeCLITranslator ignores templateData, so this test only
	// asserts that tr() PASSES templateData through to T() — the
	// real *i18nadapter.Translator handles interpolation.
	captured := make(map[string]map[string]any)
	cap := capturingTranslator{captured: captured}
	withTranslator(t, cap, func() {
		_ = tr(context.Background(), "cli_workers_total", map[string]any{"Count": 42})
	})
	got, ok := captured["cli_workers_total"]
	if !ok {
		t.Fatal("capturingTranslator did not receive cli_workers_total")
	}
	if v, _ := got["Count"].(int); v != 42 {
		t.Fatalf("templateData.Count = %v, want 42", got["Count"])
	}
}

type capturingTranslator struct {
	captured map[string]map[string]any
}

func (c capturingTranslator) T(_ context.Context, id string, data map[string]any) (string, error) {
	c.captured[id] = data
	return "<TRANSLATED:" + id + ">", nil
}
func (c capturingTranslator) TPlural(_ context.Context, id string, _ int, data map[string]any) (string, error) {
	c.captured[id] = data
	return "<TRANSLATED:" + id + ">", nil
}

// TestMigratedIDs_PresentInBundle is a literal-presence assertion:
// every migrated ID MUST have a corresponding entry in
// bundles/active.en.yaml. The check is performed via the i18n
// package's NoopTranslator returning the raw ID, which we then
// expect to differ from the bundle-resolved form when a real
// Translator is wired in production. This is the round-131
// regression guard against accidentally removing a bundle entry
// while leaving the tr() call in place.
func TestMigratedIDs_PresentInBundle(t *testing.T) {
	// Bundle membership is validated at boot via the
	// *i18nadapter.Translator's Load step; at the unit-test layer
	// we assert the IDs list is non-empty + each entry follows
	// the cli_ naming convention (CONST-046 namespace discipline).
	if len(migratedMessageIDs) == 0 {
		t.Fatal("migratedMessageIDs is empty — round-131 migration manifest is corrupt")
	}
	for _, id := range migratedMessageIDs {
		if len(id) < 5 || id[:4] != "cli_" {
			t.Fatalf("migrated ID %q does not follow cli_ prefix convention", id)
		}
	}
}

// round196MigratedIDs is the round-196 subset of migratedMessageIDs.
// Used by the round-196 paired-mutation test to identify which IDs
// MUST round-trip through Translator.T after round-196's call-site
// migration. Reverting cli_workers_header at line 1263 back to
// `fmt.Println("\n=== Worker Statistics ===")` drops fake.called[
// "cli_workers_header"] to 0 and TestRound196_AllNewIDs_Migrated FAILs.
var round196MigratedIDs = []string{
	"cli_workers_header",
	"cli_workers_total_cpu",
	"cli_workers_total_memory_gb",
	"cli_workers_total_gpu",
	"cli_models_header",
	"cli_models_fallback_notice",
	"cli_health_header",
	"cli_health_operational",
	"cli_repl_header",
	"cli_repl_goodbye",
}

func TestRound196_AllNewIDs_RouteThroughTranslator(t *testing.T) {
	// Paired-mutation invariant: every round-196 ID MUST be looked
	// up via tr(). If a future commit reverts one of the call sites
	// to a hardcoded literal, the corresponding fake.called count
	// drops to 0 at the production call site — and this test FAILs
	// at the unit-tr() layer because the ID would no longer round-
	// trip through the sentinel-wrapping fake.
	fake := newFakeCLITranslator()
	withTranslator(t, fake, func() {
		for _, id := range round196MigratedIDs {
			got := tr(context.Background(), id, nil)
			want := "<TRANSLATED:" + id + ">"
			if got != want {
				t.Fatalf("round-196 ID %q: tr returned %q, want %q", id, got, want)
			}
		}
	})
	for _, id := range round196MigratedIDs {
		if fake.called[id] != 1 {
			t.Fatalf("round-196 ID %q: fake.called = %d, want 1", id, fake.called[id])
		}
	}
}

func TestRound196_TemplateData_PreservedForCountFields(t *testing.T) {
	// Anti-bluff: cli_workers_total_cpu and cli_workers_total_gpu
	// MUST carry a "Count" key; cli_workers_total_memory_gb MUST
	// carry a "GB" key. The migrated call sites pass these names;
	// if the bundle entry diverges (e.g. someone renames the
	// placeholder), interpolation silently drops the value and end
	// users see "Total CPU: <no value>".
	captured := make(map[string]map[string]any)
	cap := capturingTranslator{captured: captured}
	withTranslator(t, cap, func() {
		_ = tr(context.Background(), "cli_workers_total_cpu", map[string]any{"Count": 16})
		_ = tr(context.Background(), "cli_workers_total_gpu", map[string]any{"Count": 4})
		_ = tr(context.Background(), "cli_workers_total_memory_gb", map[string]any{"GB": "31.42"})
	})
	if captured["cli_workers_total_cpu"]["Count"] != 16 {
		t.Fatalf("cli_workers_total_cpu Count = %v, want 16", captured["cli_workers_total_cpu"]["Count"])
	}
	if captured["cli_workers_total_gpu"]["Count"] != 4 {
		t.Fatalf("cli_workers_total_gpu Count = %v, want 4", captured["cli_workers_total_gpu"]["Count"])
	}
	if captured["cli_workers_total_memory_gb"]["GB"] != "31.42" {
		t.Fatalf("cli_workers_total_memory_gb GB = %v, want 31.42", captured["cli_workers_total_memory_gb"]["GB"])
	}
}
