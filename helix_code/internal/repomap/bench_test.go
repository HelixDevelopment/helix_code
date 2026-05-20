package repomap

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// bench_test.go — speed-programme Phase 0 task P0-T02.
//
// Baseline benchmarks for the repo-map build hot path (R1 bottlenecks B04-B07 —
// serial parse, per-file parser allocation, double-parse, per-Set cache
// goroutine). The repo-map build is the CPU bottleneck on cold context
// assembly; these benchmarks make Phase 2's parallelism + parser-pool +
// content-addressed-cache work (P2-T03/P2-T04/P2-T06) falsifiable with pasted
// before/after numbers (CONST-035 / Rule 9). No production code is changed by
// this file.
//
// P0-T04's helix_code/tests/performance/scenarios/ generator had not landed at
// the time this task was implemented, so the fixture tree is synthesized inline
// into b.TempDir().
//
// Run: go test -bench=. -benchmem -run=^$ ./internal/repomap/

// benchFixtureFile is a small but symbol-rich Go source body. The %d
// substitution keeps every generated file's symbol names unique so the
// tree-sitter tag extractor does real work per file.
const benchFixtureFile = `package fixture%d

import (
	"context"
	"fmt"
)

// Service%d is a synthesized fixture type for repo-map benchmarking.
type Service%d struct {
	name  string
	count int
}

// NewService%d constructs a Service%d.
func NewService%d(name string) *Service%d {
	return &Service%d{name: name}
}

// Process%d runs the fixture's main work loop.
func (s *Service%d) Process%d(ctx context.Context, input string) (string, error) {
	if input == "" {
		return "", fmt.Errorf("empty input for %%s", s.name)
	}
	s.count++
	return fmt.Sprintf("processed-%%d-%%s", s.count, input), nil
}

// Validate%d checks the fixture invariants.
func (s *Service%d) Validate%d() bool {
	return s.name != "" && s.count >= 0
}
`

// buildRepomapFixture synthesizes a small Go source tree of fileCount files
// under a fresh temp dir and returns the root path. Shared by the benchmarks
// and by bench_helpers_test.go's unit test so a broken fixture fails loudly
// rather than silently producing a zero-file benchmark.
func buildRepomapFixture(tb testing.TB, fileCount int) string {
	tb.Helper()
	root := tb.TempDir()
	for i := 0; i < fileCount; i++ {
		// Spread files across a few packages/subdirs for realism.
		dir := filepath.Join(root, fmt.Sprintf("pkg%d", i%5))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			tb.Fatalf("mkdir fixture dir: %v", err)
		}
		path := filepath.Join(dir, fmt.Sprintf("service%d.go", i))
		// 16 verbs in benchFixtureFile — supply i for every one.
		args := make([]interface{}, 16)
		for j := range args {
			args[j] = i
		}
		body := fmt.Sprintf(benchFixtureFile, args...)
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			tb.Fatalf("write fixture file: %v", err)
		}
	}
	return root
}

// BenchmarkRepoMapBuild measures cold repo-map construction + statistics over a
// synthesized 30-file source tree. Cache is disabled so every run does the full
// discover + parse + symbol-extract work — the serial-parse cost (B04) Phase 2
// parallelises. GetStatistics exercises the second-pass scan (B05).
func BenchmarkRepoMapBuild(b *testing.B) {
	root := buildRepomapFixture(b, 30)
	cfg := DefaultConfig()
	cfg.CacheEnabled = false // measure the cold parse path, not cache hits.

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rm, err := NewRepoMap(root, cfg)
		if err != nil {
			b.Fatalf("NewRepoMap: %v", err)
		}
		stats, err := rm.GetStatistics()
		if err != nil {
			b.Fatalf("GetStatistics: %v", err)
		}
		if stats.TotalFiles == 0 {
			b.Fatal("repo-map saw zero files")
		}
	}
}

// BenchmarkRepoMapOptimalContext measures the query path: GetOptimalContext
// ranks + assembles file contexts for a query. This is the per-turn context
// build the agent loop pays — exercises symbol extraction + ranking.
func BenchmarkRepoMapOptimalContext(b *testing.B) {
	root := buildRepomapFixture(b, 30)
	cfg := DefaultConfig()
	cfg.CacheEnabled = false
	rm, err := NewRepoMap(root, cfg)
	if err != nil {
		b.Fatalf("NewRepoMap: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		contexts, err := rm.GetOptimalContext("Process input service", nil)
		if err != nil {
			b.Fatalf("GetOptimalContext: %v", err)
		}
		_ = contexts
	}
}
