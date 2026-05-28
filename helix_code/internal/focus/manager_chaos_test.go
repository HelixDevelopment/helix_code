package focus

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"dev.helix.code/tests/stresschaos"
)

// §11.4.85(B) chaos coverage for the focus *Manager / *Chain.
//
// Chaos classes exercised against the REAL components (no fakes — real
// RWMutex-guarded chain map, real callback dispatch under lock, real
// per-Focus validation):
//
//   - callback-panic injection: the Manager invokes onCreate/onDelete/
//     onActivate callbacks WHILE HOLDING m.mu (manager.go CreateChain/
//     DeleteChain/SetActiveChain). A panicking callback with no recover()
//     propagates to the caller AND every other registered callback is
//     starved. The manager MUST isolate the panic so co-callbacks still run
//     and the manager stays usable.
//   - callback-reentrancy: a callback that calls back into the Manager
//     re-acquires the non-reentrant RWMutex while it is already write-held →
//     classic deadlock. Probed under timeout.
//   - input-corruption: structurally hostile Focus values (empty/invalid
//     fields, out-of-range priority, expiration before creation, hostile
//     context map values, huge targets) fed through the REAL validate+push
//     path. Must reject cleanly or normalise — never crash.
//   - state-corruption under contention: a single Manager is concurrently
//     created/deleted/cleared/callback-registered from many goroutines. The
//     RWMutex must serialise so the map never panics/races and ends coherent.
//   - process-death: a long create+push loop is cancelled mid-operation and
//     must unwind cleanly without leaking a goroutine.
//   - resource-pressure: a large chain store operates under bounded memory
//     pressure without OOM-crash.

// TestManager_Chaos_CallbackPanicIsolation registers a callback that panics
// alongside well-behaved co-callbacks, then drives CreateChain. The Manager
// invokes callbacks while holding m.mu.Lock(); a panicking callback with no
// recover() (a) propagates to the caller and (b) starves the co-callbacks
// registered after it. The manager MUST isolate the panic and remain usable.
func TestManager_Chaos_CallbackPanicIsolation(t *testing.T) {
	rec := stresschaos.NewChaosRecorder(t, "focus_manager_callback_panic_isolation", "process-death")

	mgr := NewManager()
	var before, after int64
	mgr.OnCreate(func(_ *Chain) { atomic.AddInt64(&before, 1) })
	mgr.OnCreate(func(_ *Chain) { panic("chaos: OnCreate callback panic") })
	mgr.OnCreate(func(_ *Chain) { atomic.AddInt64(&after, 1) })

	// Drive CreateChain on a guarded goroutine: if the manager does NOT isolate
	// the callback panic, it propagates here (we catch it and record Degraded —
	// a surfaced panic is acceptable degradation only if co-callbacks still ran
	// and the lock is not leaked, asserted below).
	func() {
		defer func() {
			if p := recover(); p != nil {
				rec.Record(stresschaos.Degraded, fmt.Sprintf("CreateChain propagated callback panic to caller: %v", p))
			}
		}()
		if _, err := mgr.CreateChain("panic-cb", true); err != nil {
			rec.Record(stresschaos.Degraded, fmt.Sprintf("CreateChain surfaced error: %v", err))
		} else {
			rec.Record(stresschaos.Recovered, "CreateChain completed despite panicking callback")
		}
	}()

	// The co-callback registered AFTER the panicking one must still have run —
	// a panicking callback that aborts the dispatch loop starves it.
	if atomic.LoadInt64(&before) == 0 || atomic.LoadInt64(&after) == 0 {
		rec.Record(stresschaos.Fatal,
			fmt.Sprintf("panicking callback starved co-callbacks (before=%d after=%d) — not isolated",
				atomic.LoadInt64(&before), atomic.LoadInt64(&after)))
	} else {
		rec.Record(stresschaos.Recovered,
			fmt.Sprintf("co-callbacks survived panic (before=%d after=%d)",
				atomic.LoadInt64(&before), atomic.LoadInt64(&after)))
	}

	// CRITICAL: the manager must remain usable. If the panic left the write lock
	// held, this follow-up CreateChain blocks forever — guard with a timeout and
	// record Fatal (deadlock) if it does not return.
	followUp := make(chan error, 1)
	go func() {
		_, err := mgr.CreateChain("follow-up", false)
		followUp <- err
	}()
	select {
	case err := <-followUp:
		if err != nil {
			rec.Record(stresschaos.Degraded, fmt.Sprintf("follow-up create errored: %v", err))
		} else {
			rec.Record(stresschaos.Recovered, "manager still usable after callback panic — lock not leaked")
		}
	case <-time.After(5 * time.Second):
		rec.Record(stresschaos.Fatal, "manager deadlocked after callback panic — write lock leaked")
	}

	rec.AssertNoFatal()
	t.Logf("focus manager survived callback-panic injection (before=%d after=%d)",
		atomic.LoadInt64(&before), atomic.LoadInt64(&after))
}

