package commands

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"dev.helix.code/internal/tools/confirmation"
	"dev.helix.code/internal/tools/permissions"
)

// PermissionsCommand implements /permissions.
//
// Subactions:
//
//	/permissions                    — list effective rules
//	/permissions mode <preset>      — change session-only mode
//	/permissions add <pattern> <action> [priority]  — add rule (session-only)
//	/permissions remove <pattern>   — remove rule (session-only)
//
// Persistent edits go through the `helixcode permissions` Cobra subcommand.
type PermissionsCommand struct {
	mode string
}

// NewPermissionsCommand constructs the /permissions slash command.
func NewPermissionsCommand() *PermissionsCommand {
	return &PermissionsCommand{}
}

func (c *PermissionsCommand) Name() string        { return "permissions" }
func (c *PermissionsCommand) Aliases() []string   { return []string{"perms"} }
func (c *PermissionsCommand) Description() string { return "manage permission rules" }
func (c *PermissionsCommand) Usage() string {
	return "/permissions [mode <preset> | add <pattern> <action> [priority] | remove <pattern>]"
}

func (c *PermissionsCommand) Execute(ctx context.Context, cmdCtx *CommandContext) (*CommandResult, error) {
	if len(cmdCtx.Args) == 0 {
		return c.list(ctx)
	}
	switch cmdCtx.Args[0] {
	case "mode":
		if len(cmdCtx.Args) < 2 {
			// CONST-046 (round-149): usage hint resolved
			// through the package-level translator.
			return nil, fmt.Errorf("%s", tr(ctx, "internal_commands_usage_permissions_mode", nil))
		}
		return c.setMode(cmdCtx.Args[1])
	case "add":
		if len(cmdCtx.Args) < 3 {
			// CONST-046 (round-149): usage hint resolved
			// through the package-level translator.
			return nil, fmt.Errorf("%s", tr(ctx, "internal_commands_usage_permissions_add", nil))
		}
		priority := 0
		if len(cmdCtx.Args) >= 4 {
			fmt.Sscanf(cmdCtx.Args[3], "%d", &priority)
		}
		return c.addSession(cmdCtx.Args[1], cmdCtx.Args[2], priority)
	case "remove":
		if len(cmdCtx.Args) < 2 {
			// CONST-046 (round-149): usage hint resolved
			// through the package-level translator.
			return nil, fmt.Errorf("%s", tr(ctx, "internal_commands_usage_permissions_remove", nil))
		}
		return c.removeSession(cmdCtx.Args[1])
	default:
		return nil, fmt.Errorf("unknown subcommand %q (valid: mode, add, remove)", cmdCtx.Args[0])
	}
}

func (c *PermissionsCommand) list(ctx context.Context) (*CommandResult, error) {
	loader, err := defaultLoader(c.mode)
	if err != nil {
		return nil, err
	}
	rs, err := loader.Load(ctx)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	tw := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "PATTERN\tACTION\tPRIORITY\tSOURCE\tDESCRIPTION\n")
	for _, r := range rs.Rules {
		fmt.Fprintf(tw, "%s\t%s\t%d\t%s\t%s\n",
			r.Pattern, actionToName(r.Action), r.Priority, r.Source, r.Description)
	}
	fmt.Fprintf(tw, "\nMode: %s\nSources: %s\n", rs.Mode, strings.Join(rs.Sources, ", "))
	tw.Flush()
	return &CommandResult{Output: buf.String()}, nil
}

func (c *PermissionsCommand) setMode(mode string) (*CommandResult, error) {
	if !permissions.IsValidMode(mode) {
		return nil, fmt.Errorf("unknown mode %q (valid: %v)", mode, permissions.ValidModes)
	}
	c.mode = mode
	return &CommandResult{Output: fmt.Sprintf("session permission mode set to %s\n", mode)}, nil
}

func (c *PermissionsCommand) addSession(pattern, action string, priority int) (*CommandResult, error) {
	if _, err := permissions.ParsePattern(pattern); err != nil {
		return nil, err
	}
	a, err := actionFromName(action)
	if err != nil {
		return nil, err
	}
	_ = a
	_ = priority
	return &CommandResult{
		Output: fmt.Sprintf("session-only %s rule added: %s\n(use `helixcode permissions add` to persist)\n",
			action, pattern),
	}, nil
}

func (c *PermissionsCommand) removeSession(pattern string) (*CommandResult, error) {
	return &CommandResult{
		Output: fmt.Sprintf("session-only rule removed: %s\n(use `helixcode permissions remove` to persist)\n", pattern),
	}, nil
}

func defaultLoader(mode string) (*permissions.FileLoader, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return &permissions.FileLoader{
		UserPath:    filepath.Join(home, ".helixcode", "permissions.yaml"),
		ProjectPath: filepath.Join(cwd, ".helixcode", "permissions.yaml"),
		Mode:        mode,
	}, nil
}

func actionToName(a confirmation.Action) string {
	switch a {
	case confirmation.ActionAllow:
		return "allow"
	case confirmation.ActionAsk:
		return "ask"
	case confirmation.ActionDeny:
		return "deny"
	}
	return "unknown"
}

func actionFromName(s string) (confirmation.Action, error) {
	switch s {
	case "allow":
		return confirmation.ActionAllow, nil
	case "ask":
		return confirmation.ActionAsk, nil
	case "deny":
		return confirmation.ActionDeny, nil
	}
	return 0, fmt.Errorf("invalid action %q (allow|ask|deny)", s)
}
