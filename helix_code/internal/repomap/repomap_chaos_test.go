package repomap

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"dev.helix.code/tests/stresschaos"
)

// §11.4.85(B) chaos coverage for the REAL repomap.RepoMap.
//
// Chaos classes exercised against the real *RepoMap (real on-disk tree, real
// tree-sitter parser pool, real disk-backed cache):
//
//   - input-corruption: a repo tree seeded with hostile "source" files —
//     truncated/garbage code, raw binary bytes with a source extension, an
//     enormous file, a zero-byte file, invalid UTF-8 — is indexed. The parse
//     pipeline MUST degrade per-file (skip the unparseable file) and NEVER
//     crash the whole index. A panic on a malformed file is a §11.4.85(B) Fatal.
//   - process-death: a long-running RefreshCache over a large tree is cancelled
//     mid-flight (the worker pool / cache background-writer must unwind cleanly,
//     no deadlock, no leaked goroutine, no torn cache).
//   - resource-exhaustion: indexing runs under bounded memory pressure; the
//     pipeline must complete (or degrade) rather than OOM-crash.
//   - state-corruption: concurrent InvalidateFile / RefreshCache / Set churn on
//     the same cache keys mid-index — the cache map must stay self-consistent.

// corruptRepoFiles returns hostile file contents keyed by relative path. Each is
// a real on-disk file the discover/parse path will try to handle.
func corruptRepoFiles() map[string][]byte {
	huge := make([]byte, 5*1024*1024) // 5 MB of 'x' bytes with a .go extension
	for i := range huge {
		huge[i] = 'x'
	}
	binary := make([]byte, 4096)
	for i := range binary {
		binary[i] = byte(i % 256) // full byte range incl. NUL and invalid UTF-8
	}
	return map[string][]byte{
		"bad/truncated.go":  []byte("package x\nfunc Broken( {{{ unterminated"),
		"bad/garbage.py":    []byte("\x00\x01\x02 def ??? : !!! \xff\xfe not python at all"),
		"bad/binary.js":     binary,
		"bad/huge.go":       huge,
		"bad/empty.go":      {},
		"bad/invalidutf.rs": {0xff, 0xfe, 0xfd, 0xfc, 'f', 'n', ' ', 0xc0, 0x80},
		"bad/onlycomment.c": []byte("/* just a comment, no symbols */\n"),
		"bad/weird.ts":      []byte("class\nclass\nclass {{{{ ]]]] ((((  export export"),
	}
}

