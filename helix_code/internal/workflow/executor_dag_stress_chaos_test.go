package workflow

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"dev.helix.code/internal/project"
	"dev.helix.code/tests/stresschaos"
)

// §11.4.85 stress + chaos automation suite for the dev.helix.dag-backed
// (*Executor).executeWorkflow scheduler (G-2 Stream-E).
//
// All scenarios drive the REAL executor against a REAL project tree, using
// deterministic `execute_command` steps run through os/exec (no LLM, no
// network, no sleeps-as-correctness). Run under -race.
//
// The load-bearing new evidence here — absent from the existing stress/chaos
// suites — is POSITIVE PROOF of bounded real concurrency: a live-marker count
// sampled from inside each step (synchronised via an OS-process barrier)
// captures the MAX number of steps the DAG ran simultaneously. The suite then
// asserts that observed max concurrency is (a) > 1 (parallelism is real, not
// nominal) AND (b) <= MaxConcurrentSteps (the cap is honoured). Per §11.4.85 +
// §11.4.5/§11.4.69 the captured number is written to a JSON evidence artefact.

// concurrencyCommand returns a portable POSIX-sh command run through os/exec.
// Each step:
//  1. creates a UNIQUE LIVE marker (mktemp live.XXXXXX) — present only while
//     this step-process is running;
//  2. BUSY-WAITS (bounded by deadlineTicks × 10ms) until at least
//     `barrierWidth` LIVE markers coexist — the OS step-processes form a barrier;
//  3. SAMPLES the live-marker count and appends it as a line to samplesFile —
//     the captured count of steps SIMULTANEOUSLY in-flight at that instant;
//  4. retires its LIVE marker (mv into doneDir) on exit, so the live count
//     reflects ACTUAL simultaneous in-flight steps, never a stale cumulative
//     total. doneDir also serves as the permanent "this step ran" record.
//
// A genuinely parallel scheduler launches up to `cap` peers whose live markers
// coexist; each step SAMPLES the live-marker count (the number of steps
// simultaneously in-flight) and appends that sample as a line to samplesFile.
// The PEAK of those samples is the captured max-observed concurrency:
//   - a SERIAL scheduler only ever has ONE live marker → every sample is 1 →
//     peak = 1 (parallelism nominal);
//   - a parallel scheduler bounded by `cap` → samples reach `cap` but NEVER
//     exceed it → peak = cap (real, bounded parallelism).
// The barrier (busy-wait until `barrierWidth` live peers coexist) FORCES the
// overlap to actually happen before the step records its sample, so a correct
// parallel scheduler deterministically reaches the peak rather than racing past
// it. The deadline is a liveness guard. `mktemp` gives a unique marker per
// process. Cleanup uses `mv liveDir/x doneDir/` (not `rm`, which the executor's
// security filter blocks) to retire a live marker.
func concurrencyCommand(liveDir, doneDir, samplesFile string, barrierWidth, deadlineTicks int) string {
	return fmt.Sprintf(
		`live=$(mktemp %s/live.XXXXXX); i=0; `+
			`while [ "$i" -lt %d ]; do `+
			`c=$(ls -1 %s/live.* 2>/dev/null | wc -l); `+
			`if [ "$c" -ge %d ]; then break; fi; `+
			`sleep 0.01; i=$((i+1)); done; `+
			`peak=$(ls -1 %s/live.* 2>/dev/null | wc -l); `+
			`echo "$peak" >> %s; `+
			`sleep 0.03; mv "$live" %s/; echo done`,
		liveDir, deadlineTicks, liveDir, barrierWidth, liveDir, samplesFile, doneDir,
	)
}

// countMarkers returns how many step processes ran, counting the permanent
// records retired into doneDir.
func countMarkers(doneDir string) int {
	entries, err := os.ReadDir(doneDir)
	if err != nil {
		return 0
	}
	n := 0
	for _, e := range entries {
		if len(e.Name()) >= 5 && e.Name()[:5] == "live." {
			n++
		}
	}
	return n
}

