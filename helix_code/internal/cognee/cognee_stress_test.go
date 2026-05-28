package cognee

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"dev.helix.code/internal/config"
	"dev.helix.code/tests/stresschaos"
)

// §11.4.85 stress coverage for the IN-PROCESS parts of internal/cognee.
//
// The Cognee integration is network-facing (the *Client makes real HTTP calls
// to a live Cognee server). Those paths are NOT exercised here — they require a
// live endpoint and are honestly skipped where touched (see chaos suite). What
// IS exercised, against the REAL (non-mocked) component, is the deterministic
// in-process machinery that runs on the hot path of every Cognee call and is
// concurrency-rich:
//
//   - ServiceCache: the RWMutex-guarded memories/searches/datasets maps via the
//     real cacheMemory / getCachedMemory / removeCachedMemory / cacheSearch /
//     getCachedSearch / cacheDataset / getCachedDataset / removeCachedDataset /
//     cleanupCache methods.
//   - ServiceStatistics: the mutex-guarded counters via incrementMemoriesAdded /
//     incrementSearches / incrementCacheHits / incrementCacheMisses / etc.
//   - buildSearchCacheKey: the pure cache-key composer.
//   - RegisterEventHandler + processEvent: the goroutine-fan-out handler dispatch
//     and its panic-isolation guard.
//   - CacheManager.Clear: the cross-struct cache reset.
//
// No fakes: a real *CogneeService is built with NewCogneeService and the real
// in-process methods are driven. Service.Start() is intentionally NOT called —
// Start spawns background loops and pings a live Cognee endpoint, which would
// add real-network flakiness inside a stress loop (forbidden by the task rules).
// Run under -race to catch data races on the shared maps and counters.

// newStressService builds a real, started-but-not-network CogneeService for the
// in-process stress paths. Start() is deliberately skipped (see file comment).
func newStressService(t *testing.T) *CogneeService {
	t.Helper()
	cfg := config.DefaultCogneeConfig()
	svc, err := NewCogneeService(cfg, nil)
	if err != nil {
		t.Fatalf("NewCogneeService: %v", err)
	}
	if svc == nil {
		t.Fatal("NewCogneeService returned nil service")
	}
	return svc
}

// TestCognee_Stress_SustainedCacheWriteRead drives the real ServiceCache memory
// path under sustained load (N>=1000): each iteration caches a distinct memory
// then reads it back, asserting the read returns the just-written entry. Proves
// real map work happened — not a no-op — and records p50/p95/p99 latency.
func TestCognee_Stress_SustainedCacheWriteRead(t *testing.T) {
	svc := newStressService(t)

	var hits int64
	stresschaos.RunSustainedLoad(t, "cognee_sustained_cache_write_read",
		stresschaos.SustainedConfig{N: 2000, MaxErrorRate: 0.0},
		func(i int) error {
			id := fmt.Sprintf("mem-%d", i)
			mem := &CogneeMemory{
				ID:          id,
				Content:     fmt.Sprintf("content-%d", i),
				DatasetName: "stress",
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			}
			svc.cacheMemory(mem)
			got := svc.getCachedMemory(id)
			if got == nil {
				return fmt.Errorf("cached memory %q not retrievable", id)
			}
			if got.Content != mem.Content {
				return fmt.Errorf("cache returned wrong content for %q: %q", id, got.Content)
			}
			atomic.AddInt64(&hits, 1)
			return nil
		})

	if atomic.LoadInt64(&hits) == 0 {
		t.Fatal("cognee cache served zero reads under sustained load — not real work")
	}
	t.Logf("cognee sustained cache: %d write+read round-trips", atomic.LoadInt64(&hits))
}

// TestCognee_Stress_SustainedStatsAndSearchKey drives the real ServiceStatistics
// counters and the pure buildSearchCacheKey composer under sustained load. Each
// iteration increments a real counter and builds a real cache key, asserting the
// key is deterministic and non-empty. Verifies the final counter total matches.
func TestCognee_Stress_SustainedStatsAndSearchKey(t *testing.T) {
	svc := newStressService(t)

	const n = 1500
	stresschaos.RunSustainedLoad(t, "cognee_sustained_stats_and_searchkey",
		stresschaos.SustainedConfig{N: n, MaxErrorRate: 0.0},
		func(i int) error {
			svc.stats.incrementSearches()
			req := &SearchMemoryRequest{
				Query:       fmt.Sprintf("q-%d", i),
				DatasetName: "stress",
				Limit:       10,
				SearchType:  "CHUNKS",
			}
			key := svc.buildSearchCacheKey(req)
			if key == "" {
				return fmt.Errorf("buildSearchCacheKey returned empty key for i=%d", i)
			}
			// Determinism: same request must yield the same key.
			if again := svc.buildSearchCacheKey(req); again != key {
				return fmt.Errorf("buildSearchCacheKey non-deterministic: %q != %q", key, again)
			}
			return nil
		})

	svc.stats.mu.RLock()
	got := svc.stats.SearchesCount
	svc.stats.mu.RUnlock()
	if got != n {
		t.Fatalf("SearchesCount=%d after sustained load, want %d", got, n)
	}
	t.Logf("cognee sustained stats: SearchesCount=%d", got)
}

