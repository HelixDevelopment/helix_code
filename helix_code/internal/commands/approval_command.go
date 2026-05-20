// Package commands — approval_command.go (P2-F21-T06).
//
// ApprovalCommand implements the /approval slash command with three
// subcommands: status (default), set <mode>, and show [<mode>|all]. It is
// the user-facing surface for HelixCode's F21 approval gate.
//
// Subcommands:
//
//	/approval                      → alias of /approval status
//	/approval status               → current mode + source + sandbox/network rules
//	/approval set <mode>           → swap the active mode at runtime via
//	                                 ApprovalManager.SetMode; reports the
//	                                 transition + warns when full-auto is
//	                                 selected (sandbox required)
//	/approval show <mode>          → describe a specific mode using
//	                                 approval.ModeDescriptors
//	/approval show all (or empty)  → describe all four modes in safety order
//
// Anti-bluff contract: the command MUST consult the live ApprovalManager
// (via ApprovalInspector) for every status read and route every set through
// SetMode. There is no cached state and no fake "success" path — when
// SetMode returns an error (e.g. ErrSandboxRequired for full-auto without
// sandbox), the command surfaces the wrapped error and the inspector's mode
// is unchanged.
//
// Style mirrors theme_command.go (commit 348630c, F20-T07): same Command
// interface, same tabwriter status block, same error envelope. F19/F20
// slash precedent applies: Name/Aliases/Description/Usage/Execute(ctx,
// *CommandContext) (*CommandResult, error).
//
// References:
//   - Spec 7128289 §6 (User surface)
//   - Plan bbb61de T06
//   - F20 /theme precedent: internal/commands/theme_command.go
package commands

import (
	"context"
	"fmt"
	"strings"
	"text/tabwriter"

	"dev.helix.code/internal/approval"
)

// ApprovalInspector is the subset of *approval.ApprovalManager that
// ApprovalCommand depends on. Defining the interface in the commands package
// keeps the slash command testable with a fake while still letting main.go
// pass the real *approval.ApprovalManager directly (Go satisfies interfaces
// structurally).
//
// Three methods only — observe (Mode, Source) and mutate (SetMode). Other
// manager surface (CheckApproval, PromptForApproval, sandbox helpers) is not
// exposed to /approval; runtime gating is the executor's job, not the user
// surface's.
type ApprovalInspector interface {
	Mode() approval.ApprovalMode
	Source() approval.ResolvedSource
	SetMode(newMode approval.ApprovalMode) error
}

// Source-label strings used by /approval status. Kept distinct from the raw
// ResolvedSource.String() values so the user surface can read more
// descriptively (e.g. "HELIXCODE_APPROVAL env var" instead of "env").
//
// CONST-046: every branch resolves through the package i18n seam so the
// /approval status block adapts to the operator's locale.
func sourceLabel(ctx context.Context, s approval.ResolvedSource) string {
	switch s {
	case approval.SourceFlag:
		return tr(ctx, "internal_commands_approval_source_flag", nil)
	case approval.SourceEnv:
		return tr(ctx, "internal_commands_approval_source_env",
			map[string]any{"Var": approval.EnvVarName})
	case approval.SourceConfig:
		return tr(ctx, "internal_commands_approval_source_config", nil)
	case approval.SourceDefault:
		return tr(ctx, "internal_commands_approval_source_default", nil)
	case approval.SourceRuntime:
		return tr(ctx, "internal_commands_approval_source_runtime", nil)
	default:
		return s.String()
	}
}

// networkLabel renders the active network policy for the status block. It
// derives the value from the descriptor table so /approval status and
// /approval show stay consistent even if the descriptor changes.
//
// CONST-046: the descriptor-missing fallback resolves through the i18n
// seam; descriptor-sourced rules are provider metadata (composed elsewhere).
func networkLabel(ctx context.Context, m approval.ApprovalMode) string {
	if d, ok := approval.ModeDescriptors()[m]; ok {
		return d.NetworkRule
	}
	return tr(ctx, "internal_commands_approval_rule_not_available", nil)
}

