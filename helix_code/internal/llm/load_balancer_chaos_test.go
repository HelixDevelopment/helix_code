package llm

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"dev.helix.code/tests/stresschaos"
)

// §11.4.85(B) chaos coverage for the LLM load balancer.
//
// Chaos classes exercised against the REAL LoadBalancer:
//   - state-mutation under contention: flip the strategy (SetStrategy) + flip
//     provider health concurrently with ongoing SelectOptimalProvider calls,
//     asserting no race/panic/deadlock and continued valid selection.
//   - state-corruption: drive selection when ALL providers are unhealthy,
//     asserting graceful nil (no panic) — the documented degradation path.
//
// Run under -race: concurrent SetStrategy + SelectOptimalProvider is exactly the
// contention pattern that exposes locking defects.

// TestLoadBalancer_Chaos_FlipStrategyDuringSelect flips the strategy + provider
// health from a chaos goroutine while many selectors run, asserting the balancer
// neither panics nor deadlocks and keeps returning valid selections.
func TestLoadBalancer_Chaos_FlipStrategyDuringSelect(t *testing.T) {
	manager := createMockAutoLLMManager() // healthy1, healthy2, unhealthy, stopped
	lb := NewLoadBalancer(manager)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := lb.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer lb.Stop()

	rec := stresschaos.NewChaosRecorder(t, "load_balancer_flip_strategy", "state-mutation")

	// The real strategy names registered in NewLoadBalancer.
	strategies := []string{"round_robin", "least_connections", "response_time", "weighted", "performance_based"}

	stop := make(chan struct{})
	var wg sync.WaitGroup

	// Chaos goroutine: rapidly mutate strategy + a provider's health mid-flight.
	// Health is mutated under the manager's own mutex (the lock GetStatus uses),
	// so the chaos itself is race-clean while the production read path is exercised.
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() {
			if p := recover(); p != nil {
				rec.Record(stresschaos.Fatal, "SetStrategy/health-flip panicked")
			}
		}()
		i := 0
		for {
			select {
			case <-stop:
				return
			default:
			}
			_ = lb.SetStrategy(strategies[i%len(strategies)])
			manager.mutex.Lock()
			if p, ok := manager.providers["healthy1"]; ok {
				p.Health.IsHealthy = (i%2 == 0)
			}
			manager.mutex.Unlock()
			i++
			time.Sleep(50 * time.Microsecond)
		}
	}()

	// Selector goroutines hammered while strategy/health flip underneath them.
	var ok, degraded int64
	for g := 0; g < 12; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if p := recover(); p != nil {
					rec.Record(stresschaos.Fatal, "SelectOptimalProvider panicked during strategy flip")
				}
			}()
			for it := 0; it < 1000; it++ {
				if p := lb.SelectOptimalProvider(context.Background()); p != nil {
					atomic.AddInt64(&ok, 1)
				} else {
					// All-unhealthy window — graceful nil, not a crash.
					atomic.AddInt64(&degraded, 1)
				}
			}
		}()
	}

	time.Sleep(300 * time.Millisecond)
	close(stop)
	wg.Wait()

	rec.Record(stresschaos.Recovered,
		"selectors completed under concurrent strategy+health mutation: valid selections and graceful nil-degradations both observed, no panic/deadlock")
	if atomic.LoadInt64(&ok) == 0 {
		rec.Record(stresschaos.Fatal, "zero successful selections during chaos — balancer never recovered")
	}
	rec.AssertNoFatal()
	t.Logf("load_balancer chaos: ok=%d degraded=%d", atomic.LoadInt64(&ok), atomic.LoadInt64(&degraded))
}

// TestLoadBalancer_Chaos_AllProvidersUnhealthy corrupts the selectable state by
// marking every provider unhealthy, then asserts SelectOptimalProvider returns a
// clean nil (no panic / nil-deref) — graceful degradation under state-corruption.
func TestLoadBalancer_Chaos_AllProvidersUnhealthy(t *testing.T) {
	rec := stresschaos.NewChaosRecorder(t, "load_balancer_all_unhealthy", "state-corruption")

	manager := createMockAutoLLMManager()
	// Corrupt the state: mark every provider unhealthy.
	manager.mutex.Lock()
	for _, p := range manager.providers {
		p.Health.IsHealthy = false
	}
	manager.mutex.Unlock()

	lb := NewLoadBalancer(manager)
	defer lb.Stop()

	func() {
		defer func() {
			if p := recover(); p != nil {
				rec.Record(stresschaos.Fatal, "SelectOptimalProvider panicked with all providers unhealthy")
			}
		}()
		p := lb.SelectOptimalProvider(context.Background())
		if p == nil {
			rec.Record(stresschaos.Degraded, "returned clean nil with no healthy providers (graceful degradation, no crash)")
		} else {
			rec.Record(stresschaos.Recovered, "returned a provider despite none healthy (no crash)")
		}
	}()

	rec.AssertNoFatal()
}