// TestManager_Chaos_CallbackReentrancy registers a callback that calls back
// into the Manager. Because the callback runs while m.mu.Lock() is held and
// sync.RWMutex is non-reentrant, a re-entrant Manager call deadlocks. The
// manager must NOT deadlock — either the callback path must run outside the
// lock, or the operation must complete. Probed under a timeout.
func TestManager_Chaos_CallbackReentrancy(t *testing.T) {
	rec := stresschaos.NewChaosRecorder(t, "focus_manager_callback_reentrancy", "state-corruption")

	mgr := NewManager()
	var reentered int64
	// A callback that re-enters the manager via a lock-taking method.
	mgr.OnCreate(func(c *Chain) {
		atomic.AddInt64(&reentered, 1)
		// Count() takes m.mu.RLock(); from inside a write-locked section this
		// would deadlock a non-reentrant RWMutex.
		_ = mgr.Count()
		// GetAllChains also takes RLock.
		_ = mgr.GetAllChains()
	})

	done := make(chan error, 1)
	go func() {
		defer func() {
			if p := recover(); p != nil {
				done <- fmt.Errorf("panic: %v", p)
			}
		}()
		_, err := mgr.CreateChain("reentrant", true)
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			rec.Record(stresschaos.Degraded, fmt.Sprintf("re-entrant create surfaced error: %v", err))
		} else {
			rec.Record(stresschaos.Recovered, "re-entrant callback did not deadlock — manager completed create")
		}
	case <-time.After(5 * time.Second):
		rec.Record(stresschaos.Fatal, "DEADLOCK — re-entrant callback re-locked the non-reentrant RWMutex")
	}

	rec.AssertNoFatal()
	t.Logf("focus manager survived re-entrant callback (reentered=%d)", atomic.LoadInt64(&reentered))
}

// TestManager_Chaos_CorruptFocusInput feeds structurally hostile Focus values
// through the REAL PushToActive→Chain.Push→Focus.Validate path. Each input is
// invalid or hostile; the manager must reject cleanly or normalise without a
// crash. A panic on malformed input is a §11.4.85(B) Fatal.
func TestManager_Chaos_CorruptFocusInput(t *testing.T) {
	mgr := NewManager()
	if _, err := mgr.CreateChain("corrupt-host", true); err != nil {
		t.Fatalf("seed chain: %v", err)
	}

	// Descriptor index → real hostile Focus reconstructed by feed().
	payloads := [][]byte{
		[]byte("0"), // empty-ID focus (Validate rejects)
		[]byte("1"), // empty-Type focus
		[]byte("2"), // empty-Target focus
		[]byte("3"), // out-of-range priority
		[]byte("4"), // expiration before creation
		[]byte("5"), // hostile context values (NaN/Inf/chan/func)
		[]byte("6"), // huge target string
		[]byte("7"), // valid focus (must be accepted)
	}

	stresschaos.ChaosCorruptInputDuring(t, "focus_manager_corrupt_focus_input", payloads,
		func(input []byte) error {
			f := hostileFocusFor(string(input))
			// Drive through the real lock-guarded push path.
			return mgr.PushToActive(f)
		})
}

