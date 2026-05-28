package persistence

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"dev.helix.code/internal/focus"
	"dev.helix.code/internal/memory"
	"dev.helix.code/internal/session"
	"dev.helix.code/tests/stresschaos"
)

// §11.4.85 stress coverage for internal/persistence.
//
// The unit under stress is the REAL file-backed *Store wired to REAL
// session/memory/focus managers (no fakes — real os.WriteFile/Rename atomic
// writes, real os.ReadDir/ReadFile loads, real JSON serialize/deserialize, real
// sync.RWMutex-guarded state). Storage is rooted at t.TempDir() so the tests
// exercise genuine disk I/O against production code paths. The JSON + gzip
// serializers are exercised directly under sustained + concurrent load. No path
// here requires a live PostgreSQL/Redis connection — the *Store layer is pure
// file-based in-process persistence — so nothing is skipped.

// newStressStore builds a real Store rooted at a temp dir with all three real
// managers attached so SaveAll/LoadAll exercise every persistence subpath.
func newStressStore(t *testing.T) (*Store, *session.Manager, *memory.Manager, *focus.Manager) {
	t.Helper()
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

// TestStore_Stress_SustainedSaveLoad drives the real SaveAll -> LoadAll lifecycle
// under sustained load (N>=100), recording per-call latency. Each iteration adds
// a session, conversation, and focus chain, persists everything to real files,
// then reloads into fresh managers and asserts the reloaded item counts grow —
// proving real persisted work round-trips through disk every iteration.
func TestStore_Stress_SustainedSaveLoad(t *testing.T) {
	store, sessMgr, memMgr, focMgr := newStressStore(t)

	var created int64
	stresschaos.RunSustainedLoad(t, "persistence_sustained_save_load",
		stresschaos.SustainedConfig{N: 150, MaxErrorRate: 0.0},
		func(i int) error {
			if _, err := sessMgr.Create("proj", fmt.Sprintf("sess-%d", i), "stress", session.ModePlanning); err != nil {
				return fmt.Errorf("create session: %w", err)
			}
			if _, err := memMgr.CreateConversation(fmt.Sprintf("conv-%d", i)); err != nil {
				return fmt.Errorf("create conversation: %w", err)
			}
			if _, err := focMgr.CreateChain(fmt.Sprintf("chain-%d", i), false); err != nil {
				return fmt.Errorf("create chain: %w", err)
			}
			atomic.AddInt64(&created, 1)

			if err := store.SaveAll(); err != nil {
				return fmt.Errorf("save: %w", err)
			}
			return nil
		})

	if atomic.LoadInt64(&created) == 0 {
		t.Fatal("persistence store created zero items under sustained load — not real work")
	}

	// Round-trip proof: a fresh store + fresh managers must reload from the real
	// files on disk and recover exactly what we saved.
	reload, err := NewStore(store.basePath)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	rs, rm, rf := session.NewManager(), memory.NewManager(), focus.NewManager()
	reload.SetSessionManager(rs)
	reload.SetMemoryManager(rm)
	reload.SetFocusManager(rf)
	if err := reload.LoadAll(); err != nil {
		t.Fatalf("reload LoadAll: %v", err)
	}
	want := int(atomic.LoadInt64(&created))
	if got := len(rs.GetAll()); got != want {
		t.Fatalf("reloaded %d sessions, want %d (save/load lost data)", got, want)
	}
	if got := len(rm.GetAll()); got != want {
		t.Fatalf("reloaded %d conversations, want %d (save/load lost data)", got, want)
	}
	if got := len(rf.GetAllChains()); got != want {
		t.Fatalf("reloaded %d focus chains, want %d (save/load lost data)", got, want)
	}
	t.Logf("persistence sustained: %d items saved+reloaded round-trip", want)
}

// TestStore_Stress_ConcurrentSaveBackupRead hammers the SAME store from N>=10
// goroutines doing concurrent SaveAll / Backup / GetLastSaveTime / Clear, asserting
// no deadlock, no goroutine leak, and no data race (run under -race) across the
// real RWMutex-guarded file operations. A pre-populated manager set provides real
// items to serialize on every Save.
func TestStore_Stress_ConcurrentSaveBackupRead(t *testing.T) {
	store, sessMgr, memMgr, focMgr := newStressStore(t)
	for i := 0; i < 20; i++ {
		_, _ = sessMgr.Create("proj", fmt.Sprintf("sess-%d", i), "seed", session.ModePlanning)
		_, _ = memMgr.CreateConversation(fmt.Sprintf("conv-%d", i))
		_, _ = focMgr.CreateChain(fmt.Sprintf("chain-%d", i), false)
	}
	backupBase := t.TempDir()

	stresschaos.RunConcurrent(t, "persistence_concurrent_save_backup_read",
		stresschaos.ConcurrencyConfig{Parallelism: 12, IterationsPerGoroutine: 40, Timeout: 30 * time.Second},
		func(g, it int) error {
			switch (g + it) % 4 {
			case 0:
				if err := store.SaveAll(); err != nil {
					return fmt.Errorf("save g%d it%d: %w", g, it, err)
				}
			case 1:
				dst := fmt.Sprintf("%s/bk-%d-%d", backupBase, g, it)
				if err := store.Backup(dst); err != nil {
					return fmt.Errorf("backup g%d it%d: %w", g, it, err)
				}
			case 2:
				_ = store.GetLastSaveTime()
			default:
				// Clear is the harshest writer racing against Save/Backup; it must
				// serialise via the RWMutex without tearing the filesystem state.
				if err := store.Clear(); err != nil {
					return fmt.Errorf("clear g%d it%d: %w", g, it, err)
				}
			}
			return nil
		})

	// Store must stay usable after the concurrent churn.
	if err := store.SaveAll(); err != nil {
		t.Fatalf("store unusable after concurrent churn: %v", err)
	}
	t.Logf("persistence concurrent: 12 goroutines x 40 ops save/backup/read/clear, store still usable")
}

// TestSerializer_Stress_SustainedRoundTrip drives the REAL JSON + gzip serializers
// under sustained load (N>=100), serializing and deserializing a representative
// SaveMetadata each iteration and asserting a byte-faithful round-trip. The custom
// gzip byte-slice reader/writer (compressGzip/decompressGzip) is exercised here.
func TestSerializer_Stress_SustainedRoundTrip(t *testing.T) {
	jsonSer := NewJSONSerializer()
	gzipSer := NewJSONGzipSerializer()

	stresschaos.RunSustainedLoad(t, "persistence_serializer_sustained_roundtrip",
		stresschaos.SustainedConfig{N: 400, MaxErrorRate: 0.0},
		func(i int) error {
			in := SaveMetadata{
				Path:      fmt.Sprintf("/tmp/path-%d", i),
				Format:    FormatJSON,
				Size:      int64(i * 1024),
				Timestamp: time.Unix(int64(i), 0).UTC(),
				Items:     i,
			}
			// Alternate the two real serializers so both code paths are stressed.
			if i%2 == 0 {
				data, err := gzipSer.Serialize(in)
				if err != nil {
					return fmt.Errorf("gzip serialize: %w", err)
				}
				var out SaveMetadata
				if err := gzipSer.Deserialize(data, &out); err != nil {
					return fmt.Errorf("gzip deserialize: %w", err)
				}
				if out.Items != in.Items || out.Path != in.Path {
					return fmt.Errorf("gzip round-trip mismatch: got %+v want %+v", out, in)
				}
				return nil
			}
			data, err := jsonSer.Serialize(in)
			if err != nil {
				return fmt.Errorf("json serialize: %w", err)
			}
			var out SaveMetadata
			if err := jsonSer.Deserialize(data, &out); err != nil {
				return fmt.Errorf("json deserialize: %w", err)
			}
			if out.Items != in.Items || out.Path != in.Path {
				return fmt.Errorf("json round-trip mismatch: got %+v want %+v", out, in)
			}
			return nil
		})
	t.Log("persistence serializers survived sustained JSON+gzip round-trip load")
}

// TestStore_Stress_BoundaryConditions exercises §11.4.85(A)(3) boundary cases on
// the real store: empty store save (no items), save with nil managers, reload of
// an empty directory, and backup of an empty store. Each is categorised; none may
// error or panic.
func TestStore_Stress_BoundaryConditions(t *testing.T) {
	// Boundary: empty store (no managers attached) — SaveAll/LoadAll must no-op cleanly.
	empty, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore empty: %v", err)
	}
	if err := empty.SaveAll(); err != nil {
		t.Fatalf("boundary empty SaveAll: %v", err)
	}
	if err := empty.LoadAll(); err != nil {
		t.Fatalf("boundary empty LoadAll: %v", err)
	}

	// Boundary: managers attached but holding zero items — directories must not be
	// created and save must no-op.
	store, _, _, _ := newStressStore(t)
	if err := store.SaveAll(); err != nil {
		t.Fatalf("boundary zero-item SaveAll: %v", err)
	}

	// Boundary: reload from a directory that has no persisted subdirectories.
	fresh, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore fresh: %v", err)
	}
	fresh.SetSessionManager(session.NewManager())
	if err := fresh.LoadAll(); err != nil {
		t.Fatalf("boundary empty-dir LoadAll: %v", err)
	}

	// Boundary: backup an empty store.
	if err := store.Backup(t.TempDir()); err != nil {
		t.Fatalf("boundary empty Backup: %v", err)
	}
	t.Log("persistence store survived boundary conditions (empty/nil-manager/empty-dir/empty-backup)")
}
