package plantree

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"dev.helix.code/internal/approval"
	"dev.helix.code/internal/tools"
)

type PlanCreateTool struct {
	store Store
	approval.DefaultLevelEdit
}

func NewPlanCreateTool(store Store) *PlanCreateTool {
	return &PlanCreateTool{store: store}
}

func (t *PlanCreateTool) Name() string { return "plan_create" }
func (t *PlanCreateTool) Description() string {
	return tr(context.Background(), "internal_plantree_tool_create_description", nil)
}
func (t *PlanCreateTool) Category() tools.ToolCategory {
	return tools.ToolCategory("plan")
}

func (t *PlanCreateTool) Schema() tools.ToolSchema {
	return tools.ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"name":        map[string]interface{}{"type": "string", "description": "Plan name (alphanumeric + underscore + hyphen)"},
			"title":       map[string]interface{}{"type": "string", "description": "Root node title"},
			"description": map[string]interface{}{"type": "string", "description": "Root node description"},
		},
		Required: []string{"name", "title", "description"},
	}
}

func (t *PlanCreateTool) Validate(params map[string]interface{}) error {
	if _, ok := params["name"].(string); !ok || params["name"].(string) == "" {
		return errors.New("name is required")
	}
	if _, ok := params["title"].(string); !ok {
		return errors.New("title is required")
	}
	if _, ok := params["description"].(string); !ok {
		return errors.New("description is required")
	}
	return nil
}

func (t *PlanCreateTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	name := params["name"].(string)
	title := params["title"].(string)
	description := params["description"].(string)

	_, err := t.store.Load(name)
	if err == nil {
		return nil, ErrPlanAlreadyExists
	}

	tree, err := CreateTree(name, title, description)
	if err != nil {
		return nil, fmt.Errorf("create tree: %w", err)
	}

	if err := t.store.Save(*tree); err != nil {
		return nil, fmt.Errorf("save tree: %w", err)
	}

	data, _ := json.Marshal(tree.Summary())
	var result map[string]interface{}
	json.Unmarshal(data, &result)
	return result, nil
}

type PlanBranchTool struct {
	store Store
	approval.DefaultLevelEdit
}

func NewPlanBranchTool(store Store) *PlanBranchTool {
	return &PlanBranchTool{store: store}
}

func (t *PlanBranchTool) Name() string { return "plan_branch" }
func (t *PlanBranchTool) Description() string {
	return tr(context.Background(), "internal_plantree_tool_branch_description", nil)
}
func (t *PlanBranchTool) Category() tools.ToolCategory {
	return tools.ToolCategory("plan")
}

func (t *PlanBranchTool) Schema() tools.ToolSchema {
	return tools.ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"plan_name":      map[string]interface{}{"type": "string", "description": "Plan name"},
			"parent_node_id": map[string]interface{}{"type": "string", "description": "Parent node ID"},
			"title":          map[string]interface{}{"type": "string", "description": "Child node title"},
			"description":    map[string]interface{}{"type": "string", "description": "Child node description"},
		},
		Required: []string{"plan_name", "parent_node_id", "title", "description"},
	}
}

func (t *PlanBranchTool) Validate(params map[string]interface{}) error {
	if _, ok := params["plan_name"].(string); !ok || params["plan_name"].(string) == "" {
		return errors.New("plan_name is required")
	}
	if _, ok := params["parent_node_id"].(string); !ok || params["parent_node_id"].(string) == "" {
		return errors.New("parent_node_id is required")
	}
	return nil
}

func (t *PlanBranchTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	planName := params["plan_name"].(string)
	parentID := params["parent_node_id"].(string)
	title := params["title"].(string)
	description := params["description"].(string)

	tree, err := t.store.Load(planName)
	if err != nil {
		return nil, err
	}

	child, err := BranchNode(&tree, parentID, title, description)
	if err != nil {
		return nil, err
	}

	if err := t.store.Save(tree); err != nil {
		return nil, fmt.Errorf("save tree: %w", err)
	}

	data, _ := json.Marshal(child)
	var result map[string]interface{}
	json.Unmarshal(data, &result)
	result["plan_tree_hint"] = "use plan_show to view the full tree; use plan_merge when a branch is complete"
	return result, nil
}

type PlanMergeTool struct {
	store Store
	approval.DefaultLevelEdit
}

func NewPlanMergeTool(store Store) *PlanMergeTool {
	return &PlanMergeTool{store: store}
}

func (t *PlanMergeTool) Name() string { return "plan_merge" }
func (t *PlanMergeTool) Description() string {
	return tr(context.Background(), "internal_plantree_tool_merge_description", nil)
}
func (t *PlanMergeTool) Category() tools.ToolCategory {
	return tools.ToolCategory("plan")
}

func (t *PlanMergeTool) Schema() tools.ToolSchema {
	return tools.ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"plan_name":    map[string]interface{}{"type": "string", "description": "Plan name"},
			"child_node_id": map[string]interface{}{"type": "string", "description": "Child node ID to merge"},
		},
		Required: []string{"plan_name", "child_node_id"},
	}
}

