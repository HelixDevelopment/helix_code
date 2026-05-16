package compression

import (
	"errors"
	"sync"
)

// ErrThrashing is returned when the guard detects N consecutive compactions
// with no intervening user message. Per claude-code's auto-compaction
// design, this signals that the agent is "overwhelmed" and the caller
// should surface the error to the human rather than silently looping.
var ErrThrashing = errors.New("compaction thrashing: N consecutive compactions with no user message")

// ThrashingGuard tracks consecutive compactions and aborts when a configured
// threshold is reached without an intervening user message.
type ThrashingGuard struct {
	threshold int
	mu        sync.Mutex
	count     int
}

// NewThrashingGuard returns a guard configured with the given threshold.
// claude-code uses threshold=3.
func NewThrashingGuard(threshold int) *ThrashingGuard {
	return &ThrashingGuard{threshold: threshold}
}

// RecordCompaction increments the consecutive-compaction counter and
// returns ErrThrashing if the counter has reached the threshold.
// The increment happens before the comparison, so a threshold of 3
// allows compactions 1, 2, 3 and rejects compaction 4.
func (g *ThrashingGuard) RecordCompaction() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.count++
	if g.count > g.threshold {
		return ErrThrashing
	}
	return nil
}

// NoteUserMessage resets the consecutive-compaction counter to zero.
// The session manager must call this whenever a user message is appended
// to the conversation.
func (g *ThrashingGuard) NoteUserMessage() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.count = 0
}
