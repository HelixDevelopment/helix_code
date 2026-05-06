// Package tools — registry_autocommit_test.go (P2-F22-T06).
//
// Exercises the registry's post-Execute fireAutoCommit hook + the
// per-tool derivePaths table. Uses real AutoCommitter against real git
// tempdir; asserts SHA differential before/after Execute.
package tools

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"dev.helix.code/internal/approval"
	"dev.helix.code/internal/autocommit"
)

// --- minimal fake tools for the path-derivation matrix ---

type acFakeEditTool struct {
	approval.DefaultLevelEdit
	name string
}

func (s *acFakeEditTool) Name() string                          { return s.name }
func (s *acFakeEditTool) Description() string                   { return "ac fake edit" }
func (s *acFakeEditTool) Schema() ToolSchema                    { return ToolSchema{Type: "object"} }
func (s *acFakeEditTool) Category() ToolCategory                { return CategoryFileSystem }
func (s *acFakeEditTool) Validate(map[string]interface{}) error { return nil }
func (s *acFakeEditTool) Execute(_ context.Context, params map[string]interface{}) (interface{}, error) {
	// Write the file so the working tree is dirty for auto-commit.
	if p, ok := params["path"].(string); ok && p != "" {
		_ = os.WriteFile(p, []byte("data"), 0644)
	}
	return map[string]interface{}{"ok": true}, nil
}

type acFakeReadOnlyTool struct {
	name string
}

func (s *acFakeReadOnlyTool) Name() string                          { return s.name }
func (s *acFakeReadOnlyTool) Description() string                   { return "ac fake read-only" }
func (s *acFakeReadOnlyTool) Schema() ToolSchema                    { return ToolSchema{Type: "object"} }
func (s *acFakeReadOnlyTool) Category() ToolCategory                { return CategoryFileSystem }
func (s *acFakeReadOnlyTool) Validate(map[string]interface{}) error { return nil }
func (s *acFakeReadOnlyTool) Execute(context.Context, map[string]interface{}) (interface{}, error) {
	return "read", nil
}
func (s *acFakeReadOnlyTool) RequiresApproval() approval.ApprovalLevel {
	return approval.LevelReadOnly
}

func setupRealRepo(t *testing.T) string {
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
	// Initial commit so HEAD exists.
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gitkeep"), []byte(""), 0644))
	for _, args := range [][]string{
		{"-C", dir, "add", ".gitkeep"},
		{"-C", dir, "commit", "-q", "-m", "init"},
	} {
		require.NoError(t, exec.Command("git", args...).Run())
	}
	return dir
}

func headSHA(t *testing.T, dir string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	require.NoError(t, err)
	return strings.TrimSpace(string(out))
}

func newRealRegistry(t *testing.T) *ToolRegistry {
	t.Helper()
	r, err := NewToolRegistry(DefaultRegistryConfig())
	require.NoError(t, err)
	return r
}

func TestRegistry_FireAutoCommit_NilCommitter_NoOp(t *testing.T) {
	reg := newRealRegistry(t)
	// SetAutoCommitter never called → autoCommitter is nil.
	require.NotPanics(t, func() {
		reg.fireAutoCommit(context.Background(), "fs_write",
			map[string]interface{}{"path": "x.txt"},
			&acFakeEditTool{name: "fs_write"}, nil)
	})
}

func TestRegistry_FireAutoCommit_EditLevel_Commits(t *testing.T) {
	dir := setupRealRepo(t)
	initial := headSHA(t, dir)

	reg := newRealRegistry(t)
	committer := autocommit.NewAutoCommitter(autocommit.Options{
		Enabled: true, WorkingDir: dir, Logger: zap.NewNop(),
	})
	reg.SetAutoCommitter(committer)

	target := filepath.Join(dir, "x.txt")
	require.NoError(t, os.WriteFile(target, []byte("hi"), 0644))

	reg.fireAutoCommit(context.Background(), "fs_write",
		map[string]interface{}{"path": "x.txt"},
		&acFakeEditTool{name: "fs_write"}, nil)

	require.NotEqual(t, initial, headSHA(t, dir))
}

