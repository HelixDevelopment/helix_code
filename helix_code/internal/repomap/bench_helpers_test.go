package repomap

import (
	"os"
	"path/filepath"
	"testing"
)

// bench_helpers_test.go — speed-programme Phase 0 task P0-T02.
//
// Unit tests asserting the bench_test.go fixture builder produces a valid Go
// source tree, so a broken fixture fails loudly in `go test` rather than
// silently producing a zero-file benchmark (CONST-050).

// TestBenchFixture_BuildsValidTree asserts buildRepomapFixture writes the
// requested number of non-empty .go files and that a RepoMap built over them
// actually discovers them.
func TestBenchFixture_BuildsValidTree(t *testing.T) {
	const want = 12
	root := buildRepomapFixture(t, want)

	var goFiles int
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && filepath.Ext(path) == ".go" {
			info, statErr := d.Info()
			if statErr != nil {
				return statErr
			}
			if info.Size() == 0 {
				t.Fatalf("fixture file %s is empty", path)
			}
			goFiles++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk fixture tree: %v", err)
	}
	if goFiles != want {
		t.Fatalf("fixture produced %d .go files, want %d", goFiles, want)
	}

	cfg := DefaultConfig()
	cfg.CacheEnabled = false
	rm, err := NewRepoMap(root, cfg)
	if err != nil {
		t.Fatalf("NewRepoMap over fixture: %v", err)
	}
	stats, err := rm.GetStatistics()
	if err != nil {
		t.Fatalf("GetStatistics over fixture: %v", err)
	}
	if stats.TotalFiles == 0 {
		t.Fatal("RepoMap discovered zero files in the benchmark fixture")
	}
}
