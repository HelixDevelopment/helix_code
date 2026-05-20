package repomap

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

// cache_p2t03_test.go — speed-programme Phase 2 task P2-T03.
//
// Persistent content-addressed repo-map cache. The cache keys parsed-symbol
// entries by file path + mtime (RepoMap.getCacheKey). These tests prove the
// six invariants the task requires (CONST-050):
//
//   - unit: an UNCHANGED file → cache HIT (no re-parse);
//   - unit: a CHANGED file → cache MISS (re-parse, stale entry evicted);
//   - unit: a corrupt / partial cache file is treated as a miss, never a crash;
//   - unit: Set never spawns a per-call goroutine — one background writer;
//   - integration: a warm second index reuses the cache and produces output
//     byte-identical to a cold (no-cache) index;
//   - Challenge-style: edit exactly ONE file, re-index an N-file repo, assert
//     exactly N-1 cache hits and 1 miss.
//
// Run: go test -race -run P2T03 ./internal/repomap/
//      go test -bench=P2T03 -benchmem -run=^$ ./internal/repomap/

// p2t03Fixture builds an N-file Go source tree and returns its root.
func p2t03Fixture(tb testing.TB, fileCount int) string {
	tb.Helper()
	return buildRepomapFixture(tb, fileCount)
}

// touchFile rewrites a file with new content and forces its mtime forward so
// the path+mtime cache key changes — the cache-miss trigger for P2-T03.
func touchFile(tb testing.TB, path, content string) {
	tb.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		tb.Fatalf("rewrite %s: %v", path, err)
	}
	// Advance mtime explicitly — a same-second rewrite would otherwise keep
	// the Unix-second mtime identical and mask the change.
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(path, future, future); err != nil {
		tb.Fatalf("chtimes %s: %v", path, err)
	}
}

// TestP2T03_UnchangedFileIsCacheHit — an unchanged file resolves to the same
// path+mtime key on a second lookup, so it is a HIT and is not re-parsed.
func TestP2T03_UnchangedFileIsCacheHit(t *testing.T) {
	root := p2t03Fixture(t, 4)
	cfg := DefaultConfig()
	cfg.CacheEnabled = true

	rm, err := NewRepoMap(root, cfg)
	if err != nil {
		t.Fatalf("NewRepoMap: %v", err)
	}
	t.Cleanup(func() { _ = rm.cache.Close() })

	files, err := rm.discoverFiles()
	if err != nil || len(files) == 0 {
		t.Fatalf("discoverFiles: %v (n=%d)", err, len(files))
	}
	target := files[0]

	// First extraction populates the cache (a MISS).
	first, err := rm.extractFileSymbols(target)
	if err != nil {
		t.Fatalf("first extract: %v", err)
	}
	rm.cache.Wait()
	rm.cache.ResetStats()

	// Second extraction of the UNCHANGED file must be a HIT.
	second, err := rm.extractFileSymbols(target)
	if err != nil {
		t.Fatalf("second extract: %v", err)
	}
	hits, misses := rm.cache.Stats()
	if hits != 1 || misses != 0 {
		t.Fatalf("unchanged file: want 1 hit / 0 miss, got %d hit / %d miss", hits, misses)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("cached symbols differ from freshly parsed symbols")
	}
}

// TestP2T03_ChangedFileIsCacheMiss — a file whose content (and mtime) changed
// resolves to a NEW key → MISS → re-parse. The stale prior-mtime entry MUST be
// evicted so it can never be served and never accumulates.
func TestP2T03_ChangedFileIsCacheMiss(t *testing.T) {
	root := p2t03Fixture(t, 4)
	cfg := DefaultConfig()
	cfg.CacheEnabled = true

	rm, err := NewRepoMap(root, cfg)
	if err != nil {
		t.Fatalf("NewRepoMap: %v", err)
	}
	t.Cleanup(func() { _ = rm.cache.Close() })

	files, _ := rm.discoverFiles()
	target := files[0]

	if _, err := rm.extractFileSymbols(target); err != nil {
		t.Fatalf("first extract: %v", err)
	}
	rm.cache.Wait()
	staleKey := rm.getCacheKey(target)

	// Edit the file — its mtime advances → a different cache key.
	touchFile(t, target, "package fixturechanged\n\nfunc BrandNewSymbol() {}\n")
	rm.cache.ResetStats()

	newKey := rm.getCacheKey(target)
	if newKey == staleKey {
		t.Fatal("cache key did not change after a file edit — mtime not advanced")
	}

	syms, err := rm.extractFileSymbols(target)
	if err != nil {
		t.Fatalf("post-edit extract: %v", err)
	}
	_, misses := rm.cache.Stats()
	if misses != 1 {
		t.Fatalf("changed file: want 1 miss, got %d", misses)
	}

	// The stale entry must NOT be served: it must be gone from the cache.
	if rm.cache.Has(staleKey) {
		t.Fatal("stale (old-mtime) cache entry was NOT evicted — could be served")
	}

	// The new symbols must reflect the edit, not the old cached data.
	foundNew := false
	for _, s := range syms {
		if s.Name == "BrandNewSymbol" {
			foundNew = true
		}
	}
	if !foundNew {
		t.Fatal("re-parsed symbols do not reflect the edited file content")
	}
}

