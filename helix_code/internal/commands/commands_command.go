package commands

import (
	"context"
	"fmt"
	"strings"
	"text/tabwriter"
)

// CommandsCommand implements the /commands slash command for inspecting,
// reloading, and running user-defined Markdown slash commands.
type CommandsCommand struct {
	loader   *MarkdownLoader
	registry *Registry
}

// NewCommandsCommand returns a /commands slash command bound to the supplied
// loader and registry.
func NewCommandsCommand(loader *MarkdownLoader, registry *Registry) *CommandsCommand {
	return &CommandsCommand{loader: loader, registry: registry}
}

func (c *CommandsCommand) Name() string      { return "commands" }
func (c *CommandsCommand) Aliases() []string { return nil }
func (c *CommandsCommand) Description() string {
	return tr(context.Background(), "internal_commands_commands_description", nil)
}
func (c *CommandsCommand) Usage() string {
	return tr(context.Background(), "internal_commands_commands_usage", nil)
}

// Execute dispatches to the appropriate subcommand handler.
func (c *CommandsCommand) Execute(ctx context.Context, cc *CommandContext) (*CommandResult, error) {
	args := cc.Args
	sub := "list"
	if len(args) > 0 {
		sub = args[0]
	}
	switch sub {
	case "list":
		return c.list(ctx), nil
	case "show":
		if len(args) < 2 {
			return nil, fmt.Errorf("%s", tr(ctx, "internal_commands_commands_show_usage", nil))
		}
		return c.show(ctx, args[1])
	case "reload":
		return c.reload(ctx)
	case "run":
		if len(args) < 2 {
			return nil, fmt.Errorf("%s", tr(ctx, "internal_commands_commands_run_usage", nil))
		}
		return c.run(ctx, cc, args[1], args[2:])
	default:
		return nil, fmt.Errorf("%s", tr(ctx, "internal_commands_commands_unknown_subcommand", map[string]any{"Sub": sub}))
	}
}

// list renders a tab-aligned table of all loaded Markdown commands.
func (c *CommandsCommand) list(ctx context.Context) *CommandResult {
	var sb strings.Builder
	tw := tabwriter.NewWriter(&sb, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, tr(ctx, "internal_commands_commands_table_header", nil))
	for name, source := range c.loader.Loaded() {
		var desc string
		if cmd, ok := c.registry.Get(name); ok {
			if mc, ok := cmd.(*MarkdownCommand); ok {
				desc = mc.Description()
			}
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\n", name, desc, source)
	}
	tw.Flush()
	return &CommandResult{Success: true, Output: sb.String()}
}

// show returns the raw body and metadata of a named Markdown command.
func (c *CommandsCommand) show(ctx context.Context, name string) (*CommandResult, error) {
	cmd, ok := c.registry.Get(name)
	if !ok {
		return nil, fmt.Errorf("%s", tr(ctx, "internal_commands_commands_show_not_found", map[string]any{"Name": name}))
	}
	mc, ok := cmd.(*MarkdownCommand)
	if !ok {
		return nil, fmt.Errorf("%s", tr(ctx, "internal_commands_commands_show_not_markdown", map[string]any{"Name": name}))
	}
	out := tr(ctx, "internal_commands_commands_show_detail", map[string]any{
		"Name":        mc.Name(),
		"Description": mc.Description(),
		"Source":      mc.SourcePath(),
		"Body":        mc.body,
	})
	return &CommandResult{Success: true, Output: out}, nil
}

// reload re-scans the command directories and reconciles the registry.
func (c *CommandsCommand) reload(ctx context.Context) (*CommandResult, error) {
	before := len(c.loader.Loaded())
	if err := c.loader.Reload(); err != nil {
		return nil, err
	}
	after := len(c.loader.Loaded())
	return &CommandResult{
		Success: true,
		Output:  tr(ctx, "internal_commands_commands_reload_result", map[string]any{"Before": before, "After": after}),
	}, nil
}

// run executes a named Markdown command with the supplied positional args.
func (c *CommandsCommand) run(ctx context.Context, parent *CommandContext, name string, args []string) (*CommandResult, error) {
	cmd, ok := c.registry.Get(name)
	if !ok {
		return nil, fmt.Errorf("%s", tr(ctx, "internal_commands_commands_run_not_found", map[string]any{"Name": name}))
	}
	innerCC := &CommandContext{
		Args:        args,
		Selection:   parent.Selection,
		CurrentFile: parent.CurrentFile,
	}
	return cmd.Execute(ctx, innerCC)
}
