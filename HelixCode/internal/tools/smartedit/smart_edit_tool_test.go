package smartedit

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/tools"
	"dev.helix.code/internal/tools/multiedit"
)

// Compile-time interface check.
func TestSmartEditTool_Compiles_AsTool(t *testing.T) {
	var _ tools.Tool = (*SmartEditTool)(nil)
}

// --- helpers ---

// realCommitter constructs a real multiedit-backed committer rooted at the
// given workspace dir. Tests use t.TempDir() to ensure isolation. The returned
// committer satisfies MultiEditCommitter and writes through to disk.
func realCommitter(t *testing.T, root string) MultiEditCommitter {
	t.Helper()
	cfg := multiedit.DefaultConfig()
	cfg.WorkspaceRoot = root
	cfg.BackupDir = filepath.Join(root, ".helix-backups")
	cfg.BackupEnabled = true
	cfg.AllowedPaths = nil
	cfg.DeniedPaths = nil
	cfg.RequirePreview = false

	mfe, err := multiedit.NewMultiFileEditor(multiedit.WithConfig(cfg))
	require.NoError(t, err)
	return NewMultieditCommitter(mfe)
}

func newTool(t *testing.T, workdir string) *SmartEditTool {
	t.Helper()
	c := realCommitter(t, workdir)
	return NewSmartEditTool(c, workdir)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(b)
}

// buildPrompt assembles a SEARCH/REPLACE prompt for one block under one path.
func buildPrompt(path, search, replace string) string {
	var b strings.Builder
	b.WriteString(path)
	b.WriteString("\n")
	b.WriteString(MarkerSearch)
	b.WriteString("\n")
	b.WriteString(search)
	if !strings.HasSuffix(search, "\n") {
		b.WriteString("\n")
	}
	b.WriteString(MarkerDivider)
	b.WriteString("\n")
	b.WriteString(replace)
	if !strings.HasSuffix(replace, "\n") {
		b.WriteString("\n")
	}
	b.WriteString(MarkerReplace)
	b.WriteString("\n")
	return b.String()
}

// --- basic Tool surface ---

func TestSmartEditTool_Name(t *testing.T) {
	tool := newTool(t, t.TempDir())
	assert.Equal(t, "smart_edit", tool.Name())
}

func TestSmartEditTool_Description_NonEmpty(t *testing.T) {
	tool := newTool(t, t.TempDir())
	assert.NotEmpty(t, tool.Description())
}

func TestSmartEditTool_Category(t *testing.T) {
	tool := newTool(t, t.TempDir())
	assert.Equal(t, tools.CategorySmartEdit, tool.Category())
}

func TestSmartEditTool_Schema_HasPromptField(t *testing.T) {
	tool := newTool(t, t.TempDir())
	schema := tool.Schema()
	assert.Equal(t, "object", schema.Type)
	assert.Contains(t, schema.Properties, "prompt")
	assert.Contains(t, schema.Required, "prompt")
}

func TestSmartEditTool_Validate_RequiresPrompt(t *testing.T) {
	tool := newTool(t, t.TempDir())
	err := tool.Validate(map[string]interface{}{})
	require.Error(t, err)
}

func TestSmartEditTool_Validate_RejectsBadTypes(t *testing.T) {
	tool := newTool(t, t.TempDir())
	cases := []map[string]interface{}{
		{"prompt": 42},
		{"prompt": "abc", "workdir": 7},
		{"prompt": "abc", "dry_run": "yes"},
	}
	for i, c := range cases {
		err := tool.Validate(c)
		require.Error(t, err, "case %d should fail", i)
	}
}

func TestSmartEditTool_Validate_AcceptsGoodArgs(t *testing.T) {
	tool := newTool(t, t.TempDir())
	err := tool.Validate(map[string]interface{}{
		"prompt":  "anything",
		"workdir": "/tmp",
		"dry_run": true,
	})
	require.NoError(t, err)
}

// --- Execute paths ---

