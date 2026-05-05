// manager.go — P1-F15-T05 SubagentManager.
//
// SubagentManager is the central control point for subagent dispatch and
// lifecycle. It accepts SubagentTasks, picks the appropriate spawner based on
// task.Isolation, enforces a max-concurrency cap, dispatches the task, fans
// streaming results through a single aggregator channel, and supports
// kill-by-id plus graceful shutdown.
//
// Architecture (per spec §4):
//
//   coordinator -> Dispatch(task)
//                       |
//                       v
//                 [validate + clamp timeout]
//                       |
//                       v
//                 [acquire semaphore slot or ErrMaxConcurrency]
//                       |
//                       v
//                 [pick spawner: in-process | subprocess]
//                       |
//                       v
//                 spawner.Spawn(ctx) -> per-task chan SubagentResult
//                       |
//                       v
//                 [forwarding goroutine drains per-task chan,
//                  sends to aggregator, releases slot, removes from running]
//
// Anti-bluff guarantee: the manager NEVER fabricates a result. Every result
// on the aggregator channel comes from a real spawner goroutine that either
// invoked the real llm.Provider (in-process) or ran a real subprocess
// (subprocess). Kill cancels via context.CancelFunc; the spawner observes the
// cancellation through ctx and emits StateCanceled.
//
// Spec: docs/superpowers/specs/2026-05-06-p1-f15-subagent-team-design.md §4
// Plan: docs/superpowers/plans/2026-05-06-p1-f15-subagent-team.md T05
package subagent

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"dev.helix.code/internal/llm"
)

// Spec defaults (§3 / earlier subagent report):
//   - max concurrency 5
//   - default per-task timeout 5 min
//   - hard ceiling 30 min
const (
	// DefaultMaxConcurrency is the default upper bound on simultaneously
	// running subagents. Picked to keep coordinator memory bounded while
	// still allowing meaningful parallelism on developer laptops.
	DefaultMaxConcurrency = 5
	// DefaultTaskTimeout is the per-task timeout applied when SubagentTask.Timeout == 0.
	DefaultTaskTimeout = 5 * time.Minute
	// HardTaskTimeoutCeiling clamps any caller-supplied Timeout that exceeds
	// this value. Prevents a runaway task from holding a slot indefinitely.
	HardTaskTimeoutCeiling = 30 * time.Minute
)

// SubagentManagerOptions configures a SubagentManager. The zero value is NOT
// valid — LLMProvider must be supplied. All other fields are optional and
// receive sensible defaults.
type SubagentManagerOptions struct {
	// MaxConcurrency caps the number of simultaneously-running subagents.
	// Default: DefaultMaxConcurrency. Values < 1 are coerced to 1.
	MaxConcurrency int

	// DefaultTimeout is applied when a SubagentTask.Timeout == 0.
	// Default: DefaultTaskTimeout. Caller-supplied timeouts that exceed
	// HardTaskTimeoutCeiling are clamped down.
	DefaultTimeout time.Duration

	// InProcessSpawner handles IsolationNone tasks. Default: NewInProcessSpawner().
	InProcessSpawner SubagentSpawner

	// SubprocessSpawner handles IsolationWorktree tasks. Default:
	// NewSubprocessSpawner(WorkDir). Construction failures are surfaced from
	// NewSubagentManager.
	SubprocessSpawner SubagentSpawner

	// LLMProvider is REQUIRED. Passed to the in-process spawner; the
	// subprocess spawner ignores it (the child process constructs its own
	// provider via T07/T08 wiring).
	LLMProvider llm.Provider

	// Logger is the zap logger used for manager-level events. Default: Nop.
	Logger *zap.Logger

	// WorkDir is forwarded to NewSubprocessSpawner when SubprocessSpawner is
	// nil. Empty string inherits the parent's cwd.
	WorkDir string
}

// runningSubagent tracks a live subagent for kill-by-id and Status.
type runningSubagent struct {
	task    SubagentTask
	cancel  context.CancelFunc
	started time.Time
}

// SubagentStatus is a public read-only snapshot of a running subagent.
type SubagentStatus struct {
	ID          string        `json:"id"`
	Description string        `json:"description"`
	Isolation   Isolation     `json:"isolation"`
	StartedAt   time.Time     `json:"started_at"`
	Elapsed     time.Duration `json:"elapsed"`
}

// SubagentManager orchestrates subagent dispatch and lifecycle.
type SubagentManager struct {
	opts SubagentManagerOptions
	log  *zap.Logger

	mu      sync.RWMutex
	running map[string]*runningSubagent

	aggregator       chan SubagentResult
	aggregatorClosed chan struct{}
	closeOnce        sync.Once

	// semaphore is a buffered channel used as a counting semaphore for the
	// concurrency cap. A non-blocking select against this channel implements
	// the "reject on full" semantics of ErrMaxConcurrency.
	semaphore chan struct{}

	// inflight is incremented when a forwarding goroutine starts and
	// decremented when it ends. Shutdown waits on this to know when all
	// forwarders have flushed their results to the aggregator.
	inflight sync.WaitGroup
}

