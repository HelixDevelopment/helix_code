package tools

import (
	"context"
	"fmt"
	"strings"

	"dev.helix.code/internal/approval"
	"dev.helix.code/internal/workflow"
)

// TaskOutputTool reads the tail output of a background task. Agent-callable.
type TaskOutputTool struct {
	manager *workflow.BackgroundManager
}

// NewTaskOutputTool returns the agent-callable TaskOutput tool.
func NewTaskOutputTool(m *workflow.BackgroundManager) *TaskOutputTool {
	return &TaskOutputTool{manager: m}
}

func (t *TaskOutputTool) Name() string { return "TaskOutput" }

// RequiresApproval — pure read of background task tail (spec §3.6).
func (t *TaskOutputTool) RequiresApproval() approval.ApprovalLevel { return approval.LevelReadOnly }
func (t *TaskOutputTool) Description() string {
	return "Read the output of a background task. Returns the last N lines (default 5) plus the task's current state."
}
func (t *TaskOutputTool) Category() ToolCategory { return CategoryFileSystem }
func (t *TaskOutputTool) Schema() ToolSchema {
	return ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"task_id": map[string]interface{}{
				"type":        "string",
				"description": "ID of the background task",
			},
			"lines": map[string]interface{}{
				"type":        "integer",
				"description": "Number of trailing lines to return (default 5)",
			},
		},
		Required: []string{"task_id"},
	}
}

func (t *TaskOutputTool) Validate(params map[string]interface{}) error {
	id, ok := params["task_id"].(string)
	if !ok || id == "" {
		return fmt.Errorf("task_id must be a non-empty string")
	}
	return nil
}

func (t *TaskOutputTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	if err := t.Validate(params); err != nil {
		return nil, err
	}
	id := params["task_id"].(string)
	n := 5
	switch v := params["lines"].(type) {
	case float64:
		n = int(v)
	case int:
		n = v
	}
	state, lines, err := t.manager.Status(id)
	if err != nil {
		return nil, err
	}
	if n <= 0 {
		n = 5
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	task, _ := t.manager.GetTask(id)
	totalLines := 0
	if task != nil {
		totalLines = len(task.LastLines(1 << 30))
	}
	return map[string]interface{}{
		"task_id":     id,
		"state":       string(state),
		"output":      strings.Join(lines, "\n"),
		"line_count":  len(lines),
		"total_lines": totalLines,
	}, nil
}

// TaskStopTool cancels a running background task. Agent-callable.
type TaskStopTool struct {
	manager *workflow.BackgroundManager
}

// NewTaskStopTool returns the agent-callable TaskStop tool.
func NewTaskStopTool(m *workflow.BackgroundManager) *TaskStopTool {
	return &TaskStopTool{manager: m}
}

func (t *TaskStopTool) Name() string { return "TaskStop" }

// RequiresApproval — terminates a running background task / process (spec §3.6).
func (t *TaskStopTool) RequiresApproval() approval.ApprovalLevel { return approval.LevelRun }
func (t *TaskStopTool) Description() string    { return "Cancel a running background task by ID." }
func (t *TaskStopTool) Category() ToolCategory { return CategoryFileSystem }
func (t *TaskStopTool) Schema() ToolSchema {
	return ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"task_id": map[string]interface{}{
				"type":        "string",
				"description": "ID of the task to cancel",
			},
		},
		Required: []string{"task_id"},
	}
}

func (t *TaskStopTool) Validate(params map[string]interface{}) error {
	id, ok := params["task_id"].(string)
	if !ok || id == "" {
		return fmt.Errorf("task_id must be a non-empty string")
	}
	return nil
}

func (t *TaskStopTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	if err := t.Validate(params); err != nil {
		return nil, err
	}
	id := params["task_id"].(string)
	if err := t.manager.StopTask(id); err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"task_id": id,
		"status":  "stopped",
	}, nil
}

// Compile-time assertions: both implement the Tool interface.
var (
	_ Tool = (*TaskOutputTool)(nil)
	_ Tool = (*TaskStopTool)(nil)
)

// RegisterTaskTools registers TaskOutput and TaskStop in the registry,
// bound to the supplied BackgroundManager.
func (r *ToolRegistry) RegisterTaskTools(m *workflow.BackgroundManager) {
	r.Register(NewTaskOutputTool(m))
	r.Register(NewTaskStopTool(m))
}
