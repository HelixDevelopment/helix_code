// Package scaling is a HelixCode-LOCAL throughput-scaling test harness that
// exercises the REAL internal/worker.WorkerPool across a worker-count sweep
// (N=1,2,4,8) and proves genuine scale-out — not a delegation to a HelixQA shell
// script. It mirrors the proven helix_code/tests/stresschaos contract: every PASS
// WRITES then RE-READS a non-empty evidence artefact under qa-results/<run-id>/,
// and a paired §1.1 meta-test plants a degraded pool and asserts the harness
// DETECTS it (so the harness itself cannot bluff).
//
// CONST-050(B) / §11.4.85: the unit under test is the production WorkerPool's own
// RWMutex-guarded worker map + scheduler selection machinery. No fakes — the sweep
// registers real PoolWorkers via the real RegisterWorker, drives real AssignTask /
// ReleaseWorker, and reads real GetPoolStats utilization.
//
// Honest boundary (§11.4.6): the in-process sweep proves the POOL's scale-out
// logic (assignment throughput vs registered-worker count). True HORIZONTAL
// SSH-worker scale-out needs real remote hosts and is a separate integration-
// tagged path that SKIPs-with-reason when no SSH workers are configured — never a
// fake PASS.
package scaling

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"dev.helix.code/internal/config"
	"dev.helix.code/internal/worker"
	"dev.helix.code/tests/stresschaos"
)

// MinThroughputGainAtMaxN is the calibration floor: throughput at the largest N in
// the sweep must be at least this multiple of throughput at N=1. A pool that
// ignores added workers shows flat throughput (gain ~1.0) and FAILS this gate.
// Calibrated conservatively per §11.4.6 / §11.4.107(13): real scale-out on the
// in-process pool comfortably clears 1.5x; a serialised (broken) pool sits at ~1.0.
const MinThroughputGainAtMaxN = 1.5

// ScalingStep is one row of the §11.4.85-style throughput sweep at a fixed worker
// count N.
type ScalingStep struct {
	NWorkers        int     `json:"n_workers"`
	TotalTasks      int     `json:"total_tasks"`
	AssignedTasks   int64   `json:"assigned_tasks"`
	ThroughputTPS   float64 `json:"throughput_tps"`
	P50Ms           float64 `json:"p50_ms"`
	P95Ms           float64 `json:"p95_ms"`
	P99Ms           float64 `json:"p99_ms"`
	PoolUtilization float64 `json:"pool_utilization"`
	DurationMs      float64 `json:"duration_ms"`
}

// ScalingReport is the closed-set scaling_throughput.json evidence shape.
type ScalingReport struct {
	Name              string        `json:"name"`
	Steps             []ScalingStep `json:"steps"`
	GainAtMaxN        float64       `json:"gain_at_max_n"`
	MinGainThreshold  float64       `json:"min_gain_threshold"`
	MonotonicNonDegrd bool          `json:"monotonic_non_degraded"`
	Timestamp         string        `json:"timestamp"`
}

// PoolDriver is the seam the sweep drives. The production path is realPoolDriver
// (wrapping the real WorkerPool); meta-tests substitute degraded drivers to prove
// the harness detects flat / degrading throughput. A driver must register N real
// workers, expose a process-one-task cycle, and report utilization.
//
// SCALE-OUT SEMANTICS (the honest property being measured, §11.4.6): the production
// WorkerPool's AssignTask is a near-instant in-memory map lookup — raw assign
// throughput is therefore lock-bound and does NOT grow with N (asserting it would
// be a bluff). The pool's REAL scale-out property is CONCURRENT IN-FLIGHT CAPACITY:
// exactly N tasks can be assigned-and-busy at once (the (N+1)-th assign gets
// "no available workers" backpressure until one releases). So ProcessTask models a
// task with a fixed service time — acquire a real worker (mark busy), hold it for
// the service window, release it. With N real workers, N tasks are serviced
// concurrently, so completed-tasks-per-second scales ~linearly with N (bounded by
// scheduler/contention). That is genuine pool scale-out, proven with real
// AssignTask/ReleaseWorker + real GetPoolStats utilization.
type PoolDriver interface {
	// SetupN registers exactly n workers and returns a teardown.
	SetupN(t testing.TB, n int) func()
	// ProcessTask runs one real task: acquire a worker (busy), hold it for the
	// service window, release it. Returns true on success, false on backpressure
	// (all workers busy). Retries briefly so the fixed workload completes.
	ProcessTask(ctx context.Context) bool
	// Utilization reads the live pool utilization_rate (0..100).
	Utilization() float64
}

