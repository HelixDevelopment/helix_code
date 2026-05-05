package commands

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"dev.helix.code/internal/workflow"
)

func newTasksCommandWithMgr(t *testing.T) (*TasksCommand, *workflow.BackgroundManager) {
	t.Helper()
	bm := workflow.NewBackgroundManager(zap.NewNop(), workflow.ManagerConfig{})
	t.Cleanup(func() { _ = bm.Close() })
	return NewTasksCommand(bm), bm
}

func TestSlashTasks_ListEmpty(t *testing.T) {
	c, _ := newTasksCommandWithMgr(t)
	res, err := c.Execute(context.Background(), &CommandContext{Args: nil})
	require.NoError(t, err)
	assert.Contains(t, res.Output, "ID")
	assert.Contains(t, res.Output, "STATE")
}

func TestSlashTasks_ListShowsTask(t *testing.T) {
	c, bm := newTasksCommandWithMgr(t)
	exec := func(ctx context.Context, args map[string]interface{}, sink workflow.LineSink) (interface{}, error) {
		return "ok", nil
	}
	task, err := bm.StartTask("FakeTool", nil, exec)
	require.NoError(t, err)
	require.Eventually(t, func() bool { return task.State() == workflow.TaskCompleted }, 1*time.Second, 10*time.Millisecond)

	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"list"}})
	require.NoError(t, err)
	assert.Contains(t, res.Output, task.ID)
	assert.Contains(t, res.Output, "completed")
}

func TestSlashTasks_OutputReturnsLines(t *testing.T) {
	c, bm := newTasksCommandWithMgr(t)
	exec := func(ctx context.Context, args map[string]interface{}, sink workflow.LineSink) (interface{}, error) {
		sink("first")
		sink("second")
		sink("third")
		return "ok", nil
	}
	task, err := bm.StartTask("FakeTool", nil, exec)
	require.NoError(t, err)
	require.Eventually(t, func() bool { return task.State() == workflow.TaskCompleted }, 1*time.Second, 10*time.Millisecond)

	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"output", task.ID}})
	require.NoError(t, err)
	for _, want := range []string{"first", "second", "third"} {
		assert.True(t, strings.Contains(res.Output, want), "want %q in output", want)
	}
}

func TestSlashTasks_StopCancels(t *testing.T) {
	c, bm := newTasksCommandWithMgr(t)
	exec := func(ctx context.Context, args map[string]interface{}, sink workflow.LineSink) (interface{}, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	task, err := bm.StartTask("Blocker", nil, exec)
	require.NoError(t, err)
	require.Eventually(t, func() bool { return task.State() == workflow.TaskRunning }, 1*time.Second, 10*time.Millisecond)

	res, err := c.Execute(context.Background(), &CommandContext{Args: []string{"stop", task.ID}})
	require.NoError(t, err)
	assert.Contains(t, res.Output, "stopped")
	require.Eventually(t, func() bool { return task.State() == workflow.TaskCancelled }, 1*time.Second, 10*time.Millisecond)
}

func TestSlashTasks_UnknownSubcommand(t *testing.T) {
	c, _ := newTasksCommandWithMgr(t)
	_, err := c.Execute(context.Background(), &CommandContext{Args: []string{"bogus"}})
	require.Error(t, err)
}
