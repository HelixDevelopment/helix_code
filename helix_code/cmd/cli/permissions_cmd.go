package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"dev.helix.code/internal/tools/confirmation"
	"dev.helix.code/internal/tools/permissions"
)

// newPermissionsCommand returns the top-level "permissions" Cobra command with
// list / add / remove / check subcommands.
func newPermissionsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "permissions",
		Short: trc("cli_permissions_root_short", nil),
	}
	cmd.AddCommand(newPermissionsListCommand())
	cmd.AddCommand(newPermissionsAddCommand())
	cmd.AddCommand(newPermissionsRemoveCommand())
	cmd.AddCommand(newPermissionsCheckCommand())
	return cmd
}

func newPermissionsListCommand() *cobra.Command {
	var mode string
	cmd := &cobra.Command{
		Use:   "list",
		Short: trc("cli_permissions_list_short", nil),
		RunE: func(cmd *cobra.Command, args []string) error {
			user, project := defaultPaths()
			return runPermissionsList(os.Stdout, user, project, mode)
		},
	}
	cmd.Flags().StringVar(&mode, "permission-mode", "", "override mode preset")
	return cmd
}

func newPermissionsAddCommand() *cobra.Command {
	var (
		scope    string
		priority int
		descr    string
	)
	cmd := &cobra.Command{
		Use:   "add <pattern> <action>",
		Short: trc("cli_permissions_add_short", nil),
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := pathForScope(scope)
			if err != nil {
				return err
			}
			return runPermissionsAdd(path, args[0], args[1], priority, descr)
		},
	}
	cmd.Flags().StringVar(&scope, "scope", "user", "user|project")
	cmd.Flags().IntVar(&priority, "priority", 0, "rule priority (higher wins)")
	cmd.Flags().StringVar(&descr, "description", "", "free-text description")
	return cmd
}

func newPermissionsRemoveCommand() *cobra.Command {
	var scope string
	cmd := &cobra.Command{
		Use:   "remove <pattern>",
		Short: trc("cli_permissions_remove_short", nil),
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := pathForScope(scope)
			if err != nil {
				return err
			}
			return runPermissionsRemove(path, args[0])
		},
	}
	cmd.Flags().StringVar(&scope, "scope", "user", "user|project")
	return cmd
}

func newPermissionsCheckCommand() *cobra.Command {
	var (
		mode string
		body string
	)
	cmd := &cobra.Command{
		Use:   "check <tool>",
		Short: trc("cli_permissions_check_short", nil),
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			user, project := defaultPaths()
			return runPermissionsCheck(os.Stdout, user, project, mode, args[0], body)
		},
	}
	cmd.Flags().StringVar(&mode, "permission-mode", "", "override mode preset")
	cmd.Flags().StringVar(&body, "command", "", trc("cli_permissions_check_command_flag", nil))
	return cmd
}

// defaultPaths returns the canonical user and project YAML file paths.
func defaultPaths() (string, string) {
	home, _ := os.UserHomeDir()
	cwd, _ := os.Getwd()
	return filepath.Join(home, ".helixcode", "permissions.yaml"),
		filepath.Join(cwd, ".helixcode", "permissions.yaml")
}

// pathForScope maps a --scope flag value to the appropriate YAML file path.
func pathForScope(scope string) (string, error) {
	user, project := defaultPaths()
	switch scope {
	case "user":
		return user, nil
	case "project":
		return project, nil
	}
	return "", fmt.Errorf("unknown --scope %q (valid: user, project)", scope)
}

// runPermissionsList loads all rules from userPath and projectPath and prints
// them in a tabwriter-formatted table followed by the mode and source list.
func runPermissionsList(out io.Writer, userPath, projectPath, mode string) error {
	loader := &permissions.FileLoader{UserPath: userPath, ProjectPath: projectPath, Mode: mode}
	rs, err := loader.Load(context.Background())
	if err != nil {
		return err
	}
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "PATTERN\tACTION\tPRIORITY\tSOURCE\tDESCRIPTION\n")
	for _, r := range rs.Rules {
		fmt.Fprintf(tw, "%s\t%s\t%d\t%s\t%s\n",
			r.Pattern, actionName(r.Action), r.Priority, r.Source, r.Description)
	}
	fmt.Fprintf(tw, "\nMode: %s\nSources: %v\n", rs.Mode, rs.Sources)
	return tw.Flush()
}

// runPermissionsAdd validates the pattern and action then writes the rule to
// the specified YAML file (creating it if necessary).
func runPermissionsAdd(path, pattern, action string, priority int, description string) error {
	if _, err := permissions.ParsePattern(pattern); err != nil {
		return err
	}
	a, err := actionFromString(action)
	if err != nil {
		return err
	}
	scope := permissions.ScopeUser
	var loader *permissions.FileLoader
	if isProjectPath(path) {
		scope = permissions.ScopeProject
		loader = &permissions.FileLoader{UserPath: "", ProjectPath: path}
	} else {
		loader = &permissions.FileLoader{UserPath: path, ProjectPath: ""}
	}
	return loader.Save(context.Background(), scope, permissions.Rule{
		Pattern:     pattern,
		Action:      a,
		Priority:    priority,
		Description: description,
	})
}

// runPermissionsRemove deletes the rule with the given pattern from the file at path.
func runPermissionsRemove(path, pattern string) error {
	scope := permissions.ScopeUser
	var loader *permissions.FileLoader
	if isProjectPath(path) {
		scope = permissions.ScopeProject
		loader = &permissions.FileLoader{UserPath: "", ProjectPath: path}
	} else {
		loader = &permissions.FileLoader{UserPath: path, ProjectPath: ""}
	}
	return loader.Remove(context.Background(), scope, pattern)
}

// runPermissionsCheck performs a dry-run evaluation of tool+body against the
// rules loaded from userPath and projectPath, then prints the decision.
func runPermissionsCheck(out io.Writer, userPath, projectPath, mode, tool, body string) error {
	loader := &permissions.FileLoader{UserPath: userPath, ProjectPath: projectPath, Mode: mode}
	pe := confirmation.NewPolicyEngine()
	eng, err := permissions.NewEngine(context.Background(), loader, pe)
	if err != nil {
		return err
	}
	req := confirmation.ConfirmationRequest{
		ToolName:   tool,
		Parameters: map[string]interface{}{"command": body, "path": body, "file_path": body},
	}
	d := eng.Decide(req)
	fmt.Fprintf(out, "decision: %s\nmatched: %s\nreason: %s\n",
		actionName(d.Action), d.MatchedPattern, d.Reason)
	return nil
}

// isProjectPath returns true when path resolves to a location inside the
// current working directory (i.e., it is project-scoped rather than user-scoped).
func isProjectPath(path string) bool {
	cwd, err := os.Getwd()
	if err != nil {
		return false
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	cwdAbs, err := filepath.Abs(cwd)
	if err != nil {
		return false
	}
	// Ensure trailing separator so we don't match partial directory names.
	if !strings.HasSuffix(cwdAbs, string(filepath.Separator)) {
		cwdAbs += string(filepath.Separator)
	}
	return strings.HasPrefix(abs, cwdAbs)
}

// actionFromString converts a string action name to the confirmation.Action type.
func actionFromString(s string) (confirmation.Action, error) {
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

// actionName converts a confirmation.Action to its string representation.
func actionName(a confirmation.Action) string {
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
