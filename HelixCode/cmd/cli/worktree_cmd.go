package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"dev.helix.code/internal/tools/worktree"
)

func newWorktreeCommand(m *worktree.Manager) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "worktree",
		Short: "Manage HelixCode-tracked git worktrees",
	}
	cmd.AddCommand(newWorktreeListCommand(m))
	cmd.AddCommand(newWorktreeEnterCommand())
	cmd.AddCommand(newWorktreeExitCommand())
	cmd.AddCommand(newWorktreeRemoveCommand(m))
	return cmd
}

func newWorktreeListCommand(m *worktree.Manager) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List helix-managed worktrees under .helix-worktrees/",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWorktreeList(os.Stdout, m)
		},
	}
}

func newWorktreeRemoveCommand(m *worktree.Manager) *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a helix-managed worktree",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWorktreeRemove(m, args[0])
		},
	}
}

func newWorktreeEnterCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "enter <name> [base-branch]",
		Short: "(stateful) Use from inside a `helixcode chat` session, not the CLI",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			baseBranch := ""
			if len(args) >= 2 {
				baseBranch = args[1]
			}
			return runWorktreeEnter(os.Stdout, args[0], baseBranch)
		},
	}
}

func newWorktreeExitCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "exit",
		Short: "(stateful) Use from inside a `helixcode chat` session, not the CLI",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWorktreeExit(os.Stdout)
		},
	}
}

func runWorktreeList(out io.Writer, m *worktree.Manager) error {
	wts, err := m.ListWorktrees(context.Background())
	if err != nil {
		return err
	}
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "NAME\tBRANCH\tPATH\n")
	for _, w := range wts {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", w.Name, w.Branch, w.Path)
	}
	if len(wts) == 0 {
		fmt.Fprintln(tw, "(no worktrees)\t\t")
	}
	return tw.Flush()
}

func runWorktreeRemove(m *worktree.Manager, name string) error {
	return m.RemoveWorktree(context.Background(), name)
}

func runWorktreeEnter(out io.Writer, name, baseBranch string) error {
	fmt.Fprintln(out, "`helixcode worktree enter` is a stateful operation.")
	fmt.Fprintln(out, "Run it from inside a `helixcode chat` session via the agent's EnterWorktree tool")
	fmt.Fprintln(out, "or the /worktree slash command. The CLI subcommand cannot persist worktree state across invocations.")
	return fmt.Errorf("stateful subcommand: use from inside helixcode chat")
}

func runWorktreeExit(out io.Writer) error {
	fmt.Fprintln(out, "`helixcode worktree exit` is a stateful operation.")
	fmt.Fprintln(out, "Run it from inside a `helixcode chat` session via the agent's ExitWorktree tool")
	fmt.Fprintln(out, "or the /worktree slash command. The CLI subcommand cannot persist worktree state across invocations.")
	return fmt.Errorf("stateful subcommand: use from inside helixcode chat")
}
