package roocode

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskDelegator(t *testing.T) {
	d := NewTaskDelegator()
	ctx := context.Background()

	task, err := d.Delegate(ctx, "Test Task", "A test task", 1)
	require.NoError(t, err)
	assert.NotEmpty(t, task.ID)
	assert.Equal(t, "Test Task", task.Title)

	list := d.ListTasks()
	assert.Len(t, list, 1)

	got, err := d.GetTask(task.ID)
	require.NoError(t, err)
	assert.Equal(t, task.ID, got.ID)

	err = d.AssignTask(task.ID, "subagent-1")
	require.NoError(t, err)
	assert.Equal(t, "subagent-1", task.AssignedTo)
}

func TestCodeGenerator(t *testing.T) {
	dir := t.TempDir()
	gen := NewCodeGenerator(dir)
	ctx := context.Background()

	path, err := gen.Generate(ctx, GenerateSpec{Type: "go", Name: "myFunc", Template: "main"})
	require.NoError(t, err)
	assert.Contains(t, path, "myFunc.go")

	src, _ := os.ReadFile(path)
	assert.Contains(t, string(src), "func myFunc()")
}

func TestCodeGenerator_Bootstrap(t *testing.T) {
	dir := t.TempDir()
	gen := NewCodeGenerator(dir)
	ctx := context.Background()

	for _, lang := range []string{"go", "python", "node"} {
		files, err := gen.Bootstrap(ctx, BootstrapSpec{ProjectType: lang, Name: "testproj", OutputDir: filepath.Join(dir, lang)})
		require.NoError(t, err)
		assert.NotEmpty(t, files)
	}
}

func TestCodeReviewer(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.go")
	os.WriteFile(path, []byte("package p\n// TODO: implement\nfunc f() {}\n"), 0644)

	r := NewCodeReviewer()
	result, err := r.Review(context.Background(), path)
	require.NoError(t, err)
	assert.False(t, result.Approved)
	assert.NotEmpty(t, result.Issues)
}

func TestCodeReviewer_Clean(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "clean.go")
	os.WriteFile(path, []byte("package p\nfunc f() { return }\n"), 0644)

	r := NewCodeReviewer()
	result, err := r.Review(context.Background(), path)
	require.NoError(t, err)
	assert.True(t, result.Approved)
}

func TestConversationStore(t *testing.T) {
	cs := NewConversationStore()
	conv := cs.Create("test conversation")
	assert.NotEmpty(t, conv.ID)

	err := cs.AddMessage(conv.ID, "user", "hello")
	require.NoError(t, err)

	got, err := cs.Get(conv.ID)
	require.NoError(t, err)
	assert.Len(t, got.Messages, 1)
	assert.Equal(t, "user", got.Messages[0].Role)

	list := cs.List()
	assert.Len(t, list, 1)
}

func TestSentinelErrors(t *testing.T) {
	assert.Error(t, ErrTaskDelegationFailed)
	assert.Error(t, ErrGenerationFailed)
	assert.Error(t, ErrReviewFailed)
}
