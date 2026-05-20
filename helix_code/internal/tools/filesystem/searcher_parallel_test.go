package filesystem

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// searcher_parallel_test.go — speed-programme Phase 2 task P2-T05.
//
// P2-T05 parallelises SearchContent (R1 bottleneck B08) with a bounded
// errgroup worker pool. The hard constraint is no regression: the parallel
// search MUST return the EXACT SAME result set (same matches, same order
// after sort) as the pre-P2-T05 serial search.
//
// To make that provable rather than asserted, this file embeds a reference
// SERIAL implementation (serialSearchContentReference) that mirrors the
// pre-P2-T05 walk-read-scan loop, and asserts the production parallel
// SearchContent yields a sorted-equal result set over a fixture tree.
//
// Run:
//   go test -race -run TestSearchContent_Parallel ./internal/tools/filesystem/

// serialSearchContentReference reproduces the pre-P2-T05 serial SearchContent
// algorithm: a single-goroutine filepath.WalkDir that reads and scans each
// candidate file inline. It reuses the production filters (matchesContentSearch
// Patterns, searchFileContent, isHidden) so the only thing under test is the
// walk/scan parallelism — not the matching semantics. It deliberately does NOT
// apply MaxMatches (the tests that compare result sets use unbounded options;
// MaxMatches determinism is covered separately).
func serialSearchContentReference(t testing.TB, s *fileSearcher, opts ContentSearchOptions) []ContentMatch {
	t.Helper()

	validationResult, err := s.fs.pathValidator.Validate(opts.Root)
	if err != nil {
		t.Fatalf("reference: validate root: %v", err)
	}
	rootPath := validationResult.NormalizedPath

	var re *regexp.Regexp
	if opts.IsRegex {
		flags := ""
		if !opts.CaseSensitive {
			flags = "(?i)"
		}
		re, err = regexp.Compile(flags + opts.Pattern)
		if err != nil {
			t.Fatalf("reference: compile regex: %v", err)
		}
	}

	var matches []ContentMatch
	err = filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if isHidden(path) {
			return nil
		}
		if opts.MaxFileSize > 0 {
			info, infoErr := d.Info()
			if infoErr != nil {
				return nil
			}
			if info.Size() > opts.MaxFileSize {
				return nil
			}
		}
		if !s.matchesContentSearchPatterns(path, opts) {
			return nil
		}
		fileMatches, scanErr := s.searchFileContent(path, opts, re)
		if scanErr != nil {
			return nil
		}
		matches = append(matches, fileMatches...)
		return nil
	})
	if err != nil {
		t.Fatalf("reference: walk: %v", err)
	}
	return matches
}

// sortedContentMatchesCopy returns a deterministically-sorted copy of the
// match slice (does not mutate the input), so the reference (walk-order) result
// can be compared against the production (already-sorted) result.
func sortedContentMatchesCopy(in []ContentMatch) []ContentMatch {
	out := make([]ContentMatch, len(in))
	copy(out, in)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		if out[i].LineNumber != out[j].LineNumber {
			return out[i].LineNumber < out[j].LineNumber
		}
		return out[i].ColumnNumber < out[j].ColumnNumber
	})
	return out
}

// contentMatchKey is a comparable digest of a ContentMatch used for set
// equality. It includes every user-visible field including the surrounding
// context lines, so a difference anywhere fails the test.
func contentMatchKey(m ContentMatch) string {
	return fmt.Sprintf("%s\x00%d\x00%d\x00%s\x00%s\x00%s",
		m.Path, m.LineNumber, m.ColumnNumber, m.Line, m.Match,
		strings.Join(m.Context, "\x01"))
}

// assertContentMatchSetsEqual fails the test unless the two slices contain the
// exact same matches in the exact same order (after deterministic sorting).
func assertContentMatchSetsEqual(t *testing.T, label string, got, want []ContentMatch) {
	t.Helper()
	gs := sortedContentMatchesCopy(got)
	ws := sortedContentMatchesCopy(want)
	if len(gs) != len(ws) {
		t.Fatalf("%s: result-set size mismatch — parallel=%d serial=%d", label, len(gs), len(ws))
	}
	for i := range gs {
		if contentMatchKey(gs[i]) != contentMatchKey(ws[i]) {
			t.Fatalf("%s: result[%d] mismatch\n parallel: %+v\n serial:   %+v", label, i, gs[i], ws[i])
		}
	}
}

