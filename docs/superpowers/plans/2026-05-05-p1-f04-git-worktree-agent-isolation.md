# P1-F04 — Git Worktree Agent Isolation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement claude-code-style git worktree isolation: agents enter named, validated worktrees at `<repoRoot>/.helix-worktrees/<name>/`, work in parallel branches, and exit back to main without polluting it. 4 agent tools + 4 Cobra subcommands + 1 `/worktree` slash command.

**Architecture:** New thin sub-package `internal/tools/worktree/` (mirrors F02/F03's pattern). Shells out to the `git` binary via `os/exec` (consistent with `internal/tools/git/`). Per-session "current worktree" state lives on `session.Manager` via a single new field. Worktree management is a workflow concern; the existing `internal/tools/git/` package (auto-commit + attribution) is left untouched.

**Tech Stack:** Go 1.26, testify v1.11, github.com/spf13/cobra v1.8, existing `internal/tools/registry`, `internal/commands/builtin`, `internal/session`. **No new dependencies.** Standard-library `os/exec`, `path/filepath`, `regexp`, `sync`, `context` only.

**Spec:** `docs/superpowers/specs/2026-05-05-p1-f04-git-worktree-agent-isolation-design.md` (commit `7ba8907`)

**Working directory for all `go` commands:** `helix_code/` (the inner Go module). Git commands run from the meta-repo root `/run/media/milosvasic/DATA4TB/Projects/helix_code/` per the F01/F02/F03 convention.

**Anti-bluff smoke (run on every commit):**
```bash
cd HelixCode && grep -rn "simulated\|for now\|TODO implement\|placeholder" internal/tools/worktree/ && echo "BLUFF FOUND" || echo "clean"
```

---

## Task 1: Bootstrap evidence + advance PROGRESS + .gitignore

**Files:**
- Modify: `docs/improvements/06_phase_1_evidence.md`
- Modify: `docs/improvements/PROGRESS.md`
- Modify: `.gitignore` (root)
- Modify: `helix_code/.gitignore` (inner module)

- [ ] **Step 1: Append F04 section header to evidence file**

Append to `docs/improvements/06_phase_1_evidence.md`:

```markdown

---

## P1-F04 — Git Worktree Agent Isolation

**Spec:** `docs/superpowers/specs/2026-05-05-p1-f04-git-worktree-agent-isolation-design.md` (commit `7ba8907`)
**Plan:** `docs/superpowers/plans/2026-05-05-p1-f04-git-worktree-agent-isolation.md`
**Started:** 2026-05-05
**Status:** active

### Task evidence trail

(filled in commit-by-commit as tasks land)
```

- [ ] **Step 2: Update PROGRESS.md current focus block**

Replace the existing "## Current focus" block with:

```markdown
## Current focus
- **Active phase:** P1 — claude-code feature porting
- **Active feature:** F04 — Git Worktree Agent Isolation
- **Active task:** P1-F04-T01 — bootstrap evidence + advance PROGRESS
- **Last completed:** P1-F03-T11 — Feature 3 (Tool Result Persistence) close-out + push
- **Owner:** agent (Claude Opus 4.7)
- **Started:** 2026-05-04
- **Last touched:** 2026-05-05
- **Blocked-on:** none
```

- [ ] **Step 3: Add F04 task list block to PROGRESS.md**

After the existing F03 task list block (all 11 items checked), insert:

```markdown
## Active feature task list (P1-F04: Git Worktree Agent Isolation)
- [ ] P1-F04-T01 — bootstrap evidence + advance PROGRESS + .gitignore
- [ ] P1-F04-T02 — internal/tools/worktree package skeleton (types + doc)
- [ ] P1-F04-T03 — git.go thin git-binary wrappers (TDD against ephemeral repo)
- [ ] P1-F04-T04 — Manager + ValidateName + GetCurrentDirectory + IsIsolated (TDD)
- [ ] P1-F04-T05 — Manager.EnterWorktree (TDD; existing/new branch + dirty rejection)
- [ ] P1-F04-T06 — Manager.ExitWorktree + ListWorktrees + RemoveWorktree (TDD)
- [ ] P1-F04-T07 — 4 tools.Tool interface implementations (TDD)
- [ ] P1-F04-T08 — session.Manager currentWorktree field + getter/setter (TDD)
- [ ] P1-F04-T09 — helixcode worktree {list,enter,exit,remove} Cobra subcommands
- [ ] P1-F04-T10 — /worktree slash command + register in builtin/register.go
- [ ] P1-F04-T11 — cmd/cli/main.go startup wiring + integration test (no mocks)
- [ ] P1-F04-T12 — Challenge with three runtime-evidence scenarios
- [ ] P1-F04-T13 — Feature 4 close-out + push
```

- [ ] **Step 4: Add `.helix-worktrees/` to root `.gitignore`**

Append to `/run/media/milosvasic/DATA4TB/Projects/helix_code/.gitignore`:

```
# Worktrees created by HelixCode P1-F04 EnterWorktree (do NOT commit)
.helix-worktrees/
```

- [ ] **Step 5: Add `.helix-worktrees/` to inner module `.gitignore`**

Append to `/run/media/milosvasic/DATA4TB/Projects/helix_code/helix_code/.gitignore`:

```
# Worktrees created by HelixCode P1-F04 EnterWorktree (do NOT commit)
.helix-worktrees/
```

- [ ] **Step 6: Commit**

```bash
git -C /run/media/milosvasic/DATA4TB/Projects/HelixCode add docs/improvements/06_phase_1_evidence.md docs/improvements/PROGRESS.md .gitignore helix_code/.gitignore
git -C /run/media/milosvasic/DATA4TB/Projects/HelixCode commit -m "$(cat <<'EOF'
docs(P1-F04-T01): bootstrap Phase 1 / Feature 4 evidence + advance PROGRESS

Adds .helix-worktrees/ to root + inner .gitignore so worktrees created by
EnterWorktree (P1-F04) never accidentally track.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: Persistence package skeleton (types + doc)

**Files:**
- Create: `helix_code/internal/tools/worktree/doc.go`
- Create: `helix_code/internal/tools/worktree/types.go`

- [ ] **Step 1: Create doc.go**

Create `helix_code/internal/tools/worktree/doc.go`:

```go
// Package worktree implements claude-code-style git worktree agent isolation.
//
// Agents (and humans) enter a named, validated worktree at
// <repoRoot>/.helix-worktrees/<name>/ via Manager.EnterWorktree, work in a
// parallel branch without polluting main, and exit via Manager.ExitWorktree.
// All worktree operations shell out to the git binary, consistent with
// internal/tools/git/. Submodules are NOT initialised — the meta-repo and
// the inner Go module at helix_code/ are present, but submodule directories
// under helix_agent/, Dependencies/, etc. are empty placeholders.
//
// See: docs/superpowers/specs/2026-05-05-p1-f04-git-worktree-agent-isolation-design.md
package worktree
```

- [ ] **Step 2: Create types.go**

Create `helix_code/internal/tools/worktree/types.go`:

```go
package worktree

import "regexp"

// Constants control validation, on-disk location, and invariants.
const (
	// WorktreeNameRegex constrains worktree names to alphanumerics + . _ - .
	// Matches claude-code's own validation pattern.
	WorktreeNameRegex = `^[a-zA-Z0-9._-]+$`

	// WorktreeNameMaxLength is the maximum allowed name length in bytes.
	WorktreeNameMaxLength = 64

	// WorktreeDir is the relative path under repoRoot for worktree checkouts.
	WorktreeDir = ".helix-worktrees"
)

// worktreeNamePattern is the compiled regex used by ValidateName.
var worktreeNamePattern = regexp.MustCompile(WorktreeNameRegex)

// Worktree describes a single isolated checkout.
//
// Path is the absolute path to the worktree on disk (under
// <repoRoot>/.helix-worktrees/<name>). Branch is best-effort and may be
// empty if the worktree has a detached HEAD.
type Worktree struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Branch string `json:"branch,omitempty"`
}
```

- [ ] **Step 3: Verify the package compiles**

```bash
cd /run/media/milosvasic/DATA4TB/Projects/helix_code/HelixCode && go build ./internal/tools/worktree/...
```
Expected: clean compile, exit 0.

- [ ] **Step 4: Anti-bluff smoke**

```bash
cd /run/media/milosvasic/DATA4TB/Projects/helix_code/HelixCode && grep -rn "simulated\|for now\|TODO implement\|placeholder" internal/tools/worktree/ && echo "BLUFF FOUND" || echo "clean"
```
Expected: `clean`.

- [ ] **Step 5: Commit**

```bash
git -C /run/media/milosvasic/DATA4TB/Projects/HelixCode add helix_code/internal/tools/worktree/doc.go helix_code/internal/tools/worktree/types.go
git -C /run/media/milosvasic/DATA4TB/Projects/HelixCode commit -m "$(cat <<'EOF'
feat(P1-F04-T02): add internal/tools/worktree package skeleton

Doc.go declares package purpose. Types.go defines the Worktree struct
and constants (WorktreeNameRegex, WorktreeNameMaxLength = 64,
WorktreeDir = ".helix-worktrees"). Compiles clean; anti-bluff clean.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: git.go thin git-binary wrappers (TDD against ephemeral repo)

**Files:**
- Create: `helix_code/internal/tools/worktree/git.go`
- Create: `helix_code/internal/tools/worktree/git_test.go`

- [ ] **Step 1: Write failing test**

Create `helix_code/internal/tools/worktree/git_test.go`:

```go
package worktree

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// initEphemeralRepo creates a real temporary git repo with one seed commit
// on `main`. Returns the absolute path. Test fails if `git` is not on PATH
// or any setup step fails.
func initEphemeralRepo(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = tmp
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %s: %s", strings.Join(args, " "), string(out))
	}
	run("init", "-b", "main")
	run("config", "user.email", "test@helixcode.dev")
	run("config", "user.name", "Test")
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "README.md"), []byte("seed\n"), 0o644))
	run("add", ".")
	run("commit", "-m", "seed")
	return tmp
}

func TestGitRevParseToplevel_Resolves(t *testing.T) {
	repo := initEphemeralRepo(t)
	got, err := gitRevParseToplevel(context.Background(), repo)
	require.NoError(t, err)
	// On macOS the temp path may resolve through /private/var; tolerate that.
	assert.True(t, strings.HasSuffix(got, filepath.Base(repo)),
		"expected toplevel ending with %q, got %q", filepath.Base(repo), got)
}

func TestGitRevParseToplevel_NotARepo(t *testing.T) {
	tmp := t.TempDir()
	_, err := gitRevParseToplevel(context.Background(), tmp)
	assert.Error(t, err, "non-repo dir must error")
}

func TestGitWorktreeAdd_NewBranch(t *testing.T) {
	repo := initEphemeralRepo(t)
	wtPath := filepath.Join(repo, WorktreeDir, "feature-x")
	require.NoError(t, os.MkdirAll(filepath.Dir(wtPath), 0o755))

	out, err := gitWorktreeAddNewBranch(context.Background(), repo, "feature-x", wtPath)
	require.NoError(t, err, "output: %s", out)

	// Worktree is on disk
	info, err := os.Stat(wtPath)
	require.NoError(t, err)
	assert.True(t, info.IsDir())

	// Worktree's seed file is the same as main's
	body, err := os.ReadFile(filepath.Join(wtPath, "README.md"))
	require.NoError(t, err)
	assert.Equal(t, "seed\n", string(body))
}

func TestGitWorktreeAdd_ExistingBranchFails(t *testing.T) {
	// gitWorktreeAdd attaches an existing branch; if the branch doesn't exist,
	// git refuses with a clear error. This is the path the Manager uses to
	// decide between "existing branch" and "create new branch with -b".
	repo := initEphemeralRepo(t)
	wtPath := filepath.Join(repo, WorktreeDir, "non-existent")
	require.NoError(t, os.MkdirAll(filepath.Dir(wtPath), 0o755))

	out, err := gitWorktreeAdd(context.Background(), repo, "non-existent-branch", wtPath)
	assert.Error(t, err)
	assert.Contains(t, string(out), "invalid reference",
		"git's error must mention the missing branch; output: %s", string(out))
}

func TestGitWorktreeList_AfterAdd(t *testing.T) {
	repo := initEphemeralRepo(t)
	wtPath := filepath.Join(repo, WorktreeDir, "feature-y")
	require.NoError(t, os.MkdirAll(filepath.Dir(wtPath), 0o755))
	_, err := gitWorktreeAddNewBranch(context.Background(), repo, "feature-y", wtPath)
	require.NoError(t, err)

	out, err := gitWorktreeList(context.Background(), repo)
	require.NoError(t, err)
	assert.Contains(t, string(out), wtPath)
	assert.Contains(t, string(out), "feature-y")
}

func TestGitStatusPorcelain_CleanThenDirty(t *testing.T) {
	repo := initEphemeralRepo(t)

	// Clean repo: empty output
	out, err := gitStatusPorcelain(context.Background(), repo)
	require.NoError(t, err)
	assert.Empty(t, strings.TrimSpace(string(out)))

	// Dirty repo: untracked file shows up
	require.NoError(t, os.WriteFile(filepath.Join(repo, "new.txt"), []byte("x"), 0o644))
	out, err = gitStatusPorcelain(context.Background(), repo)
	require.NoError(t, err)
	assert.NotEmpty(t, strings.TrimSpace(string(out)))
}

func TestGitWorktreeRemove_Roundtrip(t *testing.T) {
	repo := initEphemeralRepo(t)
	wtPath := filepath.Join(repo, WorktreeDir, "feature-z")
	require.NoError(t, os.MkdirAll(filepath.Dir(wtPath), 0o755))
	_, err := gitWorktreeAddNewBranch(context.Background(), repo, "feature-z", wtPath)
	require.NoError(t, err)

	out, err := gitWorktreeRemove(context.Background(), repo, wtPath, false)
	require.NoError(t, err, "output: %s", out)

	_, statErr := os.Stat(wtPath)
	assert.True(t, os.IsNotExist(statErr), "worktree dir must be removed")
}
```

