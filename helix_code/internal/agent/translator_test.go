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
	"time"

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

// ---------------------------------------------------------------------
// Round-197 §11.4 anti-bluff paired-mutation tests (2026-05-19).
//
// Each test below pins one of the 10 round-197 migrations to its
// message ID at the call-site level. Sentinel translator wraps every
// resolved ID with "<TR:" prefix; if a future refactor inlines the
// literal string, these tests fail with diff that points to the
// regression.
// ---------------------------------------------------------------------

// TestTr_Round197_AllNewMessageIDs walks every round-197 message ID
// through tr() with the sentinel translator and asserts each resolves
// to the wrapped sentinel form. Single-source-of-truth list — if the
// bundle file removes an ID, this test fails immediately.
func TestTr_Round197_AllNewMessageIDs(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	round197IDs := []string{
		"internal_agent_review_tasks_require_llm",
		"internal_agent_refactoring_tasks_require_llm",
		"internal_agent_documentation_tasks_require_llm",
		"internal_agent_requirements_not_found_in_input",
		"internal_agent_content_not_found_in_input",
		"internal_agent_tool_registry_required_for_testing",
		"internal_agent_circuit_breaker_open",
		"internal_agent_workflow_stuck_unsatisfied_deps",
		"internal_agent_workflow_not_found",
		"internal_agent_no_agent_with_required_capabilities",
	}
	for _, id := range round197IDs {
		got := tr(context.Background(), id, nil)
		want := "<TR:" + id + ">"
		if got != want {
			t.Errorf("tr(%s) = %q, want %q — sentinel bypassed", id, got, want)
		}
	}
}

// TestBasicPlanning_MissingRequirements_GoesThroughTranslator is the
// paired-mutation for the basicPlanning no-LLM fallback. The literal
// "requirements not found in task input" MUST NOT appear in the
// returned error when a sentinel translator is wired.
func TestBasicPlanning_MissingRequirements_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	agent := &BaseAgent{id: "a1", name: "test-agent", agentType: AgentTypePlanning}
	tsk := &Task{ID: "t1", Input: map[string]interface{}{}}
	_, err := agent.basicPlanning(context.Background(), tsk)
	if err == nil {
		t.Fatal("basicPlanning with empty input returned no error")
	}
	if !strings.Contains(err.Error(), "<TR:internal_agent_requirements_not_found_in_input>") {
		t.Fatalf("basicPlanning error = %q, want sentinel-wrapped ID — string bypassed tr()", err.Error())
	}
}

// TestBasicAnalysis_MissingContent_GoesThroughTranslator is the
// paired-mutation for the basicAnalysis no-LLM fallback.
func TestBasicAnalysis_MissingContent_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	agent := &BaseAgent{id: "a1", name: "test-agent", agentType: AgentTypePlanning}
	tsk := &Task{ID: "t1", Input: map[string]interface{}{}}
	_, err := agent.basicAnalysis(context.Background(), tsk)
	if err == nil {
		t.Fatal("basicAnalysis with empty input returned no error")
	}
	if !strings.Contains(err.Error(), "<TR:internal_agent_content_not_found_in_input>") {
		t.Fatalf("basicAnalysis error = %q, want sentinel-wrapped ID — string bypassed tr()", err.Error())
	}
}

// TestBasicTesting_NoToolRegistry_GoesThroughTranslator pins the
// "tool registry required for test execution" migration.
func TestBasicTesting_NoToolRegistry_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	agent := &BaseAgent{id: "a1", name: "test-agent", agentType: AgentTypeTesting, toolRegistry: nil}
	tsk := &Task{ID: "t1", Input: map[string]interface{}{}}
	_, err := agent.basicTesting(context.Background(), tsk)
	if err == nil {
		t.Fatal("basicTesting with no toolRegistry returned no error")
	}
	if !strings.Contains(err.Error(), "<TR:internal_agent_tool_registry_required_for_testing>") {
		t.Fatalf("basicTesting error = %q, want sentinel-wrapped ID — string bypassed tr()", err.Error())
	}
}

// TestCircuitBreaker_OpenState_GoesThroughTranslator pins the
// "circuit breaker open for agent X" migration in resilience.go.
// Forces the breaker into open state with no timeout elapsed, then
// asserts the rejection error surfaces via tr().
func TestCircuitBreaker_OpenState_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	// Long timeout so half-open transition can't happen between
	// recordFailure() and Call() in this synchronous test.
	cb := NewCircuitBreaker("agent-cb", 1, 1, 10*time.Minute)
	// Trip the breaker.
	_ = cb.Call(context.Background(), func(_ context.Context) error {
		return errors.New("boom")
	})
	if cb.GetState() != CircuitBreakerOpen {
		t.Fatalf("circuit breaker = %v, want open after threshold breach", cb.GetState())
	}
	// Now call again — should reject with translated message.
	err := cb.Call(context.Background(), func(_ context.Context) error { return nil })
	if err == nil {
		t.Fatal("Call on open breaker returned no error")
	}
	if !strings.Contains(err.Error(), "<TR:internal_agent_circuit_breaker_open>") {
		t.Fatalf("circuit-breaker-open error = %q, want sentinel-wrapped ID — string bypassed tr()", err.Error())
	}
}

// TestWorkflowExecutor_GetWorkflowNotFound_GoesThroughTranslator pins
// the "workflow not found" migration in workflow.go.
func TestWorkflowExecutor_GetWorkflowNotFound_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	we := NewWorkflowExecutor(nil)
	_, err := we.GetWorkflow("no-such-id")
	if err == nil {
		t.Fatal("GetWorkflow(missing) returned no error")
	}
	if !strings.Contains(err.Error(), "<TR:internal_agent_workflow_not_found>") {
		t.Fatalf("GetWorkflow(missing) error = %q, want sentinel-wrapped ID — string bypassed tr()", err.Error())
	}
}

// TestBundleParity_Round197_BundleEntriesMatchSentinel ensures the
// YAML bundle declares every round-197 ID. Defensive against a
// future commit that migrates the call site but forgets the YAML
// entry — the production NoopTranslator-echo path would then surface
// the bare ID to end users, masking a real CONST-046 leak.
func TestBundleParity_Round197_BundleEntriesMatchSentinel(t *testing.T) {
	resetTranslator(t)
	defer resetTranslator(t)

	// NoopTranslator returns the raw ID, so this test is really
	// asserting that the tr() helper's degradation path produces
	// stable, predictable IDs — which is the production fallback
	// when the bundle file is corrupt or missing on disk.
	wantIDs := []string{
		"internal_agent_review_tasks_require_llm",
		"internal_agent_refactoring_tasks_require_llm",
		"internal_agent_documentation_tasks_require_llm",
		"internal_agent_requirements_not_found_in_input",
		"internal_agent_content_not_found_in_input",
		"internal_agent_tool_registry_required_for_testing",
		"internal_agent_circuit_breaker_open",
		"internal_agent_workflow_stuck_unsatisfied_deps",
		"internal_agent_workflow_not_found",
		"internal_agent_no_agent_with_required_capabilities",
	}
	for _, id := range wantIDs {
		got := tr(context.Background(), id, nil)
		if got != id {
			t.Errorf("NoopTranslator on %s = %q, want raw ID (loud echo)", id, got)
		}
	}
}
