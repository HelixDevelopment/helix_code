package task

import (
	"context"
	"errors"
	"fmt"
	"time"

	"dev.helix.code/internal/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// DatabaseManager handles task lifecycle and operations with database persistence
type DatabaseManager struct {
	db database.DatabaseInterface
}

// NewDatabaseManager creates a new task manager with database persistence
func NewDatabaseManager(db database.DatabaseInterface) *DatabaseManager {
	return &DatabaseManager{
		db: db,
	}
}

// CreateTask creates a new task with database persistence
func (m *DatabaseManager) CreateTask(ctx context.Context, name, description, taskType, priority string, parameters map[string]interface{}, dependencies []string) (*Task, error) {
	// Convert priority string to TaskPriority
	var taskPriority TaskPriority
	switch priority {
	case "high":
		taskPriority = PriorityHigh
	case "critical":
		taskPriority = PriorityCritical
	case "low":
		taskPriority = PriorityLow
	default:
		taskPriority = PriorityNormal
	}

	// Convert dependencies to UUIDs
	var dependencyUUIDs []uuid.UUID
	for _, dep := range dependencies {
		if depUUID, err := uuid.Parse(dep); err == nil {
			dependencyUUIDs = append(dependencyUUIDs, depUUID)
		}
	}

	// task_data is JSONB NOT NULL in the distributed_tasks schema. When
	// the caller omits parameters (or passes an empty body), `parameters`
	// arrives as a nil map, which pgx serializes to SQL NULL — triggering
	// a "null value in column task_data violates not-null constraint"
	// error at INSERT time. Default to an empty JSON object so the
	// not-null invariant always holds.
	if parameters == nil {
		parameters = map[string]interface{}{}
	}

	task := &Task{
		ID:           uuid.New(),
		Type:         TaskType(taskType),
		Data:         parameters,
		Status:       TaskStatusPending,
		Priority:     taskPriority,
		Criticality:  CriticalityNormal,
		Dependencies: dependencyUUIDs,
		MaxRetries:   3,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// Insert into database
	query := `
		INSERT INTO distributed_tasks (
			id, task_type, task_data, status, priority, criticality, 
			dependencies, max_retries, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING created_at, updated_at
	`

	var createdAt, updatedAt time.Time
	err := m.db.QueryRow(ctx, query,
		task.ID, task.Type, task.Data, task.Status, task.Priority, task.Criticality,
		task.Dependencies, task.MaxRetries, task.CreatedAt, task.UpdatedAt,
	).Scan(&createdAt, &updatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to create task in database: %v", err)
	}

	task.CreatedAt = createdAt
	task.UpdatedAt = updatedAt

	return task, nil
}

// GetTask retrieves a task by ID from database
func (m *DatabaseManager) GetTask(ctx context.Context, id string) (*Task, error) {
	taskID, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("invalid task ID: %v", err)
	}

	query := `
		SELECT 
			id, task_type, task_data, status, priority, criticality,
			assigned_worker_id, original_worker_id, dependencies,
			retry_count, max_retries, error_message, result_data,
			checkpoint_data, estimated_duration, started_at, completed_at,
			created_at, updated_at
		FROM distributed_tasks
		WHERE id = $1
	`

	var (
		dbID              uuid.UUID
		taskType          string
		taskData          map[string]interface{}
		status            string
		priority          int
		criticality       string
		assignedWorkerID  *uuid.UUID
		originalWorkerID  *uuid.UUID
		dependencies      []uuid.UUID
		retryCount        int
		maxRetries        int
		errorMessage      *string
		resultData        map[string]interface{}
		checkpointData    map[string]interface{}
		estimatedDuration *string
		startedAt         *time.Time
		completedAt       *time.Time
		createdAt         time.Time
		updatedAt         time.Time
	)

	err = m.db.QueryRow(ctx, query, taskID).Scan(
		&dbID, &taskType, &taskData, &status, &priority, &criticality,
		&assignedWorkerID, &originalWorkerID, &dependencies,
		&retryCount, &maxRetries, &errorMessage, &resultData,
		&checkpointData, &estimatedDuration, &startedAt, &completedAt,
		&createdAt, &updatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("task not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get task from database: %v", err)
	}

	// Convert priority int to TaskPriority
	var taskPriority TaskPriority
	switch priority {
	case 1:
		taskPriority = PriorityLow
	case 5:
		taskPriority = PriorityNormal
	case 10:
		taskPriority = PriorityHigh
	case 20:
		taskPriority = PriorityCritical
	default:
		taskPriority = PriorityNormal
	}

	task := &Task{
		ID:             dbID,
		Type:           TaskType(taskType),
		Data:           taskData,
		Status:         TaskStatus(status),
		Priority:       taskPriority,
		Criticality:    TaskCriticality(criticality),
		// Anti-bluff (CONST-035): previously these two fields were
		// scanned from the DB row (lines 123-124) but never assigned
		// to the returned Task struct. Every response that included
		// a task showed `"assigned_worker": null` even after a
		// successful POST /tasks/:id/assign — silently lying about
		// the assignment state. Real persisted state in
		// `assigned_worker_id` column was correct; only the JSON
		// response was wrong.
		AssignedWorker: assignedWorkerID,
		OriginalWorker: originalWorkerID,
		Dependencies:   dependencies,
		RetryCount:     retryCount,
		MaxRetries:     maxRetries,
		ErrorMessage:   getStringFromPtr(errorMessage),
		ResultData:     resultData,
		CheckpointData: checkpointData,
		StartedAt:      startedAt,
		CompletedAt:    completedAt,
		CreatedAt:      createdAt,
		UpdatedAt:      updatedAt,
	}

	return task, nil
}

// ListTasks returns all tasks from database
func (m *DatabaseManager) ListTasks(ctx context.Context) ([]*Task, error) {
	query := `
		SELECT 
			id, task_type, task_data, status, priority, criticality,
			assigned_worker_id, original_worker_id, dependencies,
			retry_count, max_retries, error_message, result_data,
			checkpoint_data, estimated_duration, started_at, completed_at,
			created_at, updated_at
		FROM distributed_tasks
		ORDER BY created_at DESC
	`

	rows, err := m.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query tasks: %v", err)
	}
	defer rows.Close()

	var tasks []*Task
	for rows.Next() {
		var (
			dbID              uuid.UUID
			taskType          string
			taskData          map[string]interface{}
			status            string
			priority          int
			criticality       string
			assignedWorkerID  *uuid.UUID
			originalWorkerID  *uuid.UUID
			dependencies      []uuid.UUID
			retryCount        int
			maxRetries        int
			errorMessage      *string
			resultData        map[string]interface{}
			checkpointData    map[string]interface{}
			estimatedDuration *string
			startedAt         *time.Time
			completedAt       *time.Time
			createdAt         time.Time
			updatedAt         time.Time
		)

		if err := rows.Scan(
			&dbID, &taskType, &taskData, &status, &priority, &criticality,
			&assignedWorkerID, &originalWorkerID, &dependencies,
			&retryCount, &maxRetries, &errorMessage, &resultData,
			&checkpointData, &estimatedDuration, &startedAt, &completedAt,
			&createdAt, &updatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan task row: %v", err)
		}

		// Convert priority int to TaskPriority
		var taskPriority TaskPriority
		switch priority {
		case 1:
			taskPriority = PriorityLow
		case 5:
			taskPriority = PriorityNormal
		case 10:
			taskPriority = PriorityHigh
		case 20:
			taskPriority = PriorityCritical
		default:
			taskPriority = PriorityNormal
		}

		// Convert nullable fields
		var errorMsg string
		if errorMessage != nil {
			errorMsg = *errorMessage
		}

		task := &Task{
			ID:             dbID,
			Type:           TaskType(taskType),
			Data:           taskData,
			Status:         TaskStatus(status),
			Priority:       taskPriority,
			Criticality:    TaskCriticality(criticality),
			// Same bug as GetTask above — pulled from DB, never assigned.
			AssignedWorker: assignedWorkerID,
			OriginalWorker: originalWorkerID,
			Dependencies:   dependencies,
			RetryCount:     retryCount,
			MaxRetries:     maxRetries,
			ErrorMessage:   errorMsg,
			ResultData:     resultData,
			CheckpointData: checkpointData,
			StartedAt:      startedAt,
			CompletedAt:    completedAt,
			CreatedAt:      createdAt,
			UpdatedAt:      updatedAt,
		}

		tasks = append(tasks, task)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating task rows: %v", err)
	}

	return tasks, nil
}

