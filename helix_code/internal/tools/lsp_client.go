package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"sync/atomic"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// LSPClient is a thin wrapper over a jsonrpc2.Conn that drives the LSP
// handshake, ferries text-document notifications to the server, and
// translates inbound publishDiagnostics notifications into HelixCode's
// stable Diagnostic shape.
//
// The transport is decoupled: callers supply any io.ReadWriteCloser. The
// real LSPManager (T05) hands in a subprocess's stdin+stdout fused into
// one ReadWriteCloser; tests hand in a paired io.Pipe duplex.
type LSPClient struct {
	conn       jsonrpc2.Conn
	transport  io.ReadWriteCloser
	serverName string

	// initialized guards Initialize idempotency.
	initMu      sync.Mutex
	initialized bool

	// versions tracks the per-document version counter so DidChange can
	// publish strictly increasing version numbers (required by LSP).
	versionsMu sync.Mutex
	versions   map[string]int32

	// diagnostics holds the most recent publishDiagnostics result per file.
	// Each republish replaces the slice wholesale (LSP semantics: the
	// publish event is the authoritative current set).
	diagMu      sync.RWMutex
	diagnostics map[string][]Diagnostic

	// diagCounter monotonically tags each wrapped Diagnostic so its ID is
	// unique across the lifetime of the client.
	diagCounter uint64

	// shutdown is set once Shutdown has been called so Close becomes a
	// best-effort no-op rather than racing with the run goroutine.
	shutdownOnce sync.Once
}

// NewLSPClient wraps an io.ReadWriteCloser and starts the inbound dispatch
// goroutine. serverName (e.g., "gopls") is stamped onto every wrapped
// Diagnostic as Source so downstream tools can attribute findings.
func NewLSPClient(transport io.ReadWriteCloser, serverName string) *LSPClient {
	stream := jsonrpc2.NewStream(transport)
	conn := jsonrpc2.NewConn(stream)
	c := &LSPClient{
		conn:        conn,
		transport:   transport,
		serverName:  serverName,
		versions:    make(map[string]int32),
		diagnostics: make(map[string][]Diagnostic),
	}
	conn.Go(context.Background(), c.handle)
	return c
}

// handle is the jsonrpc2 inbound handler. We only care about
// textDocument/publishDiagnostics; every other server-originated message
// is acknowledged with a method-not-found so the server's run loop does
// not stall.
func (c *LSPClient) handle(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	switch req.Method() {
	case protocol.MethodTextDocumentPublishDiagnostics:
		var params protocol.PublishDiagnosticsParams
		if err := json.Unmarshal(req.Params(), &params); err != nil {
			return reply(ctx, nil, fmt.Errorf("publishDiagnostics decode: %w", err))
		}
		c.onPublishDiagnostics(params)
		// Notification: per spec, no reply is sent for notifications.
		// The jsonrpc2 library distinguishes Call vs Notification at
		// the message layer, so we just return nil.
		return nil
	}
	// For any other request, reply with method-not-found if it's a Call;
	// for Notifications, just drop it silently.
	if _, isCall := req.(*jsonrpc2.Call); isCall {
		return reply(ctx, nil, jsonrpc2.NewError(jsonrpc2.MethodNotFound, "method not handled by helixcode lsp client"))
	}
	return nil
}

// Initialize performs the LSP initialize handshake, then sends the
// `initialized` notification. Idempotent: a second call is a no-op.
//
// rootDir becomes file://<rootDir> in InitializeParams.RootURI.
// capabilities is forwarded as InitializationOptions; pass nil for none.
func (c *LSPClient) Initialize(ctx context.Context, rootDir string, capabilities map[string]any) error {
	c.initMu.Lock()
	defer c.initMu.Unlock()
	if c.initialized {
		return nil
	}

	params := &protocol.InitializeParams{
		ProcessID:             0,
		RootURI:               uri.File(rootDir),
		InitializationOptions: capabilities,
		Capabilities:          protocol.ClientCapabilities{},
	}
	var result protocol.InitializeResult
	if _, err := c.conn.Call(ctx, protocol.MethodInitialize, params, &result); err != nil {
		return fmt.Errorf("lsp initialize: %w", err)
	}
	if err := c.conn.Notify(ctx, protocol.MethodInitialized, &protocol.InitializedParams{}); err != nil {
		return fmt.Errorf("lsp initialized: %w", err)
	}
	c.initialized = true
	return nil
}

// DidOpen sends textDocument/didOpen with the file's full content.
func (c *LSPClient) DidOpen(ctx context.Context, filePath, languageID, content string) error {
	c.versionsMu.Lock()
	c.versions[filePath] = 1
	c.versionsMu.Unlock()

	params := &protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:        uri.File(filePath),
			LanguageID: protocol.LanguageIdentifier(languageID),
			Version:    1,
			Text:       content,
		},
	}
	if err := c.conn.Notify(ctx, protocol.MethodTextDocumentDidOpen, params); err != nil {
		return fmt.Errorf("lsp didOpen: %w", err)
	}
	return nil
}

