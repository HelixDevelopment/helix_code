package performance

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"dev.helix.code/tests/stresschaos"
)

// §11.4.85(B) chaos coverage for the PerformanceOptimizer.
//
// Chaos classes exercised against the REAL *PerformanceOptimizer (no fakes —
// real map, real atomic lifecycle flag, real metrics machinery):
//
//   - state-corruption under contention: StartProductionOptimization mutates the
//     shared optimizations map (po.optimizations[opt.Name] = opt) while many
//     goroutines concurrently read it via getOptimizationsByType. Without proper
//     mutex serialisation this is a `fatal error: concurrent map read and map
//     write` — an unrecoverable process crash. The optimizer MUST serialise the
//     map access so the run survives.
//   - process-death injection: StartProductionOptimization honours a context; the
//     context is cancelled mid-run. The optimizer MUST unwind without crashing,
//     deadlocking, or leaking the running flag (so a subsequent run can start).
//   - input-corruption: hostile/degenerate PerformanceConfig values (negative
//     targets, absurd thresholds) must not crash construction or metric reads.

// TestPerformanceOptimizer_Chaos_ConcurrentMapAccess runs one real
// StartProductionOptimization (which writes po.optimizations under load) while a
// fleet of goroutines hammers getOptimizationsByType reads of the SAME map. If
// the map is not mutex-guarded, the Go runtime aborts the whole process with a
// concurrent-map-access fatal error — caught here as a §11.4.85(B) Fatal. Run
// under -race for the data-race detector as well.
func TestPerformanceOptimizer_Chaos_ConcurrentMapAccess(t *testing.T) {
	rec := stresschaos.NewChaosRecorder(t, "performance_concurrent_map_access", "state-corruption")
	// Enable only fast categories so StartProductionOptimization completes in a
	// bounded time (each optimization sleeps 200ms; CPU+Concurrency+Worker = 6
	// optimizations -> ~1.2s, well within the readers' busy window).
	po, err := NewPerformanceOptimizer(PerformanceConfig{
		CPUOptimization:         true,
		ConcurrencyOptimization: true,
		WorkerOptimization:      true,
	})
	if err != nil {
		t.Fatalf("construct optimizer: %v", err)
	}

	types := []OptType{CPUOpt, ConcurrencyOpt, WorkerOpt, MemoryOpt, CacheOpt}

	stop := make(chan struct{})
	var wg sync.WaitGroup
	var reads int64

	// Reader fleet: hammer the map concurrently with the writer inside
	// StartProductionOptimization. A reader that panics or triggers the runtime's
	// concurrent-map fatal would crash the process; we guard with recover() so a
	// recoverable panic is recorded Fatal (the unrecoverable runtime abort takes
	// the whole binary down, which the four-layer suite surfaces as a hard fail).
	const readers = 16
	for r := 0; r < readers; r++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if p := recover(); p != nil {
					rec.Record(stresschaos.Fatal, fmt.Sprintf("reader %d panicked on map access: %v", id, p))
				}
			}()
			for {
				select {
				case <-stop:
					return
				default:
					_ = po.getOptimizationsByType(types[id%len(types)])
					atomic.AddInt64(&reads, 1)
				}
			}
		}(r)
	}

	// Writer: the real optimization run mutates the map under load.
	runErr := func() (e error) {
		defer func() {
			if p := recover(); p != nil {
				rec.Record(stresschaos.Fatal, fmt.Sprintf("StartProductionOptimization panicked: %v", p))
			}
		}()
		_, e = po.StartProductionOptimization(context.Background())
		return e
	}()

	close(stop)
	wg.Wait()

	if runErr != nil {
		rec.Record(stresschaos.Degraded, "optimization run surfaced error (non-fatal): "+runErr.Error())
	} else {
		rec.Record(stresschaos.Recovered, fmt.Sprintf(
			"optimization run completed while %d concurrent map reads ran — map serialised, no crash",
			atomic.LoadInt64(&reads)))
	}
	if atomic.LoadInt64(&reads) == 0 {
		rec.Record(stresschaos.Fatal, "readers performed zero reads — concurrency window never opened")
	}

	rec.AssertNoFatal()
	t.Logf("performance map-access chaos: %d concurrent reads survived optimization run", atomic.LoadInt64(&reads))
}

