// Package approval defines the type system for HelixCode's tool-approval gate.
//
// The gate is a runtime check applied to every tool invocation. Two inputs
// determine the outcome: the tool's required action level (read/edit/run/all)
// and the user's currently active approval mode (suggest, auto-edit, full-auto,
// dangerously-bypass). The gate produces a Decision (allow/deny/prompt) which
// the executor enforces.
//
// This file is the data-only foundation: enums, sentinels, descriptors, and
// the safe-default helper. Behaviour (the matrix that maps mode+level to
// Decision, prompter wiring, sandbox enforcement) lives in sibling files.
//
// References:
//   - Spec 7128289 §3 (Components/Types)
//   - Plan bbb61de T02
//   - CONST-035 (Zero-Bluff Mandate): all four modes mirror codex semantics
//     verbatim; no mode is a marketing label.
package approval

import (
	"errors"
	"fmt"
	"strings"
)

// ApprovalMode is one of four operating modes that govern how the gate handles
// mutating tool invocations. Names and semantics mirror codex.
type ApprovalMode string

const (
	// ModeSuggest is read-only: every mutating tool is rejected. The agent may
	// still propose edits in chat, but it cannot apply them.
	ModeSuggest ApprovalMode = "suggest"

	// ModeAutoEdit allows file edits without prompting; shell/run actions still
	// prompt the user. Sandboxing is optional in this mode.
	ModeAutoEdit ApprovalMode = "auto-edit"

	// ModeFullAuto allows edits and shell execution without prompting, but
	// REQUIRES the sandbox to be active and network egress to be denied.
	ModeFullAuto ApprovalMode = "full-auto"

	// ModeDangerous bypasses every approval check. Sandbox is NOT forced. This
	// mode is for users who accept full responsibility (CI agents, container
	// fly-by-wire, etc.).
	ModeDangerous ApprovalMode = "dangerously-bypass"
)

// IsValid reports whether m is one of the four canonical modes.
func (m ApprovalMode) IsValid() bool {
	switch m {
	case ModeSuggest, ModeAutoEdit, ModeFullAuto, ModeDangerous:
		return true
	default:
		return false
	}
}

// String returns the canonical mode string ("suggest", "auto-edit", etc.).
func (m ApprovalMode) String() string {
	return string(m)
}

// AllModes returns the four modes in canonical safety order, most-restrictive
// first. The order is load-bearing: callers (e.g. /approval show) rely on it
// to render a stable ladder.
func AllModes() []ApprovalMode {
	return []ApprovalMode{ModeSuggest, ModeAutoEdit, ModeFullAuto, ModeDangerous}
}

// ApprovalLevel is the action level a tool requires. Tools declare their level
// via a RequiresApproval() ApprovalLevel method (see DefaultLevelEdit for the
// safe default).
type ApprovalLevel int

const (
	// LevelReadOnly is for pure reads: file query, lsp lookups, no side effects.
	LevelReadOnly ApprovalLevel = iota

	// LevelEdit covers file mutations: fs_edit, smart_edit, write, mkdir, etc.
	LevelEdit

	// LevelRun covers subprocess and shell execution.
	LevelRun

	// LevelAll covers any mutating action (catch-all for tools that span
	// multiple categories or whose effect cannot be statically classified).
	LevelAll
)

// IsValid reports whether l is one of the four canonical levels.
func (l ApprovalLevel) IsValid() bool {
	return l >= LevelReadOnly && l <= LevelAll
}

// String returns the canonical level name.
func (l ApprovalLevel) String() string {
	switch l {
	case LevelReadOnly:
		return "read-only"
	case LevelEdit:
		return "edit"
	case LevelRun:
		return "run"
	case LevelAll:
		return "all"
	default:
		return fmt.Sprintf("unknown-level(%d)", int(l))
	}
}

// Decision is the result of an approval check.
type Decision int

const (
	// DecisionAllow means the executor may proceed without further interaction.
	DecisionAllow Decision = iota

	// DecisionDeny means the executor must refuse and surface a reason to the
	// caller (typically wrapping ErrApprovalDenied).
	DecisionDeny

	// DecisionPrompt means the executor must ask the user via the configured
	// Prompter; the user's answer maps onto Allow or Deny.
	DecisionPrompt
)

// String returns the canonical decision name.
func (d Decision) String() string {
	switch d {
	case DecisionAllow:
		return "allow"
	case DecisionDeny:
		return "deny"
	case DecisionPrompt:
		return "prompt"
	default:
		return fmt.Sprintf("unknown-decision(%d)", int(d))
	}
}

// Action is what the gate does at runtime. It is a sibling type to Decision
// retained for callers that prefer imperative naming (gate.Action vs. policy
// Decision); they are isomorphic but kept separate so the policy layer can
// evolve without churning executor code.
type Action int

const (
	// ActionAllow: proceed with the tool call.
	ActionAllow Action = iota

	// ActionPromptUser: pause and ask the user.
	ActionPromptUser

	// ActionDenyWithReason: refuse and propagate a human-readable reason.
	ActionDenyWithReason
)

// String returns the canonical action name.
func (a Action) String() string {
	switch a {
	case ActionAllow:
		return "allow"
	case ActionPromptUser:
		return "prompt-user"
	case ActionDenyWithReason:
		return "deny-with-reason"
	default:
		return fmt.Sprintf("unknown-action(%d)", int(a))
	}
}

