// Unit tests for the internal/mcp package-level translator +
// tr() helper (CONST-046 round-164 §11.4 anti-bluff sweep,
// 2026-05-19).
//
// Paired-mutation test per §11.4: planted/unplanted Translator yields
// distinguishable output at every migrated call site. Mocks ALLOWED
// per CONST-050(A) (unit tests only).
package mcp

import (
	"bytes"
	stdctx "context"
	"encoding/json"
	"errors"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	mcpi18n "dev.helix.code/internal/mcp/i18n"
)

// sentinelTranslator returns "<TR:" + id + ">" so call-site tests can
// assert tr() actually went through Translator.T rather than returning
// a hardcoded literal that happened to match the bundle value.
type sentinelTranslator struct{}

func (sentinelTranslator) T(_ stdctx.Context, id string, _ map[string]any) (string, error) {
	return "<TR:" + id + ">", nil
}
func (sentinelTranslator) TPlural(_ stdctx.Context, id string, _ int, _ map[string]any) (string, error) {
	return "<TR:" + id + ">", nil
}

type errTranslator struct{}

func (errTranslator) T(_ stdctx.Context, _ string, _ map[string]any) (string, error) {
	return "", errors.New("intentional translator failure")
}
func (errTranslator) TPlural(_ stdctx.Context, _ string, _ int, _ map[string]any) (string, error) {
	return "", errors.New("intentional translator failure")
}

// resetTranslator restores the package-level translator after each
// test so cross-test pollution can't mask a regression.
func resetTranslator(t *testing.T) {
	t.Helper()
	SetTranslator(nil)
}

// captureLog redirects log output to a buffer so call-site tests can
// assert tr() actually ran on the path.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	buf := new(bytes.Buffer)
	old := log.Writer()
	flags := log.Flags()
	prefix := log.Prefix()
	log.SetOutput(buf)
	log.SetFlags(0)
	log.SetPrefix("")
	t.Cleanup(func() {
		log.SetOutput(old)
		log.SetFlags(flags)
		log.SetPrefix(prefix)
	})
	return buf
}

func TestTr_DefaultsToNoopTranslator(t *testing.T) {
	resetTranslator(t)
	got := tr(stdctx.Background(), "internal_mcp_server_tool_registered", nil)
	if got == "internal_mcp_server_tool_registered" || got == "" {
		t.Fatalf("HXC-097 §11.4.120: default/nil path must resolve to bundle prose, got %q (raw key or empty)", got)
	}
}

func TestTr_UsesInjectedTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	got := tr(stdctx.Background(), "internal_mcp_server_session_started", nil)
	if got != "<TR:internal_mcp_server_session_started>" {
		t.Fatalf("tr = %q, want sentinel-wrapped ID — call site bypassed Translator", got)
	}
}

func TestTr_TranslatorErrorReturnsMessageID(t *testing.T) {
	// Anti-bluff: an erroring Translator MUST NOT silently return an
	// empty string (that would be a §11.4 PASS-bluff at the i18n
	// layer — user sees blank output). Implementation MUST degrade to
	// the message ID.
	resetTranslator(t)
	SetTranslator(errTranslator{})
	defer resetTranslator(t)

	got := tr(stdctx.Background(), "internal_mcp_server_method_not_found", nil)
	if got != "internal_mcp_server_method_not_found" {
		t.Fatalf("tr on err = %q, want raw message ID (no silent swallow)", got)
	}
}

func TestSetTranslator_NilResetsToNoop(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	SetTranslator(nil) // explicit reset
	defer resetTranslator(t)

	got := tr(stdctx.Background(), "internal_mcp_server_tool_registered", nil)
	if got == "internal_mcp_server_tool_registered" || got == "" {
		t.Fatalf("HXC-097 §11.4.120: default/nil path must resolve to bundle prose, got %q (raw key or empty)", got)
	}
}

func TestSetTranslator_AcceptsNoopExplicit(t *testing.T) {
	resetTranslator(t)
	defer resetTranslator(t)

	SetTranslator(mcpi18n.NoopTranslator{})
	got := tr(stdctx.Background(), "internal_mcp_server_session_ended", nil)
	if got != "internal_mcp_server_session_ended" {
		t.Fatalf("tr with explicit NoopTranslator = %q, want raw ID", got)
	}
}

