// Package projectmemory — watcher_race_test.go (DEFECT-1 regression guard).
//
// §11.4.115 RED-on-broken-artifact + polarity switch. These tests reproduce the
// concurrent-Start data race AND the cancel→double-close-of-w.done panic on the
// PRE-FIX artifact, and stand as the permanent GREEN regression guard post-fix.
//
// The race/panic only manifests when two Start calls both pass the unguarded
// `if w.started` check before either sets it true: each spawns a runEventLoop
// goroutine, both share one w.done, and on ctx cancel BOTH run
// `defer close(w.done)` → panic: close of closed channel.
//
// The -race detector is the oracle for the data-race portion (concurrent
// read+write of w.started). The panic is deterministic once two loops spawn.
//
// Polarity (§11.4.115): there is no separate happy-path guard — these SAME tests
// are the bug-catcher AND the regression-guard. On the broken artifact they
// race/panic; on the fixed artifact they pass clean under -race.
package projectmemory

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestWatcher_ConcurrentStart_SingleLoop_NoDoubleClosePanic reproduces DEFECT-1:
// 16 goroutines call Start concurrently. On the broken artifact the unguarded
// started check lets ≥2 loops spawn (data race on w.started, flagged by -race),
// and cancelling the context makes both run `defer close(w.done)` → panic.
//
// On the fixed artifact exactly one loop spawns, there is no race, and cancel
// closes w.done exactly once — no panic.
func TestWatcher_ConcurrentStart_SingleLoop_NoDoubleClosePanic(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "helixcode.md"), []byte("X"), 0644))
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	r := NewMemoryRegistry(NewMemoryLoader(zap.NewNop()), dir)
	_, err := r.Reload(context.Background())
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	w := NewMemoryWatcher(r, zap.NewNop())

	// 16 concurrent Start calls. Under -race, the unguarded read+write of
	// w.started is reported as a data race; without -race, ≥2 loops spawn.
	const n = 16
	var wg sync.WaitGroup
	wg.Add(n)
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			<-start
			_ = w.Start(ctx)
		}()
	}
	close(start)
	wg.Wait()

	// Let any spawned loop reach its select.
	time.Sleep(50 * time.Millisecond)

	// Cancel: on the broken artifact, if >1 loop spawned, both run
	// `defer close(w.done)` → panic: close of closed channel. The fixed artifact
	// closes exactly once.
	cancel()
	time.Sleep(100 * time.Millisecond)

	// Close must be safe and not block (the loop(s) must have drained).
	require.NoError(t, w.Close())
}

// TestWatcher_ConcurrentStartThenClose_NoPanic is a second, Close-driven
// reproduction: concurrent Start followed immediately by Close. On the broken
// artifact two loops + watcher.Close() racing close(w.done) can double-close.
func TestWatcher_ConcurrentStartThenClose_NoPanic(t *testing.T) {
	for iter := 0; iter < 8; iter++ {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "helixcode.md"), []byte("X"), 0644))
		xdg := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", xdg)

		r := NewMemoryRegistry(NewMemoryLoader(zap.NewNop()), dir)
		_, err := r.Reload(context.Background())
		require.NoError(t, err)

		w := NewMemoryWatcher(r, zap.NewNop())

		const n = 16
		var wg sync.WaitGroup
		wg.Add(n)
		barrier := make(chan struct{})
		for i := 0; i < n; i++ {
			go func() {
				defer wg.Done()
				<-barrier
				_ = w.Start(context.Background())
			}()
		}
		close(barrier)
		wg.Wait()

		require.NoError(t, w.Close())
		// Second Close must remain idempotent.
		require.NoError(t, w.Close())
	}
}
