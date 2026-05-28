package event

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

// §11.4.85(B) chaos coverage for the EventBus.
//
// Chaos classes exercised against the REAL *EventBus (no fakes — real handlers,
// real dispatch, real mutex-guarded state):
//
//   - handler-panic injection: a subscriber that panics mid-dispatch MUST NOT
//     take down the bus. In async mode a panicking handler runs in its own
//     goroutine, so an unrecovered panic would crash the WHOLE process (and
//     every other goroutine, including unrelated work). The bus MUST isolate a
//     panicking handler so co-subscribers still run and the bus stays usable.
//   - input-corruption: structurally hostile event Data payloads (NaN/Inf,
//     unmarshalable channel/func values, huge keys, nested cycles) are published.
//     Dispatch + the bus's own logging paths MUST not crash on them.
//   - state-corruption under contention: a single event type is concurrently
//     Subscribed/Unsubscribed/Published from many goroutines mid-flight. The bus
//     MUST never panic or race and MUST end in a self-consistent subscriber map.

// TestEventBus_Chaos_HandlerPanicIsolation registers a handler that panics
// alongside well-behaved co-subscribers, then publishes (both sync and async).
// A panicking subscriber MUST NOT crash the bus or starve its co-subscribers,
// and the bus MUST remain usable for subsequent publishes. An unrecovered panic
// — especially in async mode, where it would kill the whole process — is a
// §11.4.85(B) Fatal.
func TestEventBus_Chaos_HandlerPanicIsolation(t *testing.T) {
	rec := stresschaos.NewChaosRecorder(t, "event_bus_handler_panic_isolation", "process-death")

	run := func(async bool, label string) {
		bus := NewEventBus(async)
		ctx := context.Background()

		var before, after, panics int64
		bus.Subscribe(EventTaskFailed, func(ctx context.Context, e Event) error {
			atomic.AddInt64(&before, 1)
			return nil
		})
		bus.Subscribe(EventTaskFailed, func(ctx context.Context, e Event) error {
			atomic.AddInt64(&panics, 1)
			panic(fmt.Sprintf("chaos: handler panic in %s mode", label))
		})
		bus.Subscribe(EventTaskFailed, func(ctx context.Context, e Event) error {
			atomic.AddInt64(&after, 1)
			return nil
		})

		// Drive the publish on a guarded goroutine: if the bus does NOT isolate
		// the panic, in sync mode it propagates here (we catch it and record
		// Fatal); in async mode an unisolated panic crashes the process outright
		// (no recover possible), which the four-layer suite surfaces as a hard
		// failure of the whole `go test` binary.
		func() {
			defer func() {
				if p := recover(); p != nil {
					rec.Record(stresschaos.Fatal,
						fmt.Sprintf("%s: publish propagated handler panic to caller: %v", label, p))
				}
			}()
			var err error
			if async {
				err = bus.PublishAndWait(ctx, Event{Type: EventTaskFailed, Source: "chaos"})
			} else {
				err = bus.Publish(ctx, Event{Type: EventTaskFailed, Source: "chaos"})
			}
			// A surfaced error describing the panicking handler is graceful
			// degradation; a clean nil is full recovery. Either is non-fatal so
			// long as the bus did not crash.
			if err != nil {
				rec.Record(stresschaos.Degraded, fmt.Sprintf("%s: publish surfaced handler error: %v", label, err))
			} else {
				rec.Record(stresschaos.Recovered, fmt.Sprintf("%s: publish completed despite panicking handler", label))
			}
		}()

		// Give async goroutines a moment to settle, then assert co-subscribers
		// still ran — the panic must NOT have starved them.
		if async {
			time.Sleep(50 * time.Millisecond)
		}
		if atomic.LoadInt64(&before) == 0 || atomic.LoadInt64(&after) == 0 {
			rec.Record(stresschaos.Fatal,
				fmt.Sprintf("%s: panicking handler starved co-subscribers (before=%d after=%d) — not isolated",
					label, atomic.LoadInt64(&before), atomic.LoadInt64(&after)))
		} else {
			rec.Record(stresschaos.Recovered,
				fmt.Sprintf("%s: co-subscribers survived panic (before=%d after=%d panics=%d)",
					label, atomic.LoadInt64(&before), atomic.LoadInt64(&after), atomic.LoadInt64(&panics)))
		}

		// The bus must remain usable for a fresh publish after the panic.
		var followUp int64
		bus.Subscribe(EventSystemError, func(ctx context.Context, e Event) error {
			atomic.AddInt64(&followUp, 1)
			return nil
		})
		if err := bus.Publish(ctx, Event{Type: EventSystemError}); err != nil {
			rec.Record(stresschaos.Degraded, fmt.Sprintf("%s: follow-up publish errored: %v", label, err))
		}
		if async {
			time.Sleep(30 * time.Millisecond)
		}
		if atomic.LoadInt64(&followUp) == 0 {
			rec.Record(stresschaos.Fatal, fmt.Sprintf("%s: bus unusable after panic — follow-up publish dispatched nothing", label))
		} else {
			rec.Record(stresschaos.Recovered, fmt.Sprintf("%s: bus still usable after panic", label))
		}
	}

	run(false, "sync")
	run(true, "async")

	rec.AssertNoFatal()
	t.Log("event bus survived handler-panic injection in both sync and async modes")
}

