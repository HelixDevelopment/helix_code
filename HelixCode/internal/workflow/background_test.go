package workflow

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestTaskState_String(t *testing.T) {
	assert.Equal(t, "pending", string(TaskPending))
	assert.Equal(t, "running", string(TaskRunning))
	assert.Equal(t, "completed", string(TaskCompleted))
	assert.Equal(t, "failed", string(TaskFailed))
	assert.Equal(t, "cancelled", string(TaskCancelled))
}

func TestBackgroundTask_AppendOutputAndLastLines(t *testing.T) {
	bt := newBackgroundTaskForTest("id-1", "Bash", map[string]any{"command": "x"}, 256, 4096)
	for i := 0; i < 10; i++ {
		bt.AppendOutput("line " + string(rune('0'+i)))
	}
	last := bt.LastLines(3)
	assert.Equal(t, []string{"line 7", "line 8", "line 9"}, last)
	assert.Equal(t, []string{"line 5", "line 6", "line 7", "line 8", "line 9"}, bt.LastLines(0)) // default 5
}

func TestBackgroundTask_OutputRingBoundedAtCap(t *testing.T) {
	bt := newBackgroundTaskForTest("id-2", "Bash", nil, 4, 4096) // cap 4
	for i := 0; i < 10; i++ {
		bt.AppendOutput("line " + string(rune('0'+i)))
	}
	assert.Equal(t, 4, len(bt.LastLines(100)))
	assert.Equal(t, []string{"line 6", "line 7", "line 8", "line 9"}, bt.LastLines(100))
}

func TestBackgroundTask_LineTruncatedAtMax(t *testing.T) {
	bt := newBackgroundTaskForTest("id-3", "Bash", nil, 256, 16) // max 16 bytes per line
	bt.AppendOutput("0123456789ABCDEF_truncated_part")
	assert.Equal(t, []string{"0123456789ABCDEF"}, bt.LastLines(1))
}

func TestBackgroundTask_StateAtomicGetSet(t *testing.T) {
	bt := newBackgroundTaskForTest("id-4", "Bash", nil, 256, 4096)
	assert.Equal(t, TaskPending, bt.State())
	bt.SetState(TaskRunning)
	assert.Equal(t, TaskRunning, bt.State())
	bt.SetState(TaskCompleted)
	assert.Equal(t, TaskCompleted, bt.State())
	assert.NotNil(t, bt.EndedAt())
}

func TestBackgroundTask_SetStateRunningDoesNotSetEndedAt(t *testing.T) {
	bt := newBackgroundTaskForTest("id-5", "Bash", nil, 256, 4096)
	bt.SetState(TaskRunning)
	assert.Nil(t, bt.EndedAt())
}

func TestBackgroundTask_ResultRoundTrip(t *testing.T) {
	bt := newBackgroundTaskForTest("id-6", "Bash", nil, 256, 4096)
	bt.setResult("ok", nil)
	res, err := bt.Result()
	assert.Equal(t, "ok", res)
	assert.NoError(t, err)
}

func TestBackgroundTask_SetStateUnknownPanics(t *testing.T) {
	bt := newBackgroundTaskForTest("id-x", "Bash", nil, 256, 4096)
	assert.Panics(t, func() {
		bt.SetState(TaskState("bogus"))
	})
}

// newBackgroundTaskForTest is a package-internal constructor that bypasses
// BackgroundManager; allows unit-testing BackgroundTask in isolation.
func newBackgroundTaskForTest(id, tool string, args map[string]any, cap, lineMax int) *BackgroundTask {
	return newBackgroundTask(id, tool, args, cap, lineMax, nil, nil)
}

func newTestManager(t *testing.T, cfg ManagerConfig) *BackgroundManager {
	t.Helper()
	m := NewBackgroundManager(zap.NewNop(), cfg)
	t.Cleanup(func() { _ = m.Close() })
	return m
}