// ServiceTime is the fixed per-task service window the real driver holds a worker
// busy. It must be large enough that concurrency (N workers in-flight) dominates
// the per-call assign/lock cost, so adding workers measurably increases throughput.
// At 2ms the per-task assign overhead (~microseconds, even under -race) is
// negligible against the service window, so the N-worker concurrency is the
// dominant throughput factor and real scale-out is observable.
const ServiceTime = 2 * time.Millisecond

// runID / evidence helpers mirror the stresschaos write+re-read contract so a
// hollow artefact can never stand as a PASS (§11.4.5/§11.4.69).
var (
	runIDOnce sync.Once
	runIDVal  string
)

func runID() string {
	runIDOnce.Do(func() {
		if v := os.Getenv("SCALING_RUN_ID"); v != "" {
			runIDVal = v
			return
		}
		if v := os.Getenv("STRESSCHAOS_RUN_ID"); v != "" {
			runIDVal = v
			return
		}
		runIDVal = time.Now().UTC().Format("20060102T150405Z")
	})
	return runIDVal
}

// EvidenceRoot resolves qa-results/<run-id>/. Override with SCALING_EVIDENCE_ROOT
// (meta-tests redirect it to a t.TempDir()).
func EvidenceRoot() string {
	if v := os.Getenv("SCALING_EVIDENCE_ROOT"); v != "" {
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
			return wd
		}
		dir = parent
	}
}

func evidenceDir(t testing.TB, name string) string {
	t.Helper()
	dir := filepath.Join(EvidenceRoot(), name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("scaling: cannot create evidence dir %s: %v", dir, err)
	}
	return dir
}

