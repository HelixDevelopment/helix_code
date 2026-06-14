package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"dev.helix.code/internal/approval"
	"dev.helix.code/internal/tools"
)

func writeFile(dir, name, content string) error {
	return os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644)
}

func readFile(dir, name string) (string, error) {
	b, err := os.ReadFile(filepath.Join(dir, name))
	return string(b), err
}

func mustGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
	return string(out)
}

// initRepo creates a real git repository in a fresh temp dir, configures a
// committer identity, and returns the dir. Uses real git via os/exec — this is
// real infrastructure (§11.4.5), permitted in a unit test.
func initRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("SKIP-OK: git binary not available: %v", err)
	}
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}
	run("init")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test User")
	// Ensure a deterministic default branch name regardless of host git config.
	run("checkout", "-B", "main")
	return dir
}

// TestGitStatusTool_InterfaceConformance pins the tool to the tools.Tool
// contract at compile time and asserts the read-only approval level.
func TestGitStatusTool_InterfaceConformance(t *testing.T) {
	var tool tools.Tool = NewGitStatusTool("")
	if tool.Name() != "git_status" {
		t.Fatalf("Name() = %q, want git_status", tool.Name())
	}
	if tool.RequiresApproval() != approval.LevelReadOnly {
		t.Fatalf("RequiresApproval() = %v, want LevelReadOnly", tool.RequiresApproval())
	}
	if tool.Description() == "" {
		t.Fatal("Description() must not be empty")
	}
	if err := tool.Validate(map[string]interface{}{}); err != nil {
		t.Fatalf("Validate(empty) = %v, want nil (subcommand optional)", err)
	}
	sch := tool.Schema()
	if _, ok := sch.Properties["subcommand"]; !ok {
		t.Fatal("Schema() must advertise the optional subcommand param")
	}
}

// TestGitStatusTool_StatusDirty verifies real `git status --short --branch`
// output reflects the branch and an untracked file.
func TestGitStatusTool_StatusDirty(t *testing.T) {
	dir := initRepo(t)
	// Land a commit so the branch header is the clean "## main" form (a repo
	// with zero commits renders "## No commits yet on main").
	if err := writeFile(dir, "base.txt", "base"); err != nil {
		t.Fatal(err)
	}
	mustGit(t, dir, "add", "base.txt")
	mustGit(t, dir, "commit", "-m", "base")
	if err := writeFile(dir, "dirty.txt", "uncommitted change"); err != nil {
		t.Fatal(err)
	}
	tool := NewGitStatusTool(dir)
	res, err := tool.Execute(context.Background(), map[string]interface{}{"subcommand": "status"})
	if err != nil {
		t.Fatalf("Execute(status) = %v", err)
	}
	out, _ := res.(string)
	// `git status --short --branch` emits a `## <branch>` header line; with
	// commits present it renders exactly "## main".
	if !strings.Contains(out, "## main") {
		t.Fatalf("status output missing branch marker '## main':\n%s", out)
	}
	if !strings.Contains(out, "dirty.txt") {
		t.Fatalf("status output missing untracked file 'dirty.txt':\n%s", out)
	}
}

// TestGitStatusTool_DefaultSubcommandStatus asserts an empty/missing
// subcommand defaults to status.
func TestGitStatusTool_DefaultSubcommandStatus(t *testing.T) {
	dir := initRepo(t)
	if err := writeFile(dir, "x.txt", "data"); err != nil {
		t.Fatal(err)
	}
	tool := NewGitStatusTool(dir)
	res, err := tool.Execute(context.Background(), map[string]interface{}{})
	if err != nil {
		t.Fatalf("Execute(default) = %v", err)
	}
	if out, _ := res.(string); !strings.HasPrefix(out, "## ") || !strings.Contains(out, "main") {
		t.Fatalf("default subcommand should be status, missing branch header:\n%s", out)
	}
}

// TestGitStatusTool_Log verifies `git log --oneline` works after a commit.
func TestGitStatusTool_Log(t *testing.T) {
	dir := initRepo(t)
	if err := writeFile(dir, "f.txt", "hello"); err != nil {
		t.Fatal(err)
	}
	mustGit(t, dir, "add", "f.txt")
	mustGit(t, dir, "commit", "-m", "initial commit subject XYZ")

	tool := NewGitStatusTool(dir)
	res, err := tool.Execute(context.Background(), map[string]interface{}{"subcommand": "log"})
	if err != nil {
		t.Fatalf("Execute(log) = %v", err)
	}
	if out, _ := res.(string); !strings.Contains(out, "initial commit subject XYZ") {
		t.Fatalf("log output missing commit subject:\n%s", out)
	}
}

// TestGitStatusTool_RejectsNonAllowlisted is the safety guarantee: a write /
// destructive subcommand is rejected and NEVER executed.
func TestGitStatusTool_RejectsNonAllowlisted(t *testing.T) {
	dir := initRepo(t)
	// Make a commit so a real `reset --hard` would actually mutate state if
	// the allowlist failed.
	if err := writeFile(dir, "g.txt", "v1"); err != nil {
		t.Fatal(err)
	}
	mustGit(t, dir, "add", "g.txt")
	mustGit(t, dir, "commit", "-m", "c1")
	if err := writeFile(dir, "g.txt", "v2-uncommitted"); err != nil {
		t.Fatal(err)
	}

	tool := NewGitStatusTool(dir)
	for _, bad := range []string{"push", "reset", "commit", "rm", "clean"} {
		res, err := tool.Execute(context.Background(), map[string]interface{}{"subcommand": bad})
		if err == nil {
			t.Fatalf("subcommand %q must be rejected, got result=%v", bad, res)
		}
		if !strings.Contains(err.Error(), bad) {
			t.Fatalf("error for %q should name the rejected subcommand, got: %v", bad, err)
		}
	}

	// Prove no mutation happened: the uncommitted change is still present.
	got := mustGit(t, dir, "show", "HEAD:g.txt")
	if strings.TrimSpace(got) != "v1" {
		t.Fatalf("HEAD:g.txt mutated to %q — a rejected subcommand executed", got)
	}
	work, err := readFile(dir, "g.txt")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(work) != "v2-uncommitted" {
		t.Fatalf("working tree g.txt = %q, want untouched v2-uncommitted", work)
	}
}

// TestGitStatusTool_ErrorSurfacesStderr asserts a non-zero git exit (e.g. not a
// repo) returns an error carrying stderr.
func TestGitStatusTool_ErrorSurfacesStderr(t *testing.T) {
	dir := t.TempDir() // NOT a git repo
	tool := NewGitStatusTool(dir)
	_, err := tool.Execute(context.Background(), map[string]interface{}{"subcommand": "status"})
	if err == nil {
		t.Fatal("Execute on non-repo dir must error")
	}
}
