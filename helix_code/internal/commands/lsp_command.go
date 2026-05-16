package commands

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"text/tabwriter"

	"dev.helix.code/internal/tools"
)

// LSPManager is the subset of *tools.LSPManager that LSPCommand depends on.
//
// Defining the interface in the commands package keeps the slash command
// testable with a fake while still letting main.go pass the real
// *tools.LSPManager directly (Go satisfies interfaces structurally).
type LSPManager interface {
	Servers() []tools.ServerInfo
	Restart(ctx context.Context, name string) error
	Stop(ctx context.Context, name string) error
}

// LSPCommand implements the /lsp slash command with subcommands:
//
//	/lsp                   alias of /lsp list-servers
//	/lsp status            human-readable per-server status table
//	/lsp restart <name>    delegate to LSPManager.Restart
//	/lsp list-servers      curated allowlist annotated with on-path/running
//	/lsp stop <name>       delegate to LSPManager.Stop
//
// The default subcommand is list-servers (not status) because the curated
// allowlist + on-path indicator is the answer to "which LSPs do I have
// available?", which is the most common entry-point question. /lsp status
// is the right question once at least one server is already running.
type LSPCommand struct {
	manager      LSPManager
	curatedSpecs []tools.LSPServerSpec
}

// NewLSPCommand constructs the /lsp slash command. curatedSpecs may be
// nil (the list-servers subcommand will then render an empty table); in
// production main.go is expected to pass tools.CuratedServerSpecs().
func NewLSPCommand(manager LSPManager, curatedSpecs []tools.LSPServerSpec) *LSPCommand {
	return &LSPCommand{manager: manager, curatedSpecs: curatedSpecs}
}

// Name returns the slash command name (without the leading slash).
func (c *LSPCommand) Name() string { return "lsp" }

// Aliases returns alternative invocation names. /lsp has none.
func (c *LSPCommand) Aliases() []string { return nil }

// Description returns the one-line help blurb shown by /help.
func (c *LSPCommand) Description() string {
	return "Inspect, restart, list, or stop managed LSP servers."
}

// Usage returns the usage string shown by /help.
func (c *LSPCommand) Usage() string {
	return "/lsp [status|restart <name>|list-servers|stop <name>]"
}

// Execute dispatches to the appropriate subcommand handler.
func (c *LSPCommand) Execute(ctx context.Context, cc *CommandContext) (*CommandResult, error) {
	args := cc.Args
	sub := "list-servers"
	if len(args) > 0 {
		sub = args[0]
	}
	switch sub {
	case "status":
		return c.status(), nil
	case "restart":
		if len(args) < 2 {
			return nil, fmt.Errorf("/lsp restart <name>")
		}
		return c.restart(ctx, args[1])
	case "list-servers":
		return c.listServers(), nil
	case "stop":
		if len(args) < 2 {
			return nil, fmt.Errorf("/lsp stop <name>")
		}
		return c.stop(ctx, args[1])
	default:
		return nil, fmt.Errorf("/lsp: unknown subcommand %q (want status|restart|list-servers|stop)", sub)
	}
}

// status renders a tab-aligned table of every currently-running server
// reported by LSPManager.Servers(). When no servers are running it
// reports "no servers running" so the operator knows the manager is
// alive but has nothing spawned.
func (c *LSPCommand) status() *CommandResult {
	servers := c.manager.Servers()
	if len(servers) == 0 {
		return &CommandResult{Success: true, Output: "no servers running"}
	}
	var sb strings.Builder
	tw := tabwriter.NewWriter(&sb, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tSTATUS\tPID\tOPEN-FILES\tUPTIME")
	for _, s := range servers {
		fmt.Fprintf(tw, "%s\t%s\t%d\t%d\t%s\n",
			s.Name, s.Status.String(), s.PID, s.OpenFiles, s.Uptime.Round(0).String())
	}
	tw.Flush()
	return &CommandResult{Success: true, Output: sb.String()}
}

// restart asks the manager to tear down and forget the named server.
// The next document open targeting a matching extension will lazily
// respawn it (per LSPManager.Restart documentation).
func (c *LSPCommand) restart(ctx context.Context, name string) (*CommandResult, error) {
	if err := c.manager.Restart(ctx, name); err != nil {
		return nil, fmt.Errorf("/lsp restart %s: %w", name, err)
	}
	return &CommandResult{
		Success: true,
		Output:  fmt.Sprintf("restarted %s (next matching file will respawn)", name),
	}, nil
}

// stop asks the manager to terminate the named server. Unlike restart,
// the server entry is left in Stopped state and is not auto-respawned
// on the next file open until the manager garbage-collects the dead
// entry on its own routing path.
func (c *LSPCommand) stop(ctx context.Context, name string) (*CommandResult, error) {
	if err := c.manager.Stop(ctx, name); err != nil {
		return nil, fmt.Errorf("/lsp stop %s: %w", name, err)
	}
	return &CommandResult{
		Success: true,
		Output:  fmt.Sprintf("stopped %s", name),
	}, nil
}

// listServers renders the curated allowlist with two annotations:
//
//   - ON-PATH: yes/no, computed via exec.LookPath at command time. Pure
//     read; no subprocess is spawned.
//   - RUNNING: yes/no, computed by intersecting the curated names with
//     LSPManager.Servers() at command time.
//
// This gives the operator a single view of "which LSPs could I use,
// which are installed, and which are currently spawned".
func (c *LSPCommand) listServers() *CommandResult {
	running := make(map[string]bool, len(c.curatedSpecs))
	for _, s := range c.manager.Servers() {
		running[s.Name] = true
	}

	var sb strings.Builder
	tw := tabwriter.NewWriter(&sb, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tBINARY\tEXTENSIONS\tON-PATH\tRUNNING")
	for _, spec := range c.curatedSpecs {
		onPath := "no"
		if _, err := exec.LookPath(spec.Binary); err == nil {
			onPath = "yes"
		}
		runStr := "no"
		if running[spec.Name] {
			runStr = "yes"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
			spec.Name,
			spec.Binary,
			strings.Join(spec.FileExtensions, ","),
			onPath,
			runStr,
		)
	}
	tw.Flush()
	return &CommandResult{Success: true, Output: sb.String()}
}
