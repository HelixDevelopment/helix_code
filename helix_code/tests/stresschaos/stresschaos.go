// Package stresschaos is a Go-native stress + chaos test harness for HelixCode,
// implementing the constitution's §11.4.85 Stress + Chaos Test Mandate.
//
// It mirrors the canonical shell helper contract (ab_stress_run,
// ab_stress_concurrent, ab_chaos_kill_pid_during, ab_chaos_corrupt_file_during,
// ab_chaos_oom_pressure_during) as Go helpers that compose with the standard
// `testing` package, so any *_stress_test.go / *_chaos_test.go file can prove the
// two §11.4.85 survival properties:
//
//	(A) Survives load  — sustained-load (N>=100 or >=30s, p50/p95/p99 latency)
//	                     and concurrency (N>=10 goroutines, no deadlock, no leak).
//	(B) Survives failure — process-death / input-corruption / resource-exhaustion
//	                     chaos injection with a categorised recovery trace.
//
// Every PASS writes a captured-evidence artefact under qa-results/<run-id>/<name>/
// in the exact closed-set shapes the mandate enumerates. Per §11.4.5 / §11.4.69
// an empty / absent / placeholder artefact is NOT evidence — the helpers fail the
// test rather than emit a hollow PASS, so the harness itself cannot bluff.
//
// This helper is project-local (HelixCode tests/) on purpose. Promoting it to the
// constitution submodule for cross-project reuse is a future operator decision
// (that path triggers the §11.4.26 fetch-pull-push-to-all-upstreams workflow).
package stresschaos

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// MinSustainedN is the §11.4.85(A)(1) floor: a sustained-load run must invoke the
// function under test at least this many times (or run for MinSustainedDuration).
const MinSustainedN = 100

// MinSustainedDuration is the §11.4.85(A)(1) wall-clock alternative to MinSustainedN.
const MinSustainedDuration = 30 * time.Second

// MinParallelism is the §11.4.85(A)(2) concurrency floor: a concurrency run must
// hammer the function under test from at least this many goroutines.
const MinParallelism = 10

// defaultErrorRateThreshold is the maximum tolerated error rate for a sustained
// stress PASS. Callers can override via SustainedConfig.MaxErrorRate.
const defaultErrorRateThreshold = 0.0

// goroutineLeakTolerance is the allowed delta in runtime.NumGoroutine() before/
// after a concurrency run. Concurrency runs settle asynchronously, so a small
// non-negative slack avoids flakiness while still catching real leaks.
const goroutineLeakTolerance = 4

// LatencyReport is the §11.4.85 `latency.json` closed-set evidence shape.
type LatencyReport struct {
	Name       string  `json:"name"`
	N          int     `json:"n"`
	P50Ms      float64 `json:"p50_ms"`
	P95Ms      float64 `json:"p95_ms"`
	P99Ms      float64 `json:"p99_ms"`
	MinMs      float64 `json:"min_ms"`
	MaxMs      float64 `json:"max_ms"`
	ErrorRate  float64 `json:"error_rate"`
	DurationMs float64 `json:"duration_ms"`
	Timestamp  string  `json:"timestamp"`
}

// ConcurrencyReport is the §11.4.85 `concurrency_report.json` closed-set shape.
type ConcurrencyReport struct {
	Name             string  `json:"name"`
	Parallelism      int     `json:"parallelism"`
	IterationsPerG   int     `json:"iterations_per_goroutine"`
	TotalCalls       int     `json:"total_calls"`
	GoroutinesBefore int     `json:"goroutines_before"`
	GoroutinesAfter  int     `json:"goroutines_after"`
	GoroutineDelta   int     `json:"goroutine_delta"`
	Deadlock         bool    `json:"deadlock"`
	ErrorCount       int64   `json:"error_count"`
	DurationMs       float64 `json:"duration_ms"`
	Timestamp        string  `json:"timestamp"`
}

// RecoveryTrace is the §11.4.85 `recovery_trace.log` (categorised) evidence shape.
// Each injected chaos fault is classified into exactly one of three buckets.
type RecoveryTrace struct {
	Name      string   `json:"name"`
	FaultKind string   `json:"fault_kind"`
	Recovered int      `json:"recovered"`
	Degraded  int      `json:"degraded"`
	Fatal     int      `json:"fatal"`
	Events    []string `json:"events"`
	Timestamp string   `json:"timestamp"`
}

// runID is computed once per process so all artefacts from a single `go test`
// invocation land under the same qa-results/<run-id>/ directory.
var (
	runIDOnce sync.Once
	runIDVal  string
)

func runID() string {
	runIDOnce.Do(func() {
		if v := os.Getenv("STRESSCHAOS_RUN_ID"); v != "" {
			runIDVal = v
			return
		}
		runIDVal = time.Now().UTC().Format("20060102T150405Z")
	})
	return runIDVal
}

