package memory

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestHelixMemoryProvider_RoundTrip is the provider unit test: store, retrieve,
// search, delete against a real temp SQLite file (no mocks — CONST-050).
func TestHelixMemoryProvider_RoundTrip(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	p, err := NewHelixMemoryProvider(filepath.Join(dir, "rt.db"))
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}
	defer p.Close()

	if err := p.Store(ctx, "fav", "my favourite token is ZephyrQuartz-4471"); err != nil {
		t.Fatalf("store: %v", err)
	}
	got, err := p.Retrieve(ctx, "fav")
	if err != nil {
		t.Fatalf("retrieve: %v", err)
	}
	if s, _ := got.(string); !strings.Contains(s, "ZephyrQuartz-4471") {
		t.Fatalf("retrieve wrong value: %v", got)
	}

	res, err := p.Search(ctx, "favourite ZephyrQuartz-4471", 5)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(res) == 0 {
		t.Fatal("search returned nothing for a stored fact")
	}
	if res[0].Key != "fav" {
		t.Fatalf("expected key 'fav', got %q", res[0].Key)
	}

	if err := p.Health(ctx); err != nil {
		t.Fatalf("health: %v", err)
	}

	if err := p.Delete(ctx, "fav"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := p.Retrieve(ctx, "fav"); err == nil {
		t.Fatal("expected error after delete")
	}
}

// TestHelixMemoryProvider_ManagerWriteThrough is the integration test: a fact
// added through the Manager is durably persisted via the provider's write-through
// and recallable on a fresh Manager hydrated from the SAME on-disk DB.
func TestHelixMemoryProvider_ManagerWriteThrough(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "wt.db")

	// Session 1: write through the Manager, then close the provider.
	p1, err := NewHelixMemoryProvider(dbPath)
	if err != nil {
		t.Fatalf("provider 1: %v", err)
	}
	m1 := NewManagerWithProvider(p1)
	conv, _ := m1.CreateConversation("s1")
	_ = m1.SetActive(conv.ID)
	if err := m1.AddMessageToActive(NewUserMessage("my favourite token is ZephyrQuartz-4471")); err != nil {
		t.Fatalf("add message: %v", err)
	}
	if err := p1.Close(); err != nil {
		t.Fatalf("close p1: %v", err)
	}

	// Session 2: fresh provider + Manager over the same DB, hydrate, recall.
	p2, err := NewHelixMemoryProvider(dbPath)
	if err != nil {
		t.Fatalf("provider 2: %v", err)
	}
	defer p2.Close()
	m2 := NewManagerWithProvider(p2)
	if err := m2.HydrateFromProvider(ctx); err != nil {
		t.Fatalf("hydrate: %v", err)
	}

	recall, err := m2.RecallContext(ctx, "favourite token", 5)
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	if !strings.Contains(recall, "ZephyrQuartz-4471") {
		t.Fatalf("recall did not surface the fact across restart: %q", recall)
	}

	// Hydration also rebuilt a recalled conversation containing the message.
	found := false
	for _, msg := range m2.SearchMessages("ZephyrQuartz-4471") {
		if strings.Contains(msg.Content, "ZephyrQuartz-4471") {
			found = true
		}
	}
	if !found {
		t.Fatal("hydrated Manager did not contain the durable message")
	}
	t.Logf("WRITE-THROUGH OK: fact recalled across Manager restart from %s", dbPath)
}

// TestHelixMemoryProvider_CrossProcess is the headline anti-bluff proof: process
// A writes a distinctive fact to a real on-disk DB and EXITS; process B (a fresh
// OS process) opens the SAME DB and reads it back. This is genuine cross-process
// persistence, not an in-process map.
func TestHelixMemoryProvider_CrossProcess(t *testing.T) {
	ctx := context.Background()

	// Child mode: write the fact then exit 0.
	if os.Getenv("HELIXMEM_XPROC_WRITE") == "1" {
		p, err := NewHelixMemoryProvider(os.Getenv("HELIXMEM_XPROC_DB"))
		if err != nil {
			os.Exit(3)
		}
		if err := p.Store(context.Background(), "xproc-fav", "my favourite token is ZephyrQuartz-4471"); err != nil {
			os.Exit(4)
		}
		_ = p.Close()
		os.Exit(0)
	}

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "xproc.db")

	// --- Process A: re-exec this test binary in writer mode ---
	cmd := exec.Command(os.Args[0], "-test.run", "TestHelixMemoryProvider_CrossProcess", "-test.v")
	cmd.Env = append(os.Environ(), "HELIXMEM_XPROC_WRITE=1", "HELIXMEM_XPROC_DB="+dbPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("writer process failed: %v\n%s", err, out)
	}

	fi, err := os.Stat(dbPath)
	if err != nil || fi.Size() == 0 {
		t.Fatalf("writer process did not produce a durable DB at %s (err=%v)", dbPath, err)
	}

	// --- Process B (this process, fresh handle): read it back ---
	p, err := NewHelixMemoryProvider(dbPath)
	if err != nil {
		t.Fatalf("reader open: %v", err)
	}
	defer p.Close()
	res, err := p.Search(ctx, "ZephyrQuartz-4471", 5)
	if err != nil {
		t.Fatalf("reader search: %v", err)
	}
	recalled := false
	for _, r := range res {
		if s, _ := r.Data.(string); strings.Contains(s, "ZephyrQuartz-4471") {
			recalled = true
		}
	}
	if !recalled {
		t.Fatalf("CROSS-PROCESS FAIL: process B did NOT recall the fact process A wrote to %s", dbPath)
	}
	t.Logf("CROSS-PROCESS OK: process A wrote + exited; process B recalled %q from %s (%d bytes)",
		"ZephyrQuartz-4471", dbPath, fi.Size())
}

// TestHelixMemoryProvider_Mutation is the §1.1 paired mutation: when the
// write-through is disabled (provider detached), the fact MUST NOT survive a
// restart — proving the GREEN guard genuinely depends on the persistence path
// (it cannot pass for the wrong reason).
func TestHelixMemoryProvider_Mutation(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "mut.db")

	p1, err := NewHelixMemoryProvider(dbPath)
	if err != nil {
		t.Fatalf("provider 1: %v", err)
	}
	// MUTATION: build a Manager WITHOUT the write-through wiring (legacy mode).
	// NewManager (not NewManagerWithProvider) registers no OnMessage persist
	// callback, so AddMessage stays in-process only even though a provider
	// exists on disk.
	mBroken := NewManager()
	conv, _ := mBroken.CreateConversation("mut")
	_ = mBroken.SetActive(conv.ID)
	_ = mBroken.AddMessageToActive(NewUserMessage("my favourite token is ZephyrQuartz-4471"))
	_ = p1.Close()

	// Restart: nothing was written through, so recall MUST be empty.
	p2, err := NewHelixMemoryProvider(dbPath)
	if err != nil {
		t.Fatalf("provider 2: %v", err)
	}
	defer p2.Close()
	res, err := p2.Search(ctx, "ZephyrQuartz-4471", 5)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(res) != 0 {
		t.Fatalf("MUTATION DID NOT FAIL: recall returned %d results with write-through disabled — "+
			"the GREEN guard would pass for the wrong reason", len(res))
	}
	t.Logf("MUTATION OK: with write-through disabled, restart recalled 0 results "+
		"(the GREEN path genuinely depends on persistence)")
}
