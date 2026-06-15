package verifier

import (
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// redModeGuard reports whether the RED polarity is active for these standing
// regression guards (§11.4.115). Unlike working_models_test.go's redMode()
// (which defaults to RED), these guards default to GREEN so a plain `go test`
// exercises them as standing regression guards asserting the defect is ABSENT
// on the fixed artifact. RED_MODE=1 explicitly switches to defect-reproduction
// polarity (asserts the historical defect is PRESENT on a pre-fix artifact).
// One source, two roles: the bug-catcher IS the regression-guard.
func redModeGuard() bool {
	return os.Getenv("RED_MODE") == "1"
}

// ---------------------------------------------------------------------------
// DEFECT-1 — Poller.Stop() double-close panic (HIGH, process-crash).
//
// Production path: bootstrap.go BootstrapResult.Shutdown() -> Poller.Stop().
// HTTP-server Shutdown is idempotent by convention, so a double Shutdown must
// not crash. RED reproduces the `close of closed channel` panic; GREEN proves
// Stop is idempotent and the poll loop still terminates.
// ---------------------------------------------------------------------------

func TestGuard_Defect1_PollerStopIsIdempotent(t *testing.T) {
	br := &BootstrapResult{
		Poller: NewPoller(NewAdapter(nil, nil, nil, &AdapterConfig{Enabled: false}), 0),
	}
	br.Poller.Start()
	// give the loop a moment to enter its select
	time.Sleep(20 * time.Millisecond)

	// First shutdown stops the poller.
	br.Shutdown()
	require.False(t, br.Poller.IsRunning(), "poller should be stopped after first Shutdown")

	if redModeGuard() {
		// On the broken artifact the second Shutdown -> Stop -> close(closed chan)
		// panics. Assert the panic is genuinely reproduced.
		assert.Panics(t, func() { br.Shutdown() },
			"RED: pre-fix Stop() double-close MUST panic (defect present)")
		return
	}

	// GREEN: a second (and third) Shutdown must NOT panic — Stop is idempotent.
	assert.NotPanics(t, func() {
		br.Shutdown()
		br.Shutdown()
	}, "GREEN: Stop() must be idempotent — a double Shutdown must never panic")
	assert.False(t, br.Poller.IsRunning(), "poller stays stopped after repeated Shutdown")
}

// ---------------------------------------------------------------------------
// DEFECT-2 — GetModelScore scale inconsistency (MEDIUM correctness).
//
// The adapter-map path normalizes score>10 by /10 (intended scale 0-10); the
// cache-fallback path returned the raw cached value with NO normalization, so
// the same modelID resolved to 8.5 (map) vs 85.0 (cache). RED reproduces the
// mismatch; GREEN proves both paths agree on the 0-10 scale.
// ---------------------------------------------------------------------------

func TestGuard_Defect2_ScoreScaleConsistentAcrossPaths(t *testing.T) {
	const modelID = "scale-test-model"
	const raw = 85.0 // a 0-100-scale value as it could land in the cache

	cache := NewCache(5*time.Minute, nil)
	cache.SetScores(map[string]float64{modelID: raw})

	adapter := NewAdapter(nil, cache, nil, &AdapterConfig{Enabled: true})

	// Map path: seed the in-memory map with the same raw value.
	adapter.mu.Lock()
	adapter.modelScores[modelID] = raw
	adapter.mu.Unlock()

	mapScore, ok := adapter.GetModelScore(modelID)
	require.True(t, ok, "map-path score must be present")
	require.Equal(t, 8.5, mapScore, "map path normalizes 85 -> 8.5 (0-10 scale)")

	// Cache path: clear the map so GetModelScore falls through to the cache.
	adapter.mu.Lock()
	delete(adapter.modelScores, modelID)
	adapter.mu.Unlock()

	cacheScore, ok := adapter.GetModelScore(modelID)
	require.True(t, ok, "cache-fallback score must be present")

	if redModeGuard() {
		// On the broken artifact the cache path returns the raw 85.0.
		assert.Equal(t, raw, cacheScore,
			"RED: pre-fix cache path returns un-normalized raw value (defect present)")
		assert.NotEqual(t, mapScore, cacheScore,
			"RED: map vs cache disagree on scale (defect present)")
		return
	}

	// GREEN: both paths agree on the 0-10 scale.
	assert.Equal(t, 8.5, cacheScore, "GREEN: cache path normalizes 85 -> 8.5")
	assert.Equal(t, mapScore, cacheScore,
		"GREEN: map and cache paths MUST return the same score for the same modelID")
}

// ---------------------------------------------------------------------------
// DEFECT-3 — circuit-breaker "allow one probe" not exclusive (MEDIUM behavioral).
//
// In CircuitOpen past halfOpenTimeout, AllowRequest() (RLock, read-only)
// returned true for EVERY concurrent caller without transitioning to
// CircuitHalfOpen -> thundering herd on the upstream during an outage. RED
// reproduces "all 100 callers allowed"; GREEN proves exactly ONE probe is
// allowed per half-open window.
// ---------------------------------------------------------------------------

func TestGuard_Defect3_HalfOpenAllowsExactlyOneProbe(t *testing.T) {
	h := NewHealthMonitor(2, 1, 50*time.Millisecond)
	h.RecordFailure()
	h.RecordFailure()
	require.Equal(t, CircuitOpen, h.State(), "circuit must be open after threshold failures")

	time.Sleep(60 * time.Millisecond) // exceed halfOpenTimeout

	const callers = 100
	var allowed int64
	var wg sync.WaitGroup
	wg.Add(callers)
	for i := 0; i < callers; i++ {
		go func() {
			defer wg.Done()
			if h.AllowRequest() {
				atomic.AddInt64(&allowed, 1)
			}
		}()
	}
	wg.Wait()

	got := atomic.LoadInt64(&allowed)

	if redModeGuard() {
		// On the broken artifact every concurrent caller is allowed.
		assert.Greater(t, got, int64(1),
			"RED: pre-fix allows the thundering herd (>1 caller allowed past timeout)")
		return
	}

	// GREEN: exactly one probe is allowed; the rest are denied until it resolves.
	assert.Equal(t, int64(1), got,
		"GREEN: exactly ONE probe must be allowed per half-open window")
	assert.Equal(t, CircuitHalfOpen, h.State(),
		"GREEN: the first probe transitions Open -> HalfOpen")

	// Preserve recovery semantics: a success on the probe closes the circuit.
	h.RecordSuccess()
	assert.Equal(t, CircuitClosed, h.State(),
		"GREEN: success on the single probe (recoveryThreshold=1) closes the circuit")
}
