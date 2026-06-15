package workspace

// HXC-WS-RACE standing regression guard (§11.4.135) with §11.4.115 RED_MODE
// polarity switch.
//
//	RED_MODE=1 → reproduce the defect on a FAITHFUL pre-fix stand-in: an
//	             unguarded WorkspaceManager-like map whose writer mutates a
//	             stored *Workspace struct after releasing the lock while a
//	             reader hands out the live pointer and reads its fields. Under
//	             `go test -race` this trips a DATA RACE (the reproduction).
//	RED_MODE=0 (default) → drive the REAL fixed WorkspaceManager and assert
//	             the race is ABSENT (no -race trip) across concurrent
//	             CreateWorkspace / ListWorkspaces / GetWorkspace / Cleanup.
//
// The defect: CreateWorkspace stored ws in m.spaces, then wrote
// ws.ContainerID / ws.Status AFTER the lock was released, and the getters
// returned the live stored pointer — so a concurrent reader dereferenced the
// struct the writer was mutating. Fix: post-Run mutations run under the write
// lock + getters return snapshots (copies).

import (
	"context"
	"os"
	"sync"
	"testing"
)

// blockingRunner blocks in Run until released, so the writer's post-Run
// field mutation overlaps a concurrent reader.
type blockingRunner struct{ release chan struct{} }

func (b *blockingRunner) Run(ctx context.Context, image, name, projectDir string) (string, error) {
	<-b.release
	return "cid-" + name, nil
}
func (b *blockingRunner) Stop(ctx context.Context, id string) error   { return nil }
func (b *blockingRunner) Remove(ctx context.Context, id string) error { return nil }
func (b *blockingRunner) List(ctx context.Context) ([]ContainerInfo, error) {
	return nil, nil
}

// ---- RED_MODE=1 faithful pre-fix stand-in -------------------------------

// unguardedManager reproduces the EXACT pre-fix pattern: stored pointer +
// post-lock-release mutation + live-pointer getter.
type unguardedManager struct {
	mu     sync.RWMutex
	spaces map[string]*Workspace
}

func (u *unguardedManager) createUnguarded(release <-chan struct{}) {
	ws := &Workspace{ID: "ws", Name: "ws", Status: StatusCreating}
	u.mu.Lock()
	u.spaces[ws.ID] = ws
	u.mu.Unlock()

	<-release // stand-in for runner.Run blocking

	// PRE-FIX BUG: mutate the stored struct AFTER releasing the lock.
	ws.ContainerID = "cid-ws"
	ws.Status = StatusRunning
}

func (u *unguardedManager) listUnguarded() []*Workspace {
	u.mu.RLock()
	defer u.mu.RUnlock()
	out := make([]*Workspace, 0, len(u.spaces))
	for _, ws := range u.spaces {
		out = append(out, ws) // PRE-FIX BUG: live stored pointer escapes.
	}
	return out
}

// TestHXCWSRace_NoMutationWhileRead is the standing guard.
func TestHXCWSRace_NoMutationWhileRead(t *testing.T) {
	if os.Getenv("RED_MODE") == "1" {
		// Reproduce the defect on the faithful pre-fix stand-in.
		u := &unguardedManager{spaces: make(map[string]*Workspace)}
		release := make(chan struct{})
		var wg sync.WaitGroup

		wg.Add(1)
		go func() {
			defer wg.Done()
			u.createUnguarded(release)
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 200000; i++ {
				for _, ws := range u.listUnguarded() {
					_ = ws.Status
					_ = ws.ContainerID
				}
			}
		}()
		close(release)
		wg.Wait()
		// Under -race this test FAILs (race detected) — that IS the
		// reproduction. Without -race it completes; the value is the
		// faithful demonstration of the unsafe pattern.
		return
	}

	// RED_MODE=0 (default): drive the REAL fixed WorkspaceManager and
	// assert NO race across concurrent create/read/cleanup.
	r := &blockingRunner{release: make(chan struct{})}
	mgr := NewWorkspaceManagerWithRunner(r, RuntimeDocker)
	var wg sync.WaitGroup

	const writers = 8
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			// Capture the CreateWorkspace return and read its fields concurrently
			// with a CleanupWorkspace of the SAME workspace. If CreateWorkspace
			// handed back the LIVE stored pointer, this read would race Cleanup's
			// ws.Status write under the lock — so a revert of the CreateWorkspace
			// snapshot is caught by `-race` here.
			ws, err := mgr.CreateWorkspace(context.Background(), "ws", "alpine", "/tmp")
			if err != nil || ws == nil {
				return
			}
			done := make(chan struct{})
			go func() {
				for i := 0; i < 1000; i++ {
					_ = ws.Status
					_ = ws.ContainerID
				}
				close(done)
			}()
			_ = mgr.CleanupWorkspace(context.Background(), ws.ID)
			<-done
		}(w)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 200000; i++ {
			for _, ws := range mgr.ListWorkspaces() {
				_ = ws.Status
				_ = ws.ContainerID
				_, _ = mgr.GetWorkspace(ws.ID)
			}
		}
	}()

	close(r.release)
	wg.Wait()
}
