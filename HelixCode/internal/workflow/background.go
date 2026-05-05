// Package workflow's background.go provides BackgroundManager and BackgroundTask
// for running tools asynchronously in goroutines with line-oriented progress
// streaming and bounded output retention. This is the F07 surface; it is a
// sibling to the existing multi-step Executor/Workflow types in the same
// package and does not depend on them.
package workflow

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
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

// LineSink is invoked by an executor for each line of progress output.
type LineSink func(line string)

// BackgroundExecutor is the closure StartTask runs in a goroutine.
type BackgroundExecutor func(ctx context.Context, args map[string]any, sink LineSink) (any, error)

// ManagerConfig configures a BackgroundManager.
type ManagerConfig struct {
	OutputCap     int           // per-task ring; default 256
	LineBytesMax  int           // per-line cap; default 4096
	SweepInterval time.Duration // sweeper tick; default 5min
	MaxAge        time.Duration // post-completion retention; default 1h
	MaxConcurrent int           // concurrent in-flight limit; default 64
}

// BackgroundManager manages concurrent background tasks.
type BackgroundManager struct {
	tasks   map[string]*BackgroundTask
	mu      sync.RWMutex
	cfg     ManagerConfig
	log     *zap.Logger
	closeCh chan struct{}
	closed  bool
	wg      sync.WaitGroup
}

// Error sentinels.
var (
	ErrTaskNotFound   = errors.New("workflow: background task not found")
	ErrTaskNotRunning = errors.New("workflow: task is not running")
	ErrManagerClosed  = errors.New("workflow: background manager closed")
	ErrTooManyTasks   = errors.New("workflow: too many concurrent background tasks")
)

// NewBackgroundManager constructs a manager and starts the sweeper goroutine.
func NewBackgroundManager(log *zap.Logger, cfg ManagerConfig) *BackgroundManager {
	if log == nil {
		log = zap.NewNop()
	}
	if cfg.OutputCap <= 0 {
		cfg.OutputCap = 256
	}
	if cfg.LineBytesMax <= 0 {
		cfg.LineBytesMax = 4096
	}
	if cfg.SweepInterval <= 0 {
		cfg.SweepInterval = 5 * time.Minute
	}
	if cfg.MaxAge <= 0 {
		cfg.MaxAge = 1 * time.Hour
	}
	if cfg.MaxConcurrent <= 0 {
		cfg.MaxConcurrent = 64
	}
	bm := &BackgroundManager{
		tasks:   make(map[string]*BackgroundTask),
		cfg:     cfg,
		log:     log,
		closeCh: make(chan struct{}),
	}
	bm.wg.Add(1)
	go bm.sweepLoop()
	return bm
}

// StartTask spawns a goroutine to run the executor. Returns immediately
// with the task; the goroutine writes terminal state on exit.
func (bm *BackgroundManager) StartTask(toolName string, args map[string]any, exec BackgroundExecutor) (*BackgroundTask, error) {
	bm.mu.Lock()
	if bm.closed {
		bm.mu.Unlock()
		return nil, ErrManagerClosed
	}
	if bm.countInFlightLocked() >= bm.cfg.MaxConcurrent {
		bm.mu.Unlock()
		return nil, ErrTooManyTasks
	}
	id := uuid.New().String()
	ctx, cancel := context.WithCancel(context.Background())
	task := newBackgroundTask(id, toolName, args, bm.cfg.OutputCap, bm.cfg.LineBytesMax, ctx, cancel)
	bm.tasks[id] = task
	bm.mu.Unlock()

	bm.log.Info("background task started",
		zap.String("id", id), zap.String("tool", toolName))

	bm.wg.Add(1)
	go bm.run(task, exec)
	return task, nil
}

