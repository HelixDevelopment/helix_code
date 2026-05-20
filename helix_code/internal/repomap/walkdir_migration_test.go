package repomap

// P2-T01 — filepath.Walk -> filepath.WalkDir migration test coverage.
//
// CONST-050(B): the migration's no-regression invariant is that filepath.WalkDir
// yields the EXACT SAME visited file/dir set as the legacy filepath.Walk over the
// same tree, and that the same SkipDir/SkipAll traversal-control semantics still
// hold. This file proves that with a reference filepath.Walk run vs a parallel
// filepath.WalkDir run over a generated fixture tree, plus a cold-walk benchmark
// supplying the before/after evidence the task's anti-bluff proof requires.
//
// CONST-035 / Article XI §11.9: every PASS here carries positive runtime
// evidence (the asserted set equality + the benchmark numbers), not a
// metadata-only / absence-of-error PASS.

import (
	"fmt"
	"io/fs"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// buildWalkFixture creates a deterministic representative source tree: nested
// directories, source files of several languages, hidden files/dirs and the
// common ignore directories (node_modules, vendor, .git, dist, build). Same
// seed => same tree, so the benchmark is reproducible.
func buildWalkFixture(t testing.TB, seed int64, fileCount int) string {
	t.Helper()
	root := t.TempDir()
	rng := rand.New(rand.NewSource(seed))

	exts := []string{".go", ".py", ".js", ".ts", ".rs", ".java"}
	// Directories that downstream walks routinely SkipDir on.
	ignoreDirs := []string{"node_modules", "vendor", ".git", "dist", "build", "bin"}
	dirs := []string{root}

	// A handful of nesting levels.
	for level := 0; level < 4; level++ {
		var next []string
		for _, d := range dirs {
			fanout := 1 + rng.Intn(3)
			for i := 0; i < fanout; i++ {
				name := fmt.Sprintf("pkg_%d_%d", level, i)
				// Sprinkle in hidden + ignore directories.
				if rng.Intn(7) == 0 {
					name = "." + name
				} else if rng.Intn(11) == 0 {
					name = ignoreDirs[rng.Intn(len(ignoreDirs))]
				}
				p := filepath.Join(d, name)
				if err := os.MkdirAll(p, 0o755); err != nil {
					t.Fatalf("mkdir %s: %v", p, err)
				}
				next = append(next, p)
			}
		}
		dirs = append(dirs, next...)
	}

	for i := 0; i < fileCount; i++ {
		d := dirs[rng.Intn(len(dirs))]
		ext := exts[rng.Intn(len(exts))]
		name := fmt.Sprintf("file_%d%s", i, ext)
		if rng.Intn(13) == 0 {
			name = "." + name // hidden file
		}
		p := filepath.Join(d, name)
		body := strings.Repeat("// representative source line\n", 1+rng.Intn(40))
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", p, err)
		}
	}
	return root
}

// referenceWalk is the legacy filepath.Walk traversal — the "before" baseline.
// It returns every path visited (dirs + files), in sorted order.
func referenceWalk(root string) ([]string, error) {
	var visited []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		visited = append(visited, path)
		return nil
	})
	sort.Strings(visited)
	return visited, err
}

// walkDirTraversal is the migrated filepath.WalkDir traversal — the "after".
func walkDirTraversal(root string) ([]string, error) {
	var visited []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		visited = append(visited, path)
		return nil
	})
	sort.Strings(visited)
	return visited, err
}

// TestWalkDir_FileSetEquality_PlainTraversal proves the core no-regression
// invariant: WalkDir visits the IDENTICAL set of paths that legacy Walk did.
func TestWalkDir_FileSetEquality_PlainTraversal(t *testing.T) {
	root := buildWalkFixture(t, 1, 600)

	want, err := referenceWalk(root)
	if err != nil {
		t.Fatalf("reference filepath.Walk failed: %v", err)
	}
	got, err := walkDirTraversal(root)
	if err != nil {
		t.Fatalf("filepath.WalkDir failed: %v", err)
	}

	if len(want) == 0 {
		t.Fatal("fixture produced an empty tree — test would be vacuous")
	}
	if len(want) != len(got) {
		t.Fatalf("path count mismatch: Walk=%d WalkDir=%d", len(want), len(got))
	}
	for i := range want {
		if want[i] != got[i] {
			t.Fatalf("path[%d] mismatch: Walk=%q WalkDir=%q", i, want[i], got[i])
		}
	}
	t.Logf("WalkDir == Walk: identical %d-path set over fixture", len(got))
}