- [ ] **Step 2: Run failing test**

```bash
cd /run/media/milosvasic/DATA4TB/Projects/helix_code/HelixCode && go test -count=1 -run 'TestGit' ./internal/tools/worktree/
```
Expected: FAIL — git wrapper functions undefined.

- [ ] **Step 3: Implement git.go**

Create `helix_code/internal/tools/worktree/git.go`:

```go
package worktree

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// gitRevParseToplevel returns the absolute path of the git repository
// containing cwd. Errors if cwd is not inside a git repo.
func gitRevParseToplevel(ctx context.Context, cwd string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--show-toplevel")
	cmd.Dir = cwd
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git rev-parse --show-toplevel: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

// gitWorktreeAdd attaches an existing branch to a new worktree at path.
// Returns the combined git output and any error. The output is preserved
// even on failure so the caller can decide whether to retry with -b.
func gitWorktreeAdd(ctx context.Context, repoRoot, branch, path string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", "worktree", "add", path, branch)
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("git worktree add %s %s: %w", path, branch, err)
	}
	return out, nil
}

// gitWorktreeAddNewBranch creates a new branch and a worktree in one shot
// (git worktree add -b <branch> <path>). Used as the fallback when the
// branch doesn't already exist.
func gitWorktreeAddNewBranch(ctx context.Context, repoRoot, branch, path string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", "worktree", "add", "-b", branch, path)
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("git worktree add -b %s %s: %w", branch, path, err)
	}
	return out, nil
}

// gitWorktreeRemove removes the worktree at path. If force is true, passes
// the -f flag to allow removal of dirty / locked worktrees.
func gitWorktreeRemove(ctx context.Context, repoRoot, path string, force bool) ([]byte, error) {
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "-f")
	}
	args = append(args, path)
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("git worktree remove (force=%v) %s: %w", force, path, err)
	}
	return out, nil
}

// gitWorktreeList runs `git worktree list --porcelain` and returns the raw
// output. The Manager parses this when populating ListWorktrees.
func gitWorktreeList(ctx context.Context, repoRoot string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", "worktree", "list", "--porcelain")
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("git worktree list: %w", err)
	}
	return out, nil
}

// gitStatusPorcelain runs `git status --porcelain` in dir and returns the raw
// output. Empty output means clean.
func gitStatusPorcelain(ctx context.Context, dir string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", "status", "--porcelain")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("git status --porcelain: %w", err)
	}
	return out, nil
}
```

- [ ] **Step 4: Run tests**

```bash
cd /run/media/milosvasic/DATA4TB/Projects/helix_code/HelixCode && go test -count=1 -race -v -run 'TestGit' ./internal/tools/worktree/
```
Expected: PASS — 7 tests against real ephemeral repos.

- [ ] **Step 5: Anti-bluff smoke**

```bash
cd /run/media/milosvasic/DATA4TB/Projects/helix_code/HelixCode && grep -rn "simulated\|for now\|TODO implement\|placeholder" internal/tools/worktree/ && echo "BLUFF FOUND" || echo "clean"
```
Expected: `clean`.

- [ ] **Step 6: Commit**

```bash
git -C /run/media/milosvasic/DATA4TB/Projects/HelixCode add helix_code/internal/tools/worktree/git.go helix_code/internal/tools/worktree/git_test.go
git -C /run/media/milosvasic/DATA4TB/Projects/HelixCode commit -m "$(cat <<'EOF'
feat(P1-F04-T03): git binary wrappers for worktree operations (TDD)

Six exec.CommandContext wrappers in git.go: gitRevParseToplevel,
gitWorktreeAdd (existing branch), gitWorktreeAddNewBranch (creates -b),
gitWorktreeRemove (with force flag), gitWorktreeList (--porcelain),
gitStatusPorcelain. Each preserves combined output on error so callers
can surface verbatim git diagnostics. 7 unit tests run against real
ephemeral repos via t.TempDir() + git init.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: Manager + ValidateName + GetCurrentDirectory + IsIsolated (TDD)

**Files:**
- Create: `helix_code/internal/tools/worktree/manager.go`
- Create: `helix_code/internal/tools/worktree/manager_validate_test.go`

- [ ] **Step 1: Write failing test**

Create `helix_code/internal/tools/worktree/manager_validate_test.go`:

```go
package worktree

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateName_Valid(t *testing.T) {
	m := NewManager("/tmp/repo")
	for _, name := range []string{"feature-x", "_pre-release", "v1.2.3-rc1", "a", "a.b.c"} {
		assert.NoError(t, m.ValidateName(name), "expected %q to be valid", name)
	}
}

func TestValidateName_Empty(t *testing.T) {
	m := NewManager("/tmp/repo")
	err := m.ValidateName("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

func TestValidateName_TooLong(t *testing.T) {
	m := NewManager("/tmp/repo")
	tooLong := strings.Repeat("a", WorktreeNameMaxLength+1)
	err := m.ValidateName(tooLong)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds")
}

func TestValidateName_AtLengthLimit(t *testing.T) {
	m := NewManager("/tmp/repo")
	exact := strings.Repeat("a", WorktreeNameMaxLength)
	assert.NoError(t, m.ValidateName(exact),
		"name of length WorktreeNameMaxLength (boundary) must be valid")
}

func TestValidateName_InvalidChars(t *testing.T) {
	m := NewManager("/tmp/repo")
	bad := []string{"/", "..", "../etc", "with space", "feature/x", "with\ttab", "with\nnewline", "tildé"}
	for _, name := range bad {
		err := m.ValidateName(name)
		assert.Error(t, err, "expected %q to be rejected", name)
	}
}

func TestGetCurrentDirectory_DefaultIsRepoRoot(t *testing.T) {
	m := NewManager("/tmp/repo")
	assert.Equal(t, "/tmp/repo", m.GetCurrentDirectory())
}

func TestIsIsolated_DefaultIsFalse(t *testing.T) {
	m := NewManager("/tmp/repo")
	assert.False(t, m.IsIsolated())
}
```

- [ ] **Step 2: Run failing test**

```bash
cd /run/media/milosvasic/DATA4TB/Projects/helix_code/HelixCode && go test -count=1 -run 'TestValidateName|TestGetCurrentDirectory_DefaultIsRepoRoot|TestIsIsolated_DefaultIsFalse' ./internal/tools/worktree/
```
Expected: FAIL — `Manager`, `NewManager`, `ValidateName`, `GetCurrentDirectory`, `IsIsolated` undefined.

- [ ] **Step 3: Implement manager.go (skeleton + 3 methods)**

Create `helix_code/internal/tools/worktree/manager.go`:

```go
package worktree

import (
	"fmt"
	"sync"
)

// Manager owns worktree state for a repository.
//
// repoRoot is the absolute path to the main worktree (git rev-parse
// --show-toplevel). currentWorktree is the absolute path of the active
// isolated worktree, or "" when the agent is in the main worktree.
type Manager struct {
	repoRoot        string
	currentWorktree string
	mu              sync.RWMutex
}

// NewManager creates a Manager rooted at repoRoot. Performs no I/O.
func NewManager(repoRoot string) *Manager {
	return &Manager{repoRoot: repoRoot}
}

// ValidateName rejects empty / too-long / non-conforming worktree names.
// The pattern matches claude-code's own validation (`^[a-zA-Z0-9._-]+$`).
func (m *Manager) ValidateName(name string) error {
	if name == "" {
		return fmt.Errorf("worktree name cannot be empty")
	}
	if len(name) > WorktreeNameMaxLength {
		return fmt.Errorf("worktree name exceeds %d characters", WorktreeNameMaxLength)
	}
	if !worktreeNamePattern.MatchString(name) {
		return fmt.Errorf("worktree name %q does not match pattern %s", name, WorktreeNameRegex)
	}
	return nil
}

// GetCurrentDirectory returns the effective working directory: the active
// worktree's path if isolated, otherwise the main repo root.
func (m *Manager) GetCurrentDirectory() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.currentWorktree != "" {
		return m.currentWorktree
	}
	return m.repoRoot
}

// IsIsolated reports whether the Manager is currently inside a worktree
// (set by EnterWorktree, cleared by ExitWorktree).
func (m *Manager) IsIsolated() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentWorktree != ""
}
```

- [ ] **Step 4: Run tests**

```bash
cd /run/media/milosvasic/DATA4TB/Projects/helix_code/HelixCode && go test -count=1 -race -v -run 'TestValidateName|TestGetCurrentDirectory_DefaultIsRepoRoot|TestIsIsolated_DefaultIsFalse' ./internal/tools/worktree/
```
Expected: PASS — 7 tests.

- [ ] **Step 5: Run full package**

```bash
cd /run/media/milosvasic/DATA4TB/Projects/helix_code/HelixCode && go test -count=1 -race ./internal/tools/worktree/...
```
Expected: PASS — 7 (T03) + 7 (T04) = 14 tests.

- [ ] **Step 6: Anti-bluff smoke**

```bash
cd /run/media/milosvasic/DATA4TB/Projects/helix_code/HelixCode && grep -rn "simulated\|for now\|TODO implement\|placeholder" internal/tools/worktree/ && echo "BLUFF FOUND" || echo "clean"
```
Expected: `clean`.

- [ ] **Step 7: Commit**

```bash
git -C /run/media/milosvasic/DATA4TB/Projects/HelixCode add helix_code/internal/tools/worktree/manager.go helix_code/internal/tools/worktree/manager_validate_test.go
git -C /run/media/milosvasic/DATA4TB/Projects/HelixCode commit -m "$(cat <<'EOF'
feat(P1-F04-T04): Manager + ValidateName + GetCurrentDirectory + IsIsolated (TDD)