// TestEventBus_Chaos_CorruptEventData publishes structurally hostile event Data
// payloads to the REAL bus. Dispatch and the bus's logging/error paths must not
// panic on NaN/Inf floats, unmarshalable channel/func values, oversized keys, or
// nested cycles — a crash on malformed input is a §11.4.85(B) failure. A handler
// reads the payload (mirroring real consumers) so the corrupt data flows all the
// way through dispatch.
func TestEventBus_Chaos_CorruptEventData(t *testing.T) {
	bus := NewEventBus(false)
	ctx := context.Background()

	// Handler that genuinely touches the payload — exercises consumer-side
	// handling of the hostile values (e.g. range over the map, type-assert).
	bus.Subscribe(EventSystemError, func(ctx context.Context, e Event) error {
		for k, v := range e.Data {
			_ = fmt.Sprintf("%s=%v", k, v) // forces evaluation of hostile values
		}
		return nil
	})

	// Descriptors are JSON-serialised so the helper's [][]byte contract holds;
	// feed() reconstructs the real hostile map (incl. types that cannot survive
	// the []byte round-trip, like chan/func).
	corruptKinds := []map[string]interface{}{
		{"nan": math.NaN()},
		{"inf": math.Inf(1)},
		{"channel": "unmarshalable-marker-chan"},
		{"func": "unmarshalable-marker-func"},
		{"huge_key": makeHugeEventString(1 << 16)},
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

	stresschaos.ChaosCorruptInputDuring(t, "event_bus_corrupt_event_data", payloads,
		func(input []byte) error {
			idx := corruptEventIndexOf(input)
			data := hostileEventDataFor(idx)
			// Publishing must not panic regardless of the payload. An error from
			// a handler is graceful; a panic is fatal (caught by the helper).
			return bus.Publish(ctx, Event{Type: EventSystemError, Source: "chaos", Data: data})
		})
}

// TestEventBus_Chaos_ConcurrentSubscribeUnsubscribe hammers the SAME event type
// with concurrent Subscribe / Unsubscribe / Publish from many goroutines. The
// real bus.mutex must serialise the map mutations so the bus never panics or
// races and the subscriber map ends self-consistent. Run under -race.
func TestEventBus_Chaos_ConcurrentSubscribeUnsubscribe(t *testing.T) {
	rec := stresschaos.NewChaosRecorder(t, "event_bus_subscribe_unsubscribe_churn", "state-corruption")
	bus := NewEventBus(false)
	ctx := context.Background()

	const goroutines = 12
	const iters = 300
	var wg sync.WaitGroup
	var subs, unsubs, pubs int64

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
				switch (id + it) % 3 {
				case 0:
					bus.Subscribe(EventTaskCreated, func(ctx context.Context, e Event) error { return nil })
					atomic.AddInt64(&subs, 1)
				case 1:
					// Unsubscribe-all races against concurrent Subscribes — the
					// map delete must be serialised with the appends.
					bus.Unsubscribe(EventTaskCreated)
					atomic.AddInt64(&unsubs, 1)
				default:
					// Publish reads the (concurrently mutating) handler slice.
					_ = bus.Publish(ctx, Event{Type: EventTaskCreated, Source: fmt.Sprintf("g%d", id)})
					atomic.AddInt64(&pubs, 1)
				}
				// Read-only accessors widen the RLock contention surface.
				_ = bus.GetSubscriberCount(EventTaskCreated)
			}
		}(w)
	}
	wg.Wait()

	rec.Record(stresschaos.Recovered, fmt.Sprintf(
		"survived subscribe/unsubscribe/publish churn: %d subs, %d unsubs, %d pubs, no panic/race",
		atomic.LoadInt64(&subs), atomic.LoadInt64(&unsubs), atomic.LoadInt64(&pubs)))

	// Final state must be a coherent, non-negative count and the bus must still
	// dispatch correctly — proof the map was not left torn.
	if c := bus.GetSubscriberCount(EventTaskCreated); c < 0 {
		rec.Record(stresschaos.Fatal, fmt.Sprintf("subscriber count went negative: %d", c))
	}
	var finalHit int64
	bus.Subscribe(EventTaskCreated, func(ctx context.Context, e Event) error {
		atomic.AddInt64(&finalHit, 1)
		return nil
	})
	if err := bus.Publish(ctx, Event{Type: EventTaskCreated}); err != nil {
		rec.Record(stresschaos.Degraded, "final publish surfaced handler error: "+err.Error())
	}
	if atomic.LoadInt64(&finalHit) == 0 {
		rec.Record(stresschaos.Fatal, "bus did not dispatch to a fresh subscriber after churn — map corrupted")
	} else {
		rec.Record(stresschaos.Recovered, "bus dispatches correctly after churn — map self-consistent")
	}

	rec.AssertNoFatal()
	t.Logf("event bus churn: subs=%d unsubs=%d pubs=%d final-count=%d",
		atomic.LoadInt64(&subs), atomic.LoadInt64(&unsubs), atomic.LoadInt64(&pubs),
		bus.GetSubscriberCount(EventTaskCreated))
}

