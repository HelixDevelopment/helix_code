package commands

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/tools"
)

// fakeLSPManager is an in-test implementation of the commands.LSPManager
// interface. It lets us drive the slash command deterministically without
// spawning real subprocesses (the LSPManager itself is tested in T05).
type fakeLSPManager struct {
	servers      []tools.ServerInfo
	restartCalls []string
	stopCalls    []string
	restartErr   error
	stopErr      error
}

func (f *fakeLSPManager) Servers() []tools.ServerInfo { return f.servers }

func (f *fakeLSPManager) Restart(ctx context.Context, name string) error {
	f.restartCalls = append(f.restartCalls, name)
	return f.restartErr
}

func (f *fakeLSPManager) Stop(ctx context.Context, name string) error {
	f.stopCalls = append(f.stopCalls, name)
	return f.stopErr
}

func newLSPCommand(t *testing.T) (*LSPCommand, *fakeLSPManager) {
	t.Helper()
	mgr := &fakeLSPManager{}
	specs := []tools.LSPServerSpec{
		{Name: "gopls", Binary: "gopls", FileExtensions: []string{".go"}, LanguageID: "go"},
		{Name: "rust-analyzer", Binary: "rust-analyzer", FileExtensions: []string{".rs"}, LanguageID: "rust"},
	}
	return NewLSPCommand(mgr, specs), mgr
}

func TestLSPCommand_NameDescription(t *testing.T) {
	c, _ := newLSPCommand(t)
	assert.Equal(t, "lsp", c.Name())
	// Description()/Usage() route through the CONST-046 tr() seam; the
	// default NoopTranslator echoes the message ID verbatim.
	assert.Equal(t, "internal_commands_lsp_description", c.Description())
	assert.Equal(t, "internal_commands_lsp_usage", c.Usage())
	assert.Nil(t, c.Aliases())
}

func TestLSPCommand_DefaultIsListServers(t *testing.T) {
	c, _ := newLSPCommand(t)
	res, err := c.Execute(context.Background(), &CommandContext{Args: nil})
	require.NoError(t, err)
	assert.True(t, res.Success)
	// list-servers output header includes ON-PATH and RUNNING columns.
	assert.Contains(t, res.Output, "NAME")
	assert.Contains(t, res.Output, "ON-PATH")
	assert.Contains(t, res.Output, "RUNNING")
	// curated names appear
	assert.Contains(t, res.Output, "gopls")
	assert.Contains(t, res.Output, "rust-analyzer")
}

func TestLSPCommand_StatusListsRunningServers(t *testing.T) {
	c, mgr := newLSPCommand(t)
	mgr.servers = []tools.ServerInfo{
		{
			Name:       "gopls",
			Status:     tools.ServerStatusReady,
			PID:        12345,
			OpenFiles:  3,
			Uptime:     5 * time.Minute,
			LastActive: time.Now(),
		},
		{
			Name:       "rust-analyzer",
			Status:     tools.ServerStatusIdle,
			PID:        12346,
			OpenFiles:  0,
			Uptime:     30 * time.Second,
			LastActive: time.Now(),
		},
	}
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"status"}})
	require.NoError(t, err)
	assert.True(t, res.Success)
	assert.Contains(t, res.Output, "gopls")
	assert.Contains(t, res.Output, "rust-analyzer")
	assert.Contains(t, res.Output, "ready")
	assert.Contains(t, res.Output, "idle")
	assert.Contains(t, res.Output, "12345")
}

func TestLSPCommand_StatusEmpty(t *testing.T) {
	c, _ := newLSPCommand(t)
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"status"}})
	require.NoError(t, err)
	assert.True(t, res.Success)
	assert.Equal(t, "internal_commands_lsp_no_servers_running", res.Output)
}

func TestLSPCommand_RestartCallsManager(t *testing.T) {
	c, mgr := newLSPCommand(t)
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"restart", "gopls"}})
	require.NoError(t, err)
	assert.True(t, res.Success)
	assert.Equal(t, []string{"gopls"}, mgr.restartCalls)
	// Output routes through the CONST-046 tr() seam; the NoopTranslator
	// echoes the message ID (the server name is template data).
	assert.Contains(t, res.Output, "internal_commands_lsp_restarted")
}

func TestLSPCommand_RestartMissingName(t *testing.T) {
	c, mgr := newLSPCommand(t)
	_, err := c.Execute(context.Background(), &CommandContext{Args: []string{"restart"}})
	require.Error(t, err)
	assert.Empty(t, mgr.restartCalls)
}