Manager struct with repoRoot + currentWorktree fields and an RWMutex.
NewManager performs no I/O. ValidateName enforces empty / max-length /
regex constraints (claude-code's `^[a-zA-Z0-9._-]+$` pattern).
GetCurrentDirectory returns currentWorktree or repoRoot. IsIsolated
reports current state. 7 unit tests covering valid names, empty,
too-long, exact-length boundary, invalid chars, and default state.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 5: Manager.EnterWorktree (TDD)

**Files:**
- Modify: `helix_code/internal/tools/worktree/manager.go` (add `EnterWorktree`)
- Create: `helix_code/internal/tools/worktree/manager_enter_test.go`

- [ ] **Step 1: Write failing test**

Create `helix_code/internal/tools/worktree/manager_enter_test.go`:

```go
package worktree

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnterWorktree_NewBranchPath(t *testing.T) {
	repo := initEphemeralRepo(t)
	m := NewManager(repo)

	path, err := m.EnterWorktree(context.Background(), "feature-x", "")
	require.NoError(t, err)
	assert.True(t, filepath.IsAbs(path))
	assert.Equal(t, filepath.Join(repo, WorktreeDir, "feature-x"), path)

	// Worktree dir exists and contains the seed file
	body, err := os.ReadFile(filepath.Join(path, "README.md"))
	require.NoError(t, err)
	assert.Equal(t, "seed\n", string(body))

	// Manager state updated
	assert.True(t, m.IsIsolated())
	assert.Equal(t, path, m.GetCurrentDirectory())
}

func TestEnterWorktree_ExistingBranchPath(t *testing.T) {
	repo := initEphemeralRepo(t)

	// Create a branch ahead of EnterWorktree
	cmd := exec.Command("git", "branch", "release-1.0")
	cmd.Dir = repo
	require.NoError(t, cmd.Run())

	m := NewManager(repo)
	path, err := m.EnterWorktree(context.Background(), "release-1.0", "")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(repo, WorktreeDir, "release-1.0"), path)
	assert.True(t, m.IsIsolated())
}

func TestEnterWorktree_DirtyExistingDirRejected(t *testing.T) {
	repo := initEphemeralRepo(t)
	m := NewManager(repo)

	// First entry creates the worktree
	path, err := m.EnterWorktree(context.Background(), "feature-y", "")
	require.NoError(t, err)

	// Dirty the worktree
	require.NoError(t, os.WriteFile(filepath.Join(path, "uncommitted.txt"), []byte("dirty"), 0o644))

	// Second entry must reject
	_, err = m.EnterWorktree(context.Background(), "feature-y", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "uncommitted changes")
}

func TestEnterWorktree_CleanExistingDirReuses(t *testing.T) {
	repo := initEphemeralRepo(t)
	m := NewManager(repo)

	first, err := m.EnterWorktree(context.Background(), "feature-z", "")
	require.NoError(t, err)

	// Re-entry into the same worktree (still clean)
	second, err := m.EnterWorktree(context.Background(), "feature-z", "")
	require.NoError(t, err)
	assert.Equal(t, first, second, "re-entry returns same path")
}

func TestEnterWorktree_InvalidNameRejected(t *testing.T) {
	repo := initEphemeralRepo(t)
	m := NewManager(repo)

	_, err := m.EnterWorktree(context.Background(), "../etc", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "match pattern")
}

func TestEnterWorktree_NotARepoFails(t *testing.T) {
	tmp := t.TempDir() // not a git repo
	m := NewManager(tmp)

	_, err := m.EnterWorktree(context.Background(), "feature-w", "")
	require.Error(t, err)
}

func TestEnterWorktree_BaseBranchOverridesName(t *testing.T) {
	repo := initEphemeralRepo(t)

	// Create a base branch with extra commits
	cmd := exec.Command("git", "branch", "stable")
	cmd.Dir = repo
	require.NoError(t, cmd.Run())

	m := NewManager(repo)
	// name=feature-from-stable, baseBranch=stable → branch should be 'stable'
	path, err := m.EnterWorktree(context.Background(), "feature-from-stable", "stable")
	require.NoError(t, err)

	// Verify the worktree is on the 'stable' branch
	out, err := exec.Command("git", "-C", path, "branch", "--show-current").Output()
	require.NoError(t, err)
	assert.Contains(t, string(out), "stable")
}
```

- [ ] **Step 2: Run failing test**

```bash
cd /run/media/milosvasic/DATA4TB/Projects/helix_code/HelixCode && go test -count=1 -run 'TestEnterWorktree' ./internal/tools/worktree/
```
Expected: FAIL — `EnterWorktree` undefined.

- [ ] **Step 3: Add EnterWorktree to manager.go**

Append to `helix_code/internal/tools/worktree/manager.go`:

```go
import (
	// existing imports...
	"context"
	"os"
	"path/filepath"
	"strings"
)
```

(Adjust the existing import block to merge these in. The file currently imports only `fmt` and `sync`; expand to include `context`, `os`, `path/filepath`, `strings`.)

Then append the method:

```go
// EnterWorktree switches into a named worktree, creating it if necessary.
// If baseBranch is empty, the worktree's branch defaults to name.
//
// Behaviour:
//   - Validates name via ValidateName.
//   - If <repoRoot>/.helix-worktrees/<name>/ exists:
//       - Verifies clean (git status --porcelain empty); rejects with error
//         if dirty.
//       - Updates currentWorktree and returns the path.
//   - Otherwise:
//       - Creates the parent dir.
//       - Tries `git worktree add <path> <branch>` (existing branch).
//       - On failure, falls back to `git worktree add -b <branch> <path>`
//         (new branch).
//       - On second failure, returns a composite error with both outputs.
func (m *Manager) EnterWorktree(ctx context.Context, name, baseBranch string) (string, error) {
	if err := m.ValidateName(name); err != nil {
		return "", err
	}

	branch := baseBranch
	if branch == "" {
		branch = name
	}

	path := filepath.Join(m.repoRoot, WorktreeDir, name)

	m.mu.Lock()
	defer m.mu.Unlock()

	// Reuse existing worktree if present and clean.
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		out, sErr := gitStatusPorcelain(ctx, path)
		if sErr != nil {
			return "", fmt.Errorf("checking worktree status: %w", sErr)
		}
		if strings.TrimSpace(string(out)) != "" {
			return "", fmt.Errorf("worktree %q has uncommitted changes — clean or remove first", name)
		}
		m.currentWorktree = path
		return path, nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("creating worktree parent dir: %w", err)
	}

	// Try existing branch first.
	if out, err := gitWorktreeAdd(ctx, m.repoRoot, branch, path); err != nil {
		// Fall back to creating a new branch.
		if out2, err2 := gitWorktreeAddNewBranch(ctx, m.repoRoot, branch, path); err2 != nil {
			return "", fmt.Errorf(
				"creating worktree (existing-branch attempt: %s) (new-branch attempt: %s): %w",
				strings.TrimSpace(string(out)), strings.TrimSpace(string(out2)), err2,
			)
		}
	}

	m.currentWorktree = path
	return path, nil
}
```

- [ ] **Step 4: Run tests**

```bash
cd /run/media/milosvasic/DATA4TB/Projects/helix_code/HelixCode && go test -count=1 -race -v -run 'TestEnterWorktree' ./internal/tools/worktree/
```
Expected: PASS — 7 tests.

- [ ] **Step 5: Run full package**

```bash
cd /run/media/milosvasic/DATA4TB/Projects/helix_code/HelixCode && go test -count=1 -race ./internal/tools/worktree/...
```
Expected: PASS — 7 + 7 + 7 = 21 tests.

- [ ] **Step 6: Anti-bluff smoke**

```bash
cd /run/media/milosvasic/DATA4TB/Projects/helix_code/HelixCode && grep -rn "simulated\|for now\|TODO implement\|placeholder" internal/tools/worktree/ && echo "BLUFF FOUND" || echo "clean"
```
Expected: `clean`.

- [ ] **Step 7: Commit**

```bash
git -C /run/media/milosvasic/DATA4TB/Projects/HelixCode add helix_code/internal/tools/worktree/manager.go helix_code/internal/tools/worktree/manager_enter_test.go
git -C /run/media/milosvasic/DATA4TB/Projects/HelixCode commit -m "$(cat <<'EOF'
feat(P1-F04-T05): Manager.EnterWorktree (TDD)

EnterWorktree validates name, then either reuses an existing clean
worktree or creates a new one (tries existing branch first, falls back
to -b new-branch). Dirty pre-existing worktrees are rejected. baseBranch
override lets callers create a worktree on a different branch than the
worktree name. 7 unit tests against real ephemeral repos.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 6: Manager.ExitWorktree + ListWorktrees + RemoveWorktree (TDD)

**Files:**
- Modify: `helix_code/internal/tools/worktree/manager.go`
- Create: `helix_code/internal/tools/worktree/manager_lifecycle_test.go`

- [ ] **Step 1: Write failing test**

Create `helix_code/internal/tools/worktree/manager_lifecycle_test.go`:

```go
package worktree

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExitWorktree_ResetsState(t *testing.T) {
	repo := initEphemeralRepo(t)
	m := NewManager(repo)

	_, err := m.EnterWorktree(context.Background(), "feature-a", "")
	require.NoError(t, err)
	require.True(t, m.IsIsolated())

	m.ExitWorktree()
	assert.False(t, m.IsIsolated())
	assert.Equal(t, repo, m.GetCurrentDirectory())
}

func TestListWorktrees_EmptyRepoReturnsNil(t *testing.T) {
	repo := initEphemeralRepo(t)
	m := NewManager(repo)

	wts, err := m.ListWorktrees(context.Background())
	require.NoError(t, err)
	assert.Empty(t, wts, "no worktrees yet")
}

func TestListWorktrees_AfterEnter(t *testing.T) {
	repo := initEphemeralRepo(t)
	m := NewManager(repo)

	_, err := m.EnterWorktree(context.Background(), "feature-b", "")
	require.NoError(t, err)
	_, err = m.EnterWorktree(context.Background(), "feature-c", "")
	require.NoError(t, err)

	wts, err := m.ListWorktrees(context.Background())
	require.NoError(t, err)

	names := []string{}
	for _, w := range wts {
		names = append(names, w.Name)
	}
	assert.Contains(t, names, "feature-b")
	assert.Contains(t, names, "feature-c")
}

func TestListWorktrees_IgnoresFilesInDir(t *testing.T) {
	repo := initEphemeralRepo(t)
	m := NewManager(repo)

	require.NoError(t, os.MkdirAll(filepath.Join(repo, WorktreeDir), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(repo, WorktreeDir, "stray.txt"), []byte("x"), 0o644))

	wts, err := m.ListWorktrees(context.Background())
	require.NoError(t, err)
	for _, w := range wts {
		assert.NotEqual(t, "stray.txt", w.Name, "files in WorktreeDir must be ignored")
	}
}

func TestRemoveWorktree_DeletesDirAndBranch(t *testing.T) {
	repo := initEphemeralRepo(t)
	m := NewManager(repo)

	path, err := m.EnterWorktree(context.Background(), "feature-d", "")
	require.NoError(t, err)
	m.ExitWorktree()

	require.NoError(t, m.RemoveWorktree(context.Background(), "feature-d"))

	_, statErr := os.Stat(path)
	assert.True(t, os.IsNotExist(statErr), "worktree dir must be removed")
}

func TestRemoveWorktree_RefusesCurrent(t *testing.T) {
	repo := initEphemeralRepo(t)
	m := NewManager(repo)

	_, err := m.EnterWorktree(context.Background(), "feature-e", "")
	require.NoError(t, err)

	err = m.RemoveWorktree(context.Background(), "feature-e")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "current worktree")
}

func TestRemoveWorktree_InvalidNameRejected(t *testing.T) {
	repo := initEphemeralRepo(t)
	m := NewManager(repo)

	err := m.RemoveWorktree(context.Background(), "../etc")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "match pattern")
}
```

- [ ] **Step 2: Run failing test**

```bash
cd /run/media/milosvasic/DATA4TB/Projects/helix_code/HelixCode && go test -count=1 -run 'TestExitWorktree|TestListWorktrees|TestRemoveWorktree' ./internal/tools/worktree/
```
Expected: FAIL — methods undefined.

- [ ] **Step 3: Add methods to manager.go**

Append to `helix_code/internal/tools/worktree/manager.go`:

```go
// ExitWorktree clears the active-worktree state and returns the agent to
// the main worktree. Idempotent: calling on a non-isolated Manager is a
// no-op.
func (m *Manager) ExitWorktree() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.currentWorktree = ""
}

// ListWorktrees returns all helix-managed worktrees (the directory entries
// under <repoRoot>/.helix-worktrees/). Files in the WorktreeDir are
// ignored — only subdirectories count.
//
// The Branch field is best-effort: it parses `git worktree list --porcelain`
// output to associate paths with branches. If parsing fails for any entry,
// Branch is left empty for that entry.
func (m *Manager) ListWorktrees(ctx context.Context) ([]Worktree, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	dir := filepath.Join(m.repoRoot, WorktreeDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading %s: %w", dir, err)
	}

	branchByPath := parseWorktreeBranches(ctx, m.repoRoot)

	var out []Worktree
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		full := filepath.Join(dir, entry.Name())
		out = append(out, Worktree{
			Name:   entry.Name(),
			Path:   full,
			Branch: branchByPath[full],
		})
	}
	return out, nil
}

// parseWorktreeBranches returns a map from worktree absolute path to its
// current branch name, derived from `git worktree list --porcelain`. On
// any parse error, returns whatever was successfully parsed (best-effort).
func parseWorktreeBranches(ctx context.Context, repoRoot string) map[string]string {
	out, err := gitWorktreeList(ctx, repoRoot)
	if err != nil {
		return nil
	}
	branches := map[string]string{}
	var curPath, curBranch string
	for _, line := range strings.Split(string(out), "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			if curPath != "" {
				branches[curPath] = curBranch
			}
			curPath = strings.TrimPrefix(line, "worktree ")
			curBranch = ""
		case strings.HasPrefix(line, "branch "):
			curBranch = strings.TrimPrefix(strings.TrimPrefix(line, "branch "), "refs/heads/")
		}
	}
	if curPath != "" {
		branches[curPath] = curBranch
	}
	return branches
}

// RemoveWorktree deletes the worktree directory and unregisters its git
// metadata. Refuses to remove the currently-active worktree (call
// ExitWorktree first). On `git worktree remove` failure, retries with -f
// before returning a composite error.
func (m *Manager) RemoveWorktree(ctx context.Context, name string) error {
	if err := m.ValidateName(name); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	path := filepath.Join(m.repoRoot, WorktreeDir, name)
	if path == m.currentWorktree {
		return fmt.Errorf("cannot remove the current worktree; ExitWorktree first")
	}

	if out, err := gitWorktreeRemove(ctx, m.repoRoot, path, false); err != nil {
		// Retry with -f.
		if out2, err2 := gitWorktreeRemove(ctx, m.repoRoot, path, true); err2 != nil {
			return fmt.Errorf(
				"removing worktree (without -f: %s) (with -f: %s): %w",
				strings.TrimSpace(string(out)), strings.TrimSpace(string(out2)), err2,
			)
		}
	}
	return nil
}
```

- [ ] **Step 4: Run tests**

```bash
cd /run/media/milosvasic/DATA4TB/Projects/helix_code/HelixCode && go test -count=1 -race -v -run 'TestExitWorktree|TestListWorktrees|TestRemoveWorktree' ./internal/tools/worktree/
```
Expected: PASS — 7 tests.

- [ ] **Step 5: Run full package**

```bash
cd /run/media/milosvasic/DATA4TB/Projects/helix_code/HelixCode && go test -count=1 -race ./internal/tools/worktree/...
```
Expected: PASS — 21 + 7 = 28 tests.

- [ ] **Step 6: Anti-bluff smoke**

```bash
cd /run/media/milosvasic/DATA4TB/Projects/helix_code/HelixCode && grep -rn "simulated\|for now\|TODO implement\|placeholder" internal/tools/worktree/ && echo "BLUFF FOUND" || echo "clean"
```
Expected: `clean`.

- [ ] **Step 7: Commit**

```bash
git -C /run/media/milosvasic/DATA4TB/Projects/HelixCode add helix_code/internal/tools/worktree/manager.go helix_code/internal/tools/worktree/manager_lifecycle_test.go
git -C /run/media/milosvasic/DATA4TB/Projects/HelixCode commit -m "$(cat <<'EOF'
feat(P1-F04-T06): Manager.ExitWorktree + ListWorktrees + RemoveWorktree (TDD)

ExitWorktree resets currentWorktree (idempotent). ListWorktrees walks
WorktreeDir and ignores files (only directories counted); branch names
are populated best-effort by parsing `git worktree list --porcelain`.
RemoveWorktree refuses to remove the currently-active worktree, retries
with -f on initial failure, and surfaces composite error with both git
outputs. 7 unit tests against real ephemeral repos.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 7: 4 tools.Tool interface implementations (TDD)

