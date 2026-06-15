package hooks

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Executor executes hooks for events
type Executor struct {
	maxConcurrent int                // Maximum concurrent async executions
	semaphore     chan struct{}      // Semaphore for controlling concurrency
	wg            sync.WaitGroup     // Wait group for async executions
	results       []*ExecutionResult // Results of recent executions
	resultsMu     sync.RWMutex       // Mutex for results
	maxResults    int                // Maximum results to keep
	onComplete    []ResultCallback   // Callbacks on completion
	onError       []ResultCallback   // Callbacks on error
	// cfgMu guards the callback slices (onComplete/onError) and the concurrency
	// config (maxConcurrent/semaphore). Setters (OnComplete/OnError/
	// SetMaxConcurrent) mutate this shared state while async dispatch goroutines
	// read it (triggerCallbacks ranges the callback slices; executeAsync reads
	// the semaphore) — without this mutex those are data races (proven by the
	// -race detector in race_guard_test.go D1/D2).
	cfgMu sync.RWMutex
}

// ResultCallback is called when a hook execution completes
type ResultCallback func(*ExecutionResult)

// NewExecutor creates a new hook executor
func NewExecutor() *Executor {
	return &Executor{
		maxConcurrent: 10,
		semaphore:     make(chan struct{}, 10),
		results:       make([]*ExecutionResult, 0),
		maxResults:    100,
		onComplete:    make([]ResultCallback, 0),
		onError:       make([]ResultCallback, 0),
	}
}

// NewExecutorWithLimit creates a new executor with concurrency limit
func NewExecutorWithLimit(maxConcurrent int) *Executor {
	e := NewExecutor()
	e.maxConcurrent = maxConcurrent
	e.semaphore = make(chan struct{}, maxConcurrent)
	return e
}

// Execute executes a single hook
func (e *Executor) Execute(ctx context.Context, hook *Hook, event *Event) *ExecutionResult {
	result := NewExecutionResult(hook)

	// Check if hook should execute
	if !hook.ShouldExecute(event) {
		result.Skip() // SKIP-OK: #legacy-untriaged
		e.addResult(result)
		return result
	}

	// Execute asynchronously or synchronously
	if hook.Async {
		e.executeAsync(ctx, hook, event, result)
	} else {
		e.executeSync(ctx, hook, event, result)
	}

	return result
}

// ExecuteAll executes all hooks for an event
func (e *Executor) ExecuteAll(ctx context.Context, hooks []*Hook, event *Event) []*ExecutionResult {
	// Sort hooks by priority (highest first)
	sortedHooks := make([]*Hook, len(hooks))
	copy(sortedHooks, hooks)
	e.sortByPriority(sortedHooks)

	results := make([]*ExecutionResult, 0, len(sortedHooks))

	// Execute each hook
	for _, hook := range sortedHooks {
		result := e.Execute(ctx, hook, event)
		results = append(results, result)

		// Check if context was canceled
		if ctx.Err() != nil {
			// Cancel remaining hooks
			for i := len(results); i < len(sortedHooks); i++ {
				cancelResult := NewExecutionResult(sortedHooks[i])
				cancelResult.Cancel()
				results = append(results, cancelResult)
			}
			break
		}
	}

	return results
}

// ExecuteSync executes hooks synchronously and waits for all to complete
func (e *Executor) ExecuteSync(ctx context.Context, hooks []*Hook, event *Event) []*ExecutionResult {
	// Sort by priority
	sortedHooks := make([]*Hook, len(hooks))
	copy(sortedHooks, hooks)
	e.sortByPriority(sortedHooks)

	results := make([]*ExecutionResult, 0, len(sortedHooks))

	for _, hook := range sortedHooks {
		result := NewExecutionResult(hook)

		if !hook.ShouldExecute(event) {
			result.Skip() // SKIP-OK: #legacy-untriaged
			e.addResult(result)
			results = append(results, result)
			continue
		}

		e.executeSync(ctx, hook, event, result)
		results = append(results, result)

		// Check context
		if ctx.Err() != nil {
			break
		}
	}

	return results
}

// ExecuteAndWait executes hooks and waits for all (including async) to complete
func (e *Executor) ExecuteAndWait(ctx context.Context, hooks []*Hook, event *Event) []*ExecutionResult {
	results := e.ExecuteAll(ctx, hooks, event)
	e.Wait()
	return results
}

