package mcp

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_HandshakeSuccess(t *testing.T) {
	ft := newFakeTransport()
	c := NewClient("srv-a", ft)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go func() {
		// observe the initialize request, send synthetic reply
		time.Sleep(50 * time.Millisecond)
		sent := ft.sentMessages()
		var initID interface{}
		for _, m := range sent {
			if m.Method == "initialize" {
				initID = m.ID
				break
			}
		}
		require.NotNil(t, initID)
		ft.pushReply(&MCPMessage{JSONRPC: "2.0", ID: initID, Result: map[string]any{"capabilities": map[string]any{"tools": map[string]any{}}}})
		// then tools/list
		time.Sleep(50 * time.Millisecond)
		var toolsID interface{}
		for _, m := range ft.sentMessages() {
			if m.Method == "tools/list" {
				toolsID = m.ID
				break
			}
		}
		require.NotNil(t, toolsID)
		ft.pushReply(&MCPMessage{JSONRPC: "2.0", ID: toolsID, Result: map[string]any{"tools": []map[string]any{{"name": "echo"}}}})
	}()

	require.NoError(t, c.Connect(ctx))
	assert.Equal(t, StateReady, c.State())
	tools := c.Tools()
	require.Len(t, tools, 1)
	assert.Equal(t, "echo", tools[0].Name)
}

func TestClient_CallToolReturnsResult(t *testing.T) {
	ft := newFakeTransport()
	c := NewClient("srv-a", ft)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go func() {
		time.Sleep(50 * time.Millisecond)
		for _, m := range ft.sentMessages() {
			if m.Method == "initialize" {
				ft.pushReply(&MCPMessage{JSONRPC: "2.0", ID: m.ID, Result: map[string]any{}})
			}
		}
		time.Sleep(50 * time.Millisecond)
		for _, m := range ft.sentMessages() {
			if m.Method == "tools/list" {
				ft.pushReply(&MCPMessage{JSONRPC: "2.0", ID: m.ID, Result: map[string]any{"tools": []map[string]any{}}})
			}
		}
	}()
	require.NoError(t, c.Connect(ctx))

	go func() {
		time.Sleep(50 * time.Millisecond)
		for _, m := range ft.sentMessages() {
			if m.Method == "tools/call" {
				ft.pushReply(&MCPMessage{JSONRPC: "2.0", ID: m.ID, Result: map[string]any{"content": []map[string]any{{"type": "text", "text": "hello"}}}})
			}
		}
	}()
	res, err := c.CallTool(ctx, "echo", map[string]any{"x": 1})
	require.NoError(t, err)
	assert.NotNil(t, res)
}

func TestClient_StateTransitions(t *testing.T) {
	ft := newFakeTransport()
	c := NewClient("srv-a", ft)
	assert.Equal(t, StateDisconnected, c.State())
	c.setState(StateConnecting)
	assert.Equal(t, StateConnecting, c.State())
	c.setState(StateReady)
	assert.Equal(t, StateReady, c.State())
}

// contextWithSeededHandshake seeds the fake transport with replies and
// returns a short-lived context for Connect. Helper to dedupe boilerplate.
func contextWithSeededHandshake(t *testing.T, ft *fakeTransport) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	t.Cleanup(cancel)
	go func() {
		time.Sleep(20 * time.Millisecond)
		for _, m := range ft.sentMessages() {
			if m.Method == "initialize" {
				ft.pushReply(&MCPMessage{JSONRPC: "2.0", ID: m.ID, Result: map[string]any{}})
			}
		}
		time.Sleep(20 * time.Millisecond)
		for _, m := range ft.sentMessages() {
			if m.Method == "tools/list" {
				ft.pushReply(&MCPMessage{JSONRPC: "2.0", ID: m.ID, Result: map[string]any{"tools": []map[string]any{}}})
			}
		}
	}()
	return ctx
}

func TestClient_CloseIdempotentUnderRace(t *testing.T) {
	ft := newFakeTransport()
	c := NewClient("srv-a", ft)
	require.NoError(t, c.Connect(contextWithSeededHandshake(t, ft)))

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = c.Close()
		}()
	}
	wg.Wait()
}

func TestClient_HandshakeFailureStopsRecvLoop(t *testing.T) {
	ft := newFakeTransport()
	c := NewClient("srv-a", ft)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	// Don't push any reply — handshake will time out.
	err := c.Connect(ctx)
	require.Error(t, err)
	assert.Equal(t, StateDisconnected, c.State())
	// Give recvLoop time to exit after its context is cancelled.
	time.Sleep(50 * time.Millisecond)
	// A retry must work cleanly (no zombie recvLoop racing on the same transport).
	ctx2, cancel2 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel2()
	go func() {
		time.Sleep(20 * time.Millisecond)
		for _, m := range ft.sentMessages() {
			if m.Method == "initialize" {
				ft.pushReply(&MCPMessage{JSONRPC: "2.0", ID: m.ID, Result: map[string]any{}})
			}
		}
		time.Sleep(20 * time.Millisecond)
		for _, m := range ft.sentMessages() {
			if m.Method == "tools/list" {
				ft.pushReply(&MCPMessage{JSONRPC: "2.0", ID: m.ID, Result: map[string]any{"tools": []map[string]any{}}})
			}
		}
	}()
	require.NoError(t, c.Connect(ctx2))
	assert.Equal(t, StateReady, c.State())
	require.NoError(t, c.Close())
}
