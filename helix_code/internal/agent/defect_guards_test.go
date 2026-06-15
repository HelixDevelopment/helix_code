package agent

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"dev.helix.code/internal/agent/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// redMode reports whether the RED-reproduction polarity is active (§11.4.115).
//
//   - RED_MODE=1 : reproduce-and-assert-defect-PRESENT on the CURRENT (pre-fix)
//     artifact. Captured proof the guard is real.
//   - RED_MODE=0 (default) : standing GREEN regression guard asserting the
//     defect is ABSENT on the fixed artifact (§11.4.135).
//
// One source, two roles — the bug-catcher IS the regression-guard.
func redMode() bool { return os.Getenv("RED_MODE") == "1" }

// -----------------------------------------------------------------------------
// DEFECT-1 — BaseAgent.Stop()/Shutdown() double-close panic
//
// Root cause: Stop() did an unconditional `close(a.stopChan)`; Shutdown() calls
// Stop(). With no sync.Once guard a second Stop/Shutdown (e.g. a
// Coordinator.Shutdown retry) hits `panic: close of closed channel` and crashes
// the process.
//
// RED_MODE=1 on pre-fix code: the second Stop() panics → the test FAILs (the
// recover() observes a non-nil panic, proving the defect is present).
// RED_MODE=0 on fixed code: idempotent — second Stop()/Shutdown() is a safe
// no-op, no panic.
// -----------------------------------------------------------------------------

func TestDefect1_DoubleStopIsIdempotent(t *testing.T) {
	a := NewBaseAgentFromConfig(&AgentConfig{ID: "d1-stop", Name: "Defect1 Agent", Type: AgentTypeCoding})
	require.NoError(t, a.Start(context.Background()))

	a.Stop() // terminal stop — drains the goroutine

	panicked := func() (p bool) {
		defer func() {
			if r := recover(); r != nil {
				p = true
			}
		}()
		a.Stop() // second Stop MUST be a safe no-op after the fix
		return false
	}()

	if redMode() {
		// On the broken artifact the second close panics. Assert the defect is
		// genuinely present so the guard is not blind (§11.4.115 honest RED).
		require.True(t, panicked,
			"RED expectation: pre-fix code MUST panic on the second Stop()")
		return
	}
	// GREEN guard: fixed code never panics on a repeated Stop().
	assert.False(t, panicked, "second Stop() must be idempotent, not panic")
}

func TestDefect1_DoubleShutdownIsIdempotent(t *testing.T) {
	a := NewBaseAgentFromConfig(&AgentConfig{ID: "d1-shutdown", Name: "Defect1 Agent", Type: AgentTypeCoding})
	ctx := context.Background()
	require.NoError(t, a.Start(ctx))

	require.NoError(t, a.Shutdown(ctx)) // first shutdown stops the goroutine

	panicked := func() (p bool) {
		defer func() {
			if r := recover(); r != nil {
				p = true
			}
		}()
		// A Coordinator.Shutdown retry path calls Shutdown again.
		_ = a.Shutdown(ctx)
		return false
	}()

	if redMode() {
		require.True(t, panicked,
			"RED expectation: pre-fix code MUST panic on the second Shutdown()")
		return
	}
	assert.False(t, panicked, "second Shutdown() must be idempotent, not panic")
}

// -----------------------------------------------------------------------------
// DEFECT-2 — Coordinator.ExecuteTask nil-result deref
//
// Root cause: when err==nil the code did `t.Complete(result.Output)` +
// `c.results[taskID]=result` with no `result != nil` guard. An agent returning
// (nil,nil) → nil-pointer dereference.
//
// RED_MODE=1 on pre-fix code: ExecuteTask panics on result.Output → FAIL.
// RED_MODE=0 on fixed code: ExecuteTask returns a descriptive error, no panic,
// and the nil result is NOT stored as a success.
// -----------------------------------------------------------------------------

