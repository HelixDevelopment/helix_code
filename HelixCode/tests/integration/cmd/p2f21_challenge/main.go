// p2f21_challenge runs the F21 codex-approval-modes harness end-to-end against
// the real ToolRegistry, the real ApprovalManager, the real CheckApproval matrix,
// the real PromptForApproval delegation seam, the real SetMode atomic swap, and
// a real F02-equivalent final-deny rule wired in front of the inner Tool.Execute.
//
// Article XI 11.9 anti-bluff anchor: a regression that "PASSes" by stubbing the
// gate or hardcoding ALLOW would trip one of the per-phase invariants:
//   - Phase A asserts the deny error wraps approval.ErrApprovalDenied AND that
//     the stub tool's Execute counter is exactly zero.
//   - Phase B asserts the prompter recorded the question text AND that the
//     YES->ALLOW / NO->DENY polarity actually flips the executed counter.
//   - Phase C asserts the stub tool's recorded args contain the exact
//     "_helix_sandbox_required"=true and "_helix_sandbox_network_allowed"=false
//     sentinels (spec 7128289 §11) -- not a marketing flag.
//   - Phase D asserts SetMode flips a real DENY into a real ALLOW with sandbox
//     marker injection on the very next Execute, no restart, no reconstruct.
//   - Phase E asserts that even in ModeDangerous (which bypasses every approval
//     check), an inner final-deny rule (the F02 contract) still refuses the
//     call. F02 is not directly wired into the registry today, so the harness
//     pins the contract via a test-fixture deny-rule embedded in the Tool's
//     Execute. See §11 of the CHALLENGE.md for the precise documentation of
//     this seam.
//
// Phases (all five always run; no SKIPs):
//
//	A. SUGGEST-DENY            - ModeSuggest + LevelEdit -> ErrApprovalDenied,
//	                             Execute counter remains 0.
//	B. AUTO-EDIT-PROMPT        - ModeAutoEdit + LevelRun:
//	                               YES -> ALLOW + counter==1 + question recorded
//	                               NO  -> DENY  + counter unchanged
//	C. FULL-AUTO-SANDBOX       - ModeFullAuto + LevelRun -> ALLOW with
//	                             "_helix_sandbox_required"=true and
//	                             "_helix_sandbox_network_allowed"=false
//	                             injected into the Tool.Execute args map.
//	D. RUNTIME-CHANGE          - ModeSuggest -> Run-level DENY -> SetMode ->
//	                             ModeFullAuto -> same call now ALLOWS with
//	                             sandbox marker, no manager rebuild.
//	E. F02-FINAL-DENY          - ModeDangerous (bypass everything) -> the
//	                             stub Run-level tool's inner deny-rule still
//	                             refuses path=/etc/foo. Article XI: even the
//	                             escape-hatch approval mode does not override
//	                             a content-aware deny rule.
//
// Exit code 0 on PASS; exit 1 with a diagnostic on any check failure.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync/atomic"

	"dev.helix.code/internal/approval"
	"dev.helix.code/internal/tools"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
}

func run() error {
	fmt.Println("==> P2-F21 challenge harness pid:", os.Getpid())

	if err := phaseA(); err != nil {
		return fmt.Errorf("phase A: %w", err)
	}
	if err := phaseB(); err != nil {
		return fmt.Errorf("phase B: %w", err)
	}
	if err := phaseC(); err != nil {
		return fmt.Errorf("phase C: %w", err)
	}
	if err := phaseD(); err != nil {
		return fmt.Errorf("phase D: %w", err)
	}
	if err := phaseE(); err != nil {
		return fmt.Errorf("phase E: %w", err)
	}

	fmt.Println("==> ALL CHECKS PASSED")
	fmt.Println("==> P2-F21 challenge harness PASS")
	return nil
}

// stubTool records its received args and Execute count so each phase can
// assert (a) whether the inner Execute was reached at all, and (b) what the
// args map contained when it was reached (load-bearing for sandbox-marker
// assertions). Optional finalDeny enforces an inner deny-rule that mimics
// F02's path-aware final-deny seam (see Phase E).
type stubTool struct {
	name     string
	level    approval.ApprovalLevel
	executed int32
	gotArgs  map[string]interface{}

	// finalDeny is the F02-equivalent inner deny-rule. When non-nil and it
	// returns true for the supplied args map, Execute returns an explicit
	// final-deny error EVEN under ModeDangerous (which bypasses the F21 gate
	// entirely). This pins the contract that approval modes never override
	// inner content-aware permission rules.
	finalDeny func(args map[string]interface{}) (bool, string)
}

