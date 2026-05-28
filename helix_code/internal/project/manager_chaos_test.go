package project

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"dev.helix.code/tests/stresschaos"
)

// §11.4.85(B) chaos coverage for the REAL project.Manager.
//
// Chaos classes exercised against the REAL *Manager (no fakes — real RWMutex,
// real map, real os.Stat-driven detection):
//
//   - input-corruption: structurally hostile project paths and names (path
//     traversal, NUL bytes, control chars, oversized strings, binary-garbage
//     manifests, a path that is a regular FILE not a directory) are fed to
//     CreateProject. Creation MUST reject or normalise without crashing — a
//     panic on malformed input is a §11.4.85(B) Fatal.
//   - state-corruption under contention: the SAME project id is concurrently
//     Created / SetActive / Deleted / Get / GetActive from many goroutines
//     mid-flight. The manager MUST never panic or race and MUST end in a
//     self-consistent map (run under -race).
//   - goroutine-death mid-op: a goroutine driving a long mixed-op loop against
//     the manager is cancelled mid-flight; the manager MUST remain usable.

// TestManager_Chaos_CorruptCreateInput feeds the REAL CreateProject hostile path
// and name inputs. The manager must reject or normalise each without panicking.
// A handler-side os.Stat or detectProjectType crash on malformed input would be
// a §11.4.85(B) failure (caught by the helper's recover()).
func TestManager_Chaos_CorruptCreateInput(t *testing.T) {
	mgr := NewManager()
	ctx := context.Background()

	// A real on-disk dir so "valid path + hostile name" combinations exercise the
	// detection pipeline rather than failing at the os.Stat gate.
	validDir := buildProjectDir(t, "go")

	// A regular file (NOT a directory) — CreateProject os.Stat's it; os.Stat
	// succeeds, so detectProjectType then runs against a non-dir path. Must not
	// crash.
	fileNotDir := filepath.Join(t.TempDir(), "iam_a_file")
	if err := os.WriteFile(fileNotDir, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	// A dir containing a binary-garbage "go.mod" — detectProjectType only
	// os.Stat's the manifest (it does not parse it), so this must still detect
	// "go" without choking on the bytes; the point is it must not crash.
	garbageDir := t.TempDir()
	garbage := make([]byte, 4096)
	for i := range garbage {
		garbage[i] = byte(i % 256)
	}
	if err := os.WriteFile(filepath.Join(garbageDir, "go.mod"), garbage, 0o644); err != nil {
		t.Fatal(err)
	}

	hugeName := make([]byte, 1<<16)
	for i := range hugeName {
		hugeName[i] = 'A'
	}

	// Each corrupt input is a "name\x00path" descriptor; feed() splits it and
	// calls the REAL CreateProject. The []byte contract is the helper's; we encode
	// the (name,path) pair into it.
	type corrupt struct{ name, path string }
	cases := []corrupt{
		{"../../etc/passwd", validDir},                    // path-traversal in name
		{"name\x00injected", validDir},                    // NUL byte in name
		{"name\nwith\tcontrol\rchars", validDir},          // control chars in name
		{string(hugeName), validDir},                      // oversized name
		{"", validDir},                                    // empty name
		{"valid", "/nonexistent/" + "x/y/z"},             // missing path
		{"valid", fileNotDir},                             // path is a regular file, not a dir
		{"valid", garbageDir},                             // binary-garbage manifest
		{"emoji-名前-🚀", validDir},                          // multibyte / unicode name
		{"path\x00in\x00path", "\x00" + validDir},         // NUL in path
	}

	payloads := make([][]byte, len(cases))
	for i, c := range cases {
		payloads[i] = []byte(c.name + "\x1f" + c.path) // 0x1f unit separator avoids collision
	}

	stresschaos.ChaosCorruptInputDuring(t, "project_manager_corrupt_create_input", payloads,
		func(input []byte) error {
			// Split on the unit-separator we encoded with.
			var name, path string
			sep := -1
			for i, b := range input {
				if b == 0x1f {
					sep = i
					break
				}
			}
			if sep >= 0 {
				name = string(input[:sep])
				path = string(input[sep+1:])
			} else {
				path = string(input)
			}
			// Real CreateProject. An error is graceful rejection (Degraded); a
			// successful create on a valid dir is graceful acceptance (Recovered);
			// a panic is Fatal (caught by the helper). Either non-panic is fine.
			_, err := mgr.CreateProject(ctx, name, "chaos", path, "")
			return err
		})

	// The manager must still be fully usable after the corrupt-input barrage.
	good := buildProjectDir(t, "node")
	p, err := mgr.CreateProject(ctx, "post-chaos", "d", good, "")
	if err != nil {
		t.Fatalf("manager unusable after corrupt-input chaos: %v", err)
	}
	if p.Type != "node" {
		t.Fatalf("post-chaos detection broken: type=%q want node", p.Type)
	}
}

// TestManager_Chaos_ConcurrentStateChurn hammers the SAME logical project store
// with concurrent Create / SetActive / Delete / Get / GetActive / List from many
// goroutines. The real manager mutex must serialise the map + activeProject
// mutations so the manager never panics or races and ends self-consistent. Run
// under -race. This is the suite most likely to surface a lock-discipline defect
// (e.g. a read-locked path that writes shared state).
func TestManager_Chaos_ConcurrentStateChurn(t *testing.T) {
	rec := stresschaos.NewChaosRecorder(t, "project_manager_state_churn", "state-corruption")
	mgr := NewManager()
	ctx := context.Background()
	dir := buildProjectDir(t, "go")

	// A pool of pre-created ids so SetActive/Get/Delete always have real keys.
	const poolN = 30
	pool := make([]string, 0, poolN)
	for i := 0; i < poolN; i++ {
		p, err := mgr.CreateProject(ctx, fmt.Sprintf("pool-%d", i), "d", dir, "")
		if err != nil {
			t.Fatalf("pool CreateProject: %v", err)
		}
		pool = append(pool, p.ID)
	}

	const goroutines = 14
	const iters = 250
	var wg sync.WaitGroup
	var creates, deletes, setActives, getActives, gets, lists int64

	for w := 0; w < goroutines; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if p := recover(); p != nil {
					rec.Record(stresschaos.Fatal, fmt.Sprintf("goroutine %d panicked: %v", id, p))
				}
			}()
			for it := 0; it < iters; it++ {
				poolID := pool[(id+it)%len(pool)]
				switch (id + it) % 6 {
				case 0:
					if _, err := mgr.CreateProject(ctx, fmt.Sprintf("churn-%d-%d", id, it), "d", dir, ""); err == nil {
						atomic.AddInt64(&creates, 1)
					}
				case 1:
					// Set-active churn races against concurrent deletes of the same
					// pool ids — the activeProject mutation must be serialised.
					_ = mgr.SetActiveProject(ctx, poolID)
					atomic.AddInt64(&setActives, 1)
				case 2:
					// GetActiveProject takes an RLock but also writes m.activeProject
					// on the lazy-scan path — the highest-risk lock-discipline line.
					_, _ = mgr.GetActiveProject(ctx)
					atomic.AddInt64(&getActives, 1)
				case 3:
					_, _ = mgr.GetProject(ctx, poolID)
					atomic.AddInt64(&gets, 1)
				case 4:
					_, _ = mgr.ListProjects(ctx, "")
					atomic.AddInt64(&lists, 1)
				default:
					// Delete a pool id — concurrently with another goroutine setting
					// it active. The delete-clears-active path must hold the write
					// lock so it never tears the activeProject pointer.
					_ = mgr.DeleteProject(ctx, poolID)
					atomic.AddInt64(&deletes, 1)
				}
			}
		}(w)
	}
	wg.Wait()

	rec.Record(stresschaos.Recovered, fmt.Sprintf(
		"survived state churn: creates=%d deletes=%d setActive=%d getActive=%d gets=%d lists=%d, no panic/race",
		atomic.LoadInt64(&creates), atomic.LoadInt64(&deletes), atomic.LoadInt64(&setActives),
		atomic.LoadInt64(&getActives), atomic.LoadInt64(&gets), atomic.LoadInt64(&lists)))

	// Final state must be coherent: list works, count is non-negative, and a
	// fresh create + set-active + get-active round-trips correctly.
	finalList, err := mgr.ListProjects(ctx, "")
	if err != nil {
		rec.Record(stresschaos.Fatal, "final ListProjects errored: "+err.Error())
	}
	freshDir := buildProjectDir(t, "rust")
	fresh, err := mgr.CreateProject(ctx, "final-fresh", "d", freshDir, "")
	if err != nil {
		rec.Record(stresschaos.Fatal, "manager could not create after churn: "+err.Error())
	} else {
		if err := mgr.SetActiveProject(ctx, fresh.ID); err != nil {
			rec.Record(stresschaos.Fatal, "SetActiveProject failed after churn: "+err.Error())
		}
		ap, err := mgr.GetActiveProject(ctx)
		if err != nil || ap == nil || ap.ID != fresh.ID {
			rec.Record(stresschaos.Fatal, fmt.Sprintf("active project not coherent after churn: got=%v err=%v", ap, err))
		} else {
			rec.Record(stresschaos.Recovered, "manager fully usable after churn — map + activeProject self-consistent")
		}
	}

	rec.AssertNoFatal()
	t.Logf("project manager state churn: final map holds %d projects", len(finalList))
}

