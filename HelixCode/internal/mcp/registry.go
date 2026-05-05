package mcp

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

// ClientStatus is a snapshot for /mcp listing.
type ClientStatus struct {
	Name      string
	Transport TransportType
	State     ClientState
	ToolCount int
	LastError string
}

// Manager aggregates Clients and exposes tools to the rest of HelixCode.
type Manager struct {
	mu      sync.RWMutex
	clients map[string]*Client
	cfg     *Config
}

// NewManager returns an empty Manager.
func NewManager() *Manager {
	return &Manager{clients: map[string]*Client{}}
}

// SetConfig replaces the loaded config (used by Reload).
func (m *Manager) SetConfig(c *Config) {
	m.mu.Lock()
	m.cfg = c
	m.mu.Unlock()
}

// Config returns the active configuration (may be nil).
func (m *Manager) Config() *Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cfg
}

// addClient registers a Client (used by tests + Start/Reload).
func (m *Manager) addClient(c *Client) {
	m.mu.Lock()
	m.clients[c.Name()] = c
	m.mu.Unlock()
}

// Tools returns all tools across ready clients, server-prefixed.
func (m *Manager) Tools() []ExternalTool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []ExternalTool{}
	for _, c := range m.clients {
		if c.State() != StateReady {
			continue
		}
		for _, t := range c.Tools() {
			t.Server = c.Name()
			out = append(out, t)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Server != out[j].Server {
			return out[i].Server < out[j].Server
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// CallTool routes to the named server's Client.
func (m *Manager) CallTool(ctx context.Context, server, tool string, args map[string]any) (*CallResult, error) {
	m.mu.RLock()
	c, ok := m.clients[server]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrServerNotFound, server)
	}
	return c.CallTool(ctx, tool, args)
}

// Status returns a snapshot for /mcp listing.
func (m *Manager) Status() []ClientStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []ClientStatus{}
	for _, c := range m.clients {
		out = append(out, ClientStatus{
			Name:      c.Name(),
			Transport: c.transport.Type(),
			State:     c.State(),
			ToolCount: len(c.Tools()),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Start dials all alwaysLoad clients in parallel. cfg must be set.
func (m *Manager) Start(ctx context.Context) error {
	m.mu.RLock()
	cfg := m.cfg
	m.mu.RUnlock()
	if cfg == nil {
		return nil
	}
	var wg sync.WaitGroup
	for _, srv := range cfg.Servers {
		c, err := m.buildClient(srv)
		if err != nil {
			return err
		}
		m.addClient(c)
		if !srv.AlwaysLoad {
			continue
		}
		wg.Add(1)
		go func(c *Client) {
			defer wg.Done()
			_ = c.Connect(ctx)
		}(c)
	}
	wg.Wait()
	return nil
}

// buildClient constructs a Client from a ServerSpec via the transport factory.
func (m *Manager) buildClient(s ServerSpec) (*Client, error) {
	tr, err := buildTransport(s)
	if err != nil {
		return nil, fmt.Errorf("mcp: server %s: %w", s.Name, err)
	}
	return NewClient(s.Name, tr), nil
}

// Test runs a one-shot connect → tools/list → close cycle.
func (m *Manager) Test(ctx context.Context, name string) error {
	m.mu.RLock()
	cfg := m.cfg
	m.mu.RUnlock()
	if cfg == nil {
		return fmt.Errorf("mcp: no config loaded")
	}
	for _, s := range cfg.Servers {
		if s.Name != name {
			continue
		}
		tr, err := buildTransport(s)
		if err != nil {
			return err
		}
		c := NewClient(name, tr)
		if err := c.Connect(ctx); err != nil {
			return err
		}
		_ = c.Close()
		return nil
	}
	return ErrServerNotFound
}

// Reload diffs config and reconciles clients.
func (m *Manager) Reload(ctx context.Context, newCfg *Config) error {
	m.mu.Lock()
	old := m.cfg
	m.cfg = newCfg
	m.mu.Unlock()

	want := map[string]ServerSpec{}
	for _, s := range newCfg.Servers {
		want[s.Name] = s
	}
	have := map[string]ServerSpec{}
	if old != nil {
		for _, s := range old.Servers {
			have[s.Name] = s
		}
	}

	// removed
	for name := range have {
		if _, ok := want[name]; ok {
			continue
		}
		m.mu.Lock()
		c, ok := m.clients[name]
		delete(m.clients, name)
		m.mu.Unlock()
		if ok {
			_ = c.Close()
		}
	}
	// added or changed
	for name, spec := range want {
		prev, exists := have[name]
		if exists && specEqual(prev, spec) {
			continue
		}
		if exists {
			m.mu.Lock()
			c, ok := m.clients[name]
			if ok {
				delete(m.clients, name)
			}
			m.mu.Unlock()
			if ok {
				_ = c.Close()
			}
		}
		c, err := m.buildClient(spec)
		if err != nil {
			return err
		}
		m.addClient(c)
		if spec.AlwaysLoad {
			go func(c *Client) { _ = c.Connect(ctx) }(c)
		}
	}
	return nil
}

// specEqual is a shallow equality check for ServerSpec.
func specEqual(a, b ServerSpec) bool {
	if a.Name != b.Name || a.Transport != b.Transport ||
		a.URL != b.URL || a.SSEURL != b.SSEURL ||
		a.Cwd != b.Cwd || a.AlwaysLoad != b.AlwaysLoad ||
		a.OAuth != b.OAuth {
		return false
	}
	if len(a.Env) != len(b.Env) {
		return false
	}
	for k, v := range a.Env {
		if b.Env[k] != v {
			return false
		}
	}
	if len(a.Command) != len(b.Command) {
		return false
	}
	for i := range a.Command {
		if a.Command[i] != b.Command[i] {
			return false
		}
	}
	return true
}

// Close shuts every client down.
func (m *Manager) Close() error {
	m.mu.Lock()
	clients := make([]*Client, 0, len(m.clients))
	for _, c := range m.clients {
		clients = append(clients, c)
	}
	m.clients = map[string]*Client{}
	m.mu.Unlock()
	var wg sync.WaitGroup
	for _, c := range clients {
		wg.Add(1)
		go func(c *Client) {
			defer wg.Done()
			_ = c.Close()
		}(c)
	}
	wg.Wait()
	return nil
}

// buildTransport factors a ServerSpec into a Transport.
func buildTransport(s ServerSpec) (Transport, error) {
	switch s.Transport {
	case TransportStdio:
		return NewStdioTransport(StdioConfig{
			Command: s.Command,
			Env:     s.Env,
			Cwd:     s.Cwd,
		}), nil
	case TransportHTTP:
		return NewHTTPTransport(HTTPConfig{
			URL:          s.URL,
			OAuthEnabled: s.OAuth.Enabled,
		}), nil
	case TransportSSE:
		return NewSSETransport(SSEConfig{
			PostURL:      s.URL,
			SSEURL:       s.SSEURL,
			OAuthEnabled: s.OAuth.Enabled,
		}), nil
	case TransportWS:
		return NewWSTransport(WSConfig{
			URL:          s.URL,
			OAuthEnabled: s.OAuth.Enabled,
		}), nil
	default:
		return nil, fmt.Errorf("mcp: unsupported transport %q", s.Transport)
	}
}
