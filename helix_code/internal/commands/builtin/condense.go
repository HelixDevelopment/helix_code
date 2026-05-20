package builtin

import (
	"context"
	"fmt"

	"dev.helix.code/internal/commands"
)

// CondenseCommand summarizes conversation history
type CondenseCommand struct{}

// NewCondenseCommand creates a new /condense command
func NewCondenseCommand() *CondenseCommand {
	return &CondenseCommand{}
}

// Name returns the command name
func (c *CondenseCommand) Name() string {
	return "condense"
}

// Aliases returns command aliases
func (c *CondenseCommand) Aliases() []string {
	return []string{"smol", "compact", "summarize"}
}

// Description returns command description
func (c *CondenseCommand) Description() string {
	return trc("builtin_condense_description", nil)
}

// Usage returns usage information
func (c *CondenseCommand) Usage() string {
	return trc("builtin_condense_usage", nil)
}

// Execute runs the command
func (c *CondenseCommand) Execute(ctx context.Context, cmdCtx *commands.CommandContext) (*commands.CommandResult, error) {
	if len(cmdCtx.ChatHistory) == 0 {
		return &commands.CommandResult{
			Success: false,
			Message: tr(ctx, "builtin_condense_no_history", nil),
		}, nil
	}

	// Parse flags
	keepLast := 5 // default
	if val, ok := cmdCtx.Flags["keep-last"]; ok {
		fmt.Sscanf(val, "%d", &keepLast)
	}

	preserveCode := cmdCtx.Flags["preserve-code"] == "true"
	preserveErrors := cmdCtx.Flags["preserve-errors"] == "true"

	ratio := 0.5 // default 50% compression
	if val, ok := cmdCtx.Flags["ratio"]; ok {
		fmt.Sscanf(val, "%f", &ratio)
	}

	// Calculate message counts
	totalMessages := len(cmdCtx.ChatHistory)
	keepMessages := keepLast
	condenseCount := totalMessages - keepMessages

	if condenseCount <= 0 {
		return &commands.CommandResult{
			Success: false,
			Message: tr(ctx, "builtin_condense_not_enough_history", nil),
		}, nil
	}

	// Create action to condense history
	actions := []commands.Action{
		{
			Type: "condense_history",
			Data: map[string]interface{}{
				"total_messages":  totalMessages,
				"keep_last":       keepMessages,
				"condense_count":  condenseCount,
				"preserve_code":   preserveCode,
				"preserve_errors": preserveErrors,
				"target_ratio":    ratio,
				"session_id":      cmdCtx.SessionID,
			},
		},
	}

	return &commands.CommandResult{
		Success: true,
		Message: tr(ctx, "builtin_condense_in_progress", map[string]any{
			"Count": condenseCount,
			"Keep":  keepMessages,
		}),
		Actions:     actions,
		ShouldReply: true,
		Metadata: map[string]interface{}{
			"before_count": totalMessages,
			"after_count":  keepMessages,
			"ratio":        ratio,
		},
	}, nil
}
