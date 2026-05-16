// Package commands — git_auto_commit_command.go (P2-F22-T07).
//
// GitAutoCommitCommand implements the /git_auto_commit slash command for
// per-edit git auto-commit (F22). Subcommands:
//
//	/git_auto_commit                → alias of /git_auto_commit status
//	/git_auto_commit status         → current enabled state + git_repo + trailer
//	/git_auto_commit on             → SetEnabled(true)  via committer
//	/git_auto_commit off            → SetEnabled(false) via committer
//	/git_auto_commit show           → describe the auto-commit format
//
// Anti-bluff contract: the command MUST consult the live AutoCommitter
// (via AutoCommitInspector) for every status read and route every
// on/off through SetEnabled. There is no cached state.
//
// Style mirrors approval_command.go (commit ad8843b, F21-T06): same
// Command interface, same simple key:value status block.
//
// References:
//   - Spec 8be7fba §4 (User surface)
//   - Plan b4f217d T07
//   - F21 /approval precedent: internal/commands/approval_command.go
package commands

import (
	"context"
	"fmt"
	"strings"

	"dev.helix.code/internal/autocommit"
)

// AutoCommitInspector is the subset of *autocommit.AutoCommitter that
// GitAutoCommitCommand depends on. Defining the interface here keeps
// the slash command testable with a fake while still letting main.go
// pass the real *autocommit.AutoCommitter directly (Go satisfies
// interfaces structurally).
type AutoCommitInspector interface {
	Enabled() bool
	SetEnabled(bool)
	IsGitRepo() bool
}

// GitAutoCommitCommand is the /git_auto_commit slash command.
type GitAutoCommitCommand struct {
	committer AutoCommitInspector
}

// NewGitAutoCommitCommand constructs the /git_auto_commit slash command
// bound to the supplied inspector. nil inspector is supported but every
// subcommand will fail gracefully (status reports a disabled state).
func NewGitAutoCommitCommand(c AutoCommitInspector) *GitAutoCommitCommand {
	return &GitAutoCommitCommand{committer: c}
}

// Name returns the slash command name (without the leading slash).
func (c *GitAutoCommitCommand) Name() string { return "git_auto_commit" }

// Aliases returns alternative invocation names. None.
func (c *GitAutoCommitCommand) Aliases() []string { return nil }

// Description returns the one-line help blurb shown by /help.
func (c *GitAutoCommitCommand) Description() string {
	return "Show or change git auto-commit (per-edit) state."
}

// Usage returns the usage string shown by /help.
func (c *GitAutoCommitCommand) Usage() string {
	return "/git_auto_commit [status|on|off|show]"
}

// Execute dispatches to the appropriate subcommand handler. Default
// subcommand (no args) is `status`.
func (c *GitAutoCommitCommand) Execute(_ context.Context, cc *CommandContext) (*CommandResult, error) {
	sub := "status"
	if len(cc.Args) > 0 {
		sub = strings.ToLower(strings.TrimSpace(cc.Args[0]))
	}
	switch sub {
	case "status", "":
		return &CommandResult{Success: true, Output: c.handleStatus()}, nil
	case "on":
		return c.handleOn()
	case "off":
		return c.handleOff()
	case "show":
		return &CommandResult{Success: true, Output: c.handleShow()}, nil
	default:
		return nil, fmt.Errorf("/git_auto_commit: unknown subcommand %q (want status|on|off|show)", sub)
	}
}

// handleStatus renders the current state block.
func (c *GitAutoCommitCommand) handleStatus() string {
	state := "off"
	repoState := "no"
	if c.committer != nil {
		if c.committer.Enabled() {
			state = "on"
		}
		if c.committer.IsGitRepo() {
			repoState = "yes"
		}
	}
	return fmt.Sprintf(
		"git_auto_commit: %s\ngit_repo: %s\nco-author trailer: %s\n",
		state, repoState, autocommit.CoAuthorTrailer,
	)
}

// handleOn flips the committer's enabled flag to true.
func (c *GitAutoCommitCommand) handleOn() (*CommandResult, error) {
	if c.committer == nil {
		return nil, fmt.Errorf("/git_auto_commit: committer not configured")
	}
	c.committer.SetEnabled(true)
	return &CommandResult{Success: true, Output: "git_auto_commit -> on\n"}, nil
}

// handleOff flips the committer's enabled flag to false.
func (c *GitAutoCommitCommand) handleOff() (*CommandResult, error) {
	if c.committer == nil {
		return nil, fmt.Errorf("/git_auto_commit: committer not configured")
	}
	c.committer.SetEnabled(false)
	return &CommandResult{Success: true, Output: "git_auto_commit -> off\n"}, nil
}

// handleShow returns a descriptive block about the auto-commit format.
func (c *GitAutoCommitCommand) handleShow() string {
	return fmt.Sprintf(
		"Auto-commit format:\n"+
			"  subject: <LLM-summarised, <=72 chars>\n"+
			"  body:    (blank line)\n"+
			"           %s\n\n"+
			"Opt-out:\n"+
			"  env:     %s=off\n"+
			"  slash:   /git_auto_commit off\n"+
			"  per-edit: %s:true in tool params\n",
		autocommit.CoAuthorTrailer,
		autocommit.EnvVarName,
		autocommit.SkipParamKey,
	)
}
