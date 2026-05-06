// manager.go (P2-F21-T04): the central approval gate. Holds the active
// ApprovalMode + ResolvedSource behind atomic pointers so /approval slash and
// any other runtime caller can swap modes mid-session without locking.
//
// Two pieces of public API matter to callers:
//
//   - CheckApproval(req) -> Action: pure decision per the 4x4 matrix below.
//     Tool wrappers run this BEFORE invoking the tool.
//
//   - PromptForApproval(ctx, req) -> (allowed, error): the second leg, used
//     when CheckApproval returns ActionPromptUser. The manager delegates the
//     question to a PromptResponder (F19's stdinPrompter is the canonical
//     impl, but tests inject fakes).
//
// Decision matrix (Spec 7128289 §4):
//
//	mode \ level   read-only   edit         run        all
//	---------------------------------------------------------
//	suggest        ALLOW       DENY         DENY       DENY
//	auto-edit      ALLOW       ALLOW        PROMPT     PROMPT
//	full-auto      ALLOW       ALLOW        ALLOW*     ALLOW*
//	dangerously    ALLOW       ALLOW        ALLOW      ALLOW
//
//	*full-auto Run/All are wrapped with the sandbox + network DENY by the
//	registry pre-execute hook (see SandboxRequired / NetworkAllowed).
//
// Cross-feature contracts:
//
//   - F02 (permission rules): orthogonal — F02 makes per-rule allow/deny
//     decisions BEFORE the gate. The gate is the final fail-closed check.
//   - F14 (sandbox): full-auto requires sandbox to be available at startup.
//     SandboxRequired(level) tells the registry whether to wrap.
//   - F19 (askuser prompter): supplies the PromptResponder for ModeAutoEdit's
//     Run/All prompts.
//
// References:
//   - Spec 7128289 §3 (Components/Types) and §4 (matrix)
//   - Plan bbb61de T04
//   - CONST-035 Zero-Bluff: "ALLOW" must mean the user can actually use the
//     feature; the gate produces no fake allows or fake denies.

package approval

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"
)

// ErrSandboxRequired is returned when ModeFullAuto is selected but the host
// reports SandboxAvailable=false. Per Spec §5.1 full-auto is not viable
// without an active sandbox; the manager refuses rather than silently
// downgrading.
var ErrSandboxRequired = errors.New("approval: full-auto mode requires sandbox")

// ErrNoPromptResponder is returned when PromptForApproval is invoked on a
// manager that was constructed without a PromptResponder.
var ErrNoPromptResponder = errors.New("approval: no prompt responder configured")

// ApprovalRequest is what a tool's pre-execute gate sends to the manager.
// Args is included for prompt-context only — the manager never inspects it
// for policy decisions (F02 owns content-aware policy).
type ApprovalRequest struct {
	// ToolName is the registered name of the tool (e.g. "fs_edit",
	// "shell_sandboxed").
	ToolName string

	// Level is the tool's RequiresApproval() output. Validated by
	// CheckApproval; an out-of-range level yields ActionDenyWithReason.
	Level ApprovalLevel

	// Args is the tool's argument map, surfaced verbatim in the prompt
	// summary. Optional.
	Args map[string]any

	// Context is an optional human description that is appended to the
	// prompt question (e.g. "to apply patch X").
	Context string
}

// PromptResponder abstracts the F19 prompter (or any compatible
// yes/no-asking surface). PromptYesNo returns true=allow, false=deny; a
// non-nil error means the question could not be answered (cancellation,
// EOF, etc.) and the caller must treat the request as denied.
type PromptResponder interface {
	PromptYesNo(ctx context.Context, question string, defaultYes bool) (bool, error)
}

