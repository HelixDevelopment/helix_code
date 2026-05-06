package plantree

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlanStatus_String(t *testing.T) {
	tests := []struct {
		status   PlanStatus
		expected string
	}{
		{StatusPending, "pending"},
		{StatusInProgress, "in_progress"},
		{StatusCompleted, "completed"},
		{StatusFailed, "failed"},
		{StatusPruned, "pruned"},
		{PlanStatus(99), "unknown"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, tt.status.String(), "status %d", tt.status)
	}
}

func TestPlanStatus_Marker(t *testing.T) {
	tests := []struct {
		status   PlanStatus
		expected string
	}{
		{StatusPending, "[ ]"},
		{StatusInProgress, "[▶]"},
		{StatusCompleted, "[✓]"},
		{StatusFailed, "[✗]"},
		{StatusPruned, "[×]"},
		{PlanStatus(99), "[?]"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, tt.status.Marker(), "status %d", tt.status)
	}
}

func TestPlanStatus_JSONRoundtrip(t *testing.T) {
	statuses := []PlanStatus{StatusPending, StatusInProgress, StatusCompleted, StatusFailed, StatusPruned}

	for _, status := range statuses {
		data, err := json.Marshal(status)
		require.NoError(t, err)

		var decoded PlanStatus
		err = json.Unmarshal(data, &decoded)
		require.NoError(t, err)

		assert.Equal(t, status, decoded, "status %s", status.String())
	}
}

func TestPlanStatus_UnmarshalJSON_Unknown(t *testing.T) {
	var s PlanStatus
	err := json.Unmarshal([]byte(`"bogus"`), &s)
	assert.Error(t, err)
}

