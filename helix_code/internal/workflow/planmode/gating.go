package planmode

import (
	"sync"
)

// DefaultAllowList is the canonical claude-code-style allow-list of always-safe
// tools that may run in plan mode without explicit approval.
var DefaultAllowList = []string{
	"Read", "Glob", "Grep", "View", "LSPGetDiagnostics",
	"TaskOutput", "TaskStop", "WebFetch", "WebSearch",
}

// DefaultKeyArgMap maps each destructive tool name to the param key whose
// value identifies the action target. An approved plan action matches an
// Execute call only when the values for that key are equal (where defined).
var DefaultKeyArgMap = map[string]string{
	"Edit":         "file_path",
	"Write":        "file_path",
	"MultiEdit":    "file_path",
	"NotebookEdit": "notebook_path",
	"Bash":         "command",
	"shell":        "command",
}

// ToolGate decides whether a tool call is allowed in plan mode. It queries
// the ModeController for the current operational mode and the ApprovalPlanner
// for any active plan. Stateful only across MarkExecuted calls (which actions
// have already been consumed).
type ToolGate struct {
	mc        ModeController
	planner   ApprovalPlanner
	allowList map[string]bool
	keyArgs   map[string]string

	mu       sync.Mutex
	executed map[string]map[string]bool // planID → actionID → executed
}

// NewToolGate constructs a ToolGate using DefaultAllowList and DefaultKeyArgMap.
// The planner argument is whatever implements ApprovalPlanner (e.g.,
// *DefaultPlanner from this package).
func NewToolGate(mc ModeController, planner ApprovalPlanner) *ToolGate {
	g := &ToolGate{
		mc:        mc,
		planner:   planner,
		allowList: make(map[string]bool, len(DefaultAllowList)),
		keyArgs:   make(map[string]string, len(DefaultKeyArgMap)),
		executed:  make(map[string]map[string]bool),
	}
	for _, n := range DefaultAllowList {
		g.allowList[n] = true
	}
	for k, v := range DefaultKeyArgMap {
		g.keyArgs[k] = v
	}
	return g
}

// WithAllowList returns a NEW gate with the given extra tool names added to
// the allow-list. The original gate is unchanged.
func (g *ToolGate) WithAllowList(extra []string) *ToolGate {
	c := &ToolGate{
		mc:        g.mc,
		planner:   g.planner,
		allowList: make(map[string]bool, len(g.allowList)+len(extra)),
		keyArgs:   make(map[string]string, len(g.keyArgs)),
		executed:  make(map[string]map[string]bool),
	}
	for k := range g.allowList {
		c.allowList[k] = true
	}
	for k, v := range g.keyArgs {
		c.keyArgs[k] = v
	}
	for _, n := range extra {
		c.allowList[n] = true
	}
	return c
}

// IsBlocked reports whether the named tool call is blocked by plan mode.
// Returns (false, "") in normal mode, on allow-listed tools, or when an
// approved-but-unexecuted plan action authorises the call.
func (g *ToolGate) IsBlocked(toolName string, params map[string]any) (bool, string) {
	if g.mc.GetMode() != ModePlan {
		return false, ""
	}
	if g.allowList[toolName] {
		return false, ""
	}
	plan := g.planner.ActivePlan()
	if plan == nil {
		return true, "no active plan"
	}
	if _, _, ok := g.matchActionLocked(plan, toolName, params); ok {
		return false, ""
	}
	return true, "no approved plan action authorises this tool"
}

// MatchApprovedAction returns the planID, the matched action, and ok=true when
// an approved-but-unexecuted plan action authorises the call. Returns ok=false
// otherwise (including in non-plan mode or with no active plan).
func (g *ToolGate) MatchApprovedAction(toolName string, params map[string]any) (string, *PlanAction, bool) {
	if g.mc.GetMode() != ModePlan {
		return "", nil, false
	}
	plan := g.planner.ActivePlan()
	if plan == nil {
		return "", nil, false
	}
	planID, action, ok := g.matchActionLocked(plan, toolName, params)
	if !ok {
		return "", nil, false
	}
	return planID, action, true
}

func (g *ToolGate) matchActionLocked(plan *Plan, toolName string, params map[string]any) (string, *PlanAction, bool) {
	keyArg, hasKey := g.keyArgs[toolName]
	g.mu.Lock()
	defer g.mu.Unlock()
	planExec := g.executed[plan.ID]
	for i := range plan.Actions {
		a := &plan.Actions[i]
		if a.ToolName != toolName {
			continue
		}
		if a.Approved == nil || !*a.Approved {
			continue
		}
		if planExec != nil && planExec[a.ID] {
			continue
		}
		if hasKey {
			plannedVal := a.Args[keyArg]
			actualVal, _ := params[keyArg]
			if !valuesEqual(plannedVal, actualVal) {
				continue
			}
		}
		return plan.ID, a, true
	}
	return "", nil, false
}

// MarkExecuted records that a matched action has been consumed. Subsequent
// IsBlocked / MatchApprovedAction calls will not authorise this action again.
func (g *ToolGate) MarkExecuted(planID, actionID string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.executed[planID] == nil {
		g.executed[planID] = make(map[string]bool)
	}
	g.executed[planID][actionID] = true
}

// valuesEqual is a strict equality check that handles common scalar types.
func valuesEqual(a, b any) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	switch av := a.(type) {
	case string:
		bv, ok := b.(string)
		return ok && av == bv
	case int:
		bv, ok := b.(int)
		return ok && av == bv
	case float64:
		bv, ok := b.(float64)
		return ok && av == bv
	case bool:
		bv, ok := b.(bool)
		return ok && av == bv
	default:
		return false
	}
}