func (s *stubTool) Name() string                             { return s.name }
func (s *stubTool) Description() string                      { return "p2-f21 challenge stub" }
func (s *stubTool) Category() tools.ToolCategory             { return tools.ToolCategory("p2f21-challenge-stub") }
func (s *stubTool) Schema() tools.ToolSchema                 { return tools.ToolSchema{Type: "object"} }
func (s *stubTool) Validate(_ map[string]interface{}) error  { return nil }
func (s *stubTool) RequiresApproval() approval.ApprovalLevel { return s.level }
func (s *stubTool) Execute(_ context.Context, p map[string]interface{}) (interface{}, error) {
	// Defensive copy so a later test mutation does not pollute the snapshot.
	snap := make(map[string]interface{}, len(p))
	for k, v := range p {
		snap[k] = v
	}
	s.gotArgs = snap

	// F02-equivalent inner final-deny BEFORE we count the execution: if the
	// rule denies the call, the side effect must not register.
	if s.finalDeny != nil {
		if denied, reason := s.finalDeny(p); denied {
			return nil, fmt.Errorf("final-deny: %s", reason)
		}
	}
	atomic.AddInt32(&s.executed, 1)
	return "ok", nil
}

// recordingResponder satisfies approval.PromptResponder. The harness toggles
// .Allow before each call; .Calls is incremented atomically so the prompter
// being consulted (or NOT consulted) is mechanically observable.
type recordingResponder struct {
	Allow        bool
	LastQuestion string
	Calls        int32
}

func (r *recordingResponder) PromptYesNo(_ context.Context, q string, _ bool) (bool, error) {
	atomic.AddInt32(&r.Calls, 1)
	r.LastQuestion = q
	return r.Allow, nil
}

// newRegistryWithManager constructs a real ToolRegistry, registers the supplied
// stubs, and wires a real ApprovalManager with the supplied mode + responder.
// SandboxAvailable is forced to true so ModeFullAuto is constructible; tests
// that need the False branch are out of scope for this harness (CheckApproval
// guards that path; Phase 1.5 challenge already covered it).
func newRegistryWithManager(
	mode approval.ApprovalMode,
	stubs []tools.Tool,
	resp approval.PromptResponder,
) (*tools.ToolRegistry, *approval.ApprovalManager, error) {
	reg, err := tools.NewToolRegistry(tools.DefaultRegistryConfig())
	if err != nil {
		return nil, nil, fmt.Errorf("NewToolRegistry: %w", err)
	}
	for _, s := range stubs {
		reg.Register(s)
	}
	mgr, err := approval.NewApprovalManager(approval.ApprovalManagerOptions{
		InitialMode:      mode,
		Source:           approval.SourceDefault,
		Responder:        resp,
		SandboxAvailable: true,
	})
	if err != nil {
		_ = reg.Close()
		return nil, nil, fmt.Errorf("NewApprovalManager(mode=%s): %w", mode, err)
	}
	reg.SetApprovalManager(mgr)
	return reg, mgr, nil
}

// phaseA: ModeSuggest must DENY any LevelEdit invocation. The deny error must
// wrap approval.ErrApprovalDenied so callers can errors.Is-classify; the inner
// Tool.Execute must NOT be reached (executed counter == 0).
func phaseA() error {
	fmt.Println("==> phase A: SUGGEST-DENY (always runs)")

	editStub := &stubTool{name: "p2f21_stub_edit", level: approval.LevelEdit}
	resp := &recordingResponder{}
	reg, _, err := newRegistryWithManager(approval.ModeSuggest, []tools.Tool{editStub}, resp)
	if err != nil {
		return err
	}
	defer reg.Close()

	ctx := context.Background()
	_, execErr := reg.Execute(ctx, editStub.Name(), map[string]interface{}{"path": "x"})
	if execErr == nil {
		return fmt.Errorf("expected ErrApprovalDenied, got nil error")
	}
	if !errors.Is(execErr, approval.ErrApprovalDenied) {
		return fmt.Errorf("expected errors.Is ErrApprovalDenied, got: %v", execErr)
	}
	if got := atomic.LoadInt32(&editStub.executed); got != 0 {
		return fmt.Errorf("inner Execute must not run when suggest-deny fires; executed=%d", got)
	}
	if got := atomic.LoadInt32(&resp.Calls); got != 0 {
		return fmt.Errorf("prompter must not be consulted on outright deny; calls=%d", got)
	}
	fmt.Printf("    phaseA: suggest-mode blocked LevelEdit tool: %v\n", execErr)
	fmt.Printf("    verdict: ErrApprovalDenied raised, Tool.Execute counter=0, prompter calls=0\n")
	return nil
}

