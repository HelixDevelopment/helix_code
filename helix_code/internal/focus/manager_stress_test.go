package focus

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"dev.helix.code/tests/stresschaos"
)

// §11.4.85(A) stress coverage for the focus *Manager / *Chain.
//
// All suites exercise the REAL components (no fakes) — real RWMutex-guarded
// chain map, real callback slices, real Chain.Push/Pop navigation, real
// per-Focus validation. They prove the two §11.4.85(A) survival properties:
//
//   - sustained load: N>=100 CreateChain+PushToActive+navigate cycles with
//     p50/p95/p99 latency captured.
//   - concurrent contention: >=10 goroutines hammering CreateChain / SetActive /
//     PushToActive / DeleteChain / GetAllChains / callback-registration against a
//     single shared Manager. Run under -race to catch unsynchronised access.
//   - boundary conditions: empty manager, zero/negative GetRecent, max-chains
//     eviction, single-element chains.

// newStressFocus builds a valid Focus (passes Focus.Validate) for stress use.
func newStressFocus(i int) *Focus {
	return NewFocus(FocusTypeTask, fmt.Sprintf("stress-target-%d", i))
}

// TestManager_Stress_SustainedCreatePushNavigate drives a full create→push→
// navigate→delete lifecycle under sustained load. Each iteration creates a
// chain, sets it active, pushes several focuses, walks the chain, and reads
// statistics — exercising the lock-guarded happy path N>=100 times.
func TestManager_Stress_SustainedCreatePushNavigate(t *testing.T) {
	mgr := NewManager()

	rep := stresschaos.RunSustainedLoad(t, "focus_manager_sustained_lifecycle",
		stresschaos.SustainedConfig{N: 400, MaxErrorRate: 0.0},
		func(i int) error {
			chain, err := mgr.CreateChain(fmt.Sprintf("chain-%d", i), true)
			if err != nil {
				return fmt.Errorf("create chain: %w", err)
			}
			// Push three focuses to the active chain.
			for j := 0; j < 3; j++ {
				if err := mgr.PushToActive(newStressFocus(i*10 + j)); err != nil {
					return fmt.Errorf("push focus: %w", err)
				}
			}
			// Read back the current focus (proves push really landed).
			if _, err := mgr.GetCurrentFocus(); err != nil {
				return fmt.Errorf("get current focus: %w", err)
			}
			_ = mgr.GetStatistics()
			_ = mgr.Count()
			// Walk navigation on the chain itself.
			if _, err := chain.First(); err != nil {
				return fmt.Errorf("chain first: %w", err)
			}
			// Keep the map bounded so the run stays in-memory.
			if i%50 == 0 {
				mgr.Clear()
			}
			return nil
		})

	if rep.N < 100 {
		t.Fatalf("sustained run below §11.4.85(A) floor: N=%d", rep.N)
	}
	t.Logf("focus manager sustained lifecycle: N=%d p50=%.3fms p95=%.3fms p99=%.3fms",
		rep.N, rep.P50Ms, rep.P95Ms, rep.P99Ms)
}

// TestManager_Stress_ConcurrentContention hammers ONE shared Manager from >=10
// goroutines doing create/activate/push/delete/read/callback-register. The
// RWMutex must serialise every mutation so the chain map never races or panics.
// Run under -race. Errors are tolerated inside the closure (a delete losing a
// race to another goroutine's delete is normal) — what matters is no panic, no
// deadlock, no race, and a coherent final state.
func TestManager_Stress_ConcurrentContention(t *testing.T) {
	mgr := NewManager()
	// Seed callbacks so notify-under-lock contends with the readers/writers.
	var cbHits int64
	mgr.OnCreate(func(_ *Chain) { atomic.AddInt64(&cbHits, 1) })
	mgr.OnDelete(func(_ *Chain) { atomic.AddInt64(&cbHits, 1) })
	mgr.OnActivate(func(_ *Chain) { atomic.AddInt64(&cbHits, 1) })

	// Track chain IDs created by each goroutine so deletes target real chains.
	var idMu sync.Mutex
	ids := make([]string, 0, 1024)

	stresschaos.RunConcurrent(t, "focus_manager_concurrent_contention",
		stresschaos.ConcurrencyConfig{Parallelism: 16, IterationsPerGoroutine: 200, Timeout: 30 * time.Second},
		func(gid, iter int) error {
			switch (gid + iter) % 7 {
			case 0:
				ch, err := mgr.CreateChain(fmt.Sprintf("g%d-i%d", gid, iter), iter%2 == 0)
				if err == nil {
					idMu.Lock()
					ids = append(ids, ch.ID)
					idMu.Unlock()
				}
			case 1:
				_ = mgr.PushToActive(newStressFocus(gid*1000 + iter))
			case 2:
				var target string
				idMu.Lock()
				if len(ids) > 0 {
					target = ids[(gid+iter)%len(ids)]
				}
				idMu.Unlock()
				if target != "" {
					_ = mgr.SetActiveChain(target)
				}
			case 3:
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
			case 4:
				_ = mgr.GetAllChains()
				_ = mgr.Count()
				_ = mgr.GetStatistics()
			case 5:
				_ = mgr.FindChainsByName("g")
				_ = mgr.GetRecentChains(5)
			default:
				// Register a callback mid-churn — exercises the callback-slice
				// mutation racing against notify-under-lock.
				mgr.OnCreate(func(_ *Chain) {})
			}
			return nil
		})

	// Final state must be coherent and the manager must still work.
	if c := mgr.Count(); c < 0 {
		t.Fatalf("chain count went negative after churn: %d", c)
	}
	final, err := mgr.CreateChain("final", true)
	if err != nil {
		t.Fatalf("manager unusable after churn — create failed: %v", err)
	}
	if err := mgr.PushToActive(newStressFocus(999999)); err != nil {
		t.Fatalf("manager unusable after churn — push failed: %v", err)
	}
	if _, err := mgr.GetChain(final.ID); err != nil {
		t.Fatalf("manager unusable after churn — get failed: %v", err)
	}
	t.Logf("focus manager concurrent contention survived; callback-hits=%d final-count=%d",
		atomic.LoadInt64(&cbHits), mgr.Count())
}

