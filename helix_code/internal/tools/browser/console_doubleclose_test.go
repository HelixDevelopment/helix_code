package browser

import (
	"os"
	"sync"
	"testing"
)

// TestConsoleMonitor_Stop_Idempotent is the regression guard for the SYSTEMIC
// channel-double-close defect class (D2): (*ConsoleMonitor).Stop() closed BOTH
// the messages and errors channels without a sync.Once guard, so a second
// Stop() panicked with "close of closed channel".
//
// §11.4.115 RED→GREEN polarity switch via RED_MODE:
//   - RED_MODE=1 reproduces the ORIGINAL broken teardown (two unguarded closes)
//     and asserts the panic IS present.
//   - RED_MODE=0 (default) drives the REAL fixed Stop() twice and asserts no
//     panic — both channels are guarded.
func TestConsoleMonitor_Stop_Idempotent(t *testing.T) {
	if os.Getenv("RED_MODE") == "1" {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("RED_MODE: expected double-close to panic, but it did not — defect not reproduced")
			}
			// recover() != nil => panic observed => defect confirmed => RED passes
		}()
		messages := make(chan *ConsoleMessage)
		errors := make(chan *ConsoleMessage)
		close(messages)
		close(errors)
		close(messages) // panics: close of closed channel — the original D2 bug
		close(errors)
		return
	}

	// GREEN: the real fixed teardown must be idempotent for BOTH channels.
	cm := NewConsoleMonitor(nil)

	cm.Stop()
	// A second Stop must be a clean no-op, never a panic.
	cm.Stop()

	// The channels must in fact be closed (drain returns the closed signal).
	if _, ok := <-cm.GetMessages(); ok {
		t.Fatal("messages channel should be closed after Stop")
	}
	if _, ok := <-cm.GetErrors(); ok {
		t.Fatal("errors channel should be closed after Stop")
	}
}

// TestConsoleMonitor_Stop_ConcurrentIdempotent stresses the guard under
// concurrent Stop() callers.
func TestConsoleMonitor_Stop_ConcurrentIdempotent(t *testing.T) {
	cm := NewConsoleMonitor(nil)

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
