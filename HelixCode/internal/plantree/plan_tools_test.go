package plantree

import (
	"context"
	"testing"

	"dev.helix.code/internal/approval"
	"dev.helix.code/internal/tools"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func tempDirStore(t *testing.T) Store {
	t.Helper()
	return NewFileStore(t.TempDir())
}

func TestPlanCreateTool(t *testing.T) {
	store := tempDirStore(t)
	tool := NewPlanCreateTool(store)

	params := map[string]interface{}{
		"name":        "my-plan",
		"title":       "Test Plan",
		"description": "A test plan",
	}

	result, err := tool.Execute(context.Background(), params)
	require.NoError(t, err)

	resultMap := result.(map[string]interface{})
	assert.Equal(t, "my-plan", resultMap["name"])
	assert.Equal(t, float64(1), resultMap["node_count"])

	_, err = store.Load("my-plan")
	require.NoError(t, err)
}

func TestPlanCreateTool_DuplicateName(t *testing.T) {
	store := tempDirStore(t)
	tool := NewPlanCreateTool(store)

	params := map[string]interface{}{
		"name":        "dup-plan",
		"title":       "Plan",
		"description": "Desc",
	}

	_, err := tool.Execute(context.Background(), params)
	require.NoError(t, err)

	_, err = tool.Execute(context.Background(), params)
	assert.ErrorIs(t, err, ErrPlanAlreadyExists)
}

func TestPlanBranchTool(t *testing.T) {
	store := tempDirStore(t)
	createTool := NewPlanCreateTool(store)
	branchTool := NewPlanBranchTool(store)

	_, err := createTool.Execute(context.Background(), map[string]interface{}{
		"name": "branch-plan", "title": "Root", "description": "Root desc",
	})
	require.NoError(t, err)

	tree, _ := store.Load("branch-plan")
	rootID := tree.Root.ID

	result, err := branchTool.Execute(context.Background(), map[string]interface{}{
		"plan_name":      "branch-plan",
		"parent_node_id": rootID,
		"title":          "Child Task",
		"description":    "A child",
	})
	require.NoError(t, err)

	resultMap := result.(map[string]interface{})
	assert.NotEmpty(t, resultMap["id"])
	assert.Equal(t, rootID, resultMap["parent_id"])
	assert.Contains(t, resultMap["plan_tree_hint"], "plan_merge")
}

func TestPlanBranchTool_ParentNotFound(t *testing.T) {
	store := tempDirStore(t)
	createTool := NewPlanCreateTool(store)
	branchTool := NewPlanBranchTool(store)

	_, err := createTool.Execute(context.Background(), map[string]interface{}{
		"name": "p-plan", "title": "Root", "description": "Root",
	})
	require.NoError(t, err)

	_, err = branchTool.Execute(context.Background(), map[string]interface{}{
		"plan_name":      "p-plan",
		"parent_node_id": "nonexistent",
		"title":          "Child",
		"description":    "Desc",
	})
	assert.ErrorIs(t, err, ErrNodeNotFound)
}

func TestPlanMergeTool(t *testing.T) {
	store := tempDirStore(t)
	createTool := NewPlanCreateTool(store)
	branchTool := NewPlanBranchTool(store)
	mergeTool := NewPlanMergeTool(store)

	_, err := createTool.Execute(context.Background(), map[string]interface{}{
		"name": "merge-plan", "title": "Root", "description": "Root",
	})
	require.NoError(t, err)

	tree, _ := store.Load("merge-plan")

	branchResult, err := branchTool.Execute(context.Background(), map[string]interface{}{
		"plan_name": "merge-plan", "parent_node_id": tree.Root.ID,
		"title": "Task 1", "description": "First task",
	})
	require.NoError(t, err)
	childID := branchResult.(map[string]interface{})["id"].(string)

	result, err := mergeTool.Execute(context.Background(), map[string]interface{}{
		"plan_name": "merge-plan", "child_node_id": childID,
	})
	require.NoError(t, err)

	resultMap := result.(map[string]interface{})
	assert.NotEmpty(t, resultMap["metadata"])

	updatedTree, _ := store.Load("merge-plan")
	mergedChild := findNode(updatedTree.Root, childID)
	require.NotNil(t, mergedChild)
	assert.Equal(t, StatusCompleted, mergedChild.Status)
}

func TestPlanListTool(t *testing.T) {
	store := tempDirStore(t)
	createTool := NewPlanCreateTool(store)
	listTool := NewPlanListTool(store)

	_, err := createTool.Execute(context.Background(), map[string]interface{}{
		"name": "plan-a", "title": "Plan A", "description": "First",
	})
	require.NoError(t, err)
	_, err = createTool.Execute(context.Background(), map[string]interface{}{
		"name": "plan-b", "title": "Plan B", "description": "Second",
	})
	require.NoError(t, err)

	result, err := listTool.Execute(context.Background(), map[string]interface{}{})
	require.NoError(t, err)

	resultMap := result.(map[string]interface{})
	plans := resultMap["plans"]
	assert.NotNil(t, plans)
}

func TestPlanShowTool(t *testing.T) {
	store := tempDirStore(t)
	createTool := NewPlanCreateTool(store)
	branchTool := NewPlanBranchTool(store)
	showTool := NewPlanShowTool(store)

	_, err := createTool.Execute(context.Background(), map[string]interface{}{
		"name": "show-plan", "title": "Root Plan", "description": "Root",
	})
	require.NoError(t, err)

	tree, _ := store.Load("show-plan")
	_, err = branchTool.Execute(context.Background(), map[string]interface{}{
		"plan_name": "show-plan", "parent_node_id": tree.Root.ID,
		"title": "Child", "description": "Child task",
	})
	require.NoError(t, err)

	result, err := showTool.Execute(context.Background(), map[string]interface{}{
		"plan_name": "show-plan",
	})
	require.NoError(t, err)

	resultMap := result.(map[string]interface{})
	treeStr := resultMap["tree"].(string)
	assert.Contains(t, treeStr, "Root Plan")
	assert.Contains(t, treeStr, "Child")
	assert.Contains(t, treeStr, "[ ]")
}

func TestPlanDeleteTool(t *testing.T) {
	store := tempDirStore(t)
	createTool := NewPlanCreateTool(store)
	deleteTool := NewPlanDeleteTool(store)

	_, err := createTool.Execute(context.Background(), map[string]interface{}{
		"name": "delete-me", "title": "Del", "description": "Del",
	})
	require.NoError(t, err)

	result, err := deleteTool.Execute(context.Background(), map[string]interface{}{
		"plan_name": "delete-me",
	})
	require.NoError(t, err)

	resultMap := result.(map[string]interface{})
	assert.Equal(t, "delete-me", resultMap["deleted"])

	_, err = store.Load("delete-me")
	assert.ErrorIs(t, err, ErrPlanNotFound)
}

func TestPlanDeleteTool_NotFound(t *testing.T) {
	store := tempDirStore(t)
	deleteTool := NewPlanDeleteTool(store)

	_, err := deleteTool.Execute(context.Background(), map[string]interface{}{
		"plan_name": "nonexistent",
	})
	assert.ErrorIs(t, err, ErrPlanNotFound)
}

func TestAllPlanTools_RequiresApproval(t *testing.T) {
	store := tempDirStore(t)

	tests := []struct {
		tool  tools.Tool
		level approval.ApprovalLevel
	}{
		{NewPlanCreateTool(store), approval.LevelEdit},
		{NewPlanBranchTool(store), approval.LevelEdit},
		{NewPlanMergeTool(store), approval.LevelEdit},
		{NewPlanListTool(store), approval.LevelReadOnly},
		{NewPlanShowTool(store), approval.LevelReadOnly},
		{NewPlanDeleteTool(store), approval.LevelEdit},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.level, tt.tool.RequiresApproval(), "tool %s", tt.tool.Name())
	}
}

func TestAllPlanTools_CategoryName(t *testing.T) {
	store := tempDirStore(t)
	allTools := []tools.Tool{
		NewPlanCreateTool(store),
		NewPlanBranchTool(store),
		NewPlanMergeTool(store),
		NewPlanListTool(store),
		NewPlanShowTool(store),
		NewPlanDeleteTool(store),
	}

	for _, tool := range allTools {
		assert.NotEmpty(t, tool.Name())
		assert.NotEmpty(t, tool.Description())
		assert.Equal(t, tools.ToolCategory("plan"), tool.Category())

		schema := tool.Schema()
		assert.Equal(t, "object", schema.Type)
	}
}
