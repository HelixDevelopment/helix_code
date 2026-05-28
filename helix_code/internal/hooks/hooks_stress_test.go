package hooks

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"dev.helix.code/tests/stresschaos"
)

// §11.4.85 stress coverage for the hooks Manager + Executor.
//
// The unit under stress is the REAL *Manager (and its real *Executor) — its
// RWMutex-guarded hooks/hooksAll maps, the real Register/Unregister/Trigger
// dispatch machinery (sync + async), and the priority-sorting executor. No
// fakes: handlers are real closures that count invocations through atomics, so
// every PASS proves real dispatch happened. Sustained trigger load (N>=100,
// p50/p95/p99 captured) + N>=10 concurrent register/unregister/trigger
// producers driving the shared maps under genuine read/write contention (run
// under -race to catch data races in the dispatch path).

// TestHooks_Stress_SustainedSyncTrigger drives the real TriggerSync dispatch
// under sustained load (N>=100), recording per-call latency. Each iteration
// triggers a real event to a set of real registered hooks and asserts every
// hook ran exactly once per trigger, so the run proves real synchronous
// dispatch work — not a no-op.
func TestHooks_Stress_SustainedSyncTrigger(t *testing.T) {
	mgr := NewManager()
	ctx := context.Background()

	const handlers = 4
	var dispatched int64
	for h := 0; h < handlers; h++ {
		hook := NewHook(fmt.Sprintf("sync-stress-%d", h), HookTypeBeforeTask,
			func(ctx context.Context, e *Event) error {
				atomic.AddInt64(&dispatched, 1)
				return nil
			})
		if err := mgr.Register(hook); err != nil {
			t.Fatalf("register hook %d: %v", h, err)
		}
	}
	if mgr.CountByType(HookTypeBeforeTask) != handlers {
		t.Fatalf("expected %d hooks, got %d", handlers, mgr.CountByType(HookTypeBeforeTask))
	}

	var triggered int64
	stresschaos.RunSustainedLoad(t, "hooks_sustained_sync_trigger",
		stresschaos.SustainedConfig{N: 1500, MaxErrorRate: 0.0},
		func(i int) error {
			before := atomic.LoadInt64(&dispatched)
			results := mgr.TriggerSync(ctx, HookTypeBeforeTask)
			// Sync dispatch is synchronous: by the time TriggerSync returns, all
			// hooks must have run for THIS trigger.
			if delta := atomic.LoadInt64(&dispatched) - before; delta != handlers {
				return fmt.Errorf("sync trigger dispatched %d handlers, want %d", delta, handlers)
			}
			if len(results) != handlers {
				return fmt.Errorf("sync trigger returned %d results, want %d", len(results), handlers)
			}
			for _, r := range results {
				if r.Status != StatusCompleted {
					return fmt.Errorf("hook %s status %s, want completed", r.HookName, r.Status)
				}
			}
			atomic.AddInt64(&triggered, 1)
			return nil
		})

	if atomic.LoadInt64(&triggered) == 0 {
		t.Fatal("hooks manager triggered zero events under sustained load — not real work")
	}
	wantDispatch := atomic.LoadInt64(&triggered) * handlers
	if got := atomic.LoadInt64(&dispatched); got != wantDispatch {
		t.Fatalf("total dispatch count %d != triggered*handlers %d", got, wantDispatch)
	}
	t.Logf("hooks sustained sync: %d events triggered, %d total handler dispatches",
		atomic.LoadInt64(&triggered), atomic.LoadInt64(&dispatched))
}