// TestManager_Chaos_ConcurrentGetActiveLazyScan deterministically drives the
// REAL GetActiveProject lazy-scan branch from many goroutines AT ONCE while the
// in-memory activeProject pointer is nil but a project is flagged Active. On that
// branch GetActiveProject assigns m.activeProject while holding only an RLock —
// so N concurrent callers all reach the assignment under a shared read lock and
// write the SAME field with no mutual exclusion: a write/write data race the
// race detector MUST flag. A clean run proves the assignment is properly
// write-locked; a race report is a genuine §11.4.85(B) state-corruption defect.
//
// This is the highest-value chaos probe in the package: it targets the exact
// non-reentrant / wrong-lock-mode hazard class the session has been hunting.
func TestManager_Chaos_ConcurrentGetActiveLazyScan(t *testing.T) {
	rec := stresschaos.NewChaosRecorder(t, "project_manager_get_active_lazy_scan", "state-corruption")
	ctx := context.Background()
	dir := buildProjectDir(t, "go")

	const rounds = 40
	for round := 0; round < rounds; round++ {
		mgr := NewManager()
		// Create one project and mark it Active via the real SetActiveProject,
		// then force the cached pointer back to nil so the NEXT GetActiveProject
		// callers all take the lazy-scan-and-assign branch concurrently.
		p, err := mgr.CreateProject(ctx, fmt.Sprintf("active-%d", round), "d", dir, "")
		if err != nil {
			t.Fatalf("CreateProject: %v", err)
		}
		if err := mgr.SetActiveProject(ctx, p.ID); err != nil {
			t.Fatalf("SetActiveProject: %v", err)
		}
		// Clear the cached pointer (under the real write lock) WITHOUT clearing
		// project.Active — exactly the state reached transiently in production
		// when activeProject is reset but a project still carries Active=true.
		mgr.mu.Lock()
		mgr.activeProject = nil
		mgr.mu.Unlock()

		const goroutines = 12
		var wg sync.WaitGroup
		var got int64
		start := make(chan struct{})
		for g := 0; g < goroutines; g++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer func() {
					if pn := recover(); pn != nil {
						rec.Record(stresschaos.Fatal, fmt.Sprintf("GetActiveProject panicked: %v", pn))
					}
				}()
				<-start // release all at once to maximise overlap on the lazy branch
				if ap, err := mgr.GetActiveProject(ctx); err == nil && ap != nil {
					atomic.AddInt64(&got, 1)
				}
			}()
		}
		close(start)
		wg.Wait()
		if atomic.LoadInt64(&got) == 0 {
			rec.Record(stresschaos.Fatal, fmt.Sprintf("round %d: no goroutine resolved the active project via lazy scan", round))
		}
	}

	rec.Record(stresschaos.Recovered, fmt.Sprintf("survived %d rounds of concurrent lazy-scan GetActiveProject with no race/panic", rounds))
	rec.AssertNoFatal()
}

