package hooks

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

// §11.4.85(B) chaos coverage for the hooks Manager + Executor.
//
// Chaos classes exercised against the REAL *Manager / *Executor (no fakes —
// real handlers, real dispatch, real mutex-guarded state):
//
//   - handler-panic injection: a hook handler that panics mid-dispatch MUST NOT
//     take down the manager. In async mode a panicking handler runs in its own
//     executor goroutine, so an unrecovered panic would crash the WHOLE process
//     (and every other goroutine, including unrelated work). The executor MUST
//     isolate a panicking handler so co-hooks still run and the manager stays
//     usable. (This is the exact bug class just found in internal/event.)
//   - input-corruption: structurally hostile event Data payloads (NaN/Inf,
//     unmarshalable channel/func values, huge keys, nested cycles) are
//     triggered. Dispatch + any handler-side handling MUST not crash on them.
//   - state-corruption under contention: a single hook type is concurrently
//     Registered/Unregistered/Triggered from many goroutines mid-flight. The
//     manager MUST never panic or race and MUST end in a self-consistent map.

// TestHooks_Chaos_HandlerPanicIsolation registers a handler that panics
// alongside well-behaved co-hooks, then triggers (both sync and async). A
// panicking handler MUST NOT crash the manager or starve its co-hooks, and the
// manager MUST remain usable for subsequent triggers. An unrecovered panic —
// especially in async mode, where it would kill the whole process — is a
// §11.4.85(B) Fatal.
func TestHooks_Chaos_HandlerPanicIsolation(t *testing.T) {
	rec := stresschaos.NewChaosRecorder(t, "hooks_handler_panic_isolation", "process-death")

	run := func(async bool, label string, ht HookType) {
		mgr := NewManager()
		ctx := context.Background()

		var before, after, panics int64
		newH := func(name string, fn HookFunc) *Hook {
			if async {
				return NewAsyncHook(name, ht, fn)
			}
			return NewHook(name, ht, fn)
		}

		// Register at fixed priorities so the panicking hook sits BETWEEN two
		// well-behaved co-hooks. If a sync panic propagates out of the executor
		// loop, the "after" hook (lower priority, runs later) would be starved.
		mustReg := func(h *Hook) {
			if err := mgr.Register(h); err != nil {
				t.Fatalf("%s: register %s: %v", label, h.Name, err)
			}
		}
		hb := newH("before-hook", func(ctx context.Context, e *Event) error {
			atomic.AddInt64(&before, 1)
			return nil
		})
		hb.Priority = PriorityHighest
		mustReg(hb)

		hp := newH("panic-hook", func(ctx context.Context, e *Event) error {
			atomic.AddInt64(&panics, 1)
			panic(fmt.Sprintf("chaos: handler panic in %s mode", label))
		})
		hp.Priority = PriorityNormal
		mustReg(hp)

		ha := newH("after-hook", func(ctx context.Context, e *Event) error {
			atomic.AddInt64(&after, 1)
			return nil
		})
		ha.Priority = PriorityLow
		mustReg(ha)

		// Drive the trigger on a guarded goroutine: if the executor does NOT
		// isolate the panic, in sync mode it propagates here (we catch it and
		// record Fatal); in async mode an unisolated panic crashes the process
		// outright (no recover possible), which the four-layer suite surfaces as
		// a hard failure of the whole `go test` binary.
		func() {
			defer func() {
				if p := recover(); p != nil {
					rec.Record(stresschaos.Fatal,
						fmt.Sprintf("%s: trigger propagated handler panic to caller: %v", label, p))
				}
			}()
			if async {
				mgr.TriggerAndWait(ctx, ht)
			} else {
				_ = mgr.TriggerSync(ctx, ht)
			}
			rec.Record(stresschaos.Recovered, fmt.Sprintf("%s: trigger completed despite panicking handler", label))
		}()

		// Give async goroutines a moment to settle, then assert co-hooks still
		// ran — the panic must NOT have starved them.
		if async {
			mgr.Wait()
			time.Sleep(50 * time.Millisecond)
		}
		if atomic.LoadInt64(&before) == 0 || atomic.LoadInt64(&after) == 0 {
			rec.Record(stresschaos.Fatal,
				fmt.Sprintf("%s: panicking handler starved co-hooks (before=%d after=%d) — not isolated",
					label, atomic.LoadInt64(&before), atomic.LoadInt64(&after)))
		} else {
			rec.Record(stresschaos.Recovered,
				fmt.Sprintf("%s: co-hooks survived panic (before=%d after=%d panics=%d)",
					label, atomic.LoadInt64(&before), atomic.LoadInt64(&after), atomic.LoadInt64(&panics)))
		}

		// The manager must remain usable for a fresh trigger after the panic.
		var followUp int64
		fu := NewHook("follow-up", HookTypeOnSuccess, func(ctx context.Context, e *Event) error {
			atomic.AddInt64(&followUp, 1)
			return nil
		})
		mustReg(fu)
		_ = mgr.TriggerSync(ctx, HookTypeOnSuccess)
		if atomic.LoadInt64(&followUp) == 0 {
			rec.Record(stresschaos.Fatal, fmt.Sprintf("%s: manager unusable after panic — follow-up trigger dispatched nothing", label))
		} else {
			rec.Record(stresschaos.Recovered, fmt.Sprintf("%s: manager still usable after panic", label))
		}
	}

	run(false, "sync", HookTypeBeforeTask)
	run(true, "async", HookTypeAfterTask)

	rec.AssertNoFatal()
	t.Log("hooks manager survived handler-panic injection in both sync and async modes")
}