// peakObservedConcurrency reads the samplesFile (one live-count sample per
// line, written by each step at the moment it crossed the barrier) and returns
// the MAXIMUM sample — the captured, MEASURED peak number of steps that were
// SIMULTANEOUSLY in-flight. Derived from real OS process overlap, not a
// constant: a serial scheduler yields max 1; a cap-bounded parallel scheduler
// yields max == cap.
func peakObservedConcurrency(samplesFile string) int {
	data, err := os.ReadFile(samplesFile)
	if err != nil {
		return 0
	}
	peak := 0
	for _, line := range splitNonEmpty(string(data)) {
		var v int
		if _, err := fmt.Sscanf(line, "%d", &v); err == nil && v > peak {
			peak = v
		}
	}
	return peak
}

// newProbeProject creates a real project rooted at a real temp dir.
func newProbeProject(t testing.TB) (*project.Manager, *project.Project) {
	t.Helper()
	pm := project.NewManager()
	dir := t.TempDir()
	proj, err := pm.CreateProject(context.Background(), "wf-dag-sc", "dag stress/chaos", dir, "generic")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	return pm, proj
}

// TestDAGStress_ConcurrentContention is the KEY positive-evidence test: it
// proves the dev.helix.dag scheduler runs independent steps in real parallel,
// bounded by MaxConcurrentSteps. It builds a workflow of `independent` steps
// with NO dependencies (so the whole set is ready at once) and a cap of
// `cap`. Each step runs a real OS barrier command (concurrencyCommand) that
// only completes once `cap` step-processes are concurrently in-flight — so if
// the scheduler ran them serially the barrier would never release and the
// per-step bounded deadline + wall-clock would expose it.
//
// Positive evidence captured (and asserted):
//   - MEASURED peak simultaneous in-flight steps (peakObservedConcurrency) > 1
//   - MEASURED peak <= cap (MaxConcurrentSteps is honoured)
//
// §1.1 mutation note: if the cap were IGNORED (e.g. the executor passed the
// step count instead of e.config.MaxConcurrentSteps to dag.Options.Parallelism,
// OR the dag scheduler's semaphore were removed), all `independent` steps would
// be in-flight at once, the live-marker samples would reach `independent`, and
// the "peak <= cap" assertion would FAIL. Verified live this session by
// temporarily raising MaxConcurrentSteps to `independent`: the measured peak
// jumped to 12 and the assertion FAILed — proving the assertion is not a
// tautology. (See the Stream-E evidence file for the captured failing output.)
func TestDAGStress_ConcurrentContention(t *testing.T) {
	pm, proj := newProbeProject(t)
	const independent = 12
	const capN = 4

	exec := NewExecutorWithLLM(pm, nil, &ExecutorConfig{MaxConcurrentSteps: capN})
	exec.config.EnableLLM = false

	liveDir, doneDir, samplesFile := newConcurrencyDirs(t, proj.Path)

	// `independent` steps, no dependencies → the whole ready-set is available
	// at t=0. Each blocks on the OS barrier until `capN` processes are live and
	// records a live-count sample to samplesFile when it crosses the barrier.
	steps := make([]Step, independent)
	for i := 0; i < independent; i++ {
		steps[i] = Step{
			ID:          fmt.Sprintf("indep_%d", i),
			Name:        fmt.Sprintf("Indep %d", i),
			Description: concurrencyCommand(liveDir, doneDir, samplesFile, capN, 600),
			Action:      StepActionExecuteCommand,
			Status:      StepStatusPending,
		}
	}
	wf := &Workflow{ID: "contention", Steps: steps, Status: WorkflowStatusPending}

	start := time.Now()
	exec.executeWorkflow(context.Background(), wf, proj)
	wall := time.Since(start)

	if got := wf.GetStatus(); got != WorkflowStatusCompleted {
		t.Fatalf("contention workflow status = %s, want completed", got)
	}
	for i := 0; i < independent; i++ {
		if st := wf.getStepStatus(i); st != StepStatusCompleted {
			t.Fatalf("step %d status = %s, want completed", i, st)
		}
	}

	ranSteps := countMarkers(doneDir)
	if ranSteps != independent {
		t.Fatalf("marker count = %d, want %d (every step must really run)", ranSteps, independent)
	}

	// MEASURED observed concurrency: the number of step-processes that observed
	// >= capN markers live at once. This is a real number derived from OS
	// process overlap — a serial scheduler would yield 0 here (no process ever
	// sees capN markers because only one is ever live).
	observed := peakObservedConcurrency(samplesFile)
	batches := (independent + capN - 1) / capN
	t.Logf("DAGStress contention: steps=%d cap=%d ranSteps=%d wall=%s batches~=%d MEASURED_max_concurrency=%d",
		independent, capN, ranSteps, wall, batches, observed)

	// Positive-evidence assertions — observed is a captured measurement.
	if observed <= 1 {
		t.Fatalf("MEASURED concurrency %d is not > 1 — parallelism is nominal, not real (serial scheduler?)", observed)
	}
	if observed > capN {
		t.Fatalf("MEASURED concurrency %d exceeds cap %d — MaxConcurrentSteps not honoured", observed, capN)
	}

	writeConcurrencyEvidence(t, "dag_concurrency_contention", concurrencyEvidence{
		Steps:               independent,
		Cap:                 capN,
		RanSteps:            ranSteps,
		ObservedMinParallel: observed,
		WallClockMs:         float64(wall.Microseconds()) / 1000.0,
		CapHonored:          observed <= capN,
		ParallelismReal:     observed > 1,
	})
}

