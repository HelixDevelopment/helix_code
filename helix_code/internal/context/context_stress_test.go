package context

import (
	stdctx "context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"dev.helix.code/internal/config"
	"dev.helix.code/tests/stresschaos"
)

// §11.4.85 stress coverage for the context package.
//
// The units under stress are the REAL components — no fakes for the concurrency
// surface:
//   - ContextManager: RWMutex-guarded items/sessions/projects maps + the real
//     Store/Retrieve/Search/Delete machinery, plus its Start/Stop background
//     cleanup goroutine lifecycle (config is a real minimal *config.ContextConfig).
//   - SessionContext: RWMutex-guarded per-session items map (Store/Retrieve/
//     Delete/Size).
//   - Builder: RWMutex-guarded message slice + metadata map (AddMessage/
//     SetMetadata/Build/ToText/Clone).
//
// Sustained Store/Retrieve/Build load (N>=100) + N>=10 concurrent producers,
// capturing latency + concurrency evidence under -race.

// stressConfig returns a real minimal *config.ContextConfig so the manager runs
// against a genuine (non-nil) configuration rather than a fake.
func stressConfig() *config.ContextConfig {
	return &config.ContextConfig{
		Enabled:         true,
		MaxSize:         1 << 20,
		RetentionPeriod: time.Hour,
		Compression:     false,
	}
}

// stressManager builds a real ContextManager and starts its background cleanup
// goroutine, returning a cleanup that Stops it so no goroutine leaks.
func stressManager(t *testing.T) *ContextManager {
	t.Helper()
	cm := NewContextManager(stressConfig())
	if err := cm.Start(stdctx.Background()); err != nil {
		t.Fatalf("start context manager: %v", err)
	}
	t.Cleanup(cm.Stop)
	return cm
}

// TestContextManager_Stress_SustainedStoreRetrieve drives the real
// Store -> Retrieve -> Search -> Delete lifecycle under sustained load (N>=100),
// recording per-call latency. Every iteration stores a real global-scoped item,
// reads it back, searches for it, and deletes it, asserting non-zero processed
// work so the run proves real throughput.
func TestContextManager_Stress_SustainedStoreRetrieve(t *testing.T) {
	cm := stressManager(t)
	ctx := stdctx.Background()

	var processed int64
	stresschaos.RunSustainedLoad(t, "context_manager_sustained_store_retrieve",
		stresschaos.SustainedConfig{N: 1500, MaxErrorRate: 0.0},
		func(i int) error {
			id := fmt.Sprintf("item-%d", i)
			item := &ContextItem{
				ID:    id,
				Type:  ContextTypeGlobal,
				Key:   fmt.Sprintf("key-%d", i),
				Value: fmt.Sprintf("value-%d", i),
			}
			if err := cm.Store(ctx, item); err != nil {
				return fmt.Errorf("store: %w", err)
			}
			got, err := cm.Retrieve(ctx, id)
			if err != nil {
				return fmt.Errorf("retrieve: %w", err)
			}
			if got.ID != id {
				return fmt.Errorf("retrieve returned wrong item: want %s got %s", id, got.ID)
			}
			if _, err := cm.Search(ctx, got.Key, ContextTypeGlobal); err != nil {
				return fmt.Errorf("search: %w", err)
			}
			if err := cm.Delete(ctx, id); err != nil {
				return fmt.Errorf("delete: %w", err)
			}
			atomic.AddInt64(&processed, 1)
			return nil
		})

	if atomic.LoadInt64(&processed) == 0 {
		t.Fatal("context manager processed zero items under sustained load — not real work")
	}
	t.Logf("context_manager sustained: %d items stored+read+searched+deleted", atomic.LoadInt64(&processed))
}

