// manager_test.go (P2-F21-T04): exhaustive coverage of the 4x4 matrix gate,
// runtime mode swap, sandbox/network propagation, dangerous-mode startup
// pause, and prompter wiring.
//
// All tests are unit-scope and use a fakePromptResponder; no real stdin.
//
// References:
//   - Spec 7128289 §3 (Components/Types) and §4 (matrix)
//   - Plan bbb61de T04
//   - F19 prompter at internal/tools/askuser

package approval

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// fakePromptResponder — records calls + returns canned answers / errors.
// ---------------------------------------------------------------------------

type fakePromptResponder struct {
	calls   atomic.Int32
	answer  bool
	err     error
	lastQ   atomic.Pointer[string]
	lastDef atomic.Bool
}

func (f *fakePromptResponder) PromptYesNo(ctx context.Context, question string, defaultYes bool) (bool, error) {
	f.calls.Add(1)
	q := question
	f.lastQ.Store(&q)
	f.lastDef.Store(defaultYes)
	if f.err != nil {
		return false, f.err
	}
	return f.answer, nil
}

func newFakeResponder(answer bool) *fakePromptResponder {
	r := &fakePromptResponder{answer: answer}
	return r
}

// recordingSleep returns a sleep function that records every Duration it
// receives, so the dangerous-mode startup pause can be asserted without
// actually blocking the test.
type recordingSleep struct {
	calls atomic.Int32
	last  atomic.Int64 // nanoseconds
}

func (r *recordingSleep) Sleep(d time.Duration) {
	r.calls.Add(1)
	r.last.Store(int64(d))
}

// ---------------------------------------------------------------------------
// Constructor tests
// ---------------------------------------------------------------------------

func TestNewApprovalManager_ConstructsAllModes(t *testing.T) {
	for _, m := range AllModes() {
		m := m
		t.Run(m.String(), func(t *testing.T) {
			rs := &recordingSleep{}
			mgr, err := NewApprovalManager(ApprovalManagerOptions{
				InitialMode:      m,
				Source:           SourceFlag,
				SandboxAvailable: true,
				PauseDangerous:   0,
				SleepFunc:        rs.Sleep,
			})
			if err != nil {
				t.Fatalf("NewApprovalManager(%q) error: %v", m, err)
			}
			if got := mgr.Mode(); got != m {
				t.Errorf("Mode() = %q, want %q", got, m)
			}
			if got := mgr.Source(); got != SourceFlag {
				t.Errorf("Source() = %q, want %q", got, SourceFlag)
			}
		})
	}
}

func TestNewApprovalManager_FullAutoRequiresSandbox(t *testing.T) {
	_, err := NewApprovalManager(ApprovalManagerOptions{
		InitialMode:      ModeFullAuto,
		Source:           SourceFlag,
		SandboxAvailable: false,
	})
	if err == nil {
		t.Fatal("NewApprovalManager(full-auto, !sandboxOK) returned nil error; want error")
	}
	if !errors.Is(err, ErrSandboxRequired) {
		t.Errorf("error chain missing ErrSandboxRequired: %v", err)
	}
}

func TestNewApprovalManager_DangerousStartupPause(t *testing.T) {
	rs := &recordingSleep{}
	mgr, err := NewApprovalManager(ApprovalManagerOptions{
		InitialMode:      ModeDangerous,
		Source:           SourceFlag,
		SandboxAvailable: true,
		PauseDangerous:   100 * time.Millisecond,
		SleepFunc:        rs.Sleep,
	})
	if err != nil {
		t.Fatalf("NewApprovalManager error: %v", err)
	}
	if rs.calls.Load() != 1 {
		t.Fatalf("recordingSleep calls = %d, want 1", rs.calls.Load())
	}
	if got := time.Duration(rs.last.Load()); got != 100*time.Millisecond {
		t.Errorf("recorded sleep = %v, want 100ms", got)
	}
	if mgr.Mode() != ModeDangerous {
		t.Errorf("Mode() = %q, want dangerously-bypass", mgr.Mode())
	}
}

