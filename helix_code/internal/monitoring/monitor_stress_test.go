package monitoring

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"dev.helix.code/tests/stresschaos"
)

// §11.4.85(A) stress coverage for the monitoring Monitor.
//
// The unit under stress is the REAL *Monitor — its sync.RWMutex-guarded
// metrics map + collectors slice, the real AddCollector / CollectMetrics /
// GetMetric / GetAllMetrics / HealthCheck machinery. No fakes for the unit
// under test: the Monitor itself is real and its mutex is genuinely
// contended. Collectors are real implementations of the Collector interface
// (counterCollector below) that mutate atomics on every Collect(), so every
// PASS proves real collection work happened — not a no-op.
//
// Covered: sustained CollectMetrics load (N>=100, p50/p95/p99 captured),
// N>=10 concurrent goroutines hammering AddCollector / CollectMetrics /
// GetMetric / GetAllMetrics under genuine read/write contention (run under
// -race to catch data races in the dispatch + map-copy paths), and boundary
// conditions (no collectors, huge metric cardinality, off-by-one overwrite).

// counterCollector is a REAL Collector implementation (not a mock/fake) used
// to drive genuine collection work under stress. Each Collect() increments an
// atomic so the test can prove the Monitor actually invoked it, and emits a
// configurable number of distinct metric keys to exercise the map writes.
type counterCollector struct {
	name      string
	collects  int64
	keyPrefix string
	keyCount  int
	baseValue int64
}

func (c *counterCollector) Name() string { return c.name }

func (c *counterCollector) Collect() (map[string]interface{}, error) {
	n := atomic.AddInt64(&c.collects, 1)
	out := make(map[string]interface{}, c.keyCount)
	for k := 0; k < c.keyCount; k++ {
		out[fmt.Sprintf("%s_%d", c.keyPrefix, k)] = atomic.LoadInt64(&c.baseValue) + n
	}
	return out, nil
}

// TestMonitor_Stress_SustainedCollect drives the real AddCollector ->
// CollectMetrics -> GetMetric pipeline under sustained load (N>=100),
// recording per-call latency. Each iteration collects from a set of real
// collectors and asserts the metric was actually written + readable, so the
// run proves real collection + map-write + map-read work, not a no-op.
func TestMonitor_Stress_SustainedCollect(t *testing.T) {
	m := NewMonitor()
	ctx := context.Background()

	const collectors = 4
	const keysEach = 8
	cs := make([]*counterCollector, collectors)
	for i := 0; i < collectors; i++ {
		cs[i] = &counterCollector{
			name:      fmt.Sprintf("collector_%d", i),
			keyPrefix: fmt.Sprintf("c%d_metric", i),
			keyCount:  keysEach,
		}
		m.AddCollector(cs[i])
	}

	var collected int64
	stresschaos.RunSustainedLoad(t, "monitor_sustained_collect",
		stresschaos.SustainedConfig{N: 1200, MaxErrorRate: 0.0},
		func(i int) error {
			if err := m.CollectMetrics(ctx); err != nil {
				return fmt.Errorf("collect: %w", err)
			}
			// Prove the collection actually wrote a readable metric for THIS
			// round: collector 0's first key must exist and be non-nil.
			v, ok := m.GetMetric("c0_metric_0")
			if !ok {
				return fmt.Errorf("metric c0_metric_0 absent after collect %d", i)
			}
			if v == nil {
				return fmt.Errorf("metric c0_metric_0 nil after collect %d", i)
			}
			atomic.AddInt64(&collected, 1)
			return nil
		})

	if atomic.LoadInt64(&collected) == 0 {
		t.Fatal("monitor collected zero times under sustained load — not real work")
	}
	// Every collector must have been invoked at least once per CollectMetrics
	// call — proof the loop drove every collector, not just a subset.
	for i, c := range cs {
		if got := atomic.LoadInt64(&c.collects); got < atomic.LoadInt64(&collected) {
			t.Fatalf("collector %d invoked %d times, want >= %d", i, got, atomic.LoadInt64(&collected))
		}
	}
	all := m.GetAllMetrics()
	if len(all) != collectors*keysEach {
		t.Fatalf("expected %d distinct metrics, got %d", collectors*keysEach, len(all))
	}
	t.Logf("monitor sustained: %d collect rounds, %d distinct metrics across %d collectors",
		atomic.LoadInt64(&collected), len(all), collectors)
}

