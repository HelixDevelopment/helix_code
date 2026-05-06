package commands

import (
	"context"
	"strings"
	"testing"
	"time"

	"dev.helix.code/internal/plantree"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockPlanStore struct {
	plans map[string]plantree.PlanTree
}

func newMockPlanStore() *mockPlanStore {
	return &mockPlanStore{plans: make(map[string]plantree.PlanTree)}
}

func (m *mockPlanStore) Save(tree plantree.PlanTree) error {
	m.plans[tree.Name] = tree
	return nil
}

func (m *mockPlanStore) Load(name string) (plantree.PlanTree, error) {
	tree, ok := m.plans[name]
	if !ok {
		return plantree.PlanTree{}, plantree.ErrPlanNotFound
	}
	return tree, nil
}

func (m *mockPlanStore) List() ([]plantree.PlanTreeSummary, error) {
	var summaries []plantree.PlanTreeSummary
	for _, tree := range m.plans {
		summaries = append(summaries, tree.Summary())
	}
	if summaries == nil {
		summaries = []plantree.PlanTreeSummary{}
	}
	return summaries, nil
}

func (m *mockPlanStore) Delete(name string) error {
	delete(m.plans, name)
	return nil
}

type mockPlanSummariser struct {
	summary string
	err     error
}

func (m *mockPlanSummariser) Summarise(text string, maxWords int) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.summary, nil
}

func makeTestTree() plantree.PlanTree {
	now := time.Now().UTC()
	root := &plantree.PlanNode{
		ID:          "root-1",
		Title:       "Test Plan",
		Description: "A test plan",
		Status:      plantree.StatusPending,
		CreatedAt:   now,
		UpdatedAt:   now,
		Children: []*plantree.PlanNode{
			{
				ID:          "task-1",
				Title:       "Task One",
				Description: "First task",
				Status:      plantree.StatusInProgress,
				ParentID:    "root-1",
				CreatedAt:   now,
				UpdatedAt:   now,
			},
		},
	}
	return plantree.PlanTree{
		Name:      "test-plan",
		Version:   1,
		Root:      root,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func TestPlanCommand_ListEmpty(t *testing.T) {
	store := newMockPlanStore()
	cmd := NewPlanCommand(store, &mockPlanSummariser{summary: "summary"})

	result, err := cmd.Execute(context.Background(), &CommandContext{})
	require.NoError(t, err)

	assert.True(t, result.Success)
	assert.Contains(t, result.Message, "No plans found")
}

func TestPlanCommand_ListWithPlans(t *testing.T) {
	store := newMockPlanStore()
	store.Save(makeTestTree())
	cmd := NewPlanCommand(store, &mockPlanSummariser{summary: "summary"})

	result, err := cmd.Execute(context.Background(), &CommandContext{})
	require.NoError(t, err)

	assert.True(t, result.Success)
	assert.Contains(t, result.Message, "1 plan")
	assert.Contains(t, result.Output, "test-plan")
}

func TestPlanCommand_Show(t *testing.T) {
	store := newMockPlanStore()
	store.Save(makeTestTree())
	cmd := NewPlanCommand(store, &mockPlanSummariser{summary: "summary"})

	result, err := cmd.Execute(context.Background(), &CommandContext{
		Args:  []string{"show", "test-plan"},
		Flags: map[string]string{},
	})
	require.NoError(t, err)

	assert.True(t, result.Success)
	assert.Contains(t, result.Output, "Test Plan")
	assert.Contains(t, result.Output, "Task One")
	assert.Contains(t, result.Output, "[ ]")
	assert.Contains(t, result.Output, "[▶]")
}

func TestPlanCommand_Show_NotFound(t *testing.T) {
	store := newMockPlanStore()
	cmd := NewPlanCommand(store, &mockPlanSummariser{summary: "summary"})

	result, err := cmd.Execute(context.Background(), &CommandContext{
		Args: []string{"show", "nonexistent"},
	})
	require.NoError(t, err)

	assert.False(t, result.Success)
	assert.Contains(t, result.Message, "plan not found")
}

func TestPlanCommand_Show_Subtree(t *testing.T) {
	store := newMockPlanStore()
	store.Save(makeTestTree())
	cmd := NewPlanCommand(store, &mockPlanSummariser{summary: "summary"})

	result, err := cmd.Execute(context.Background(), &CommandContext{
		Args:  []string{"show", "test-plan"},
		Flags: map[string]string{"id": "task-1"},
	})
	require.NoError(t, err)

	assert.True(t, result.Success)
	assert.Contains(t, result.Output, "Task One")
	assert.Contains(t, result.Output, "subtree")
	assert.NotContains(t, result.Output, "Test Plan")
}

func TestPlanCommand_Compact(t *testing.T) {
	store := newMockPlanStore()
	tree := makeTestTree()
	store.Save(tree)
	cmd := NewPlanCommand(store, &mockPlanSummariser{summary: "summary"})

	result, err := cmd.Execute(context.Background(), &CommandContext{
		Args: []string{"compact", "test-plan"},
	})
	require.NoError(t, err)

	assert.True(t, result.Success)
	assert.True(t, strings.Contains(result.Message, "no compaction needed") || strings.Contains(result.Message, "compacted"))
}

func TestPlanCommand_Verify_Clean(t *testing.T) {
	store := newMockPlanStore()
	store.Save(makeTestTree())
	cmd := NewPlanCommand(store, &mockPlanSummariser{summary: "summary"})

	result, err := cmd.Execute(context.Background(), &CommandContext{
		Args: []string{"verify", "test-plan"},
	})
	require.NoError(t, err)

	assert.True(t, result.Success)
	assert.Contains(t, result.Message, "valid")
}

func TestPlanCommand_Verify_Corrupt(t *testing.T) {
	store := newMockPlanStore()
	tree := makeTestTree()
	tree.Root.Children[0].ParentID = "nonexistent-parent"
	store.Save(tree)
	cmd := NewPlanCommand(store, &mockPlanSummariser{summary: "summary"})

	result, err := cmd.Execute(context.Background(), &CommandContext{
		Args: []string{"verify", "test-plan"},
	})
	require.NoError(t, err)

	assert.True(t, result.Success)
	assert.Contains(t, result.Output, "ERROR")
	assert.Contains(t, result.Output, "orphan")
}

func TestPlanCommand_DefaultToList(t *testing.T) {
	store := newMockPlanStore()
	cmd := NewPlanCommand(store, &mockPlanSummariser{summary: "summary"})

	result, err := cmd.Execute(context.Background(), &CommandContext{})
	require.NoError(t, err)

	assert.True(t, result.Success)
	assert.Contains(t, result.Message, "No plans found")
}

func TestPlanCommand_UnknownSubcommand(t *testing.T) {
	store := newMockPlanStore()
	cmd := NewPlanCommand(store, &mockPlanSummariser{summary: "summary"})

	result, err := cmd.Execute(context.Background(), &CommandContext{
		Args: []string{"bogus"},
	})
	require.NoError(t, err)

	assert.False(t, result.Success)
	assert.Contains(t, result.Message, "unknown subcommand")
}