// buildParallelSearchFixture synthesizes a representative source tree for the
// P2-T05 tests. It deliberately mixes: nested directories, hidden files (must
// be skipped), .go and .txt files, multiple matches per file, files with no
// match, and a binary-ish large file used to exercise the MaxFileSize filter.
func buildParallelSearchFixture(tb testing.TB, fileCount int) string {
	tb.Helper()
	root := tb.TempDir()
	for i := 0; i < fileCount; i++ {
		dir := filepath.Join(root, fmt.Sprintf("pkg%d", i%5), fmt.Sprintf("sub%d", i%3))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			tb.Fatalf("mkdir fixture dir: %v", err)
		}
		var b strings.Builder
		fmt.Fprintf(&b, "// fixture file %d\npackage fixture\n", i)
		// Every other file embeds the needle, some twice.
		if i%2 == 0 {
			fmt.Fprintf(&b, "// FINDME first occurrence in file %d\n", i)
		}
		fmt.Fprintf(&b, "func F%d() int { return %d }\n", i, i)
		if i%4 == 0 {
			fmt.Fprintf(&b, "// FINDME second occurrence in file %d\n", i)
		}
		fmt.Fprintf(&b, "// trailing line for file %d\n", i)

		ext := ".txt"
		if i%3 == 0 {
			ext = ".go"
		}
		path := filepath.Join(dir, fmt.Sprintf("file%d%s", i, ext))
		if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
			tb.Fatalf("write fixture file: %v", err)
		}
	}
	// A hidden file that DOES contain the needle — must be skipped by both
	// the serial reference and the parallel production path.
	if err := os.WriteFile(filepath.Join(root, ".hidden_findme.txt"),
		[]byte("// FINDME hidden — should never appear in results\n"), 0o644); err != nil {
		tb.Fatalf("write hidden fixture file: %v", err)
	}
	return root
}

// newParallelTestSearcher builds the concrete *fileSearcher rooted at root so
// the reference implementation can reuse its unexported helpers.
func newParallelTestSearcher(tb testing.TB, root string) *fileSearcher {
	tb.Helper()
	cfg := DefaultConfig()
	cfg.WorkspaceRoot = root
	fsTools, err := NewFileSystemTools(cfg)
	if err != nil {
		tb.Fatalf("NewFileSystemTools: %v", err)
	}
	fs := fsTools.Searcher()
	concrete, ok := fs.(*fileSearcher)
	if !ok {
		tb.Fatalf("Searcher() is %T, want *fileSearcher", fs)
	}
	return concrete
}

// TestSearchContent_ParallelEqualsSerial proves the no-regression contract:
// the production (parallel) SearchContent returns the EXACT same result set as
// the embedded serial reference, across several search option combinations.
func TestSearchContent_ParallelEqualsSerial(t *testing.T) {
	root := buildParallelSearchFixture(t, 120)
	s := newParallelTestSearcher(t, root)
	ctx := context.Background()

	cases := []struct {
		name string
		opts ContentSearchOptions
	}{
		{
			name: "literal-substring",
			opts: ContentSearchOptions{Root: root, Pattern: "FINDME"},
		},
		{
			name: "literal-case-insensitive",
			opts: ContentSearchOptions{Root: root, Pattern: "findme", CaseSensitive: false},
		},
		{
			name: "literal-case-sensitive-no-hit",
			opts: ContentSearchOptions{Root: root, Pattern: "findme", CaseSensitive: true},
		},
		{
			name: "regex",
			opts: ContentSearchOptions{Root: root, Pattern: `FIND[A-Z]+`, IsRegex: true},
		},
		{
			name: "regex-case-insensitive",
			opts: ContentSearchOptions{Root: root, Pattern: `find[a-z]+`, IsRegex: true, CaseSensitive: false},
		},
		{
			name: "with-context-lines",
			opts: ContentSearchOptions{Root: root, Pattern: "FINDME", ContextLines: 2},
		},
		{
			name: "include-go-only",
			opts: ContentSearchOptions{Root: root, Pattern: "FINDME", IncludeFiles: []string{"*.go"}},
		},
		{
			name: "exclude-txt",
			opts: ContentSearchOptions{Root: root, Pattern: "FINDME", ExcludeFiles: []string{"*.txt"}},
		},
		{
			name: "func-pattern",
			opts: ContentSearchOptions{Root: root, Pattern: `func F\d+`, IsRegex: true},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			parallel, err := s.SearchContent(ctx, tc.opts)
			if err != nil {
				t.Fatalf("parallel SearchContent: %v", err)
			}
			serial := serialSearchContentReference(t, s, tc.opts)
			assertContentMatchSetsEqual(t, tc.name, parallel, serial)
		})
	}
}

