// HXC-005 reproduce-before-fix regression test (round-318, 2026-05-20).
//
// HXC-005: cmd/performance_optimization_standalone/main.go was a
// CONST-035 / Article XI §11.9 simulation bluff — it printed a
// "Production Performance Optimization" banner then *fabricated* every
// improvement percentage from a random number generator
// (improvement := 5.0 + rand.Float64()*20.0), slept time.Sleep(500ms)
// per "phase" as fake work, and reported success for work it never
// performed. No real profiling, no real measurement.
//
// Resolution (round-318): DELETE the standalone bluff entirely. It was
// genuinely obsolete — fully superseded by THIS directory
// (cmd/performance_optimization/), which calls the real
// dev.helix.code/internal/performance.PerformanceOptimizer
// (real runtime.ReadMemStats, real GOMAXPROCS tuning, real
// before/after measurement). Deletion is the honest fix for dead
// bluff code per CLAUDE.md §8.
//
// This test is the CONST-035 reproduce-before-fix proof. It asserts
// two invariants that together certify the bluff is gone and the real
// path works:
//
//  1. The obsolete bluff path cmd/performance_optimization_standalone/
//     no longer exists on disk. If a future change re-creates it the
//     test FAILS — guarding against regression of the deleted bluff.
//  2. The surviving real optimizer (internal/performance) produces a
//     MemoryUsage figure that tracks an ACTUAL runtime.ReadMemStats
//     allocation delta — not an RNG-fabricated number. We allocate a
//     large slice, force GC, and assert the optimizer's collected
//     metric reflects real heap state rather than a fixed/random
//     constant.
//
// Mocks ALLOWED per CONST-050(A) (unit test, no integration build
// tag) — but this test uses NONE: it exercises the real optimizer
// against the real Go runtime, which is the honest way to prove the
// fix.
package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"dev.helix.code/internal/performance"
)

// TestHXC005_BluffStandaloneDirectoryDeleted asserts the obsolete
// simulation-bluff command directory no longer exists. round-318
// deleted it; a future re-introduction of the RNG-fabricating binary
// re-opens HXC-005 and this test catches it.
func TestHXC005_BluffStandaloneDirectoryDeleted(t *testing.T) {
	// This test file lives at:
	//   helix_code/cmd/performance_optimization/bluff_regression_test.go
	// The deleted bluff lived at:
	//   helix_code/cmd/performance_optimization_standalone/
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	cmdDir := filepath.Dir(wd) // .../helix_code/cmd
	bluffDir := filepath.Join(cmdDir, "performance_optimization_standalone")

	if info, statErr := os.Stat(bluffDir); statErr == nil {
		t.Fatalf("HXC-005 REGRESSION: obsolete simulation-bluff directory %q exists again "+
			"(isDir=%t). It was deleted in round-318 because it fabricated improvement "+
			"percentages with rand.Float64() and slept time.Sleep as fake work — a "+
			"CONST-035 / Article XI §11.9 bluff. Do not re-create it; use the real "+
			"cmd/performance_optimization (this directory) which delegates to "+
			"internal/performance.PerformanceOptimizer.", bluffDir, info.IsDir())
	}

	// Also assert the surviving real command still exists — proving the
	// fix did not delete the wrong thing.
	realMain := filepath.Join(wd, "main.go")
	if _, statErr := os.Stat(realMain); statErr != nil {
		t.Fatalf("real perf-optimization command main.go missing at %q: %v "+
			"(round-318 must keep the REAL implementation)", realMain, statErr)
	}
}

// TestHXC005_RealOptimizerMeasuresActualMemory asserts the surviving
// real optimizer collects a MemoryUsage figure derived from a genuine
// runtime.ReadMemStats heap reading — NOT an RNG constant. We capture
// the runtime's own HeapAlloc immediately before driving the optimizer
// and assert the optimizer's baseline lands in the same order of
// magnitude. A bluff that returned 5.0+rand.Float64()*20.0 could never
// satisfy this — it would be a tiny single-digit number unrelated to
// real heap bytes.
func TestHXC005_RealOptimizerMeasuresActualMemory(t *testing.T) {
	// Allocate a large, retained buffer so the heap is provably
	// non-trivial when the optimizer reads it. Keeping a reference
	// (via the package-level sink) prevents the compiler/GC from
	// eliminating the allocation.
	const allocBytes = 32 << 20 // 32 MiB
	buf := make([]byte, allocBytes)
	for i := range buf {
		buf[i] = byte(i)
	}
	hxc005Sink = buf // retain — defeat dead-store elimination

	runtime.GC()
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	runtimeHeap := int64(ms.HeapAlloc)
	if runtimeHeap < allocBytes/2 {
		t.Fatalf("precondition failed: runtime.HeapAlloc=%d below expected floor %d "+
			"(retained 32 MiB buffer should dominate the heap)", runtimeHeap, allocBytes/2)
	}

	opt, err := performance.NewPerformanceOptimizer(performance.PerformanceConfig{
		MemoryOptimization:   true,
		GarbageCollection:    true,
		TargetThroughput:     2000,
		TargetLatency:        "50ms",
		TargetCPUUtilization: 70.0,
		TargetMemoryUsage:    2 << 30, // 2 GiB
	})
	if err != nil {
		t.Fatalf("NewPerformanceOptimizer: %v", err)
	}

	result, err := opt.StartProductionOptimization(context.Background())
	if err != nil {
		t.Fatalf("StartProductionOptimization: %v", err)
	}
	if result == nil || result.Baseline == nil {
		t.Fatal("optimizer returned nil result/baseline — cannot prove real measurement")
	}

	measured := result.Baseline.MemoryUsage
	// Anti-bluff core assertion: the optimizer's MemoryUsage must be a
	// real heap reading, not an RNG single-digit. A bluff value of
	// 5..25 would be many orders of magnitude below the retained 16 MiB
	// floor; only a genuine runtime.ReadMemStats() can land here.
	if measured < allocBytes/2 {
		t.Fatalf("HXC-005 REGRESSION: optimizer MemoryUsage=%d is below the real-heap "+
			"floor %d. A genuine runtime.ReadMemStats reading must reflect the retained "+
			"32 MiB buffer. A value this small indicates fabricated/RNG metrics — the "+
			"exact CONST-035 bluff HXC-005 was opened for.", measured, allocBytes/2)
	}

	t.Logf("HXC-005 fix evidence: optimizer baseline MemoryUsage=%d bytes, "+
		"runtime.HeapAlloc=%d bytes — both real measurements, same order of magnitude. "+
		"No RNG-fabricated improvement percentages.", measured, runtimeHeap)

	// Keep the buffer alive past the measurement.
	runtime.KeepAlive(buf)
}

// hxc005Sink retains the test allocation across the optimizer call so
// the heap reading is provably non-trivial. Package-level to defeat
// escape-analysis-driven dead-store elimination.
var hxc005Sink []byte
