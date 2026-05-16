package plantree

import "fmt"

func VerifyTree(tree *PlanTree) VerifyResult {
	if tree == nil || tree.Root == nil {
		return VerifyResult{
			Valid: false,
			Issues: []PlanIssue{
				{Severity: SeverityError, Message: "tree is empty"},
			},
		}
	}

	var issues []PlanIssue
	nodeMap := make(map[string]*PlanNode)
	buildNodeMap(tree.Root, nodeMap)

	idCount := make(map[string]int)
	countIDs(tree.Root, idCount)
	for id, count := range idCount {
		if count > 1 {
			issues = append(issues, PlanIssue{
				Severity: SeverityError,
				NodeID:   id,
				Message:  fmt.Sprintf("duplicate node ID appears %d times", count),
			})
		}
	}

	for _, node := range nodeMap {
		if node.ParentID != "" {
			if _, ok := nodeMap[node.ParentID]; !ok {
				issues = append(issues, PlanIssue{
					Severity: SeverityError,
					NodeID:   node.ID,
					Message:  fmt.Sprintf("parent %s not found (orphan)", node.ParentID),
				})
			}
		}
	}

	if hasCycle(tree.Root, make(map[string]bool)) {
		issues = append(issues, PlanIssue{
			Severity: SeverityError,
			Message:  "plan tree contains a cycle",
		})
	}

	if depth := MaxDepth(tree.Root); depth > MaxNodeDepth {
		issues = append(issues, PlanIssue{
			Severity: SeverityError,
			Message:  fmt.Sprintf("maximum depth %d exceeds limit %d", depth, MaxNodeDepth),
		})
	}

	if count := CountNodes(tree.Root); count > MaxNodes {
		issues = append(issues, PlanIssue{
			Severity: SeverityError,
			Message:  fmt.Sprintf("node count %d exceeds maximum %d", count, MaxNodes),
		})
	}

	for _, node := range nodeMap {
		if node.ParentID == node.ID {
			issues = append(issues, PlanIssue{
				Severity: SeverityError,
				NodeID:   node.ID,
				Message:  "node parent references itself",
			})
		}
	}

	valid := true
	for _, issue := range issues {
		if issue.Severity == SeverityError {
			valid = false
			break
		}
	}

	return VerifyResult{Valid: valid, Issues: issues}
}

func buildNodeMap(node *PlanNode, m map[string]*PlanNode) {
	buildNodeMapVisited(node, m, make(map[string]bool))
}

func buildNodeMapVisited(node *PlanNode, m map[string]*PlanNode, visited map[string]bool) {
	if node == nil || visited[node.ID] {
		return
	}
	visited[node.ID] = true
	m[node.ID] = node
	for _, child := range node.Children {
		buildNodeMapVisited(child, m, visited)
	}
}

func countIDs(node *PlanNode, m map[string]int) {
	countIDsPath(node, make(map[string]bool), m)
}

func countIDsPath(node *PlanNode, path map[string]bool, result map[string]int) {
	if node == nil || path[node.ID] {
		return
	}
	path[node.ID] = true
	result[node.ID]++
	for _, child := range node.Children {
		countIDsPath(child, path, result)
	}
	path[node.ID] = false
}

func hasCycle(node *PlanNode, visited map[string]bool) bool {
	if node == nil {
		return false
	}
	if visited[node.ID] {
		return true
	}
	visited[node.ID] = true
	for _, child := range node.Children {
		if hasCycle(child, visited) {
			return true
		}
	}
	visited[node.ID] = false
	return false
}