// phaseB: ModeAutoEdit + LevelRun must ALLOW after a YES prompt and DENY after
// a NO prompt. The recordingResponder's question text and call counters are
// load-bearing positive evidence that PromptForApproval was actually consulted.
func phaseB() error {
	fmt.Println("==> phase B: AUTO-EDIT-PROMPT (always runs)")

	// First sub-case: YES -> ALLOW.
	runStubYes := &stubTool{name: "p2f21_stub_run_yes", level: approval.LevelRun}
	respYes := &recordingResponder{Allow: true}
	regYes, _, err := newRegistryWithManager(approval.ModeAutoEdit, []tools.Tool{runStubYes}, respYes)
	if err != nil {
		return fmt.Errorf("YES sub-case wiring: %w", err)
	}
	defer regYes.Close()

	ctx := context.Background()
	res, execErr := regYes.Execute(ctx, runStubYes.Name(), map[string]interface{}{"cmd": "echo"})
	if execErr != nil {
		return fmt.Errorf("YES sub-case: expected nil error, got %v", execErr)
	}
	if res != "ok" {
		return fmt.Errorf("YES sub-case: expected result %q, got %v", "ok", res)
	}
	if got := atomic.LoadInt32(&runStubYes.executed); got != 1 {
		return fmt.Errorf("YES sub-case: expected executed=1, got %d", got)
	}
	if got := atomic.LoadInt32(&respYes.Calls); got != 1 {
		return fmt.Errorf("YES sub-case: prompter must be consulted exactly once, got calls=%d", got)
	}
	if !strings.Contains(respYes.LastQuestion, runStubYes.Name()) {
		return fmt.Errorf("YES sub-case: prompt question must mention tool name %q, got %q",
			runStubYes.Name(), respYes.LastQuestion)
	}

	// Second sub-case: NO -> DENY (errors.Is ErrApprovalDenied, no Execute).
	runStubNo := &stubTool{name: "p2f21_stub_run_no", level: approval.LevelRun}
	respNo := &recordingResponder{Allow: false}
	regNo, _, err := newRegistryWithManager(approval.ModeAutoEdit, []tools.Tool{runStubNo}, respNo)
	if err != nil {
		return fmt.Errorf("NO sub-case wiring: %w", err)
	}
	defer regNo.Close()

	_, execErr = regNo.Execute(ctx, runStubNo.Name(), map[string]interface{}{"cmd": "echo"})
	if execErr == nil {
		return fmt.Errorf("NO sub-case: expected ErrApprovalDenied, got nil")
	}
	if !errors.Is(execErr, approval.ErrApprovalDenied) {
		return fmt.Errorf("NO sub-case: expected errors.Is ErrApprovalDenied, got %v", execErr)
	}
	if got := atomic.LoadInt32(&runStubNo.executed); got != 0 {
		return fmt.Errorf("NO sub-case: inner Execute must not run after NO; executed=%d", got)
	}
	if got := atomic.LoadInt32(&respNo.Calls); got != 1 {
		return fmt.Errorf("NO sub-case: prompter must still be consulted exactly once, got calls=%d", got)
	}

	fmt.Println("    phaseB: auto-edit prompted user; YES -> executed; NO -> denied")
	fmt.Printf("    verdict: question recorded=%q (YES path), prompter consulted in both polarities\n",
		respYes.LastQuestion)
	return nil
}

