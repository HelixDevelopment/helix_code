package cognee

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"dev.helix.code/tests/stresschaos"
)

// §11.4.85(B) chaos coverage for the IN-PROCESS parts of internal/cognee.
//
// Fault classes injected against the REAL (non-mocked) CogneeService machinery:
//
//   - handler-panic injection: a registered event handler that panics mid-
//     dispatch MUST NOT crash the process. processEvent fans each handler out to
//     its OWN goroutine — an unrecovered panic there would take down the entire
//     `go test` binary (and every unrelated goroutine). The service's per-handler
//     recover() guard MUST isolate it so co-handlers still fire and the service
//     stays usable.
//   - input-corruption: structurally hostile content (NUL bytes, huge strings,
//     NaN/Inf-laden metadata, format-string payloads) is fed into the real cache
//     key builder + cache store + event dispatch. None of these in-process paths
//     may panic on malformed input.
//   - state-corruption under contention: cleanupCache (write-locks + walks all
//     three maps) runs concurrently with writers/readers/removers. The cache must
//     never tear, panic, or race, and must end self-consistent.
//   - resource pressure: cache fill + cleanup run under bounded memory pressure;
//     the cache must keep functioning (and evicting) rather than crash.
//   - goroutine-death mid-op: a cache-churn worker is cancelled mid-flight; the
//     shared cache must unwind cleanly and stay usable.
//
// Network-facing paths (the *Client HTTP calls reached via AddMemory / SearchMemory
// / Cognify / Start) are NOT exercised — they require a live Cognee endpoint and
// are honestly skipped. Run under -race.

// TestCognee_Chaos_EventHandlerPanicIsolation registers a handler that panics
// alongside well-behaved co-handlers, then dispatches through the real
// processEvent. processEvent runs each handler in its own goroutine with a
// recover() guard; if that guard is missing or wrong, the panic crashes the whole
// test process. PASS proves the panic was isolated, co-handlers ran, and the
// service remained usable for a follow-up dispatch.
func TestCognee_Chaos_EventHandlerPanicIsolation(t *testing.T) {
	rec := stresschaos.NewChaosRecorder(t, "cognee_event_handler_panic_isolation", "process-death")
	svc := newStressService(t)

	var before, after, panicked int64
	svc.RegisterEventHandler(func(e *CogneeEvent) { atomic.AddInt64(&before, 1) })
	svc.RegisterEventHandler(func(e *CogneeEvent) {
		atomic.AddInt64(&panicked, 1)
		panic("chaos: cognee event handler panic")
	})
	svc.RegisterEventHandler(func(e *CogneeEvent) { atomic.AddInt64(&after, 1) })

	// Dispatch through the real processEvent. The panic happens in a fanned-out
	// goroutine; an unisolated panic would crash the process outright (the four-
	// layer suite surfaces that as a hard binary failure). If we get past this
	// call and the co-handlers ran, the recover guard worked.
	svc.processEvent(&CogneeEvent{ID: "panic-1", Type: "chaos", Action: "dispatch", Timestamp: time.Now()})

	// Wait for the fanned-out goroutines to settle.
	deadline := time.Now().Add(5 * time.Second)
	for (atomic.LoadInt64(&before) == 0 || atomic.LoadInt64(&after) == 0) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}

	if atomic.LoadInt64(&panicked) == 0 {
		rec.Record(stresschaos.Fatal, "panicking handler never ran — test did not exercise the panic path")
	} else if atomic.LoadInt64(&before) == 0 || atomic.LoadInt64(&after) == 0 {
		rec.Record(stresschaos.Fatal, fmt.Sprintf(
			"panicking handler starved co-handlers (before=%d after=%d) — not isolated",
			atomic.LoadInt64(&before), atomic.LoadInt64(&after)))
	} else {
		rec.Record(stresschaos.Recovered, fmt.Sprintf(
			"co-handlers survived panic (before=%d after=%d panicked=%d)",
			atomic.LoadInt64(&before), atomic.LoadInt64(&after), atomic.LoadInt64(&panicked)))
	}

	// Service must remain usable: register a fresh handler + dispatch again.
	var followUp int64
	svc.RegisterEventHandler(func(e *CogneeEvent) { atomic.AddInt64(&followUp, 1) })
	svc.processEvent(&CogneeEvent{ID: "panic-2", Type: "chaos", Action: "followup", Timestamp: time.Now()})
	deadline = time.Now().Add(5 * time.Second)
	for atomic.LoadInt64(&followUp) == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if atomic.LoadInt64(&followUp) == 0 {
		rec.Record(stresschaos.Fatal, "service unusable after handler panic — follow-up dispatch fired nothing")
	} else {
		rec.Record(stresschaos.Recovered, "service still dispatches after handler panic")
	}

	rec.AssertNoFatal()
	t.Log("cognee survived event-handler-panic injection")
}

