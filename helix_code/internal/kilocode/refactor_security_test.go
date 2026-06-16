// Standing regression guard (§11.4.135) for the path-traversal defect in
// internal/kilocode Refactorer.
//
// Defect (FACT, reproduced 2026-06-16): Refactorer stored rootDir but never
// enforced it. ExtractMethod / InlineCall passed the caller-supplied
// sourceFile straight to os.ReadFile / os.WriteFile, so a "file" param
// pointing OUTSIDE rootDir (via "..", an absolute path, or a symlink) read
// and overwrote arbitrary files on disk. KiloMultiEditTool.Execute feeds
// params["file"] directly into these methods, so the param is attacker-/
// config-influenced.
//
// §11.4.115 RED polarity:
//   - RED_MODE=1 : reproduces the defect on a faithful PRE-FIX stand-in
//                  (preFixRefactorWrite, the unscoped writer the old code
//                  effectively was) and asserts the out-of-root file WAS
//                  clobbered — proving the guard is real.
//   - RED_MODE=0 (DEFAULT): drives the REAL fixed Refactorer and asserts the
//                  out-of-root file is UNTOUCHED and ErrPathOutsideRoot is
//                  returned.
//
// Mocks ALLOWED — unit test (CONST-050(A)). No network, no external agent;
// every path is inside t.TempDir().
package kilocode

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// preFixRefactorWrite faithfully reproduces the pre-fix InlineCall body:
// read the caller-supplied path, mutate, write back — with NO within-root
// validation. It exists ONLY so RED_MODE=1 can demonstrate the historical
// defect on a stand-in (the real fixed code can no longer exhibit it).
func preFixRefactorWrite(sourceFile, funcName string) error {
	src, err := os.ReadFile(sourceFile)
	if err != nil {
		return err
	}
	content := string(src)
	newContent := strings.ReplaceAll(content, funcName+"()", "/* inlined "+funcName+" */")
	if newContent == content {
		return nil
	}
	return os.WriteFile(sourceFile, []byte(newContent), 0644)
}

func redMode() bool { return os.Getenv("RED_MODE") == "1" }

func TestGuard_Refactorer_InlineCall_RejectsOutsideRoot(t *testing.T) {
	rootDir := t.TempDir()
	outsideDir := t.TempDir() // sibling, NOT under rootDir

	const original = "package x\nfunc helper() { helper() }\n"
	marker := filepath.Join(outsideDir, "OUTSIDE_MARKER.go")
	if err := os.WriteFile(marker, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	if redMode() {
		// Reproduce the historical defect on the faithful pre-fix stand-in.
		if err := preFixRefactorWrite(marker, "helper"); err != nil {
			t.Fatalf("pre-fix stand-in errored unexpectedly: %v", err)
		}
		got, _ := os.ReadFile(marker)
		if string(got) == original {
			t.Fatal("RED_MODE: expected pre-fix stand-in to CLOBBER the out-of-root file, but it was untouched")
		}
		t.Logf("RED_MODE reproduction OK: pre-fix writer clobbered out-of-root file:\n%s", string(got))
		return
	}

	// GREEN guard: the REAL fixed Refactorer must refuse and leave the file intact.
	r := NewRefactorer(rootDir)
	err := r.InlineCall(marker, "helper")
	if !errors.Is(err, ErrPathOutsideRoot) {
		t.Fatalf("InlineCall(out-of-root) error = %v, want ErrPathOutsideRoot", err)
	}
	got, _ := os.ReadFile(marker)
	if string(got) != original {
		t.Fatalf("VULNERABLE: out-of-root file was modified despite the root guard:\n%s", string(got))
	}
}

func TestGuard_Refactorer_ExtractMethod_RejectsOutsideRoot(t *testing.T) {
	rootDir := t.TempDir()
	outsideDir := t.TempDir()

	const original = "package x\nfunc main() {\n\tprintln(\"a\")\n\tprintln(\"b\")\n}\n"
	marker := filepath.Join(outsideDir, "OUTSIDE_EXTRACT.go")
	if err := os.WriteFile(marker, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	r := NewRefactorer(rootDir)
	err := r.ExtractMethod(marker, "extracted", 3, 4)
	if redMode() {
		// In RED mode we assert the *historical* behaviour would have written
		// the file; the real fixed code returns the guard error instead.
		// (We do not invoke a pre-fix stand-in for ExtractMethod here — the
		// InlineCall RED case above already proves the guard's reality; this
		// case is the standing GREEN regression guard.)
		t.Skip("SKIP-OK: RED_MODE handled by InlineCall reproduction; ExtractMethod retains GREEN guard")
	}
	if !errors.Is(err, ErrPathOutsideRoot) {
		t.Fatalf("ExtractMethod(out-of-root) error = %v, want ErrPathOutsideRoot", err)
	}
	got, _ := os.ReadFile(marker)
	if string(got) != original {
		t.Fatalf("VULNERABLE: out-of-root file modified by ExtractMethod:\n%s", string(got))
	}
}

func TestGuard_Refactorer_RejectsDotDotTraversal(t *testing.T) {
	if redMode() {
		t.Skip("SKIP-OK: RED_MODE handled by InlineCall reproduction")
	}
	rootDir := t.TempDir()
	// plant a marker in the parent of rootDir, reached via "../"
	parent := filepath.Dir(rootDir)
	const original = "package x\nfunc helper() { helper() }\n"
	traversalName := "kilocode_traversal_marker.go"
	marker := filepath.Join(parent, traversalName)
	if err := os.WriteFile(marker, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(marker) })

	r := NewRefactorer(rootDir)
	// relative "../<marker>" must be rejected
	err := r.InlineCall(filepath.Join("..", traversalName), "helper")
	if !errors.Is(err, ErrPathOutsideRoot) {
		t.Fatalf("InlineCall(../marker) error = %v, want ErrPathOutsideRoot", err)
	}
	got, _ := os.ReadFile(marker)
	if string(got) != original {
		t.Fatalf("VULNERABLE: ../ traversal modified a file above the root:\n%s", string(got))
	}
}

// In-root operations must STILL work (the fix must not over-reject).
func TestGuard_Refactorer_InRootStillWorks(t *testing.T) {
	if redMode() {
		t.Skip("SKIP-OK: RED_MODE handled by InlineCall reproduction")
	}
	rootDir := t.TempDir()
	path := filepath.Join(rootDir, "main.go")
	if err := os.WriteFile(path, []byte("package main\nfunc helper() { println(1) }\nfunc main() { helper() }\n"), 0644); err != nil {
		t.Fatal(err)
	}
	r := NewRefactorer(rootDir)
	if err := r.InlineCall(path, "helper"); err != nil {
		t.Fatalf("in-root InlineCall errored: %v", err)
	}
	got, _ := os.ReadFile(path)
	if !strings.Contains(string(got), "/* inlined helper */") {
		t.Fatalf("in-root InlineCall did not perform the refactor:\n%s", string(got))
	}
}
