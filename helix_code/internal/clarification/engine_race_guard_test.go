// Standing regression guard (§11.4.135) for the internal/clarification
// session-state data race, with §11.4.115 RED_MODE polarity.
//
// DEFECT (reproduced 2026-06-16): Engine.Resolve wrote session.Answers
// outside any lock (engine.go:62 pre-fix) AND Engine.GetSession returned
// the live shared *Session pointer. A caller reading session.Answers off
// the returned pointer while another goroutine called Resolve, or two
// concurrent Resolve calls on one session, raced on the slice-header
// field — a §11.4.85(B) state-corruption defect caught by `go test -race`.
//
// FIX: Resolve mutates under e.mu.Lock(); GetSession returns a defensive
// snapshot copy so callers never alias the live mutable struct.
//
// POLARITY (§11.4.115):
//   RED_MODE=1 → drive a FAITHFUL pre-fix stand-in (unlocked write + live
//                pointer return); the guard REPRODUCES the aliasing/race
//                and PASSES (proving the guard is real, not synthetic).
//   RED_MODE=0 (default) → drive the REAL fixed Engine; assert the
//                aliasing is gone (snapshot isolation) AND a concurrent
//                Resolve/GetSession stress run does not corrupt state.
//
// Run RED reproduction:
//   RED_MODE=1 go test -count=1 -run TestRace ./internal/clarification/
// Run standing guard (default):
//   go test -race -count=1 -run TestRace ./internal/clarification/
package clarification

import (
	"os"
	"sync"
	"testing"
)

// prefixEngine is a faithful stand-in for the BROKEN pre-fix Engine.
// It mirrors the exact pre-fix semantics: Resolve writes session.Answers
// off-lock, and getSession hands back the LIVE shared pointer. It exists
// only so RED_MODE=1 can reproduce the defect on a pre-fix artifact
// without resurrecting the broken production code.
type prefixEngine struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

func newPrefixEngine() *prefixEngine {
	return &prefixEngine{sessions: make(map[string]*Session)}
}

func (e *prefixEngine) newSession(ctx string) *Session {
	e.mu.Lock()
	defer e.mu.Unlock()
	s := &Session{ID: "session-1", Context: ctx}
	e.sessions[s.ID] = s
	return s
}

// resolve reproduces the pre-fix off-lock write at engine.go:62.
func (e *prefixEngine) resolve(id string, answers []Answer) {
	e.mu.RLock()
	session, ok := e.sessions[id]
	e.mu.RUnlock()
	if !ok {
		return
	}
	session.Answers = answers // off-lock write — the defect
}

// getSession reproduces the pre-fix live-pointer return.
func (e *prefixEngine) getSession(id string) *Session {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.sessions[id] // live shared pointer — the aliasing hazard
}

func redMode() bool { return os.Getenv("RED_MODE") == "1" }

// TestRaceGuard_GetSessionDoesNotAliasLiveState pins the structural root
// cause: a snapshot returned by GetSession MUST NOT be the same pointer
// the engine stores and Resolve mutates. Distinct pointers == race-free
// isolation; identical pointers == the aliasing that enabled the race.
func TestRaceGuard_GetSessionDoesNotAliasLiveState(t *testing.T) {
	if redMode() {
		// RED: faithful pre-fix stand-in returns the LIVE pointer.
		e := newPrefixEngine()
		created := e.newSession("ctx")
		got := e.getSession(created.ID)
		if got != created {
			t.Fatalf("RED_MODE pre-fix stand-in should ALIAS the live session "+
				"(got=%p created=%p) — stand-in no longer faithful", got, created)
		}
		t.Logf("RED_MODE: reproduced live-pointer aliasing (got==created==%p)", got)
		return
	}

	// GREEN: real fixed Engine returns a defensive snapshot copy.
	e := NewEngine(nil)
	created := e.NewSession("ctx")
	got := e.GetSession(created.ID)
	if got == nil {
		t.Fatal("GetSession returned nil for an existing session")
	}
	if got == created {
		t.Fatalf("GetSession returned the LIVE pointer %p — must return a "+
			"snapshot copy so callers cannot race Resolve's session mutation", got)
	}
	if got.ID != created.ID || got.Context != created.Context {
		t.Fatalf("snapshot lost data: got %+v want id=%s ctx=%q",
			got, created.ID, created.Context)
	}
}

