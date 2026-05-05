package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/oauth2"
)

// SSEConfig holds configuration for the SSE transport.
// Outgoing messages are POSTed to PostURL; incoming events are read from a
// GET SSE stream at SSEURL. The transport auto-reconnects on stream EOF/error
// using exponential backoff (or BackoffOverride for tests).
type SSEConfig struct {
	PostURL         string
	SSEURL          string
	TokenSource     oauth2.TokenSource
	OAuthEnabled    bool
	Headers         map[string]string
	BackoffOverride time.Duration
}

type sseTransport struct {
	cfg        SSEConfig
	client     *http.Client
	recvCh     chan *recvItem
	closeCh    chan struct{}
	cancel     context.CancelFunc
	mu         sync.Mutex
	closed     bool
	reconnects atomic.Int64
}

// NewSSETransport creates a new SSE transport with the given config.
// Call Open to start the SSE receive loop before Send/Recv.
func NewSSETransport(cfg SSEConfig) *sseTransport {
	return &sseTransport{
		cfg:     cfg,
		client:  &http.Client{Timeout: 0}, // no timeout — streaming connection
		recvCh:  make(chan *recvItem, 16),
		closeCh: make(chan struct{}),
	}
}

// Type returns TransportSSE.
func (t *sseTransport) Type() TransportType { return TransportSSE }

// Reconnects returns the number of times the SSE stream has been reconnected.
func (t *sseTransport) Reconnects() int64 { return t.reconnects.Load() }

// Open validates config and starts the background SSE receive loop.
func (t *sseTransport) Open(ctx context.Context) error {
	if t.cfg.PostURL == "" || t.cfg.SSEURL == "" {
		return fmt.Errorf("mcp sse: empty URL(s)")
	}
	rctx, cancel := context.WithCancel(ctx)
	t.cancel = cancel
	go t.runSSELoop(rctx)
	return nil
}

// runSSELoop establishes the SSE stream and reconnects on failure.
func (t *sseTransport) runSSELoop(ctx context.Context) {
	bs := NewBackoffSchedule()
	first := true
	for {
		if ctx.Err() != nil {
			return
		}
		select {
		case <-t.closeCh:
			return
		default:
		}
		if !first {
			t.reconnects.Add(1)
			delay := t.backoffDelay(bs)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return
			case <-t.closeCh:
				return
			}
		}
		first = false
		err := t.streamOnce(ctx)
		if ctx.Err() != nil {
			return
		}
		if err == nil {
			bs.Reset() // clean EOF: server closed cleanly, reset for next time
		}
		// on error: keep bs advancing through the schedule
	}
}

// backoffDelay returns the next backoff delay, respecting BackoffOverride for tests.
func (t *sseTransport) backoffDelay(bs *BackoffSchedule) time.Duration {
	if t.cfg.BackoffOverride > 0 {
		return t.cfg.BackoffOverride
	}
	return bs.Next()
}

// streamOnce opens one SSE GET stream and reads events until it ends or errors.
func (t *sseTransport) streamOnce(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", t.cfg.SSEURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "text/event-stream")
	for k, v := range t.cfg.Headers {
		req.Header.Set(k, v)
	}
	if t.cfg.TokenSource != nil {
		tok, err := t.cfg.TokenSource.Token()
		if err != nil {
			return fmt.Errorf("%w: %v", ErrOAuthRequired, err)
		}
		tok.SetAuthHeader(req)
	}
	resp, err := t.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: SSE GET status %d", ErrProtocol, resp.StatusCode)
	}
	rd := bufio.NewReaderSize(resp.Body, 1024*1024)
	var data bytes.Buffer
	for {
		line, err := rd.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		line = bytes.TrimRight(line, "\r\n")
		if len(line) == 0 {
			if data.Len() > 0 {
				t.dispatch(data.Bytes())
				data.Reset()
			}
			continue
		}
		if bytes.HasPrefix(line, []byte(":")) {
			// SSE comment — ignore
			continue
		}
		if bytes.HasPrefix(line, []byte("data:")) {
			payload := bytes.TrimPrefix(line, []byte("data:"))
			payload = bytes.TrimPrefix(payload, []byte(" "))
			data.Write(payload)
			data.WriteByte('\n')
		}
	}
}

// dispatch parses an accumulated SSE data block and pushes it to recvCh.
func (t *sseTransport) dispatch(data []byte) {
	data = bytes.TrimRight(data, "\n")
	var m MCPMessage
	if err := json.Unmarshal(data, &m); err != nil {
		t.pushRecv(&recvItem{err: fmt.Errorf("%w: SSE parse: %v", ErrProtocol, err)})
		return
	}
	t.pushRecv(&recvItem{msg: &m})
}

// pushRecv sends item to recvCh, or drops it if the transport is closing.
func (t *sseTransport) pushRecv(item *recvItem) {
	select {
	case t.recvCh <- item:
	case <-t.closeCh:
	}
}

// Send POSTs msg to cfg.PostURL and awaits a 2xx response.
func (t *sseTransport) Send(ctx context.Context, msg *MCPMessage) error {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return ErrTransportClosed
	}
	t.mu.Unlock()

	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", t.cfg.PostURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range t.cfg.Headers {
		req.Header.Set(k, v)
	}
	if t.cfg.TokenSource != nil {
		tok, err := t.cfg.TokenSource.Token()
		if err != nil {
			return fmt.Errorf("%w: %v", ErrOAuthRequired, err)
		}
		tok.SetAuthHeader(req)
	}
	resp, err := t.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized && t.cfg.OAuthEnabled {
		return ErrOAuthRequired
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
		return fmt.Errorf("%w: SSE POST %d: %s", ErrProtocol, resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return nil
}

// Recv blocks until an MCPMessage arrives, the context expires, or the
// transport is closed. Returns ErrTransportClosed when Close has been called.
func (t *sseTransport) Recv(ctx context.Context) (*MCPMessage, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-t.closeCh:
		return nil, ErrTransportClosed
	case item := <-t.recvCh:
		if item.err != nil {
			return nil, item.err
		}
		return item.msg, nil
	}
}

// Close is idempotent. It cancels the SSE loop and unblocks any pending Recv.
func (t *sseTransport) Close() error {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return nil
	}
	t.closed = true
	close(t.closeCh)
	t.mu.Unlock()
	if t.cancel != nil {
		t.cancel()
	}
	return nil
}