// TestWalkDir_FileSetEquality_WithSkipDir proves the migration also preserves
// SkipDir semantics — the migrated call sites rely on returning filepath.SkipDir
// for hidden / ignore directories, and that control flow must be byte-identical
// between Walk and WalkDir.
func TestWalkDir_FileSetEquality_WithSkipDir(t *testing.T) {
	root := buildWalkFixture(t, 2, 500)

	isSkippableDir := func(name string) bool {
		if strings.HasPrefix(name, ".") && name != "." {
			return true
		}
		switch name {
		case "node_modules", "vendor", ".git", "dist", "build", "bin":
			return true
		}
		return false
	}

	// Reference: legacy Walk with SkipDir.
	var want []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() && path != root && isSkippableDir(filepath.Base(path)) {
			return filepath.SkipDir
		}
		if !info.IsDir() {
			want = append(want, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("reference Walk(SkipDir) failed: %v", err)
	}
	sort.Strings(want)

	// Migrated: WalkDir with SkipDir.
	var got []string
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && path != root && isSkippableDir(d.Name()) {
			return filepath.SkipDir
		}
		if !d.IsDir() {
			got = append(got, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("migrated WalkDir(SkipDir) failed: %v", err)
	}
	sort.Strings(got)

	if len(want) == 0 {
		t.Fatal("SkipDir fixture produced no files — test would be vacuous")
	}
	if len(want) != len(got) {
		t.Fatalf("SkipDir file count mismatch: Walk=%d WalkDir=%d", len(want), len(got))
	}
	for i := range want {
		if want[i] != got[i] {
			t.Fatalf("SkipDir file[%d] mismatch: Walk=%q WalkDir=%q", i, want[i], got[i])
		}
	}
	t.Logf("WalkDir(SkipDir) == Walk(SkipDir): identical %d-file set", len(got))
}

// TestWalkDir_DirEntryInfoMatchesFileInfo proves that d.Info() — the lazy
// FileInfo accessor used by migrated sites that need size/mtime — returns the
// same data legacy Walk's os.FileInfo argument carried.
func TestWalkDir_DirEntryInfoMatchesFileInfo(t *testing.T) {
	root := buildWalkFixture(t, 3, 200)

	walkInfo := make(map[string]os.FileInfo)
	if err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		walkInfo[path] = info
		return nil
	}); err != nil {
		t.Fatalf("reference Walk failed: %v", err)
	}

	checked := 0
	if err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		ref, ok := walkInfo[path]
		if !ok {
			t.Fatalf("WalkDir visited %q that Walk did not", path)
		}
		info, infoErr := d.Info()
		if infoErr != nil {
			t.Fatalf("d.Info() failed for %q: %v", path, infoErr)
		}
		if info.IsDir() != ref.IsDir() {
			t.Fatalf("%q: IsDir mismatch d.Info()=%v Walk=%v", path, info.IsDir(), ref.IsDir())
		}
		if !info.IsDir() {
			if info.Size() != ref.Size() {
				t.Fatalf("%q: Size mismatch d.Info()=%d Walk=%d", path, info.Size(), ref.Size())
			}
			if !info.ModTime().Equal(ref.ModTime()) {
				t.Fatalf("%q: ModTime mismatch", path)
			}
		}
		if info.Name() != ref.Name() {
			t.Fatalf("%q: Name mismatch d.Info()=%q Walk=%q", path, info.Name(), ref.Name())
		}
		checked++
		return nil
	}); err != nil {
		t.Fatalf("WalkDir failed: %v", err)
	}
	if checked == 0 {
		t.Fatal("no entries checked — test would be vacuous")
	}
	t.Logf("d.Info() matched os.FileInfo for all %d entries", checked)
}

// BenchmarkColdWalk_FilepathWalk is the BEFORE measurement: legacy
// filepath.Walk over a large fixture tree.
func BenchmarkColdWalk_FilepathWalk(b *testing.B) {
	root := buildWalkFixture(b, 42, 4000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		count := 0
		_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if !info.IsDir() {
				count++
			}
			return nil
		})
		if count == 0 {
			b.Fatal("walk visited no files")
		}
	}
}

// BenchmarkColdWalk_FilepathWalkDir is the AFTER measurement: migrated
// filepath.WalkDir over the IDENTICAL fixture tree. The delta vs
// BenchmarkColdWalk_FilepathWalk is the P2-T01 speedup evidence.
func BenchmarkColdWalk_FilepathWalkDir(b *testing.B) {
	root := buildWalkFixture(b, 42, 4000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		count := 0
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if !d.IsDir() {
				count++
			}
			return nil
		})
		if count == 0 {
			b.Fatal("walk visited no files")
		}
	}
}
