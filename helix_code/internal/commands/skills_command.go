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
	return tr(context.Background(), "internal_commands_skills_description", nil)
}
func (c *SkillsCommand) Usage() string {
	return tr(context.Background(), "internal_commands_skills_usage", nil)
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
		return c.list(ctx), nil
	case "show":
		if len(args) < 2 {
			return nil, fmt.Errorf("%s", tr(ctx, "internal_commands_skills_show_usage", nil))
		}
		return c.show(ctx, args[1])
	case "reload":
		return c.reload(ctx)
	case "invoke":
		if len(args) < 2 {
			return nil, fmt.Errorf("%s", tr(ctx, "internal_commands_skills_invoke_usage", nil))
		}
		return c.invoke(ctx, cc, args[1], args[2:])
	default:
		return nil, fmt.Errorf("%s", tr(ctx, "internal_commands_skills_unknown_subcommand", map[string]any{"Sub": sub}))
	}
}

// list renders a tab-aligned table of all loaded skills.
func (c *SkillsCommand) list(ctx context.Context) *CommandResult {
	var sb strings.Builder
	tw := tabwriter.NewWriter(&sb, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, tr(ctx, "internal_commands_skills_table_header", nil))
	for _, s := range c.registry.List() {
		patterns := strings.Join(s.TriggerPatterns(), " | ")
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", s.Name(), s.Description(), patterns, s.SourcePath())
	}
	tw.Flush()
	return &CommandResult{Success: true, Output: sb.String()}
}

// show returns the raw body and metadata of a named skill.
func (c *SkillsCommand) show(ctx context.Context, name string) (*CommandResult, error) {
	s, ok := c.registry.Get(name)
	if !ok {
		return nil, fmt.Errorf("%s", tr(ctx, "internal_commands_skills_show_not_found", map[string]any{"Name": name}))
	}
	out := tr(ctx, "internal_commands_skills_show_detail", map[string]any{
		"Name":              s.Name(),
		"Description":       s.Description(),
		"Source":            s.SourcePath(),
		"RequiresIsolation": s.RequiresIsolation(),
		"Triggers":          strings.Join(s.TriggerPatterns(), "\n  "),
		"Body":              s.Body(),
	})
	return &CommandResult{Success: true, Output: out}, nil
}

// reload re-scans the skill directories and reconciles the registry.
func (c *SkillsCommand) reload(ctx context.Context) (*CommandResult, error) {
	before := len(c.loader.Loaded())
	if err := c.loader.Reload(); err != nil {
		return nil, err
	}
	after := len(c.loader.Loaded())
	return &CommandResult{
		Success: true,
		Output:  tr(ctx, "internal_commands_skills_reload_result", map[string]any{"Before": before, "After": after}),
	}, nil
}

// invoke renders a named skill with the supplied positional args.
func (c *SkillsCommand) invoke(ctx context.Context, parent *CommandContext, name string, args []string) (*CommandResult, error) {
	s, ok := c.registry.Get(name)
	if !ok {
		return nil, fmt.Errorf("%s", tr(ctx, "internal_commands_skills_invoke_not_found", map[string]any{"Name": name}))
	}
	rendered, err := s.Render(args, parent.Selection, parent.CurrentFile)
	if err != nil {
		return nil, err
	}
	return &CommandResult{Success: true, Output: rendered}, nil
}
