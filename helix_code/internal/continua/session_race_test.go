// session_race_test.go — §11.4.135 standing regression guard for the
// HXC-continua ChatManager getter-escapes-lock data race.
//
// §11.4.115 RED_MODE polarity:
//   RED_MODE=1 — reproduce the historical defect on a faithful pre-fix
//     stand-in (a getter that hands out the LIVE stored *ChatSession) and
//     assert -race trips, PROVING the guard catches the real bug. Run with:
//       RED_MODE=1 go test -race -count=1 -run TestChatManager_GetterSnapshot_NoRace ./internal/continua/
//   RED_MODE unset / 0 (DEFAULT) — drive the REAL fixed ChatManager and
//     assert NO race: GetSession/ListSessions/CreateSession return deep-copy
//     snapshots whose Messages slice never aliases the live stored session.
//
// Mocks ALLOWED here per CONST-050(A) — this is a unit (*_test.go) file. The
// RED stand-in is a faithful reproduction of the pre-fix getter, NOT a fake
// of the system under test: in default mode the REAL ChatManager is exercised.
package continua

import (
	"context"
	"os"
	"sync"
	"testing"
)

// raceVulnerableManager is a faithful stand-in for the PRE-FIX ChatManager
// getter: it stores *ChatSession and hands out the LIVE pointer (no snapshot),
// exactly as session.go did before the HXC-continua fix. Used only under
// RED_MODE=1 to prove the -race trip is real.
type raceVulnerableManager struct {
	mu       sync.RWMutex
	sessions map[string]*ChatSession
}

func newRaceVulnerableManager() *raceVulnerableManager {
	return &raceVulnerableManager{sessions: make(map[string]*ChatSession)}
}

func (c *raceVulnerableManager) create(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sessions[id] = &ChatSession{ID: id}
}

func (c *raceVulnerableManager) add(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	s := c.sessions[id]
	s.Messages = append(s.Messages, ChatMessage{Role: "user", Content: "x"})
}

// getLive returns the LIVE stored pointer — the defect.
func (c *raceVulnerableManager) getLive(id string) *ChatSession {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.sessions[id]
}

// hammerLive exercises the RED stand-in: concurrent writer (append under
// write lock) + reader (reads the live returned slice). On the vulnerable
// getter this is a data race the -race detector aborts on.
func hammerLive(t *testing.T) {
	t.Helper()
	cm := newRaceVulnerableManager()
	const id = "s1"
	cm.create(id)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 2000; i++ {
			cm.add(id)
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 2000; i++ {
			s := cm.getLive(id)
			_ = len(s.Messages)
			for range s.Messages {
			}
		}
	}()
	wg.Wait()
}

// hammerReal exercises the REAL fixed ChatManager. With the snapshot fix the
// returned sessions own their Messages backing array, so concurrent
// AddMessage appends cannot race the reader. The -race detector must NOT trip.
func hammerReal(t *testing.T) {
	t.Helper()
	cm := NewChatManager()
	ctx := context.Background()
	s := cm.CreateSession("t", "m")
	id := s.ID

	// CreateSession returned a snapshot — reading its Messages while a
	// concurrent writer appends to the STORED session must also be race-free.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 2000; i++ {
			_ = cm.AddMessage(ctx, id, "user", "x")
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 2000; i++ {
			got, err := cm.GetSession(id)
			if err != nil {
				continue
			}
			_ = len(got.Messages)
			for range got.Messages {
			}
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			for _, ls := range cm.ListSessions() {
				_ = len(ls.Messages)
			}
		}
	}()
	// The snapshot returned by CreateSession must itself stay stable while
	// the stored session grows.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 2000; i++ {
			_ = len(s.Messages)
		}
	}()
	wg.Wait()
}

func TestChatManager_GetterSnapshot_NoRace(t *testing.T) {
	if os.Getenv("RED_MODE") == "1" {
		// Reproduce the historical defect on the faithful pre-fix stand-in.
		// Under `-race` this aborts the test with "race detected", PROVING
		// the guard observes a real bug (not a synthetic failure).
		hammerLive(t)
		return
	}
	// DEFAULT: drive the REAL fixed code; must be race-free.
	hammerReal(t)
}