// run executes the task in a goroutine with panic recovery.
func (bm *BackgroundManager) run(task *BackgroundTask, exec BackgroundExecutor) {
	defer bm.wg.Done()
	task.SetState(TaskRunning)
	defer func() {
		if r := recover(); r != nil {
			err := fmt.Errorf("panic: %v", r)
			task.AppendOutput(err.Error())
			task.setResult(nil, err)
			task.SetState(TaskFailed)
			bm.log.Warn("background task panicked",
				zap.String("id", task.ID), zap.Any("panic", r))
		}
	}()
	res, err := exec(task.ctx, task.Args, task.AppendOutput)
	task.setResult(res, err)
	switch {
	case err != nil && errors.Is(err, context.Canceled):
		task.SetState(TaskCancelled)
		bm.log.Info("background task cancelled", zap.String("id", task.ID))
	case err != nil:
		task.AppendOutput(fmt.Sprintf("Error: %v", err))
		task.SetState(TaskFailed)
		bm.log.Warn("background task failed",
			zap.String("id", task.ID), zap.Error(err))
	default:
		task.SetState(TaskCompleted)
		bm.log.Info("background task completed", zap.String("id", task.ID))
	}
}

// GetTask returns a task by ID.
func (bm *BackgroundManager) GetTask(id string) (*BackgroundTask, error) {
	bm.mu.RLock()
	defer bm.mu.RUnlock()
	task, ok := bm.tasks[id]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrTaskNotFound, id)
	}
	return task, nil
}

// StopTask cancels a running task. Returns ErrTaskNotRunning if the task
// is already in a terminal state.
func (bm *BackgroundManager) StopTask(id string) error {
	bm.mu.RLock()
	task, ok := bm.tasks[id]
	bm.mu.RUnlock()
	if !ok {
		return fmt.Errorf("%w: %s", ErrTaskNotFound, id)
	}
	st := task.State()
	if st != TaskRunning && st != TaskPending {
		return fmt.Errorf("%w: state=%s", ErrTaskNotRunning, st)
	}
	task.cancel()
	return nil
}

// ListTasks returns a snapshot of all current tasks.
func (bm *BackgroundManager) ListTasks() []*BackgroundTask {
	bm.mu.RLock()
	defer bm.mu.RUnlock()
	out := make([]*BackgroundTask, 0, len(bm.tasks))
	for _, t := range bm.tasks {
		out = append(out, t)
	}
	return out
}

// Status returns the current state and last output snapshot.
func (bm *BackgroundManager) Status(id string) (TaskState, []string, error) {
	task, err := bm.GetTask(id)
	if err != nil {
		return TaskState(""), nil, err
	}
	return task.State(), task.LastLines(0), nil
}

// Close stops the sweeper, cancels all in-flight tasks, and waits briefly
// for goroutines to exit. Idempotent.
func (bm *BackgroundManager) Close() error {
	bm.mu.Lock()
	if bm.closed {
		bm.mu.Unlock()
		return nil
	}
	bm.closed = true
	close(bm.closeCh)
	snap := make([]*BackgroundTask, 0, len(bm.tasks))
	for _, t := range bm.tasks {
		snap = append(snap, t)
	}
	bm.mu.Unlock()
	for _, t := range snap {
		if t.cancel != nil {
			t.cancel()
		}
	}
	done := make(chan struct{})
	go func() { bm.wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		bm.log.Warn("background manager close: drain timeout (5s)")
	}
	return nil
}

// countInFlightLocked counts non-terminal tasks. Caller must hold bm.mu.
func (bm *BackgroundManager) countInFlightLocked() int {
	n := 0
	for _, t := range bm.tasks {
		if !t.State().IsTerminal() {
			n++
		}
	}
	return n
}

// sweepLoop runs in a goroutine and periodically prunes terminal tasks
// older than cfg.MaxAge.
func (bm *BackgroundManager) sweepLoop() {
	defer bm.wg.Done()
	ticker := time.NewTicker(bm.cfg.SweepInterval)
	defer ticker.Stop()
	for {
		select {
		case <-bm.closeCh:
			return
		case <-ticker.C:
			bm.sweep()
		}
	}
}

func (bm *BackgroundManager) sweep() {
	cutoff := time.Now().Add(-bm.cfg.MaxAge)
	bm.mu.Lock()
	defer bm.mu.Unlock()
	for id, t := range bm.tasks {
		if !t.State().IsTerminal() {
			continue
		}
		// endedAt is unexported; same-package access is permitted but must
		// take t.mu.RLock() to avoid the data race documented in T02 fix-up.
		t.mu.RLock()
		ended := t.endedAt
		t.mu.RUnlock()
		if ended != nil && ended.Before(cutoff) {
			delete(bm.tasks, id)
			bm.log.Debug("background task swept", zap.String("id", id))
		}
	}
}
