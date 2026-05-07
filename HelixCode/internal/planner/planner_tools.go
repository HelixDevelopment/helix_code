package planner

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"dev.helix.code/internal/approval"
	"dev.helix.code/internal/tools"

	"github.com/google/uuid"
)

type TaskPlanTool struct {
	executor *SequentialExecutor
	approval.DefaultLevelEdit
}

func NewTaskPlanTool(executor *SequentialExecutor) *TaskPlanTool {
	return &TaskPlanTool{executor: executor}
}

func (t *TaskPlanTool) Name() string        { return "task_plan" }
func (t *TaskPlanTool) Description() string { return "Create and execute a task plan from a plan tree" }
func (t *TaskPlanTool) Category() tools.ToolCategory {
	return tools.ToolCategory("planner")
}

func (t *TaskPlanTool) Schema() tools.ToolSchema {
	return tools.ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"name":   map[string]interface{}{"type": "string", "description": "Plan name"},
			"steps":  map[string]interface{}{"type": "array", "description": "List of step objects"},
		},
		Required: []string{"name", "steps"},
	}
}

func (t *TaskPlanTool) Validate(params map[string]interface{}) error {
	if _, ok := params["name"].(string); !ok || params["name"].(string) == "" {
		return errors.New("name is required")
	}
	return nil
}

func (t *TaskPlanTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	name := params["name"].(string)

	stepsRaw, ok := params["steps"].([]interface{})
	if !ok || len(stepsRaw) == 0 {
		return nil, errors.New("steps must be a non-empty array")
	}

	plan := &TaskPlan{
		ID:     uuid.New().String(),
		Name:   name,
		Status: PlanStatusPending,
	}

	for _, raw := range stepsRaw {
		stepMap, ok := raw.(map[string]interface{})
		if !ok {
			return nil, errors.New("each step must be an object")
		}

		step := TaskStep{
			ID:     uuid.New().String(),
			Type:   StepShell,
			Status: StepPending,
		}

		if cmd, ok := stepMap["command"].(string); ok {
			step.Command = cmd
			step.Type = StepShell
		} else if prompt, ok := stepMap["prompt"].(string); ok {
			step.Prompt = prompt
			step.Type = StepLLM
		} else {
			return nil, errors.New("step must have 'command' or 'prompt'")
		}

		if nodeID, ok := stepMap["plan_node_id"].(string); ok {
			step.PlanNodeID = nodeID
		}

		step.MaxRetries = DefaultRetries
		step.Timeout = DefaultTimeout
		plan.Steps = append(plan.Steps, step)
	}

	if err := t.executor.ExecutePlan(ctx, plan); err != nil {
		data, _ := json.Marshal(plan)
		var result map[string]interface{}
		json.Unmarshal(data, &result)
		result["error"] = err.Error()
		return result, nil
	}

	data, _ := json.Marshal(plan)
	var result map[string]interface{}
	json.Unmarshal(data, &result)
	return result, nil
}

type TaskStepTool struct {
	executor *SequentialExecutor
	approval.DefaultLevelEdit
}

func NewTaskStepTool(executor *SequentialExecutor) *TaskStepTool {
	return &TaskStepTool{executor: executor}
}

func (t *TaskStepTool) Name() string        { return "task_step" }
func (t *TaskStepTool) Description() string { return "Execute a single task step" }
func (t *TaskStepTool) Category() tools.ToolCategory {
	return tools.ToolCategory("planner")
}

func (t *TaskStepTool) Schema() tools.ToolSchema {
	return tools.ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"command":     map[string]interface{}{"type": "string", "description": "Shell command to execute"},
			"timeout":     map[string]interface{}{"type": "number", "description": "Timeout in seconds"},
			"max_retries": map[string]interface{}{"type": "number", "description": "Max retry attempts"},
		},
		Required: []string{"command"},
	}
}

func (t *TaskStepTool) Validate(params map[string]interface{}) error {
	if _, ok := params["command"].(string); !ok || params["command"].(string) == "" {
		return errors.New("command is required")
	}
	return nil
}

func (t *TaskStepTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	command := params["command"].(string)

	step := &TaskStep{
		ID:         uuid.New().String(),
		Type:       StepShell,
		Command:    command,
		Status:     StepPending,
		MaxRetries: DefaultRetries,
		Timeout:    DefaultTimeout,
	}

	if tr, ok := params["max_retries"].(float64); ok {
		step.MaxRetries = int(tr)
	}
	if to, ok := params["timeout"].(float64); ok && to > 0 {
		step.Timeout = time.Duration(to) * time.Second
	}

	if err := t.executor.ExecuteStep(ctx, step); err != nil {
		data, _ := json.Marshal(step)
		var result map[string]interface{}
		json.Unmarshal(data, &result)
		result["error"] = err.Error()
		return result, nil
	}

	data, _ := json.Marshal(step)
	var result map[string]interface{}
	json.Unmarshal(data, &result)
	return result, nil
}
