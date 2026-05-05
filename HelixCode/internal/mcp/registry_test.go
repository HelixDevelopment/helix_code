package mcp

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManager_AddClientAndListTools(t *testing.T) {
	m := NewManager()
	c := NewClient("srv-a", newFakeTransport())
	m.addClient(c)
	c.tools = []ExternalTool{{Name: "echo"}, {Name: "time"}}
	c.state.Store(int32(StateReady))

	tools := m.Tools()
	require.Len(t, tools, 2)
	names := []string{tools[0].Server + ":" + tools[0].Name, tools[1].Server + ":" + tools[1].Name}
	assert.Contains(t, names, "srv-a:echo")
	assert.Contains(t, names, "srv-a:time")
}

func TestManager_CallToolRoutes(t *testing.T) {
	m := NewManager()
	ft := newFakeTransport()
	c := NewClient("srv-a", ft)
	c.tools = []ExternalTool{{Name: "echo"}}
	c.state.Store(int32(StateReady))
	m.addClient(c)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go func() {
		time.Sleep(20 * time.Millisecond)
		for _, msg := range ft.sentMessages() {
			if msg.Method == "tools/call" {
				ft.pushReply(&MCPMessage{JSONRPC: "2.0", ID: msg.ID, Result: map[string]any{"content": []map[string]any{{"type": "text", "text": "ok"}}}})
				return
			}
		}
	}()
	go c.recvLoop(ctx)

	res, err := m.CallTool(ctx, "srv-a", "echo", map[string]any{"x": 1})
	require.NoError(t, err)
	assert.NotNil(t, res)
}

func TestManager_ServerNotFound(t *testing.T) {
	m := NewManager()
	_, err := m.CallTool(context.Background(), "missing", "x", nil)
	assert.ErrorIs(t, err, ErrServerNotFound)
}

func TestManager_StatusReportsAllClients(t *testing.T) {
	m := NewManager()
	a := NewClient("a", newFakeTransport())
	a.state.Store(int32(StateReady))
	a.tools = []ExternalTool{{Name: "x"}}
	b := NewClient("b", newFakeTransport())
	b.state.Store(int32(StateDisconnected))
	m.addClient(a)
	m.addClient(b)
	st := m.Status()
	require.Len(t, st, 2)
}