// TestHooks_Stress_SustainedAsyncTriggerAndWait drives the real async-mode
// TriggerAndWait dispatch under sustained load (N>=100). Async hooks fan out to
// goroutines and the manager joins on the executor WaitGroup, so by the time
// TriggerAndWait returns every async hook must have run for that trigger —
// asserted per-iteration.
func TestHooks_Stress_SustainedAsyncTriggerAndWait(t *testing.T) {
	mgr := NewManager()
	ctx := context.Background()

	const handlers = 4
	var dispatched int64
	for h := 0; h < handlers; h++ {
		hook := NewAsyncHook(fmt.Sprintf("async-stress-%d", h), HookTypeAfterTask,
			func(ctx context.Context, e *Event) error {
				atomic.AddInt64(&dispatched, 1)
				return nil
			})
		if err := mgr.Register(hook); err != nil {
			t.Fatalf("register async hook %d: %v", h, err)
		}
	}

	var triggered int64
	stresschaos.RunSustainedLoad(t, "hooks_sustained_async_trigger_wait",
		stresschaos.SustainedConfig{N: 800, MaxErrorRate: 0.0},
		func(i int) error {
			before := atomic.LoadInt64(&dispatched)
			mgr.TriggerAndWait(ctx, HookTypeAfterTask)
			if delta := atomic.LoadInt64(&dispatched) - before; delta != handlers {
				return fmt.Errorf("async trigger-and-wait dispatched %d handlers, want %d", delta, handlers)
			}
			atomic.AddInt64(&triggered, 1)
			return nil
		})

	if atomic.LoadInt64(&triggered) == 0 {
		t.Fatal("async hooks manager triggered zero events under sustained load")
	}
	t.Logf("hooks sustained async: %d events triggered-and-waited, %d total dispatches",
		atomic.LoadInt64(&triggered), atomic.LoadInt64(&dispatched))
}

// TestHooks_Stress_ConcurrentRegisterTrigger hammers the shared hooks maps from
// N>=10 concurrent goroutines that interleave Register + Trigger +
// CountByType + GetByType + Unregister, asserting no deadlock, no goroutine
// leak, and no data race (run under -race) on the RWMutex-guarded maps. Each
// goroutine registers a uniquely-IDed hook under its own type then triggers it,
// so real read/write contention is generated against the maps.
func TestHooks_Stress_ConcurrentRegisterTrigger(t *testing.T) {
	mgr := NewManager()
	ctx := context.Background()

	// A small fixed pool of hook types so different goroutines genuinely
	// contend on the SAME map keys (write-write + read-write contention).
	hookTypes := []HookType{
		HookTypeBeforeTask, HookTypeAfterTask, HookTypeBeforeLLM,
		HookTypeAfterLLM, HookTypeOnError,
	}

	var registers, triggers int64
	stresschaos.RunConcurrent(t, "hooks_concurrent_register_trigger",
		stresschaos.ConcurrencyConfig{Parallelism: 16, IterationsPerGoroutine: 120, Timeout: 25 * time.Second},
		func(g, it int) error {
			ht := hookTypes[(g+it)%len(hookTypes)]
			// Unique ID per (goroutine,iter) so concurrent Registers never
			// collide on the duplicate-ID check — they genuinely append to the
			// same type slice under the write-lock (contends with other writers).
			hook := NewHook(fmt.Sprintf("g%d-it%d", g, it), ht,
				func(ctx context.Context, e *Event) error { return nil })
			if err := mgr.Register(hook); err != nil {
				return fmt.Errorf("register: %w", err)
			}
			atomic.AddInt64(&registers, 1)
			// Trigger under read-lock (contends with the writers above).
			_ = mgr.Trigger(ctx, ht)
			atomic.AddInt64(&triggers, 1)
			// Mix in read-only accessors to widen the RLock surface.
			_ = mgr.CountByType(ht)
			_ = mgr.GetByType(ht)
			_ = mgr.Count()
			// Remove our own hook so the map churns both directions.
			_ = mgr.Unregister(hook.ID)
			return nil
		})

	if atomic.LoadInt64(&triggers) == 0 {
		t.Fatal("hooks manager triggered zero events under concurrent load")
	}
	if atomic.LoadInt64(&registers) == 0 {
		t.Fatal("hooks manager registered zero hooks under concurrent load")
	}
	t.Logf("hooks concurrent: %d registers, %d triggers, final count=%d",
		atomic.LoadInt64(&registers), atomic.LoadInt64(&triggers), mgr.Count())
}

