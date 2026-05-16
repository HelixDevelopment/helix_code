package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runSSEServer returns (postURL, sseURL, controlCloseStream, cleanup).
// controlCloseStream() closes the active SSE stream so the client must reconnect.
func runSSEServer(t *testing.T) (string, string, func(), func()) {
	t.Helper()
	mux := http.NewServeMux()
	var sessionID atomic.Int64
	type session struct {
		flusher http.Flusher
		w       http.ResponseWriter
		done    chan struct{}
	}
	var current atomic.Pointer[session]

	mux.HandleFunc("/sse", func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "no flush", 500)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		flusher.Flush()
		s := &session{flusher: flusher, w: w, done: make(chan struct{})}
		current.Store(s)
		sessionID.Add(1)
		<-s.done
	})
	mux.HandleFunc("/post", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req MCPMessage
		require.NoError(t, json.Unmarshal(body, &req))
		s := current.Load()
		if s == nil {
			http.Error(w, "no session", 503)
			return
		}
		resp := MCPMessage{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{"ok": true}}
		b, _ := json.Marshal(&resp)
		fmt.Fprintf(s.w, "data: %s\n\n", string(b))
		s.flusher.Flush()
		w.WriteHeader(204)
	})
	srv := httptest.NewServer(mux)
	closeStream := func() {
		s := current.Load()
		if s != nil {
			close(s.done)
			current.Store(nil)
		}
	}
	cleanup := func() {
		closeStream()
		srv.Close()
	}
	return srv.URL + "/post", srv.URL + "/sse", closeStream, cleanup
}

func TestSSETransport_RoundTrip(t *testing.T) {
	postURL, sseURL, _, cleanup := runSSEServer(t)
	defer cleanup()
	tr := NewSSETransport(SSEConfig{PostURL: postURL, SSEURL: sseURL, BackoffOverride: 50 * time.Millisecond})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, tr.Open(ctx))
	defer tr.Close()
	time.Sleep(100 * time.Millisecond)
	require.NoError(t, tr.Send(ctx, &MCPMessage{JSONRPC: "2.0", ID: "1", Method: "ping"}))
	resp, err := tr.Recv(ctx)
	require.NoError(t, err)
	assert.Equal(t, "1", resp.ID)
	assert.Equal(t, TransportSSE, tr.Type())
}

func TestSSETransport_ReconnectAfterStreamClose(t *testing.T) {
	postURL, sseURL, closeStream, cleanup := runSSEServer(t)
	defer cleanup()
	tr := NewSSETransport(SSEConfig{PostURL: postURL, SSEURL: sseURL, BackoffOverride: 50 * time.Millisecond})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, tr.Open(ctx))
	defer tr.Close()
	time.Sleep(100 * time.Millisecond)
	closeStream()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if tr.Reconnects() >= 1 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	assert.GreaterOrEqual(t, tr.Reconnects(), int64(1))
	time.Sleep(200 * time.Millisecond)
	require.NoError(t, tr.Send(ctx, &MCPMessage{JSONRPC: "2.0", ID: "2", Method: "ping"}))
	resp, err := tr.Recv(ctx)
	require.NoError(t, err)
	assert.Equal(t, "2", resp.ID)
}

// REQUIRED regression test (added based on T03/T04 lesson)
func TestSSETransport_CloseUnblocksRecv(t *testing.T) {
	postURL, sseURL, _, cleanup := runSSEServer(t)
	defer cleanup()
	tr := NewSSETransport(SSEConfig{PostURL: postURL, SSEURL: sseURL, BackoffOverride: 50 * time.Millisecond})
	require.NoError(t, tr.Open(context.Background()))

	done := make(chan error, 1)
	go func() {
		_, err := tr.Recv(context.Background())
		done <- err
	}()

	require.NoError(t, tr.Close())
	select {
	case err := <-done:
		require.ErrorIs(t, err, ErrTransportClosed)
	case <-time.After(2 * time.Second):
		t.Fatal("Recv did not unblock after Close")
	}
}