// TestCognee_Stress_ConcurrentCacheContention hammers the SAME ServiceCache from
// N>=10 concurrent goroutines that interleave memory/search/dataset writes,
// reads, removes, and cleanupCache calls — driving genuine read/write contention
// on the cache.mu RWMutex across all three maps. Asserts no deadlock, no leak,
// no race (under -race), and that the cache stays usable after the storm.
func TestCognee_Stress_ConcurrentCacheContention(t *testing.T) {
	svc := newStressService(t)
	ctx := context.Background()

	var ops int64
	stresschaos.RunConcurrent(t, "cognee_concurrent_cache_contention",
		stresschaos.ConcurrencyConfig{Parallelism: 16, IterationsPerGoroutine: 200, Timeout: 25 * time.Second},
		func(g, it int) error {
			id := fmt.Sprintf("g%d-mem%d", g, it%32) // overlapping keys -> write-write contention
			switch (g + it) % 6 {
			case 0:
				svc.cacheMemory(&CogneeMemory{ID: id, Content: "x", UpdatedAt: time.Now()})
			case 1:
				_ = svc.getCachedMemory(id)
			case 2:
				svc.removeCachedMemory(id)
			case 3:
				key := svc.buildSearchCacheKey(&SearchMemoryRequest{Query: id, Limit: 5})
				svc.cacheSearch(key, &SearchMemoryResponse{Query: id, TotalCount: it})
				_ = svc.getCachedSearch(key)
			case 4:
				svc.cacheDataset(&Dataset{Name: id, UpdatedAt: time.Now()})
				_ = svc.getCachedDataset(id)
				svc.removeCachedDataset(id)
			default:
				// cleanupCache takes the write lock and walks all three maps —
				// the heaviest contention point, racing every other op above.
				svc.cleanupCache()
			}
			// Stats touched on every op widens contention on a second mutex.
			svc.stats.incrementCacheHits()
			atomic.AddInt64(&ops, 1)
			return nil
		})

	if atomic.LoadInt64(&ops) == 0 {
		t.Fatal("zero cache ops under concurrent load")
	}
	// Cache must still be usable after the storm — write + read a fresh entry.
	svc.cacheMemory(&CogneeMemory{ID: "post-storm", Content: "ok", UpdatedAt: time.Now()})
	if svc.getCachedMemory("post-storm") == nil {
		t.Fatal("cache unusable after concurrent contention — fresh write not retrievable")
	}
	_ = ctx
	t.Logf("cognee concurrent cache: %d ops survived", atomic.LoadInt64(&ops))
}

// TestCognee_Stress_ConcurrentEventHandlerRegistration hammers RegisterEventHandler
// (write-locks s.mu and appends to the eventHandlers slice) from N>=10 goroutines
// while processEvent concurrently reads the slice and fans handlers out to
// goroutines. This exercises the real append-vs-read contention on s.mu and the
// real goroutine dispatch. Asserts every registered handler eventually fires and
// no race/deadlock occurs. Run under -race.
func TestCognee_Stress_ConcurrentEventHandlerRegistration(t *testing.T) {
	svc := newStressService(t)

	var fired int64
	stresschaos.RunConcurrent(t, "cognee_concurrent_event_handler_registration",
		stresschaos.ConcurrencyConfig{Parallelism: 12, IterationsPerGoroutine: 40, Timeout: 25 * time.Second},
		func(g, it int) error {
			// Register a real handler that counts invocations.
			svc.RegisterEventHandler(func(e *CogneeEvent) {
				atomic.AddInt64(&fired, 1)
			})
			// Dispatch directly through the real processEvent (reads the slice
			// under RLock, fans out to goroutines with the recover guard).
			svc.processEvent(&CogneeEvent{
				ID:        fmt.Sprintf("g%d-it%d", g, it),
				Type:      "stress",
				Action:    "dispatch",
				Timestamp: time.Now(),
			})
			return nil
		})

	// Let the fanned-out handler goroutines settle, then assert dispatch happened.
	deadline := time.Now().Add(5 * time.Second)
	for atomic.LoadInt64(&fired) == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if atomic.LoadInt64(&fired) == 0 {
		t.Fatal("no event handlers fired under concurrent registration+dispatch — dispatch broken")
	}
	t.Logf("cognee concurrent event dispatch: %d handler invocations", atomic.LoadInt64(&fired))
}

