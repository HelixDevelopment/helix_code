package worktree

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnterWorktreeTool_NameDescriptionCategory(t *testing.T) {
	repo := initEphemeralRepo(t)
	m := NewManager(repo)
	tool := NewEnterWorktreeTool(m)

	assert.Equal(t, "EnterWorktree", tool.Name())
	assert.NotEmpty(t, tool.Description())
	assert.Contains(t, tool.Description(), "Submodules are NOT initialised",
		"description must teach the LLM about the submodule omission")
	assert.NotEmpty(t, tool.Category())
}

func TestEnterWorktreeTool_Schema(t *testing.T) {
	repo := initEphemeralRepo(t)
	m := NewManager(repo)
	tool := NewEnterWorktreeTool(m)

	schema := tool.Schema()
	assert.Equal(t, "object", schema.Type)
	props, ok := schema.Properties["name"]
	require.True(t, ok)
	assert.NotNil(t, props)
	assert.Contains(t, schema.Required, "name")
	assert.NotContains(t, schema.Required, "baseBranch")
}

func TestEnterWorktreeTool_Validate(t *testing.T) {
	repo := initEphemeralRepo(t)
	m := NewManager(repo)
	tool := NewEnterWorktreeTool(m)

	assert.NoError(t, tool.Validate(map[string]interface{}{"name": "feature-x"}))
	assert.Error(t, tool.Validate(map[string]interface{}{}), "missing name must error")
	assert.Error(t, tool.Validate(map[string]interface{}{"name": 42}), "wrong type must error")
}

func TestEnterWorktreeTool_Execute(t *testing.T) {
	repo := initEphemeralRepo(t)
	m := NewManager(repo)
	tool := NewEnterWorktreeTool(m)

	res, err := tool.Execute(context.Background(), map[string]interface{}{"name": "feature-y"})
	require.NoError(t, err)
	resMap, ok := res.(map[string]interface{})
	require.True(t, ok)
	path, ok := resMap["path"].(string)
	require.True(t, ok)
	assert.Equal(t, filepath.Join(repo, WorktreeDir, "feature-y"), path)
	assert.True(t, m.IsIsolated())
}

func TestExitWorktreeTool_Execute(t *testing.T) {
	repo := initEphemeralRepo(t)
	m := NewManager(repo)
	enter := NewEnterWorktreeTool(m)
	exit := NewExitWorktreeTool(m)

	_, err := enter.Execute(context.Background(), map[string]interface{}{"name": "feature-z"})
	require.NoError(t, err)
	require.True(t, m.IsIsolated())

	res, err := exit.Execute(context.Background(), map[string]interface{}{})
	require.NoError(t, err)
	resMap := res.(map[string]interface{})
	assert.Equal(t, true, resMap["exited"])
	assert.False(t, m.IsIsolated())
}

func TestListWorktreesTool_Execute(t *testing.T) {
	repo := initEphemeralRepo(t)
	m := NewManager(repo)
	enter := NewEnterWorktreeTool(m)
	list := NewListWorktreesTool(m)

	_, err := enter.Execute(context.Background(), map[string]interface{}{"name": "feature-a"})
	require.NoError(t, err)

	res, err := list.Execute(context.Background(), map[string]interface{}{})
	require.NoError(t, err)
	resMap := res.(map[string]interface{})
	wts, ok := resMap["worktrees"].([]Worktree)
	require.True(t, ok)
	require.NotEmpty(t, wts)

	names := []string{}
	for _, w := range wts {
		names = append(names, w.Name)
	}
	assert.Contains(t, names, "feature-a")
}

func TestRemoveWorktreeTool_Execute(t *testing.T) {
	repo := initEphemeralRepo(t)
	m := NewManager(repo)
	enter := NewEnterWorktreeTool(m)
	exit := NewExitWorktreeTool(m)
	remove := NewRemoveWorktreeTool(m)

	_, err := enter.Execute(context.Background(), map[string]interface{}{"name": "feature-b"})
	require.NoError(t, err)
	_, err = exit.Execute(context.Background(), map[string]interface{}{})
	require.NoError(t, err)

	res, err := remove.Execute(context.Background(), map[string]interface{}{"name": "feature-b"})
	require.NoError(t, err)
	resMap := res.(map[string]interface{})
	assert.Equal(t, true, resMap["removed"])

	_, statErr := os.Stat(filepath.Join(repo, WorktreeDir, "feature-b"))
	assert.True(t, os.IsNotExist(statErr))
}

func TestEnterWorktreeTool_DescriptionMentionsBaseBranch(t *testing.T) {
	repo := initEphemeralRepo(t)
	m := NewManager(repo)
	tool := NewEnterWorktreeTool(m)
	desc := tool.Description()
	assert.True(t, strings.Contains(desc, "branch") || strings.Contains(desc, "Branch"),
		"description must explain the optional base-branch parameter")
}
