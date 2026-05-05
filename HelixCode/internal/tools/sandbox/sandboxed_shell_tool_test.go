package sandbox

// Tests for SandboxedShellTool (P1-F14-T07).
//
// Strategy: SandboxedShellTool depends on the SandboxedShellExecutor seam
// (a subset of *SandboxManager). We exercise the tool against a fakeExecutor
// implementing that seam — this is a hexagonal port, NOT a mock substituting
// for the real Execute behaviour of SandboxManager. SandboxManager itself is
// covered by manager_test.go with real CONST-033 deny + real spy backend.
//
// Tests verify:
//   - Tool interface contract (Name / Description / Category / Schema /
//     Validate / Execute) — matches dev.helix.code/internal/tools.Tool.
//   - Args → SandboxPolicy mapping (network / timeout_seconds / memory_limit_mb).
//   - Validate rejects bad types and out-of-range timeouts.
//   - DenyError + FailClosedError propagation surfaces a clear human message
//     containing the matched rule / reason so the agent can show it back.

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"dev.helix.code/internal/tools"
)

// ---------- fake executor (hexagonal seam) ----------

// fakeExecutor records the policy + command passed by the tool and returns
// caller-controlled (result, err). It exposes Capabilities() so the tool can
// surface backend info if/when it needs to.
type fakeExecutor struct {
	mu sync.Mutex

	// last invocation
	gotCommand string
	gotPolicy  SandboxPolicy
	calls      int

	// programmable response
	result *SandboxResult
	err    error

	caps SandboxCapabilities
}

func (f *fakeExecutor) Execute(ctx context.Context, command string, policy SandboxPolicy) (*SandboxResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.gotCommand = command
	f.gotPolicy = policy
	return f.result, f.err
}

func (f *fakeExecutor) Capabilities() SandboxCapabilities { return f.caps }

func (f *fakeExecutor) snapshot() (string, SandboxPolicy, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.gotCommand, f.gotPolicy, f.calls
}

// newFakeOK returns a fake that always succeeds with a canned result.
func newFakeOK() *fakeExecutor {
	return &fakeExecutor{
		result: &SandboxResult{
			Stdout:   "hi\n",
			Stderr:   "",
			ExitCode: 0,
			Backend:  BackendBubblewrap,
			Duration: 5 * time.Millisecond,
		},
		caps: SandboxCapabilities{SelectedBackend: BackendBubblewrap},
	}
}

// ---------- shape: Name / Description / Category / Schema ----------

func TestSandboxedShellTool_Name(t *testing.T) {
	tool := NewSandboxedShellTool(newFakeOK())
	if got, want := tool.Name(), "shell_sandboxed"; got != want {
		t.Errorf("Name(): got %q want %q", got, want)
	}
}

func TestSandboxedShellTool_DescriptionNonEmpty(t *testing.T) {
	tool := NewSandboxedShellTool(newFakeOK())
	if strings.TrimSpace(tool.Description()) == "" {
		t.Errorf("Description(): empty")
	}
}

func TestSandboxedShellTool_Category(t *testing.T) {
	tool := NewSandboxedShellTool(newFakeOK())
	if got, want := tool.Category(), tools.CategorySandbox; got != want {
		t.Errorf("Category(): got %q want %q", got, want)
	}
}

func TestSandboxedShellTool_Schema_HasRequiredFields(t *testing.T) {
	tool := NewSandboxedShellTool(newFakeOK())
	schema := tool.Schema()

	if schema.Type != "object" {
		t.Errorf("Schema().Type: got %q want %q", schema.Type, "object")
	}
	for _, key := range []string{"command", "network", "timeout_seconds", "memory_limit_mb"} {
		if _, ok := schema.Properties[key]; !ok {
			t.Errorf("Schema(): missing property %q", key)
		}
	}
	// command must be Required.
	foundCmd := false
	for _, r := range schema.Required {
		if r == "command" {
			foundCmd = true
		}
	}
	if !foundCmd {
		t.Errorf("Schema().Required: missing %q (got %v)", "command", schema.Required)
	}
}

// ---------- Validate ----------

func TestSandboxedShellTool_Validate_RequiresCommand(t *testing.T) {
	tool := NewSandboxedShellTool(newFakeOK())

	if err := tool.Validate(map[string]interface{}{}); err == nil {
		t.Errorf("Validate(empty): expected error for missing command, got nil")
	}

	// empty string is also rejected.
	if err := tool.Validate(map[string]interface{}{"command": ""}); err == nil {
		t.Errorf("Validate(command=\"\"): expected error, got nil")
	}

	// whitespace-only is also rejected.
	if err := tool.Validate(map[string]interface{}{"command": "   "}); err == nil {
		t.Errorf("Validate(command=spaces): expected error, got nil")
	}
}