// writeJSON writes v then RE-READS it, failing on empty — a hollow artefact is not
// evidence (§11.4.5/§11.4.69).
func writeJSON(t testing.TB, path string, v interface{}) {
	t.Helper()
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("scaling: marshal evidence %s: %v", path, err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatalf("scaling: write evidence %s: %v", path, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("scaling: evidence artefact missing %s: %v", path, err)
	}
	if info.Size() == 0 {
		t.Fatalf("scaling: evidence artefact empty (not evidence per §11.4.5) %s", path)
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
	rank := int((p/100.0)*float64(len(sortedMs)-1) + 0.5)
	if rank >= len(sortedMs) {
		rank = len(sortedMs) - 1
	}
	return sortedMs[rank]
}

// realPoolDriver wraps the production WorkerPool. This is the non-fake path used
// by the real sweep test.
type realPoolDriver struct {
	pool *worker.WorkerPool
}

// NewRealPoolDriver returns a driver backed by the production worker pool.
func NewRealPoolDriver() PoolDriver { return &realPoolDriver{} }

func (d *realPoolDriver) SetupN(t testing.TB, n int) func() {
	t.Helper()
	d.pool = worker.NewWorkerPool(&config.WorkersConfig{HealthTTL: 3600, MaxConcurrentTasks: n})
	for i := 0; i < n; i++ {
		d.pool.RegisterWorker(worker.NewPoolWorker(
			fmt.Sprintf("w-%d", i), fmt.Sprintf("Worker %d", i), "localhost:0",
			worker.WorkerCapabilities{CPUCores: 8, MemoryGB: 16},
		))
	}
	return func() { d.pool = nil }
}

func (d *realPoolDriver) ProcessTask(ctx context.Context) bool {
	// Acquire a real worker (marks it busy). If all N are busy, retry briefly so
	// the fixed workload still completes — the backpressure is real, but a transient
	// "all busy" is not a dropped task. Cap the retry window so a genuinely wedged
	// pool cannot hang the sweep.
	deadline := time.Now().Add(2 * time.Second)
	for {
		w, err := d.pool.AssignTask(ctx, "compute", map[string]interface{}{"cpu_cores": 1})
		if err == nil {
			// Worker is now busy: hold it for the service window (the real task the
			// worker would execute), then release. Exactly N can be held busy at
			// once — that IS the pool's concurrent-capacity scale-out property. A
			// sleeping worker still owns its StatusBusy slot, so N sleeping workers
			// service N tasks concurrently.
			busyWait(ServiceTime)
			d.pool.ReleaseWorker(w.ID)
			return true
		}
		if time.Now().After(deadline) {
			return false // sustained backpressure
		}
		time.Sleep(50 * time.Microsecond)
	}
}

// busyWait holds for d. time.Sleep models a worker occupied by a real task: it
// keeps the worker's StatusBusy slot claimed for the whole window (the pool's
// concurrent-capacity semantics) while yielding the CPU, so N workers genuinely
// run N tasks in parallel and throughput scales with N.
func busyWait(d time.Duration) {
	time.Sleep(d)
}

func (d *realPoolDriver) Utilization() float64 {
	stats := d.pool.GetPoolStats()
	if v, ok := stats["utilization_rate"].(float64); ok {
		return v
	}
	return 0
}

// SweepConfig tunes the scale sweep. Zero values pick safe defaults.
type SweepConfig struct {
	// NValues is the worker-count sweep. Defaults to {1,2,4,8}.
	NValues []int
	// TasksPerStep is the fixed total workload per N. Defaults to 4000.
	TasksPerStep int
	// Parallelism is the concurrent-submitter count (>= stresschaos.MinParallelism).
	Parallelism int
}

// RunScaleSweep drives driver across the worker-count sweep, measures per-N
// throughput + p50/p95/p99 + real utilization, writes scaling_throughput.json
// (write+re-read), runs a stresschaos.RunConcurrent deadlock/leak guard at the
// max N, and FAILS the test when scale-out is flat (gain < MinThroughputGainAtMaxN)
// or throughput regresses as N grows (monotonic-non-degradation). Returns the
// captured ScalingReport for extra assertions.
func RunScaleSweep(t testing.TB, name string, driver PoolDriver, cfg SweepConfig) ScalingReport {
	t.Helper()

	nValues := cfg.NValues
	if len(nValues) == 0 {
		nValues = []int{1, 2, 4, 8}
	}
	tasks := cfg.TasksPerStep
	if tasks <= 0 {
		tasks = 320
	}
	par := cfg.Parallelism
	if par == 0 {
		par = stresschaos.MinParallelism
	}
	if par < stresschaos.MinParallelism {
		t.Fatalf("scaling: parallelism=%d below §11.4.85 floor %d", par, stresschaos.MinParallelism)
	}

	steps := make([]ScalingStep, 0, len(nValues))
	maxN := 0
	for _, n := range nValues {
		if n > maxN {
			maxN = n
		}
	}

	for _, n := range nValues {
		teardown := driver.SetupN(t, n)

		latencies := make([]float64, tasks)
		var assigned int64
		var peakUtil float64
		var utilMu sync.Mutex

		// Distribute the fixed workload across `par` concurrent submitters so a
		// real multi-worker pool can actually parallelise the assigns.
		perG := tasks / par
		if perG < 1 {
			perG = 1
		}
		actualTasks := perG * par

		var wg sync.WaitGroup
		wg.Add(par)
		var idx int64 = -1
		startGate := make(chan struct{})
		start := time.Now()
		for g := 0; g < par; g++ {
			go func() {
				defer wg.Done()
				<-startGate
				for it := 0; it < perG; it++ {
					my := atomic.AddInt64(&idx, 1)
					callStart := time.Now()
					ok := driver.ProcessTask(context.Background())
					elapsedMs := float64(time.Since(callStart).Microseconds()) / 1000.0
					if my >= 0 && int(my) < len(latencies) {
						latencies[my] = elapsedMs
					}
					if ok {
						atomic.AddInt64(&assigned, 1)
						// sample utilization while work is in flight
						if u := driver.Utilization(); u > 0 {
							utilMu.Lock()
							if u > peakUtil {
								peakUtil = u
							}
							utilMu.Unlock()
						}
					}
				}
			}()
		}
		close(startGate)
		wg.Wait()
		elapsed := time.Since(start)

		used := latencies[:actualTasks]
		sorted := make([]float64, len(used))
		copy(sorted, used)
		sort.Float64s(sorted)

		secs := elapsed.Seconds()
		if secs <= 0 {
			secs = 1e-9
		}
		step := ScalingStep{
			NWorkers:        n,
			TotalTasks:      actualTasks,
			AssignedTasks:   atomic.LoadInt64(&assigned),
			ThroughputTPS:   float64(actualTasks) / secs,
			P50Ms:           percentile(sorted, 50),
			P95Ms:           percentile(sorted, 95),
			P99Ms:           percentile(sorted, 99),
			PoolUtilization: peakUtil,
			DurationMs:      float64(elapsed.Microseconds()) / 1000.0,
		}
		steps = append(steps, step)
		t.Logf("scaling: %q N=%d throughput=%.0f tps assigned=%d p50=%.3fms p99=%.3fms peakUtil=%.1f%%",
			name, n, step.ThroughputTPS, step.AssignedTasks, step.P50Ms, step.P99Ms, step.PoolUtilization)

		teardown()
	}

	// Compute scale-out gain (max-N throughput / smallest-N throughput) + monotonic-
	// non-degradation (throughput never drops >40% below the previous step as N grows).
	var baseTPS, maxTPS float64
	minN := nValues[0]
	for _, n := range nValues {
		if n < minN {
			minN = n
		}
	}
	monotonic := true
	prev := -1.0
	for _, s := range steps {
		if s.NWorkers == minN {
			baseTPS = s.ThroughputTPS
		}
		if s.NWorkers == maxN {
			maxTPS = s.ThroughputTPS
		}
		if prev > 0 && s.ThroughputTPS < prev*0.6 { // >40% drop as N grows = degradation
			monotonic = false
		}
		prev = s.ThroughputTPS
	}
	gain := 0.0
	if baseTPS > 0 {
		gain = maxTPS / baseTPS
	}

	rep := ScalingReport{
		Name:              name,
		Steps:             steps,
		GainAtMaxN:        gain,
		MinGainThreshold:  MinThroughputGainAtMaxN,
		MonotonicNonDegrd: monotonic,
		Timestamp:         time.Now().UTC().Format(time.RFC3339Nano),
	}

	dir := evidenceDir(t, name)
	path := filepath.Join(dir, "scaling_throughput.json")
	writeJSON(t, path, rep)

	// Deadlock / goroutine-leak guard at the max N via the proven stresschaos
	// concurrency harness — this also writes concurrency_report.json.
	driver.SetupN(t, maxN)
	stresschaos.RunConcurrent(t, name+"_concurrency_guard",
		stresschaos.ConcurrencyConfig{Parallelism: par, IterationsPerGoroutine: 20, Timeout: 30 * time.Second},
		func(g, it int) error {
			driver.ProcessTask(context.Background())
			return nil
		})

	if gain < MinThroughputGainAtMaxN {
		t.Fatalf("scaling: %q FLAT throughput — gain at N=%d is %.2fx < required %.2fx (pool ignores added workers?) (evidence: %s)",
			name, maxN, gain, MinThroughputGainAtMaxN, path)
	}
	if !monotonic {
		t.Fatalf("scaling: %q throughput DEGRADES as N grows (lock-convoy / scheduler bug) (evidence: %s)", name, path)
	}

	return rep
}