// StartTask marks a task as running
func (m *DatabaseManager) StartTask(ctx context.Context, id string) error {
	taskID, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid task ID: %v", err)
	}

	query := `
		UPDATE distributed_tasks 
		SET status = 'running', started_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND status = 'pending'
	`

	result, err := m.db.Exec(ctx, query, taskID)
	if err != nil {
		return fmt.Errorf("failed to start task: %v", err)
	}

	if result.RowsAffected() == 0 {
		// Wrap the sentinel so handlers can map to 422 (not 500).
		return fmt.Errorf("%w: task %s not in pending state", ErrTaskInvalidStateTransition, id)
	}

	return nil
}

// CompleteTask marks a task as completed
func (m *DatabaseManager) CompleteTask(ctx context.Context, id string, result map[string]interface{}) error {
	taskID, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid task ID: %v", err)
	}

	query := `
		UPDATE distributed_tasks
		SET status = 'completed', result_data = $1, completed_at = NOW(), updated_at = NOW()
		WHERE id = $2 AND status = 'running'
	`

	execResult, err := m.db.Exec(ctx, query, result, taskID)
	if err != nil {
		return fmt.Errorf("failed to complete task: %v", err)
	}

	if execResult.RowsAffected() == 0 {
		// Wrap the sentinel so handlers can map to 422 (not 500).
		return fmt.Errorf("%w: task %s not in running state", ErrTaskInvalidStateTransition, id)
	}

	return nil
}

