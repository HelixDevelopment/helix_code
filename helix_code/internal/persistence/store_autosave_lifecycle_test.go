package persistence

// Regression guard for the auto-save channel-lifecycle defect (process crash +
// silent dead loop). §11.4.115 RED-on-broken-artifact + GREEN-guard via the
// RED_MODE polarity switch; §11.4.135 standing regression guard.
//
// THE DEFECT (pre-fix store.go):
//   DisableAutoSave does close(s.stopAutoSave) but NEVER re-creates the channel.
//   Therefore, after one disable:
//     (1) a subsequent EnableAutoSave starts autoSaveLoop, which select{}s on the
//         already-CLOSED stopAutoSave and returns immediately -> auto-save SILENTLY
//         never runs again (lastSaveTime never advances after the re-enable).
//     (2) a subsequent DisableAutoSave calls close() on the already-closed channel
//         -> "panic: close of closed channel" (process crash).
//
// RED_MODE polarity (§11.4.115):
//   Default (RED_MODE unset or RED_MODE=0): standing GREEN regression guard
//     asserting the defect is ABSENT (re-enabled loop ticks again;
//     disable-after-re-enable does not panic). This is the always-on guard that
//     runs on every `go test` / build per §11.4.135 — it MUST be GREEN by default.
//   RED_MODE=1: reproduce-and-assert-the-defect-is-PRESENT. These runs are
//     expected to PASS on the *broken* artifact (proving the guard genuinely
//     reproduces the historical defect) and to FAIL on the *fixed* artifact.
//
// Run GREEN guard (default, on fixed code): go test -race -run TestAutoSaveLifecycle ./internal/persistence/...
// Run RED reproduction (on broken code):    RED_MODE=1 go test -race -run TestAutoSaveLifecycle ./internal/persistence/...