// TestHooks_Stress_BoundaryConditions exercises the §11.4.85(A)(3) boundary
// cases against the real manager: (empty) trigger with NO hooks must be a clean
// no-op empty slice; (max) one hook type with many hooks must dispatch to every
// one in priority order; (off-by-one) Unregister then trigger must dispatch to
// zero.
func TestHooks_Stress_BoundaryConditions(t *testing.T) {
	ctx := context.Background()

	// Empty: no hooks at all — Trigger must return an empty slice, dispatch nothing.
	t.Run("no_hooks", func(t *testing.T) {
		mgr := NewManager()
		if mgr.CountByType(HookTypeOnError) != 0 {
			t.Fatalf("fresh manager should have 0 hooks, got %d", mgr.CountByType(HookTypeOnError))
		}
		results := mgr.TriggerSync(ctx, HookTypeOnError)
		if len(results) != 0 {
			t.Fatalf("trigger on empty manager must dispatch nothing, got %d results", len(results))
		}
	})

	// Max: a single hook type with a large fan-out must dispatch to every one.
	t.Run("many_hooks", func(t *testing.T) {
		mgr := NewManager()
		const many = 500
		var hits int64
		for i := 0; i < many; i++ {
			hook := NewHook(fmt.Sprintf("many-%d", i), HookTypeAfterTest,
				func(ctx context.Context, e *Event) error {
					atomic.AddInt64(&hits, 1)
					return nil
				})
			if err := mgr.Register(hook); err != nil {
				t.Fatalf("register many[%d]: %v", i, err)
			}
		}
		if mgr.CountByType(HookTypeAfterTest) != many {
			t.Fatalf("want %d hooks, got %d", many, mgr.CountByType(HookTypeAfterTest))
		}
		results := mgr.TriggerSync(ctx, HookTypeAfterTest)
		if len(results) != many {
			t.Fatalf("trigger returned %d results, want %d", len(results), many)
		}
		if atomic.LoadInt64(&hits) != many {
			t.Fatalf("dispatched to %d/%d hooks", atomic.LoadInt64(&hits), many)
		}
	})

	// Priority ordering: hooks must execute highest-priority-first.
	t.Run("priority_order", func(t *testing.T) {
		mgr := NewManager()
		var order []HookPriority
		var mu sync.Mutex
		add := func(p HookPriority) {
			hook := NewHookWithPriority(fmt.Sprintf("prio-%d", p), HookTypeOnSuccess,
				func(ctx context.Context, e *Event) error {
					mu.Lock()
					order = append(order, p)
					mu.Unlock()
					return nil
				}, p)
			if err := mgr.Register(hook); err != nil {
				t.Fatalf("register prio %d: %v", p, err)
			}
		}
		// Register out of order.
		add(PriorityLow)
		add(PriorityHighest)
		add(PriorityNormal)
		mgr.TriggerSync(ctx, HookTypeOnSuccess)
		if len(order) != 3 {
			t.Fatalf("expected 3 executions, got %d", len(order))
		}
		// Highest first.
		if !(order[0] >= order[1] && order[1] >= order[2]) {
			t.Fatalf("hooks not executed in highest-priority-first order: %v", order)
		}
	})

	// Off-by-one: register one, unregister it, then trigger — zero dispatch.
	t.Run("unregister_then_trigger", func(t *testing.T) {
		mgr := NewManager()
		var hits int64
		hook := NewHook("transient", HookTypeOnError,
			func(ctx context.Context, e *Event) error {
				atomic.AddInt64(&hits, 1)
				return nil
			})
		if err := mgr.Register(hook); err != nil {
			t.Fatalf("register: %v", err)
		}
		if err := mgr.Unregister(hook.ID); err != nil {
			t.Fatalf("unregister: %v", err)
		}
		if mgr.CountByType(HookTypeOnError) != 0 {
			t.Fatalf("after Unregister count should be 0, got %d", mgr.CountByType(HookTypeOnError))
		}
		results := mgr.TriggerSync(ctx, HookTypeOnError)
		if len(results) != 0 {
			t.Fatalf("trigger after unregister returned %d results, want 0", len(results))
		}
		if atomic.LoadInt64(&hits) != 0 {
			t.Fatalf("handler ran %d times after unregister — should be 0", atomic.LoadInt64(&hits))
		}
	})
}
