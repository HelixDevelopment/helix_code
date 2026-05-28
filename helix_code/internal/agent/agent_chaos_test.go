package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"dev.helix.code/internal/agent/task"
	"dev.helix.code/internal/hooks"
	"dev.helix.code/tests/stresschaos"
)

// §11.4.85(B) chaos coverage for the REAL agent orchestrator (no fakes — real
// *Coordinator, *AgentRegistry, *BaseAgent, real hooks.Manager dispatch). The
// network-dependent LLM execution path is intentionally out of scope here;
// chaos is injected at the in-process orchestration / registry / lifecycle /
// callback surface, which is where the concurrency state lives.
//
// Chaos classes exercised:
//   - callback-panic isolation: a hooks OnError handler that panics mid-dispatch
//     MUST NOT take down the agent that fires it. The agent fires OnError
//     synchronously from its Execute/LLM error path; an unrecovered handler
//     panic would propagate out of the agent and crash the goroutine driving it.
//   - reentrant read-lock deadlock probe: a method that takes the registry RLock
//     while a writer is queued must not self-deadlock by re-taking the lock.
//   - state-corruption under contention: the registry is concurrently
//     Registered / Unregistered / listed mid-flight; must end self-consistent.
//   - input-corruption: structurally hostile task Input payloads are fed through
//     real execution; dispatch must reject/normalise without crashing.
//   - process-death: a running BaseAgent worker is cancelled mid-loop; Stop must
//     unwind cleanly with no leaked goroutine.
//   - resource-pressure: orchestration must complete under bounded memory load.

// TestAgent_Chaos_HookCallbackPanicIsolation registers a hooks.Manager on a real
// BaseAgent whose OnError handler PANICS, then drives the agent down its error
// path (Execute of a code task with no LLM provider returns an error, which
// fires dispatchOnError synchronously). If the agent does not isolate the
// panicking handler, the panic propagates out of Execute and crashes the
// driving goroutine — a §11.4.85(B) Fatal. A well-behaved co-handler must still
// run, proving isolation rather than total suppression.
func TestAgent_Chaos_HookCallbackPanicIsolation(t *testing.T) {
	rec := stresschaos.NewChaosRecorder(t, "agent_hook_callback_panic_isolation", "process-death")

	mgr := hooks.NewManager()
	var coHandler, panicHandler int64

	co := hooks.NewHook("co-handler", hooks.HookTypeOnError, func(ctx context.Context, e *hooks.Event) error {
		atomic.AddInt64(&coHandler, 1)
		return nil
	})
	if err := mgr.Register(co); err != nil {
		t.Fatalf("register co-handler: %v", err)
	}
	panicH := hooks.NewHook("panic-handler", hooks.HookTypeOnError, func(ctx context.Context, e *hooks.Event) error {
		atomic.AddInt64(&panicHandler, 1)
		panic("chaos: agent OnError handler panic")
	})
	if err := mgr.Register(panicH); err != nil {
		t.Fatalf("register panic-handler: %v", err)
	}

	ag := NewBaseAgent("panic-agent", "panic", AgentTypeCoding, nil)
	ag.SetHooksManager(mgr)
	ctx := context.Background()

	// CodeGeneration with no LLM provider deterministically errors → fires OnError.
	codeTask := task.NewTask(task.TaskTypeCodeGeneration, "x", "y", task.PriorityNormal)
	codeTask.Input = map[string]interface{}{"requirements": "anything"}

	func() {
		defer func() {
			if p := recover(); p != nil {
				rec.Record(stresschaos.Fatal,
					fmt.Sprintf("agent Execute propagated OnError handler panic to caller: %v", p))
			}
		}()
		_, err := ag.Execute(ctx, codeTask)
		if err != nil {
			rec.Record(stresschaos.Degraded, fmt.Sprintf("agent surfaced expected task error: %v", err))
		} else {
			rec.Record(stresschaos.Recovered, "agent Execute completed despite panicking OnError handler")
		}
	}()

	if atomic.LoadInt64(&coHandler) == 0 {
		rec.Record(stresschaos.Fatal, "panicking OnError handler starved the co-handler — not isolated")
	} else {
		rec.Record(stresschaos.Recovered, fmt.Sprintf(
			"co-handler survived panic (co=%d panic=%d)", coHandler, panicHandler))
	}

	// Agent must remain usable for a subsequent execution after the panic.
	analysis := newAnalysisTask("after-panic", "a\nb\nc")
	if _, err := ag.Execute(ctx, analysis); err != nil {
		rec.Record(stresschaos.Degraded, "post-panic analysis surfaced error: "+err.Error())
	} else {
		rec.Record(stresschaos.Recovered, "agent still usable after handler panic")
	}

	rec.AssertNoFatal()
	t.Log("agent survived OnError handler-panic injection")
}