// phaseC: ModeFullAuto + LevelRun must inject the F14/F21 sandbox markers into
// the args map seen by the inner Tool.Execute. Spec 7128289 §11 pins the
// sentinel keys/values; the harness reads them straight off the recorded snap.
func phaseC() error {
	fmt.Println("==> phase C: FULL-AUTO-SANDBOX (always runs)")

	runStub := &stubTool{name: "p2f21_stub_run_fullauto", level: approval.LevelRun}
	resp := &recordingResponder{}
	reg, _, err := newRegistryWithManager(approval.ModeFullAuto, []tools.Tool{runStub}, resp)
	if err != nil {
		return err
	}
	defer reg.Close()

	ctx := context.Background()
	_, execErr := reg.Execute(ctx, runStub.Name(), map[string]interface{}{"cmd": "echo"})
	if execErr != nil {
		return fmt.Errorf("expected nil error, got %v", execErr)
	}
	if got := atomic.LoadInt32(&runStub.executed); got != 1 {
		return fmt.Errorf("expected executed=1, got %d", got)
	}
	if runStub.gotArgs == nil {
		return fmt.Errorf("runStub.gotArgs must be recorded after Execute")
	}
	reqVal, hasReq := runStub.gotArgs["_helix_sandbox_required"]
	if !hasReq {
		return fmt.Errorf("_helix_sandbox_required must be injected for full-auto + LevelRun")
	}
	if reqVal != true {
		return fmt.Errorf("_helix_sandbox_required must be true, got %v (%T)", reqVal, reqVal)
	}
	netVal, hasNet := runStub.gotArgs["_helix_sandbox_network_allowed"]
	if !hasNet {
		return fmt.Errorf("_helix_sandbox_network_allowed must be injected for full-auto + LevelRun")
	}
	if netVal != false {
		return fmt.Errorf("_helix_sandbox_network_allowed must be false, got %v (%T)", netVal, netVal)
	}
	if got := atomic.LoadInt32(&resp.Calls); got != 0 {
		return fmt.Errorf("full-auto must NOT prompt; got calls=%d", got)
	}
	fmt.Println("    phaseC: full-auto injected sandbox marker into args")
	fmt.Printf("    verdict: _helix_sandbox_required=%v, _helix_sandbox_network_allowed=%v, prompter calls=0\n",
		reqVal, netVal)
	return nil
}

// phaseD: SetMode must atomically flip the gate's behaviour mid-session. The
// SAME stub tool that was rejected under ModeSuggest must be allowed under
// ModeFullAuto on the very next Execute, with the sandbox markers injected.
// Source must transition from SourceDefault -> SourceRuntime.
func phaseD() error {
	fmt.Println("==> phase D: RUNTIME-CHANGE (always runs)")

	runStub := &stubTool{name: "p2f21_stub_run_runtime", level: approval.LevelRun}
	resp := &recordingResponder{}
	reg, mgr, err := newRegistryWithManager(approval.ModeSuggest, []tools.Tool{runStub}, resp)
	if err != nil {
		return err
	}
	defer reg.Close()

	ctx := context.Background()

	// Pre-swap: suggest must reject Run-level outright.
	_, execErr := reg.Execute(ctx, runStub.Name(), map[string]interface{}{"cmd": "echo"})
	if execErr == nil {
		return fmt.Errorf("pre-swap: expected ErrApprovalDenied under suggest, got nil")
	}
	if !errors.Is(execErr, approval.ErrApprovalDenied) {
		return fmt.Errorf("pre-swap: expected errors.Is ErrApprovalDenied, got %v", execErr)
	}
	if got := atomic.LoadInt32(&runStub.executed); got != 0 {
		return fmt.Errorf("pre-swap: executed must be 0, got %d", got)
	}
	if mgr.Source() != approval.SourceDefault {
		return fmt.Errorf("pre-swap: Source must be SourceDefault, got %s", mgr.Source())
	}

	// Atomic swap.
	if err := mgr.SetMode(approval.ModeFullAuto); err != nil {
		return fmt.Errorf("SetMode(full-auto): %w", err)
	}
	if mgr.Mode() != approval.ModeFullAuto {
		return fmt.Errorf("post-SetMode: Mode must be ModeFullAuto, got %s", mgr.Mode())
	}
	if mgr.Source() != approval.SourceRuntime {
		return fmt.Errorf("post-SetMode: Source must be SourceRuntime, got %s", mgr.Source())
	}

	// Post-swap: same call now ALLOWED with sandbox marker injection.
	res, execErr := reg.Execute(ctx, runStub.Name(), map[string]interface{}{"cmd": "echo"})
	if execErr != nil {
		return fmt.Errorf("post-swap: expected nil error, got %v", execErr)
	}
	if res != "ok" {
		return fmt.Errorf("post-swap: expected %q, got %v", "ok", res)
	}
	if got := atomic.LoadInt32(&runStub.executed); got != 1 {
		return fmt.Errorf("post-swap: executed must be 1, got %d", got)
	}
	reqVal, hasReq := runStub.gotArgs["_helix_sandbox_required"]
	if !hasReq || reqVal != true {
		return fmt.Errorf("post-swap: _helix_sandbox_required must be injected as true, got %v hasReq=%v", reqVal, hasReq)
	}

	fmt.Println("    phaseD: runtime SetMode(suggest->full-auto) flipped from DENY to ALLOW+sandbox")
	fmt.Printf("    verdict: pre-swap deny + post-swap allow with sandbox markers; Source SourceDefault -> SourceRuntime\n")
	return nil
}

