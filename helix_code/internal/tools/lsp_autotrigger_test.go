package tools_test

// Tests for the post-Execute LSP auto-trigger wired by ToolRegistry.SetLSPManager.
//
// The auto-trigger fires NotifyChange (or implicit-soft-open via NotifyChange)
// against the registered LSPManager AFTER an Edit-class tool (fs_edit, fs_write,
// multiedit_commit) successfully runs. We verify the trigger by registering a
// real LSPManager backed by the in-tree fake LSP server (built in TestMain) and
// asserting that GetDiagnostics returns the published diagnostics for the file
// edited by the tool.
//
// Tests reuse the fakeSpec / writeTempFile / waitForDiagnostics helpers from
// lsp_manager_test.go (same package: tools_test).

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	"dev.helix.code/internal/approval"
	"dev.helix.code/internal/tools"
)

// stubEditTool is a registry-pluggable Tool that simulates an Edit-class tool
// by writing the requested content to disk under the path given in params.
// It does NOT invoke the LSP manager itself — that is the auto-trigger's job.
//
// The tool name is configurable so we can register the same impl as fs_edit,
// fs_write, or multiedit_commit interchangeably across the tests.
type stubEditTool struct {
	approval.DefaultLevelEdit
	name string

	// executeCalled bumps each Execute so a test can assert
	// the underlying tool actually ran.
	executeCalled int

	// failOnExecute, when true, makes Execute return an error without
	// writing the file, so we can verify the auto-trigger does not
	// fire on failure.
	failOnExecute bool
}

func (s *stubEditTool) Name() string                          { return s.name }
func (s *stubEditTool) Description() string                   { return "stub edit tool for autotrigger tests" }
func (s *stubEditTool) Schema() tools.ToolSchema              { return tools.ToolSchema{Type: "object"} }
func (s *stubEditTool) Category() tools.ToolCategory          { return tools.CategoryFileSystem }
func (s *stubEditTool) Validate(map[string]interface{}) error { return nil }

func (s *stubEditTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	s.executeCalled++
	if s.failOnExecute {
		return nil, errors.New("stub edit failed")
	}
	path, _ := params["path"].(string)
	content, _ := params["content"].(string)
	if path == "" {
		return nil, nil
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return nil, err
	}
	return "ok", nil
}

// newRegistryWithFakeLSP builds a real ToolRegistry rooted at a per-test
// workspace dir, attaches a real LSPManager driving the in-tree fake LSP
// subprocess, and registers cleanup. Returns the registry, the manager, and
// the workspace dir so tests can write files under it (the filesystem tools
// reject writes outside WorkspaceRoot).
func newRegistryWithFakeLSP(t *testing.T) (*tools.ToolRegistry, *tools.LSPManager, string) {
	t.Helper()
	workspace := t.TempDir()
	cfg := tools.DefaultRegistryConfig()
	cfg.FileSystemConfig.WorkspaceRoot = workspace
	cfg.MappingWorkspace = workspace
	r, err := tools.NewToolRegistry(cfg)
	if err != nil {
		t.Fatalf("NewToolRegistry: %v", err)
	}
	m := tools.NewLSPManager(workspace, []tools.LSPServerSpec{fakeSpec()}, zap.NewNop())
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = m.Shutdown(ctx)
	})
	return r, m, workspace
}

// ---------- SetLSPManager ----------

func TestRegistry_SetLSPManager_NilIsDisabled(t *testing.T) {
	// No SetLSPManager call → Execute must not panic and must not touch any
	// LSP machinery.
	r, err := tools.NewToolRegistry(tools.DefaultRegistryConfig())
	if err != nil {
		t.Fatalf("NewToolRegistry: %v", err)
	}

	stub := &stubEditTool{name: "fs_edit"}
	r.Register(stub)

	dir := t.TempDir()
	path := filepath.Join(dir, "x.fake")
	_, err = r.Execute(context.Background(), "fs_edit", map[string]interface{}{
		"path":    path,
		"content": "// @fake-error: should-not-be-published\n",
	})
	if err != nil {
		t.Fatalf("Execute (no LSP): %v", err)
	}
	if stub.executeCalled != 1 {
		t.Errorf("stub.executeCalled = %d, want 1", stub.executeCalled)
	}
}

