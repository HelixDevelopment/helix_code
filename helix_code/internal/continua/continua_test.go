package continua

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompletionEngine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.go")
	os.WriteFile(path, []byte("package p\nfunc main() {\n\tprint\n"), 0644)

	e := NewCompletionEngine()
	result, err := e.Complete(context.Background(), path, 3, 6)
	require.NoError(t, err)
	assert.NotEmpty(t, result.Suggestion)
	assert.Equal(t, 3, result.Line)
}

func TestWorkspaceEditor_Open(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.go")
	os.WriteFile(path, []byte("line1\nline2\n"), 0644)

	e := NewWorkspaceEditor()
	result, err := e.Open(context.Background(), path)
	require.NoError(t, err)
	assert.Equal(t, 2, result.Lines)
}

func TestWorkspaceEditor_Edit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.go")
	os.WriteFile(path, []byte("old\n"), 0644)

	e := NewWorkspaceEditor()
	result, err := e.Edit(context.Background(), path, "new content\n")
	require.NoError(t, err)
	assert.Equal(t, 1, result.Lines)

	src, _ := os.ReadFile(path)
	assert.Contains(t, string(src), "new content")
}

func TestChatManager(t *testing.T) {
	cm := NewChatManager()
	ctx := context.Background()

	s := cm.CreateSession("test", "model-1")
	assert.NotEmpty(t, s.ID)

	err := cm.AddMessage(ctx, s.ID, "user", "hello")
	require.NoError(t, err)

	got, err := cm.GetSession(s.ID)
	require.NoError(t, err)
	assert.Len(t, got.Messages, 1)

	err = cm.SetModel(s.ID, "model-2")
	require.NoError(t, err)

	list := cm.ListSessions()
	assert.Len(t, list, 1)
}

func TestDiff(t *testing.T) {
	result := Diff("line1\nline2", "line1\nline3")
	assert.Equal(t, 1, result.Additions)
	assert.Equal(t, 1, result.Deletions)
	assert.Contains(t, result.Patch, "+ line3")
}

func TestSentinelErrors(t *testing.T) {
	assert.Error(t, ErrCompletionFailed)
	assert.Error(t, ErrEditorFailed)
	assert.Error(t, ErrChatFailed)
}
