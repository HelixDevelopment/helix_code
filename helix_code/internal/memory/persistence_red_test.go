package memory

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestPersistence_RED is a §11.4.115 RED-on-broken-artifact test with a single
// polarity switch (RED_MODE, default "1").
//
//   - RED_MODE=1 (default): REPRODUCE the defect on the CURRENT artifact. It
//     stores a distinctive fact in a plain memory.Manager, simulates a process
//     restart by constructing a SECOND, fresh Manager (no provider wired), and
//     asserts the fact is GONE — proving the pre-fix in-process map has NO
//     durable persistence. This run must PASS while persistence is broken and is
//     the captured evidence that the defect is genuinely present.
//
//   - RED_MODE=0 (post-fix GREEN guard): the SAME source asserts the fact
//     SURVIVES a restart when a durable provider is wired through
//     ProviderManagerOption. The bug-catcher IS the regression-guard.
//
// The distinctive fact never collides with anything else in the suite.
const redFavouriteToken = "ZephyrQuartz-4471"

func redMode() bool {
	v := os.Getenv("RED_MODE")
	// Default is "1" (reproduce-the-defect) per §11.4.115.
	return v == "" || v == "1"
}

func TestPersistence_RED(t *testing.T) {
	ctx := context.Background()

	if redMode() {
		// ---- RED_MODE=1: prove the in-process map does NOT persist ----
		m1 := NewManager()
		conv, err := m1.CreateConversation("red-session")
		if err != nil {
			t.Fatalf("create conversation: %v", err)
		}
		if err := m1.SetActive(conv.ID); err != nil {
			t.Fatalf("set active: %v", err)
		}
		if err := m1.AddMessageToActive(NewUserMessage("my favourite token is " + redFavouriteToken)); err != nil {
			t.Fatalf("add message: %v", err)
		}

		// Confirm it is readable in-process (sanity — the fact really landed).
		found := false
		for _, msg := range m1.SearchMessages(redFavouriteToken) {
			if msg.Content != "" {
				found = true
			}
		}
		if !found {
			t.Fatalf("RED setup invalid: fact not even stored in-process")
		}

		// Simulate process restart: a brand-new Manager with no shared state.
		m2 := NewManager()
		survivors := m2.SearchMessages(redFavouriteToken)
		if len(survivors) != 0 {
			t.Fatalf("UNEXPECTED: a fresh Manager recalled %q — persistence already works; "+
				"the RED test is blind and must be re-examined", redFavouriteToken)
		}
		t.Logf("RED PASS: fresh Manager recalled 0 results for %q — "+
			"in-process map has no durable persistence (defect reproduced)", redFavouriteToken)
		return
	}

	// ---- RED_MODE=0: GREEN regression-guard — the fact MUST survive restart ----
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "helix_memory_green.db")

	prov1, err := NewHelixMemoryProvider(dbPath)
	if err != nil {
		t.Fatalf("open provider (process A): %v", err)
	}
	m1 := NewManagerWithProvider(prov1)
	conv, err := m1.CreateConversation("green-session")
	if err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	_ = m1.SetActive(conv.ID)
	if err := m1.AddMessageToActive(NewUserMessage("my favourite token is " + redFavouriteToken)); err != nil {
		t.Fatalf("add message: %v", err)
	}
	if err := prov1.Close(); err != nil {
		t.Fatalf("close provider (process A): %v", err)
	}

	// "Restart": new provider over the SAME on-disk DB + fresh Manager.
	prov2, err := NewHelixMemoryProvider(dbPath)
	if err != nil {
		t.Fatalf("open provider (process B): %v", err)
	}
	defer prov2.Close()
	m2 := NewManagerWithProvider(prov2)
	if err := m2.HydrateFromProvider(ctx); err != nil {
		t.Fatalf("hydrate: %v", err)
	}

	results, err := prov2.Search(ctx, redFavouriteToken, 10)
	if err != nil {
		t.Fatalf("search after restart: %v", err)
	}
	recalled := false
	for _, r := range results {
		if s, ok := r.Data.(string); ok && contains(s, redFavouriteToken) {
			recalled = true
		}
	}
	if !recalled {
		t.Fatalf("GREEN FAIL: fact %q did NOT survive restart — persistence broken", redFavouriteToken)
	}
	t.Logf("GREEN PASS: fact %q recalled across restart from %s", redFavouriteToken, dbPath)
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (haystack == needle ||
		indexOf(haystack, needle) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