func TestDefect2_NilResultWithoutErrorBecomesError(t *testing.T) {
	coordinator := NewCoordinator(nil)

	agent := newMockCoordAgent("d2-agent", AgentTypeCoding, []Capability{CapabilityCodeGeneration})
	// The pathological agent: returns (nil, nil) — success with no result.
	agent.executeFunc = func(context.Context, *task.Task) (*task.Result, error) {
		return nil, nil
	}
	coordinator.RegisterAgent(agent)

	testTask := task.NewTask(task.TaskType("test"), "D2 Task", "nil-result task", task.PriorityNormal)
	ctx := context.Background()
	require.NoError(t, coordinator.SubmitTask(ctx, testTask))

	var (
		result   *task.Result
		execErr  error
		panicked bool
	)
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		result, execErr = coordinator.ExecuteTask(ctx, testTask.ID)
	}()

	if redMode() {
		require.True(t, panicked,
			"RED expectation: pre-fix code MUST nil-deref panic on (nil,nil)")
		return
	}

	// GREEN guard: graceful error, no panic, no stored-nil-success.
	assert.False(t, panicked, "ExecuteTask must not panic on a nil result")
	require.Error(t, execErr, "(nil,nil) from an agent must surface as an error")
	assert.Nil(t, result, "no result should be returned for the nil-result case")

	// The nil result must NOT have been recorded as a success: either no
	// result is stored (lookup error) or, if one is, it is non-nil.
	stored, lookupErr := coordinator.GetResult(testTask.ID)
	if lookupErr == nil {
		assert.NotNil(t, stored, "a nil result must never be stored as success")
	}
}

// Negative case: the normal non-nil success path is unaffected by the guard.
func TestDefect2_NonNilResultStillSucceeds(t *testing.T) {
	coordinator := NewCoordinator(nil)
	agent := newMockCoordAgent("d2-ok", AgentTypeCoding, []Capability{CapabilityCodeGeneration})
	coordinator.RegisterAgent(agent)

	testTask := task.NewTask(task.TaskType("test"), "D2 OK", "normal task", task.PriorityNormal)
	ctx := context.Background()
	require.NoError(t, coordinator.SubmitTask(ctx, testTask))

	result, err := coordinator.ExecuteTask(ctx, testTask.ID)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Success)
}

// -----------------------------------------------------------------------------
// DEFECT-3 — Workflow.GetReadySteps -> IsStepReady reentrant RLock deadlock
//
// Root cause: GetReadySteps() held w.mu.RLock() and then called IsStepReady()
// which acquired w.mu.RLock() AGAIN. Per the Go RWMutex contract, if a writer
// (SetStepResult's Lock()) blocks between the two read-locks, the recursive
// RLock can never be granted → permanent deadlock.
//
// The test drives GetReadySteps() concurrently with SetStepResult() (the
// blocking writer) under a watchdog so a regression FAILS FAST instead of
// hanging the suite forever. Run with -race to also surface lock ordering.
//
// RED_MODE=1 on pre-fix code: the watchdog fires (deadlock) → FAIL.
// RED_MODE=0 on fixed code: all GetReadySteps() calls return promptly.
// -----------------------------------------------------------------------------

func TestDefect3_GetReadyStepsNoDeadlockUnderWriter(t *testing.T) {
	wf := NewWorkflow("d3", "deadlock repro")
	// A small dependency graph so GetReadySteps walks steps and consults
	// IsStepReady for each.
	wf.AddStep(&WorkflowStep{ID: "a"})
	wf.AddStep(&WorkflowStep{ID: "b", DependsOn: []string{"a"}})
	wf.AddStep(&WorkflowStep{ID: "c", DependsOn: []string{"a", "b"}})

	done := make(chan struct{})
	go func() {
		defer close(done)
		var wg sync.WaitGroup
		// Many concurrent writers (Lock) interleaved with readers
		// (GetReadySteps -> IsStepReady reentrant RLock).
		for i := 0; i < 50; i++ {
			wg.Add(2)
			go func(n int) {
				defer wg.Done()
				id := "a"
				if n%2 == 0 {
					id = "b"
				}
				wf.SetStepResult(id, &task.Result{TaskID: id, Success: true})
			}(i)
			go func() {
				defer wg.Done()
				_ = wf.GetReadySteps()
			}()
		}
		wg.Wait()
	}()

	// Watchdog: a deadlock manifests as the goroutine never closing `done`.
	const watchdog = 5 * time.Second
	select {
	case <-done:
		if redMode() {
			t.Fatalf("RED expectation: pre-fix code MUST deadlock; instead it completed")
		}
		// GREEN guard: completed without deadlock.
	case <-time.After(watchdog):
		if redMode() {
			// Deadlock observed — defect reproduced. The goroutine is parked
			// on the recursive RLock; we intentionally leave it (test process
			// exits at suite end). This is the captured proof of the defect.
			t.Logf("RED reproduced: GetReadySteps deadlocked within %s", watchdog)
			return
		}
		t.Fatalf("deadlock: GetReadySteps did not complete within %s (regression of reentrant RLock)", watchdog)
	}
}
