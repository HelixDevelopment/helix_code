package commands

import (
	"bytes"
	"context"
	"fmt"
	"text/tabwriter"

	"dev.helix.code/internal/hooks"
)

// HooksCommand implements /hooks.
//
// Subactions:
//
//	/hooks                          — list (default)
//	/hooks list                     — explicit list
//	/hooks test <event-name>        — fire all hooks for the event
type HooksCommand struct {
	mgr *hooks.Manager
}

// NewHooksCommand wires a hooks.Manager into the slash command.
func NewHooksCommand(mgr *hooks.Manager) *HooksCommand {
	return &HooksCommand{mgr: mgr}
}

func (c *HooksCommand) Name() string      { return "hooks" }
func (c *HooksCommand) Aliases() []string { return []string{"hk"} }
func (c *HooksCommand) Description() string {
	return tr(context.Background(), "internal_commands_hooks_description", nil)
}
func (c *HooksCommand) Usage() string {
	return tr(context.Background(), "internal_commands_hooks_usage", nil)
}

func (c *HooksCommand) Execute(ctx context.Context, cmdCtx *CommandContext) (*CommandResult, error) {
	if len(cmdCtx.Args) == 0 {
		return c.list(ctx)
	}
	switch cmdCtx.Args[0] {
	case "list":
		return c.list(ctx)
	case "test":
		if len(cmdCtx.Args) < 2 {
			// CONST-046 (round-149): usage hint resolved
			// through the package-level translator.
			return nil, fmt.Errorf("%s", tr(ctx, "internal_commands_usage_hooks_test", nil))
		}
		return c.test(ctx, cmdCtx.Args[1])
	default:
		// CONST-046 (round-416): operator error resolved
		// through the package-level translator.
		return nil, fmt.Errorf("%s", tr(ctx, "internal_commands_hooks_unknown_subaction",
			map[string]any{"Sub": cmdCtx.Args[0]}))
	}
}

func (c *HooksCommand) list(ctx context.Context) (*CommandResult, error) {
	all := c.mgr.GetAll()
	var buf bytes.Buffer
	tw := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
		tr(ctx, "internal_commands_hooks_col_id", nil),
		tr(ctx, "internal_commands_hooks_col_event", nil),
		tr(ctx, "internal_commands_hooks_col_priority", nil),
		tr(ctx, "internal_commands_hooks_col_async", nil),
		tr(ctx, "internal_commands_hooks_col_enabled", nil))
	for _, h := range all {
		fmt.Fprintf(tw, "%s\t%s\t%d\t%v\t%v\n", h.ID, h.Type, h.Priority, h.Async, h.Enabled)
	}
	if len(all) == 0 {
		fmt.Fprintf(tw, "%s\t\t\t\t\n", tr(ctx, "internal_commands_hooks_none_registered", nil))
	}
	tw.Flush()
	return &CommandResult{Output: buf.String(), Success: true}, nil
}

func (c *HooksCommand) test(ctx context.Context, eventName string) (*CommandResult, error) {
	event := hooks.NewEventWithContext(ctx, hooks.HookType(eventName))
	results := c.mgr.TriggerEventAndWait(event)
	var buf bytes.Buffer
	for _, r := range results {
		fmt.Fprintln(&buf, tr(ctx, "internal_commands_hooks_test_result", map[string]any{
			"HookID":   r.HookID,
			"Status":   r.Status,
			"Error":    r.Error,
			"Duration": r.Duration,
		}))
	}
	if len(results) == 0 {
		fmt.Fprintln(&buf, tr(ctx, "internal_commands_hooks_none_for_event",
			map[string]any{"Event": eventName}))
	}
	return &CommandResult{Output: buf.String(), Success: true}, nil
}