// hostileFocusFor reconstructs a hostile/invalid Focus for a chaos index.
func hostileFocusFor(idx string) *Focus {
	switch strings.TrimSpace(idx) {
	case "0":
		f := NewFocus(FocusTypeTask, "x")
		f.ID = "" // Validate: id empty
		return f
	case "1":
		f := NewFocus(FocusTypeTask, "x")
		f.Type = "" // Validate: type empty
		return f
	case "2":
		return NewFocus(FocusTypeTask, "") // Validate: target empty
	case "3":
		f := NewFocus(FocusTypeTask, "x")
		f.Priority = FocusPriority(99999) // Validate: out of range
		return f
	case "4":
		f := NewFocus(FocusTypeTask, "x")
		past := f.CreatedAt.Add(-time.Hour)
		f.ExpiresAt = &past // Validate: expiration before creation
		return f
	case "5":
		f := NewFocus(FocusTypeTask, "hostile-context")
		f.SetContext("nan", math.NaN())
		f.SetContext("inf", math.Inf(1))
		f.SetContext("chan", make(chan int))
		f.SetContext("func", func() {})
		f.SetContext("nested", map[string]interface{}{"a": map[string]interface{}{"b": math.NaN()}})
		return f
	case "6":
		return NewFocus(FocusTypeTask, strings.Repeat("x", 1<<16))
	default:
		return NewFocus(FocusTypeTask, "valid-target")
	}
}

// TestManager_Chaos_ConcurrentChurnWithClear hammers the SAME Manager with
// concurrent CreateChain / DeleteChain / SetActive / Clear / GetAll / callback
// registration from many goroutines. Clear mid-flight (full map wipe) races
// against concurrent creations/reads — the harshest state-corruption surface.
// The manager must never panic or race and must stay self-consistent. Run -race.
func TestManager_Chaos_ConcurrentChurnWithClear(t *testing.T) {
	rec := stresschaos.NewChaosRecorder(t, "focus_manager_concurrent_churn_with_clear", "state-corruption")
	mgr := NewManager()
	mgr.OnCreate(func(_ *Chain) {})
	mgr.OnDelete(func(_ *Chain) {})

	const goroutines = 12
	const iters = 250
	var wg sync.WaitGroup
	var creates, deletes, clears, reads int64
	var idMu sync.Mutex
	ids := make([]string, 0, 512)

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
				switch (id + it) % 6 {
				case 0:
					if ch, err := mgr.CreateChain(fmt.Sprintf("churn-%d-%d", id, it), it%2 == 0); err == nil {
						idMu.Lock()
						ids = append(ids, ch.ID)
						idMu.Unlock()
					}
					atomic.AddInt64(&creates, 1)
				case 1:
					var target string
					idMu.Lock()
					if len(ids) > 0 {
						target = ids[0]
						ids = ids[1:]
					}
					idMu.Unlock()
					if target != "" {
						_ = mgr.DeleteChain(target)
					}
					atomic.AddInt64(&deletes, 1)
				case 2:
					if it%50 == 0 {
						mgr.Clear()
						idMu.Lock()
						ids = ids[:0]
						idMu.Unlock()
						atomic.AddInt64(&clears, 1)
					} else {
						_ = mgr.GetAllChains()
						_ = mgr.Count()
						atomic.AddInt64(&reads, 1)
					}
				case 3:
					_ = mgr.GetStatistics()
					_ = mgr.GetRecentChains(3)
					atomic.AddInt64(&reads, 1)
				case 4:
					_ = mgr.PushToActive(newStressFocus(id*1000 + it))
					_ = mgr.CleanExpiredFocuses()
					atomic.AddInt64(&reads, 1)
				default:
					mgr.OnActivate(func(_ *Chain) {})
					atomic.AddInt64(&reads, 1)
				}
			}
		}(w)
	}
	wg.Wait()

	rec.Record(stresschaos.Recovered, fmt.Sprintf(
		"survived churn+clear: %d creates, %d deletes, %d clears, %d reads, no panic/race",
		atomic.LoadInt64(&creates), atomic.LoadInt64(&deletes),
		atomic.LoadInt64(&clears), atomic.LoadInt64(&reads)))

	// Final state must be coherent and the store must still work.
	if c := mgr.Count(); c < 0 {
		rec.Record(stresschaos.Fatal, fmt.Sprintf("chain count went negative: %d", c))
	}
	final, err := mgr.CreateChain("final", true)
	if err != nil {
		rec.Record(stresschaos.Fatal, "manager could not create after churn: "+err.Error())
	} else if err := mgr.PushToActive(newStressFocus(0)); err != nil {
		rec.Record(stresschaos.Fatal, "manager could not push after churn: "+err.Error())
	} else if _, err := mgr.GetChain(final.ID); err != nil {
		rec.Record(stresschaos.Fatal, "manager could not get after churn — map corrupted: "+err.Error())
	} else {
		rec.Record(stresschaos.Recovered, "manager fully usable after churn — map self-consistent")
	}

	rec.AssertNoFatal()
	t.Logf("focus manager churn: creates=%d deletes=%d clears=%d reads=%d final-count=%d",
		atomic.LoadInt64(&creates), atomic.LoadInt64(&deletes),
		atomic.LoadInt64(&clears), atomic.LoadInt64(&reads), mgr.Count())
}

