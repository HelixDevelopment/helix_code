package tools

import "errors"

// ErrPlanModeGated is returned by ToolRegistry.Execute when a destructive tool
// is invoked in plan mode without an approved plan action authorising it.
// Wrapped with the tool name and reason for diagnostic clarity:
//
//	fmt.Errorf("%w: <name> (<reason>)", ErrPlanModeGated)
//
// Callers detect via errors.Is.
var ErrPlanModeGated = errors.New("tools: blocked by plan mode")
