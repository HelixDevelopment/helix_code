package commands

import (
	"context"
	"fmt"

	"dev.helix.code/internal/roocode"
)

type RooCodeCommand struct {
	delegator *roocode.TaskDelegator
	gen       *roocode.CodeGenerator
	reviewer  *roocode.CodeReviewer
	convStore *roocode.ConversationStore
}

func NewRooCodeCommand(d *roocode.TaskDelegator, g *roocode.CodeGenerator, r *roocode.CodeReviewer, c *roocode.ConversationStore) *RooCodeCommand {
	return &RooCodeCommand{delegator: d, gen: g, reviewer: r, convStore: c}
}

func (c *RooCodeCommand) Name() string      { return "roocode" }
func (c *RooCodeCommand) Aliases() []string { return []string{"rc"} }
func (c *RooCodeCommand) Description() string {
	return "Roo-code: delegate tasks, generate code, review, conversations"
}
func (c *RooCodeCommand) Usage() string {
	return "/roocode [delegate|generate|bootstrap|review|conv]"
}

func (c *RooCodeCommand) Execute(ctx context.Context, cmdCtx *CommandContext) (*CommandResult, error) {
	args := cmdCtx.Args
	if len(args) == 0 {
		return &CommandResult{Success: true, Message: "/roocode delegate <title> <desc> | generate <lang> <name> | bootstrap <lang> <name> | review <file> | conv [list|create|add]"}, nil
	}

	switch args[0] {
	case "delegate":
		if len(args) < 2 {
			return &CommandResult{Success: false, Message: "usage: /roocode delegate <title> [desc]"}, nil
		}
		desc := ""
		if len(args) > 2 { desc = args[2] }
		task, err := c.delegator.Delegate(ctx, args[1], desc, 1)
		if err != nil {
			return &CommandResult{Success: false, Message: fmt.Sprintf("delegate: %v", err)}, nil
		}
		return &CommandResult{Success: true, Message: fmt.Sprintf("Task delegated: %s (ID: %s)", task.Title, task.ID)}, nil

	case "generate":
		if len(args) < 3 {
			return &CommandResult{Success: false, Message: "usage: /roocode generate <lang> <name>"}, nil
		}
		path, err := c.gen.Generate(ctx, roocode.GenerateSpec{Type: args[1], Name: args[2]})
		if err != nil {
			return &CommandResult{Success: false, Message: fmt.Sprintf("generate: %v", err)}, nil
		}
		return &CommandResult{Success: true, Message: fmt.Sprintf("Generated: %s", path)}, nil

	case "bootstrap":
		if len(args) < 3 {
			return &CommandResult{Success: false, Message: "usage: /roocode bootstrap <go|python|node> <name>"}, nil
		}
		files, err := c.gen.Bootstrap(ctx, roocode.BootstrapSpec{ProjectType: args[1], Name: args[2]})
		if err != nil {
			return &CommandResult{Success: false, Message: fmt.Sprintf("bootstrap: %v", err)}, nil
		}
		return &CommandResult{Success: true, Message: fmt.Sprintf("Bootstrapped %s project: %d files", args[1], len(files))}, nil

	case "review":
		if len(args) < 2 {
			return &CommandResult{Success: false, Message: "usage: /roocode review <file>"}, nil
		}
		result, err := c.reviewer.Review(ctx, args[1])
		if err != nil {
			return &CommandResult{Success: false, Message: fmt.Sprintf("review: %v", err)}, nil
		}
		status := "REJECTED"
		if result.Approved { status = "APPROVED" }
		return &CommandResult{Success: true, Message: fmt.Sprintf("Review: %s — %d issues, %d suggestions", status, len(result.Issues), len(result.Suggestions))}, nil

	case "conv":
		subcmd := "list"
		if len(args) > 1 { subcmd = args[1] }
		switch subcmd {
		case "create":
			title := "conversation"
			if len(args) > 2 { title = args[2] }
			conv := c.convStore.Create(title)
			return &CommandResult{Success: true, Message: fmt.Sprintf("Conversation created: %s", conv.ID)}, nil
		case "add":
			return &CommandResult{Success: true, Message: "Use tools to add messages to conversations"}, nil
		default:
			list := c.convStore.List()
			return &CommandResult{Success: true, Message: fmt.Sprintf("%d conversation(s)", len(list))}, nil
		}

	default:
		return &CommandResult{Success: false, Message: fmt.Sprintf("unknown: %s", args[0])}, nil
	}
}
