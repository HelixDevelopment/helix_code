// Unit tests for the internal/agent package-level translator + tr()
// helper (CONST-046 round-147 §11.4 anti-bluff sweep, 2026-05-18).
//
// Paired-mutation test per §11.4: planted/unplanted Translator yields
// distinguishable output at every migrated call site. Mocks ALLOWED
// per CONST-050(A) (unit tests only).
package agent

import (
	"context"
	"errors"
	"strings"
	"testing"

	agenti18n "dev.helix.code/internal/agent/i18n"
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
	got := tr(context.Background(), "internal_agent_task_cannot_be_nil", nil)
	if got != "internal_agent_task_cannot_be_nil" {
		t.Fatalf("tr default = %q, want raw message ID (loud echo)", got)
	}
}

func TestTr_UsesInjectedTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	got := tr(context.Background(), "internal_agent_no_available_agent_found", nil)
	if got != "<TR:internal_agent_no_available_agent_found>" {
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

	got := tr(context.Background(), "internal_agent_result_not_found", map[string]any{"TaskID": "x"})
	if got != "internal_agent_result_not_found" {
		t.Fatalf("tr on err = %q, want raw message ID (no silent swallow)", got)
	}
}

func TestSetTranslator_NilResetsToNoop(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	SetTranslator(nil) // explicit reset
	defer resetTranslator(t)

	got := tr(context.Background(), "internal_agent_workflow_executor_not_initialized", nil)
	if got != "internal_agent_workflow_executor_not_initialized" {
		t.Fatalf("tr after nil-reset = %q, want raw ID (Noop restored)", got)
	}
}

// TestCoordinator_SubmitNilTask_GoesThroughTranslator is the call-site
// paired-mutation: with a sentinel translator wired, the migrated
// fmt.Errorf path MUST surface "<TR:internal_agent_task_cannot_be_nil>"
// — proving the literal was NOT hardcoded anywhere on the path. If a
// future refactor inlines the string, this test fails.
func TestCoordinator_SubmitNilTask_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	coord := NewCoordinator(nil)
	err := coord.SubmitTask(context.Background(), nil)
	if err == nil {
		t.Fatal("SubmitTask(nil) returned no error")
	}
	if !strings.Contains(err.Error(), "<TR:internal_agent_task_cannot_be_nil>") {
		t.Fatalf("SubmitTask(nil) error = %q, want sentinel-wrapped message ID — string bypassed tr()", err.Error())
	}
}

// TestCoordinator_ExecuteTaskNotFound_GoesThroughTranslator is the
// counterpart paired-mutation for the task-not-found path. Verifies
// the migrated coordinator.ExecuteTask call site actually invokes
// Translator.T with the right message ID.
func TestCoordinator_ExecuteTaskNotFound_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	coord := NewCoordinator(nil)
	_, err := coord.ExecuteTask(context.Background(), "missing-id")
	if err == nil {
		t.Fatal("ExecuteTask(missing) returned no error")
	}
	if !strings.Contains(err.Error(), "<TR:internal_agent_task_not_found>") {
		t.Fatalf("ExecuteTask(missing) error = %q, want sentinel-wrapped message ID", err.Error())
	}
}

// TestSetTranslator_AcceptsNoopExplicit confirms the public API
// allows an explicit NoopTranslator (used by tests + ad-hoc tools)
// without unexpected behaviour.
func TestSetTranslator_AcceptsNoopExplicit(t *testing.T) {
	resetTranslator(t)
	defer resetTranslator(t)

	SetTranslator(agenti18n.NoopTranslator{})
	got := tr(context.Background(), "internal_agent_code_tasks_require_llm", nil)
	if got != "internal_agent_code_tasks_require_llm" {
		t.Fatalf("tr with explicit NoopTranslator = %q, want raw ID", got)
	}
}
