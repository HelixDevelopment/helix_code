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

func (c *HooksCommand) Name() string        { return "hooks" }
func (c *HooksCommand) Aliases() []string   { return []string{"hk"} }
func (c *HooksCommand) Description() string { return "manage hook scripts" }
func (c *HooksCommand) Usage() string       { return "/hooks [list | test <event>]" }

func (c *HooksCommand) Execute(ctx context.Context, cmdCtx *CommandContext) (*CommandResult, error) {
	if len(cmdCtx.Args) == 0 {
		return c.list()
	}
	switch cmdCtx.Args[0] {
	case "list":
		return c.list()
	case "test":
		if len(cmdCtx.Args) < 2 {
			// CONST-046 (round-149): usage hint resolved
			// through the package-level translator.
			return nil, fmt.Errorf("%s", tr(ctx, "internal_commands_usage_hooks_test", nil))
		}
		return c.test(ctx, cmdCtx.Args[1])
	default:
		return nil, fmt.Errorf("unknown /hooks subaction %q (valid: list, test)", cmdCtx.Args[0])
	}
}

func (c *HooksCommand) list() (*CommandResult, error) {
	all := c.mgr.GetAll()
	var buf bytes.Buffer
	tw := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "ID\tEVENT\tPRIORITY\tASYNC\tENABLED\n")
	for _, h := range all {
		fmt.Fprintf(tw, "%s\t%s\t%d\t%v\t%v\n", h.ID, h.Type, h.Priority, h.Async, h.Enabled)
	}
	if len(all) == 0 {
		fmt.Fprintln(tw, "(no hooks registered)\t\t\t\t")
	}
	tw.Flush()
	return &CommandResult{Output: buf.String(), Success: true}, nil
}

func (c *HooksCommand) test(ctx context.Context, eventName string) (*CommandResult, error) {
	event := hooks.NewEventWithContext(ctx, hooks.HookType(eventName))
	results := c.mgr.TriggerEventAndWait(event)
	var buf bytes.Buffer
	for _, r := range results {
		fmt.Fprintf(&buf, "%s: status=%s err=%v duration=%s\n", r.HookID, r.Status, r.Error, r.Duration)
	}
	if len(results) == 0 {
		fmt.Fprintf(&buf, "(no hooks registered for event %q)\n", eventName)
	}
	return &CommandResult{Output: buf.String(), Success: true}, nil
}
