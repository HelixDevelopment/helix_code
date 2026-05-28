package context

import (
	stdctx "context"
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"dev.helix.code/tests/stresschaos"
)

// §11.4.85(B) chaos coverage for the context package.
//
// Chaos classes exercised against the REAL components (config is a real minimal
// *config.ContextConfig; the RWMutex-guarded state machines are the real units
// under test):
//
//   - process-death / lifecycle injection: a ContextManager's background cleanup
//     goroutine is Stop()ped mid-flight while a load is in progress. Stop() MUST
//     unwind the goroutine cleanly (cm.wg.Wait returns) and concurrent ongoing
//     Store/Retrieve calls MUST keep working (the maps are independent of the
//     cleanup routine's liveness) — no panic, no deadlock, no leaked goroutine.
//   - state-corruption under contention: a single ContextItem ID is stored,
//     deleted, re-stored, and read concurrently by many goroutines. The manager
//     MUST never panic/race and MUST end in a self-consistent state.
//   - input-corruption: structurally hostile ContextItem values (NaN/Inf floats,
//     channels, funcs, huge keys, nil metadata) are fed to Store. The manager
//     MUST accept/normalise or reject without crashing.
//   - resource-exhaustion: sustained Store under bounded memory pressure MUST
//     degrade gracefully, never OOM-crash.

// TestContextManager_Chaos_StopDuringLoad starts a real ContextManager, drives a
// concurrent Store/Retrieve load against it, then Stop()s it mid-flight. Stop
// MUST unwind the background cleanup goroutine within the harness window and the
// in-flight operations MUST not panic. Run under -race.
func TestContextManager_Chaos_StopDuringLoad(t *testing.T) {
	stresschaos.ChaosKillDuring(t, "context_manager_stop_during_load", 60*time.Millisecond,
		func(ctx stdctx.Context, rec *stresschaos.ChaosRecorder) {
			cm := NewContextManager(stressConfig())
			if err := cm.Start(stdctx.Background()); err != nil {
				rec.Record(stresschaos.Fatal, "start failed: "+err.Error())
				return
			}

			var wg sync.WaitGroup
			var ops int64
			stopOnce := make(chan struct{})

			// Workers store/retrieve until the injected context is cancelled.
			const workers = 8
			for w := 0; w < workers; w++ {
				wg.Add(1)
				go func(id int) {
					defer wg.Done()
					defer func() {
						if p := recover(); p != nil {
							rec.Record(stresschaos.Fatal, fmt.Sprintf("worker %d panicked: %v", id, p))
						}
					}()
					i := 0
					for {
						select {
						case <-ctx.Done():
							return
						case <-stopOnce:
							return
						default:
						}
						itemID := fmt.Sprintf("w%d-%d", id, i)
						_ = cm.Store(stdctx.Background(), &ContextItem{
							ID: itemID, Type: ContextTypeGlobal, Key: itemID, Value: i,
						})
						_, _ = cm.Retrieve(stdctx.Background(), itemID)
						atomic.AddInt64(&ops, 1)
						i++
					}
				}(w)
			}

			// Wait for the injected cancellation (process-death signal), then Stop
			// the manager mid-load and verify it unwinds.
			<-ctx.Done()
			close(stopOnce)

			done := make(chan struct{})
			go func() {
				cm.Stop() // closes stopChan + wg.Wait() on the cleanup routine
				close(done)
			}()
			select {
			case <-done:
				rec.Record(stresschaos.Recovered, fmt.Sprintf("manager Stop() unwound cleanly mid-load after %d ops", atomic.LoadInt64(&ops)))
			case <-time.After(5 * time.Second):
				rec.Record(stresschaos.Fatal, "manager Stop() did not unwind within 5s (deadlock)")
			}

			wg.Wait() // ensure no worker goroutine leaks
			rec.Record(stresschaos.Recovered, "all workers unwound after stop")
		})
}