// TestContextManager_Stress_ConcurrentStoreRetrieve hammers Store + Retrieve +
// Search + GetStatistics from N>=10 concurrent goroutines, asserting no deadlock,
// no goroutine leak, and no data race (run under -race) on the manager's shared
// items map. Each goroutine stores its own session-scoped items (which also
// mutate the shared sessions map via getOrCreateSession) and reads them back, so
// the RWMutex is exercised under genuine read/write contention.
func TestContextManager_Stress_ConcurrentStoreRetrieve(t *testing.T) {
	cm := stressManager(t)
	ctx := stdctx.Background()

	var stored int64
	stresschaos.RunConcurrent(t, "context_manager_concurrent_store_retrieve",
		stresschaos.ConcurrencyConfig{Parallelism: 16, IterationsPerGoroutine: 120, Timeout: 25 * time.Second},
		func(g, it int) error {
			id := fmt.Sprintf("g%d-it%d", g, it)
			sessionID := fmt.Sprintf("session-%d", g)
			item := &ContextItem{
				ID:   id,
				Type: ContextTypeSession,
				Key:  fmt.Sprintf("k-%d", it),
				Value: map[string]interface{}{"g": g, "it": it},
				Metadata: map[string]interface{}{"session_id": sessionID},
			}
			if err := cm.Store(ctx, item); err != nil {
				return fmt.Errorf("store: %w", err)
			}
			atomic.AddInt64(&stored, 1)
			if _, err := cm.Retrieve(ctx, id); err != nil {
				return fmt.Errorf("retrieve: %w", err)
			}
			// Concurrent reads of shared aggregate state widen the RLock surface.
			_, _ = cm.Search(ctx, item.Key, ContextTypeSession)
			_ = cm.GetStatistics()
			// A definitely-missing retrieve exercises the not-found error path.
			_, _ = cm.Retrieve(ctx, "definitely-missing")
			return nil
		})

	if atomic.LoadInt64(&stored) == 0 {
		t.Fatal("context manager stored zero items under concurrent load")
	}
	t.Logf("context_manager concurrent: %d items stored+read", atomic.LoadInt64(&stored))
}

// TestSessionContext_Stress_ConcurrentMutation hammers the REAL SessionContext's
// Store/Retrieve/Delete/Size from N>=10 goroutines on a single shared session,
// exercising its RWMutex under genuine read/write/delete contention. No deadlock,
// no leak, no race.
func TestSessionContext_Stress_ConcurrentMutation(t *testing.T) {
	sc := NewSessionContext("shared-session")

	var ops int64
	stresschaos.RunConcurrent(t, "session_context_concurrent_mutation",
		stresschaos.ConcurrencyConfig{Parallelism: 14, IterationsPerGoroutine: 150, Timeout: 25 * time.Second},
		func(g, it int) error {
			id := fmt.Sprintf("g%d-%d", g, it)
			sc.Store(&ContextItem{ID: id, Type: ContextTypeSession, Key: id, Value: it})
			if _, ok := sc.Retrieve(id); !ok {
				return fmt.Errorf("retrieve missed just-stored item %s", id)
			}
			_ = sc.Size()
			sc.Delete(id)
			atomic.AddInt64(&ops, 1)
			return nil
		})

	if atomic.LoadInt64(&ops) == 0 {
		t.Fatal("session context performed zero ops under concurrent load")
	}
	// After delete-each-stored, the session must be empty — proof no leak.
	if sz := sc.Size(); sz != 0 {
		t.Fatalf("session context size %d after balanced store/delete, want 0", sz)
	}
	t.Logf("session_context concurrent: %d store/retrieve/delete cycles, final size 0", atomic.LoadInt64(&ops))
}

// TestBuilder_Stress_SustainedAppendBuild drives the REAL Builder under sustained
// AddUserMessage/AddAssistantMessage/SetMetadata/Build/ToText load (N>=100),
// asserting the built conversation grows monotonically so the run proves real
// accumulation.
func TestBuilder_Stress_SustainedAppendBuild(t *testing.T) {
	b := NewBuilder()
	b.SetSystemRole("stress system role")

	var appended int64
	stresschaos.RunSustainedLoad(t, "builder_sustained_append_build",
		stresschaos.SustainedConfig{N: 800, MaxErrorRate: 0.0},
		func(i int) error {
			b.AddUserMessage(fmt.Sprintf("user %d", i))
			b.AddAssistantMessage(fmt.Sprintf("assistant %d", i))
			b.SetMetadata(fmt.Sprintf("k-%d", i), fmt.Sprintf("v-%d", i))
			atomic.AddInt64(&appended, 2)

			conv := b.Build()
			// system + (i+1)*2 messages must be present.
			wantMsgs := int(atomic.LoadInt64(&appended)) + 1 // +1 system role
			if got := len(conv.GetMessages()); got != wantMsgs {
				return fmt.Errorf("built conversation has %d messages, want %d", got, wantMsgs)
			}
			if txt := b.ToText(); txt == "" {
				return fmt.Errorf("ToText returned empty after %d appends", i)
			}
			return nil
		})

	if atomic.LoadInt64(&appended) == 0 {
		t.Fatal("builder appended zero messages under sustained load — not real work")
	}
	t.Logf("builder sustained: %d messages appended+built", atomic.LoadInt64(&appended))
}

