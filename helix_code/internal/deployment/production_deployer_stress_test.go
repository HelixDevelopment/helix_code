package deployment

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"dev.helix.code/internal/security"
	"dev.helix.code/tests/stresschaos"
)

// §11.4.85(A) stress coverage for the in-process surfaces of the
// ProductionDeployer state machine.
//
// The unit under stress is the REAL ProductionDeployer (no fakes for the
// concurrency surface): the RWMutex-guarded pd.status notification ledger
// (addNotification — the HXC-014 data-race-fixed path), the atomic.Bool
// single-flight deployment guard (StartProductionDeployment's
// running.CompareAndSwap), the package-level CONST-046 translator DI seam
// (SetTranslator/tr — the second HXC-014 data-race-fixed path), and the
// pure classification/helper functions (countCriticalIssues / countHighIssues
// / calculateAverageResponseTime / parseDuration / generateDeploymentID).
//
// What we deliberately do NOT stress: the deployment phases that only log an
// honest gap (prepareEnvironment / validateTargetServers do no real work after
// the HXC-083 anti-bluff repair removed their no-op sleeps + fabricated success
// logs) or reach for real infrastructure (deployToServer requires SSH transport,
// checkServerHealth requires HTTP/SSH access — both honestly refuse without
// it). Driving those in a hot loop would be slow and would exercise infra we
// do not own, not the in-process state machine. They are covered for the
// single-flight guard only, at a modest count, in the concurrency test below.

// stressConfig returns a deployer config that exercises the in-process state
// machine without requiring any real infrastructure.
func stressConfig(name string) *DeploymentConfig {
	return &DeploymentConfig{
		ProjectName:        name,
		BinaryPath:         "/nonexistent/helixcode-stress-binary",
		Environment:        "test",
		DeploymentStrategy: ProductionDeploy,
		TargetServers:      []string{"server-a", "server-b"},
		Credentials:        map[string]string{},
	}
}

// TestProductionDeployer_Stress_SustainedNotifyAndClassify drives the real
// NewProductionDeployer -> addNotification ledger path plus the real
// severity-classification helpers under sustained load (N>=1000), recording
// per-call latency. Every iteration constructs a fresh deployer, appends a
// notification (real RWMutex-guarded append + metrics increment), and runs the
// real countCritical/countHigh classifiers over a heterogeneous Issues slice,
// asserting the counts match the known fixture so the run proves real work.
func TestProductionDeployer_Stress_SustainedNotifyAndClassify(t *testing.T) {
	var processed int64
	stresschaos.RunSustainedLoad(t, "deployer_sustained_notify_and_classify",
		stresschaos.SustainedConfig{N: 1000, MaxErrorRate: 0.0},
		func(i int) error {
			pd, err := NewProductionDeployer(stressConfig(fmt.Sprintf("stress-%d", i)))
			if err != nil {
				return fmt.Errorf("new deployer: %w", err)
			}
			pd.addNotification("stress_event", fmt.Sprintf("iteration %d", i), "system")
			if got := len(pd.status.Notifications); got != 1 {
				return fmt.Errorf("notification ledger: want 1 got %d", got)
			}
			if got := pd.status.Metrics.Notifications; got != 1 {
				return fmt.Errorf("notification metric: want 1 got %d", got)
			}

			// Real classification over a fixed heterogeneous fixture: one
			// CRITICAL-keyword string, one HIGH-keyword string, plus a typed
			// SecurityIssue. countCriticalIssues must see exactly 1, countHigh 1.
			res := &security.FeatureScanResult{
				Issues: []interface{}{
					"Running as root - security risk",   // critical keyword
					"Binary has loose permissions",      // high keyword
					security.SecurityIssue{Severity: "BLOCKER"}, // critical via tag
				},
			}
			if c := countCriticalIssues(res); c != 2 {
				return fmt.Errorf("critical count: want 2 got %d", c)
			}
			if h := countHighIssues(res); h != 1 {
				return fmt.Errorf("high count: want 1 got %d", h)
			}

			// Helper-function exercise: parseDuration + average-response-time.
			if d := parseDuration("250ms"); d != 250*time.Millisecond {
				return fmt.Errorf("parseDuration: got %v", d)
			}
			avg := calculateAverageResponseTime([]ServerHealth{
				{ResponseTime: 10 * time.Millisecond},
				{ResponseTime: 30 * time.Millisecond},
			})
			if avg != 20*time.Millisecond {
				return fmt.Errorf("avg response time: got %v", avg)
			}
			if generateDeploymentID() == "" {
				return fmt.Errorf("empty deployment id")
			}
			atomic.AddInt64(&processed, 1)
			return nil
		})

	if atomic.LoadInt64(&processed) == 0 {
		t.Fatal("deployer processed zero iterations under sustained load — not real work")
	}
	t.Logf("deployer sustained: %d notify+classify iterations", atomic.LoadInt64(&processed))
}

