package workspace

import (
	"context"
	"encoding/json"
	"errors"

	"dev.helix.code/internal/approval"
	"dev.helix.code/internal/tools"
)

type WorkspaceCreateTool struct {
	mgr *WorkspaceManager
	approval.DefaultLevelEdit
}

func NewWorkspaceCreateTool(mgr *WorkspaceManager) *WorkspaceCreateTool {
	return &WorkspaceCreateTool{mgr: mgr}
}

func (t *WorkspaceCreateTool) Name() string        { return "workspace_create" }
func (t *WorkspaceCreateTool) Description() string { return "Create a new container-based workspace" }
func (t *WorkspaceCreateTool) Category() tools.ToolCategory {
	return tools.ToolCategory("workspace")
}

func (t *WorkspaceCreateTool) Schema() tools.ToolSchema {
	return tools.ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"name":        map[string]interface{}{"type": "string", "description": "Workspace name"},
			"image":       map[string]interface{}{"type": "string", "description": "Container image (default: alpine:latest)"},
			"project_dir": map[string]interface{}{"type": "string", "description": "Host project directory to mount"},
		},
		Required: []string{"name", "project_dir"},
	}
}

func (t *WorkspaceCreateTool) Validate(params map[string]interface{}) error {
	if _, ok := params["name"].(string); !ok || params["name"].(string) == "" {
		return errors.New("name is required")
	}
	if _, ok := params["project_dir"].(string); !ok || params["project_dir"].(string) == "" {
		return errors.New("project_dir is required")
	}
	return nil
}

func (t *WorkspaceCreateTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	name := params["name"].(string)
	projectDir := params["project_dir"].(string)
	image, _ := params["image"].(string)

	ws, err := t.mgr.CreateWorkspace(ctx, name, image, projectDir)
	if err != nil {
		return nil, err
	}

	data, _ := json.Marshal(ws)
	var result map[string]interface{}
	json.Unmarshal(data, &result)
	return result, nil
}

type WorkspaceListTool struct {
	mgr *WorkspaceManager
	approval.DefaultLevelEdit
}

func NewWorkspaceListTool(mgr *WorkspaceManager) *WorkspaceListTool {
	return &WorkspaceListTool{mgr: mgr}
}

func (t *WorkspaceListTool) Name() string        { return "workspace_list" }
func (t *WorkspaceListTool) Description() string { return "List all workspaces" }
func (t *WorkspaceListTool) Category() tools.ToolCategory {
	return tools.ToolCategory("workspace")
}
func (t *WorkspaceListTool) RequiresApproval() approval.ApprovalLevel { return approval.LevelReadOnly }

func (t *WorkspaceListTool) Schema() tools.ToolSchema {
	return tools.ToolSchema{
		Type:       "object",
		Properties: map[string]interface{}{},
		Required:   []string{},
	}
}

func (t *WorkspaceListTool) Validate(params map[string]interface{}) error { return nil }

func (t *WorkspaceListTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	list := t.mgr.ListWorkspaces()
	data, _ := json.Marshal(list)
	var result []map[string]interface{}
	json.Unmarshal(data, &result)
	if result == nil {
		result = []map[string]interface{}{}
	}
	return result, nil
}

type WorkspaceCleanupTool struct {
	mgr *WorkspaceManager
	approval.DefaultLevelEdit
}

func NewWorkspaceCleanupTool(mgr *WorkspaceManager) *WorkspaceCleanupTool {
	return &WorkspaceCleanupTool{mgr: mgr}
}

func (t *WorkspaceCleanupTool) Name() string        { return "workspace_cleanup" }
func (t *WorkspaceCleanupTool) Description() string { return "Stop and remove a workspace container" }
func (t *WorkspaceCleanupTool) Category() tools.ToolCategory {
	return tools.ToolCategory("workspace")
}

func (t *WorkspaceCleanupTool) Schema() tools.ToolSchema {
	return tools.ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"id": map[string]interface{}{"type": "string", "description": "Workspace ID"},
		},
		Required: []string{"id"},
	}
}

func (t *WorkspaceCleanupTool) Validate(params map[string]interface{}) error {
	if _, ok := params["id"].(string); !ok || params["id"].(string) == "" {
		return errors.New("id is required")
	}
	return nil
}

func (t *WorkspaceCleanupTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	id := params["id"].(string)
	if err := t.mgr.CleanupWorkspace(ctx, id); err != nil {
		return nil, err
	}
	return map[string]interface{}{"status": "cleaned", "id": id}, nil
}
