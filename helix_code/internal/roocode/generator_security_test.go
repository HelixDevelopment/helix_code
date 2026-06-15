package roocode

// §11.4.115 RED_MODE polarity regression guards for the path-traversal
// defect class in CodeGenerator (Generate + Bootstrap).
//
// Defect (reproduced 2026-06-16): spec.Name / spec.OutputDir are
// external (LLM- or user-supplied) inputs joined into a filesystem path
// with NO within-base validation. A ".."-bearing spec.Name or an
// absolute/escaping spec.OutputDir let Roo-code-driven generation write
// files ANYWHERE on the host, outside the configured outputDir — a real
// path-traversal write primitive.
//
// Fix: withinBase() in generator.go confines every resolved path to
// outputDir before any mkdir/write, returning ErrPathTraversal otherwise.
//
// Polarity:
//   RED_MODE=1 → drive a faithful pre-fix stand-in (raw filepath.Join +
//                os.WriteFile, exactly the old behaviour) and PROVE the
//                traversal happens (marker written outside the base).
//   RED_MODE=0 (default) → drive the REAL fixed CodeGenerator and assert
//                the traversal is REJECTED (ErrPathTraversal) and no file
//                lands outside the base.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func redMode(t *testing.T) bool {
	t.Helper()
	return os.Getenv("RED_MODE") == "1"
}

// preFixGenerate reproduces the EXACT pre-fix Generate path-building:
// raw join of outputDir + name, no validation. Used only by RED_MODE=1.
func preFixGenerate(outputDir string, spec GenerateSpec) (string, error) {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", err
	}
	fileName := spec.Name + fileExtension(spec.Type)
	filePath := filepath.Join(outputDir, fileName)
	if err := os.WriteFile(filePath, []byte(buildContent(spec)), 0644); err != nil {
		return "", err
	}
	return filePath, nil
}

func TestGenerate_PathTraversal_Guard(t *testing.T) {
	base := t.TempDir()
	outDir := filepath.Join(base, "out")
	if err := os.MkdirAll(outDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Marker would land in base/ (one level ABOVE outDir) on traversal.
	marker := filepath.Join(base, "victim.txt")
	spec := GenerateSpec{Type: "txt", Name: "../victim"}

	if redMode(t) {
		// Reproduce the defect on the faithful pre-fix stand-in.
		if _, err := preFixGenerate(outDir, spec); err != nil {
			t.Fatalf("pre-fix stand-in unexpectedly errored: %v", err)
		}
		if _, err := os.Stat(marker); err != nil {
			t.Fatalf("RED expected traversal to write %s, but it did not: %v", marker, err)
		}
		t.Logf("RED reproduced: pre-fix Generate wrote OUTSIDE base at %s", marker)
		return
	}

	// GREEN: the real fixed generator must reject the escape.
	_, err := NewCodeGenerator(outDir).Generate(context.Background(), spec)
	if !errors.Is(err, ErrPathTraversal) {
		t.Fatalf("expected ErrPathTraversal, got %v", err)
	}
	if _, statErr := os.Stat(marker); statErr == nil {
		t.Fatalf("traversal still wrote outside base at %s despite error", marker)
	}
}

func TestGenerate_WithinBase_StillWorks(t *testing.T) {
	// Ensure the guard does NOT over-reject a legitimate in-base name.
	outDir := t.TempDir()
	got, err := NewCodeGenerator(outDir).Generate(
		context.Background(), GenerateSpec{Type: "go", Name: "myFunc", Template: "main"})
	if err != nil {
		t.Fatalf("legitimate generate rejected: %v", err)
	}
	abs, _ := filepath.Abs(outDir)
	if rel, _ := filepath.Rel(abs, got); rel != "myFunc.go" {
		t.Fatalf("unexpected output path %s (rel %s)", got, rel)
	}
	if _, err := os.Stat(got); err != nil {
		t.Fatalf("expected generated file at %s: %v", got, err)
	}
}

func TestBootstrap_PathTraversal_Guard(t *testing.T) {
	base := t.TempDir()
	outDir := filepath.Join(base, "out")
	if err := os.MkdirAll(outDir, 0755); err != nil {
		t.Fatal(err)
	}
	escape := filepath.Join(base, "escaped") // sibling of outDir, outside it
	spec := BootstrapSpec{ProjectType: "go", Name: "x", OutputDir: escape}

	if redMode(t) {
		// Pre-fix Bootstrap used spec.OutputDir verbatim, no validation.
		if err := os.MkdirAll(escape, 0755); err != nil {
			t.Fatal(err)
		}
		mainFile := filepath.Join(escape, "main.go")
		if err := os.WriteFile(mainFile, []byte("package main\n"), 0644); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(mainFile); err != nil {
			t.Fatalf("RED expected escape write at %s: %v", mainFile, err)
		}
		t.Logf("RED reproduced: pre-fix Bootstrap wrote OUTSIDE base at %s", escape)
		return
	}

	// GREEN: fixed Bootstrap rejects the out-of-base OutputDir.
	_, err := NewCodeGenerator(outDir).Bootstrap(context.Background(), spec)
	if !errors.Is(err, ErrPathTraversal) {
		t.Fatalf("expected ErrPathTraversal, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(escape, "main.go")); statErr == nil {
		t.Fatalf("bootstrap still wrote outside base despite error")
	}
}
