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
	// Description/Usage route through the CONST-046 tr() seam; under
	// the default NoopTranslator they echo the message ID (round-399).
	assert.NotEmpty(t, c.Description())
	assert.Contains(t, c.Usage(), "internal_commands_sandbox_usage")
	assert.Nil(t, c.Aliases())
}

func TestSandboxCommand_DefaultIsStatus(t *testing.T) {
	c, _ := newSandboxCommand(t)
	res, err := c.Execute(context.Background(), &CommandContext{Args: nil})
	require.NoError(t, err)
	assert.True(t, res.Success)
	// status output mentions the status header (CONST-046 message ID
	// under the default NoopTranslator) + the Backend table line
	// (also a CONST-046 message ID since round-406).
	assert.Contains(t, res.Output, "internal_commands_sandbox_status_header")
	assert.Contains(t, res.Output, "internal_commands_sandbox_label_backend")
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
	// CONST-046: the "Sandbox unavailable: <reason>" line is translated;
	// the default NoopTranslator echoes the message ID and the reason is
	// only interpolated into {{.Reason}} under a real translator.
	assert.Contains(t, res.Output, "internal_commands_sandbox_unavailable")
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
	// round-406: "Exit code:" label is now a CONST-046 message ID.
	assert.Contains(t, res.Output, "internal_commands_sandbox_label_exit_code")
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
	// round-406: "Exit code:" label is now a CONST-046 message ID.
	assert.Contains(t, res.Output, "internal_commands_sandbox_label_exit_code")
	assert.Contains(t, res.Output, "42")
}

func TestSandboxCommand_PolicyShowsDenyList(t *testing.T) {
	c, mgr := newSandboxCommand(t)
	mgr.userDeny = []string{`^rm -rf /`, `dd\s+if=`}
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"policy"}})
	require.NoError(t, err)
	assert.True(t, res.Success)
	assert.Contains(t, res.Output, "CONST-033")
	// CONST-046: deny-list headers are translated message IDs under the
	// default NoopTranslator.
	assert.Contains(t, res.Output, "internal_commands_sandbox_user_denylist_header")
	assert.Contains(t, res.Output, "^rm -rf /")
	assert.Contains(t, res.Output, "dd\\s+if=")
}

func TestSandboxCommand_PolicyEmptyUserList(t *testing.T) {
	c, _ := newSandboxCommand(t)
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"policy"}})
	require.NoError(t, err)
	assert.True(t, res.Success)
	// CONST-046: header + empty-list note are translated message IDs
	// under the default NoopTranslator.
	assert.Contains(t, res.Output, "internal_commands_sandbox_user_denylist_header")
	assert.Contains(t, res.Output, "internal_commands_sandbox_user_denylist_empty")
}

func TestSandboxCommand_PolicyShowsDefaultPolicy(t *testing.T) {
	c, _ := newSandboxCommand(t)
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"policy"}})
	require.NoError(t, err)
	assert.True(t, res.Success)
	// CONST-046: the "Default policy:" header and (since round-406) the
	// tabwriter field labels are translated message IDs under the default
	// NoopTranslator; the data values below them remain literal.
	assert.Contains(t, res.Output, "internal_commands_sandbox_default_policy_header")
	assert.Contains(t, res.Output, "internal_commands_sandbox_label_network_allowed")
	assert.Contains(t, res.Output, "internal_commands_sandbox_label_timeout")
	assert.Contains(t, res.Output, "internal_commands_sandbox_label_read_only_root")
	assert.Contains(t, res.Output, "30s")
}

func TestSandboxCommand_UnknownSubcommandErrors(t *testing.T) {
	c, _ := newSandboxCommand(t)
	_, err := c.Execute(context.Background(), &CommandContext{Args: []string{"bogus"}})
	require.Error(t, err)
}

// --- Round-346 CONST-046 paired-mutation tests -----------------------------
//
// Each asserts the migrated user-facing literal now routes through the
// package tr() seam. With a sentinel translator wired the output MUST
// contain the sentinel-wrapped message ID; an inlined literal fails it.

