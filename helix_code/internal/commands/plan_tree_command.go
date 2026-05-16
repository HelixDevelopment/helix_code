package commands

import (
	"context"
	"fmt"
	"strings"

	"dev.helix.code/internal/plantree"
)

type PlanTreeCommand struct {
	store      plantree.Store
	summariser plantree.Summariser
}

func NewPlanTreeCommand(store plantree.Store, summariser plantree.Summariser) *PlanTreeCommand {
	return &PlanTreeCommand{store: store, summariser: summariser}
}

func (c *PlanTreeCommand) Name() string      { return "plantree" }
func (c *PlanTreeCommand) Aliases() []string { return []string{"pt", "plans"} }
func (c *PlanTreeCommand) Description() string {
	return "Manage plan trees (create, branch, merge, inspect, compact, verify)"
}
func (c *PlanTreeCommand) Usage() string {
	return "/plantree [list|show <name>|compact <name>|verify <name>]"
}

func (c *PlanTreeCommand) Execute(ctx context.Context, cmdCtx *CommandContext) (*CommandResult, error) {
	args := cmdCtx.Args
	subcmd := "list"
	if len(args) > 0 {
		subcmd = args[0]
	}

	switch subcmd {
	case "list":
		return c.handleList(ctx, cmdCtx)
	case "show":
		return c.handleShow(ctx, cmdCtx)
	case "compact":
		return c.handleCompact(ctx, cmdCtx)
	case "verify":
		return c.handleVerify(ctx, cmdCtx)
	default:
		return &CommandResult{
			Success: false,
			Message: fmt.Sprintf("unknown subcommand: %s. Available: list, show, compact, verify", subcmd),
		}, nil
	}
}

func (c *PlanTreeCommand) handleList(ctx context.Context, cmdCtx *CommandContext) (*CommandResult, error) {
	summaries, err := c.store.List()
	if err != nil {
		return &CommandResult{Success: false, Message: fmt.Sprintf("list plans: %v", err)}, nil
	}

	if len(summaries) == 0 {
		return &CommandResult{Success: true, Message: "No plan trees found.", Output: "No plan trees found."}, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%-20s %6s %-20s %s\n", "NAME", "NODES", "ROOT TITLE", "UPDATED"))
	sb.WriteString(strings.Repeat("-", 80) + "\n")
	for _, s := range summaries {
		sb.WriteString(fmt.Sprintf("%-20s %6d %-20s %s\n", s.Name, s.NodeCount, truncateStr(s.RootTitle, 20), s.UpdatedAt.Format("2006-01-02 15:04")))
	}

	output := sb.String()
	return &CommandResult{Success: true, Message: fmt.Sprintf("%d plan tree(s)", len(summaries)), Output: output}, nil
}

func (c *PlanTreeCommand) handleShow(ctx context.Context, cmdCtx *CommandContext) (*CommandResult, error) {
	if len(cmdCtx.Args) < 2 {
		return &CommandResult{Success: false, Message: "usage: /plantree show <name>"}, nil
	}

	name := cmdCtx.Args[1]
	tree, err := c.store.Load(name)
	if err != nil {
		return &CommandResult{Success: false, Message: fmt.Sprintf("plan tree '%s': %v", name, err)}, nil
	}

	nodeID := cmdCtx.Flags["id"]
	var output string
	if nodeID != "" {
		node := findNodeByID(tree.Root, nodeID)
		if node == nil {
			return &CommandResult{Success: false, Message: fmt.Sprintf("node %s not found in plan tree '%s'", nodeID, name)}, nil
		}
		output = fmt.Sprintf("Plan Tree: %s (subtree at %s)\n\n%s", name, nodeID, plantree.RenderTree(node, 0))
	} else {
		output = fmt.Sprintf("Plan Tree: %s\n\n%s", name, plantree.RenderTree(tree.Root, 0))
	}

	return &CommandResult{Success: true, Message: output, Output: output}, nil
}

func (c *PlanTreeCommand) handleCompact(ctx context.Context, cmdCtx *CommandContext) (*CommandResult, error) {
	if len(cmdCtx.Args) < 2 {
		return &CommandResult{Success: false, Message: "usage: /plantree compact <name>"}, nil
	}

	name := cmdCtx.Args[1]
	tree, err := c.store.Load(name)
	if err != nil {
		return &CommandResult{Success: false, Message: fmt.Sprintf("plan tree '%s': %v", name, err)}, nil
	}

	result, err := plantree.CompactTree(&tree, c.summariser)
	if err != nil {
		return &CommandResult{Success: false, Message: fmt.Sprintf("compact: %v", err)}, nil
	}

	if result.NodesCompacted == 0 {
		return &CommandResult{
			Success: true,
			Message: fmt.Sprintf("Plan tree '%s': %d nodes, no compaction needed (%d bytes under threshold).", name, plantree.CountNodes(tree.Root), result.OriginalBytes),
		}, nil
	}

	if err := c.store.Save(result.Tree); err != nil {
		return &CommandResult{Success: false, Message: fmt.Sprintf("save compacted plan tree: %v", err)}, nil
	}

	reduction := result.OriginalBytes - result.NewBytes
	return &CommandResult{
		Success: true,
		Message: fmt.Sprintf("Plan tree '%s' compacted: %d nodes (%d bytes → %d bytes, -%d bytes).",
			name, result.NodesCompacted, result.OriginalBytes, result.NewBytes, reduction),
	}, nil
}

func (c *PlanTreeCommand) handleVerify(ctx context.Context, cmdCtx *CommandContext) (*CommandResult, error) {
	if len(cmdCtx.Args) < 2 {
		return &CommandResult{Success: false, Message: "usage: /plantree verify <name>"}, nil
	}

	name := cmdCtx.Args[1]
	tree, err := c.store.Load(name)
	if err != nil {
		return &CommandResult{Success: false, Message: fmt.Sprintf("plan tree '%s': %v", name, err)}, nil
	}

	result := plantree.VerifyTree(&tree)

	if result.Valid {
		return &CommandResult{Success: true, Message: fmt.Sprintf("Plan tree '%s' is valid.", name)}, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Plan tree '%s' has %d issue(s):\n", name, len(result.Issues)))
	for _, issue := range result.Issues {
		severity := "WARN"
		if issue.Severity == plantree.SeverityError {
			severity = "ERROR"
		}
		nodeInfo := ""
		if issue.NodeID != "" {
			nodeInfo = fmt.Sprintf(" [%s]", issue.NodeID)
		}
		sb.WriteString(fmt.Sprintf("  [%s]%s %s\n", severity, nodeInfo, issue.Message))
	}

	output := sb.String()
	return &CommandResult{Success: true, Message: output, Output: output}, nil
}

func findNodeByID(node *plantree.PlanNode, id string) *plantree.PlanNode {
	if node == nil {
		return nil
	}
	if node.ID == id {
		return node
	}
	for _, child := range node.Children {
		if found := findNodeByID(child, id); found != nil {
			return found
		}
	}
	return nil
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
