package tools

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"dev.helix.code/internal/hooks"
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

// fakeValidatingTool fails validation when params contain "bad_key".
// Used to verify that Validate is called synchronously at dispatch time,
// not deferred to inside the background goroutine.
type fakeValidatingTool struct {
	name string
}

func (f *fakeValidatingTool) Name() string        { return f.name }
func (f *fakeValidatingTool) Description() string { return "" }
func (f *fakeValidatingTool) Schema() ToolSchema  { return ToolSchema{Type: "object"} }
func (f *fakeValidatingTool) Category() ToolCategory {
	return CategoryFileSystem
}
func (f *fakeValidatingTool) Validate(params map[string]interface{}) error {
	if _, has := params["bad_key"]; has {
		return errors.New("bad_key is not allowed")
	}
	return nil
}
func (f *fakeValidatingTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	return "ok", nil
}

// TestRegistry_RunInBackgroundCallsValidateSynchronously confirms that a tool
// with invalid params is rejected at dispatch time (not inside the goroutine).
// Before the fix, Validate was not called on the background path, so bad params
// would reach the goroutine and fail with an opaque error (or succeed silently
// if the tool itself was lenient). After the fix, Execute returns a wrapped
// "parameter validation failed" error immediately and no task is queued.
func TestRegistry_RunInBackgroundCallsValidateSynchronously(t *testing.T) {
	r, _ := newRegistryWithBgMgr(t)
	tool := &fakeValidatingTool{name: "ValidatingTool"}
	r.Register(tool)

	_, err := r.Execute(context.Background(), "ValidatingTool", map[string]interface{}{
		"run_in_background": true,
		"bad_key":           "x",
	})
	require.Error(t, err, "should fail synchronously due to validation error")
	assert.Contains(t, err.Error(), "validation", "error should mention validation")
	assert.Contains(t, err.Error(), "bad_key", "error should surface the rejected key")

	// Confirm no task was created — the dispatch was aborted before StartTask.
	list := r.bgManager.ListTasks()
	assert.Empty(t, list, "no task must be queued when validation fails at dispatch")
}

// TestRegistry_RunInBackgroundHookBlocksPreventsDispatch confirms that a
// blocking before-hook prevents background task dispatch (spec §4.7). Before
// the fix, fireBefore was skipped on the background path, so a hook configured
// to block (e.g. user-confirmation on Bash) was silently bypassed when the
// caller added run_in_background:true. After the fix, the hook fires
// synchronously and Execute returns the blocker error immediately.
func TestRegistry_RunInBackgroundHookBlocksPreventsDispatch(t *testing.T) {
	r, _ := newRegistryWithBgMgr(t)
	tool := &fakePlainTool{name: "BlockedTool", finalResult: "never"}
	r.Register(tool)

	hm := hooks.NewManager()
	blockHook := hooks.NewHook("blocker", hooks.HookTypeBeforeToolCall,
		func(ctx context.Context, e *hooks.Event) error {
			return errors.New("hook rejected the call")
		})
	require.NoError(t, hm.Register(blockHook))
	r.SetHooksManager(hm)

	_, err := r.Execute(context.Background(), "BlockedTool", map[string]interface{}{
		"run_in_background": true,
	})
	require.Error(t, err, "blocking hook must prevent background dispatch")
	assert.Contains(t, err.Error(), "blocked", "error should mention the hook block")

	// Confirm no task was queued.
	list := r.bgManager.ListTasks()
	assert.Empty(t, list, "no task must be queued when a before-hook blocks at dispatch")
}