// TestManager_Chaos_GoroutineDeathMidOp drives a long mixed-op loop against the
// real manager inside a context-cancellable goroutine, then kills it mid-flight.
// The manager (shared mutable state) MUST remain usable for fresh operations
// after the goroutine is torn down — proof that a dying worker cannot leave the
// store locked or torn.
func TestManager_Chaos_GoroutineDeathMidOp(t *testing.T) {
	mgr := NewManager()
	ctx := context.Background()
	dir := buildProjectDir(t, "go")

	// Seed so the worker has real ids to churn.
	var seed []string
	for i := 0; i < 10; i++ {
		p, err := mgr.CreateProject(ctx, fmt.Sprintf("seed-%d", i), "d", dir, "")
		if err != nil {
			t.Fatalf("seed: %v", err)
		}
		seed = append(seed, p.ID)
	}

	stresschaos.ChaosKillDuring(t, "project_manager_goroutine_death", 150_000_000, // 150ms
		func(opCtx context.Context, rec *stresschaos.ChaosRecorder) {
			i := 0
			for {
				select {
				case <-opCtx.Done():
					rec.Record(stresschaos.Recovered, "worker observed cancellation and stopped touching the manager")
					return
				default:
				}
				id := seed[i%len(seed)]
				switch i % 4 {
				case 0:
					_, _ = mgr.CreateProject(ctx, fmt.Sprintf("worker-%d", i), "d", dir, "")
				case 1:
					_ = mgr.SetActiveProject(ctx, id)
				case 2:
					_, _ = mgr.GetActiveProject(ctx)
				default:
					_, _ = mgr.GetProject(ctx, id)
				}
				i++
			}
		})

	// After the worker is killed mid-op, the manager must still serve correctly.
	freshDir := buildProjectDir(t, "node")
	p, err := mgr.CreateProject(ctx, "after-death", "d", freshDir, "")
	if err != nil {
		t.Fatalf("manager unusable after worker goroutine death: %v", err)
	}
	if _, err := mgr.GetProject(ctx, p.ID); err != nil {
		t.Fatalf("GetProject failed after worker death: %v", err)
	}
	if _, err := mgr.ListProjects(ctx, ""); err != nil {
		t.Fatalf("ListProjects failed after worker death: %v", err)
	}
}
