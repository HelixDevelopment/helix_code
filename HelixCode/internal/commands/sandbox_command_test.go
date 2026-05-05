package commands

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/tools/sandbox"
)

// fakeSandboxManager is a hexagonal-seam impl of the commands.SandboxManager
// interface used by /sandbox tests. It records Execute calls so the tests
// can assert that /sandbox test actually delegates to the manager (no
// fmt.Printf + sleep simulation), and lets us drive backend / capability
// fields deterministically without a real bwrap binary.
type fakeSandboxManager struct {
	caps          sandbox.SandboxCapabilities
	cfg           sandbox.SandboxConfig
	constDeny     []string
	userDeny      []string
	executeResult *sandbox.SandboxResult
	executeErr    error

	// Recorded inputs from the last Execute call.
	executeCalls   int
	lastCommand    string
	lastPolicy     sandbox.SandboxPolicy
	lastContextNil bool
}

func (f *fakeSandboxManager) Capabilities() sandbox.SandboxCapabilities { return f.caps }

func (f *fakeSandboxManager) SelectedBackend() sandbox.BackendKind {
	return f.caps.SelectedBackend
}

func (f *fakeSandboxManager) Config() sandbox.SandboxConfig { return f.cfg }

func (f *fakeSandboxManager) MergedDenyList() ([]string, []string) {
	return f.constDeny, f.userDeny
}

func (f *fakeSandboxManager) Execute(ctx context.Context, command string, policy sandbox.SandboxPolicy) (*sandbox.SandboxResult, error) {
	f.executeCalls++
	f.lastCommand = command
	f.lastPolicy = policy
	f.lastContextNil = ctx == nil
	return f.executeResult, f.executeErr
}

func newSandboxCommand(t *testing.T) (*SandboxCommand, *fakeSandboxManager) {
	t.Helper()
	mgr := &fakeSandboxManager{
		caps: sandbox.SandboxCapabilities{
			GOOS:               "linux",
			BubblewrapPath:     "/usr/bin/bwrap",
			UnprivilegedUserNS: true,
			CGroupsV2:          true,
			SelectedBackend:    sandbox.BackendBubblewrap,
		},
		cfg: sandbox.DefaultSandboxConfig(),
		constDeny: []string{
			"CONST-033: systemctl power-management subcommand",
			"CONST-033: loginctl power+session management",
		},
		userDeny: nil,
	}
	return NewSandboxCommand(mgr), mgr
}

func TestSandboxCommand_NameDescription(t *testing.T) {
	c, _ := newSandboxCommand(t)
	assert.Equal(t, "sandbox", c.Name())
	assert.NotEmpty(t, c.Description())
	assert.Contains(t, c.Usage(), "/sandbox")
	assert.Nil(t, c.Aliases())
}

func TestSandboxCommand_DefaultIsStatus(t *testing.T) {
	c, _ := newSandboxCommand(t)
	res, err := c.Execute(context.Background(), &CommandContext{Args: nil})
	require.NoError(t, err)
	assert.True(t, res.Success)
	// status output mentions backend + GOOS lines.
	assert.Contains(t, res.Output, "Sandbox status")
	assert.Contains(t, res.Output, "Backend")
}

func TestSandboxCommand_StatusShowsBackend(t *testing.T) {
	c, _ := newSandboxCommand(t)
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"status"}})
	require.NoError(t, err)
	assert.True(t, res.Success)
	assert.Contains(t, res.Output, "bubblewrap")
	assert.Contains(t, res.Output, "/usr/bin/bwrap")
	assert.Contains(t, res.Output, "linux")
	// default policy timeout from cfg is 30s
	assert.Contains(t, res.Output, "30s")
}

func TestSandboxCommand_StatusShowsFailClosedReason(t *testing.T) {
	c, mgr := newSandboxCommand(t)
	mgr.caps = sandbox.SandboxCapabilities{
		GOOS:              "linux",
		SelectedBackend:   sandbox.BackendNone,
		UnavailableReason: "bubblewrap not found; install bwrap",
	}
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"status"}})
	require.NoError(t, err)
	assert.True(t, res.Success)
	assert.Contains(t, res.Output, "Sandbox unavailable")
	assert.Contains(t, res.Output, "bubblewrap not found; install bwrap")
}