// TestSearchContent_ParallelDeterministicOrdering runs the same parallel search
// 10 times and asserts every run produces a byte-identical ordered result —
// proving the sort makes output independent of goroutine scheduling.
func TestSearchContent_ParallelDeterministicOrdering(t *testing.T) {
	root := buildParallelSearchFixture(t, 150)
	s := newParallelTestSearcher(t, root)
	ctx := context.Background()
	opts := ContentSearchOptions{Root: root, Pattern: "FINDME", ContextLines: 1}

	first, err := s.SearchContent(ctx, opts)
	if err != nil {
		t.Fatalf("run 0: %v", err)
	}
	if len(first) == 0 {
		t.Fatal("fixture embeds the needle but SearchContent found nothing")
	}
	firstKeys := make([]string, len(first))
	for i, m := range first {
		firstKeys[i] = contentMatchKey(m)
	}

	for run := 1; run < 10; run++ {
		got, err := s.SearchContent(ctx, opts)
		if err != nil {
			t.Fatalf("run %d: %v", run, err)
		}
		if len(got) != len(first) {
			t.Fatalf("run %d: result count %d != run 0 count %d", run, len(got), len(first))
		}
		for i, m := range got {
			if contentMatchKey(m) != firstKeys[i] {
				t.Fatalf("run %d: result[%d] differs from run 0 — ordering is non-deterministic", run, i)
			}
		}
	}
}

