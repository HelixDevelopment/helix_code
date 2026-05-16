package plantree

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type PlanStatus int

const (
	StatusPending    PlanStatus = iota
	StatusInProgress
	StatusCompleted
	StatusFailed
	StatusPruned
)

func (s PlanStatus) String() string {
	switch s {
	case StatusPending:
		return "pending"
	case StatusInProgress:
		return "in_progress"
	case StatusCompleted:
		return "completed"
	case StatusFailed:
		return "failed"
	case StatusPruned:
		return "pruned"
	default:
		return "unknown"
	}
}

func (s PlanStatus) Marker() string {
	switch s {
	case StatusPending:
		return "[ ]"
	case StatusInProgress:
		return "[▶]"
	case StatusCompleted:
		return "[✓]"
	case StatusFailed:
		return "[✗]"
	case StatusPruned:
		return "[×]"
	default:
		return "[?]"
	}
}

func (s PlanStatus) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

func (s *PlanStatus) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	switch str {
	case "pending":
		*s = StatusPending
	case "in_progress":
		*s = StatusInProgress
	case "completed":
		*s = StatusCompleted
	case "failed":
		*s = StatusFailed
	case "pruned":
		*s = StatusPruned
	default:
		return fmt.Errorf("unknown plan status: %s", str)
	}
	return nil
}

type PlanNode struct {
	ID          string            `json:"id"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Status      PlanStatus        `json:"status"`
	Children    []*PlanNode       `json:"children,omitempty"`
	ParentID    string            `json:"parent_id,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

type PlanTree struct {
	Name      string    `json:"name"`
	Version   int       `json:"version"`
	Root      *PlanNode `json:"root"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type PlanTreeSummary struct {
	Name      string    `json:"name"`
	NodeCount int       `json:"node_count"`
	RootID    string    `json:"root_id"`
	RootTitle string    `json:"root_title"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type PlanIssueSeverity int

const (
	SeverityError   PlanIssueSeverity = iota
	SeverityWarning
)

type PlanIssue struct {
	Severity PlanIssueSeverity `json:"severity"`
	NodeID   string            `json:"node_id,omitempty"`
	Message  string            `json:"message"`
}

type VerifyResult struct {
	Valid  bool        `json:"valid"`
	Issues []PlanIssue `json:"issues"`
}

var (
	ErrPlanNotFound      = errors.New("plan not found")
	ErrNodeNotFound      = errors.New("node not found in tree")
	ErrCyclicMerge       = errors.New("merge would create a cycle")
	ErrPlanAlreadyExists = errors.New("plan already exists")
	ErrTooManyNodes      = errors.New("plan tree exceeds maximum node count")
	ErrTreeCorrupt       = errors.New("plan tree is corrupt")
)

const (
	MaxNodes            = 500
	MaxNodeDepth        = 20
	MaxDescriptionBytes = 32 * 1024
	CompactThreshold    = 128 * 1024
	StorageDir          = ".helixcode/plans"
	CompactMarker       = "[c]"
)

func RenderTree(node *PlanNode, depth int) string {
	return renderTreeVisited(node, depth, make(map[string]bool))
}

func renderTreeVisited(node *PlanNode, depth int, visited map[string]bool) string {
	if node == nil || visited[node.ID] {
		return ""
	}
	visited[node.ID] = true
	indent := strings.Repeat("  ", depth)
	compactTag := ""
	if node.Metadata != nil && node.Metadata["compacted"] == "true" {
		compactTag = " " + CompactMarker
	}
	result := fmt.Sprintf("%s%s %s (%s)%s\n", indent, node.Status.Marker(), node.Title, node.ID, compactTag)
	for _, child := range node.Children {
		result += renderTreeVisited(child, depth+1, visited)
	}
	return result
}

func CountNodes(node *PlanNode) int {
	return countNodesVisited(node, make(map[string]bool))
}

func countNodesVisited(node *PlanNode, visited map[string]bool) int {
	if node == nil || visited[node.ID] {
		return 0
	}
	visited[node.ID] = true
	count := 1
	for _, child := range node.Children {
		count += countNodesVisited(child, visited)
	}
	return count
}

func MaxDepth(node *PlanNode) int {
	return maxDepthVisited(node, make(map[string]bool))
}

func maxDepthVisited(node *PlanNode, visited map[string]bool) int {
	if node == nil || visited[node.ID] {
		return 0
	}
	visited[node.ID] = true
	maxChild := 0
	for _, child := range node.Children {
		d := maxDepthVisited(child, visited)
		if d > maxChild {
			maxChild = d
		}
	}
	return maxChild + 1
}

func (pt *PlanTree) Summary() PlanTreeSummary {
	if pt == nil || pt.Root == nil {
		return PlanTreeSummary{}
	}
	return PlanTreeSummary{
		Name:      pt.Name,
		NodeCount: CountNodes(pt.Root),
		RootID:    pt.Root.ID,
		RootTitle: pt.Root.Title,
		CreatedAt: pt.CreatedAt,
		UpdatedAt: pt.UpdatedAt,
	}
}
