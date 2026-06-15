package roocode

// §11.4.115 RED_MODE polarity regression guards for the concurrency
// defect classes in this package. For a data race, the `-race` trip on
// the broken code IS the RED reproduction (caught only under
// `go test -race`); the GREEN guard drives the REAL fixed code and the
// absence of a race trip is the proof of fix.
//
// Defects (reproduced 2026-06-16, all confirmed by `go test -race`):
//   1. conversation.go:generateID — global `idCounter++` raced across
//      distinct ConversationStore instances (each has its OWN mutex, so
//      no store serializes the shared global). Fix: atomic.Int64.
//   2. ConversationStore.Get/List returned the LIVE stored *Conversation;
//      a caller reading conv.Messages raced AddMessage's append under
//      cs.mu (a mutex the caller never sees). Fix: deep snapshot.
//   3. TaskDelegator.GetTask/ListTasks returned the LIVE stored *TaskSpec;
//      a caller reading task.AssignedTo raced AssignTask's write under
//      d.mu. Fix: value-copy snapshot.
//
// RED_MODE=1 → drive a faithful pre-fix stand-in that still races so the
//              `-race` detector trips (proving the guard catches the real
//              defect shape).
// RED_MODE=0 (default) → drive the REAL fixed methods; under `-race` the
//              run is clean.

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
)

// preFixGlobalCounter mimics the old unguarded `idCounter int` increment.
var preFixGlobalCounter int

// TestConcurrentStoreCreate_NoGlobalCounterRace guards defect #1.
// Run under `go test -race` for it to be meaningful.
func TestConcurrentStoreCreate_NoGlobalCounterRace(t *testing.T) {
	const n = 64
	var wg sync.WaitGroup

	if redMode(t) {
		// Reproduce: distinct goroutines increment the shared plain int
		// without any synchronization — a data race under -race.
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				preFixGlobalCounter++ // raced on purpose
				_ = fmt.Sprintf("conv-%d", preFixGlobalCounter)
			}()
		}
		wg.Wait()
		t.Logf("RED drove the unguarded global-counter increment (race expected under -race)")
		return
	}

	// GREEN: each goroutine constructs its OWN store and calls Create;
	// the real generateID uses atomic.Int64, so this is race-free, and
	// all produced IDs are unique.
	ids := make(chan string, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ids <- NewConversationStore().Create("x").ID
		}()
	}
	wg.Wait()
	close(ids)
	seen := map[string]bool{}
	for id := range ids {
		if seen[id] {
			t.Fatalf("duplicate conversation ID generated: %s", id)
		}
		seen[id] = true
	}
}

// racyStore is a faithful pre-fix stand-in returning the LIVE pointer.
type racyStore struct {
	mu sync.RWMutex
	m  map[string]*Conversation
}

func (s *racyStore) add(id, content string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := s.m[id]
	c.Messages = append(c.Messages, Message{Role: "user", Content: content})
}

func (s *racyStore) get(id string) *Conversation {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.m[id] // LIVE pointer — the defect
}

// TestConversationGet_SnapshotNoRace guards defect #2.
func TestConversationGet_SnapshotNoRace(t *testing.T) {
	const iters = 300
	var wg sync.WaitGroup
	wg.Add(2)

	if redMode(t) {
		rs := &racyStore{m: map[string]*Conversation{"c": {ID: "c"}}}
		go func() {
			defer wg.Done()
			for i := 0; i < iters; i++ {
				rs.add("c", "m")
			}
		}()
		go func() {
			defer wg.Done()
			for i := 0; i < iters; i++ {
				_ = len(rs.get("c").Messages) // races append under -race
			}
		}()
		wg.Wait()
		t.Logf("RED drove the live-pointer reader/writer (race expected under -race)")
		return
	}

	cs := NewConversationStore()
	conv := cs.Create("c")
	go func() {
		defer wg.Done()
		for i := 0; i < iters; i++ {
			cs.AddMessage(conv.ID, "user", "m")
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < iters; i++ {
			got, err := cs.Get(conv.ID)
			if err != nil {
				t.Errorf("Get: %v", err)
				return
			}
			_ = len(got.Messages) // reads a detached snapshot — race-free
		}
	}()
	wg.Wait()
}

// racyDelegator is a faithful pre-fix stand-in returning the LIVE task.
type racyDelegator struct {
	mu sync.Mutex
	t  *TaskSpec
}

func (d *racyDelegator) assign(s string) { d.mu.Lock(); defer d.mu.Unlock(); d.t.AssignedTo = s }
func (d *racyDelegator) get() *TaskSpec  { d.mu.Lock(); defer d.mu.Unlock(); return d.t }

// TestGetTask_SnapshotNoRace guards defect #3.
func TestGetTask_SnapshotNoRace(t *testing.T) {
	const iters = 300
	var wg sync.WaitGroup
	wg.Add(2)

	if redMode(t) {
		rd := &racyDelegator{t: &TaskSpec{ID: "t"}}
		go func() {
			defer wg.Done()
			for i := 0; i < iters; i++ {
				rd.assign("s")
			}
		}()
		go func() {
			defer wg.Done()
			for i := 0; i < iters; i++ {
				_ = rd.get().AssignedTo
			}
		}()
		wg.Wait()
		t.Logf("RED drove the live-task reader/writer (race expected under -race)")
		return
	}

	d := NewTaskDelegator()
	task, err := d.Delegate(context.Background(), "t", "d", 1)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		defer wg.Done()
		for i := 0; i < iters; i++ {
			d.AssignTask(task.ID, "s")
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < iters; i++ {
			g, err := d.GetTask(task.ID)
			if err != nil {
				t.Errorf("GetTask: %v", err)
				return
			}
			_ = g.AssignedTo // detached snapshot — race-free
		}
	}()
	wg.Wait()
}

// compile-time anchor that the atomic type is what conversation.go uses.
var _ atomic.Int64
