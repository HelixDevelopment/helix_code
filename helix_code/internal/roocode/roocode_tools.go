package roocode

import (
	"context"
	"encoding/json"
	"errors"

	"dev.helix.code/internal/approval"
	"dev.helix.code/internal/tools"
)

type RooDelegateTool struct {
	delegator *TaskDelegator
	approval.DefaultLevelEdit
}

func NewRooDelegateTool(d *TaskDelegator) *RooDelegateTool {
	return &RooDelegateTool{delegator: d}
}

func (t *RooDelegateTool) Name() string  { return "roo_delegate" }
func (t *RooDelegateTool) Description() string {
	return tr(context.Background(), "internal_roocode_tool_delegate_description", nil)
}
func (t *RooDelegateTool) Category() tools.ToolCategory { return tools.ToolCategory("roocode") }

func (t *RooDelegateTool) Schema() tools.ToolSchema {
	return tools.ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"title":       map[string]interface{}{"type": "string"},
			"description": map[string]interface{}{"type": "string"},
			"priority":    map[string]interface{}{"type": "number"},
		},
		Required: []string{"title", "description"},
	}
}

func (t *RooDelegateTool) Validate(p map[string]interface{}) error {
	if _, ok := p["title"].(string); !ok {
		return errors.New(tr(context.Background(), "internal_roocode_validate_title_required", nil))
	}
	return nil
}

func (t *RooDelegateTool) Execute(ctx context.Context, p map[string]interface{}) (interface{}, error) {
	title := p["title"].(string)
	desc, _ := p["description"].(string)
	prio := 1
	if v, ok := p["priority"].(float64); ok { prio = int(v) }

	task, err := t.delegator.Delegate(ctx, title, desc, prio)
	if err != nil { return nil, err }

	data, _ := json.Marshal(task)
	var out map[string]interface{}
	json.Unmarshal(data, &out)
	return out, nil
}

type RooGenerateTool struct {
	gen *CodeGenerator
	approval.DefaultLevelEdit
}

func NewRooGenerateTool(g *CodeGenerator) *RooGenerateTool { return &RooGenerateTool{gen: g} }
func (t *RooGenerateTool) Name() string { return "roo_generate" }
func (t *RooGenerateTool) Description() string {
	return tr(context.Background(), "internal_roocode_tool_generate_description", nil)
}
func (t *RooGenerateTool) Category() tools.ToolCategory { return tools.ToolCategory("roocode") }

func (t *RooGenerateTool) Schema() tools.ToolSchema {
	return tools.ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"type":     map[string]interface{}{"type": "string"},
			"name":     map[string]interface{}{"type": "string"},
			"template": map[string]interface{}{"type": "string"},
			"prompt":   map[string]interface{}{"type": "string"},
		},
		Required: []string{"type", "name"},
	}
}

func (t *RooGenerateTool) Validate(p map[string]interface{}) error {
	if _, ok := p["type"].(string); !ok {
		return errors.New(tr(context.Background(), "internal_roocode_validate_type_required", nil))
	}
	if _, ok := p["name"].(string); !ok {
		return errors.New(tr(context.Background(), "internal_roocode_validate_name_required", nil))
	}
	return nil
}

func (t *RooGenerateTool) Execute(ctx context.Context, p map[string]interface{}) (interface{}, error) {
	spec := GenerateSpec{
		Type: p["type"].(string),
		Name: p["name"].(string),
	}
	if v, ok := p["template"].(string); ok { spec.Template = v }
	if v, ok := p["prompt"].(string); ok { spec.Prompt = v }

	path, err := t.gen.Generate(ctx, spec)
	if err != nil { return nil, err }
	return map[string]interface{}{"file": path, "status": "generated"}, nil
}

type RooBootstrapTool struct {
	gen *CodeGenerator
	approval.DefaultLevelEdit
}

func NewRooBootstrapTool(g *CodeGenerator) *RooBootstrapTool { return &RooBootstrapTool{gen: g} }
func (t *RooBootstrapTool) Name() string { return "roo_bootstrap" }
func (t *RooBootstrapTool) Description() string {
	return tr(context.Background(), "internal_roocode_tool_bootstrap_description", nil)
}
func (t *RooBootstrapTool) Category() tools.ToolCategory { return tools.ToolCategory("roocode") }

func (t *RooBootstrapTool) Schema() tools.ToolSchema {
	return tools.ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"project_type": map[string]interface{}{"type": "string"},
			"name":         map[string]interface{}{"type": "string"},
			"output_dir":   map[string]interface{}{"type": "string"},
		},
		Required: []string{"project_type", "name"},
	}
}

func (t *RooBootstrapTool) Validate(p map[string]interface{}) error {
	if _, ok := p["project_type"].(string); !ok {
		return errors.New(tr(context.Background(), "internal_roocode_validate_project_type_required", nil))
	}
	return nil
}

func (t *RooBootstrapTool) Execute(ctx context.Context, p map[string]interface{}) (interface{}, error) {
	spec := BootstrapSpec{
		ProjectType: p["project_type"].(string),
		Name:        p["name"].(string),
	}
	if v, ok := p["output_dir"].(string); ok { spec.OutputDir = v }

	files, err := t.gen.Bootstrap(ctx, spec)
	if err != nil { return nil, err }
	return map[string]interface{}{"files": files, "status": "bootstrapped"}, nil
}
