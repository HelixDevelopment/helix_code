package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/oauth2"
)

// WSConfig holds configuration for the WebSocket MCP transport.
type WSConfig struct {
	URL          string
	TokenSource  oauth2.TokenSource
	OAuthEnabled bool
	Headers      http.Header
	PingInterval time.Duration
	PongTimeout  time.Duration
}

// wsTransport implements Transport over a WebSocket connection using gorilla/websocket.
type wsTransport struct {
	cfg     WSConfig
	conn    *websocket.Conn
	recvCh  chan *recvItem
	closeCh chan struct{}
	cancel  context.CancelFunc
	mu      sync.Mutex
	closed  bool
}

// NewWSTransport creates a new WebSocket transport with the given config.
// The closeCh is initialised here so that Close and Recv work correctly
// even before Open is called.
func NewWSTransport(cfg WSConfig) *wsTransport {
	return &wsTransport{
		cfg:     cfg,
		recvCh:  make(chan *recvItem, 16),
		closeCh: make(chan struct{}),
	}
}

// Type returns TransportWS.
func (t *wsTransport) Type() TransportType { return TransportWS }

// Open dials the WebSocket server and starts the read and ping goroutines.
func (t *wsTransport) Open(ctx context.Context) error {
	if t.cfg.URL == "" {
		return fmt.Errorf("mcp ws: empty URL")
	}

	hdr := http.Header{}
	for k, v := range t.cfg.Headers {
		hdr[k] = v
	}

	if t.cfg.TokenSource != nil {
		tok, err := t.cfg.TokenSource.Token()
		if err != nil {
			return fmt.Errorf("%w: %v", ErrOAuthRequired, err)
		}
		// SetAuthHeader requires *http.Request; use a throwaway request to
		// populate the Authorization header, then copy it into our dialer header.
		req := &http.Request{Header: hdr}
		tok.SetAuthHeader(req)
	}

	dialer := websocket.DefaultDialer
	c, resp, err := dialer.DialContext(ctx, t.cfg.URL, hdr)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusUnauthorized && t.cfg.OAuthEnabled {
			return ErrOAuthRequired
		}
		return fmt.Errorf("mcp ws: dial: %w", err)
	}
	t.conn = c

	pong := t.cfg.PongTimeout
	if pong == 0 {
		pong = 30 * time.Second
	}
	c.SetReadLimit(16 * 1024 * 1024)
	_ = c.SetReadDeadline(time.Now().Add(pong))
	c.SetPongHandler(func(string) error {
		return c.SetReadDeadline(time.Now().Add(pong))
	})

	rctx, cancel := context.WithCancel(context.Background())
	t.cancel = cancel
	go t.readLoop(rctx)
	go t.pingLoop(rctx)
	return nil
}

// readLoop reads messages from the WebSocket connection until the context is
// cancelled or an error occurs, then pushes a terminal error into recvCh.
func (t *wsTransport) readLoop(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}
		_, data, err := t.conn.ReadMessage()
		if err != nil {
			t.pushRecv(&recvItem{err: fmt.Errorf("mcp ws: read: %w", err)})
			return
		}
		var m MCPMessage
		if err := json.Unmarshal(data, &m); err != nil {
			t.pushRecv(&recvItem{err: fmt.Errorf("%w: ws parse: %v", ErrProtocol, err)})
			continue
		}
		t.pushRecv(&recvItem{msg: &m})
	}
}

// pingLoop sends WebSocket ping frames at the configured interval to keep the
// connection alive. It stops when the context is cancelled or closeCh is closed.
func (t *wsTransport) pingLoop(ctx context.Context) {
	iv := t.cfg.PingInterval
	if iv == 0 {
		iv = 25 * time.Second
	}
	tk := time.NewTicker(iv)
	defer tk.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.closeCh:
			return
		case <-tk.C:
			t.mu.Lock()
			if t.conn != nil && !t.closed {
				_ = t.conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second))
			}
			t.mu.Unlock()
		}
	}
}

// pushRecv sends item to recvCh, but abandons the push if closeCh fires first.
// This prevents the read loop from blocking forever after Close() is called.
func (t *wsTransport) pushRecv(item *recvItem) {
	select {
	case t.recvCh <- item:
	case <-t.closeCh:
	}
}

// Send marshals msg and writes it to the WebSocket as a text frame.
func (t *wsTransport) Send(ctx context.Context, msg *MCPMessage) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed || t.conn == nil {
		return ErrTransportClosed
	}
	b, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("mcp ws: marshal: %w", err)
	}
	return t.conn.WriteMessage(websocket.TextMessage, b)
}

// Recv blocks until a message arrives, the context expires, or Close is called.
// Close MUST unblock any pending Recv with ErrTransportClosed (closeCh guarantee).
func (t *wsTransport) Recv(ctx context.Context) (*MCPMessage, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-t.closeCh:
		return nil, ErrTransportClosed
	case item, ok := <-t.recvCh:
		if !ok {
			return nil, ErrTransportClosed
		}
		if item.err != nil {
			return nil, item.err
		}
		return item.msg, nil
	}
}

// Close is idempotent: it closes closeCh once (guarded by mu + closed bool),
// cancels the read/ping context, sends a WS close frame, and closes the conn.
func (t *wsTransport) Close() error {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return nil
	}
	t.closed = true
	close(t.closeCh)
	conn := t.conn
	t.mu.Unlock()

	if t.cancel != nil {
		t.cancel()
	}
	if conn != nil {
		t.mu.Lock()
		_ = conn.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
			time.Now().Add(time.Second),
		)
		t.mu.Unlock()
		_ = conn.Close()
	}
	return nil
}