**Files:**
- Create: `helix_code/internal/tools/worktree/tools.go`
- Create: `helix_code/internal/tools/worktree/tools_test.go`

- [ ] **Step 1: Write failing test**

Create `helix_code/internal/tools/worktree/tools_test.go`:

```go
package worktree

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnterWorktreeTool_NameDescriptionCategory(t *testing.T) {
	repo := initEphemeralRepo(t)
	m := NewManager(repo)
	tool := NewEnterWorktreeTool(m)

	assert.Equal(t, "EnterWorktree", tool.Name())
	assert.NotEmpty(t, tool.Description())
	assert.Contains(t, tool.Description(), "Submodules are NOT initialised",
		"description must teach the LLM about the submodule omission")
	assert.NotEmpty(t, tool.Category())
}

func TestEnterWorktreeTool_Schema(t *testing.T) {
	repo := initEphemeralRepo(t)
	m := NewManager(repo)
	tool := NewEnterWorktreeTool(m)

	schema := tool.Schema()
	assert.Equal(t, "object", schema.Type)
	props, ok := schema.Properties["name"]
	require.True(t, ok)
	assert.NotNil(t, props)
	assert.Contains(t, schema.Required, "name")
	// baseBranch is optional
	assert.NotContains(t, schema.Required, "baseBranch")
}

func TestEnterWorktreeTool_Validate(t *testing.T) {
	repo := initEphemeralRepo(t)
	m := NewManager(repo)
	tool := NewEnterWorktreeTool(m)

	assert.NoError(t, tool.Validate(map[string]interface{}{"name": "feature-x"}))
	assert.Error(t, tool.Validate(map[string]interface{}{}), "missing name must error")
	assert.Error(t, tool.Validate(map[string]interface{}{"name": 42}), "wrong type must error")
}

func TestEnterWorktreeTool_Execute(t *testing.T) {
	repo := initEphemeralRepo(t)
	m := NewManager(repo)
	tool := NewEnterWorktreeTool(m)

	res, err := tool.Execute(context.Background(), map[string]interface{}{"name": "feature-y"})
	require.NoError(t, err)
	resMap, ok := res.(map[string]interface{})
	require.True(t, ok)
	path, ok := resMap["path"].(string)
	require.True(t, ok)
	assert.Equal(t, filepath.Join(repo, WorktreeDir, "feature-y"), path)
	assert.True(t, m.IsIsolated())
}

func TestExitWorktreeTool_Execute(t *testing.T) {
	repo := initEphemeralRepo(t)
	m := NewManager(repo)
	enter := NewEnterWorktreeTool(m)
	exit := NewExitWorktreeTool(m)

	_, err := enter.Execute(context.Background(), map[string]interface{}{"name": "feature-z"})
	require.NoError(t, err)
	require.True(t, m.IsIsolated())

	res, err := exit.Execute(context.Background(), map[string]interface{}{})
	require.NoError(t, err)
	resMap := res.(map[string]interface{})
	assert.Equal(t, true, resMap["exited"])
	assert.False(t, m.IsIsolated())
}

func TestListWorktreesTool_Execute(t *testing.T) {
	repo := initEphemeralRepo(t)
	m := NewManager(repo)
	enter := NewEnterWorktreeTool(m)
	list := NewListWorktreesTool(m)

	_, err := enter.Execute(context.Background(), map[string]interface{}{"name": "feature-a"})
	require.NoError(t, err)

	res, err := list.Execute(context.Background(), map[string]interface{}{})
	require.NoError(t, err)
	resMap := res.(map[string]interface{})
	wts, ok := resMap["worktrees"].([]Worktree)
	require.True(t, ok)
	require.NotEmpty(t, wts)

	names := []string{}
	for _, w := range wts {
		names = append(names, w.Name)
	}
	assert.Contains(t, names, "feature-a")
}

func TestRemoveWorktreeTool_Execute(t *testing.T) {
	repo := initEphemeralRepo(t)
	m := NewManager(repo)
	enter := NewEnterWorktreeTool(m)
	exit := NewExitWorktreeTool(m)
	remove := NewRemoveWorktreeTool(m)

	_, err := enter.Execute(context.Background(), map[string]interface{}{"name": "feature-b"})
	require.NoError(t, err)
	_, err = exit.Execute(context.Background(), map[string]interface{}{})
	require.NoError(t, err)

	res, err := remove.Execute(context.Background(), map[string]interface{}{"name": "feature-b"})
	require.NoError(t, err)
	resMap := res.(map[string]interface{})
	assert.Equal(t, true, resMap["removed"])

	_, statErr := os.Stat(filepath.Join(repo, WorktreeDir, "feature-b"))
	assert.True(t, os.IsNotExist(statErr))
}

func TestEnterWorktreeTool_DescriptionMentionsBaseBranch(t *testing.T) {
	repo := initEphemeralRepo(t)
	m := NewManager(repo)
	tool := NewEnterWorktreeTool(m)
	desc := tool.Description()
	assert.True(t, strings.Contains(desc, "branch") || strings.Contains(desc, "Branch"),
		"description must explain the optional base-branch parameter")
}
```

- [ ] **Step 2: Run failing test**

```bash
cd /run/media/milosvasic/DATA4TB/Projects/helix_code/HelixCode && go test -count=1 -run 'TestEnterWorktreeTool|TestExitWorktreeTool|TestListWorktreesTool|TestRemoveWorktreeTool' ./internal/tools/worktree/
```
Expected: FAIL — Tool constructors undefined.

- [ ] **Step 3: Implement tools.go**

Create `helix_code/internal/tools/worktree/tools.go`:

```go
package worktree

import (
	"context"
	"fmt"

	"dev.helix.code/internal/tools"
)

// ─── EnterWorktreeTool ────────────────────────────────────────────────

// EnterWorktreeTool implements the EnterWorktree agent tool.
type EnterWorktreeTool struct{ m *Manager }

// NewEnterWorktreeTool wires a Manager into an EnterWorktree tool.
func NewEnterWorktreeTool(m *Manager) *EnterWorktreeTool { return &EnterWorktreeTool{m: m} }

func (t *EnterWorktreeTool) Name() string { return "EnterWorktree" }

func (t *EnterWorktreeTool) Description() string {
	return "Enter a named git worktree for isolated development. Creates the worktree if it doesn't exist (using the worktree name as the branch name when no base-branch is supplied; otherwise uses the supplied base-branch). Submodules are NOT initialised — the meta-repo and the inner Go module at helix_code/ are present, but submodule directories under helix_agent/, Dependencies/, etc. are empty placeholders. If your work needs submodule code, run `git submodule update --init --recursive` from inside the worktree using Bash."
}

func (t *EnterWorktreeTool) Schema() tools.ToolSchema {
	return tools.ToolSchema{
		Type:        "object",
		Description: "Enter a named git worktree for isolated development.",
		Properties: map[string]interface{}{
			"name": map[string]interface{}{
				"type":        "string",
				"description": "Worktree name. Must match ^[a-zA-Z0-9._-]+$ and be ≤ 64 chars.",
			},
			"baseBranch": map[string]interface{}{
				"type":        "string",
				"description": "Optional. Existing branch to base the worktree on. Defaults to the worktree name.",
			},
		},
		Required: []string{"name"},
	}
}

func (t *EnterWorktreeTool) Category() tools.ToolCategory { return tools.CategoryShell }

func (t *EnterWorktreeTool) Validate(params map[string]interface{}) error {
	name, ok := params["name"].(string)
	if !ok {
		return fmt.Errorf("EnterWorktree: missing or non-string parameter 'name'")
	}
	if err := t.m.ValidateName(name); err != nil {
		return fmt.Errorf("EnterWorktree: %w", err)
	}
	if bb, present := params["baseBranch"]; present {
		if _, ok := bb.(string); !ok {
			return fmt.Errorf("EnterWorktree: 'baseBranch' must be a string")
		}
	}
	return nil
}

func (t *EnterWorktreeTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	if err := t.Validate(params); err != nil {
		return nil, err
	}
	name := params["name"].(string)
	baseBranch, _ := params["baseBranch"].(string)
	path, err := t.m.EnterWorktree(ctx, name, baseBranch)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"path": path}, nil
}

// ─── ExitWorktreeTool ─────────────────────────────────────────────────

type ExitWorktreeTool struct{ m *Manager }

func NewExitWorktreeTool(m *Manager) *ExitWorktreeTool { return &ExitWorktreeTool{m: m} }

func (t *ExitWorktreeTool) Name() string        { return "ExitWorktree" }
func (t *ExitWorktreeTool) Description() string { return "Return to the main worktree. No-op when not in a worktree." }

func (t *ExitWorktreeTool) Schema() tools.ToolSchema {
	return tools.ToolSchema{
		Type:        "object",
		Description: "Return to the main worktree.",
		Properties:  map[string]interface{}{},
		Required:    []string{},
	}
}

func (t *ExitWorktreeTool) Category() tools.ToolCategory       { return tools.CategoryShell }
func (t *ExitWorktreeTool) Validate(map[string]interface{}) error { return nil }

func (t *ExitWorktreeTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	t.m.ExitWorktree()
	return map[string]interface{}{"exited": true}, nil
}

// ─── ListWorktreesTool ────────────────────────────────────────────────

type ListWorktreesTool struct{ m *Manager }

func NewListWorktreesTool(m *Manager) *ListWorktreesTool { return &ListWorktreesTool{m: m} }

func (t *ListWorktreesTool) Name() string { return "ListWorktrees" }
func (t *ListWorktreesTool) Description() string {
	return "List all helix-managed git worktrees under .helix-worktrees/. Returns name, absolute path, and best-effort branch."
}

func (t *ListWorktreesTool) Schema() tools.ToolSchema {
	return tools.ToolSchema{
		Type:        "object",
		Description: "List all helix-managed worktrees.",
		Properties:  map[string]interface{}{},
		Required:    []string{},
	}
}

func (t *ListWorktreesTool) Category() tools.ToolCategory       { return tools.CategoryShell }
func (t *ListWorktreesTool) Validate(map[string]interface{}) error { return nil }

func (t *ListWorktreesTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	wts, err := t.m.ListWorktrees(ctx)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"worktrees": wts}, nil
}

// ─── RemoveWorktreeTool ───────────────────────────────────────────────

type RemoveWorktreeTool struct{ m *Manager }

func NewRemoveWorktreeTool(m *Manager) *RemoveWorktreeTool { return &RemoveWorktreeTool{m: m} }

func (t *RemoveWorktreeTool) Name() string { return "RemoveWorktree" }
func (t *RemoveWorktreeTool) Description() string {
	return "Delete a helix-managed git worktree and unregister its branch. Refuses to remove the currently-active worktree."
}

func (t *RemoveWorktreeTool) Schema() tools.ToolSchema {
	return tools.ToolSchema{
		Type:        "object",
		Description: "Remove a helix-managed worktree.",
		Properties: map[string]interface{}{
			"name": map[string]interface{}{
				"type":        "string",
				"description": "Worktree name to remove.",
			},
		},
		Required: []string{"name"},
	}
}

func (t *RemoveWorktreeTool) Category() tools.ToolCategory { return tools.CategoryShell }

func (t *RemoveWorktreeTool) Validate(params map[string]interface{}) error {
	name, ok := params["name"].(string)
	if !ok {
		return fmt.Errorf("RemoveWorktree: missing or non-string parameter 'name'")
	}
	if err := t.m.ValidateName(name); err != nil {
		return fmt.Errorf("RemoveWorktree: %w", err)
	}
	return nil
}

func (t *RemoveWorktreeTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	if err := t.Validate(params); err != nil {
		return nil, err
	}
	name := params["name"].(string)
	if err := t.m.RemoveWorktree(ctx, name); err != nil {
		return nil, err
	}
	return map[string]interface{}{"removed": true}, nil
}
```

- [ ] **Step 4: Run tests**

```bash
cd /run/media/milosvasic/DATA4TB/Projects/helix_code/HelixCode && go test -count=1 -race -v -run 'TestEnterWorktreeTool|TestExitWorktreeTool|TestListWorktreesTool|TestRemoveWorktreeTool' ./internal/tools/worktree/
```
Expected: PASS — 8 tests.

- [ ] **Step 5: Run full package**

```bash
cd /run/media/milosvasic/DATA4TB/Projects/helix_code/HelixCode && go test -count=1 -race ./internal/tools/worktree/...
```
Expected: PASS — 28 + 8 = 36 tests.

- [ ] **Step 6: Anti-bluff smoke**

```bash
cd /run/media/milosvasic/DATA4TB/Projects/helix_code/HelixCode && grep -rn "simulated\|for now\|TODO implement\|placeholder" internal/tools/worktree/ && echo "BLUFF FOUND" || echo "clean"
```
Expected: `clean`.

- [ ] **Step 7: Commit**

```bash
git -C /run/media/milosvasic/DATA4TB/Projects/HelixCode add helix_code/internal/tools/worktree/tools.go helix_code/internal/tools/worktree/tools_test.go
git -C /run/media/milosvasic/DATA4TB/Projects/HelixCode commit -m "$(cat <<'EOF'
feat(P1-F04-T07): 4 tools.Tool implementations for worktree (TDD)

EnterWorktreeTool, ExitWorktreeTool, ListWorktreesTool,
RemoveWorktreeTool — each implements tools.Tool (Name, Description,
Schema, Category=CategoryShell, Validate, Execute). Schema uses
ToolSchema struct (typed; matches registry's interface). Description
for EnterWorktreeTool teaches the LLM about the submodule-not-init
caveat. 8 unit tests cover Name/Description/Schema/Validate/Execute
for all four tools.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 8: session.Manager currentWorktree field + getter/setter (TDD)

**Files:**
- Modify: `helix_code/internal/session/manager.go`
- Create: `helix_code/internal/session/manager_worktree_test.go`

- [ ] **Step 1: Write failing test**

Create `helix_code/internal/session/manager_worktree_test.go`:

```go
package session

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestManager_GetCurrentWorktree_DefaultEmpty(t *testing.T) {
	m := NewManager()
	assert.Equal(t, "", m.GetCurrentWorktree())
}

