package mcp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"sync"
	"sync/atomic"
)

// ExternalTool is what Manager exposes to tools/registry: a tool living on
// some MCP server, identified by server name + tool name.
type ExternalTool struct {
	Server string
	Name   string
	Schema map[string]any // jsonSchema for inputSchema
	Title  string
	Desc   string
}

// CallResult is what CallTool returns.
type CallResult struct {
	Content []map[string]any
	IsError bool
	Raw     interface{}
}

// Client is one connected MCP server.
type Client struct {
	name       string
	transport  Transport
	state      atomic.Int32
	mu         sync.Mutex
	tools      []ExternalTool
	pending    map[string]chan *MCPMessage
	nextID     atomic.Int64
	done       chan struct{}
	onEvent    func(Event)
	pendCap    int
	closed     bool               // guarded by mu
	recvCancel context.CancelFunc // guarded by mu; cancels the active recvLoop goroutine
}

// NewClient builds a Client around a Transport. transport.Open is called on Connect.
func NewClient(name string, transport Transport) *Client {
	c := &Client{
		name:      name,
		transport: transport,
		pending:   make(map[string]chan *MCPMessage),
		done:      make(chan struct{}),
		pendCap:   1024,
	}
	c.state.Store(int32(StateDisconnected))
	return c
}

// Name returns the configured server name.
func (c *Client) Name() string { return c.name }

// Tools returns the cached tool list (post-handshake).
func (c *Client) Tools() []ExternalTool {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]ExternalTool, len(c.tools))
	copy(out, c.tools)
	return out
}

// State returns the current state.
func (c *Client) State() ClientState { return ClientState(c.state.Load()) }

func (c *Client) setState(s ClientState) {
	c.state.Store(int32(s))
	c.mu.Lock()
	fn := c.onEvent
	c.mu.Unlock()
	if fn != nil {
		fn(Event{Server: c.name, State: s})
	}
}

// SetOnEvent installs an event callback.
func (c *Client) SetOnEvent(fn func(Event)) {
	c.mu.Lock()
	c.onEvent = fn
	c.mu.Unlock()
}

func (c *Client) nextRPCID() string {
	return "rpc-" + strconv.FormatInt(c.nextID.Add(1), 10)
}

// Connect opens the transport and runs the initialize/tools-list handshake.
func (c *Client) Connect(ctx context.Context) error {
	c.setState(StateConnecting)
	if err := c.transport.Open(ctx); err != nil {
		c.setState(StateDisconnected)
		return fmt.Errorf("mcp client %s: open: %w", c.name, err)
	}
	rctx, cancel := context.WithCancel(context.Background())
	c.mu.Lock()
	c.recvCancel = cancel
	c.mu.Unlock()
	go c.recvLoop(rctx)
	c.setState(StateInitializing)
	if err := c.handshake(ctx); err != nil {
		c.mu.Lock()
		if c.recvCancel != nil {
			c.recvCancel()
			c.recvCancel = nil
		}
		c.mu.Unlock()
		c.setState(StateDisconnected)
		return err
	}
	c.setState(StateReady)
	return nil
}

func (c *Client) handshake(ctx context.Context) error {
	initID := c.nextRPCID()
	initResp, err := c.callRaw(ctx, initID, "initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "helixcode", "version": "1.0"},
	})
	if err != nil {
		return fmt.Errorf("%w: initialize: %v", ErrInitFailed, err)
	}
	if initResp.Error != nil {
		return fmt.Errorf("%w: server rejected initialize: %v", ErrInitFailed, initResp.Error)
	}

	if err := c.transport.Send(ctx, &MCPMessage{JSONRPC: "2.0", Method: "notifications/initialized"}); err != nil {
		return fmt.Errorf("%w: notifications/initialized: %v", ErrInitFailed, err)
	}

	listID := c.nextRPCID()
	listResp, err := c.callRaw(ctx, listID, "tools/list", map[string]any{})
	if err != nil {
		return fmt.Errorf("%w: tools/list: %v", ErrInitFailed, err)
	}
	c.mu.Lock()
	c.tools = parseToolsListResult(listResp.Result)
	c.mu.Unlock()
	return nil
}

