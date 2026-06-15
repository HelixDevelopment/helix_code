package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAiderRecordingPath_NoSharedTmpClobber is the standing regression guard
// (§11.4.135) for the insecure-temporary-file defect in /aider voice start.
//
// Defect (CWE-377 / CWE-379): the pre-fix handler hardcoded the fixed,
// world-shared, predictable path "/tmp/helixcode_aider_recording.wav". On a
// multi-user host a fixed name inside a world-writable directory is a
// symlink-clobber / pre-creation surface and same-user concurrent invocations
// collide on the one name.
//
// §11.4.115 polarity:
//
//   - RED_MODE=1  → exercise a faithful inline stand-in of the REMOVED pre-fix
//     algorithm (the fixed "/tmp/..." path) and ASSERT the unsafe outcome holds
//     (fixed name in a world-writable shared dir; two calls return the IDENTICAL
//     path). This run PASSES on the broken algorithm, proving the guard targets
//     a real defect. All filesystem writes are confined to t.TempDir() — the
//     stand-in builds the path under a planted world-writable dir, never the
//     real host /tmp.
//
//   - RED_MODE=0 (DEFAULT, no env) → drive the REAL aiderRecordingPath() and
//     assert the SAFE outcome: each call returns a UNIQUE path inside a freshly
//     created OWNER-ONLY (0700) directory with an unpredictable random
//     component, and the path is NOT the fixed legacy literal.
func TestAiderRecordingPath_NoSharedTmpClobber(t *testing.T) {
	if os.Getenv("RED_MODE") == "1" {
		// --- RED: reproduce the defect on a faithful pre-fix stand-in. ---
		// Plant a world-writable "shared tmp" inside t.TempDir() to stand in
		// for the host's /tmp, so we never touch the real host filesystem.
		sharedTmp := filepath.Join(t.TempDir(), "shared_tmp")
		if err := os.MkdirAll(sharedTmp, 0o777); err != nil {
			t.Fatalf("plant shared tmp: %v", err)
		}

		// Inline copy of the REMOVED pre-fix algorithm: a fixed name inside a
		// world-shared directory, identical on every invocation.
		prefixPath := func() string {
			return filepath.Join(sharedTmp, "helixcode_aider_recording.wav")
		}

		p1 := prefixPath()
		p2 := prefixPath()

		// Unsafe property 1: predictable + collides — two calls are identical.
		if p1 != p2 {
			t.Fatalf("RED expected identical (colliding) paths from the fixed-name "+
				"pre-fix algorithm, got %q and %q", p1, p2)
		}

		// Unsafe property 2: the containing directory is world-writable, i.e. a
		// symlink-clobber / pre-creation surface. Demonstrate the clobber: a
		// "foreign" process pre-creates a symlink at the predicted path pointing
		// at a victim file; a writer that opens the predicted path with default
		// (symlink-following, truncating) semantics overwrites the victim.
		victim := filepath.Join(t.TempDir(), "victim_secret.txt")
		if err := os.WriteFile(victim, []byte("VICTIM-DATA"), 0o600); err != nil {
			t.Fatalf("write victim: %v", err)
		}
		if err := os.Symlink(victim, p1); err != nil {
			t.Fatalf("plant symlink (attacker step): %v", err)
		}
		// The aider capture process opens the predicted path for writing. os
		// file APIs follow symlinks by default — emulate that write.
		if err := os.WriteFile(p1, []byte("CAPTURE-OUTPUT"), 0o600); err != nil {
			t.Fatalf("capture write through predicted path: %v", err)
		}
		clobbered, err := os.ReadFile(victim)
		if err != nil {
			t.Fatalf("read victim after capture: %v", err)
		}
		if string(clobbered) != "CAPTURE-OUTPUT" {
			t.Fatalf("RED expected the fixed-shared-path algorithm to enable a "+
				"symlink clobber of the victim file, but victim still contains %q",
				string(clobbered))
		}
		// RED reproduction succeeded: the pre-fix algorithm is genuinely unsafe.
		return
	}

	// --- GREEN (default): the REAL fixed code must be safe. ---
	p1, err := aiderRecordingPath()
	if err != nil {
		t.Fatalf("aiderRecordingPath() #1: %v", err)
	}
	defer os.RemoveAll(filepath.Dir(p1))
	p2, err := aiderRecordingPath()
	if err != nil {
		t.Fatalf("aiderRecordingPath() #2: %v", err)
	}
	defer os.RemoveAll(filepath.Dir(p2))

	// Safe property 1: not the removed legacy literal.
	if p1 == "/tmp/helixcode_aider_recording.wav" {
		t.Fatalf("regressed to the fixed world-shared legacy path: %q", p1)
	}

	// Safe property 2: unique per invocation (no same-user collision).
	if p1 == p2 {
		t.Fatalf("two recording paths collided: %q == %q", p1, p2)
	}
	if filepath.Dir(p1) == filepath.Dir(p2) {
		t.Fatalf("two recording dirs collided: %q", filepath.Dir(p1))
	}

	// Safe property 3: the containing directory exists and is OWNER-ONLY (0700)
	// — no group/other access, so it is not a cross-user clobber surface.
	dir := filepath.Dir(p1)
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat recording dir: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("recording parent is not a directory: %q", dir)
	}
	if perm := info.Mode().Perm(); perm&0o077 != 0 {
		t.Fatalf("recording dir %q is group/world accessible (perm %o); want owner-only 0700",
			dir, perm)
	}

	// Safe property 4: unpredictable random component (the MkdirTemp suffix),
	// so a foreign process cannot pre-create a symlink at the path.
	base := filepath.Base(dir)
	if !strings.HasPrefix(base, "helixcode-aider-") || len(base) <= len("helixcode-aider-") {
		t.Fatalf("recording dir name %q lacks an unpredictable random suffix", base)
	}
}
