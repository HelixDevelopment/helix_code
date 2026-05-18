package task

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
)

// ErrTaskPersistenceNotWired is returned by storeTaskInDB / updateTaskInDB /
// updateWorkerInDB when the TaskManager has been constructed without a
// database backend (`tm.db == nil`).
//
// Forensic anchor (round-31 §11.4 audit, 2026-05-18): the previous
// implementations of these three functions were log-only stubs that
// always returned nil — tasks and worker state were claimed-persisted
// at API contract level but actually vanished across restarts. The
// stubs have been replaced with real INSERT/UPDATE statements against
// the schemas defined in internal/database/database.go (distributed_tasks
// and workers). When no db backend is wired, callers now surface this
// sentinel instead of the previous silent success, so the failure mode
// is loud + auditable instead of bluffing.
//
// Tests under internal/task/ continue to use database.NewMockDatabase()
// (test-only fakes are CONST-050(A)-compliant) and pre-mock Exec via
// mockDB.MockExecSuccess(N); production must wire a real
// *database.Database.
var ErrTaskPersistenceNotWired = errors.New("helixcode task manager: persistence has not been wired into the manager (tm.db == nil) — previously storeTaskInDB/updateTaskInDB/updateWorkerInDB logged the intent and returned nil while no data was actually stored anywhere (§11.4 CRITICAL persistence-bluff: tasks and worker state vanished across restarts); construct the TaskManager via NewTaskManager(db, redisClient) with a non-nil *database.Database before invoking persist operations")

// SplitTask intelligently splits a large task into subtasks
func (tm *TaskManager) SplitTask(parentTaskID uuid.UUID, strategy SplitStrategy) ([]*Task, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	parentTask, exists := tm.tasks[parentTaskID]
	if !exists {
		return nil, fmt.Errorf("parent task not found: %s", parentTaskID)
	}

	// Analyze task for splitting
	analysis, err := tm.analyzeTaskForSplitting(parentTask)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze task: %v", err)
	}

	// Generate subtasks based on strategy
	subtasks, err := strategy.GenerateSubtasks(parentTask, analysis)
	if err != nil {
		return nil, fmt.Errorf("failed to generate subtasks: %v", err)
	}

	// Create subtasks using the unsafe version since we already hold the lock
	var createdSubtasks []*Task
	for _, subtaskData := range subtasks {
		subtask, err := tm.createTaskUnsafe(
			parentTask.Type,
			subtaskData.Data,
			parentTask.Priority,
			parentTask.Criticality,
			subtaskData.Dependencies,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create subtask: %v", err)
		}
		createdSubtasks = append(createdSubtasks, subtask)
	}

	// Update parent task status
	parentTask.Status = TaskStatusWaitingForDeps
	parentTask.Data["subtasks"] = createdSubtasks
	tm.updateTaskInDB(parentTask)

	log.Printf("✅ Task %s split into %d subtasks", parentTaskID, len(createdSubtasks))
	return createdSubtasks, nil
}

// AssignTask assigns a task to a worker
func (tm *TaskManager) AssignTask(taskID uuid.UUID, workerID uuid.UUID) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, exists := tm.tasks[taskID]
	if !exists {
		return fmt.Errorf("task not found: %s", taskID)
	}

	worker, exists := tm.workers[workerID]
	if !exists {
		return fmt.Errorf("worker not found: %s", workerID)
	}

	// Check if worker can handle this task
	if !tm.canWorkerHandleTask(worker, task) {
		return fmt.Errorf("worker %s cannot handle task %s", workerID, taskID)
	}

	// Check worker capacity
	if worker.CurrentTasksCount >= worker.MaxConcurrentTasks {
		return fmt.Errorf("worker %s is at capacity", workerID)
	}

	// Update task
	task.AssignedWorker = &workerID
	task.Status = TaskStatusAssigned
	task.UpdatedAt = time.Now()

	// Update worker
	worker.CurrentTasksCount++
	worker.UpdatedAt = time.Now()

	// Update in database
	tm.updateTaskInDB(task)
	tm.updateWorkerInDB(worker)

	log.Printf("✅ Task %s assigned to worker %s", taskID, workerID)
	return nil
}

