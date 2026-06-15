package helixqa

// wrapper_guard_test.go — standing regression guards (§11.4.135) for
// internal/helixqa defects discovered by the §11.4.118 discovery sweep.
//
// Each guard carries a §11.4.115 RED_MODE polarity switch:
//   RED_MODE=1  reproduces the PRE-FIX (broken) behaviour inline and
//               asserts the WRONG outcome (proving the defect is real).
//   RED_MODE=0  (DEFAULT, no env) drives the REAL fixed production code
//               and asserts the CORRECT outcome (the standing guard).
//
// Mocks are NOT used here — these guards drive the real Engine /
// SessionState production types against in-process state (no helix_qa
// backend, no network).

import (
	"context"
	"os"
	"testing"
	"time"

	"dev.helix.code/internal/config"

	"github.com/stretchr/testify/require"
)

func redMode() bool { return os.Getenv("RED_MODE") == "1" }

// newEnabledEngine builds a real, enabled Engine backed by a temp dir.
func newEnabledEngine(t *testing.T) *Engine {
	t.Helper()
	tmpDir := t.TempDir()
	cfg := &config.Config{
		QA: config.QAConfig{
			Enabled:   true,
			OutputDir: tmpDir,
			Platforms: []string{"web"},
			BanksDir:  tmpDir,
		},
		Logging: config.LoggingConfig{Level: "info"},
	}
	engine, err := NewEngine(cfg)
	require.NoError(t, err)
	require.True(t, engine.Enabled())
	return engine
}

// TestCancelSession_DoesNotClobberTerminalStatus is the standing guard
// for DEFECT-1: CancelSession unconditionally overwrote a session's
// terminal Status (completed/failed/cancelled) with "cancelled" because
// it gated only on `CancelFunc != nil` (which is never cleared after a
// session terminates). For a QA orchestrator this is a §11.4 PASS-bluff:
// a session that genuinely COMPLETED — with a real Result + ReportPath —
// is silently relabelled "cancelled", destroying the truthful record of
// what actually happened. A late "stop" click on an already-finished
// session triggers it.
//
// Counterexample: insert a session in terminal state "completed" (with a
// non-nil CancelFunc, as the orchestrator leaves it), call
// CancelSession(id), then read Status. PRE-FIX it becomes "cancelled";
// POST-FIX it stays "completed".
func TestCancelSession_DoesNotClobberTerminalStatus(t *testing.T) {
	engine := newEnabledEngine(t)

	_, cancel := context.WithCancel(context.Background())
	completedAt := time.Now()
	state := &SessionState{
		ID:         "already-done",
		Status:     "completed",
		ReportPath: "/evidence/report.md",
		EndTime:    &completedAt,
		CancelFunc: cancel, // never cleared after termination — the trap
	}
	engine.sessionMu.Lock()
	engine.sessions[state.ID] = state
	engine.sessionMu.Unlock()

	if redMode() {
		// Reproduce the PRE-FIX behaviour inline: gate only on CancelFunc.
		state.Mu.Lock()
		if state.CancelFunc != nil {
			state.CancelFunc()
			state.Status = "cancelled"
			now := time.Now()
			state.EndTime = &now
		}
		state.Mu.Unlock()

		state.Mu.RLock()
		got := state.Status
		state.Mu.RUnlock()
		// The bug: a completed session is now reported as cancelled.
		require.Equal(t, "cancelled", got,
			"RED_MODE: pre-fix CancelSession clobbers terminal 'completed' to 'cancelled'")
		return
	}

	// POST-FIX: real production CancelSession must NOT touch a terminal
	// session's Status / EndTime.
	require.NoError(t, engine.CancelSession(state.ID))

	state.Mu.RLock()
	got := state.Status
	gotEnd := state.EndTime
	state.Mu.RUnlock()

	require.Equal(t, "completed", got,
		"CancelSession must not relabel a terminal 'completed' session as 'cancelled'")
	require.NotNil(t, gotEnd)
	require.Equal(t, completedAt, *gotEnd,
		"CancelSession must not overwrite a terminal session's EndTime")
}

// TestCancelSession_CancelsRunningSession proves the fix preserves the
// intended behaviour: a NON-terminal (running/pending) session IS
// transitioned to cancelled. Without this companion, the DEFECT-1 fix
// could trivially pass by making CancelSession a no-op.
func TestCancelSession_CancelsRunningSession(t *testing.T) {
	engine := newEnabledEngine(t)

	_, cancel := context.WithCancel(context.Background())
	state := &SessionState{
		ID:         "still-running",
		Status:     "running",
		CancelFunc: cancel,
	}
	engine.sessionMu.Lock()
	engine.sessions[state.ID] = state
	engine.sessionMu.Unlock()

	require.NoError(t, engine.CancelSession(state.ID))

	state.Mu.RLock()
	got := state.Status
	gotEnd := state.EndTime
	state.Mu.RUnlock()

	require.Equal(t, "cancelled", got,
		"CancelSession must transition a running session to cancelled")
	require.NotNil(t, gotEnd, "CancelSession must stamp EndTime on a freshly-cancelled session")
}