func TestPlanNode_Creation(t *testing.T) {
	now := time.Now().UTC()
	node := &PlanNode{
		ID:          "node-1",
		Title:       "Test Node",
		Description: "A test node",
		Status:      StatusPending,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	assert.Equal(t, "node-1", node.ID)
	assert.Equal(t, "Test Node", node.Title)
	assert.Equal(t, "A test node", node.Description)
	assert.Equal(t, StatusPending, node.Status)
	assert.Nil(t, node.Children)
	assert.Empty(t, node.ParentID)
	assert.Nil(t, node.Metadata)
}

func TestPlanTree_Creation(t *testing.T) {
	now := time.Now().UTC()
	root := &PlanNode{
		ID:    "root-1",
		Title: "Root Plan",
	}

	tree := &PlanTree{
		Name:      "my-plan",
		Version:   1,
		Root:      root,
		CreatedAt: now,
		UpdatedAt: now,
	}

	assert.Equal(t, "my-plan", tree.Name)
	assert.Equal(t, 1, tree.Version)
	assert.Equal(t, root, tree.Root)
	assert.Equal(t, "Root Plan", tree.Root.Title)
}

func TestRenderTree_SingleNode(t *testing.T) {
	node := &PlanNode{
		ID:     "abc123",
		Title:  "Test Plan",
		Status: StatusPending,
	}

	output := RenderTree(node, 0)

	assert.Contains(t, output, "[ ]")
	assert.Contains(t, output, "Test Plan")
	assert.Contains(t, output, "(abc123)")
	assert.NotContains(t, output, CompactMarker)
}

func TestRenderTree_MultiLevel(t *testing.T) {
	root := &PlanNode{
		ID:     "root",
		Title:  "Root",
		Status: StatusPending,
		Children: []*PlanNode{
			{
				ID:     "child1",
				Title:  "Child One",
				Status: StatusInProgress,
				Children: []*PlanNode{
					{
						ID:     "grandchild",
						Title:  "Grandchild",
						Status: StatusPending,
					},
				},
			},
			{
				ID:     "child2",
				Title:  "Child Two",
				Status: StatusCompleted,
			},
		},
	}

	output := RenderTree(root, 0)

	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	require.Len(t, lines, 4)

	assert.Contains(t, lines[0], "[ ]")
	assert.Contains(t, lines[0], "Root (root)")
	assert.True(t, strings.HasPrefix(lines[0], "[ ]"), "root should be at indent 0")

	assert.Contains(t, lines[1], "[▶]")
	assert.Contains(t, lines[1], "Child One (child1)")
	assert.True(t, strings.HasPrefix(lines[1], "  [▶]"), "child1 at indent 2")

	assert.Contains(t, lines[2], "[ ]")
	assert.Contains(t, lines[2], "Grandchild (grandchild)")
	assert.True(t, strings.HasPrefix(lines[2], "    [ ]"), "grandchild at indent 4")

	assert.Contains(t, lines[3], "[✓]")
	assert.Contains(t, lines[3], "Child Two (child2)")
	assert.True(t, strings.HasPrefix(lines[3], "  [✓]"), "child2 at indent 2")
}

func TestRenderTree_CompactedNode(t *testing.T) {
	node := &PlanNode{
		ID:     "compacted-1",
		Title:  "Compacted Task",
		Status: StatusCompleted,
		Metadata: map[string]string{
			"compacted": "true",
		},
	}

	output := RenderTree(node, 0)

	assert.Contains(t, output, CompactMarker)
	assert.Contains(t, output, "[c]")
}

func TestRenderTree_NilNode(t *testing.T) {
	output := RenderTree(nil, 0)
	assert.Empty(t, output)
}

func TestCountNodes(t *testing.T) {
	root := &PlanNode{
		ID: "root",
		Children: []*PlanNode{
			{
				ID: "c1",
				Children: []*PlanNode{
					{ID: "c1a"},
					{ID: "c1b"},
				},
			},
			{ID: "c2"},
			{ID: "c3"},
		},
	}

	assert.Equal(t, 6, CountNodes(root))
	assert.Equal(t, 0, CountNodes(nil))
}

func TestMaxDepth(t *testing.T) {
	root := &PlanNode{
		ID: "root",
		Children: []*PlanNode{
			{
				ID: "c1",
				Children: []*PlanNode{
					{
						ID: "c1a",
						Children: []*PlanNode{
							{ID: "c1a1"},
						},
					},
				},
			},
			{ID: "c2"},
		},
	}

	assert.Equal(t, 4, MaxDepth(root))
	assert.Equal(t, 0, MaxDepth(nil))
}

func TestPlanTree_Summary(t *testing.T) {
	now := time.Now().UTC()
	root := &PlanNode{
		ID:     "root-1",
		Title:  "Implementation Plan",
		Status: StatusPending,
		Children: []*PlanNode{
			{ID: "task-1", Title: "Task 1"},
			{ID: "task-2", Title: "Task 2"},
		},
	}

	tree := &PlanTree{
		Name:      "my-plan",
		Version:   1,
		Root:      root,
		CreatedAt: now,
		UpdatedAt: now,
	}

	summary := tree.Summary()

	assert.Equal(t, "my-plan", summary.Name)
	assert.Equal(t, 3, summary.NodeCount)
	assert.Equal(t, "root-1", summary.RootID)
	assert.Equal(t, "Implementation Plan", summary.RootTitle)
	assert.Equal(t, now, summary.CreatedAt)
	assert.Equal(t, now, summary.UpdatedAt)
}

func TestPlanTree_Summary_Nil(t *testing.T) {
	var tree *PlanTree
	assert.Equal(t, PlanTreeSummary{}, tree.Summary())
}

func TestPlanTree_Summary_NilRoot(t *testing.T) {
	tree := &PlanTree{Name: "empty"}
	assert.Equal(t, PlanTreeSummary{}, tree.Summary())
}

func TestSentinelErrors(t *testing.T) {
	assert.Error(t, ErrPlanNotFound)
	assert.Error(t, ErrNodeNotFound)
	assert.Error(t, ErrCyclicMerge)
	assert.Error(t, ErrPlanAlreadyExists)
	assert.Error(t, ErrTooManyNodes)
	assert.Error(t, ErrTreeCorrupt)
}

func TestConstants(t *testing.T) {
	assert.Equal(t, 500, MaxNodes)
	assert.Equal(t, 20, MaxNodeDepth)
	assert.Equal(t, 32*1024, MaxDescriptionBytes)
	assert.Equal(t, 128*1024, CompactThreshold)
	assert.Equal(t, ".helixcode/plans", StorageDir)
	assert.Equal(t, "[c]", CompactMarker)
}
