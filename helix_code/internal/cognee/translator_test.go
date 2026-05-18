// Unit tests for the internal/cognee package-level translator + tr()
// helper (CONST-046 round-148 §11.4 anti-bluff sweep, 2026-05-18).
//
// Paired-mutation test per §11.4: planted/unplanted Translator yields
// distinguishable output at every migrated call site. Mocks ALLOWED
// per CONST-050(A) (unit tests only).
package cognee

import (
	"context"
	"errors"
	"strings"
	"testing"

	cogneei18n "dev.helix.code/internal/cognee/i18n"
)

// sentinelTranslator returns "<TR:" + id + ">" so call-site tests can
// assert tr() actually went through Translator.T rather than returning
// a hardcoded literal that happened to match the bundle value.
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
	got := tr(context.Background(), "internal_cognee_service_not_initialized", nil)
	if got != "internal_cognee_service_not_initialized" {
		t.Fatalf("tr default = %q, want raw message ID (loud echo)", got)
	}
}

func TestTr_UsesInjectedTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	got := tr(context.Background(), "internal_cognee_content_cannot_be_empty", nil)
	if got != "<TR:internal_cognee_content_cannot_be_empty>" {
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

	got := tr(context.Background(), "internal_cognee_failed_process_knowledge", map[string]any{"Err": "x"})
	if got != "internal_cognee_failed_process_knowledge" {
		t.Fatalf("tr on err = %q, want raw message ID (no silent swallow)", got)
	}
}

func TestSetTranslator_NilResetsToNoop(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	SetTranslator(nil) // explicit reset
	defer resetTranslator(t)

	got := tr(context.Background(), "internal_cognee_query_cannot_be_empty", nil)
	if got != "internal_cognee_query_cannot_be_empty" {
		t.Fatalf("tr after nil-reset = %q, want raw ID (Noop restored)", got)
	}
}

// TestProcessKnowledge_NilService_GoesThroughTranslator is the call-site
// paired-mutation: with a sentinel translator wired, the migrated
// fmt.Errorf path MUST surface "<TR:internal_cognee_service_not_initialized>"
// — proving the literal was NOT hardcoded anywhere on the path. If a
// future refactor inlines the string, this test fails.
func TestProcessKnowledge_NilService_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	cm := &CogneeManager{service: nil}
	err := cm.ProcessKnowledge(context.Background(), "some content")
	if err == nil {
		t.Fatal("ProcessKnowledge with nil service returned no error")
	}
	if !strings.Contains(err.Error(), "<TR:internal_cognee_service_not_initialized>") {
		t.Fatalf("ProcessKnowledge(nil-service) error = %q, want sentinel-wrapped message ID — string bypassed tr()", err.Error())
	}
}

// TestProcessKnowledge_EmptyContent_GoesThroughTranslator is the
// counterpart paired-mutation for the empty-input guard. Verifies the
// migrated call site actually invokes Translator.T with the right
// message ID.
func TestProcessKnowledge_EmptyContent_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	// Build a CogneeManager whose service is non-nil but never
	// reached because content is empty. We don't construct a real
	// CogneeService (would need a backing HTTP server); a sentinel
	// pointer is enough since the empty-content guard returns before
	// any service method call.
	cm := &CogneeManager{service: &CogneeService{}}
	err := cm.ProcessKnowledge(context.Background(), "")
	if err == nil {
		t.Fatal("ProcessKnowledge with empty content returned no error")
	}
	if !strings.Contains(err.Error(), "<TR:internal_cognee_content_cannot_be_empty>") {
		t.Fatalf("ProcessKnowledge(empty) error = %q, want sentinel-wrapped message ID", err.Error())
	}
}

// TestSearchKnowledge_EmptyQuery_GoesThroughTranslator covers the
// query-guard call site through the same paired-mutation pattern.
func TestSearchKnowledge_EmptyQuery_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	cm := &CogneeManager{service: &CogneeService{}}
	_, err := cm.SearchKnowledge(context.Background(), "")
	if err == nil {
		t.Fatal("SearchKnowledge with empty query returned no error")
	}
	if !strings.Contains(err.Error(), "<TR:internal_cognee_query_cannot_be_empty>") {
		t.Fatalf("SearchKnowledge(empty) error = %q, want sentinel-wrapped message ID", err.Error())
	}
}

// TestProcessCode_EmptyCode_GoesThroughTranslator covers the code-guard
// call site through the same paired-mutation pattern.
func TestProcessCode_EmptyCode_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	cm := &CogneeManager{service: &CogneeService{}}
	err := cm.ProcessCode(context.Background(), "", "go")
	if err == nil {
		t.Fatal("ProcessCode with empty code returned no error")
	}
	if !strings.Contains(err.Error(), "<TR:internal_cognee_code_cannot_be_empty>") {
		t.Fatalf("ProcessCode(empty) error = %q, want sentinel-wrapped message ID", err.Error())
	}
}

// TestSetTranslator_AcceptsNoopExplicit confirms the public API
// allows an explicit NoopTranslator (used by tests + ad-hoc tools)
// without unexpected behaviour.
func TestSetTranslator_AcceptsNoopExplicit(t *testing.T) {
	resetTranslator(t)
	defer resetTranslator(t)

	SetTranslator(cogneei18n.NoopTranslator{})
	got := tr(context.Background(), "internal_cognee_failed_cognify", nil)
	if got != "internal_cognee_failed_cognify" {
		t.Fatalf("tr with explicit NoopTranslator = %q, want raw ID", got)
	}
}