func TestManager_SetCurrentWorktree_RoundTrip(t *testing.T) {
	m := NewManager()
	m.SetCurrentWorktree("/tmp/repo/.helix-worktrees/feature-x")
	assert.Equal(t, "/tmp/repo/.helix-worktrees/feature-x", m.GetCurrentWorktree())
}

func TestManager_SetCurrentWorktree_Empty(t *testing.T) {
	m := NewManager()
	m.SetCurrentWorktree("/tmp/repo/.helix-worktrees/feature-x")
	m.SetCurrentWorktree("")
	assert.Equal(t, "", m.GetCurrentWorktree())
}
```

- [ ] **Step 2: Run failing test**

```bash
cd /run/media/milosvasic/DATA4TB/Projects/helix_code/HelixCode && go test -count=1 -run 'TestManager_GetCurrentWorktree|TestManager_SetCurrentWorktree' ./internal/session/
```
Expected: FAIL — methods undefined.

- [ ] **Step 3: Modify manager.go**

In `helix_code/internal/session/manager.go`, find the `Manager` struct (around line 15-34) and add `currentWorktree string` as the last field BEFORE the closing brace. The existing `mu sync.RWMutex` already covers the field for thread-safety.

```go
type Manager struct {
	sessions       map[string]*Session
	activeSession  *Session
	focusManager   *focus.Manager
	hooksManager   *hooks.Manager
	mu             sync.RWMutex
	onCreate       []SessionCallback
	onStart        []SessionCallback
	onPause        []SessionCallback
	onResume       []SessionCallback
	onComplete     []SessionCallback
	onDelete       []SessionCallback
	onSwitch       []SwitchCallback
	maxHistory     int
	thrashingGuard *compression.ThrashingGuard
	currentWorktree string  // NEW: P1-F04 — active worktree path; "" = main worktree
}
```

Add the two methods at the bottom of the file:

```go
// GetCurrentWorktree returns the absolute path of the active worktree, or
// "" if the session is in the main worktree.
func (m *Manager) GetCurrentWorktree() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentWorktree
}

// SetCurrentWorktree records the active worktree path. Pass "" to indicate
// the session has exited a worktree (returned to main).
func (m *Manager) SetCurrentWorktree(path string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.currentWorktree = path
}
```

- [ ] **Step 4: Run tests**

```bash
cd /run/media/milosvasic/DATA4TB/Projects/helix_code/HelixCode && go test -count=1 -race -v -run 'TestManager_GetCurrentWorktree|TestManager_SetCurrentWorktree' ./internal/session/
```
Expected: PASS — 3 tests.

- [ ] **Step 5: Run full session package (regression check)**

```bash
cd /run/media/milosvasic/DATA4TB/Projects/helix_code/HelixCode && go test -count=1 -race ./internal/session/...
```
Expected: PASS — no regression in existing session tests.

- [ ] **Step 6: Anti-bluff smoke**

```bash
cd /run/media/milosvasic/DATA4TB/Projects/helix_code/HelixCode && grep -rn "simulated\|for now\|TODO implement\|placeholder" internal/session/manager.go internal/session/manager_worktree_test.go && echo "BLUFF FOUND" || echo "clean"
```
Expected: `clean`.

- [ ] **Step 7: Commit**

```bash
git -C /run/media/milosvasic/DATA4TB/Projects/HelixCode add helix_code/internal/session/manager.go helix_code/internal/session/manager_worktree_test.go
git -C /run/media/milosvasic/DATA4TB/Projects/HelixCode commit -m "$(cat <<'EOF'
feat(P1-F04-T08): session.Manager.currentWorktree field + getter/setter

Adds a single string field to session.Manager plus GetCurrentWorktree
and SetCurrentWorktree methods. Reuses the existing mu RWMutex for
thread-safety. Default is empty string (main worktree). The slash
command and CLI subcommand wiring (T09/T10) calls SetCurrentWorktree
when the user enters/exits a worktree so the session knows which
directory to use as cwd. 3 unit tests cover default, set, and clear.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 9: helixcode worktree {list,enter,exit,remove} Cobra subcommands

**Files:**
- Create: `helix_code/cmd/cli/worktree_cmd.go`
- Create: `helix_code/cmd/cli/worktree_cmd_test.go`
- Modify: `helix_code/cmd/cli/main.go` (add dispatcher entry)

- [ ] **Step 1: Write failing test**

Create `helix_code/cmd/cli/worktree_cmd_test.go`:

```go
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/tools/worktree"
)

func initEphemeralRepoForCLI(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = tmp
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v: %s", args, string(out))
	}
	run("init", "-b", "main")
	run("config", "user.email", "test@helixcode.dev")
	run("config", "user.name", "Test")
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "README.md"), []byte("seed\n"), 0o644))
	run("add", ".")
	run("commit", "-m", "seed")
	return tmp
}

func TestRunWorktreeList_EmptyShowsHeader(t *testing.T) {
	repo := initEphemeralRepoForCLI(t)
	m := worktree.NewManager(repo)
	var buf bytes.Buffer
	require.NoError(t, runWorktreeList(&buf, m))
	out := buf.String()
	assert.Contains(t, out, "NAME") // header row
}

func TestRunWorktreeList_AfterEnter(t *testing.T) {
	repo := initEphemeralRepoForCLI(t)
	m := worktree.NewManager(repo)

	// Use Manager directly to set up state (not through Cobra)
	_, err := m.EnterWorktree(t.Context(), "feature-cli", "")
	require.NoError(t, err)

	var buf bytes.Buffer
	require.NoError(t, runWorktreeList(&buf, m))
	assert.Contains(t, buf.String(), "feature-cli")
}

func TestRunWorktreeRemove_Works(t *testing.T) {
	repo := initEphemeralRepoForCLI(t)
	m := worktree.NewManager(repo)
	_, err := m.EnterWorktree(t.Context(), "feature-rm", "")
	require.NoError(t, err)
	m.ExitWorktree()

	require.NoError(t, runWorktreeRemove(m, "feature-rm"))
	_, statErr := os.Stat(filepath.Join(repo, worktree.WorktreeDir, "feature-rm"))
	assert.True(t, os.IsNotExist(statErr))
}

func TestRunWorktreeEnter_PrintsHelpAndErrors(t *testing.T) {
	var buf bytes.Buffer
	err := runWorktreeEnter(&buf, "feature-x", "")
	assert.Error(t, err, "stateful subcommand must error from CLI")
	assert.Contains(t, buf.String(), "helixcode chat",
		"output must direct user to interactive session")
}

func TestRunWorktreeExit_PrintsHelpAndErrors(t *testing.T) {
	var buf bytes.Buffer
	err := runWorktreeExit(&buf)
	assert.Error(t, err)
	assert.Contains(t, buf.String(), "helixcode chat")
}
```

- [ ] **Step 2: Run failing test**

```bash
cd /run/media/milosvasic/DATA4TB/Projects/helix_code/HelixCode && go test -count=1 -run 'TestRunWorktree' ./cmd/cli/
```
Expected: FAIL — functions undefined.

- [ ] **Step 3: Implement worktree_cmd.go**

Create `helix_code/cmd/cli/worktree_cmd.go`:

```go
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"dev.helix.code/internal/tools/worktree"
)

func newWorktreeCommand(m *worktree.Manager) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "worktree",
		Short: "Manage HelixCode-tracked git worktrees",
	}
	cmd.AddCommand(newWorktreeListCommand(m))
	cmd.AddCommand(newWorktreeEnterCommand())
	cmd.AddCommand(newWorktreeExitCommand())
	cmd.AddCommand(newWorktreeRemoveCommand(m))
	return cmd
}

func newWorktreeListCommand(m *worktree.Manager) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List helix-managed worktrees under .helix-worktrees/",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWorktreeList(os.Stdout, m)
		},
	}
}

func newWorktreeRemoveCommand(m *worktree.Manager) *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a helix-managed worktree",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWorktreeRemove(m, args[0])
		},
	}
}

func newWorktreeEnterCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "enter <name> [base-branch]",
		Short: "(stateful) Use from inside a `helixcode chat` session, not the CLI",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			baseBranch := ""
			if len(args) >= 2 {
				baseBranch = args[1]
			}
			return runWorktreeEnter(os.Stdout, args[0], baseBranch)
		},
	}
}

func newWorktreeExitCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "exit",
		Short: "(stateful) Use from inside a `helixcode chat` session, not the CLI",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWorktreeExit(os.Stdout)
		},
	}
}

func runWorktreeList(out io.Writer, m *worktree.Manager) error {
	wts, err := m.ListWorktrees(context.Background())
	if err != nil {
		return err
	}
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "NAME\tBRANCH\tPATH\n")
	for _, w := range wts {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", w.Name, w.Branch, w.Path)
	}
	if len(wts) == 0 {
		fmt.Fprintln(tw, "(no worktrees)\t\t")
	}
	return tw.Flush()
}

func runWorktreeRemove(m *worktree.Manager, name string) error {
	return m.RemoveWorktree(context.Background(), name)
}

func runWorktreeEnter(out io.Writer, name, baseBranch string) error {
	fmt.Fprintln(out, "`helixcode worktree enter` is a stateful operation.")
	fmt.Fprintln(out, "Run it from inside a `helixcode chat` session via the agent's EnterWorktree tool")
	fmt.Fprintln(out, "or the /worktree slash command. The CLI subcommand cannot persist worktree state across invocations.")
	return fmt.Errorf("stateful subcommand: use from inside helixcode chat")
}

func runWorktreeExit(out io.Writer) error {
	fmt.Fprintln(out, "`helixcode worktree exit` is a stateful operation.")
	fmt.Fprintln(out, "Run it from inside a `helixcode chat` session via the agent's ExitWorktree tool")
	fmt.Fprintln(out, "or the /worktree slash command. The CLI subcommand cannot persist worktree state across invocations.")
	return fmt.Errorf("stateful subcommand: use from inside helixcode chat")
}
```

- [ ] **Step 4: Wire the dispatcher in main.go**

Find the existing dispatcher block in `helix_code/cmd/cli/main.go` (added by F02 — it intercepts `os.Args[1] == "permissions"`). Add an analogous block immediately after it for "worktree":

```go
if len(os.Args) >= 2 && os.Args[1] == "worktree" {
	// Build the worktree manager directly (no full CLI startup needed for stateless ops).
	cwd, _ := os.Getwd()
	m := worktree.NewManager(cwd)
	cmd := newWorktreeCommand(m)
	cmd.SetArgs(os.Args[2:])
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
	return
}
```

Add the import for `dev.helix.code/internal/tools/worktree` at the top of `main.go`.

- [ ] **Step 5: Verify it compiles**

```bash
cd /run/media/milosvasic/DATA4TB/Projects/helix_code/HelixCode && go build ./cmd/cli/...
```
Expected: clean compile.

- [ ] **Step 6: Run tests**

```bash
cd /run/media/milosvasic/DATA4TB/Projects/helix_code/HelixCode && go test -count=1 -race -v -run 'TestRunWorktree' ./cmd/cli/
```
Expected: PASS — 5 tests.

- [ ] **Step 7: Smoke-test the binary**

```bash
cd /run/media/milosvasic/DATA4TB/Projects/helix_code/HelixCode && go build -o bin/helixcode ./cmd/cli && ./bin/helixcode worktree --help
```
Expected: prints subcommand list including `list`, `enter`, `exit`, `remove`.

- [ ] **Step 8: Anti-bluff smoke**

```bash
cd /run/media/milosvasic/DATA4TB/Projects/helix_code/HelixCode && grep -rn "simulated\|for now\|TODO implement\|placeholder" cmd/cli/worktree_cmd.go cmd/cli/worktree_cmd_test.go && echo "BLUFF FOUND" || echo "clean"
```
Expected: `clean`.

- [ ] **Step 9: Commit**

Note: `cmd/cli/` is matched by `helix_code/.gitignore` line 124 (bare `cli`), so new files there need `git add -f`.

```bash
git -C /run/media/milosvasic/DATA4TB/Projects/HelixCode add -f helix_code/cmd/cli/worktree_cmd.go helix_code/cmd/cli/worktree_cmd_test.go helix_code/cmd/cli/main.go
git -C /run/media/milosvasic/DATA4TB/Projects/HelixCode commit -m "$(cat <<'EOF'
feat(P1-F04-T09): helixcode worktree {list,enter,exit,remove} subcommands

Cobra subcommand group dispatched from cmd/cli/main.go's args sniffer
(same pattern as F02's helixcode permissions). list / remove call the
WorktreeManager directly. enter / exit print a help message directing
the user to the interactive session and exit non-zero (stateful ops
can't persist across stateless CLI invocations). 5 unit tests cover
list (empty + after enter), remove, and the helpful errors for
enter/exit.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 10: /worktree slash command + register in builtin/register.go

**Files:**
- Create: `helix_code/internal/commands/worktree_command.go`
- Create: `helix_code/internal/commands/worktree_command_test.go`
- Modify: `helix_code/internal/commands/builtin/register.go`
- Create: `helix_code/internal/commands/builtin/worktree_register_test.go`

- [ ] **Step 1: Write failing test for the slash command itself**

Create `helix_code/internal/commands/worktree_command_test.go`:

```go
package commands

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/tools/worktree"
)