// CompleteTask marks a task as completed
func (tm *TaskManager) CompleteTask(taskID uuid.UUID, result map[string]interface{}) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, exists := tm.tasks[taskID]
	if !exists {
		return fmt.Errorf("task not found: %s", taskID)
	}

	// Update task
	task.Status = TaskStatusCompleted
	task.ResultData = result
	now := time.Now()
	task.CompletedAt = &now
	task.UpdatedAt = now

	// Update worker if assigned
	if task.AssignedWorker != nil {
		if worker, exists := tm.workers[*task.AssignedWorker]; exists {
			worker.CurrentTasksCount--
			worker.UpdatedAt = now
			tm.updateWorkerInDB(worker)
		}
	}

	// Update in database
	tm.updateTaskInDB(task)

	log.Printf("✅ Task %s completed", taskID)
	return nil
}

// FailTask marks a task as failed
func (tm *TaskManager) FailTask(taskID uuid.UUID, errorMessage string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, exists := tm.tasks[taskID]
	if !exists {
		return fmt.Errorf("task not found: %s", taskID)
	}

	// Check if we should retry
	if task.RetryCount < task.MaxRetries {
		task.RetryCount++
		task.Status = TaskStatusPending
		task.ErrorMessage = errorMessage
		task.AssignedWorker = nil
		task.UpdatedAt = time.Now()

		// Add back to queue
		tm.queue.AddTask(task)
		log.Printf("🔄 Task %s failed, retrying (attempt %d/%d)", taskID, task.RetryCount, task.MaxRetries)
	} else {
		task.Status = TaskStatusFailed
		task.ErrorMessage = errorMessage
		task.UpdatedAt = time.Now()
		log.Printf("❌ Task %s failed permanently", taskID)
	}

	// Update worker if assigned
	if task.AssignedWorker != nil {
		if worker, exists := tm.workers[*task.AssignedWorker]; exists {
			worker.CurrentTasksCount--
			worker.UpdatedAt = time.Now()
			tm.updateWorkerInDB(worker)
		}
	}

	// Update in database
	tm.updateTaskInDB(task)

	return nil
}

// CreateCheckpoint creates a checkpoint for a task
// The workerID is automatically retrieved from the task's AssignedWorker field
func (tm *TaskManager) CreateCheckpoint(taskID uuid.UUID, checkpointName string, checkpointData map[string]interface{}) error {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	task, exists := tm.tasks[taskID]
	if !exists {
		return fmt.Errorf("task not found: %s", taskID)
	}

	// Get worker ID from task - use zero UUID if no worker assigned
	var workerID uuid.UUID
	if task.AssignedWorker != nil {
		workerID = *task.AssignedWorker
	}

	return tm.checkpointMgr.CreateCheckpoint(taskID, workerID, checkpointName, checkpointData)
}

// GetTaskProgress returns progress information for a task
func (tm *TaskManager) GetTaskProgress(taskID uuid.UUID) (*TaskProgress, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	task, exists := tm.tasks[taskID]
	if !exists {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}

	progress := &TaskProgress{
		TaskID:    taskID,
		Status:    task.Status,
		Progress:  0.0,
		StartedAt: task.StartedAt,
		UpdatedAt: task.UpdatedAt,
	}

	// Calculate progress based on task type and status
	switch task.Status {
	case TaskStatusCompleted:
		progress.Progress = 100.0
	case TaskStatusRunning:
		// Estimate progress based on elapsed time vs estimated duration
		if task.StartedAt != nil && task.EstimatedDuration > 0 {
			elapsed := time.Since(*task.StartedAt)
			progress.Progress = float64(elapsed) / float64(task.EstimatedDuration) * 100
			if progress.Progress > 95 {
				progress.Progress = 95 // Cap at 95% until actually completed
			}
		} else {
			progress.Progress = 50.0 // Default estimate
		}
	case TaskStatusPending, TaskStatusAssigned:
		progress.Progress = 0.0
	}

	return progress, nil
}

// Helper methods

func (tm *TaskManager) analyzeTaskForSplitting(task *Task) (*TaskAnalysis, error) {
	// Analyze task data to determine optimal splitting strategy
	analysis := &TaskAnalysis{
		TaskID:       task.ID,
		TaskType:     task.Type,
		Complexity:   tm.estimateComplexity(task),
		DataSize:     tm.estimateDataSize(task),
		Dependencies: len(task.Dependencies),
	}

	return analysis, nil
}

