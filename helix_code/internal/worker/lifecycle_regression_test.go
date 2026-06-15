package worker

// Regression battery for three reproduced concurrency/lifecycle defects in the
// worker package, authored TDD per 11.4.115 (RED-on-broken-artifact +
// polarity-switch). Each test carries the package-shared RED_MODE polarity
// switch (redMode() lives in consensus_election_test.go):
//
//   RED_MODE=1           reproduce + assert the DEFECT IS PRESENT on the
//                        current (pre-fix) artifact. Run BEFORE the fix lands;
//                        these MUST observe the defect.
//   RED_MODE=0 (default) the standing GREEN regression guard asserting the
//                        defect is ABSENT on the fixed artifact (11.4.135).
//
// Defects (all REAL, reproduced):
//   DEFECT-1  WorkerPool.Stop() double-close panic on close(wp.stopChan).
//   DEFECT-2  SSHWorkerPool leaks 2 goroutines per pool (consensus run() on a
//             non-cancellable context.Background() + startSandboxCleanup with no
//             stop path); ConsensusManager.Stop() does not stop its run() loop.
//   DEFECT-3  WorkerPool.GetPoolStats() returns NaN utilization with 0 workers,
//             which makes the result un-marshalable by encoding/json.

import (
	"encoding/json"
	"math"
	"runtime"
	"sync"
	"testing"
	"time"

	"dev.helix.code/internal/config"
)

// settleGoroutines waits for transient goroutines (timer fan-out, GC) to drain
// so a goroutine-count delta reflects genuine leaks, not in-flight churn. It
// polls until the count is stable for two consecutive samples or a watchdog
// elapses (a regression then FAILs fast instead of hanging).
func settleGoroutines(watchdog time.Duration) int {
	deadline := time.Now().Add(watchdog)
	prev := runtime.NumGoroutine()
	for time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
		cur := runtime.NumGoroutine()
		if cur == prev {
			return cur
		}
		prev = cur
	}
	return prev
}

func newTestWorkersConfig() *config.WorkersConfig {
	return &config.WorkersConfig{HealthTTL: 300}
}

// --- DEFECT-1: WorkerPool.Stop() double-close panic -------------------------

func TestWorkerPool_Stop_Idempotent_NoDoubleClosePanic(t *testing.T) {
	wp := NewWorkerPool(newTestWorkersConfig())

	if redMode() {
		// RED: prove the defect is PRESENT on the pre-fix artifact: the second
		// Stop() must panic with "close of closed channel". We recover it so the
		// test reports the defect rather than crashing the whole run.
		wp.Stop()
		panicked := func() (p bool) {
			defer func() {
				if r := recover(); r != nil {
					p = true
					t.Logf("DEFECT-1 reproduced: second Stop() panicked: %v", r)
				}
			}()
			wp.Stop()
			return false
		}()
		if !panicked {
			t.Fatalf("RED expected: second Stop() should panic on the pre-fix artifact (double close of stopChan), but it did not - defect not reproduced")
		}
		return
	}

	// GREEN guard: Stop() is idempotent - calling it multiple times (and from
	// concurrent goroutines) must never panic.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("GREEN guard FAILED: Stop() panicked on repeat invocation: %v", r)
		}
	}()
	wp.Stop()
	wp.Stop()
	wp.Stop()

	// Concurrent Stop() must also be safe.
	wp2 := NewWorkerPool(newTestWorkersConfig())
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); wp2.Stop() }()
	}
	wg.Wait()
}

// TestWorkerPool_StartStop_DrainsLoop proves Stop() still joins the
// healthCheckLoop goroutine (wg.Wait) after the idempotency fix.
func TestWorkerPool_StartStop_DrainsLoop(t *testing.T) {
	if redMode() {
		t.Skip("RED_MODE: drain-after-fix guard runs in GREEN mode (RED_MODE=0)")
	}
	base := settleGoroutines(2 * time.Second)
	wp := NewWorkerPool(newTestWorkersConfig())
	if err := wp.Start(t.Context()); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	wp.Stop() // must return only after healthCheckLoop exits (wg.Wait)
	after := settleGoroutines(3 * time.Second)
	if after > base {
		t.Fatalf("GREEN guard FAILED: healthCheckLoop not drained by Stop(): base=%d after=%d", base, after)
	}
}

// --- DEFECT-2: SSHWorkerPool goroutine leak; ConsensusManager.Stop loop ------

