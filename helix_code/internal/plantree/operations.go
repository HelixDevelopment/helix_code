package plantree

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

func CreateTree(name, title, description string) (*PlanTree, error) {
	if name == "" {
		return nil, errors.New("plan name required")
	}
	now := time.Now().UTC()
	root := &PlanNode{
		ID:          uuid.New().String(),
		Title:       title,
		Description: description,
		Status:      StatusPending,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	return &PlanTree{
		Name:      name,
		Version:   1,
		Root:      root,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func BranchNode(tree *PlanTree, parentID, title, description string) (*PlanNode, error) {
	if tree == nil || tree.Root == nil {
		return nil, errors.New("invalid tree")
	}
	if CountNodes(tree.Root) >= MaxNodes {
		return nil, ErrTooManyNodes
	}

	parent := findNode(tree.Root, parentID)
	if parent == nil {
		return nil, ErrNodeNotFound
	}

	depth := nodeDepth(tree.Root, parentID, 1)
	if depth >= MaxNodeDepth {
		return nil, fmt.Errorf("max depth %d exceeded", MaxNodeDepth)
	}

	now := time.Now().UTC()
	child := &PlanNode{
		ID:          uuid.New().String(),
		Title:       title,
		Description: description,
		Status:      StatusPending,
		ParentID:    parentID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	parent.Children = append(parent.Children, child)
	tree.UpdatedAt = now
	return child, nil
}

func MergeNode(tree *PlanTree, childID string) (*PlanNode, error) {
	if tree == nil || tree.Root == nil {
		return nil, errors.New("invalid tree")
	}

	child := findNode(tree.Root, childID)
	if child == nil {
		return nil, ErrNodeNotFound
	}
	if child.ParentID == "" {
		return nil, errors.New("root node cannot be merged")
	}

	parent := findNode(tree.Root, child.ParentID)
	if parent == nil {
		return nil, fmt.Errorf("parent %s not found: %w", child.ParentID, ErrTreeCorrupt)
	}

	if wouldCycle(tree.Root, parent.ID, childID) {
		return nil, ErrCyclicMerge
	}

	if parent.Metadata == nil {
		parent.Metadata = make(map[string]string)
	}
	parent.Metadata["merged_from"] = childID
	if history, ok := parent.Metadata["merged_history"]; ok && history != "" {
		parent.Metadata["merged_history"] = history + "," + childID
	} else {
		parent.Metadata["merged_history"] = childID
	}

	child.Status = StatusCompleted
	tree.UpdatedAt = time.Now().UTC()
	return parent, nil
}

func findNode(node *PlanNode, id string) *PlanNode {
	return findNodeVisited(node, id, make(map[string]bool))
}

func findNodeVisited(node *PlanNode, id string, visited map[string]bool) *PlanNode {
	if node == nil || visited[node.ID] {
		return nil
	}
	visited[node.ID] = true
	if node.ID == id {
		return node
	}
	for _, child := range node.Children {
		if found := findNodeVisited(child, id, visited); found != nil {
			return found
		}
	}
	return nil
}

func nodeDepth(node *PlanNode, id string, depth int) int {
	return nodeDepthVisited(node, id, depth, make(map[string]bool))
}

func nodeDepthVisited(node *PlanNode, id string, depth int, visited map[string]bool) int {
	if node == nil || visited[node.ID] {
		return 0
	}
	visited[node.ID] = true
	if node.ID == id {
		return depth
	}
	for _, child := range node.Children {
		if d := nodeDepthVisited(child, id, depth+1, visited); d > 0 {
			return d
		}
	}
	return 0
}

func wouldCycle(node *PlanNode, ancestorID, descendantID string) bool {
	// Walk the ParentID chain upward from ancestorID. A malformed tree
	// (e.g. loaded from corrupt JSON) can contain a ParentID cycle that
	// does NOT include descendantID — without a visited guard this loop
	// never terminates (infinite-loop / hang reachable via MergeNode).
	// The visited set bounds the walk to each node at most once, matching
	// the cycle-safe traversal pattern used elsewhere in this package.
	visited := make(map[string]bool)
	current := findNode(node, ancestorID)
	for current != nil && current.ParentID != "" {
		if current.ParentID == descendantID {
			return true
		}
		if visited[current.ID] {
			// ParentID chain looped back on itself without reaching
			// descendantID — no path from descendant to ancestor exists.
			return false
		}
		visited[current.ID] = true
		current = findNode(node, current.ParentID)
	}
	return false
}
