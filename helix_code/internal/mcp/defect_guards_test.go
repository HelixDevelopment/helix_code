package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// redMode reports whether the RED polarity switch is active (§11.4.115).
//
// Default (RED_MODE unset or "0"): GREEN — the same source is the STANDING
// regression guard, asserting the defect is ABSENT on the fixed artifact. This
// is the default so the standing suite stays green (§11.4.135).
//
// RED_MODE=1: RED — reproduce the historical defect against a PRE-FIX artifact
// and assert it is PRESENT (the proof the guard is real). Run against the
// pre-fix build to capture the RED evidence.
func redMode(t *testing.T) bool {
	t.Helper()
	return os.Getenv("RED_MODE") == "1"
}

// syntheticSecret is a fake, obviously-not-real credential used so no real
// secret value is ever printed or written by these tests (CONST-042/§11.4.10).
const syntheticSecret = "SYNTHETIC-FAKE-TOKEN-do-not-use-7f3a"

// --- DEFECT-1: config secrets written plaintext 0644 (secret leak) ---
//
// LoadConfig expands ${ENV} references into in-memory Env plaintext values;
// SaveConfig wrote the file with mode 0644 (world-readable) AND persisted the
// expanded plaintext secret. A load->save round-trip therefore leaked the
// expanded secret to a world-readable file. Fix: write 0600 AND re-collapse the
// ${ENV} reference on save so the plaintext is never written to disk.
func TestDefect1_SaveConfig_DoesNotLeakExpandedSecret(t *testing.T) {
	t.Setenv("HELIX_MCP_TEST_SECRET", syntheticSecret)

	dir := t.TempDir()
	srcPath := filepath.Join(dir, "mcp.yml")
	yaml := []byte(`
servers:
  - name: secretsrv
    transport: stdio
    command: ["echo"]
    env:
      API_TOKEN: "${HELIX_MCP_TEST_SECRET}"
`)
	require.NoError(t, os.WriteFile(srcPath, yaml, 0o644))

	cfg, err := LoadConfig(srcPath)
	require.NoError(t, err)
	require.Len(t, cfg.Servers, 1)

	outPath := filepath.Join(dir, "out.yml")
	require.NoError(t, SaveConfig(outPath, cfg))

	info, err := os.Stat(outPath)
	require.NoError(t, err)
	mode := info.Mode().Perm()

	data, err := os.ReadFile(outPath)
	require.NoError(t, err)
	content := string(data)

	// Report only booleans, NEVER the secret value, so a failing test cannot
	// leak the synthetic (or, in a misconfig, a real) secret into logs.
	leaksPlaintext := contains(content, syntheticSecret)
	worldReadable := mode&0o077 != 0

	if redMode(t) {
		// RED on the broken artifact: at least one defect facet must be present.
		assert.True(t, leaksPlaintext || worldReadable,
			"RED expected the pre-fix defect (plaintext-secret-on-disk and/or world-readable mode); leaksPlaintext=%v mode=%v", leaksPlaintext, mode)
	} else {
		// GREEN guard: neither facet may be present.
		assert.False(t, leaksPlaintext, "saved config must NOT contain the plaintext secret on disk")
		assert.False(t, worldReadable, "saved config must be mode 0600 (got %v)", mode)
		// The ${ENV} reference must be preserved so the round-trip stays correct.
		assert.True(t, contains(content, "${HELIX_MCP_TEST_SECRET}"),
			"saved config must preserve the ${ENV} reference instead of the expanded value")
	}
}

// Non-secret fields must round-trip correctly regardless of the secret handling.
func TestDefect1_SaveConfig_PreservesNonSecretFields(t *testing.T) {
	if redMode(t) {
		t.Skip("SKIP-OK: correctness invariant only asserted as GREEN guard (RED_MODE=0)")
	}
	t.Setenv("HELIX_MCP_TEST_SECRET", syntheticSecret)
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "mcp.yml")
	yaml := []byte(`
servers:
  - name: secretsrv
    transport: stdio
    command: ["echo", "hello"]
    cwd: /tmp/work
    alwaysLoad: true
    env:
      API_TOKEN: "${HELIX_MCP_TEST_SECRET}"
      PLAIN: "literal-value"
`)
	require.NoError(t, os.WriteFile(srcPath, yaml, 0o644))

	cfg, err := LoadConfig(srcPath)
	require.NoError(t, err)

	outPath := filepath.Join(dir, "out.yml")
	require.NoError(t, SaveConfig(outPath, cfg))

	got, err := LoadConfig(outPath)
	require.NoError(t, err)
	require.Len(t, got.Servers, 1)
	s := got.Servers[0]
	assert.Equal(t, "secretsrv", s.Name)
	assert.Equal(t, []string{"echo", "hello"}, s.Command)
	assert.Equal(t, "/tmp/work", s.Cwd)
	assert.True(t, s.AlwaysLoad)
	// Non-secret literal env survives untouched.
	assert.Equal(t, "literal-value", s.Env["PLAIN"])
	// The ${ENV} reference re-expands to the runtime secret value on reload.
	assert.Equal(t, syntheticSecret, s.Env["API_TOKEN"])
}

