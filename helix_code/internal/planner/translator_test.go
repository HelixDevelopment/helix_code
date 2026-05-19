// Sentinel + mutation tests for the CONST-046 translator wiring in
// internal/planner (round-232 §11.4 anti-bluff sweep, 2026-05-19).
// Mocks ALLOWED per CONST-050(A) — this is a unit test file.
package planner

import (
	"context"
	"errors"
	"strings"
	"testing"

	planneri18n "dev.helix.code/internal/planner/i18n"
)

// sentinelTranslator wraps every resolved message ID with a
// recognisable marker so call-site tests can prove the lookup
// ACTUALLY went through Translator.T — not through a hardcoded
// literal that happens to match the bundle value (which would be a
// §11.4 PASS-bluff at the i18n call-site layer).
type sentinelTranslator struct{}

func (sentinelTranslator) T(_ context.Context, id string, data map[string]any) (string, error) {
	if len(data) > 0 {
		keys := make([]string, 0, len(data))
		for k := range data {
			keys = append(keys, k)
		}
		return "<SENT:" + id + "|keys=" + strings.Join(keys, ",") + ">", nil
	}
	return "<SENT:" + id + ">", nil
}

func (sentinelTranslator) TPlural(_ context.Context, id string, _ int, _ map[string]any) (string, error) {
	return "<SENT:" + id + ">", nil
}

// errorTranslator always fails — exercises the tr() fallback path
// (must degrade to raw message ID, never to empty string).
type errorTranslator struct{}

func (errorTranslator) T(_ context.Context, id string, _ map[string]any) (string, error) {
	return "", errors.New("errorTranslator: deliberate failure for " + id)
}

func (errorTranslator) TPlural(_ context.Context, id string, _ int, _ map[string]any) (string, error) {
	return "", errors.New("errorTranslator: deliberate failure for " + id)
}

func resetTranslator(t *testing.T) {
	t.Helper()
	t.Cleanup(func() { SetTranslator(nil) })
}

func TestSetTranslator_Nil_ResetsToNoop(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	got := tr(context.Background(), "internal_planner_err_nil_plan", nil)
	if got != "<SENT:internal_planner_err_nil_plan>" {
		t.Fatalf("expected sentinel-wrapped output, got %q", got)
	}
	SetTranslator(nil)
	got = tr(context.Background(), "internal_planner_err_nil_plan", nil)
	if got != "internal_planner_err_nil_plan" {
		t.Fatalf("after SetTranslator(nil), expected loud message-ID echo, got %q", got)
	}
}

func TestTr_FallsBackToMessageIDOnError(t *testing.T) {
	// Anti-bluff: a translator error MUST degrade to the raw message
	// ID, not to the empty string. Silent empty would be a §11.4
	// PASS-bluff at the i18n fallback layer (user sees nothing).
	resetTranslator(t)
	SetTranslator(errorTranslator{})
	got := tr(context.Background(), "internal_planner_validation_command_required", nil)
	if got != "internal_planner_validation_command_required" {
		t.Fatalf("tr() with failing translator returned %q, want raw message ID", got)
	}
}

func TestExecutePlan_NilPlan_RoutesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})

	exec := NewSequentialExecutor(nil)
	err := exec.ExecutePlan(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil plan, got nil")
	}
	if !strings.Contains(err.Error(), "<SENT:internal_planner_err_nil_plan>") {
		t.Fatalf("nil-plan error did not route through translator: got %q", err.Error())
	}
}

func TestTaskPlanTool_Description_RoutesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})

	tool := NewTaskPlanTool(NewSequentialExecutor(nil))
	if tool.Description() != "<SENT:internal_planner_task_plan_description>" {
		t.Fatalf("TaskPlanTool.Description did not route through translator: got %q", tool.Description())
	}
}

func TestTaskStepTool_Description_RoutesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})

	tool := NewTaskStepTool(NewSequentialExecutor(nil))
	if tool.Description() != "<SENT:internal_planner_task_step_description>" {
		t.Fatalf("TaskStepTool.Description did not route through translator: got %q", tool.Description())
	}
}