// NewSubagentManager constructs a manager from the given options. Returns an
// error when LLMProvider is nil (required for in-process dispatch). Defaults
// are applied for all other fields.
func NewSubagentManager(opts SubagentManagerOptions) (*SubagentManager, error) {
	if opts.LLMProvider == nil {
		return nil, errors.New("subagent: NewSubagentManager: LLMProvider is required")
	}
	if opts.MaxConcurrency < 1 {
		opts.MaxConcurrency = DefaultMaxConcurrency
	}
	if opts.DefaultTimeout <= 0 {
		opts.DefaultTimeout = DefaultTaskTimeout
	}
	if opts.Logger == nil {
		opts.Logger = zap.NewNop()
	}
	if opts.InProcessSpawner == nil {
		opts.InProcessSpawner = NewInProcessSpawner()
	}
	if opts.SubprocessSpawner == nil {
		sp, err := NewSubprocessSpawner(opts.WorkDir)
		if err != nil {
			return nil, fmt.Errorf("subagent: NewSubagentManager: subprocess spawner: %w", err)
		}
		opts.SubprocessSpawner = sp
	}

	m := &SubagentManager{
		opts:             opts,
		log:              opts.Logger,
		running:          make(map[string]*runningSubagent),
		aggregator:       make(chan SubagentResult, opts.MaxConcurrency*2),
		aggregatorClosed: make(chan struct{}),
		semaphore:        make(chan struct{}, opts.MaxConcurrency),
	}
	return m, nil
}

// Dispatch starts a subagent for the given task. If task.ID is empty, a
// UUIDv4 is assigned. The returned ID identifies the subagent for Kill /
// Status / WaitAll calls. The result will appear on the aggregator channel
// (Results()) when the subagent completes.
//
// Errors:
//   - ErrMaxConcurrency: the concurrency cap is reached. Caller may retry.
//   - ErrUnknownIsolation: task.Isolation is invalid.
//   - validation errors when task.Description / task.Prompt is empty.
//
// Dispatch returns immediately after spawning; it does NOT wait for the
// subagent to complete.
func (m *SubagentManager) Dispatch(ctx context.Context, task SubagentTask) (string, error) {
	// Validate before we touch anything else.
	if task.Prompt == "" {
		return "", errors.New("subagent: Dispatch: task.Prompt is empty")
	}
	if task.Description == "" {
		return "", errors.New("subagent: Dispatch: task.Description is empty")
	}
	if task.Isolation == "" {
		task.Isolation = IsolationNone
	}
	switch task.Isolation {
	case IsolationNone, IsolationWorktree:
		// ok
	default:
		return "", fmt.Errorf("%w: %q", ErrUnknownIsolation, task.Isolation)
	}

	// Apply / clamp timeout per spec §3.
	switch {
	case task.Timeout <= 0:
		task.Timeout = m.opts.DefaultTimeout
	case task.Timeout > HardTaskTimeoutCeiling:
		task.Timeout = HardTaskTimeoutCeiling
	}

	// Assign UUIDv4 if missing.
	if task.ID == "" {
		task.ID = uuid.NewString()
	}

	// Acquire semaphore slot non-blocking. If the manager has been shut
	// down, the aggregatorClosed channel will be closed and we should
	// reject new work.
	select {
	case <-m.aggregatorClosed:
		return "", errors.New("subagent: Dispatch: manager is shut down")
	default:
	}
	select {
	case m.semaphore <- struct{}{}:
	default:
		return "", ErrMaxConcurrency
	}

	// Pick the spawner.
	var spawner SubagentSpawner
	switch task.Isolation {
	case IsolationNone:
		spawner = m.opts.InProcessSpawner
	case IsolationWorktree:
		spawner = m.opts.SubprocessSpawner
	}

	// Build a child ctx so Kill(id) / Shutdown can cancel this specific
	// subagent without affecting others.
	childCtx, cancel := context.WithCancel(ctx)

	rs := &runningSubagent{
		task:    task,
		cancel:  cancel,
		started: time.Now(),
	}

	// Register BEFORE Spawn so Status / Kill see this subagent immediately.
	m.mu.Lock()
	m.running[task.ID] = rs
	m.mu.Unlock()

	resultCh, err := spawner.Spawn(childCtx, task, m.opts.LLMProvider)
	if err != nil {
		// Spawn failed before the goroutine started. Roll back state.
		m.mu.Lock()
		delete(m.running, task.ID)
		m.mu.Unlock()
		cancel()
		<-m.semaphore
		return "", fmt.Errorf("subagent: Dispatch: spawn failed: %w", err)
	}

	m.inflight.Add(1)
	go m.forward(task.ID, resultCh)

	m.log.Info("subagent: dispatched",
		zap.String("id", task.ID),
		zap.String("description", task.Description),
		zap.String("isolation", string(task.Isolation)),
		zap.String("spawner", spawner.Kind()),
		zap.Duration("timeout", task.Timeout))
	return task.ID, nil
}

