package commands

import (
	"context"
	"fmt"
	"strings"

	"dev.helix.code/internal/workspace"
)

type OpenhandsCommand struct {
	mgr *workspace.WorkspaceManager
}

func NewOpenhandsCommand(mgr *workspace.WorkspaceManager) *OpenhandsCommand {
	return &OpenhandsCommand{mgr: mgr}
}

func (c *OpenhandsCommand) Name() string      { return "openhands" }
func (c *OpenhandsCommand) Aliases() []string { return []string{"oh"} }
func (c *OpenhandsCommand) Description() string {
	return tr(context.Background(), "internal_commands_openhands_description", nil)
}
func (c *OpenhandsCommand) Usage() string {
	return tr(context.Background(), "internal_commands_openhands_usage", nil)
}

func (c *OpenhandsCommand) Execute(ctx context.Context, cmdCtx *CommandContext) (*CommandResult, error) {
	args := cmdCtx.Args
	subcmd := "list"
	if len(args) > 0 {
		subcmd = args[0]
	}

	switch subcmd {
	case "list":
		return c.handleList(ctx, cmdCtx)
	case "create":
		return c.handleCreate(ctx, cmdCtx)
	case "cleanup":
		return c.handleCleanup(ctx, cmdCtx)
	default:
		return &CommandResult{
			Success: false,
			Message: tr(ctx, "internal_commands_openhands_unknown_subcommand", map[string]any{"Subcommand": subcmd}),
		}, nil
	}
}

func (c *OpenhandsCommand) handleList(ctx context.Context, cmdCtx *CommandContext) (*CommandResult, error) {
	list := c.mgr.ListWorkspaces()
	if len(list) == 0 {
		return &CommandResult{Success: true, Message: tr(ctx, "internal_commands_openhands_none_found", nil)}, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%-36s %-20s %-15s %s\n", "ID", "NAME", "STATUS", "IMAGE"))
	sb.WriteString(strings.Repeat("-", 80) + "\n")
	for _, ws := range list {
		sb.WriteString(fmt.Sprintf("%-36s %-20s %-15s %s\n", ws.ID, ws.Name, ws.Status.String(), ws.Image))
	}
	output := sb.String()
	return &CommandResult{
		Success: true,
		Message: tr(ctx, "internal_commands_openhands_count", map[string]any{"Count": len(list)}),
		Output:  output,
	}, nil
}

func (c *OpenhandsCommand) handleCreate(ctx context.Context, cmdCtx *CommandContext) (*CommandResult, error) {
	if len(cmdCtx.Args) < 2 {
		return &CommandResult{Success: false, Message: tr(ctx, "internal_commands_openhands_create_usage", nil)}, nil
	}
	name := cmdCtx.Args[1]
	image := ""
	projectDir := "."
	if len(cmdCtx.Args) > 2 {
		image = cmdCtx.Args[2]
	}
	if len(cmdCtx.Args) > 3 {
		projectDir = cmdCtx.Args[3]
	}

	ws, err := c.mgr.CreateWorkspace(ctx, name, image, projectDir)
	if err != nil {
		return &CommandResult{Success: false, Message: fmt.Sprintf("create workspace: %v", err)}, nil
	}

	return &CommandResult{
		Success: true,
		Message: tr(ctx, "internal_commands_openhands_created", map[string]any{
			"Name":      ws.Name,
			"ID":        ws.ID,
			"Container": ws.ContainerID,
			"Image":     ws.Image,
		}),
	}, nil
}

func (c *OpenhandsCommand) handleCleanup(ctx context.Context, cmdCtx *CommandContext) (*CommandResult, error) {
	if len(cmdCtx.Args) < 2 {
		return &CommandResult{Success: false, Message: tr(ctx, "internal_commands_openhands_cleanup_usage", nil)}, nil
	}
	id := cmdCtx.Args[1]

	if err := c.mgr.CleanupWorkspace(ctx, id); err != nil {
		return &CommandResult{Success: false, Message: fmt.Sprintf("cleanup workspace: %v", err)}, nil
	}

	return &CommandResult{
		Success: true,
		Message: tr(ctx, "internal_commands_openhands_cleaned_up", map[string]any{"ID": id}),
	}, nil
}