// TestManager_Chaos_CancelDuringCreateLoop injects a process-death fault: a
// long create+push loop honours a cancellable context and must unwind cleanly
// when the context is cancelled mid-flight, without leaking the worker goroutine.
func TestManager_Chaos_CancelDuringCreateLoop(t *testing.T) {
	stresschaos.ChaosKillDuring(t, "focus_manager_cancel_during_create_loop", 40*time.Millisecond,
		func(ctx context.Context, rec *stresschaos.ChaosRecorder) {
			mgr := NewManagerWithLimit(16) // bounded so the loop stays in-memory
			iterations := 0
			for {
				select {
				case <-ctx.Done():
					rec.Record(stresschaos.Recovered, fmt.Sprintf("create loop observed cancellation after %d iterations", iterations))
					return
				default:
				}
				if _, err := mgr.CreateChain(fmt.Sprintf("loop-%d", iterations), true); err != nil {
					rec.Record(stresschaos.Degraded, "create errored mid-loop: "+err.Error())
					return
				}
				_ = mgr.PushToActive(newStressFocus(iterations))
				iterations++
			}
		})
}

// TestManager_Chaos_OperateUnderMemoryPressure asserts a large chain store
// operates under bounded memory pressure without OOM-crash (§11.4.85(B)(4)).
func TestManager_Chaos_OperateUnderMemoryPressure(t *testing.T) {
	mgr := NewManager()
	const n = 500
	ids := make([]string, n)
	for i := 0; i < n; i++ {
		ch, err := mgr.CreateChain(fmt.Sprintf("mem-%d", i), true)
		if err != nil {
			t.Fatalf("create %d: %v", i, err)
		}
		if err := mgr.PushToActive(newStressFocus(i)); err != nil {
			t.Fatalf("push %d: %v", i, err)
		}
		ids[i] = ch.ID
	}

	stresschaos.ChaosResourcePressureDuring(t, "focus_manager_operate_under_memory_pressure", 32,
		func(rec *stresschaos.ChaosRecorder) {
			for i := 0; i < n; i++ {
				if _, err := mgr.GetChain(ids[i]); err != nil {
					rec.Record(stresschaos.Fatal, fmt.Sprintf("get %d errored under pressure: %v", i, err))
					return
				}
			}
			_ = mgr.GetStatistics()
			_ = mgr.GetAllChains()
			if c := mgr.Count(); c != n {
				rec.Record(stresschaos.Fatal, fmt.Sprintf("count %d != %d under pressure — store corrupted", c, n))
				return
			}
			rec.Record(stresschaos.Recovered, fmt.Sprintf("operated %d-chain store under memory pressure", n))
		})
}
