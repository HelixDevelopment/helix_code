package plantree

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateTree(t *testing.T) {
	tree, err := CreateTree("my-plan", "Test Plan", "A plan for testing")
	require.NoError(t, err)

	assert.Equal(t, "my-plan", tree.Name)
	assert.Equal(t, 1, tree.Version)
	assert.NotNil(t, tree.Root)
	assert.NotEmpty(t, tree.Root.ID)
	assert.Equal(t, "Test Plan", tree.Root.Title)
	assert.Equal(t, "A plan for testing", tree.Root.Description)
	assert.Equal(t, StatusPending, tree.Root.Status)
	assert.Nil(t, tree.Root.Children)
	assert.False(t, tree.CreatedAt.IsZero())
	assert.False(t, tree.UpdatedAt.IsZero())
}

func TestCreateTree_EmptyName(t *testing.T) {
	_, err := CreateTree("", "Title", "Desc")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "plan name required")
}

func TestBranchNode(t *testing.T) {
	tree, err := CreateTree("test", "Root", "Root desc")
	require.NoError(t, err)

	child, err := BranchNode(tree, tree.Root.ID, "Child Task", "Child description")
	require.NoError(t, err)

	assert.NotEmpty(t, child.ID)
	assert.Equal(t, "Child Task", child.Title)
	assert.Equal(t, "Child description", child.Description)
	assert.Equal(t, StatusPending, child.Status)
	assert.Equal(t, tree.Root.ID, child.ParentID)

	require.Len(t, tree.Root.Children, 1)
	assert.Equal(t, child.ID, tree.Root.Children[0].ID)
}

func TestBranchNode_ParentNotFound(t *testing.T) {
	tree, err := CreateTree("test", "Root", "Root desc")
	require.NoError(t, err)

	_, err = BranchNode(tree, "nonexistent-id", "Child", "Desc")
	assert.ErrorIs(t, err, ErrNodeNotFound)
}

func TestBranchNode_MaxDepth(t *testing.T) {
	tree, err := CreateTree("deep", "Root", "Root")
	require.NoError(t, err)

	currentID := tree.Root.ID
	for i := 1; i < MaxNodeDepth; i++ {
		child, err := BranchNode(tree, currentID, "Level", "Desc")
		require.NoError(t, err)
		currentID = child.ID
	}

	_, err = BranchNode(tree, currentID, "Too Deep", "Desc")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "max depth")
}

func TestBranchNode_MaxNodes(t *testing.T) {
	tree, err := CreateTree("big", "Root", "Root")
	require.NoError(t, err)

	for i := 1; i < MaxNodes; i++ {
		_, err := BranchNode(tree, tree.Root.ID, "Task", "Desc")
		require.NoError(t, err)
	}

	assert.Equal(t, MaxNodes, CountNodes(tree.Root))

	_, err = BranchNode(tree, tree.Root.ID, "One Too Many", "Desc")
	assert.ErrorIs(t, err, ErrTooManyNodes)
}

func TestMergeNode(t *testing.T) {
	tree, err := CreateTree("merge-test", "Root", "Root desc")
	require.NoError(t, err)

	child, err := BranchNode(tree, tree.Root.ID, "Child", "Child desc")
	require.NoError(t, err)

	parent, err := MergeNode(tree, child.ID)
	require.NoError(t, err)

	assert.Equal(t, tree.Root.ID, parent.ID)
	assert.Equal(t, child.ID, parent.Metadata["merged_from"])

	refreshedChild := findNode(tree.Root, child.ID)
	require.NotNil(t, refreshedChild)
	assert.Equal(t, StatusCompleted, refreshedChild.Status)
}

func TestMergeNode_RootCannotMerge(t *testing.T) {
	tree, err := CreateTree("test", "Root", "Root desc")
	require.NoError(t, err)

	_, err = MergeNode(tree, tree.Root.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "root node cannot be merged")
}

