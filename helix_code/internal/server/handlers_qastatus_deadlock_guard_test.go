package server

// Standing regression guard for the getQASessionStatus recursive-RLock
// self-deadlock (§11.4.135 permanent guard, §11.4.115 RED-on-broken-artifact
// polarity, §11.4.118 discovery-pressure — the existing QA-status tests
// asserted success but never exercised the writer-contention window where the
// deadlock fires).
//
// Defect (reproduced before the fix): getQASessionStatus held
// state.Mu.RLock() across json.Marshal(state) / c.JSON(state). But
// *helixqa.SessionState implements MarshalJSON (internal/helixqa/wrapper.go),
// which ITSELF acquires state.Mu.RLock(). Holding the lock in the handler too
// is a RECURSIVE read-lock. Go's sync.RWMutex is documented as NOT safe for
// recursive read-locking: "if a goroutine holds a RWMutex for reading and
// another goroutine might call Lock, no goroutine should expect to be able to
// acquire a read lock until the initial read lock is released." The QA
// orchestrator goroutine spawned by StartSession calls state.Mu.Lock() on
// every phase transition (wrapper.go:138/160/173). If that write-Lock lands
// between the handler's OUTER RLock and MarshalJSON's INNER RLock, the inner
// RLock blocks behind the pending writer while the writer blocks behind the
// outer RLock held by the same handler goroutine — a permanent self-deadlock
// that hangs the request forever (DoS).
//
// Fix: the handler hands the bare *SessionState pointer to json.Marshal /
// c.JSON and does NOT pre-lock — MarshalJSON is the single correct lock point
// and acquires state.Mu exactly once.
//
// Polarity switch (§11.4.115): set HELIX_QASTATUS_RED_MODE=1 to run the RED
// reproduction. It performs the EXACT lock sequence the pre-fix handler
// performed (outer RLock, then the MarshalJSON-equivalent inner RLock) against
// a faithful SessionState with a DETERMINISTICALLY-PENDING writer, and asserts
// the inner RLock genuinely never completes within a deadline (proving the
// deadlock is real). DEFAULT (no env) runs the GREEN guard — it drives the
// REAL fixed getQASessionStatus while a writer goroutine hammers state.Mu.Lock()
// and asserts the handler completes well within a deadline (no hang).

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"dev.helix.code/internal/helixqa"

	"github.com/gin-gonic/gin"
)

// reproduceRecursiveRLockDeadlock performs the precise lock sequence the pre-fix
// getQASessionStatus performed: acquire an OUTER state.Mu.RLock() (the handler's
// own lock), then — with a writer deterministically pending — attempt the INNER
// state.Mu.RLock() that (*SessionState).MarshalJSON performs. It returns true if
// the inner RLock completed within the deadline, false if it deadlocked.
//
// This is the literal bug mechanism (handler RLock + MarshalJSON RLock + a
// pending writer), not a synthetic failure: it is exactly what the shipped
// handler + wrapper.go did together.
func reproduceRecursiveRLockDeadlock(deadline time.Duration) (innerAcquired bool) {
	st := &helixqa.SessionState{ID: "red", Status: "running"}

	// Handler's OUTER read lock.
	st.Mu.RLock()
	defer st.Mu.RUnlock()

	writerBlocked := make(chan struct{})
	go func() {
		// Signal we are about to request the write lock, then block on it.
		// Because the outer RLock above is held, this Lock() cannot proceed —
		// it becomes the "pending writer" that poisons all subsequent RLocks.
		close(writerBlocked)
		st.Mu.Lock()
		st.Status = "completed"
		st.Mu.Unlock()
	}()

	<-writerBlocked
	// Give the writer goroutine time to actually enter Lock() and register as
	// a pending writer before we attempt the recursive inner RLock.
	time.Sleep(50 * time.Millisecond)

	done := make(chan struct{})
	go func() {
		// MarshalJSON's INNER read lock. With a writer pending, RWMutex blocks
		// this forever on the pre-fix code path.
		st.Mu.RLock()
		st.Mu.RUnlock()
		close(done)
	}()

	select {
	case <-done:
		return true
	case <-time.After(deadline):
		return false
	}
}

func TestGuard_GetQASessionStatus_RecursiveRLockDeadlock(t *testing.T) {
	if os.Getenv("HELIX_QASTATUS_RED_MODE") == "1" {
		// RED reproduction: the inner (recursive) RLock MUST NOT complete while
		// a writer is pending — i.e. it deadlocks. If it DID complete, the
		// defect is not reproduced and the guard would be blind (§11.4.115
		// honest boundary).
		if reproduceRecursiveRLockDeadlock(2 * time.Second) {
			t.Fatal("RED_MODE: expected the recursive inner RLock to deadlock " +
				"behind a pending writer, but it completed — the defect did not reproduce")
		}
		// Note: this leaves a goroutine blocked on Lock()/RLock() forever; that
		// is inherent to demonstrating the deadlock and only runs in RED_MODE.
		return
	}

	// GREEN guard (DEFAULT): drive the REAL fixed handler while a writer
	// goroutine hammers state.Mu.Lock(). The handler must finish quickly.
	gin.SetMode(gin.TestMode)
	server, w, c, bankFile := setupQATestServer(t)

	// Start a real session so getQASessionStatus has a live *SessionState
	// (the same one the orchestrator goroutine mutates under state.Mu.Lock()).
	startReq := StartSessionRequest{Platforms: []string{"web"}, Banks: []string{bankFile}}
	body, _ := json.Marshal(startReq)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/qa/session", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	server.startQASession(c)
	if w.Code != http.StatusCreated {
		t.Fatalf("setup: startQASession returned %d (body=%s)", w.Code, w.Body.String())
	}
	var created helixqa.SessionState
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("setup: bad start response: %v", err)
	}

	state, ok := server.qaEngine.GetSession(created.ID)
	if !ok {
		t.Fatalf("setup: session %s not found in engine", created.ID)
	}

	// Hammer the writer lock concurrently to widen the deadlock window — this
	// is precisely the orchestrator's write pattern that the bug raced against.
	stop := make(chan struct{})
	var hammer sync.WaitGroup
	hammer.Add(1)
	go func() {
		defer hammer.Done()
		for {
			select {
			case <-stop:
				return
			default:
				state.Mu.Lock()
				state.Phase = "orchestration"
				state.PhaseProgress += 0.0
				state.Mu.Unlock()
			}
		}
	}()

	// Run the real handler under contention with a hard deadline. A hang here
	// is the deadlock recurring.
	for _, accept := range []string{"", "text/event-stream"} {
		w2 := httptest.NewRecorder()
		c2, _ := gin.CreateTestContext(w2)
		c2.Request = httptest.NewRequest(http.MethodGet, "/api/v1/qa/session/"+created.ID+"/status", nil)
		if accept != "" {
			c2.Request.Header.Set("Accept", accept)
		}
		c2.Params = gin.Params{{Key: "id", Value: created.ID}}

		done := make(chan struct{})
		go func() {
			server.getQASessionStatus(c2)
			close(done)
		}()

		select {
		case <-done:
			if w2.Code != http.StatusOK && accept == "" {
				t.Fatalf("getQASessionStatus(accept=%q) returned %d, want 200 (body=%s)",
					accept, w2.Code, w2.Body.String())
			}
		case <-time.After(5 * time.Second):
			close(stop)
			t.Fatalf("DEADLOCK: getQASessionStatus(accept=%q) did not return within "+
				"5s under concurrent writer contention — the recursive-RLock "+
				"self-deadlock has regressed", accept)
		}
	}

	close(stop)
	hammer.Wait()
}