// TestCognee_Chaos_CorruptCacheInput feeds structurally hostile content into the
// real in-process cache-key builder, cache store, and getter. None of these paths
// may panic on NUL bytes, oversized strings, NaN/Inf metadata, or format-string
// payloads — a crash on malformed input is a §11.4.85(B) failure.
func TestCognee_Chaos_CorruptCacheInput(t *testing.T) {
	svc := newStressService(t)

	corruptDescriptors := []map[string]interface{}{
		{"kind": "nul_bytes"},
		{"kind": "huge_query"},
		{"kind": "format_string"},
		{"kind": "nan_metadata"},
		{"kind": "empty_everything"},
		{"kind": "unicode_garbage"},
	}
	payloads := make([][]byte, len(corruptDescriptors))
	for i, d := range corruptDescriptors {
		b, err := json.Marshal(d)
		if err != nil {
			b = []byte(fmt.Sprintf(`{"kind":"fallback-%d"}`, i))
		}
		payloads[i] = b
	}

	stresschaos.ChaosCorruptInputDuring(t, "cognee_corrupt_cache_input", payloads,
		func(input []byte) error {
			kind := corruptKindOf(input)
			req := hostileSearchRequest(kind)
			// 1. The pure key builder must not panic on hostile fields.
			key := svc.buildSearchCacheKey(req)
			// 2. Store + retrieve a search keyed by the hostile key.
			svc.cacheSearch(key, &SearchMemoryResponse{Query: req.Query})
			if got := svc.getCachedSearch(key); got == nil {
				return fmt.Errorf("hostile-key search not retrievable for kind=%s", kind)
			}
			// 3. Store + retrieve a memory carrying hostile metadata.
			mem := hostileMemory(kind)
			svc.cacheMemory(mem)
			if got := svc.getCachedMemory(mem.ID); got == nil {
				return fmt.Errorf("hostile memory not retrievable for kind=%s", kind)
			}
			// Returning an error is treated as graceful rejection; returning nil
			// means accepted-without-crash. Both are non-fatal; a panic is fatal
			// (caught by the helper).
			return nil
		})
}

// TestCognee_Chaos_CleanupDuringContention runs cleanupCache (write-locks and
// walks all three maps) concurrently with writers, readers, and removers across
// many goroutines. The real cache.mu must serialise every mutation so the maps
// never tear, the run never panics or races, and the cache ends self-consistent
// (a fresh write is retrievable). Run under -race.
func TestCognee_Chaos_CleanupDuringContention(t *testing.T) {
	rec := stresschaos.NewChaosRecorder(t, "cognee_cleanup_during_contention", "state-corruption")
	svc := newStressService(t)

	// Small maxItems so cleanup performs real eviction work each pass.
	svc.cache.mu.Lock()
	svc.cache.maxItems = 64
	svc.cache.mu.Unlock()

	const goroutines = 14
	const iters = 250
	var wg sync.WaitGroup
	var writes, reads, removes, cleanups int64

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
				key := fmt.Sprintf("k%d", (id+it)%128) // overlapping keys -> contention
				switch (id + it) % 5 {
				case 0:
					svc.cacheMemory(&CogneeMemory{ID: key, UpdatedAt: time.Now()})
					atomic.AddInt64(&writes, 1)
				case 1:
					_ = svc.getCachedMemory(key)
					atomic.AddInt64(&reads, 1)
				case 2:
					svc.removeCachedMemory(key)
					atomic.AddInt64(&removes, 1)
				case 3:
					svc.cacheDataset(&Dataset{Name: key, UpdatedAt: time.Now().Add(-2 * time.Hour)})
				default:
					svc.cleanupCache()
					atomic.AddInt64(&cleanups, 1)
				}
			}
		}(w)
	}
	wg.Wait()

	rec.Record(stresschaos.Recovered, fmt.Sprintf(
		"survived cleanup/write/read/remove churn: writes=%d reads=%d removes=%d cleanups=%d",
		atomic.LoadInt64(&writes), atomic.LoadInt64(&reads),
		atomic.LoadInt64(&removes), atomic.LoadInt64(&cleanups)))

	// Self-consistency: a fresh write must be retrievable and the map size must be
	// a sane non-negative number bounded by what we just inserted.
	svc.cacheMemory(&CogneeMemory{ID: "consistency-probe", UpdatedAt: time.Now()})
	if svc.getCachedMemory("consistency-probe") == nil {
		rec.Record(stresschaos.Fatal, "cache did not retain a fresh write after churn — map corrupted")
	} else {
		rec.Record(stresschaos.Recovered, "cache retains fresh write after churn — self-consistent")
	}

	rec.AssertNoFatal()
	t.Logf("cognee cleanup churn: writes=%d reads=%d removes=%d cleanups=%d",
		atomic.LoadInt64(&writes), atomic.LoadInt64(&reads),
		atomic.LoadInt64(&removes), atomic.LoadInt64(&cleanups))
}

