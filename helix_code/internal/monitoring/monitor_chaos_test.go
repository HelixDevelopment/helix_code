package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"dev.helix.code/tests/stresschaos"
)

// §11.4.85(B) chaos coverage for the monitoring Monitor.
//
// Chaos classes exercised against the REAL *Monitor (no fakes — real
// collectors, real mutex-guarded state):
//
//   - collector-panic injection: a Collector that panics inside Collect() MUST
//     NOT take down the Monitor or the whole process. CollectMetrics invokes
//     collector.Collect() while holding the write mutex; an unrecovered panic
//     would crash the process AND (because the panic unwinds through the locked
//     critical section) destabilise every other goroutine using the Monitor.
//     A real collector (HTTP scrape, /proc read, disk stat) panicking is a
//     realistic failure mode. The Monitor MUST isolate a panicking collector so
//     its co-collectors still run and the Monitor stays usable.
//   - input-corruption: structurally hostile metric payloads (NaN/Inf, channel/
//     func values, oversized keys, deeply nested cycles) are emitted by a real
//     collector. Collection + the GetAllMetrics copy path MUST not crash on them.
//   - state-corruption under contention: collectors are concurrently added while
//     CollectMetrics + GetMetric + GetAllMetrics run mid-flight. The Monitor MUST
//     never panic or race and MUST end in a self-consistent metrics map.

// panicCollector is a REAL Collector whose Collect() panics — modelling a
// collector whose backing source (HTTP/disk/proc) faults unexpectedly.
type panicCollector struct {
	name    string
	calls   *int64
	message string
}

func (p *panicCollector) Name() string { return p.name }

func (p *panicCollector) Collect() (map[string]interface{}, error) {
	atomic.AddInt64(p.calls, 1)
	panic(p.message)
}

// goodCollector is a REAL co-collector that records that it ran.
type goodCollector struct {
	name  string
	calls *int64
	key   string
}

func (g *goodCollector) Name() string { return g.name }

func (g *goodCollector) Collect() (map[string]interface{}, error) {
	atomic.AddInt64(g.calls, 1)
	return map[string]interface{}{g.key: atomic.LoadInt64(g.calls)}, nil
}

// TestMonitor_Chaos_CollectorPanicIsolation registers a collector that panics
// alongside well-behaved co-collectors, then runs CollectMetrics. A panicking
// collector MUST NOT crash the Monitor or starve its co-collectors, and the
// Monitor MUST remain usable for subsequent collections. An unrecovered panic
// — which propagates out of CollectMetrics through the locked critical section
// — is a §11.4.85(B) Fatal.
func TestMonitor_Chaos_CollectorPanicIsolation(t *testing.T) {
	rec := stresschaos.NewChaosRecorder(t, "monitor_collector_panic_isolation", "process-death")

	m := NewMonitor()
	ctx := context.Background()

	var beforeCalls, panicCalls, afterCalls int64
	m.AddCollector(&goodCollector{name: "before", calls: &beforeCalls, key: "before_metric"})
	m.AddCollector(&panicCollector{name: "panicker", calls: &panicCalls, message: "chaos: collector source faulted mid-scrape"})
	m.AddCollector(&goodCollector{name: "after", calls: &afterCalls, key: "after_metric"})

	// Drive CollectMetrics on a guarded goroutine: if the Monitor does NOT
	// isolate the panic it propagates out here (caught + recorded Fatal). If it
	// did not even use a goroutine the panic would crash the whole `go test`
	// binary, which the four-layer suite surfaces as a hard failure.
	func() {
		defer func() {
			if p := recover(); p != nil {
				rec.Record(stresschaos.Fatal,
					fmt.Sprintf("CollectMetrics propagated collector panic to caller: %v", p))
			}
		}()
		if err := m.CollectMetrics(ctx); err != nil {
			rec.Record(stresschaos.Degraded, fmt.Sprintf("collect surfaced collector error: %v", err))
		} else {
			rec.Record(stresschaos.Recovered, "collect completed despite panicking collector")
		}
	}()

	// Co-collectors must still have run — the panic must NOT have starved them.
	if atomic.LoadInt64(&beforeCalls) == 0 {
		rec.Record(stresschaos.Fatal, "collector registered BEFORE the panicker never ran")
	}
	if atomic.LoadInt64(&afterCalls) == 0 {
		rec.Record(stresschaos.Fatal,
			fmt.Sprintf("panicking collector starved the AFTER co-collector (before=%d after=%d) — not isolated",
				atomic.LoadInt64(&beforeCalls), atomic.LoadInt64(&afterCalls)))
	} else {
		rec.Record(stresschaos.Recovered,
			fmt.Sprintf("co-collectors survived panic (before=%d after=%d panics=%d)",
				atomic.LoadInt64(&beforeCalls), atomic.LoadInt64(&afterCalls), atomic.LoadInt64(&panicCalls)))
	}

	// The Monitor must remain usable — neither crashed nor left mutex-locked.
	// A still-locked mutex would deadlock this follow-up call (caught by the
	// RunConcurrent/test timeout in the suite; here we just assert it returns).
	var followUp int64
	m.AddCollector(&goodCollector{name: "followup", calls: &followUp, key: "followup_metric"})
	if err := m.CollectMetrics(ctx); err != nil {
		rec.Record(stresschaos.Degraded, fmt.Sprintf("follow-up collect errored: %v", err))
	}
	if atomic.LoadInt64(&followUp) == 0 {
		rec.Record(stresschaos.Fatal, "Monitor unusable after panic — follow-up collect ran no collectors")
	} else {
		rec.Record(stresschaos.Recovered, "Monitor still usable after collector panic")
	}
	// And reads must still work (proves the metrics map is not torn / mutex free).
	if _, ok := m.GetMetric("before_metric"); !ok {
		rec.Record(stresschaos.Degraded, "before_metric not readable after panic round")
	}

	rec.AssertNoFatal()
	t.Log("monitor survived collector-panic injection")
}

