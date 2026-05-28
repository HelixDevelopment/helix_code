package persistence

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"dev.helix.code/internal/focus"
	"dev.helix.code/internal/memory"
	"dev.helix.code/internal/session"
	"dev.helix.code/tests/stresschaos"
)

// §11.4.85(B) chaos coverage for internal/persistence.
//
// Chaos classes exercised against the REAL file-backed *Store and the REAL
// JSON/gzip serializers (no fakes — real disk I/O via t.TempDir(), real
// RWMutex-guarded state, real callback dispatch):
//
//   - input-corruption: structurally hostile SERIALIZED data fed to the real
//     Deserialize / DetectFormat / Validate path (truncated gzip, bad magic
//     bytes, malformed JSON, binary garbage, empty). Must reject cleanly or
//     normalise — a panic on malformed bytes is a §11.4.85(B) Fatal.
//   - corrupt persisted FILES on disk: a save directory is populated with
//     garbage files, then LoadAll must skip the unreadable entries (the code's
//     per-file `continue`) and never crash.
//   - callback-panic injection: an OnSave/OnLoad/OnError callback that panics
//     mid-dispatch MUST NOT take down the store or leave its write lock held —
//     the store must stay usable afterwards (deadlock guard).
//   - state-corruption under contention: concurrent SaveAll / LoadAll / Clear /
//     callback-registration on one store. The RWMutex + slices must never panic
//     or race and the store must end self-consistent. Run under -race.
//   - process-death: a long save/create loop is cancelled mid-operation; it must
//     observe cancellation and unwind without leaking a goroutine.
//   - resource-pressure: serializing a large store proceeds under bounded memory
//     pressure without OOM-crash.

// TestSerializer_Chaos_CorruptDeserializeInput feeds structurally hostile bytes
// to the REAL Deserialize / DetectFormat / Validate paths of both serializers.
// The custom gzip byte-slice reader is a prime crash candidate on truncated
// input. Graceful rejection (error) is desired; a panic is Fatal.
func TestSerializer_Chaos_CorruptDeserializeInput(t *testing.T) {
	corrupt := [][]byte{
		nil,                                          // 0: nil slice
		{},                                           // 1: empty
		{0x1f, 0x8b},                                 // 2: gzip magic only, no body (truncated)
		{0x1f, 0x8b, 0x08, 0x00, 0x00},               // 3: gzip header, truncated body
		[]byte("{not valid json"),                    // 4: malformed JSON
		[]byte("\x00\x01\x02\xff\xfe binary garbage"), // 5: binary garbage
		[]byte("[1,2,3"),                             // 6: truncated JSON array
		[]byte(`{"deeply":{"nested":{"but":"valid"}}}`), // 7: valid JSON, wrong shape for target
		[]byte("null"),                               // 8: JSON null
		make([]byte, 1<<16),                          // 9: 64KiB of zero bytes
	}

	jsonSer := NewJSONSerializer()
	gzipSer := NewJSONGzipSerializer()

	stresschaos.ChaosCorruptInputDuring(t, "persistence_serializer_corrupt_deserialize", corrupt,
		func(input []byte) error {
			// DetectFormat + Validate must not crash on hostile bytes.
			format, _ := DetectFormat(input)
			_ = Validate(input, format)
			_ = Validate(input, FormatJSONGzip)
			_ = Validate(input, FormatBinary)

			var out SaveMetadata
			if err := jsonSer.Deserialize(input, &out); err != nil {
				return err // graceful rejection (Degraded) — desired
			}
			if err := gzipSer.Deserialize(input, &out); err != nil {
				return err
			}
			return nil
		})
}

