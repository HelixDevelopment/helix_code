package deployment

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"dev.helix.code/internal/security"
	"dev.helix.code/tests/stresschaos"
)

// §11.4.85(B) chaos coverage for the in-process ProductionDeployer surfaces.
//
// Chaos classes exercised against the REAL components (no fakes):
//
//   - input-corruption: structurally hostile security-scan Issues slices
//     (nil result, nil entries, empty strings, oversized strings, deeply
//     nested values, typed nil-pointer SecurityIssue) are fed to the REAL
//     countCriticalIssues / countHighIssues / classifyIssue classifiers. They
//     MUST classify or skip without panicking — a crash on malformed scanner
//     output would take down the deployment pipeline.
//   - state-corruption under contention: addNotification (writer) races
//     against status-reader goroutines and a concurrent failDeployment ->
//     triggerRollback path on the SAME deployer. The RWMutex-guarded ledger
//     MUST never panic, race, or end torn.
//   - process-death: a deployment is cancelled mid-flight via context; the
//     state machine MUST unwind without deadlock and leave a coherent status.
//   - resource-pressure: the classifiers run under bounded memory pressure and
//     MUST still return correct counts without OOM-crashing.

// TestProductionDeployer_Chaos_CorruptScanResult feeds hostile FeatureScanResult
// Issues shapes through the REAL severity classifiers. classifyIssue handles a
// heterogeneous []interface{} (strings, value structs, pointer structs); a nil
// entry, a typed nil *SecurityIssue, an empty string, or an oversized string
// must all be classified or skipped WITHOUT a panic. A crash here is a
// §11.4.85(B) Fatal.
func TestProductionDeployer_Chaos_CorruptScanResult(t *testing.T) {
	rec := stresschaos.NewChaosRecorder(t, "deployer_corrupt_scan_result", "input-corruption")

	hostile := []*security.FeatureScanResult{
		nil, // entirely nil result
		{Issues: nil},
		{Issues: []interface{}{}},
		{Issues: []interface{}{nil}},                                    // nil entry
		{Issues: []interface{}{""}},                                     // empty string
		{Issues: []interface{}{(*security.SecurityIssue)(nil)}},         // typed nil pointer
		{Issues: []interface{}{strings.Repeat("x", 1<<16)}},             // oversized string
		{Issues: []interface{}{security.SecurityIssue{Severity: ""}}},   // empty severity
		{Issues: []interface{}{map[string]any{"weird": []int{1, 2, 3}}}}, // unexpected type
		{Issues: []interface{}{"Running as root - security risk", nil, security.SecurityIssue{Severity: "BLOCKER"}}},
	}

	for i, res := range hostile {
		func(idx int, r *security.FeatureScanResult) {
			defer func() {
				if p := recover(); p != nil {
					rec.Record(stresschaos.Fatal, fmt.Sprintf("classifier[%d] panicked on corrupt scan result: %v", idx, p))
				}
			}()
			c := countCriticalIssues(r)
			h := countHighIssues(r)
			if c < 0 || h < 0 {
				rec.Record(stresschaos.Fatal, fmt.Sprintf("classifier[%d] returned negative counts c=%d h=%d", idx, c, h))
				return
			}
			rec.Record(stresschaos.Recovered, fmt.Sprintf("classifier[%d] handled corrupt input: critical=%d high=%d", idx, c, h))
		}(i, res)
	}

	// Sanity anchor: the last fixture has exactly one critical-keyword string +
	// one BLOCKER-tagged struct = 2 critical, and no high-severity entries.
	if c := countCriticalIssues(hostile[len(hostile)-1]); c != 2 {
		rec.Record(stresschaos.Fatal, fmt.Sprintf("classifier mis-counted known fixture: want 2 critical got %d", c))
	} else {
		rec.Record(stresschaos.Recovered, "classifier counted known mixed fixture correctly under corruption sweep")
	}

	rec.AssertNoFatal()
	t.Log("classifiers survived corrupt-scan-result injection without crash")
}