func initEphemeralRepoForCommands(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = tmp
		require.NoError(t, cmd.Run())
	}
	run("init", "-b", "main")
	run("config", "user.email", "test@helixcode.dev")
	run("config", "user.name", "Test")
	require.NoError(t, exec.Command("touch", filepath.Join(tmp, "README.md")).Run())
	run("add", ".")
	run("commit", "-m", "seed", "--allow-empty")
	return tmp
}

func TestWorktreeCommand_NameAliases(t *testing.T) {
	repo := initEphemeralRepoForCommands(t)
	m := worktree.NewManager(repo)
	cmd := NewWorktreeCommand(m)

	assert.Equal(t, "worktree", cmd.Name())
	assert.Contains(t, cmd.Aliases(), "wt")
}

func TestWorktreeCommand_ListSubaction_Empty(t *testing.T) {
	repo := initEphemeralRepoForCommands(t)
	m := worktree.NewManager(repo)
	cmd := NewWorktreeCommand(m)

	res, err := cmd.Execute(context.Background(), &CommandContext{
		Args:     []string{},
		RawInput: "/worktree",
	})
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Contains(t, res.Output, "(no worktrees)",
		"empty list must explicitly say so")
}

func TestWorktreeCommand_EnterAndExit(t *testing.T) {
	repo := initEphemeralRepoForCommands(t)
	m := worktree.NewManager(repo)
	cmd := NewWorktreeCommand(m)

	res, err := cmd.Execute(context.Background(), &CommandContext{
		Args:     []string{"enter", "feature-cmd"},
		RawInput: "/worktree enter feature-cmd",
	})
	require.NoError(t, err)
	assert.Contains(t, res.Output, "feature-cmd")
	assert.True(t, m.IsIsolated())

	res, err = cmd.Execute(context.Background(), &CommandContext{
		Args:     []string{"exit"},
		RawInput: "/worktree exit",
	})
	require.NoError(t, err)
	assert.False(t, m.IsIsolated())
}

func TestWorktreeCommand_RemoveSubaction(t *testing.T) {
	repo := initEphemeralRepoForCommands(t)
	m := worktree.NewManager(repo)
	cmd := NewWorktreeCommand(m)

	// Set up: enter then exit
	_, err := m.EnterWorktree(context.Background(), "feature-rm-cmd", "")
	require.NoError(t, err)
	m.ExitWorktree()

	res, err := cmd.Execute(context.Background(), &CommandContext{
		Args:     []string{"remove", "feature-rm-cmd"},
		RawInput: "/worktree remove feature-rm-cmd",
	})
	require.NoError(t, err)
	assert.Contains(t, res.Output, "feature-rm-cmd")
}

func TestWorktreeCommand_RejectsUnknownSubaction(t *testing.T) {
	repo := initEphemeralRepoForCommands(t)
	m := worktree.NewManager(repo)
	cmd := NewWorktreeCommand(m)

	_, err := cmd.Execute(context.Background(), &CommandContext{
		Args:     []string{"frobnicate"},
		RawInput: "/worktree frobnicate",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown")
}

func TestWorktreeCommand_EnterRequiresName(t *testing.T) {
	repo := initEphemeralRepoForCommands(t)
	m := worktree.NewManager(repo)
	cmd := NewWorktreeCommand(m)

	_, err := cmd.Execute(context.Background(), &CommandContext{
		Args:     []string{"enter"},
		RawInput: "/worktree enter",
	})
	require.Error(t, err)
}
```

- [ ] **Step 2: Run failing test**

```bash
cd /run/media/milosvasic/DATA4TB/Projects/helix_code/HelixCode && go test -count=1 -run 'TestWorktreeCommand' ./internal/commands/
```
Expected: FAIL — `NewWorktreeCommand` undefined.

- [ ] **Step 3: Implement worktree_command.go**

Create `helix_code/internal/commands/worktree_command.go`:

```go
package commands

import (
	"bytes"
	"context"
	"fmt"
	"text/tabwriter"

	"dev.helix.code/internal/tools/worktree"
)

// WorktreeCommand implements /worktree.
//
// Subactions:
//   /worktree                          — list (default)
//   /worktree list                     — explicit list
//   /worktree enter <name> [branch]    — enters worktree (mutates state)
//   /worktree exit                     — returns to main
//   /worktree remove <name>            — deletes worktree + branch
type WorktreeCommand struct {
	m *worktree.Manager
}

// NewWorktreeCommand wires a Manager into the slash command.
func NewWorktreeCommand(m *worktree.Manager) *WorktreeCommand {
	return &WorktreeCommand{m: m}
}

func (c *WorktreeCommand) Name() string         { return "worktree" }
func (c *WorktreeCommand) Aliases() []string    { return []string{"wt"} }
func (c *WorktreeCommand) Description() string  { return "manage helix-tracked git worktrees" }
func (c *WorktreeCommand) Usage() string {
	return "/worktree [list | enter <name> [branch] | exit | remove <name>]"
}

func (c *WorktreeCommand) Execute(ctx context.Context, cmdCtx *CommandContext) (*CommandResult, error) {
	if len(cmdCtx.Args) == 0 {
		return c.list(ctx)
	}
	switch cmdCtx.Args[0] {
	case "list":
		return c.list(ctx)
	case "enter":
		if len(cmdCtx.Args) < 2 {
			return nil, fmt.Errorf("usage: /worktree enter <name> [branch]")
		}
		baseBranch := ""
		if len(cmdCtx.Args) >= 3 {
			baseBranch = cmdCtx.Args[2]
		}
		return c.enter(ctx, cmdCtx.Args[1], baseBranch)
	case "exit":
		return c.exit()
	case "remove":
		if len(cmdCtx.Args) < 2 {
			return nil, fmt.Errorf("usage: /worktree remove <name>")
		}
		return c.remove(ctx, cmdCtx.Args[1])
	default:
		return nil, fmt.Errorf("unknown /worktree subaction %q (valid: list, enter, exit, remove)", cmdCtx.Args[0])
	}
}

func (c *WorktreeCommand) list(ctx context.Context) (*CommandResult, error) {
	wts, err := c.m.ListWorktrees(ctx)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	tw := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "NAME\tBRANCH\tPATH\n")
	for _, w := range wts {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", w.Name, w.Branch, w.Path)
	}
	if len(wts) == 0 {
		fmt.Fprintln(tw, "(no worktrees)\t\t")
	}
	tw.Flush()
	return &CommandResult{Output: buf.String(), Success: true}, nil
}

func (c *WorktreeCommand) enter(ctx context.Context, name, baseBranch string) (*CommandResult, error) {
	path, err := c.m.EnterWorktree(ctx, name, baseBranch)
	if err != nil {
		return nil, err
	}
	return &CommandResult{
		Output:  fmt.Sprintf("entered worktree %q at %s\n", name, path),
		Success: true,
	}, nil
}

func (c *WorktreeCommand) exit() (*CommandResult, error) {
	c.m.ExitWorktree()
	return &CommandResult{
		Output:  "exited worktree; returned to main\n",
		Success: true,
	}, nil
}

func (c *WorktreeCommand) remove(ctx context.Context, name string) (*CommandResult, error) {
	if err := c.m.RemoveWorktree(ctx, name); err != nil {
		return nil, err
	}
	return &CommandResult{
		Output:  fmt.Sprintf("removed worktree %q\n", name),
		Success: true,
	}, nil
}
```

- [ ] **Step 4: Run slash command tests**

```bash
cd /run/media/milosvasic/DATA4TB/Projects/helix_code/HelixCode && go test -count=1 -race -v -run 'TestWorktreeCommand' ./internal/commands/
```
Expected: PASS — 7 tests.

- [ ] **Step 5: Write registration test**

Create `helix_code/internal/commands/builtin/worktree_register_test.go`:

```go
package builtin_test

import (
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/commands"
	"dev.helix.code/internal/commands/builtin"
	"dev.helix.code/internal/tools/worktree"
)

func TestRegisterBuiltinCommands_IncludesWorktree(t *testing.T) {
	tmp := t.TempDir()
	exec.Command("git", "init", tmp).Run()
	require.NoError(t, exec.Command("git", "-C", tmp, "config", "user.email", "x@y").Run())
	require.NoError(t, exec.Command("git", "-C", tmp, "config", "user.name", "x").Run())
	require.NoError(t, exec.Command("touch", filepath.Join(tmp, "f")).Run())
	require.NoError(t, exec.Command("git", "-C", tmp, "add", ".").Run())
	require.NoError(t, exec.Command("git", "-C", tmp, "commit", "-m", "x", "--allow-empty").Run())

	m := worktree.NewManager(tmp)
	registry := commands.NewRegistry()
	require.NoError(t, builtin.RegisterBuiltinCommandsWithWorktree(registry, m))

	cmd, ok := registry.Get("worktree")
	require.True(t, ok)
	assert.Equal(t, "worktree", cmd.Name())

	cmd2, ok := registry.Get("wt")
	require.True(t, ok)
	assert.Equal(t, "worktree", cmd2.Name(), "alias resolves to worktree")
}
```

- [ ] **Step 6: Run failing registration test**

```bash
cd /run/media/milosvasic/DATA4TB/Projects/helix_code/HelixCode && go test -count=1 -run 'TestRegisterBuiltinCommands_IncludesWorktree' ./internal/commands/builtin/
```
Expected: FAIL — `RegisterBuiltinCommandsWithWorktree` undefined.

- [ ] **Step 7: Modify builtin/register.go**

In `helix_code/internal/commands/builtin/register.go`:

1. Add the import for `dev.helix.code/internal/tools/worktree` at the top (existing imports include `"dev.helix.code/internal/commands"` from F02).

2. Add a NEW exported function (don't modify the existing `RegisterBuiltinCommands` — its signature is set):

```go
// RegisterBuiltinCommandsWithWorktree extends RegisterBuiltinCommands with
// the /worktree command, which requires a worktree.Manager dependency.
// Callers that have a Manager (cmd/cli/main.go startup) use this; callers
// without one (legacy paths) use the original RegisterBuiltinCommands.
func RegisterBuiltinCommandsWithWorktree(registry *commands.Registry, m *worktree.Manager) error {
	if err := RegisterBuiltinCommands(registry); err != nil {
		return err
	}
	return registry.Register(commands.NewWorktreeCommand(m))
}
```

3. Update `GetBuiltinCommandNames()` to include `"worktree"`.
4. Update `GetBuiltinCommandAliases()` to include `"wt": "worktree"`.

- [ ] **Step 8: Run registration test**

```bash
cd /run/media/milosvasic/DATA4TB/Projects/helix_code/HelixCode && go test -count=1 -race -v -run 'TestRegisterBuiltinCommands_IncludesWorktree' ./internal/commands/builtin/
```
Expected: PASS.

- [ ] **Step 9: Run full commands package + builtin package**

```bash
cd /run/media/milosvasic/DATA4TB/Projects/helix_code/HelixCode && go test -count=1 -race ./internal/commands/...
```
Expected: PASS — no regression in F02's permissions tests.

- [ ] **Step 10: Anti-bluff smoke**

```bash
cd /run/media/milosvasic/DATA4TB/Projects/helix_code/HelixCode && grep -rn "simulated\|for now\|TODO implement\|placeholder" internal/commands/worktree_command.go internal/commands/builtin/register.go && echo "BLUFF FOUND" || echo "clean"
```
Expected: `clean`.

- [ ] **Step 11: Commit**

```bash
git -C /run/media/milosvasic/DATA4TB/Projects/HelixCode add helix_code/internal/commands/worktree_command.go helix_code/internal/commands/worktree_command_test.go helix_code/internal/commands/builtin/register.go helix_code/internal/commands/builtin/worktree_register_test.go
git -C /run/media/milosvasic/DATA4TB/Projects/HelixCode commit -m "$(cat <<'EOF'
feat(P1-F04-T10): /worktree slash command + register in builtin

WorktreeCommand implements commands.Command. Subactions: list (default),
enter, exit, remove. Reports tabular list output with NAME/BRANCH/PATH
columns. Aliased to /wt.

builtin/register.go gains RegisterBuiltinCommandsWithWorktree (the
existing RegisterBuiltinCommands keeps its signature for legacy callers).
GetBuiltinCommandNames + GetBuiltinCommandAliases extended.

7 unit tests for the slash command (Name/Aliases, list-empty, enter+exit,
remove, unknown-subaction-rejection, missing-name) + 1 registration
test verifying /worktree and /wt both resolve via the registry.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 11: cmd/cli/main.go startup wiring + integration test (no mocks)

**Files:**
- Modify: `helix_code/cmd/cli/main.go`
- Create: `helix_code/tests/integration/worktree/worktree_integration_test.go`

- [ ] **Step 1: Investigate main.go's CLI struct + init pattern**

```bash
grep -n 'permissionsEngine\|persistenceManager\|initPermissions\|initPersistence' /run/media/milosvasic/DATA4TB/Projects/helix_code/helix_code/cmd/cli/main.go | head
```

Confirm the F02/F03 pattern: a field on the CLI struct + an `initX` method called during `Run()` after the dispatcher's other inits.

- [ ] **Step 2: Add worktree.Manager bootstrap to main.go**

Add the import (alongside `permissions` and `persistence`):

```go
import (
	// existing imports...
	"dev.helix.code/internal/tools/worktree"
)
```

Add the field to the CLI struct (near `permissionsEngine` and `persistenceManager`):

```go
	worktreeManager *worktree.Manager
```

Add the bootstrap method (near `initPermissions` / `initPersistence`):

```go
// initWorktree bootstraps the worktree.Manager with repoRoot resolved via
// `git rev-parse --show-toplevel`, falling back to os.Getwd() if the cwd
// is not a git repo.
func (c *CLI) initWorktree(ctx context.Context) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolving cwd for worktree: %w", err)
	}
	repoRoot := cwd
	if root, err := worktreeRevParseToplevel(ctx, cwd); err == nil {
		repoRoot = root
	}
	c.worktreeManager = worktree.NewManager(repoRoot)
	return nil
}

// worktreeRevParseToplevel is a tiny shim to avoid leaking the worktree
// package's internal helpers; it shells out to git directly.
func worktreeRevParseToplevel(ctx context.Context, cwd string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--show-toplevel")
	cmd.Dir = cwd
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
```

(`exec` and `strings` need imports if they aren't already imported. `context` likely is.)

Call `initWorktree` from the existing CLI startup sequence (after `initPermissions` / `initPersistence`):

```go
if err := c.initPersistence(); err != nil {
	return fmt.Errorf("persistence init: %w", err)
}
if err := c.initWorktree(ctx); err != nil {
	return fmt.Errorf("worktree init: %w", err)
}
```

- [ ] **Step 3: Verify it compiles**

```bash
cd /run/media/milosvasic/DATA4TB/Projects/helix_code/HelixCode && go build ./cmd/cli/...
```
Expected: clean compile.

- [ ] **Step 4: Write the integration test**

```bash
mkdir -p /run/media/milosvasic/DATA4TB/Projects/helix_code/helix_code/tests/integration/worktree
```

Create `helix_code/tests/integration/worktree/worktree_integration_test.go`:

```go
//go:build integration

package worktree_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/tools/worktree"
)

func initEphemeralRepo(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = tmp
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v: %s", args, string(out))
	}
	run("init", "-b", "main")
	run("config", "user.email", "test@helixcode.dev")
	run("config", "user.name", "Test")
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "README.md"), []byte("seed\n"), 0o644))
	run("add", ".")
	run("commit", "-m", "seed")
	return tmp
}

// TestIntegration_EnterCommitDoesNotPolluteMain proves that a commit made
// in a worktree does NOT change main's HEAD. NO mocks.
func TestIntegration_EnterCommitDoesNotPolluteMain(t *testing.T) {
	repo := initEphemeralRepo(t)
	m := worktree.NewManager(repo)

	mainHEADBefore, err := exec.Command("git", "-C", repo, "rev-parse", "main").Output()
	require.NoError(t, err)

	wtPath, err := m.EnterWorktree(context.Background(), "feature-x", "")
	require.NoError(t, err)

	// Make a commit on feature-x inside the worktree
	require.NoError(t, os.WriteFile(filepath.Join(wtPath, "new.txt"), []byte("isolated"), 0o644))
	add := exec.Command("git", "-C", wtPath, "add", ".")
	require.NoError(t, add.Run())
	commit := exec.Command("git", "-C", wtPath, "commit", "-m", "isolated work")
	require.NoError(t, commit.Run())

	mainHEADAfter, err := exec.Command("git", "-C", repo, "rev-parse", "main").Output()
	require.NoError(t, err)
	assert.Equal(t, strings.TrimSpace(string(mainHEADBefore)), strings.TrimSpace(string(mainHEADAfter)),
		"main's HEAD must not change after committing inside the worktree")

	// And the new file must NOT exist in the main worktree
	_, statErr := os.Stat(filepath.Join(repo, "new.txt"))
	assert.True(t, os.IsNotExist(statErr), "new.txt must only exist in the worktree, not main")
}

// TestIntegration_RoundTripCreateRemove proves end-to-end worktree creation
// and removal against a real git repo, no mocks.
func TestIntegration_RoundTripCreateRemove(t *testing.T) {
	repo := initEphemeralRepo(t)
	m := worktree.NewManager(repo)

	wtPath, err := m.EnterWorktree(context.Background(), "feature-y", "")
	require.NoError(t, err)
	assert.True(t, m.IsIsolated())

	// Verify directory exists
	info, err := os.Stat(wtPath)
	require.NoError(t, err)
	assert.True(t, info.IsDir())

	// Exit and remove
	m.ExitWorktree()
	require.NoError(t, m.RemoveWorktree(context.Background(), "feature-y"))

	_, statErr := os.Stat(wtPath)
	assert.True(t, os.IsNotExist(statErr))

	// `git worktree list` should no longer mention feature-y
	out, err := exec.Command("git", "-C", repo, "worktree", "list", "--porcelain").Output()
	require.NoError(t, err)
	assert.NotContains(t, string(out), "feature-y")
}

// TestIntegration_PathTraversalRejected proves that ValidateName rejects
// names that would escape the persistence directory.
func TestIntegration_PathTraversalRejected(t *testing.T) {
	repo := initEphemeralRepo(t)
	m := worktree.NewManager(repo)

	_, err := m.EnterWorktree(context.Background(), "../etc", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "match pattern")
}
```

- [ ] **Step 5: Run integration tests**

```bash
cd /run/media/milosvasic/DATA4TB/Projects/helix_code/HelixCode && go test -count=1 -race -v -tags=integration ./tests/integration/worktree/...
```
Expected: PASS — 3 tests.

- [ ] **Step 6: Anti-bluff smoke**

```bash
cd /run/media/milosvasic/DATA4TB/Projects/helix_code/HelixCode && grep -rn "simulated\|for now\|TODO implement\|placeholder" cmd/cli/main.go internal/tools/worktree/ tests/integration/worktree/ && echo "BLUFF FOUND" || echo "clean"
```
Expected: `clean`. Note: `cmd/cli/main.go` may have pre-existing hits — only flag NEW ones.

- [ ] **Step 7: Commit**

```bash
git -C /run/media/milosvasic/DATA4TB/Projects/HelixCode add -f helix_code/cmd/cli/main.go
git -C /run/media/milosvasic/DATA4TB/Projects/HelixCode add helix_code/tests/integration/worktree/
git -C /run/media/milosvasic/DATA4TB/Projects/HelixCode commit -m "$(cat <<'EOF'
feat(P1-F04-T11): cmd/cli/main.go startup wiring + integration tests

CLI bootstrap constructs worktree.Manager with repoRoot resolved via
`git rev-parse --show-toplevel` (fallback to os.Getwd if the cwd is not
a git repo). Three integration tests with -tags=integration and NO
mocks: commit in worktree leaves main's HEAD unchanged, round-trip
create+remove works, path-traversal name rejected.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 12: Challenge with three runtime-evidence scenarios

**Files:**
- Create: `helix_code/tests/e2e/challenges/worktree/expected.json`
- Create: `helix_code/tests/e2e/challenges/worktree/run.sh`
- Create: `helix_code/tests/e2e/challenges/worktree/README.md`

- [ ] **Step 1: Create the directory**

```bash
mkdir -p /run/media/milosvasic/DATA4TB/Projects/helix_code/helix_code/tests/e2e/challenges/worktree
```

- [ ] **Step 2: Write expected.json**

Create `helix_code/tests/e2e/challenges/worktree/expected.json`:

```json
{
  "name": "worktree/agent-isolation-end-to-end",
  "feature": "P1-F04 — Git Worktree Agent Isolation",
  "scenarios": [
    {
      "id": "S1-isolation-preserves-main",
      "expected_main_head_unchanged": true,
      "expected_new_file_not_in_main": true
    },
    {
      "id": "S2-clean-reentry-idempotent",
      "expected_first_path_equals_second_path": true
    },
    {
      "id": "S3-invalid-names-rejected",
      "names": ["../etc", "", "name with spaces", "65-chars-long-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"],
      "expected_all_rejected": true
    }
  ]
}
```

- [ ] **Step 3: Write run.sh**

Create `helix_code/tests/e2e/challenges/worktree/run.sh`:

```bash
#!/usr/bin/env bash
# Challenge: P1-F04 — Git Worktree Agent Isolation end-to-end runtime evidence.
# Drives the worktree.Manager directly through a Go test binary.
set -euo pipefail

HERE=$(cd "$(dirname "$0")" && pwd)
ROOT=$(cd "$HERE/../../../.." && pwd)
WORK=$(mktemp -d -p "$ROOT/cmd")
trap 'rm -rf "$WORK"' EXIT

# Build a tiny Go driver. It must live inside the module tree because Go's
# internal/-package rules forbid imports from outside.
cat > "$WORK/driver.go" <<'EOF'
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"dev.helix.code/internal/tools/worktree"
)

func mustRun(dir string, args ...string) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "git %v in %s: %v\n%s\n", args, dir, err, out)
		os.Exit(1)
	}
}