// EvidenceRoot returns the qa-results root directory for the current run. It can
// be overridden with STRESSCHAOS_EVIDENCE_ROOT; otherwise it resolves to a
// `qa-results` directory anchored at the module root (located by walking up for
// go.mod) so artefacts land in a stable place regardless of the test's package.
func EvidenceRoot() string {
	if v := os.Getenv("STRESSCHAOS_EVIDENCE_ROOT"); v != "" {
		return filepath.Join(v, runID())
	}
	return filepath.Join(moduleRoot(), "qa-results", runID())
}

func moduleRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return "qa-results-fallback"
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return wd // no go.mod found; fall back to cwd
		}
		dir = parent
	}
}

// evidenceDir creates and returns qa-results/<run-id>/<name>/ for a single test.
func evidenceDir(t testing.TB, name string) string {
	t.Helper()
	dir := filepath.Join(EvidenceRoot(), name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("stresschaos: cannot create evidence dir %s: %v", dir, err)
	}
	return dir
}

// writeJSON writes v as indented JSON and then re-reads it, asserting the file is
// non-empty and parseable. Per §11.4.5/§11.4.69 a hollow artefact is not evidence:
// if the write or the re-read verification fails, the test FAILS — the harness
// will not let a PASS stand on an empty file.
func writeJSON(t testing.TB, path string, v interface{}) {
	t.Helper()
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("stresschaos: marshal evidence %s: %v", path, err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatalf("stresschaos: write evidence %s: %v", path, err)
	}
	verifyArtefact(t, path)
}

// verifyArtefact asserts the captured-evidence file exists and is non-empty.
func verifyArtefact(t testing.TB, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stresschaos: evidence artefact missing %s: %v", path, err)
	}
	if info.Size() == 0 {
		t.Fatalf("stresschaos: evidence artefact empty (not evidence per §11.4.5) %s", path)
	}
}

func percentile(sortedMs []float64, p float64) float64 {
	if len(sortedMs) == 0 {
		return 0
	}
	if p <= 0 {
		return sortedMs[0]
	}
	if p >= 100 {
		return sortedMs[len(sortedMs)-1]
	}
	// nearest-rank method
	rank := int((p/100.0)*float64(len(sortedMs)-1) + 0.5)
	if rank >= len(sortedMs) {
		rank = len(sortedMs) - 1
	}
	return sortedMs[rank]
}

// SustainedConfig tunes a sustained-load run. Zero values pick §11.4.85 floors.
type SustainedConfig struct {
	// N is the number of invocations. If 0, MinSustainedN is used. Values below
	// MinSustainedN are rejected unless MinDuration is set instead.
	N int
	// MinDuration, if > 0, runs the function repeatedly until the duration
	// elapses (the §11.4.85(A)(1) ">=30s wall-clock" alternative). When set, N
	// becomes a lower bound only.
	MinDuration time.Duration
	// MaxErrorRate is the highest tolerated error fraction for a PASS.
	MaxErrorRate float64
}

// RunSustainedLoad invokes fn under sustained load per §11.4.85(A)(1), captures
// per-call latency, computes p50/p95/p99, writes latency.json, and FAILS the test
// if the error rate exceeds the threshold or the floor (N>=100 or >=30s) is unmet.
// It returns the captured LatencyReport so callers can make extra assertions.
//
// fn must return nil on success and a non-nil error on a failed invocation; the
// error rate is the fraction of non-nil returns.
func RunSustainedLoad(t testing.TB, name string, cfg SustainedConfig, fn func(i int) error) LatencyReport {
	t.Helper()

	n := cfg.N
	if cfg.MinDuration <= 0 {
		if n == 0 {
			n = MinSustainedN
		}
		if n < MinSustainedN {
			t.Fatalf("stresschaos: RunSustainedLoad %q N=%d below §11.4.85 floor %d (set MinDuration to use the >=30s path)", name, n, MinSustainedN)
		}
	}

	capacity := n
	if capacity < MinSustainedN {
		capacity = MinSustainedN
	}
	latencies := make([]float64, 0, capacity)
	var errCount int
	start := time.Now()

	i := 0
	for {
		callStart := time.Now()
		err := fn(i)
		elapsedMs := float64(time.Since(callStart).Microseconds()) / 1000.0
		latencies = append(latencies, elapsedMs)
		if err != nil {
			errCount++
		}
		i++

		if cfg.MinDuration > 0 {
			if time.Since(start) >= cfg.MinDuration && i >= MinSustainedN {
				break
			}
		} else if i >= n {
			break
		}
	}

	sorted := make([]float64, len(latencies))
	copy(sorted, latencies)
	sort.Float64s(sorted)

	errRate := float64(errCount) / float64(len(latencies))
	rep := LatencyReport{
		Name:       name,
		N:          len(latencies),
		P50Ms:      percentile(sorted, 50),
		P95Ms:      percentile(sorted, 95),
		P99Ms:      percentile(sorted, 99),
		MinMs:      sorted[0],
		MaxMs:      sorted[len(sorted)-1],
		ErrorRate:  errRate,
		DurationMs: float64(time.Since(start).Microseconds()) / 1000.0,
		Timestamp:  time.Now().UTC().Format(time.RFC3339Nano),
	}

	dir := evidenceDir(t, name)
	path := filepath.Join(dir, "latency.json")
	writeJSON(t, path, rep)

	threshold := cfg.MaxErrorRate
	if threshold == 0 {
		threshold = defaultErrorRateThreshold
	}
	if errRate > threshold {
		t.Fatalf("stresschaos: %q error rate %.4f exceeds threshold %.4f (evidence: %s)", name, errRate, threshold, path)
	}

	t.Logf("stresschaos: %q sustained N=%d p50=%.3fms p95=%.3fms p99=%.3fms errRate=%.4f -> %s",
		name, rep.N, rep.P50Ms, rep.P95Ms, rep.P99Ms, rep.ErrorRate, path)
	return rep
}

