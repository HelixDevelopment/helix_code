package filesystem

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// bench_helpers_test.go — speed-programme Phase 0 task P0-T02.
//
// Unit tests asserting the bench_test.go fixture builder + searcher helper work,
// so a broken fixture fails loudly in `go test` rather than silently producing a
// no-match benchmark (CONST-050).

// TestBenchSearchFixture_BuildsAndMatches asserts buildSearchFixture writes the
// requested number of non-empty files, that one-in-three embeds the needle, and
// that newBenchSearcher's SearchContent actually finds those needles.
func TestBenchSearchFixture_BuildsAndMatches(t *testing.T) {
	const want = 18
	root := buildSearchFixture(t, want)

	var files int
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && filepath.Ext(path) == ".txt" {
			info, statErr := d.Info()
			if statErr != nil {
				return statErr
			}
			if info.Size() == 0 {
				t.Fatalf("search fixture file %s is empty", path)
			}
			files++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk search fixture tree: %v", err)
	}
	if files != want {
		t.Fatalf("search fixture produced %d .txt files, want %d", files, want)
	}

	searcher := newBenchSearcher(t, root)
	matches, err := searcher.SearchContent(context.Background(), ContentSearchOptions{
		Root:    root,
		Pattern: "NEEDLE_TOKEN",
		IsRegex: false,
	})
	if err != nil {
		t.Fatalf("SearchContent over fixture: %v", err)
	}
	if len(matches) == 0 {
		t.Fatal("benchmark search fixture embeds the needle but SearchContent found nothing")
	}
}
