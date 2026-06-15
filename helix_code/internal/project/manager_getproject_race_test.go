// manager_getproject_race_test.go — §11.4.135 STANDING regression guard for the
// shared-pointer data race in Manager.GetProject / ListProjects /
// GetActiveProject (HXC §11.4.85, 2026-06-15).
//
// Root cause (FACT, reproduced under `go test -race`): the read methods returned
// the LIVE map-stored *Project pointer to the caller while the write methods
// (SetActiveProject mutating project.Active at manager.go:153 and
// project.UpdatedAt at :154, UpdateProject, UpdateProjectMetadata) mutate that
// SAME struct's fields under the write Lock. A caller reading the returned
// pointer's fields outside the lock races those writes:
//
//	WARNING: DATA RACE
//	Write at 0x… by goroutine N:  (*Manager).SetActiveProject() manager.go:153
//	Previous read at 0x… by goroutine M:  caller reading gp.Active
//
// Fix: GetProject / ListProjects / GetActiveProject return a DEEP-COPY snapshot
// (copyProject) so the caller never shares mutable state with the internal
// store. The store keeps its live pointers for active-tracking; only the value
// handed out is decoupled.
//
// Polarity switch per §11.4.115 — RED_MODE (default "0") flips this single
// source between two roles:
//
//	RED_MODE=1 — reproduce-and-assert-defect on a faithful PRE-FIX stand-in
//	             (an inline aliasing "manager" that returns the live stored
//	             pointer). Assert the returned pointer IS the stored pointer
//	             (aliasing) AND that a write to the stored struct is observable
//	             through the caller's handle — exactly the shared-mutable-state
//	             that the race detector flagged. PASSES on the pre-fix shape.
//	RED_MODE=0 — standing GREEN regression guard: drive the REAL fixed code and
//	             assert (a) the returned value is an INDEPENDENT snapshot (not
//	             the stored pointer), (b) a subsequent internal mutation does NOT
//	             leak into the already-returned snapshot, and (c) a concurrent
//	             read/write workload runs clean (this whole file is run under
//	             `go test -race`, so a regression to pointer-return re-triggers
//	             the detector and FAILs the suite).
package project

import (
	"context"
	"os"
	"sync"
	"testing"
)

// prefixAliasingStore is a faithful PRE-FIX stand-in: it stores *Project and
// hands the LIVE pointer back, reproducing the unsafe sharing that caused the
// race. Used only by RED_MODE.
type prefixAliasingStore struct {
	mu       sync.RWMutex
	projects map[string]*Project
}

func (s *prefixAliasingStore) get(id string) *Project {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.projects[id] // PRE-FIX: returns the live stored pointer (aliasing)
}

func (s *prefixAliasingStore) setActive(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.projects[id].Active = true // mutates the SAME struct the caller may hold
}

func TestHXC_GetProjectNoSharedPointerRace(t *testing.T) {
	ctx := context.Background()
	tempDir, err := os.MkdirTemp("", "hxc_getproject_race")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	defer os.RemoveAll(tempDir)

	if os.Getenv("RED_MODE") == "1" {
		// Build the pre-fix aliasing store with one project (Active=false).
		stored := &Project{ID: "p1", Name: "p", Path: tempDir, Active: false}
		store := &prefixAliasingStore{projects: map[string]*Project{"p1": stored}}

		got := store.get("p1")
		if got != stored {
			t.Fatalf("RED_MODE: pre-fix store expected to ALIAS the live pointer "+
				"(got=%p stored=%p) — stand-in does not reproduce the defect shape", got, stored)
		}
		// Mutate through the store; the caller's handle observes it (shared state).
		store.setActive("p1")
		if !got.Active {
			t.Fatalf("RED_MODE: expected the internal mutation to leak into the caller's " +
				"handle (shared mutable state — the race root cause); it did not")
		}
		t.Logf("RED_MODE reproduced shared-pointer aliasing: caller handle saw Active=%v "+
			"after internal setActive (this is what `go test -race` flagged)", got.Active)
		return
	}

	// GREEN guard — REAL fixed code.
	m := NewManager()
	p, err := m.CreateProject(ctx, "p", "d", tempDir, "generic")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	id := p.ID

	// (a) returned value must be an INDEPENDENT snapshot, not the stored pointer.
	got, err := m.GetProject(ctx, id)
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	m.mu.RLock()
	stored := m.projects[id]
	m.mu.RUnlock()
	if got == stored {
		t.Fatalf("REGRESSION: GetProject returned the LIVE stored pointer (%p) — "+
			"must return a deep-copy snapshot to avoid the shared-pointer race", got)
	}

	// (b) an internal mutation MUST NOT leak into the already-returned snapshot.
	if got.Active {
		t.Fatalf("snapshot unexpectedly Active before SetActiveProject")
	}
	if err := m.SetActiveProject(ctx, id); err != nil {
		t.Fatalf("SetActiveProject: %v", err)
	}
	if got.Active {
		t.Fatalf("REGRESSION: SetActiveProject mutated the previously-returned GetProject " +
			"snapshot — snapshot is not decoupled from the store")
	}

	// ListProjects + GetActiveProject must likewise snapshot.
	list, err := m.ListProjects(ctx, "")
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	for _, lp := range list {
		m.mu.RLock()
		sp := m.projects[lp.ID]
		m.mu.RUnlock()
		if lp == sp {
			t.Fatalf("REGRESSION: ListProjects returned the live stored pointer for %s", lp.ID)
		}
	}
	ap, err := m.GetActiveProject(ctx)
	if err != nil {
		t.Fatalf("GetActiveProject: %v", err)
	}
	m.mu.RLock()
	storedActive := m.activeProject
	m.mu.RUnlock()
	if ap == storedActive {
		t.Fatalf("REGRESSION: GetActiveProject returned the live activeProject pointer")
	}

	// UpdateProject also hands back a *Project and MUST snapshot (same race class).
	up, err := m.UpdateProject(ctx, id, "renamed", "")
	if err != nil {
		t.Fatalf("UpdateProject: %v", err)
	}
	m.mu.RLock()
	storedAfterUpdate := m.projects[id]
	m.mu.RUnlock()
	if up == storedAfterUpdate {
		t.Fatalf("REGRESSION: UpdateProject returned the LIVE stored pointer (%p) — "+
			"must return a deep-copy snapshot to avoid the shared-pointer race", up)
	}

	// (c) concurrent read/write workload — runs clean under `go test -race`;
	// a regression to pointer-return re-triggers the detector here.
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			if gp, _ := m.GetProject(ctx, id); gp != nil {
				_ = gp.Active
				_ = gp.Name
				_ = gp.UpdatedAt
				if gp.Metadata.Environment != nil {
					gp.Metadata.Environment["caller"] = "mutates-its-own-copy"
				}
			}
		}()
		go func() {
			defer wg.Done()
			_ = m.SetActiveProject(ctx, id)
		}()
	}
	wg.Wait()
}