// TestProductionDeployer_Stress_ConcurrentNotify hammers addNotification on a
// SINGLE shared *ProductionDeployer from N>=16 goroutines. This is the exact
// surface of the HXC-014 data race: the shared pd.status.Notifications slice
// append + pd.status.Metrics.Notifications increment. Run under -race, this
// FAILS if the RWMutex guard is removed (anti-bluff proof below). After the
// run the ledger length and the metrics counter must equal the total call
// count — proof no append was lost to a torn write.
func TestProductionDeployer_Stress_ConcurrentNotify(t *testing.T) {
	pd, err := NewProductionDeployer(stressConfig("concurrent-notify"))
	if err != nil {
		t.Fatalf("new deployer: %v", err)
	}

	const parallelism = 16
	const iters = 200
	stresschaos.RunConcurrent(t, "deployer_concurrent_notify",
		stresschaos.ConcurrencyConfig{Parallelism: parallelism, IterationsPerGoroutine: iters, Timeout: 25 * time.Second},
		func(g, it int) error {
			pd.addNotification("concurrent", fmt.Sprintf("g%d-i%d", g, it), "system")
			return nil
		})

	want := parallelism * iters
	pd.mutex.RLock()
	gotLedger := len(pd.status.Notifications)
	gotMetric := pd.status.Metrics.Notifications
	pd.mutex.RUnlock()
	if gotLedger != want {
		t.Fatalf("notification ledger lost writes under contention: want %d got %d", want, gotLedger)
	}
	if gotMetric != want {
		t.Fatalf("notification metric lost increments under contention: want %d got %d", want, gotMetric)
	}
	t.Logf("deployer concurrent notify: %d notifications, ledger+metric consistent", want)
}

// TestProductionDeployer_Stress_ConcurrentTranslatorSeam hammers the
// package-level CONST-046 translator DI seam from N>=16 goroutines — half
// writing (SetTranslator) and half reading (tr) — the exact surface of the
// second HXC-014 data race (translator.go SetTranslator write vs tr read).
// Run under -race this FAILS if the RWMutex guard is removed. Every tr() must
// still resolve a non-empty string (NoopTranslator echoes the message ID).
func TestProductionDeployer_Stress_ConcurrentTranslatorSeam(t *testing.T) {
	defer SetTranslator(nil) // restore the loud-echo default for sibling tests

	var resolved int64
	stresschaos.RunConcurrent(t, "deployer_concurrent_translator_seam",
		stresschaos.ConcurrencyConfig{Parallelism: 16, IterationsPerGoroutine: 200, Timeout: 25 * time.Second},
		func(g, it int) error {
			if g%2 == 0 {
				SetTranslator(nil) // resets to NoopTranslator under the write lock
				return nil
			}
			out := tr(context.Background(), "internal_deployment_already_running", nil)
			if out == "" {
				return fmt.Errorf("tr returned empty string — seam degraded silently")
			}
			atomic.AddInt64(&resolved, 1)
			return nil
		})

	if atomic.LoadInt64(&resolved) == 0 {
		t.Fatal("translator seam resolved zero strings under concurrent load")
	}
	t.Logf("deployer concurrent translator seam: %d resolutions", atomic.LoadInt64(&resolved))
}

// TestProductionDeployer_Stress_SingleFlightGuard exercises the real
// atomic.Bool single-flight guard in StartProductionDeployment under
// concurrent contention: many goroutines call Start simultaneously and EXACTLY
// ONE must win the CompareAndSwap while the rest are rejected with the
// (CONST-046-resolved) "already running" message. This is bounded (one
// concurrent burst) because each full run drives the real phase machine
// (which sleeps and honestly refuses real infra) — we are testing the
// concurrency guard, not the deployment itself.
func TestProductionDeployer_Stress_SingleFlightGuard(t *testing.T) {
	pd, err := NewProductionDeployer(stressConfig("single-flight"))
	if err != nil {
		t.Fatalf("new deployer: %v", err)
	}
	ctx := context.Background()

	const racers = 12
	var wg sync.WaitGroup
	var rejected int64
	var completed int64
	wg.Add(racers)
	start := make(chan struct{})
	for i := 0; i < racers; i++ {
		go func() {
			defer wg.Done()
			<-start
			_, callErr := pd.StartProductionDeployment(ctx)
			// The deployment will fail in a later phase (no real infra), but
			// StartProductionDeployment returns (status, nil) for phase
			// failures — only the single-flight rejection returns a non-nil
			// error carrying the "already running" message ID.
			if callErr != nil && callErr.Error() == "internal_deployment_already_running" {
				atomic.AddInt64(&rejected, 1)
			} else {
				atomic.AddInt64(&completed, 1)
			}
		}()
	}
	close(start)
	wg.Wait()

	// Exactly one goroutine should have entered the critical section; the rest
	// must have been rejected by the CAS guard. (The winner returns nil even on
	// phase failure, so completed counts the winner.)
	if atomic.LoadInt64(&completed) != 1 {
		t.Fatalf("single-flight guard breached: %d goroutines entered deployment (want exactly 1), %d rejected",
			atomic.LoadInt64(&completed), atomic.LoadInt64(&rejected))
	}
	if atomic.LoadInt64(&rejected) != racers-1 {
		t.Fatalf("single-flight guard rejection count: want %d got %d", racers-1, atomic.LoadInt64(&rejected))
	}
	t.Logf("single-flight guard: 1 winner, %d rejected — atomic.Bool guard holds", atomic.LoadInt64(&rejected))
}