// forward drains a per-task spawner channel and emits results on the
// aggregator. Releases the semaphore slot and removes the subagent from the
// running map after the per-task channel closes.
func (m *SubagentManager) forward(id string, ch <-chan SubagentResult) {
	defer m.inflight.Done()
	defer func() {
		// Always release the slot + remove from running, even on panic.
		m.mu.Lock()
		if rs, ok := m.running[id]; ok {
			rs.cancel()
			delete(m.running, id)
		}
		m.mu.Unlock()
		<-m.semaphore
	}()

	for r := range ch {
		// Stamp the TaskID if the spawner forgot to.
		if r.TaskID == "" {
			r.TaskID = id
		}
		// Forward to aggregator unless it's already closed.
		select {
		case <-m.aggregatorClosed:
			// Manager is shutting down; drop the result rather than panic
			// on send-to-closed-channel.
			return
		case m.aggregator <- r:
		}
	}
}

// Results returns the aggregator channel that receives results from ALL
// running subagents. The channel is closed when Shutdown completes.
//
// Idempotent: repeated calls return the same channel.
func (m *SubagentManager) Results() <-chan SubagentResult {
	return m.aggregator
}

// Status returns a snapshot of currently-running subagents.
func (m *SubagentManager) Status() []SubagentStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	now := time.Now()
	out := make([]SubagentStatus, 0, len(m.running))
	for id, rs := range m.running {
		out = append(out, SubagentStatus{
			ID:          id,
			Description: rs.task.Description,
			Isolation:   rs.task.Isolation,
			StartedAt:   rs.started,
			Elapsed:     now.Sub(rs.started),
		})
	}
	return out
}

// Kill cancels the subagent with the given ID. The subagent's spawner will
// observe the ctx cancellation and emit a StateCanceled result on the
// aggregator. Returns nil on success, error if no such ID is running.
func (m *SubagentManager) Kill(id string) error {
	m.mu.Lock()
	rs, ok := m.running[id]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("subagent: Kill: no running subagent with id %q", id)
	}
	cancel := rs.cancel
	m.mu.Unlock()

	cancel()
	m.log.Info("subagent: kill requested", zap.String("id", id))
	return nil
}

// WaitAll blocks until results have been received for every ID in taskIDs,
// returning them in completion order. If ctx is canceled before all results
// arrive, returns the partial slice and ctx.Err().
//
// NOTE: WaitAll consumes from the aggregator. Callers using WaitAll should
// NOT also drain Results() in another goroutine for the same IDs.
func (m *SubagentManager) WaitAll(ctx context.Context, taskIDs []string) ([]SubagentResult, error) {
	if len(taskIDs) == 0 {
		return nil, nil
	}
	pending := make(map[string]struct{}, len(taskIDs))
	for _, id := range taskIDs {
		pending[id] = struct{}{}
	}
	results := make([]SubagentResult, 0, len(taskIDs))

	for len(pending) > 0 {
		select {
		case <-ctx.Done():
			return results, ctx.Err()
		case r, ok := <-m.aggregator:
			if !ok {
				return results, errors.New("subagent: WaitAll: aggregator closed before all IDs completed")
			}
			if _, want := pending[r.TaskID]; want {
				delete(pending, r.TaskID)
				results = append(results, r)
			}
			// If the result is for an ID we don't care about, drop it.
			// (In practice, callers should either use WaitAll OR drain
			// Results() — not both. This branch exists for forward-compat.)
		}
	}
	return results, nil
}

// Shutdown gracefully cancels all running subagents and closes the aggregator
// channel. Idempotent.
func (m *SubagentManager) Shutdown(ctx context.Context) error {
	first := false
	m.closeOnce.Do(func() {
		first = true
	})
	if !first {
		return nil
	}

	// Cancel all running subagents.
	m.mu.Lock()
	for _, rs := range m.running {
		rs.cancel()
	}
	m.mu.Unlock()

	// Wait for all forward goroutines to finish flushing.
	done := make(chan struct{})
	go func() {
		m.inflight.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-ctx.Done():
		// Even on ctx timeout we proceed to close the aggregator so callers
		// reading Results() don't hang forever. The forwarder goroutines
		// will check aggregatorClosed and exit on their next iteration.
	}

	// Close the aggregatorClosed sentinel so any forwarder that's still
	// running can detect shutdown and stop sending; then close the
	// aggregator channel itself.
	close(m.aggregatorClosed)
	close(m.aggregator)

	return nil
}