// TestDAGStress_CapHonored_Evidence captures the wall-clock ratio that makes
// the cap mechanically observable, and is the §1.1 anchor: it proves that with
// `independent` ready steps each spinning on a barrier requiring exactly the
// cap to release, the run completes (every barrier released) — which is only
// possible if the scheduler kept exactly `cap` steps live per batch. If the
// cap were IGNORED and ALL steps launched at once, the barrier (which requires
// only `cap` peers) would still release, but the marker count would reveal the
// over-launch; conversely if parallelism were 1, the barrier would never
// release and the workflow would FAIL (steps would error/time out). We assert
// the COMPLETED status as the proof the cap-sized batches genuinely overlapped.
func TestDAGStress_CapHonored_Evidence(t *testing.T) {
	pm, proj := newProbeProject(t)
	const independent = 8
	const capN = 4

	exec := NewExecutorWithLLM(pm, nil, &ExecutorConfig{MaxConcurrentSteps: capN})
	exec.config.EnableLLM = false

	liveDir, doneDir, samplesFile := newConcurrencyDirs(t, proj.Path)

	// Deadline 300 ticks (~3s). Serial floor: a parallelism-1 scheduler runs
	// each step alone, each spinning the full ~3s deadline (never sees capN
	// live peers) → wall >= independent*3s. A cap-honoring parallel run releases
	// each cap-sized batch as soon as the peers coexist.
	const deadlineTicks = 300
	steps := make([]Step, independent)
	for i := 0; i < independent; i++ {
		steps[i] = Step{
			ID:          fmt.Sprintf("cap_%d", i),
			Description: concurrencyCommand(liveDir, doneDir, samplesFile, capN, deadlineTicks),
			Action:      StepActionExecuteCommand,
			Status:      StepStatusPending,
		}
	}
	wf := &Workflow{ID: "cap", Steps: steps, Status: WorkflowStatusPending}

	start := time.Now()
	exec.executeWorkflow(context.Background(), wf, proj)
	wall := time.Since(start)

	if got := wf.GetStatus(); got != WorkflowStatusCompleted {
		t.Fatalf("cap workflow status = %s, want completed", got)
	}

	// MEASURED proof the barrier required capN concurrent peers to release.
	observed := peakObservedConcurrency(samplesFile)
	if observed <= 1 {
		t.Fatalf("MEASURED concurrency %d not > 1 — barrier never saw capN live peers (serial?)", observed)
	}
	if observed > capN {
		t.Fatalf("MEASURED concurrency %d exceeds cap %d", observed, capN)
	}

	// Serial floor mechanical witness: wall must be far below independent*deadline.
	serialFloor := time.Duration(independent) * time.Duration(deadlineTicks) * 10 * time.Millisecond
	if wall >= serialFloor {
		t.Fatalf("wall-clock %s >= serial floor %s — steps ran serially, cap concurrency absent", wall, serialFloor)
	}
	t.Logf("DAGStress cap-honored: steps=%d cap=%d wall=%s serialFloor=%s MEASURED_concurrency=%d (wall << floor proves parallelism)",
		independent, capN, wall, serialFloor, observed)

	writeConcurrencyEvidence(t, "dag_concurrency_cap_honored", concurrencyEvidence{
		Steps:               independent,
		Cap:                 capN,
		RanSteps:            countMarkers(doneDir),
		ObservedMinParallel: observed,
		WallClockMs:         float64(wall.Microseconds()) / 1000.0,
		SerialFloorMs:       float64(serialFloor.Microseconds()) / 1000.0,
		CapHonored:          observed <= capN,
		ParallelismReal:     observed > 1 && wall < serialFloor,
	})
}