// TestSearchContent_ParallelResultsAreSorted asserts the production result is
// already sorted by (path, line, column) — operators consume it directly.
func TestSearchContent_ParallelResultsAreSorted(t *testing.T) {
	root := buildParallelSearchFixture(t, 90)
	s := newParallelTestSearcher(t, root)
	ctx := context.Background()

	got, err := s.SearchContent(ctx, ContentSearchOptions{Root: root, Pattern: "FINDME"})
	if err != nil {
		t.Fatalf("SearchContent: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("expected matches over fixture")
	}
	for i := 1; i < len(got); i++ {
		prev, cur := got[i-1], got[i]
		if prev.Path > cur.Path {
			t.Fatalf("result not sorted by path at index %d: %q > %q", i, prev.Path, cur.Path)
		}
		if prev.Path == cur.Path && prev.LineNumber > cur.LineNumber {
			t.Fatalf("result not sorted by line within %q at index %d: %d > %d",
				prev.Path, i, prev.LineNumber, cur.LineNumber)
		}
	}
}

// TestSearchContent_ParallelHiddenFilesSkipped proves the hidden-file filter
// still holds after parallelisation — the .hidden_findme.txt that contains the
// needle must NOT appear in the result set.
func TestSearchContent_ParallelHiddenFilesSkipped(t *testing.T) {
	root := buildParallelSearchFixture(t, 30)
	s := newParallelTestSearcher(t, root)
	ctx := context.Background()

	got, err := s.SearchContent(ctx, ContentSearchOptions{Root: root, Pattern: "FINDME"})
	if err != nil {
		t.Fatalf("SearchContent: %v", err)
	}
	for _, m := range got {
		if strings.Contains(filepath.Base(m.Path), ".hidden") {
			t.Fatalf("hidden file leaked into results: %s", m.Path)
		}
	}
}

// TestSearchContent_ParallelMaxMatchesDeterministic asserts MaxMatches caps the
// result set and that the capped set is deterministic (the first N of the
// sorted set) across repeated runs.
func TestSearchContent_ParallelMaxMatchesDeterministic(t *testing.T) {
	root := buildParallelSearchFixture(t, 120)
	s := newParallelTestSearcher(t, root)
	ctx := context.Background()

	const cap = 17
	opts := ContentSearchOptions{Root: root, Pattern: "FINDME", MaxMatches: cap}

	first, err := s.SearchContent(ctx, opts)
	if err != nil {
		t.Fatalf("SearchContent: %v", err)
	}
	if len(first) != cap {
		t.Fatalf("MaxMatches=%d but got %d matches", cap, len(first))
	}
	// The capped set must be the first `cap` of the full sorted set.
	full, err := s.SearchContent(ctx, ContentSearchOptions{Root: root, Pattern: "FINDME"})
	if err != nil {
		t.Fatalf("SearchContent (full): %v", err)
	}
	if len(full) < cap {
		t.Fatalf("fixture produced only %d matches, need >= %d to test the cap", len(full), cap)
	}
	for i := 0; i < cap; i++ {
		if contentMatchKey(first[i]) != contentMatchKey(full[i]) {
			t.Fatalf("capped result[%d] is not the i-th of the full sorted set", i)
		}
	}
	// Determinism across runs.
	for run := 1; run < 10; run++ {
		got, err := s.SearchContent(ctx, opts)
		if err != nil {
			t.Fatalf("run %d: %v", run, err)
		}
		if len(got) != cap {
			t.Fatalf("run %d: capped count %d != %d", run, len(got), cap)
		}
		for i := range got {
			if contentMatchKey(got[i]) != contentMatchKey(first[i]) {
				t.Fatalf("run %d: capped result[%d] differs — non-deterministic cap", run, i)
			}
		}
	}
}

// TestSearchContent_ParallelEmptyTree exercises the early-return path: a tree
// with no candidate files must yield a nil/empty result and no error.
func TestSearchContent_ParallelEmptyTree(t *testing.T) {
	root := t.TempDir()
	s := newParallelTestSearcher(t, root)
	got, err := s.SearchContent(context.Background(), ContentSearchOptions{Root: root, Pattern: "anything"})
	if err != nil {
		t.Fatalf("SearchContent over empty tree: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected zero matches over empty tree, got %d", len(got))
	}
}

// TestSearchContent_ParallelContextLinesPreserved asserts the surrounding
// context lines are captured identically by the parallel path and the serial
// reference (context buffer logic runs per-file, so concurrency must not
// disturb it).
func TestSearchContent_ParallelContextLinesPreserved(t *testing.T) {
	root := buildParallelSearchFixture(t, 60)
	s := newParallelTestSearcher(t, root)
	ctx := context.Background()
	opts := ContentSearchOptions{Root: root, Pattern: "FINDME", ContextLines: 3}

	parallel, err := s.SearchContent(ctx, opts)
	if err != nil {
		t.Fatalf("parallel SearchContent: %v", err)
	}
	serial := serialSearchContentReference(t, s, opts)
	assertContentMatchSetsEqual(t, "context-lines", parallel, serial)

	// At least one match must carry context lines (sanity — proves we are not
	// vacuously asserting equality of two empty sets).
	withContext := 0
	for _, m := range parallel {
		if len(m.Context) > 0 {
			withContext++
		}
	}
	if withContext == 0 {
		t.Fatal("ContextLines=3 but no match carried context — test would be vacuous")
	}
}

// --- Benchmarks: S3-style content-search wall-clock, serial vs parallel. ---
//
// BenchmarkSearchContent_S3_Parallel measures the production (parallel)
// SearchContent over a large fixture tree. Pair it with the existing
// BenchmarkSearchContent in bench_test.go (60-file tree) and with
// BenchmarkSearchContent_S3_SerialReference below to read the speedup delta.
//
// Run:
//   go test -bench=S3 -benchmem -run=^$ ./internal/tools/filesystem/

const s3BenchFileCount = 2000

// BenchmarkSearchContent_S3_Parallel — production parallel path over a large tree.
func BenchmarkSearchContent_S3_Parallel(b *testing.B) {
	root := buildParallelSearchFixture(b, s3BenchFileCount)
	s := newParallelTestSearcher(b, root)
	ctx := context.Background()
	opts := ContentSearchOptions{Root: root, Pattern: "FINDME"}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		matches, err := s.SearchContent(ctx, opts)
		if err != nil {
			b.Fatalf("SearchContent: %v", err)
		}
		if len(matches) == 0 {
			b.Fatal("S3 benchmark found zero matches over a fixture that embeds the needle")
		}
	}
}

// BenchmarkSearchContent_S3_SerialReference — the pre-P2-T05 serial walk-scan
// over the SAME large tree, so the wall-clock delta vs the parallel benchmark
// is the P2-T05 speedup (anti-bluff: the before number, CONST-035 / Rule 9).
func BenchmarkSearchContent_S3_SerialReference(b *testing.B) {
	root := buildParallelSearchFixture(b, s3BenchFileCount)
	s := newParallelTestSearcher(b, root)
	opts := ContentSearchOptions{Root: root, Pattern: "FINDME"}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		matches := benchSerialSearchContent(b, s, opts)
		if len(matches) == 0 {
			b.Fatal("serial reference found zero matches")
		}
	}
}

