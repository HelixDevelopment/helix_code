package main

import (
	"context"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"dev.helix.code/internal/commands"
)

// commandsCmdDeps wires test seams for the "commands" cobra subcommand group.
type commandsCmdDeps struct {
	Loader   *commands.MarkdownLoader
	Registry *commands.Registry
}

// newCommandsCmd builds the "commands" cobra root with list/show/run/reload
// subcommands. Both the slash-command surface and the cobra subcommand share
// the same MarkdownLoader instance, so they always see the same registry state.
func newCommandsCmd(deps commandsCmdDeps) *cobra.Command {
	root := &cobra.Command{
		Use:   "commands",
		Short: trc("cli_commands_root_short", nil),
	}
	root.AddCommand(newCommandsListCmd(deps))
	root.AddCommand(newCommandsShowCmd(deps))
	root.AddCommand(newCommandsRunCmd(deps))
	root.AddCommand(newCommandsReloadCmd(deps))
	return root
}

func newCommandsListCmd(deps commandsCmdDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: trc("cli_commands_list_short", nil),
		RunE: func(cmd *cobra.Command, args []string) error {
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "NAME\tDESCRIPTION\tSOURCE")
			for name, source := range deps.Loader.Loaded() {
				var desc string
				if c, ok := deps.Registry.Get(name); ok {
					if mc, ok := c.(*commands.MarkdownCommand); ok {
						desc = mc.Description()
					}
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\n", name, desc, source)
			}
			return tw.Flush()
		},
	}
}

func newCommandsShowCmd(deps commandsCmdDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "show <name>",
		Short: trc("cli_commands_show_short", nil),
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			c, ok := deps.Registry.Get(name)
			if !ok {
				return fmt.Errorf("commands show: %q not found", name)
			}
			mc, ok := c.(*commands.MarkdownCommand)
			if !ok {
				return fmt.Errorf("commands show: %q is not a Markdown command", name)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Name: %s\nDescription: %s\nSource: %s\n\n--- Body ---\n%s\n",
				mc.Name(), mc.Description(), mc.SourcePath(), mc.Body())
			return nil
		},
	}
}

func newCommandsRunCmd(deps commandsCmdDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "run <name> [args...]",
		Short: trc("cli_commands_run_short", nil),
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			c, ok := deps.Registry.Get(name)
			if !ok {
				return fmt.Errorf("commands run: %q not found", name)
			}
			cc := &commands.CommandContext{Args: args[1:]}
			res, err := c.Execute(context.Background(), cc)
			if err != nil {
				return err
			}
			out := strings.TrimRight(res.Output, "\n") + "\n"
			fmt.Fprint(cmd.OutOrStdout(), out)
			return nil
		},
	}
}

func newCommandsReloadCmd(deps commandsCmdDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "reload",
		Short: trc("cli_commands_reload_short", nil),
		RunE: func(cmd *cobra.Command, args []string) error {
			before := len(deps.Loader.Loaded())
			if err := deps.Loader.Reload(); err != nil {
				return err
			}
			after := len(deps.Loader.Loaded())
			fmt.Fprintln(cmd.OutOrStdout(),
				trc("cli_commands_reload_result", map[string]any{"Before": before, "After": after}))
			return nil
		},
	}
}