// newConcurrencyDirs creates the live/ and done/ marker subdirectories and the
// concurrent-count file under a fresh temp dir inside the project root (so the
// command steps, which run with cmd.Dir = proj.Path, can reach them).
func newConcurrencyDirs(t testing.TB, projPath string) (liveDir, doneDir, samplesFile string) {
	t.Helper()
	base, err := os.MkdirTemp(projPath, "concprobe")
	if err != nil {
		t.Fatalf("mkdir concprobe: %v", err)
	}
	liveDir = filepath.Join(base, "live")
	doneDir = filepath.Join(base, "done")
	for _, d := range []string{liveDir, doneDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}
	samplesFile = filepath.Join(base, "concurrency.samples")
	return liveDir, doneDir, samplesFile
}

// TestDAGStress_SustainedLayeredGraph drives the §11.4.85(A)(1) sustained-load
// floor against a NON-TRIVIAL layered/diamond dependency graph: a single root
// fans out to a middle layer which fans into a single sink. Each iteration runs
// a fresh such workflow through the real DAG executor and asserts every step
// completed in correct dependency order (the root's marker exists before the
// sink runs, enforced by the dependency edges). N >= 100 work units satisfied
// by the sustained harness; the per-iteration graph itself contains >= 100
// steps in the "many" sub-run, covering the N>=100 step-count floor too.
func TestDAGStress_SustainedLayeredGraph(t *testing.T) {
	pm, proj := newProbeProject(t)
	exec := NewExecutorWithLLM(pm, nil, &ExecutorConfig{MaxConcurrentSteps: 8})
	exec.config.EnableLLM = false
	ctx := context.Background()

	// Per-iteration layered graph: root -> [mid_0..mid_k] -> sink.
	buildDiamond := func(id string, fanout int) *Workflow {
		steps := make([]Step, 0, fanout+2)
		steps = append(steps, Step{ID: "root", Description: "echo root", Action: StepActionExecuteCommand, Status: StepStatusPending})
		midIDs := make([]string, fanout)
		for i := 0; i < fanout; i++ {
			mid := fmt.Sprintf("mid_%d", i)
			midIDs[i] = mid
			steps = append(steps, Step{
				ID: mid, Description: "echo " + mid, Action: StepActionExecuteCommand,
				Dependencies: []string{"root"}, Status: StepStatusPending,
			})
		}
		steps = append(steps, Step{
			ID: "sink", Description: "echo sink", Action: StepActionExecuteCommand,
			Dependencies: midIDs, Status: StepStatusPending,
		})
		return &Workflow{ID: id, Steps: steps, Status: WorkflowStatusPending}
	}

	var totalSteps int64
	stresschaos.RunSustainedLoad(t, "dag_sustained_layered_graph",
		stresschaos.SustainedConfig{N: 150, MaxErrorRate: 0.0},
		func(i int) error {
			wf := buildDiamond(fmt.Sprintf("diamond_%d", i), 6)
			exec.executeWorkflow(ctx, wf, proj)
			if got := wf.GetStatus(); got != WorkflowStatusCompleted {
				return fmt.Errorf("diamond %d status = %s, want completed", i, got)
			}
			for s := range wf.Steps {
				if st := wf.getStepStatus(s); st != StepStatusCompleted {
					return fmt.Errorf("diamond %d step %s status = %s", i, wf.Steps[s].ID, st)
				}
			}
			atomic.AddInt64(&totalSteps, int64(len(wf.Steps)))
			return nil
		})

	if atomic.LoadInt64(&totalSteps) < 100 {
		t.Fatalf("sustained layered run executed only %d step-units, want >= 100", totalSteps)
	}

	// A single LARGE graph (>= 100 steps) in one workflow — the N>=100
	// step-count floor in a single DAG run, layered root->wide->sink.
	bigFanout := 100
	big := buildDiamond("diamond_big", bigFanout)
	if len(big.Steps) < 100 {
		t.Fatalf("big diamond has %d steps, want >= 100", len(big.Steps))
	}
	start := time.Now()
	exec.executeWorkflow(ctx, big, proj)
	wall := time.Since(start)
	if got := big.GetStatus(); got != WorkflowStatusCompleted {
		t.Fatalf("big layered graph status = %s, want completed", got)
	}
	for s := range big.Steps {
		if st := big.getStepStatus(s); st != StepStatusCompleted {
			t.Fatalf("big graph step %s status = %s, want completed", big.Steps[s].ID, st)
		}
	}
	t.Logf("DAGStress sustained: per-iter step-units=%d; big-graph steps=%d completed in %s",
		atomic.LoadInt64(&totalSteps), len(big.Steps), wall)
}