// buildLargeFileSearchFixture synthesizes a tree of fewer but LARGER files
// (~400 lines each), so the per-file regex/substring scan — the CPU-bound
// work the worker pool parallelises — dominates over the directory walk. This
// is the scenario closest to grep over a real source repo and where the
// P2-T05 speedup is most pronounced.
func buildLargeFileSearchFixture(tb testing.TB, fileCount, linesPerFile int) string {
	tb.Helper()
	root := tb.TempDir()
	for i := 0; i < fileCount; i++ {
		dir := filepath.Join(root, fmt.Sprintf("pkg%d", i%8))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			tb.Fatalf("mkdir large fixture dir: %v", err)
		}
		var b strings.Builder
		fmt.Fprintf(&b, "// large fixture file %d\npackage fixture\n", i)
		for ln := 0; ln < linesPerFile; ln++ {
			// One needle per file keeps match work non-zero; the rest is
			// non-matching body the scanner still has to read line-by-line.
			if ln == linesPerFile/2 && i%3 == 0 {
				fmt.Fprintf(&b, "// FINDME line %d of file %d\n", ln, i)
			} else {
				fmt.Fprintf(&b, "var sym%d_%d = compute(%d, %d) // body filler line\n", i, ln, i, ln)
			}
		}
		path := filepath.Join(dir, fmt.Sprintf("file%d.go", i))
		if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
			tb.Fatalf("write large fixture file: %v", err)
		}
	}
	return root
}

// BenchmarkSearchContent_LargeFiles_Parallel — production parallel path over a
// tree of larger files, where the per-file scan dominates.
func BenchmarkSearchContent_LargeFiles_Parallel(b *testing.B) {
	root := buildLargeFileSearchFixture(b, 400, 400)
	s := newParallelTestSearcher(b, root)
	ctx := context.Background()
	opts := ContentSearchOptions{Root: root, Pattern: `FIND[A-Z]+`, IsRegex: true}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		matches, err := s.SearchContent(ctx, opts)
		if err != nil {
			b.Fatalf("SearchContent: %v", err)
		}
		if len(matches) == 0 {
			b.Fatal("large-file benchmark found zero matches")
		}
	}
}

// BenchmarkSearchContent_LargeFiles_SerialReference — pre-P2-T05 serial scan
// over the SAME large-file tree; the delta vs the parallel benchmark is the
// P2-T05 speedup in the CPU-scan-dominated regime.
func BenchmarkSearchContent_LargeFiles_SerialReference(b *testing.B) {
	root := buildLargeFileSearchFixture(b, 400, 400)
	s := newParallelTestSearcher(b, root)
	opts := ContentSearchOptions{Root: root, Pattern: `FIND[A-Z]+`, IsRegex: true}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		matches := benchSerialSearchContent(b, s, opts)
		if len(matches) == 0 {
			b.Fatal("serial reference found zero matches")
		}
	}
}

// benchSerialSearchContent is a fatal-free serial walk-scan used by the
// serial-reference benchmark (testing.TB.Fatalf in the per-iteration body would
// be valid but the helper keeps the benchmark loop tight).
func benchSerialSearchContent(b *testing.B, s *fileSearcher, opts ContentSearchOptions) []ContentMatch {
	b.Helper()
	validationResult, err := s.fs.pathValidator.Validate(opts.Root)
	if err != nil {
		b.Fatalf("validate: %v", err)
	}
	rootPath := validationResult.NormalizedPath

	var re *regexp.Regexp
	if opts.IsRegex {
		flags := ""
		if !opts.CaseSensitive {
			flags = "(?i)"
		}
		re, err = regexp.Compile(flags + opts.Pattern)
		if err != nil {
			b.Fatalf("compile regex: %v", err)
		}
	}

	var matches []ContentMatch
	_ = filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() || isHidden(path) {
			return nil
		}
		if !s.matchesContentSearchPatterns(path, opts) {
			return nil
		}
		fm, scanErr := s.searchFileContent(path, opts, re)
		if scanErr != nil {
			return nil
		}
		matches = append(matches, fm...)
		return nil
	})
	return matches
}