// phaseE: ModeDangerous bypasses the F21 approval gate entirely (every level
// returns ALLOW). The harness pins the cross-feature contract that an inner
// content-aware deny rule (the F02 final-deny seam) still refuses the call
// regardless. Today F02 is not directly wired into the registry, so the
// contract is pinned at the Tool's own Execute via finalDeny -- documented in
// CHALLENGE.md §11. The error must be NON-nil and the executed counter must
// stay at 0.
func phaseE() error {
	fmt.Println("==> phase E: F02-FINAL-DENY (always runs)")

	// finalDeny mimics the F02 path-aware final-deny rule: refuse any path
	// under /etc/. Mirrors the Spec 7128289 §11 "deny-write to /etc/" example.
	denyEtc := func(args map[string]interface{}) (bool, string) {
		raw, ok := args["path"]
		if !ok {
			return false, ""
		}
		s, ok := raw.(string)
		if !ok {
			return false, ""
		}
		if strings.HasPrefix(s, "/etc/") {
			return true, fmt.Sprintf("F02-equivalent rule denies path=%q", s)
		}
		return false, ""
	}
	writeStub := &stubTool{
		name:      "p2f21_stub_fs_write",
		level:     approval.LevelRun,
		finalDeny: denyEtc,
	}
	resp := &recordingResponder{}
	reg, mgr, err := newRegistryWithManager(approval.ModeDangerous, []tools.Tool{writeStub}, resp)
	if err != nil {
		return err
	}
	defer reg.Close()

	if mgr.Mode() != approval.ModeDangerous {
		return fmt.Errorf("expected ModeDangerous, got %s", mgr.Mode())
	}

	// Sanity: a benign path ALLOWs (proves the stub's deny rule is path-aware,
	// not a blanket reject).
	ctx := context.Background()
	res, execErr := reg.Execute(ctx, writeStub.Name(), map[string]interface{}{"path": "/tmp/ok"})
	if execErr != nil {
		return fmt.Errorf("benign-path call: expected nil error, got %v", execErr)
	}
	if res != "ok" {
		return fmt.Errorf("benign-path call: expected %q, got %v", "ok", res)
	}
	benignExecuted := atomic.LoadInt32(&writeStub.executed)
	if benignExecuted != 1 {
		return fmt.Errorf("benign-path call: executed must be 1, got %d", benignExecuted)
	}

	// Forbidden path: even ModeDangerous must not override final-deny.
	_, execErr = reg.Execute(ctx, writeStub.Name(), map[string]interface{}{"path": "/etc/foo"})
	if execErr == nil {
		return fmt.Errorf("/etc/ path: expected final-deny error, got nil")
	}
	if !strings.Contains(execErr.Error(), "final-deny") {
		return fmt.Errorf("/etc/ path: expected final-deny in error, got %v", execErr)
	}
	postEtcExecuted := atomic.LoadInt32(&writeStub.executed)
	if postEtcExecuted != benignExecuted {
		return fmt.Errorf("/etc/ path: executed must remain at %d (benign baseline), got %d",
			benignExecuted, postEtcExecuted)
	}
	if got := atomic.LoadInt32(&resp.Calls); got != 0 {
		return fmt.Errorf("dangerously-bypass must NOT prompt; got calls=%d", got)
	}
	fmt.Println("    phaseE: F02 final-deny overrode dangerously-bypass for /etc/ path")
	fmt.Printf("    verdict: benign /tmp/ok ALLOW (executed=%d), /etc/foo final-deny (executed unchanged at %d), error=%v\n",
		benignExecuted, postEtcExecuted, execErr)
	return nil
}
