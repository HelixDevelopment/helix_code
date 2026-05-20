package commands

import (
	"context"
	"fmt"
	"strings"
	"text/tabwriter"

	"dev.helix.code/internal/session"
)

// SessionsCommand implements the /sessions slash command.
type SessionsCommand struct {
	store          session.SessionStore
	currentProject string
}

// NewSessionsCommand returns a /sessions command bound to a SessionStore.
// currentProject is used to filter the default `list` output by project.
func NewSessionsCommand(store session.SessionStore, currentProject string) *SessionsCommand {
	return &SessionsCommand{store: store, currentProject: currentProject}
}

func (c *SessionsCommand) Name() string        { return "sessions" }
func (c *SessionsCommand) Aliases() []string   { return nil }
func (c *SessionsCommand) Description() string {
	// CONST-046 (round-393): genuine user-facing CLI help text
	// resolved through the package-level translator.
	return tr(context.Background(), "internal_commands_sessions_description", nil)
}
func (c *SessionsCommand) Usage() string {
	return tr(context.Background(), "internal_commands_sessions_usage", nil)
}

// Execute dispatches to the appropriate subcommand handler.
func (c *SessionsCommand) Execute(ctx context.Context, cc *CommandContext) (*CommandResult, error) {
	args := cc.Args
	sub := "list"
	if len(args) > 0 {
		sub = args[0]
	}
	var rest []string
	if len(args) > 1 {
		rest = args[1:]
	}
	switch sub {
	case "list":
		return c.list(ctx, rest)
	case "show":
		if len(rest) == 0 {
			return nil, fmt.Errorf("%s", tr(ctx, "internal_commands_sessions_show_usage", nil))
		}
		return c.show(ctx, rest[0])
	case "resume":
		if len(rest) == 0 {
			return nil, fmt.Errorf("%s", tr(ctx, "internal_commands_sessions_resume_usage", nil))
		}
		return c.resume(ctx, rest[0])
	case "delete":
		if len(rest) == 0 {
			return nil, fmt.Errorf("%s", tr(ctx, "internal_commands_sessions_delete_usage", nil))
		}
		return c.delete(ctx, rest[0])
	default:
		return nil, fmt.Errorf("%s", tr(ctx, "internal_commands_sessions_unknown_subcommand", map[string]any{"Sub": sub}))
	}
}

// list renders a tab-aligned table of all sessions, project-scoped by default.
func (c *SessionsCommand) list(ctx context.Context, rest []string) (*CommandResult, error) {
	all := false
	for _, a := range rest {
		if a == "--all" {
			all = true
		}
	}
	scope := c.currentProject
	if all {
		scope = ""
	}
	metas, err := c.store.ListSessionMetadata(ctx, scope)
	if err != nil {
		return nil, err
	}
	var sb strings.Builder
	tw := tabwriter.NewWriter(&sb, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tPROJECT\tSTARTED\tLAST-ACTIVITY\tMSG-COUNT")
	for _, m := range metas {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%d\n",
			m.SessionID, m.ProjectName,
			m.StartedAt.Format("2006-01-02 15:04:05"),
			m.LastActivity.Format("2006-01-02 15:04:05"),
			m.MessageCount)
	}
	tw.Flush()
	return &CommandResult{Success: true, Output: sb.String()}, nil
}

// show returns the metadata and last 20 transcript messages of a session.
func (c *SessionsCommand) show(ctx context.Context, id string) (*CommandResult, error) {
	meta, err := c.store.GetSessionMetadata(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("/sessions show: %w", err)
	}
	msgs, _ := c.store.ReadTranscript(ctx, id)
	var sb strings.Builder
	fmt.Fprintf(&sb, "Session: %s\n", meta.SessionID)
	fmt.Fprintf(&sb, "Project: %s (%s)\n", meta.ProjectName, meta.ProjectPath)
	fmt.Fprintf(&sb, "Started: %s\n", meta.StartedAt.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(&sb, "Last activity: %s\n", meta.LastActivity.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(&sb, "Messages: %d\n\n", meta.MessageCount)
	fmt.Fprintln(&sb, "--- Transcript (last 20) ---")
	start := 0
	if len(msgs) > 20 {
		start = len(msgs) - 20
	}
	for _, m := range msgs[start:] {
		fmt.Fprintf(&sb, "[%s] %s\n", m.Role, strings.TrimSpace(m.Content))
	}
	return &CommandResult{Success: true, Output: sb.String()}, nil
}

// resume returns a confirmation message for resuming the given session.
func (c *SessionsCommand) resume(ctx context.Context, id string) (*CommandResult, error) {
	meta, err := c.store.GetSessionMetadata(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("/sessions resume: %w", err)
	}
	return &CommandResult{
		Success: true,
		Output: tr(ctx, "internal_commands_sessions_resume_result", map[string]any{
			"ID":           meta.SessionID,
			"MessageCount": meta.MessageCount,
			"LastActive":   meta.LastActivity.Format("2006-01-02 15:04:05"),
		}),
	}, nil
}

// delete removes a session and its transcript from the store.
func (c *SessionsCommand) delete(ctx context.Context, id string) (*CommandResult, error) {
	if err := c.store.DeleteSession(ctx, id); err != nil {
		return nil, fmt.Errorf("/sessions delete: %w", err)
	}
	return &CommandResult{Success: true, Output: fmt.Sprintf("deleted session %s", id)}, nil
}