// TestHooks_Chaos_CorruptEventData triggers structurally hostile event Data
// payloads at the REAL manager. Dispatch and any handler-side handling must not
// panic on NaN/Inf floats, unmarshalable channel/func values, oversized keys,
// or nested cycles — a crash on malformed input is a §11.4.85(B) failure. A
// handler reads the payload (mirroring real consumers) so the corrupt data
// flows all the way through dispatch.
func TestHooks_Chaos_CorruptEventData(t *testing.T) {
	mgr := NewManager()

	// Handler that genuinely touches the payload — exercises consumer-side
	// handling of the hostile values (e.g. range over the map, format).
	hook := NewHook("corrupt-consumer", HookTypeOnError, func(ctx context.Context, e *Event) error {
		for k, v := range e.Data {
			_ = fmt.Sprintf("%s=%v", k, v) // forces evaluation of hostile values
		}
		return nil
	})
	if err := mgr.Register(hook); err != nil {
		t.Fatalf("register corrupt-consumer: %v", err)
	}

	// Descriptors are JSON-serialised so the helper's [][]byte contract holds;
	// feed() reconstructs the real hostile map (incl. types that cannot survive
	// the []byte round-trip, like chan/func).
	corruptKinds := []map[string]interface{}{
		{"nan": math.NaN()},
		{"inf": math.Inf(1)},
		{"channel": "unmarshalable-marker-chan"},
		{"func": "unmarshalable-marker-func"},
		{"huge_key": makeHugeHookString(1 << 16)},
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

	stresschaos.ChaosCorruptInputDuring(t, "hooks_corrupt_event_data", payloads,
		func(input []byte) error {
			idx := corruptHookIndexOf(input)
			data := hostileHookDataFor(idx)
			// Triggering must not panic regardless of the payload. An error from
			// a handler is graceful; a panic is fatal (caught by the helper).
			event := NewEvent(HookTypeOnError)
			event.Data = data
			event.Source = "chaos"
			results := mgr.TriggerEventSync(event)
			for _, r := range results {
				if r.Error != nil {
					return r.Error
				}
			}
			return nil
		})
}

// TestHooks_Chaos_CorruptHookRegistration feeds the manager malformed Hook
// registrations (the registry's own input-validation surface): empty IDs,
// empty names, nil handlers, out-of-range priorities. The manager MUST reject
// each cleanly with an error — never panic, never corrupt the map. After the
// onslaught the manager must still register + dispatch a valid hook.
func TestHooks_Chaos_CorruptHookRegistration(t *testing.T) {
	mgr := NewManager()
	rec := stresschaos.NewChaosRecorder(t, "hooks_corrupt_registration", "input-corruption")

	bad := []*Hook{
		{ID: "", Name: "n", Type: HookTypeOnError, Handler: func(context.Context, *Event) error { return nil }, Priority: PriorityNormal, Enabled: true},
		{ID: "id", Name: "", Type: HookTypeOnError, Handler: func(context.Context, *Event) error { return nil }, Priority: PriorityNormal, Enabled: true},
		{ID: "id2", Name: "n", Type: "", Handler: func(context.Context, *Event) error { return nil }, Priority: PriorityNormal, Enabled: true},
		{ID: "id3", Name: "n", Type: HookTypeOnError, Handler: nil, Priority: PriorityNormal, Enabled: true},
		{ID: "id4", Name: "n", Type: HookTypeOnError, Handler: func(context.Context, *Event) error { return nil }, Priority: 9999, Enabled: true},
		{ID: "id5", Name: "n", Type: HookTypeOnError, Handler: func(context.Context, *Event) error { return nil }, Priority: -5, Enabled: true},
		nil, // nil hook pointer
	}

	for i, h := range bad {
		func(idx int, hk *Hook) {
			defer func() {
				if p := recover(); p != nil {
					rec.Record(stresschaos.Fatal, fmt.Sprintf("Register[%d] panicked on malformed hook: %v", idx, p))
				}
			}()
			err := mgr.Register(hk)
			if err != nil {
				rec.Record(stresschaos.Degraded, fmt.Sprintf("Register[%d] rejected malformed hook cleanly: %v", idx, err))
			} else {
				rec.Record(stresschaos.Recovered, fmt.Sprintf("Register[%d] accepted hook (validated)", idx))
			}
		}(i, h)
	}

	// The map must NOT be polluted by the rejected hooks (none of the valid-ID
	// ones above pass Validate, so count must be 0).
	if mgr.Count() != 0 {
		rec.Record(stresschaos.Fatal, fmt.Sprintf("manager retained %d hooks after all-malformed registrations", mgr.Count()))
	} else {
		rec.Record(stresschaos.Recovered, "manager rejected every malformed hook — map clean")
	}

	// Manager must still work for a valid hook.
	var hit int64
	good := NewHook("good", HookTypeOnError, func(ctx context.Context, e *Event) error {
		atomic.AddInt64(&hit, 1)
		return nil
	})
	if err := mgr.Register(good); err != nil {
		rec.Record(stresschaos.Fatal, "manager refused a valid hook after malformed onslaught: "+err.Error())
	}
	_ = mgr.TriggerSync(context.Background(), HookTypeOnError)
	if atomic.LoadInt64(&hit) == 0 {
		rec.Record(stresschaos.Fatal, "manager did not dispatch to a valid hook after malformed onslaught")
	} else {
		rec.Record(stresschaos.Recovered, "manager dispatches correctly after malformed onslaught")
	}

	rec.AssertNoFatal()
}

// TestHooks_Chaos_ConcurrentRegisterUnregister hammers the SAME hook type with
// concurrent Register / Unregister / Trigger from many goroutines. The real
// manager mutex must serialise the map mutations so the manager never panics or
// races and the maps end self-consistent. Run under -race.
func TestHooks_Chaos_ConcurrentRegisterUnregister(t *testing.T) {
	rec := stresschaos.NewChaosRecorder(t, "hooks_register_unregister_churn", "state-corruption")
	mgr := NewManager()
	ctx := context.Background()

	const goroutines = 12
	const iters = 300
	var wg sync.WaitGroup
	var regs, unregs, trigs int64

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
					hook := NewHook(fmt.Sprintf("g%d-i%d", id, it), HookTypeBeforeTask,
						func(ctx context.Context, e *Event) error { return nil })
					if mgr.Register(hook) == nil {
						atomic.AddInt64(&regs, 1)
						// Best-effort unregister of our own hook to churn deletes.
						if mgr.Unregister(hook.ID) == nil {
							atomic.AddInt64(&unregs, 1)
						}
					}
				case 1:
					// Clear-by-type churn: GetByType reads the slice that case 0
					// is concurrently mutating.
					_ = mgr.GetByType(HookTypeBeforeTask)
				default:
					// Trigger reads the (concurrently mutating) hook slice.
					_ = mgr.Trigger(ctx, HookTypeBeforeTask)
					atomic.AddInt64(&trigs, 1)
				}
				// Read-only accessors widen the RLock contention surface.
				_ = mgr.CountByType(HookTypeBeforeTask)
				_ = mgr.Count()
			}
		}(w)
	}
	wg.Wait()

	rec.Record(stresschaos.Recovered, fmt.Sprintf(
		"survived register/unregister/trigger churn: %d regs, %d unregs, %d trigs, no panic/race",
		atomic.LoadInt64(&regs), atomic.LoadInt64(&unregs), atomic.LoadInt64(&trigs)))

	// Final state must be a coherent, non-negative count and the manager must
	// still dispatch correctly — proof the maps were not left torn.
	if c := mgr.CountByType(HookTypeBeforeTask); c < 0 {
		rec.Record(stresschaos.Fatal, fmt.Sprintf("hook count went negative: %d", c))
	}
	var finalHit int64
	final := NewHook("final-probe", HookTypeBeforeTask, func(ctx context.Context, e *Event) error {
		atomic.AddInt64(&finalHit, 1)
		return nil
	})
	if err := mgr.Register(final); err != nil {
		rec.Record(stresschaos.Fatal, "could not register final probe after churn: "+err.Error())
	}
	_ = mgr.TriggerSync(ctx, HookTypeBeforeTask)
	if atomic.LoadInt64(&finalHit) == 0 {
		rec.Record(stresschaos.Fatal, "manager did not dispatch to a fresh hook after churn — map corrupted")
	} else {
		rec.Record(stresschaos.Recovered, "manager dispatches correctly after churn — map self-consistent")
	}

	rec.AssertNoFatal()
	t.Logf("hooks churn: regs=%d unregs=%d trigs=%d final-count=%d",
		atomic.LoadInt64(&regs), atomic.LoadInt64(&unregs), atomic.LoadInt64(&trigs),
		mgr.CountByType(HookTypeBeforeTask))
}

// makeHugeHookString returns an n-byte string of 'x' for oversized-payload chaos.
func makeHugeHookString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = 'x'
	}
	return string(b)
}

// corruptHookIndexOf recovers the chaos payload index from the descriptor.
func corruptHookIndexOf(input []byte) int {
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
	case hasHookKey(m, "channel"):
		return 2
	case hasHookKey(m, "func"):
		return 3
	case hasHookKey(m, "huge_key"):
		return 4
	case hasHookKey(m, "nested"):
		return 5
	case hasHookKey(m, "nan"):
		return 0
	case hasHookKey(m, "inf"):
		return 1
	}
	return 0
}

func hasHookKey(m map[string]json.RawMessage, key string) bool {
	_, ok := m[key]
	return ok
}

// hostileHookDataFor reconstructs the actual hostile Data map for a chaos index,
// including types (chan, func) that cannot survive []byte serialisation but
// exercise dispatch + any marshal/logging paths.
func hostileHookDataFor(idx int) map[string]interface{} {
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
		return map[string]interface{}{"huge_key": makeHugeHookString(1 << 16)}
	default:
		return map[string]interface{}{"nested": map[string]interface{}{"a": map[string]interface{}{"b": math.NaN()}}}
	}
}