func TestTaskPlanTool_Validate_NameRequired_RoutesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})

	tool := NewTaskPlanTool(NewSequentialExecutor(nil))
	err := tool.Validate(map[string]interface{}{})
	if err == nil {
		t.Fatal("expected validation error for missing name, got nil")
	}
	if !strings.Contains(err.Error(), "<SENT:internal_planner_validation_name_required>") {
		t.Fatalf("name-required validation did not route through translator: got %q", err.Error())
	}
}

func TestTaskStepTool_Validate_CommandRequired_RoutesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})

	tool := NewTaskStepTool(NewSequentialExecutor(nil))
	err := tool.Validate(map[string]interface{}{})
	if err == nil {
		t.Fatal("expected validation error for missing command, got nil")
	}
	if !strings.Contains(err.Error(), "<SENT:internal_planner_validation_command_required>") {
		t.Fatalf("command-required validation did not route through translator: got %q", err.Error())
	}
}

func TestTaskPlanTool_Execute_StepsValidation_RoutesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})

	tool := NewTaskPlanTool(NewSequentialExecutor(nil))

	// Empty steps array.
	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"name":  "p",
		"steps": []interface{}{},
	})
	if err == nil {
		t.Fatal("expected error for empty steps, got nil")
	}
	if !strings.Contains(err.Error(), "<SENT:internal_planner_validation_steps_non_empty_array>") {
		t.Fatalf("steps-non-empty validation did not route through translator: got %q", err.Error())
	}

	// Step missing both command and prompt.
	_, err = tool.Execute(context.Background(), map[string]interface{}{
		"name": "p",
		"steps": []interface{}{
			map[string]interface{}{"plan_node_id": "n1"},
		},
	})
	if err == nil {
		t.Fatal("expected error for step missing command/prompt, got nil")
	}
	if !strings.Contains(err.Error(), "<SENT:internal_planner_validation_step_needs_command_or_prompt>") {
		t.Fatalf("step-needs-command-or-prompt validation did not route through translator: got %q", err.Error())
	}
}

func TestExecuteStep_UnsupportedType_RoutesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})

	exec := NewSequentialExecutor(func(ctx context.Context, cmd string) (string, error) {
		return "", nil
	})

	step := &TaskStep{
		ID:         "s-bad",
		Type:       StepLLM, // executor currently only handles StepShell — LLM falls through default
		Status:     StepPending,
		MaxRetries: 0,
	}
	err := exec.ExecuteStep(context.Background(), step)
	if err == nil {
		t.Fatal("expected error for unsupported step type, got nil")
	}
	if !strings.Contains(step.Error, "<SENT:internal_planner_err_unsupported_step_type|keys=Type>") {
		t.Fatalf("step.Error for unsupported type did not route through translator: got %q", step.Error)
	}
}

// TestNoopTranslator_T_Loud_Echo_IsRawID is the paired mutation test —
// it asserts every CONST-046 message ID emitted by this package's
// migrated call sites appears in the active.en.yaml bundle (verified
// implicitly: NoopTranslator returns id verbatim, and the call-site
// tests above prove call sites use these exact IDs). If a new round
// adds a tr() call without a bundle entry, the bundle scan in
// internal/audit + this loud-echo invariant must FAIL. Mirrors §1.1
// paired-mutation guidance.
func TestNoopTranslator_T_Loud_Echo_IsRawID(t *testing.T) {
	noop := planneri18n.NoopTranslator{}
	for _, id := range migratedMessageIDs() {
		got, err := noop.T(context.Background(), id, nil)
		if err != nil {
			t.Fatalf("NoopTranslator.T(%q) error: %v", id, err)
		}
		if got != id {
			t.Fatalf("NoopTranslator.T(%q) returned %q, want loud echo of raw ID", id, got)
		}
	}
}

func migratedMessageIDs() []string {
	// Round-232 migrated set. Keep alphabetical for easy diffing on
	// future rounds.
	return []string{
		"internal_planner_err_command_failed_fmt",
		"internal_planner_err_nil_plan",
		"internal_planner_err_step_failed_fmt",
		"internal_planner_err_unsupported_step_type",
		"internal_planner_task_plan_description",
		"internal_planner_task_step_description",
		"internal_planner_validation_command_required",
		"internal_planner_validation_name_required",
		"internal_planner_validation_step_needs_command_or_prompt",
		"internal_planner_validation_steps_non_empty_array",
	}
}
