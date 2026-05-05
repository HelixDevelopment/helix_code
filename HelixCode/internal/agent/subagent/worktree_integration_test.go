package subagent

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

// initSubagentRepo creates a real temporary git repo with one seed commit on
// `main`. Mirrors worktree.initEphemeralRepo; copied here because that helper
// is package-private to internal/tools/worktree.
func initSubagentRepo(t *testing.T) string {
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

func TestWorktreeIntegration_Setup_CreatesRealWorktree(t *testing.T) {
	repo := initSubagentRepo(t)
	mgr := worktree.NewManager(repo)
	wi := NewWorktreeIntegration(mgr)
	require.NotNil(t, wi)

	task := SubagentTask{
		ID:          "11111111-2222-3333-4444-555555555555",
		Description: "create-real-worktree",
		Prompt:      "do work",
		Isolation:   IsolationWorktree,
	}

	workDir, cleanup, err := wi.Setup(context.Background(), task)
	require.NoError(t, err)
	require.NotEqual(t, repo, workDir, "worktree path must differ from repo root")
	require.NotNil(t, cleanup)

	info, err := os.Stat(workDir)
	require.NoError(t, err)
	require.True(t, info.IsDir())

	out, err := exec.Command("git", "-C", repo, "worktree", "list", "--porcelain").Output()
	require.NoError(t, err)
	assert.Contains(t, string(out), workDir, "git worktree list must show the new worktree")

	require.NoError(t, cleanup())
	_, statErr := os.Stat(workDir)
	assert.True(t, os.IsNotExist(statErr), "cleanup must remove the worktree dir")
}

func TestWorktreeIntegration_Setup_BaseBranchHonored(t *testing.T) {
	repo := initSubagentRepo(t)
	cmd := exec.Command("git", "branch", "feature")
	cmd.Dir = repo
	require.NoError(t, cmd.Run())

	mgr := worktree.NewManager(repo)
	wi := NewWorktreeIntegration(mgr)

	task := SubagentTask{
		ID:          "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		Description: "base-branch-feature",
		Prompt:      "do work on feature",
		Isolation:   IsolationWorktree,
		BaseBranch:  "feature",
	}

	workDir, cleanup, err := wi.Setup(context.Background(), task)
	require.NoError(t, err)
	defer func() { _ = cleanup() }()

	out, err := exec.Command("git", "-C", workDir, "branch", "--show-current").Output()
	require.NoError(t, err)
	assert.Equal(t, "feature", strings.TrimSpace(string(out)))
}

func TestWorktreeIntegration_Setup_EmptyBaseBranchUsesDefault(t *testing.T) {
	repo := initSubagentRepo(t)
	mgr := worktree.NewManager(repo)
	wi := NewWorktreeIntegration(mgr)

	task := SubagentTask{
		ID:          "ffffffff-0000-1111-2222-333333333333",
		Description: "default-branch",
		Prompt:      "do work",
		Isolation:   IsolationWorktree,
		// BaseBranch intentionally empty.
	}

	workDir, cleanup, err := wi.Setup(context.Background(), task)
	require.NoError(t, err)
	defer func() { _ = cleanup() }()

	// With BaseBranch="" the F04 helper defaults branch to the worktree
	// name, which yields the deterministic "helixcode-subagent-<id>" branch.
	out, err := exec.Command("git", "-C", workDir, "branch", "--show-current").Output()
	require.NoError(t, err)
	branch := strings.TrimSpace(string(out))
	assert.True(t, strings.HasPrefix(branch, "helixcode-subagent-"),
		"empty BaseBranch defaults to subagent-name branch, got %q", branch)
}

func TestWorktreeIntegration_CaptureDiff_ReturnsActualChanges(t *testing.T) {
	repo := initSubagentRepo(t)
	mgr := worktree.NewManager(repo)
	wi := NewWorktreeIntegration(mgr)

	task := SubagentTask{
		ID:          "diff-id-1",
		Description: "capture-diff",
		Prompt:      "modify",
		Isolation:   IsolationWorktree,
	}

	workDir, cleanup, err := wi.Setup(context.Background(), task)
	require.NoError(t, err)
	defer func() { _ = cleanup() }()

	// Add a real file and stage it inside the worktree.
	newPath := filepath.Join(workDir, "added.txt")
	require.NoError(t, os.WriteFile(newPath, []byte("subagent-content\n"), 0o644))
	addCmd := exec.Command("git", "-C", workDir, "add", "added.txt")
	require.NoError(t, addCmd.Run())

	diff, err := wi.CaptureDiff(context.Background(), workDir)
	require.NoError(t, err)
	assert.NotEmpty(t, diff, "diff must be non-empty after staging a new file")
	assert.Contains(t, diff, "added.txt")
	assert.Contains(t, diff, "subagent-content")
}

func TestWorktreeIntegration_CaptureDiff_GracefulOnError(t *testing.T) {
	repo := initSubagentRepo(t)
	mgr := worktree.NewManager(repo)
	wi := NewWorktreeIntegration(mgr)

	notARepo := t.TempDir() // no `git init`
	diff, err := wi.CaptureDiff(context.Background(), notARepo)
	// Documented contract: returns ("", error) on failure.
	assert.Empty(t, diff)
	assert.Error(t, err)

	// repo unused — but reference it so an unused-warning never appears
	// even if the test body is later trimmed.
	_ = repo
}

// TestWorktreeIntegration_DoesNotMutateParentState is the load-bearing
// anti-bluff anchor. If Setup ever started using EnterWorktree (which mutates
// currentWorktree), this test fails: the parent agent's view of "where am I"
// must NOT change just because a subagent was dispatched.
func TestWorktreeIntegration_DoesNotMutateParentState(t *testing.T) {
	repo := initSubagentRepo(t)
	mgr := worktree.NewManager(repo)
	wi := NewWorktreeIntegration(mgr)

	require.False(t, mgr.IsIsolated(), "fresh manager must not be isolated")
	require.Equal(t, repo, mgr.GetCurrentDirectory())

	task := SubagentTask{
		ID:          "no-mutate",
		Description: "no-mutate",
		Prompt:      "noop",
		Isolation:   IsolationWorktree,
	}

	_, cleanup, err := wi.Setup(context.Background(), task)
	require.NoError(t, err)
	defer func() { _ = cleanup() }()

	assert.False(t, mgr.IsIsolated(), "Setup must NOT mutate parent IsIsolated()")
	assert.Equal(t, repo, mgr.GetCurrentDirectory(),
		"Setup must NOT mutate parent GetCurrentDirectory()")
}

// fakeWorktreeProvider records calls for the unit test below.
type fakeWorktreeProvider struct {
	gotName       string
	gotBaseBranch string
	returnPath    string
	returnErr     error
	cleanupCalls  int
}

func (f *fakeWorktreeProvider) CreateWorktreeForSubagent(_ context.Context, name, baseBranch string) (string, func() error, error) {
	f.gotName = name
	f.gotBaseBranch = baseBranch
	if f.returnErr != nil {
		return "", nil, f.returnErr
	}
	return f.returnPath, func() error {
		f.cleanupCalls++
		return nil
	}, nil
}

func TestWorktreeIntegration_FakeProvider_RecordsCall(t *testing.T) {
	fake := &fakeWorktreeProvider{returnPath: "/tmp/fake/wt"}
	wi := NewWorktreeIntegration(fake)

	task := SubagentTask{
		ID:          "abc-123",
		Description: "fake-call",
		Prompt:      "x",
		Isolation:   IsolationWorktree,
		BaseBranch:  "develop",
	}

	path, cleanup, err := wi.Setup(context.Background(), task)
	require.NoError(t, err)
	assert.Equal(t, "/tmp/fake/wt", path)

	// Setup must build the worktree name from the task ID.
	assert.Equal(t, "helixcode-subagent-abc-123", fake.gotName)
	assert.Equal(t, "develop", fake.gotBaseBranch)

	require.NoError(t, cleanup())
	assert.Equal(t, 1, fake.cleanupCalls)
}

func TestWorktreeIntegration_NilProviderConstructor(t *testing.T) {
	// Documented contract: NewWorktreeIntegration(nil) returns nil.
	wi := NewWorktreeIntegration(nil)
	assert.Nil(t, wi, "NewWorktreeIntegration(nil) must return nil")
}