// TestStore_Chaos_CorruptPersistedFiles writes garbage files into the real save
// directories on disk, then drives LoadAll. The loader's per-file deserialize
// failure path (`continue`) must skip every unreadable entry without crashing,
// while still loading any valid sibling — proving real corrupt-file resilience.
func TestStore_Chaos_CorruptPersistedFiles(t *testing.T) {
	rec := stresschaos.NewChaosRecorder(t, "persistence_corrupt_persisted_files", "input-corruption")

	store, sessMgr, memMgr, focMgr := newStressStore(t)
	// Persist a few real, valid items first.
	for i := 0; i < 5; i++ {
		_, _ = sessMgr.Create("proj", fmt.Sprintf("good-%d", i), "valid", session.ModePlanning)
		_, _ = memMgr.CreateConversation(fmt.Sprintf("good-conv-%d", i))
		_, _ = focMgr.CreateChain(fmt.Sprintf("good-chain-%d", i), false)
	}
	if err := store.SaveAll(); err != nil {
		t.Fatalf("seed SaveAll: %v", err)
	}

	// Now inject corrupt files alongside the valid ones in each subdir.
	garbage := [][]byte{
		[]byte("{not json"),
		{0x1f, 0x8b, 0xde, 0xad},
		[]byte("\x00\x00\x00\x00"),
		[]byte(""),
	}
	for _, sub := range []string{"sessions", "conversations", "focus"} {
		for gi, g := range garbage {
			fn := fmt.Sprintf("%s/%s/corrupt-%d.json", store.basePath, sub, gi)
			if err := writeAtomic(fn, g); err != nil {
				t.Fatalf("plant corrupt file: %v", err)
			}
		}
	}

	// LoadAll into a fresh manager set: the corrupt files must be skipped, valid
	// items still recovered, and no crash.
	func() {
		defer func() {
			if p := recover(); p != nil {
				rec.Record(stresschaos.Fatal, fmt.Sprintf("LoadAll panicked on corrupt files: %v", p))
			}
		}()
		reload, err := NewStore(store.basePath)
		if err != nil {
			rec.Record(stresschaos.Fatal, "reopen store: "+err.Error())
			return
		}
		rs, rm, rf := session.NewManager(), memory.NewManager(), focus.NewManager()
		reload.SetSessionManager(rs)
		reload.SetMemoryManager(rm)
		reload.SetFocusManager(rf)
		if err := reload.LoadAll(); err != nil {
			rec.Record(stresschaos.Degraded, "LoadAll surfaced error on corrupt dir: "+err.Error())
			return
		}
		// The 5 valid sessions must survive; corrupt files skipped.
		if got := len(rs.GetAll()); got != 5 {
			rec.Record(stresschaos.Fatal, fmt.Sprintf("recovered %d sessions, want 5 (corrupt files were not skipped cleanly)", got))
			return
		}
		rec.Record(stresschaos.Recovered, fmt.Sprintf("loaded %d valid sessions, skipped corrupt files without crash", len(rs.GetAll())))
	}()

	rec.AssertNoFatal()
	t.Log("persistence LoadAll survived corrupt-persisted-file injection")
}

// TestStore_Chaos_SaveCallbackPanicIsolation registers an OnSave callback that
// panics and then drives SaveAll. The callback is dispatched from inside SaveAll
// while the store holds its write lock; if the store does not isolate the panic
// it propagates to the caller AND — critically — leaves s.mu locked, deadlocking
// every subsequent operation. The store MUST stay usable.
func TestStore_Chaos_SaveCallbackPanicIsolation(t *testing.T) {
	rec := stresschaos.NewChaosRecorder(t, "persistence_save_callback_panic_isolation", "process-death")

	store, sessMgr, _, _ := newStressStore(t)
	_, _ = sessMgr.Create("proj", "cb-sess", "for callback", session.ModePlanning)

	var goodHits int64
	store.OnSave(func(_ *SaveMetadata) { atomic.AddInt64(&goodHits, 1) })
	store.OnSave(func(_ *SaveMetadata) { panic("chaos: OnSave callback panic") })
	store.OnSave(func(_ *SaveMetadata) { atomic.AddInt64(&goodHits, 1) })

	// Drive SaveAll on a guarded scope: if the store does not isolate the
	// callback panic it propagates here. We catch it and record Degraded, but the
	// real test is whether the store mutex was left locked (deadlock below).
	func() {
		defer func() {
			if p := recover(); p != nil {
				rec.Record(stresschaos.Degraded, fmt.Sprintf("SaveAll propagated callback panic to caller: %v", p))
			}
		}()
		if err := store.SaveAll(); err != nil {
			rec.Record(stresschaos.Degraded, "SaveAll surfaced error: "+err.Error())
		} else {
			rec.Record(stresschaos.Recovered, "SaveAll completed despite panicking callback")
		}
	}()

	// CRITICAL: the store must remain usable. If the write lock was left held by
	// the panicking callback path, this follow-up op blocks forever — we guard it
	// with a timeout and record Fatal (deadlock) if it does not return.
	followUp := make(chan error, 1)
	go func() { followUp <- store.SaveAll() }()
	select {
	case err := <-followUp:
		if err != nil {
			rec.Record(stresschaos.Degraded, "follow-up SaveAll errored: "+err.Error())
		} else {
			rec.Record(stresschaos.Recovered, "store still usable after callback panic — lock not leaked")
		}
	case <-time.After(5 * time.Second):
		rec.Record(stresschaos.Fatal, "store deadlocked after callback panic — write lock leaked")
	}

	rec.AssertNoFatal()
	t.Logf("store survived save-callback-panic injection (good-callback hits=%d)", atomic.LoadInt64(&goodHits))
}

