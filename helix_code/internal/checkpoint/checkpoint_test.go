package checkpoint

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// initGitRepo makes dir a real git repo with an initial commit so the git
// backend is exercised against actual git plumbing (no mocks — §11.4/CONST-035).
func initGitRepo(t *testing.T, dir string) {
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
	// An initial commit so HEAD exists (commit-tree parent / read-tree HEAD paths).
	if err := os.WriteFile(filepath.Join(dir, ".keep"), []byte("seed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".keep")
	run("commit", "-m", "init")
}

func gitAvailable(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("SKIP-OK: git binary not available in this environment")
	}
}

// TestRestoreRestoresRealBytes_Git is the load-bearing anti-bluff test: write
// file v1, checkpoint, modify to v2, restore, assert the bytes ON DISK are v1.
func TestRestoreRestoresRealBytes_Git(t *testing.T) {
	gitAvailable(t)
	dir := t.TempDir()
	initGitRepo(t, dir)

	target := filepath.Join(dir, "app.txt")
	v1 := []byte("version one\nline two\n")
	v2 := []byte("VERSION TWO — totally different bytes\n")

	if err := os.WriteFile(target, v1, 0o644); err != nil {
		t.Fatal(err)
	}
	// Track the file so the snapshot captures it (git stash create only captures
	// tracked changes; adding it to the index is the realistic agent workflow).
	addCmd := exec.Command("git", "add", "app.txt")
	addCmd.Dir = dir
	if out, err := addCmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}

	m, err := NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := m.Backend(); got != "git" {
		t.Fatalf("expected git backend, got %q", got)
	}

	id, err := m.Create("before-edit")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Modify to v2 on disk.
	if err := os.WriteFile(target, v2, 0o644); err != nil {
		t.Fatal(err)
	}
	if cur, _ := os.ReadFile(target); string(cur) != string(v2) {
		t.Fatalf("precondition: file should be v2, got %q", cur)
	}

	if err := m.Restore(id); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(v1) {
		t.Fatalf("restore did not bring back real v1 bytes.\n got: %q\nwant: %q", got, v1)
	}
}

// TestRestoreSurvivesProcessRestart_Git proves the snapshot is persisted (a
// fresh Manager — simulating a new process — can List and Restore it).
func TestRestoreSurvivesProcessRestart_Git(t *testing.T) {
	gitAvailable(t)
	dir := t.TempDir()
	initGitRepo(t, dir)

	target := filepath.Join(dir, "data.txt")
	v1 := []byte("persisted bytes\n")
	if err := os.WriteFile(target, v1, 0o644); err != nil {
		t.Fatal(err)
	}
	addCmd := exec.Command("git", "add", "data.txt")
	addCmd.Dir = dir
	if out, err := addCmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}

	m1, err := NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	id, err := m1.Create("snap")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Mutate after snapshot, then restore via a brand-new Manager.
	if err := os.WriteFile(target, []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	m2, err := NewManager(dir) // fresh handle == fresh process semantics
	if err != nil {
		t.Fatal(err)
	}
	list := m2.List()
	found := false
	for _, cp := range list {
		if cp.ID == id {
			found = true
			if cp.Label != "snap" {
				t.Errorf("label not persisted: %q", cp.Label)
			}
		}
	}
	if !found {
		t.Fatalf("checkpoint %s not found by fresh Manager; List=%+v", id, list)
	}
	if err := m2.Restore(id); err != nil {
		t.Fatalf("Restore from fresh Manager: %v", err)
	}
	got, _ := os.ReadFile(target)
	if string(got) != string(v1) {
		t.Fatalf("cross-process restore bytes mismatch: got %q want %q", got, v1)
	}
}