func (t *PlanMergeTool) Validate(params map[string]interface{}) error {
	if _, ok := params["plan_name"].(string); !ok || params["plan_name"].(string) == "" {
		return errors.New("plan_name is required")
	}
	if _, ok := params["child_node_id"].(string); !ok || params["child_node_id"].(string) == "" {
		return errors.New("child_node_id is required")
	}
	return nil
}

func (t *PlanMergeTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	planName := params["plan_name"].(string)
	childID := params["child_node_id"].(string)

	tree, err := t.store.Load(planName)
	if err != nil {
		return nil, err
	}

	parent, err := MergeNode(&tree, childID)
	if err != nil {
		return nil, err
	}

	if err := t.store.Save(tree); err != nil {
		return nil, fmt.Errorf("save tree: %w", err)
	}

	data, _ := json.Marshal(parent)
	var result map[string]interface{}
	json.Unmarshal(data, &result)
	return result, nil
}

type PlanListTool struct {
	store Store
	approval.DefaultLevelEdit
}

func NewPlanListTool(store Store) *PlanListTool {
	return &PlanListTool{store: store}
}

func (t *PlanListTool) Name() string { return "plan_list" }
func (t *PlanListTool) Description() string {
	return tr(context.Background(), "internal_plantree_tool_list_description", nil)
}
func (t *PlanListTool) Category() tools.ToolCategory {
	return tools.ToolCategory("plan")
}
func (t *PlanListTool) RequiresApproval() approval.ApprovalLevel { return approval.LevelReadOnly }

func (t *PlanListTool) Schema() tools.ToolSchema {
	return tools.ToolSchema{
		Type:       "object",
		Properties: map[string]interface{}{},
		Required:   []string{},
	}
}

func (t *PlanListTool) Validate(params map[string]interface{}) error {
	return nil
}

func (t *PlanListTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	summaries, err := t.store.List()
	if err != nil {
		return nil, fmt.Errorf("list plans: %w", err)
	}

	if summaries == nil {
		summaries = []PlanTreeSummary{}
	}

	return map[string]interface{}{"plans": summaries}, nil
}

type PlanShowTool struct {
	store Store
	approval.DefaultLevelEdit
}

func NewPlanShowTool(store Store) *PlanShowTool {
	return &PlanShowTool{store: store}
}

func (t *PlanShowTool) Name() string { return "plan_show" }
func (t *PlanShowTool) Description() string {
	return tr(context.Background(), "internal_plantree_tool_show_description", nil)
}
func (t *PlanShowTool) Category() tools.ToolCategory {
	return tools.ToolCategory("plan")
}
func (t *PlanShowTool) RequiresApproval() approval.ApprovalLevel { return approval.LevelReadOnly }

func (t *PlanShowTool) Schema() tools.ToolSchema {
	return tools.ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"plan_name": map[string]interface{}{"type": "string", "description": "Plan name"},
		},
		Required: []string{"plan_name"},
	}
}

func (t *PlanShowTool) Validate(params map[string]interface{}) error {
	if _, ok := params["plan_name"].(string); !ok || params["plan_name"].(string) == "" {
		return errors.New("plan_name is required")
	}
	return nil
}

func (t *PlanShowTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	planName := params["plan_name"].(string)

	tree, err := t.store.Load(planName)
	if err != nil {
		return nil, err
	}

	output := RenderTree(tree.Root, 0)
	return map[string]interface{}{"tree": output, "plan_name": planName}, nil
}

type PlanDeleteTool struct {
	store Store
	approval.DefaultLevelEdit
}

func NewPlanDeleteTool(store Store) *PlanDeleteTool {
	return &PlanDeleteTool{store: store}
}

func (t *PlanDeleteTool) Name() string { return "plan_delete" }
func (t *PlanDeleteTool) Description() string {
	return tr(context.Background(), "internal_plantree_tool_delete_description", nil)
}
func (t *PlanDeleteTool) Category() tools.ToolCategory {
	return tools.ToolCategory("plan")
}

func (t *PlanDeleteTool) Schema() tools.ToolSchema {
	return tools.ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"plan_name": map[string]interface{}{"type": "string", "description": "Plan name to delete"},
		},
		Required: []string{"plan_name"},
	}
}

func (t *PlanDeleteTool) Validate(params map[string]interface{}) error {
	if _, ok := params["plan_name"].(string); !ok || params["plan_name"].(string) == "" {
		return errors.New("plan_name is required")
	}
	return nil
}

func (t *PlanDeleteTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	planName := params["plan_name"].(string)

	_, err := t.store.Load(planName)
	if err != nil {
		return nil, err
	}

	if err := t.store.Delete(planName); err != nil {
		return nil, fmt.Errorf("delete plan: %w", err)
	}

	return map[string]interface{}{"deleted": planName}, nil
}
