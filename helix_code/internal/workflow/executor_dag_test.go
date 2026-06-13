package workflow

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dev.helix.code/internal/project"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExecuteWorkflow_OutOfOrderDependency_DAG is the anti-bluff RED→GREEN proof
// for the G-2 Stream-A DAG integration (§11.4.115).
//
// It builds a Workflow whose Steps slice is in an order where stepB (which
// depends on stepA) appears BEFORE stepA in the slice. Each step runs a REAL
// command via os/exec (StepActionExecuteCommand) that appends its ID to a
// shared on-disk order file — so the asserted execution order is captured from
// the real execution path, not a fake.
//
// Expected (DAG, GREEN): both steps reach StepStatusCompleted, the workflow is
// WorkflowStatusCompleted, and the order file shows stepA ran BEFORE stepB
// (dependency order), even though stepB precedes stepA in the slice.
//
// Pre-fix (RED): the former slice-order loop evaluated stepB first, found its
// dependency stepA not yet completed (stepA appears later in the slice), and
// permanently marked stepB StepStatusSkipped — never revisiting it. The RED
// reproduction of that behaviour is asserted by TestSliceOrderLoop_RED below.
//
// RED_MODE polarity switch (§11.4.115): with RED_MODE=1 this test reproduces
// the OLD broken behaviour against the in-test reference slice-order loop and
// asserts the defect is PRESENT; with RED_MODE=0 (default) it exercises the
// real executeWorkflow DAG path and asserts the defect is ABSENT.
func TestExecuteWorkflow_OutOfOrderDependency_DAG(t *testing.T) {
	red := os.Getenv("RED_MODE") == "1"

	tempDir, err := os.MkdirTemp("", "dag_order_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	orderFile := filepath.Join(tempDir, "order.txt")

	// Steps in DELIBERATELY broken order: stepB (depends on stepA) is FIRST,
	// stepA is SECOND. Each command appends its id to orderFile via real os/exec.
	newWorkflow := func() *Workflow {
		return &Workflow{
			ID:     "out-of-order",
			Status: WorkflowStatusPending,
			Steps: []Step{
				{
					ID:           "stepB",
					Action:       StepActionExecuteCommand,
					Description:  "echo stepB >> " + orderFile,
					Dependencies: []string{"stepA"},
					Status:       StepStatusPending,
				},
				{
					ID:          "stepA",
					Action:      StepActionExecuteCommand,
					Description: "echo stepA >> " + orderFile,
					Status:      StepStatusPending,
				},
			},
		}
	}

	projectManager := project.NewManager()
	executor := NewExecutorWithLLM(projectManager, nil, &ExecutorConfig{MaxConcurrentSteps: 4})
	proj := &project.Project{Type: "generic", Path: tempDir}

	wf := newWorkflow()

	if red {
		// RED: reproduce the OLD slice-order loop behaviour against the same
		// inputs and assert the defect (stepB skipped, stepA never gating it)
		// is genuinely PRESENT on the pre-fix algorithm.
		simulateLegacySliceOrderLoop(context.Background(), executor, wf, proj)

		assert.Equal(t, StepStatusSkipped, wf.getStepStatus(0),
			"RED: legacy loop must SKIP stepB because its dep stepA appears later in the slice")
		// The order file must NOT contain stepB (it never ran).
		data, _ := os.ReadFile(orderFile)
		assert.NotContains(t, string(data), "stepB",
			"RED: stepB must never execute under the legacy loop")
		return
	}

	// GREEN: the real DAG-backed executor.
	executor.executeWorkflow(context.Background(), wf, proj)

	// Both steps completed.
	assert.Equal(t, WorkflowStatusCompleted, wf.GetStatus(),
		"workflow must complete: DAG resolves the out-of-order dependency")
	assert.Equal(t, StepStatusCompleted, wf.getStepStatus(0), "stepB must complete")
	assert.Equal(t, StepStatusCompleted, wf.getStepStatus(1), "stepA must complete")

	// Real execution order captured on disk: stepA BEFORE stepB.
	data, readErr := os.ReadFile(orderFile)
	require.NoError(t, readErr)
	lines := splitNonEmpty(string(data))
	require.Len(t, lines, 2, "both steps must have run exactly once; got %q", string(data))
	assert.Equal(t, "stepA", lines[0], "stepA (dependency) must run before stepB")
	assert.Equal(t, "stepB", lines[1], "stepB (dependent) must run after stepA")
}

// TestSliceOrderLoop_RED captures the historical defect directly: it drives the
// in-test reference implementation of the OLD slice-order loop and proves it
// mis-skips an out-of-order dependent step. This is the standing RED evidence
// that the bug was real (§11.4.115 — RED on the broken algorithm).
func TestSliceOrderLoop_RED(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "dag_order_red")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	orderFile := filepath.Join(tempDir, "order.txt")

	projectManager := project.NewManager()
	executor := NewExecutorWithLLM(projectManager, nil, &ExecutorConfig{MaxConcurrentSteps: 4})
	proj := &project.Project{Type: "generic", Path: tempDir}

	wf := &Workflow{
		ID:     "red",
		Status: WorkflowStatusPending,
		Steps: []Step{
			{ID: "stepB", Action: StepActionExecuteCommand, Description: "echo stepB >> " + orderFile, Dependencies: []string{"stepA"}, Status: StepStatusPending},
			{ID: "stepA", Action: StepActionExecuteCommand, Description: "echo stepA >> " + orderFile, Status: StepStatusPending},
		},
	}

	simulateLegacySliceOrderLoop(context.Background(), executor, wf, proj)

	// The defect: stepB is permanently skipped.
	assert.Equal(t, StepStatusSkipped, wf.getStepStatus(0),
		"legacy loop mis-skips stepB (dep appears later in slice) — this is the bug being fixed")
}

// simulateLegacySliceOrderLoop reproduces the pre-fix executeWorkflow body
// (the buggy slice-order loop) faithfully, so the RED tests assert against the
// actual historical algorithm rather than a synthetic failure.
func simulateLegacySliceOrderLoop(ctx context.Context, e *Executor, workflow *Workflow, proj *project.Project) {
	workflow.SetStatus(WorkflowStatusRunning)
	for i := range workflow.Steps {
		step := &workflow.Steps[i]
		if !legacyAreDependenciesCompleted(workflow, step) {
			workflow.setStepStatus(i, StepStatusSkipped, "")
			continue
		}
		workflow.setStepStatus(i, StepStatusRunning, "")
		_, err := e.executeStep(ctx, step, proj)
		if err != nil {
			workflow.setStepStatus(i, StepStatusFailed, err.Error())
			workflow.SetStatus(WorkflowStatusFailed)
			return
		}
		workflow.setStepStatus(i, StepStatusCompleted, "")
	}
	workflow.SetStatus(WorkflowStatusCompleted)
}

// legacyAreDependenciesCompleted is the deleted production method, kept here in
// _test.go only as the RED reference oracle.
func legacyAreDependenciesCompleted(workflow *Workflow, step *Step) bool {
	for _, depID := range step.Dependencies {
		depCompleted := false
		for i := range workflow.Steps {
			if workflow.Steps[i].ID == depID && workflow.getStepStatus(i) == StepStatusCompleted {
				depCompleted = true
				break
			}
		}
		if !depCompleted {
			return false
		}
	}
	return true
}

func splitNonEmpty(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(l); t != "" {
			out = append(out, t)
		}
	}
	return out
}