// TestContextManager_Chaos_ConcurrentStoreDeleteSameID hammers a single ID with
// concurrent Store / Delete / Retrieve / Search from many goroutines. The real
// cm.mu must serialise so no panic/race occurs and the manager ends consistent
// (the ID is either present or absent, never torn). Run under -race.
func TestContextManager_Chaos_ConcurrentStoreDeleteSameID(t *testing.T) {
	rec := stresschaos.NewChaosRecorder(t, "context_manager_store_delete_churn", "state-corruption")
	cm := NewContextManager(stressConfig())
	if err := cm.Start(stdctx.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer cm.Stop()
	ctx := stdctx.Background()

	const id = "churn-target"
	const writers = 8
	const readers = 8
	const iters = 400
	var wg sync.WaitGroup
	var stores, deletes, reads int64

	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(gid int) {
			defer wg.Done()
			defer func() {
				if p := recover(); p != nil {
					rec.Record(stresschaos.Fatal, fmt.Sprintf("writer %d panicked: %v", gid, p))
				}
			}()
			for it := 0; it < iters; it++ {
				if (gid+it)%2 == 0 {
					if err := cm.Store(ctx, &ContextItem{ID: id, Type: ContextTypeGlobal, Key: id, Value: gid}); err == nil {
						atomic.AddInt64(&stores, 1)
					}
				} else {
					if err := cm.Delete(ctx, id); err == nil {
						atomic.AddInt64(&deletes, 1)
					}
				}
			}
		}(w)
	}

	for r := 0; r < readers; r++ {
		wg.Add(1)
		go func(gid int) {
			defer wg.Done()
			defer func() {
				if p := recover(); p != nil {
					rec.Record(stresschaos.Fatal, fmt.Sprintf("reader %d panicked: %v", gid, p))
				}
			}()
			for it := 0; it < iters; it++ {
				if item, err := cm.Retrieve(ctx, id); err == nil {
					if item.ID != id {
						rec.Record(stresschaos.Fatal, fmt.Sprintf("torn read: id=%q want %q", item.ID, id))
					}
					atomic.AddInt64(&reads, 1)
				}
				_, _ = cm.Search(ctx, id, ContextTypeGlobal)
			}
		}(r)
	}

	wg.Wait()
	rec.Record(stresschaos.Recovered, fmt.Sprintf(
		"survived store/delete churn: %d stores, %d deletes, %d clean reads, no panic/race",
		atomic.LoadInt64(&stores), atomic.LoadInt64(&deletes), atomic.LoadInt64(&reads)))

	// Final state must be self-consistent: a final Store then Retrieve must work,
	// and Statistics must be queryable without panic.
	if err := cm.Store(ctx, &ContextItem{ID: id, Type: ContextTypeGlobal, Key: id, Value: "final"}); err != nil {
		rec.Record(stresschaos.Fatal, "final store failed: "+err.Error())
	} else if _, err := cm.Retrieve(ctx, id); err != nil {
		rec.Record(stresschaos.Fatal, "final retrieve failed: "+err.Error())
	} else {
		_ = cm.GetStatistics()
		rec.Record(stresschaos.Recovered, "final state consistent and queryable")
	}

	rec.AssertNoFatal()
	t.Logf("context chaos churn: stores=%d deletes=%d reads=%d",
		atomic.LoadInt64(&stores), atomic.LoadInt64(&deletes), atomic.LoadInt64(&reads))
}