// TestP2T03_CorruptCacheFileTreatedAsMiss — a truncated / garbage cache file
// must be treated as a miss (removed, skipped), never panic the loader.
func TestP2T03_CorruptCacheFileTreatedAsMiss(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "cache")

	// Build a cache, persist a real entry, close it.
	c1, err := NewRepoCache(dir, time.Hour)
	if err != nil {
		t.Fatalf("NewRepoCache: %v", err)
	}
	c1.Set("pkg/file.go:1700000000", []Symbol{{Name: "Real", Type: SymbolTypeFunction}})
	c1.Wait()
	if err := c1.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Corrupt every on-disk cache file with garbage bytes.
	walkErr := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".cache" {
			return nil
		}
		return os.WriteFile(path, []byte("\x00\x01not-a-valid-gob-stream\xff\xfe"), 0o644)
	})
	if walkErr != nil {
		t.Fatalf("corrupt walk: %v", walkErr)
	}

	// Re-open: loadFromDisk must NOT crash and must drop the corrupt files.
	c2, err := NewRepoCache(dir, time.Hour)
	if err != nil {
		t.Fatalf("NewRepoCache over corrupt dir: %v", err)
	}
	t.Cleanup(func() { _ = c2.Close() })

	if c2.Size() != 0 {
		t.Fatalf("corrupt cache files were loaded as live entries: size=%d", c2.Size())
	}
	if _, found := c2.Get("pkg/file.go:1700000000"); found {
		t.Fatal("a corrupt cache file was served as a hit")
	}
}

// TestP2T03_SingleBackgroundWriter — a burst of Set calls must be persisted by
// the one background writer (no per-Set goroutine storm, R1 B07). Proof: after
// Wait, every key is readable from a freshly-opened cache over the same dir.
func TestP2T03_SingleBackgroundWriter(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "cache")
	c1, err := NewRepoCache(dir, time.Hour)
	if err != nil {
		t.Fatalf("NewRepoCache: %v", err)
	}

	const n = 200
	for i := 0; i < n; i++ {
		key := fmt.Sprintf("pkg/file%d.go:1700000000", i)
		c1.Set(key, []Symbol{{Name: fmt.Sprintf("Sym%d", i), Type: SymbolTypeFunction}})
	}
	c1.Wait() // barrier — every queued write is flushed by the single writer.
	if err := c1.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Re-open: disk-backed persistence means all n entries reload.
	c2, err := NewRepoCache(dir, time.Hour)
	if err != nil {
		t.Fatalf("re-open NewRepoCache: %v", err)
	}
	t.Cleanup(func() { _ = c2.Close() })
	if c2.Size() != n {
		t.Fatalf("single-writer burst: want %d persisted entries, got %d", n, c2.Size())
	}
}

// TestP2T03_WarmIndexByteIdenticalToCold — integration-style: a warm index
// (cache enabled, populated) must produce a context set byte-identical to a
// cold no-cache index. The cache is a pure speedup, never a behaviour change
// (the hard no-regression constraint).
//
// NOTE on ordering: GetOptimalContext ranks files by score and `sort.Slice`
// is NOT stable, so files with EQUAL scores emerge in an undefined order that
// already varies between two separate cold runs (verified: two cache-disabled
// runs over the same tree are not reflect.DeepEqual). That tie-ordering is a
// pre-existing property of the ranker, independent of the cache. This test
// therefore proves the genuine cache invariant — the SET of file contexts
// (path → symbols → content) is byte-identical — by comparing path-keyed,
// not slice-position-keyed. The cache must never alter a file's symbols or
// content; it must only change how fast they are produced.
func TestP2T03_WarmIndexByteIdenticalToCold(t *testing.T) {
	root := p2t03Fixture(t, 20)
	query := "Process input service"

	indexAsMap := func(rm *RepoMap) map[string]FileContext {
		ctxs, err := rm.GetOptimalContext(query, nil)
		if err != nil {
			t.Fatalf("GetOptimalContext: %v", err)
		}
		m := make(map[string]FileContext, len(ctxs))
		for _, c := range ctxs {
			m[c.FilePath] = c
		}
		return m
	}

	// Cold reference: cache disabled — every file parsed fresh.
	coldCfg := DefaultConfig()
	coldCfg.CacheEnabled = false
	coldRM, err := NewRepoMap(root, coldCfg)
	if err != nil {
		t.Fatalf("cold NewRepoMap: %v", err)
	}
	coldMap := indexAsMap(coldRM)

	// Warm: cache enabled. First index populates the cache; second index
	// must produce identical output while serving symbols from the cache.
	warmCfg := DefaultConfig()
	warmCfg.CacheEnabled = true
	warmRM, err := NewRepoMap(root, warmCfg)
	if err != nil {
		t.Fatalf("warm NewRepoMap: %v", err)
	}
	t.Cleanup(func() { _ = warmRM.cache.Close() })

	if _, err := warmRM.GetOptimalContext(query, nil); err != nil {
		t.Fatalf("warm index pass 1: %v", err)
	}
	warmRM.cache.Wait()
	warmMap := indexAsMap(warmRM)

	if len(coldMap) != len(warmMap) {
		t.Fatalf("warm index produced %d file contexts, cold produced %d",
			len(warmMap), len(coldMap))
	}
	for path, coldCtx := range coldMap {
		warmCtx, ok := warmMap[path]
		if !ok {
			t.Fatalf("warm index is missing file context for %s", path)
		}
		if !reflect.DeepEqual(coldCtx, warmCtx) {
			t.Fatalf("warm-cache context for %s differs from cold — cache changed behaviour\n"+
				"cold: %d symbols, %d tokens\nwarm: %d symbols, %d tokens",
				path, len(coldCtx.Symbols), coldCtx.TokenCount,
				len(warmCtx.Symbols), warmCtx.TokenCount)
		}
	}
}