func TestSandboxCommand_StatusHeader_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c, _ := newSandboxCommand(t)
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"status"}})
	require.NoError(t, err)
	assert.Contains(t, res.Output, "<TR:internal_commands_sandbox_status_header>")
}

func TestSandboxCommand_Unavailable_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c, mgr := newSandboxCommand(t)
	mgr.caps = sandbox.SandboxCapabilities{
		GOOS:              "linux",
		SelectedBackend:   sandbox.BackendNone,
		UnavailableReason: "bubblewrap not found",
	}
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"status"}})
	require.NoError(t, err)
	assert.Contains(t, res.Output, "<TR:internal_commands_sandbox_unavailable>")
}

func TestSandboxCommand_UnknownSubcommand_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c, _ := newSandboxCommand(t)
	_, err := c.Execute(context.Background(), &CommandContext{Args: []string{"bogus"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "<TR:internal_commands_sandbox_unknown_subcommand>")
}

func TestSandboxCommand_PolicyHeader_GoesThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c, _ := newSandboxCommand(t)
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"policy"}})
	require.NoError(t, err)
	assert.Contains(t, res.Output, "<TR:internal_commands_sandbox_default_policy_header>")
	assert.Contains(t, res.Output, "<TR:internal_commands_sandbox_const_denylist_header>")
	assert.Contains(t, res.Output, "<TR:internal_commands_sandbox_user_denylist_header>")
}

// --- Round-406 CONST-046 paired-mutation tests -----------------------------
//
// The status/test/policy tabwriter field labels migrated in round-406.
// With the sentinel translator wired, every migrated label MUST surface
// as a sentinel-wrapped message ID; an inlined literal fails the assert.

func TestSandboxCommand_StatusLabels_GoThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c, _ := newSandboxCommand(t)
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"status"}})
	require.NoError(t, err)
	for _, id := range []string{
		"internal_commands_sandbox_label_goos",
		"internal_commands_sandbox_label_backend",
		"internal_commands_sandbox_label_bubblewrap_path",
		"internal_commands_sandbox_label_unprivileged_userns",
		"internal_commands_sandbox_label_cgroups_v2",
		"internal_commands_sandbox_label_default_network",
		"internal_commands_sandbox_label_default_timeout",
		"internal_commands_sandbox_label_user_deny_rules",
		"internal_commands_sandbox_value_deny",
	} {
		assert.Contains(t, res.Output, "<TR:"+id+">")
	}
}

func TestSandboxCommand_TestLabels_GoThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c, mgr := newSandboxCommand(t)
	mgr.executeResult = &sandbox.SandboxResult{
		Stdout:   "hi\n",
		ExitCode: 0,
		Backend:  sandbox.BackendBubblewrap,
		Duration: 7 * time.Millisecond,
		TimedOut: true,
	}
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"test", "echo", "hi"}})
	require.NoError(t, err)
	for _, id := range []string{
		"internal_commands_sandbox_label_test_command",
		"internal_commands_sandbox_label_backend",
		"internal_commands_sandbox_label_exit_code",
		"internal_commands_sandbox_label_duration",
		"internal_commands_sandbox_label_timed_out",
	} {
		assert.Contains(t, res.Output, "<TR:"+id+">")
	}
}

func TestSandboxCommand_PolicyLabels_GoThroughTranslator(t *testing.T) {
	resetTranslator(t)
	SetTranslator(sentinelTranslator{})
	defer resetTranslator(t)

	c, _ := newSandboxCommand(t)
	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"policy"}})
	require.NoError(t, err)
	for _, id := range []string{
		"internal_commands_sandbox_label_network_allowed",
		"internal_commands_sandbox_label_timeout",
		"internal_commands_sandbox_label_read_only_root",
		"internal_commands_sandbox_label_memory_limit_mb",
		"internal_commands_sandbox_label_cpu_limit_pct",
	} {
		assert.Contains(t, res.Output, "<TR:"+id+">")
	}
}
