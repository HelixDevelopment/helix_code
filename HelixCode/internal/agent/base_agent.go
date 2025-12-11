package agent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"dev.helix.code/internal/agent/task"
	"dev.helix.code/internal/config"
)

// Task represents a task that can be executed by an agent
type Task = task.Task

// TaskResult represents the result of a task execution
type TaskResult struct {
	TaskID      string                 `json:"task_id"`
	Success     bool                   `json:"success"`
	Result      interface{}            `json:"result,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Duration    time.Duration          `json:"duration"`
	CompletedAt time.Time              `json:"completed_at"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// BaseAgent provides a basic implementation of the Agent interface
type BaseAgent struct {
	id           string
	name         string
	status       AgentStatus
	capabilities []Capability
	taskQueue    chan *Task
	resultChan   chan *TaskResult
	stopChan     chan struct{}
	wg           sync.WaitGroup
	mu           sync.RWMutex

	// Statistics
	tasksProcessed int
	tasksSucceeded int
	tasksFailed    int
	totalDuration  time.Duration
	lastActivity   time.Time

	// Configuration
	maxConcurrency int
	timeout        time.Duration
	retryCount     int
}

// NewBaseAgent creates a new base agent
func NewBaseAgent(id, name string, config *config.AgentConfig) *BaseAgent {
	maxConcurrency := 1
	timeout := 30 * time.Second
	retryCount := 3

	if config != nil {
		if config.MaxConcurrency > 0 {
			maxConcurrency = config.MaxConcurrency
		}
		if config.Timeout > 0 {
			timeout = time.Duration(config.Timeout) * time.Second
		}
		if config.RetryCount >= 0 {
			retryCount = config.RetryCount
		}
	}

	return &BaseAgent{
		id:             id,
		name:           name,
		status:         StatusIdle,
		capabilities:   []Capability{},
		taskQueue:      make(chan *Task, 100),
		resultChan:     make(chan *TaskResult, 100),
		stopChan:       make(chan struct{}),
		maxConcurrency: maxConcurrency,
		timeout:        timeout,
		retryCount:     retryCount,
		lastActivity:   time.Now(),
	}
}

// ID returns the agent ID
func (a *BaseAgent) ID() string {
	return a.id
}

// Name returns the agent name
func (a *BaseAgent) Name() string {
	return a.name
}

// Status returns the current agent status
func (a *BaseAgent) Status() AgentStatus {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.status
}

// SetStatus sets the agent status
func (a *BaseAgent) SetStatus(status AgentStatus) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.status = status
	a.lastActivity = time.Now()
}

// Capabilities returns the agent capabilities
func (a *BaseAgent) Capabilities() []Capability {
	a.mu.RLock()
	defer a.mu.RUnlock()
	caps := make([]Capability, len(a.capabilities))
	copy(caps, a.capabilities)
	return caps
}

// AddCapability adds a capability to the agent
func (a *BaseAgent) AddCapability(capability Capability) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.capabilities = append(a.capabilities, capability)
}

// RemoveCapability removes a capability from the agent
func (a *BaseAgent) RemoveCapability(name string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	for i, cap := range a.capabilities {
		if string(cap) == name {
			a.capabilities = append(a.capabilities[:i], a.capabilities[i+1:]...)
			break
		}
	}
}

// CanHandleTaskType checks if the agent can handle a specific task type
func (a *BaseAgent) CanHandleTaskType(taskType string) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()

	for _, cap := range a.capabilities {
		if string(cap) == taskType {
			return true
		}
	}
	return false
}

// SubmitTask submits a task for execution
func (a *BaseAgent) SubmitTask(ctx context.Context, task *Task) (*TaskResult, error) {
	if task == nil {
		return nil, fmt.Errorf("task cannot be nil")
	}

	// Check if agent can handle this task type
	if !a.CanHandleTaskType(string(task.Type)) {
		return nil, fmt.Errorf("agent cannot handle task type: %s", task.Type)
	}

	// Set status to busy
	a.SetStatus(StatusBusy)

	// Create a context with timeout
	taskCtx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()

	// Execute the task
	startTime := time.Now()
	result, err := a.executeTask(taskCtx, task)
	duration := time.Since(startTime)

	// Update statistics
	a.mu.Lock()
	a.tasksProcessed++
	a.totalDuration += duration
	if err != nil {
		a.tasksFailed++
	} else {
		a.tasksSucceeded++
	}
	a.mu.Unlock()

	// Set status back to idle
	a.SetStatus(StatusIdle)

	if err != nil {
		return &TaskResult{
			TaskID:      task.ID,
			Success:     false,
			Error:       err.Error(),
			Duration:    duration,
			CompletedAt: time.Now(),
		}, nil
	}

	return &TaskResult{
		TaskID:      task.ID,
		Success:     true,
		Result:      result,
		Duration:    duration,
		CompletedAt: time.Now(),
	}, nil
}