func TestSandboxCommand_TestRunsExecute(t *testing.T) {
	c, mgr := newSandboxCommand(t)
	mgr.executeResult = &sandbox.SandboxResult{
		Stdout:   "hi\n",
		Stderr:   "",
		ExitCode: 0,
		Backend:  sandbox.BackendBubblewrap,
		Duration: 45 * time.Millisecond,
	}
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"test", "echo", "hi"}})
	require.NoError(t, err)
	assert.True(t, res.Success)
	assert.Equal(t, 1, mgr.executeCalls)
	assert.Equal(t, "echo hi", mgr.lastCommand)
	// Default policy was passed (zero policy → DefaultSandboxPolicy applied
	// downstream by manager; here we just assert the command surfaced what
	// the fake reported).
	assert.Contains(t, res.Output, "echo hi")
	assert.Contains(t, res.Output, "bubblewrap")
	assert.Contains(t, res.Output, "Exit code")
	assert.Contains(t, res.Output, "0")
	assert.Contains(t, res.Output, "hi")
}

func TestSandboxCommand_TestDefaultCommand(t *testing.T) {
	c, mgr := newSandboxCommand(t)
	mgr.executeResult = &sandbox.SandboxResult{
		Stdout:   "helix-sandbox-test\n",
		ExitCode: 0,
		Backend:  sandbox.BackendBubblewrap,
		Duration: 30 * time.Millisecond,
	}
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"test"}})
	require.NoError(t, err)
	assert.True(t, res.Success)
	assert.Equal(t, 1, mgr.executeCalls)
	assert.Equal(t, "echo helix-sandbox-test", mgr.lastCommand)
	assert.Contains(t, res.Output, "helix-sandbox-test")
}

func TestSandboxCommand_TestPropagatesError(t *testing.T) {
	c, mgr := newSandboxCommand(t)
	mgr.executeErr = errors.New("sandbox: bubblewrap binary missing")
	_, err := c.Execute(context.Background(), &CommandContext{Args: []string{"test", "echo", "hi"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sandbox: bubblewrap binary missing")
}

func TestSandboxCommand_TestShowsExitCode(t *testing.T) {
	c, mgr := newSandboxCommand(t)
	mgr.executeResult = &sandbox.SandboxResult{
		Stdout:   "",
		Stderr:   "boom\n",
		ExitCode: 42,
		Backend:  sandbox.BackendBubblewrap,
		Duration: 5 * time.Millisecond,
	}
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"test", "false"}})
	require.NoError(t, err)
	assert.True(t, res.Success)
	assert.Contains(t, res.Output, "Exit code")
	assert.Contains(t, res.Output, "42")
}

func TestSandboxCommand_PolicyShowsDenyList(t *testing.T) {
	c, mgr := newSandboxCommand(t)
	mgr.userDeny = []string{`^rm -rf /`, `dd\s+if=`}
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"policy"}})
	require.NoError(t, err)
	assert.True(t, res.Success)
	assert.Contains(t, res.Output, "CONST-033")
	assert.Contains(t, res.Output, "User deny-list")
	assert.Contains(t, res.Output, "^rm -rf /")
	assert.Contains(t, res.Output, "dd\\s+if=")
	// Reports user deny count of 2.
	assert.Contains(t, res.Output, "2")
}

func TestSandboxCommand_PolicyEmptyUserList(t *testing.T) {
	c, _ := newSandboxCommand(t)
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"policy"}})
	require.NoError(t, err)
	assert.True(t, res.Success)
	assert.Contains(t, res.Output, "User deny-list")
	assert.Contains(t, res.Output, "empty")
}

func TestSandboxCommand_PolicyShowsDefaultPolicy(t *testing.T) {
	c, _ := newSandboxCommand(t)
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"policy"}})
	require.NoError(t, err)
	assert.True(t, res.Success)
	assert.Contains(t, res.Output, "Default policy")
	assert.Contains(t, res.Output, "network_allowed")
	assert.Contains(t, res.Output, "timeout")
	assert.Contains(t, res.Output, "read_only_root")
	assert.Contains(t, res.Output, "30s")
}

func TestSandboxCommand_UnknownSubcommandErrors(t *testing.T) {
	c, _ := newSandboxCommand(t)
	_, err := c.Execute(context.Background(), &CommandContext{Args: []string{"bogus"}})
	require.Error(t, err)
}