// ApprovalManagerOptions configures NewApprovalManager. Zero-valued options
// are valid for read-only managers (Suggest mode, no responder).
type ApprovalManagerOptions struct {
	// InitialMode is the mode the manager starts in. Must be one of the
	// four canonical modes; any other value is rejected with ErrInvalidMode.
	InitialMode ApprovalMode

	// Source records how InitialMode was selected (from Selector.Select).
	// Reported by /approval status. After SetMode, this is overwritten with
	// SourceRuntime.
	Source ResolvedSource

	// Responder is the F19 prompter (or compatible). Only consulted by
	// PromptForApproval; CheckApproval never calls it. Required if any
	// caller will reach ActionPromptUser.
	Responder PromptResponder

	// SandboxAvailable is checked at construction. ModeFullAuto requires
	// it to be true, otherwise NewApprovalManager returns ErrSandboxRequired.
	SandboxAvailable bool

	// PauseDangerous is the startup pause applied when InitialMode ==
	// ModeDangerous. Default 2s if you want it; 0 disables. The manager
	// never picks a default itself — callers (CLI bootstrap) decide.
	PauseDangerous time.Duration

	// SleepFunc is the function used to apply PauseDangerous. Tests inject
	// a recording sleep. nil falls back to time.Sleep.
	SleepFunc func(time.Duration)
}

// ApprovalManager is the central gate. Mode and Source are stored behind
// atomic.Pointer so reads from CheckApproval (hot path) never block, and
// SetMode is a single-pointer swap.
type ApprovalManager struct {
	mode      atomic.Pointer[ApprovalMode]
	source    atomic.Pointer[ResolvedSource]
	responder PromptResponder
	sandboxOK bool
}

// NewApprovalManager constructs the manager and applies the dangerous-mode
// startup pause synchronously when applicable. Errors:
//   - ErrInvalidMode  : InitialMode is not one of the four canonical modes
//   - ErrSandboxRequired : ModeFullAuto + !SandboxAvailable
func NewApprovalManager(opts ApprovalManagerOptions) (*ApprovalManager, error) {
	if !opts.InitialMode.IsValid() {
		return nil, fmt.Errorf("%w: %q", ErrInvalidMode, opts.InitialMode)
	}
	if opts.InitialMode == ModeFullAuto && !opts.SandboxAvailable {
		return nil, fmt.Errorf("%w (mode=%s)", ErrSandboxRequired, opts.InitialMode)
	}

	m := &ApprovalManager{
		responder: opts.Responder,
		sandboxOK: opts.SandboxAvailable,
	}
	mode := opts.InitialMode
	src := opts.Source
	m.mode.Store(&mode)
	m.source.Store(&src)

	if opts.InitialMode == ModeDangerous && opts.PauseDangerous > 0 {
		sleep := opts.SleepFunc
		if sleep == nil {
			sleep = time.Sleep
		}
		sleep(opts.PauseDangerous)
	}

	return m, nil
}

// Mode returns the active mode (atomic read).
func (m *ApprovalManager) Mode() ApprovalMode {
	if p := m.mode.Load(); p != nil {
		return *p
	}
	return ModeSuggest
}

// Source returns how the active mode was resolved (atomic read).
func (m *ApprovalManager) Source() ResolvedSource {
	if p := m.source.Load(); p != nil {
		return *p
	}
	return SourceDefault
}

// SetMode atomically swaps the mode. Used by /approval slash for runtime
// mode change. Errors:
//   - ErrInvalidMode      : newMode is not one of the four canonical modes
//   - ErrSandboxRequired  : newMode is ModeFullAuto but the manager was
//     constructed with SandboxAvailable=false
//
// On success Source becomes SourceRuntime so /approval status can show the
// user that they (or the CLI shim) overrode the original source.
func (m *ApprovalManager) SetMode(newMode ApprovalMode) error {
	if !newMode.IsValid() {
		return fmt.Errorf("%w: %q", ErrInvalidMode, newMode)
	}
	if newMode == ModeFullAuto && !m.sandboxOK {
		return fmt.Errorf("%w (mode=%s)", ErrSandboxRequired, newMode)
	}
	mode := newMode
	src := SourceRuntime
	m.mode.Store(&mode)
	m.source.Store(&src)
	return nil
}