func initRepo() string {
	tmp, err := os.MkdirTemp("", "f04-driver-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	mustRun(tmp, "init", "-b", "main")
	mustRun(tmp, "config", "user.email", "x@y")
	mustRun(tmp, "config", "user.name", "x")
	if err := os.WriteFile(filepath.Join(tmp, "README.md"), []byte("seed\n"), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	mustRun(tmp, "add", ".")
	mustRun(tmp, "commit", "-m", "seed")
	return tmp
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: driver <scenario>")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "s1":
		repo := initRepo()
		defer os.RemoveAll(repo)
		m := worktree.NewManager(repo)
		mainHEADBefore, _ := exec.Command("git", "-C", repo, "rev-parse", "main").Output()
		wtPath, err := m.EnterWorktree(context.Background(), "feature-x", "")
		if err != nil {
			fmt.Fprintln(os.Stderr, "EnterWorktree:", err)
			os.Exit(1)
		}
		_ = os.WriteFile(filepath.Join(wtPath, "new.txt"), []byte("isolated"), 0o644)
		mustRun(wtPath, "add", ".")
		mustRun(wtPath, "commit", "-m", "isolated work")
		mainHEADAfter, _ := exec.Command("git", "-C", repo, "rev-parse", "main").Output()
		_, statErr := os.Stat(filepath.Join(repo, "new.txt"))
		fmt.Printf("main_head_unchanged=%v\n", strings.TrimSpace(string(mainHEADBefore)) == strings.TrimSpace(string(mainHEADAfter)))
		fmt.Printf("new_file_not_in_main=%v\n", os.IsNotExist(statErr))
	case "s2":
		repo := initRepo()
		defer os.RemoveAll(repo)
		m := worktree.NewManager(repo)
		first, err := m.EnterWorktree(context.Background(), "feature-y", "")
		if err != nil {
			fmt.Fprintln(os.Stderr, "first enter:", err)
			os.Exit(1)
		}
		second, err := m.EnterWorktree(context.Background(), "feature-y", "")
		if err != nil {
			fmt.Fprintln(os.Stderr, "second enter:", err)
			os.Exit(1)
		}
		fmt.Printf("first_path_equals_second_path=%v\n", first == second)
	case "s3":
		repo := initRepo()
		defer os.RemoveAll(repo)
		m := worktree.NewManager(repo)
		bads := []string{"../etc", "", "name with spaces", strings.Repeat("a", 65)}
		allRejected := true
		for _, name := range bads {
			if _, err := m.EnterWorktree(context.Background(), name, ""); err == nil {
				fmt.Fprintf(os.Stderr, "name %q was accepted but should be rejected\n", name)
				allRejected = false
			}
		}
		fmt.Printf("all_rejected=%v\n", allRejected)
	default:
		fmt.Fprintf(os.Stderr, "unknown scenario %q\n", os.Args[1])
		os.Exit(2)
	}
}
EOF

DRIVER_BIN="$WORK/driver"
(cd "$ROOT" && go build -o "$DRIVER_BIN" "$WORK/driver.go")

# S1: isolation preserves main
echo "=== S1: isolation preserves main ==="
S1_OUT=$("$DRIVER_BIN" s1)
echo "$S1_OUT"
if ! echo "$S1_OUT" | grep -q "^main_head_unchanged=true$"; then
  echo "FAIL S1: main HEAD changed after worktree commit"
  exit 1
fi
if ! echo "$S1_OUT" | grep -q "^new_file_not_in_main=true$"; then
  echo "FAIL S1: new file leaked into main worktree"
  exit 1
fi

# S2: clean re-entry is idempotent
echo
echo "=== S2: clean re-entry idempotent ==="
S2_OUT=$("$DRIVER_BIN" s2)
echo "$S2_OUT"
if ! echo "$S2_OUT" | grep -q "^first_path_equals_second_path=true$"; then
  echo "FAIL S2: re-entry returned different path"
  exit 1
fi

# S3: invalid names rejected
echo
echo "=== S3: invalid names rejected ==="
S3_OUT=$("$DRIVER_BIN" s3)
echo "$S3_OUT"
if ! echo "$S3_OUT" | grep -q "^all_rejected=true$"; then
  echo "FAIL S3: at least one invalid name was accepted"
  exit 1
fi

echo
echo "PASS: all three scenarios produced expected outcomes"
```

- [ ] **Step 4: Make it executable**

```bash
chmod +x /run/media/milosvasic/DATA4TB/Projects/helix_code/helix_code/tests/e2e/challenges/worktree/run.sh
```

- [ ] **Step 5: Write README.md**

Create `helix_code/tests/e2e/challenges/worktree/README.md`:

```markdown
# Challenge — Git Worktree Agent Isolation (P1-F04)

End-to-end runtime evidence that worktree isolation actually keeps a
worktree's commits OUT of the main worktree.

## Scenarios

1. **S1 — isolation preserves main**: enter `feature-x`, write a file,
   commit on `feature-x`, then verify main's HEAD is unchanged and the
   new file does NOT appear in the main worktree.
2. **S2 — clean re-entry is idempotent**: enter `feature-y` twice; both
   calls return the same path.
3. **S3 — invalid names rejected**: try `../etc`, empty string, names
   containing spaces, and 65-char names. All four must be rejected.

## Run

```bash
cd HelixCode && tests/e2e/challenges/worktree/run.sh
```

Exit 0 means PASS. Exit non-zero means at least one scenario failed.

## Mutation test (CONST-039)

To verify the Challenge actually catches a broken engine:

```go
// in internal/tools/worktree/manager.go ValidateName():
//     if !worktreeNamePattern.MatchString(name) { ... }
//
// Comment out the regex check.
```

Re-run `run.sh`. S3 MUST FAIL because invalid names are now accepted.
Revert the mutation and confirm PASS.
```

(Backticks in the inner ```bash and ```go blocks must be real backticks in the file.)

- [ ] **Step 6: Run the Challenge**

```bash
cd /run/media/milosvasic/DATA4TB/Projects/helix_code/HelixCode && tests/e2e/challenges/worktree/run.sh 2>&1 | tee /tmp/p1-f04-t12-evidence.txt
```
Expected: PASS at the end. Exit 0.

- [ ] **Step 7: Anti-bluff smoke**

```bash
cd /run/media/milosvasic/DATA4TB/Projects/helix_code/HelixCode && grep -rn "simulated\|for now\|TODO implement\|placeholder" tests/e2e/challenges/worktree/ && echo "BLUFF FOUND" || echo "clean"
```
Expected: `clean`.

- [ ] **Step 8: Commit**

```bash
git -C /run/media/milosvasic/DATA4TB/Projects/HelixCode add helix_code/tests/e2e/challenges/worktree/
git -C /run/media/milosvasic/DATA4TB/Projects/HelixCode commit -m "$(cat <<'EOF'
feat(P1-F04-T12): Challenge for worktree isolation with runtime evidence

Three scenarios driven by a Go-built driver invoking worktree.Manager
directly:
  S1: enter feature-x, commit a file, verify main's HEAD unchanged AND
      the new file is absent from the main worktree.
  S2: enter feature-y twice → both calls return the same path
      (idempotent re-entry).
  S3: try ../etc, empty, "name with spaces", 65-char name → all four
      rejected by ValidateName.

Mutation-test recipe in README.md ensures the Challenge will FAIL if
the regex check in ValidateName is disabled.

Runtime evidence: see commit body of P1-F04-T13 close-out.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 13: Feature 4 close-out + push (no force, CONST-043)

**Files:**
- Modify: `docs/improvements/06_phase_1_evidence.md`
- Modify: `docs/improvements/PROGRESS.md`

- [ ] **Step 1: Re-run the Challenge to capture fresh evidence**

```bash
cd /run/media/milosvasic/DATA4TB/Projects/helix_code/HelixCode && tests/e2e/challenges/worktree/run.sh 2>&1 | tee /tmp/p1-f04-t13-rerun.txt
```
Expected: PASS. If FAIL, STOP.

- [ ] **Step 2: Run final regression test**

```bash
cd /run/media/milosvasic/DATA4TB/Projects/helix_code/HelixCode && \
  go test -count=1 -race ./internal/tools/worktree/... ./internal/tools/persistence/... ./internal/tools/permissions/... ./internal/tools/confirmation/... ./internal/llm/... ./internal/agent/... ./internal/session/... ./internal/commands/... ./cmd/cli/... 2>&1 | tee /tmp/p1-f04-t13-tests.txt && \
  go test -count=1 -race -tags=integration ./tests/integration/worktree/... ./tests/integration/persistence/... ./tests/integration/permissions/... 2>&1 | tee -a /tmp/p1-f04-t13-tests.txt
```
Expected: PASS for every package.

- [ ] **Step 3: Run verify-foundation gate**

```bash
cd /run/media/milosvasic/DATA4TB/Projects/HelixCode && make verify-foundation 2>&1 | tee /tmp/p1-f04-t13-verify.txt
```

Phase 0 LLMsVerifier-pin baseline (exit 2 warn-only) is acceptable — same as F01/F02/F03.

- [ ] **Step 4: Anti-bluff smoke (broad)**

```bash
cd /run/media/milosvasic/DATA4TB/Projects/helix_code/HelixCode && grep -rn "simulated\|for now\|TODO implement\|placeholder" \
  internal/tools/worktree/ tests/e2e/challenges/worktree/ \
  tests/integration/worktree/ \
  internal/commands/worktree_command.go internal/commands/worktree_command_test.go \
  internal/commands/builtin/worktree_register_test.go \
  cmd/cli/worktree_cmd.go cmd/cli/worktree_cmd_test.go \
  internal/session/manager_worktree_test.go && echo "BLUFF FOUND" || echo "clean"
```
Expected: `clean`.

- [ ] **Step 5: Append runtime evidence to evidence file**

In `docs/improvements/06_phase_1_evidence.md`, replace the F04 `### Task evidence trail` placeholder with:

```markdown
### Task evidence trail

- T01 — `<sha-T01>` — bootstrap evidence + advance PROGRESS + .gitignore
- T02 — `<sha-T02>` — package skeleton (types + doc)
- T03 — `<sha-T03>` — git binary wrappers (7 unit tests against ephemeral repos)
- T04 — `<sha-T04>` — Manager + ValidateName + GetCurrentDirectory + IsIsolated (7 tests)
- T05 — `<sha-T05>` — Manager.EnterWorktree (7 tests)
- T06 — `<sha-T06>` — Manager.ExitWorktree + ListWorktrees + RemoveWorktree (7 tests)
- T07 — `<sha-T07>` — 4 tools.Tool implementations (8 tests)
- T08 — `<sha-T08>` — session.Manager.currentWorktree field + getter/setter (3 tests)
- T09 — `<sha-T09>` — helixcode worktree {list,enter,exit,remove} subcommands (5 tests)
- T10 — `<sha-T10>` — /worktree slash command + builtin registration (7+1 tests)
- T11 — `<sha-T11>` — cmd/cli/main.go startup wiring + integration tests (3 tests, no mocks)
- T12 — `<sha-T12>` — Challenge with runtime evidence (3 scenarios)

### Challenge runtime evidence (from T12, re-verified at T13 close-out)

```
<paste verbatim contents of /tmp/p1-f04-t13-rerun.txt>
```

### Anti-bluff scan

```
<paste actual command + 'clean' output from Step 4>
```

### Verify-foundation gate

```
<paste verbatim contents of /tmp/p1-f04-t13-verify.txt>
```

### Closure

F04 closed 2026-05-05. F05 (Hook-Based Extensibility) unblocked.
```

Replace `<sha-TNN>` placeholders with actual short SHAs from `git log --oneline -16`.

- [ ] **Step 6: Update PROGRESS.md**

Edit `docs/improvements/PROGRESS.md`:

1. Update the Current focus block:

```markdown
## Current focus
- **Active phase:** P1 — claude-code feature porting
- **Active feature:** F05 — Hook-Based Extensibility (awaits its own writing-plans cycle)
- **Active task:** pending
- **Last completed:** P1-F04-T13 — Feature 4 (Git Worktree Agent Isolation) close-out + push
- **Owner:** agent (Claude Opus 4.7)
- **Started:** 2026-05-04
- **Last touched:** 2026-05-05
- **Blocked-on:** none
```

2. Mark every F04 task `[x]` in the F04 task list block. Append SHAs.

3. Append a Decision-log entry:

```markdown
- 2026-05-05 — Feature 4 (Git Worktree Agent Isolation) closed. Thirteen sub-commits. New thin sub-package `internal/tools/worktree` mirrors F02/F03's pattern. Shells out to the git binary consistent with internal/tools/git/. Worktrees stored at <repoRoot>/.helix-worktrees/<name>/ (in-repo; .gitignore'd). Meta-only — no submodule auto-init; agents that need submodule code run `git submodule update --init --recursive` from inside the worktree. Full surface: 4 agent tools + 4 Cobra subcommands (enter/exit print help when called from CLI) + 1 /worktree slash command. Per-session state via single field on session.Manager rather than a parallel worktree_state.go file.
```

- [ ] **Step 7: Commit close-out**

```bash
git -C /run/media/milosvasic/DATA4TB/Projects/HelixCode add docs/improvements/06_phase_1_evidence.md docs/improvements/PROGRESS.md
git -C /run/media/milosvasic/DATA4TB/Projects/HelixCode commit -m "$(cat <<'EOF'
chore(P1-F04-T13): Feature 4 (Git Worktree Agent Isolation) close-out

Thirteen sub-commits. New internal/tools/worktree sub-package shells
out to the git binary (consistent with internal/tools/git/) to manage
isolated worktrees at <repoRoot>/.helix-worktrees/<name>/. Meta-only
checkouts (no submodule auto-init). Full surface: 4 agent tools, 4
Cobra subcommands (enter/exit print help when called from CLI), 1
/worktree slash command. Per-session "current worktree" state lives
on session.Manager via a single field.

Challenge runtime evidence (verbatim from tests/e2e/challenges/worktree/run.sh):

<paste full S1/S2/S3 transcript from /tmp/p1-f04-t13-rerun.txt>

Anti-bluff scan: clean.
Verify-foundation gate: <exit code + summary>.

PROGRESS advanced: F04 done; F05 (Hook-Based Extensibility) unblocked.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

(Replace placeholders with real values before committing.)

- [ ] **Step 8: Push to all four configured remotes (NON-FORCE per CONST-043)**

```bash
git -C /run/media/milosvasic/DATA4TB/Projects/HelixCode push origin main
git -C /run/media/milosvasic/DATA4TB/Projects/HelixCode push github main
git -C /run/media/milosvasic/DATA4TB/Projects/HelixCode push gitlab main
git -C /run/media/milosvasic/DATA4TB/Projects/HelixCode push upstream main
```

If any push fails non-fast-forward, STOP and report — do NOT use `--force`.

- [ ] **Step 9: Verify upstream parity**

```bash
HEAD=$(git -C /run/media/milosvasic/DATA4TB/Projects/HelixCode rev-parse HEAD)
echo "HEAD: $HEAD"
for r in origin github gitlab upstream; do
  echo "=== $r ==="
  git -C /run/media/milosvasic/DATA4TB/Projects/HelixCode ls-remote --heads "$r" main
done
```

Each remote's `main` SHA should equal HEAD.

---

## Self-review against the spec

Walked spec section-by-section against the plan:

- **§1.4 S1 (`make verify-compile` exits 0)** — covered by T02 step 3, T07 step 5, T11 step 3, T13 step 2.
- **§1.4 S2 (unit tests with `-race`)** — every TDD task uses `-race`.
- **§1.4 S3 (integration test, no mocks)** — T11.
- **§1.4 S4 (Challenge + runtime evidence pasted)** — T12 + T13.
- **§1.4 S5 (anti-bluff smoke clean)** — every task ends with the smoke check.
- **§1.4 S6 (4 agent tools registered + `helixcode worktree list` smoke + `/worktree` discoverable)** — T07 (tools impl), T09 (Cobra), T10 (slash + registry test).
- **§1.4 S7 (`session.Manager.GetCurrentWorktree`)** — T08.
- **§2.3 component table** — every entry maps to T02–T11.
- **§3.1 constants + §3.2 PersistedResult-equivalent (Worktree)** — T02.
- **§3.3 Manager API** — T04 (Validate/GetCurrentDirectory/IsIsolated), T05 (EnterWorktree), T06 (ExitWorktree/ListWorktrees/RemoveWorktree).
- **§4 branch-creation semantics** — T05 implements; T05 step 1 has 7 named tests covering both paths + dirty + reuse.
- **§5 submodule handling** — T07's EnterWorktreeTool.Description() includes the documented note; verified by `TestEnterWorktreeTool_NameDescriptionCategory`.
- **§6 CLI surface** — T07 (tools), T09 (Cobra), T10 (slash).
- **§7 per-session state** — T08.
- **§8 error handling** — T05/T06 unit tests cover the named cases (dirty, invalid-name, current-worktree-removal).
- **§9.5 mutation test** — T12 README.md documents it.

No spec section is uncovered.

Type consistency: `Manager`, `NewManager`, `EnterWorktree`, `ExitWorktree`, `ListWorktrees`, `RemoveWorktree`, `GetCurrentDirectory`, `IsIsolated`, `ValidateName`, `Worktree`, `WorktreeNameRegex`, `WorktreeNameMaxLength`, `WorktreeDir`, `worktreeNamePattern`, `gitWorktreeAdd`, `gitWorktreeAddNewBranch`, `gitWorktreeRemove`, `gitWorktreeList`, `gitStatusPorcelain`, `gitRevParseToplevel`, `EnterWorktreeTool`, `ExitWorktreeTool`, `ListWorktreesTool`, `RemoveWorktreeTool`, `WorktreeCommand`, `NewWorktreeCommand`, `RegisterBuiltinCommandsWithWorktree` — all referenced consistently across tasks.

Placeholder scan: every step has either real code, a real command, or a real verification check. The literal `<sha-TNN>` strings in T13 are intentional (cannot be known until prior commits land).

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-05-05-p1-f04-git-worktree-agent-isolation.md`. Two execution options:

**1. Subagent-Driven (recommended)** — fresh subagent per task, review between tasks, fast iteration.

**2. Inline Execution** — execute tasks in this session using executing-plans, batch execution with checkpoints.

Which approach?
