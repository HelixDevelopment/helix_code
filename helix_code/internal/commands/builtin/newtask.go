package builtin

import (
	"context"
	"strings"

	"dev.helix.code/internal/commands"
)

// NewTaskCommand creates a new task with preserved context
type NewTaskCommand struct{}

// NewNewTaskCommand creates a new /newtask command
func NewNewTaskCommand() *NewTaskCommand {
	return &NewTaskCommand{}
}

// Name returns the command name
func (c *NewTaskCommand) Name() string {
	return "newtask"
}

// Aliases returns command aliases
func (c *NewTaskCommand) Aliases() []string {
	return []string{"nt", "task"}
}

// Description returns command description
func (c *NewTaskCommand) Description() string {
	return trc("builtin_newtask_description", nil)
}

// Usage returns usage information
func (c *NewTaskCommand) Usage() string {
	return trc("builtin_newtask_usage", nil)
}

// Execute runs the command
func (c *NewTaskCommand) Execute(ctx context.Context, cmdCtx *commands.CommandContext) (*commands.CommandResult, error) {
	// Extract task description
	description := strings.Join(cmdCtx.Args, " ")
	if description == "" {
		return &commands.CommandResult{
			Success: false,
			Message: tr(ctx, "builtin_newtask_description_required", nil),
		}, nil
	}

	// Parse flags
	linkPrevious := cmdCtx.Flags["link-previous"] == "true"
	priority := cmdCtx.Flags["priority"]
	if priority == "" {
		priority = "normal"
	}
	transferFiles := cmdCtx.Flags["transfer-files"] == "true"

	// Collect context to preserve
	contextData := make(map[string]interface{})

	// Preserve relevant files from chat history
	if transferFiles {
		files := extractFilesFromHistory(cmdCtx.ChatHistory)
		if len(files) > 0 {
			contextData["files"] = files
		}
	}

	// Link to previous task if requested
	if linkPrevious && cmdCtx.Metadata != nil {
		if prevTaskID, ok := cmdCtx.Metadata["current_task_id"].(string); ok {
			contextData["previous_task_id"] = prevTaskID
		}
	}

	// Preserve working directory
	if cmdCtx.WorkingDir != "" {
		contextData["working_dir"] = cmdCtx.WorkingDir
	}

	// Create actions to be executed
	actions := []commands.Action{
		{
			Type: "create_task",
			Data: map[string]interface{}{
				"description": description,
				"priority":    priority,
				"context":     contextData,
				"created_by":  cmdCtx.UserID,
				"project_id":  cmdCtx.ProjectID,
			},
		},
	}

	// If linking to previous, also add a link action
	if linkPrevious {
		actions = append(actions, commands.Action{
			Type: "link_tasks",
			Data: map[string]interface{}{
				"type": "continuation",
			},
		})
	}

	return &commands.CommandResult{
		Success:     true,
		Message:     tr(ctx, "builtin_newtask_created", map[string]any{"Description": description}),
		Actions:     actions,
		ShouldReply: true,
		Metadata: map[string]interface{}{
			"task_description": description,
			"priority":         priority,
			"linked":           linkPrevious,
		},
	}, nil
}

// extractFilesFromHistory extracts file references from chat history
func extractFilesFromHistory(history []commands.ChatMessage) []string {
	files := make(map[string]bool)

	for _, msg := range history {
		// Look for common file patterns
		// This is a simple implementation - could be enhanced
		words := strings.Fields(msg.Content)
		for _, word := range words {
			// Check if word looks like a file path
			if strings.Contains(word, "/") || strings.Contains(word, ".") {
				// Simple heuristic: has extension or path separator
				ext := strings.LastIndex(word, ".")
				if ext > 0 && ext < len(word)-1 {
					files[word] = true
				}
			}
		}
	}

	result := make([]string, 0, len(files))
	for file := range files {
		result = append(result, file)
	}
	return result
}