// TestAgent_Chaos_ReentrantReadLockProbe stresses the exact pattern that
// deadlocks a non-reentrant sync.RWMutex: many readers taking the registry
// RLock (via ListAgents / GetByType-equivalent ListAgents + GetAgentStats)
// while writers (RegisterAgent) are continuously queued. Go's RWMutex blocks
// NEW readers once a writer is waiting; if any read path re-took the read lock
// while already holding it, a queued writer would wedge the whole registry.
// The run is wrapped in a hard timeout — a hang is reported as a Fatal deadlock.
func TestAgent_Chaos_ReentrantReadLockProbe(t *testing.T) {
	rec := stresschaos.NewChaosRecorder(t, "agent_reentrant_read_lock_probe", "state-corruption")
	c := NewCoordinator(nil)

	// Seed so reads have content to walk.
	for i := 0; i < 8; i++ {
		_ = c.RegisterAgent(newStressAgent(fmt.Sprintf("seed-%d", i)))
	}

	const goroutines = 16
	const iters = 400
	var wg sync.WaitGroup
	var reads, writes int64
	done := make(chan struct{})

	wg.Add(goroutines)
	go func() {
		defer close(done)
		for w := 0; w < goroutines; w++ {
			go func(id int) {
				defer wg.Done()
				defer func() {
					if p := recover(); p != nil {
						rec.Record(stresschaos.Fatal, fmt.Sprintf("goroutine %d panicked: %v", id, p))
					}
				}()
				for it := 0; it < iters; it++ {
					if (id+it)%4 == 0 {
						// Writer: continuously queues a write so readers contend
						// against a pending writer (the deadlock trigger).
						_ = c.RegisterAgent(newStressAgent(fmt.Sprintf("g%d-i%d", id, it)))
						atomic.AddInt64(&writes, 1)
					} else {
						// Reader paths that each take the registry RLock.
						_ = c.ListAgents()
						_ = c.GetAgentStats()
						atomic.AddInt64(&reads, 1)
					}
				}
			}(w)
		}
		wg.Wait()
	}()

	select {
	case <-done:
		rec.Record(stresschaos.Recovered, fmt.Sprintf(
			"registry survived reader/writer contention with no deadlock: reads=%d writes=%d",
			atomic.LoadInt64(&reads), atomic.LoadInt64(&writes)))
	case <-time.After(30 * time.Second):
		rec.Record(stresschaos.Fatal,
			"registry DEADLOCKED under reader/writer contention (reentrant read-lock?)")
	}

	rec.AssertNoFatal()
	t.Logf("reentrant-read-lock probe: reads=%d writes=%d", reads, writes)
}

// TestAgent_Chaos_ConcurrentRegistryChurn hammers the registry with concurrent
// Register / Unregister / List / stats from many goroutines. The registry must
// never panic or race and must end self-consistent — a fresh agent registered
// after the churn must be visible and routable. Run under -race.
func TestAgent_Chaos_ConcurrentRegistryChurn(t *testing.T) {
	rec := stresschaos.NewChaosRecorder(t, "agent_registry_churn", "state-corruption")
	c := NewCoordinator(nil)
	reg := c.registry // exercise the registry directly for Unregister churn

	const goroutines = 14
	const iters = 350
	var wg sync.WaitGroup
	var regs, unregs, lists int64

	for w := 0; w < goroutines; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if p := recover(); p != nil {
					rec.Record(stresschaos.Fatal, fmt.Sprintf("goroutine %d panicked: %v", id, p))
				}
			}()
			for it := 0; it < iters; it++ {
				ag := newStressAgent(fmt.Sprintf("g%d-i%d", id, it))
				switch (id + it) % 3 {
				case 0:
					if reg.Register(ag) == nil {
						atomic.AddInt64(&regs, 1)
						reg.Unregister(ag.ID())
						atomic.AddInt64(&unregs, 1)
					}
				case 1:
					_ = reg.GetByType(AgentTypeCoding)
					_ = reg.Count()
				default:
					_ = reg.List()
					atomic.AddInt64(&lists, 1)
				}
			}
		}(w)
	}
	wg.Wait()

	rec.Record(stresschaos.Recovered, fmt.Sprintf(
		"survived registry churn: regs=%d unregs=%d lists=%d", regs, unregs, lists))

	// Final coherence: a fresh agent must be visible + routable after churn.
	probe := newStressAgent("final-probe")
	if err := reg.Register(probe); err != nil {
		rec.Record(stresschaos.Fatal, "could not register final probe after churn: "+err.Error())
	}
	got, err := reg.Get("final-probe")
	if err != nil || got == nil {
		rec.Record(stresschaos.Fatal, "registry lost the final probe — map corrupted")
	} else {
		rec.Record(stresschaos.Recovered, "registry coherent after churn — final probe routable")
	}

	rec.AssertNoFatal()
	t.Logf("registry churn: regs=%d unregs=%d lists=%d count=%d", regs, unregs, lists, reg.Count())
}