func TestNewApprovalManager_PauseZeroSkipsSleep(t *testing.T) {
	rs := &recordingSleep{}
	_, err := NewApprovalManager(ApprovalManagerOptions{
		InitialMode:      ModeDangerous,
		Source:           SourceFlag,
		SandboxAvailable: true,
		PauseDangerous:   0,
		SleepFunc:        rs.Sleep,
	})
	if err != nil {
		t.Fatalf("NewApprovalManager error: %v", err)
	}
	if rs.calls.Load() != 0 {
		t.Errorf("recordingSleep called %d times with PauseDangerous=0; want 0", rs.calls.Load())
	}
}

func TestNewApprovalManager_NonDangerousNoSleep(t *testing.T) {
	rs := &recordingSleep{}
	_, err := NewApprovalManager(ApprovalManagerOptions{
		InitialMode:      ModeAutoEdit,
		Source:           SourceFlag,
		SandboxAvailable: true,
		PauseDangerous:   1 * time.Hour, // non-zero, should still NOT fire for non-dangerous
		SleepFunc:        rs.Sleep,
	})
	if err != nil {
		t.Fatalf("NewApprovalManager error: %v", err)
	}
	if rs.calls.Load() != 0 {
		t.Errorf("recordingSleep called %d times for ModeAutoEdit; want 0", rs.calls.Load())
	}
}

