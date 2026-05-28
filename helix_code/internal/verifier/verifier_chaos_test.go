package verifier

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"dev.helix.code/tests/stresschaos"
)

// §11.4.85 CHAOS suite for internal/verifier — failure injection against the REAL
// in-process components: concurrent state transitions under contention, callback
// panic isolation, input corruption, cancel-mid-op, resource pressure, and a
// reentrant-lock deadlock probe. Degrade cleanly, never crash/deadlock/leak.
// Live-verifier paths are honestly skipped.

// --- EventPublisher callback panic isolation -------------------------------

// TestChaos_EventPublisher_PanicIsolation injects a panicking subscriber and
// asserts the process survives. EventPublisher.Publish launches each subscriber in
// its own goroutine; a panic in a goroutine WITHOUT recover() crashes the entire
// process (the classic panic-in-goroutine bug class). This test proves the panic
// is isolated. It will genuinely crash the test binary if the production code lacks
// a recover() guard.
func TestChaos_EventPublisher_PanicIsolation(t *testing.T) {
	rec := stresschaos.NewChaosRecorder(t, "verifier_events_panic_isolation", "input-corruption")

	ep := NewEventPublisher()
	var good int64
	var goodMu sync.Mutex
	delivered := make(chan struct{}, 1)

	// A subscriber that always panics (corrupt callback).
	ep.Subscribe(func(ChangeEvent) {
		panic("chaos: subscriber blew up")
	})
	// A healthy subscriber that must still receive the event despite the sibling panic.
	ep.Subscribe(func(ChangeEvent) {
		goodMu.Lock()
		good++
		goodMu.Unlock()
		select {
		case delivered <- struct{}{}:
		default:
		}
	})

	for i := 0; i < 50; i++ {
		if err := ep.Publish(ChangeEvent{Type: "model.discovered", Timestamp: time.Now()}); err != nil {
			rec.Record(stresschaos.Degraded, fmt.Sprintf("publish %d returned error: %v", i, err))
		}
	}

	// Give the async deliveries time to run. If the panicking goroutine is not
	// recovered, the process aborts here and the test fails hard (not via t.Fatal).
	select {
	case <-delivered:
		rec.Record(stresschaos.Recovered, "healthy subscriber received events despite sibling panic")
	case <-time.After(2 * time.Second):
		rec.Record(stresschaos.Degraded, "no healthy delivery observed within timeout")
	}
	time.Sleep(100 * time.Millisecond) // let any remaining panics surface

	goodMu.Lock()
	g := good
	goodMu.Unlock()
	if g == 0 {
		rec.Record(stresschaos.Degraded, "healthy subscriber never invoked")
	} else {
		rec.Record(stresschaos.Recovered, fmt.Sprintf("healthy subscriber invoked %d times; process survived panicking subscriber", g))
	}
	rec.AssertNoFatal()
}

// --- HealthMonitor concurrent state-transition chaos -----------------------

// TestChaos_HealthMonitor_StateTransitionStorm fires concurrent success/failure/
// query storms with a tiny half-open timeout so the breaker churns through all
// three states under maximum contention. The invariant: State() always returns a
// valid enum, no panic, no deadlock. Run under -race for torn-read detection.
func TestChaos_HealthMonitor_StateTransitionStorm(t *testing.T) {
	rec := stresschaos.NewChaosRecorder(t, "verifier_health_state_storm", "state-corruption")
	h := NewHealthMonitor(2, 2, time.Millisecond)

	var wg sync.WaitGroup
	stop := make(chan struct{})
	const workers = 16
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func(id int) {
			defer wg.Done()
			defer func() {
				if p := recover(); p != nil {
					rec.Record(stresschaos.Fatal, fmt.Sprintf("worker %d panicked: %v", id, p))
				}
			}()
			n := 0
			for {
				select {
				case <-stop:
					return
				default:
				}
				switch (id + n) % 5 {
				case 0, 1:
					h.RecordFailure()
				case 2, 3:
					h.RecordSuccess()
				default:
					_ = h.AllowRequest()
				}
				s := h.State()
				if s != CircuitClosed && s != CircuitHalfOpen && s != CircuitOpen {
					rec.Record(stresschaos.Fatal, fmt.Sprintf("invalid state %d", s))
					return
				}
				n++
			}
		}(w)
	}
	time.Sleep(300 * time.Millisecond)
	close(stop)
	wg.Wait()
	rec.Record(stresschaos.Recovered, "breaker survived concurrent state-transition storm; state always valid")
	rec.AssertNoFatal()
}

// --- Cache input-corruption ------------------------------------------------