func TestLSPCommand_RestartManagerErrorPropagates(t *testing.T) {
	c, mgr := newLSPCommand(t)
	mgr.restartErr = assert.AnError
	_, err := c.Execute(context.Background(), &CommandContext{Args: []string{"restart", "gopls"}})
	require.Error(t, err)
	assert.Equal(t, []string{"gopls"}, mgr.restartCalls)
}

func TestLSPCommand_StopCallsManager(t *testing.T) {
	c, mgr := newLSPCommand(t)
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"stop", "gopls"}})
	require.NoError(t, err)
	assert.True(t, res.Success)
	assert.Equal(t, []string{"gopls"}, mgr.stopCalls)
	// Output routes through the CONST-046 tr() seam; the NoopTranslator
	// echoes the message ID (the server name is template data).
	assert.Contains(t, res.Output, "internal_commands_lsp_stopped")
}

func TestLSPCommand_StopMissingName(t *testing.T) {
	c, mgr := newLSPCommand(t)
	_, err := c.Execute(context.Background(), &CommandContext{Args: []string{"stop"}})
	require.Error(t, err)
	assert.Empty(t, mgr.stopCalls)
}

func TestLSPCommand_StopManagerErrorPropagates(t *testing.T) {
	c, mgr := newLSPCommand(t)
	mgr.stopErr = assert.AnError
	_, err := c.Execute(context.Background(), &CommandContext{Args: []string{"stop", "gopls"}})
	require.Error(t, err)
	assert.Equal(t, []string{"gopls"}, mgr.stopCalls)
}

func TestLSPCommand_ListServersShowsCurated(t *testing.T) {
	c, _ := newLSPCommand(t)
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"list-servers"}})
	require.NoError(t, err)
	assert.True(t, res.Success)
	assert.Contains(t, res.Output, "gopls")
	assert.Contains(t, res.Output, "rust-analyzer")
	// Both extensions surface so users can see what files each spec covers.
	assert.Contains(t, res.Output, ".go")
	assert.Contains(t, res.Output, ".rs")
	// Neither is running in this test; both should report RUNNING: no.
	// We assert at least one "no" appears somewhere in the table body
	// (the literal "no" is independent of locale).
	assert.Contains(t, res.Output, "no")
}

func TestLSPCommand_ListServersOnPathReflectsLookPath(t *testing.T) {
	c, _ := newLSPCommand(t)
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"list-servers"}})
	require.NoError(t, err)
	// gopls's on-path value depends on the host. Recompute it the same
	// way the production code does and assert the row reflects reality.
	_, goplsErr := exec.LookPath("gopls")
	if goplsErr == nil {
		// If gopls really is on PATH the row must say "yes" somewhere.
		assert.Contains(t, res.Output, "yes")
	}
	// Either way, the row must contain the spec name.
	assert.Contains(t, res.Output, "gopls")
}

func TestLSPCommand_ListServersShowsRunning(t *testing.T) {
	c, mgr := newLSPCommand(t)
	mgr.servers = []tools.ServerInfo{
		{
			Name:       "gopls",
			Status:     tools.ServerStatusReady,
			PID:        9000,
			OpenFiles:  2,
			Uptime:     time.Minute,
			LastActive: time.Now(),
		},
	}
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"list-servers"}})
	require.NoError(t, err)
	assert.True(t, res.Success)
	// gopls is now running; rust-analyzer is not.
	assert.Contains(t, res.Output, "gopls")
	assert.Contains(t, res.Output, "rust-analyzer")
	assert.Contains(t, res.Output, "yes")
	assert.Contains(t, res.Output, "no")
}

func TestLSPCommand_UnknownSubcommandErrors(t *testing.T) {
	c, _ := newLSPCommand(t)
	_, err := c.Execute(context.Background(), &CommandContext{Args: []string{"bogus"}})
	require.Error(t, err)
}

func TestLSPCommand_NilCuratedSpecsIsAllowed(t *testing.T) {
	mgr := &fakeLSPManager{}
	cmd := NewLSPCommand(mgr, nil)
	// list-servers with no curated specs renders an empty table (header
	// only) — no panic, no error.
	res, err := cmd.Execute(context.Background(), &CommandContext{Args: []string{"list-servers"}})
	require.NoError(t, err)
	assert.True(t, res.Success)
	assert.Contains(t, res.Output, "NAME")
}