// TestStore_Chaos_LoadAndErrorCallbackPanicIsolation extends the panic-isolation
// proof to the OnLoad and OnError callback dispatch paths, which also run under
// the write lock (LoadAll) / unguarded (triggerError). A panic must not crash the
// process or leave the store unusable.
func TestStore_Chaos_LoadAndErrorCallbackPanicIsolation(t *testing.T) {
	rec := stresschaos.NewChaosRecorder(t, "persistence_load_error_callback_panic", "process-death")

	store, sessMgr, _, _ := newStressStore(t)
	_, _ = sessMgr.Create("proj", "ld-sess", "for load cb", session.ModePlanning)
	if err := store.SaveAll(); err != nil {
		t.Fatalf("seed SaveAll: %v", err)
	}

	store.OnLoad(func(_ *LoadMetadata) { panic("chaos: OnLoad callback panic") })
	store.OnError(func(_ error) { panic("chaos: OnError callback panic") })

	// OnLoad panic path.
	func() {
		defer func() {
			if p := recover(); p != nil {
				rec.Record(stresschaos.Degraded, fmt.Sprintf("LoadAll propagated callback panic: %v", p))
			}
		}()
		if err := store.LoadAll(); err != nil {
			rec.Record(stresschaos.Degraded, "LoadAll surfaced error: "+err.Error())
		} else {
			rec.Record(stresschaos.Recovered, "LoadAll completed despite panicking callback")
		}
	}()

	// OnError panic path: triggerError is invoked from autoSaveLoop on save error.
	func() {
		defer func() {
			if p := recover(); p != nil {
				rec.Record(stresschaos.Degraded, fmt.Sprintf("triggerError propagated callback panic: %v", p))
			}
		}()
		store.triggerError(fmt.Errorf("synthetic chaos error"))
		rec.Record(stresschaos.Recovered, "triggerError dispatched despite panicking callback")
	}()

	// Store must remain usable: a follow-up LoadAll must not deadlock.
	followUp := make(chan error, 1)
	go func() { followUp <- store.LoadAll() }()
	select {
	case err := <-followUp:
		if err != nil {
			rec.Record(stresschaos.Degraded, "follow-up LoadAll errored: "+err.Error())
		} else {
			rec.Record(stresschaos.Recovered, "store usable after load/error callback panic — lock not leaked")
		}
	case <-time.After(5 * time.Second):
		rec.Record(stresschaos.Fatal, "store deadlocked after load callback panic — lock leaked")
	}

	rec.AssertNoFatal()
	t.Log("store survived load+error callback-panic injection")
}

// TestStore_Chaos_ConcurrentChurnWithCallbackRegistration hammers ONE store with
// concurrent SaveAll / LoadAll / Clear / GetLastSaveTime PLUS concurrent
// OnSave/OnLoad/OnError registration from many goroutines. The callback-slice
// mutation racing against the SaveAll/LoadAll iteration of those slices is the
// harshest state-corruption surface in this package. The store must never panic
// or race and must end self-consistent. Run -race.
func TestStore_Chaos_ConcurrentChurnWithCallbackRegistration(t *testing.T) {
	rec := stresschaos.NewChaosRecorder(t, "persistence_concurrent_churn_callbacks", "state-corruption")

	store, sessMgr, memMgr, focMgr := newStressStore(t)
	for i := 0; i < 10; i++ {
		_, _ = sessMgr.Create("proj", fmt.Sprintf("seed-%d", i), "seed", session.ModePlanning)
		_, _ = memMgr.CreateConversation(fmt.Sprintf("seed-conv-%d", i))
		_, _ = focMgr.CreateChain(fmt.Sprintf("seed-chain-%d", i), false)
	}

	const goroutines = 12
	const iters = 120
	var wg sync.WaitGroup
	var saves, loads, clears, regs, reads int64

	for w := 0; w < goroutines; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if p := recover(); p != nil {
					rec.Record(stresschaos.Fatal, fmt.Sprintf("goroutine %d panicked: %v", id, p))
				}
			}()
			rs, rm, rf := session.NewManager(), memory.NewManager(), focus.NewManager()
			for it := 0; it < iters; it++ {
				switch (id + it) % 5 {
				case 0:
					_ = store.SaveAll()
					atomic.AddInt64(&saves, 1)
				case 1:
					// LoadAll into this goroutine's own managers to avoid mutating
					// the shared seed managers, but still drive the locked load path.
					store.SetSessionManager(rs)
					store.SetMemoryManager(rm)
					store.SetFocusManager(rf)
					_ = store.LoadAll()
					atomic.AddInt64(&loads, 1)
				case 2:
					if it%30 == 0 {
						_ = store.Clear()
						atomic.AddInt64(&clears, 1)
					} else {
						_ = store.GetLastSaveTime()
						atomic.AddInt64(&reads, 1)
					}
				case 3:
					// Register callbacks mid-churn — exercises the callback-slice
					// mutation racing against Save/Load iteration of those slices.
					store.OnSave(func(_ *SaveMetadata) {})
					store.OnLoad(func(_ *LoadMetadata) {})
					store.OnError(func(_ error) {})
					atomic.AddInt64(&regs, 1)
				default:
					_ = store.GetLastSaveTime()
					atomic.AddInt64(&reads, 1)
				}
			}
		}(w)
	}
	wg.Wait()

	rec.Record(stresschaos.Recovered, fmt.Sprintf(
		"survived churn: %d saves, %d loads, %d clears, %d callback-regs, %d reads, no panic/race",
		atomic.LoadInt64(&saves), atomic.LoadInt64(&loads),
		atomic.LoadInt64(&clears), atomic.LoadInt64(&regs), atomic.LoadInt64(&reads)))

	// Store must still work after the churn.
	store.SetSessionManager(sessMgr)
	store.SetMemoryManager(memMgr)
	store.SetFocusManager(focMgr)
	if err := store.SaveAll(); err != nil {
		rec.Record(stresschaos.Fatal, "store unusable after churn: "+err.Error())
	} else {
		rec.Record(stresschaos.Recovered, "store saves correctly after churn — self-consistent")
	}

	rec.AssertNoFatal()
	t.Logf("persistence churn: saves=%d loads=%d clears=%d regs=%d reads=%d",
		atomic.LoadInt64(&saves), atomic.LoadInt64(&loads),
		atomic.LoadInt64(&clears), atomic.LoadInt64(&regs), atomic.LoadInt64(&reads))
}

