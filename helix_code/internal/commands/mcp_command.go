package commands

import (
	"bytes"
	"context"
	"fmt"
	"text/tabwriter"

	"dev.helix.code/internal/mcp"
)

// MCPCommand implements /mcp.
//
// Subactions:
//
//	/mcp             — list (default)
//	/mcp list        — explicit list
//	/mcp test <name> — probe a server (connect → tools/list → close)
//	/mcp reload      — diff config and reconcile clients
type MCPCommand struct {
	manager *mcp.Manager
}

// NewMCPCommand wires an mcp.Manager into the slash command.
func NewMCPCommand(m *mcp.Manager) *MCPCommand {
	return &MCPCommand{manager: m}
}

func (c *MCPCommand) Name() string        { return "mcp" }
func (c *MCPCommand) Aliases() []string   { return []string{} }
func (c *MCPCommand) Description() string { return "Manage MCP server connections" }
func (c *MCPCommand) Usage() string       { return "/mcp [list | test <name> | reload]" }

func (c *MCPCommand) Execute(ctx context.Context, cmdCtx *CommandContext) (*CommandResult, error) {
	if c.manager == nil {
		// CONST-046 (round-149): manager-not-initialised message
		// resolved through the package-level translator.
		return nil, fmt.Errorf("%s", tr(ctx, "internal_commands_mcp_manager_not_initialised", nil))
	}
	sub := "list"
	if len(cmdCtx.Args) > 0 {
		sub = cmdCtx.Args[0]
	}
	switch sub {
	case "list":
		return &CommandResult{Output: c.list(), Success: true}, nil
	case "test":
		if len(cmdCtx.Args) < 2 {
			return nil, fmt.Errorf("/mcp test <name>")
		}
		if err := c.manager.Test(ctx, cmdCtx.Args[1]); err != nil {
			return nil, err
		}
		return &CommandResult{Output: "ready", Success: true}, nil
	case "reload":
		cfg := c.manager.Config()
		if cfg == nil {
			return nil, fmt.Errorf("/mcp reload: no config loaded")
		}
		if err := c.manager.Reload(ctx, cfg); err != nil {
			return nil, err
		}
		return &CommandResult{Output: "reloaded", Success: true}, nil
	default:
		return nil, fmt.Errorf("/mcp: unknown subcommand %q (want list|test|reload)", sub)
	}
}

func (c *MCPCommand) list() string {
	var buf bytes.Buffer
	tw := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tTRANSPORT\tSTATE\tTOOLS")
	for _, s := range c.manager.Status() {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%d\n", s.Name, s.Transport, s.State, s.ToolCount)
	}
	tw.Flush()
	return buf.String()
}