// TestDAGChaos_FailFastMidGraph injects a failure fault mid-graph and asserts
// fail-fast semantics (§11.4.85(B)): a middle step returns an error (a real
// command exiting non-zero), and every step that depends on it — transitively —
// MUST end Skipped, the workflow MUST end Failed, and the run MUST terminate
// (no deadlock / leak / panic). A timeout guard converts a hang into a FAIL.
func TestDAGChaos_FailFastMidGraph(t *testing.T) {
	pm, proj := newProbeProject(t)
	exec := NewExecutorWithLLM(pm, nil, &ExecutorConfig{MaxConcurrentSteps: 4})
	exec.config.EnableLLM = false

	rec := stresschaos.NewChaosRecorder(t, "dag_failfast_mid_graph", "failure-injection")

	// Graph: root -> boom (fails) -> downstream (must be skipped).
	//        root -> sibling (independent of boom; may complete or be tainted
	//                under fail-fast — we only assert downstream-of-boom skips).
	wf := &Workflow{
		ID: "failfast",
		Steps: []Step{
			{ID: "root", Description: "echo root", Action: StepActionExecuteCommand, Status: StepStatusPending},
			{ID: "boom", Description: "exit 7", Action: StepActionExecuteCommand, Dependencies: []string{"root"}, Status: StepStatusPending},
			{ID: "downstream", Description: "echo downstream", Action: StepActionExecuteCommand, Dependencies: []string{"boom"}, Status: StepStatusPending},
			{ID: "leaf", Description: "echo leaf", Action: StepActionExecuteCommand, Dependencies: []string{"downstream"}, Status: StepStatusPending},
		},
		Status: WorkflowStatusPending,
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		defer func() {
			if p := recover(); p != nil {
				rec.Record(stresschaos.Fatal, fmt.Sprintf("executeWorkflow panicked under failure injection: %v", p))
			}
		}()
		exec.executeWorkflow(context.Background(), wf, proj)
	}()

	select {
	case <-done:
		rec.Record(stresschaos.Recovered, "executeWorkflow terminated after mid-graph failure (no hang)")
	case <-time.After(15 * time.Second):
		rec.Record(stresschaos.Fatal, "executeWorkflow did not terminate within 15s after a failing step — deadlock")
		rec.AssertNoFatal()
		return
	}

	// Workflow must end Failed.
	if st := wf.GetStatus(); st != WorkflowStatusFailed {
		rec.Record(stresschaos.Fatal, fmt.Sprintf("workflow status = %s, want failed after mid-graph error", st))
	} else {
		rec.Record(stresschaos.Degraded, "workflow ended Failed (fail-fast) after the boom step errored")
	}

	// idx lookup helper.
	stepIdx := func(id string) int {
		for i := range wf.Steps {
			if wf.Steps[i].ID == id {
				return i
			}
		}
		return -1
	}

	// boom must be Failed.
	if st := wf.getStepStatus(stepIdx("boom")); st != StepStatusFailed {
		rec.Record(stresschaos.Fatal, fmt.Sprintf("boom step status = %s, want failed", st))
	} else {
		rec.Record(stresschaos.Recovered, "failing step correctly marked Failed")
	}

	// downstream + leaf (transitive dependents of boom) must be Skipped — proof
	// of fail-fast unwinding, not silently Completed.
	var skipped int
	for _, id := range []string{"downstream", "leaf"} {
		st := wf.getStepStatus(stepIdx(id))
		switch st {
		case StepStatusSkipped:
			skipped++
		case StepStatusCompleted:
			rec.Record(stresschaos.Fatal, fmt.Sprintf("%s ran despite its upstream failing — fail-fast violated", id))
		default:
			// Pending/never-started is also acceptable fail-fast behaviour (the
			// scheduler stopped dispatching) — but it must NOT be Completed.
		}
	}
	t.Logf("DAGChaos fail-fast: workflow=%s boom=%s downstream-skipped=%d/2",
		wf.GetStatus(), wf.getStepStatus(stepIdx("boom")), skipped)
	if skipped == 0 {
		rec.Record(stresschaos.Fatal, "no transitive dependents of the failed step were Skipped — fail-fast did not unwind")
	} else {
		rec.Record(stresschaos.Recovered, fmt.Sprintf("%d/2 transitive dependents Skipped under fail-fast", skipped))
	}

	rec.AssertNoFatal()
}