func TestSmartEditTool_Execute_SingleFileEdit_Applied(t *testing.T) {
	dir := t.TempDir()
	tool := newTool(t, dir)
	target := filepath.Join(dir, "hello.go")
	writeFile(t, target, "package main\n\nfunc main() {\n\tprintln(\"old\")\n}\n")

	// SEARCH must match a line as it appears in the file (line-aligned),
	// because the parser appends \n to each captured line.
	prompt := buildPrompt("hello.go",
		"\tprintln(\"old\")",
		"\tprintln(\"new\")",
	)
	got, err := tool.Execute(context.Background(), map[string]interface{}{
		"prompt": prompt,
	})
	require.NoError(t, err)
	res, ok := got.(*SmartEditResult)
	require.True(t, ok, "expected *SmartEditResult, got %T", got)
	assert.True(t, res.Atomic, "expected Atomic=true; AtomicError=%q", res.AtomicError)
	assert.Equal(t, 1, res.AppliedCount)
	assert.Equal(t, 0, res.FailedCount)
	assert.NotEmpty(t, res.Diff)

	// Real disk reflects the change.
	got2 := readFile(t, target)
	assert.Contains(t, got2, "println(\"new\")")
	assert.NotContains(t, got2, "println(\"old\")")
}

func TestSmartEditTool_Execute_DryRun_DoesNotWriteToDisk(t *testing.T) {
	dir := t.TempDir()
	tool := newTool(t, dir)
	target := filepath.Join(dir, "f.go")
	original := "package main\n\nvar x = 1\n"
	writeFile(t, target, original)

	prompt := buildPrompt("f.go", "var x = 1", "var x = 2")
	got, err := tool.Execute(context.Background(), map[string]interface{}{
		"prompt":  prompt,
		"dry_run": true,
	})
	require.NoError(t, err)
	res := got.(*SmartEditResult)
	assert.True(t, res.Atomic)
	assert.NotEmpty(t, res.Diff)
	assert.Equal(t, 1, res.AppliedCount)

	// Disk MUST be unchanged.
	assert.Equal(t, original, readFile(t, target))
}

func TestSmartEditTool_Execute_SearchNotFound_AbortsBeforeCommit(t *testing.T) {
	dir := t.TempDir()
	tool := newTool(t, dir)
	target := filepath.Join(dir, "f.go")
	original := "package main\n\nvar x = 1\n"
	writeFile(t, target, original)

	prompt := buildPrompt("f.go", "this text is absent", "replacement")
	got, err := tool.Execute(context.Background(), map[string]interface{}{
		"prompt": prompt,
	})
	require.NoError(t, err)
	res := got.(*SmartEditResult)
	assert.False(t, res.Atomic)
	assert.Equal(t, 0, res.AppliedCount)
	assert.Equal(t, 1, res.FailedCount)
	assert.NotEmpty(t, res.AtomicError)
	require.Len(t, res.Results, 1)
	assert.Equal(t, OutcomeNotFound, res.Results[0].Outcome)

	// Disk unchanged.
	assert.Equal(t, original, readFile(t, target))
}

func TestSmartEditTool_Execute_AmbiguousMatch_AbortsBeforeCommit(t *testing.T) {
	dir := t.TempDir()
	tool := newTool(t, dir)
	target := filepath.Join(dir, "f.go")
	original := "dup\ndup\n"
	writeFile(t, target, original)

	prompt := buildPrompt("f.go", "dup", "uniq")
	got, err := tool.Execute(context.Background(), map[string]interface{}{
		"prompt": prompt,
	})
	require.NoError(t, err)
	res := got.(*SmartEditResult)
	assert.False(t, res.Atomic)
	assert.Equal(t, 1, res.FailedCount)
	require.Len(t, res.Results, 1)
	assert.Equal(t, OutcomeAmbiguous, res.Results[0].Outcome)

	// Disk unchanged.
	assert.Equal(t, original, readFile(t, target))
}

func TestSmartEditTool_Execute_BinaryFile_Refused(t *testing.T) {
	dir := t.TempDir()
	tool := newTool(t, dir)
	target := filepath.Join(dir, "blob.bin")
	binary := []byte{0x7f, 'E', 'L', 'F', 0x00, 0x01, 0x02, 0x03, 'a', 'b', 'c'}
	require.NoError(t, os.WriteFile(target, binary, 0o644))

	prompt := buildPrompt("blob.bin", "abc", "xyz")
	got, err := tool.Execute(context.Background(), map[string]interface{}{
		"prompt": prompt,
	})
	require.NoError(t, err)
	res := got.(*SmartEditResult)
	assert.False(t, res.Atomic)
	require.GreaterOrEqual(t, len(res.Results), 1)
	assert.Equal(t, OutcomeBinary, res.Results[0].Outcome)

	// Disk unchanged.
	got2, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, binary, got2)
}

