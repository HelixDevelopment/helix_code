package planner

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
	"unicode/utf8"
)

type StepExecutor interface {
	ExecuteStep(ctx context.Context, step *TaskStep) error
}

type SequentialExecutor struct {
	shellRunner ShellRunner
}

type ShellRunner func(ctx context.Context, command string) (string, error)

func DefaultShellRunner(ctx context.Context, command string) (string, error) {
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	out, err := cmd.CombinedOutput()
	output := string(out)
	if err != nil {
		prefix := tr(ctx, "internal_planner_err_command_failed_fmt", nil)
		return output, fmt.Errorf("%s: %w\n%s", prefix, err, output)
	}
	return output, nil
}

func NewSequentialExecutor(runner ShellRunner) *SequentialExecutor {
	if runner == nil {
		runner = DefaultShellRunner
	}
	return &SequentialExecutor{shellRunner: runner}
}

func (e *SequentialExecutor) ExecuteStep(ctx context.Context, step *TaskStep) error {
	if step.Status == StepCompleted {
		return nil
	}

	step.StartedAt = time.Now().UTC()
	step.Status = StepRunning

	timeout := step.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}

	for attempt := 0; attempt <= step.MaxRetries; attempt++ {
		// Honour parent-context cancellation BEFORE doing any work for this
		// attempt. A cancelled/expired parent ctx (user abort, server
		// shutdown, request deadline) must abort the retry loop immediately
		// instead of burning through MaxRetries with exponential sleeps.
		if err := ctx.Err(); err != nil {
			step.Status = StepFailed
			step.Error = err.Error()
			step.CompletedAt = time.Now().UTC()
			return err
		}

		if attempt > 0 {
			step.RetryCount = attempt
			// Context-aware backoff: a parent-context cancellation during the
			// sleep must abort the retry immediately rather than block for the
			// full exponential interval.
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			timer := time.NewTimer(backoff)
			select {
			case <-ctx.Done():
				timer.Stop()
				step.Status = StepFailed
				step.Error = ctx.Err().Error()
				step.CompletedAt = time.Now().UTC()
				return ctx.Err()
			case <-timer.C:
			}
		}

		stepCtx, cancel := context.WithTimeout(ctx, timeout)

		var output string
		var err error

		switch step.Type {
		case StepShell:
			output, err = e.shellRunner(stepCtx, step.Command)
		default:
			cancel()
			step.Status = StepFailed
			step.Error = tr(ctx, "internal_planner_err_unsupported_step_type", map[string]any{
				"Type": step.Type.String(),
			})
			return fmt.Errorf("%w: %s", ErrInvalidStep, step.Type)
		}

		cancel()

		if err == nil {
			step.Output = sanitizeOutput(output, MaxStepOutput)
			step.Status = StepCompleted
			step.CompletedAt = time.Now().UTC()
			return nil
		}

		if stepCtx.Err() == context.DeadlineExceeded {
			step.Error = ErrStepTimeout.Error()
		} else {
			step.Error = err.Error()
		}
	}

	step.Status = StepFailed
	step.CompletedAt = time.Now().UTC()
	return fmt.Errorf("%w after %d retries", ErrMaxRetries, step.MaxRetries)
}

func (e *SequentialExecutor) ExecutePlan(ctx context.Context, plan *TaskPlan) error {
	if plan == nil {
		return errors.New(tr(ctx, "internal_planner_err_nil_plan", nil))
	}
	if plan.Status == PlanStatusCompleted {
		return ErrPlanComplete
	}

	plan.Status = PlanStatusRunning

	for i := plan.CurrentStep; i < len(plan.Steps); i++ {
		plan.CurrentStep = i
		step := &plan.Steps[i]

		if err := e.ExecuteStep(ctx, step); err != nil {
			plan.Status = PlanStatusFailed
			prefix := tr(ctx, "internal_planner_err_step_failed_fmt", map[string]any{
				"Index": i,
				"ID":    step.ID,
			})
			// Wrap via fmt.Errorf("%s: %w", ...) so callers retain
			// errors.Is identity against the underlying step error
			// (e.g. ErrMaxRetries, context.DeadlineExceeded), while the
			// human-facing prefix is fully translator-resolved.
			return fmt.Errorf("%s: %w", prefix, err)
		}
	}

	plan.CurrentStep = len(plan.Steps)
	plan.Status = PlanStatusCompleted
	return nil
}

func sanitizeOutput(output string, maxLen int) string {
	output = strings.TrimSpace(output)
	if len(output) > maxLen {
		// Truncate on a UTF-8 rune boundary, never mid-rune. A byte-index
		// slice (output[:maxLen]) can split a multi-byte rune, producing
		// invalid UTF-8 that json.Marshal silently rewrites to U+FFFD
		// replacement characters — corrupting the output the user reads.
		truncated := output[:maxLen]
		for len(truncated) > 0 && !utf8.ValidString(truncated) {
			truncated = truncated[:len(truncated)-1]
		}
		output = truncated
	}
	return output
}
