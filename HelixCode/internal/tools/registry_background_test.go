package tools

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"dev.helix.code/internal/workflow"
)

type fakeStreamingTool struct {
	name        string
	streamLines []string
	finalResult any
	gotParams   map[string]interface{}
}

func (f *fakeStreamingTool) Name() string        { return f.name }
func (f *fakeStreamingTool) Description() string { return "" }
func (f *fakeStreamingTool) Schema() ToolSchema  { return ToolSchema{Type: "object"} }
func (f *fakeStreamingTool) Category() ToolCategory {
	return CategoryFileSystem
}
func (f *fakeStreamingTool) Validate(params map[string]interface{}) error { return nil }
func (f *fakeStreamingTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	f.gotParams = params
	return f.finalResult, nil
}
func (f *fakeStreamingTool) ExecuteWithProgress(ctx context.Context, params map[string]interface{}, sink LineSink) (interface{}, error) {
	f.gotParams = params
	for _, l := range f.streamLines {
		sink(l)
	}
	return f.finalResult, nil
}

type fakePlainTool struct {
	name        string
	finalResult any
	gotParams   map[string]interface{}
}

func (f *fakePlainTool) Name() string        { return f.name }
func (f *fakePlainTool) Description() string { return "" }
func (f *fakePlainTool) Schema() ToolSchema  { return ToolSchema{Type: "object"} }
func (f *fakePlainTool) Category() ToolCategory {
	return CategoryFileSystem
}
func (f *fakePlainTool) Validate(params map[string]interface{}) error { return nil }
func (f *fakePlainTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	f.gotParams = params
	return f.finalResult, nil
}

func newRegistryWithBgMgr(t *testing.T) (*ToolRegistry, *workflow.BackgroundManager) {
	t.Helper()
	r, err := NewToolRegistry(DefaultRegistryConfig())
	require.NoError(t, err)
	bm := workflow.NewBackgroundManager(zap.NewNop(), workflow.ManagerConfig{})
	t.Cleanup(func() { _ = bm.Close() })
	r.SetBackgroundManager(bm)
	return r, bm
}

func TestRegistry_RunInBackgroundFlagDispatchesToManager(t *testing.T) {
	r, bm := newRegistryWithBgMgr(t)
	tool := &fakeStreamingTool{name: "Streamer", streamLines: []string{"x", "y"}, finalResult: "ok"}
	r.Register(tool)
	res, err := r.Execute(context.Background(), "Streamer", map[string]interface{}{
		"run_in_background": true,
		"key":               "value",
	})
	require.NoError(t, err)
	m, ok := res.(map[string]interface{})
	require.True(t, ok)
	taskID, ok := m["task_id"].(string)
	require.True(t, ok)
	require.NotEmpty(t, taskID)

	require.Eventually(t, func() bool {
		task, err := bm.GetTask(taskID)
		return err == nil && task.State() == workflow.TaskCompleted
	}, 2*time.Second, 10*time.Millisecond)

	task, err := bm.GetTask(taskID)
	require.NoError(t, err)
	assert.Equal(t, []string{"x", "y"}, task.LastLines(10))
	// Param flag stripped
	assert.NotContains(t, tool.gotParams, "run_in_background")
	assert.Equal(t, "value", tool.gotParams["key"])
}

func TestRegistry_RunInBackgroundWithoutManagerErrors(t *testing.T) {
	r, err := NewToolRegistry(DefaultRegistryConfig())
	require.NoError(t, err)
	tool := &fakeStreamingTool{name: "Streamer"}
	r.Register(tool)
	_, err = r.Execute(context.Background(), "Streamer", map[string]interface{}{
		"run_in_background": true,
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNoBackgroundMgr))
}

func TestRegistry_NonBackgroundAwareUsesFallback(t *testing.T) {
	r, bm := newRegistryWithBgMgr(t)
	tool := &fakePlainTool{name: "Plain", finalResult: "the-result"}
	r.Register(tool)
	res, err := r.Execute(context.Background(), "Plain", map[string]interface{}{
		"run_in_background": true,
	})
	require.NoError(t, err)
	m := res.(map[string]interface{})
	taskID := m["task_id"].(string)
	require.Eventually(t, func() bool {
		task, err := bm.GetTask(taskID)
		return err == nil && task.State() == workflow.TaskCompleted
	}, 2*time.Second, 10*time.Millisecond)
	task, _ := bm.GetTask(taskID)
	last := task.LastLines(1)
	require.Len(t, last, 1)
	assert.Contains(t, last[0], "the-result")
}

func TestRegistry_BackgroundFlagFalseTakesForegroundPath(t *testing.T) {
	r, _ := newRegistryWithBgMgr(t)
	tool := &fakePlainTool{name: "Plain", finalResult: "fg"}
	r.Register(tool)
	res, err := r.Execute(context.Background(), "Plain", map[string]interface{}{
		"run_in_background": false,
	})
	require.NoError(t, err)
	assert.Equal(t, "fg", res)
}
