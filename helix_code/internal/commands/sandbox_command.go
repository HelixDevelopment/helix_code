// Package commands — sandbox_command.go.
//
// SandboxCommand implements the /sandbox slash command with three
// subcommands: status, test, policy. It is the user-facing surface for
// HelixCode's F14 sandbox feature.
//
// Subcommands:
//
//	/sandbox                 alias of /sandbox status
//	/sandbox status          backend + capabilities + current config summary
//	/sandbox test [<cmd...>] runs <cmd> (default `echo helix-sandbox-test`)
//	                         inside the sandbox to prove it works; reports
//	                         stdout + exit code + backend + duration
//	/sandbox policy          merged deny-list (CONST-033 immutable + user)
//	                         and default policy from config
//
// Anti-bluff contract: /sandbox test MUST call the manager's real Execute
// which dispatches to the real backend (bubblewrap/native). There is no
// fake-output path. The fake manager used in tests is a hexagonal seam --
// production wiring (T10) hands the command the real *sandbox.SandboxManager.
package commands

import (
	"context"
	"fmt"
	"strings"
	"text/tabwriter"

	"dev.helix.code/internal/tools/sandbox"
)

// SandboxManager is the subset of *sandbox.SandboxManager that
// SandboxCommand depends on.
//
// Defining the interface in the commands package keeps the slash command
// testable with a fake while still letting main.go pass the real
// *sandbox.SandboxManager directly (Go satisfies interfaces structurally).
type SandboxManager interface {
	Capabilities() sandbox.SandboxCapabilities
	SelectedBackend() sandbox.BackendKind
	Config() sandbox.SandboxConfig
	MergedDenyList() (constitutional []string, user []string)
	Execute(ctx context.Context, command string, policy sandbox.SandboxPolicy) (*sandbox.SandboxResult, error)
}

// SandboxCommand is the /sandbox slash command.
type SandboxCommand struct {
	manager SandboxManager
}

// NewSandboxCommand constructs the /sandbox slash command.
func NewSandboxCommand(m SandboxManager) *SandboxCommand {
	return &SandboxCommand{manager: m}
}

// Name returns the slash command name (without the leading slash).
func (c *SandboxCommand) Name() string { return "sandbox" }

// Aliases returns alternative invocation names. /sandbox has none.
func (c *SandboxCommand) Aliases() []string { return nil }

// Description returns the one-line help blurb shown by /help.
func (c *SandboxCommand) Description() string {
	return tr(context.Background(), "internal_commands_sandbox_description", nil)
}

// Usage returns the usage string shown by /help.
func (c *SandboxCommand) Usage() string {
	return tr(context.Background(), "internal_commands_sandbox_usage", nil)
}

// Execute dispatches to the appropriate subcommand handler.
//
// The default subcommand (no args) is `status` — it answers "is the
// sandbox available and what's it using" which is the most common
// entry-point question.
func (c *SandboxCommand) Execute(ctx context.Context, cc *CommandContext) (*CommandResult, error) {
	args := cc.Args
	sub := "status"
	if len(args) > 0 {
		sub = args[0]
	}
	switch sub {
	case "status":
		return c.handleStatus(ctx), nil
	case "test":
		return c.handleTest(ctx, args[1:])
	case "policy":
		return c.handlePolicy(ctx), nil
	default:
		return nil, fmt.Errorf("%s", tr(ctx, "internal_commands_sandbox_unknown_subcommand", map[string]any{"Subcommand": sub}))
	}
}

// handleStatus renders the sandbox status: GOOS, selected backend, the
// host-detected capabilities, and the default policy summary from the
// current config.
//
// When SelectedBackend is BackendNone we lead with a "Sandbox unavailable:
// <reason>" line so the user immediately sees why nothing else will work.
func (c *SandboxCommand) handleStatus(ctx context.Context) *CommandResult {
	caps := c.manager.Capabilities()
	cfg := c.manager.Config()

	var sb strings.Builder
	sb.WriteString(tr(ctx, "internal_commands_sandbox_status_header", nil) + "\n")
	if caps.SelectedBackend == sandbox.BackendNone {
		reason := caps.UnavailableReason
		if reason == "" {
			reason = tr(ctx, "internal_commands_sandbox_no_backend_selected", nil)
		}
		fmt.Fprintf(&sb, "%s\n", tr(ctx, "internal_commands_sandbox_unavailable", map[string]any{"Reason": reason}))
	}

	tw := tabwriter.NewWriter(&sb, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "  GOOS:\t%s\n", caps.GOOS)
	fmt.Fprintf(tw, "  Backend:\t%s\n", caps.SelectedBackend.String())
	bwPath := caps.BubblewrapPath
	if bwPath == "" {
		bwPath = "(not found)"
	}
	fmt.Fprintf(tw, "  Bubblewrap path:\t%s\n", bwPath)
	fmt.Fprintf(tw, "  Unprivileged userns:\t%t\n", caps.UnprivilegedUserNS)
	fmt.Fprintf(tw, "  Cgroups v2:\t%t\n", caps.CGroupsV2)

	netDefault := "deny"
	if cfg.DefaultPolicy.NetworkAllowed {
		netDefault = "allow"
	}
	fmt.Fprintf(tw, "  Default network:\t%s\n", netDefault)
	fmt.Fprintf(tw, "  Default timeout:\t%s\n", cfg.DefaultPolicy.Timeout.String())
	fmt.Fprintf(tw, "  User deny rules:\t%d\n", len(cfg.UserDenyList))
	tw.Flush()

	return &CommandResult{Success: true, Output: sb.String()}
}

