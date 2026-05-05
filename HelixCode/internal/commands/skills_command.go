package commands

import (
	"context"
	"fmt"
	"strings"
	"text/tabwriter"
)

// SkillsCommand implements the /skills slash command for inspecting,
// reloading, and invoking agent-loaded Markdown skills.
type SkillsCommand struct {
	loader   *SkillLoader
	registry *SkillRegistry
}

// NewSkillsCommand returns a /skills slash command bound to the supplied
// loader and registry.
func NewSkillsCommand(loader *SkillLoader, registry *SkillRegistry) *SkillsCommand {
	return &SkillsCommand{loader: loader, registry: registry}
}

func (c *SkillsCommand) Name() string      { return "skills" }
func (c *SkillsCommand) Aliases() []string { return nil }
func (c *SkillsCommand) Description() string {
	return "Inspect, reload, or invoke agent-loaded skills."
}
func (c *SkillsCommand) Usage() string {
	return "/skills [list|show <name>|reload|invoke <name> [args...]]"
}

// Execute dispatches to the appropriate subcommand handler.
func (c *SkillsCommand) Execute(ctx context.Context, cc *CommandContext) (*CommandResult, error) {
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
			return nil, fmt.Errorf("/skills show <name>")
		}
		return c.show(args[1])
	case "reload":
		return c.reload()
	case "invoke":
		if len(args) < 2 {
			return nil, fmt.Errorf("/skills invoke <name> [args...]")
		}
		return c.invoke(ctx, cc, args[1], args[2:])
	default:
		return nil, fmt.Errorf("/skills: unknown subcommand %q (want list|show|reload|invoke)", sub)
	}
}

// list renders a tab-aligned table of all loaded skills.
func (c *SkillsCommand) list() *CommandResult {
	var sb strings.Builder
	tw := tabwriter.NewWriter(&sb, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tDESCRIPTION\tTRIGGERS\tSOURCE")
	for _, s := range c.registry.List() {
		patterns := strings.Join(s.TriggerPatterns(), " | ")
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", s.Name(), s.Description(), patterns, s.SourcePath())
	}
	tw.Flush()
	return &CommandResult{Success: true, Output: sb.String()}
}

// show returns the raw body and metadata of a named skill.
func (c *SkillsCommand) show(name string) (*CommandResult, error) {
	s, ok := c.registry.Get(name)
	if !ok {
		return nil, fmt.Errorf("/skills show: skill %q not found", name)
	}
	out := fmt.Sprintf(
		"Name: %s\nDescription: %s\nSource: %s\nRequires isolation: %t\nTriggers:\n  %s\n\n--- Body ---\n%s",
		s.Name(), s.Description(), s.SourcePath(), s.RequiresIsolation(),
		strings.Join(s.TriggerPatterns(), "\n  "), s.Body())
	return &CommandResult{Success: true, Output: out}, nil
}

// reload re-scans the skill directories and reconciles the registry.
func (c *SkillsCommand) reload() (*CommandResult, error) {
	before := len(c.loader.Loaded())
	if err := c.loader.Reload(); err != nil {
		return nil, err
	}
	after := len(c.loader.Loaded())
	return &CommandResult{
		Success: true,
		Output:  fmt.Sprintf("skills reload: %d → %d", before, after),
	}, nil
}

// invoke renders a named skill with the supplied positional args.
func (c *SkillsCommand) invoke(ctx context.Context, parent *CommandContext, name string, args []string) (*CommandResult, error) {
	s, ok := c.registry.Get(name)
	if !ok {
		return nil, fmt.Errorf("/skills invoke: skill %q not found", name)
	}
	rendered, err := s.Render(args, parent.Selection, parent.CurrentFile)
	if err != nil {
		return nil, err
	}
	return &CommandResult{Success: true, Output: rendered}, nil
}