func TestRegistry_FireAutoCommit_NonEditLevel_NoCommit(t *testing.T) {
	dir := setupRealRepo(t)
	initial := headSHA(t, dir)

	reg := newRealRegistry(t)
	committer := autocommit.NewAutoCommitter(autocommit.Options{
		Enabled: true, WorkingDir: dir, Logger: zap.NewNop(),
	})
	reg.SetAutoCommitter(committer)

	// Read-only tool should NOT trigger a commit even if working tree
	// is dirty.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "x.txt"), []byte("hi"), 0644))

	reg.fireAutoCommit(context.Background(), "read_file",
		map[string]interface{}{"path": "x.txt"},
		&acFakeReadOnlyTool{name: "read_file"}, nil)

	require.Equal(t, initial, headSHA(t, dir))
}

func TestRegistry_FireAutoCommit_SkipParam_NoCommit(t *testing.T) {
	dir := setupRealRepo(t)
	initial := headSHA(t, dir)

	reg := newRealRegistry(t)
	committer := autocommit.NewAutoCommitter(autocommit.Options{
		Enabled: true, WorkingDir: dir, Logger: zap.NewNop(),
	})
	reg.SetAutoCommitter(committer)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "x.txt"), []byte("hi"), 0644))

	reg.fireAutoCommit(context.Background(), "fs_write",
		map[string]interface{}{
			"path":                       "x.txt",
			autocommit.SkipParamKey:     true,
		},
		&acFakeEditTool{name: "fs_write"}, nil)

	require.Equal(t, initial, headSHA(t, dir))
}

func TestRegistry_SetAutoCommitter_NilDisablesHook(t *testing.T) {
	dir := setupRealRepo(t)
	initial := headSHA(t, dir)

	reg := newRealRegistry(t)
	committer := autocommit.NewAutoCommitter(autocommit.Options{
		Enabled: true, WorkingDir: dir, Logger: zap.NewNop(),
	})
	reg.SetAutoCommitter(committer)
	// Now disable.
	reg.SetAutoCommitter(nil)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "x.txt"), []byte("hi"), 0644))

	reg.fireAutoCommit(context.Background(), "fs_write",
		map[string]interface{}{"path": "x.txt"},
		&acFakeEditTool{name: "fs_write"}, nil)

	require.Equal(t, initial, headSHA(t, dir))
}

func TestDerivePaths_TableDriven(t *testing.T) {
	cases := []struct {
		name   string
		tool   string
		params map[string]interface{}
		want   []string
	}{
		{"fs_write_single_path", "fs_write", map[string]interface{}{"path": "a.go"}, []string{"a.go"}},
		{"fs_edit_single_path", "fs_edit", map[string]interface{}{"path": "a.go"}, []string{"a.go"}},
		{"smart_edit_single_path", "smart_edit", map[string]interface{}{"path": "a.go"}, []string{"a.go"}},
		{"notebook_edit_single_path", "notebook_edit", map[string]interface{}{"path": "a.ipynb"}, []string{"a.ipynb"}},
		{"write_file_single_path", "write_file", map[string]interface{}{"path": "a.go"}, []string{"a.go"}},
		{"multiedit_dedup", "multiedit_commit", map[string]interface{}{
			"edits": []interface{}{
				map[string]interface{}{"path": "a.go"},
				map[string]interface{}{"path": "b.go"},
				map[string]interface{}{"path": "a.go"}, // dup
			},
		}, []string{"a.go", "b.go"}},
		{"mapping_edit_target_file", "mapping_edit", map[string]interface{}{"target_file": "x.go"}, []string{"x.go"}},
		{"unknown_tool_fallthrough", "weird_tool", map[string]interface{}{}, nil},
		{"fs_write_empty_path", "fs_write", map[string]interface{}{"path": ""}, nil},
		{"multiedit_no_edits", "multiedit_commit", map[string]interface{}{}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := derivePaths(tc.tool, tc.params)
			require.Equal(t, tc.want, got)
		})
	}
}
