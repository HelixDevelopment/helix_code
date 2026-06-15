package session

// Standing regression guard (§11.4.135) for the callback-registration data
// race in *Manager. The On* registrars used to append to the callback slices
// without holding m.mu, while the lifecycle methods (Create/Start/Pause/…)
// range over those same slices under the lock — a classic concurrent
// map/slice read-vs-write data race the Go race detector flags.
//
// §11.4.115 polarity: a single RED_MODE switch.
//   RED_MODE=1 — reproduce the defect on a FAITHFUL pre-fix stand-in: append
//                to a shared slice from one goroutine while another ranges it,
//                with NO synchronisation. Under -race this is the exact race
//                pattern the bug exhibited. The stand-in models the unguarded
//                registrar so the test proves the defect class is real
//                independent of the (now-fixed) production code.
//   RED_MODE=0 (default) — drive the REAL fixed *Manager: concurrently
//                register callbacks via OnCreate while another goroutine calls
//                Create (which ranges m.onCreate under the lock). With the fix
//                in place this is race-free; run under `go test -race` it must
//                pass clean.

import (
	"os"
	"sync"
	"testing"
)

func TestManagerCallbackRegistration_Race(t *testing.T) {
	if os.Getenv("RED_MODE") == "1" {
		// Faithful pre-fix stand-in: an unsynchronised slice mutated by one
		// goroutine while another reads it — the unguarded-registrar shape.
		// Under `go test -race` the detector reports a DATA RACE here, which
		// is exactly the defect this guard pins. Run WITHOUT -race it simply
		// exercises the pattern; the RED proof is the race report.
		var shared []func()
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				shared = append(shared, func() {}) // unguarded write
			}
		}()
		go func() {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				for range shared { // unguarded read
				}
			}
		}()
		wg.Wait()
		t.Log("RED_MODE: exercised the unguarded-registrar race stand-in " +
			"(run under -race to observe the DATA RACE the fix removes)")
		return
	}

	// GREEN: the real fixed code. Concurrent OnCreate registration vs Create
	// must be race-free now that every On* registrar holds m.mu.
	m := NewManager()
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 500; i++ {
			m.OnCreate(func(*Session) {})
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 500; i++ {
			if _, err := m.Create("proj", "name", "desc", ModeBuilding); err != nil {
				t.Errorf("Create failed: %v", err)
				return
			}
		}
	}()
	// Also hammer the other registrars concurrently to cover them all.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 500; i++ {
			m.OnStart(func(*Session) {})
			m.OnPause(func(*Session) {})
			m.OnResume(func(*Session) {})
			m.OnComplete(func(*Session) {})
			m.OnDelete(func(*Session) {})
			m.OnSwitch(func(_, _ *Session) {})
		}
	}()
	wg.Wait()
}
