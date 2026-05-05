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
		Short: "Inspect, invoke, or reload agent-loaded skills",
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
		Short: "List loaded skills",
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
		Short: "Show metadata + body of a skill",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			s, ok := deps.Registry.Get(name)
			if !ok {
				return fmt.Errorf("skills show: %q not found", name)
			}
			fmt.Fprintf(cmd.OutOrStdout(),
				"Name: %s\nDescription: %s\nSource: %s\nRequires isolation: %t\nTriggers:\n  %s\n\n--- Body ---\n%s\n",
				s.Name(), s.Description(), s.SourcePath(), s.RequiresIsolation(),
				strings.Join(s.TriggerPatterns(), "\n  "), s.Body())
			return nil
		},
	}
}

func newSkillsInvokeCmd(deps skillsCmdDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "invoke <name> [args...]",
		Short: "Render a skill body to stdout (bypassing trigger matching)",
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
		Short: "Re-scan project + user skill directories",
		RunE: func(cmd *cobra.Command, args []string) error {
			before := len(deps.Loader.Loaded())
			if err := deps.Loader.Reload(); err != nil {
				return err
			}
			after := len(deps.Loader.Loaded())
			fmt.Fprintf(cmd.OutOrStdout(), "skills reload: %d → %d\n", before, after)
			return nil
		},
	}
}
