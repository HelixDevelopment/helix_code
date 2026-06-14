package memory

import (
	"context"
	"path/filepath"
	"testing"
)

// TestHydrate_NoDuplicateOnRestart is a regression guard for the
// hydration-re-persists-everything defect: each restart's HydrateFromProvider
// MUST NOT write the recalled rows back to the durable store. Three restarts of
// the SAME store must keep the row count stable at the number of genuinely
// distinct facts written.
func TestHydrate_NoDuplicateOnRestart(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "nodup.db")

	// Session 1: write exactly ONE fact through the Manager.
	p1, err := NewHelixMemoryProvider(dbPath)
	if err != nil {
		t.Fatalf("provider 1: %v", err)
	}
	m1 := NewManagerWithProvider(p1)
	conv, _ := m1.CreateConversation("s1")
	_ = m1.SetActive(conv.ID)
	_ = m1.AddMessageToActive(NewUserMessage("only-fact ZephyrQuartz-4471"))
	c1, _ := p1.Count(ctx)
	if c1 != 1 {
		t.Fatalf("after first write expected 1 row, got %d", c1)
	}
	_ = p1.Close()

	// Restart 3 times — each hydrate must NOT grow the corpus.
	for i := 0; i < 3; i++ {
		p, err := NewHelixMemoryProvider(dbPath)
		if err != nil {
			t.Fatalf("provider restart %d: %v", i, err)
		}
		m := NewManagerWithProvider(p)
		if err := m.HydrateFromProvider(ctx); err != nil {
			t.Fatalf("hydrate restart %d: %v", i, err)
		}
		cnt, _ := p.Count(ctx)
		if cnt != 1 {
			_ = p.Close()
			t.Fatalf("restart %d: hydration re-persisted rows — expected 1, got %d (unbounded growth bug)", i, cnt)
		}
		_ = p.Close()
	}
	t.Log("NO-DUPLICATE OK: 3 restarts kept the durable corpus at 1 row")
}
