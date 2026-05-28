package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"dev.helix.code/internal/project"
	"dev.helix.code/tests/stresschaos"
)

// §11.4.85(B) chaos coverage for the workflow package.
//
// Chaos classes exercised against the REAL workflow machinery (no fakes — real
// Executor, real BackgroundManager, real os/exec command steps, real mutexes):
//
//   - process-death: a long-running workflow step (a real `sleep` command via
//     executeCommandStep / exec.CommandContext) is interrupted by cancelling its
//     context mid-execution. The executor MUST observe the cancellation, mark the
//     workflow failed, and unwind without leaking the executor goroutine or
//     deadlocking. Symmetrically, a running BackgroundTask is StopTask'd mid-op.
//   - executor-panic: a BackgroundExecutor that panics mid-op MUST be isolated by
//     BackgroundManager.run's recover() — the panic must mark the task Failed and
//     MUST NOT crash the whole process / other tasks.
//   - input-corruption: structurally hostile step descriptions and hostile task
//     args (control bytes, huge strings, embedded shell metacharacters that the
//     security filter must catch) are fed to the real executor — it must reject or
//     handle them without crashing.
//   - state-corruption under contention: a single workflow + a single manager are
//     concurrently mutated (status flips / start+stop+close churn) from many
//     goroutines; the state MUST stay self-consistent, never panic/race.
//   - resource-pressure: workflow + background work run under bounded memory
//     pressure and must not OOM-crash.

// TestWorkflow_Chaos_CancelDuringExecution injects a process-death fault: a real
// workflow whose first step is a real `sleep` command (run via os/exec) is
// cancelled mid-execution. The executor MUST observe ctx cancellation, fail the
// step + workflow, and the executeWorkflow goroutine MUST unwind (no leak, no
// deadlock). An op that does not unwind within the helper's bound is Fatal.
func TestWorkflow_Chaos_CancelDuringExecution(t *testing.T) {
	pm := project.NewManager()
	proj, err := pm.CreateProject(context.Background(), "wf-chaos", "chaos", t.TempDir(), "generic")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	exec := NewExecutor(pm)
	exec.config.EnableLLM = false

	var terminal int32
	stresschaos.ChaosKillDuring(t, "workflow_cancel_during_execution", 150*time.Millisecond,
		func(ctx context.Context, rec *stresschaos.ChaosRecorder) {
			// A workflow whose first step blocks in a real `sleep 30` command —
			// long enough that the injected cancellation lands mid-execution.
			wf := &Workflow{
				ID: "long",
				Steps: []Step{
					{ID: "block", Description: "sleep 30", Action: StepActionExecuteCommand, Status: StepStatusPending},
					{ID: "after", Description: "echo never", Action: StepActionExecuteCommand, Status: StepStatusPending},
				},
				Status: WorkflowStatusPending,
			}
			// Run synchronously on THIS goroutine; the helper cancels ctx after the
			// delay, which exec.CommandContext propagates to kill the sleep.
			exec.executeWorkflow(ctx, wf, proj)

			st := wf.GetStatus()
			atomic.StoreInt32(&terminal, 1)
			// Graceful degradation: the cancelled command fails the step, which
			// marks the workflow Failed. That is the correct controlled outcome.
			if st == WorkflowStatusFailed {
				rec.Record(stresschaos.Degraded,
					"workflow failed cleanly after mid-execution cancellation (step killed by ctx)")
			} else if st == WorkflowStatusCompleted {
				rec.Record(stresschaos.Recovered, "workflow completed despite cancellation race")
			} else {
				rec.Record(stresschaos.Fatal,
					fmt.Sprintf("workflow left in non-terminal status %q after cancellation", st))
			}
			// The second step must NOT have completed (proof the cancel really cut
			// execution short rather than letting everything run).
			if wf.getStepStatus(1) == StepStatusCompleted {
				rec.Record(stresschaos.Fatal, "step after the killed step completed — cancellation did not cut execution")
			}
		})

	if atomic.LoadInt32(&terminal) == 0 {
		t.Fatal("workflow op never recorded a terminal observation under cancellation")
	}
}