// Wait waits for all async executions to complete
func (e *Executor) Wait() {
	e.wg.Wait()
}

// executeSync executes a hook synchronously
func (e *Executor) executeSync(ctx context.Context, hook *Hook, event *Event, result *ExecutionResult) {
	result.Status = StatusRunning

	// Execute handler with panic isolation: a panicking hook handler MUST NOT
	// propagate out of the executor and crash the caller (or the whole process,
	// in the async path). A panic is converted to a failed ExecutionResult so
	// co-hooks still run and the manager stays usable (graceful degradation).
	err := e.runHandler(ctx, hook, event)

	result.Complete(err)
	e.addResult(result)
	e.triggerCallbacks(result)
}

// executeAsync executes a hook asynchronously
func (e *Executor) executeAsync(ctx context.Context, hook *Hook, event *Event, result *ExecutionResult) {
	e.wg.Add(1)

	go func() {
		defer e.wg.Done()

		// Snapshot the semaphore under cfgMu so a concurrent SetMaxConcurrent
		// swapping e.semaphore cannot race this read. The token is acquired
		// from and released to the SAME channel instance even if the executor's
		// active semaphore is swapped mid-flight — so an in-flight goroutine
		// never releases into a different channel than it acquired from.
		e.cfgMu.RLock()
		sem := e.semaphore
		e.cfgMu.RUnlock()

		// Acquire semaphore slot
		sem <- struct{}{}
		defer func() { <-sem }()

		result.Status = StatusRunning

		// Execute handler with panic isolation. Without recovery an unrecovered
		// panic in this goroutine would terminate the entire process — every
		// other goroutine, including unrelated work, dies with it.
		err := e.runHandler(ctx, hook, event)

		result.Complete(err)
		e.addResult(result)
		e.triggerCallbacks(result)
	}()
}

// runHandler invokes the hook handler, isolating any panic as an error so a
// misbehaving handler degrades gracefully instead of crashing the executor.
func (e *Executor) runHandler(ctx context.Context, hook *Hook, event *Event) (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("hook handler %q panicked: %v", hook.Name, p)
		}
	}()
	return hook.Execute(ctx, event)
}

// sortByPriority sorts hooks by priority (highest first)
func (e *Executor) sortByPriority(hooks []*Hook) {
	// Simple bubble sort by priority
	for i := 0; i < len(hooks)-1; i++ {
		for j := i + 1; j < len(hooks); j++ {
			if hooks[j].Priority > hooks[i].Priority {
				hooks[i], hooks[j] = hooks[j], hooks[i]
			}
		}
	}
}

// addResult adds a result to the results list
func (e *Executor) addResult(result *ExecutionResult) {
	e.resultsMu.Lock()
	defer e.resultsMu.Unlock()

	e.results = append(e.results, result)

	// Trim if exceeds max
	if e.maxResults > 0 && len(e.results) > e.maxResults {
		e.results = e.results[len(e.results)-e.maxResults:]
	}
}

// GetResults returns recent execution results
func (e *Executor) GetResults(n int) []*ExecutionResult {
	e.resultsMu.RLock()
	defer e.resultsMu.RUnlock()

	if n <= 0 || n > len(e.results) {
		n = len(e.results)
	}

	results := make([]*ExecutionResult, n)
	copy(results, e.results[len(e.results)-n:])
	return results
}

// GetAllResults returns all execution results
func (e *Executor) GetAllResults() []*ExecutionResult {
	e.resultsMu.RLock()
	defer e.resultsMu.RUnlock()

	results := make([]*ExecutionResult, len(e.results))
	copy(results, e.results)
	return results
}

// GetResultsByStatus returns results with specific status
func (e *Executor) GetResultsByStatus(status HookStatus) []*ExecutionResult {
	e.resultsMu.RLock()
	defer e.resultsMu.RUnlock()

	results := make([]*ExecutionResult, 0)
	for _, result := range e.results {
		if result.Status == status {
			results = append(results, result)
		}
	}
	return results
}

// ClearResults clears all stored results
func (e *Executor) ClearResults() {
	e.resultsMu.Lock()
	defer e.resultsMu.Unlock()

	e.results = make([]*ExecutionResult, 0)
}