func TestBackgroundManager_StartTaskRunsExecutor(t *testing.T) {
	m := newTestManager(t, ManagerConfig{})
	exec := func(ctx context.Context, args map[string]any, sink LineSink) (any, error) {
		sink("a")
		sink("b")
		sink("c")
		return "result-x", nil
	}
	task, err := m.StartTask("FakeTool", map[string]any{"k": "v"}, exec)
	require.NoError(t, err)
	require.NotNil(t, task)
	require.NotEmpty(t, task.ID)
	require.Eventually(t, func() bool {
		return task.State() == TaskCompleted
	}, 2*time.Second, 10*time.Millisecond)
	assert.Equal(t, []string{"a", "b", "c"}, task.LastLines(10))
	res, terr := task.Result()
	assert.Equal(t, "result-x", res)
	assert.NoError(t, terr)
}

func TestBackgroundManager_StopTaskCancelsCtx(t *testing.T) {
	m := newTestManager(t, ManagerConfig{})
	exec := func(ctx context.Context, args map[string]any, sink LineSink) (any, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	task, err := m.StartTask("FakeTool", nil, exec)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		return task.State() == TaskRunning
	}, 1*time.Second, 10*time.Millisecond)
	require.NoError(t, m.StopTask(task.ID))
	require.Eventually(t, func() bool {
		return task.State() == TaskCancelled
	}, 1*time.Second, 10*time.Millisecond)
}

func TestBackgroundManager_StopUnknownTaskReturnsError(t *testing.T) {
	m := newTestManager(t, ManagerConfig{})
	err := m.StopTask("nonexistent")
	assert.True(t, errors.Is(err, ErrTaskNotFound))
}

func TestBackgroundManager_StopAlreadyDoneTaskRejects(t *testing.T) {
	m := newTestManager(t, ManagerConfig{})
	exec := func(ctx context.Context, args map[string]any, sink LineSink) (any, error) { return "ok", nil }
	task, err := m.StartTask("FakeTool", nil, exec)
	require.NoError(t, err)
	require.Eventually(t, func() bool { return task.State() == TaskCompleted }, 1*time.Second, 10*time.Millisecond)
	err = m.StopTask(task.ID)
	assert.True(t, errors.Is(err, ErrTaskNotRunning))
}

func TestBackgroundManager_GetTaskMissing(t *testing.T) {
	m := newTestManager(t, ManagerConfig{})
	_, err := m.GetTask("missing")
	assert.True(t, errors.Is(err, ErrTaskNotFound))
}

func TestBackgroundManager_PanickingExecRecovers(t *testing.T) {
	m := newTestManager(t, ManagerConfig{})
	exec := func(ctx context.Context, args map[string]any, sink LineSink) (any, error) {
		panic("boom")
	}
	task, err := m.StartTask("Panicker", nil, exec)
	require.NoError(t, err)
	require.Eventually(t, func() bool { return task.State() == TaskFailed }, 1*time.Second, 10*time.Millisecond)
	_, terr := task.Result()
	require.Error(t, terr)
	assert.Contains(t, terr.Error(), "panic")
	exec2 := func(ctx context.Context, args map[string]any, sink LineSink) (any, error) { return "ok", nil }
	task2, err := m.StartTask("Survivor", nil, exec2)
	require.NoError(t, err)
	require.Eventually(t, func() bool { return task2.State() == TaskCompleted }, 1*time.Second, 10*time.Millisecond)
}

func TestBackgroundManager_NilResultNilError(t *testing.T) {
	m := newTestManager(t, ManagerConfig{})
	exec := func(ctx context.Context, args map[string]any, sink LineSink) (any, error) {
		return nil, nil
	}
	task, err := m.StartTask("FakeTool", nil, exec)
	require.NoError(t, err)
	require.Eventually(t, func() bool { return task.State() == TaskCompleted }, 1*time.Second, 10*time.Millisecond)
	res, terr := task.Result()
	assert.Nil(t, res)
	assert.NoError(t, terr)
}