// TestWorkflow_Chaos_BackgroundTaskStopDuringRun injects process-death at the
// BackgroundManager layer: a task with a long-running in-process op is StopTask'd
// mid-run. The op MUST observe ctx.Done() and the task MUST reach a terminal
// (Cancelled/Failed) state without leaking the goroutine.
func TestWorkflow_Chaos_BackgroundTaskStopDuringRun(t *testing.T) {
	rec := stresschaos.NewChaosRecorder(t, "workflow_background_stop_during_run", "process-death")
	bm := NewBackgroundManager(nil, ManagerConfig{})
	t.Cleanup(func() { _ = bm.Close() })

	started := make(chan struct{})
	op := func(ctx context.Context, args map[string]any, sink LineSink) (any, error) {
		close(started)
		// Block until cancelled — a worker busy on a long task.
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(30 * time.Second):
			return "completed-unexpectedly", nil
		}
	}

	task, err := bm.StartTask("long-runner", nil, op)
	if err != nil {
		rec.Record(stresschaos.Fatal, "could not start task: "+err.Error())
		rec.AssertNoFatal()
		return
	}

	select {
	case <-started:
	case <-time.After(5 * time.Second):
		rec.Record(stresschaos.Fatal, "task op never started")
		rec.AssertNoFatal()
		return
	}

	rec.Record(stresschaos.Degraded, "injecting process-death: StopTask mid-run")
	if err := bm.StopTask(task.ID); err != nil {
		rec.Record(stresschaos.Degraded, "StopTask returned (acceptable race): "+err.Error())
	}

	// The task must reach a terminal state promptly — proof the cancellation
	// propagated and the goroutine unwound.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if task.State().IsTerminal() {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !task.State().IsTerminal() {
		rec.Record(stresschaos.Fatal, fmt.Sprintf("task did not terminate after stop — state=%s (goroutine leaked)", task.State()))
	} else {
		rec.Record(stresschaos.Recovered, fmt.Sprintf("task terminated cleanly after stop: state=%s", task.State()))
	}

	rec.AssertNoFatal()
}

// TestWorkflow_Chaos_ExecutorPanicIsolation injects an executor-panic fault: a
// BackgroundExecutor that panics mid-op MUST be isolated by the manager's
// recover() so the panic marks ONLY that task Failed and does NOT crash the
// process or starve co-running tasks. An unisolated panic in the run goroutine
// would kill the whole `go test` binary.
func TestWorkflow_Chaos_ExecutorPanicIsolation(t *testing.T) {
	rec := stresschaos.NewChaosRecorder(t, "workflow_executor_panic_isolation", "process-death")
	bm := NewBackgroundManager(nil, ManagerConfig{})
	t.Cleanup(func() { _ = bm.Close() })

	// A well-behaved co-task that should still complete despite a sibling panic.
	var coDone int32
	coOp := func(ctx context.Context, args map[string]any, sink LineSink) (any, error) {
		sink("co-task ran")
		atomic.StoreInt32(&coDone, 1)
		return "ok", nil
	}
	panicOp := func(ctx context.Context, args map[string]any, sink LineSink) (any, error) {
		panic("chaos: executor panic mid-op")
	}

	coTask, err := bm.StartTask("co", nil, coOp)
	if err != nil {
		rec.Record(stresschaos.Fatal, "start co-task: "+err.Error())
		rec.AssertNoFatal()
		return
	}
	panicTask, err := bm.StartTask("boom", nil, panicOp)
	if err != nil {
		rec.Record(stresschaos.Fatal, "start panic-task: "+err.Error())
		rec.AssertNoFatal()
		return
	}

	// Wait for both to reach terminal state.
	waitTerminal := func(task *BackgroundTask) bool {
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			if task.State().IsTerminal() {
				return true
			}
			time.Sleep(10 * time.Millisecond)
		}
		return false
	}

	if !waitTerminal(panicTask) {
		rec.Record(stresschaos.Fatal, "panicking task never reached terminal state — recover() missing")
	} else if panicTask.State() != TaskFailed {
		rec.Record(stresschaos.Fatal, fmt.Sprintf("panicking task state=%s, want failed (panic not classified)", panicTask.State()))
	} else {
		// The recorded error must mention the panic — proof recover() captured it.
		if _, e := panicTask.Result(); e == nil || !strings.Contains(e.Error(), "panic") {
			rec.Record(stresschaos.Fatal, fmt.Sprintf("panic task error did not record panic: %v", e))
		} else {
			rec.Record(stresschaos.Recovered, "manager isolated executor panic and marked task Failed: "+e.Error())
		}
	}

	if !waitTerminal(coTask) || atomic.LoadInt32(&coDone) == 0 {
		rec.Record(stresschaos.Fatal, "co-task starved/did not complete despite sibling panic — panic not isolated")
	} else {
		rec.Record(stresschaos.Recovered, "co-task completed despite sibling executor panic")
	}

	// Manager must remain usable for a fresh task after the panic.
	follow, err := bm.StartTask("after", nil, coOp)
	if err != nil {
		rec.Record(stresschaos.Fatal, "manager unusable after panic: "+err.Error())
	} else if !waitTerminal(follow) {
		rec.Record(stresschaos.Fatal, "follow-up task did not complete — manager broken after panic")
	} else {
		rec.Record(stresschaos.Recovered, "manager still schedules tasks after isolated panic")
	}

	rec.AssertNoFatal()
}