// TestContextManager_Chaos_CorruptInputValue feeds structurally hostile
// ContextItem values to the REAL Store. The manager must accept/normalise or
// reject each without panicking — a crash on malformed input is a §11.4.85(B)
// failure. The Value field is interface{}, so non-marshalable types (chan/func),
// NaN/Inf, oversized keys, and nil metadata exercise every Store branch.
func TestContextManager_Chaos_CorruptInputValue(t *testing.T) {
	cm := NewContextManager(stressConfig())
	if err := cm.Start(stdctx.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer cm.Stop()
	ctx := stdctx.Background()

	// Descriptor payloads honour the helper's [][]byte contract; feed() maps each
	// index to a real hostile ContextItem.
	descriptors := []map[string]interface{}{
		{"corrupt_index": 0}, // NaN value
		{"corrupt_index": 1}, // +Inf value
		{"corrupt_index": 2}, // channel value (not marshalable)
		{"corrupt_index": 3}, // func value (not marshalable)
		{"corrupt_index": 4}, // 64 KiB key + value
		{"corrupt_index": 5}, // nil metadata on a session-typed item
		{"corrupt_index": 6}, // empty ID
	}
	payloads := make([][]byte, len(descriptors))
	for i, d := range descriptors {
		b, err := json.Marshal(d)
		if err != nil {
			b = []byte(fmt.Sprintf(`{"corrupt_index":%d}`, i))
		}
		payloads[i] = b
	}

	stresschaos.ChaosCorruptInputDuring(t, "context_manager_corrupt_input", payloads,
		func(input []byte) error {
			var probe struct {
				CorruptIndex int `json:"corrupt_index"`
			}
			_ = json.Unmarshal(input, &probe)
			item := hostileItemFor(probe.CorruptIndex)
			// A non-nil error is graceful rejection; nil (accepted/normalised) is
			// also fine as long as no panic — the helper records both as non-fatal.
			return cm.Store(ctx, item)
		})
}

// hostileItemFor builds the actual hostile ContextItem for a chaos index —
// including types that cannot survive []byte serialisation but exercise Store.
func hostileItemFor(idx int) *ContextItem {
	switch idx {
	case 0:
		return &ContextItem{ID: "nan", Type: ContextTypeGlobal, Key: "nan", Value: math.NaN()}
	case 1:
		return &ContextItem{ID: "inf", Type: ContextTypeGlobal, Key: "inf", Value: math.Inf(1)}
	case 2:
		return &ContextItem{ID: "chan", Type: ContextTypeGlobal, Key: "chan", Value: make(chan int)}
	case 3:
		return &ContextItem{ID: "func", Type: ContextTypeGlobal, Key: "func", Value: func() {}}
	case 4:
		h := makeHugeString(1 << 16)
		return &ContextItem{ID: "huge", Type: ContextTypeGlobal, Key: h, Value: h}
	case 5:
		// Session-typed item with nil metadata: the session_id type-assert branch
		// must not panic on a nil map.
		return &ContextItem{ID: "nilmeta", Type: ContextTypeSession, Key: "nilmeta", Value: 1, Metadata: nil}
	default:
		// Empty ID — boundary: must store under "" without panicking.
		return &ContextItem{ID: "", Type: ContextTypeGlobal, Key: "empty", Value: nil}
	}
}

// makeHugeString returns an n-byte string for oversized-input chaos.
func makeHugeString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = 'x'
	}
	return string(b)
}

// TestContextManager_Chaos_ResourcePressure drives sustained Store under bounded
// memory pressure (capped by the harness at <=128 MiB, §12.6-safe). The manager
// must keep storing/retrieving without OOM-crash.
func TestContextManager_Chaos_ResourcePressure(t *testing.T) {
	stresschaos.ChaosResourcePressureDuring(t, "context_manager_resource_pressure", 64,
		func(rec *stresschaos.ChaosRecorder) {
			cm := NewContextManager(stressConfig())
			if err := cm.Start(stdctx.Background()); err != nil {
				rec.Record(stresschaos.Fatal, "start under pressure failed: "+err.Error())
				return
			}
			defer cm.Stop()
			ctx := stdctx.Background()

			var stored int
			for i := 0; i < 2000; i++ {
				id := fmt.Sprintf("rp-%d", i)
				if err := cm.Store(ctx, &ContextItem{ID: id, Type: ContextTypeGlobal, Key: id, Value: i}); err != nil {
					rec.Record(stresschaos.Degraded, fmt.Sprintf("store %d backpressured: %v", i, err))
					continue
				}
				if _, err := cm.Retrieve(ctx, id); err != nil {
					rec.Record(stresschaos.Degraded, fmt.Sprintf("retrieve %d backpressured: %v", i, err))
					continue
				}
				stored++
			}
			if stored == 0 {
				rec.Record(stresschaos.Fatal, "manager stored nothing under memory pressure")
				return
			}
			rec.Record(stresschaos.Recovered, fmt.Sprintf("stored+retrieved %d items under bounded memory pressure", stored))
		})
}