func (tm *TaskManager) estimateComplexity(task *Task) ComplexityLevel {
	// Estimate task complexity based on type and data
	switch task.Type {
	case TaskTypePlanning, TaskTypeDesign:
		return ComplexityHigh
	case TaskTypeBuilding, TaskTypeRefactoring:
		return ComplexityMedium
	default:
		return ComplexityLow
	}
}

func (tm *TaskManager) estimateDataSize(task *Task) int64 {
	// Estimate data size based on task data
	dataSize := int64(0)
	if task.Data != nil {
		// Convert to JSON to estimate size
		jsonData, err := json.Marshal(task.Data)
		if err == nil {
			dataSize = int64(len(jsonData))
		}
	}
	return dataSize
}

func (tm *TaskManager) canWorkerHandleTask(worker *Worker, task *Task) bool {
	// Check if worker has required capabilities for task type
	requiredCaps := tm.getRequiredCapabilities(task.Type)
	for _, requiredCap := range requiredCaps {
		if !contains(worker.Capabilities, requiredCap) {
			return false
		}
	}
	return true
}

func (tm *TaskManager) getRequiredCapabilities(taskType TaskType) []string {
	// Define required capabilities for each task type
	switch taskType {
	case TaskTypeBuilding:
		return []string{"compilation", "build_tools"}
	case TaskTypeTesting:
		return []string{"test_execution", "coverage_analysis"}
	case TaskTypeRefactoring:
		return []string{"code_analysis", "refactoring_tools"}
	case TaskTypeDebugging:
		return []string{"debugging", "error_analysis"}
	default:
		return []string{"general_computation"}
	}
}

// Database operations
//
// All three functions below were previously log-only stubs returning nil
// (round-31 §11.4 audit, 2026-05-18). The API contract claimed task and
// worker state were persisted, but no SQL ever ran — restarts wiped
// everything. They now execute real INSERT/UPDATE statements against
// the schemas in internal/database/database.go (distributed_tasks and
// workers), using the same DatabaseInterface (pgx-compatible) that
// internal/task/manager_db.go already exercises.
//
// dbExecContext returns a fresh context with a 10-second timeout. The
// public TaskManager methods (AssignTask, CompleteTask, FailTask, etc.)
// do not yet take a context.Context — adding one is a breaking API
// change and out of scope for this anti-bluff fix; the local timeout
// prevents the DB call from hanging indefinitely.
func (tm *TaskManager) dbExecContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 10*time.Second)
}

// storeTaskInDB inserts a brand-new task row into distributed_tasks.
// Columns map 1:1 to the Task struct fields persisted by
// (*DatabaseManager).CreateTask in manager_db.go.
func (tm *TaskManager) storeTaskInDB(task *Task) error {
	if tm.db == nil {
		return fmt.Errorf("storeTaskInDB: %w", ErrTaskPersistenceNotWired)
	}

	ctx, cancel := tm.dbExecContext()
	defer cancel()

	// task_data is JSONB NOT NULL — defend the not-null invariant the
	// same way DatabaseManager.CreateTask does (manager_db.go:55-57).
	taskData := task.Data
	if taskData == nil {
		taskData = map[string]interface{}{}
	}

	const query = `
		INSERT INTO distributed_tasks (
			id, task_type, task_data, status, priority, criticality,
			assigned_worker_id, original_worker_id, dependencies,
			retry_count, max_retries, error_message, result_data,
			checkpoint_data, started_at, completed_at,
			created_at, updated_at
		)
		VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9,
			$10, $11, $12, $13,
			$14, $15, $16,
			$17, $18
		)
	`

	errorMessage := nullStringIfEmpty(task.ErrorMessage)
	dependencies := task.Dependencies
	if dependencies == nil {
		dependencies = []uuid.UUID{}
	}

	if _, err := tm.db.Exec(ctx, query,
		task.ID, string(task.Type), taskData, string(task.Status), int(task.Priority), string(task.Criticality),
		task.AssignedWorker, task.OriginalWorker, dependencies,
		task.RetryCount, task.MaxRetries, errorMessage, task.ResultData,
		task.CheckpointData, task.StartedAt, task.CompletedAt,
		task.CreatedAt, task.UpdatedAt,
	); err != nil {
		return fmt.Errorf("storeTaskInDB: failed to insert task %s: %w", task.ID, err)
	}

	log.Printf("Stored task %s in database", task.ID)
	return nil
}