// TestProductionDeployer_Chaos_ConcurrentNotifyAndRollback drives the
// state-corruption class: notification writers, status readers, and a
// concurrent failDeployment->triggerRollback path all hit the SAME deployer's
// pd.status simultaneously. The RWMutex-guarded ledger must never panic or
// race (run under -race) and must end self-consistent. triggerRollback runs in
// its own goroutine because it sleeps 300ms/server; we keep TargetServers/
// ServersDeployed empty so it does no real per-server work, only mutates the
// shared status under contention.
func TestProductionDeployer_Chaos_ConcurrentNotifyAndRollback(t *testing.T) {
	rec := stresschaos.NewChaosRecorder(t, "deployer_concurrent_notify_rollback", "state-corruption")
	pd, err := NewProductionDeployer(stressConfig("notify-rollback"))
	if err != nil {
		t.Fatalf("new deployer: %v", err)
	}

	const writers = 10
	const readers = 6
	const iters = 150
	var wg sync.WaitGroup
	var writes, reads int64

	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if p := recover(); p != nil {
					rec.Record(stresschaos.Fatal, fmt.Sprintf("writer %d panicked: %v", id, p))
				}
			}()
			for it := 0; it < iters; it++ {
				pd.addNotification("chaos_write", fmt.Sprintf("w%d-i%d", id, it), "system")
				atomic.AddInt64(&writes, 1)
			}
		}(w)
	}
	for r := 0; r < readers; r++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if p := recover(); p != nil {
					rec.Record(stresschaos.Fatal, fmt.Sprintf("reader %d panicked: %v", id, p))
				}
			}()
			for it := 0; it < iters; it++ {
				pd.mutex.RLock()
				_ = len(pd.status.Notifications)
				_ = pd.status.Metrics.Notifications
				pd.mutex.RUnlock()
				atomic.AddInt64(&reads, 1)
			}
		}(r)
	}
	// One concurrent failure/rollback path mutating status mid-flight.
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() {
			if p := recover(); p != nil {
				rec.Record(stresschaos.Fatal, fmt.Sprintf("rollback path panicked: %v", p))
			}
		}()
		pd.failDeployment(PhaseDeployment, fmt.Errorf("chaos-injected failure"))
	}()
	wg.Wait()

	rec.Record(stresschaos.Recovered, fmt.Sprintf("survived %d notify-writes + %d reads + rollback under contention", writes, reads))

	// Final consistency: every successful addNotification appended exactly once,
	// and the metric matches the ledger length (the rollback path also notifies).
	pd.mutex.RLock()
	ledger := len(pd.status.Notifications)
	metric := pd.status.Metrics.Notifications
	pd.mutex.RUnlock()
	if ledger != metric {
		rec.Record(stresschaos.Fatal, fmt.Sprintf("ledger/metric diverged under contention: ledger=%d metric=%d (torn write)", ledger, metric))
	} else {
		rec.Record(stresschaos.Recovered, fmt.Sprintf("ledger==metric==%d after churn — status self-consistent", ledger))
	}

	rec.AssertNoFatal()
	t.Logf("deployer state-corruption chaos: writes=%d reads=%d ledger=%d", writes, reads, ledger)
}

// TestProductionDeployer_Chaos_KillDeploymentMidFlight injects a process-death
// fault: a real StartProductionDeployment is launched and the context is
// cancelled mid-flight (during the preparation phase, which sleeps ~1s). The
// deployer must unwind without deadlock and the single-flight guard must be
// released (a fresh Start must not be permanently rejected). No real infra is
// touched — the run fails honestly in a later phase regardless.
func TestProductionDeployer_Chaos_KillDeploymentMidFlight(t *testing.T) {
	pd, err := NewProductionDeployer(stressConfig("kill-mid-flight"))
	if err != nil {
		t.Fatalf("new deployer: %v", err)
	}

	stresschaos.ChaosKillDuring(t, "deployer_kill_deployment_mid_flight", 100*time.Millisecond,
		func(ctx context.Context, rec *stresschaos.ChaosRecorder) {
			_, derr := pd.StartProductionDeployment(ctx)
			// StartProductionDeployment returns (status, nil) for in-flight
			// phase outcomes; a non-nil error here would be the single-flight
			// rejection, which should not happen for the sole caller.
			if derr != nil {
				rec.Record(stresschaos.Degraded, fmt.Sprintf("deployment returned error under cancellation: %v", derr))
			} else {
				rec.Record(stresschaos.Recovered, "deployment unwound and returned status after mid-flight cancellation")
			}
		})

	// Post-cancellation the single-flight guard must be free: a fresh Start
	// must be admitted (it will itself fail honestly later, but must NOT be
	// rejected with "already running" — proof the guard was released on unwind).
	_, err2 := pd.StartProductionDeployment(context.Background())
	if err2 != nil && err2.Error() == "internal_deployment_already_running" {
		t.Fatalf("single-flight guard leaked after mid-flight cancellation — deployer permanently stuck")
	}
	t.Log("deployer survived mid-flight cancellation and the single-flight guard was released")
}

// TestProductionDeployer_Chaos_ClassifyUnderMemoryPressure runs the REAL
// severity classifiers under bounded memory pressure (resource-exhaustion
// class). The classifiers MUST still return correct counts without
// OOM-crashing. The pressure is strictly bounded by the harness (cap 128 MB).
func TestProductionDeployer_Chaos_ClassifyUnderMemoryPressure(t *testing.T) {
	res := &security.FeatureScanResult{
		Issues: []interface{}{
			"Running as root - security risk",
			"Binary signature verification failed",
			"Binary has loose permissions",
			security.SecurityIssue{Severity: "CRITICAL"},
			security.SecurityIssue{Severity: "HIGH"},
		},
	}

	stresschaos.ChaosResourcePressureDuring(t, "deployer_classify_under_memory_pressure", 64,
		func(rec *stresschaos.ChaosRecorder) {
			for i := 0; i < 500; i++ {
				c := countCriticalIssues(res)
				h := countHighIssues(res)
				// Two critical-keyword strings + one CRITICAL tag = 3 critical;
				// one high-keyword string + one HIGH tag = 2 high.
				if c != 3 || h != 2 {
					rec.Record(stresschaos.Fatal, fmt.Sprintf("classifier mis-counted under pressure: critical=%d (want 3) high=%d (want 2)", c, h))
					return
				}
			}
			rec.Record(stresschaos.Recovered, "classifiers returned correct counts under bounded memory pressure")
		})

	t.Log("classifiers survived bounded-memory-pressure injection")
}