// makeHugeEventString returns an n-byte string of 'x' for oversized-payload chaos.
func makeHugeEventString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = 'x'
	}
	return string(b)
}

// corruptEventIndexOf recovers the chaos payload index from the descriptor.
func corruptEventIndexOf(input []byte) int {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(input, &m); err != nil {
		return 0
	}
	if _, ok := m["corrupt_index"]; ok {
		var probe struct {
			CorruptIndex int `json:"corrupt_index"`
		}
		if json.Unmarshal(input, &probe) == nil {
			return probe.CorruptIndex
		}
	}
	switch {
	case hasEventKey(m, "channel"):
		return 2
	case hasEventKey(m, "func"):
		return 3
	case hasEventKey(m, "huge_key"):
		return 4
	case hasEventKey(m, "nested"):
		return 5
	case hasEventKey(m, "nan"):
		return 0
	case hasEventKey(m, "inf"):
		return 1
	}
	return 0
}

func hasEventKey(m map[string]json.RawMessage, key string) bool {
	_, ok := m[key]
	return ok
}

// hostileEventDataFor reconstructs the actual hostile Data map for a chaos index,
// including types (chan, func) that cannot survive []byte serialisation but
// exercise the bus's dispatch + any marshal/logging paths.
func hostileEventDataFor(idx int) map[string]interface{} {
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
		return map[string]interface{}{"huge_key": makeHugeEventString(1 << 16)}
	default:
		return map[string]interface{}{"nested": map[string]interface{}{"a": map[string]interface{}{"b": math.NaN()}}}
	}
}
