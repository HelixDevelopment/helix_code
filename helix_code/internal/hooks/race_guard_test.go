package hooks

import (
	"context"
	"errors"
	"sync"
	"testing"
)

// race_guard_test.go — §11.4.115 RED→GREEN regression guards for three real
// data races in package hooks: callback/config setters mutated shared state
// with no lock while readers (async dispatch goroutines, Register, Trigger*)
// accessed that same state under the package mutexes.
//
// The Go -race detector is the oracle. On the PRE-FIX code each test below
// reliably reports "DATA RACE" and fails under `go test -race`; on the fixed
// code they pass cleanly. They are the permanent GREEN guards (§11.4.135):
// concurrent setter + dispatch must be race-free.

// raceErr is a sentinel so failing hooks drive the StatusFailed / onError path.
var raceErr = errors.New("race-guard induced failure")

// D1: Executor.OnComplete / OnError append to e.onComplete / e.onError with no
// lock, concurrent with triggerCallbacks ranging those slices from async hook
// goroutines (executeAsync -> triggerCallbacks). RED reproduces the race by
// registering callbacks while async hooks dispatch and fire callbacks.
func TestExecutor_OnComplete_OnError_ConcurrentWithDispatch_RaceFree_D1(t *testing.T) {
	e := NewExecutor()

	// A failing async hook so BOTH onComplete and onError fire.
	hook := NewAsyncHook("d1-hook", HookTypeCustom, func(ctx context.Context, ev *Event) error {
		return raceErr
	})

	var wg sync.WaitGroup

	// Writer goroutines: continuously register callbacks (the racing writes).
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				e.OnComplete(func(*ExecutionResult) {})
				e.OnError(func(*ExecutionResult) {})
			}
		}()
	}

	// Reader goroutines: dispatch async hooks, whose completion ranges the
	// onComplete / onError slices in triggerCallbacks.
	for r := 0; r < 4; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				e.Execute(context.Background(), hook, NewEvent(HookTypeCustom))
			}
		}()
	}

	wg.Wait()
	e.Wait()
}

// D2: Executor.SetMaxConcurrent writes e.maxConcurrent AND replaces e.semaphore
// with no lock, concurrent with async goroutines reading e.semaphore in
// executeAsync. RED reproduces by swapping the semaphore while async hooks
// acquire/release it.
func TestExecutor_SetMaxConcurrent_ConcurrentWithDispatch_RaceFree_D2(t *testing.T) {
	e := NewExecutor()

	hook := NewAsyncHook("d2-hook", HookTypeCustom, func(ctx context.Context, ev *Event) error {
		return nil
	})

	var wg sync.WaitGroup

	// Writer: continuously change the concurrency limit (swaps e.semaphore).
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 500; i++ {
			e.SetMaxConcurrent((i % 8) + 1)
		}
	}()

	// Readers: dispatch async hooks which read e.semaphore.
	for r := 0; r < 4; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 300; i++ {
				e.Execute(context.Background(), hook, NewEvent(HookTypeCustom))
			}
		}()
	}

	wg.Wait()
	e.Wait()
}

// D3: Manager.OnCreate / OnExecute append to m.onCreate / m.onExecute with no
// lock, concurrent with Register (reads m.onCreate under m.mu) and TriggerEvent
// (reads m.onExecute). RED reproduces by registering callbacks while hooks are
// registered and events triggered.
func TestManager_OnCreate_OnExecute_ConcurrentWithRegisterTrigger_RaceFree_D3(t *testing.T) {
	m := NewManager()

	var wg sync.WaitGroup
	var idCounter int64
	var idMu sync.Mutex
	nextID := func() string {
		idMu.Lock()
		defer idMu.Unlock()
		idCounter++
		// Distinct names → distinct IDs so Register does not collide.
		return string(rune('a'+(idCounter%26))) + "-" + itoa(idCounter)
	}

	// Writers: continuously register lifecycle callbacks.
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				m.OnCreate(func(*Hook) {})
				m.OnExecute(func(*Event, []*ExecutionResult) {})
			}
		}()
	}

	// Readers: Register hooks (reads m.onCreate) and Trigger events (reads m.onExecute).
	for r := 0; r < 4; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				h := NewHook(nextID(), HookTypeCustom, func(ctx context.Context, ev *Event) error {
					return nil
				})
				_ = m.Register(h)
				m.Trigger(context.Background(), HookTypeCustom)
			}
		}()
	}

	wg.Wait()
	m.Wait()
}

// itoa is a tiny allocation-free-ish integer formatter to avoid importing
// strconv solely for unique-ID generation in the D3 reader loop.
func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
