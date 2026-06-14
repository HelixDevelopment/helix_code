package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dev.helix.code/internal/tools/filesystem"
)

// TestFSReadTool_Execute_DirectoryListing is the RED→GREEN guard for the
// operator-reported defect: fs_read called on a directory path returned a
// NEGATIVE error to the model — "error: is_directory: path is a directory".
// The fix makes fs_read on a directory return a readable directory listing
// (like a helpful "ls") instead of erroring, so the model gets a positive,
// usable result.
func TestFSReadTool_Execute_DirectoryListing(t *testing.T) {
	tmpDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmpDir, "alpha.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("write alpha.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "beta.go"), []byte("package x"), 0o644); err != nil {
		t.Fatalf("write beta.go: %v", err)
	}
	if err := os.Mkdir(filepath.Join(tmpDir, "subdir"), 0o755); err != nil {
		t.Fatalf("mkdir subdir: %v", err)
	}

	config := DefaultRegistryConfig()
	config.FileSystemConfig.WorkspaceRoot = tmpDir
	registry, err := NewToolRegistry(config)
	if err != nil {
		t.Fatalf("NewToolRegistry: %v", err)
	}
	defer registry.Close()

	tool := &FSReadTool{registry: registry}
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"path": tmpDir,
	})
	if err != nil {
		t.Fatalf("fs_read on a directory must NOT error; got NEGATIVE result: %v", err)
	}

	// The result must render as readable text via the tool loop's
	// stringifyResult (strings + fmt.Stringer). Reproduce that here.
	rendered := stringify(result)
	if strings.Contains(rendered, "is_directory") {
		t.Fatalf("rendered result must NOT contain the is_directory error; got:\n%s", rendered)
	}
	for _, want := range []string{"alpha.txt", "beta.go", "subdir"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered directory listing must contain entry %q; got:\n%s", want, rendered)
		}
	}
	if !strings.Contains(rendered, "subdir/") {
		t.Fatalf("rendered listing must mark the subdir with a trailing slash; got:\n%s", rendered)
	}
}

// TestFSReadTool_Execute_FileStillReads guards that fs_read on a normal FILE
// still returns the file content unchanged (no regression from the directory
// branch).
func TestFSReadTool_Execute_FileStillReads(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	body := "Hello, World!\nLine two.\n"
	if err := os.WriteFile(testFile, []byte(body), 0o644); err != nil {
		t.Fatalf("write test.txt: %v", err)
	}

	config := DefaultRegistryConfig()
	config.FileSystemConfig.WorkspaceRoot = tmpDir
	registry, err := NewToolRegistry(config)
	if err != nil {
		t.Fatalf("NewToolRegistry: %v", err)
	}
	defer registry.Close()

	tool := &FSReadTool{registry: registry}
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"path": testFile,
	})
	if err != nil {
		t.Fatalf("fs_read on a file must succeed: %v", err)
	}

	fc, ok := result.(*filesystem.FileContent)
	if !ok {
		t.Fatalf("fs_read on a file must return *filesystem.FileContent; got %T", result)
	}
	if !strings.Contains(string(fc.Content), "Hello, World!") {
		t.Fatalf("file content must be returned unchanged; got:\n%s", string(fc.Content))
	}
	rendered := stringify(result)
	if !strings.Contains(rendered, "Hello, World!") {
		t.Fatalf("rendered file result must contain the file body; got:\n%s", rendered)
	}
}

// stringify mirrors the agent tool-loop's stringifyResult for strings +
// fmt.Stringer values, so the tests assert on what the model actually sees.
func stringify(v interface{}) string {
	switch s := v.(type) {
	case string:
		return s
	case fmt.Stringer:
		return s.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}