// TestWorkflow_Chaos_CorruptStepInput feeds structurally hostile step
// descriptions to the REAL executor's command step. The executor must reject or
// handle them without crashing: empty/whitespace commands are rejected, shell
// metacharacters / dangerous patterns are blocked by the security filter, and
// control-byte / huge inputs must not panic the executor.
func TestWorkflow_Chaos_CorruptStepInput(t *testing.T) {
	pm := project.NewManager()
	proj, err := pm.CreateProject(context.Background(), "wf-corrupt", "corrupt", t.TempDir(), "generic")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	exec := NewExecutor(pm)
	exec.config.EnableLLM = false
	ctx := context.Background()

	hostile := []string{
		"",                                  // empty -> rejected
		"   \t  ",                           // whitespace only -> rejected
		"rm -rf /",                          // dangerous -> blocked by security filter
		"echo hi && rm -rf /",               // chained rm -> blocked
		"echo $(rm -rf /)",                  // command substitution -> blocked
		"echo \x00\x01\x02 control bytes",   // control bytes -> handled, not crash
		"echo " + strings.Repeat("A", 1<<16), // huge arg -> handled, not crash
		":(){ :|:& };:",                     // fork bomb -> blocked
	}
	payloads := make([][]byte, len(hostile))
	for i, h := range hostile {
		payloads[i], _ = json.Marshal(map[string]string{"cmd": h})
	}

	stresschaos.ChaosCorruptInputDuring(t, "workflow_corrupt_step_input", payloads,
		func(input []byte) error {
			var m map[string]string
			if err := json.Unmarshal(input, &m); err != nil {
				return fmt.Errorf("descriptor unmarshal: %w", err)
			}
			step := &Step{ID: "corrupt", Description: m["cmd"], Action: StepActionExecuteCommand}
			// Must not panic regardless of input. An error (rejection/block/exec
			// failure) is the graceful path; a clean run is also acceptable for the
			// benign control-byte/huge echo cases.
			_, err := exec.executeCommandStep(ctx, step, proj)
			return err
		})
}