func TestRegistry_SetLSPManager_AcceptsNil(t *testing.T) {
	// Calling SetLSPManager(nil) explicitly must be a valid disable operation.
	r, err := tools.NewToolRegistry(tools.DefaultRegistryConfig())
	if err != nil {
		t.Fatalf("NewToolRegistry: %v", err)
	}
	r.SetLSPManager(nil) // must not panic

	stub := &stubEditTool{name: "fs_edit"}
	r.Register(stub)
	dir := t.TempDir()
	path := filepath.Join(dir, "x.fake")
	if _, err := r.Execute(context.Background(), "fs_edit", map[string]interface{}{
		"path":    path,
		"content": "// @fake-error: nope\n",
	}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
}

// ---------- Auto-trigger: positive cases ----------

func TestRegistry_AutoTriggerOnFSEdit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	r, m, workspace := newRegistryWithFakeLSP(t)
	r.SetLSPManager(m)

	stub := &stubEditTool{name: "fs_edit"}
	r.Register(stub)

	path := filepath.Join(workspace, "doc.fake")
	if _, err := r.Execute(ctx, "fs_edit", map[string]interface{}{
		"path":    path,
		"content": "// @fake-error: triggered-by-fs-edit\n",
	}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if stub.executeCalled != 1 {
		t.Fatalf("stub.executeCalled = %d, want 1", stub.executeCalled)
	}

	got := waitForDiagnostics(m, path, func(n int) bool { return n >= 1 }, 5*time.Second)
	if len(got) != 1 {
		t.Fatalf("auto-trigger fs_edit: want 1 diagnostic for %s, got %d", path, len(got))
	}
	if !strings.Contains(got[0].Message, "triggered-by-fs-edit") {
		t.Errorf("diagnostic message: got %q want substring 'triggered-by-fs-edit'", got[0].Message)
	}
}

func TestRegistry_AutoTriggerOnFSWrite(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	r, m, workspace := newRegistryWithFakeLSP(t)
	r.SetLSPManager(m)

	stub := &stubEditTool{name: "fs_write"}
	r.Register(stub)

	path := filepath.Join(workspace, "doc.fake")
	if _, err := r.Execute(ctx, "fs_write", map[string]interface{}{
		"path":    path,
		"content": "// @fake-error: triggered-by-fs-write\n",
	}); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	got := waitForDiagnostics(m, path, func(n int) bool { return n >= 1 }, 5*time.Second)
	if len(got) != 1 {
		t.Fatalf("auto-trigger fs_write: want 1 diagnostic, got %d", len(got))
	}
	if !strings.Contains(got[0].Message, "triggered-by-fs-write") {
		t.Errorf("diagnostic message: got %q want substring 'triggered-by-fs-write'", got[0].Message)
	}
}

func TestRegistry_AutoTriggerOnMultiEditCommit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	r, m, workspace := newRegistryWithFakeLSP(t)
	r.SetLSPManager(m)

	path := filepath.Join(workspace, "doc.fake")

	// Drive the multi-edit lifecycle through the registered tools so we
	// hit the same code paths the agent would.
	beginRes, err := r.Execute(ctx, "multiedit_begin", map[string]interface{}{
		"description": "autotrigger test",
	})
	if err != nil {
		t.Fatalf("Execute multiedit_begin: %v", err)
	}
	txID := transactionIDFromResult(t, beginRes)
	if txID == "" {
		t.Fatalf("could not extract transaction ID from %T = %+v", beginRes, beginRes)
	}

	if _, err := r.Execute(ctx, "multiedit_add", map[string]interface{}{
		"transaction_id": txID,
		"file_path":      path,
		"operation":      "create",
		"new_content":    "// @fake-error: triggered-by-multiedit-commit\n",
	}); err != nil {
		t.Fatalf("Execute multiedit_add: %v", err)
	}

	// Preview is required before commit (transitions tx to StateReady).
	if _, err := r.Execute(ctx, "multiedit_preview", map[string]interface{}{
		"transaction_id": txID,
	}); err != nil {
		t.Fatalf("Execute multiedit_preview: %v", err)
	}

	if _, err := r.Execute(ctx, "multiedit_commit", map[string]interface{}{
		"transaction_id": txID,
	}); err != nil {
		t.Fatalf("Execute multiedit_commit: %v", err)
	}

	got := waitForDiagnostics(m, path, func(n int) bool { return n >= 1 }, 5*time.Second)
	if len(got) != 1 {
		t.Fatalf("auto-trigger multiedit_commit: want 1 diagnostic, got %d", len(got))
	}
	if !strings.Contains(got[0].Message, "triggered-by-multiedit-commit") {
		t.Errorf("diagnostic message: got %q want substring 'triggered-by-multiedit-commit'", got[0].Message)
	}
}

// ---------- Auto-trigger: negative cases ----------