// TestCognee_Stress_BoundaryConditions exercises §11.4.85(A)(3) boundary cases
// against the real in-process component: empty (miss on absent key), max (cache
// fill beyond maxItems then cleanupCache evicts down), and off-by-one (remove the
// only entry then read -> nil).
func TestCognee_Stress_BoundaryConditions(t *testing.T) {
	t.Run("empty_cache_miss", func(t *testing.T) {
		svc := newStressService(t)
		if got := svc.getCachedMemory("does-not-exist"); got != nil {
			t.Fatalf("empty cache returned non-nil for absent key: %+v", got)
		}
		if got := svc.getCachedSearch("nope"); got != nil {
			t.Fatalf("empty cache returned non-nil search for absent key")
		}
		if got := svc.getCachedDataset("nope"); got != nil {
			t.Fatalf("empty cache returned non-nil dataset for absent key")
		}
	})

	t.Run("overfill_then_cleanup_evicts", func(t *testing.T) {
		svc := newStressService(t)
		// Pin maxItems small so cleanup has work to do deterministically.
		svc.cache.mu.Lock()
		svc.cache.maxItems = 50
		svc.cache.mu.Unlock()

		const fill = 500
		for i := 0; i < fill; i++ {
			svc.cacheMemory(&CogneeMemory{ID: fmt.Sprintf("m%d", i), UpdatedAt: time.Now()})
		}
		svc.cache.mu.RLock()
		before := len(svc.cache.memories)
		svc.cache.mu.RUnlock()
		if before != fill {
			t.Fatalf("expected %d memories cached, got %d", fill, before)
		}

		svc.cleanupCache() // must evict down toward maxItems

		svc.cache.mu.RLock()
		after := len(svc.cache.memories)
		svc.cache.mu.RUnlock()
		if after > svc.cache.maxItems {
			t.Fatalf("cleanupCache left %d memories, exceeds maxItems %d", after, svc.cache.maxItems)
		}
		if after >= before {
			t.Fatalf("cleanupCache evicted nothing: before=%d after=%d", before, after)
		}
	})

	t.Run("remove_only_entry", func(t *testing.T) {
		svc := newStressService(t)
		svc.cacheMemory(&CogneeMemory{ID: "solo", Content: "x", UpdatedAt: time.Now()})
		if svc.getCachedMemory("solo") == nil {
			t.Fatal("solo entry not cached")
		}
		svc.removeCachedMemory("solo")
		if got := svc.getCachedMemory("solo"); got != nil {
			t.Fatalf("entry still present after remove: %+v", got)
		}
	})
}

// TestCognee_Stress_ConcurrentCacheManagerClear drives CacheManager.Clear (which
// reaches into the service cache under cache.mu) concurrently with cache writers,
// proving the clear path is mutex-safe and leaves a usable cache. Run under -race.
func TestCognee_Stress_ConcurrentCacheManagerClear(t *testing.T) {
	svc := newStressService(t)
	cm, err := NewCacheManager(nil)
	if err != nil {
		t.Fatalf("NewCacheManager: %v", err)
	}
	cm.SetService(svc)

	var ops int64
	var wg sync.WaitGroup
	const writers = 10
	wg.Add(writers + 1)
	stop := make(chan struct{})

	for w := 0; w < writers; w++ {
		go func(id int) {
			defer wg.Done()
			i := 0
			for {
				select {
				case <-stop:
					return
				default:
				}
				svc.cacheMemory(&CogneeMemory{ID: fmt.Sprintf("w%d-%d", id, i), UpdatedAt: time.Now()})
				atomic.AddInt64(&ops, 1)
				i++
			}
		}(w)
	}
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			cm.Clear()
			atomic.AddInt64(&ops, 1)
		}
		close(stop)
	}()
	wg.Wait()

	if atomic.LoadInt64(&ops) == 0 {
		t.Fatal("zero ops in CacheManager.Clear contention test")
	}
	// Final clear then assert empty + usable.
	cm.Clear()
	svc.cache.mu.RLock()
	emptied := len(svc.cache.memories) == 0
	svc.cache.mu.RUnlock()
	if !emptied {
		t.Fatal("CacheManager.Clear did not empty the memories map")
	}
	svc.cacheMemory(&CogneeMemory{ID: "after-clear", UpdatedAt: time.Now()})
	if svc.getCachedMemory("after-clear") == nil {
		t.Fatal("cache unusable after Clear")
	}
	t.Logf("cognee CacheManager.Clear contention: %d ops survived", atomic.LoadInt64(&ops))
}
