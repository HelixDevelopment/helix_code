package cmd

// Round-42 §11.4 anti-bluff coverage for runWatch / runWatchLoop — proves the
// fsnotify wiring delivers what the watcher banner advertises (real
// filesystem-event detection with debounce + clean ctx-cancellation +
// add-error propagation). CONST-035 / CONST-050(A) / Article XI §11.9.
//
// These are unit-level tests: ephemeral temp directories created by
// t.TempDir(), no external services, no mocks of fsnotify (the real watcher
// is exercised against the real kernel notify API). CONST-050(A)-compliant
// (this is a *_test.go file invoked without the integration build tag — the
// only layer where fakes are permitted, and we do not use any).

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestRunWatchLoop_DetectsFileChange writes a file inside a watched temp
// directory and asserts the onChange callback fires within a generous
// wall-clock budget (1 s). Proves the fsnotify wiring actually surfaces
// real filesystem events (not a polling loop wearing an fsnotify hat).
func TestRunWatchLoop_DetectsFileChange(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	fired := make(chan struct{}, 8)
	onChange := func() { fired <- struct{}{} }

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- runWatchLoop(ctx, []string{dir}, 20*time.Millisecond, &bytes.Buffer{}, onChange)
	}()

	// Allow the watcher goroutine to register the path before we mutate it.
	time.Sleep(50 * time.Millisecond)

	target := filepath.Join(dir, "trigger.txt")
	if err := os.WriteFile(target, []byte("hello"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	select {
	case <-fired:
		// success — at least one debounced refresh observed.
	case <-time.After(1 * time.Second):
		t.Fatal("onChange did not fire within 1s of a real file write inside the watched dir")
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runWatchLoop returned non-nil after cancel: %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("runWatchLoop did not return within 500ms of ctx cancellation")
	}
}

// TestRunWatchLoop_DebouncesBurstEvents writes multiple files inside the
// debounce window and asserts onChange fires exactly once. Proves the
// debounce design promised by the banner ("events debounced 80 ms") is
// real, not advisory.
func TestRunWatchLoop_DebouncesBurstEvents(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	var fireCount int32
	onChange := func() { atomic.AddInt32(&fireCount, 1) }

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Use a long debounce window (250 ms) so we can write the whole burst
	// inside one window without racing the timer.
	debounce := 250 * time.Millisecond

	done := make(chan error, 1)
	go func() {
		done <- runWatchLoop(ctx, []string{dir}, debounce, &bytes.Buffer{}, onChange)
	}()

	time.Sleep(50 * time.Millisecond)

	// Fire 5 writes well inside the debounce window.
	for i := 0; i < 5; i++ {
		f := filepath.Join(dir, "burst.txt")
		if err := os.WriteFile(f, []byte{byte(i)}, 0o600); err != nil {
			t.Fatalf("WriteFile burst %d: %v", i, err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Wait long enough for the debounce timer to elapse + onChange to run.
	time.Sleep(debounce + 200*time.Millisecond)

	got := atomic.LoadInt32(&fireCount)
	if got != 1 {
		t.Fatalf("expected exactly 1 debounced onChange invocation for a 5-event burst, got %d", got)
	}

	cancel()
	<-done
}

// TestRunWatchLoop_HonoursContextCancel cancels the context and asserts the
// loop returns nil within a generous deadline — proves ctx.Done() is wired
// and the watcher does not leak its goroutine.
func TestRunWatchLoop_HonoursContextCancel(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- runWatchLoop(ctx, []string{dir}, 50*time.Millisecond, &bytes.Buffer{}, func() {})
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runWatchLoop returned non-nil after cancel: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("runWatchLoop did not return within 1s of ctx cancellation — likely goroutine leak")
	}
}

// TestRunWatchLoop_PropagatesAddError tries to watch a path that does not
// exist and asserts the error is returned with the offending path in the
// message (so operators can see which entry of paths[] failed). Proves the
// wrapper does not silently swallow add errors — a §11.4 PASS-bluff class
// failure mode.
func TestRunWatchLoop_PropagatesAddError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	missing := filepath.Join(dir, "does-not-exist-deliberate")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := runWatchLoop(ctx, []string{missing}, 50*time.Millisecond, &bytes.Buffer{}, func() {})
	if err == nil {
		t.Fatal("expected non-nil error when watching a nonexistent path, got nil")
	}
	if !errorMentionsPath(err, missing) {
		t.Fatalf("error %q does not mention offending path %q — operator cannot diagnose which add failed", err, missing)
	}
}

// TestRunWatchLoop_IgnoresChmodOnlyEvents writes a file *before* the loop
// starts, then chmods it after. Chmod alone must NOT trigger onChange —
// proves the Op filter is in place and the loop does not produce refresh
// storms from permission fixups (a real bug we saw during prototyping).
func TestRunWatchLoop_IgnoresChmodOnlyEvents(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	target := filepath.Join(dir, "preexisting.txt")
	if err := os.WriteFile(target, []byte("seed"), 0o600); err != nil {
		t.Fatalf("WriteFile seed: %v", err)
	}

	var fireCount int32
	var firedMu sync.Mutex
	onChange := func() {
		firedMu.Lock()
		defer firedMu.Unlock()
		atomic.AddInt32(&fireCount, 1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- runWatchLoop(ctx, []string{dir}, 50*time.Millisecond, &bytes.Buffer{}, onChange)
	}()

	time.Sleep(50 * time.Millisecond)

	// Chmod-only — must be filtered out.
	if err := os.Chmod(target, 0o644); err != nil {
		t.Fatalf("Chmod: %v", err)
	}

	// Wait several debounce windows.
	time.Sleep(300 * time.Millisecond)

	if got := atomic.LoadInt32(&fireCount); got != 0 {
		t.Fatalf("expected 0 onChange invocations for chmod-only event, got %d (Op filter not honoured)", got)
	}

	cancel()
	<-done
}

// errorMentionsPath returns true if err.Error() contains substr.
func errorMentionsPath(err error, substr string) bool {
	if err == nil {
		return false
	}
	return bytes.Contains([]byte(err.Error()), []byte(substr))
}

// Ensure the standard-library error sentinel is reachable for any future
// errors.Is checks if we promote a wrapped error to a sentinel later.
var _ = errors.Is
