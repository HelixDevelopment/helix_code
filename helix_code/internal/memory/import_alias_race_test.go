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

// ─────────────────────────────────────────────────────────────────────────────
// HXC-MEM-UPDATE-ALIAS / HXC-MEM-RESOLVE-ALIAS — §11.4.118 same-class store-side
// aliasing guards for the two remaining caller-supplied-pointer store paths:
// UpdateConversationWithVersion and ResolveConflict("overwrite"). Both stored the
// caller's live *Conversation directly (same class as the now-fixed Import). They
// reuse the package-local importAliasRedMode() polarity: DEFAULT (no env) runs the
// GREEN guard; RED_MODE=1 reproduces the alias/race against a faithful pre-fix
// stand-in.

// updateAliasingPreFix is a faithful stand-in for the pre-fix
// UpdateConversationWithVersion overwrite path: it stores the caller's pointer
// directly (the defective behaviour).
func updateAliasingPreFix(m *Manager, id string, updated *Conversation) {
	m.mu.Lock()
	defer m.mu.Unlock()
	current := m.conversations[id]
	updated.Version = current.Version + 1
	m.conversations[id] = updated // ALIAS LEAK
	if m.activeConv != nil && m.activeConv.ID == id {
		m.activeConv = updated
	}
}

// resolveOverwriteAliasingPreFix is a faithful stand-in for the pre-fix
// ResolveConflict("overwrite") path: it stores the caller's pointer directly.
func resolveOverwriteAliasingPreFix(m *Manager, id string, incoming *Conversation) {
	m.mu.Lock()
	defer m.mu.Unlock()
	current := m.conversations[id]
	incoming.Version = current.Version + 1
	m.conversations[id] = incoming // ALIAS LEAK
	if m.activeConv != nil && m.activeConv.ID == id {
		m.activeConv = incoming
	}
}

func TestUpdateConversationWithVersion_NoAliasLeak_Guard(t *testing.T) {
	m := NewManager()
	base, err := m.CreateConversation("base")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}
	updated := NewConversation("base")
	updated.ID = base.ID
	updated.AddMessage(NewMessage(RoleUser, "v2"))

	if importAliasRedMode() {
		// RED: prove the pre-fix behaviour aliases (PASSES on broken stand-in).
		updateAliasingPreFix(m, base.ID, updated)
		if m.conversations[base.ID] != updated {
			t.Fatalf("RED_MODE expected the pre-fix update to ALIAS the caller pointer, but it did not")
		}
		return
	}

	// GREEN (default): the REAL fixed update must NOT alias the caller pointer.
	res, err := m.UpdateConversationWithVersion(base.ID, updated, base.Version)
	if err != nil {
		t.Fatalf("UpdateConversationWithVersion: %v", err)
	}
	if !res.Resolved {
		t.Fatalf("expected resolved update, got %+v", res)
	}
	stored := m.conversations[base.ID]
	if stored == updated {
		t.Fatalf("update stored the caller's live pointer (alias leak); want a clone")
	}
	if stored.MessageCount != updated.MessageCount {
		t.Fatalf("stored clone differs from source: MessageCount=%d want %d", stored.MessageCount, updated.MessageCount)
	}
	// Mutating the caller's retained pointer must NOT affect the stored copy.
	updated.AddMessage(NewMessage(RoleUser, "mutated after update"))
	got, _ := m.GetConversation(base.ID)
	if got.MessageCount != 1 {
		t.Fatalf("caller mutation leaked into stored conversation: MessageCount=%d want 1", got.MessageCount)
	}
}

// TestUpdateConversationWithVersion_ConcurrentMutation_Race trips -race on the
// pre-fix aliasing: the manager mutates the stored conversation via AddMessage
// (write lock) while the caller reads its retained pointer. With the fix (clone on
// store) the caller's pointer is independent, so no race.
func TestUpdateConversationWithVersion_ConcurrentMutation_Race(t *testing.T) {
	m := NewManager()
	base, err := m.CreateConversation("base")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}
	updated := NewConversation("base")
	updated.ID = base.ID

	if importAliasRedMode() {
		updateAliasingPreFix(m, base.ID, updated)
	} else {
		if _, err := m.UpdateConversationWithVersion(base.ID, updated, base.Version); err != nil {
			t.Fatalf("UpdateConversationWithVersion: %v", err)
		}
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 500; i++ {
			_ = m.AddMessage(base.ID, NewMessage(RoleUser, "w"))
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 500; i++ {
			_ = len(updated.Messages)
			_ = updated.MessageCount
		}
	}()
	wg.Wait()
}

func TestResolveConflictOverwrite_NoAliasLeak_Guard(t *testing.T) {
	m := NewManager()
	base, err := m.CreateConversation("base")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}
	incoming := NewConversation("base")
	incoming.ID = base.ID
	incoming.AddMessage(NewMessage(RoleUser, "incoming"))

	if importAliasRedMode() {
		// RED: prove the pre-fix behaviour aliases (PASSES on broken stand-in).
		resolveOverwriteAliasingPreFix(m, base.ID, incoming)
		if m.conversations[base.ID] != incoming {
			t.Fatalf("RED_MODE expected the pre-fix overwrite to ALIAS the caller pointer, but it did not")
		}
		return
	}

	// GREEN (default): the REAL fixed overwrite must NOT alias the caller pointer.
	res, err := m.ResolveConflict(base.ID, incoming, "overwrite")
	if err != nil {
		t.Fatalf("ResolveConflict: %v", err)
	}
	if !res.Resolved {
		t.Fatalf("expected resolved conflict, got %+v", res)
	}
	stored := m.conversations[base.ID]
	if stored == incoming {
		t.Fatalf("overwrite stored the caller's live pointer (alias leak); want a clone")
	}
	if stored.MessageCount != incoming.MessageCount {
		t.Fatalf("stored clone differs from source: MessageCount=%d want %d", stored.MessageCount, incoming.MessageCount)
	}
	// Mutating the caller's retained pointer must NOT affect the stored copy.
	incoming.AddMessage(NewMessage(RoleUser, "mutated after overwrite"))
	got, _ := m.GetConversation(base.ID)
	if got.MessageCount != 1 {
		t.Fatalf("caller mutation leaked into stored conversation: MessageCount=%d want 1", got.MessageCount)
	}
}

// TestResolveConflictOverwrite_ConcurrentMutation_Race trips -race on the pre-fix
// aliasing: manager mutates the stored conversation while the caller reads its
// retained pointer. With the fix (clone on store) no race.
func TestResolveConflictOverwrite_ConcurrentMutation_Race(t *testing.T) {
	m := NewManager()
	base, err := m.CreateConversation("base")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}
	incoming := NewConversation("base")
	incoming.ID = base.ID

	if importAliasRedMode() {
		resolveOverwriteAliasingPreFix(m, base.ID, incoming)
	} else {
		if _, err := m.ResolveConflict(base.ID, incoming, "overwrite"); err != nil {
			t.Fatalf("ResolveConflict: %v", err)
		}
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 500; i++ {
			_ = m.AddMessage(base.ID, NewMessage(RoleUser, "w"))
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 500; i++ {
			_ = len(incoming.Messages)
			_ = incoming.MessageCount
		}
	}()
	wg.Wait()
}
