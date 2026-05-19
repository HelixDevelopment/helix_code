package main

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"dev.helix.code/internal/commands"
)

// skillsCmdDeps wires test seams for the "skills" cobra subcommand group.
type skillsCmdDeps struct {
	Loader   *commands.SkillLoader
	Registry *commands.SkillRegistry
}

// newSkillsCmd builds the "skills" cobra root with list/show/invoke/reload
// subcommands. Both the slash-command surface and the cobra subcommand share
// the same SkillLoader instance, so they always see the same registry state.
func newSkillsCmd(deps skillsCmdDeps) *cobra.Command {
	root := &cobra.Command{
		Use:   "skills",
		Short: trc("cli_skills_root_short", nil),
	}
	root.AddCommand(newSkillsListCmd(deps))
	root.AddCommand(newSkillsShowCmd(deps))
	root.AddCommand(newSkillsInvokeCmd(deps))
	root.AddCommand(newSkillsReloadCmd(deps))
	return root
}

func newSkillsListCmd(deps skillsCmdDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: trc("cli_skills_list_short", nil),
		RunE: func(cmd *cobra.Command, args []string) error {
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "NAME\tDESCRIPTION\tTRIGGERS\tSOURCE")
			for _, s := range deps.Registry.List() {
				patterns := strings.Join(s.TriggerPatterns(), " | ")
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", s.Name(), s.Description(), patterns, s.SourcePath())
			}
			return tw.Flush()
		},
	}
}

func newSkillsShowCmd(deps skillsCmdDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "show <name>",
		Short: trc("cli_skills_show_short", nil),
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			s, ok := deps.Registry.Get(name)
			if !ok {
				return fmt.Errorf("skills show: %q not found", name)
			}
			fmt.Fprintf(cmd.OutOrStdout(),
				"%s\nTriggers:\n  %s\n\n--- Body ---\n%s\n",
				trc("cli_skills_show_header", map[string]any{
					"Name":              s.Name(),
					"Description":       s.Description(),
					"Source":            s.SourcePath(),
					"RequiresIsolation": s.RequiresIsolation(),
				}),
				strings.Join(s.TriggerPatterns(), "\n  "), s.Body())
			return nil
		},
	}
}

func newSkillsInvokeCmd(deps skillsCmdDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "invoke <name> [args...]",
		Short: trc("cli_skills_invoke_short", nil),
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			s, ok := deps.Registry.Get(name)
			if !ok {
				return fmt.Errorf("skills invoke: %q not found", name)
			}
			rendered, err := s.Render(args[1:], "", "")
			if err != nil {
				return err
			}
			out := strings.TrimRight(rendered, "\n") + "\n"
			fmt.Fprint(cmd.OutOrStdout(), out)
			return nil
		},
	}
}

func newSkillsReloadCmd(deps skillsCmdDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "reload",
		Short: trc("cli_skills_reload_short", nil),
		RunE: func(cmd *cobra.Command, args []string) error {
			before := len(deps.Loader.Loaded())
			if err := deps.Loader.Reload(); err != nil {
				return err
			}
			after := len(deps.Loader.Loaded())
			fmt.Fprintln(cmd.OutOrStdout(),
				trc("cli_skills_reload_result", map[string]any{"Before": before, "After": after}))
			return nil
		},
	}
}
