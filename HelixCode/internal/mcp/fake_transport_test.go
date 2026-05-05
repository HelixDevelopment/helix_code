package mcp

import (
	"context"
	"sync"
)

// fakeTransport is a programmable transport for unit-testing Client.
type fakeTransport struct {
	mu      sync.Mutex
	sent    []*MCPMessage
	recvCh  chan *recvItem
	openErr error
	t       TransportType
	closed  bool
}

func newFakeTransport() *fakeTransport {
	return &fakeTransport{
		recvCh: make(chan *recvItem, 32),
		t:      TransportType("fake"),
	}
}

func (f *fakeTransport) Type() TransportType { return f.t }
func (f *fakeTransport) Open(ctx context.Context) error {
	if f.openErr != nil {
		return f.openErr
	}
	return nil
}
func (f *fakeTransport) Send(ctx context.Context, m *MCPMessage) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return ErrTransportClosed
	}
	f.sent = append(f.sent, m)
	return nil
}
func (f *fakeTransport) Recv(ctx context.Context) (*MCPMessage, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case item := <-f.recvCh:
		if item.err != nil {
			return nil, item.err
		}
		return item.msg, nil
	}
}
func (f *fakeTransport) Close() error {
	f.mu.Lock()
	f.closed = true
	f.mu.Unlock()
	return nil
}

// pushReply queues a synthetic server response.
func (f *fakeTransport) pushReply(m *MCPMessage) {
	f.recvCh <- &recvItem{msg: m}
}

// pushError queues a synthetic transport error.
func (f *fakeTransport) pushError(err error) {
	f.recvCh <- &recvItem{err: err}
}

// sentMessages returns a snapshot of all messages Send received.
func (f *fakeTransport) sentMessages() []*MCPMessage {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]*MCPMessage, len(f.sent))
	copy(out, f.sent)
	return out
}