// DidChange sends textDocument/didChange using full-content sync (the
// simplest LSP sync mode and the only one we promise to T05's manager).
//
// The document version is bumped from whatever DidOpen seeded; if DidOpen
// was never called we start at 1 anyway so the server still gets a valid
// monotonically-increasing version stream.
func (c *LSPClient) DidChange(ctx context.Context, filePath, content string) error {
	c.versionsMu.Lock()
	c.versions[filePath]++
	if c.versions[filePath] < 2 {
		c.versions[filePath] = 2
	}
	v := c.versions[filePath]
	c.versionsMu.Unlock()

	params := &protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: uri.File(filePath)},
			Version:                v,
		},
		ContentChanges: []protocol.TextDocumentContentChangeEvent{{Text: content}},
	}
	if err := c.conn.Notify(ctx, protocol.MethodTextDocumentDidChange, params); err != nil {
		return fmt.Errorf("lsp didChange: %w", err)
	}
	return nil
}

// DidClose sends textDocument/didClose. The client also drops any cached
// diagnostics for the file so subsequent GetDiagnostics returns empty.
func (c *LSPClient) DidClose(ctx context.Context, filePath string) error {
	params := &protocol.DidCloseTextDocumentParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(filePath)},
	}
	if err := c.conn.Notify(ctx, protocol.MethodTextDocumentDidClose, params); err != nil {
		return fmt.Errorf("lsp didClose: %w", err)
	}
	c.diagMu.Lock()
	delete(c.diagnostics, filePath)
	c.diagMu.Unlock()
	return nil
}

// GetDiagnostics returns the most recent diagnostics for filePath. The
// returned slice is a defensive copy; mutating it does not affect the
// client's internal state. Returns an empty (non-nil) slice if the file
// has never been seen.
func (c *LSPClient) GetDiagnostics(filePath string) []Diagnostic {
	c.diagMu.RLock()
	defer c.diagMu.RUnlock()
	src, ok := c.diagnostics[filePath]
	if !ok {
		return []Diagnostic{}
	}
	out := make([]Diagnostic, len(src))
	copy(out, src)
	return out
}

// AllDiagnostics returns the union of every cached diagnostic across all
// known files. Order is unspecified.
func (c *LSPClient) AllDiagnostics() []Diagnostic {
	c.diagMu.RLock()
	defer c.diagMu.RUnlock()
	var out []Diagnostic
	for _, ds := range c.diagnostics {
		out = append(out, ds...)
	}
	return out
}

// Shutdown sends LSP `shutdown` (request) followed by `exit` (notification),
// then closes the underlying transport. After Shutdown, the client is
// no longer usable.
func (c *LSPClient) Shutdown(ctx context.Context) error {
	var first error
	c.shutdownOnce.Do(func() {
		var ack json.RawMessage
		if _, err := c.conn.Call(ctx, protocol.MethodShutdown, nil, &ack); err != nil {
			first = fmt.Errorf("lsp shutdown: %w", err)
			// keep going so the server still sees exit
		}
		if err := c.conn.Notify(ctx, protocol.MethodExit, nil); err != nil && first == nil {
			first = fmt.Errorf("lsp exit: %w", err)
		}
		// Close the conn (and through it the transport) so the
		// inbound goroutine drains.
		_ = c.conn.Close()
	})
	return first
}

// Close closes the underlying transport. Safe to call multiple times and
// safe to call after Shutdown (in which case it is a no-op).
func (c *LSPClient) Close() error {
	_ = c.conn.Close()
	_ = c.transport.Close()
	return nil
}

// onPublishDiagnostics maps protocol.PublishDiagnosticsParams onto the
// internal cache. We store one slice per file path; each entry is wrapped
// with a fresh unique ID, the file path attached, and Source defaulted to
// the configured serverName when the upstream diagnostic did not specify
// one.
func (c *LSPClient) onPublishDiagnostics(p protocol.PublishDiagnosticsParams) {
	filePath := p.URI.Filename()
	wrapped := make([]Diagnostic, 0, len(p.Diagnostics))
	for _, d := range p.Diagnostics {
		id := atomic.AddUint64(&c.diagCounter, 1)
		// Per T04 spec, the configured serverName is authoritative for
		// Source — that's what the LLM and downstream tools attribute
		// findings to. The upstream protocol.Diagnostic.Source (e.g.
		// "compiler") is dropped on the floor; if we want it back later
		// we can store it in Code or a future RawSource field.
		wrapped = append(wrapped, Diagnostic{
			ID: fmt.Sprintf("%s:%s:%d", c.serverName, filePath, id),
			// LSP severity values 1-4 align with our enum 1-4 by
			// design (lsp_types.go documents the cast).
			Severity: DiagnosticSeverity(int(d.Severity)),
			Code:     codeToString(d.Code),
			Source:   c.serverName,
			Message:  d.Message,
			Range: Range{
				Start: Position{Line: int(d.Range.Start.Line), Character: int(d.Range.Start.Character)},
				End:   Position{Line: int(d.Range.End.Line), Character: int(d.Range.End.Character)},
			},
			FilePath: filePath,
		})
	}
	c.diagMu.Lock()
	c.diagnostics[filePath] = wrapped
	c.diagMu.Unlock()
}

// codeToString flattens LSP's interface{} Code field (string|int) into a
// stable string form. nil becomes "" so JSON omits the field via omitempty.
func codeToString(code interface{}) string {
	switch v := code.(type) {
	case nil:
		return ""
	case string:
		return v
	case float64:
		return fmt.Sprintf("%g", v)
	case int:
		return fmt.Sprintf("%d", v)
	case int32:
		return fmt.Sprintf("%d", v)
	case int64:
		return fmt.Sprintf("%d", v)
	default:
		return fmt.Sprintf("%v", v)
	}
}
