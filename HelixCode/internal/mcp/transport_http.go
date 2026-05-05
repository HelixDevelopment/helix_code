package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"golang.org/x/oauth2"
)

type HTTPConfig struct {
	URL          string
	TokenSource  oauth2.TokenSource
	OAuthEnabled bool
	Headers      map[string]string
	Timeout      time.Duration
}

type httpTransport struct {
	cfg    HTTPConfig
	client *http.Client
	recvCh chan *recvItem
	mu     sync.Mutex
	closed bool
}

type recvItem struct {
	msg *MCPMessage
	err error
}

func NewHTTPTransport(cfg HTTPConfig) *httpTransport {
	to := cfg.Timeout
	if to == 0 {
		to = 60 * time.Second
	}
	return &httpTransport{
		cfg:    cfg,
		client: &http.Client{Timeout: to},
		recvCh: make(chan *recvItem, 16),
	}
}

func (t *httpTransport) Type() TransportType { return TransportHTTP }

func (t *httpTransport) Open(ctx context.Context) error {
	if t.cfg.URL == "" {
		return fmt.Errorf("mcp http: empty URL")
	}
	return nil
}

func (t *httpTransport) Send(ctx context.Context, msg *MCPMessage) error {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return ErrTransportClosed
	}
	t.mu.Unlock()
	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("mcp http: marshal: %w", err)
	}
	go t.sendOne(ctx, body)
	return nil
}

func (t *httpTransport) sendOne(ctx context.Context, body []byte) {
	req, err := http.NewRequestWithContext(ctx, "POST", t.cfg.URL, bytes.NewReader(body))
	if err != nil {
		t.recvCh <- &recvItem{err: fmt.Errorf("mcp http: build request: %w", err)}
		return
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range t.cfg.Headers {
		req.Header.Set(k, v)
	}
	if t.cfg.TokenSource != nil {
		tok, err := t.cfg.TokenSource.Token()
		if err != nil {
			t.recvCh <- &recvItem{err: fmt.Errorf("%w: %v", ErrOAuthRequired, err)}
			return
		}
		req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	}
	resp, err := t.client.Do(req)
	if err != nil {
		t.recvCh <- &recvItem{err: fmt.Errorf("mcp http: do: %w", err)}
		return
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 32*1024*1024))
	if resp.StatusCode == 401 {
		if t.cfg.OAuthEnabled {
			t.recvCh <- &recvItem{err: ErrOAuthRequired}
		} else {
			t.recvCh <- &recvItem{err: fmt.Errorf("%w: 401 %s", ErrProtocol, string(respBody))}
		}
		return
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		t.recvCh <- &recvItem{err: fmt.Errorf("%w: status %d: %s", ErrProtocol, resp.StatusCode, string(respBody))}
		return
	}
	var m MCPMessage
	if err := json.Unmarshal(respBody, &m); err != nil {
		t.recvCh <- &recvItem{err: fmt.Errorf("%w: parse response: %v", ErrProtocol, err)}
		return
	}
	t.recvCh <- &recvItem{msg: &m}
}

func (t *httpTransport) Recv(ctx context.Context) (*MCPMessage, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case item := <-t.recvCh:
		if item.err != nil {
			return nil, item.err
		}
		return item.msg, nil
	}
}

func (t *httpTransport) Close() error {
	t.mu.Lock()
	t.closed = true
	t.mu.Unlock()
	return nil
}
