package builtin

import (
	"context"
	"fmt"
	"strings"

	"dev.helix.code/internal/commands"
)

// DeepPlanningCommand enters extended planning mode
type DeepPlanningCommand struct{}

// NewDeepPlanningCommand creates a new /deepplanning command
func NewDeepPlanningCommand() *DeepPlanningCommand {
	return &DeepPlanningCommand{}
}

// Name returns the command name
func (c *DeepPlanningCommand) Name() string {
	return "deepplanning"
}

// Aliases returns command aliases
func (c *DeepPlanningCommand) Aliases() []string {
	return []string{"deepplan", "dp", "architect"}
}

// Description returns command description
func (c *DeepPlanningCommand) Description() string {
	return trc("builtin_deepplanning_description", nil)
}

// Usage returns usage information
func (c *DeepPlanningCommand) Usage() string {
	return trc("builtin_deepplanning_usage", nil)
}

// Execute runs the command
func (c *DeepPlanningCommand) Execute(ctx context.Context, cmdCtx *commands.CommandContext) (*commands.CommandResult, error) {
	// Extract planning topic
	topic := strings.Join(cmdCtx.Args, " ")
	if topic == "" && cmdCtx.Flags["resume"] == "" {
		return &commands.CommandResult{
			Success: false,
			Message: tr(ctx, "builtin_deepplanning_topic_required", nil),
		}, nil
	}

	// Check for resume
	if resumeID, ok := cmdCtx.Flags["resume"]; ok {
		return c.resumePlanning(ctx, resumeID, cmdCtx)
	}

	// Parse planning depth
	depth := 3 // default
	if val, ok := cmdCtx.Flags["depth"]; ok {
		fmt.Sscanf(val, "%d", &depth)
		if depth < 1 {
			depth = 1
		} else if depth > 5 {
			depth = 5
		}
	}

	// Parse output format
	outputFile := cmdCtx.Flags["output"]
	outputFormat := "markdown"
	if outputFile != "" {
		if strings.HasSuffix(outputFile, ".json") {
			outputFormat = "json"
		}
	}

	// Parse focus areas
	focusAreas := []string{"architecture", "implementation"}
	if focus, ok := cmdCtx.Flags["focus"]; ok {
		focusAreas = strings.Split(focus, ",")
		for i, area := range focusAreas {
			focusAreas[i] = strings.TrimSpace(area)
		}
	}

	// Parse constraints
	constraints := parseConstraints(cmdCtx.Flags["constraints"])

	// Include diagrams flag
	includeDiagrams := cmdCtx.Flags["include-diagrams"] == "true"

	// Extract context from chat history
	planningContext := extractPlanningContext(cmdCtx.ChatHistory, topic)

	// Create action to start deep planning
	actions := []commands.Action{
		{
			Type: "start_deep_planning",
			Data: map[string]interface{}{
				"topic":            topic,
				"depth":            depth,
				"focus_areas":      focusAreas,
				"constraints":      constraints,
				"include_diagrams": includeDiagrams,
				"output_file":      outputFile,
				"output_format":    outputFormat,
				"context":          planningContext,
				"session_id":       cmdCtx.SessionID,
				"project_id":       cmdCtx.ProjectID,
				"working_dir":      cmdCtx.WorkingDir,
			},
		},
	}

	message := tr(ctx, "builtin_deepplanning_starting", map[string]any{
		"Topic": topic,
		"Depth": depth,
	})
	if outputFile != "" {
		message += "\n" + tr(ctx, "builtin_deepplanning_save_location", map[string]any{"File": outputFile})
	}

	return &commands.CommandResult{
		Success:     true,
		Message:     message,
		Actions:     actions,
		ShouldReply: true,
		Metadata: map[string]interface{}{
			"topic":       topic,
			"depth":       depth,
			"focus_areas": focusAreas,
			"output_file": outputFile,
		},
	}, nil
}

// resumePlanning resumes a previous planning session
func (c *DeepPlanningCommand) resumePlanning(ctx context.Context, planID string, cmdCtx *commands.CommandContext) (*commands.CommandResult, error) {
	actions := []commands.Action{
		{
			Type: "resume_deep_planning",
			Data: map[string]interface{}{
				"plan_id":    planID,
				"session_id": cmdCtx.SessionID,
				"user_id":    cmdCtx.UserID,
			},
		},
	}

	return &commands.CommandResult{
		Success:     true,
		Message:     tr(ctx, "builtin_deepplanning_resuming", map[string]any{"PlanID": planID}),
		Actions:     actions,
		ShouldReply: true,
		Metadata: map[string]interface{}{
			"plan_id": planID,
			"resumed": true,
		},
	}, nil
}

// parseConstraints parses constraint string into map
func parseConstraints(constraintStr string) map[string]string {
	constraints := make(map[string]string)

	if constraintStr == "" {
		return constraints
	}

	pairs := strings.Split(constraintStr, ",")
	for _, pair := range pairs {
		parts := strings.SplitN(strings.TrimSpace(pair), "=", 2)
		if len(parts) == 2 {
			constraints[parts[0]] = parts[1]
		}
	}

	return constraints
}

// extractPlanningContext extracts relevant context for planning
func extractPlanningContext(history []commands.ChatMessage, topic string) map[string]interface{} {
	context := make(map[string]interface{})

	context["topic"] = topic
	context["history_size"] = len(history)

	// Extract recent user requirements
	requirements := make([]string, 0)
	for i := len(history) - 1; i >= 0 && len(requirements) < 10; i-- {
		msg := history[i]
		if msg.Role == "user" {
			// Look for requirement-related keywords
			content := strings.ToLower(msg.Content)
			if strings.Contains(content, "need") ||
				strings.Contains(content, "should") ||
				strings.Contains(content, "require") ||
				strings.Contains(content, "must") ||
				strings.Contains(content, "want") {
				requirements = append(requirements, msg.Content)
			}
		}
	}
	context["requirements"] = requirements

	// Extract mentioned technologies
	technologies := extractTechnologies(history)
	if len(technologies) > 0 {
		context["mentioned_technologies"] = technologies
	}

	return context
}

// extractTechnologies extracts technology mentions from history
func extractTechnologies(history []commands.ChatMessage) []string {
	techKeywords := []string{
		"go", "golang", "python", "javascript", "typescript", "rust", "java",
		"postgres", "postgresql", "mysql", "mongodb", "redis",
		"docker", "kubernetes", "aws", "gcp", "azure",
		"react", "vue", "angular", "svelte",
		"gin", "echo", "fiber", "chi",
		"grpc", "graphql", "rest", "websocket",
	}

	found := make(map[string]bool)
	technologies := make([]string, 0)

	for _, msg := range history {
		content := strings.ToLower(msg.Content)
		for _, tech := range techKeywords {
			if strings.Contains(content, tech) && !found[tech] {
				found[tech] = true
				technologies = append(technologies, tech)
			}
		}
	}

	return technologies
}
