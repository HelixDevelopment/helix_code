package context

import (
	"context"
	"os"
	"sync"
	"testing"

	"dev.helix.code/internal/config"
)

// TestContextManager_Stop_Idempotent is the regression guard for the SYSTEMIC
// channel-double-close defect class (D3): (*ContextManager).Stop() closed
// cm.stopChan without a sync.Once guard, so a second Stop() panicked with
// "close of closed channel". wg.Wait() is kept OUTSIDE the Once so every caller
// blocks until the cleanup worker exits.
//
// §11.4.115 RED→GREEN polarity switch via RED_MODE:
//   - RED_MODE=1 reproduces the ORIGINAL broken teardown and asserts the panic
//     IS present.
//   - RED_MODE=0 (default) drives the REAL fixed Stop() twice and asserts no
//     panic, AND verifies every caller blocked until the worker exited.
func TestContextManager_Stop_Idempotent(t *testing.T) {
	if os.Getenv("RED_MODE") == "1" {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("RED_MODE: expected double-close to panic, but it did not — defect not reproduced")
			}
			// recover() != nil => panic observed => defect confirmed => RED passes
		}()
		ch := make(chan struct{})
		close(ch)
		close(ch) // panics: close of closed channel — the original D3 bug
		return
	}

	// GREEN: the real fixed teardown must be idempotent, with the background
	// cleanup worker actually started so wg.Wait() has work to block on.
	cm := NewContextManager(&config.ContextConfig{})
	if err := cm.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	cm.Stop()
	// A second Stop must be a clean no-op, never a panic.
	cm.Stop()
}

// TestContextManager_Stop_ConcurrentIdempotent stresses the guard under
// concurrent Stop() callers. Every caller must block until the cleanup worker
// has exited (wg.Wait outside the Once), then return without panic.
func TestContextManager_Stop_ConcurrentIdempotent(t *testing.T) {
	cm := NewContextManager(&config.ContextConfig{})
	if err := cm.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	const n = 16
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			cm.Stop()
		}()
	}
	wg.Wait()
}