// FailTask marks a task as failed
func (m *DatabaseManager) FailTask(ctx context.Context, id, errorMessage string) error {
	taskID, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid task ID: %v", err)
	}

	query := `
		UPDATE distributed_tasks 
		SET status = 'failed', error_message = $1, updated_at = NOW()
		WHERE id = $2
	`

	result, err := m.db.Exec(ctx, query, errorMessage, taskID)
	if err != nil {
		return fmt.Errorf("failed to mark task as failed: %v", err)
	}

	if result.RowsAffected() == 0 {
		// Wrap sentinel so handler can map to 404 instead of 500.
		return fmt.Errorf("%w: %s", ErrTaskNotFound, id)
	}

	return nil
}

// DeleteTask deletes a task from database
func (m *DatabaseManager) DeleteTask(ctx context.Context, id string) error {
	taskID, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid task ID: %v", err)
	}

	query := `DELETE FROM distributed_tasks WHERE id = $1`

	result, err := m.db.Exec(ctx, query, taskID)
	if err != nil {
		return fmt.Errorf("failed to delete task: %v", err)
	}

	if result.RowsAffected() == 0 {
		// Wrap sentinel so handler can map to 404 instead of 500.
		return fmt.Errorf("%w: %s", ErrTaskNotFound, id)
	}

	return nil
}

