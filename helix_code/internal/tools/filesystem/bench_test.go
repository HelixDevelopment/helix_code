package filesystem

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// bench_test.go — speed-programme Phase 0 task P0-T02.
//
// Baseline benchmark for the content-search hot path (R1 bottleneck B08 —
// SearchContent serially walks the tree and reads every candidate file). This
// is the agent `Grep` tool path; the benchmark makes Phase 2's parallel-search
// work (P2-T05) falsifiable with pasted before/after numbers (CONST-035 /
// Rule 9). No production code is changed by this file.
//
// P0-T04's helix_code/tests/performance/scenarios/ generator had not landed at
// the time this task was implemented, so the fixture tree is synthesized inline
// into b.TempDir().
//
// Run: go test -bench=. -benchmem -run=^$ ./internal/tools/filesystem/

// benchSearchFixtureFile is a small text body. Roughly one in every few files
// contains the search needle ("NEEDLE_TOKEN") so SearchContent produces real,
// non-empty match work rather than walking past everything.
const benchSearchFixtureFile = `// synthesized search fixture file %d
package fixture

func helper%d() int {
	x := %d
	y := x * 2
	return y + %d
}

// filler line one for fixture %d
// filler line two for fixture %d
`

const benchSearchNeedleLine = "\n// NEEDLE_TOKEN marker line for fixture %d\n"

// buildSearchFixture synthesizes a text-file tree of fileCount files under a
// fresh temp dir and returns the root path. Every third file embeds the search
// needle. Shared by the benchmark and by bench_helpers_test.go's unit test so a
// broken fixture fails loudly rather than silently producing a no-match
// benchmark.
func buildSearchFixture(tb testing.TB, fileCount int) string {
	tb.Helper()
	root := tb.TempDir()
	for i := 0; i < fileCount; i++ {
		dir := filepath.Join(root, fmt.Sprintf("dir%d", i%6))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			tb.Fatalf("mkdir search fixture dir: %v", err)
		}
		body := fmt.Sprintf(benchSearchFixtureFile, i, i, i, i, i, i)
		if i%3 == 0 {
			body += fmt.Sprintf(benchSearchNeedleLine, i)
		}
		path := filepath.Join(dir, fmt.Sprintf("file%d.txt", i))
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			tb.Fatalf("write search fixture file: %v", err)
		}
	}
	return root
}

// newBenchSearcher builds a FileSearcher rooted at the given workspace.
func newBenchSearcher(tb testing.TB, root string) FileSearcher {
	tb.Helper()
	cfg := DefaultConfig()
	cfg.WorkspaceRoot = root
	fs, err := NewFileSystemTools(cfg)
	if err != nil {
		tb.Fatalf("NewFileSystemTools: %v", err)
	}
	return fs.Searcher()
}

// BenchmarkSearchContent measures a full SearchContent pass over a synthesized
// 60-file text tree — the serial walk+read the agent Grep tool pays today
// (B08). This is the number P2-T05 (parallel search) must move.
func BenchmarkSearchContent(b *testing.B) {
	root := buildSearchFixture(b, 60)
	searcher := newBenchSearcher(b, root)
	ctx := context.Background()
	opts := ContentSearchOptions{
		Root:    root,
		Pattern: "NEEDLE_TOKEN",
		IsRegex: false,
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		matches, err := searcher.SearchContent(ctx, opts)
		if err != nil {
			b.Fatalf("SearchContent: %v", err)
		}
		if len(matches) == 0 {
			b.Fatal("SearchContent found zero matches over a fixture that embeds the needle")
		}
	}
}

// BenchmarkSearchContent_Regex measures the same pass with a regex pattern —
// surfaces the per-file regexp evaluation cost on top of the walk+read.
func BenchmarkSearchContent_Regex(b *testing.B) {
	root := buildSearchFixture(b, 60)
	searcher := newBenchSearcher(b, root)
	ctx := context.Background()
	opts := ContentSearchOptions{
		Root:    root,
		Pattern: `NEEDLE_[A-Z]+`,
		IsRegex: true,
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		matches, err := searcher.SearchContent(ctx, opts)
		if err != nil {
			b.Fatalf("SearchContent: %v", err)
		}
		if len(matches) == 0 {
			b.Fatal("SearchContent (regex) found zero matches")
		}
	}
}