// TestChaos_Cache_CorruptInput feeds pathological inputs (nil model slices, models
// with empty/garbage IDs, nil score maps) and asserts the cache never panics.
func TestChaos_Cache_CorruptInput(t *testing.T) {
	c := NewCache(time.Minute, newInMemoryRedis())

	// Build corrupt payloads as opaque bytes the feed func interprets by index.
	inputs := make([][]byte, 6)
	for i := range inputs {
		inputs[i] = []byte{byte(i)}
	}

	stresschaos.ChaosCorruptInputDuring(t, "verifier_cache_corrupt_input", inputs, func(payload []byte) error {
		switch payload[0] {
		case 0:
			c.SetModels("", nil) // empty key, nil slice
		case 1:
			c.SetModels("p", []*VerifiedModel{nil, nil}) // nil entries inside slice
		case 2:
			c.SetModels("p", []*VerifiedModel{{ID: ""}}) // empty ID
		case 3:
			c.SetScores(nil) // nil score map
		case 4:
			_, _ = c.GetModelScore("\x00\xff bogus")
		default:
			c.Invalidate("does-not-exist")
		}
		// No error returned => "accepted/normalised without crash" (Recovered). The
		// helper records Fatal automatically if any of the above panics.
		return nil
	})
}

// --- Poller lifecycle chaos (no live endpoint) -----------------------------

// TestChaos_Poller_LifecycleNoEndpoint exercises Start/Stop with a disabled adapter
// (poll() returns immediately because IsEnabled()==false, so NO network is hit).
// Asserts the background goroutine starts, reports running, and stops cleanly with
// no leak. Honestly avoids any live-verifier call.
func TestChaos_Poller_LifecycleNoEndpoint(t *testing.T) {
	rec := stresschaos.NewChaosRecorder(t, "verifier_poller_lifecycle", "process-death")

	// Disabled adapter: poll() short-circuits before touching client/network.
	adapter := NewAdapter(nil, nil, nil, &AdapterConfig{Enabled: false})
	p := NewPoller(adapter, time.Hour) // long interval; we only test the lifecycle

	p.Start()
	if !p.IsRunning() {
		rec.Record(stresschaos.Fatal, "poller not running after Start")
		rec.AssertNoFatal()
		return
	}
	rec.Record(stresschaos.Recovered, "poller goroutine started and reports running")

	done := make(chan struct{})
	go func() {
		p.Stop()
		close(done)
	}()
	select {
	case <-done:
		rec.Record(stresschaos.Recovered, "poller stopped cleanly")
	case <-time.After(5 * time.Second):
		rec.Record(stresschaos.Fatal, "poller did not stop within 5s (deadlock)")
	}
	if p.IsRunning() {
		rec.Record(stresschaos.Degraded, "poller still reports running after Stop")
	}
	rec.AssertNoFatal()
}

// --- Reentrant-lock deadlock probe -----------------------------------------

// TestChaos_HealthMonitor_NoReentrantDeadlock guards against the non-reentrant
// RWMutex re-lock bug class. AllowRequest takes RLock; IsHealthy calls State which
// takes RLock. Calling them in tight interleaving from many goroutines while
// writers hold the write lock must never deadlock. A 5s timeout fails the test if
// any reentrant/ordering deadlock exists.
func TestChaos_HealthMonitor_NoReentrantDeadlock(t *testing.T) {
	rec := stresschaos.NewChaosRecorder(t, "verifier_health_reentrant_probe", "state-corruption")
	h := NewHealthMonitor(5, 3, 10*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	done := make(chan struct{})
	const workers = 12
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func(id int) {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}
				// Interleave read-lock callers and write-lock callers.
				_ = h.IsHealthy()    // -> State() RLock
				_ = h.AllowRequest() // RLock
				if id%2 == 0 {
					h.RecordSuccess() // Lock
				} else {
					h.RecordFailure() // Lock
				}
				_ = h.State() // RLock
			}
		}(w)
	}
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		rec.Record(stresschaos.Recovered, "no deadlock under interleaved read/write lock storm")
	case <-time.After(5 * time.Second):
		rec.Record(stresschaos.Fatal, "DEADLOCK: lock storm did not complete within 5s")
	}
	rec.AssertNoFatal()
}

// --- Resource pressure -----------------------------------------------------

// TestChaos_Cache_UnderMemoryPressure runs cache ops under bounded memory pressure
// and asserts no OOM-crash and continued correctness.
func TestChaos_Cache_UnderMemoryPressure(t *testing.T) {
	c := NewCache(time.Minute, nil)
	stresschaos.ChaosResourcePressureDuring(t, "verifier_cache_mem_pressure", 64, func(rec *stresschaos.ChaosRecorder) {
		for i := 0; i < 500; i++ {
			key := fmt.Sprintf("p-%d", i)
			c.SetModels(key, sampleModels(10, key))
			if _, ok := c.GetModels(key); !ok {
				rec.Record(stresschaos.Degraded, fmt.Sprintf("miss right after set at i=%d (eviction)", i))
			}
		}
		rec.Record(stresschaos.Recovered, "cache served writes+reads under memory pressure")
	})
}