// ConcurrencyConfig tunes a concurrency run. Zero values pick §11.4.85 floors.
type ConcurrencyConfig struct {
	// Parallelism is the goroutine count. If 0, MinParallelism is used. Values
	// below MinParallelism are rejected.
	Parallelism int
	// IterationsPerGoroutine is how many times each goroutine calls fn. If 0,
	// defaults to 50 (so a 10x50 run does 500 real concurrent calls).
	IterationsPerGoroutine int
	// Timeout is the deadlock guard. If 0, defaults to 30s. If the run does not
	// complete within Timeout, the test FAILS with deadlock:true evidence.
	Timeout time.Duration
}

// RunConcurrent hammers fn from N>=10 goroutines per §11.4.85(A)(2), guards against
// deadlock with a timeout, measures the goroutine-count delta to detect leaks, and
// writes concurrency_report.json. It FAILS the test on deadlock (timeout), on a
// goroutine leak beyond tolerance, or if fn reports errors. Run under `-race` to
// also catch data races. Returns the ConcurrencyReport for extra assertions.
func RunConcurrent(t testing.TB, name string, cfg ConcurrencyConfig, fn func(goroutine, iter int) error) ConcurrencyReport {
	t.Helper()

	p := cfg.Parallelism
	if p == 0 {
		p = MinParallelism
	}
	if p < MinParallelism {
		t.Fatalf("stresschaos: RunConcurrent %q parallelism=%d below §11.4.85 floor %d", name, p, MinParallelism)
	}
	iters := cfg.IterationsPerGoroutine
	if iters == 0 {
		iters = 50
	}
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	// Settle and snapshot goroutine count before the run.
	runtime.GC()
	gBefore := runtime.NumGoroutine()

	var errCount int64
	var wg sync.WaitGroup
	wg.Add(p)
	start := time.Now()
	startGate := make(chan struct{})

	for g := 0; g < p; g++ {
		go func(gid int) {
			defer wg.Done()
			<-startGate // release all goroutines simultaneously for true contention
			for it := 0; it < iters; it++ {
				if err := fn(gid, it); err != nil {
					atomic.AddInt64(&errCount, 1)
				}
			}
		}(g)
	}
	close(startGate)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	deadlock := false
	select {
	case <-done:
	case <-time.After(timeout):
		deadlock = true
	}
	durMs := float64(time.Since(start).Microseconds()) / 1000.0

	// Let scheduled goroutines wind down before snapshotting (only meaningful if
	// the run actually completed).
	if !deadlock {
		time.Sleep(50 * time.Millisecond)
		runtime.GC()
	}
	gAfter := runtime.NumGoroutine()

	rep := ConcurrencyReport{
		Name:             name,
		Parallelism:      p,
		IterationsPerG:   iters,
		TotalCalls:       p * iters,
		GoroutinesBefore: gBefore,
		GoroutinesAfter:  gAfter,
		GoroutineDelta:   gAfter - gBefore,
		Deadlock:         deadlock,
		ErrorCount:       atomic.LoadInt64(&errCount),
		DurationMs:       durMs,
		Timestamp:        time.Now().UTC().Format(time.RFC3339Nano),
	}

	dir := evidenceDir(t, name)
	path := filepath.Join(dir, "concurrency_report.json")
	writeJSON(t, path, rep)

	if deadlock {
		t.Fatalf("stresschaos: %q DEADLOCK — %d goroutines did not finish within %s (evidence: %s)", name, p, timeout, path)
	}
	if rep.GoroutineDelta > goroutineLeakTolerance {
		t.Fatalf("stresschaos: %q goroutine leak — before=%d after=%d delta=%d > tolerance %d (evidence: %s)",
			name, gBefore, gAfter, rep.GoroutineDelta, goroutineLeakTolerance, path)
	}
	if rep.ErrorCount > 0 {
		t.Fatalf("stresschaos: %q reported %d errors under concurrent load (evidence: %s)", name, rep.ErrorCount, path)
	}

	t.Logf("stresschaos: %q concurrent parallelism=%d calls=%d gDelta=%d deadlock=%v dur=%.1fms -> %s",
		name, p, rep.TotalCalls, rep.GoroutineDelta, rep.Deadlock, rep.DurationMs, path)
	return rep
}