// TestMonitor_Stress_ConcurrentCollectAndRead hammers the shared metrics map +
// collectors slice from N>=10 concurrent goroutines that interleave
// AddCollector + CollectMetrics + GetMetric + GetAllMetrics, asserting no
// deadlock, no goroutine leak, and no data race (run under -race) on the
// RWMutex-guarded state. The interleaved write-lock (CollectMetrics/
// AddCollector) vs read-lock (GetMetric/GetAllMetrics) traffic generates real
// read/write contention against the same Monitor.
func TestMonitor_Stress_ConcurrentCollectAndRead(t *testing.T) {
	m := NewMonitor()
	ctx := context.Background()

	// Seed one collector so reads have something to find from the start.
	seed := &counterCollector{name: "seed", keyPrefix: "seed_metric", keyCount: 4}
	m.AddCollector(seed)

	var collects, reads int64
	stresschaos.RunConcurrent(t, "monitor_concurrent_collect_read",
		stresschaos.ConcurrencyConfig{Parallelism: 16, IterationsPerGoroutine: 120, Timeout: 25 * time.Second},
		func(g, it int) error {
			switch (g + it) % 4 {
			case 0:
				// Write path: register a new collector (contends with collectors-slice append).
				m.AddCollector(&counterCollector{
					name:      fmt.Sprintf("g%d_it%d", g, it),
					keyPrefix: fmt.Sprintf("g%d_metric", g),
					keyCount:  2,
				})
			case 1:
				// Write path: collect from all collectors (write-locks the map).
				if err := m.CollectMetrics(ctx); err != nil {
					return fmt.Errorf("collect: %w", err)
				}
				atomic.AddInt64(&collects, 1)
			case 2:
				// Read path: single-key read (read-locks the map).
				_, _ = m.GetMetric("seed_metric_0")
				atomic.AddInt64(&reads, 1)
			default:
				// Read path: full snapshot copy (read-locks + ranges the map).
				_ = m.GetAllMetrics()
				atomic.AddInt64(&reads, 1)
			}
			return nil
		})

	if atomic.LoadInt64(&collects) == 0 || atomic.LoadInt64(&reads) == 0 {
		t.Fatalf("concurrent run did not exercise both paths: collects=%d reads=%d",
			atomic.LoadInt64(&collects), atomic.LoadInt64(&reads))
	}
	// After the run the seed metric must be present + the map self-consistent.
	if _, ok := m.GetMetric("seed_metric_0"); !ok {
		t.Fatal("seed metric lost after concurrent churn — map mutations corrupted")
	}
	t.Logf("monitor concurrent: %d collects, %d reads, %d metrics final",
		atomic.LoadInt64(&collects), atomic.LoadInt64(&reads), len(m.GetAllMetrics()))
}

// TestMonitor_Stress_BoundaryConditions exercises the §11.4.85(A)(3) boundary
// cases against the real Monitor: (empty) collect with NO collectors must be a
// clean no-op nil leaving zero metrics; (max) one collector emitting huge
// cardinality must write every key; (off-by-one) two collectors with the same
// key — last write wins.
func TestMonitor_Stress_BoundaryConditions(t *testing.T) {
	ctx := context.Background()

	// Empty: no collectors — CollectMetrics is a clean no-op, HealthCheck fails.
	t.Run("no_collectors", func(t *testing.T) {
		m := NewMonitor()
		if err := m.CollectMetrics(ctx); err != nil {
			t.Fatalf("collect with no collectors must be a clean no-op, got: %v", err)
		}
		if got := len(m.GetAllMetrics()); got != 0 {
			t.Fatalf("no-collector monitor should have 0 metrics, got %d", got)
		}
		if err := m.HealthCheck(); err == nil {
			t.Fatal("HealthCheck must fail when no collectors are registered")
		}
	})

	// Max: a single collector with very high metric cardinality must write every key.
	t.Run("huge_cardinality", func(t *testing.T) {
		m := NewMonitor()
		const huge = 5000
		m.AddCollector(&counterCollector{name: "huge", keyPrefix: "huge_metric", keyCount: huge})
		if err := m.CollectMetrics(ctx); err != nil {
			t.Fatalf("collect huge cardinality: %v", err)
		}
		all := m.GetAllMetrics()
		if len(all) != huge {
			t.Fatalf("want %d metrics, got %d", huge, len(all))
		}
		// Spot-check first + last keys are present.
		if _, ok := m.GetMetric("huge_metric_0"); !ok {
			t.Fatal("first huge-cardinality key missing")
		}
		if _, ok := m.GetMetric(fmt.Sprintf("huge_metric_%d", huge-1)); !ok {
			t.Fatal("last huge-cardinality key missing")
		}
	})

	// Off-by-one: two collectors emit the SAME key — last collector in the
	// slice wins per the documented overwrite semantics.
	t.Run("same_key_last_write_wins", func(t *testing.T) {
		m := NewMonitor()
		// Both emit key "shared_metric_0"; second registered overwrites first.
		first := &counterCollector{name: "first", keyPrefix: "shared_metric", keyCount: 1, baseValue: 1000}
		second := &counterCollector{name: "second", keyPrefix: "shared_metric", keyCount: 1, baseValue: 9000}
		m.AddCollector(first)
		m.AddCollector(second)
		if err := m.CollectMetrics(ctx); err != nil {
			t.Fatalf("collect: %v", err)
		}
		v, ok := m.GetMetric("shared_metric_0")
		if !ok {
			t.Fatal("shared metric absent")
		}
		// second has baseValue 9000 and was collected last, so the value must
		// be >= 9000 (9000 + its collect count), proving last-write-wins.
		if iv, ok := v.(int64); !ok || iv < 9000 {
			t.Fatalf("expected last-write-wins value >= 9000, got %v", v)
		}
	})
}