// TestAgent_Chaos_CorruptTaskInput feeds structurally hostile task Input
// payloads through a real agent's execution path. Dispatch (and the no-LLM
// basicAnalysis path that reads Input["content"]) must reject/normalise without
// crashing on NaN/Inf, unmarshalable values, oversized strings, or nested maps.
func TestAgent_Chaos_CorruptTaskInput(t *testing.T) {
	c := NewCoordinator(&CoordinatorConfig{TaskTimeout: 5 * time.Second, EnableResilience: false})
	ag := newStressAgent("corrupt-consumer")
	if err := c.RegisterAgent(ag); err != nil {
		t.Fatalf("register: %v", err)
	}
	ctx := context.Background()

	corruptKinds := []map[string]interface{}{
		{"nan": math.NaN()},
		{"inf": math.Inf(1)},
		{"huge": makeHugeAgentString(1 << 16)},
		{"nested": map[string]interface{}{"a": map[string]interface{}{"b": math.NaN()}}},
		{"wrong_type": 12345},                 // content is not a string
		{"content": makeHugeAgentString(1e5)}, // oversized but valid content
	}
	payloads := make([][]byte, len(corruptKinds))
	for i, k := range corruptKinds {
		b, err := json.Marshal(k)
		if err != nil {
			b = []byte(fmt.Sprintf(`{"corrupt_index":%d}`, i))
		}
		payloads[i] = b
	}

	stresschaos.ChaosCorruptInputDuring(t, "agent_corrupt_task_input", payloads,
		func(input []byte) error {
			var m map[string]interface{}
			if err := json.Unmarshal(input, &m); err != nil {
				m = map[string]interface{}{"content": "fallback"}
			}
			tk := task.NewTask(task.TaskTypeAnalysis, "chaos", "corrupt input", task.PriorityNormal)
			tk.Input = m
			if err := c.SubmitTask(ctx, tk); err != nil {
				return err
			}
			ag.SetStatus(StatusIdle)
			// Execution may legitimately error (e.g. missing content) — that is
			// graceful rejection; a panic would be caught by the helper as Fatal.
			_, err := c.ExecuteTask(ctx, tk.ID)
			return err
		})
}

// TestAgent_Chaos_WorkerCancelMidLoop starts a real BaseAgent worker goroutine,
// lets it run, then cancels it mid-loop and asserts Stop() unwinds cleanly with
// no leaked goroutine and no hang. A worker that ignores cancellation would
// wedge Stop()'s wg.Wait() — surfaced as a Fatal deadlock by the helper.
func TestAgent_Chaos_WorkerCancelMidLoop(t *testing.T) {
	stresschaos.ChaosKillDuring(t, "agent_worker_cancel_mid_loop", 100*time.Millisecond,
		func(ctx context.Context, rec *stresschaos.ChaosRecorder) {
			ag := NewBaseAgent("worker", "worker", AgentTypeCoding, nil)
			if err := ag.Start(ctx); err != nil {
				rec.Record(stresschaos.Fatal, "agent failed to start: "+err.Error())
				return
			}
			rec.Record(stresschaos.Recovered, "agent worker started")

			// Block until the injected cancellation fires, then Stop() the agent.
			// Stop closes stopChan and waits for the worker goroutine to exit;
			// if the worker ignored both ctx and stopChan this would hang.
			<-ctx.Done()
			stopped := make(chan struct{})
			go func() {
				ag.Stop()
				close(stopped)
			}()
			select {
			case <-stopped:
				rec.Record(stresschaos.Recovered, "agent worker stopped cleanly after cancellation")
			case <-time.After(5 * time.Second):
				rec.Record(stresschaos.Fatal, "agent worker did not stop within 5s — leaked/wedged")
			}
		})
}

// TestAgent_Chaos_ResourcePressureRegistration runs registry registration +
// listing under bounded memory pressure. The orchestrator must complete without
// an OOM crash; the helper allocates bounded ballast and asserts graceful
// completion.
func TestAgent_Chaos_ResourcePressureRegistration(t *testing.T) {
	stresschaos.ChaosResourcePressureDuring(t, "agent_resource_pressure_registration", 32,
		func(rec *stresschaos.ChaosRecorder) {
			c := NewCoordinator(nil)
			for i := 0; i < 2000; i++ {
				if err := c.RegisterAgent(newStressAgent(fmt.Sprintf("rp-%d", i))); err != nil {
					rec.Record(stresschaos.Fatal, "register under pressure failed: "+err.Error())
					return
				}
			}
			if got := len(c.ListAgents()); got != 2000 {
				rec.Record(stresschaos.Fatal, fmt.Sprintf("registry lost agents under pressure: got %d want 2000", got))
				return
			}
			rec.Record(stresschaos.Recovered, "registered + listed 2000 agents under bounded memory pressure")
		})
}

// makeHugeAgentString returns an n-byte string of 'x' for oversized-payload chaos.
func makeHugeAgentString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = 'x'
	}
	return string(b)
}
