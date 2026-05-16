//go:build integration

package integration

// approval_test.go (P2-F21-T07): end-to-end integration tests for the F21
// approval gate wired into the real ToolRegistry.
//
// Test surface (8 cases):
//
//  1. TestApproval_SuggestMode_BlocksEdit
//  2. TestApproval_AutoEdit_AllowsEdit
//  3. TestApproval_AutoEdit_PromptsRun_UserAllows
//  4. TestApproval_AutoEdit_PromptsRun_UserDenies
//  5. TestApproval_FullAuto_InjectsSandboxMarker
//  6. TestApproval_FullAuto_NetworkAllowedFalse
//  7. TestApproval_Dangerous_NoSandboxInjection
//  8. TestApproval_RuntimeChange_AffectsNextExecute
//
// All tests use:
//   - Real *tools.ToolRegistry (DefaultRegistryConfig) — no mocked registry.
//   - Real *approval.ApprovalManager — exercises CheckApproval +
//     PromptForApproval + SetMode through the production code path.
//   - In-process stub tools (stubEditTool, stubRunTool) registered on the
//     real registry. The stubs record the args their Execute receives so we
//     can assert sandbox-marker injection (test cases 5-7) without needing
//     the F14 sandbox backend. These are integration-test fixtures, not
//     production code, and live in the test file only.
//   - A recordingResponder that supplies a deterministic yes/no answer to
//     PromptForApproval. The responder is the only path through the F19
//     prompter we mock (PromptResponder is exactly the seam designed for
//     this; askuser.Prompter has its own integration tests in
//     askuser_test.go).
//
// Anti-bluff anchor: every PASS in this file demonstrates a real gate
// decision propagating through the real registry. There are no temporary
// or stand-in paths in this file. The registry's applyApprovalGate is
// exercised on the hot path; the sandbox-marker assertions are made on
// the args map the stub tool ACTUALLY received from Execute, not on a
// recorded intent.

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/approval"
	"dev.helix.code/internal/tools"
)

// recordingResponder satisfies approval.PromptResponder. Tests configure
// .Allow before each call; the responder records the question text so a
// test can assert PromptForApproval was actually invoked.
type recordingResponder struct {
	Allow        bool
	Err          error
	LastQuestion string
	Calls        int32
}

func (r *recordingResponder) PromptYesNo(_ context.Context, q string, _ bool) (bool, error) {
	atomic.AddInt32(&r.Calls, 1)
	r.LastQuestion = q
	if r.Err != nil {
		return false, r.Err
	}
	return r.Allow, nil
}

// stubTool is the test fixture shared by edit and run-level stubs. It
// records the args it received on Execute (so tests can assert sandbox
// markers were or were not injected) and returns a benign success result.
type stubTool struct {
	name     string
	level    approval.ApprovalLevel
	executed int32
	gotArgs  map[string]interface{}
}

func (s *stubTool) Name() string                                            { return s.name }
func (s *stubTool) Description() string                                     { return "test stub for approval gate" }
func (s *stubTool) Category() tools.ToolCategory                            { return tools.ToolCategory("test-stub") }
func (s *stubTool) Schema() tools.ToolSchema                                { return tools.ToolSchema{Type: "object"} }
func (s *stubTool) Validate(_ map[string]interface{}) error                 { return nil }
func (s *stubTool) RequiresApproval() approval.ApprovalLevel                { return s.level }
func (s *stubTool) Execute(_ context.Context, p map[string]interface{}) (interface{}, error) {
	atomic.AddInt32(&s.executed, 1)
	// Defensive copy so post-Execute mutations by the test don't pollute
	// the recorded snapshot.
	s.gotArgs = make(map[string]interface{}, len(p))
	for k, v := range p {
		s.gotArgs[k] = v
	}
	return "ok", nil
}

// approvalTestEnv bundles a fully-wired registry + manager + tools so each
// test starts from a clean slate. The env is sandbox-available=true by
// default so ModeFullAuto is constructible; tests that need the False
// branch override the field before NewApprovalManager.
type approvalTestEnv struct {
	registry *tools.ToolRegistry
	manager  *approval.ApprovalManager
	editTool *stubTool
	runTool  *stubTool
	resp     *recordingResponder
}

func newApprovalTestEnv(t *testing.T, mode approval.ApprovalMode) *approvalTestEnv {
	t.Helper()
	reg, err := tools.NewToolRegistry(tools.DefaultRegistryConfig())
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = reg.Close()
	})

	editTool := &stubTool{name: "stub_edit_for_approval", level: approval.LevelEdit}
	runTool := &stubTool{name: "stub_run_for_approval", level: approval.LevelRun}
	reg.Register(editTool)
	reg.Register(runTool)

	resp := &recordingResponder{}
	mgr, err := approval.NewApprovalManager(approval.ApprovalManagerOptions{
		InitialMode:      mode,
		Source:           approval.SourceDefault,
		Responder:        resp,
		SandboxAvailable: true,
	})
	require.NoError(t, err, "manager init for mode=%s", mode)
	reg.SetApprovalManager(mgr)

	return &approvalTestEnv{
		registry: reg,
		manager:  mgr,
		editTool: editTool,
		runTool:  runTool,
		resp:     resp,
	}
}

func TestApproval_SuggestMode_BlocksEdit(t *testing.T) {
	env := newApprovalTestEnv(t, approval.ModeSuggest)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := env.registry.Execute(ctx, env.editTool.Name(), map[string]interface{}{"k": "v"})

	require.Error(t, err)
	assert.True(t, errors.Is(err, approval.ErrApprovalDenied),
		"expected ErrApprovalDenied, got: %v", err)
	assert.Equal(t, int32(0), atomic.LoadInt32(&env.editTool.executed),
		"edit tool must NOT execute when suggest-mode denies")
	assert.Equal(t, int32(0), atomic.LoadInt32(&env.resp.Calls),
		"prompter must not be consulted on outright deny")
}