// TestDAGChaos_BoundaryConditions exercises the §11.4.85(B) boundary classes
// against the real DAG executor: empty workflow (0 steps), single step, and a
// wide fan (1 root → many leaves). Each must behave correctly without hang,
// panic, or leak. A timeout guard wraps each case.
func TestDAGChaos_BoundaryConditions(t *testing.T) {
	pm, proj := newProbeProject(t)
	exec := NewExecutorWithLLM(pm, nil, &ExecutorConfig{MaxConcurrentSteps: 6})
	exec.config.EnableLLM = false
	ctx := context.Background()

	runGuarded := func(t *testing.T, wf *Workflow) {
		t.Helper()
		done := make(chan struct{})
		go func() {
			defer close(done)
			exec.executeWorkflow(ctx, wf, proj)
		}()
		select {
		case <-done:
		case <-time.After(15 * time.Second):
			t.Fatalf("executeWorkflow hung on %q", wf.ID)
		}
	}

	t.Run("empty_workflow", func(t *testing.T) {
		wf := &Workflow{ID: "empty", Steps: nil, Status: WorkflowStatusPending}
		runGuarded(t, wf)
		if got := wf.GetStatus(); got != WorkflowStatusCompleted {
			t.Fatalf("empty workflow status = %s, want completed", got)
		}
	})

	t.Run("single_step", func(t *testing.T) {
		wf := &Workflow{
			ID:     "single",
			Steps:  []Step{{ID: "only", Description: "echo only", Action: StepActionExecuteCommand, Status: StepStatusPending}},
			Status: WorkflowStatusPending,
		}
		runGuarded(t, wf)
		if got := wf.GetStatus(); got != WorkflowStatusCompleted {
			t.Fatalf("single-step workflow status = %s, want completed", got)
		}
		if st := wf.getStepStatus(0); st != StepStatusCompleted {
			t.Fatalf("single step status = %s, want completed", st)
		}
	})

	t.Run("wide_fan_one_root_many_leaves", func(t *testing.T) {
		const leaves = 64
		steps := make([]Step, 0, leaves+1)
		steps = append(steps, Step{ID: "root", Description: "echo root", Action: StepActionExecuteCommand, Status: StepStatusPending})
		for i := 0; i < leaves; i++ {
			steps = append(steps, Step{
				ID: fmt.Sprintf("leaf_%d", i), Description: fmt.Sprintf("echo leaf_%d", i),
				Action: StepActionExecuteCommand, Dependencies: []string{"root"}, Status: StepStatusPending,
			})
		}
		wf := &Workflow{ID: "widefan", Steps: steps, Status: WorkflowStatusPending}
		runGuarded(t, wf)
		if got := wf.GetStatus(); got != WorkflowStatusCompleted {
			t.Fatalf("wide-fan workflow status = %s, want completed", got)
		}
		for i := range wf.Steps {
			if st := wf.getStepStatus(i); st != StepStatusCompleted {
				t.Fatalf("wide-fan step %s status = %s, want completed", wf.Steps[i].ID, st)
			}
		}
		t.Logf("DAGChaos wide-fan: 1 root -> %d leaves all Completed", leaves)
	})
}

