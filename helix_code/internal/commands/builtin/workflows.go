package builtin

import (
	"context"
	"strings"

	"dev.helix.code/internal/commands"
)

// WorkflowsCommand manages and executes workflows
type WorkflowsCommand struct{}

// NewWorkflowsCommand creates a new /workflows command
func NewWorkflowsCommand() *WorkflowsCommand {
	return &WorkflowsCommand{}
}

// Name returns the command name
func (c *WorkflowsCommand) Name() string {
	return "workflows"
}

// Aliases returns command aliases
func (c *WorkflowsCommand) Aliases() []string {
	return []string{"wf", "flow"}
}

// Description returns command description
func (c *WorkflowsCommand) Description() string {
	return trc("builtin_workflows_description", nil)
}

// Usage returns usage information
func (c *WorkflowsCommand) Usage() string {
	return trc("builtin_workflows_usage", nil)
}

// Execute runs the command
func (c *WorkflowsCommand) Execute(ctx context.Context, cmdCtx *commands.CommandContext) (*commands.CommandResult, error) {
	// Check for list flag
	if _, ok := cmdCtx.Flags["list"]; ok {
		return c.listWorkflows(ctx, cmdCtx)
	}

	// Check for status flag
	if workflowID, ok := cmdCtx.Flags["status"]; ok {
		return c.checkWorkflowStatus(ctx, workflowID, cmdCtx)
	}

	// Check for cancel flag
	if workflowID, ok := cmdCtx.Flags["cancel"]; ok {
		return c.cancelWorkflow(ctx, workflowID, cmdCtx)
	}

	// Execute workflow if name provided
	if len(cmdCtx.Args) > 0 {
		workflowName := cmdCtx.Args[0]
		return c.executeWorkflow(ctx, workflowName, cmdCtx)
	}

	// Default: list workflows
	return c.listWorkflows(ctx, cmdCtx)
}

// listWorkflows lists all available workflows
func (c *WorkflowsCommand) listWorkflows(ctx context.Context, cmdCtx *commands.CommandContext) (*commands.CommandResult, error) {
	workflows := []map[string]string{
		{
			"name":        "planning",
			"description": tr(ctx, "builtin_workflows_wf_planning_description", nil),
			"steps":       "3",
		},
		{
			"name":        "building",
			"description": tr(ctx, "builtin_workflows_wf_building_description", nil),
			"steps":       "4",
		},
		{
			"name":        "testing",
			"description": tr(ctx, "builtin_workflows_wf_testing_description", nil),
			"steps":       "5",
		},
		{
			"name":        "refactoring",
			"description": tr(ctx, "builtin_workflows_wf_refactoring_description", nil),
			"steps":       "3",
		},
		{
			"name":        "debugging",
			"description": tr(ctx, "builtin_workflows_wf_debugging_description", nil),
			"steps":       "4",
		},
		{
			"name":        "deployment",
			"description": tr(ctx, "builtin_workflows_wf_deployment_description", nil),
			"steps":       "6",
		},
	}

	// Create action to list workflows
	actions := []commands.Action{
		{
			Type: "list_workflows",
			Data: map[string]interface{}{
				"workflows":   workflows,
				"project_id":  cmdCtx.ProjectID,
				"working_dir": cmdCtx.WorkingDir,
			},
		},
	}

	return &commands.CommandResult{
		Success:     true,
		Message:     tr(ctx, "builtin_workflows_found", map[string]any{"Count": len(workflows)}),
		Actions:     actions,
		ShouldReply: true,
		Metadata: map[string]interface{}{
			"workflow_count": len(workflows),
			"workflows":      workflows,
		},
	}, nil
}