func TestMergeNode_NodeNotFound(t *testing.T) {
	tree, err := CreateTree("test", "Root", "Root desc")
	require.NoError(t, err)

	_, err = MergeNode(tree, "nonexistent")
	assert.ErrorIs(t, err, ErrNodeNotFound)
}

func TestMergeNode_CyclicMerge(t *testing.T) {
	tree, err := CreateTree("cycle-test", "Root", "Root")
	require.NoError(t, err)

	child, err := BranchNode(tree, tree.Root.ID, "Child", "Child")
	require.NoError(t, err)

	child.ParentID = tree.Root.ID

	tree.Root.ParentID = child.ID

	_, err = MergeNode(tree, child.ID)
	assert.ErrorIs(t, err, ErrCyclicMerge)
}

func TestMergeNode_MergeHistory(t *testing.T) {
	tree, err := CreateTree("history-test", "Root", "Root")
	require.NoError(t, err)

	child1, err := BranchNode(tree, tree.Root.ID, "Task 1", "First task")
	require.NoError(t, err)

	child2, err := BranchNode(tree, tree.Root.ID, "Task 2", "Second task")
	require.NoError(t, err)

	_, err = MergeNode(tree, child1.ID)
	require.NoError(t, err)

	_, err = MergeNode(tree, child2.ID)
	require.NoError(t, err)

	assert.Equal(t, child2.ID, tree.Root.Metadata["merged_from"])
	assert.Contains(t, tree.Root.Metadata["merged_history"], child1.ID)
	assert.Contains(t, tree.Root.Metadata["merged_history"], child2.ID)
}

func TestMergeNode_NilTree(t *testing.T) {
	_, err := MergeNode(nil, "any")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid tree")
}

func TestFindNode(t *testing.T) {
	tree, err := CreateTree("find-test", "Root", "Root")
	require.NoError(t, err)

	child, err := BranchNode(tree, tree.Root.ID, "Child", "Child")
	require.NoError(t, err)

	grandchild, err := BranchNode(tree, child.ID, "Grandchild", "Grandchild")
	require.NoError(t, err)

	assert.NotNil(t, findNode(tree.Root, tree.Root.ID))
	assert.NotNil(t, findNode(tree.Root, child.ID))
	assert.NotNil(t, findNode(tree.Root, grandchild.ID))
	assert.Nil(t, findNode(tree.Root, "nonexistent"))
	assert.Nil(t, findNode(nil, "any"))
}

func TestNodeDepth(t *testing.T) {
	tree, err := CreateTree("depth-test", "Root", "Root")
	require.NoError(t, err)

	child, err := BranchNode(tree, tree.Root.ID, "Child", "Child")
	require.NoError(t, err)

	grandchild, err := BranchNode(tree, child.ID, "Grandchild", "Grandchild")
	require.NoError(t, err)

	assert.Equal(t, 1, nodeDepth(tree.Root, tree.Root.ID, 1))
	assert.Equal(t, 2, nodeDepth(tree.Root, child.ID, 1))
	assert.Equal(t, 3, nodeDepth(tree.Root, grandchild.ID, 1))
	assert.Equal(t, 0, nodeDepth(tree.Root, "nonexistent", 1))
	assert.Equal(t, 0, nodeDepth(nil, "any", 1))
}

func TestWouldCycle(t *testing.T) {
	tree, err := CreateTree("cycle-check", "Root", "Root")
	require.NoError(t, err)

	child, err := BranchNode(tree, tree.Root.ID, "Child", "Child")
	require.NoError(t, err)

	grandchild, err := BranchNode(tree, child.ID, "Grandchild", "Grandchild")
	require.NoError(t, err)

	assert.False(t, wouldCycle(tree.Root, tree.Root.ID, grandchild.ID))
	assert.True(t, wouldCycle(tree.Root, grandchild.ID, tree.Root.ID))
	assert.False(t, wouldCycle(tree.Root, child.ID, grandchild.ID))
}