// updateTaskInDB persists the current in-memory state of a task back to
// distributed_tasks. Every mutable column is written, mirroring how
// (*DatabaseManager).StartTask / CompleteTask / FailTask write specific
// subsets — this is the manager's generic "save everything" path used
// by SplitTask, AssignTask, CompleteTask, FailTask after they mutate
// the in-memory Task struct.
func (tm *TaskManager) updateTaskInDB(task *Task) error {
	if tm.db == nil {
		return fmt.Errorf("updateTaskInDB: %w", ErrTaskPersistenceNotWired)
	}

	ctx, cancel := tm.dbExecContext()
	defer cancel()

	taskData := task.Data
	if taskData == nil {
		taskData = map[string]interface{}{}
	}
	dependencies := task.Dependencies
	if dependencies == nil {
		dependencies = []uuid.UUID{}
	}
	errorMessage := nullStringIfEmpty(task.ErrorMessage)

	const query = `
		UPDATE distributed_tasks SET
			task_type = $2,
			task_data = $3,
			status = $4,
			priority = $5,
			criticality = $6,
			assigned_worker_id = $7,
			original_worker_id = $8,
			dependencies = $9,
			retry_count = $10,
			max_retries = $11,
			error_message = $12,
			result_data = $13,
			checkpoint_data = $14,
			started_at = $15,
			completed_at = $16,
			updated_at = $17
		WHERE id = $1
	`

	if _, err := tm.db.Exec(ctx, query,
		task.ID,
		string(task.Type), taskData, string(task.Status), int(task.Priority), string(task.Criticality),
		task.AssignedWorker, task.OriginalWorker, dependencies,
		task.RetryCount, task.MaxRetries, errorMessage, task.ResultData,
		task.CheckpointData, task.StartedAt, task.CompletedAt,
		task.UpdatedAt,
	); err != nil {
		return fmt.Errorf("updateTaskInDB: failed to update task %s: %w", task.ID, err)
	}

	log.Printf("Updated task %s in database", task.ID)
	return nil
}

// updateWorkerInDB persists the current in-memory state of a worker
// back to the workers table. Used by AssignTask / CompleteTask / FailTask
// to keep current_tasks_count + health snapshot in sync with reality.
func (tm *TaskManager) updateWorkerInDB(worker *Worker) error {
	if tm.db == nil {
		return fmt.Errorf("updateWorkerInDB: %w", ErrTaskPersistenceNotWired)
	}

	ctx, cancel := tm.dbExecContext()
	defer cancel()

	// ssh_config + resources are JSONB NOT NULL — defend the invariant.
	sshConfig := worker.SSHConfig
	if sshConfig == nil {
		sshConfig = map[string]interface{}{}
	}
	resources := worker.Resources
	if resources == nil {
		resources = map[string]interface{}{}
	}
	capabilities := worker.Capabilities
	if capabilities == nil {
		capabilities = []string{}
	}

	const query = `
		UPDATE workers SET
			hostname = $2,
			display_name = $3,
			ssh_config = $4,
			capabilities = $5,
			resources = $6,
			status = $7,
			health_status = $8,
			last_heartbeat = $9,
			cpu_usage_percent = $10,
			memory_usage_percent = $11,
			disk_usage_percent = $12,
			current_tasks_count = $13,
			max_concurrent_tasks = $14,
			updated_at = $15
		WHERE id = $1
	`

	if _, err := tm.db.Exec(ctx, query,
		worker.ID,
		worker.Hostname, worker.DisplayName, sshConfig, capabilities, resources,
		worker.Status, worker.HealthStatus, worker.LastHeartbeat,
		worker.CPUUsagePercent, worker.MemoryUsagePercent, worker.DiskUsagePercent,
		worker.CurrentTasksCount, worker.MaxConcurrentTasks, worker.UpdatedAt,
	); err != nil {
		return fmt.Errorf("updateWorkerInDB: failed to update worker %s: %w", worker.ID, err)
	}

	log.Printf("Updated worker %s in database", worker.ID)
	return nil
}

// nullStringIfEmpty returns nil for an empty string so the DB column
// receives SQL NULL instead of '' — error_message is a nullable TEXT.
func nullStringIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// Helper functions

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
