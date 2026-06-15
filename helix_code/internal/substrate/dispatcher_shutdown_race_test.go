package substrate

// §11.4.135 STANDING regression guard for the Dispatcher.Dispatch vs
// Dispatcher.Shutdown DATA RACE / "panic: send on closed channel".
//
// Root cause (FACT, reproduced under `go test -race`): pre-fix, Dispatch called
// pool.SubmitWait (whose `p.tasks <- task` send is unsynchronised) concurrently
// with Shutdown's pool.Shutdown (whose `close(p.tasks)` is likewise
// unsynchronised). A send racing the close panics "send on closed channel" and
// the race detector flags a DATA RACE on the channel state.
//
// Fix: Dispatch holds d.mu.RLock() across the submit; Shutdown holds the
// exclusive d.mu.Lock() across the pool teardown + marks d.shutdown so later
// Dispatch calls return ErrDispatcherShutdown. RLock excludes the write Lock, so
// a send can never run concurrently with the close.
//
// §11.4.115 polarity switch via RED_MODE (default "0" = standing GREEN guard):
//   - RED_MODE=1 drives a FAITHFUL pre-fix stand-in (raw pool, unsynchronised
//     SubmitWait raced with Shutdown) and asserts the defect manifests (a
//     recovered "send on closed channel" panic and/or the race detector trips).
//   - RED_MODE=0 (default) drives the REAL fixed Dispatcher under the same
//     concurrent Dispatch+Shutdown load and asserts NO panic occurs.

import (
	"context"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"digital.vasic.concurrency/pkg/pool"
)

func redModeSubstrate() bool { return os.Getenv("RED_MODE") == "1" }

func newTestUnit(id string) Unit {
	return NewUnitFunc(id, "", PriorityNormal, func(ctx context.Context) (interface{}, error) {
		return id, nil
	})
}

// TestDispatcher_DispatchShutdownRace is the polarity-switch guard.
func TestDispatcher_DispatchShutdownRace(t *testing.T) {
	if redModeSubstrate() {
		// RED: faithful pre-fix reproduction on a raw pool with NO Dispatcher
		// lock — concurrent SubmitWait sends racing a Shutdown close.
		var panicked atomic.Bool
		for iter := 0; iter < 50; iter++ {
			cfg := pool.DefaultPoolConfig()
			cfg.Workers = 4
			wp := pool.NewWorkerPool(cfg)
			wp.Start()
			var wg sync.WaitGroup
			for i := 0; i < 8; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					defer func() {
						if r := recover(); r != nil {
							panicked.Store(true) // "send on closed channel" — the defect
						}
					}()
					_, _ = wp.SubmitWait(context.Background(), unitTask{unit: newTestUnit("u")})
				}()
			}
			wg.Add(1)
			go func() { defer wg.Done(); _ = wp.Shutdown(time.Second) }()
			wg.Wait()
			if panicked.Load() {
				break
			}
		}
		// The reliable reproduction ORACLE is the race detector: RED_MODE=1 is
		// only meaningful under `-race`, where the unsynchronised send/close is
		// flagged as a DATA RACE on every run (the exit-code failure IS the
		// reproduction). The "send on closed channel" panic is a secondary,
		// non-deterministic signal. To avoid a blind PASS when invoked WITHOUT
		// -race and no panic happened to fire, fail explicitly so a bare
		// `RED_MODE=1 go test` never falsely reports the defect as absent.
		if !panicked.Load() && !raceEnabled {
			t.Fatal("RED_MODE=1 requires -race to reproduce the Dispatch/Shutdown DATA RACE; run `RED_MODE=1 go test -race`")
		}
		return
	}

	// GREEN: the REAL fixed Dispatcher must never panic under concurrent
	// Dispatch + Shutdown.
	var panicked atomic.Bool
	for iter := 0; iter < 50; iter++ {
		d := NewDispatcher(4, NewResolver())
		var wg sync.WaitGroup
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer func() {
					if r := recover(); r != nil {
						panicked.Store(true)
					}
				}()
				_ = d.Dispatch(context.Background(), newTestUnit("u"))
			}()
		}
		wg.Add(1)
		go func() { defer wg.Done(); _ = d.Shutdown(time.Second) }()
		wg.Wait()
	}
	if panicked.Load() {
		t.Fatal("REGRESSION: Dispatch/Shutdown raced into a panic (send on closed channel) — the RWMutex guard is not protecting the submit against the teardown")
	}
}

// TestDispatcher_ShutdownIdempotent guards that repeated + concurrent Shutdown
// is a clean no-op (the sync.Once never double-closes the pool).
func TestDispatcher_ShutdownIdempotent(t *testing.T) {
	d := NewDispatcher(2, NewResolver())
	// Sequential double-shutdown.
	_ = d.Shutdown(time.Second)
	if err := func() (err error) {
		defer func() {
			if r := recover(); r != nil {
				err = errFromPanic(r)
			}
		}()
		return d.Shutdown(time.Second)
	}(); err != nil {
		t.Fatalf("second Shutdown panicked/errored: %v", err)
	}

	// Concurrent shutdown on a fresh dispatcher.
	d2 := NewDispatcher(2, NewResolver())
	const n = 16
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("concurrent Shutdown panicked: %v", r)
				}
			}()
			_ = d2.Shutdown(time.Second)
		}()
	}
	wg.Wait()
}

// TestDispatcher_DispatchAfterShutdown guards that dispatching onto a shut-down
// dispatcher fails cleanly with ErrDispatcherShutdown, never a panic.
func TestDispatcher_DispatchAfterShutdown(t *testing.T) {
	d := NewDispatcher(2, NewResolver())
	_ = d.Shutdown(time.Second)
	res := d.Dispatch(context.Background(), newTestUnit("after"))
	if res.Err == nil {
		t.Fatal("Dispatch after Shutdown returned no error; want ErrDispatcherShutdown")
	}
	if res.Err != ErrDispatcherShutdown {
		t.Fatalf("Dispatch after Shutdown err = %v; want ErrDispatcherShutdown", res.Err)
	}
}

func errFromPanic(r interface{}) error {
	if e, ok := r.(error); ok {
		return e
	}
	return &panicErr{r}
}

type panicErr struct{ v interface{} }

func (p *panicErr) Error() string { return "panic: " + toString(p.v) }
func toString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return "non-string panic"
}
