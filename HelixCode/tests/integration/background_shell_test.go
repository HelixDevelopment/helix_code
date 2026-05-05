//go:build integration

package integration

import (
	"context"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"dev.helix.code/internal/tools"
	"dev.helix.code/internal/workflow"
)

func skipIfWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("SKIP-OK: shell-based integration tests are POSIX-only on this branch")
	}
}

// TestBackground_ShellEcho_StreamsAndCompletes goes through the full
// ToolRegistry.Execute path with run_in_background:true, using the real
// shell BackgroundAware adapter against a real subprocess.
func TestBackground_ShellEcho_StreamsAndCompletes(t *testing.T) {
	skipIfWindows(t)
	reg, err := tools.NewToolRegistry(tools.DefaultRegistryConfig())
	require.NoError(t, err)
	bm := workflow.NewBackgroundManager(zap.NewNop(), workflow.ManagerConfig{})
	defer bm.Close()
	reg.SetBackgroundManager(bm)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	res, err := reg.Execute(ctx, "shell", map[string]interface{}{
		"command":           "echo hello",
		"run_in_background": true,
	})
	require.NoError(t, err)
	m := res.(map[string]interface{})
	taskID := m["task_id"].(string)
	require.Eventually(t, func() bool {
		task, err := bm.GetTask(taskID)
		return err == nil && task.State() == workflow.TaskCompleted
	}, 5*time.Second, 25*time.Millisecond)
	task, err := bm.GetTask(taskID)
	require.NoError(t, err)
	all := strings.Join(task.LastLines(100), "\n")
	assert.Contains(t, all, "hello")
}

func TestBackground_ShellSleep_StopCancels(t *testing.T) {
	skipIfWindows(t)
	reg, err := tools.NewToolRegistry(tools.DefaultRegistryConfig())
	require.NoError(t, err)
	bm := workflow.NewBackgroundManager(zap.NewNop(), workflow.ManagerConfig{})
	defer bm.Close()
	reg.SetBackgroundManager(bm)

	res, err := reg.Execute(context.Background(), "shell", map[string]interface{}{
		"command":           "sleep 30",
		"run_in_background": true,
	})
	require.NoError(t, err)
	m := res.(map[string]interface{})
	taskID := m["task_id"].(string)
	require.Eventually(t, func() bool {
		task, _ := bm.GetTask(taskID)
		return task != nil && task.State() == workflow.TaskRunning
	}, 2*time.Second, 25*time.Millisecond)

	require.NoError(t, bm.StopTask(taskID))
	require.Eventually(t, func() bool {
		task, _ := bm.GetTask(taskID)
		return task != nil && (task.State() == workflow.TaskCancelled || task.State() == workflow.TaskFailed)
	}, 3*time.Second, 25*time.Millisecond)

	out, _ := exec.Command("pgrep", "-x", "sleep").Output()
	pids := strings.TrimSpace(string(out))
	if pids != "" {
		t.Logf("note: pgrep found sleep processes: %s (may belong to other tests/users)", pids)
	}
}

func TestBackground_ConcurrentTasks(t *testing.T) {
	skipIfWindows(t)
	reg, err := tools.NewToolRegistry(tools.DefaultRegistryConfig())
	require.NoError(t, err)
	bm := workflow.NewBackgroundManager(zap.NewNop(), workflow.ManagerConfig{})
	defer bm.Close()
	reg.SetBackgroundManager(bm)

	ids := make([]string, 0, 5)
	for i := 0; i < 5; i++ {
		res, err := reg.Execute(context.Background(), "shell", map[string]interface{}{
			"command":           "sleep 0.5",
			"run_in_background": true,
		})
		require.NoError(t, err)
		ids = append(ids, res.(map[string]interface{})["task_id"].(string))
	}
	require.Eventually(t, func() bool {
		for _, id := range ids {
			task, err := bm.GetTask(id)
			if err != nil || task.State() != workflow.TaskCompleted {
				return false
			}
		}
		return true
	}, 5*time.Second, 50*time.Millisecond)
	assert.Len(t, bm.ListTasks(), 5)
}