func TestRegistry_AutoTriggerSkipsNonEditTools(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	r, m, workspace := newRegistryWithFakeLSP(t)
	r.SetLSPManager(m)

	// Register a stub web_fetch that writes a .fake file with a fake-error
	// pragma — but because the tool is not in the Edit allowlist, no
	// auto-trigger should fire and no diagnostics should appear.
	stub := &stubEditTool{name: "test_web_fetch"}
	r.Register(stub)

	path := filepath.Join(workspace, "doc.fake")
	if _, err := r.Execute(ctx, "test_web_fetch", map[string]interface{}{
		"path":    path,
		"content": "// @fake-error: should-not-trigger\n",
	}); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// Wait briefly for any spurious publish; expect zero.
	got := waitForDiagnostics(m, path, func(n int) bool { return n >= 1 }, 500*time.Millisecond)
	if len(got) != 0 {
		t.Errorf("non-edit tool must not auto-trigger LSP; got %d diagnostics: %+v", len(got), got)
	}
}

func TestRegistry_AutoTriggerSkipsOnExecuteError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	r, m, workspace := newRegistryWithFakeLSP(t)
	r.SetLSPManager(m)

	// fs_edit stub that fails: the file is never written, and the
	// auto-trigger must not fire.
	stub := &stubEditTool{name: "fs_edit", failOnExecute: true}
	r.Register(stub)

	path := filepath.Join(workspace, "doc.fake")
	_, err := r.Execute(ctx, "fs_edit", map[string]interface{}{
		"path":    path,
		"content": "// @fake-error: never-written\n",
	})
	if err == nil {
		t.Fatalf("Execute: expected stub error, got nil")
	}

	got := waitForDiagnostics(m, path, func(n int) bool { return n >= 1 }, 500*time.Millisecond)
	if len(got) != 0 {
		t.Errorf("failed Execute must not auto-trigger LSP; got %d diagnostics", len(got))
	}
}

// ---------- ExtractEditedPathsForTest unit tests ----------

func TestExtractEditedPaths_FSEdit(t *testing.T) {
	got := tools.ExtractEditedPathsForTest("fs_edit", map[string]interface{}{"path": "/x"})
	if len(got) != 1 || got[0] != "/x" {
		t.Errorf("fs_edit: got %v want [/x]", got)
	}
}

func TestExtractEditedPaths_FSWrite(t *testing.T) {
	got := tools.ExtractEditedPathsForTest("fs_write", map[string]interface{}{"path": "/y"})
	if len(got) != 1 || got[0] != "/y" {
		t.Errorf("fs_write: got %v want [/y]", got)
	}
}

func TestExtractEditedPaths_UnknownTool(t *testing.T) {
	got := tools.ExtractEditedPathsForTest("nope", map[string]interface{}{"path": "/x"})
	if len(got) != 0 {
		t.Errorf("unknown tool: got %v want []", got)
	}
}

func TestExtractEditedPaths_FSEditMissingPath(t *testing.T) {
	got := tools.ExtractEditedPathsForTest("fs_edit", map[string]interface{}{})
	if len(got) != 0 {
		t.Errorf("fs_edit no path: got %v want []", got)
	}
}

func TestExtractEditedPaths_FSEditWrongPathType(t *testing.T) {
	got := tools.ExtractEditedPathsForTest("fs_edit", map[string]interface{}{"path": 42})
	if len(got) != 0 {
		t.Errorf("fs_edit non-string path: got %v want []", got)
	}
}

func TestExtractEditedPaths_MultiEditCommitNoPathInArgs(t *testing.T) {
	// multiedit_commit args only carry transaction_id; the resolver returns
	// no paths from the args alone (registry-level resolver looks them up
	// from the transaction).
	got := tools.ExtractEditedPathsForTest("multiedit_commit", map[string]interface{}{"transaction_id": "abc"})
	if len(got) != 0 {
		t.Errorf("multiedit_commit args-only: got %v want [] (paths resolve via transaction)", got)
	}
}

// ---------- helpers ----------

// transactionIDFromResult extracts the transaction_id from a multiedit_begin
// result. The result is *multiedit.EditTransaction — we read its exported ID
// field via reflection so we don't have to import the multiedit package here.
func transactionIDFromResult(t *testing.T, res interface{}) string {
	t.Helper()
	if res == nil {
		t.Fatalf("transactionIDFromResult: nil result")
	}
	v := reflect.ValueOf(res)
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			t.Fatalf("transactionIDFromResult: nil pointer")
		}
		v = v.Elem()
	}
	if v.Kind() == reflect.Struct {
		f := v.FieldByName("ID")
		if f.IsValid() && f.Kind() == reflect.String {
			return f.String()
		}
	}
	if m, ok := res.(map[string]interface{}); ok {
		if id, ok := m["transaction_id"].(string); ok {
			return id
		}
		if id, ok := m["id"].(string); ok {
			return id
		}
	}
	t.Fatalf("transactionIDFromResult: unrecognised result shape %T = %+v", res, res)
	return ""
}
