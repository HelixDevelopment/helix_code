package git

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"sort"
	"strings"

	"dev.helix.code/internal/approval"
	"dev.helix.code/internal/tools"
)

// GitStatusTool is a SAFE, READ-ONLY git inspection tool for the LLM tool loop.
//
// It lets an agent answer "check git status / git log / git diff" questions
// without any capability to mutate the repository. Safety is enforced by a
// closed allowlist of read-only subcommands ({status, log, diff, branch,
// show}) with fixed, non-mutating flags — there is NO arbitrary git or shell
// execution path. A subcommand outside the allowlist is rejected before any
// process is spawned (§11.4.133 target-safety: a read-only tool can never write).
//
// The tool is constructed and registered explicitly by the caller (e.g. the
// TUI); it is intentionally NOT auto-registered into a global registry.
type GitStatusTool struct {
	// repoDir is the working directory passed to `git -C <repoDir> ...`. When
	// empty it defaults to "." (the process working directory).
	repoDir string
}

// NewGitStatusTool constructs a read-only git inspection tool rooted at repoDir.
// An empty repoDir defaults to "." so the tool operates on the process working
// directory. repoDir is injected — no repository path is hardcoded.
func NewGitStatusTool(repoDir string) *GitStatusTool {
	if repoDir == "" {
		repoDir = "."
	}
	return &GitStatusTool{repoDir: repoDir}
}

// Compile-time assertion that GitStatusTool satisfies the tools.Tool contract.
var _ tools.Tool = (*GitStatusTool)(nil)

// gitReadOnlySubcommands is the closed allowlist of permitted subcommands mapped
// to their fixed, non-mutating argument vectors. NO write/mutating subcommand
// (add, commit, push, reset, rm, clean, checkout, merge, rebase, ...) appears
// here and none is reachable — this map is the entire safety surface.
var gitReadOnlySubcommands = map[string][]string{
	"status": {"status", "--short", "--branch"},
	"log":    {"log", "--oneline", "-n", "20"},
	"diff":   {"diff", "--stat"},
	"branch": {"branch", "-vv"},
	"show":   {"show", "--stat", "-n", "1"},
}

// defaultSubcommand is used when the caller supplies no "subcommand" param.
const defaultSubcommand = "status"

// Name returns the tool name.
func (t *GitStatusTool) Name() string { return "git_status" }

// Description returns a concise description. Structural/diagnostic label
// (CONST-046 exempt — not user-facing prose).
func (t *GitStatusTool) Description() string {
	return "Read-only git inspection: status, log, diff, branch, show. Never mutates the repository."
}

// RequiresApproval — pure read of repository state. LevelReadOnly bypasses the
// approval gate since the allowlist guarantees no mutation is possible.
func (t *GitStatusTool) RequiresApproval() approval.ApprovalLevel {
	return approval.LevelReadOnly
}

// Category returns the git tool category.
func (t *GitStatusTool) Category() tools.ToolCategory {
	return tools.ToolCategory("git")
}

// ParallelSafe declares the tool safe to run concurrently in a tool-call batch:
// it has no side effects (pure read of repository state).
func (t *GitStatusTool) ParallelSafe() bool { return true }

// allowedSubcommands returns the allowlist keys sorted for stable display in
// schema/errors.
func allowedSubcommands() []string {
	out := make([]string, 0, len(gitReadOnlySubcommands))
	for k := range gitReadOnlySubcommands {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Schema advertises the optional "subcommand" enum parameter.
func (t *GitStatusTool) Schema() tools.ToolSchema {
	return tools.ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"subcommand": map[string]interface{}{
				"type": "string",
				"enum": allowedSubcommands(),
				"description": fmt.Sprintf(
					"Optional read-only git subcommand (default %q). One of: %s.",
					defaultSubcommand, strings.Join(allowedSubcommands(), ", ")),
			},
		},
		Required:    []string{},
		Description: "Run a read-only git inspection subcommand against the repository.",
	}
}

// Validate checks the optional subcommand against the allowlist. A missing or
// empty subcommand is valid (defaults to status). A non-allowlisted value is
// rejected here so the gate fails fast before Execute.
func (t *GitStatusTool) Validate(params map[string]interface{}) error {
	sub, _, err := resolveSubcommand(params)
	if err != nil {
		return err
	}
	_ = sub
	return nil
}

// resolveSubcommand extracts and validates the subcommand param. Returns the
// resolved subcommand name, its argument vector, and an error when the supplied
// value is not in the allowlist.
func resolveSubcommand(params map[string]interface{}) (string, []string, error) {
	sub := defaultSubcommand
	if raw, ok := params["subcommand"]; ok {
		s, isStr := raw.(string)
		if !isStr {
			return "", nil, fmt.Errorf("git_status: subcommand must be a string")
		}
		if trimmed := strings.TrimSpace(s); trimmed != "" {
			sub = trimmed
		}
	}
	args, ok := gitReadOnlySubcommands[sub]
	if !ok {
		return "", nil, fmt.Errorf(
			"git_status: subcommand %q is not permitted; allowed read-only subcommands: %s",
			sub, strings.Join(allowedSubcommands(), ", "))
	}
	return sub, args, nil
}

// Execute runs the allowlisted read-only git subcommand and returns combined
// stdout as a string. On a non-zero exit it returns an error carrying stderr.
// There is no code path that runs an arbitrary or write subcommand.
func (t *GitStatusTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	_, subArgs, err := resolveSubcommand(params)
	if err != nil {
		return nil, err
	}

	// Build: git -C <repoDir> <fixed read-only args...>. The args come ONLY
	// from the allowlist map — never from caller-supplied free text.
	fullArgs := append([]string{"-C", t.repoDir}, subArgs...)
	cmd := exec.CommandContext(ctx, "git", fullArgs...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if runErr := cmd.Run(); runErr != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg == "" {
			errMsg = runErr.Error()
		}
		return nil, fmt.Errorf("git_status: %s failed: %s", subArgs[0], errMsg)
	}

	return stdout.String(), nil
}
