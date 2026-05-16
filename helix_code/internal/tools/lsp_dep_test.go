package tools_test

import (
	"testing"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
)

// TestLSPDepsImportable proves the LSP dependencies are wired so subsequent
// F13 tasks can build against them. P1-F13-T02 deliverable.
func TestLSPDepsImportable(t *testing.T) {
	_ = jsonrpc2.NewStream
	_ = protocol.MethodInitialize
}