// AssignTask assigns a task to a worker
func (m *DatabaseManager) AssignTask(ctx context.Context, taskID, workerID string) error {
	taskUUID, err := uuid.Parse(taskID)
	if err != nil {
		return fmt.Errorf("invalid task ID: %v", err)
	}

	workerUUID, err := uuid.Parse(workerID)
	if err != nil {
		return fmt.Errorf("invalid worker ID: %v", err)
	}

	query := `
		UPDATE distributed_tasks
		SET status = 'assigned', assigned_worker_id = $1, updated_at = NOW()
		WHERE id = $2 AND status = 'pending'
	`

	result, err := m.db.Exec(ctx, query, workerUUID, taskUUID)
	if err != nil {
		return fmt.Errorf("failed to assign task: %v", err)
	}

	if result.RowsAffected() == 0 {
		// Wrap the sentinel so handlers can map to 422 (not 500) for
		// the assign-on-already-assigned case (BUG #23, same family
		// as #13 retry and #21 start/complete).
		return fmt.Errorf("%w: task %s not in pending state for assign", ErrTaskInvalidStateTransition, taskID)
	}

	return nil
}

// ErrTaskNotRetryable is returned by RetryTask when the target task
// doesn't exist, isn't in the failed state, or has exhausted its retry
// budget. It is a CLIENT-side condition (the request asked for an
// invalid state transition), NOT a server-side fault — handlers MUST
// map it to a 4xx response, not a generic 500. Previously the function
// returned an unstructured fmt.Errorf for the same condition, and the
// handler couldn't distinguish "DB exec failed" from "wrong state",
// returning 500 for both. CONST-035 territory: a 500 lies about the
// nature of the problem (callers think the server is broken when
// really their request was incompatible with task state).
var ErrTaskNotRetryable = errors.New("task not found, not in failed state, or max retries exceeded")

// ErrTaskInvalidStateTransition is returned by StartTask / CompleteTask
// when the target task is not in the prerequisite state for the
// requested transition (start requires pending; complete requires
// running). Same CONST-035 reasoning as ErrTaskNotRetryable — client-
// state errors must surface as 4xx, not 500. Handlers MUST errors.Is-
// check this sentinel and return 422 Unprocessable Entity.
var ErrTaskInvalidStateTransition = errors.New("task not in prerequisite state for the requested transition")

// ErrTaskNotFound is returned by DeleteTask / FailTask when the
// target task doesn't exist (RowsAffected==0 with no state filter).
// 404 Not Found, NOT 500 — the client asked for a non-existent
// resource. CONST-035: a 500 lies about the nature of the problem.
var ErrTaskNotFound = errors.New("task not found")

// RetryTask resets a failed task for retry
func (m *DatabaseManager) RetryTask(ctx context.Context, id string) error {
	taskID, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid task ID: %v", err)
	}

	query := `
		UPDATE distributed_tasks
		SET status = 'pending', retry_count = retry_count + 1, assigned_worker_id = NULL,
			error_message = NULL, updated_at = NOW()
		WHERE id = $1 AND status = 'failed' AND retry_count < max_retries
	`

	result, err := m.db.Exec(ctx, query, taskID)
	if err != nil {
		return fmt.Errorf("failed to retry task: %v", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("%w: %s", ErrTaskNotRetryable, id)
	}

	return nil
}

// ErrCheckpointRequiresAssignment is returned by CreateCheckpoint when
// the target task has no assigned_worker_id. task_checkpoints.worker_id
// is NOT NULL in the schema, so a checkpoint cannot be inserted without
// a worker context. Handlers MUST map this to a 4xx response.
var ErrCheckpointRequiresAssignment = errors.New("task must be assigned to a worker before creating a checkpoint")

