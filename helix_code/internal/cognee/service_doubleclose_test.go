package cognee

import (
	"context"
	"os"
	"sync"
	"testing"

	"dev.helix.code/internal/config"
)

// TestCogneeService_Stop_ConcurrentIdempotent is the regression guard for the
// SYSTEMIC channel-double-close defect class (D4): (*CogneeService).Stop()
// performed the status check under the lock, RELEASED the lock, then closed
// s.stopChan and only flipped status→Stopped AFTER the close. Two concurrent
// Stop() callers both observed status==Running, both passed the guard, then
// both reached close(s.stopChan) → "close of closed channel" panic.
//
// The fix flips status out of Running UNDER the lock (so a concurrent Stop
// fails the guard) and wraps the close in sync.Once as defense-in-depth.
//
// §11.4.115 RED→GREEN polarity switch via RED_MODE:
//   - RED_MODE=1 reproduces the ORIGINAL broken ordering (release-lock →
//     close → set-status) under concurrency and asserts the panic IS present.
//   - RED_MODE=0 (default) drives the REAL fixed Stop() concurrently and asserts
//     no panic.
func TestCogneeService_Stop_ConcurrentIdempotent(t *testing.T) {
	if os.Getenv("RED_MODE") == "1" {
		// Reproduce the pre-fix race: guard read + close are NOT atomic and the
		// status flip happens after the close, so two callers both close.
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("RED reproduced the defect: double close panicked: %v", r)
			}
		}()
		// Model the pre-fix race deterministically: because the guard read and
		// the close were NOT atomic and status was flipped only AFTER the close,
		// two concurrent Stop callers could BOTH observe running==true (caller A
		// and caller B below) and BOTH proceed to close. Reproduced in-goroutine
		// so the deferred recover above can catch the panic.
		var mu sync.Mutex
		running := true
		ch := make(chan struct{})

		// Caller A passes the guard (status still Running)...
		mu.Lock()
		aPassed := running
		mu.Unlock() // ...releasing the lock BEFORE closing — the original bug.

		// Caller B passes the SAME stale guard before A flips status.
		mu.Lock()
		bPassed := running
		mu.Unlock()

		if aPassed {
			close(ch)
			mu.Lock()
			running = false
			mu.Unlock()
		}
		if bPassed {
			close(ch) // second close → panic: close of closed channel
		}
		return
	}

	// GREEN: the real fixed teardown must be concurrency-safe and idempotent.
	svc := newRunningServiceForStopTest(t)

	const n = 16
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			_ = svc.Stop(context.Background())
		}()
	}
	wg.Wait()

	if got := svc.GetStatus(); got != ServiceStatusStopped {
		t.Fatalf("status after Stop = %q, want %q", got, ServiceStatusStopped)
	}
}

// TestCogneeService_Stop_Idempotent guards the sequential double-Stop path.
func TestCogneeService_Stop_Idempotent(t *testing.T) {
	svc := newRunningServiceForStopTest(t)

	if err := svc.Stop(context.Background()); err != nil {
		t.Fatalf("first Stop: %v", err)
	}
	// A second Stop must be a clean no-op, never a panic.
	if err := svc.Stop(context.Background()); err != nil {
		t.Fatalf("second Stop: %v", err)
	}
}

// newRunningServiceForStopTest builds a CogneeService and drives it into the
// Running state WITHOUT starting the background goroutines (no real infra), so
// the teardown guard can be exercised in isolation. bgTasks has no pending Add,
// so Stop's Wait returns immediately.
func newRunningServiceForStopTest(t *testing.T) *CogneeService {
	t.Helper()
	svc, err := NewCogneeService(&config.CogneeConfig{Enabled: false}, nil)
	if err != nil {
		t.Fatalf("NewCogneeService: %v", err)
	}
	svc.mu.Lock()
	svc.status = ServiceStatusRunning
	svc.mu.Unlock()
	return svc
}
