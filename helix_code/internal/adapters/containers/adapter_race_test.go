// Standing regression guard (§11.4.135) for the §11.4.85 data race in
// the containers Adapter: initRuntime()'s sync.Once block mutated the
// shared fields (a.rt, a.rtName, a.rtDetected, a.compose) WITHOUT
// holding a.mu, while the getters RuntimeName() / RuntimeAvailable() /
// ListContainers() read those same fields under a.mu.RLock(). sync.Once
// only establishes a happens-before edge for callers that route THROUGH
// initRuntime; a mutex-guarded getter that reads a.rtName without first
// calling initRuntime shares no synchronisation with the Once writer, so
// the write was a genuine data race (reproduced: adapter.go:72 write vs
// adapter.go:96 read under `go test -race`).
//
// §11.4.115 polarity switch:
//   RED_MODE=1  → reproduce the PRE-FIX pattern (a faithful inline
//                 replica of the old unsynchronised Once-write racing a
//                 mutex-guarded read). Under `-race` this trips the
//                 detector, proving the guard genuinely catches the
//                 race class. The replica is a local type so the test
//                 cannot accidentally pass by exercising already-fixed
//                 production code.
//   RED_MODE=0  → (default, no env) drive the REAL fixed Adapter:
//                 concurrent initRuntime() writer vs RuntimeName() /
//                 RuntimeAvailable() readers. Under `-race` this MUST be
//                 clean — the standing GREEN regression guard.
//
// Mocks ALLOWED here per CONST-050(A): this is a unit *_test.go file.
package containers

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"
)

// redMode reports whether the §11.4.115 reproduction polarity is active.
func redMode() bool { return os.Getenv("RED_MODE") == "1" }

// legacyAdapter is a faithful inline replica of the PRE-FIX containers
// Adapter synchronisation: the Once block writes the shared fields with
// NO mutex, while the getter reads them under the RWMutex. This exists
// only to prove (RED_MODE=1, under -race) that the guard catches the
// race class the production fix removed.
type legacyAdapter struct {
	mu         sync.RWMutex
	initOnce   sync.Once
	rtName     string
	rtDetected bool
}

func (l *legacyAdapter) initRuntimeBuggy() {
	l.initOnce.Do(func() {
		// PRE-FIX: no l.mu held around these writes — the exact defect.
		l.rtName = "podman"
		l.rtDetected = true
	})
}

func (l *legacyAdapter) name() string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.rtName
}

func (l *legacyAdapter) available() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.rtDetected
}

// TestContainersAdapter_InitRuntime_NoRace_RED85 is the standing race
// guard. Default polarity drives the real fixed Adapter and MUST be
// race-clean; RED_MODE=1 drives the legacy replica that reproduces the
// pre-fix race (trips `-race`).
func TestContainersAdapter_InitRuntime_NoRace_RED85(t *testing.T) {
	const iterations = 60
	for i := 0; i < iterations; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

		var wg sync.WaitGroup
		wg.Add(2)

		if redMode() {
			// RED_MODE=1: reproduce the pre-fix unsynchronised pattern.
			la := &legacyAdapter{}
			go func() { defer wg.Done(); la.initRuntimeBuggy() }()
			go func() { defer wg.Done(); _ = la.name(); _ = la.available() }()
		} else {
			// RED_MODE=0 (default): drive the REAL fixed production code.
			a := NewAdapter()
			go func() { defer wg.Done(); _ = a.initRuntime(ctx) }()
			go func() {
				defer wg.Done()
				_ = a.RuntimeName()
				_ = a.RuntimeAvailable(ctx)
			}()
		}

		wg.Wait()
		cancel()
	}

	// In default (GREEN) mode the test asserts behavioural correctness in
	// addition to race-freedom: after initRuntime the getters agree with
	// the detected state. (When no runtime is present this is a no-op;
	// when podman/docker is present rtDetected must be true and the name
	// non-empty — never a half-written field.)
	if !redMode() {
		a := NewAdapter()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := a.initRuntime(ctx); err == nil {
			if a.RuntimeAvailable(ctx) && a.RuntimeName() == "" {
				t.Fatalf("runtime detected but RuntimeName() empty — torn field read")
			}
		}
	}
}