func TestNewApprovalManager_RejectsInvalidMode(t *testing.T) {
	_, err := NewApprovalManager(ApprovalManagerOptions{
		InitialMode:      ApprovalMode("nonsense"),
		Source:           SourceFlag,
		SandboxAvailable: true,
	})
	if err == nil {
		t.Fatal("expected error for invalid initial mode; got nil")
	}
	if !errors.Is(err, ErrInvalidMode) {
		t.Errorf("error chain missing ErrInvalidMode: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Mode / Source / SetMode
// ---------------------------------------------------------------------------

func TestApprovalManager_Mode_AtomicRead(t *testing.T) {
	mgr := mustManager(t, ModeAutoEdit, SourceConfig, true)
	if got := mgr.Mode(); got != ModeAutoEdit {
		t.Errorf("Mode() = %q, want auto-edit", got)
	}
}

func TestApprovalManager_Source_AtomicRead(t *testing.T) {
	mgr := mustManager(t, ModeSuggest, SourceConfig, true)
	if got := mgr.Source(); got != SourceConfig {
		t.Errorf("Source() = %q, want config", got)
	}
}

func TestApprovalManager_SetMode_AllowsRuntimeChange(t *testing.T) {
	mgr := mustManager(t, ModeSuggest, SourceFlag, true)
	if err := mgr.SetMode(ModeAutoEdit); err != nil {
		t.Fatalf("SetMode(auto-edit) error: %v", err)
	}
	if got := mgr.Mode(); got != ModeAutoEdit {
		t.Errorf("Mode() = %q, want auto-edit", got)
	}
	if got := mgr.Source(); got != SourceRuntime {
		t.Errorf("Source() after SetMode = %q, want runtime", got)
	}
}

func TestApprovalManager_SetMode_RejectsFullAutoNoSandbox(t *testing.T) {
	mgr := mustManager(t, ModeSuggest, SourceFlag, false /* no sandbox */)
	err := mgr.SetMode(ModeFullAuto)
	if err == nil {
		t.Fatal("SetMode(full-auto, !sandboxOK) returned nil; want error")
	}
	if !errors.Is(err, ErrSandboxRequired) {
		t.Errorf("error chain missing ErrSandboxRequired: %v", err)
	}
	if got := mgr.Mode(); got != ModeSuggest {
		t.Errorf("Mode() after rejected SetMode = %q, want suggest (unchanged)", got)
	}
	if got := mgr.Source(); got != SourceFlag {
		t.Errorf("Source() after rejected SetMode = %q, want flag (unchanged)", got)
	}
}

func TestApprovalManager_SetMode_RejectsInvalidMode(t *testing.T) {
	mgr := mustManager(t, ModeSuggest, SourceFlag, true)
	err := mgr.SetMode(ApprovalMode("garbage"))
	if err == nil {
		t.Fatal("SetMode(garbage) returned nil; want error")
	}
	if !errors.Is(err, ErrInvalidMode) {
		t.Errorf("error chain missing ErrInvalidMode: %v", err)
	}
	if got := mgr.Mode(); got != ModeSuggest {
		t.Errorf("Mode() after rejected SetMode = %q, want suggest", got)
	}
}

// ---------------------------------------------------------------------------
// CheckApproval — 4 modes x 4 levels matrix (16 cells)
// ---------------------------------------------------------------------------

func TestCheckApproval_Suggest_ReadOnlyAllowed(t *testing.T) {
	mgr := mustManager(t, ModeSuggest, SourceFlag, true)
	got, err := mgr.CheckApproval(ApprovalRequest{ToolName: "fs_query", Level: LevelReadOnly})
	if err != nil {
		t.Fatalf("CheckApproval error: %v", err)
	}
	if got != ActionAllow {
		t.Errorf("Suggest+ReadOnly = %q, want allow", got)
	}
}

func TestCheckApproval_Suggest_EditDenied(t *testing.T) {
	mgr := mustManager(t, ModeSuggest, SourceFlag, true)
	got, err := mgr.CheckApproval(ApprovalRequest{ToolName: "fs_edit", Level: LevelEdit})
	if got != ActionDenyWithReason {
		t.Errorf("Suggest+Edit = %q, want deny", got)
	}
	if err == nil || !errors.Is(err, ErrApprovalDenied) {
		t.Errorf("Suggest+Edit error = %v, want chain containing ErrApprovalDenied", err)
	}
}

func TestCheckApproval_Suggest_RunDenied(t *testing.T) {
	mgr := mustManager(t, ModeSuggest, SourceFlag, true)
	got, err := mgr.CheckApproval(ApprovalRequest{ToolName: "shell", Level: LevelRun})
	if got != ActionDenyWithReason {
		t.Errorf("Suggest+Run = %q, want deny", got)
	}
	if err == nil || !errors.Is(err, ErrApprovalDenied) {
		t.Errorf("Suggest+Run error = %v, want chain containing ErrApprovalDenied", err)
	}
}

func TestCheckApproval_Suggest_AllDenied(t *testing.T) {
	mgr := mustManager(t, ModeSuggest, SourceFlag, true)
	got, err := mgr.CheckApproval(ApprovalRequest{ToolName: "complex", Level: LevelAll})
	if got != ActionDenyWithReason {
		t.Errorf("Suggest+All = %q, want deny", got)
	}
	if err == nil || !errors.Is(err, ErrApprovalDenied) {
		t.Errorf("Suggest+All error = %v, want chain containing ErrApprovalDenied", err)
	}
}

func TestCheckApproval_AutoEdit_ReadOnlyAllowed(t *testing.T) {
	mgr := mustManager(t, ModeAutoEdit, SourceFlag, true)
	got, err := mgr.CheckApproval(ApprovalRequest{ToolName: "fs_query", Level: LevelReadOnly})
	if err != nil {
		t.Fatalf("CheckApproval error: %v", err)
	}
	if got != ActionAllow {
		t.Errorf("AutoEdit+ReadOnly = %q, want allow", got)
	}
}

func TestCheckApproval_AutoEdit_EditAllowed(t *testing.T) {
	mgr := mustManager(t, ModeAutoEdit, SourceFlag, true)
	got, err := mgr.CheckApproval(ApprovalRequest{ToolName: "fs_edit", Level: LevelEdit})
	if err != nil {
		t.Fatalf("CheckApproval error: %v", err)
	}
	if got != ActionAllow {
		t.Errorf("AutoEdit+Edit = %q, want allow", got)
	}
}

func TestCheckApproval_AutoEdit_RunPrompts(t *testing.T) {
	mgr := mustManager(t, ModeAutoEdit, SourceFlag, true)
	got, err := mgr.CheckApproval(ApprovalRequest{ToolName: "shell", Level: LevelRun})
	if err != nil {
		t.Fatalf("CheckApproval error: %v", err)
	}
	if got != ActionPromptUser {
		t.Errorf("AutoEdit+Run = %q, want prompt-user", got)
	}
}

func TestCheckApproval_AutoEdit_AllPrompts(t *testing.T) {
	mgr := mustManager(t, ModeAutoEdit, SourceFlag, true)
	got, err := mgr.CheckApproval(ApprovalRequest{ToolName: "complex", Level: LevelAll})
	if err != nil {
		t.Fatalf("CheckApproval error: %v", err)
	}
	if got != ActionPromptUser {
		t.Errorf("AutoEdit+All = %q, want prompt-user", got)
	}
}

func TestCheckApproval_FullAuto_AllAllowed(t *testing.T) {
	mgr := mustManager(t, ModeFullAuto, SourceFlag, true)
	for _, lvl := range []ApprovalLevel{LevelReadOnly, LevelEdit, LevelRun, LevelAll} {
		got, err := mgr.CheckApproval(ApprovalRequest{ToolName: "any", Level: lvl})
		if err != nil {
			t.Fatalf("CheckApproval(%v) error: %v", lvl, err)
		}
		if got != ActionAllow {
			t.Errorf("FullAuto+%v = %q, want allow", lvl, got)
		}
	}
}

func TestCheckApproval_Dangerous_AllAllowed(t *testing.T) {
	mgr := mustManager(t, ModeDangerous, SourceFlag, true)
	for _, lvl := range []ApprovalLevel{LevelReadOnly, LevelEdit, LevelRun, LevelAll} {
		got, err := mgr.CheckApproval(ApprovalRequest{ToolName: "any", Level: lvl})
		if err != nil {
			t.Fatalf("CheckApproval(%v) error: %v", lvl, err)
		}
		if got != ActionAllow {
			t.Errorf("Dangerous+%v = %q, want allow", lvl, got)
		}
	}
}

func TestCheckApproval_RejectsInvalidLevel(t *testing.T) {
	mgr := mustManager(t, ModeAutoEdit, SourceFlag, true)
	bad := ApprovalLevel(99)
	got, err := mgr.CheckApproval(ApprovalRequest{ToolName: "weird", Level: bad})
	if got != ActionDenyWithReason {
		t.Errorf("invalid level action = %q, want deny", got)
	}
	if err == nil || !errors.Is(err, ErrInvalidLevel) {
		t.Errorf("invalid level error chain missing ErrInvalidLevel: %v", err)
	}
}

// ---------------------------------------------------------------------------
// PromptForApproval
// ---------------------------------------------------------------------------

func TestPromptForApproval_AutoEditRunPrompts(t *testing.T) {
	resp := newFakeResponder(true)
	mgr := mustManagerWithResponder(t, ModeAutoEdit, SourceFlag, true, resp)
	allowed, err := mgr.PromptForApproval(context.Background(),
		ApprovalRequest{ToolName: "shell_sandboxed", Level: LevelRun, Args: map[string]any{"cmd": "ls"}})
	if err != nil {
		t.Fatalf("PromptForApproval error: %v", err)
	}
	if !allowed {
		t.Fatal("expected allowed=true (responder said yes)")
	}
	if resp.calls.Load() != 1 {
		t.Errorf("responder calls = %d, want 1", resp.calls.Load())
	}
	if got := resp.lastQ.Load(); got == nil || *got == "" {
		t.Error("responder received empty question text")
	}
}

func TestPromptForApproval_UserDenies(t *testing.T) {
	resp := newFakeResponder(false)
	mgr := mustManagerWithResponder(t, ModeAutoEdit, SourceFlag, true, resp)
	allowed, err := mgr.PromptForApproval(context.Background(),
		ApprovalRequest{ToolName: "shell_sandboxed", Level: LevelRun})
	if err != nil {
		t.Fatalf("PromptForApproval error: %v", err)
	}
	if allowed {
		t.Fatal("expected allowed=false (responder said no)")
	}
}

func TestPromptForApproval_UserCancels(t *testing.T) {
	resp := &fakePromptResponder{answer: false, err: context.Canceled}
	mgr := mustManagerWithResponder(t, ModeAutoEdit, SourceFlag, true, resp)
	allowed, err := mgr.PromptForApproval(context.Background(),
		ApprovalRequest{ToolName: "shell_sandboxed", Level: LevelRun})
	if err == nil {
		t.Fatal("expected error from cancelled responder; got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error chain missing context.Canceled: %v", err)
	}
	if allowed {
		t.Error("expected allowed=false on cancellation")
	}
}

func TestPromptForApproval_NoResponder(t *testing.T) {
	mgr := mustManager(t, ModeAutoEdit, SourceFlag, true) // no responder
	allowed, err := mgr.PromptForApproval(context.Background(),
		ApprovalRequest{ToolName: "shell", Level: LevelRun})
	if err == nil {
		t.Fatal("expected error when no responder configured; got nil")
	}
	if !errors.Is(err, ErrNoPromptResponder) {
		t.Errorf("error chain missing ErrNoPromptResponder: %v", err)
	}
	if allowed {
		t.Error("expected allowed=false when no responder configured")
	}
}

func TestPromptForApproval_DefaultYesForEditLevel(t *testing.T) {
	resp := newFakeResponder(true)
	mgr := mustManagerWithResponder(t, ModeAutoEdit, SourceFlag, true, resp)
	_, _ = mgr.PromptForApproval(context.Background(),
		ApprovalRequest{ToolName: "fs_edit", Level: LevelEdit})
	if !resp.lastDef.Load() {
		t.Error("expected defaultYes=true for LevelEdit prompts")
	}
}

func TestPromptForApproval_DefaultNoForRunLevel(t *testing.T) {
	resp := newFakeResponder(true)
	mgr := mustManagerWithResponder(t, ModeAutoEdit, SourceFlag, true, resp)
	_, _ = mgr.PromptForApproval(context.Background(),
		ApprovalRequest{ToolName: "shell", Level: LevelRun})
	if resp.lastDef.Load() {
		t.Error("expected defaultYes=false for LevelRun prompts")
	}
}

// ---------------------------------------------------------------------------
// SandboxRequired / NetworkAllowed
// ---------------------------------------------------------------------------

func TestSandboxRequired_FullAutoRunTrue(t *testing.T) {
	mgr := mustManager(t, ModeFullAuto, SourceFlag, true)
	if !mgr.SandboxRequired(LevelRun) {
		t.Error("FullAuto+Run: SandboxRequired = false, want true")
	}
}

func TestSandboxRequired_FullAutoAllTrue(t *testing.T) {
	mgr := mustManager(t, ModeFullAuto, SourceFlag, true)
	if !mgr.SandboxRequired(LevelAll) {
		t.Error("FullAuto+All: SandboxRequired = false, want true")
	}
}

func TestSandboxRequired_FullAutoEditFalse(t *testing.T) {
	mgr := mustManager(t, ModeFullAuto, SourceFlag, true)
	if mgr.SandboxRequired(LevelEdit) {
		t.Error("FullAuto+Edit: SandboxRequired = true, want false (only Run/All forces sandbox)")
	}
}

func TestSandboxRequired_FullAutoReadOnlyFalse(t *testing.T) {
	mgr := mustManager(t, ModeFullAuto, SourceFlag, true)
	if mgr.SandboxRequired(LevelReadOnly) {
		t.Error("FullAuto+ReadOnly: SandboxRequired = true, want false")
	}
}

func TestSandboxRequired_OtherModesFalse(t *testing.T) {
	for _, m := range []ApprovalMode{ModeSuggest, ModeAutoEdit, ModeDangerous} {
		m := m
		t.Run(m.String(), func(t *testing.T) {
			mgr := mustManager(t, m, SourceFlag, true)
			for _, lvl := range []ApprovalLevel{LevelReadOnly, LevelEdit, LevelRun, LevelAll} {
				if mgr.SandboxRequired(lvl) {
					t.Errorf("%v+%v: SandboxRequired = true, want false", m, lvl)
				}
			}
		})
	}
}

func TestNetworkAllowed_FullAutoFalse(t *testing.T) {
	mgr := mustManager(t, ModeFullAuto, SourceFlag, true)
	if mgr.NetworkAllowed() {
		t.Error("FullAuto: NetworkAllowed = true, want false")
	}
}

func TestNetworkAllowed_OtherModes_True(t *testing.T) {
	for _, m := range []ApprovalMode{ModeSuggest, ModeAutoEdit, ModeDangerous} {
		m := m
		t.Run(m.String(), func(t *testing.T) {
			mgr := mustManager(t, m, SourceFlag, true)
			if !mgr.NetworkAllowed() {
				t.Errorf("%v: NetworkAllowed = false, want true (caller-controlled)", m)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Runtime mode change reflected in CheckApproval semantics
// ---------------------------------------------------------------------------

func TestRuntimeModeChange_ReflectedInCheckApproval(t *testing.T) {
	mgr := mustManager(t, ModeSuggest, SourceFlag, true)

	// Initially in Suggest: a Run-level call must be denied.
	got, _ := mgr.CheckApproval(ApprovalRequest{ToolName: "shell", Level: LevelRun})
	if got != ActionDenyWithReason {
		t.Fatalf("pre-swap Suggest+Run = %q, want deny", got)
	}
	if mgr.SandboxRequired(LevelRun) {
		t.Error("pre-swap Suggest: SandboxRequired = true, want false")
	}

	// Swap to FullAuto.
	if err := mgr.SetMode(ModeFullAuto); err != nil {
		t.Fatalf("SetMode(full-auto) error: %v", err)
	}

	// Same Run-level call must now be Allow with sandbox forced.
	got, err := mgr.CheckApproval(ApprovalRequest{ToolName: "shell", Level: LevelRun})
	if err != nil {
		t.Fatalf("post-swap CheckApproval error: %v", err)
	}
	if got != ActionAllow {
		t.Errorf("post-swap FullAuto+Run = %q, want allow", got)
	}
	if !mgr.SandboxRequired(LevelRun) {
		t.Error("post-swap FullAuto: SandboxRequired(Run) = false, want true")
	}
	if mgr.NetworkAllowed() {
		t.Error("post-swap FullAuto: NetworkAllowed = true, want false")
	}
	if got := mgr.Source(); got != SourceRuntime {
		t.Errorf("post-swap Source = %q, want runtime", got)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func mustManager(t *testing.T, mode ApprovalMode, src ResolvedSource, sandbox bool) *ApprovalManager {
	t.Helper()
	mgr, err := NewApprovalManager(ApprovalManagerOptions{
		InitialMode:      mode,
		Source:           src,
		SandboxAvailable: sandbox,
	})
	if err != nil {
		t.Fatalf("NewApprovalManager(%q, %q, sandbox=%v) error: %v", mode, src, sandbox, err)
	}
	return mgr
}

func mustManagerWithResponder(t *testing.T, mode ApprovalMode, src ResolvedSource, sandbox bool, resp PromptResponder) *ApprovalManager {
	t.Helper()
	mgr, err := NewApprovalManager(ApprovalManagerOptions{
		InitialMode:      mode,
		Source:           src,
		SandboxAvailable: sandbox,
		Responder:        resp,
	})
	if err != nil {
		t.Fatalf("NewApprovalManager(%q, %q, sandbox=%v, resp) error: %v", mode, src, sandbox, err)
	}
	return mgr
}
