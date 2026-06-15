package memory

// §11.4.115 RED-on-broken-artifact regression guard for DEFECT M2:
// data race in the On* callback-registration methods.
//
// Root cause (pre-fix): OnCreate/OnMessage/OnClear/OnDelete append to
// m.onCreate/m.onMessage/m.onClear/m.onDelete WITHOUT acquiring m.mu, while
// CreateConversation/AddMessage/ClearConversation/DeleteConversation READ those
// same slices UNDER m.mu (e.g. AddMessage ranges m.onMessage). A concurrent
// register-during-operation is therefore an unsynchronised read/write of the
// same slice header — a data race the Go race detector flags.
//
// This test is designed to be run with `-race`. On the BROKEN artifact it
// trips the race detector (test fails under -race); on the FIXED artifact (the
// On* methods take m.mu while appending) it passes cleanly under -race. There
// is no RED_MODE polarity here because the oracle IS the race detector: the
// same source is the bug-catcher on the broken build and the regression guard
// on the fixed build (run `go test -race`).

import (
	"sync"
	"testing"
)

// TestManager_CallbackRegistration_Race_M2 concurrently registers callbacks
// (On*) while operations that read those callback slices run, to expose the
// unsynchronised slice access. Run with -race.
func TestManager_CallbackRegistration_Race_M2(t *testing.T) {
	m := NewManager()

	// Seed a conversation so AddMessage/ClearConversation have a target.
	conv, err := m.CreateConversation("seed")
	if err != nil {
		t.Fatalf("seed CreateConversation: %v", err)
	}

	const workers = 16
	var wg sync.WaitGroup

	// Half the goroutines register callbacks (write side, unlocked pre-fix).
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.OnCreate(func(*Conversation) {})
			m.OnMessage(func(*Conversation, *Message) {})
			m.OnClear(func(*Conversation) {})
			m.OnDelete(func(*Conversation) {})
		}()
	}

	// The other half drive operations that RANGE those slices under m.mu
	// (read side) — the concurrent read vs unlocked write is the race.
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if c, e := m.CreateConversation("c"); e == nil {
				_ = m.AddMessage(c.ID, &Message{Role: "user", Content: "hi"})
				_ = m.ClearConversation(c.ID)
			}
			_ = m.AddMessage(conv.ID, &Message{Role: "user", Content: "x"})
		}()
	}

	wg.Wait()
}
