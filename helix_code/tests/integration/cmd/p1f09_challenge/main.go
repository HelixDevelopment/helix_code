// p1f09_challenge runs the full Markdown slash-command flow against a real
// filesystem and a real registry. Runtime-evidence harness for the F09 Challenge.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dev.helix.code/internal/commands"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
}

func run() error {
	dir, err := os.MkdirTemp("", "p1f09-challenge-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	cmds := filepath.Join(dir, ".helix", "commands")
	if err := os.MkdirAll(cmds, 0755); err != nil {
		return err
	}

	fmt.Println("==> step 1: write echo.md with body 'Got: {{ARG1}}'")
	echoPath := filepath.Join(cmds, "echo.md")
	if err := os.WriteFile(echoPath, []byte("Got: {{ARG1}}"), 0644); err != nil {
		return err
	}

	fmt.Println("==> step 2: load registry from", cmds)
	reg := commands.NewRegistry()
	loader := commands.NewMarkdownLoader(reg, cmds, "")
	if err := loader.Load(); err != nil {
		return err
	}
	loaded := loader.Loaded()
	fmt.Printf("    loaded: %v\n", loaded)
	if _, ok := reg.Get("echo"); !ok {
		return fmt.Errorf("echo command not registered after Load")
	}

	fmt.Println("==> step 3: execute echo command with arg 'hello world'")
	cmd, _ := reg.Get("echo")
	res, err := cmd.Execute(context.Background(), &commands.CommandContext{Args: []string{"hello world"}})
	if err != nil {
		return fmt.Errorf("execute: %w", err)
	}
	out := strings.TrimSpace(res.Output)
	fmt.Println("    output =", out)
	if !strings.Contains(out, "Got: hello world") {
		return fmt.Errorf("expected 'Got: hello world' in output, got %q", out)
	}

	fmt.Println("==> step 4: mutate file body to 'New: {{ARG1}}'; reload; re-run")
	if err := os.WriteFile(echoPath, []byte("New: {{ARG1}}"), 0644); err != nil {
		return err
	}
	if err := loader.Reload(); err != nil {
		return err
	}
	cmd2, _ := reg.Get("echo")
	res2, err := cmd2.Execute(context.Background(), &commands.CommandContext{Args: []string{"second-run"}})
	if err != nil {
		return err
	}
	out2 := strings.TrimSpace(res2.Output)
	fmt.Println("    output =", out2)
	if !strings.Contains(out2, "New: second-run") {
		return fmt.Errorf("expected 'New: second-run' in output, got %q", out2)
	}

	fmt.Println("==> step 5: delete file; reload; verify command unregistered")
	if err := os.Remove(echoPath); err != nil {
		return err
	}
	if err := loader.Reload(); err != nil {
		return err
	}
	if _, ok := reg.Get("echo"); ok {
		return fmt.Errorf("echo command should be unregistered after file deletion")
	}
	fmt.Println("    echo command unregistered: ok")

	fmt.Println("==> P1-F09 challenge harness PASS")
	return nil
}
