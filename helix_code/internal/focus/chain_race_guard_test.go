package focus

// Standing regression guard (§11.4.135) for the *Chain concurrency fix.
//
// Background (the defect this guards against): Manager hands LIVE *Chain
// pointers back to callers (GetChain / GetActiveChain / GetAllChains) while it
// concurrently mutates the SAME chain through PushToActive / MergeChains /
// CleanExpiredFocuses. Before the fix, *Chain had no internal lock, so a reader
// holding an escaped pointer raced a concurrent writer — a real DATA RACE on
// Focuses / CurrentIdx / Context / Metadata / UpdatedAt. The fix added a
// per-Chain sync.RWMutex guarding every mutable-field access, plus address-
// ordered dual-locking in Merge so a.Merge(b) and b.Merge(a) cannot deadlock.
//
// §11.4.115 polarity switch: RED_MODE=1 reproduces the historical defect on a
// faithful pre-fix stand-in (an unguarded chain-like type); under `go test
// -race` it trips DATA RACE (the proof the guard catches a real bug). RED_MODE=0
// (default) drives the REAL fixed Chain and asserts it runs clean under -race
// and completes without deadlock under the concurrent-Merge scenario.
//
// Run GREEN guard:   go test -race -run TestChainRaceGuard ./internal/focus/
// Reproduce RED:     RED_MODE=1 go test -race -run TestChainConcurrencyRED ./internal/focus/

import (
	"os"
	"sync"
	"testing"
	"time"
)

// redMode reports whether the RED reproduction polarity is selected.
func redMode() bool {
	v := os.Getenv("RED_MODE")
	return v == "1" || v == "true"
}

// unguardedChain is a FAITHFUL stand-in for the pre-fix *Chain: the same mutable
// fields, the same Push/read shapes, but WITHOUT the mutex. It exists only to
// reproduce the historical data race under -race so the guard provably catches a
// real bug (an analyzer that cannot fail on the known-broken artifact is a blind
// test). It is NEVER used by production code.
type unguardedChain struct {
	focuses    []*Focus
	currentIdx int
	updatedAt  time.Time
}

func (u *unguardedChain) push(f *Focus) {
	// Deliberately lock-free — mirrors the pre-fix Chain.Push body.
	u.focuses = append(u.focuses, f)
	u.currentIdx = len(u.focuses) - 1
	u.updatedAt = time.Now()
}

func (u *unguardedChain) size() int { return len(u.focuses) } // lock-free read

// TestChainConcurrencyRED reproduces the historical DATA RACE on a faithful
// pre-fix (unguarded) stand-in. Under `RED_MODE=1 go test -race` this MUST trip
// the race detector (concurrent unsynchronised read+write of focuses/currentIdx/
// updatedAt). When RED_MODE is unset it skips (it is a reproduction harness, not
// a standing assertion — the standing assertion is the GREEN guard below).
func TestChainConcurrencyRED(t *testing.T) {
	if !redMode() {
		t.Skip("SKIP-OK: RED reproduction harness; set RED_MODE=1 to reproduce the pre-fix data race under -race")
	}

	u := &unguardedChain{focuses: make([]*Focus, 0), currentIdx: -1}

	var wg sync.WaitGroup
	wg.Add(2)
	// Writer goroutine.
	go func() {
		defer wg.Done()
		for i := 0; i < 2000; i++ {
			u.push(NewFocus(FocusTypeTask, "red"))
		}
	}()
	// Reader goroutine — concurrent unsynchronised reads of the same fields.
	go func() {
		defer wg.Done()
		for i := 0; i < 2000; i++ {
			_ = u.size()
			_ = u.currentIdx
			_ = u.updatedAt
		}
	}()
	wg.Wait()

	// NOTE (deadlock-reproduction note for the pre-fix Merge): the pre-fix Merge
	// locked BOTH chains in call order (c then other). Two goroutines running
	// a.Merge(b) and b.Merge(a) concurrently would each grab their first lock and
	// block forever on the second — a classic lock-order inversion. The fix's
	// address-ordering eliminates it; the GREEN guard below proves the fixed
	// version completes. We do not model the pre-fix deadlock as a hanging test
	// here (a hang has no clean -race signal and would just time out the suite).
}

// TestChainRaceGuard is the STANDING GREEN guard (default, RED_MODE unset). It
// drives the REAL fixed Chain under maximum contention and asserts:
//   - clean under `go test -race` (every mutable-field access is locked), and
//   - no deadlock in the concurrent-Merge scenario (address-ordered dual-lock),
//
// bounded by a select/time.After watchdog so a regression deadlocks the test
// (t.Fatal) instead of hanging the whole suite.
func TestChainRaceGuard(t *testing.T) {
	if redMode() {
		t.Skip("SKIP-OK: RED_MODE set; this is the GREEN guard — run TestChainConcurrencyRED for the reproduction")
	}

	// Bound MaxSize so each Merge is O(MaxSize) work — the concurrent-Merge loop
	// below merges in BOTH directions, which without a cap accumulates focuses
	// super-linearly and would make the watchdog fire on slow growth rather than
	// on an actual deadlock. With the cap, completion-vs-timeout is a clean
	// deadlock signal (address-ordered dual-lock must keep both directions live).
	a := NewChainWithSize("guard-a", 64)
	b := NewChainWithSize("guard-b", 64)
	for i := 0; i < 5; i++ {
		if err := a.Push(NewFocus(FocusTypeTask, "seed-a")); err != nil {
			t.Fatalf("seed push a: %v", err)
		}
		if err := b.Push(NewFocus(FocusTypeTask, "seed-b")); err != nil {
			t.Fatalf("seed push b: %v", err)
		}
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		var wg sync.WaitGroup

		// Concurrent writers + readers on a single escaped chain pointer.
		wg.Add(2)
		go func() {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				_ = a.Push(NewFocus(FocusTypeTask, "w"))
				a.SetContext("k", i)
				a.SetMetadata("m", "v")
			}
		}()
		go func() {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				_ = a.Size()
				_, _ = a.Current()
				_ = a.GetRecent(3)
				_, _ = a.GetContext("k")
				_, _ = a.GetMetadata("m")
				_ = a.String()
			}
		}()

		// Concurrent opposite-direction merges on Clone() args — exercises Merge
		// under contention with an escaped writer, deadlock-free.
		wg.Add(2)
		go func() {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				_ = a.Merge(b.Clone())
			}
		}()
		go func() {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				_ = b.Merge(a.Clone())
			}
		}()

		// SAME-PAIR opposite-direction merges — the lock-order-inversion scenario
		// the address-ordering exists to prevent. c.Merge(d) and d.Merge(c) run
		// concurrently on the SAME two chains: a naive both-locks-in-call-order
		// Merge would deadlock here (each grabs its first lock, blocks on the
		// second forever); the address-ordered dual-lock keeps both live. The
		// bounded select watchdog converts a regression into t.Fatal, not a hang.
		c := NewChainWithSize("guard-c", 64)
		d := NewChainWithSize("guard-d", 64)
		for i := 0; i < 5; i++ {
			_ = c.Push(NewFocus(FocusTypeTask, "seed-c"))
			_ = d.Push(NewFocus(FocusTypeTask, "seed-d"))
		}
		wg.Add(2)
		go func() {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				_ = c.Merge(d)
			}
		}()
		go func() {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				_ = d.Merge(c)
			}
		}()

		wg.Wait()
	}()

	select {
	case <-done:
		// Completed without deadlock.
	case <-time.After(30 * time.Second):
		t.Fatal("deadlock: concurrent Chain Push/read/Merge did not complete within 30s")
	}
}
