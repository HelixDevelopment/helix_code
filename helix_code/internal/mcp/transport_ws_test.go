package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runWSEchoServer(t *testing.T) (string, func()) {
	t.Helper()
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer c.Close()
		for {
			_, msg, err := c.ReadMessage()
			if err != nil {
				return
			}
			var req MCPMessage
			if err := json.Unmarshal(msg, &req); err != nil {
				return
			}
			resp := MCPMessage{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{"ok": true}}
			b, _ := json.Marshal(&resp)
			c.WriteMessage(websocket.TextMessage, b)
		}
	}))
	cleanup := func() { srv.Close() }
	url := "ws" + strings.TrimPrefix(srv.URL, "http")
	return url, cleanup
}

func TestWSTransport_RoundTrip(t *testing.T) {
	url, cleanup := runWSEchoServer(t)
	defer cleanup()
	tr := NewWSTransport(WSConfig{URL: url})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, tr.Open(ctx))
	defer tr.Close()
	require.NoError(t, tr.Send(ctx, &MCPMessage{JSONRPC: "2.0", ID: "1", Method: "ping"}))
	resp, err := tr.Recv(ctx)
	require.NoError(t, err)
	assert.Equal(t, "1", resp.ID)
	assert.Equal(t, TransportWS, tr.Type())
}

func TestWSTransport_CloseStopsRecv(t *testing.T) {
	url, cleanup := runWSEchoServer(t)
	defer cleanup()
	tr := NewWSTransport(WSConfig{URL: url})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, tr.Open(ctx))
	require.NoError(t, tr.Close())
	_, err := tr.Recv(ctx)
	assert.Error(t, err)
}

// REQUIRED regression test (T03/T04/T05 lesson)
func TestWSTransport_CloseUnblocksRecv(t *testing.T) {
	url, cleanup := runWSEchoServer(t)
	defer cleanup()
	tr := NewWSTransport(WSConfig{URL: url})
	require.NoError(t, tr.Open(context.Background()))

	done := make(chan error, 1)
	go func() {
		_, err := tr.Recv(context.Background())
		done <- err
	}()

	require.NoError(t, tr.Close())
	select {
	case err := <-done:
		require.Error(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("Recv did not unblock after Close")
	}
}
