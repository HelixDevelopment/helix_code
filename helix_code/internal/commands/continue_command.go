package commands

import (
	"context"
	"fmt"

	"dev.helix.code/internal/continua"
)

type ContinueCommand struct {
	editor     *continua.WorkspaceEditor
	completion *continua.CompletionEngine
	chat       *continua.ChatManager
}

func NewContinueCommand(e *continua.WorkspaceEditor, c *continua.CompletionEngine, ch *continua.ChatManager) *ContinueCommand {
	return &ContinueCommand{editor: e, completion: c, chat: ch}
}

func (c *ContinueCommand) Name() string      { return "continue" }
func (c *ContinueCommand) Aliases() []string { return []string{"cont"} }
func (c *ContinueCommand) Description() string {
	return tr(context.Background(), "internal_commands_continue_description", nil)
}
func (c *ContinueCommand) Usage() string {
	return tr(context.Background(), "internal_commands_continue_usage", nil)
}

func (c *ContinueCommand) Execute(ctx context.Context, cmdCtx *CommandContext) (*CommandResult, error) {
	args := cmdCtx.Args
	if len(args) == 0 {
		return &CommandResult{Success: true, Message: tr(ctx, "internal_commands_continue_usage_full", nil)}, nil
	}

	switch args[0] {
	case "edit":
		return c.handleEdit(ctx, cmdCtx, args[1:])
	case "complete":
		return c.handleComplete(ctx, cmdCtx, args[1:])
	case "chat":
		return c.handleChat(ctx, cmdCtx, args[1:])
	case "diff":
		return c.handleDiff(ctx, cmdCtx, args[1:])
	default:
		return &CommandResult{Success: false, Message: tr(ctx, "internal_commands_continue_unknown_subcommand", map[string]any{"Subcommand": args[0]})}, nil
	}
}

func (c *ContinueCommand) handleEdit(ctx context.Context, cmdCtx *CommandContext, args []string) (*CommandResult, error) {
	if len(args) < 2 {
		return &CommandResult{Success: false, Message: tr(ctx, "internal_commands_continue_edit_usage", nil)}, nil
	}
	action := args[0]
	file := args[1]

	if action == "open" {
		result, err := c.editor.Open(ctx, file)
		if err != nil {
			return &CommandResult{Success: false, Message: fmt.Sprintf("open: %v", err)}, nil
		}
		return &CommandResult{Success: true, Message: tr(ctx, "internal_commands_continue_edit_opened", map[string]any{"File": result.FilePath, "Lines": result.Lines})}, nil
	}
	return &CommandResult{Success: false, Message: tr(ctx, "internal_commands_continue_edit_unknown_action", map[string]any{"Action": action})}, nil
}

func (c *ContinueCommand) handleComplete(ctx context.Context, cmdCtx *CommandContext, args []string) (*CommandResult, error) {
	if len(args) < 3 {
		return &CommandResult{Success: false, Message: tr(ctx, "internal_commands_continue_complete_usage", nil)}, nil
	}
	line, col := 0, 0
	fmt.Sscanf(args[1], "%d", &line)
	fmt.Sscanf(args[2], "%d", &col)

	result, err := c.completion.Complete(ctx, args[0], line, col)
	if err != nil {
		return &CommandResult{Success: false, Message: fmt.Sprintf("complete: %v", err)}, nil
	}
	return &CommandResult{Success: true, Message: result.Suggestion}, nil
}

func (c *ContinueCommand) handleChat(ctx context.Context, cmdCtx *CommandContext, args []string) (*CommandResult, error) {
	if len(args) == 0 {
		return &CommandResult{Success: false, Message: tr(ctx, "internal_commands_continue_chat_usage", nil)}, nil
	}
	action := args[0]
	switch action {
	case "create":
		title := "chat"
		if len(args) > 1 {
			title = args[1]
		}
		session := c.chat.CreateSession(title, "")
		return &CommandResult{Success: true, Message: tr(ctx, "internal_commands_continue_chat_created", map[string]any{"ID": session.ID})}, nil
	case "list":
		list := c.chat.ListSessions()
		return &CommandResult{Success: true, Message: tr(ctx, "internal_commands_continue_chat_count", map[string]any{"Count": len(list)})}, nil
	case "add":
		return &CommandResult{Success: true, Message: tr(ctx, "internal_commands_continue_chat_add_hint", nil)}, nil
	default:
		return &CommandResult{Success: false, Message: tr(ctx, "internal_commands_continue_unknown_subcommand", map[string]any{"Subcommand": action})}, nil
	}
}

func (c *ContinueCommand) handleDiff(ctx context.Context, cmdCtx *CommandContext, args []string) (*CommandResult, error) {
	if len(args) < 2 {
		return &CommandResult{Success: false, Message: tr(ctx, "internal_commands_continue_diff_usage", nil)}, nil
	}
	return &CommandResult{Success: true, Message: tr(ctx, "internal_commands_continue_diff_hint", nil)}, nil
}
