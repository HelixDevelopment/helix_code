// Command lsp_fakeserver is a tiny but real Language Server Protocol
// server used by the LSPManager test suite (and the F13 challenge harness)
// to exercise the LSP pipeline end-to-end without requiring a real
// language server such as gopls or rust-analyzer to be installed.
//
// The binary speaks real LSP-framed JSON-RPC over stdio via
// go.lsp.dev/jsonrpc2 — the LSPManager subprocess driver cannot tell
// it apart from a real server. This is NOT a mock: it is a real binary
// compiled by go build, started by the test as a real subprocess, that
// reads/writes real LSP frames on its real stdin/stdout.
//
// Behaviour:
//
//   - "initialize" request → returns ServerCapabilities with full text
//     document sync.
//   - "initialized" notification → no-op acknowledgement.
//   - "textDocument/didOpen" notification → scans the document content
//     for lines matching the regex `^//\s*@fake-error:\s*(.+)$`; for
//     each match, emits one error-severity entry in a
//     "textDocument/publishDiagnostics" notification back to the client.
//   - "textDocument/didChange" notification → re-runs the same scan on
//     the new full-content payload (this server only supports full
//     sync, matching what LSPClient sends in T04).
//   - "shutdown" request → replies with null.
//   - "exit" notification → process exits with code 0.
//
// All other inbound messages are silently ignored (notifications) or
// answered with method-not-found (calls). stderr is silent unless
// `-debug` is passed, so tests get clean output.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"io"
	"log"
	"os"
	"regexp"
	"strings"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// fakeErrorPattern matches a single "// @fake-error: <message>" line.
// One match per line; one diagnostic per match.
var fakeErrorPattern = regexp.MustCompile(`(?m)^//\s*@fake-error:\s*(.+)$`)

// stdioTransport adapts os.Stdin / os.Stdout into the io.ReadWriteCloser
// shape that jsonrpc2.NewStream expects.
type stdioTransport struct{}

func (stdioTransport) Read(p []byte) (int, error)  { return os.Stdin.Read(p) }
func (stdioTransport) Write(p []byte) (int, error) { return os.Stdout.Write(p) }
func (stdioTransport) Close() error {
	// We intentionally do not close os.Stdin/os.Stdout here — the
	// process will exit on the LSP `exit` notification, and closing
	// the underlying file descriptors during shutdown can race with
	// jsonrpc2's writer goroutine.
	return nil
}

func main() {
	debug := flag.Bool("debug", false, "enable stderr debug logging")
	flag.Parse()

	if !*debug {
		// Silence the default logger; tests want clean stderr.
		log.SetOutput(io.Discard)
	}

	stream := jsonrpc2.NewStream(stdioTransport{})
	conn := jsonrpc2.NewConn(stream)
	srv := &server{conn: conn, debug: *debug}

	ctx := context.Background()
	conn.Go(ctx, srv.handle)

	<-conn.Done()
}

type server struct {
	conn  jsonrpc2.Conn
	debug bool
}

func (s *server) logf(format string, args ...any) {
	if s.debug {
		log.Printf(format, args...)
	}
}

func (s *server) handle(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	switch req.Method() {

	case protocol.MethodInitialize:
		s.logf("initialize")
		return reply(ctx, &protocol.InitializeResult{
			Capabilities: protocol.ServerCapabilities{
				TextDocumentSync: protocol.TextDocumentSyncKindFull,
			},
			ServerInfo: &protocol.ServerInfo{
				Name:    "helix-fake-lsp",
				Version: "0.1",
			},
		}, nil)

	case protocol.MethodInitialized:
		s.logf("initialized")
		return nil

	case protocol.MethodTextDocumentDidOpen:
		var p protocol.DidOpenTextDocumentParams
		if err := json.Unmarshal(req.Params(), &p); err != nil {
			s.logf("didOpen decode: %v", err)
			return nil
		}
		s.publishDiagnostics(ctx, p.TextDocument.URI, p.TextDocument.Text)
		return nil

	case protocol.MethodTextDocumentDidChange:
		var p protocol.DidChangeTextDocumentParams
		if err := json.Unmarshal(req.Params(), &p); err != nil {
			s.logf("didChange decode: %v", err)
			return nil
		}
		// We negotiated full sync; the last change carries the full content.
		var text string
		if len(p.ContentChanges) > 0 {
			text = p.ContentChanges[len(p.ContentChanges)-1].Text
		}
		s.publishDiagnostics(ctx, p.TextDocument.URI, text)
		return nil

	case protocol.MethodTextDocumentDidClose:
		s.logf("didClose")
		return nil

	case protocol.MethodShutdown:
		s.logf("shutdown")
		return reply(ctx, nil, nil)

	case protocol.MethodExit:
		s.logf("exit")
		// Flush stderr just in case before bailing.
		os.Exit(0)
		return nil

	default:
		// Unknown method: if it's a Call we must reply with method-not-found
		// so the client doesn't hang; for Notifications we drop silently.
		if _, isCall := req.(*jsonrpc2.Call); isCall {
			return reply(ctx, nil, jsonrpc2.NewError(jsonrpc2.MethodNotFound, "method not implemented by fake server"))
		}
		return nil
	}
}

// publishDiagnostics scans `text` for the @fake-error pragma and pushes
// one publishDiagnostics notification with one diagnostic per match.
// An empty match list is published explicitly so the client can see
// "no errors" after a successful didChange.
func (s *server) publishDiagnostics(ctx context.Context, docURI uri.URI, text string) {
	lines := strings.Split(text, "\n")
	var diags []protocol.Diagnostic
	for i, line := range lines {
		m := fakeErrorPattern.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		msg := strings.TrimSpace(m[1])
		diags = append(diags, protocol.Diagnostic{
			Severity: protocol.DiagnosticSeverityError,
			Source:   "helix-fake-lsp",
			Code:     "FAKE001",
			Message:  msg,
			Range: protocol.Range{
				Start: protocol.Position{Line: uint32(i), Character: 0},
				End:   protocol.Position{Line: uint32(i), Character: uint32(len(line))},
			},
		})
	}
	params := protocol.PublishDiagnosticsParams{
		URI:         docURI,
		Diagnostics: diags,
	}
	if err := s.conn.Notify(ctx, protocol.MethodTextDocumentPublishDiagnostics, params); err != nil {
		s.logf("publishDiagnostics send failed: %v", err)
	}
}
