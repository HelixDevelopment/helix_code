package tools

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"dev.helix.code/internal/approval"
	"dev.helix.code/internal/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMcpTool_RequiresApproval_ReportsConfiguredLevel verifies the core
// read-only mechanism: an mcpTool registered at LevelReadOnly reports
// LevelReadOnly (so the ReadOnlyOnly agent loop accepts it), while one
// registered at LevelEdit reports LevelEdit (blocked by that loop).
func TestMcpTool_RequiresApproval_ReportsConfiguredLevel(t *testing.T) {
	ro := &mcpTool{name: "fs__read_file", approvalLevel: approval.LevelReadOnly}
	assert.Equal(t, approval.LevelReadOnly, ro.RequiresApproval(),
		"read-only-configured mcpTool must report LevelReadOnly")

	rw := &mcpTool{name: "fs__write_file", approvalLevel: approval.LevelEdit}
	assert.Equal(t, approval.LevelEdit, rw.RequiresApproval(),
		"unclassified mcpTool must keep the conservative LevelEdit default")
}

// TestIsReadOnlyMCPToolName checks the well-known read-only tool-name
// allowlist used when a server is not explicitly flagged readOnly.
func TestIsReadOnlyMCPToolName(t *testing.T) {
	readOnly := []string{
		"read_file", "read_text_file", "read_multiple_files",
		"list_directory", "directory_tree", "search_files",
		"search", "get_file_info", "list_allowed_directories",
		"READ_FILE", // case-insensitive
	}
	for _, n := range readOnly {
		assert.Truef(t, isReadOnlyMCPToolName(n), "%q should be read-only", n)
	}
	mutating := []string{"write_file", "edit_file", "move_file", "create_directory"}
	for _, n := range mutating {
		assert.Falsef(t, isReadOnlyMCPToolName(n), "%q must NOT be read-only", n)
	}
}

// TestRegisterMCPManager_ReadOnlyServer_Live boots the real official
// @modelcontextprotocol/server-filesystem over stdio, registers it via a
// readOnly:true config, and asserts EVERY registered MCP tool — including
// the server's mutating tools (write_file, edit_file) — reports
// LevelReadOnly because the server is flagged read-only. This is the
// anti-bluff proof that the gate actually changes the level the
// ReadOnlyOnly loop sees. Skips (never fakes) if npx is unavailable.
func TestRegisterMCPManager_ReadOnlyServer_Live(t *testing.T) {
	if _, err := exec.LookPath("npx"); err != nil {
		t.Skip("SKIP-OK: npx not on PATH; cannot boot the real filesystem MCP server")
	}

	cfg := &mcp.Config{Servers: []mcp.ServerSpec{{
		Name:       "fs",
		Transport:  mcp.TransportStdio,
		Command:    []string{"npx", "-y", "@modelcontextprotocol/server-filesystem", "."},
		AlwaysLoad: true,
		ReadOnly:   true,
	}}}
	require.NoError(t, cfg.Validate())

	m := mcp.NewManager()
	m.SetConfig(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	require.NoError(t, m.Start(ctx))
	defer m.Close()

	// alwaysLoad connect is async — wait for tools to appear.
	var live []mcp.ExternalTool
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if live = m.Tools(); len(live) > 0 {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	require.NotEmpty(t, live, "fs MCP server exposed no tools; cannot validate read-only gating")

	reg, err := NewToolRegistry(nil)
	require.NoError(t, err)
	reg.RegisterMCPManager(m)

	// Every fs tool — read AND write — must report LevelReadOnly because the
	// server is readOnly:true.
	for _, et := range live {
		name := mcpToolRegisteredName(et.Server, et.Name)
		tool, err := reg.Get(name)
		require.NoErrorf(t, err, "tool %q should be registered", name)
		assert.Equalf(t, approval.LevelReadOnly, tool.RequiresApproval(),
			"read-only server's tool %q must report LevelReadOnly so the ReadOnlyOnly loop accepts it", name)
	}

	// Spot-check a couple of well-known names exist (live wire evidence).
	_, err = reg.Get("fs__read_text_file")
	assert.NoError(t, err, "fs__read_text_file should be registered from the live server")
}

// TestRegisterMCPManager_DefaultServer_NameBasedReadOnly boots the same
// live server WITHOUT the readOnly flag and asserts the per-tool name
// allowlist still gates the read tools to LevelReadOnly while the mutating
// tools keep LevelEdit.
func TestRegisterMCPManager_DefaultServer_NameBasedReadOnly(t *testing.T) {
	if _, err := exec.LookPath("npx"); err != nil {
		t.Skip("SKIP-OK: npx not on PATH; cannot boot the real filesystem MCP server")
	}

	cfg := &mcp.Config{Servers: []mcp.ServerSpec{{
		Name:       "fs",
		Transport:  mcp.TransportStdio,
		Command:    []string{"npx", "-y", "@modelcontextprotocol/server-filesystem", "."},
		AlwaysLoad: true,
		// ReadOnly intentionally NOT set — relies on the name allowlist.
	}}}
	require.NoError(t, cfg.Validate())

	m := mcp.NewManager()
	m.SetConfig(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	require.NoError(t, m.Start(ctx))
	defer m.Close()

	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if len(m.Tools()) > 0 {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	require.NotEmpty(t, m.Tools())

	reg, err := NewToolRegistry(nil)
	require.NoError(t, err)
	reg.RegisterMCPManager(m)

	readTool, err := reg.Get("fs__read_text_file")
	require.NoError(t, err)
	assert.Equal(t, approval.LevelReadOnly, readTool.RequiresApproval(),
		"read_text_file is a known read-only name → LevelReadOnly even without the server flag")

	writeTool, err := reg.Get("fs__write_file")
	require.NoError(t, err)
	assert.Equal(t, approval.LevelEdit, writeTool.RequiresApproval(),
		"write_file is not a known read-only name and server is not flagged → LevelEdit")
}