// --- DEFECT-2: ID vs Name dispatch mismatch (tools uncallable) ---
//
// RegisterTool keyed the map by tool.ID; handleListTools advertised tool.Name;
// handleCallTool looked up s.tools[params.Name]. When ID != Name a spec client
// lists tools (gets Names) then calls by Name -> -32601 Tool not found. Fix:
// register/advertise/dispatch consistently by the advertised Name.
func TestDefect2_ToolCallableByAdvertisedName(t *testing.T) {
	server := NewMCPServer()
	tool := &Tool{
		ID:          "internal-id-123",
		Name:        "advertised_tool_name",
		Description: "A tool whose ID differs from its Name",
		Parameters:  map[string]interface{}{},
		Handler: func(ctx context.Context, session *MCPSession, args map[string]interface{}) (interface{}, error) {
			return "ok", nil
		},
	}
	require.NoError(t, server.RegisterTool(tool))

	// 1) list tools -> capture the advertised name a spec client would receive.
	listConn := &MockConn{}
	listSession := newGuardSession(listConn)
	server.handleListTools(listSession, &MCPMessage{ID: "list", Method: "tools/list"})
	advertised := firstAdvertisedToolName(t, listConn.lastMessage)
	require.Equal(t, "advertised_tool_name", advertised,
		"spec client receives the Name field from tools/list")

	// 2) call the tool using the EXACT advertised name.
	callConn := &MockConn{}
	callSession := newGuardSession(callConn)
	callParams, _ := json.Marshal(map[string]interface{}{
		"name":      advertised,
		"arguments": map[string]interface{}{},
	})
	server.handleCallTool(context.Background(), callSession, &MCPMessage{
		ID:     "call",
		Method: "tools/call",
		Params: callParams,
	})

	var resp MCPMessage
	require.NoError(t, json.Unmarshal(callConn.lastMessage, &resp))

	if redMode(t) {
		// RED on the broken artifact: calling by advertised Name is -32601.
		require.NotNil(t, resp.Error, "RED expected the dispatch mismatch (-32601 on call-by-advertised-name)")
		assert.Equal(t, -32601, resp.Error.Code)
	} else {
		// GREEN guard: dispatch by advertised Name succeeds.
		assert.Nil(t, resp.Error, "calling by advertised name must dispatch (got error %+v)", resp.Error)
		require.NotNil(t, resp.Result)
	}
}

// Duplicate rejection must keep working after the key change.
func TestDefect2_DuplicateRejectionStillWorks(t *testing.T) {
	if redMode(t) {
		t.Skip("SKIP-OK: invariant only asserted as GREEN guard (RED_MODE=0)")
	}
	server := NewMCPServer()
	mk := func() *Tool {
		return &Tool{
			ID:   "dup-id",
			Name: "dup_name",
			Handler: func(ctx context.Context, s *MCPSession, a map[string]interface{}) (interface{}, error) {
				return nil, nil
			},
		}
	}
	require.NoError(t, server.RegisterTool(mk()))
	err := server.RegisterTool(mk())
	require.Error(t, err, "registering a tool with the same advertised name must be rejected")
	assert.Equal(t, 1, server.GetToolCount())
}

// --- helpers ---

func newGuardSession(conn WebSocketConn) *MCPSession {
	return &MCPSession{
		ID:           uuid.New(),
		Conn:         conn,
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
		Context:      make(map[string]interface{}),
	}
}

func firstAdvertisedToolName(t *testing.T, raw []byte) string {
	t.Helper()
	var msg MCPMessage
	require.NoError(t, json.Unmarshal(raw, &msg))
	result, ok := msg.Result.(map[string]interface{})
	require.True(t, ok, "tools/list result must be an object")
	tools, ok := result["tools"].([]interface{})
	require.True(t, ok, "tools/list result must contain a tools array")
	require.NotEmpty(t, tools)
	first, ok := tools[0].(map[string]interface{})
	require.True(t, ok)
	name, ok := first["name"].(string)
	require.True(t, ok, "advertised tool must carry a name")
	return name
}

func contains(haystack, needle string) bool {
	return len(needle) > 0 && len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
