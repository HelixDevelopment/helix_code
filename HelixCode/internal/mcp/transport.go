package mcp

import (
	"context"
	"math/rand"
	"sync"
	"time"
)

// Transport is the seam every MCP client transport plugs into.
// Implementations must be safe to use from a single Recv goroutine and a
// concurrent Send caller. Close MUST be idempotent and unblock any pending
// Recv with io.EOF or ErrTransportClosed.
type Transport interface {
	Open(ctx context.Context) error
	Send(ctx context.Context, msg *MCPMessage) error
	Recv(ctx context.Context) (*MCPMessage, error)
	Close() error
	Type() TransportType
}

// BackoffSchedule produces exponentially increasing delays with ±20% jitter,
// capped at 30s. Reset() returns to the 1s base.
type BackoffSchedule struct {
	mu    sync.Mutex
	steps []time.Duration
	idx   int
	rng   *rand.Rand
}

func NewBackoffSchedule() *BackoffSchedule {
	return &BackoffSchedule{
		steps: []time.Duration{
			1 * time.Second,
			2 * time.Second,
			4 * time.Second,
			8 * time.Second,
			16 * time.Second,
			30 * time.Second,
		},
		rng: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (b *BackoffSchedule) Next() time.Duration {
	b.mu.Lock()
	defer b.mu.Unlock()
	base := b.steps[b.idx]
	if b.idx < len(b.steps)-1 {
		b.idx++
	}
	jitter := 0.8 + 0.4*b.rng.Float64() // [0.8, 1.2)
	return time.Duration(float64(base) * jitter)
}

func (b *BackoffSchedule) Reset() {
	b.mu.Lock()
	b.idx = 0
	b.mu.Unlock()
}
