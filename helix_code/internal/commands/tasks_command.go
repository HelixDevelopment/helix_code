package commands

import (
	"context"
	"fmt"
	"strings"
	"text/tabwriter"

	"dev.helix.code/internal/workflow"
)

// TasksCommand implements the /tasks slash command.
//
// Subactions:
//
//	/tasks             — list (default)
//	/tasks list        — explicit list
//	/tasks output <id> — show last output lines for a task
//	/tasks stop <id>   — cancel a running task
type TasksCommand struct {
	manager *workflow.BackgroundManager
}

// NewTasksCommand returns a /tasks command bound to a BackgroundManager.
func NewTasksCommand(m *workflow.BackgroundManager) *TasksCommand {
	return &TasksCommand{manager: m}
}

func (c *TasksCommand) Name() string        { return "tasks" }
func (c *TasksCommand) Aliases() []string   { return []string{} }
func (c *TasksCommand) Description() string {
	// CONST-046 (round-393): genuine user-facing CLI help text
	// resolved through the package-level translator.
	return tr(context.Background(), "internal_commands_tasks_description", nil)
}
func (c *TasksCommand) Usage() string {
	return tr(context.Background(), "internal_commands_tasks_usage", nil)
}

// Execute runs the slash command. Subcommands: list (default), output <id>, stop <id>.
func (c *TasksCommand) Execute(ctx context.Context, cmdCtx *CommandContext) (*CommandResult, error) {
	if c.manager == nil {
		// CONST-046 (round-149): manager-not-initialised message
		// resolved through the package-level translator.
		return nil, fmt.Errorf("%s", tr(ctx, "internal_commands_tasks_manager_not_initialised", nil))
	}
	sub := "list"
	if len(cmdCtx.Args) > 0 {
		sub = cmdCtx.Args[0]
	}
	switch sub {
	case "list":
		return &CommandResult{Output: c.list(), Success: true}, nil
	case "output":
		if len(cmdCtx.Args) < 2 {
			return nil, fmt.Errorf("%s", tr(ctx, "internal_commands_tasks_output_usage", nil))
		}
		return c.output(cmdCtx.Args[1])
	case "stop":
		if len(cmdCtx.Args) < 2 {
			return nil, fmt.Errorf("%s", tr(ctx, "internal_commands_tasks_stop_usage", nil))
		}
		return c.stop(cmdCtx.Args[1])
	default:
		return nil, fmt.Errorf("%s", tr(ctx, "internal_commands_tasks_unknown_subcommand", map[string]any{"Sub": sub}))
	}
}

func (c *TasksCommand) list() string {
	var sb strings.Builder
	tw := tabwriter.NewWriter(&sb, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tTOOL\tSTATE\tSTARTED")
	for _, t := range c.manager.ListTasks() {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
			t.ID, t.ToolName, t.State(), t.StartedAt.Format("15:04:05"))
	}
	tw.Flush()
	return sb.String()
}

func (c *TasksCommand) output(id string) (*CommandResult, error) {
	state, lines, err := c.manager.Status(id)
	if err != nil {
		return nil, err
	}
	if len(lines) > 20 {
		lines = lines[len(lines)-20:]
	}
	return &CommandResult{
		Output:  fmt.Sprintf("[state=%s]\n%s", state, strings.Join(lines, "\n")),
		Success: true,
	}, nil
}

func (c *TasksCommand) stop(id string) (*CommandResult, error) {
	if err := c.manager.StopTask(id); err != nil {
		return nil, err
	}
	return &CommandResult{Output: fmt.Sprintf("stopped %s", id), Success: true}, nil
}