// TestPerformanceOptimizer_Chaos_CancelMidRun injects a process-death fault by
// cancelling the context mid StartProductionOptimization. The optimizer must
// unwind without crashing or deadlocking, and — critically — must NOT leak the
// running flag: a fresh run must be admissible afterwards (the single-flight
// guard must reset).
func TestPerformanceOptimizer_Chaos_CancelMidRun(t *testing.T) {
	po, err := NewPerformanceOptimizer(PerformanceConfig{
		CPUOptimization:         true,
		MemoryOptimization:      true,
		ConcurrencyOptimization: true,
		WorkerOptimization:      true,
		LLMOptimization:         true,
	})
	if err != nil {
		t.Fatalf("construct optimizer: %v", err)
	}

	stresschaos.ChaosKillDuring(t, "performance_cancel_mid_run", 150*time.Millisecond,
		func(ctx context.Context, rec *stresschaos.ChaosRecorder) {
			// The real run sleeps 200ms per optimization; cancelling after 150ms
			// interrupts it mid-flight. The optimizer must observe the cancellation
			// (or finish) without panicking.
			_, err := po.StartProductionOptimization(ctx)
			if err != nil {
				rec.Record(stresschaos.Degraded, "run returned error after cancellation: "+err.Error())
			} else {
				rec.Record(stresschaos.Recovered, "run completed despite cancellation pressure")
			}
			// Whatever happened, the single-flight flag MUST be released so a new
			// run can start — a leaked flag permanently bricks the optimizer.
			if po.running.Load() {
				rec.Record(stresschaos.Fatal, "running flag leaked after run — optimizer permanently bricked")
			} else {
				rec.Record(stresschaos.Recovered, "running flag released — optimizer reusable after cancellation")
			}
		})

	// Prove reusability post-chaos: the flag must admit a fresh acquire.
	if !po.running.CompareAndSwap(false, true) {
		t.Fatal("optimizer not reusable after cancel chaos — running flag stuck set")
	}
	po.running.Store(false)
}

// TestPerformanceOptimizer_Chaos_HostileConfig feeds structurally hostile /
// degenerate PerformanceConfig values to the REAL constructor and metric paths.
// Construction and metric collection must not panic on negative targets, absurd
// thresholds, or empty target strings — a crash on malformed config is a
// §11.4.85(B) failure. Each payload encodes one hostile config variant.
func TestPerformanceOptimizer_Chaos_HostileConfig(t *testing.T) {
	hostile := []PerformanceConfig{
		{TargetThroughput: -1, TargetCPUUtilization: -50.0},      // negative targets
		{TargetMemoryUsage: -1 << 40, MinCacheHitRate: 99.0},     // absurd memory + >1.0 hit-rate
		{MaxErrorRate: -0.5, TargetLatency: ""},                  // negative error rate, empty latency
		{TargetThroughput: 1 << 30, TargetCPUUtilization: 1e9},   // overflow-ish targets
		{CPUOptimization: true, MemoryOptimization: true, TargetMemoryUsage: 0}, // enabled with zero target
	}

	// Encode each variant index as the corrupt-input payload; feed reconstructs
	// the real hostile config and drives the real constructor + metric paths.
	payloads := make([][]byte, len(hostile))
	for i := range hostile {
		payloads[i] = []byte(fmt.Sprintf(`{"config_index":%d}`, i))
	}

	stresschaos.ChaosCorruptInputDuring(t, "performance_hostile_config", payloads,
		func(input []byte) error {
			idx := hostileConfigIndexOf(input, len(hostile))
			cfg := hostile[idx]
			po, err := NewPerformanceOptimizer(cfg)
			if err != nil {
				// A clean construction error is graceful rejection.
				return fmt.Errorf("constructor rejected hostile config %d: %w", idx, err)
			}
			// Drive the real metric paths with the hostile config — must not panic.
			if _, err := po.collectMetrics(); err != nil {
				return fmt.Errorf("collectMetrics on hostile config %d: %w", idx, err)
			}
			for _, ot := range []OptType{CPUOpt, MemoryOpt, CacheOpt} {
				_, _ = po.getOptimizationMetric(ot)
			}
			return nil
		})
}

// hostileConfigIndexOf recovers the hostile-config variant index from the payload.
func hostileConfigIndexOf(input []byte, n int) int {
	var probe struct {
		ConfigIndex int `json:"config_index"`
	}
	if err := json.Unmarshal(input, &probe); err == nil {
		if probe.ConfigIndex >= 0 && probe.ConfigIndex < n {
			return probe.ConfigIndex
		}
	}
	return 0
}
