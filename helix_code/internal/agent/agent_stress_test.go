package agent

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"dev.helix.code/internal/agent/task"
	"dev.helix.code/tests/stresschaos"
)

// §11.4.85(A) stress coverage for the REAL agent orchestrator (no fakes —
// real *Coordinator, real *AgentRegistry, real *BaseAgent constructed with no
// LLM provider so execution stays in-process and deterministic; the
// network-dependent LLM path is honestly out of scope here and exercised
// elsewhere).
//
// Properties proven:
//   - sustained load (N>=100): repeated SubmitTask + ExecuteTask cycles against
//     a real coordinator with a registered no-LLM agent, p50/p95/p99 captured.
//   - concurrent contention (>=10 goroutines): RegisterAgent / ListAgents /
//     GetAgentStats / SubmitTask / GetCircuitBreakerStats hammered in parallel
//     — the registry + coordinator state must never race, panic, or deadlock.
//   - boundary conditions: empty registry, nil task, missing task IDs,
//     duplicate IDs, max-concurrency churn.

// newStressAgent builds a real BaseAgent with no LLM provider (deterministic,
// in-process execution via executeTaskBasic) and the analysis capability so the
// coordinator's findSuitableAgent can route AnalysisTasks to it.
func newStressAgent(id string) *BaseAgent {
	a := NewBaseAgent(id, "stress-"+id, AgentTypeCoding, nil)
	a.AddCapability(Capability(string(task.TaskTypeAnalysis)))
	a.SetStatus(StatusIdle)
	return a
}

// newAnalysisTask builds a real analysis task the no-LLM agent can execute
// deterministically (basicAnalysis echoes line/char counts — no network).
func newAnalysisTask(id, content string) *task.Task {
	tk := task.NewTask(task.TaskTypeAnalysis, "stress", "stress analysis", task.PriorityNormal)
	tk.ID = id
	tk.Input = map[string]interface{}{"content": content}
	return tk
}

// TestAgent_Stress_SustainedSubmitExecute drives N>=100 real submit+execute
// cycles through a real Coordinator with a registered no-LLM agent. Every cycle
// persists a task, routes it to a real agent, and runs basicAnalysis end to end.
// p50/p95/p99 latency + a zero error rate prove the orchestrator survives load.
func TestAgent_Stress_SustainedSubmitExecute(t *testing.T) {
	c := NewCoordinator(&CoordinatorConfig{
		MaxConcurrentTasks: 10,
		TaskTimeout:        30 * time.Second,
		EnableResilience:   false, // keep execution direct + deterministic
	})
	ag := newStressAgent("sustained-agent")
	if err := c.RegisterAgent(ag); err != nil {
		t.Fatalf("register agent: %v", err)
	}
	ctx := context.Background()

	rep := stresschaos.RunSustainedLoad(t, "agent_sustained_submit_execute",
		stresschaos.SustainedConfig{N: 500}, func(i int) error {
			tk := newAnalysisTask(fmt.Sprintf("sustained-%d", i),
				fmt.Sprintf("line one\nline two\niteration %d", i))
			if err := c.SubmitTask(ctx, tk); err != nil {
				return fmt.Errorf("submit: %w", err)
			}
			res, err := c.ExecuteTask(ctx, tk.ID)
			if err != nil {
				return fmt.Errorf("execute: %w", err)
			}
			if res == nil {
				return fmt.Errorf("nil result for %s", tk.ID)
			}
			// Agent must return to idle so the next iteration can route to it.
			ag.SetStatus(StatusIdle)
			return nil
		})

	if rep.N < 100 {
		t.Fatalf("sustained N=%d below §11.4.85 floor", rep.N)
	}
	t.Logf("agent sustained: N=%d p50=%.3fms p95=%.3fms p99=%.3fms",
		rep.N, rep.P50Ms, rep.P95Ms, rep.P99Ms)
}

// TestAgent_Stress_ConcurrentRegistryAndStats hammers the coordinator's
// registry + stats surface from >=10 goroutines. Each goroutine registers a
// fresh agent, lists agents, reads per-agent stats, and reads circuit-breaker
// stats — all concurrently. Before the HXC-014 fix the AgentRegistry map had no
// mutex, so this raced (concurrent map write) and could crash the process. The
// run must complete with zero errors, no deadlock, no goroutine leak under -race.
func TestAgent_Stress_ConcurrentRegistryAndStats(t *testing.T) {
	c := NewCoordinator(nil)

	rep := stresschaos.RunConcurrent(t, "agent_concurrent_registry_stats",
		stresschaos.ConcurrencyConfig{Parallelism: 16, IterationsPerGoroutine: 80},
		func(gid, iter int) error {
			ag := newStressAgent(fmt.Sprintf("g%d-i%d", gid, iter))
			if err := c.RegisterAgent(ag); err != nil {
				return fmt.Errorf("register: %w", err)
			}
			_ = c.ListAgents()
			_ = c.GetAgentStats()
			_ = c.GetCircuitBreakerStats()
			_ = c.GetCircuitBreakerState(ag.ID())
			// Read-only task accessors widen the coordinator RLock surface.
			_, _ = c.GetTaskStatus("never-exists")
			return nil
		})

	if rep.Deadlock {
		t.Fatalf("concurrent registry churn deadlocked")
	}
	t.Logf("agent concurrent registry: calls=%d gDelta=%d dur=%.1fms",
		rep.TotalCalls, rep.GoroutineDelta, rep.DurationMs)
}

