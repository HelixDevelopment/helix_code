package main

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/mcp"
)

func TestMCPAdd_StdioWritesYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "mcp.yml")
	cmd := newMCPCommand(MCPCommandDeps{ConfigPath: cfgPath})
	cmd.SetArgs([]string{"add", "echo", "--transport=stdio", "--command", "echo", "--command", "hello"})
	require.NoError(t, cmd.Execute())
	cfg, err := mcp.LoadConfig(cfgPath)
	require.NoError(t, err)
	require.Len(t, cfg.Servers, 1)
	assert.Equal(t, mcp.TransportStdio, cfg.Servers[0].Transport)
	assert.Equal(t, []string{"echo", "hello"}, cfg.Servers[0].Command)
}

func TestMCPRemove_DropsEntry(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "mcp.yml")
	require.NoError(t, mcp.SaveConfig(cfgPath, &mcp.Config{
		Servers: []mcp.ServerSpec{
			{Name: "a", Transport: mcp.TransportStdio, Command: []string{"x"}},
			{Name: "b", Transport: mcp.TransportStdio, Command: []string{"y"}},
		},
	}))
	cmd := newMCPCommand(MCPCommandDeps{ConfigPath: cfgPath})
	cmd.SetArgs([]string{"remove", "a"})
	require.NoError(t, cmd.Execute())
	cfg, err := mcp.LoadConfig(cfgPath)
	require.NoError(t, err)
	require.Len(t, cfg.Servers, 1)
	assert.Equal(t, "b", cfg.Servers[0].Name)
}

func TestMCPList_PrintsTable(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "mcp.yml")
	require.NoError(t, mcp.SaveConfig(cfgPath, &mcp.Config{
		Servers: []mcp.ServerSpec{{Name: "a", Transport: mcp.TransportStdio, Command: []string{"x"}}},
	}))
	cmd := newMCPCommand(MCPCommandDeps{ConfigPath: cfgPath})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"list"})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, buf.String(), "a")
	assert.Contains(t, buf.String(), "stdio")
}

func TestMCPTest_InvokesManagerTest(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "mcp.yml")
	require.NoError(t, mcp.SaveConfig(cfgPath, &mcp.Config{
		Servers: []mcp.ServerSpec{{Name: "a", Transport: mcp.TransportStdio, Command: []string{"true"}}},
	}))
	called := false
	cmd := newMCPCommand(MCPCommandDeps{
		ConfigPath: cfgPath,
		TestServer: func(ctx context.Context, name string) error {
			called = true
			assert.Equal(t, "a", name)
			return nil
		},
	})
	cmd.SetArgs([]string{"test", "a"})
	require.NoError(t, cmd.Execute())
	assert.True(t, called)
}