// executeTask executes a task (to be implemented by subclasses)
func (a *BaseAgent) executeTask(ctx context.Context, task *Task) (interface{}, error) {
	// This is a placeholder implementation
	// Subclasses should override this method
	return map[string]interface{}{
		"message":   "Task executed successfully",
		"task_id":   task.ID,
		"task_type": task.Type,
	}, nil
}

// Start starts the agent
func (a *BaseAgent) Start(ctx context.Context) error {
	a.wg.Add(1)
	go a.processTasks(ctx)
	return nil
}

// Stop stops the agent
func (a *BaseAgent) Stop() {
	close(a.stopChan)
	a.wg.Wait()
}

// processTasks processes tasks from the queue
func (a *BaseAgent) processTasks(ctx context.Context) {
	defer a.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case <-a.stopChan:
			return
		case task := <-a.taskQueue:
			a.processTask(ctx, task)
		}
	}
}

// processTask processes a single task
func (a *BaseAgent) processTask(ctx context.Context, task *Task) {
	result, err := a.SubmitTask(ctx, task)

	select {
	case a.resultChan <- result:
	default:
		// Channel is full, log error
		fmt.Printf("Agent %s: result channel full, dropping result for task %s\n", a.id, task.ID)
	}

	if err != nil {
		fmt.Printf("Agent %s: error processing task %s: %v\n", a.id, task.ID, err)
	}
}

// Health returns the agent health status
func (a *BaseAgent) Health() map[string]interface{} {
	a.mu.RLock()
	defer a.mu.RUnlock()

	totalTasks := a.tasksProcessed
	successRate := float64(0)
	if totalTasks > 0 {
		successRate = float64(a.tasksSucceeded) / float64(totalTasks) * 100
	}

	avgDuration := time.Duration(0)
	if a.tasksProcessed > 0 {
		avgDuration = a.totalDuration / time.Duration(a.tasksProcessed)
	}

	return map[string]interface{}{
		"id":              a.id,
		"name":            a.name,
		"status":          string(a.status),
		"capabilities":    len(a.capabilities),
		"tasks_processed": a.tasksProcessed,
		"tasks_succeeded": a.tasksSucceeded,
		"tasks_failed":    a.tasksFailed,
		"success_rate":    successRate,
		"avg_duration":    avgDuration.String(),
		"last_activity":   a.lastActivity.Format(time.RFC3339),
		"queue_size":      len(a.taskQueue),
	}
}

// GetStatistics returns agent statistics
func (a *BaseAgent) GetStatistics() map[string]interface{} {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return map[string]interface{}{
		"tasks_processed": a.tasksProcessed,
		"tasks_succeeded": a.tasksSucceeded,
		"tasks_failed":    a.tasksFailed,
		"total_duration":  a.totalDuration.String(),
		"average_duration": func() string {
			if a.tasksProcessed == 0 {
				return "0s"
			}
			return (a.totalDuration / time.Duration(a.tasksProcessed)).String()
		}(),
		"success_rate": func() float64 {
			if a.tasksProcessed == 0 {
				return 0.0
			}
			return float64(a.tasksSucceeded) / float64(a.tasksProcessed) * 100
		}(),
		"last_activity": a.lastActivity.Format(time.RFC3339),
	}
}

// ResetStatistics resets the agent statistics
func (a *BaseAgent) ResetStatistics() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.tasksProcessed = 0
	a.tasksSucceeded = 0
	a.tasksFailed = 0
	a.totalDuration = 0
}

// IsHealthy checks if the agent is healthy
func (a *BaseAgent) IsHealthy() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()

	// Agent is healthy if it's not in error status and has been active recently
	if a.status == StatusError {
		return false
	}

	// Consider agent unhealthy if no activity for more than 5 minutes
	if time.Since(a.lastActivity) > 5*time.Minute {
		return false
	}

	return true
}

// UpdateLastActivity updates the last activity timestamp
func (a *BaseAgent) UpdateLastActivity() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.lastActivity = time.Now()
}

// GetTaskQueueSize returns the current task queue size
func (a *BaseAgent) GetTaskQueueSize() int {
	return len(a.taskQueue)
}

// GetMaxConcurrency returns the maximum concurrency
func (a *BaseAgent) GetMaxConcurrency() int {
	return a.maxConcurrency
}

// SetMaxConcurrency sets the maximum concurrency
func (a *BaseAgent) SetMaxConcurrency(concurrency int) {
	if concurrency > 0 {
		a.maxConcurrency = concurrency
	}
}

// GetTimeout returns the task timeout
func (a *BaseAgent) GetTimeout() time.Duration {
	return a.timeout
}

// SetTimeout sets the task timeout
func (a *BaseAgent) SetTimeout(timeout time.Duration) {
	if timeout > 0 {
		a.timeout = timeout
	}
}

// GetRetryCount returns the retry count
func (a *BaseAgent) GetRetryCount() int {
	return a.retryCount
}

// SetRetryCount sets the retry count
func (a *BaseAgent) SetRetryCount(count int) {
	if count >= 0 {
		a.retryCount = count
	}
}