func TestSandboxedShellTool_Validate_RejectsCommandWrongType(t *testing.T) {
	tool := NewSandboxedShellTool(newFakeOK())
	if err := tool.Validate(map[string]interface{}{"command": 42}); err == nil {
		t.Errorf("Validate(command=int): expected error, got nil")
	}
}

func TestSandboxedShellTool_Validate_RejectsNetworkWrongType(t *testing.T) {
	tool := NewSandboxedShellTool(newFakeOK())
	err := tool.Validate(map[string]interface{}{
		"command": "echo hi",
		"network": "yes", // string not bool
	})
	if err == nil {
		t.Errorf("Validate(network=string): expected error, got nil")
	}
}

func TestSandboxedShellTool_Validate_RejectsTimeoutOutOfRange(t *testing.T) {
	tool := NewSandboxedShellTool(newFakeOK())

	// 0 is invalid (must be >= 1).
	if err := tool.Validate(map[string]interface{}{
		"command":         "echo hi",
		"timeout_seconds": 0,
	}); err == nil {
		t.Errorf("Validate(timeout=0): expected error, got nil")
	}

	// >600 is invalid.
	if err := tool.Validate(map[string]interface{}{
		"command":         "echo hi",
		"timeout_seconds": 601,
	}); err == nil {
		t.Errorf("Validate(timeout=601): expected error, got nil")
	}

	// negative is invalid.
	if err := tool.Validate(map[string]interface{}{
		"command":         "echo hi",
		"timeout_seconds": -1,
	}); err == nil {
		t.Errorf("Validate(timeout=-1): expected error, got nil")
	}

	// non-int is invalid.
	if err := tool.Validate(map[string]interface{}{
		"command":         "echo hi",
		"timeout_seconds": "30",
	}); err == nil {
		t.Errorf("Validate(timeout=string): expected error, got nil")
	}
}

func TestSandboxedShellTool_Validate_AcceptsMissingOptionalFields(t *testing.T) {
	tool := NewSandboxedShellTool(newFakeOK())
	if err := tool.Validate(map[string]interface{}{
		"command": "echo hi",
	}); err != nil {
		t.Errorf("Validate(only command): unexpected error: %v", err)
	}
}

func TestSandboxedShellTool_Validate_AcceptsAllValidFields(t *testing.T) {
	tool := NewSandboxedShellTool(newFakeOK())
	if err := tool.Validate(map[string]interface{}{
		"command":         "echo hi",
		"network":         true,
		"timeout_seconds": 60,
		"memory_limit_mb": 256,
	}); err != nil {
		t.Errorf("Validate(all valid): unexpected error: %v", err)
	}
}

func TestSandboxedShellTool_Validate_RejectsNegativeMemory(t *testing.T) {
	tool := NewSandboxedShellTool(newFakeOK())
	if err := tool.Validate(map[string]interface{}{
		"command":         "echo hi",
		"memory_limit_mb": -1,
	}); err == nil {
		t.Errorf("Validate(memory=-1): expected error, got nil")
	}
}

// ---------- Execute: policy mapping ----------

func TestSandboxedShellTool_Execute_DefaultPolicy_DeniesNetwork(t *testing.T) {
	fake := newFakeOK()
	tool := NewSandboxedShellTool(fake)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"command": "echo hi",
	})
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}

	cmd, pol, calls := fake.snapshot()
	if calls != 1 {
		t.Fatalf("expected 1 executor call, got %d", calls)
	}
	if cmd != "echo hi" {
		t.Errorf("command passthrough: got %q want %q", cmd, "echo hi")
	}
	if pol.NetworkAllowed {
		t.Errorf("default policy must DENY network, got NetworkAllowed=true")
	}
}

func TestSandboxedShellTool_Execute_NetworkAllowedFromArg(t *testing.T) {
	fake := newFakeOK()
	tool := NewSandboxedShellTool(fake)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"command": "curl example.com",
		"network": true,
	})
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}
	_, pol, _ := fake.snapshot()
	if !pol.NetworkAllowed {
		t.Errorf("policy.NetworkAllowed: got false want true")
	}
}

func TestSandboxedShellTool_Execute_TimeoutFromArg(t *testing.T) {
	fake := newFakeOK()
	tool := NewSandboxedShellTool(fake)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"command":         "sleep 5",
		"timeout_seconds": 60,
	})
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}
	_, pol, _ := fake.snapshot()
	if pol.Timeout != 60*time.Second {
		t.Errorf("policy.Timeout: got %v want %v", pol.Timeout, 60*time.Second)
	}
}

