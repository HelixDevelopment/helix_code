package plantree

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVerifyTree_Clean(t *testing.T) {
	tree, err := CreateTree("clean", "Root", "Root")
	assert.NoError(t, err)

	child, err := BranchNode(tree, tree.Root.ID, "Child", "Child")
	assert.NoError(t, err)

	_, err = BranchNode(tree, child.ID, "Grandchild", "Grandchild")
	assert.NoError(t, err)

	result := VerifyTree(tree)
	assert.True(t, result.Valid)
	assert.Empty(t, result.Issues)
}

func TestVerifyTree_Orphan(t *testing.T) {
	tree, err := CreateTree("orphan-test", "Root", "Root")
	assert.NoError(t, err)

	_, err = BranchNode(tree, tree.Root.ID, "Child", "Child")
	assert.NoError(t, err)

	tree.Root.Children[0].ParentID = "nonexistent-parent"

	result := VerifyTree(tree)
	assert.False(t, result.Valid)
	assert.NotEmpty(t, result.Issues)

	foundOrphan := false
	for _, issue := range result.Issues {
		if issue.Severity == SeverityError {
			foundOrphan = true
			assert.Contains(t, issue.Message, "not found")
			assert.Contains(t, issue.Message, "orphan")
		}
	}
	assert.True(t, foundOrphan, "should have found orphan issue")
}

func TestVerifyTree_Cycle(t *testing.T) {
	tree, err := CreateTree("cycle-test", "Root", "Root")
	assert.NoError(t, err)

	child1, err := BranchNode(tree, tree.Root.ID, "Child1", "Child1")
	assert.NoError(t, err)

	_, err = BranchNode(tree, child1.ID, "Child2", "Child2")
	assert.NoError(t, err)

	child1.Children[0].Children = append(child1.Children[0].Children, tree.Root)

	result := VerifyTree(tree)
	assert.False(t, result.Valid)

	foundCycle := false
	for _, issue := range result.Issues {
		if issue.Severity == SeverityError && issue.Message == "plan tree contains a cycle" {
			foundCycle = true
		}
	}
	assert.True(t, foundCycle, "should have found cycle issue")
}

func TestVerifyTree_DepthOverflow(t *testing.T) {
	tree, err := CreateTree("deep", "Root", "Root")
	assert.NoError(t, err)

	current := tree.Root
	for i := 1; i <= MaxNodeDepth; i++ {
		current.Children = []*PlanNode{{
			ID:     "level-" + string(rune('A'+i-1)),
			Title:  "Level",
			Status: StatusPending,
		}}
		current = current.Children[0]
	}

	result := VerifyTree(tree)
	assert.False(t, result.Valid)

	foundDepth := false
	for _, issue := range result.Issues {
		if issue.Severity == SeverityError {
			foundDepth = true
			assert.Contains(t, issue.Message, "depth")
		}
	}
	assert.True(t, foundDepth, "should have found depth issue")
}

func TestVerifyTree_DuplicateIDs(t *testing.T) {
	root := &PlanNode{
		ID:     "root",
		Title:  "Root",
		Status: StatusPending,
		Children: []*PlanNode{
			{ID: "dup-id", Title: "Child1", Status: StatusPending},
			{ID: "dup-id", Title: "Child2", Status: StatusPending},
		},
	}
	tree := &PlanTree{Name: "dup-test", Version: 1, Root: root}

	result := VerifyTree(tree)
	assert.False(t, result.Valid)

	foundDup := false
	for _, issue := range result.Issues {
		if issue.Severity == SeverityError && issue.NodeID == "dup-id" {
			foundDup = true
			assert.Contains(t, issue.Message, "duplicate")
		}
	}
	assert.True(t, foundDup, "should have found duplicate ID issue")
}

func TestVerifyTree_SelfParenting(t *testing.T) {
	tree, err := CreateTree("self-parent", "Root", "Root")
	assert.NoError(t, err)

	child, err := BranchNode(tree, tree.Root.ID, "Child", "Child")
	assert.NoError(t, err)

	child.ParentID = child.ID

	result := VerifyTree(tree)
	assert.False(t, result.Valid)

	foundSelf := false
	for _, issue := range result.Issues {
		if issue.Severity == SeverityError && issue.NodeID == child.ID {
			foundSelf = true
			assert.Contains(t, issue.Message, "parent references itself")
		}
	}
	assert.True(t, foundSelf, "should have found self-parenting issue")
}

func TestVerifyTree_NodeCountOverflow(t *testing.T) {
	root := &PlanNode{ID: "root", Title: "Root", Status: StatusPending}
	tree := &PlanTree{Name: "big", Version: 1, Root: root}

	current := root
	for i := 0; i < MaxNodes; i++ {
		child := &PlanNode{
			ID:     fmt.Sprintf("node-%d", i),
			Title:  "Node",
			Status: StatusPending,
		}
		current.Children = append(current.Children, child)
		current = child
	}

	result := VerifyTree(tree)
	assert.False(t, result.Valid)

	foundCount := false
	for _, issue := range result.Issues {
		if issue.Severity == SeverityError && strings.Contains(issue.Message, "node count") {
			foundCount = true
		}
	}
	assert.True(t, foundCount, "should have found node count issue; issues: %v", result.Issues)
}

func TestVerifyTree_NilTree(t *testing.T) {
	result := VerifyTree(nil)
	assert.False(t, result.Valid)
	requireIssueContains(t, result, "tree is empty")
}

func TestVerifyTree_NilRoot(t *testing.T) {
	result := VerifyTree(&PlanTree{Name: "empty"})
	assert.False(t, result.Valid)
	requireIssueContains(t, result, "tree is empty")
}

func requireIssueContains(t *testing.T, result VerifyResult, substr string) {
	t.Helper()
	for _, issue := range result.Issues {
		assert.Contains(t, issue.Message, substr)
	}
	assert.NotEmpty(t, result.Issues)
}