// TestWorkflow_Chaos_StateChurn hammers a single workflow AND a single
// background manager with concurrent state mutation from many goroutines: the
// workflow's status/step state is flipped while readers poll, and the manager is
// driven through Start/Stop/Status/List/Close churn. The shared mutexes MUST
// serialise everything so state stays self-consistent — no panic, no race.
func TestWorkflow_Chaos_StateChurn(t *testing.T) {
	rec := stresschaos.NewChaosRecorder(t, "workflow_state_churn", "state-corruption")
	wf := echoCommandWorkflow("churn", 6)
	bm := NewBackgroundManager(nil, ManagerConfig{MaxConcurrent: 4096, OutputCap: 16})
	t.Cleanup(func() { _ = bm.Close() })

	noop := func(ctx context.Context, args map[string]any, sink LineSink) (any, error) {
		sink("x")
		return nil, nil
	}

	const goroutines = 14
	const iters = 250
	var wg sync.WaitGroup
	var wfOps, bmOps int64
	wg.Add(goroutines)
	for w := 0; w < goroutines; w++ {
		go func(id int) {
			defer wg.Done()
			defer func() {
				if p := recover(); p != nil {
					rec.Record(stresschaos.Fatal, fmt.Sprintf("goroutine %d panicked: %v", id, p))
				}
			}()
			for it := 0; it < iters; it++ {
				switch (id + it) % 5 {
				case 0:
					wf.SetStatus(WorkflowStatusRunning)
					atomic.AddInt64(&wfOps, 1)
				case 1:
					_ = wf.GetStatus()
					wf.setStepStatus((id+it)%len(wf.Steps), StepStatusRunning, "")
					atomic.AddInt64(&wfOps, 1)
				case 2:
					if task, err := bm.StartTask("churn-tool", nil, noop); err == nil {
						_ = bm.StopTask(task.ID)
					}
					atomic.AddInt64(&bmOps, 1)
				case 3:
					_ = bm.ListTasks()
					atomic.AddInt64(&bmOps, 1)
				default:
					_ = wf.getStepStatus((id + it) % len(wf.Steps))
					_ = wf.GetUpdatedAt()
					atomic.AddInt64(&wfOps, 1)
				}
			}
		}(w)
	}
	wg.Wait()

	rec.Record(stresschaos.Recovered, fmt.Sprintf(
		"survived concurrent state churn: %d workflow ops, %d manager ops, no panic/race",
		atomic.LoadInt64(&wfOps), atomic.LoadInt64(&bmOps)))

	// Final state must be coherent + readable.
	final := wf.GetStatus()
	switch final {
	case WorkflowStatusPending, WorkflowStatusRunning, WorkflowStatusCompleted, WorkflowStatusFailed:
		rec.Record(stresschaos.Recovered, "workflow status coherent after churn: "+string(final))
	default:
		rec.Record(stresschaos.Fatal, "workflow status incoherent after churn: "+string(final))
	}
	// Manager must still accept a fresh task — proof its map was not left torn.
	if task, err := bm.StartTask("post-churn", nil, noop); err != nil {
		rec.Record(stresschaos.Degraded, "manager rejected post-churn task: "+err.Error())
	} else if task == nil {
		rec.Record(stresschaos.Fatal, "manager returned nil task after churn")
	} else {
		rec.Record(stresschaos.Recovered, "manager schedules fresh task after churn — map self-consistent")
	}

	rec.AssertNoFatal()
}

// TestWorkflow_Chaos_ResourcePressure runs the real workflow execution + a real
// background task under bounded memory pressure (§11.4.85(B)(4)). The work must
// complete without an OOM-crash, proving the lock-guarded paths degrade rather
// than die under pressure.
func TestWorkflow_Chaos_ResourcePressure(t *testing.T) {
	pm := project.NewManager()
	proj, err := pm.CreateProject(context.Background(), "wf-pressure", "pressure", t.TempDir(), "generic")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	exec := NewExecutor(pm)
	exec.config.EnableLLM = false
	ctx := context.Background()

	stresschaos.ChaosResourcePressureDuring(t, "workflow_resource_pressure", 48,
		func(rec *stresschaos.ChaosRecorder) {
			wf := echoCommandWorkflow("pressure", 5)
			exec.executeWorkflow(ctx, wf, proj)
			if wf.GetStatus() != WorkflowStatusCompleted {
				rec.Record(stresschaos.Fatal, "workflow did not complete under memory pressure: "+string(wf.GetStatus()))
				return
			}
			rec.Record(stresschaos.Recovered, "workflow completed under bounded memory pressure")
		})
}