func TestSmartEditTool_Execute_FileTooLarge_Refused(t *testing.T) {
	dir := t.TempDir()
	tool := newTool(t, dir)
	target := filepath.Join(dir, "big.txt")
	// MaxFileBytes = 10 MiB. Make it just larger.
	big := make([]byte, MaxFileBytes+1)
	for i := range big {
		big[i] = 'a'
	}
	require.NoError(t, os.WriteFile(target, big, 0o644))

	prompt := buildPrompt("big.txt", "aaa", "bbb")
	got, err := tool.Execute(context.Background(), map[string]interface{}{
		"prompt": prompt,
	})
	require.NoError(t, err)
	res := got.(*SmartEditResult)
	assert.False(t, res.Atomic)
	require.GreaterOrEqual(t, len(res.Results), 1)
	assert.Equal(t, OutcomeTooLarge, res.Results[0].Outcome)

	// Disk size unchanged (best-effort proof).
	info, err := os.Stat(target)
	require.NoError(t, err)
	assert.Equal(t, int64(MaxFileBytes+1), info.Size())
}

func TestSmartEditTool_Execute_MultiFileEdit_AllApplied(t *testing.T) {
	dir := t.TempDir()
	tool := newTool(t, dir)

	a := filepath.Join(dir, "a.go")
	b := filepath.Join(dir, "b.go")
	writeFile(t, a, "package a\nvar A = 1\n")
	writeFile(t, b, "package b\nvar B = 2\n")

	prompt := buildPrompt("a.go", "var A = 1", "var A = 11") +
		buildPrompt("b.go", "var B = 2", "var B = 22")
	got, err := tool.Execute(context.Background(), map[string]interface{}{
		"prompt": prompt,
	})
	require.NoError(t, err)
	res := got.(*SmartEditResult)
	assert.True(t, res.Atomic, "AtomicError=%q", res.AtomicError)
	assert.Equal(t, 2, res.AppliedCount)
	assert.Equal(t, 0, res.FailedCount)

	assert.Contains(t, readFile(t, a), "var A = 11")
	assert.Contains(t, readFile(t, b), "var B = 22")
}

func TestSmartEditTool_Execute_MultiFile_OneSearchNotFound_NoFilesWritten(t *testing.T) {
	dir := t.TempDir()
	tool := newTool(t, dir)

	a := filepath.Join(dir, "a.go")
	b := filepath.Join(dir, "b.go")
	origA := "package a\nvar A = 1\n"
	origB := "package b\nvar B = 2\n"
	writeFile(t, a, origA)
	writeFile(t, b, origB)

	// b.go's SEARCH does NOT exist — whole prompt aborts.
	prompt := buildPrompt("a.go", "var A = 1", "var A = 11") +
		buildPrompt("b.go", "this is not in b.go", "anything")
	got, err := tool.Execute(context.Background(), map[string]interface{}{
		"prompt": prompt,
	})
	require.NoError(t, err)
	res := got.(*SmartEditResult)
	assert.False(t, res.Atomic, "atomicity gate must fire")
	assert.Equal(t, 1, res.AppliedCount, "applier still records a.go's block as applied in-memory")
	assert.GreaterOrEqual(t, res.FailedCount, 1)

	// NEITHER file is changed on disk — that's the whole-prompt atomicity guarantee.
	assert.Equal(t, origA, readFile(t, a))
	assert.Equal(t, origB, readFile(t, b))
}

func TestSmartEditTool_Execute_PostWriteReRead_ReflectsActualDisk(t *testing.T) {
	dir := t.TempDir()
	tool := newTool(t, dir)
	target := filepath.Join(dir, "f.go")
	writeFile(t, target, "package main\nvar x = 1\n")

	prompt := buildPrompt("f.go", "var x = 1", "var x = 999")
	got, err := tool.Execute(context.Background(), map[string]interface{}{
		"prompt": prompt,
	})
	require.NoError(t, err)
	res := got.(*SmartEditResult)
	require.True(t, res.Atomic)

	// Re-read disk independently — the diff must reflect it.
	disk := readFile(t, target)
	assert.Contains(t, disk, "var x = 999")
	// The reported aggregate diff must mention the new value (post-write read).
	assert.Contains(t, res.Diff, "var x = 999")
}