// TestBuilder_Stress_ConcurrentAppendCloneBuild hammers the REAL Builder from
// N>=10 goroutines doing AddMessage/SetMetadata/Clone/Build/MessageCount
// concurrently, exercising its RWMutex. Clone and Build both take RLock while
// writers take Lock, so this surfaces any lock-ordering or torn-slice race under
// -race.
func TestBuilder_Stress_ConcurrentAppendCloneBuild(t *testing.T) {
	b := NewBuilder()

	var appended int64
	stresschaos.RunConcurrent(t, "builder_concurrent_append_clone_build",
		stresschaos.ConcurrencyConfig{Parallelism: 12, IterationsPerGoroutine: 100, Timeout: 25 * time.Second},
		func(g, it int) error {
			b.AddUserMessage(fmt.Sprintf("g%d-it%d", g, it))
			atomic.AddInt64(&appended, 1)
			b.SetMetadata(fmt.Sprintf("g%d", g), fmt.Sprintf("it%d", it))
			// Clone takes RLock + reads the slice; concurrent with writers above.
			clone := b.Clone()
			_ = clone.MessageCount()
			_ = b.Build()
			_ = b.MessageCount()
			return nil
		})

	if atomic.LoadInt64(&appended) == 0 {
		t.Fatal("builder appended zero messages under concurrent load")
	}
	// Every concurrent AddUserMessage must be present — no lost appends.
	if got := b.MessageCount(); got != int(atomic.LoadInt64(&appended)) {
		t.Fatalf("builder has %d messages after concurrent appends, want %d (lost/duplicated)", got, atomic.LoadInt64(&appended))
	}
	t.Logf("builder concurrent: %d messages appended, count consistent", atomic.LoadInt64(&appended))
}

// TestContextManager_Stress_Boundary exercises empty/max/off-by-one boundary
// conditions against the REAL ContextManager: empty manager retrieve, empty-key
// search, a large (max-ish) item value, and rapid store/delete of the same ID.
func TestContextManager_Stress_Boundary(t *testing.T) {
	cm := stressManager(t)
	ctx := stdctx.Background()

	// Empty: retrieving from a fresh manager must error, not panic.
	if _, err := cm.Retrieve(ctx, "nope"); err == nil {
		t.Fatal("retrieve on empty manager should error")
	}
	// Empty key Search must succeed (matches all of the type, here none).
	if res, err := cm.Search(ctx, "", ContextTypeGlobal); err != nil {
		t.Fatalf("empty-key search errored: %v", err)
	} else if len(res) != 0 {
		t.Fatalf("empty manager search returned %d items, want 0", len(res))
	}

	// Max-ish: a large value payload must store + retrieve intact.
	huge := make([]byte, 1<<20) // 1 MiB
	for i := range huge {
		huge[i] = byte(i % 251)
	}
	big := &ContextItem{ID: "big", Type: ContextTypeGlobal, Key: "big", Value: string(huge)}
	if err := cm.Store(ctx, big); err != nil {
		t.Fatalf("store huge item: %v", err)
	}
	got, err := cm.Retrieve(ctx, "big")
	if err != nil {
		t.Fatalf("retrieve huge item: %v", err)
	}
	if s, ok := got.Value.(string); !ok || len(s) != len(huge) {
		t.Fatalf("huge item value corrupted: ok=%v len=%d want %d", ok, len(s), len(huge))
	}

	// Off-by-one: store then delete same ID; second delete must error (already gone).
	if err := cm.Delete(ctx, "big"); err != nil {
		t.Fatalf("first delete of big: %v", err)
	}
	if err := cm.Delete(ctx, "big"); err == nil {
		t.Fatal("second delete of already-deleted item should error")
	}
	t.Logf("context_manager boundary: empty/max(1MiB)/off-by-one all handled cleanly")
}
