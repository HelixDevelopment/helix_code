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
	return tr(context.Background(), "internal_commands_roocode_description", nil)
}
func (c *RooCodeCommand) Usage() string {
	return tr(context.Background(), "internal_commands_roocode_usage", nil)
}

func (c *RooCodeCommand) Execute(ctx context.Context, cmdCtx *CommandContext) (*CommandResult, error) {
	args := cmdCtx.Args
	if len(args) == 0 {
		return &CommandResult{Success: true, Message: tr(ctx, "internal_commands_roocode_usage_full", nil)}, nil
	}

	switch args[0] {
	case "delegate":
		if len(args) < 2 {
			return &CommandResult{Success: false, Message: tr(ctx, "internal_commands_roocode_delegate_usage", nil)}, nil
		}
		desc := ""
		if len(args) > 2 { desc = args[2] }
		task, err := c.delegator.Delegate(ctx, args[1], desc, 1)
		if err != nil {
			return &CommandResult{Success: false, Message: fmt.Sprintf("delegate: %v", err)}, nil
		}
		return &CommandResult{Success: true, Message: tr(ctx, "internal_commands_roocode_task_delegated", map[string]any{"Title": task.Title, "ID": task.ID})}, nil

	case "generate":
		if len(args) < 3 {
			return &CommandResult{Success: false, Message: tr(ctx, "internal_commands_roocode_generate_usage", nil)}, nil
		}
		path, err := c.gen.Generate(ctx, roocode.GenerateSpec{Type: args[1], Name: args[2]})
		if err != nil {
			return &CommandResult{Success: false, Message: fmt.Sprintf("generate: %v", err)}, nil
		}
		return &CommandResult{Success: true, Message: tr(ctx, "internal_commands_roocode_generated", map[string]any{"Path": path})}, nil

	case "bootstrap":
		if len(args) < 3 {
			return &CommandResult{Success: false, Message: tr(ctx, "internal_commands_roocode_bootstrap_usage", nil)}, nil
		}
		files, err := c.gen.Bootstrap(ctx, roocode.BootstrapSpec{ProjectType: args[1], Name: args[2]})
		if err != nil {
			return &CommandResult{Success: false, Message: fmt.Sprintf("bootstrap: %v", err)}, nil
		}
		return &CommandResult{Success: true, Message: tr(ctx, "internal_commands_roocode_bootstrapped", map[string]any{"Type": args[1], "Files": len(files)})}, nil

	case "review":
		if len(args) < 2 {
			return &CommandResult{Success: false, Message: tr(ctx, "internal_commands_roocode_review_usage", nil)}, nil
		}
		result, err := c.reviewer.Review(ctx, args[1])
		if err != nil {
			return &CommandResult{Success: false, Message: fmt.Sprintf("review: %v", err)}, nil
		}
		status := "REJECTED"
		if result.Approved { status = "APPROVED" }
		return &CommandResult{Success: true, Message: tr(ctx, "internal_commands_roocode_review_result", map[string]any{
			"Status": status, "Issues": len(result.Issues), "Suggestions": len(result.Suggestions)})}, nil

	case "conv":
		subcmd := "list"
		if len(args) > 1 { subcmd = args[1] }
		switch subcmd {
		case "create":
			title := "conversation"
			if len(args) > 2 { title = args[2] }
			conv := c.convStore.Create(title)
			return &CommandResult{Success: true, Message: tr(ctx, "internal_commands_roocode_conversation_created", map[string]any{"ID": conv.ID})}, nil
		case "add":
			return &CommandResult{Success: true, Message: tr(ctx, "internal_commands_roocode_conv_add_hint", nil)}, nil
		default:
			list := c.convStore.List()
			return &CommandResult{Success: true, Message: tr(ctx, "internal_commands_roocode_conv_count", map[string]any{"Count": len(list)})}, nil
		}

	default:
		return &CommandResult{Success: false, Message: tr(ctx, "internal_commands_roocode_unknown_subcommand", map[string]any{"Subcommand": args[0]})}, nil
	}
}
