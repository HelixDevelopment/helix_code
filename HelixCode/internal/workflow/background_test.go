package workflow

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