func TestSmartEditTool_Execute_RelativePathsResolveAgainstWorkdir(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "src")
	require.NoError(t, os.MkdirAll(sub, 0o755))

	tool := newTool(t, dir)

	target := filepath.Join(sub, "rel.go")
	writeFile(t, target, "package src\nvar y = 0\n")

	// Use relative path "src/rel.go" — must resolve to sub/rel.go.
	prompt := buildPrompt("src/rel.go", "var y = 0", "var y = 1")
	got, err := tool.Execute(context.Background(), map[string]interface{}{
		"prompt": prompt,
	})
	require.NoError(t, err)
	res := got.(*SmartEditResult)
	require.True(t, res.Atomic, "atomic err: %s", res.AtomicError)
	assert.Contains(t, readFile(t, target), "var y = 1")
}

func TestSmartEditTool_Execute_AbsolutePathsHonored(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "abs.go")
	writeFile(t, target, "package main\nvar z = \"a\"\n")

	tool := newTool(t, dir)
	prompt := buildPrompt(target, "var z = \"a\"", "var z = \"b\"")
	got, err := tool.Execute(context.Background(), map[string]interface{}{
		"prompt": prompt,
	})
	require.NoError(t, err)
	res := got.(*SmartEditResult)
	require.True(t, res.Atomic, "atomic err: %s", res.AtomicError)
	assert.Contains(t, readFile(t, target), "var z = \"b\"")
}

func TestSmartEditTool_Execute_NonexistentFile_OutcomeReadFailed(t *testing.T) {
	dir := t.TempDir()
	tool := newTool(t, dir)

	prompt := buildPrompt("does-not-exist.go", "anything", "else")
	got, err := tool.Execute(context.Background(), map[string]interface{}{
		"prompt": prompt,
	})
	require.NoError(t, err)
	res := got.(*SmartEditResult)
	assert.False(t, res.Atomic)
	require.GreaterOrEqual(t, len(res.Results), 1)
	assert.Equal(t, OutcomeReadFailed, res.Results[0].Outcome)
}

func TestSmartEditTool_Execute_EmptyPrompt_NoOpSuccess(t *testing.T) {
	dir := t.TempDir()
	tool := newTool(t, dir)
	got, err := tool.Execute(context.Background(), map[string]interface{}{
		"prompt": "",
	})
	require.NoError(t, err)
	res := got.(*SmartEditResult)
	assert.Equal(t, 0, res.AppliedCount)
	assert.Equal(t, 0, res.FailedCount)
	assert.Empty(t, res.Results)
	// No work done — Atomic vacuously true; no AtomicError.
	assert.True(t, res.Atomic)
	assert.Empty(t, res.AtomicError)
}

// --- failing-committer behaviour ---

// fakeFailingCommitter always returns an error from CommitFiles, simulating a
// disk-level transaction failure (e.g. backup error, fsync error). The tool
// must surface AtomicError and leave files untouched.
type fakeFailingCommitter struct {
	called bool
	err    error
}

func (f *fakeFailingCommitter) CommitFiles(ctx context.Context, files map[string][]byte) error {
	f.called = true
	if f.err != nil {
		return f.err
	}
	return errors.New("synthetic commit failure")
}

func TestSmartEditTool_Execute_CommitFails_AtomicErrorPopulated(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "x.go")
	original := "package main\nvar a = 1\n"
	writeFile(t, target, original)

	failer := &fakeFailingCommitter{err: errors.New("backup fsync failed")}
	tool := NewSmartEditTool(failer, dir)

	prompt := buildPrompt("x.go", "var a = 1", "var a = 2")
	got, err := tool.Execute(context.Background(), map[string]interface{}{
		"prompt": prompt,
	})
	require.NoError(t, err)
	res := got.(*SmartEditResult)
	assert.False(t, res.Atomic)
	assert.Contains(t, res.AtomicError, "backup fsync failed")
	assert.True(t, failer.called)
	// Disk untouched (the committer never wrote anything).
	assert.Equal(t, original, readFile(t, target))
}
