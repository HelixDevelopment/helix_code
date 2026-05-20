package main

import (
	"os"
	"testing"
)

// silenceStdout redirects os.Stdout to /dev/null and returns a restore func.
// `config.Load()` (invoked transitively by NewCLI) prints an
// "internal_config_info_using_config_file" notice via fmt.Println to stdout;
// silencing it for the timed loop keeps the captured baseline parseable. The
// benchmark framework writes its own result lines AFTER the benchmark function
// returns, so restoring stdout before return leaves those untouched. No
// production code changes — this is a test-process I/O concern only.
func silenceStdout(b *testing.B) func() {
	b.Helper()
	orig := os.Stdout
	devnull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		b.Fatalf("open %s: %v", os.DevNull, err)
	}
	os.Stdout = devnull
	return func() {
		os.Stdout = orig
		_ = devnull.Close()
	}
}

// bench_test.go — speed-programme Phase 0 task P0-T02.
//
// Baseline benchmarks for the CLI cold-start hot path (R1 bottleneck B01/B02 —
// CLI startup). These benchmarks measure the cost of `NewCLI()` construction so
// that any Phase 1 lazy-startup speedup (P1-T02 / P1-T03) is falsifiable with
// pasted before/after numbers (CONST-035 / Rule 9). No production code is
// changed by this file — it adds only `Benchmark*` functions.
//
// Run: go test -bench=. -benchmem -run=^$ ./cmd/cli/

// BenchmarkNewCLI measures the full `*CLI` construction cost — the cold-start
// path every short CLI command pays before doing any useful work. This is the
// number Phase 1's lazy-startup work (P1-T03) must move.
//
// Construction is benchmarked SERIALLY only. A `b.RunParallel` variant was
// trialled and exposed a real concurrency defect — `NewCLI()` calls
// `config.Load()`, which calls the process-global `viper.SetDefault`; the
// global Viper singleton is not goroutine-safe and concurrent construction
// panics with "concurrent map writes" (viper.go:1492). `NewCLI()` is
// constructed exactly once per process so this is not a supported usage
// pattern; the defect is noted for the speed programme but a parallel
// benchmark would only re-trigger an unfixable-without-production-change panic
// and is therefore deliberately omitted (P0-T02 changes no production code).
func BenchmarkNewCLI(b *testing.B) {
	restore := silenceStdout(b)
	defer restore()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cli := NewCLI()
		if cli == nil {
			b.Fatal("NewCLI returned nil")
		}
	}
}