// TestRegisterTool_DuplicateGoesThroughTranslator covers the duplicate
// tool registration error string. With a sentinel translator wired,
// the returned error MUST embed the sentinel-wrapped message ID —
// proving the literal was NOT hardcoded on the path.
func TestRegisterTool_DuplicateGoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	srv := NewMCPServer()
	tool := &Tool{ID: "dup-tool", Name: "Dup", Parameters: map[string]interface{}{}}
	if err := srv.RegisterTool(tool); err != nil {
		t.Fatalf("first RegisterTool failed: %v", err)
	}
	err := srv.RegisterTool(tool)
	if err == nil {
		t.Fatalf("expected duplicate registration error, got nil")
	}
	want := "<TR:internal_mcp_server_tool_already_registered>"
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("err = %q, want contain %q — duplicate path bypassed tr()", err.Error(), want)
	}
}

// TestRegisterTool_SuccessLogsThroughTranslator covers the
// registration-confirmation log line. With a sentinel translator
// wired, the captured log MUST surface the sentinel-wrapped ID.
func TestRegisterTool_SuccessLogsThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	buf := captureLog(t)

	srv := NewMCPServer()
	tool := &Tool{ID: "log-tool", Name: "Log", Parameters: map[string]interface{}{}}
	if err := srv.RegisterTool(tool); err != nil {
		t.Fatalf("RegisterTool failed: %v", err)
	}

	out := buf.String()
	want := "<TR:internal_mcp_server_tool_registered>"
	if !strings.Contains(out, want) {
		t.Fatalf("log = %q, want contain %q — registration log bypassed tr()", out, want)
	}
}

// TestHandleMessage_UnknownMethodGoesThroughTranslator covers the
// JSON-RPC -32601 "Method not found" string emitted by handleMessage's
// default branch. With a sentinel translator wired, the response error
// message MUST surface the sentinel-wrapped ID.
func TestHandleMessage_UnknownMethodGoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	srv := NewMCPServer()
	mockConn := &MockConn{}
	session := &MCPSession{
		ID:           uuid.New(),
		Conn:         mockConn,
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
		Context:      make(map[string]interface{}),
	}

	msg := &MCPMessage{ID: "x-1", Type: "request", Method: "completely-unknown-method"}
	srv.handleMessage(session, msg)

	if !mockConn.writeCalled {
		t.Fatalf("handleMessage did not write a response")
	}
	var resp MCPMessage
	if err := json.Unmarshal(mockConn.lastMessage, &resp); err != nil {
		t.Fatalf("response unmarshal failed: %v", err)
	}
	if resp.Error == nil {
		t.Fatalf("expected JSON-RPC error, got nil")
	}
	want := "<TR:internal_mcp_server_method_not_found>"
	if resp.Error.Message != want {
		t.Fatalf("error.message = %q, want %q — unknown-method path bypassed tr()", resp.Error.Message, want)
	}
}

// TestHandleInitialize_ServerNameGoesThroughTranslator covers the
// "HelixCode MCP Server" serverInfo.name returned in the initialize
// handshake. With a sentinel translator wired, the published name MUST
// surface the sentinel-wrapped ID.
func TestHandleInitialize_ServerNameGoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	srv := NewMCPServer()
	mockConn := &MockConn{}
	session := &MCPSession{
		ID:           uuid.New(),
		Conn:         mockConn,
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
		Context:      make(map[string]interface{}),
	}

	params := map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]interface{}{},
		"clientInfo":      map[string]interface{}{"name": "t", "version": "1"},
	}
	paramsJSON, _ := json.Marshal(params)
	msg := &MCPMessage{ID: "i-1", Type: "request", Method: "initialize", Params: paramsJSON}

	srv.handleInitialize(session, msg)

	if !mockConn.writeCalled {
		t.Fatalf("handleInitialize did not write a response")
	}
	var resp MCPMessage
	if err := json.Unmarshal(mockConn.lastMessage, &resp); err != nil {
		t.Fatalf("response unmarshal failed: %v", err)
	}
	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("response result not a map")
	}
	info, ok := result["serverInfo"].(map[string]interface{})
	if !ok {
		t.Fatalf("serverInfo not a map")
	}
	want := "<TR:internal_mcp_server_info_name>"
	if got := info["name"]; got != want {
		t.Fatalf("serverInfo.name = %v, want %q — initialize path bypassed tr()", got, want)
	}
}

// TestRawText_RegisterToolDefaultEcho asserts that with the default
// bundle translator (loaded via init), RegisterTool emits resolved
// prose — confirming the HXC-097 init() path works correctly.
func TestRawText_RegisterToolDefaultEcho(t *testing.T) {
	resetTranslator(t)

	buf := captureLog(t)

	srv := NewMCPServer()
	tool := &Tool{ID: "echo-tool", Name: "Echo", Parameters: map[string]interface{}{}}
	if err := srv.RegisterTool(tool); err != nil {
		t.Fatalf("RegisterTool failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "✅ MCP Tool registered: Echo (echo-tool)") {
		t.Fatalf("log = %q, want resolved prose for tool registration (HXC-097)", out)
	}
}