// sandboxLabel mirrors networkLabel for sandbox policy.
func sandboxLabel(ctx context.Context, m approval.ApprovalMode) string {
	if d, ok := approval.ModeDescriptors()[m]; ok {
		return d.SandboxRule
	}
	return tr(ctx, "internal_commands_approval_rule_not_available", nil)
}

// ApprovalCommand is the /approval slash command.
type ApprovalCommand struct {
	manager ApprovalInspector
}

// NewApprovalCommand constructs the /approval slash command bound to the
// supplied inspector. Passing a nil inspector is a programmer error and is
// not defended against here — main.go always wires the real manager.
func NewApprovalCommand(m ApprovalInspector) *ApprovalCommand {
	return &ApprovalCommand{manager: m}
}

// Name returns the slash command name (without the leading slash).
func (c *ApprovalCommand) Name() string { return "approval" }

// Aliases returns alternative invocation names. /approval has none.
func (c *ApprovalCommand) Aliases() []string { return nil }

// Description returns the one-line help blurb shown by /help.
func (c *ApprovalCommand) Description() string {
	return tr(context.Background(), "internal_commands_approval_description", nil)
}

// Usage returns the usage string shown by /help.
func (c *ApprovalCommand) Usage() string {
	return tr(context.Background(), "internal_commands_approval_usage", nil)
}

// Execute dispatches to the appropriate subcommand handler.
//
// The default subcommand (no args) is `status` — it answers the most common
// entry-point question: "what mode am I in and what put me there?"
func (c *ApprovalCommand) Execute(ctx context.Context, cc *CommandContext) (*CommandResult, error) {
	args := cc.Args
	sub := "status"
	if len(args) > 0 {
		sub = args[0]
	}
	switch sub {
	case "status":
		return &CommandResult{Success: true, Output: c.handleStatus(ctx)}, nil
	case "set":
		out, err := c.handleSet(ctx, args[1:])
		if err != nil {
			return nil, err
		}
		return &CommandResult{Success: true, Output: out}, nil
	case "show":
		out, err := c.handleShow(ctx, args[1:])
		if err != nil {
			return nil, err
		}
		return &CommandResult{Success: true, Output: out}, nil
	default:
		return nil, fmt.Errorf("/approval: unknown subcommand %q (want status|set|show)", sub)
	}
}

// handleStatus renders the active-mode block: mode, source, sandbox rule,
// network rule. Format mirrors /theme status (tabwriter-aligned key:value).
func (c *ApprovalCommand) handleStatus(ctx context.Context) string {
	var sb strings.Builder
	sb.WriteString(tr(ctx, "internal_commands_approval_status_header", nil) + "\n")

	mode := c.manager.Mode()
	src := c.manager.Source()

	tw := tabwriter.NewWriter(&sb, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "  %s:\t%s\n", tr(ctx, "internal_commands_approval_label_mode", nil), mode.String())
	fmt.Fprintf(tw, "  %s:\t%s\n", tr(ctx, "internal_commands_approval_label_source", nil), sourceLabel(ctx, src))
	fmt.Fprintf(tw, "  %s:\t%s\n", tr(ctx, "internal_commands_approval_label_sandbox", nil), sandboxLabel(ctx, mode))
	fmt.Fprintf(tw, "  %s:\t%s\n", tr(ctx, "internal_commands_approval_label_network", nil), networkLabel(ctx, mode))
	tw.Flush()
	return sb.String()
}