// ResolvedSource identifies how the active mode was selected. Reported by
// /approval status so the user can see whether their current mode came from a
// CLI flag, an env var, the config file, or the built-in default.
type ResolvedSource int

const (
	// SourceFlag: command-line flag (highest priority).
	SourceFlag ResolvedSource = iota

	// SourceEnv: environment variable (HELIX_APPROVAL_MODE or similar).
	SourceEnv

	// SourceConfig: config file (YAML/TOML).
	SourceConfig

	// SourceDefault: hard-coded built-in default (lowest priority).
	SourceDefault

	// SourceRuntime: mode was changed at runtime (e.g. via /approval set or
	// ApprovalManager.SetMode). Sits outside the precedence chain — it is
	// only assigned after Select has already picked an initial source and
	// the user later asks the manager to swap modes mid-session.
	SourceRuntime
)

// String returns the canonical source name.
func (s ResolvedSource) String() string {
	switch s {
	case SourceFlag:
		return "flag"
	case SourceEnv:
		return "env"
	case SourceConfig:
		return "config"
	case SourceDefault:
		return "default"
	case SourceRuntime:
		return "runtime"
	default:
		return fmt.Sprintf("unknown-source(%d)", int(s))
	}
}

// ModeDescriptor describes a single mode for /approval show output and other
// human-facing surfaces. It is data-only; behaviour belongs in the policy
// engine.
type ModeDescriptor struct {
	// Mode is the mode this descriptor describes.
	Mode ApprovalMode

	// Description is a 1-2 sentence summary suitable for CLI output.
	Description string

	// SafetyOrder ranks this mode against its peers; 0 is the safest, 3 is
	// the most permissive. Matches the index in AllModes().
	SafetyOrder int

	// SandboxRule describes how the sandbox is treated in this mode:
	// "n/a" (suggest, no mutations possible), "optional" (auto-edit), "required"
	// (full-auto), or "skipped" (dangerously-bypass).
	SandboxRule string

	// NetworkRule documents network egress policy. Only ModeFullAuto sets this
	// to "denied"; others use "n/a".
	NetworkRule string
}

// ModeDescriptors returns the four canonical mode descriptors keyed by mode.
// The map has exactly four entries; callers may rely on len()==4.
func ModeDescriptors() map[ApprovalMode]ModeDescriptor {
	return map[ApprovalMode]ModeDescriptor{
		ModeSuggest: {
			Mode:        ModeSuggest,
			Description: "Read-only mode. The agent may inspect files and propose changes but cannot apply edits or run commands.",
			SafetyOrder: 0,
			SandboxRule: "n/a",
			NetworkRule: "n/a",
		},
		ModeAutoEdit: {
			Mode:        ModeAutoEdit,
			Description: "File edits proceed without prompting; shell and subprocess execution prompts the user. Sandboxing is optional.",
			SafetyOrder: 1,
			SandboxRule: "optional",
			NetworkRule: "n/a",
		},
		ModeFullAuto: {
			Mode:        ModeFullAuto,
			Description: "Edits and shell execution proceed without prompts. The sandbox is forced on and network egress is denied.",
			SafetyOrder: 2,
			SandboxRule: "required",
			NetworkRule: "denied",
		},
		ModeDangerous: {
			Mode:        ModeDangerous,
			Description: "All approval checks are bypassed and the sandbox is skipped. Only for trusted automation contexts.",
			SafetyOrder: 3,
			SandboxRule: "skipped",
			NetworkRule: "n/a",
		},
	}
}

// ParseMode parses a string into an ApprovalMode. The match is
// case-insensitive and treats underscores as dashes, so "AUTO_EDIT",
// "Auto-Edit", and "auto-edit" all resolve to ModeAutoEdit. Unknown or empty
// input yields ErrInvalidMode wrapped with the offending value.
func ParseMode(s string) (ApprovalMode, error) {
	if s == "" {
		return "", fmt.Errorf("%w: empty string", ErrInvalidMode)
	}
	normalised := strings.ToLower(strings.ReplaceAll(s, "_", "-"))
	candidate := ApprovalMode(normalised)
	if candidate.IsValid() {
		return candidate, nil
	}
	return "", fmt.Errorf("%w: %q", ErrInvalidMode, s)
}

// Sentinel errors. All errors returned by this package wrap one of these so
// callers can use errors.Is for classification.
var (
	// ErrInvalidMode is returned by ParseMode when the input is unrecognised.
	ErrInvalidMode = errors.New("invalid approval mode")

	// ErrInvalidLevel is returned when a tool reports an unknown ApprovalLevel.
	ErrInvalidLevel = errors.New("invalid approval level")

	// ErrApprovalDenied is returned when the gate refuses a tool call.
	ErrApprovalDenied = errors.New("approval denied")

	// ErrApprovalRequired is returned when a mode forbids a level outright
	// (e.g. ModeSuggest + LevelEdit) and the call cannot even be prompted.
	ErrApprovalRequired = errors.New("approval required")

	// ErrUserCancelled is returned when the user declines an interactive prompt.
	ErrUserCancelled = errors.New("user cancelled")
)

// DefaultLevelEdit is an embeddable struct that provides a safe-default
// RequiresApproval() implementation returning LevelEdit. Tools that do not
// explicitly classify themselves should embed this so the gate errs on the
// side of asking, not allowing.
type DefaultLevelEdit struct{}

// RequiresApproval returns LevelEdit (the safe default for unspecified tools).
func (DefaultLevelEdit) RequiresApproval() ApprovalLevel { return LevelEdit }
