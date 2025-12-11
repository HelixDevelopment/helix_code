package task

import (
	"context"
	"fmt"
	"sync"
	"time"

	"dev.helix.code/internal/config"
)

// TaskStatus represents the status of a task
type TaskStatus string

const (
	// StatusPending indicates the task is waiting to be assigned
	StatusPending TaskStatus = "pending"
	// StatusAssigned indicates the task has been assigned to a worker
	StatusAssigned TaskStatus = "assigned"
	// StatusRunning indicates the task is currently running
	StatusRunning TaskStatus = "running"
	// StatusCompleted indicates the task has completed successfully
	StatusCompleted TaskStatus = "completed"
	// StatusFailed indicates the task has failed
	StatusFailed TaskStatus = "failed"
	// StatusCancelled indicates the task has been cancelled
	StatusCancelled TaskStatus = "cancelled"
)

// TaskPriority represents the priority of a task
type TaskPriority int

const (
	// PriorityLow indicates low priority
	PriorityLow TaskPriority = iota
	// PriorityNormal indicates normal priority
	PriorityNormal
	// PriorityHigh indicates high priority
	PriorityHigh
	// PriorityCritical indicates critical priority
	PriorityCritical
)

// Task represents a unit of work to be executed
type Task struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Priority    TaskPriority           `json:"priority"`
	Status      TaskStatus             `json:"status"`
	WorkerID    string                 `json:"worker_id,omitempty"`
	Payload     map[string]interface{} `json:"payload"`
	Result      interface{}            `json:"result,omitempty"`
	Error       string                 `json:"error,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	StartedAt   *time.Time             `json:"started_at,omitempty"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	Deadline    *time.Time             `json:"deadline,omitempty"`
	Progress    int                    `json:"progress"` // 0-100
	Retries     int                    `json:"retries"`
	MaxRetries  int                    `json:"max_retries"`
	Tags        []string               `json:"tags,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	mu          sync.RWMutex
}

// NewTask creates a new task
func NewTask(id, taskType, name string) *Task {
	now := time.Now()
	return &Task{
		ID:         id,
		Type:       taskType,
		Name:       name,
		Priority:   PriorityNormal,
		Status:     StatusPending,
		Payload:    make(map[string]interface{}),
		CreatedAt:  now,
		UpdatedAt:  now,
		Progress:   0,
		MaxRetries: 3,
		Tags:       []string{},
		Metadata:   make(map[string]interface{}),
	}
}

// UpdateStatus updates the task status
func (t *Task) UpdateStatus(status TaskStatus) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.Status = status
	t.UpdatedAt = time.Now()

	switch status {
	case StatusRunning:
		now := time.Now()
		t.StartedAt = &now
	case StatusCompleted, StatusFailed, StatusCancelled:
		now := time.Now()
		t.CompletedAt = &now
	}
}

// SetProgress sets the task progress
func (t *Task) SetProgress(progress int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}

	t.Progress = progress
	t.UpdatedAt = time.Now()
}

// IncrementRetries increments the retry count
func (t *Task) IncrementRetries() bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.Retries++
	t.UpdatedAt = time.Now()

	return t.Retries < t.MaxRetries
}

// CanRetry checks if the task can be retried
func (t *Task) CanRetry() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.Retries < t.MaxRetries
}

// IsExpired checks if the task has expired
func (t *Task) IsExpired() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.Deadline == nil {
		return false
	}

	return time.Now().After(*t.Deadline)
}

// Duration returns the task execution duration
func (t *Task) Duration() time.Duration {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.StartedAt == nil || t.CompletedAt == nil {
		return 0
	}

	return t.CompletedAt.Sub(*t.StartedAt)
}

// AssignToWorker assigns the task to a worker
func (t *Task) AssignToWorker(workerID string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.WorkerID = workerID
	t.Status = StatusAssigned
	t.UpdatedAt = time.Now()
}

// SetResult sets the task result
func (t *Task) SetResult(result interface{}) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.Result = result
	t.Progress = 100
	t.UpdatedAt = time.Now()
}

// SetError sets the task error
func (t *Task) SetError(err error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if err != nil {
		t.Error = err.Error()
	} else {
		t.Error = ""
	}
	t.UpdatedAt = time.Now()
}

// GetInfo returns task information
func (t *Task) GetInfo() map[string]interface{} {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return map[string]interface{}{
		"id":           t.ID,
		"type":         t.Type,
		"name":         t.Name,
		"description":  t.Description,
		"priority":     t.Priority,
		"status":       t.Status,
		"worker_id":    t.WorkerID,
		"progress":     t.Progress,
		"retries":      t.Retries,
		"max_retries":  t.MaxRetries,
		"created_at":   t.CreatedAt,
		"updated_at":   t.UpdatedAt,
		"started_at":   t.StartedAt,
		"completed_at": t.CompletedAt,
		"deadline":     t.Deadline,
		"tags":         t.Tags,
	}
}

// TaskManager manages tasks
type TaskManager struct {
	tasks    map[string]*Task
	queue    *TaskQueue
	workers  map[string]bool // available workers
	config   *config.TasksConfig
	stopChan chan struct{}
	wg       sync.WaitGroup
	mu       sync.RWMutex
}

// NewTaskManager creates a new task manager
func NewTaskManager(config *config.TasksConfig) *TaskManager {
	return &TaskManager{
		tasks:    make(map[string]*Task),
		queue:    NewTaskQueue(),
		workers:  make(map[string]bool),
		config:   config,
		stopChan: make(chan struct{}),
	}
}

// SubmitTask submits a new task
func (tm *TaskManager) SubmitTask(task *Task) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if _, exists := tm.tasks[task.ID]; exists {
		return fmt.Errorf("task with ID %s already exists", task.ID)
	}

	tm.tasks[task.ID] = task
	tm.queue.Enqueue(task)

	return nil
}

// GetTask gets a task by ID
func (tm *TaskManager) GetTask(taskID string) (*Task, bool) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	task, exists := tm.tasks[taskID]
	return task, exists
}

// CancelTask cancels a task
func (tm *TaskManager) CancelTask(taskID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, exists := tm.tasks[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	if task.Status == StatusCompleted || task.Status == StatusFailed {
		return fmt.Errorf("cannot cancel task in status %s", task.Status)
	}

	task.UpdateStatus(StatusCancelled)
	return nil
}

// RetryTask retries a failed task
func (tm *TaskManager) RetryTask(taskID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, exists := tm.tasks[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	if task.Status != StatusFailed {
		return fmt.Errorf("can only retry failed tasks")
	}

	if !task.CanRetry() {
		return fmt.Errorf("task has exceeded maximum retries")
	}

	// Reset task for retry
	task.Status = StatusPending
	task.Error = ""
	task.Progress = 0
	task.StartedAt = nil
	task.CompletedAt = nil
	task.UpdatedAt = time.Now()

	// Re-queue the task
	tm.queue.Enqueue(task)

	return nil
}

// GetTasksByStatus gets tasks by status
func (tm *TaskManager) GetTasksByStatus(status TaskStatus) []*Task {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	tasks := make([]*Task, 0)
	for _, task := range tm.tasks {
		if task.Status == status {
			tasks = append(tasks, task)
		}
	}

	return tasks
}

// GetPendingTasks gets all pending tasks
func (tm *TaskManager) GetPendingTasks() []*Task {
	return tm.GetTasksByStatus(StatusPending)
}

// GetRunningTasks gets all running tasks
func (tm *TaskManager) GetRunningTasks() []*Task {
	return tm.GetTasksByStatus(StatusRunning)
}

// GetCompletedTasks gets all completed tasks
func (tm *TaskManager) GetCompletedTasks() []*Task {
	return tm.GetTasksByStatus(StatusCompleted)
}

// GetFailedTasks gets all failed tasks
func (tm *TaskManager) GetFailedTasks() []*Task {
	return tm.GetTasksByStatus(StatusFailed)
}

// RegisterWorker registers a worker
func (tm *TaskManager) RegisterWorker(workerID string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.workers[workerID] = true
}

// UnregisterWorker unregisters a worker
func (tm *TaskManager) UnregisterWorker(workerID string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	delete(tm.workers, workerID)
}

// GetAvailableWorkers returns available workers
func (tm *TaskManager) GetAvailableWorkers() []string {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	workers := make([]string, 0, len(tm.workers))
	for workerID, available := range tm.workers {
		if available {
			workers = append(workers, workerID)
		}
	}

	return workers
}

// AssignTask assigns a task to a worker
func (tm *TaskManager) AssignTask(ctx context.Context, workerID string) (*Task, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Check if worker is available
	if available, exists := tm.workers[workerID]; !exists || !available {
		return nil, fmt.Errorf("worker %s is not available", workerID)
	}

	// Get next task from queue
	task := tm.queue.Dequeue()
	if task == nil {
		return nil, fmt.Errorf("no tasks available")
	}

	// Assign task to worker
	task.AssignToWorker(workerID)
	task.UpdateStatus(StatusAssigned)

	// Mark worker as busy
	tm.workers[workerID] = false

	return task, nil
}

// CompleteTask marks a task as completed
func (tm *TaskManager) CompleteTask(taskID string, result interface{}) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, exists := tm.tasks[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	task.SetResult(result)
	task.UpdateStatus(StatusCompleted)

	// Free up the worker
	if task.WorkerID != "" {
		tm.workers[task.WorkerID] = true
	}

	return nil
}

// FailTask marks a task as failed
func (tm *TaskManager) FailTask(taskID string, err error) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, exists := tm.tasks[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	task.SetError(err)
	task.UpdateStatus(StatusFailed)

	// Free up the worker
	if task.WorkerID != "" {
		tm.workers[task.WorkerID] = true
	}

	return nil
}

// GetStatistics returns task manager statistics
func (tm *TaskManager) GetStatistics() map[string]interface{} {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	stats := map[string]interface{}{
		"total_tasks":       len(tm.tasks),
		"pending_tasks":     len(tm.GetTasksByStatus(StatusPending)),
		"running_tasks":     len(tm.GetTasksByStatus(StatusRunning)),
		"completed_tasks":   len(tm.GetTasksByStatus(StatusCompleted)),
		"failed_tasks":      len(tm.GetTasksByStatus(StatusFailed)),
		"cancelled_tasks":   len(tm.GetTasksByStatus(StatusCancelled)),
		"available_workers": len(tm.GetAvailableWorkers()),
		"queue_size":        tm.queue.Size(),
	}

	return stats
}

// Start starts the task manager
func (tm *TaskManager) Start(ctx context.Context) error {
	tm.wg.Add(1)
	go tm.taskProcessor(ctx)
	return nil
}

// Stop stops the task manager
func (tm *TaskManager) Stop() {
	close(tm.stopChan)
	tm.wg.Wait()
}

// taskProcessor processes tasks
func (tm *TaskManager) taskProcessor(ctx context.Context) {
	defer tm.wg.Done()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-tm.stopChan:
			return
		case <-ticker.C:
			tm.processExpiredTasks()
		}
	}
}

// processExpiredTasks processes expired tasks
func (tm *TaskManager) processExpiredTasks() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	for _, task := range tm.tasks {
		if task.Status == StatusPending || task.Status == StatusAssigned || task.Status == StatusRunning {
			if task.IsExpired() {
				task.UpdateStatus(StatusFailed)
				task.SetError(fmt.Errorf("task expired"))

				// Free up worker if assigned
				if task.WorkerID != "" {
					tm.workers[task.WorkerID] = true
				}
			}
		}
	}
}

// TaskQueue represents a priority queue for tasks
type TaskQueue struct {
	tasks []*Task
	mu    sync.RWMutex
}

// NewTaskQueue creates a new task queue
func NewTaskQueue() *TaskQueue {
	return &TaskQueue{
		tasks: make([]*Task, 0),
	}
}

// Enqueue adds a task to the queue
func (tq *TaskQueue) Enqueue(task *Task) {
	tq.mu.Lock()
	defer tq.mu.Unlock()

	// Insert based on priority (higher priority first)
	insertIndex := len(tq.tasks)
	for i, t := range tq.tasks {
		if task.Priority > t.Priority {
			insertIndex = i
			break
		}
	}

	// Insert at the correct position
	tq.tasks = append(tq.tasks[:insertIndex], append([]*Task{task}, tq.tasks[insertIndex:]...)...)
}

// Dequeue removes and returns the highest priority task
func (tq *TaskQueue) Dequeue() *Task {
	tq.mu.Lock()
	defer tq.mu.Unlock()

	if len(tq.tasks) == 0 {
		return nil
	}

	task := tq.tasks[0]
	tq.tasks = tq.tasks[1:]
	return task
}

// Peek returns the highest priority task without removing it
func (tq *TaskQueue) Peek() *Task {
	tq.mu.RLock()
	defer tq.mu.RUnlock()

	if len(tq.tasks) == 0 {
		return nil
	}

	return tq.tasks[0]
}

// Size returns the queue size
func (tq *TaskQueue) Size() int {
	tq.mu.RLock()
	defer tq.mu.RUnlock()
	return len(tq.tasks)
}

// IsEmpty checks if the queue is empty
func (tq *TaskQueue) IsEmpty() bool {
	return tq.Size() == 0
}

// Clear clears the queue
func (tq *TaskQueue) Clear() {
	tq.mu.Lock()
	defer tq.mu.Unlock()
	tq.tasks = make([]*Task, 0)
}

// Global task manager instance
var globalManager *TaskManager

// GetGlobalManager returns the global task manager
func GetGlobalManager() *TaskManager {
	return globalManager
}

// SetGlobalManager sets the global task manager
func SetGlobalManager(manager *TaskManager) {
	globalManager = manager
}

// InitializeGlobalManager initializes the global task manager
func InitializeGlobalManager(config *config.TasksConfig) {
	globalManager = NewTaskManager(config)
}

// SubmitTaskGlobal submits a task using the global manager
func SubmitTaskGlobal(task *Task) error {
	if globalManager == nil {
		return fmt.Errorf("global task manager not initialized")
	}
	return globalManager.SubmitTask(task)
}

// GetTaskGlobal gets a task using the global manager
func GetTaskGlobal(taskID string) (*Task, bool) {
	if globalManager == nil {
		return nil, false
	}
	return globalManager.GetTask(taskID)
}

// AssignTaskGlobal assigns a task using the global manager
func AssignTaskGlobal(ctx context.Context, workerID string) (*Task, error) {
	if globalManager == nil {
		return nil, fmt.Errorf("global task manager not initialized")
	}
	return globalManager.AssignTask(ctx, workerID)
}