// CreateCheckpoint creates a checkpoint for a task.
//
// Anti-bluff (CONST-035): the previous implementation
//   1. UPDATEd distributed_tasks.checkpoint_data (overwriting any
//      previous checkpoint — no history kept on that field)
//   2. INSERTed into task_checkpoints with `_, _ = m.db.Exec(...)` —
//      SILENTLY DISCARDING BOTH the result AND the error
//   3. Returned nil to the caller regardless
//
// The INSERT always failed because task_checkpoints.worker_id is
// NOT NULL REFERENCES workers(id) but the INSERT omitted that column.
// Net effect: POST /tasks/:id/checkpoint returned 201 "success"
// while task_checkpoints stayed permanently empty, and the subsequent
// GET /tasks/:id/checkpoints saw nothing. Triple-bluff: discarded
// error, fabricated success response, history table never populated.
//
// Fix: read the task's assigned_worker_id, refuse if not assigned
// (real 4xx — checkpoints conceptually require a worker context),
// supply it in the INSERT, and surface the error if it still fails.
func (m *DatabaseManager) CreateCheckpoint(ctx context.Context, taskID string, checkpointName string, checkpointData map[string]interface{}) error {
	taskUUID, err := uuid.Parse(taskID)
	if err != nil {
		return fmt.Errorf("invalid task ID: %v", err)
	}
	if checkpointData == nil {
		checkpointData = map[string]interface{}{}
	}

	// Fetch the task's assigned worker (the schema requires worker_id
	// on every checkpoint row).
	var assignedWorker *uuid.UUID
	if err := m.db.QueryRow(ctx,
		`SELECT assigned_worker_id FROM distributed_tasks WHERE id = $1`,
		taskUUID,
	).Scan(&assignedWorker); err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("task not found: %s", taskID)
		}
		return fmt.Errorf("failed to lookup task for checkpoint: %v", err)
	}
	if assignedWorker == nil {
		return fmt.Errorf("%w: %s", ErrCheckpointRequiresAssignment, taskID)
	}

	checkpointWithName := map[string]interface{}{
		"name":       checkpointName,
		"data":       checkpointData,
		"created_at": time.Now(),
	}

	// Step 1: update the task's latest-checkpoint snapshot.
	result, err := m.db.Exec(ctx,
		`UPDATE distributed_tasks SET checkpoint_data = $1, updated_at = NOW() WHERE id = $2`,
		checkpointWithName, taskUUID,
	)
	if err != nil {
		return fmt.Errorf("failed to update task checkpoint_data: %v", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("task not found: %s", taskID)
	}

	// Step 2: insert into the history table. The previous version
	// discarded the error from this INSERT; we now surface it. The
	// worker_id column is required by the schema, so we pass the
	// task's currently-assigned worker.
	_, err = m.db.Exec(ctx,
		`INSERT INTO task_checkpoints (id, task_id, checkpoint_name, checkpoint_data, worker_id, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		uuid.New(), taskUUID, checkpointName, checkpointData, *assignedWorker, time.Now(),
	)
	if err != nil {
		return fmt.Errorf("failed to insert into task_checkpoints history: %v", err)
	}

	return nil
}

// GetCheckpoints retrieves all checkpoints for a task
func (m *DatabaseManager) GetCheckpoints(ctx context.Context, taskID string) ([]map[string]interface{}, error) {
	taskUUID, err := uuid.Parse(taskID)
	if err != nil {
		return nil, fmt.Errorf("invalid task ID: %v", err)
	}

	query := `
		SELECT id, checkpoint_name, checkpoint_data, worker_id, created_at
		FROM task_checkpoints
		WHERE task_id = $1
		ORDER BY created_at DESC
	`

	rows, err := m.db.Query(ctx, query, taskUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to query checkpoints: %v", err)
	}
	defer rows.Close()

	var checkpoints []map[string]interface{}
	for rows.Next() {
		var (
			id             uuid.UUID
			checkpointName string
			checkpointData map[string]interface{}
			workerID       *uuid.UUID
			createdAt      time.Time
		)

		if err := rows.Scan(&id, &checkpointName, &checkpointData, &workerID, &createdAt); err != nil {
			return nil, fmt.Errorf("failed to scan checkpoint row: %v", err)
		}

		checkpoint := map[string]interface{}{
			"id":         id.String(),
			"name":       checkpointName,
			"data":       checkpointData,
			"created_at": createdAt,
		}

		if workerID != nil {
			checkpoint["worker_id"] = workerID.String()
		}

		checkpoints = append(checkpoints, checkpoint)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating checkpoint rows: %v", err)
	}

	return checkpoints, nil
}

// Helper function to convert pointer to string
func getStringFromPtr(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
}
