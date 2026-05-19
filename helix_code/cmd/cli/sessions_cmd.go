package main

import (
	"context"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"dev.helix.code/internal/session"
)

// sessionsCmdDeps wires test seams for the "sessions" cobra subcommand group.
type sessionsCmdDeps struct {
	Store          session.SessionStore
	CurrentProject string
}

// newSessionsCmd builds the "sessions" cobra root with list/show/delete
// subcommands. It shares the same SessionStore instance as the /sessions slash
// command so both surfaces always see the same state.
func newSessionsCmd(deps sessionsCmdDeps) *cobra.Command {
	root := &cobra.Command{
		Use:   "sessions",
		Short: trc("cli_sessions_root_short", nil),
	}
	root.AddCommand(newSessionsListCmd(deps))
	root.AddCommand(newSessionsShowCmd(deps))
	root.AddCommand(newSessionsDeleteCmd(deps))
	return root
}

func newSessionsListCmd(deps sessionsCmdDeps) *cobra.Command {
	var allFlag bool
	c := &cobra.Command{
		Use:   "list",
		Short: trc("cli_sessions_list_short", nil),
		RunE: func(cmd *cobra.Command, args []string) error {
			scope := deps.CurrentProject
			if allFlag {
				scope = ""
			}
			metas, err := deps.Store.ListSessionMetadata(context.Background(), scope)
			if err != nil {
				return err
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "ID\tPROJECT\tSTARTED\tLAST-ACTIVITY\tMSG-COUNT")
			for _, m := range metas {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%d\n",
					m.SessionID, m.ProjectName,
					m.StartedAt.Format("2006-01-02 15:04:05"),
					m.LastActivity.Format("2006-01-02 15:04:05"),
					m.MessageCount)
			}
			return tw.Flush()
		},
	}
	c.Flags().BoolVar(&allFlag, "all", false, "list all sessions across projects")
	return c
}

func newSessionsShowCmd(deps sessionsCmdDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: trc("cli_sessions_show_short", nil),
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			meta, err := deps.Store.GetSessionMetadata(ctx, args[0])
			if err != nil {
				return err
			}
			msgs, _ := deps.Store.ReadTranscript(ctx, args[0])
			fmt.Fprintf(cmd.OutOrStdout(), "%s\n\n--- Transcript (last 20) ---\n",
				tr(ctx, "cli_sessions_show_header", map[string]any{
					"ID":           meta.SessionID,
					"Project":      meta.ProjectName,
					"Path":         meta.ProjectPath,
					"Started":      meta.StartedAt.Format("2006-01-02 15:04:05"),
					"LastActivity": meta.LastActivity.Format("2006-01-02 15:04:05"),
					"MessageCount": meta.MessageCount,
				}))
			start := 0
			if len(msgs) > 20 {
				start = len(msgs) - 20
			}
			for _, m := range msgs[start:] {
				fmt.Fprintf(cmd.OutOrStdout(), "[%s] %s\n", m.Role, strings.TrimSpace(m.Content))
			}
			return nil
		},
	}
}

func newSessionsDeleteCmd(deps sessionsCmdDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: trc("cli_sessions_delete_short", nil),
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := deps.Store.DeleteSession(context.Background(), args[0]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "deleted session %s\n", args[0])
			return nil
		},
	}
}
