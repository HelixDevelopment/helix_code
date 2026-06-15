package checkpoint

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
)

// idBurstCount is the number of newID() calls made back-to-back to detect
// same-timestamp collisions deterministically. On hosts whose wall clock has
// coarse (sub-microsecond but not sub-nanosecond) resolution, a tight burst
// reliably lands many calls in the same formatted instant — the exact data-loss
// trigger when two Create calls share an id (same .helix/checkpoints/<id> dir,
// same refs/helix/checkpoints/<id>, same meta.json — second clobbers first).
const idBurstCount = 50000

// initGitRepoForCollision makes dir a real git repo with an initial commit so
// the git backend's update-ref path is exercised against actual git plumbing
// (no mocks — §11.4 / CONST-035). Mirrors initGitRepo in checkpoint_test.go.
func initGitRepoForCollision(t *testing.T, dir string) {
	t.Helper()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init")
	run("config", "user.email", "test@helix.code")
	run("config", "user.name", "Helix Test")
	run("config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(dir, ".keep"), []byte("seed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".keep")
	run("commit", "-m", "init")
}

func uniqueCount(ids []string) int {
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		seen[id] = struct{}{}
	}
	return len(seen)
}

// burstNewIDs calls newID() n times back-to-back and returns every id produced.
func burstNewIDs(n int) []string {
	ids := make([]string, n)
	for i := 0; i < n; i++ {
		ids[i] = newID()
	}
	return ids
}

// TestNewIDCollision_RED is the §11.4.115 RED-on-broken-artifact reproduction
// AND the standing GREEN regression guard for the same source — one source, two
// roles, selected by the RED_MODE polarity switch:
//
//   - DEFAULT (RED_MODE unset, or any value != "1"): the standing GREEN guard
//     per §11.4.135 — asserts the defect is ABSENT (every id in the burst is
//     unique). The permanent regression test runs in this mode in CI / the
//     normal suite, so the suite stays green on the fixed artifact.
//   - RED_MODE=1: reproduction mode — asserts the DEFECT IS PRESENT on the
//     current artifact (a deterministic burst of newID() calls produces FEWER
//     unique ids than calls made: same-timestamp collision). Captures the
//     collision count as positive evidence the defect is genuine, never
//     synthetic. Against the UNFIXED wall-clock-only newID() this PASSes (defect
//     reproduced — the captured RED proof); against the FIXED newID() it FAILs
//     (no collision left to reproduce — the signal the fix took).
func TestNewIDCollision_RED(t *testing.T) {
	reproduceMode := os.Getenv("RED_MODE") == "1" // opt-in: assert the defect is present

	ids := burstNewIDs(idBurstCount)
	uniq := uniqueCount(ids)

	if reproduceMode {
		if uniq >= idBurstCount {
			t.Fatalf("RED(RED_MODE=1) expected to reproduce the id-collision defect on the "+
				"current artifact, but all %d ids were unique. This is the FIXED build — the "+
				"defect is gone, so reproduction mode correctly fails. Unset RED_MODE to run "+
				"the standing GREEN guard.", idBurstCount)
		}
		t.Logf("RED reproduced data-loss id collision: calls=%d unique_ids=%d collisions=%d",
			idBurstCount, uniq, idBurstCount-uniq)
		return
	}

	// GREEN guard (default): every id in the burst must be distinct.
	if uniq != idBurstCount {
		t.Fatalf("id collision regressed: calls=%d unique_ids=%d collisions=%d (want 0 collisions)",
			idBurstCount, uniq, idBurstCount-uniq)
	}
	t.Logf("GREEN: %d back-to-back newID() calls produced %d unique ids (0 collisions)",
		idBurstCount, uniq)
}

// TestConcurrentNewID_AllUnique is the concurrency-flavoured GREEN guard: many
// goroutines hammer newID() simultaneously and EVERY id must be unique. This is
// the real-world shape of the data-loss bug (parallel agent Create calls). Run
// under -race it also proves the uniqueness source is race-free.
func TestConcurrentNewID_AllUnique(t *testing.T) {
	if os.Getenv("RED_MODE") == "1" {
		// In reproduction mode this guard is informational: concurrent
		// same-nanosecond collisions are timing-dependent (may or may not
		// reproduce per host), so we do not FAIL the broken build here — the
		// deterministic burst test above is the authoritative RED. By default
		// (fixed build) this is a hard guard.
		t.Skip("SKIP-OK: concurrent collision is timing-dependent; deterministic RED is TestNewIDCollision_RED. Runs as a hard guard by default.")
	}
	const n = 2000
	ids := make([]string, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			ids[i] = newID()
		}(i)
	}
	wg.Wait()
	if u := uniqueCount(ids); u != n {
		t.Fatalf("concurrent newID collision: calls=%d unique_ids=%d collisions=%d", n, u, n-u)
	}
	t.Logf("GREEN: %d concurrent newID() calls all unique", n)
}