import (
	"os"
	"testing"
	"time"

	"dev.helix.code/internal/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// redMode reports whether the polarity switch is in reproduce-the-defect mode.
// Default (unset / "0") is the GREEN standing regression guard so a bare
// `go test` on the fixed artifact stays GREEN (§11.4.135). Reproduction on the
// broken artifact is the explicit opt-in RED_MODE=1 (§11.4.115).
func redMode() bool {
	return os.Getenv("RED_MODE") == "1"
}

// waitForSaveAfter polls GetLastSaveTime until it advances past `baseline` or the
// deadline elapses. Returns true if a save was observed (the loop ticked).
func waitForSaveAfter(s *Store, baseline time.Time, deadline time.Duration) bool {
	end := time.Now().Add(deadline)
	for time.Now().Before(end) {
		last := s.GetLastSaveTime()
		if last.After(baseline) {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return false
}

// TestAutoSaveLifecycle_ReEnableTicks reproduces SYMPTOM 1 (silent dead loop):
// after enable -> disable -> re-enable, the loop must tick again.
//
// On BROKEN code the re-enabled loop selects on the closed stopAutoSave and
// returns immediately, so no save ever happens after the re-enable.
func TestAutoSaveLifecycle_ReEnableTicks(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	require.NoError(t, err)

	sessionMgr := session.NewManager()
	store.SetSessionManager(sessionMgr)
	sessionMgr.Create("project1", "Session 1", "Test session", session.ModePlanning)

	// First enable/disable cycle.
	store.EnableAutoSave(20 * time.Millisecond)
	require.True(t, waitForSaveAfter(store, time.Time{}, 2*time.Second),
		"first auto-save cycle must tick at least once")
	store.DisableAutoSave()

	// Record the baseline AFTER disable so we only observe ticks from the re-enable.
	baseline := store.GetLastSaveTime()
	// Give the disabled loop time to fully stop; baseline must not advance here.
	time.Sleep(60 * time.Millisecond)
	require.Equal(t, baseline, store.GetLastSaveTime(),
		"disabled auto-save must not tick")

	// RE-ENABLE: the bug makes this loop a no-op.
	store.EnableAutoSave(20 * time.Millisecond)

	ticked := waitForSaveAfter(store, baseline, 2*time.Second)

	if redMode() {
		// RED: on the broken artifact the re-enabled loop never ticks. We do NOT
		// call DisableAutoSave() here because on broken code that second disable
		// would panic (close of closed channel) — that crash symptom is isolated
		// into TestAutoSaveLifecycle_DisableAfterReEnableNoPanic. This test asserts
		// ONLY the silent-dead-loop symptom.
		assert.False(t, ticked,
			"RED_MODE=1: broken code's re-enabled auto-save loop is a silent no-op (never ticks)")
	} else {
		// GREEN guard: on the fixed artifact the re-enabled loop ticks again, and
		// the cleanup disable is safe.
		defer store.DisableAutoSave()
		assert.True(t, ticked,
			"RED_MODE=0: re-enabled auto-save loop MUST tick again (lastSaveTime advances)")
	}
}

// TestAutoSaveLifecycle_DisableAfterReEnableNoPanic reproduces SYMPTOM 2
// (process crash): enable -> disable -> re-enable -> disable must not panic.
//
// On BROKEN code the second disable calls close() on the already-closed channel,
// which panics with "close of closed channel".
func TestAutoSaveLifecycle_DisableAfterReEnableNoPanic(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	require.NoError(t, err)

	// panicked captures whether the disable-after-re-enable sequence panicked.
	panicked := func() (p bool) {
		defer func() {
			if r := recover(); r != nil {
				p = true
			}
		}()
		store.EnableAutoSave(1 * time.Second)
		store.DisableAutoSave()
		store.EnableAutoSave(1 * time.Second)
		store.DisableAutoSave() // BROKEN: close of already-closed channel -> panic
		return false
	}()

	if redMode() {
		// RED: on the broken artifact this sequence panics.
		assert.True(t, panicked,
			"RED_MODE=1: broken code panics on disable-after-re-enable (close of closed channel)")
	} else {
		// GREEN guard: on the fixed artifact the sequence is safe.
		assert.False(t, panicked,
			"RED_MODE=0: disable-after-re-enable MUST NOT panic")
	}
}

// TestAutoSaveLifecycle_ManyCyclesStable is a GREEN-only stability guard: many
// enable/disable cycles plus a final live tick must be stable and panic-free.
// In RED mode it is skipped (the broken code crashes on the 2nd disable, which
// the dedicated RED test above already asserts).
func TestAutoSaveLifecycle_ManyCyclesStable(t *testing.T) {
	if redMode() {
		t.Skip("SKIP-OK: many-cycles stability is a GREEN-only guard; RED reproduction is covered by the panic + dead-loop RED tests")
	}

	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	require.NoError(t, err)

	sessionMgr := session.NewManager()
	store.SetSessionManager(sessionMgr)
	sessionMgr.Create("project1", "Session 1", "Test session", session.ModePlanning)

	// Hammer the lifecycle: 20 enable/disable cycles must neither panic nor leak.
	for i := 0; i < 20; i++ {
		store.EnableAutoSave(1 * time.Second)
		store.DisableAutoSave()
	}

	// Idempotency: double-enable then double-disable must be safe no-ops.
	store.EnableAutoSave(1 * time.Second)
	store.EnableAutoSave(1 * time.Second) // no-op
	store.DisableAutoSave()
	store.DisableAutoSave() // safe no-op
	assert.False(t, store.autoSaveEnabled)

	// After all that churn, a fresh enable must still produce a live tick.
	baseline := store.GetLastSaveTime()
	store.EnableAutoSave(20 * time.Millisecond)
	defer store.DisableAutoSave()
	assert.True(t, waitForSaveAfter(store, baseline, 2*time.Second),
		"after many lifecycle cycles a fresh enable MUST still tick")
}