func parseToolsListResult(raw interface{}) []ExternalTool {
	out := []ExternalTool{}
	m, ok := raw.(map[string]any)
	if !ok {
		return out
	}
	tools, ok := m["tools"].([]interface{})
	if !ok {
		// Try as []map[string]any (test helpers may pass this shape).
		if arr, ok2 := m["tools"].([]map[string]any); ok2 {
			for _, td := range arr {
				if t := parseSingleTool(td); t.Name != "" {
					out = append(out, t)
				}
			}
			return out
		}
		return out
	}
	for _, item := range tools {
		td, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if t := parseSingleTool(td); t.Name != "" {
			out = append(out, t)
		}
	}
	return out
}

func parseSingleTool(td map[string]any) ExternalTool {
	t := ExternalTool{}
	if v, ok := td["name"].(string); ok {
		t.Name = v
	}
	if v, ok := td["title"].(string); ok {
		t.Title = v
	}
	if v, ok := td["description"].(string); ok {
		t.Desc = v
	}
	if v, ok := td["inputSchema"].(map[string]any); ok {
		t.Schema = v
	}
	return t
}

// CallTool invokes a tool on the server. State must be ready.
func (c *Client) CallTool(ctx context.Context, name string, args map[string]any) (*CallResult, error) {
	if c.State() != StateReady {
		return nil, ErrNotReady
	}
	id := c.nextRPCID()
	resp, err := c.callRaw(ctx, id, "tools/call", map[string]any{
		"name":      name,
		"arguments": args,
	})
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		if resp.Error.Code == -32601 {
			return nil, ErrToolNotFound
		}
		return nil, fmt.Errorf("mcp call %s: %s", name, resp.Error.Message)
	}
	cr := &CallResult{Raw: resp.Result}
	if rm, ok := resp.Result.(map[string]any); ok {
		if v, ok := rm["isError"].(bool); ok {
			cr.IsError = v
		}
		if items, ok := rm["content"].([]interface{}); ok {
			for _, it := range items {
				if m, ok := it.(map[string]any); ok {
					cr.Content = append(cr.Content, m)
				}
			}
		} else if items, ok := rm["content"].([]map[string]any); ok {
			cr.Content = append(cr.Content, items...)
		}
	}
	return cr, nil
}

func (c *Client) callRaw(ctx context.Context, id, method string, params map[string]any) (*MCPMessage, error) {
	c.mu.Lock()
	if len(c.pending) >= c.pendCap {
		c.mu.Unlock()
		return nil, ErrTooManyPending
	}
	ch := make(chan *MCPMessage, 1)
	c.pending[id] = ch
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
	}()

	if err := c.transport.Send(ctx, &MCPMessage{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}); err != nil {
		return nil, err
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.done:
		return nil, ErrTransportClosed
	case msg := <-ch:
		return msg, nil
	}
}

func (c *Client) recvLoop(ctx context.Context) {
	for {
		msg, err := c.transport.Recv(ctx)
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, ErrTransportClosed) || ctx.Err() != nil {
				return
			}
			// fail all pending RPCs with a transport error
			c.mu.Lock()
			for id, ch := range c.pending {
				select {
				case ch <- &MCPMessage{Error: &MCPError{Code: -32000, Message: err.Error()}, ID: id}:
				default:
				}
			}
			c.mu.Unlock()
			return
		}
		if msg == nil {
			continue
		}
		idStr := messageIDString(msg.ID)
		if idStr == "" {
			// notifications are not surfaced (no agent surface for them yet)
			continue
		}
		c.mu.Lock()
		ch, ok := c.pending[idStr]
		c.mu.Unlock()
		if ok {
			select {
			case ch <- msg:
			default:
			}
		}
	}
}

func messageIDString(id interface{}) string {
	switch v := id.(type) {
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case int:
		return strconv.Itoa(v)
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", v)
	}
}

// Close shuts the client down. It is safe to call concurrently and more than once.
func (c *Client) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	close(c.done)
	if c.recvCancel != nil {
		c.recvCancel()
	}
	c.mu.Unlock()
	c.setState(StateClosed)
	return c.transport.Close()
}
