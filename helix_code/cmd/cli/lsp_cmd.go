package main

// lsp_cmd.go (P1-F13-T10): cobra subcommand surface for `helixcode lsp`.
//
// The `helixcode lsp` cobra subcommand is a thin operator-facing wrapper
// around the same renderer that powers the /lsp slash command (T09). The
// underlying LSPCommand is constructed with the same LSPManager and curated
// allowlist as the slash command, so both surfaces produce identical output
// for status / list-servers / restart / stop. This avoids any chance of
// drift between the slash and cobra surfaces and keeps the table-rendering
// logic in exactly one place (internal/commands/lsp_command.go).
//
// Anti-bluff anchor: this file performs no simulation. status / list-servers
// query the live LSPManager.Servers() and exec.LookPath; restart / stop call
// LSPManager.Restart / LSPManager.Stop directly. There is no in-memory
// shadow state and no stand-in stub logic.

import (
	"fmt"

	"github.com/spf13/cobra"

	"dev.helix.code/internal/commands"
	"dev.helix.code/internal/tools"
)

// lspCmdDeps wires test seams for the "lsp" cobra subcommand group. It
// mirrors sessionsCmdDeps / wizardCmdDeps: a manager (real *tools.LSPManager
// in production, satisfies commands.LSPManager structurally) plus the
// curated allowlist so list-servers always sees the canonical 5 entries.
type lspCmdDeps struct {
	Manager      commands.LSPManager
	CuratedSpecs []tools.LSPServerSpec
}

// newLSPCmd builds the `helixcode lsp` cobra root with status, list-servers,
// restart, and stop subcommands. The default action when no subcommand is
// supplied is `status`, matching the operator's most common entry-point
// question once at least one server is running. (The /lsp slash command
// defaults to list-servers; the cobra surface defaults to status because
// CLI invocation typically follows a session in which servers were already
// spawned by file-open events.)
func newLSPCmd(deps lspCmdDeps) *cobra.Command {
	root := &cobra.Command{
		Use:   "lsp",
		Short: "Inspect, list, restart, or stop managed LSP servers",
		Long: "Operator surface for the LSPManager. Subcommands:\n" +
			"  status         show running servers (default when no subcommand supplied)\n" +
			"  list-servers   show curated allowlist with on-path + running annotations\n" +
			"  restart <name> tear down the named server (next file-open respawns it)\n" +
			"  stop <name>    stop the named server without auto-respawn",
		// Default subcommand: when invoked as `helixcode lsp` with no args,
		// run status. cobra resolves this via RunE on the root command.
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLSPSubcommand(cmd, deps, "status", nil)
		},
	}
	root.AddCommand(newLSPStatusCmd(deps))
	root.AddCommand(newLSPListServersCmd(deps))
	root.AddCommand(newLSPRestartCmd(deps))
	root.AddCommand(newLSPStopCmd(deps))
	return root
}

func newLSPStatusCmd(deps lspCmdDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show currently-running LSP servers in a tab-aligned table",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLSPSubcommand(cmd, deps, "status", nil)
		},
	}
}

func newLSPListServersCmd(deps lspCmdDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "list-servers",
		Short: "Show curated server allowlist with on-path + running annotations",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLSPSubcommand(cmd, deps, "list-servers", nil)
		},
	}
}

func newLSPRestartCmd(deps lspCmdDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "restart <name>",
		Short: "Restart the named server (next matching file open will respawn it)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLSPSubcommand(cmd, deps, "restart", args)
		},
	}
}

func newLSPStopCmd(deps lspCmdDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "stop <name>",
		Short: "Stop the named server without auto-respawn",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLSPSubcommand(cmd, deps, "stop", args)
		},
	}
}

// runLSPSubcommand delegates to the same internal/commands.LSPCommand that
// powers /lsp. The renderer is shared so both surfaces always produce the
// same output. Errors from the underlying Execute are surfaced as cobra
// errors (non-zero exit). Successful results are printed verbatim to the
// command's stdout (which tests can override via cmd.SetOut).
func runLSPSubcommand(cmd *cobra.Command, deps lspCmdDeps, sub string, extra []string) error {
	if deps.Manager == nil {
		return fmt.Errorf("lsp: manager not configured (call newLSPCmd with a non-nil Manager)")
	}
	args := []string{sub}
	args = append(args, extra...)

	lsp := commands.NewLSPCommand(deps.Manager, deps.CuratedSpecs)
	res, err := lsp.Execute(cmd.Context(), &commands.CommandContext{Args: args})
	if err != nil {
		return err
	}
	if res != nil && res.Output != "" {
		fmt.Fprint(cmd.OutOrStdout(), res.Output)
		// Ensure trailing newline for terminal-friendly output. tabwriter
		// emits a final \n per row, but the empty-state ("no servers
		// running") string does not, so add one if missing.
		if last := res.Output[len(res.Output)-1]; last != '\n' {
			fmt.Fprintln(cmd.OutOrStdout())
		}
	}
	return nil
}