// handleTest runs a probe command inside the sandbox and surfaces the
// result so the user can confirm the backend really works on this host.
//
// args is the remaining argv after the `test` subcommand (e.g. ["echo",
// "hi"]). When empty we use a deterministic default `echo helix-sandbox-test`
// — chosen so an operator can grep for the literal token in the output to
// confirm the backend round-tripped a real string.
//
// We pass the zero SandboxPolicy to Execute so the manager substitutes
// DefaultSandboxPolicy() (network DENY, 30s timeout, RO root). This is
// the safest probe and matches what `/sandbox test` is documented to do.
func (c *SandboxCommand) handleTest(ctx context.Context, args []string) (*CommandResult, error) {
	command := "echo helix-sandbox-test"
	if len(args) > 0 {
		command = strings.Join(args, " ")
	}

	result, err := c.manager.Execute(ctx, command, sandbox.SandboxPolicy{})
	if err != nil {
		return nil, fmt.Errorf("/sandbox test: %w", err)
	}
	if result == nil {
		return nil, fmt.Errorf("%s", tr(ctx, "internal_commands_sandbox_test_no_result", nil))
	}

	var sb strings.Builder
	tw := tabwriter.NewWriter(&sb, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "Test command:\t%s\n", command)
	fmt.Fprintf(tw, "Backend:\t%s\n", result.Backend.String())
	fmt.Fprintf(tw, "Exit code:\t%d\n", result.ExitCode)
	fmt.Fprintf(tw, "Duration:\t%s\n", result.Duration.String())
	if result.TimedOut {
		fmt.Fprintf(tw, "Timed out:\t%t\n", result.TimedOut)
	}
	tw.Flush()

	if result.Stdout != "" {
		sb.WriteString(tr(ctx, "internal_commands_sandbox_stdout_label", nil) + "\n")
		sb.WriteString(indent(result.Stdout, "  "))
		if !strings.HasSuffix(result.Stdout, "\n") {
			sb.WriteString("\n")
		}
	}
	if result.Stderr != "" {
		sb.WriteString(tr(ctx, "internal_commands_sandbox_stderr_label", nil) + "\n")
		sb.WriteString(indent(result.Stderr, "  "))
		if !strings.HasSuffix(result.Stderr, "\n") {
			sb.WriteString("\n")
		}
	}

	return &CommandResult{Success: true, Output: sb.String()}, nil
}

// handlePolicy renders the merged deny-list (CONST-033 + user-configured)
// and the default policy from the current config.
//
// The CONST-033 list is described in human-readable terms (the
// `MergedDenyList` first slice contains entry descriptions, not raw
// regex sources); the user list is the raw user-authored patterns so
// the operator can copy them back into config to edit.
func (c *SandboxCommand) handlePolicy(ctx context.Context) *CommandResult {
	cfg := c.manager.Config()
	constDeny, userDeny := c.manager.MergedDenyList()

	var sb strings.Builder
	sb.WriteString(tr(ctx, "internal_commands_sandbox_default_policy_header", nil) + "\n")
	tw := tabwriter.NewWriter(&sb, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "  network_allowed:\t%t\n", cfg.DefaultPolicy.NetworkAllowed)
	fmt.Fprintf(tw, "  timeout:\t%s\n", cfg.DefaultPolicy.Timeout.String())
	fmt.Fprintf(tw, "  read_only_root:\t%t\n", cfg.DefaultPolicy.ReadOnlyRoot)
	fmt.Fprintf(tw, "  memory_limit_mb:\t%d\n", cfg.DefaultPolicy.MemoryLimitMB)
	fmt.Fprintf(tw, "  cpu_limit_pct:\t%d\n", cfg.DefaultPolicy.CPULimitPct)
	tw.Flush()

	fmt.Fprintf(&sb, "\n%s\n", tr(ctx, "internal_commands_sandbox_const_denylist_header", map[string]any{"Count": len(constDeny)}))
	for _, d := range constDeny {
		fmt.Fprintf(&sb, "  - %s\n", d)
	}

	fmt.Fprintf(&sb, "\n%s\n", tr(ctx, "internal_commands_sandbox_user_denylist_header", map[string]any{"Count": len(userDeny)}))
	if len(userDeny) == 0 {
		sb.WriteString("  " + tr(ctx, "internal_commands_sandbox_user_denylist_empty", nil) + "\n")
	} else {
		for _, d := range userDeny {
			fmt.Fprintf(&sb, "  - %s\n", d)
		}
	}

	return &CommandResult{Success: true, Output: sb.String()}
}

// indent prefixes every non-empty line of `s` with `prefix`. Used to render
// stdout/stderr blocks under their headers in `/sandbox test` output.
func indent(s, prefix string) string {
	if s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	var out strings.Builder
	for i, line := range lines {
		if i == len(lines)-1 && line == "" {
			// Preserve the original trailing newline shape: don't prefix an
			// empty trailing chunk produced by Split on a string that ends
			// in "\n".
			break
		}
		out.WriteString(prefix)
		out.WriteString(line)
		if i < len(lines)-1 {
			out.WriteByte('\n')
		}
	}
	return out.String()
}
