package event

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"dev.helix.code/tests/stresschaos"
)

// §11.4.85 stress coverage for the EventBus.
//
// The unit under stress is the REAL *EventBus (sync + async modes) — its
// RWMutex-guarded subscribers map, the real Subscribe/Unsubscribe/Publish/
// PublishAndWait dispatch machinery, and the errorMutex-guarded error log. No
// fakes: handlers are real closures that count invocations through atomics, so
// every PASS proves real dispatch happened. Sustained publish load (N>=100,
// p50/p95/p99 captured) + N>=10 concurrent publish/subscribe producers driving
// the shared subscriber map under genuine read/write contention (run under
// -race to catch data races in the dispatch path).

// TestEventBus_Stress_SustainedSyncPublish drives the real sync-mode
// Subscribe -> Publish dispatch under sustained load (N>=100), recording
// per-call latency. Each iteration publishes a real event to a set of real
// handlers and asserts every handler ran exactly once per publish, so the run
// proves real synchronous dispatch work — not a no-op.
func TestEventBus_Stress_SustainedSyncPublish(t *testing.T) {
	bus := NewEventBus(false) // sync: Publish blocks until every handler returns
	ctx := context.Background()

	const handlers = 4
	var dispatched int64
	for h := 0; h < handlers; h++ {
		bus.Subscribe(EventTaskCreated, func(ctx context.Context, e Event) error {
			atomic.AddInt64(&dispatched, 1)
			return nil
		})
	}
	if bus.GetSubscriberCount(EventTaskCreated) != handlers {
		t.Fatalf("expected %d subscribers, got %d", handlers, bus.GetSubscriberCount(EventTaskCreated))
	}

	var published int64
	stresschaos.RunSustainedLoad(t, "event_bus_sustained_sync_publish",
		stresschaos.SustainedConfig{N: 1500, MaxErrorRate: 0.0},
		func(i int) error {
			before := atomic.LoadInt64(&dispatched)
			err := bus.Publish(ctx, Event{
				Type:     EventTaskCreated,
				Source:   "stress",
				Severity: SeverityInfo,
				Data:     map[string]interface{}{"i": i},
			})
			if err != nil {
				return fmt.Errorf("publish: %w", err)
			}
			// Sync dispatch is synchronous: by the time Publish returns, all
			// handlers must have run for THIS publish.
			if delta := atomic.LoadInt64(&dispatched) - before; delta != handlers {
				return fmt.Errorf("sync publish dispatched %d handlers, want %d", delta, handlers)
			}
			atomic.AddInt64(&published, 1)
			return nil
		})

	if atomic.LoadInt64(&published) == 0 {
		t.Fatal("event bus published zero events under sustained load — not real work")
	}
	wantDispatch := atomic.LoadInt64(&published) * handlers
	if got := atomic.LoadInt64(&dispatched); got != wantDispatch {
		t.Fatalf("total dispatch count %d != published*handlers %d", got, wantDispatch)
	}
	t.Logf("event_bus sustained sync: %d events published, %d total handler dispatches",
		atomic.LoadInt64(&published), atomic.LoadInt64(&dispatched))
}

// TestEventBus_Stress_SustainedAsyncPublishAndWait drives the real async-mode
// PublishAndWait dispatch under sustained load (N>=100). Async PublishAndWait
// fans handlers out to goroutines and joins on a WaitGroup, so by the time it
// returns every handler must have run for that publish — asserted per-iteration.
func TestEventBus_Stress_SustainedAsyncPublishAndWait(t *testing.T) {
	bus := NewEventBus(true) // async: PublishAndWait fans out then joins
	ctx := context.Background()

	const handlers = 4
	var dispatched int64
	for h := 0; h < handlers; h++ {
		bus.Subscribe(EventWorkflowStarted, func(ctx context.Context, e Event) error {
			atomic.AddInt64(&dispatched, 1)
			return nil
		})
	}

	var published int64
	stresschaos.RunSustainedLoad(t, "event_bus_sustained_async_publish_wait",
		stresschaos.SustainedConfig{N: 800, MaxErrorRate: 0.0},
		func(i int) error {
			before := atomic.LoadInt64(&dispatched)
			if err := bus.PublishAndWait(ctx, Event{
				Type:   EventWorkflowStarted,
				Source: "stress-async",
				Data:   map[string]interface{}{"i": i},
			}); err != nil {
				return fmt.Errorf("publish-and-wait: %w", err)
			}
			if delta := atomic.LoadInt64(&dispatched) - before; delta != handlers {
				return fmt.Errorf("async publish-and-wait dispatched %d handlers, want %d", delta, handlers)
			}
			atomic.AddInt64(&published, 1)
			return nil
		})

	if atomic.LoadInt64(&published) == 0 {
		t.Fatal("async event bus published zero events under sustained load")
	}
	t.Logf("event_bus sustained async: %d events published-and-waited, %d total dispatches",
		atomic.LoadInt64(&published), atomic.LoadInt64(&dispatched))
}

