package memory

import (
	"os"
	"sync"
	"testing"
)

// HXC-MEM-IMPORT-ALIAS — standing regression guard (§11.4.135) with §11.4.115
// RED_MODE polarity.
//
// Defect: Manager.Import stored the caller's live *Conversation pointer
// (snapshot.Conversation) directly into the manager map without cloning. The
// caller therefore retained a reference to the SAME struct the manager now owns.
// Any subsequent manager-side mutation (AddMessage, which appends to
// conv.Messages and mutates MessageCount/TokenCount under the write lock)
// concurrent with a caller-side read of the retained snapshot is a genuine data
// race the Manager RWMutex CANNOT protect — the mutex guards the map, not the
// contents of a pointer that escaped the critical section. This is the exact
// aliasing class the HXC-014 §11.4.85 snapshot-getter fix eliminated on the read
// path; Import re-introduced it on the store path.
//
// RED_MODE=1: reproduce the aliasing as a FACT against a faithful pre-fix stand-in
//             (importAliasingPreFix) — asserts the stored pointer == the caller's
//             pointer, the property that makes the race possible. PASSES on the
//             broken behaviour (proof the guard is real).
// RED_MODE=0 (DEFAULT): drive the REAL fixed Manager.Import and assert the stored
//             conversation is NOT the caller's pointer (a clone), so a later
//             caller-side mutation cannot alias manager-owned memory.

// importAliasRedMode reports reproduce-the-defect mode. DEFAULT (no env) is the
// GREEN guard per this work item's contract: only RED_MODE=1 reproduces.
func importAliasRedMode() bool { return os.Getenv("RED_MODE") == "1" }

// importAliasingPreFix is a faithful stand-in for the pre-fix Import: it stores
// the caller's pointer directly (the defective behaviour).
func importAliasingPreFix(m *Manager, snapshot *ConversationSnapshot) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.conversations[snapshot.Conversation.ID]; exists {
		return errAlreadyExists(snapshot.Conversation.ID)
	}
	m.conversations[snapshot.Conversation.ID] = snapshot.Conversation // ALIAS LEAK
	return nil
}

func errAlreadyExists(id string) error { return &alreadyExistsErr{id} }

type alreadyExistsErr struct{ id string }

func (e *alreadyExistsErr) Error() string { return "conversation already exists: " + e.id }

func TestImport_NoAliasLeak_Guard(t *testing.T) {
	m := NewManager()
	conv := NewConversation("imported")
	conv.AddMessage(NewMessage(RoleUser, "hello"))
	snap := &ConversationSnapshot{Conversation: conv}

	if importAliasRedMode() {
		// RED: prove the pre-fix behaviour aliases (PASSES on broken stand-in).
		if err := importAliasingPreFix(m, snap); err != nil {
			t.Fatalf("pre-fix import: %v", err)
		}
		stored := m.conversations[conv.ID]
		if stored != conv {
			t.Fatalf("RED_MODE expected the pre-fix import to ALIAS the caller pointer, but it did not")
		}
		return
	}

	// GREEN (default): the REAL fixed Import must NOT alias the caller pointer.
	if err := m.Import(snap); err != nil {
		t.Fatalf("Import: %v", err)
	}
	stored := m.conversations[conv.ID]
	if stored == conv {
		t.Fatalf("Import stored the caller's live pointer (alias leak); want a clone")
	}
	// The stored clone must be value-equal at import time.
	if stored.ID != conv.ID || stored.Title != conv.Title || stored.MessageCount != conv.MessageCount {
		t.Fatalf("imported clone differs from source: got %+v", stored)
	}
	// Mutating the caller's retained snapshot must NOT affect the stored copy.
	conv.AddMessage(NewMessage(RoleUser, "mutated after import"))
	if got, _ := m.GetConversation(conv.ID); got.MessageCount != 1 {
		t.Fatalf("caller mutation leaked into stored conversation: MessageCount=%d want 1", got.MessageCount)
	}
}

// TestImport_ConcurrentMutation_Race trips -race on the pre-fix aliasing: the
// manager mutates the imported conversation via AddMessage (write lock) while the
// caller reads its retained snapshot. With the fix (clone on import) the caller's
// pointer is independent, so no race. The -race trip IS the RED reproduction.
func TestImport_ConcurrentMutation_Race(t *testing.T) {
	m := NewManager()
	conv := NewConversation("imported")
	snap := &ConversationSnapshot{Conversation: conv}

	if importAliasRedMode() {
		if err := importAliasingPreFix(m, snap); err != nil {
			t.Fatalf("pre-fix import: %v", err)
		}
	} else {
		if err := m.Import(snap); err != nil {
			t.Fatalf("Import: %v", err)
		}
	}

	var wg sync.WaitGroup
	wg.Add(2)
	// Writer: manager mutates the stored conversation under the write lock.
	go func() {
		defer wg.Done()
		for i := 0; i < 500; i++ {
			_ = m.AddMessage(conv.ID, NewMessage(RoleUser, "w"))
		}
	}()
	// Reader: caller reads its retained snapshot's slice header + counters.
	go func() {
		defer wg.Done()
		for i := 0; i < 500; i++ {
			_ = len(conv.Messages)
			_ = conv.MessageCount
		}
	}()
	wg.Wait()
}