// corruptCollector is a REAL Collector that emits structurally hostile metric
// values, modelling a collector whose upstream returns malformed data.
type corruptCollector struct {
	name string
	idx  int64 // selects which hostile payload to emit on each Collect()
}

func (c *corruptCollector) Name() string { return c.name }

func (c *corruptCollector) Collect() (map[string]interface{}, error) {
	return hostileMetricsFor(int(atomic.LoadInt64(&c.idx))), nil
}

// TestMonitor_Chaos_CorruptMetricValues feeds the REAL Monitor structurally
// hostile metric payloads (NaN/Inf floats, channel/func values, oversized keys,
// nested cycles) via a real collector. CollectMetrics, the GetAllMetrics copy
// path, and a consumer that ranges + stringifies every value MUST NOT panic on
// them — a crash on malformed metric data is a §11.4.85(B) failure.
func TestMonitor_Chaos_CorruptMetricValues(t *testing.T) {
	ctx := context.Background()

	corruptKinds := []map[string]interface{}{
		{"nan": math.NaN()},
		{"inf": math.Inf(1)},
		{"channel": "unmarshalable-marker-chan"},
		{"func": "unmarshalable-marker-func"},
		{"huge_key": makeHugeMetricString(1 << 16)},
		{"nested": map[string]interface{}{"a": map[string]interface{}{"b": math.NaN()}}},
	}
	payloads := make([][]byte, len(corruptKinds))
	for i, k := range corruptKinds {
		b, err := json.Marshal(k)
		if err != nil {
			b = []byte(fmt.Sprintf(`{"corrupt_index":%d}`, i))
		}
		payloads[i] = b
	}

	cc := &corruptCollector{name: "corrupt"}
	m := NewMonitor()
	m.AddCollector(cc)

	stresschaos.ChaosCorruptInputDuring(t, "monitor_corrupt_metric_values", payloads,
		func(input []byte) error {
			atomic.StoreInt64(&cc.idx, int64(corruptMetricIndexOf(input)))
			// Collecting must not panic regardless of the hostile payload.
			if err := m.CollectMetrics(ctx); err != nil {
				return err
			}
			// Exercise the snapshot-copy path + a real consumer that touches
			// every value — forces evaluation of NaN/Inf/chan/func/huge values.
			all := m.GetAllMetrics()
			for key, v := range all {
				_ = fmt.Sprintf("%s=%v", key, v)
			}
			return nil
		})
}

// TestMonitor_Chaos_ConcurrentAddAndCollect hammers the SAME Monitor with
// concurrent AddCollector / CollectMetrics / GetMetric / GetAllMetrics from many
// goroutines. The real mutex must serialise the slice append + map writes so the
// Monitor never panics or races and the metrics map ends self-consistent. Run
// under -race.
func TestMonitor_Chaos_ConcurrentAddAndCollect(t *testing.T) {
	rec := stresschaos.NewChaosRecorder(t, "monitor_add_collect_churn", "state-corruption")
	m := NewMonitor()
	ctx := context.Background()

	// Seed so reads have a stable target.
	var seedCalls int64
	m.AddCollector(&goodCollector{name: "seed", calls: &seedCalls, key: "seed_metric"})

	const goroutines = 12
	const iters = 250
	var wg sync.WaitGroup
	var adds, collects, reads int64
	var sharedCalls int64

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
				switch (id + it) % 4 {
				case 0:
					m.AddCollector(&goodCollector{
						name:  fmt.Sprintf("g%d_it%d", id, it),
						calls: &sharedCalls,
						key:   fmt.Sprintf("g%d_metric", id),
					})
					atomic.AddInt64(&adds, 1)
				case 1:
					_ = m.CollectMetrics(ctx)
					atomic.AddInt64(&collects, 1)
				case 2:
					_, _ = m.GetMetric("seed_metric")
					atomic.AddInt64(&reads, 1)
				default:
					_ = m.GetAllMetrics()
					atomic.AddInt64(&reads, 1)
				}
			}
		}(w)
	}
	wg.Wait()

	rec.Record(stresschaos.Recovered, fmt.Sprintf(
		"survived add/collect/read churn: %d adds, %d collects, %d reads, no panic/race",
		atomic.LoadInt64(&adds), atomic.LoadInt64(&collects), atomic.LoadInt64(&reads)))

	// Final state must be coherent + the Monitor must still collect correctly.
	var finalCalls int64
	m.AddCollector(&goodCollector{name: "final", calls: &finalCalls, key: "final_metric"})
	if err := m.CollectMetrics(ctx); err != nil {
		rec.Record(stresschaos.Degraded, "final collect surfaced error: "+err.Error())
	}
	if atomic.LoadInt64(&finalCalls) == 0 {
		rec.Record(stresschaos.Fatal, "Monitor did not invoke a fresh collector after churn — state corrupted")
	} else {
		rec.Record(stresschaos.Recovered, "Monitor collects correctly after churn — map self-consistent")
	}
	if _, ok := m.GetMetric("final_metric"); !ok {
		rec.Record(stresschaos.Fatal, "final_metric not readable after churn — map torn")
	}

	rec.AssertNoFatal()
	t.Logf("monitor churn: adds=%d collects=%d reads=%d final-metrics=%d",
		atomic.LoadInt64(&adds), atomic.LoadInt64(&collects), atomic.LoadInt64(&reads), len(m.GetAllMetrics()))
}

