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
	return "Inspect, reload, or run user-defined Markdown slash commands."
}
func (c *CommandsCommand) Usage() string {
	return "/commands [list|show <name>|reload|run <name> [args...]]"
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
		return c.list(), nil
	case "show":
		if len(args) < 2 {
			return nil, fmt.Errorf("/commands show <name>")
		}
		return c.show(args[1])
	case "reload":
		return c.reload()
	case "run":
		if len(args) < 2 {
			return nil, fmt.Errorf("/commands run <name> [args...]")
		}
		return c.run(ctx, cc, args[1], args[2:])
	default:
		return nil, fmt.Errorf("/commands: unknown subcommand %q (want list|show|reload|run)", sub)
	}
}

// list renders a tab-aligned table of all loaded Markdown commands.
func (c *CommandsCommand) list() *CommandResult {
	var sb strings.Builder
	tw := tabwriter.NewWriter(&sb, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tDESCRIPTION\tSOURCE")
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
func (c *CommandsCommand) show(name string) (*CommandResult, error) {
	cmd, ok := c.registry.Get(name)
	if !ok {
		return nil, fmt.Errorf("/commands show: command %q not found", name)
	}
	mc, ok := cmd.(*MarkdownCommand)
	if !ok {
		return nil, fmt.Errorf("/commands show: %q is not a Markdown command", name)
	}
	out := fmt.Sprintf("Name: %s\nDescription: %s\nSource: %s\n\n--- Body ---\n%s",
		mc.Name(), mc.Description(), mc.SourcePath(), mc.body)
	return &CommandResult{Success: true, Output: out}, nil
}

// reload re-scans the command directories and reconciles the registry.
func (c *CommandsCommand) reload() (*CommandResult, error) {
	before := len(c.loader.Loaded())
	if err := c.loader.Reload(); err != nil {
		return nil, err
	}
	after := len(c.loader.Loaded())
	return &CommandResult{
		Success: true,
		Output:  fmt.Sprintf("commands reload: %d → %d", before, after),
	}, nil
}

// run executes a named Markdown command with the supplied positional args.
func (c *CommandsCommand) run(ctx context.Context, parent *CommandContext, name string, args []string) (*CommandResult, error) {
	cmd, ok := c.registry.Get(name)
	if !ok {
		return nil, fmt.Errorf("/commands run: command %q not found", name)
	}
	innerCC := &CommandContext{
		Args:        args,
		Selection:   parent.Selection,
		CurrentFile: parent.CurrentFile,
	}
	return cmd.Execute(ctx, innerCC)
}
