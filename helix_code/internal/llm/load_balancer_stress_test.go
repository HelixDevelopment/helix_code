package llm

import (
	"context"
	"testing"
	"time"

	"dev.helix.code/tests/stresschaos"
)

// §11.4.85 stress coverage for the LLM load balancer.
//
// The unit under stress is the REAL LoadBalancer's RWMutex-guarded selection +
// stats machinery, driven against a real AutoLLMManager populated with healthy
// providers via the package's existing createMockAutoLLMManager() helper (the
// production GetStatus path — no fakes; the LoadBalancer logic itself is real).
// collectStats runs in the background via Start() — this is exactly where
// concurrent-access defects surface, so these tests MUST run under -race.

// startedStressLB builds a real LoadBalancer over a real manager with healthy
// providers and starts the background collectStats goroutine bound to a
// cancellable context, registering deterministic cleanup (§11.4.14).
func startedStressLB(t *testing.T) *LoadBalancer {
	t.Helper()
	manager := createMockAutoLLMManager() // 2 healthy + 1 unhealthy + 1 stopped
	lb := NewLoadBalancer(manager)
	ctx, cancel := context.WithCancel(context.Background())
	if err := lb.Start(ctx); err != nil { // launches collectStats goroutine
		t.Fatalf("load balancer Start failed: %v", err)
	}
	t.Cleanup(func() { cancel(); lb.Stop() })
	return lb
}

// TestLoadBalancer_Stress_SustainedSelect drives SelectOptimalProvider under
// sustained load (N>=100) while collectStats runs, recording per-selection
// latency. The manager has healthy providers, so a nil selection is a real
// failure surfaced to the harness.
func TestLoadBalancer_Stress_SustainedSelect(t *testing.T) {
	lb := startedStressLB(t)

	stresschaos.RunSustainedLoad(t, "load_balancer_sustained_select",
		stresschaos.SustainedConfig{N: 5000, MaxErrorRate: 0.0},
		func(i int) error {
			p := lb.SelectOptimalProvider(context.Background())
			if p == nil {
				return context.DeadlineExceeded // surfaced as an error to the harness
			}
			return nil
		})
}

// TestLoadBalancer_Stress_ConcurrentSelect hammers SelectOptimalProvider from
// N>=10 goroutines concurrently with the background collectStats goroutine + a
// concurrent GetStats reader. This is the direct regression guard for concurrent
// selection/stats access — it MUST pass clean under -race with no deadlock/leak.
func TestLoadBalancer_Stress_ConcurrentSelect(t *testing.T) {
	lb := startedStressLB(t)

	stresschaos.RunConcurrent(t, "load_balancer_concurrent_select",
		stresschaos.ConcurrencyConfig{Parallelism: 24, IterationsPerGoroutine: 200, Timeout: 20 * time.Second},
		func(g, it int) error {
			p := lb.SelectOptimalProvider(context.Background())
			if p == nil {
				return context.DeadlineExceeded
			}
			_ = lb.GetStats() // concurrent stats read widens the race surface
			return nil
		})
}
