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

func newTaskToolsManager(t *testing.T) *workflow.BackgroundManager {
	t.Helper()
	bm := workflow.NewBackgroundManager(zap.NewNop(), workflow.ManagerConfig{})
	t.Cleanup(func() { _ = bm.Close() })
	return bm
}

func startSimpleTask(t *testing.T, bm *workflow.BackgroundManager, lines []string) string {
	t.Helper()
	exec := func(ctx context.Context, args map[string]interface{}, sink workflow.LineSink) (interface{}, error) {
		for _, l := range lines {
			sink(l)
		}
		return "ok", nil
	}
	task, err := bm.StartTask("FakeTool", nil, exec)
	require.NoError(t, err)
	require.Eventually(t, func() bool { return task.State() == workflow.TaskCompleted }, 2*time.Second, 10*time.Millisecond)
	return task.ID
}

func TestTaskOutputTool_ReturnsLastNLines(t *testing.T) {
	bm := newTaskToolsManager(t)
	id := startSimpleTask(t, bm, []string{"l1", "l2", "l3", "l4", "l5", "l6"})
	tool := NewTaskOutputTool(bm)
	res, err := tool.Execute(context.Background(), map[string]interface{}{
		"task_id": id,
		"lines":   3,
	})
	require.NoError(t, err)
	m := res.(map[string]interface{})
	assert.Equal(t, "l4\nl5\nl6", m["output"])
	assert.Equal(t, 3, m["line_count"])
}

func TestTaskOutputTool_DefaultLinesIs5(t *testing.T) {
	bm := newTaskToolsManager(t)
	id := startSimpleTask(t, bm, []string{"a", "b", "c", "d", "e", "f", "g"})
	tool := NewTaskOutputTool(bm)
	res, err := tool.Execute(context.Background(), map[string]interface{}{
		"task_id": id,
	})
	require.NoError(t, err)
	m := res.(map[string]interface{})
	assert.Equal(t, 5, m["line_count"])
}

func TestTaskOutputTool_UnknownTaskID(t *testing.T) {
	bm := newTaskToolsManager(t)
	tool := NewTaskOutputTool(bm)
	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"task_id": "missing",
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, workflow.ErrTaskNotFound))
}

func TestTaskStopTool_CancelsRunning(t *testing.T) {
	bm := newTaskToolsManager(t)
	exec := func(ctx context.Context, args map[string]interface{}, sink workflow.LineSink) (interface{}, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	task, err := bm.StartTask("Blocker", nil, exec)
	require.NoError(t, err)
	require.Eventually(t, func() bool { return task.State() == workflow.TaskRunning }, 1*time.Second, 10*time.Millisecond)

	tool := NewTaskStopTool(bm)
	res, err := tool.Execute(context.Background(), map[string]interface{}{"task_id": task.ID})
	require.NoError(t, err)
	m := res.(map[string]interface{})
	assert.Equal(t, "stopped", m["status"])

	require.Eventually(t, func() bool { return task.State() == workflow.TaskCancelled }, 1*time.Second, 10*time.Millisecond)
}

func TestTaskStopTool_NotRunningRejects(t *testing.T) {
	bm := newTaskToolsManager(t)
	id := startSimpleTask(t, bm, []string{"x"})
	tool := NewTaskStopTool(bm)
	_, err := tool.Execute(context.Background(), map[string]interface{}{"task_id": id})
	require.Error(t, err)
	assert.True(t, errors.Is(err, workflow.ErrTaskNotRunning))
}
