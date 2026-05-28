package security

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"dev.helix.code/tests/stresschaos"
)

// §11.4.85 stress coverage for the REAL internal/security primitives.
//
// The unit under stress is the REAL *SecurityManager — its RWMutex-guarded
// scanResults map and securityScore/criticalIssues/highIssues counters, the real
// ScanFeatureContext deterministic no-scanner path, plus the pure calculateScore
// scoring function and the package-level tr() CONST-046 resolver. No fakes:
//
//   - The managers are built via NewSecurityManagerWithScanners() with ZERO
//     scanners, so ScanFeatureContext takes the deterministic "no scanners
//     available" branch — no net, no os/exec, no env dependence. (The net/exec
//     SonarQube/Snyk Scan paths are deliberately AVOIDED in stress loops per the
//     no-flaky-real-network rule.)
//   - calculateScore is a pure function exercised across the full severity domain.
//   - tr() is the real translator resolver (NoopTranslator default).
//
// Run under -race to catch data races on the shared scanResults map + counters.

// TestSecurityManager_Stress_SustainedScanFeature drives the real
// ScanFeatureContext deterministic path under sustained load (N>=100), recording
// per-call latency. Each iteration scans a distinct feature and asserts the
// result is non-nil, can-proceed, and was persisted in the RWMutex-guarded map,
// so the run proves real scan-and-store work — not a no-op.
func TestSecurityManager_Stress_SustainedScanFeature(t *testing.T) {
	sm := NewSecurityManagerWithScanners() // zero scanners -> deterministic, no net/exec
	ctx := context.Background()

	var stored int64
	stresschaos.RunSustainedLoad(t, "security_sustained_scan_feature",
		stresschaos.SustainedConfig{N: 1500, MaxErrorRate: 0.0},
		func(i int) error {
			feature := fmt.Sprintf("feature-%d", i)
			res, err := sm.ScanFeatureContext(ctx, feature)
			if err != nil {
				return fmt.Errorf("scan: %w", err)
			}
			if res == nil {
				return fmt.Errorf("scan returned nil result for %q", feature)
			}
			if res.FeatureName != feature {
				return fmt.Errorf("scan returned feature %q, want %q", res.FeatureName, feature)
			}
			if !res.CanProceed {
				return fmt.Errorf("no-scanner scan should allow proceed for %q", feature)
			}
			if res.Recommendations == nil || len(res.Recommendations) == 0 {
				return fmt.Errorf("no-scanner scan must carry a recommendation for %q", feature)
			}
			atomic.AddInt64(&stored, 1)
			return nil
		})

	if atomic.LoadInt64(&stored) == 0 {
		t.Fatal("security manager scanned zero features under sustained load — not real work")
	}
	// Every distinct feature scanned must remain in the map — proof of real persistence.
	sm.mutex.RLock()
	mapLen := len(sm.scanResults)
	sm.mutex.RUnlock()
	if int64(mapLen) != atomic.LoadInt64(&stored) {
		t.Fatalf("map holds %d results but %d distinct features scanned — lost writes", mapLen, atomic.LoadInt64(&stored))
	}
	t.Logf("security sustained: %d features scanned + persisted", atomic.LoadInt64(&stored))
}

// TestSecurityManager_Stress_SustainedMetricsUpdate drives the real metrics
// update/read path (UpdateSecurityMetrics + Get* + ValidateZeroTolerance) under
// sustained load. Each iteration writes a deterministic metric set then reads it
// back, asserting the read observes the just-written value (single-writer here),
// proving the RWMutex protects the counters during real read/write traffic.
func TestSecurityManager_Stress_SustainedMetricsUpdate(t *testing.T) {
	sm := NewSecurityManagerWithScanners()

	stresschaos.RunSustainedLoad(t, "security_sustained_metrics_update",
		stresschaos.SustainedConfig{N: 2000, MaxErrorRate: 0.0},
		func(i int) error {
			crit := i % 7
			high := i % 11
			score := i % 101
			sm.UpdateSecurityMetrics(crit, high, score)
			if got := sm.GetCriticalIssues(); got != crit {
				return fmt.Errorf("critical readback %d != %d", got, crit)
			}
			if got := sm.GetHighIssues(); got != high {
				return fmt.Errorf("high readback %d != %d", got, high)
			}
			if got := sm.GetSecurityScore(); got != score {
				return fmt.Errorf("score readback %d != %d", got, score)
			}
			wantZeroTol := crit == 0
			if got := sm.ValidateZeroTolerance(); got != wantZeroTol {
				return fmt.Errorf("zero-tolerance %v != want %v (crit=%d)", got, wantZeroTol, crit)
			}
			return nil
		})
}

