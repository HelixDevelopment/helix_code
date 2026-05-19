package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"dev.helix.code/internal/hooks"
)

func newHooksCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hooks",
		Short: trc("cli_hooks_root_short", nil),
	}
	cmd.AddCommand(newHooksListCommand())
	cmd.AddCommand(newHooksValidateCommand())
	cmd.AddCommand(newHooksTestCommand())
	cmd.AddCommand(newHooksEnableCommand())
	cmd.AddCommand(newHooksDisableCommand())
	return cmd
}

func newHooksListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: trc("cli_hooks_list_short", nil),
		RunE: func(cmd *cobra.Command, args []string) error {
			user, project := defaultHooksPaths()
			return runHooksList(os.Stdout, user, project)
		},
	}
}

func newHooksValidateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: trc("cli_hooks_validate_short", nil),
		RunE: func(cmd *cobra.Command, args []string) error {
			user, project := defaultHooksPaths()
			return runHooksValidate(os.Stdout, user, project)
		},
	}
}

func newHooksTestCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "test <event-name>",
		Short: trc("cli_hooks_test_short", nil),
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			user, project := defaultHooksPaths()
			return runHooksTest(os.Stdout, user, project, args[0])
		},
	}
}

func newHooksEnableCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "enable <id>",
		Short: trc("cli_hooks_enable_short", nil),
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			user, _ := defaultHooksPaths()
			return runHooksEnable(user, args[0])
		},
	}
}

func newHooksDisableCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "disable <id>",
		Short: trc("cli_hooks_disable_short", nil),
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			user, _ := defaultHooksPaths()
			return runHooksDisable(user, args[0])
		},
	}
}

func defaultHooksPaths() (string, string) {
	home, _ := os.UserHomeDir()
	cwd, _ := os.Getwd()
	return filepath.Join(home, ".helixcode", "hooks.yaml"),
		filepath.Join(cwd, ".helixcode", "hooks.yaml")
}

func runHooksList(out io.Writer, userPath, projectPath string) error {
	loader := &hooks.FileLoader{UserPath: userPath, ProjectPath: projectPath}
	hs, sources, err := loader.Load(context.Background())
	if err != nil {
		return err
	}
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "ID\tEVENT\tPRIORITY\tASYNC\tSCRIPT\n")
	for _, h := range hs {
		fmt.Fprintf(tw, "%s\t%s\t%d\t%v\t%s\n", h.ID, h.Type, h.Priority, h.Async, h.Metadata["script"])
	}
	if len(hs) == 0 {
		fmt.Fprintln(tw, "(no hooks loaded)\t\t\t\t")
	}
	fmt.Fprintf(tw, "\nSources: %v\n", sources)
	return tw.Flush()
}

func runHooksValidate(out io.Writer, userPath, projectPath string) error {
	loader := &hooks.FileLoader{UserPath: userPath, ProjectPath: projectPath}
	hs, sources, err := loader.Load(context.Background())
	if err != nil {
		return err
	}
	fmt.Fprintln(out, trc("cli_hooks_validate_ok",
		map[string]any{"Count": len(hs), "Sources": fmt.Sprintf("%v", sources)}))
	return nil
}

func runHooksTest(out io.Writer, userPath, projectPath, eventName string) error {
	loader := &hooks.FileLoader{UserPath: userPath, ProjectPath: projectPath}
	hs, _, err := loader.Load(context.Background())
	if err != nil {
		return err
	}
	mgr := hooks.NewManager()
	for _, h := range hs {
		scriptPath := h.Metadata["script"]
		h.Handler = hooks.NewShellRunner(scriptPath, h.Timeout)
		// Hooks loaded from YAML with no explicit priority get 0; clamp to Normal.
		if h.Priority < hooks.PriorityLowest {
			h.Priority = hooks.PriorityNormal
		}
		if err := mgr.Register(h); err != nil {
			return fmt.Errorf("registering %s: %w", h.ID, err)
		}
	}
	event := hooks.NewEvent(hooks.HookType(eventName))
	results := mgr.TriggerEventAndWait(event)
	for _, r := range results {
		fmt.Fprintln(out, trc("cli_hooks_test_result", map[string]any{
			"HookID":   r.HookID,
			"Status":   fmt.Sprintf("%v", r.Status),
			"Error":    fmt.Sprintf("%v", r.Error),
			"Duration": fmt.Sprintf("%v", r.Duration),
		}))
	}
	if len(results) == 0 {
		fmt.Fprintf(out, "(no hooks registered for event %q)\n", eventName)
	}
	return nil
}

func runHooksEnable(userPath, id string) error {
	return setHookEnabled(userPath, id, true)
}

func runHooksDisable(userPath, id string) error {
	return setHookEnabled(userPath, id, false)
}

// setHookEnabled mutates user's hooks.yaml using yaml.v3 Node-based round
// trip so user comments are preserved.
func setHookEnabled(userPath, id string, want bool) error {
	body, err := os.ReadFile(userPath)
	if err != nil {
		return err
	}
	var root yaml.Node
	if err := yaml.Unmarshal(body, &root); err != nil {
		return err
	}
	if root.Kind != yaml.DocumentNode || len(root.Content) == 0 {
		return fmt.Errorf("unexpected YAML structure in %s", userPath)
	}
	doc := root.Content[0]
	if doc.Kind != yaml.MappingNode {
		return fmt.Errorf("expected top-level mapping in %s", userPath)
	}
	hooksNode := childByKey(doc, "hooks")
	if hooksNode == nil || hooksNode.Kind != yaml.SequenceNode {
		return fmt.Errorf("hooks: key not a sequence in %s", userPath)
	}
	for _, item := range hooksNode.Content {
		if item.Kind != yaml.MappingNode {
			continue
		}
		idNode := childByKey(item, "id")
		if idNode == nil || idNode.Value != id {
			continue
		}
		setOrInsertBool(item, "enabled", want)
		out, err := yaml.Marshal(&root)
		if err != nil {
			return err
		}
		return os.WriteFile(userPath, out, 0o600)
	}
	return fmt.Errorf("hook %q not found in %s", id, userPath)
}

func childByKey(m *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

func setOrInsertBool(m *yaml.Node, key string, val bool) {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			m.Content[i+1].Value = boolStr(val)
			m.Content[i+1].Tag = "!!bool"
			return
		}
	}
	m.Content = append(m.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: key},
		&yaml.Node{Kind: yaml.ScalarNode, Value: boolStr(val), Tag: "!!bool"},
	)
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
