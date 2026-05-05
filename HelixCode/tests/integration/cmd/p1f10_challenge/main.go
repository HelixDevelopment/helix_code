// p1f10_challenge runs the agent-invoked Skill flow against real .md files.
// Runtime-evidence harness for the F10 Challenge.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dev.helix.code/internal/agent"
	"dev.helix.code/internal/commands"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
}

func run() error {
	dir, err := os.MkdirTemp("", "p1f10-")
	if err != nil {
		return fmt.Errorf("tempdir: %w", err)
	}
	defer os.RemoveAll(dir)
	skills := filepath.Join(dir, ".helix", "skills")
	if err := os.MkdirAll(skills, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	fmt.Println("==> step 1: write refactor.md with named-capture trigger")
	refactorPath := filepath.Join(skills, "refactor.md")
	body := "---\ndescription: Refactor a React component\ntriggers:\n  - \"refactor (?P<comp>[A-Z][A-Za-z]+) component\"\n---\n\nRefactoring {{ARG.comp}}"
	if err := os.WriteFile(refactorPath, []byte(body), 0644); err != nil {
		return fmt.Errorf("write skill: %w", err)
	}

	reg := commands.NewSkillRegistry()
	loader := commands.NewSkillLoader(reg, skills, "")
	if err := loader.Load(); err != nil {
		return fmt.Errorf("load: %w", err)
	}
	loaded := loader.Loaded()
	if _, ok := loaded["refactor"]; !ok {
		return fmt.Errorf("expected refactor in loaded; got %v", loaded)
	}
	fmt.Println("    loaded:", loaded)

	fmt.Println("==> step 2: dispatcher.Match on 'refactor LoginButton component'")
	dispatcher := agent.NewSkillDispatcher(reg, nil)
	rendered, skill, caps, ok, err := dispatcher.Match(
		context.Background(),
		"please refactor LoginButton component now",
		"", "")
	if err != nil {
		return fmt.Errorf("match: %w", err)
	}
	if !ok {
		return fmt.Errorf("expected match")
	}
	if skill.Name() != "refactor" {
		return fmt.Errorf("expected skill 'refactor'; got %q", skill.Name())
	}
	if caps["comp"] != "LoginButton" {
		return fmt.Errorf("expected captures[comp]=LoginButton; got %q", caps["comp"])
	}
	if !strings.Contains(rendered, "Refactoring LoginButton") {
		return fmt.Errorf("rendered body missing 'Refactoring LoginButton'; got %q", rendered)
	}
	fmt.Println("    rendered =", strings.TrimSpace(rendered))
	fmt.Println("    captures =", caps)

	fmt.Println("==> step 3: mutate refactor.md to 'Now: {{ARG.comp}}', reload, re-match")
	body2 := "---\ndescription: New version\ntriggers:\n  - \"refactor (?P<comp>[A-Z][A-Za-z]+) component\"\n---\n\nNow: {{ARG.comp}}"
	if err := os.WriteFile(refactorPath, []byte(body2), 0644); err != nil {
		return fmt.Errorf("rewrite skill: %w", err)
	}
	if err := loader.Reload(); err != nil {
		return fmt.Errorf("reload: %w", err)
	}
	rendered2, _, _, ok, err := dispatcher.Match(
		context.Background(),
		"please refactor MainNav component",
		"", "")
	if err != nil {
		return fmt.Errorf("match after reload: %w", err)
	}
	if !ok {
		return fmt.Errorf("expected match after reload")
	}
	if !strings.Contains(rendered2, "Now: MainNav") {
		return fmt.Errorf("expected 'Now: MainNav' after reload; got %q", rendered2)
	}
	fmt.Println("    rendered =", strings.TrimSpace(rendered2))

	fmt.Println("==> step 4: remove file, reload, registry no longer matches")
	if err := os.Remove(refactorPath); err != nil {
		return fmt.Errorf("remove skill: %w", err)
	}
	if err := loader.Reload(); err != nil {
		return fmt.Errorf("reload after remove: %w", err)
	}
	_, _, _, stillMatched, _ := dispatcher.Match(
		context.Background(),
		"refactor LoginButton component",
		"", "")
	if stillMatched {
		return fmt.Errorf("expected no match after file removal")
	}
	fmt.Println("    skill unregistered after removal: OK")

	fmt.Println("==> P1-F10 challenge harness PASS")
	return nil
}
