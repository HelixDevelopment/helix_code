// Package commands — subagents_command.go (P1-F15-T09).
//
// SubagentsCommand implements the /subagents slash command with three
// subcommands: list (default), status, kill <id>. It is the user-facing
// surface for observing and terminating running subagents (F15).
//
// Subcommands:
//
//	/subagents              alias of /subagents list
//	/subagents list         table of running subagents:
//	                        ID / DESCRIPTION / ISOLATION / ELAPSED
//	/subagents status       same rows as list with extra STARTED-AT column
//	/subagents kill <id>    delegates to SubagentManager.Kill
//
// Why slash-only (no cobra): per spec §6 (Q5=B), subagent observation is
// always interactive — the user is in the chat session that dispatched
// them. There is no headless / scripted use case, so the cobra surface
// would be dead weight.
//
// CONST-042 anchor (No-Secret-Leak):
//
//	SubagentsCommand intentionally renders subagent.SubagentStatus and
//	NOTHING else. SubagentStatus (defined in internal/agent/subagent/
//	manager.go) deliberately has NO Prompt field — only ID, Description,
//	Isolation, StartedAt, Elapsed. That structural absence is the
//	anti-leak guarantee: if a future change adds a Prompt-shaped field
//	to SubagentStatus, the reflection test
//	TestSubagentsCommand_StatusStructHasNoPromptField fails immediately,
//	blocking the leak at the type level rather than at the rendering
//	level. We do not log full prompts in this package.
package commands

import (
	"context"
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	"dev.helix.code/internal/agent/subagent"
)

// SubagentManager is the subset of *subagent.SubagentManager that
// SubagentsCommand depends on.
//
// Defining the interface in the commands package keeps the slash command
// testable with a fake while letting main.go pass the real
// *subagent.SubagentManager directly (Go satisfies interfaces structurally).
//
// Deliberately narrow: only Status() and Kill() are exposed here. The
// slash command is request-response and never consumes the streaming
// Results() channel — that belongs to the dispatcher (T07/T08), not the
// observer surface.
type SubagentManager interface {
	Status() []subagent.SubagentStatus
	Kill(id string) error
}

// SubagentsCommand is the /subagents slash command.
type SubagentsCommand struct {
	manager SubagentManager
}

// NewSubagentsCommand constructs the /subagents slash command.
func NewSubagentsCommand(m SubagentManager) *SubagentsCommand {
	return &SubagentsCommand{manager: m}
}

// Name returns the slash command name (without the leading slash).
func (c *SubagentsCommand) Name() string { return "subagents" }

// Aliases returns alternative invocation names. /subagents has none.
func (c *SubagentsCommand) Aliases() []string { return nil }

// Description returns the one-line help blurb shown by /help.
func (c *SubagentsCommand) Description() string {
	return tr(context.Background(), "internal_commands_subagents_description", nil)
}

// Usage returns the usage string shown by /help.
func (c *SubagentsCommand) Usage() string {
	return tr(context.Background(), "internal_commands_subagents_usage", nil)
}

// Execute dispatches to the appropriate subcommand handler.
//
// The default subcommand (no args) is `list` — it answers "which
// subagents are currently running" which is the most common entry-point
// question. /subagents status is the same table with the StartedAt
// column added, useful when the operator wants timestamps.
func (c *SubagentsCommand) Execute(ctx context.Context, cc *CommandContext) (*CommandResult, error) {
	args := cc.Args
	sub := "list"
	if len(args) > 0 {
		sub = args[0]
	}
	switch sub {
	case "list":
		return c.handleList(ctx), nil
	case "status":
		return c.handleStatus(ctx), nil
	case "kill":
		if len(args) < 2 {
			return nil, fmt.Errorf("/subagents kill <id>")
		}
		return c.handleKill(ctx, args[1])
	default:
		return nil, fmt.Errorf("/subagents: unknown subcommand %q (want list|status|kill)", sub)
	}
}

// handleList renders the running-subagents table without timestamps.
//
// Columns: ID / DESCRIPTION / ISOLATION / ELAPSED. ELAPSED is rounded to
// the nearest second so the table doesn't churn with sub-second jitter.
func (c *SubagentsCommand) handleList(ctx context.Context) *CommandResult {
	statuses := c.manager.Status()
	if len(statuses) == 0 {
		return &CommandResult{Success: true, Output: tr(ctx, "internal_commands_subagents_none_running", nil)}
	}

	var sb strings.Builder
	tw := tabwriter.NewWriter(&sb, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tDESCRIPTION\tISOLATION\tELAPSED")
	for _, s := range statuses {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
			s.ID,
			s.Description,
			string(s.Isolation),
			s.Elapsed.Round(time.Second).String(),
		)
	}
	tw.Flush()
	return &CommandResult{Success: true, Output: sb.String()}
}

// handleStatus renders the running-subagents table with the extra
// STARTED-AT column.
//
// Columns: ID / DESCRIPTION / ISOLATION / STARTED-AT / ELAPSED. The
// timestamp is formatted in RFC3339 (UTC) so it sorts lexically and
// remains parseable by downstream tooling.
func (c *SubagentsCommand) handleStatus(ctx context.Context) *CommandResult {
	statuses := c.manager.Status()
	if len(statuses) == 0 {
		return &CommandResult{Success: true, Output: tr(ctx, "internal_commands_subagents_none_running", nil)}
	}

	var sb strings.Builder
	tw := tabwriter.NewWriter(&sb, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tDESCRIPTION\tISOLATION\tSTARTED-AT\tELAPSED")
	for _, s := range statuses {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
			s.ID,
			s.Description,
			string(s.Isolation),
			s.StartedAt.UTC().Format(time.RFC3339),
			s.Elapsed.Round(time.Second).String(),
		)
	}
	tw.Flush()
	return &CommandResult{Success: true, Output: sb.String()}
}

// handleKill asks the manager to cancel the subagent with the given ID.
// The manager's Kill is non-blocking — it cancels the per-subagent
// context and returns; the spawner observes the cancellation and emits a
// StateCanceled result on the aggregator.
//
// On error (typically ID not found) we wrap the manager's error in a
// /subagents-prefixed message so the user knows which surface failed.
func (c *SubagentsCommand) handleKill(ctx context.Context, id string) (*CommandResult, error) {
	if err := c.manager.Kill(id); err != nil {
		return nil, fmt.Errorf("/subagents kill %s: %w", id, err)
	}
	return &CommandResult{
		Success: true,
		Output:  tr(ctx, "internal_commands_subagents_killed", map[string]any{"ID": id}),
	}, nil
}
