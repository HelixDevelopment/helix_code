package browser

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// serializeChromiumLaunches acquires an exclusive advisory file lock used as
// a global mutex across every Go test binary that exercises chromium. Real
// chromium launches contend for CPU, file descriptors, and TCP ports; running
// many concurrently — as `go test ./...` does by default, with one binary per
// package — produced "context deadline exceeded" flakes that are NOT a bug
// in the browser tool but a load-driven environmental race.
//
// Earlier revisions handled this by gating the chromium tests behind
// `testing.Short()` and skipping them in the short-mode suite. That defeated
// the anti-bluff mandate (CONST-035 / Article XI §11.9): the gate said "test
// passes" when in fact the test was skipped, hiding the work the test claims
// to certify.
//
// This helper instead serialises chromium launches across the entire `go
// test` process tree. The lock is held only for the duration of the test
// body, so adjacent non-browser tests keep running in parallel. Under the
// lock, exactly one chromium starts at a time across all test binaries; the
// underlying race is resolved at root, not masked.
//
// The returned unlock func must be deferred. It is safe to call on the same
// goroutine that acquired the lock; the underlying flock is released when
// the FD is closed.
func serializeChromiumLaunches(t *testing.T) func() {
	t.Helper()

	// Lock-file lives in the OS tempdir so the lock spans every test
	// binary launched under `go test ./...` — they all share /tmp on a
	// given run, and `go test` does not isolate it per-binary.
	lockPath := filepath.Join(os.TempDir(), "helixcode-chromium-test.lock")

	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatalf("failed to open chromium serialisation lock %s: %v", lockPath, err)
	}

	// LOCK_EX = exclusive, blocks until acquired. No timeout: chromium tests
	// finish in tens of seconds each, the queue depth is bounded by the
	// number of test binaries, and waiting is the whole point.
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		_ = f.Close()
		t.Fatalf("failed to acquire chromium serialisation lock: %v", err)
	}

	return func() {
		// Releasing the lock is best-effort on shutdown — if either call
		// fails the OS will reclaim the FD when the process exits, and the
		// next test binary will simply acquire the lock against an empty
		// holder. Logging the error keeps the failure visible without
		// breaking the test.
		if err := syscall.Flock(int(f.Fd()), syscall.LOCK_UN); err != nil {
			t.Logf("warning: failed to release chromium lock: %v", err)
		}
		if err := f.Close(); err != nil {
			t.Logf("warning: failed to close chromium lock file: %v", err)
		}
	}
}