// TestNewFileAfterCheckpointNotDeleted documents + verifies the new-file
// semantics: a file created AFTER the checkpoint is left intact by Restore
// (additive/overwrite restore, not a destructive sync).
func TestNewFileAfterCheckpointNotDeleted(t *testing.T) {
	gitAvailable(t)
	dir := t.TempDir()
	initGitRepo(t, dir)

	tracked := filepath.Join(dir, "tracked.txt")
	if err := os.WriteFile(tracked, []byte("orig\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	addCmd := exec.Command("git", "add", "tracked.txt")
	addCmd.Dir = dir
	if out, err := addCmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}

	m, err := NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	id, err := m.Create("c1")
	if err != nil {
		t.Fatal(err)
	}

	// Create a brand-new file after the checkpoint.
	newFile := filepath.Join(dir, "created-later.txt")
	if err := os.WriteFile(newFile, []byte("new work\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := m.Restore(id); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	if _, err := os.Stat(newFile); err != nil {
		t.Fatalf("new file created after checkpoint must survive restore, but it is gone: %v", err)
	}
	if data, _ := os.ReadFile(newFile); string(data) != "new work\n" {
		t.Fatalf("new file bytes mutated by restore: %q", data)
	}
}

// TestFilesBackendRoundTrip exercises the non-git fallback: real file-copy
// snapshot + restore of real bytes in a directory that is NOT a git repo.
func TestFilesBackendRoundTrip(t *testing.T) {
	dir := t.TempDir() // NOT a git repo

	target := filepath.Join(dir, "sub", "note.txt")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	v1 := []byte("fallback v1\n")
	v2 := []byte("fallback v2 changed\n")
	if err := os.WriteFile(target, v1, 0o644); err != nil {
		t.Fatal(err)
	}

	m, err := NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := m.Backend(); got != "files" {
		t.Fatalf("expected files backend in non-git dir, got %q", got)
	}

	id, err := m.Create("fallback")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := os.WriteFile(target, v2, 0o644); err != nil {
		t.Fatal(err)
	}

	// Fresh Manager to prove persistence across process boundary.
	m2, err := NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := m2.Restore(id); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	got, _ := os.ReadFile(target)
	if string(got) != string(v1) {
		t.Fatalf("files-backend restore bytes mismatch: got %q want %q", got, v1)
	}
}

func TestRestoreUnknownID(t *testing.T) {
	dir := t.TempDir()
	m, err := NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Restore("does-not-exist"); err == nil {
		t.Fatal("expected error restoring unknown checkpoint id")
	}
}

func TestNewManagerRejectsNonDir(t *testing.T) {
	if _, err := NewManager(""); err == nil {
		t.Fatal("expected error for empty working dir")
	}
	if _, err := NewManager(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Fatal("expected error for missing working dir")
	}
}

// TestRestoreReportsUntrackedNotRestored_Git is the §11.4.115 RED→GREEN polarity
// guard for the anti-bluff defect captured in
// docs/qa/retro-cli-edit-safety-20260616/cli_checkpoint_restore_isolated.txt:
// the git-backend /checkpoint restore printed an unconditional success even when
// untracked working-tree files were NOT restored.
//
// It drives a REAL git checkpoint + restore on a temp dir (no mocks — §11.4 /
// CONST-035): a TRACKED file is committed before the checkpoint, an UNTRACKED
// file is created (never git-added). After a bad edit to BOTH and a restore:
//   - the tracked file's bytes on disk come back (snapshot covered it),
//   - the untracked file's bytes stay corrupted (snapshot never captured it),
//   - the RestoreReport TRUTHFULLY lists the untracked file as NOT restored.
//
// Polarity switch RED_MODE (default "1" = reproduce the defect surface on the
// real artifact: assert the untracked file is genuinely left corrupted AND that
// the honest report flags it — the pre-fix unconditional-success path could not
// distinguish this). RED_MODE=0 is the standing GREEN regression guard asserting
// the report flags the untracked file so the CLI prints the honest warning.
func TestRestoreReportsUntrackedNotRestored_Git(t *testing.T) {
	gitAvailable(t)
	dir := t.TempDir()
	initGitRepo(t, dir)

	// Tracked file: committed before the checkpoint so the git snapshot captures
	// it (this half of the behaviour genuinely works and must keep working).
	tracked := filepath.Join(dir, "tracked.txt")
	if err := os.WriteFile(tracked, []byte("TRACKED good v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("add", "tracked.txt")
	run("commit", "-m", "add tracked")

	// Untracked file: created in the working tree, NEVER git-added — exactly the
	// case the captured evidence shows the success message lied about.
	untracked := filepath.Join(dir, "work.txt")
	untrackedGood := []byte("UNTRACKED good v1\n")
	if err := os.WriteFile(untracked, untrackedGood, 0o644); err != nil {
		t.Fatal(err)
	}

	m, err := NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := m.Backend(); got != "git" {
		t.Fatalf("expected git backend, got %q", got)
	}

	id, err := m.Create("good-state")
	if err != nil {
		t.Fatalf("create checkpoint: %v", err)
	}

	// Corrupt BOTH files (simulating the agent / external bad edit).
	bad := []byte("CORRUPTED BAD EDIT xxxx\n")
	if err := os.WriteFile(tracked, bad, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(untracked, bad, 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := m.RestoreReported(id)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}

	// Tracked file MUST be restored (this part works and must not regress).
	gotTracked, err := os.ReadFile(tracked)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotTracked) != "TRACKED good v1\n" {
		t.Fatalf("tracked file not restored: got %q want %q", gotTracked, "TRACKED good v1\n")
	}

	// Untracked file is NOT restored by the git backend (it was never in the
	// snapshot tree). This is the underlying behaviour; the DEFECT was that the
	// CLI claimed full restore anyway. The honest report MUST surface it.
	gotUntracked, err := os.ReadFile(untracked)
	if err != nil {
		t.Fatal(err)
	}
	untrackedRestored := string(gotUntracked) == string(untrackedGood)

	listed := false
	for _, f := range report.UntrackedNotRestored {
		if f == "work.txt" {
			listed = true
			break
		}
	}

	redMode := os.Getenv("RED_MODE") != "0" // default "1": reproduce the defect

	if redMode {
		// RED: prove the defect surface exists on the real artifact — the
		// untracked file is genuinely left corrupted (so an unconditional
		// "working tree restored" message would be a lie). If the untracked file
		// were somehow restored, there would be no defect to guard against.
		if untrackedRestored {
			t.Fatalf("RED expectation broken: untracked file WAS restored — the documented defect (untracked files not restored) does not reproduce; the guard is blind")
		}
		// And the fix's report must be able to tell the truth about it.
		if !listed {
			t.Fatalf("RED→FIX: report did not flag the un-restored untracked file work.txt; got %v — the pre-fix unconditional-success path cannot distinguish this", report.UntrackedNotRestored)
		}
		t.Logf("RED reproduced: untracked work.txt left corrupted by restore; honest report flags it (UntrackedNotRestored=%v) so the CLI no longer prints an unconditional success", report.UntrackedNotRestored)
	} else {
		// GREEN standing guard: the report MUST flag the un-restored untracked
		// file so the CLI prints the honest warning, never a false success.
		if !listed {
			t.Fatalf("GREEN guard failed: RestoreReport.UntrackedNotRestored must include work.txt; got %v", report.UntrackedNotRestored)
		}
		if report.FullyRestored() {
			t.Fatalf("GREEN guard failed: FullyRestored() must be false when untracked files were not restored")
		}
	}
}

// TestRestoreReportFullyRestored_FilesBackend confirms the files backend (which
// snapshots untracked content too) reports a full restore so the CLI keeps
// printing the unqualified success there — the honest-report fix must not
// false-warn on the backend that genuinely covers everything.
func TestRestoreReportFullyRestored_FilesBackend(t *testing.T) {
	dir := t.TempDir() // not a git repo → files backend
	f := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(f, []byte("v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := m.Backend(); got != "files" {
		t.Fatalf("expected files backend, got %q", got)
	}
	id, err := m.Create("fb")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f, []byte("BAD\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	report, err := m.RestoreReported(id)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	if !report.FullyRestored() {
		t.Fatalf("files backend must report full restore; got UntrackedNotRestored=%v", report.UntrackedNotRestored)
	}
	got, _ := os.ReadFile(f)
	if string(got) != "v1\n" {
		t.Fatalf("files backend did not restore bytes: got %q", got)
	}
}

// keep context import used even if future edits drop a usage.
var _ = context.Background