// TestManager_Stress_MaxChainsEviction stresses the bounded-manager eviction
// path under load: a manager with a small maxChains is flooded with creations,
// each forcing removeOldest. The count must never exceed the configured limit
// and the manager must stay self-consistent.
func TestManager_Stress_MaxChainsEviction(t *testing.T) {
	const limit = 8
	mgr := NewManagerWithLimit(limit)

	rep := stresschaos.RunSustainedLoad(t, "focus_manager_maxchains_eviction",
		stresschaos.SustainedConfig{N: 300, MaxErrorRate: 0.0},
		func(i int) error {
			// setActive=false so removeOldest always has a non-active victim.
			if _, err := mgr.CreateChain(fmt.Sprintf("evict-%d", i), false); err != nil {
				return fmt.Errorf("create under limit: %w", err)
			}
			if c := mgr.Count(); c > limit {
				return fmt.Errorf("count %d exceeded limit %d", c, limit)
			}
			return nil
		})

	if c := mgr.Count(); c > limit {
		t.Fatalf("final count %d exceeds limit %d", c, limit)
	}
	t.Logf("maxchains eviction held limit=%d across N=%d (final-count=%d) p95=%.3fms",
		limit, rep.N, mgr.Count(), rep.P95Ms)
}

// TestManager_Stress_Boundaries exercises boundary inputs against the REAL
// manager: empty-manager reads, zero/negative GetRecentChains, get-missing,
// delete-missing, push-with-no-active, and single-chain navigation. None may
// panic and each must return the documented error / empty result.
func TestManager_Stress_Boundaries(t *testing.T) {
	mgr := NewManager()

	// Empty-manager reads must be safe.
	if c := mgr.Count(); c != 0 {
		t.Fatalf("fresh manager count=%d, want 0", c)
	}
	if got := mgr.GetAllChains(); len(got) != 0 {
		t.Fatalf("fresh manager GetAllChains len=%d, want 0", len(got))
	}
	if _, err := mgr.GetActiveChain(); err == nil {
		t.Fatal("GetActiveChain on empty manager should error")
	}
	if _, err := mgr.GetCurrentFocus(); err == nil {
		t.Fatal("GetCurrentFocus on empty manager should error")
	}
	if err := mgr.PushToActive(newStressFocus(0)); err == nil {
		t.Fatal("PushToActive with no active chain should error")
	}

	// Zero / negative GetRecentChains must return empty, not panic.
	for _, n := range []int{0, -1, -1000} {
		if got := mgr.GetRecentChains(n); len(got) != 0 {
			t.Fatalf("GetRecentChains(%d) len=%d, want 0", n, len(got))
		}
	}

	// Get/Delete missing IDs must error cleanly.
	if _, err := mgr.GetChain("does-not-exist"); err == nil {
		t.Fatal("GetChain(missing) should error")
	}
	if err := mgr.DeleteChain("does-not-exist"); err == nil {
		t.Fatal("DeleteChain(missing) should error")
	}
	if err := mgr.SetActiveChain("does-not-exist"); err == nil {
		t.Fatal("SetActiveChain(missing) should error")
	}

	// Single-chain navigation boundary.
	ch, err := mgr.CreateChain("single", true)
	if err != nil {
		t.Fatalf("create single: %v", err)
	}
	if err := mgr.PushToActive(newStressFocus(1)); err != nil {
		t.Fatalf("push single: %v", err)
	}
	if _, err := ch.Next(); err == nil {
		t.Fatal("Next on single-element chain should error (already at last)")
	}
	if _, err := ch.Previous(); err == nil {
		t.Fatal("Previous on single-element chain should error (already at first)")
	}
	// GetRecentChains larger than population must clamp, not panic.
	if got := mgr.GetRecentChains(1000); len(got) != 1 {
		t.Fatalf("GetRecentChains(1000) len=%d, want 1", len(got))
	}
	t.Log("focus manager boundary conditions all handled without panic")
}