// TestStore_Chaos_CancelDuringSaveLoop injects a process-death fault: a long
// create+save loop honours a cancellable context and must unwind cleanly when the
// context is cancelled mid-flight, without leaking the worker goroutine.
func TestStore_Chaos_CancelDuringSaveLoop(t *testing.T) {
	stresschaos.ChaosKillDuring(t, "persistence_cancel_during_save_loop", 40*time.Millisecond,
		func(ctx context.Context, rec *stresschaos.ChaosRecorder) {
			store, sessMgr, _, _ := newStressStoreCtx(t)
			iterations := 0
			for {
				select {
				case <-ctx.Done():
					rec.Record(stresschaos.Recovered, fmt.Sprintf("save loop observed cancellation after %d iterations", iterations))
					return
				default:
				}
				if _, err := sessMgr.Create("proj", fmt.Sprintf("loop-%d", iterations), "loop", session.ModePlanning); err != nil {
					rec.Record(stresschaos.Degraded, "create errored mid-loop: "+err.Error())
					return
				}
				if err := store.SaveAll(); err != nil {
					rec.Record(stresschaos.Degraded, "save errored mid-loop: "+err.Error())
					return
				}
				iterations++
			}
		})
}

// TestStore_Chaos_SaveUnderMemoryPressure asserts serializing a large populated
// store proceeds under bounded memory pressure without OOM-crash (§11.4.85(B)(4)).
func TestStore_Chaos_SaveUnderMemoryPressure(t *testing.T) {
	store, sessMgr, memMgr, focMgr := newStressStore(t)
	const n = 300
	for i := 0; i < n; i++ {
		_, _ = sessMgr.Create("proj", fmt.Sprintf("mem-%d", i), "mem pressure", session.ModePlanning)
		_, _ = memMgr.CreateConversation(fmt.Sprintf("mem-conv-%d", i))
		_, _ = focMgr.CreateChain(fmt.Sprintf("mem-chain-%d", i), false)
	}

	stresschaos.ChaosResourcePressureDuring(t, "persistence_save_under_memory_pressure", 32,
		func(rec *stresschaos.ChaosRecorder) {
			for i := 0; i < 10; i++ {
				if err := store.SaveAll(); err != nil {
					rec.Record(stresschaos.Fatal, fmt.Sprintf("SaveAll[%d] errored under pressure: %v", i, err))
					return
				}
			}
			rec.Record(stresschaos.Recovered, fmt.Sprintf("saved %d-item store under memory pressure", n))
		})
}

// newStressStoreCtx is a *testing.T-friendly variant used inside chaos closures
// that need their own store; mirrors newStressStore but is named distinctly so
// the closure body stays readable.
func newStressStoreCtx(t testing.TB) (*Store, *session.Manager, *memory.Manager, *focus.Manager) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	sessMgr := session.NewManager()
	memMgr := memory.NewManager()
	focMgr := focus.NewManager()
	store.SetSessionManager(sessMgr)
	store.SetMemoryManager(memMgr)
	store.SetFocusManager(focMgr)
	return store, sessMgr, memMgr, focMgr
}
