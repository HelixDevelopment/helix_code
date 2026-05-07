package planner

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSequentialExecutor_ExecuteStep_ShellSuccess(t *testing.T) {
	runner := func(ctx context.Context, cmd string) (string, error) {
		return "hello from " + cmd, nil
	}
	executor := NewSequentialExecutor(runner)

	step := &TaskStep{
		ID:         "step-1",
		Type:       StepShell,
		Command:    "echo hello",
		Status:     StepPending,
		MaxRetries: 1,
		Timeout:    5 * time.Second,
	}

	err := executor.ExecuteStep(context.Background(), step)
	require.NoError(t, err)
	assert.Equal(t, StepCompleted, step.Status)
	assert.Contains(t, step.Output, "hello from echo hello")
	assert.False(t, step.CompletedAt.IsZero())
}

func TestSequentialExecutor_ExecuteStep_ShellFailure(t *testing.T) {
	runner := func(ctx context.Context, cmd string) (string, error) {
		return "error output", errors.New("command failed")
	}
	executor := NewSequentialExecutor(runner)

	step := &TaskStep{
		ID:         "step-2",
		Type:       StepShell,
		Command:    "false",
		Status:     StepPending,
		MaxRetries: 0,
		Timeout:    1 * time.Second,
	}

	err := executor.ExecuteStep(context.Background(), step)
	assert.Error(t, err)
	assert.Equal(t, StepFailed, step.Status)
	assert.Contains(t, step.Error, "command failed")
}

func TestSequentialExecutor_ExecuteStep_Retry(t *testing.T) {
	attempts := 0
	runner := func(ctx context.Context, cmd string) (string, error) {
		attempts++
		if attempts < 3 {
			return "fail", errors.New("transient")
		}
		return "success", nil
	}
	executor := NewSequentialExecutor(runner)

	step := &TaskStep{
		ID:         "step-3",
		Type:       StepShell,
		Command:    "retry-me",
		Status:     StepPending,
		MaxRetries: 3,
		Timeout:    5 * time.Second,
	}

	err := executor.ExecuteStep(context.Background(), step)
	require.NoError(t, err)
	assert.Equal(t, StepCompleted, step.Status)
	assert.Equal(t, 2, step.RetryCount)
	assert.Equal(t, 3, attempts)
}

func TestSequentialExecutor_ExecuteStep_RetryExhausted(t *testing.T) {
	runner := func(ctx context.Context, cmd string) (string, error) {
		return "", errors.New("always fail")
	}
	executor := NewSequentialExecutor(runner)

	step := &TaskStep{
		ID:         "step-4",
		Type:       StepShell,
		Command:    "always-fail",
		Status:     StepPending,
		MaxRetries: 2,
		Timeout:    1 * time.Second,
	}

	err := executor.ExecuteStep(context.Background(), step)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrMaxRetries)
}

func TestSequentialExecutor_ExecuteStep_AlreadyCompleted(t *testing.T) {
	runner := func(ctx context.Context, cmd string) (string, error) {
		t.Fatal("should not be called")
		return "", nil
	}
	executor := NewSequentialExecutor(runner)

	step := &TaskStep{
		ID:         "step-5",
		Status:     StepCompleted,
		MaxRetries: 1,
	}

	err := executor.ExecuteStep(context.Background(), step)
	assert.NoError(t, err)
}

func TestSequentialExecutor_ExecutePlan(t *testing.T) {
	runner := func(ctx context.Context, cmd string) (string, error) {
		return "done: " + cmd, nil
	}
	executor := NewSequentialExecutor(runner)

	plan := &TaskPlan{
		ID:     "plan-1",
		Name:   "test-plan",
		Status: PlanStatusPending,
		Steps: []TaskStep{
			{ID: "s1", Type: StepShell, Command: "step1", Status: StepPending, MaxRetries: 1},
			{ID: "s2", Type: StepShell, Command: "step2", Status: StepPending, MaxRetries: 1},
		},
	}

	err := executor.ExecutePlan(context.Background(), plan)
	require.NoError(t, err)
	assert.Equal(t, PlanStatusCompleted, plan.Status)
	assert.Equal(t, 2, plan.CurrentStep)
	assert.Equal(t, StepCompleted, plan.Steps[0].Status)
	assert.Equal(t, StepCompleted, plan.Steps[1].Status)
}

func TestSequentialExecutor_ExecutePlan_StepFails(t *testing.T) {
	runner := func(ctx context.Context, cmd string) (string, error) {
		if cmd == "step2" {
			return "", errors.New("step2 failed")
		}
		return "ok", nil
	}
	executor := NewSequentialExecutor(runner)

	plan := &TaskPlan{
		ID:     "plan-2",
		Name:   "fail-plan",
		Status: PlanStatusPending,
		Steps: []TaskStep{
			{ID: "s1", Type: StepShell, Command: "step1", Status: StepPending, MaxRetries: 0},
			{ID: "s2", Type: StepShell, Command: "step2", Status: StepPending, MaxRetries: 0},
			{ID: "s3", Type: StepShell, Command: "step3", Status: StepPending, MaxRetries: 0},
		},
	}

	err := executor.ExecutePlan(context.Background(), plan)
	assert.Error(t, err)
	assert.Equal(t, PlanStatusFailed, plan.Status)
	assert.Equal(t, 1, plan.CurrentStep)
	assert.Equal(t, StepCompleted, plan.Steps[0].Status)
	assert.Equal(t, StepFailed, plan.Steps[1].Status)
	assert.Equal(t, StepPending, plan.Steps[2].Status)
}

func TestSequentialExecutor_ExecutePlan_Nil(t *testing.T) {
	executor := NewSequentialExecutor(nil)
	err := executor.ExecutePlan(context.Background(), nil)
	assert.Error(t, err)
}

func TestSequentialExecutor_ExecutePlan_AlreadyComplete(t *testing.T) {
	executor := NewSequentialExecutor(nil)
	plan := &TaskPlan{Status: PlanStatusCompleted}
	err := executor.ExecutePlan(context.Background(), plan)
	assert.ErrorIs(t, err, ErrPlanComplete)
}

func TestSanitizeOutput(t *testing.T) {
	assert.Equal(t, "hello", sanitizeOutput("  hello  ", 1024))
	assert.Equal(t, "ab", sanitizeOutput("abcdef", 2))
}

func TestStepType_String(t *testing.T) {
	assert.Equal(t, "shell", StepShell.String())
	assert.Equal(t, "llm", StepLLM.String())
	assert.Equal(t, "unknown", StepType(99).String())
}

func TestStepStatus_String(t *testing.T) {
	assert.Equal(t, "pending", StepPending.String())
	assert.Equal(t, "running", StepRunning.String())
	assert.Equal(t, "completed", StepCompleted.String())
	assert.Equal(t, "failed", StepFailed.String())
}
