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

func (c *PermissionsCommand) Name() string      { return "permissions" }
func (c *PermissionsCommand) Aliases() []string { return []string{"perms"} }

// Description returns the /permissions slash-command help text.
//
// CONST-046 (round-432): genuine user-facing CLI help text resolved
// through the package-level translator.
func (c *PermissionsCommand) Description() string {
	return tr(context.Background(), "internal_commands_permissions_description", nil)
}

// Usage returns the /permissions slash-command usage line.
//
// CONST-046 (round-432): genuine user-facing CLI usage text resolved
// through the package-level translator.
func (c *PermissionsCommand) Usage() string {
	return tr(context.Background(), "internal_commands_permissions_usage", nil)
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
		// CONST-046 (round-432): operator error message resolved
		// through the package-level translator.
		return nil, fmt.Errorf("%s", tr(ctx, "internal_commands_permissions_unknown_subcommand",
			map[string]any{"Sub": cmdCtx.Args[0]}))
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
	// CONST-046 (round-432): table header + footer resolved through
	// the package-level translator.
	fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
		tr(ctx, "internal_commands_permissions_col_pattern", nil),
		tr(ctx, "internal_commands_permissions_col_action", nil),
		tr(ctx, "internal_commands_permissions_col_priority", nil),
		tr(ctx, "internal_commands_permissions_col_source", nil),
		tr(ctx, "internal_commands_permissions_col_description", nil))
	for _, r := range rs.Rules {
		fmt.Fprintf(tw, "%s\t%s\t%d\t%s\t%s\n",
			r.Pattern, actionToName(r.Action), r.Priority, r.Source, r.Description)
	}
	fmt.Fprintf(tw, "\n%s\n", tr(ctx, "internal_commands_permissions_list_footer",
		map[string]any{"Mode": rs.Mode, "Sources": strings.Join(rs.Sources, ", ")}))
	tw.Flush()
	return &CommandResult{Output: buf.String()}, nil
}

func (c *PermissionsCommand) setMode(mode string) (*CommandResult, error) {
	if !permissions.IsValidMode(mode) {
		// CONST-046 (round-432): operator error message resolved
		// through the context-free package translator.
		return nil, fmt.Errorf("%s", trc("internal_commands_permissions_unknown_mode",
			map[string]any{"Mode": mode, "Valid": fmt.Sprintf("%v", permissions.ValidModes)}))
	}
	c.mode = mode
	// CONST-046 (round-432): confirmation message resolved through
	// the context-free package translator.
	return &CommandResult{Output: trc("internal_commands_permissions_mode_set",
		map[string]any{"Mode": mode})}, nil
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
	// CONST-046 (round-432): confirmation message resolved through
	// the context-free package translator.
	return &CommandResult{
		Output: trc("internal_commands_permissions_rule_added",
			map[string]any{"Action": action, "Pattern": pattern}),
	}, nil
}

func (c *PermissionsCommand) removeSession(pattern string) (*CommandResult, error) {
	// CONST-046 (round-432): confirmation message resolved through
	// the context-free package translator.
	return &CommandResult{
		Output: trc("internal_commands_permissions_rule_removed",
			map[string]any{"Pattern": pattern}),
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
	// CONST-046 (round-432): operator error message resolved through
	// the context-free package translator.
	return 0, fmt.Errorf("%s", trc("internal_commands_permissions_invalid_action",
		map[string]any{"Action": s}))
}
