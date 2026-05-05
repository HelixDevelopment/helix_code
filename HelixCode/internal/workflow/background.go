// Package workflow's background.go provides BackgroundManager and BackgroundTask
// for running tools asynchronously in goroutines with line-oriented progress
// streaming and bounded output retention. This is the F07 surface; it is a
// sibling to the existing multi-step Executor/Workflow types in the same
// package and does not depend on them.
package workflow

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// TaskState is the lifecycle state of a background task.
type TaskState string

const (
	TaskPending   TaskState = "pending"
	TaskRunning   TaskState = "running"
	TaskCompleted TaskState = "completed"
	TaskFailed    TaskState = "failed"
	TaskCancelled TaskState = "cancelled"
)

// IsTerminal reports whether the state is one in which the task is done.
func (s TaskState) IsTerminal() bool {
	return s == TaskCompleted || s == TaskFailed || s == TaskCancelled
}

// BackgroundTask is one async tool execution.
type BackgroundTask struct {
	ID        string
	ToolName  string
	Args      map[string]any
	StartedAt time.Time

	state        atomic.Int32
	mu           sync.RWMutex
	endedAt      *time.Time
	output       []string
	outputCap    int
	lineBytesMax int
	result       any
	err          error
	ctx          context.Context
	cancel       context.CancelFunc
}

// newBackgroundTask constructs a BackgroundTask. Used by the manager.
// ctx and cancel may be nil for unit tests that bypass the manager.
func newBackgroundTask(id, toolName string, args map[string]any, outputCap, lineBytesMax int,
	ctx context.Context, cancel context.CancelFunc) *BackgroundTask {
	if outputCap <= 0 {
		outputCap = 256
	}
	if lineBytesMax <= 0 {
		lineBytesMax = 4096
	}
	bt := &BackgroundTask{
		ID:           id,
		ToolName:     toolName,
		Args:         args,
		StartedAt:    time.Now(),
		outputCap:    outputCap,
		lineBytesMax: lineBytesMax,
		ctx:          ctx,
		cancel:       cancel,
	}
	bt.state.Store(int32(taskStateOrdinal(TaskPending)))
	return bt
}

// State returns the current state via atomic load.
func (bt *BackgroundTask) State() TaskState {
	return ordinalToTaskState(bt.state.Load())
}

// EndedAt returns the task's end time (or nil if still running). Lock-guarded.
func (bt *BackgroundTask) EndedAt() *time.Time {
	bt.mu.RLock()
	defer bt.mu.RUnlock()
	return bt.endedAt
}

// SetState updates the state. On terminal transitions, endedAt is set.
// Panics if s is an unknown TaskState.
func (bt *BackgroundTask) SetState(s TaskState) {
	ord := taskStateOrdinal(s)
	if ord < 0 {
		panic(fmt.Sprintf("workflow: SetState called with unknown TaskState %q", s))
	}
	bt.state.Store(int32(ord))
	if s.IsTerminal() {
		bt.mu.Lock()
		now := time.Now()
		bt.endedAt = &now
		bt.mu.Unlock()
	}
}

// AppendOutput adds a line to the bounded output ring. Lines longer than
// lineBytesMax are truncated. When the ring exceeds outputCap, the oldest
// line is dropped.
func (bt *BackgroundTask) AppendOutput(line string) {
	if len(line) > bt.lineBytesMax {
		line = line[:bt.lineBytesMax]
	}
	bt.mu.Lock()
	defer bt.mu.Unlock()
	bt.output = append(bt.output, line)
	if len(bt.output) > bt.outputCap {
		drop := len(bt.output) - bt.outputCap
		bt.output = append([]string(nil), bt.output[drop:]...)
	}
}

// LastLines returns the last n lines (n<=0 means default 5).
func (bt *BackgroundTask) LastLines(n int) []string {
	if n <= 0 {
		n = 5
	}
	bt.mu.RLock()
	defer bt.mu.RUnlock()
	if len(bt.output) <= n {
		out := make([]string, len(bt.output))
		copy(out, bt.output)
		return out
	}
	out := make([]string, n)
	copy(out, bt.output[len(bt.output)-n:])
	return out
}

// setResult records the final tool result. Called by the manager goroutine.
func (bt *BackgroundTask) setResult(res any, err error) {
	bt.mu.Lock()
	bt.result = res
	bt.err = err
	bt.mu.Unlock()
}

// Result returns the final (result, err) tuple. Meaningful only after
// the task reaches a terminal state.
func (bt *BackgroundTask) Result() (any, error) {
	bt.mu.RLock()
	defer bt.mu.RUnlock()
	return bt.result, bt.err
}

// Err returns the recorded error (terminal-state convenience).
func (bt *BackgroundTask) Err() error {
	bt.mu.RLock()
	defer bt.mu.RUnlock()
	return bt.err
}

// taskStateOrdinal maps TaskState to a stable ordinal for atomic.Int32.
func taskStateOrdinal(s TaskState) int {
	switch s {
	case TaskPending:
		return 0
	case TaskRunning:
		return 1
	case TaskCompleted:
		return 2
	case TaskFailed:
		return 3
	case TaskCancelled:
		return 4
	default:
		return -1
	}
}

func ordinalToTaskState(o int32) TaskState {
	switch o {
	case 0:
		return TaskPending
	case 1:
		return TaskRunning
	case 2:
		return TaskCompleted
	case 3:
		return TaskFailed
	case 4:
		return TaskCancelled
	default:
		return TaskState(fmt.Sprintf("unknown(%d)", o))
	}
}