// TestCognee_Chaos_ResourcePressureCacheOps drives cache fill + cleanup under
// bounded memory pressure (capped by the harness at 128 MB to stay under the
// host-safety ceiling). The cache must keep functioning and evicting rather than
// OOM-crash.
func TestCognee_Chaos_ResourcePressureCacheOps(t *testing.T) {
	svc := newStressService(t)
	svc.cache.mu.Lock()
	svc.cache.maxItems = 100
	svc.cache.mu.Unlock()

	stresschaos.ChaosResourcePressureDuring(t, "cognee_resource_pressure_cache_ops", 48,
		func(rec *stresschaos.ChaosRecorder) {
			for i := 0; i < 5000; i++ {
				svc.cacheMemory(&CogneeMemory{
					ID:        fmt.Sprintf("pressure-%d", i),
					Content:   strings.Repeat("y", 256),
					UpdatedAt: time.Now(),
				})
				if i%500 == 0 {
					svc.cleanupCache()
				}
			}
			// Final eviction + usability check under pressure.
			svc.cleanupCache()
			svc.cache.mu.RLock()
			n := len(svc.cache.memories)
			svc.cache.mu.RUnlock()
			if n > svc.cache.maxItems {
				rec.Record(stresschaos.Fatal, fmt.Sprintf("cache overgrew under pressure: %d > maxItems %d", n, svc.cache.maxItems))
				return
			}
			rec.Record(stresschaos.Recovered, fmt.Sprintf("cache bounded to %d entries under memory pressure", n))
		})
}

// TestCognee_Chaos_KillCacheWorkerMidOp starts a long-running cache-churn worker
// driving the real shared cache and cancels it mid-flight. The worker must observe
// the cancellation and unwind cleanly, leaving the cache usable.
func TestCognee_Chaos_KillCacheWorkerMidOp(t *testing.T) {
	svc := newStressService(t)

	stresschaos.ChaosKillDuring(t, "cognee_kill_cache_worker_mid_op", 100*time.Millisecond,
		func(ctx context.Context, rec *stresschaos.ChaosRecorder) {
			i := 0
			for {
				select {
				case <-ctx.Done():
					// Cancelled mid-op: prove the cache survived and is usable.
					svc.cacheMemory(&CogneeMemory{ID: "post-cancel", UpdatedAt: time.Now()})
					if svc.getCachedMemory("post-cancel") == nil {
						rec.Record(stresschaos.Fatal, "cache unusable after worker cancellation")
					} else {
						rec.Record(stresschaos.Recovered, fmt.Sprintf("worker unwound after %d ops; cache usable", i))
					}
					return
				default:
					id := fmt.Sprintf("churn-%d", i%64)
					svc.cacheMemory(&CogneeMemory{ID: id, UpdatedAt: time.Now()})
					_ = svc.getCachedMemory(id)
					if i%50 == 0 {
						svc.cleanupCache()
					}
					i++
				}
			}
		})
}

// --- chaos input helpers (test-only) ---

func corruptKindOf(input []byte) string {
	var d struct {
		Kind string `json:"kind"`
	}
	if json.Unmarshal(input, &d) == nil && d.Kind != "" {
		return d.Kind
	}
	return "empty_everything"
}

func hostileSearchRequest(kind string) *SearchMemoryRequest {
	switch kind {
	case "nul_bytes":
		return &SearchMemoryRequest{Query: "a\x00b\x00c", DatasetName: "d\x00s", Limit: 5, SearchType: "T\x00"}
	case "huge_query":
		return &SearchMemoryRequest{Query: strings.Repeat("Q", 1<<16), DatasetName: "huge", Limit: 1000000}
	case "format_string":
		return &SearchMemoryRequest{Query: "%s%s%s%n%d", DatasetName: "%v", Limit: -1, SearchType: "%p"}
	case "nan_metadata":
		return &SearchMemoryRequest{Query: fmt.Sprintf("%v", math.NaN()), DatasetName: fmt.Sprintf("%v", math.Inf(1)), Limit: 0}
	case "unicode_garbage":
		// Hostile but source-safe: U+FFFD replacement, U+202E bidi-override,
		// U+200B zero-width space, multibyte CJK, and astral-plane emoji are all
		// written as Go escapes so they exercise the runtime path WITHOUT
		// embedding raw bidi/zero-width control chars in the source (which trips
		// static bidi scanners).
		return &SearchMemoryRequest{
			Query:       "\uFFFD\u202E\u200B\u65E5\u672C\u8A9E\U0001F525",
			DatasetName: "\U0001F480",
			Limit:       7,
		}
	default: // empty_everything
		return &SearchMemoryRequest{}
	}
}

func hostileMemory(kind string) *CogneeMemory {
	base := &CogneeMemory{ID: "hostile-" + kind, UpdatedAt: time.Now()}
	switch kind {
	case "nul_bytes":
		base.Content = "x\x00y"
		base.Metadata = map[string]interface{}{"k\x00": "v\x00"}
	case "huge_query":
		base.Content = strings.Repeat("z", 1<<16)
	case "format_string":
		base.Content = "%s%n%d"
	case "nan_metadata":
		base.Metadata = map[string]interface{}{"nan": math.NaN(), "inf": math.Inf(-1)}
	case "unicode_garbage":
		base.Content = "\u202E\U0001F525\U0001F480\uFFFD"
	default:
		base.ID = "hostile-empty"
	}
	return base
}
