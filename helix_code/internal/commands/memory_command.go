// Package commands — memory_command.go (P2-F24-T06).
//
// MemoryCommand implements the /memory slash command with four subcommands:
// status (default), show, edit, and reload. The user-facing surface for the
// F24 project memory subsystem.
//
// Subcommands:
//
//	/memory                → alias of /memory status
//	/memory status         → discovered paths + sizes + truncation flags +
//	                          last-loaded timestamp
//	/memory show           → concatenated content (Memory.Render()) — project
//	                          first, user overlay second with delimiter
//	/memory edit           → opens the project memory file in $EDITOR
//	                          (default vi). When no project memory file
//	                          exists yet, opens helixcode.md at cwd; the
//	                          editor's save creates the file.
//	/memory reload         → calls projectmemory.MemoryRegistry.Reload(ctx)
//	                          and reports the new sizes.
//
// Anti-bluff contract: every subcommand consults the LIVE registry. There
// is no cached state. /memory show returns whatever Memory.Render() returns
// at the moment of the call — including empty-string when no memory is
// loaded — and never fakes content.
//
// Style mirrors approval_command.go (commit 348630c, F21-T06): same Command
// interface, same tabwriter status block, same error envelope.
//
// References:
//   - Spec c31b9ac §3.4 (User surface)
//   - Plan 19094b8 T06
//   - F21 /approval precedent: internal/commands/approval_command.go
package commands

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"dev.helix.code/internal/projectmemory"
)

// MemoryCommand is the /memory slash command.
type MemoryCommand struct {
	registry *projectmemory.MemoryRegistry
	// editor is a test seam: returns the editor binary path/name to invoke
	// for /memory edit. Production sets this to default-resolution from
	// $EDITOR / fallback to "vi". Tests override to "true" (a unix exit-0
	// binary) so the command runs in CI without an interactive editor.
	editor func() string
}

// NewMemoryCommand constructs a /memory command bound to the given registry.
// Passing a nil registry is a programmer error and is not defended against —
// main.go always wires the real registry.
func NewMemoryCommand(r *projectmemory.MemoryRegistry) *MemoryCommand {
	return &MemoryCommand{
		registry: r,
		editor:   defaultEditor,
	}
}

// defaultEditor returns the user's preferred editor or "vi" as the POSIX-
// mandatory fallback. (Note: NOT "editor" / "nano" — those are debian-isms.)
func defaultEditor() string {
	if e := os.Getenv("EDITOR"); strings.TrimSpace(e) != "" {
		return e
	}
	return "vi"
}

// Name returns the slash command name (without the leading slash).
func (c *MemoryCommand) Name() string { return "memory" }

// Aliases returns alternative invocation names. /memory has none.
func (c *MemoryCommand) Aliases() []string { return nil }

// Description returns the one-line help blurb shown by /help.
func (c *MemoryCommand) Description() string {
	return "Inspect, show, edit, or reload project memory."
}

// Usage returns the usage string shown by /help.
func (c *MemoryCommand) Usage() string {
	return "/memory [status|show|edit|reload]"
}

// Execute dispatches to the appropriate subcommand handler.
//
// The default subcommand (no args) is `status` — answers the most common
// entry-point question: "what memory does the agent see right now?"
func (c *MemoryCommand) Execute(ctx context.Context, cc *CommandContext) (*CommandResult, error) {
	args := cc.Args
	sub := "status"
	if len(args) > 0 {
		sub = args[0]
	}
	switch sub {
	case "status":
		return &CommandResult{Success: true, Output: c.handleStatus()}, nil
	case "show":
		return &CommandResult{Success: true, Output: c.handleShow()}, nil
	case "edit":
		return c.handleEdit(ctx, cc)
	case "reload":
		return c.handleReload(ctx)
	default:
		return nil, fmt.Errorf("/memory: unknown subcommand %q (want status|show|edit|reload)", sub)
	}
}

// handleStatus renders paths + sizes + truncation flags + last-loaded
// timestamp. tabwriter alignment matches /approval and /theme so the visual
// shape stays consistent across slash commands.
func (c *MemoryCommand) handleStatus() string {
	m := c.registry.Snapshot()

	var sb strings.Builder
	sb.WriteString("Project memory status\n")
	tw := tabwriter.NewWriter(&sb, 0, 0, 2, ' ', 0)

	projPath := m.ProjectPath
	if projPath == "" {
		projPath = "(none)"
	}
	userPath := m.UserPath
	if userPath == "" {
		userPath = "(none)"
	}

	loadedAt := "(never loaded)"
	if !m.LoadedAt.IsZero() {
		loadedAt = m.LoadedAt.Format(time.RFC3339)
	}

	fmt.Fprintf(tw, "  Project path:\t%s\n", projPath)
	fmt.Fprintf(tw, "  Project size:\t%d bytes\n", len(m.Project))
	fmt.Fprintf(tw, "  Project truncated:\t%t\n", m.TruncatedProject)
	fmt.Fprintf(tw, "  User path:\t%s\n", userPath)
	fmt.Fprintf(tw, "  User size:\t%d bytes\n", len(m.User))
	fmt.Fprintf(tw, "  User truncated:\t%t\n", m.TruncatedUser)
	fmt.Fprintf(tw, "  Loaded at:\t%s\n", loadedAt)
	tw.Flush()
	return sb.String()
}

// handleShow renders Memory.Render(). When both project + user are empty,
// returns a clear message rather than an empty string (the user explicitly
// asked to see content; silent empty would feel broken).
func (c *MemoryCommand) handleShow() string {
	rendered := c.registry.Snapshot().Render()
	if rendered == "" {
		return "(no project memory loaded)\n"
	}
	return rendered
}

// handleEdit launches $EDITOR (or vi) on the resolved project memory path.
// When no project memory file exists yet, opens helixcode.md at the
// CommandContext's working directory (or process cwd as fallback). The
// editor's save will create the file; the watcher (if running) will then
// fire and the next /memory show / next LLM call sees the new content.
//
// Inherits stdio so the editor works normally. Blocks until the editor exits.
func (c *MemoryCommand) handleEdit(ctx context.Context, cc *CommandContext) (*CommandResult, error) {
	m := c.registry.Snapshot()
	path := m.ProjectPath
	if path == "" {
		base := cc.WorkingDir
		if base == "" {
			if cwd, err := os.Getwd(); err == nil {
				base = cwd
			}
		}
		path = filepath.Join(base, "helixcode.md")
	}

	editor := c.editor()
	cmd := exec.CommandContext(ctx, editor, path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("/memory edit: %s %s: %w", editor, path, err)
	}
	return &CommandResult{
		Success: true,
		Output:  fmt.Sprintf("edited: %s\n", path),
	}, nil
}

// handleReload triggers a fresh Discover via the registry and reports the
// new sizes. On error, surfaces the wrapped error to the user; the
// registry preserves its previous Snapshot.
func (c *MemoryCommand) handleReload(ctx context.Context) (*CommandResult, error) {
	m, err := c.registry.Reload(ctx)
	if err != nil {
		return nil, fmt.Errorf("/memory reload: %w", err)
	}
	return &CommandResult{
		Success: true,
		Output: fmt.Sprintf(
			"reloaded: project=%d bytes (truncated=%t), user=%d bytes (truncated=%t)\n",
			len(m.Project), m.TruncatedProject, len(m.User), m.TruncatedUser),
	}, nil
}
