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
	return "Continue IDE: completions, editor, chat, diff"
}
func (c *ContinueCommand) Usage() string {
	return "/continue [edit|complete|chat|diff]"
}

func (c *ContinueCommand) Execute(ctx context.Context, cmdCtx *CommandContext) (*CommandResult, error) {
	args := cmdCtx.Args
	if len(args) == 0 {
		return &CommandResult{Success: true, Message: "/continue edit <open|save> <file> | complete <file> <line> <col> | chat <create|add|list> | diff <old> <new>"}, nil
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
		return &CommandResult{Success: false, Message: fmt.Sprintf("unknown: %s", args[0])}, nil
	}
}

func (c *ContinueCommand) handleEdit(ctx context.Context, cmdCtx *CommandContext, args []string) (*CommandResult, error) {
	if len(args) < 2 {
		return &CommandResult{Success: false, Message: "usage: /continue edit <open> <file>"}, nil
	}
	action := args[0]
	file := args[1]

	if action == "open" {
		result, err := c.editor.Open(ctx, file)
		if err != nil {
			return &CommandResult{Success: false, Message: fmt.Sprintf("open: %v", err)}, nil
		}
		return &CommandResult{Success: true, Message: fmt.Sprintf("Opened %s: %d lines", result.FilePath, result.Lines)}, nil
	}
	return &CommandResult{Success: false, Message: fmt.Sprintf("unknown edit action: %s", action)}, nil
}

func (c *ContinueCommand) handleComplete(ctx context.Context, cmdCtx *CommandContext, args []string) (*CommandResult, error) {
	if len(args) < 3 {
		return &CommandResult{Success: false, Message: "usage: /continue complete <file> <line> <col>"}, nil
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
		return &CommandResult{Success: false, Message: "usage: /continue chat <create|add|list>"}, nil
	}
	action := args[0]
	switch action {
	case "create":
		title := "chat"
		if len(args) > 1 { title = args[1] }
		session := c.chat.CreateSession(title, "")
		return &CommandResult{Success: true, Message: fmt.Sprintf("Chat session created: %s", session.ID)}, nil
	case "list":
		list := c.chat.ListSessions()
		return &CommandResult{Success: true, Message: fmt.Sprintf("%d chat session(s)", len(list))}, nil
	case "add":
		return &CommandResult{Success: true, Message: "Use tools to add messages to chat sessions"}, nil
	default:
		return &CommandResult{Success: false, Message: fmt.Sprintf("unknown: %s", action)}, nil
	}
}

func (c *ContinueCommand) handleDiff(ctx context.Context, cmdCtx *CommandContext, args []string) (*CommandResult, error) {
	if len(args) < 2 {
		return &CommandResult{Success: false, Message: "usage: /continue diff <file1> <file2>"}, nil
	}
	return &CommandResult{Success: true, Message: "Diff: use continue_edit tool to compare file versions"}, nil
}