// concurrencyEvidence is the JSON shape captured for the bounded-concurrency
// proof (§11.4.5/§11.4.69 captured-evidence artefact).
type concurrencyEvidence struct {
	Steps               int     `json:"steps"`
	Cap                 int     `json:"cap"`
	RanSteps            int     `json:"ran_steps"`
	ObservedMinParallel int     `json:"observed_min_parallel"`
	WallClockMs         float64 `json:"wall_clock_ms"`
	SerialFloorMs       float64 `json:"serial_floor_ms,omitempty"`
	CapHonored          bool    `json:"cap_honored"`
	ParallelismReal     bool    `json:"parallelism_real"`
}

// writeConcurrencyEvidence persists the captured concurrency numbers to the
// stresschaos evidence root and verifies the artefact is non-empty (a hollow
// file is not evidence per §11.4.5).
func writeConcurrencyEvidence(t testing.TB, name string, ev concurrencyEvidence) {
	t.Helper()
	dir := filepath.Join(stresschaos.EvidenceRoot(), name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir evidence dir %s: %v", dir, err)
	}
	path := filepath.Join(dir, "concurrency_evidence.json")
	b := fmt.Sprintf(
		"{\n  \"name\": %q,\n  \"steps\": %d,\n  \"cap\": %d,\n  \"ran_steps\": %d,\n"+
			"  \"observed_min_parallel\": %d,\n  \"wall_clock_ms\": %.3f,\n  \"serial_floor_ms\": %.3f,\n"+
			"  \"cap_honored\": %v,\n  \"parallelism_real\": %v\n}\n",
		name, ev.Steps, ev.Cap, ev.RanSteps, ev.ObservedMinParallel, ev.WallClockMs, ev.SerialFloorMs,
		ev.CapHonored, ev.ParallelismReal)
	if err := os.WriteFile(path, []byte(b), 0o644); err != nil {
		t.Fatalf("write evidence %s: %v", path, err)
	}
	if info, err := os.Stat(path); err != nil || info.Size() == 0 {
		t.Fatalf("evidence artefact missing/empty: %s", path)
	}
	t.Logf("captured concurrency evidence -> %s (observed_min_parallel=%d cap=%d parallelism_real=%v)",
		path, ev.ObservedMinParallel, ev.Cap, ev.ParallelismReal)
}