// TestSSHWorkerPool_StartStop_NoGoroutineLeak starts N pools and stops them,
// asserting no net goroutine growth.
func TestSSHWorkerPool_StartStop_NoGoroutineLeak(t *testing.T) {
	const n = 8

	if redMode() {
		// RED: pre-fix, each NewSSHWorkerPool leaks 2 goroutines (consensus
		// run() + startSandboxCleanup) and there is no way to stop them, so the
		// count grows by ~2n with no recovery path.
		base := settleGoroutines(2 * time.Second)
		for i := 0; i < n; i++ {
			p := NewSSHWorkerPool(false)
			p.StopConsensus() // pre-fix: only stops timers, NOT run() or cleanup
		}
		after := settleGoroutines(3 * time.Second)
		grew := after - base
		if grew < n {
			t.Fatalf("RED expected: starting %d pools should leak goroutines (~%d) on the pre-fix artifact, but net growth was only %d - defect not reproduced", n, 2*n, grew)
		}
		t.Logf("DEFECT-2 reproduced: %d pools leaked %d goroutines (no stop path)", n, grew)
		return
	}

	// GREEN guard: after Close(), every pool's background goroutines (consensus
	// run loop + sandbox cleanup) must be joined -> no net growth.
	base := settleGoroutines(2 * time.Second)
	for i := 0; i < n; i++ {
		p := NewSSHWorkerPool(false)
		p.Close()
	}
	after := settleGoroutines(4 * time.Second)
	if after > base {
		t.Fatalf("GREEN guard FAILED: %d Start/Close cycles leaked goroutines: base=%d after=%d (delta=%d)", n, base, after, after-base)
	}
}

// TestConsensusManager_Stop_StopsRunLoop proves ConsensusManager.Stop() now
// cancels and joins its run() goroutine (delta must be 0 after Stop()).
func TestConsensusManager_Stop_StopsRunLoop(t *testing.T) {
	if redMode() {
		// RED: pre-fix, Start(ctx) with a non-cancellable ctx + Stop() that only
		// stops timers leaves run() selecting forever -> delta survives Stop().
		base := settleGoroutines(2 * time.Second)
		cm := NewConsensusManager(ConsensusConfig{NodeID: "red-node"})
		_ = cm.Start(t.Context())
		cm.Stop()
		after := settleGoroutines(3 * time.Second)
		if after-base < 1 {
			t.Fatalf("RED expected: ConsensusManager.Start+Stop should leave run() alive (delta>=1) on the pre-fix artifact, got delta=%d", after-base)
		}
		t.Logf("DEFECT-2 (consensus) reproduced: run() survived Stop(), delta=%d", after-base)
		return
	}

	// GREEN guard: Stop() cancels + joins run(); no surviving goroutine.
	base := settleGoroutines(2 * time.Second)
	cm := NewConsensusManager(ConsensusConfig{NodeID: "green-node"})
	if err := cm.Start(t.Context()); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	cm.Stop()
	after := settleGoroutines(3 * time.Second)
	if after > base {
		t.Fatalf("GREEN guard FAILED: ConsensusManager.Stop() did not join run(): base=%d after=%d", base, after)
	}
}

// --- DEFECT-3: GetPoolStats() NaN utilization with 0 workers ----------------

func TestWorkerPool_GetPoolStats_ZeroWorkers_NoNaN(t *testing.T) {
	wp := NewWorkerPool(newTestWorkersConfig())
	stats := wp.GetPoolStats()
	util, _ := stats["utilization_rate"].(float64)

	if redMode() {
		// RED: pre-fix, 0/0*100 = NaN, and json.Marshal rejects NaN.
		if !math.IsNaN(util) {
			t.Fatalf("RED expected: utilization_rate should be NaN with 0 workers on the pre-fix artifact, got %v - defect not reproduced", util)
		}
		if _, err := json.Marshal(stats); err == nil {
			t.Fatalf("RED expected: json.Marshal(stats) should ERROR on NaN on the pre-fix artifact, but it succeeded - defect not reproduced")
		}
		t.Logf("DEFECT-3 reproduced: utilization_rate=NaN, json.Marshal errors")
		return
	}

	// GREEN guard: 0 workers -> utilization 0, and the stats marshal cleanly.
	if math.IsNaN(util) {
		t.Fatalf("GREEN guard FAILED: utilization_rate is NaN with 0 workers")
	}
	if util != 0 {
		t.Fatalf("GREEN guard FAILED: expected utilization_rate 0 with 0 workers, got %v", util)
	}
	b, err := json.Marshal(stats)
	if err != nil {
		t.Fatalf("GREEN guard FAILED: json.Marshal(stats) errored: %v", err)
	}
	t.Logf("GREEN: 0-worker stats marshal OK: %s", b)

	// And utilization is still correct with workers present.
	wp.RegisterWorker(NewPoolWorker("w1", "W1", "h1", WorkerCapabilities{}))
	wp.RegisterWorker(NewPoolWorker("w2", "W2", "h2", WorkerCapabilities{}))
	if w, ok := wp.GetWorker("w1"); ok {
		w.UpdateStatus(StatusBusy)
	}
	stats2 := wp.GetPoolStats()
	if u, _ := stats2["utilization_rate"].(float64); u != 50 {
		t.Fatalf("GREEN guard FAILED: expected 50%% utilization (1 busy / 2 total), got %v", u)
	}
}