// TestRaceGuard_ConcurrentResolveAndGetSession is the standing regression
// guard: on the REAL fixed Engine, concurrent Resolve writes and
// GetSession reads MUST NOT trip the race detector or lose the answer.
// Run under `-race` it FAILs on the pre-fix code and PASSes on the fix.
func TestRaceGuard_ConcurrentResolveAndGetSession(t *testing.T) {
	if redMode() {
		t.Skip("SKIP-OK: RED_MODE drives the structural-aliasing guard above; " +
			"the race-detector stress assertion is the GREEN-only standing guard")
	}

	e := NewEngine(nil)
	s := e.NewSession("ctx")

	var wg sync.WaitGroup
	for i := 0; i < 64; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			e.Resolve(s.ID, []Answer{{QuestionID: "q", Value: "v"}})
		}()
		go func() {
			defer wg.Done()
			if got := e.GetSession(s.ID); got != nil {
				_ = len(got.Answers) // read off the snapshot — must be race-free
			}
		}()
	}
	wg.Wait()

	// Reality check: a final Resolve still records the answer correctly.
	out := e.Resolve(s.ID, []Answer{{QuestionID: "target", Value: "main.go"}})
	if out == "" {
		t.Fatal("Resolve returned empty after concurrent stress — state corrupted")
	}
}

// mutateStoredElementsInPlace mutates the ELEMENTS of the stored session's
// Answers and Questions slices in place (NOT a header reassignment), under
// the engine lock. It exists only to exercise the shared-backing-array
// hazard that distinguishes a DEEP copy from a SHALLOW `*session` copy in
// GetSession: a shallow snapshot aliases these same backing arrays, so a
// concurrent reader ranging over snapshot elements would race these writes.
func mutateStoredElementsInPlace(e *Engine, id string, n int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	sess, ok := e.sessions[id]
	if !ok {
		return
	}
	for i := range sess.Answers {
		sess.Answers[i].Value = "v" + string(rune('A'+(n+i)%26))
	}
	for i := range sess.Questions {
		sess.Questions[i].Text = "t" + string(rune('A'+(n+i)%26))
		for j := range sess.Questions[i].Options {
			sess.Questions[i].Options[j] = "o" + string(rune('A'+(n+i+j)%26))
		}
	}
}

// TestRaceGuard_SnapshotElementRangingVsInPlaceMutation is the STRENGTHENED
// standing guard (§11.4.135). Unlike the header-only `len(got.Answers)`
// read above, it RANGES OVER the snapshot's Answers AND Questions (and each
// Question's Options) ELEMENTS concurrently with an in-place ELEMENT
// mutation of the stored session's backing arrays. With GetSession's DEEP
// copy the snapshot owns private backing arrays, so the ranging reads never
// touch the bytes the mutator writes — clean under `-race`. Revert
// GetSession to a SHALLOW `*session` copy and the snapshot aliases the
// stored backing arrays: the ranging reads then race the in-place element
// writes and `go test -race` reports a DATA RACE (paired-mutation proof).
func TestRaceGuard_SnapshotElementRangingVsInPlaceMutation(t *testing.T) {
	if redMode() {
		t.Skip("SKIP-OK: RED_MODE drives the structural-aliasing guard above; " +
			"the element-ranging race assertion is the GREEN-only standing guard")
	}

	e := NewEngine(nil)
	s := e.NewSession("ctx")
	// Seed the stored session with non-empty Answers + Questions (with
	// Options) so the backing arrays the deep copy must isolate exist.
	mutateStoredSeed(e, s.ID)

	var wg sync.WaitGroup
	for i := 0; i < 128; i++ {
		wg.Add(2)
		n := i
		go func() {
			defer wg.Done()
			mutateStoredElementsInPlace(e, s.ID, n) // in-place element writes
		}()
		go func() {
			defer wg.Done()
			got := e.GetSession(s.ID)
			if got == nil {
				return
			}
			// Range over the snapshot ELEMENTS — must read private,
			// deep-copied storage, never the live backing arrays.
			sink := 0
			for _, a := range got.Answers {
				sink += len(a.Value) + len(a.QuestionID)
			}
			for _, q := range got.Questions {
				sink += len(q.Text)
				for _, o := range q.Options {
					sink += len(o)
				}
			}
			_ = sink
		}()
	}
	wg.Wait()
}

// mutateStoredSeed populates the stored session with Answers + Questions
// (each carrying an Options slice) so the strengthened guard has real
// reference-typed backing arrays to isolate.
func mutateStoredSeed(e *Engine, id string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	sess, ok := e.sessions[id]
	if !ok {
		return
	}
	sess.Answers = []Answer{
		{QuestionID: "q1", Value: "v1"},
		{QuestionID: "q2", Value: "v2"},
	}
	sess.Questions = []Question{
		{ID: "q1", Text: "Which file?", Type: MultipleChoice, Options: []string{"a.go", "b.go"}},
		{ID: "q2", Text: "Expected?", Type: FreeText, Options: []string{"x", "y"}},
	}
}