// CheckApproval is the per-call gate. See file header for the 4x4 matrix.
// Returns ActionDenyWithReason wrapped with ErrApprovalDenied for refusals
// and ErrInvalidLevel for out-of-range levels. The PromptResponder is NOT
// invoked here; ActionPromptUser tells the caller to invoke
// PromptForApproval next.
func (m *ApprovalManager) CheckApproval(req ApprovalRequest) (Action, error) {
	if !req.Level.IsValid() {
		return ActionDenyWithReason, fmt.Errorf("%w: %d (tool %q)", ErrInvalidLevel, int(req.Level), req.ToolName)
	}

	switch m.Mode() {
	case ModeSuggest:
		if req.Level == LevelReadOnly {
			return ActionAllow, nil
		}
		return ActionDenyWithReason,
			fmt.Errorf("%w: tool %q requires %s but mode is %s (read-only)",
				ErrApprovalDenied, req.ToolName, req.Level, ModeSuggest)

	case ModeAutoEdit:
		switch req.Level {
		case LevelReadOnly, LevelEdit:
			return ActionAllow, nil
		case LevelRun, LevelAll:
			return ActionPromptUser, nil
		default:
			return ActionDenyWithReason,
				fmt.Errorf("%w: %d", ErrInvalidLevel, int(req.Level))
		}

	case ModeFullAuto:
		// All levels allowed; sandbox + network DENY enforced by the
		// registry pre-execute hook via SandboxRequired/NetworkAllowed.
		return ActionAllow, nil

	case ModeDangerous:
		// All checks bypassed.
		return ActionAllow, nil

	default:
		// Defensive: should be unreachable because IsValid() gates the
		// constructor and SetMode.
		return ActionDenyWithReason,
			fmt.Errorf("%w: %q", ErrInvalidMode, m.Mode())
	}
}

// PromptForApproval asks the user (via the PromptResponder) whether to
// allow a tool call that CheckApproval routed to ActionPromptUser. The
// returned bool is the user's answer; the error is non-nil only when the
// prompter could not deliver an answer (no responder configured,
// cancellation, I/O failure).
//
// Question phrasing:
//
//	"Allow tool '<name>' (level=<level>)? <args summary> <context>"
//
// Default polarity:
//   - LevelEdit     : defaultYes=true   (matrix already auto-allows edits;
//     this branch is only reached if a future caller routes Edit-level to
//     prompt; we keep the polarity consistent for forward compat)
//   - LevelRun/All  : defaultYes=false  (safer)
func (m *ApprovalManager) PromptForApproval(ctx context.Context, req ApprovalRequest) (bool, error) {
	if m.responder == nil {
		return false, fmt.Errorf("%w (tool %q)", ErrNoPromptResponder, req.ToolName)
	}
	question := buildPromptQuestion(req)
	defaultYes := req.Level == LevelEdit
	allowed, err := m.responder.PromptYesNo(ctx, question, defaultYes)
	if err != nil {
		return false, fmt.Errorf("approval prompt for %q: %w", req.ToolName, err)
	}
	return allowed, nil
}

// SandboxRequired reports whether the active mode requires the sandbox for
// the given level. Only ModeFullAuto + LevelRun/LevelAll forces the
// sandbox. Other (mode, level) pairs return false; the caller may still
// elect to wrap on its own (e.g. shell_sandboxed always wraps).
func (m *ApprovalManager) SandboxRequired(level ApprovalLevel) bool {
	if m.Mode() != ModeFullAuto {
		return false
	}
	return level == LevelRun || level == LevelAll
}

// NetworkAllowed reports whether the active mode allows network access in
// sandbox-wrapped calls. Only ModeFullAuto denies network egress; every
// other mode is caller-controlled (returns true).
func (m *ApprovalManager) NetworkAllowed() bool {
	return m.Mode() != ModeFullAuto
}

// buildPromptQuestion renders the human-facing question shown by the F19
// prompter. Kept private; tests assert via the recorded question text on
// the fake responder.
func buildPromptQuestion(req ApprovalRequest) string {
	q := fmt.Sprintf("Allow tool %q (level=%s)?", req.ToolName, req.Level)
	if len(req.Args) > 0 {
		q += fmt.Sprintf(" args=%v", req.Args)
	}
	if req.Context != "" {
		q += " " + req.Context
	}
	return q
}