// TestMonitor_Chaos_PeriodicCollectionKilledMidFlight injects a process-death
// fault (§11.4.85(B)(1)) against the REAL StartPeriodicCollection loop: the loop
// is started under a cancellable context, allowed to tick against real
// collectors, then the context is cancelled mid-flight. The loop MUST observe
// the cancellation and unwind cleanly (no leaked ticker goroutine, no deadlock).
func TestMonitor_Chaos_PeriodicCollectionKilledMidFlight(t *testing.T) {
	m := NewMonitor()
	var calls int64
	m.AddCollector(&goodCollector{name: "periodic", calls: &calls, key: "periodic_metric"})

	stresschaos.ChaosKillDuring(t, "monitor_periodic_collection_killed", 120*time.Millisecond,
		func(ctx context.Context, rec *stresschaos.ChaosRecorder) {
			// Real periodic-collection loop; honours ctx.Done() for unwind.
			m.StartPeriodicCollection(ctx, 20*time.Millisecond)
			// Reaching here means the loop observed the cancellation and returned.
			if atomic.LoadInt64(&calls) == 0 {
				rec.Record(stresschaos.Degraded, "periodic loop unwound before any tick fired")
			} else {
				rec.Record(stresschaos.Recovered,
					fmt.Sprintf("periodic loop ticked %d times then unwound on cancellation", atomic.LoadInt64(&calls)))
			}
		})

	// After cancellation the Monitor must still be usable for a direct collect.
	if err := m.CollectMetrics(context.Background()); err != nil {
		t.Fatalf("Monitor unusable after periodic-collection cancellation: %v", err)
	}
}

// makeHugeMetricString returns an n-byte string of 'x' for oversized-payload chaos.
func makeHugeMetricString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = 'x'
	}
	return string(b)
}

// corruptMetricIndexOf recovers the chaos payload index from the descriptor.
func corruptMetricIndexOf(input []byte) int {
	var mm map[string]json.RawMessage
	if err := json.Unmarshal(input, &mm); err != nil {
		return 0
	}
	if _, ok := mm["corrupt_index"]; ok {
		var probe struct {
			CorruptIndex int `json:"corrupt_index"`
		}
		if json.Unmarshal(input, &probe) == nil {
			return probe.CorruptIndex
		}
	}
	switch {
	case hasMetricKey(mm, "channel"):
		return 2
	case hasMetricKey(mm, "func"):
		return 3
	case hasMetricKey(mm, "huge_key"):
		return 4
	case hasMetricKey(mm, "nested"):
		return 5
	case hasMetricKey(mm, "nan"):
		return 0
	case hasMetricKey(mm, "inf"):
		return 1
	}
	return 0
}

func hasMetricKey(m map[string]json.RawMessage, key string) bool {
	_, ok := m[key]
	return ok
}

// hostileMetricsFor reconstructs the actual hostile metric map for a chaos index,
// including types (chan, func) that cannot survive []byte serialisation but
// exercise the Monitor's map-write + the consumer's stringify paths.
func hostileMetricsFor(idx int) map[string]interface{} {
	switch idx {
	case 0:
		return map[string]interface{}{"nan": math.NaN()}
	case 1:
		return map[string]interface{}{"inf": math.Inf(1)}
	case 2:
		return map[string]interface{}{"channel": make(chan int)}
	case 3:
		return map[string]interface{}{"func": func() {}}
	case 4:
		return map[string]interface{}{"huge_key": makeHugeMetricString(1 << 16)}
	default:
		return map[string]interface{}{"nested": map[string]interface{}{"a": map[string]interface{}{"b": math.NaN()}}}
	}
}