// TestConcurrentCreate_AllUnique_AllListable_CorrectRestore is the load-bearing
// GREEN guard proving the FIX actually prevents data loss end-to-end: each of N
// concurrent checkpoints round-trips a DISTINCT payload, every id is unique +
// listable, and every Restore brings back ITS OWN bytes (not a colliding
// sibling's). On the unfixed code an id collision makes two checkpoints share a
// dir/meta, so at least one Restore returns the wrong snapshot's bytes — caught
// here. Each goroutine uses its own working dir so the assertion isolates the
// id-uniqueness property (not git's index.lock serialization).
func TestConcurrentCreate_AllUnique_AllListable_CorrectRestore(t *testing.T) {
	const n = 64
	type cp struct {
		id      string
		payload []byte
		file    string
	}

	cps := make([]cp, n)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			dir := t.TempDir() // files backend, isolated per goroutine
			file := filepath.Join(dir, "payload.txt")
			payload := []byte(fmt.Sprintf("unique-payload-%d-%x\n", i, i*2654435761))
			fail := func(err error) {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
			}
			if err := os.WriteFile(file, payload, 0o644); err != nil {
				fail(err)
				return
			}
			m, err := NewManager(dir)
			if err != nil {
				fail(err)
				return
			}
			id, err := m.Create(fmt.Sprintf("payload-%d", i))
			if err != nil {
				fail(err)
				return
			}
			mu.Lock()
			cps[i] = cp{id: id, payload: payload, file: file}
			mu.Unlock()
		}(i)
	}
	wg.Wait()
	if firstErr != nil {
		t.Fatalf("setup error: %v", firstErr)
	}

	// All ids unique.
	ids := make([]string, n)
	for i := range cps {
		ids[i] = cps[i].id
	}
	if u := uniqueCount(ids); u != n {
		t.Fatalf("expected %d unique ids, got %d (collisions=%d)", n, u, n-u)
	}

	// Each checkpoint listable in its own store + Restore returns ITS OWN bytes.
	for i := range cps {
		c := cps[i]
		dir := filepath.Dir(c.file)
		m, err := NewManager(dir)
		if err != nil {
			t.Fatalf("cp %d NewManager: %v", i, err)
		}
		listed := false
		for _, lc := range m.List() {
			if lc.ID == c.id {
				listed = true
				break
			}
		}
		if !listed {
			t.Fatalf("cp %d id %q not listable", i, c.id)
		}
		if err := os.WriteFile(c.file, []byte("mutated\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := m.Restore(c.id); err != nil {
			t.Fatalf("cp %d Restore: %v", i, err)
		}
		got, _ := os.ReadFile(c.file)
		if string(got) != string(c.payload) {
			t.Fatalf("cp %d Restore returned WRONG bytes (collision data-loss): got %q want %q",
				i, got, c.payload)
		}
	}
	t.Logf("GREEN: %d concurrent checkpoints all-unique, all-listable, each Restore returned its own bytes", n)
}

// TestSequentialCreate_AllUnique_Git guards id uniqueness through the GIT
// backend (update-ref refs/helix/checkpoints/<id>), where an id collision
// overwrites the first snapshot's pinned commit — the irrecoverable path.
// Creates are sequential to isolate the id-uniqueness property from git's
// single-index.lock serialization (a separate concern outside newID()).
func TestSequentialCreate_AllUnique_Git(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("SKIP-OK: git binary not available in this environment")
	}
	dir := t.TempDir()
	initGitRepoForCollision(t, dir)
	m, err := NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	if m.Backend() != "git" {
		t.Fatalf("expected git backend, got %q", m.Backend())
	}

	const n = 200 // tight sequential loop — same-timestamp window for old newID()
	ids := make([]string, 0, n)
	for i := 0; i < n; i++ {
		id, err := m.Create(fmt.Sprintf("git-cp-%d", i))
		if err != nil {
			t.Fatalf("Create %d: %v", i, err)
		}
		ids = append(ids, id)
	}
	if u := uniqueCount(ids); u != n {
		t.Fatalf("git id collision: created=%d unique_ids=%d collisions=%d", n, u, n-u)
	}
	// Each id must resolve to a distinct ref still present.
	for _, id := range ids {
		if _, err := m.git(context.Background(), "rev-parse", "--verify", refPrefix+id+"^{commit}"); err != nil {
			t.Fatalf("ref for id %q missing (clobbered?): %v", id, err)
		}
	}
	t.Logf("GREEN(git): %d sequential Create produced %d unique ids, all refs intact", n, u(ids))
}

func u(ids []string) int { return uniqueCount(ids) }