// GetStatistics returns execution statistics
func (e *Executor) GetStatistics() *ExecutorStatistics {
	e.resultsMu.RLock()
	defer e.resultsMu.RUnlock()

	stats := &ExecutorStatistics{
		TotalExecutions: len(e.results),
		ByStatus:        make(map[HookStatus]int),
	}

	var totalDuration time.Duration
	for _, result := range e.results {
		stats.ByStatus[result.Status]++
		totalDuration += result.Duration
	}

	if stats.TotalExecutions > 0 {
		stats.AverageDuration = totalDuration / time.Duration(stats.TotalExecutions)
	}

	stats.SuccessRate = 0
	if stats.TotalExecutions > 0 {
		completed := stats.ByStatus[StatusCompleted]
		stats.SuccessRate = float64(completed) / float64(stats.TotalExecutions)
	}

	return stats
}

// OnComplete registers a callback for completed executions
func (e *Executor) OnComplete(callback ResultCallback) {
	e.cfgMu.Lock()
	defer e.cfgMu.Unlock()
	e.onComplete = append(e.onComplete, callback)
}

// OnError registers a callback for failed executions
func (e *Executor) OnError(callback ResultCallback) {
	e.cfgMu.Lock()
	defer e.cfgMu.Unlock()
	e.onError = append(e.onError, callback)
}

// triggerCallbacks triggers registered callbacks.
//
// The callback slices are snapshotted under cfgMu (RLock) and the user
// callbacks are invoked WITHOUT holding the lock — both to avoid the
// OnComplete/OnError write-vs-iterate race and so an arbitrary callback cannot
// deadlock by re-entering a setter while the executor holds its config lock.
func (e *Executor) triggerCallbacks(result *ExecutionResult) {
	e.cfgMu.RLock()
	onComplete := e.onComplete
	onError := e.onError
	failed := result.Status == StatusFailed
	e.cfgMu.RUnlock()

	// Trigger completion callbacks
	for _, callback := range onComplete {
		callback(result)
	}

	// Trigger error callbacks if failed
	if failed {
		for _, callback := range onError {
			callback(result)
		}
	}
}

// SetMaxConcurrent sets the maximum concurrent async executions.
//
// Mutates maxConcurrent and replaces the semaphore under cfgMu so concurrent
// async dispatch goroutines (which snapshot e.semaphore under cfgMu.RLock in
// executeAsync) never race this write. In-flight goroutines keep using the
// semaphore instance they snapshotted at acquire time, so swapping the channel
// here does not mis-route their token release; new dispatches pick up the new
// bound. SetMaxConcurrent is therefore safe to call at any time, not only at
// setup.
func (e *Executor) SetMaxConcurrent(max int) {
	e.cfgMu.Lock()
	defer e.cfgMu.Unlock()
	e.maxConcurrent = max
	e.semaphore = make(chan struct{}, max)
}

// SetMaxResults sets the maximum number of results to keep
func (e *Executor) SetMaxResults(max int) {
	e.resultsMu.Lock()
	defer e.resultsMu.Unlock()

	e.maxResults = max

	// Trim if needed
	if max > 0 && len(e.results) > max {
		e.results = e.results[len(e.results)-max:]
	}
}

// ExecutorStatistics contains execution statistics
type ExecutorStatistics struct {
	TotalExecutions int                // Total number of executions
	ByStatus        map[HookStatus]int // Count by status
	AverageDuration time.Duration      // Average execution duration
	SuccessRate     float64            // Success rate (0.0 - 1.0)
}

// String returns a string representation of the statistics
func (s *ExecutorStatistics) String() string {
	return fmt.Sprintf("Total: %d, Success: %.1f%%, Avg: %.2fms",
		s.TotalExecutions,
		s.SuccessRate*100,
		float64(s.AverageDuration.Microseconds())/1000)
}

// ExecuteWithTimeout executes a hook with a specific timeout
func (e *Executor) ExecuteWithTimeout(hook *Hook, event *Event, timeout time.Duration) *ExecutionResult {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return e.Execute(ctx, hook, event)
}

// ExecuteWithDeadline executes a hook with a specific deadline
func (e *Executor) ExecuteWithDeadline(hook *Hook, event *Event, deadline time.Time) *ExecutionResult {
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()

	return e.Execute(ctx, hook, event)
}