// TestSecurityManager_Stress_ConcurrentScanAndMetrics hammers the shared manager
// state from N>=10 concurrent goroutines that interleave ScanFeatureContext (map
// write under Lock), the Get* accessors (RLock), UpdateSecurityMetrics (Lock),
// and ValidateZeroTolerance (RLock) — generating genuine read/write contention on
// BOTH the scanResults map and the metric counters. Asserts no deadlock, no
// goroutine leak, no data race (run under -race). Each goroutine scans its own
// feature key so the map genuinely grows under concurrent writers.
func TestSecurityManager_Stress_ConcurrentScanAndMetrics(t *testing.T) {
	sm := NewSecurityManagerWithScanners()
	ctx := context.Background()

	var scans int64
	stresschaos.RunConcurrent(t, "security_concurrent_scan_and_metrics",
		stresschaos.ConcurrencyConfig{Parallelism: 16, IterationsPerGoroutine: 150, Timeout: 25 * time.Second},
		func(g, it int) error {
			// Map write under Lock (contends with other writers + readers).
			feature := fmt.Sprintf("g%d-it%d", g, it)
			res, err := sm.ScanFeatureContext(ctx, feature)
			if err != nil {
				return fmt.Errorf("scan: %w", err)
			}
			if res == nil {
				return fmt.Errorf("nil scan result for %q", feature)
			}
			atomic.AddInt64(&scans, 1)

			// Metric write under Lock (contends with the map writers above).
			sm.UpdateSecurityMetrics(g%5, it%5, (g*it)%101)

			// Read-only accessors under RLock (widen the read surface).
			_ = sm.GetCriticalIssues()
			_ = sm.GetHighIssues()
			_ = sm.GetSecurityScore()
			_ = sm.ValidateZeroTolerance()
			return nil
		})

	if atomic.LoadInt64(&scans) == 0 {
		t.Fatal("zero scans under concurrent load")
	}
	// All distinct feature keys must have landed in the map — proof concurrent
	// writes were not lost / did not corrupt the map.
	sm.mutex.RLock()
	mapLen := len(sm.scanResults)
	sm.mutex.RUnlock()
	if int64(mapLen) != atomic.LoadInt64(&scans) {
		t.Fatalf("map holds %d entries but %d concurrent scans succeeded — torn map", mapLen, atomic.LoadInt64(&scans))
	}
	t.Logf("security concurrent: %d scans persisted under 16-goroutine contention", atomic.LoadInt64(&scans))
}

// TestSecurityManager_Stress_ConcurrentTranslator hammers the package-level tr()
// resolver + SetTranslator from N>=10 goroutines. SetTranslator(nil) must reset
// to NoopTranslator (never leave a nil translator), and tr() must never panic
// while the translator is being swapped concurrently — the resolver guards
// against a nil translator. Run under -race to catch a torn translator pointer.
func TestSecurityManager_Stress_ConcurrentTranslator(t *testing.T) {
	t.Cleanup(func() { SetTranslator(nil) }) // restore default after the run

	ctx := context.Background()
	var resolved int64
	stresschaos.RunConcurrent(t, "security_concurrent_translator",
		stresschaos.ConcurrencyConfig{Parallelism: 12, IterationsPerGoroutine: 200, Timeout: 20 * time.Second},
		func(g, it int) error {
			// Half the goroutines churn the translator pointer; all of them resolve.
			if g%2 == 0 {
				SetTranslator(nil) // must reset to NoopTranslator, never leave nil
			}
			out := tr(ctx, fmt.Sprintf("msg_id_%d_%d", g, it), map[string]any{"k": it})
			if out == "" {
				return fmt.Errorf("tr returned empty string — should fall back to message ID")
			}
			atomic.AddInt64(&resolved, 1)
			return nil
		})
	if atomic.LoadInt64(&resolved) == 0 {
		t.Fatal("translator resolved nothing under concurrent load")
	}
}

// TestSecurityManager_Stress_BoundaryConditions exercises §11.4.85(A)(3) boundary
// cases against the REAL components.
func TestSecurityManager_Stress_BoundaryConditions(t *testing.T) {
	ctx := context.Background()

	// Empty: ScanFeatureContext with an empty feature name must return a clean,
	// can't-proceed result with no error (the documented empty-name branch).
	t.Run("empty_feature_name", func(t *testing.T) {
		sm := NewSecurityManagerWithScanners()
		res, err := sm.ScanFeatureContext(ctx, "")
		if err != nil {
			t.Fatalf("empty feature name must not error, got: %v", err)
		}
		if res == nil {
			t.Fatal("empty feature name returned nil result")
		}
		if res.CanProceed {
			t.Fatal("empty feature name must NOT allow proceed (success=false branch)")
		}
	})

	// calculateScore boundaries: no issues -> 100; floor clamp at 0; per-severity.
	t.Run("calculate_score_boundaries", func(t *testing.T) {
		if got := calculateScore(nil); got != 100 {
			t.Fatalf("calculateScore(nil) = %d, want 100", got)
		}
		if got := calculateScore([]SecurityIssue{}); got != 100 {
			t.Fatalf("calculateScore(empty) = %d, want 100", got)
		}
		// 6 BLOCKERs = 120 penalty -> must clamp at 0, never go negative.
		many := make([]SecurityIssue, 6)
		for i := range many {
			many[i] = SecurityIssue{Severity: "BLOCKER"}
		}
		if got := calculateScore(many); got != 0 {
			t.Fatalf("calculateScore(6 BLOCKER) = %d, want 0 (floor clamp)", got)
		}
		// Single MINOR = 2 penalty -> 98.
		if got := calculateScore([]SecurityIssue{{Severity: "MINOR"}}); got != 98 {
			t.Fatalf("calculateScore(1 MINOR) = %d, want 98", got)
		}
		// Unknown severity = 1 penalty (default branch) -> 99.
		if got := calculateScore([]SecurityIssue{{Severity: "totally-unknown-severity"}}); got != 99 {
			t.Fatalf("calculateScore(unknown) = %d, want 99", got)
		}
	})

	// Cancelled context: ScanFeatureContext must return a clean can't-proceed
	// result (the documented ctx.Done() branch), not panic or hang.
	t.Run("cancelled_context", func(t *testing.T) {
		sm := NewSecurityManagerWithScanners()
		cctx, cancel := context.WithCancel(ctx)
		cancel() // cancel BEFORE the scan
		res, err := sm.ScanFeatureContext(cctx, "feat")
		if err != nil {
			t.Fatalf("cancelled-context scan must not error, got: %v", err)
		}
		if res == nil {
			t.Fatal("cancelled-context scan returned nil result")
		}
		if res.CanProceed {
			t.Fatal("cancelled-context scan must NOT allow proceed")
		}
	})
}