func TestSandboxedShellTool_Execute_MemoryLimitFromArg(t *testing.T) {
	fake := newFakeOK()
	tool := NewSandboxedShellTool(fake)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"command":         "echo hi",
		"memory_limit_mb": 256,
	})
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}
	_, pol, _ := fake.snapshot()
	if pol.MemoryLimitMB != 256 {
		t.Errorf("policy.MemoryLimitMB: got %d want %d", pol.MemoryLimitMB, 256)
	}
}

func TestSandboxedShellTool_Execute_DefaultTimeoutAppliedWhenAbsent(t *testing.T) {
	fake := newFakeOK()
	tool := NewSandboxedShellTool(fake)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"command": "echo hi",
	})
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}
	_, pol, _ := fake.snapshot()
	// Spec contract: default timeout 30s when caller doesn't specify.
	if pol.Timeout != 30*time.Second {
		t.Errorf("default policy.Timeout: got %v want %v", pol.Timeout, 30*time.Second)
	}
}

// ---------- Execute: error propagation ----------

func TestSandboxedShellTool_Execute_PropagatesDenyError(t *testing.T) {
	fake := &fakeExecutor{
		err: &DenyError{
			Command:     "systemctl suspend",
			MatchedRule: "CONST-033: systemctl power-management subcommand",
			Pattern:     `systemctl\s+suspend`,
		},
	}
	tool := NewSandboxedShellTool(fake)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"command": "systemctl suspend",
	})
	if err == nil {
		t.Fatalf("Execute: expected DenyError, got nil")
	}
	msg := err.Error()
	if !strings.Contains(strings.ToLower(msg), "denied") {
		t.Errorf("error message must contain 'denied', got %q", msg)
	}
	if !strings.Contains(msg, "CONST-033") {
		t.Errorf("error message must surface the matched rule, got %q", msg)
	}
	// errors.Is must still trace to the sentinel so callers can branch.
	if !errors.Is(err, ErrCommandDenied) {
		t.Errorf("errors.Is(err, ErrCommandDenied): false; expected true")
	}
}

func TestSandboxedShellTool_Execute_PropagatesFailClosedError(t *testing.T) {
	reason := "bubblewrap not found and unprivileged user namespaces disabled (sysctl kernel.unprivileged_userns_clone=0); install bubblewrap or enable userns"
	fake := &fakeExecutor{
		err: &FailClosedError{Reason: reason},
	}
	tool := NewSandboxedShellTool(fake)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"command": "echo hi",
	})
	if err == nil {
		t.Fatalf("Execute: expected FailClosedError, got nil")
	}
	if !strings.Contains(err.Error(), "bubblewrap") {
		t.Errorf("error message must surface the verbatim reason, got %q", err.Error())
	}
	if !errors.Is(err, ErrSandboxUnavailable) {
		t.Errorf("errors.Is(err, ErrSandboxUnavailable): false; expected true")
	}
}

func TestSandboxedShellTool_Execute_ReturnsResultOnSuccess(t *testing.T) {
	canned := &SandboxResult{
		Stdout:   "hello\n",
		Stderr:   "",
		ExitCode: 0,
		Backend:  BackendBubblewrap,
		Duration: 12 * time.Millisecond,
	}
	fake := &fakeExecutor{result: canned}
	tool := NewSandboxedShellTool(fake)

	got, err := tool.Execute(context.Background(), map[string]interface{}{
		"command": "echo hello",
	})
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}
	res, ok := got.(*SandboxResult)
	if !ok {
		t.Fatalf("Execute: result type: got %T want *SandboxResult", got)
	}
	if res.Stdout != "hello\n" {
		t.Errorf("Stdout: got %q want %q", res.Stdout, "hello\n")
	}
	if res.ExitCode != 0 {
		t.Errorf("ExitCode: got %d want 0", res.ExitCode)
	}
	if res.Backend != BackendBubblewrap {
		t.Errorf("Backend: got %q want %q", res.Backend, BackendBubblewrap)
	}
}

func TestSandboxedShellTool_Execute_ValidatesBeforeExecutor(t *testing.T) {
	// If validate fails, the executor must NOT be called — i.e. no surprise
	// dispatch on bad input. We force a validation failure by sending no command.
	fake := newFakeOK()
	tool := NewSandboxedShellTool(fake)

	// Tool.Execute itself calls Validate first per the registry contract;
	// confirm the fake stayed at zero calls when args are bad.
	_, err := tool.Execute(context.Background(), map[string]interface{}{})
	if err == nil {
		t.Fatalf("Execute(no command): expected error, got nil")
	}
	_, _, calls := fake.snapshot()
	if calls != 0 {
		t.Errorf("executor called %d times despite invalid args; want 0", calls)
	}
}