// TestAgent_Stress_ConcurrentBaseAgentState hammers a single real BaseAgent's
// mutex-guarded state from >=10 goroutines: capability add/remove, status
// flips, statistics reads, and full SubmitTask cycles. The agent's internal
// RWMutex must serialise everything with no race/panic/deadlock under -race.
func TestAgent_Stress_ConcurrentBaseAgentState(t *testing.T) {
	ag := newStressAgent("shared")
	ctx := context.Background()

	rep := stresschaos.RunConcurrent(t, "agent_concurrent_base_agent_state",
		stresschaos.ConcurrencyConfig{Parallelism: 12, IterationsPerGoroutine: 100},
		func(gid, iter int) error {
			switch (gid + iter) % 5 {
			case 0:
				ag.AddCapability(Capability(fmt.Sprintf("cap-%d-%d", gid, iter)))
			case 1:
				ag.RemoveCapability(fmt.Sprintf("cap-%d-%d", gid, iter))
			case 2:
				ag.SetStatus(StatusBusy)
				ag.SetStatus(StatusIdle)
			case 3:
				_ = ag.GetStatistics()
				_ = ag.Health()
				_ = ag.HealthMap()
			default:
				tk := newAnalysisTask(fmt.Sprintf("c-%d-%d", gid, iter), "x\ny")
				if _, err := ag.SubmitTask(ctx, tk); err != nil {
					return fmt.Errorf("submit: %w", err)
				}
			}
			return nil
		})

	if rep.Deadlock {
		t.Fatalf("base-agent state churn deadlocked")
	}
	t.Logf("base-agent concurrent state: calls=%d gDelta=%d", rep.TotalCalls, rep.GoroutineDelta)
}

// TestAgent_Stress_BoundaryConditions covers the orchestrator's boundary surface
// against the REAL coordinator: empty registry, nil task submit, missing task
// IDs, duplicate registration, and circuit-breaker queries for unknown agents.
// Each boundary must be handled deterministically (clean error or coherent
// zero-value) — never a panic, never silent corruption.
func TestAgent_Stress_BoundaryConditions(t *testing.T) {
	ctx := context.Background()
	var checked int64

	// (1) empty registry: stats + list must be empty, no panic.
	c := NewCoordinator(nil)
	if len(c.ListAgents()) != 0 || len(c.GetAgentStats()) != 0 {
		t.Fatalf("empty coordinator reported non-empty registry")
	}
	atomic.AddInt64(&checked, 1)

	// (2) nil task submit must return a clean error, never panic.
	if err := c.SubmitTask(ctx, nil); err == nil {
		t.Fatalf("SubmitTask(nil) accepted a nil task")
	}
	atomic.AddInt64(&checked, 1)

	// (3) execute / status / result for a never-submitted task ID.
	if _, err := c.ExecuteTask(ctx, "missing"); err == nil {
		t.Fatalf("ExecuteTask(missing) returned nil error")
	}
	if _, err := c.GetTaskStatus("missing"); err == nil {
		t.Fatalf("GetTaskStatus(missing) returned nil error")
	}
	if _, err := c.GetResult("missing"); err == nil {
		t.Fatalf("GetResult(missing) returned nil error")
	}
	atomic.AddInt64(&checked, 1)

	// (4) duplicate registration: last-writer-wins, count stays coherent.
	ag := newStressAgent("dup")
	if err := c.RegisterAgent(ag); err != nil {
		t.Fatalf("first register: %v", err)
	}
	if err := c.RegisterAgent(ag); err != nil {
		t.Fatalf("duplicate register errored: %v", err)
	}
	if got := len(c.ListAgents()); got != 1 {
		t.Fatalf("duplicate registration produced %d agents, want 1", got)
	}
	atomic.AddInt64(&checked, 1)

	// (5) execute with no suitable idle agent (agent set busy) → clean error.
	ag.SetStatus(StatusBusy)
	tk := newAnalysisTask("boundary-busy", "content")
	if err := c.SubmitTask(ctx, tk); err != nil {
		t.Fatalf("submit boundary task: %v", err)
	}
	if _, err := c.ExecuteTask(ctx, tk.ID); err == nil {
		t.Fatalf("ExecuteTask with no idle agent returned nil error")
	}
	atomic.AddInt64(&checked, 1)

	// (6) circuit-breaker query for an unknown agent → closed (zero value).
	if st := c.GetCircuitBreakerState("nobody"); st != CircuitBreakerClosed {
		t.Fatalf("unknown-agent circuit-breaker state = %q, want closed", st)
	}
	atomic.AddInt64(&checked, 1)

	if atomic.LoadInt64(&checked) != 6 {
		t.Fatalf("expected 6 boundary checks, ran %d", checked)
	}
	t.Logf("agent boundary conditions: %d cases handled cleanly", checked)
}
