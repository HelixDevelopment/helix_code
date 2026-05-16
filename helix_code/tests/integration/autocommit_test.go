//go:build integration

package integration

// autocommit_test.go (P2-F22-T08): end-to-end integration tests for the
// F22 git auto-commit hook wired into the real ToolRegistry.
//
// Each test exercises the production path:
//   real ToolRegistry → fireAutoCommit hook → real *AutoCommitter →
//   real Git wrapper → real `git` subprocess → real commit.
//
// Anti-bluff anchor: every PASS demonstrates a real commit (or its
// real absence) by reading `git log` SHA differential and `git status
// --porcelain`. There are no metadata-only assertions.

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"dev.helix.code/internal/approval"
	"dev.helix.code/internal/autocommit"
	"dev.helix.code/internal/tools"
)

// acStubEditTool is a level-Edit stub that writes a file at params["path"]
// when Execute fires. The test asserts the auto-commit hook then commits
// the file.
type acStubEditTool struct {
	name     string
	executed int32
}

func (s *acStubEditTool) Name() string                                    { return s.name }
func (s *acStubEditTool) Description() string                             { return "ac stub edit" }
func (s *acStubEditTool) Schema() tools.ToolSchema                        { return tools.ToolSchema{Type: "object"} }
func (s *acStubEditTool) Category() tools.ToolCategory                    { return tools.ToolCategory("test-stub") }
func (s *acStubEditTool) Validate(_ map[string]interface{}) error         { return nil }
func (s *acStubEditTool) RequiresApproval() approval.ApprovalLevel        { return approval.LevelEdit }
func (s *acStubEditTool) Execute(_ context.Context, p map[string]interface{}) (interface{}, error) {
	atomic.AddInt32(&s.executed, 1)
	if path, ok := p["path"].(string); ok {
		_ = os.WriteFile(path, []byte("hello\n"), 0644)
	}
	return "ok", nil
}

func setupAutocommitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "test@helixcode.dev"},
		{"config", "user.name", "Test"},
		{"config", "commit.gpgsign", "false"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		require.NoError(t, cmd.Run())
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gitkeep"), []byte(""), 0644))
	for _, args := range [][]string{
		{"-C", dir, "add", ".gitkeep"},
		{"-C", dir, "commit", "-q", "-m", "init"},
	} {
		require.NoError(t, exec.Command("git", args...).Run())
	}
	return dir
}

func acHeadSHA(t *testing.T, dir string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	require.NoError(t, err)
	return strings.TrimSpace(string(out))
}

func acPorcelain(t *testing.T, dir string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", dir, "status", "--porcelain").Output()
	require.NoError(t, err)
	return string(out)
}

func acCommitBody(t *testing.T, dir string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", dir, "log", "-1", "--format=%B").Output()
	require.NoError(t, err)
	return string(out)
}

// buildAcRegistry constructs a real registry + AutoCommitter wired in,
// no approval manager (so all tools run without F21 gate). Returns the
// registry, committer, and the registered stub tool.
func buildAcRegistry(t *testing.T, dir string, enabled bool) (*tools.ToolRegistry, *autocommit.AutoCommitter, *acStubEditTool) {
	t.Helper()
	reg, err := tools.NewToolRegistry(tools.DefaultRegistryConfig())
	require.NoError(t, err)
	t.Cleanup(func() { _ = reg.Close() })

	stub := &acStubEditTool{name: "ac_test_edit"}
	reg.Register(stub)

	committer := autocommit.NewAutoCommitter(autocommit.Options{
		Enabled: enabled, WorkingDir: dir, Logger: zap.NewNop(),
	})
	reg.SetAutoCommitter(committer)
	return reg, committer, stub
}

func TestAutoCommit_Integration_DefaultOn_RealEdit_RealCommit(t *testing.T) {
	dir := setupAutocommitRepo(t)
	initial := acHeadSHA(t, dir)
	reg, _, stub := buildAcRegistry(t, dir, true)

	target := filepath.Join(dir, "x.txt")
	_, err := reg.Execute(context.Background(), stub.Name(),
		map[string]interface{}{"path": target})
	require.NoError(t, err)
	require.NotEqual(t, initial, acHeadSHA(t, dir))
	require.Contains(t, acCommitBody(t, dir),
		"Co-Authored-By: HelixCode <noreply@helixcode.dev>")
	require.Empty(t, strings.TrimSpace(acPorcelain(t, dir)))
}

func TestAutoCommit_Integration_Disabled_NoCommit(t *testing.T) {
	dir := setupAutocommitRepo(t)
	initial := acHeadSHA(t, dir)
	reg, _, stub := buildAcRegistry(t, dir, false)

	target := filepath.Join(dir, "x.txt")
	_, err := reg.Execute(context.Background(), stub.Name(),
		map[string]interface{}{"path": target})
	require.NoError(t, err)
	require.Equal(t, initial, acHeadSHA(t, dir))
	require.Contains(t, acPorcelain(t, dir), "x.txt")
}

func TestAutoCommit_Integration_RuntimeToggle(t *testing.T) {
	dir := setupAutocommitRepo(t)
	initial := acHeadSHA(t, dir)
	reg, c, stub := buildAcRegistry(t, dir, false)

	// First call: off → no commit.
	target1 := filepath.Join(dir, "a.txt")
	_, err := reg.Execute(context.Background(), stub.Name(),
		map[string]interface{}{"path": target1})
	require.NoError(t, err)
	require.Equal(t, initial, acHeadSHA(t, dir))

	// Manually commit so a.txt doesn't pollute the second call's diff.
	require.NoError(t, exec.Command("git", "-C", dir, "add", "a.txt").Run())
	require.NoError(t, exec.Command("git", "-C", dir, "commit", "-q", "-m", "manual").Run())
	mid := acHeadSHA(t, dir)

	// Toggle on; next call SHOULD commit.
	c.SetEnabled(true)
	target2 := filepath.Join(dir, "b.txt")
	_, err = reg.Execute(context.Background(), stub.Name(),
		map[string]interface{}{"path": target2})
	require.NoError(t, err)
	require.NotEqual(t, mid, acHeadSHA(t, dir))
}

func TestAutoCommit_Integration_PerEditSkip_HonouredViaParam(t *testing.T) {
	dir := setupAutocommitRepo(t)
	initial := acHeadSHA(t, dir)
	reg, _, stub := buildAcRegistry(t, dir, true)

	target := filepath.Join(dir, "x.txt")
	_, err := reg.Execute(context.Background(), stub.Name(), map[string]interface{}{
		"path":                  target,
		autocommit.SkipParamKey: true,
	})
	require.NoError(t, err)
	require.Equal(t, initial, acHeadSHA(t, dir))
	require.Contains(t, acPorcelain(t, dir), "x.txt")
}

func TestAutoCommit_Integration_NotAGitRepo_NoOp(t *testing.T) {
	dir := t.TempDir() // NOT a git repo
	reg, _, stub := buildAcRegistry(t, dir, true)
	target := filepath.Join(dir, "x.txt")
	_, err := reg.Execute(context.Background(), stub.Name(),
		map[string]interface{}{"path": target})
	require.NoError(t, err)
	// Tool succeeded; auto-commit silently skipped because not a git repo.
	_, statErr := os.Stat(target)
	require.NoError(t, statErr)
}
