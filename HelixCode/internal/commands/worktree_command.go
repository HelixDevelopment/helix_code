package commands

import (
	"bytes"
	"context"
	"fmt"
	"text/tabwriter"

	"dev.helix.code/internal/tools/worktree"
)

// WorktreeCommand implements /worktree.
//
// Subactions:
//
//	/worktree                          — list (default)
//	/worktree list                     — explicit list
//	/worktree enter <name> [branch]    — enters worktree (mutates state)
//	/worktree exit                     — returns to main
//	/worktree remove <name>            — deletes worktree + branch
type WorktreeCommand struct {
	m *worktree.Manager
}

// NewWorktreeCommand wires a Manager into the slash command.
func NewWorktreeCommand(m *worktree.Manager) *WorktreeCommand {
	return &WorktreeCommand{m: m}
}

func (c *WorktreeCommand) Name() string        { return "worktree" }
func (c *WorktreeCommand) Aliases() []string   { return []string{"wt"} }
func (c *WorktreeCommand) Description() string { return "manage helix-tracked git worktrees" }
func (c *WorktreeCommand) Usage() string {
	return "/worktree [list | enter <name> [branch] | exit | remove <name>]"
}

func (c *WorktreeCommand) Execute(ctx context.Context, cmdCtx *CommandContext) (*CommandResult, error) {
	if len(cmdCtx.Args) == 0 {
		return c.list(ctx)
	}
	switch cmdCtx.Args[0] {
	case "list":
		return c.list(ctx)
	case "enter":
		if len(cmdCtx.Args) < 2 {
			return nil, fmt.Errorf("usage: /worktree enter <name> [branch]")
		}
		baseBranch := ""
		if len(cmdCtx.Args) >= 3 {
			baseBranch = cmdCtx.Args[2]
		}
		return c.enter(ctx, cmdCtx.Args[1], baseBranch)
	case "exit":
		return c.exit()
	case "remove":
		if len(cmdCtx.Args) < 2 {
			return nil, fmt.Errorf("usage: /worktree remove <name>")
		}
		return c.remove(ctx, cmdCtx.Args[1])
	default:
		return nil, fmt.Errorf("unknown /worktree subaction %q (valid: list, enter, exit, remove)", cmdCtx.Args[0])
	}
}

func (c *WorktreeCommand) list(ctx context.Context) (*CommandResult, error) {
	wts, err := c.m.ListWorktrees(ctx)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	tw := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "NAME\tBRANCH\tPATH\n")
	for _, w := range wts {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", w.Name, w.Branch, w.Path)
	}
	if len(wts) == 0 {
		fmt.Fprintln(tw, "(no worktrees)\t\t")
	}
	tw.Flush()
	return &CommandResult{Output: buf.String(), Success: true}, nil
}

func (c *WorktreeCommand) enter(ctx context.Context, name, baseBranch string) (*CommandResult, error) {
	path, err := c.m.EnterWorktree(ctx, name, baseBranch)
	if err != nil {
		return nil, err
	}
	return &CommandResult{
		Output:  fmt.Sprintf("entered worktree %q at %s\n", name, path),
		Success: true,
	}, nil
}

func (c *WorktreeCommand) exit() (*CommandResult, error) {
	c.m.ExitWorktree()
	return &CommandResult{
		Output:  "exited worktree; returned to main\n",
		Success: true,
	}, nil
}

func (c *WorktreeCommand) remove(ctx context.Context, name string) (*CommandResult, error) {
	if err := c.m.RemoveWorktree(ctx, name); err != nil {
		return nil, err
	}
	return &CommandResult{
		Output:  fmt.Sprintf("removed worktree %q\n", name),
		Success: true,
	}, nil
}
