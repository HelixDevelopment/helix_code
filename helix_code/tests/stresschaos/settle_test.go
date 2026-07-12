package stresschaos

import (
	"runtime"
	"sync"
	"testing"
	"time"
)

// TestSettle_PollUntilStable_WaitsOutDelayedGoroutineExit is the deterministic
// RED/GREEN regression guard for the HXC-144 settle-logic fix in
// settleGoroutines() / RunConcurrent (§11.4.115 polarity).
//
// RED (reproduced inline below, not by reverting production code): the
// original RunConcurrent settle step was a single
// `time.Sleep(50*time.Millisecond); runtime.GC()` sample. net/http
// client/server connection-teardown goroutines (persistConn.readLoop /
// writeLoop) exit ASYNCHRONOUSLY, independently of each other, and their exit
// is scheduler-timed, not synchronous with Close() — so goroutines that take
// longer than the fixed window to actually exit are still counted as
// "leaked" at snapshot time. This test plants goroutines with STAGGERED exit
// delays (60ms..160ms after release, in 20ms steps — the same
// independently-timed-exit shape real persistConn teardown has, deliberately
// NOT a simultaneous burst, which would let any stable-streak poll coincide
// with the burst window) and proves the OLD single-fixed-sleep protocol still
// counts every one of them as present at the 50ms mark (a false/flaky leak
// signal, exactly the HXC-144 measurement-window artifact class; see
// golang/go#25621, golang/go#9092).
//
// GREEN: settleGoroutines() (the function RunConcurrent now uses) polls up to
// settlePollBudget and correctly waits out every staggered exit, reporting
// the TRUE post-settle count.
func TestSettle_PollUntilStable_WaitsOutDelayedGoroutineExit(t *testing.T) {
	// Staggered so each goroutine's exit is independently observable — the
	// same shape real net/http connection teardown has (each persistConn
	// exits on its own schedule, not in lockstep).
	delays := []time.Duration{
		60 * time.Millisecond,
		80 * time.Millisecond,
		100 * time.Millisecond,
		120 * time.Millisecond,
		140 * time.Millisecond,
		160 * time.Millisecond,
	}
	delayedExitGoroutines := len(delays)

	runtime.GC()
	before := runtime.NumGoroutine()

	release := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(delayedExitGoroutines)
	for _, d := range delays {
		d := d
		go func() {
			defer wg.Done()
			<-release
			time.Sleep(d) // mimics asynchronous, independently-timed persistConn teardown
		}()
	}
	close(release)

	// --- RED: reproduce the exact OLD RunConcurrent settle protocol inline.
	// All staggered delays (60ms..160ms) are still >= the old fixed 50ms
	// window, so none have exited yet at the snapshot point. ---
	time.Sleep(50 * time.Millisecond)
	runtime.GC()
	oldProtocolAfter := runtime.NumGoroutine()
	oldProtocolDelta := oldProtocolAfter - before
	t.Logf("RED (old fixed-50ms-sleep protocol): before=%d after=%d delta=%d",
		before, oldProtocolAfter, oldProtocolDelta)
	if oldProtocolDelta < delayedExitGoroutines {
		t.Fatalf("RED reproduction failed to reproduce the HXC-144 artifact: expected the old fixed-50ms-sleep protocol to still count all %d staggered-exit (%v..%v) goroutines as present (delta>=%d), got delta=%d — the old-protocol simulation itself is broken, not the phenomenon it's meant to demonstrate",
			delayedExitGoroutines, delays[0], delays[len(delays)-1], delayedExitGoroutines, oldProtocolDelta)
	}
	if oldProtocolDelta > goroutineLeakTolerance {
		t.Logf("RED CONFIRMED: old fixed-sleep protocol's delta (%d) exceeds goroutineLeakTolerance (%d) — this is the exact false-leak signal HXC-144 diagnosed",
			oldProtocolDelta, goroutineLeakTolerance)
	}

	// --- GREEN: the current (fixed) poll-until-stable settle logic ---
	stableAfter := settleGoroutines()
	wg.Wait() // the planted goroutines must have already exited by now; this is just a safety net
	stableDelta := stableAfter - before
	t.Logf("GREEN (poll-until-stable settle): before=%d after=%d delta=%d",
		before, stableAfter, stableDelta)

	if stableDelta > goroutineLeakTolerance {
		t.Fatalf("settleGoroutines() did not wait out the planted staggered-exit (%v..%v) goroutines: before=%d after=%d delta=%d > tolerance %d",
			delays[0], delays[len(delays)-1], before, stableAfter, stableDelta, goroutineLeakTolerance)
	}
}