// handleSet parses the requested mode, calls SetMode on the manager, and
// reports the transition. Errors from SetMode (ErrInvalidMode,
// ErrSandboxRequired) propagate up to Execute and surface to the user.
//
// Output format on success:
//
//	"Approval mode set: <old> -> <new> (source: runtime)"
//
// followed by mode-specific advisory lines (currently only full-auto, which
// gets a sandbox warning).
func (c *ApprovalCommand) handleSet(ctx context.Context, args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("/approval set: missing mode (usage: /approval set <suggest|auto-edit|full-auto|dangerously-bypass>)")
	}
	newMode, err := approval.ParseMode(args[0])
	if err != nil {
		return "", fmt.Errorf("/approval set: %w", err)
	}
	oldMode := c.manager.Mode()
	if err := c.manager.SetMode(newMode); err != nil {
		return "", fmt.Errorf("/approval set: %w", err)
	}

	var sb strings.Builder
	sb.WriteString(tr(ctx, "internal_commands_approval_mode_set", map[string]any{
		"Old": oldMode.String(),
		"New": newMode.String(),
	}) + "\n")
	if newMode == approval.ModeFullAuto {
		sb.WriteString(tr(ctx, "internal_commands_approval_warn_full_auto", nil) + "\n")
	}
	if newMode == approval.ModeDangerous {
		sb.WriteString(tr(ctx, "internal_commands_approval_warn_dangerous", nil) + "\n")
	}
	return sb.String(), nil
}

// handleShow renders the descriptor block for a specific mode (or all four
// when args is empty / "all"). The single-mode rendering is also the per-row
// shape used by the all-modes path so output stays uniform.
func (c *ApprovalCommand) handleShow(ctx context.Context, args []string) (string, error) {
	descriptors := approval.ModeDescriptors()
	if len(args) == 0 || args[0] == "all" {
		var sb strings.Builder
		for _, m := range approval.AllModes() {
			sb.WriteString(renderDescriptor(ctx, descriptors[m]))
		}
		return sb.String(), nil
	}
	mode, err := approval.ParseMode(args[0])
	if err != nil {
		return "", fmt.Errorf("/approval show: %w", err)
	}
	d, ok := descriptors[mode]
	if !ok {
		// Defensive: ParseMode succeeded but descriptor missing. Should be
		// unreachable because ModeDescriptors covers all four canonical modes.
		return "", fmt.Errorf("/approval show: no descriptor for mode %q", mode)
	}
	return renderDescriptor(ctx, d), nil
}

// renderDescriptor formats a single ModeDescriptor as a labeled block.
// Identical shape across single-mode and all-modes paths.
func renderDescriptor(ctx context.Context, d approval.ModeDescriptor) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s: %s\n", tr(ctx, "internal_commands_approval_label_mode", nil), d.Mode.String())
	tw := tabwriter.NewWriter(&sb, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "  %s:\t%s\n", tr(ctx, "internal_commands_approval_label_description", nil), d.Description)
	fmt.Fprintf(tw, "  %s:\t%s\n", tr(ctx, "internal_commands_approval_label_sandbox", nil), d.SandboxRule)
	fmt.Fprintf(tw, "  %s:\t%s\n", tr(ctx, "internal_commands_approval_label_network", nil), d.NetworkRule)
	fmt.Fprintf(tw, "  %s:\t%d (%s)\n", tr(ctx, "internal_commands_approval_label_safety", nil),
		d.SafetyOrder, safetyLabel(ctx, d.SafetyOrder))
	tw.Flush()
	sb.WriteString("\n")
	return sb.String()
}

// safetyLabel maps the descriptor's SafetyOrder index to a human-readable
// position on the safety ladder. Kept private; the suggest=0 ↔ dangerous=3
// ordering matches AllModes() and ModeDescriptors() and is load-bearing for
// /approval show all output ordering tests.
//
// CONST-046: each rung resolves through the package i18n seam.
func safetyLabel(ctx context.Context, order int) string {
	switch order {
	case 0:
		return tr(ctx, "internal_commands_approval_safety_most_restrictive", nil)
	case 1:
		return tr(ctx, "internal_commands_approval_safety_moderate", nil)
	case 2:
		return tr(ctx, "internal_commands_approval_safety_permissive", nil)
	case 3:
		return tr(ctx, "internal_commands_approval_safety_least_restrictive", nil)
	default:
		return tr(ctx, "internal_commands_approval_safety_rank", map[string]any{"Order": order})
	}
}
