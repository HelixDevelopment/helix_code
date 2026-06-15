package session

// Standing regression guard (§11.4.135) for the callback-reentrancy DEADLOCK
// in *Manager. Once the On* registrars started taking m.mu (the data-race fix
// in manager_callback_race_test.go), every lifecycle method that invoked its
// registered callbacks WHILE STILL HOLDING m.mu created a deadlock surface:
// sync.RWMutex is not reentrant, so any user SessionCallback / SwitchCallback
// that called back into the same *Manager (m.Get, m.GetAll, m.OnCreate, …)
// blocked forever waiting for a lock its own caller still held.
//
// The fix: lifecycle methods snapshot the callback slice under m.mu, release
// the lock, then invoke the callbacks (and emitHook) with NO lock held.
//
// §11.4.115 polarity: a single RED_MODE switch.
//
//	RED_MODE=1 — reproduce the defect on a FAITHFUL pre-fix stand-in: a struct
//	             that takes a lock and invokes a re-entrant callback WHILE the
//	             lock is held (the exact pre-fix invoke-under-lock shape). The
//	             callback tries to take the same lock and blocks forever; the
//	             bounded select reports the deadlock as a clean FAIL instead of
//	             hanging the suite.
//	RED_MODE=0 (default) — drive the REAL fixed *Manager: register an OnCreate
//	             callback that itself calls m.GetAll() and m.Get() on the same
//	             Manager, then call m.Create. With the snapshot-then-invoke fix
//	             the callback re-enters cleanly and Create returns; pre-fix it
//	             would deadlock and the select would fire t.Fatal.

import (
	"os"
	"sync"
	"testing"
	"time"
)

// reentrantPrefixStandin models the pre-fix lifecycle method: it acquires its
// lock and invokes the callback while STILL holding the lock. A callback that
// re-enters (re-locks) the same struct deadlocks — exactly the production bug
// before the snapshot-then-invoke fix.
type reentrantPrefixStandin struct {
	mu        sync.Mutex
	callbacks []func(s *reentrantPrefixStandin)
}

func (r *reentrantPrefixStandin) register(cb func(*reentrantPrefixStandin)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.callbacks = append(r.callbacks, cb)
}

// fireUnderLock is the pre-fix invocation pattern: range + invoke WITH the lock
// held. This is what every lifecycle method used to do.
func (r *reentrantPrefixStandin) fireUnderLock() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, cb := range r.callbacks {
		cb(r) // invoked WHILE holding r.mu — re-entrant callbacks deadlock here
	}
}

// readReentrant is the operation a re-entrant callback performs (analogous to
// m.GetAll / m.Get): it tries to take the same lock.
func (r *reentrantPrefixStandin) readReentrant() {
	r.mu.Lock()
	defer r.mu.Unlock()
}

func TestManagerCallbackReentry_NoDeadlock(t *testing.T) {
	done := make(chan struct{})

	if os.Getenv("RED_MODE") == "1" {
		// Faithful pre-fix stand-in: invoke a re-entrant callback under the
		// lock. The callback blocks on the same lock => deadlock. The bounded
		// select turns the hang into a clean, attributable FAIL.
		go func() {
			r := &reentrantPrefixStandin{}
			r.register(func(s *reentrantPrefixStandin) {
				s.readReentrant() // re-enters under the held lock => deadlocks
			})
			r.fireUnderLock()
			close(done)
		}()

		select {
		case <-done:
			t.Fatal("RED_MODE: expected the pre-fix invoke-under-lock stand-in " +
				"to DEADLOCK on a re-entrant callback, but it returned — stand-in " +
				"does not model the defect")
		case <-time.After(2 * time.Second):
			t.Log("RED_MODE: reproduced the defect — a callback invoked under the " +
				"lock that re-enters the same lock deadlocks (bounded-select detected " +
				"the hang)")
		}
		return
	}

	// GREEN: the real fixed *Manager. Register an OnCreate callback that
	// re-enters the SAME Manager (GetAll + Get) and then call Create. With the
	// snapshot-then-invoke fix the callback runs with no lock held and Create
	// returns; pre-fix it would deadlock and the select would fire t.Fatal.
	go func() {
		m := NewManager()

		var gotAll int
		m.OnCreate(func(s *Session) {
			// These re-enter the Manager: pre-fix they would block on m.mu
			// (still held by Create), deadlocking. Post-fix they succeed.
			all := m.GetAll()
			gotAll = len(all)
			if _, err := m.Get(s.ID); err != nil {
				t.Errorf("re-entrant Get inside OnCreate failed: %v", err)
			}
		})

		sess, err := m.Create("proj", "name", "desc", ModeBuilding)
		if err != nil {
			t.Errorf("Create failed: %v", err)
			close(done)
			return
		}
		// The callback must have observed the session already stored.
		if gotAll < 1 {
			t.Errorf("re-entrant GetAll inside OnCreate saw %d sessions, want >=1", gotAll)
		}

		// Also prove the switch-callback path is re-entrant-safe: an OnSwitch
		// callback that calls GetActive while Start fires it.
		m.OnSwitch(func(_, to *Session) {
			_ = m.GetActive()
		})
		if err := m.Start(sess.ID); err != nil {
			t.Errorf("Start failed: %v", err)
		}

		close(done)
	}()

	select {
	case <-done:
		// Returned without hanging — re-entrant callbacks did not deadlock.
	case <-time.After(2 * time.Second):
		t.Fatal("deadlock: callback re-entered Manager under lock " +
			"(Create/Start did not return within 2s)")
	}
}
