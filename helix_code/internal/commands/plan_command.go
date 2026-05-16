package commands

import (
	"context"
	"fmt"
	"strings"
	"text/tabwriter"

	"dev.helix.code/internal/workflow/planmode"
)

// PlanCommand implements the /plan slash command.
//
// Subcommands:
//
//	/plan              — show (default)
//	/plan show         — display the active plan and its actions
//	/plan approve      — approve the entire active plan
//	/plan approve <id> — approve a single action by ID
//	/plan reject       — reject the active plan and return to normal mode
//	/plan status       — report current mode and plan summary
type PlanCommand struct {
	planner planmode.ApprovalPlanner
	mc      planmode.ModeController
}

// NewPlanCommand returns a /plan command bound to a planner and mode controller.
func NewPlanCommand(planner planmode.ApprovalPlanner, mc planmode.ModeController) *PlanCommand {
	return &PlanCommand{planner: planner, mc: mc}
}

func (c *PlanCommand) Name() string        { return "plan" }
func (c *PlanCommand) Aliases() []string   { return nil }
func (c *PlanCommand) Description() string { return "Inspect, approve, or reject the active plan." }
func (c *PlanCommand) Usage() string       { return "/plan [show|approve [<action-id>]|reject|status]" }

// Execute dispatches to the appropriate subcommand handler.
func (c *PlanCommand) Execute(ctx context.Context, cc *CommandContext) (*CommandResult, error) {
	sub := "show"
	if len(cc.Args) > 0 {
		sub = cc.Args[0]
	}
	switch sub {
	case "show":
		return c.show()
	case "approve":
		return c.approve(cc.Args[1:])
	case "reject":
		return c.reject()
	case "status":
		return c.status()
	default:
		return nil, fmt.Errorf("/plan: unknown subcommand %q (want show|approve|reject|status)", sub)
	}
}

// show renders the active plan and its actions.
func (c *PlanCommand) show() (*CommandResult, error) {
	plan := c.planner.ActivePlan()
	if plan == nil {
		return &CommandResult{Success: true, Output: "No active plan."}, nil
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Plan: %s — %s\n", plan.ID, plan.Title)
	if plan.Description != "" {
		fmt.Fprintf(&sb, "Description: %s\n", plan.Description)
	}
	fmt.Fprintf(&sb, "Status: %s\n\n", plan.Status)

	tw := tabwriter.NewWriter(&sb, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tTOOL\tAPPROVED\tEXECUTED\tDESCRIPTION")
	for _, a := range plan.Actions {
		approved := "no"
		if a.Approved != nil && *a.Approved {
			approved = "yes"
		}
		executed := "no"
		if a.Executed {
			executed = "yes"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", a.ID, a.ToolName, approved, executed, a.Description)
	}
	tw.Flush()

	return &CommandResult{Success: true, Output: sb.String()}, nil
}

// approve approves the whole active plan (no args) or a single action (one arg).
func (c *PlanCommand) approve(rest []string) (*CommandResult, error) {
	plan := c.planner.ActivePlan()
	if plan == nil {
		return nil, fmt.Errorf("/plan approve: no active plan")
	}
	if len(rest) == 0 {
		if err := c.planner.ApprovePlan(plan.ID); err != nil {
			return nil, err
		}
		return &CommandResult{
			Success: true,
			Output:  fmt.Sprintf("Plan %s approved (%d actions).", plan.ID, len(plan.Actions)),
		}, nil
	}
	actionID := rest[0]
	if err := c.planner.ApproveAction(plan.ID, actionID); err != nil {
		return nil, err
	}
	return &CommandResult{
		Success: true,
		Output:  fmt.Sprintf("Approved action %s.", actionID),
	}, nil
}

// reject rejects the active plan and transitions back to ModeNormal if in plan mode.
func (c *PlanCommand) reject() (*CommandResult, error) {
	plan := c.planner.ActivePlan()
	if plan == nil {
		return nil, fmt.Errorf("/plan reject: no active plan")
	}
	if err := c.planner.RejectPlan(plan.ID); err != nil {
		return nil, err
	}
	if c.mc.GetMode() == planmode.ModePlan {
		_ = c.mc.TransitionTo(planmode.ModeNormal)
	}
	return &CommandResult{
		Success: true,
		Output:  fmt.Sprintf("Plan %s rejected. Returning to normal mode.", plan.ID),
	}, nil
}

// status reports the current mode and a brief plan summary.
func (c *PlanCommand) status() (*CommandResult, error) {
	mode := c.mc.GetMode()
	plan := c.planner.ActivePlan()
	planSummary := "no active plan"
	if plan != nil {
		planSummary = fmt.Sprintf("plan=%s status=%s actions=%d", plan.ID, plan.Status, len(plan.Actions))
	}
	return &CommandResult{
		Success: true,
		Output:  fmt.Sprintf("Mode: %s | %s", mode, planSummary),
	}, nil
}