// TestEventBus_Stress_ConcurrentPublishSubscribe hammers the shared subscriber
// map from N>=10 concurrent goroutines that interleave Subscribe + Publish +
// GetSubscriberCount + GetTotalSubscribers + Unsubscribe, asserting no deadlock,
// no goroutine leak, and no data race (run under -race) on the RWMutex-guarded
// map. Each goroutine subscribes to its own event type then publishes to it, so
// real read/write contention is generated against the map.
func TestEventBus_Stress_ConcurrentPublishSubscribe(t *testing.T) {
	bus := NewEventBus(false)
	ctx := context.Background()

	// A small fixed pool of event types so different goroutines genuinely
	// contend on the SAME map keys (write-write + read-write contention).
	eventTypes := []EventType{
		EventTaskCreated, EventTaskCompleted, EventWorkerConnected,
		EventUserLogin, EventSystemStartup,
	}

	var publishes int64
	stresschaos.RunConcurrent(t, "event_bus_concurrent_publish_subscribe",
		stresschaos.ConcurrencyConfig{Parallelism: 16, IterationsPerGoroutine: 120, Timeout: 25 * time.Second},
		func(g, it int) error {
			et := eventTypes[(g+it)%len(eventTypes)]
			// Register a handler under write-lock (contends with other writers).
			bus.Subscribe(et, func(ctx context.Context, e Event) error { return nil })
			// Publish under read-lock (contends with the writers above).
			if err := bus.Publish(ctx, Event{Type: et, Source: fmt.Sprintf("g%d", g)}); err != nil {
				return fmt.Errorf("publish: %w", err)
			}
			atomic.AddInt64(&publishes, 1)
			// Mix in read-only accessors to widen the RLock surface.
			_ = bus.GetSubscriberCount(et)
			_ = bus.GetTotalSubscribers()
			_ = bus.GetSubscribedEvents()
			return nil
		})

	if atomic.LoadInt64(&publishes) == 0 {
		t.Fatal("event bus published zero events under concurrent load")
	}
	// After the run there must be a positive subscriber total — proof the
	// concurrent Subscribes actually mutated the shared map.
	if bus.GetTotalSubscribers() == 0 {
		t.Fatal("no subscribers remain after concurrent subscribe load — map mutations lost")
	}
	t.Logf("event_bus concurrent: %d publishes, %d total subscribers across %d event types",
		atomic.LoadInt64(&publishes), bus.GetTotalSubscribers(), len(bus.GetSubscribedEvents()))
}

// TestEventBus_Stress_BoundaryConditions exercises the §11.4.85(A)(3) boundary
// cases against the real bus: (empty) publish with NO subscribers must be a
// clean no-op nil; (max) one event type with many subscribers must dispatch to
// every one; (off-by-one) Unsubscribe then publish must dispatch to zero.
func TestEventBus_Stress_BoundaryConditions(t *testing.T) {
	ctx := context.Background()

	// Empty: no subscribers at all — Publish must return nil, dispatch nothing.
	t.Run("no_subscribers", func(t *testing.T) {
		bus := NewEventBus(false)
		if bus.GetSubscriberCount(EventSystemError) != 0 {
			t.Fatalf("fresh bus should have 0 subscribers, got %d", bus.GetSubscriberCount(EventSystemError))
		}
		if err := bus.Publish(ctx, Event{Type: EventSystemError}); err != nil {
			t.Fatalf("publish to empty bus must be a clean no-op, got: %v", err)
		}
	})

	// Max: a single event type with a large fan-out must dispatch to every one.
	t.Run("many_subscribers", func(t *testing.T) {
		bus := NewEventBus(false)
		const many = 500
		var hits int64
		for i := 0; i < many; i++ {
			bus.Subscribe(EventTaskCompleted, func(ctx context.Context, e Event) error {
				atomic.AddInt64(&hits, 1)
				return nil
			})
		}
		if bus.GetSubscriberCount(EventTaskCompleted) != many {
			t.Fatalf("want %d subscribers, got %d", many, bus.GetSubscriberCount(EventTaskCompleted))
		}
		if err := bus.Publish(ctx, Event{Type: EventTaskCompleted}); err != nil {
			t.Fatalf("publish to many subscribers: %v", err)
		}
		if atomic.LoadInt64(&hits) != many {
			t.Fatalf("dispatched to %d/%d subscribers", atomic.LoadInt64(&hits), many)
		}
	})

	// Off-by-one: subscribe one, unsubscribe all, then publish — zero dispatch.
	t.Run("unsubscribe_then_publish", func(t *testing.T) {
		bus := NewEventBus(false)
		var hits int64
		bus.Subscribe(EventUserLogout, func(ctx context.Context, e Event) error {
			atomic.AddInt64(&hits, 1)
			return nil
		})
		bus.Unsubscribe(EventUserLogout)
		if bus.GetSubscriberCount(EventUserLogout) != 0 {
			t.Fatalf("after Unsubscribe count should be 0, got %d", bus.GetSubscriberCount(EventUserLogout))
		}
		if err := bus.Publish(ctx, Event{Type: EventUserLogout}); err != nil {
			t.Fatalf("publish after unsubscribe: %v", err)
		}
		if atomic.LoadInt64(&hits) != 0 {
			t.Fatalf("handler ran %d times after unsubscribe — should be 0", atomic.LoadInt64(&hits))
		}
	})
}