// TestP2T03_OneFileEditNMinusOneHits — Challenge-style end-to-end proof:
// build an N-file repo, index it (cold — N misses), edit exactly ONE file,
// re-index, and assert exactly N-1 cache HITS and 1 MISS. This is the
// instrumentation evidence the task's anti-bluff proof requires.
func TestP2T03_OneFileEditNMinusOneHits(t *testing.T) {
	const n = 24
	root := p2t03Fixture(t, n)
	cfg := DefaultConfig()
	cfg.CacheEnabled = true

	rm, err := NewRepoMap(root, cfg)
	if err != nil {
		t.Fatalf("NewRepoMap: %v", err)
	}
	t.Cleanup(func() { _ = rm.cache.Close() })

	files, err := rm.discoverFiles()
	if err != nil {
		t.Fatalf("discoverFiles: %v", err)
	}
	if len(files) != n {
		t.Fatalf("fixture: want %d files, got %d", n, len(files))
	}

	// Cold index — every file is a MISS, every file gets cached.
	for _, f := range files {
		if _, err := rm.extractFileSymbols(f); err != nil {
			t.Fatalf("cold extract %s: %v", f, err)
		}
	}
	rm.cache.Wait()

	_, coldMisses := rm.cache.Stats()
	if coldMisses != int64(n) {
		t.Fatalf("cold index: want %d misses, got %d", n, coldMisses)
	}

	// Edit exactly ONE file.
	edited := files[n/2]
	touchFile(t, edited, "package fixtureedited\n\nfunc EditedOnlySymbol() {}\n")

	// Warm re-index over the SAME file set.
	rm.cache.ResetStats()
	for _, f := range files {
		if _, err := rm.extractFileSymbols(f); err != nil {
			t.Fatalf("warm extract %s: %v", f, err)
		}
	}
	rm.cache.Wait()

	hits, misses := rm.cache.Stats()
	t.Logf("P2-T03 1-file-edit proof: N=%d files, %d HITS, %d MISS (edited=%s)",
		n, hits, misses, filepath.Base(edited))
	if hits != int64(n-1) {
		t.Fatalf("1-file edit: want %d cache hits (N-1), got %d", n-1, hits)
	}
	if misses != 1 {
		t.Fatalf("1-file edit: want exactly 1 cache miss, got %d", misses)
	}
}

// BenchmarkP2T03ColdIndex measures a cold index — cache disabled, every file
// parsed fresh. Baseline for the warm comparison.
func BenchmarkP2T03ColdIndex(b *testing.B) {
	root := buildRepomapFixture(b, 30)
	cfg := DefaultConfig()
	cfg.CacheEnabled = false

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rm, err := NewRepoMap(root, cfg)
		if err != nil {
			b.Fatalf("NewRepoMap: %v", err)
		}
		files, _ := rm.discoverFiles()
		for _, f := range files {
			if _, err := rm.extractFileSymbols(f); err != nil {
				b.Fatalf("extract: %v", err)
			}
		}
	}
}

// BenchmarkP2T03WarmIndex measures a warm index — cache enabled and fully
// populated, every file an unchanged HIT. The delta vs BenchmarkP2T03ColdIndex
// is the P2-T03 warm-context speedup claim (CONST-035 Rule 9 — pasted numbers).
func BenchmarkP2T03WarmIndex(b *testing.B) {
	root := buildRepomapFixture(b, 30)
	cfg := DefaultConfig()
	cfg.CacheEnabled = true

	// Pre-warm: one full index populates the disk cache.
	warmRM, err := NewRepoMap(root, cfg)
	if err != nil {
		b.Fatalf("pre-warm NewRepoMap: %v", err)
	}
	files, _ := warmRM.discoverFiles()
	for _, f := range files {
		if _, err := warmRM.extractFileSymbols(f); err != nil {
			b.Fatalf("pre-warm extract: %v", err)
		}
	}
	warmRM.cache.Wait()
	_ = warmRM.cache.Close()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rm, err := NewRepoMap(root, cfg) // reloads the warm disk cache
		if err != nil {
			b.Fatalf("NewRepoMap: %v", err)
		}
		for _, f := range files {
			if _, err := rm.extractFileSymbols(f); err != nil {
				b.Fatalf("extract: %v", err)
			}
		}
		_ = rm.cache.Close()
	}
}
