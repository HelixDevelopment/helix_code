package notification

import (
	"context"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// redMode reports whether the RED-baseline polarity is active (§11.4.115).
//
// Default (RED_MODE unset or "0") = GREEN regression-guard role: assert the
// defect is ABSENT on the fixed artifact. This is the STANDING guard that runs
// on every `go test ./...` and blocks the release on any failure (§11.4.135).
//
// RED_MODE=1 = reproduce-on-broken-artifact role: assert the defect is PRESENT,
// capturing positive evidence that the historical defect was real. Running
// RED_MODE=1 against the FIXED artifact is EXPECTED to fail — that is the
// polarity switch proving the fix changed observable behaviour.
//
// One source, two roles: the bug-catcher IS the permanent regression guard.
func redMode() bool {
	return os.Getenv("RED_MODE") == "1"
}

// TestNotificationQueue_Regression_DoubleStopNoPanic guards DEFECT-1
// (NotificationQueue.Stop() double-close panic "close of closed channel").
//
//	RED_MODE=1: prove a second Stop() panics on the broken artifact.
//	RED_MODE=0: GREEN guard — Stop() is idempotent, a second call never panics.
func TestNotificationQueue_Regression_DoubleStopNoPanic(t *testing.T) {
	engine := NewNotificationEngine()
	queue := NewNotificationQueue(engine, 1, 10)
	queue.Start()

	queue.Stop()

	if redMode() {
		// RED baseline: the second Stop() MUST panic on the broken artifact.
		// Capturing that panic is the positive evidence the defect is real.
		var panicked bool
		func() {
			defer func() {
				if r := recover(); r != nil {
					panicked = true
					t.Logf("RED evidence: second Stop() panicked as expected: %v", r)
				}
			}()
			queue.Stop()
		}()
		if !panicked {
			t.Fatal("RED expectation unmet: second Stop() did NOT panic — " +
				"either the defect is already fixed (flip RED_MODE=0) or the test is blind")
		}
		return
	}

	// GREEN guard: a second (and third) Stop() MUST be a clean no-op, never panic.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("GREEN regression: second Stop() panicked: %v", r)
		}
	}()
	queue.Stop()
	queue.Stop()
}

// TestNotificationQueue_Regression_StopStillStopsWorkers proves the idempotent
// Stop() still genuinely stops the workers (wg.Wait completes promptly) — the
// fix must not turn Stop() into a no-op that leaks goroutines.
func TestNotificationQueue_Regression_StopStillStopsWorkers(t *testing.T) {
	if redMode() {
		t.Skip("GREEN-only guard; runs under RED_MODE=0")
	}
	engine := NewNotificationEngine()
	queue := NewNotificationQueue(engine, 4, 10)
	queue.Start()

	done := make(chan struct{})
	go func() {
		queue.Stop()
		close(done)
	}()
	select {
	case <-done:
		// Stop() returned, meaning wg.Wait() unblocked => all 4 workers exited.
	case <-time.After(5 * time.Second):
		t.Fatal("Stop() did not return within 5s — workers were not stopped (wg.Wait hung)")
	}
}

// TestNotificationQueue_Regression_GetStatsRaceFree guards DEFECT-2
// (GetStats() handing out the live shared *QueueStats while workers mutate it).
//
// This test concurrently READS the documented stats fields returned by
// GetStats() while worker goroutines (and direct Enqueue calls) WRITE those
// same int64 fields. Under `go test -race`:
//
//	RED_MODE=1: on the broken artifact the race detector flags a data race on
//	            QueueStats.Enqueued (read in the test, write in Enqueue), failing
//	            the test — the positive evidence the defect is real.
//	RED_MODE=0: GREEN guard — GetStats() returns a value snapshot taken under the
//	            mutex, so reading its fields can never race with worker writes.
//
// The test FAILS under -race iff the data race exists. Without -race the race
// is latent; the standing guard MUST be run as `go test -race`.
func TestNotificationQueue_Regression_GetStatsRaceFree(t *testing.T) {
	engine := NewNotificationEngine()
	// Register a sink so workers actually run processNext -> stats writes.
	mockCh := &retryMockChannel{
		sendFunc: func(ctx context.Context, notif *Notification) error { return nil },
	}
	engine.RegisterChannel(mockCh)

	queue := NewNotificationQueue(engine, 4, 0) // unbounded, 4 workers
	queue.Start()
	defer queue.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup

	// Writers: hammer Enqueue (writes stats.Enqueued under the stats mutex) and
	// let the 4 workers write stats.Dequeued/Succeeded/Failed concurrently.
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					_ = queue.Enqueue(&Notification{
						Title: "race", Message: "race", Type: NotificationTypeInfo,
					}, []string{"mock"}, 1)
				}
			}
		}()
	}

	// Readers: read the documented public stats fields via GetStats().
	// On the broken artifact these reads happen on the live shared struct with
	// NO lock held (GetStats released the mutex on return) while writers mutate
	// it -> data race flagged by -race.
	var observed int64
	for r := 0; r < 4; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					s := queue.GetStats()
					// Read every documented field — this is the unlocked read on
					// the broken artifact.
					atomic.StoreInt64(&observed,
						s.Enqueued+s.Dequeued+s.Failed+s.Succeeded)
				}
			}
		}()
	}

	time.Sleep(300 * time.Millisecond)
	cancel()
	wg.Wait()

	if redMode() {
		// If we reach here under RED_MODE=1 + -race WITHOUT the detector aborting
		// the process, the race did not fire. That can mean the defect is already
		// fixed (flip RED_MODE=0) or -race was not enabled.
		t.Log("RED note: reached end without -race abort — run with `go test -race` " +
			"on the BROKEN artifact to capture the data-race evidence; if fixed, set RED_MODE=0")
	}
	// GREEN guard simply asserts the concurrent access completed cleanly; the
	// real assertion is delegated to the -race detector (no race => pass).
	_ = atomic.LoadInt64(&observed)
}
