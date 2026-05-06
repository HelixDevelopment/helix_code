package plantree

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type Summariser interface {
	Summarise(text string, maxWords int) (string, error)
}

type CompactResult struct {
	Tree           PlanTree
	OriginalBytes  int
	NewBytes       int
	NodesCompacted int
}

type DeterministicSummariser struct{}

func (DeterministicSummariser) Summarise(text string, maxWords int) (string, error) {
	words := strings.Fields(text)
	if len(words) <= maxWords {
		return text, nil
	}
	return strings.Join(words[:maxWords], " ") + "...", nil
}

func CompactTree(tree *PlanTree, summariser Summariser) (CompactResult, error) {
	if tree == nil || tree.Root == nil {
		return CompactResult{}, errors.New("invalid tree")
	}

	origJSON, err := json.Marshal(tree)
	if err != nil {
		return CompactResult{}, fmt.Errorf("marshal: %w", err)
	}
	originalBytes := len(origJSON)

	if originalBytes < CompactThreshold {
		treeCopy := *tree
		return CompactResult{Tree: treeCopy, OriginalBytes: originalBytes, NewBytes: originalBytes, NodesCompacted: 0}, nil
	}

	compacted := 0
	walkAndCompact(tree.Root, 1, summariser, &compacted)

	newJSON, err := json.Marshal(tree)
	if err != nil {
		return CompactResult{}, fmt.Errorf("marshal after compact: %w", err)
	}

	return CompactResult{
		Tree:           *tree,
		OriginalBytes:  originalBytes,
		NewBytes:       len(newJSON),
		NodesCompacted: compacted,
	}, nil
}

func walkAndCompact(node *PlanNode, depth int, summariser Summariser, compacted *int) {
	if node == nil {
		return
	}

	for _, child := range node.Children {
		walkAndCompact(child, depth+1, summariser, compacted)
	}

	if depth >= 3 && len(node.Children) == 0 && (node.Status == StatusCompleted || node.Status == StatusFailed) {
		summary, err := summariser.Summarise(node.Description, 50)
		if err == nil && summary != "" {
			if node.Metadata == nil {
				node.Metadata = make(map[string]string)
			}
			node.Metadata["compacted_bytes"] = fmt.Sprintf("%d", len(node.Description))
			node.Metadata["compacted"] = "true"
			node.Description = summary
			*compacted++
		}
	}
}
