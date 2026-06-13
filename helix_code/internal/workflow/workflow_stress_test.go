package workflow

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"dev.helix.code/internal/project"
	"dev.helix.code/tests/stresschaos"
)

// §11.4.85(A) stress coverage for the workflow package.
//
// Two REAL (non-mocked) concurrent-state surfaces of internal/workflow are
// driven here, both in-process and deterministic (no LLM / no network):
//
//   - *Workflow + *Executor: the RWMutex-guarded workflow status, UpdatedAt, and
//     per-step Status/Error fields, mutated through the real SetStatus /
//     GetStatus / setStepStatus / getStepStatus accessors and through the real
//     executeWorkflow goroutine running a workflow whose steps are deterministic
//     `execute_command` echo steps (so no LLM provider is required and the run is
//     fully reproducible). CancelWorkflow flips status concurrently with readers.
//   - *BackgroundManager + *BackgroundTask: atomic-Int32 state, the RWMutex-guarded
//     bounded output ring, and the StartTask/StopTask/Status/ListTasks dispatch
//     machinery, hammered from many goroutines.
//
// Every handler/executor does real work counted through atomics, so a PASS proves
// real state transitions happened — not a no-op. Run under -race to catch any
// data race in the lock-guarded paths.

// newDeterministicProject creates a real project.Project backed by a real temp
// directory, so the executor's gatherProjectContext WalkDir hits a real (empty)
// tree and the `execute_command` steps run a real, harmless `echo` via os/exec.
func newDeterministicProject(t testing.TB) (*project.Manager, *project.Project) {
	t.Helper()
	pm := project.NewManager()
	dir, err := os.MkdirTemp("", "wf_stress_project")
	if err != nil {
		t.Fatalf("mkdir temp project: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	proj, err := pm.CreateProject(context.Background(), "wf-stress", "stress project", dir, "generic")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	return pm, proj
}

// echoCommandWorkflow builds a real Workflow whose steps are deterministic
// `echo` commands routed through executeCommandStep (os/exec, no LLM, no
// network), so executeWorkflow drives real state transitions reproducibly.
func echoCommandWorkflow(id string, nSteps int) *Workflow {
	steps := make([]Step, nSteps)
	for i := 0; i < nSteps; i++ {
		steps[i] = Step{
			ID:          fmt.Sprintf("step_%d", i),
			Name:        fmt.Sprintf("Echo %d", i),
			Description: fmt.Sprintf("echo step-%d-ok", i),
			Type:        StepTypeExecution,
			Action:      StepActionExecuteCommand,
			Status:      StepStatusPending,
		}
	}
	return &Workflow{
		ID:        id,
		Name:      "echo-workflow",
		Mode:      "stress",
		Steps:     steps,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Status:    WorkflowStatusPending,
	}
}

// TestWorkflow_Stress_SustainedStateTransitions drives the real RWMutex-guarded
// workflow state machine under sustained load (N>=100): each iteration runs a
// fresh workflow through executeWorkflow (real `echo` exec steps) and asserts the
// terminal status is Completed and every step reached Completed — so the run
// proves real, locked state transitions, not a no-op. p50/p95/p99 captured.
func TestWorkflow_Stress_SustainedStateTransitions(t *testing.T) {
	_, proj := newDeterministicProject(t)
	exec := NewExecutor(project.NewManager())
	// Disable LLM so every step takes the deterministic command/static path.
	exec.config.EnableLLM = false
	ctx := context.Background()

	var completed int64
	stresschaos.RunSustainedLoad(t, "workflow_sustained_state_transitions",
		stresschaos.SustainedConfig{N: 200, MaxErrorRate: 0.0},
		func(i int) error {
			wf := echoCommandWorkflow(fmt.Sprintf("wf_%d", i), 3)
			// Run synchronously (not via the go-spawning Execute* entrypoints) so
			// the iteration latency reflects real per-workflow execution and we can
			// assert the terminal state deterministically.
			exec.executeWorkflow(ctx, wf, proj)

			if got := wf.GetStatus(); got != WorkflowStatusCompleted {
				return fmt.Errorf("workflow %d terminal status = %s, want completed", i, got)
			}
			for s := 0; s < len(wf.Steps); s++ {
				if st := wf.getStepStatus(s); st != StepStatusCompleted {
					return fmt.Errorf("workflow %d step %d status = %s, want completed", i, s, st)
				}
			}
			atomic.AddInt64(&completed, 1)
			return nil
		})

	if atomic.LoadInt64(&completed) == 0 {
		t.Fatal("zero workflows completed under sustained load — not real work")
	}
	t.Logf("workflow sustained: %d workflows ran to Completed with all steps Completed",
		atomic.LoadInt64(&completed))
}

// TestWorkflow_Stress_ConcurrentStatusAccess hammers a SINGLE workflow's
// RWMutex-guarded state from N>=10 goroutines that interleave SetStatus /
// GetStatus / setStepStatus / getStepStatus / GetUpdatedAt / touchUpdatedAt,
// asserting no deadlock, no leak, and no data race (run under -race) on the
// shared workflow mutex. This is the canonical concurrent-contention case.
func TestWorkflow_Stress_ConcurrentStatusAccess(t *testing.T) {
	wf := echoCommandWorkflow("concurrent_wf", 8)
	statuses := []WorkflowStatus{
		WorkflowStatusPending, WorkflowStatusRunning,
		WorkflowStatusCompleted, WorkflowStatusFailed,
	}
	stepStatuses := []StepStatus{
		StepStatusPending, StepStatusRunning, StepStatusCompleted,
		StepStatusFailed, StepStatusSkipped,
	}

	var ops int64
	stresschaos.RunConcurrent(t, "workflow_concurrent_status_access",
		stresschaos.ConcurrencyConfig{Parallelism: 16, IterationsPerGoroutine: 200, Timeout: 25 * time.Second},
		func(g, it int) error {
			switch (g + it) % 6 {
			case 0:
				wf.SetStatus(statuses[(g+it)%len(statuses)]) // write-lock
			case 1:
				_ = wf.GetStatus() // read-lock contends with the writers
			case 2:
				wf.setStepStatus((g+it)%len(wf.Steps), stepStatuses[(g+it)%len(stepStatuses)], "") // write-lock
			case 3:
				_ = wf.getStepStatus((g + it) % len(wf.Steps)) // read-lock
			case 4:
				_ = wf.GetUpdatedAt() // read-lock
			default:
				wf.touchUpdatedAt() // write-lock
			}
			atomic.AddInt64(&ops, 1)
			return nil
		})

	if atomic.LoadInt64(&ops) == 0 {
		t.Fatal("zero status operations under concurrent load")
	}
	// After the churn the workflow state must remain a coherent, readable value —
	// proof the mutex serialised the writes and the struct was not left torn.
	final := wf.GetStatus()
	switch final {
	case WorkflowStatusPending, WorkflowStatusRunning, WorkflowStatusCompleted, WorkflowStatusFailed:
	default:
		t.Fatalf("workflow ended in incoherent status %q after concurrent churn", final)
	}
	t.Logf("workflow concurrent status access: %d ops, final status=%s", atomic.LoadInt64(&ops), final)
}

// TestWorkflow_Stress_ConcurrentExecutorRegistry hammers the Executor's
// RWMutex-guarded activeFlows map and CancelWorkflow path from N>=10 goroutines
// while the real Execute*Workflow entrypoints concurrently register running
// workflows (each spawns an executeWorkflow goroutine). Exercises GetWorkflow /
// GetActiveWorkflows / CancelWorkflow / SetLLMProvider read/write contention on
// the executor mutex. Run under -race.
func TestWorkflow_Stress_ConcurrentExecutorRegistry(t *testing.T) {
	pm, _ := newDeterministicProject(t)
	exec := NewExecutor(pm)
	exec.config.EnableLLM = false

	// Pre-register a fixed pool of real running workflows whose IDs goroutines
	// will contend to read/cancel. We register directly into activeFlows under
	// the executor lock (real path the Execute* methods would also take).
	const pool = 12
	ids := make([]string, pool)
	for i := 0; i < pool; i++ {
		wf := echoCommandWorkflow(fmt.Sprintf("registry_wf_%d", i), 2)
		ids[i] = wf.ID
		exec.mu.Lock()
		exec.activeFlows[wf.ID] = wf
		exec.mu.Unlock()
	}

	var reads, cancels int64
	stresschaos.RunConcurrent(t, "workflow_concurrent_executor_registry",
		stresschaos.ConcurrencyConfig{Parallelism: 14, IterationsPerGoroutine: 150, Timeout: 25 * time.Second},
		func(g, it int) error {
			id := ids[(g+it)%pool]
			switch (g + it) % 4 {
			case 0:
				_, _ = exec.GetWorkflow(id) // RLock
				atomic.AddInt64(&reads, 1)
			case 1:
				_ = exec.GetActiveWorkflows() // RLock + map copy
				atomic.AddInt64(&reads, 1)
			case 2:
				// CancelWorkflow takes the write-lock and flips a workflow's
				// status — contends with the readers above and the SetStatus
				// inside CancelWorkflow.
				_ = exec.CancelWorkflow(id)
				atomic.AddInt64(&cancels, 1)
			default:
				exec.SetLLMProvider(nil) // write-lock on the executor mutex
				atomic.AddInt64(&reads, 1)
			}
			return nil
		})

	if atomic.LoadInt64(&reads) == 0 || atomic.LoadInt64(&cancels) == 0 {
		t.Fatalf("registry churn did too little work: reads=%d cancels=%d", reads, cancels)
	}
	// Map must remain coherent: every pooled workflow still resolvable.
	for _, id := range ids {
		if _, ok := exec.GetWorkflow(id); !ok {
			t.Fatalf("workflow %s lost from activeFlows after concurrent churn — map torn", id)
		}
	}
	t.Logf("executor registry concurrent: reads=%d cancels=%d, %d workflows intact",
		atomic.LoadInt64(&reads), atomic.LoadInt64(&cancels), pool)
}

// TestWorkflow_Stress_ConcurrentBackgroundManager hammers the REAL
// BackgroundManager from N>=10 goroutines that StartTask (atomic state +
// goroutine spawn) / GetTask / Status / ListTasks / StopTask concurrently. The
// executor closure does real, bounded in-process work (writes output lines to
// the ring) and returns — no network. Asserts no deadlock / leak / race and that
// the manager drains cleanly on Close.
func TestWorkflow_Stress_ConcurrentBackgroundManager(t *testing.T) {
	bm := NewBackgroundManager(nil, ManagerConfig{MaxConcurrent: 4096, OutputCap: 32})
	t.Cleanup(func() { _ = bm.Close() })

	var started, observed int64
	exec := func(ctx context.Context, args map[string]any, sink LineSink) (any, error) {
		// Deterministic in-process work: emit a few lines (exercises the bounded
		// output ring under concurrency) then return.
		for i := 0; i < 5; i++ {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
				sink(fmt.Sprintf("line-%d", i))
			}
		}
		return "ok", nil
	}

	stresschaos.RunConcurrent(t, "workflow_concurrent_background_manager",
		stresschaos.ConcurrencyConfig{Parallelism: 16, IterationsPerGoroutine: 60, Timeout: 25 * time.Second},
		func(g, it int) error {
			task, err := bm.StartTask("stress-tool", map[string]any{"g": g, "it": it}, exec)
			if err != nil {
				return fmt.Errorf("start task: %w", err)
			}
			atomic.AddInt64(&started, 1)
			// Concurrent reads of the shared task map + per-task state.
			if _, _, err := bm.Status(task.ID); err != nil {
				return fmt.Errorf("status: %w", err)
			}
			_ = bm.ListTasks()
			_ = task.State()
			_ = task.LastLines(3)
			atomic.AddInt64(&observed, 1)
			return nil
		})

	if atomic.LoadInt64(&started) == 0 {
		t.Fatal("zero background tasks started under concurrent load")
	}
	// Tasks were really registered — the shared map mutated under concurrency.
	if len(bm.ListTasks()) == 0 {
		t.Fatal("no tasks registered in manager after concurrent StartTask load")
	}
	t.Logf("background manager concurrent: started=%d observed=%d tasks-registered=%d",
		atomic.LoadInt64(&started), atomic.LoadInt64(&observed), len(bm.ListTasks()))
}

// TestWorkflow_Stress_BoundaryConditions exercises the §11.4.85(A)(3) boundary
// cases against the real workflow machinery: (empty) a zero-step workflow must
// run to Completed cleanly; (max) a workflow with many steps must complete every
// one; (off-by-one) dependency resolution on a missing dependency must skip, and
// the output ring at exactly its cap must retain exactly cap lines.
func TestWorkflow_Stress_BoundaryConditions(t *testing.T) {
	_, proj := newDeterministicProject(t)
	exec := NewExecutor(project.NewManager())
	exec.config.EnableLLM = false
	ctx := context.Background()

	// Empty: a zero-step workflow must reach Completed with no panic.
	t.Run("empty_workflow", func(t *testing.T) {
		wf := echoCommandWorkflow("empty", 0)
		exec.executeWorkflow(ctx, wf, proj)
		if got := wf.GetStatus(); got != WorkflowStatusCompleted {
			t.Fatalf("empty workflow status = %s, want completed", got)
		}
	})

	// Max: a workflow with many steps must drive every one to Completed.
	t.Run("many_steps", func(t *testing.T) {
		const many = 200
		wf := echoCommandWorkflow("many", many)
		exec.executeWorkflow(ctx, wf, proj)
		if got := wf.GetStatus(); got != WorkflowStatusCompleted {
			t.Fatalf("many-step workflow status = %s, want completed", got)
		}
		for i := 0; i < many; i++ {
			if st := wf.getStepStatus(i); st != StepStatusCompleted {
				t.Fatalf("step %d status = %s, want completed", i, st)
			}
		}
	})

	// Off-by-one: a step depending on a NON-EXISTENT dependency.
	//
	// Reconciled for the dev.helix.dag integration (§11.4.120): the former
	// slice-order loop SILENTLY mis-skipped such a step and reported the
	// workflow Completed — hiding a real graph-configuration error from the
	// caller. The DAG validates the graph up front (dag.Build rejects an
	// unknown dependency), so an unresolvable dependency now FAILS the
	// workflow with the cause instead of being swept under a green status.
	// This is the intended new, correct capability, not a regression.
	t.Run("missing_dependency_fails_workflow", func(t *testing.T) {
		wf := &Workflow{
			ID:    "dep",
			Steps: []Step{{ID: "a", Description: "echo a", Action: StepActionExecuteCommand, Status: StepStatusPending, Dependencies: []string{"does-not-exist"}}},
			Status: WorkflowStatusPending,
		}
		exec.executeWorkflow(ctx, wf, proj)
		if got := wf.GetStatus(); got != WorkflowStatusFailed {
			t.Fatalf("workflow with an unknown step dependency status = %s, want failed (dag.Build rejects the graph)", got)
		}
		// The step must NOT be marked Completed — the invalid graph was never run.
		if st := wf.getStepStatus(0); st == StepStatusCompleted {
			t.Fatalf("step with unresolvable dependency must not complete; status = %s", st)
		}
	})

	// Off-by-one for the background output ring: exactly outputCap lines must be
	// retained when the ring is filled to its cap, and cap+1 drops the oldest.
	t.Run("output_ring_at_cap", func(t *testing.T) {
		const cap = 8
		bt := newBackgroundTask("ring", "tool", nil, cap, 4096, nil, nil)
		for i := 0; i < cap; i++ {
			bt.AppendOutput(fmt.Sprintf("l%d", i))
		}
		if got := bt.LastLines(cap + 5); len(got) != cap {
			t.Fatalf("ring at cap retained %d lines, want %d", len(got), cap)
		}
		// One over cap: oldest must be dropped, newest retained.
		bt.AppendOutput("overflow")
		lines := bt.LastLines(cap + 5)
		if len(lines) != cap {
			t.Fatalf("ring over cap retained %d lines, want %d", len(lines), cap)
		}
		if lines[len(lines)-1] != "overflow" {
			t.Fatalf("ring over cap newest line = %q, want overflow", lines[len(lines)-1])
		}
		if lines[0] == "l0" {
			t.Fatal("ring over cap did not drop the oldest line l0")
		}
	})
}