// buildCorruptRepo writes the hostile file set (plus a couple of valid files so
// the index still has real work to do) and returns the root.
func buildCorruptRepo(t testing.TB) string {
	t.Helper()
	root := t.TempDir()
	// A few valid files so a correct index isn't empty.
	for rel, content := range realSourceFiles() {
		dst := filepath.Join(root, "good", filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dst, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for rel, content := range corruptRepoFiles() {
		dst := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dst, content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// TestRepoMap_Chaos_CorruptSourceFiles indexes a repo tree riddled with
// malformed / binary / huge / empty / invalid-UTF-8 source files. The real
// GetOptimalContext / GetStatistics / RefreshCache pipeline must handle every
// hostile file gracefully (skip the unparseable ones) and NEVER panic or crash
// the whole index. A crash on any malformed file is a §11.4.85(B) Fatal.
func TestRepoMap_Chaos_CorruptSourceFiles(t *testing.T) {
	root := buildCorruptRepo(t)
	cfg := DefaultConfig()
	cfg.CacheEnabled = true
	cfg.MaxConcurrency = 4
	rm, err := NewRepoMap(root, cfg)
	if err != nil {
		t.Fatalf("NewRepoMap over corrupt repo: %v", err)
	}
	if rm.cache != nil {
		t.Cleanup(func() { _ = rm.cache.Close() })
	}

	// Feed each operation through the corruption-chaos helper. Each "input" is a
	// label selecting which real pipeline op to run over the hostile tree; a
	// clean return (or graceful error) is non-fatal, a panic is Fatal.
	ops := [][]byte{
		[]byte("GetStatistics"),
		[]byte("GetOptimalContext"),
		[]byte("RefreshCache"),
		[]byte("GetStatistics-again"),
		[]byte("GetOptimalContext-empty-query"),
	}

	stresschaos.ChaosCorruptInputDuring(t, "repomap_corrupt_source_files", ops,
		func(input []byte) error {
			switch string(input) {
			case "GetStatistics", "GetStatistics-again":
				stats, err := rm.GetStatistics()
				if err != nil {
					return err
				}
				// The valid "good/" subtree must still be discovered despite the
				// hostile siblings — proof the index degraded per-file, not wholesale.
				if stats.TotalFiles == 0 {
					return fmt.Errorf("index discovered zero files (valid files lost amid corrupt ones)")
				}
				return nil
			case "GetOptimalContext":
				_, err := rm.GetOptimalContext("Server", nil)
				return err
			case "GetOptimalContext-empty-query":
				_, err := rm.GetOptimalContext("", nil)
				return err
			case "RefreshCache":
				return rm.RefreshCache()
			}
			return nil
		})

	// After surviving all the corruption, the index must still serve the valid
	// files correctly — proof state was not torn by the malformed inputs.
	stats, err := rm.GetStatistics()
	if err != nil {
		t.Fatalf("post-chaos GetStatistics: %v", err)
	}
	if stats.TotalSymbols == 0 {
		t.Fatal("post-chaos index extracted zero symbols — valid files no longer parse")
	}
	t.Logf("repomap survived corrupt-source chaos: files=%d symbols=%d", stats.TotalFiles, stats.TotalSymbols)
}

// TestRepoMap_Chaos_RefreshCacheKilledMidFlight starts a real RefreshCache over
// a large tree and cancels the surrounding context mid-operation. The real
// worker pool + cache background-writer must unwind without deadlock, leak, or
// leaving the cache torn. The harness records Fatal if the op fails to unwind
// within its bounded wait. (RefreshCache has no ctx parameter, so the op honours
// cancellation by checking ctx between repeated refresh passes — modelling a
// caller that abandons the work.)
func TestRepoMap_Chaos_RefreshCacheKilledMidFlight(t *testing.T) {
	rm := newStressRepoMap(t, 8, true) // 8 * 9 = 72 real files — a multi-pass refresh

	stresschaos.ChaosKillDuring(t, "repomap_refresh_killed_midflight", 30*time.Millisecond,
		func(ctx context.Context, rec *stresschaos.ChaosRecorder) {
			passes := 0
			for {
				select {
				case <-ctx.Done():
					rec.Record(stresschaos.Degraded,
						fmt.Sprintf("abandoned refresh after %d completed passes on cancellation", passes))
					return
				default:
				}
				if err := rm.RefreshCache(); err != nil {
					rec.Record(stresschaos.Degraded, "RefreshCache surfaced error: "+err.Error())
					return
				}
				passes++
			}
		})

	// The cache + map must still be usable after the abandoned refresh.
	stats, err := rm.GetStatistics()
	if err != nil {
		t.Fatalf("post-cancellation GetStatistics: %v", err)
	}
	if stats.TotalFiles == 0 {
		t.Fatal("repomap unusable after mid-flight refresh cancellation")
	}
	t.Logf("repomap usable after killed refresh: files=%d symbols=%d", stats.TotalFiles, stats.TotalSymbols)
}

// TestRepoMap_Chaos_IndexUnderMemoryPressure runs the real index pipeline while
// the harness holds bounded memory pressure. The pipeline must complete (or
// degrade) rather than OOM-crash — a panic under pressure is Fatal.
func TestRepoMap_Chaos_IndexUnderMemoryPressure(t *testing.T) {
	rm := newStressRepoMap(t, 6, true)

	stresschaos.ChaosResourcePressureDuring(t, "repomap_index_under_memory_pressure", 64,
		func(rec *stresschaos.ChaosRecorder) {
			for i := 0; i < 5; i++ {
				stats, err := rm.GetStatistics()
				if err != nil {
					rec.Record(stresschaos.Degraded, "GetStatistics under pressure: "+err.Error())
					return
				}
				if stats.TotalFiles == 0 {
					rec.Record(stresschaos.Fatal, "index lost all files under memory pressure")
					return
				}
				if _, err := rm.GetOptimalContext("Server", nil); err != nil {
					rec.Record(stresschaos.Degraded, "GetOptimalContext under pressure: "+err.Error())
					return
				}
			}
			rec.Record(stresschaos.Recovered, "completed 5 full index passes under bounded memory pressure")
		})
}

// TestRepoMap_Chaos_ConcurrentInvalidateRefreshChurn hammers the SAME cache keys
// with concurrent InvalidateFile / RefreshCache / GetOptimalContext from many
// goroutines mid-index. The cache's RWMutex + pathIndex must serialise the map
// mutations so nothing panics or races and the cache ends self-consistent.
// Run under -race.
func TestRepoMap_Chaos_ConcurrentInvalidateRefreshChurn(t *testing.T) {
	rec := stresschaos.NewChaosRecorder(t, "repomap_invalidate_refresh_churn", "state-corruption")
	rm := newStressRepoMap(t, 4, true)

	// Collect real file paths to target with InvalidateFile.
	var files []string
	_ = filepath.Walk(rm.rootPath, func(path string, info os.FileInfo, err error) error {
		if err == nil && info != nil && !info.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	if len(files) == 0 {
		t.Fatal("no files collected from built repo")
	}

	const goroutines = 12
	const iters = 60
	var wg sync.WaitGroup
	var invalidations, refreshes, queries int64

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
					if err := rm.InvalidateFile(files[(id+it)%len(files)]); err != nil {
						rec.Record(stresschaos.Degraded, "InvalidateFile error: "+err.Error())
					}
					atomic.AddInt64(&invalidations, 1)
				case 1:
					if err := rm.RefreshCache(); err != nil {
						rec.Record(stresschaos.Degraded, "RefreshCache error: "+err.Error())
					}
					atomic.AddInt64(&refreshes, 1)
				default:
					if _, err := rm.GetOptimalContext("Worker", nil); err != nil {
						rec.Record(stresschaos.Degraded, "GetOptimalContext error: "+err.Error())
					}
					atomic.AddInt64(&queries, 1)
				}
			}
		}(w)
	}
	wg.Wait()

	rec.Record(stresschaos.Recovered, fmt.Sprintf(
		"survived churn: %d invalidations, %d refreshes, %d queries, no panic/race",
		atomic.LoadInt64(&invalidations), atomic.LoadInt64(&refreshes), atomic.LoadInt64(&queries)))

	// Cache state must remain coherent: size non-negative and the index must
	// still serve correct results.
	if rm.cache != nil && rm.cache.Size() < 0 {
		rec.Record(stresschaos.Fatal, fmt.Sprintf("cache size went negative: %d", rm.cache.Size()))
	}
	stats, err := rm.GetStatistics()
	if err != nil {
		rec.Record(stresschaos.Fatal, "post-churn GetStatistics failed: "+err.Error())
	} else if stats.TotalSymbols == 0 {
		rec.Record(stresschaos.Fatal, "post-churn index extracts zero symbols — cache/map torn")
	} else {
		rec.Record(stresschaos.Recovered, fmt.Sprintf("post-churn index coherent: files=%d symbols=%d", stats.TotalFiles, stats.TotalSymbols))
	}

	rec.AssertNoFatal()
	t.Logf("repomap churn: invalidations=%d refreshes=%d queries=%d",
		atomic.LoadInt64(&invalidations), atomic.LoadInt64(&refreshes), atomic.LoadInt64(&queries))
}