func TestApproval_AutoEdit_AllowsEdit(t *testing.T) {
	env := newApprovalTestEnv(t, approval.ModeAutoEdit)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	res, err := env.registry.Execute(ctx, env.editTool.Name(), map[string]interface{}{"k": "v"})

	require.NoError(t, err)
	assert.Equal(t, "ok", res)
	assert.Equal(t, int32(1), atomic.LoadInt32(&env.editTool.executed))
	assert.Equal(t, int32(0), atomic.LoadInt32(&env.resp.Calls),
		"auto-edit allows Edit-level without prompting")
}

func TestApproval_AutoEdit_PromptsRun_UserAllows(t *testing.T) {
	env := newApprovalTestEnv(t, approval.ModeAutoEdit)
	env.resp.Allow = true

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	res, err := env.registry.Execute(ctx, env.runTool.Name(), map[string]interface{}{"cmd": "echo"})

	require.NoError(t, err)
	assert.Equal(t, "ok", res)
	assert.Equal(t, int32(1), atomic.LoadInt32(&env.runTool.executed))
	assert.Equal(t, int32(1), atomic.LoadInt32(&env.resp.Calls),
		"auto-edit must prompt for Run-level")
	assert.Contains(t, env.resp.LastQuestion, env.runTool.Name())
}

func TestApproval_AutoEdit_PromptsRun_UserDenies(t *testing.T) {
	env := newApprovalTestEnv(t, approval.ModeAutoEdit)
	env.resp.Allow = false

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := env.registry.Execute(ctx, env.runTool.Name(), map[string]interface{}{"cmd": "echo"})

	require.Error(t, err)
	assert.True(t, errors.Is(err, approval.ErrApprovalDenied),
		"expected ErrApprovalDenied, got: %v", err)
	assert.Equal(t, int32(0), atomic.LoadInt32(&env.runTool.executed),
		"run tool must NOT execute after user denies prompt")
	assert.Equal(t, int32(1), atomic.LoadInt32(&env.resp.Calls))
}

func TestApproval_FullAuto_InjectsSandboxMarker(t *testing.T) {
	env := newApprovalTestEnv(t, approval.ModeFullAuto)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := env.registry.Execute(ctx, env.runTool.Name(), map[string]interface{}{"cmd": "echo"})

	require.NoError(t, err)
	require.Equal(t, int32(1), atomic.LoadInt32(&env.runTool.executed))
	require.NotNil(t, env.runTool.gotArgs, "args snapshot must be recorded")

	v, ok := env.runTool.gotArgs["_helix_sandbox_required"]
	require.True(t, ok, "_helix_sandbox_required must be injected for full-auto + Run-level")
	assert.Equal(t, true, v, "_helix_sandbox_required must be true under full-auto")
}

func TestApproval_FullAuto_NetworkAllowedFalse(t *testing.T) {
	env := newApprovalTestEnv(t, approval.ModeFullAuto)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := env.registry.Execute(ctx, env.runTool.Name(), map[string]interface{}{"cmd": "echo"})

	require.NoError(t, err)
	v, ok := env.runTool.gotArgs["_helix_sandbox_network_allowed"]
	require.True(t, ok, "_helix_sandbox_network_allowed must be injected for full-auto + Run-level")
	assert.Equal(t, false, v, "full-auto must DENY network egress (network_allowed=false)")
}

func TestApproval_Dangerous_NoSandboxInjection(t *testing.T) {
	env := newApprovalTestEnv(t, approval.ModeDangerous)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := env.registry.Execute(ctx, env.runTool.Name(), map[string]interface{}{"cmd": "echo"})

	require.NoError(t, err)
	require.NotNil(t, env.runTool.gotArgs)

	_, hasReq := env.runTool.gotArgs["_helix_sandbox_required"]
	_, hasNet := env.runTool.gotArgs["_helix_sandbox_network_allowed"]
	assert.False(t, hasReq, "dangerously-bypass must NOT inject _helix_sandbox_required")
	assert.False(t, hasNet, "dangerously-bypass must NOT inject _helix_sandbox_network_allowed")
}

func TestApproval_RuntimeChange_AffectsNextExecute(t *testing.T) {
	env := newApprovalTestEnv(t, approval.ModeSuggest)
	env.resp.Allow = true // not actually consulted in full-auto, but defensive

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// First call under Suggest: Run-level rejected.
	_, err := env.registry.Execute(ctx, env.runTool.Name(), map[string]interface{}{"cmd": "echo"})
	require.Error(t, err, "suggest must reject Run-level")
	assert.True(t, errors.Is(err, approval.ErrApprovalDenied))
	assert.Equal(t, int32(0), atomic.LoadInt32(&env.runTool.executed))

	// Runtime swap to full-auto.
	require.NoError(t, env.manager.SetMode(approval.ModeFullAuto))
	assert.Equal(t, approval.ModeFullAuto, env.manager.Mode())
	assert.Equal(t, approval.SourceRuntime, env.manager.Source())

	// Second call under FullAuto: same Run-level tool now proceeds.
	res, err := env.registry.Execute(ctx, env.runTool.Name(), map[string]interface{}{"cmd": "echo"})
	require.NoError(t, err, "full-auto must allow Run-level after SetMode")
	assert.Equal(t, "ok", res)
	assert.Equal(t, int32(1), atomic.LoadInt32(&env.runTool.executed))
	// Sandbox marker must now be injected.
	v, ok := env.runTool.gotArgs["_helix_sandbox_required"]
	require.True(t, ok)
	assert.Equal(t, true, v)
}