func TestBackgroundManager_MaxConcurrentEnforced(t *testing.T) {
	m := newTestManager(t, ManagerConfig{MaxConcurrent: 2})
	block := make(chan struct{})
	exec := func(ctx context.Context, args map[string]any, sink LineSink) (any, error) {
		<-block
		return nil, nil
	}
	t1, err := m.StartTask("FakeTool", nil, exec)
	require.NoError(t, err)
	t2, err := m.StartTask("FakeTool", nil, exec)
	require.NoError(t, err)
	_, err = m.StartTask("FakeTool", nil, exec)
	assert.True(t, errors.Is(err, ErrTooManyTasks))
	close(block)
	_ = t1
	_ = t2
}

func TestBackgroundManager_SweeperReapsCompleted(t *testing.T) {
	m := newTestManager(t, ManagerConfig{
		SweepInterval: 50 * time.Millisecond,
		MaxAge:        100 * time.Millisecond,
	})
	exec := func(ctx context.Context, args map[string]any, sink LineSink) (any, error) { return "ok", nil }
	task, err := m.StartTask("FakeTool", nil, exec)
	require.NoError(t, err)
	require.Eventually(t, func() bool { return task.State() == TaskCompleted }, 1*time.Second, 10*time.Millisecond)
	require.Eventually(t, func() bool {
		_, gerr := m.GetTask(task.ID)
		return errors.Is(gerr, ErrTaskNotFound)
	}, 2*time.Second, 50*time.Millisecond)
}

func TestBackgroundManager_SweeperLeavesRunning(t *testing.T) {
	m := newTestManager(t, ManagerConfig{
		SweepInterval: 50 * time.Millisecond,
		MaxAge:        50 * time.Millisecond,
	})
	block := make(chan struct{})
	exec := func(ctx context.Context, args map[string]any, sink LineSink) (any, error) {
		<-block
		return nil, nil
	}
	task, err := m.StartTask("FakeTool", nil, exec)
	require.NoError(t, err)
	require.Eventually(t, func() bool { return task.State() == TaskRunning }, 1*time.Second, 10*time.Millisecond)
	time.Sleep(300 * time.Millisecond)
	got, err := m.GetTask(task.ID)
	require.NoError(t, err)
	assert.Equal(t, task, got)
	close(block)
}

func TestBackgroundManager_CloseIdempotent(t *testing.T) {
	m := NewBackgroundManager(zap.NewNop(), ManagerConfig{})
	require.NoError(t, m.Close())
	require.NoError(t, m.Close())
	exec := func(ctx context.Context, args map[string]any, sink LineSink) (any, error) { return nil, nil }
	_, err := m.StartTask("X", nil, exec)
	assert.True(t, errors.Is(err, ErrManagerClosed))
}

func TestBackgroundManager_CloseCancelsInFlight(t *testing.T) {
	m := NewBackgroundManager(zap.NewNop(), ManagerConfig{})
	exec := func(ctx context.Context, args map[string]any, sink LineSink) (any, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	task1, err := m.StartTask("X", nil, exec)
	require.NoError(t, err)
	task2, err := m.StartTask("Y", nil, exec)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		return task1.State() == TaskRunning && task2.State() == TaskRunning
	}, 1*time.Second, 10*time.Millisecond)
	require.NoError(t, m.Close())
	assert.True(t, task1.State() == TaskCancelled || task1.State() == TaskFailed)
	assert.True(t, task2.State() == TaskCancelled || task2.State() == TaskFailed)
}

func TestBackgroundManager_ListTasksSnapshot(t *testing.T) {
	m := newTestManager(t, ManagerConfig{})
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		exec := func(ctx context.Context, args map[string]any, sink LineSink) (any, error) {
			defer wg.Done()
			return "ok", nil
		}
		_, err := m.StartTask("FakeTool", nil, exec)
		require.NoError(t, err)
	}
	wg.Wait()
	list := m.ListTasks()
	assert.Len(t, list, 3)
}
