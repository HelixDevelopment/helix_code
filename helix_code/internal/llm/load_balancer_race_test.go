package llm

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestLoadBalancer_DataRace_HXC012 reproduces HXC-012: a data race in
// load_balancer.go between the strategy selection read path
// (SelectOptimalProvider reading lb.currentStrategy / lb.strategies) and
// concurrent writers (SetStrategy writing lb.currentStrategy, and the
// background collectStats goroutine writing lb.stats fields).
//
// RED: before the fix this test FAILS under `go test -race` with a
// data-race report pointing at load_balancer.go — the strategy fields are
// read in SelectOptimalProvider without holding lb.mutex while SetStrategy
// holds the mutex when writing them.
//
// GREEN: after synchronizing the read path the test PASSES under -race
// with zero data-race reports and unchanged observable behaviour.
func TestLoadBalancer_DataRace_HXC012(t *testing.T) {
	manager := createMockAutoLLMManager()
	lb := NewLoadBalancer(manager)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the load balancer — this launches the background collectStats
	// goroutine that mutates lb.stats concurrently with the selection path.
	err := lb.Start(ctx)
	assert.NoError(t, err)
	defer lb.Stop()

	const goroutines = 16
	const iterations = 400

	strategies := []string{
		"round_robin",
		"least_connections",
		"response_time",
		"weighted",
		"performance_based",
	}

	var wg sync.WaitGroup

	// Reader goroutines: hammer SelectOptimalProvider — which reads
	// lb.currentStrategy and lb.strategies on the hot path.
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				_ = lb.SelectOptimalProvider(ctx)
				_ = lb.GetStats()
			}
		}()
	}

	// Writer goroutines: concurrently flip the strategy, which writes
	// lb.currentStrategy and lb.stats.Strategy.
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(seed int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				_ = lb.SetStrategy(strategies[(seed+i)%len(strategies)])
			}
		}(g)
	}

	wg.Wait()

	// Behaviour invariant: after the storm the load balancer is still in a
	// consistent, usable state — a valid strategy is set and selection works.
	stats := lb.GetStats()
	assert.NotNil(t, stats)
	assert.Contains(t, strategies, stats.Strategy)

	selected := lb.SelectOptimalProvider(ctx)
	assert.NotNil(t, selected, "load balancer must still select a provider after concurrent load")
	assert.Contains(t, []string{"healthy1", "healthy2"}, selected.Name)
}
