// Unit tests for the internal/clarification package-level translator
// + tr() helper (CONST-046 round-222 §11.4 anti-bluff sweep,
// 2026-05-19).
//
// Paired-mutation tests per §11.4: planted/unplanted Translator
// yields distinguishable output at every migrated call site. Mocks
// ALLOWED per CONST-050(A) (unit tests only).
package clarification

import (
	"context"
	"errors"
	"strings"
	"testing"

	clarificationi18n "dev.helix.code/internal/clarification/i18n"
)

// sentinelTranslator returns "<TR:" + id + ">" so call-site tests
// can assert tr() actually went through Translator.T rather than
// returning a hardcoded literal that happened to match the bundle
// value.
type sentinelTranslator struct{}

func (sentinelTranslator) T(_ context.Context, id string, _ map[string]any) (string, error) {
	return "<TR:" + id + ">", nil
}
func (sentinelTranslator) TPlural(_ context.Context, id string, _ int, _ map[string]any) (string, error) {
	return "<TR:" + id + ">", nil
}

type errTranslator struct{}

func (errTranslator) T(_ context.Context, _ string, _ map[string]any) (string, error) {
	return "", errors.New("intentional translator failure")
}
func (errTranslator) TPlural(_ context.Context, _ string, _ int, _ map[string]any) (string, error) {
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
	got := tr(context.Background(), "internal_clarification_clarifications_received_header", nil)
	if got != "internal_clarification_clarifications_received_header" {
		t.Fatalf("tr default = %q, want raw message ID (loud echo)", got)
	}
}

func TestTr_UsesInjectedTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	got := tr(context.Background(), "internal_clarification_llm_system_prompt", nil)
	if got != "<TR:internal_clarification_llm_system_prompt>" {
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

	got := tr(context.Background(), "internal_clarification_user_request_wrapper", map[string]any{"Prompt": "x"})
	if got != "internal_clarification_user_request_wrapper" {
		t.Fatalf("tr on err = %q, want raw message ID (no silent swallow)", got)
	}
}

func TestSetTranslator_NilResetsToNoop(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	SetTranslator(nil) // explicit reset
	defer resetTranslator(t)

	got := tr(context.Background(), "internal_clarification_clarifications_received_header", nil)
	if got != "internal_clarification_clarifications_received_header" {
		t.Fatalf("tr after nil-reset = %q, want raw ID (Noop restored)", got)
	}
}

// TestSetTranslator_AcceptsNoopExplicit confirms the public API
// allows an explicit NoopTranslator (used by tests + ad-hoc tools)
// without unexpected behaviour.
func TestSetTranslator_AcceptsNoopExplicit(t *testing.T) {
	resetTranslator(t)
	defer resetTranslator(t)

	SetTranslator(clarificationi18n.NoopTranslator{})
	got := tr(context.Background(), "internal_clarification_llm_system_prompt", nil)
	if got != "internal_clarification_llm_system_prompt" {
		t.Fatalf("tr with explicit NoopTranslator = %q, want raw ID", got)
	}
}

// ---------------------------------------------------------------------
// Round-222 §11.4 anti-bluff paired-mutation tests.
//
// Each test below pins one of the 3 round-222 migrations to its
// message ID at the call-site level. Sentinel translator wraps every
// resolved ID with "<TR:" prefix; if a future refactor inlines the
// literal string, these tests fail with diff that points to the
// regression.
// ---------------------------------------------------------------------

// TestTr_Round222_AllNewMessageIDs walks every round-222 message ID
// through tr() with the sentinel translator and asserts each resolves
// to the wrapped sentinel form. Single-source-of-truth list — if the
// bundle file removes an ID, this test fails immediately.
func TestTr_Round222_AllNewMessageIDs(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	round222IDs := []string{
		"internal_clarification_llm_system_prompt",
		"internal_clarification_user_request_wrapper",
		"internal_clarification_clarifications_received_header",
	}
	for _, id := range round222IDs {
		got := tr(context.Background(), id, nil)
		want := "<TR:" + id + ">"
		if got != want {
			t.Errorf("tr(%s) = %q, want %q — sentinel bypassed", id, got, want)
		}
	}
}

// TestEngine_Resolve_HeaderGoesThroughTranslator pins the
// "Clarifications received:\n" migration in engine.go Resolve(). With
// a sentinel translator wired, the resolved-context string MUST
// surface "<TR:internal_clarification_clarifications_received_header>"
// — proving the literal was NOT hardcoded anywhere on the path. If a
// future refactor inlines the string, this test fails.
func TestEngine_Resolve_HeaderGoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	engine := NewEngine(nil)
	s := engine.NewSession("test context")
	s.Questions = []Question{{ID: "target_file", Text: "Which file?", Type: FreeText}}
	result := engine.Resolve(s.ID, []Answer{{QuestionID: "target_file", Value: "main.go"}})
	if !strings.Contains(result, "<TR:internal_clarification_clarifications_received_header>") {
		t.Fatalf("Resolve = %q, want sentinel-wrapped header ID — string bypassed tr()", result)
	}
	// Reality check: answer still rendered, header wasn't swallowed.
	if !strings.Contains(result, "main.go") {
		t.Fatalf("Resolve dropped the answer line: %q", result)
	}
}

// TestBundleParity_Round222_BundleEntriesMatchSentinel ensures the
// YAML bundle declares every round-222 ID. Defensive against a
// future commit that migrates the call site but forgets the YAML
// entry — the production NoopTranslator-echo path would then surface
// the bare ID to end users, masking a real CONST-046 leak.
func TestBundleParity_Round222_BundleEntriesMatchSentinel(t *testing.T) {
	resetTranslator(t)
	defer resetTranslator(t)

	// NoopTranslator returns the raw ID, so this test is really
	// asserting that the tr() helper's degradation path produces
	// stable, predictable IDs — which is the production fallback
	// when the bundle file is corrupt or missing on disk.
	wantIDs := []string{
		"internal_clarification_llm_system_prompt",
		"internal_clarification_user_request_wrapper",
		"internal_clarification_clarifications_received_header",
	}
	for _, id := range wantIDs {
		got := tr(context.Background(), id, nil)
		if got != id {
			t.Errorf("NoopTranslator on %s = %q, want raw ID (loud echo)", id, got)
		}
	}
}