// executeWorkflow executes a specific workflow
func (c *WorkflowsCommand) executeWorkflow(ctx context.Context, workflowName string, cmdCtx *commands.CommandContext) (*commands.CommandResult, error) {
	// Parse workflow parameters
	params := make(map[string]interface{})
	if paramsStr, ok := cmdCtx.Flags["params"]; ok {
		params = parseWorkflowParams(paramsStr)
	}

	// Check if async execution
	async := cmdCtx.Flags["async"] == "true"

	// Validate workflow name
	validWorkflows := map[string]bool{
		"planning":    true,
		"building":    true,
		"testing":     true,
		"refactoring": true,
		"debugging":   true,
		"deployment":  true,
	}

	if !validWorkflows[workflowName] {
		return &commands.CommandResult{
			Success: false,
			Message: tr(ctx, "builtin_workflows_unknown", map[string]any{"Name": workflowName}),
		}, nil
	}

	// Create action to execute workflow
	actions := []commands.Action{
		{
			Type: "execute_workflow",
			Data: map[string]interface{}{
				"workflow_name": workflowName,
				"parameters":    params,
				"async":         async,
				"session_id":    cmdCtx.SessionID,
				"project_id":    cmdCtx.ProjectID,
				"working_dir":   cmdCtx.WorkingDir,
				"context":       extractWorkflowContext(cmdCtx.ChatHistory),
			},
		},
	}

	message := tr(ctx, "builtin_workflows_executing", map[string]any{"Name": workflowName})
	if async {
		message += tr(ctx, "builtin_workflows_async_suffix", nil)
	}

	return &commands.CommandResult{
		Success:     true,
		Message:     message,
		Actions:     actions,
		ShouldReply: true,
		Metadata: map[string]interface{}{
			"workflow_name": workflowName,
			"async":         async,
			"params":        params,
		},
	}, nil
}

// checkWorkflowStatus checks the status of a running workflow
func (c *WorkflowsCommand) checkWorkflowStatus(ctx context.Context, workflowID string, cmdCtx *commands.CommandContext) (*commands.CommandResult, error) {
	actions := []commands.Action{
		{
			Type: "check_workflow_status",
			Data: map[string]interface{}{
				"workflow_id": workflowID,
				"session_id":  cmdCtx.SessionID,
			},
		},
	}

	return &commands.CommandResult{
		Success:     true,
		Message:     tr(ctx, "builtin_workflows_checking_status", map[string]any{"ID": workflowID}),
		Actions:     actions,
		ShouldReply: true,
		Metadata: map[string]interface{}{
			"workflow_id": workflowID,
		},
	}, nil
}

// cancelWorkflow cancels a running workflow
func (c *WorkflowsCommand) cancelWorkflow(ctx context.Context, workflowID string, cmdCtx *commands.CommandContext) (*commands.CommandResult, error) {
	actions := []commands.Action{
		{
			Type: "cancel_workflow",
			Data: map[string]interface{}{
				"workflow_id": workflowID,
				"session_id":  cmdCtx.SessionID,
				"user_id":     cmdCtx.UserID,
			},
		},
	}

	return &commands.CommandResult{
		Success:     true,
		Message:     tr(ctx, "builtin_workflows_cancelling", map[string]any{"ID": workflowID}),
		Actions:     actions,
		ShouldReply: true,
		Metadata: map[string]interface{}{
			"workflow_id": workflowID,
			"cancelled":   true,
		},
	}, nil
}

// parseWorkflowParams parses workflow parameters from string
func parseWorkflowParams(paramsStr string) map[string]interface{} {
	params := make(map[string]interface{})

	// Try to parse as key=value pairs
	pairs := strings.Split(paramsStr, ",")
	for _, pair := range pairs {
		parts := strings.SplitN(strings.TrimSpace(pair), "=", 2)
		if len(parts) == 2 {
			params[parts[0]] = parts[1]
		}
	}

	// If no pairs found, treat as single parameter
	if len(params) == 0 {
		params["input"] = paramsStr
	}

	return params
}

// extractWorkflowContext extracts relevant context from chat history for workflow
func extractWorkflowContext(history []commands.ChatMessage) map[string]interface{} {
	context := make(map[string]interface{})

	// Extract last user request
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Role == "user" {
			context["last_request"] = history[i].Content
			break
		}
	}

	// Count messages for context size
	context["history_size"] = len(history)

	return context
}
