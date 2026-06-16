package agent

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"dev.helix.code/internal/agent/task"
	"dev.helix.code/internal/hooks"
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

// -----------------------------------------------------------------------------
// DEFECT-4 — BaseAgent.Initialize unsynchronised state mutation (data race)
//
// Root cause: Initialize() mutated a.id / a.name / a.agentType / a.capabilities
// WITHOUT holding a.mu, while getSystemPrompt / Capabilities / Health /
// HealthMap / GetStatistics read those same fields UNDER a.mu. A re-configuring
// Initialize() racing a live agent loop reading the system prompt was a genuine
// DATA RACE: a torn read of the capabilities slice header (len/cap/ptr observed
// mid-reallocation) or a torn name/agentType string read — undefined behaviour,
// not merely a stale value.
//
// For a data race the -race tripwire IS the reproduction (§11.4.115): on the
// pre-fix artifact `go test -race -run TestDefect4...` reports "DATA RACE" and
// the run FAILs; on the fixed artifact (Initialize guarded by a.mu) the run is
// clean. RED_MODE here governs only the descriptive assertion — the load-bearing
// signal is the race detector. Run under -race for the real proof.
// -----------------------------------------------------------------------------

func TestDefect4_InitializeIsRaceFree(t *testing.T) {
	a := NewBaseAgentFromConfig(&AgentConfig{ID: "d4", Name: "Defect4 Agent", Type: AgentTypeCoding})

	const readers = 4
	const writes = 4000

	var wg sync.WaitGroup
	stop := make(chan struct{})

	for r := 0; r < readers; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					// Concurrent readers exercising every locked accessor that
					// touches the fields Initialize mutates. ID()/Name()/Type()
					// read a.id/a.name/a.agentType respectively — the exact fields
					// the writer below re-configures — so they MUST be exercised
					// here for the race detector to certify their RLock guards
					// against the Initialize() write side (review MUST-FIX 1).
					_ = a.ID()
					_ = a.Name()
					_ = a.Type()
					_ = a.Capabilities()
					_ = a.getSystemPrompt()
					_ = a.Health()
					_ = a.GetStatistics()
				}
			}
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < writes; i++ {
			// Reconfigure ID/Name/Type/Capabilities — ALL four fields the
			// accessors read — so the race detector exercises every accessor's
			// lock against a live write. ID MUST be set or ID()'s guard would
			// read a field nobody writes (no race possible, blind guard).
			_ = a.Initialize(context.Background(), &AgentConfig{
				ID:           "reconfigured-id",
				Name:         "reconfigured",
				Type:         AgentTypePlanning,
				Capabilities: []Capability{CapabilityCodeGeneration, CapabilityDebugging},
			})
		}
		close(stop)
	}()

	wg.Wait()

	// Reaching here under -race with no DATA RACE report is the GREEN proof.
	// (Under RED_MODE on the pre-fix artifact, -race aborts the binary before
	// this point with a non-zero exit.)
	if redMode() {
		t.Log("RED_MODE: the load-bearing reproduction is the -race DATA RACE report on the pre-fix Initialize")
	}
	assert.NotEmpty(t, a.Capabilities(), "post-reconfigure capabilities must be observable")
}

// -----------------------------------------------------------------------------
// DEFECT-5 — BaseAgent.SetHooksManager unsynchronised pointer mutation (race)
//
// Root cause: SetHooksManager() wrote a.hooksManager with NO lock, while
// dispatchOnError() and RequestPlanApproval() READ a.hooksManager with NO lock.
// dispatchOnError fires on every tool/LLM error inside the live agent loop
// (executeTaskWithLLM, Execute), so a host wiring a hooks manager while the
// agent is processing was a real DATA RACE on a pointer field.
//
// As with DEFECT-4 the -race detector is the reproduction (§11.4.115): pre-fix
// → "DATA RACE" + FAIL; fixed (both writer and readers snapshot under a.mu) →
// clean. Run under -race for the real proof.
// -----------------------------------------------------------------------------

func TestDefect5_HooksManagerAccessIsRaceFree(t *testing.T) {
	a := NewBaseAgentFromConfig(&AgentConfig{ID: "d5", Name: "Defect5 Agent", Type: AgentTypeCoding})

	const writes = 4000

	var wg sync.WaitGroup
	stop := make(chan struct{})

	// Reader: the live error-dispatch path.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				a.dispatchOnError(context.Background(), context.Canceled, "probe")
				_ = a.RequestPlanApproval(context.Background(), "plan")
			}
		}
	}()

	// Writer: a host re-wiring the hooks manager concurrently.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < writes; i++ {
			a.SetHooksManager(hooks.NewManager())
		}
		close(stop)
	}()

	wg.Wait()

	if redMode() {
		t.Log("RED_MODE: the load-bearing reproduction is the -race DATA RACE report on the pre-fix SetHooksManager/dispatchOnError")
	}
	// Reaching here under -race with no DATA RACE report is the GREEN proof.
}
