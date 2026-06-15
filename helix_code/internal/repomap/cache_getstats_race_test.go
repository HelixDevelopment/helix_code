// Package repomap — cache_getstats_race_test.go (DEFECT-2 regression guard).
//
// §11.4.115 RED-on-broken-artifact + polarity switch. Reproduces the data race
// in (*RepoCache).GetStats: it holds only rc.mu.RLock() but lazily WRITES
// entry.SizeBytes = size for entries with SizeBytes==0 (the legacy / pre-SizeBytes
// disk-load path). Two concurrent GetStats calls write the same *cacheEntry under
// a READ lock → data race (flagged by -race).
//
// Polarity (§11.4.115): these SAME tests are the bug-catcher AND the standing
// GREEN regression guard. On the broken artifact -race fires on the shared
// entry.SizeBytes write; on the fixed artifact GetStats does not mutate shared
// state under the read lock, so concurrent calls are race-free and the totals
// stay correct.
//
// The -race detector is the oracle for the data-race portion. Total-correctness
// is asserted independently so a "fix" that silently drops legacy-entry size
// from the total cannot pass.
package repomap

import (
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// newGetStatsTestCache builds a real RepoCache (real background writer, real temp
// cache dir) for the GetStats race guard.
func newGetStatsTestCache(t *testing.T) *RepoCache {
	t.Helper()
	rc, err := NewRepoCache(t.TempDir(), time.Hour)
	require.NoError(t, err)
	t.Cleanup(func() { _ = rc.Close() })
	return rc
}

// makeLegacyEntries inserts n entries directly into rc.entries with SizeBytes==0
// (simulating entries loaded from a pre-SizeBytes disk file / Import path), each
// with a non-trivial Value so estimateSize returns >0.
func makeLegacyEntries(t *testing.T, rc *RepoCache, n int) {
	t.Helper()
	farFuture := time.Now().Add(24 * time.Hour)
	rc.mu.Lock()
	defer rc.mu.Unlock()
	for i := 0; i < n; i++ {
		key := "legacy/file_" + strconv.Itoa(i) + ".go:123"
		rc.entries[key] = &cacheEntry{
			Key:       key,
			Value:     map[string]interface{}{"symbols": []string{"foo", "bar", "baz"}, "path": key},
			ExpiresAt: farFuture,
			SizeBytes: 0, // legacy / pre-SizeBytes — triggers the lazy back-fill path
		}
	}
}

// TestGetStats_ConcurrentLegacyEntries_NoRace reproduces DEFECT-2: 16 goroutines
// call GetStats concurrently over entries with SizeBytes==0. On the broken
// artifact each GetStats writes entry.SizeBytes under the RLock → the -race
// detector reports concurrent writes to the same *cacheEntry.
//
// On the fixed artifact GetStats computes the size into a local for the total and
// never mutates the shared entry under the read lock — no race, total correct.
func TestGetStats_ConcurrentLegacyEntries_NoRace(t *testing.T) {
	rc := newGetStatsTestCache(t)
	const numEntries = 32
	makeLegacyEntries(t, rc, numEntries)

	// Expected total: each legacy entry contributes estimateSize(Value).
	var expectedTotal int64
	rc.mu.RLock()
	for _, e := range rc.entries {
		expectedTotal += estimateSize(e.Value)
	}
	rc.mu.RUnlock()
	require.Greater(t, expectedTotal, int64(0), "test fixture must have positive sizes")

	const goroutines = 16
	var wg sync.WaitGroup
	wg.Add(goroutines)
	start := make(chan struct{})
	results := make([]int64, goroutines)
	for g := 0; g < goroutines; g++ {
		go func(idx int) {
			defer wg.Done()
			<-start
			s := rc.GetStats()
			results[idx] = s.TotalSize
		}(g)
	}
	close(start)
	wg.Wait()

	// Every concurrent reader must observe the correct total — a fix that drops
	// legacy-entry size, or that races and under/over-counts, fails here.
	for g := 0; g < goroutines; g++ {
		require.Equal(t, expectedTotal, results[g],
			"GetStats total mismatch in goroutine %d", g)
	}

	// And a final sequential call still agrees.
	require.Equal(t, expectedTotal, rc.GetStats().TotalSize)
	require.Equal(t, numEntries, rc.GetStats().TotalEntries)
}
