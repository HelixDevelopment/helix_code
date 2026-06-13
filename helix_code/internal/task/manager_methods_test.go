package task

import (
	"errors"
	"strings"
	"testing"
	"time"

	"dev.helix.code/internal/database"
	"dev.helix.code/internal/redis"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockSplitStrategy implements SplitStrategy for testing
type MockSplitStrategy struct {
	subtasks []SubtaskData
	err      error
}

func (m *MockSplitStrategy) GenerateSubtasks(parent *Task, analysis *TaskAnalysis) ([]SubtaskData, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.subtasks, nil
}

// TestSplitTask tests task splitting functionality
func TestSplitTask(t *testing.T) {
	mockDB := database.NewMockDatabase()
	// Anti-bluff (round-31 §11.4 audit, 2026-05-18): CreateTask now runs
	// a real INSERT through tm.db.Exec (was log-only stub). Pre-mock
	// success for any Exec — including subsequent subtask inserts and
	// the parent-task UPDATE that SplitTask performs.
	mockDB.MockExecSuccess(1)
	tm := NewTaskManager(mockDB, &redis.Client{})

	// Create a large task
	task, err := tm.CreateTask(
		TaskTypePlanning,
		map[string]interface{}{
			"complexity": "high",
			"dataSize":   1000000,
		},
		PriorityHigh,
		CriticalityHigh,
		[]uuid.UUID{},
	)
	require.NoError(t, err)

	// Create mock split strategy
	strategy := &MockSplitStrategy{
		subtasks: []SubtaskData{
			{
				Data:         map[string]interface{}{"part": 1},
				Dependencies: []uuid.UUID{},
			},
			{
				Data:         map[string]interface{}{"part": 2},
				Dependencies: []uuid.UUID{},
			},
		},
	}

	// Mock database for subtask creation
	mockDB.MockExecSuccess(1) // For parent task update

	// Split the task
	subtasks, err := tm.SplitTask(task.ID, strategy)

	// Should complete without error
	assert.NoError(t, err)
	assert.Len(t, subtasks, 2)
}

// TestSplitTask_NotFound tests splitting non-existent task
func TestSplitTask_NotFound(t *testing.T) {
	mockDB := database.NewMockDatabase()
	tm := NewTaskManager(mockDB, &redis.Client{})

	nonExistentID := uuid.New()
	strategy := &MockSplitStrategy{}

	_, err := tm.SplitTask(nonExistentID, strategy)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// TestAssignTask tests task assignment to workers
func TestAssignTask(t *testing.T) {
	mockDB := database.NewMockDatabase()
	// Anti-bluff (round-31 §11.4 audit, 2026-05-18): CreateTask runs a
	// real INSERT through tm.db.Exec. Pre-mock for the INSERT plus the
	// later worker/task UPDATEs that AssignTask performs.
	mockDB.MockExecSuccess(1)
	tm := NewTaskManager(mockDB, &redis.Client{})

	// Create a task
	task, err := tm.CreateTask(
		TaskTypeBuilding,
		map[string]interface{}{"test": "data"},
		PriorityNormal,
		CriticalityNormal,
		[]uuid.UUID{},
	)
	require.NoError(t, err)

	// Create a worker manually in the tasks map
	// Note: TaskTypeBuilding requires ["compilation", "build_tools"] capabilities
	workerID := uuid.New()
	worker := &Worker{
		ID:                 workerID,
		Hostname:           "test-worker",
		DisplayName:        "Test Worker",
		Capabilities:       []string{"compilation", "build_tools"},
		Status:             "active",
		HealthStatus:       "healthy",
		MaxConcurrentTasks: 5,
		CurrentTasksCount:  0,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	tm.mu.Lock()
	tm.workers[workerID] = worker
	tm.mu.Unlock()

	// Mock database expectations for assignment
	mockDB.MockExecSuccess(1) // Update task
	mockDB.MockExecSuccess(1) // Update worker

	// Assign task to worker
	err = tm.AssignTask(task.ID, workerID)

	// Should complete without error
	assert.NoError(t, err)
}

// TestAssignTask_NotFound tests assigning non-existent task
func TestAssignTask_NotFound(t *testing.T) {
	mockDB := database.NewMockDatabase()
	tm := NewTaskManager(mockDB, &redis.Client{})

	nonExistentID := uuid.New()
	workerID := uuid.New()

	err := tm.AssignTask(nonExistentID, workerID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// TestCompleteTask tests task completion
func TestCompleteTask(t *testing.T) {
	mockDB := database.NewMockDatabase()
	// Anti-bluff (round-31 §11.4 audit, 2026-05-18): CreateTask now runs
	// a real INSERT through tm.db.Exec. Pre-mock so the create call
	// doesn't crash before we reach the completion path.
	mockDB.MockExecSuccess(1)
	tm := NewTaskManager(mockDB, &redis.Client{})

	// Create a task
	task, err := tm.CreateTask(
		TaskTypeBuilding,
		map[string]interface{}{"test": "data"},
		PriorityNormal,
		CriticalityNormal,
		[]uuid.UUID{},
	)
	require.NoError(t, err)

	// Mock database update (redundant with the pre-mock above but
	// preserved for documentation of intent)
	mockDB.MockExecSuccess(1)

	// Complete the task
	result := map[string]interface{}{
		"output": "success",
		"lines":  100,
	}
	err = tm.CompleteTask(task.ID, result)

	// Should complete without error
	assert.NoError(t, err)
}

// TestFailTask tests marking a task as failed
func TestFailTask(t *testing.T) {
	mockDB := database.NewMockDatabase()
	// Anti-bluff (round-31 §11.4 audit, 2026-05-18): CreateTask now runs
	// a real INSERT through tm.db.Exec. Pre-mock so the create call
	// doesn't crash before we reach the fail-task path.
	mockDB.MockExecSuccess(1)
	tm := NewTaskManager(mockDB, &redis.Client{})

	// Create a task
	task, err := tm.CreateTask(
		TaskTypeTesting,
		map[string]interface{}{"test": "data"},
		PriorityNormal,
		CriticalityNormal,
		[]uuid.UUID{},
	)
	require.NoError(t, err)

	// Mock database update
	mockDB.MockExecSuccess(1)

	// Fail the task
	err = tm.FailTask(task.ID, "test error")

	// Should complete without error
	assert.NoError(t, err)
}

// TestFailTask_NotFound tests failing non-existent task
func TestFailTask_NotFound(t *testing.T) {
	mockDB := database.NewMockDatabase()
	tm := NewTaskManager(mockDB, &redis.Client{})

	nonExistentID := uuid.New()

	err := tm.FailTask(nonExistentID, "error")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// TestCreateCheckpoint tests checkpoint creation
func TestCreateCheckpoint(t *testing.T) {
	mockDB := database.NewMockDatabase()
	// Anti-bluff (round-31 §11.4 audit, 2026-05-18): CreateTask now runs
	// a real INSERT through tm.db.Exec.
	mockDB.MockExecSuccess(1)
	tm := NewTaskManager(mockDB, &redis.Client{})

	// Create a task
	task, err := tm.CreateTask(
		TaskTypeRefactoring,
		map[string]interface{}{"test": "data"},
		PriorityNormal,
		CriticalityNormal,
		[]uuid.UUID{},
	)
	require.NoError(t, err)

	// Mock database insert for checkpoint
	mockDB.MockExecSuccess(1)

	// Create checkpoint
	err = tm.CreateCheckpoint(task.ID, "progress-checkpoint", map[string]interface{}{
		"progress": 50,
		"step":     "analysis",
	})

	// Should complete without error
	assert.NoError(t, err)
}

// TestCreateCheckpoint_NotFound tests checkpoint for non-existent task
func TestCreateCheckpoint_NotFound(t *testing.T) {
	mockDB := database.NewMockDatabase()
	tm := NewTaskManager(mockDB, &redis.Client{})

	nonExistentID := uuid.New()

	err := tm.CreateCheckpoint(nonExistentID, "checkpoint", map[string]interface{}{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// TestGetTaskProgress tests retrieving task progress
func TestGetTaskProgress(t *testing.T) {
	mockDB := database.NewMockDatabase()
	// Anti-bluff (round-31 §11.4 audit, 2026-05-18): CreateTask now runs
	// a real INSERT through tm.db.Exec.
	mockDB.MockExecSuccess(1)
	tm := NewTaskManager(mockDB, &redis.Client{})

	// Create a task
	task, err := tm.CreateTask(
		TaskTypeDeployment,
		map[string]interface{}{"test": "data"},
		PriorityNormal,
		CriticalityNormal,
		[]uuid.UUID{},
	)
	require.NoError(t, err)

	// Get progress
	progress, err := tm.GetTaskProgress(task.ID)

	// Should complete without error
	assert.NoError(t, err)
	assert.NotNil(t, progress)
	assert.Equal(t, task.ID, progress.TaskID)
}

// TestGetTaskProgress_NotFound tests progress for non-existent task
func TestGetTaskProgress_NotFound(t *testing.T) {
	mockDB := database.NewMockDatabase()
	tm := NewTaskManager(mockDB, &redis.Client{})

	nonExistentID := uuid.New()

	progress, err := tm.GetTaskProgress(nonExistentID)
	assert.Error(t, err)
	assert.Nil(t, progress)
	assert.Contains(t, err.Error(), "not found")
}

// TestAnalyzeTaskForSplitting tests task analysis logic
func TestAnalyzeTaskForSplitting(t *testing.T) {
	mockDB := database.NewMockDatabase()
	tm := NewTaskManager(mockDB, &redis.Client{})

	// Create a complex task
	task := &Task{
		ID:   uuid.New(),
		Type: TaskTypePlanning,
		Data: map[string]interface{}{
			"complexity": "high",
			"dataSize":   5000000,
		},
		Priority:     PriorityHigh,
		Criticality:  CriticalityHigh,
		Dependencies: []uuid.UUID{},
	}

	// Analyze for splitting
	analysis, err := tm.analyzeTaskForSplitting(task)

	// Should return analysis
	assert.NoError(t, err)
	assert.NotNil(t, analysis)
	assert.Equal(t, task.ID, analysis.TaskID)
	assert.Equal(t, task.Type, analysis.TaskType)
}

// TestEstimateComplexity tests complexity estimation
func TestEstimateComplexity(t *testing.T) {
	mockDB := database.NewMockDatabase()
	tm := NewTaskManager(mockDB, &redis.Client{})

	tests := []struct {
		name string
		task *Task
	}{
		{
			name: "simple task",
			task: &Task{
				Type: TaskTypePlanning,
				Data: map[string]interface{}{
					"complexity": "low",
				},
			},
		},
		{
			name: "complex task",
			task: &Task{
				Type: TaskTypeBuilding,
				Data: map[string]interface{}{
					"complexity": "high",
					"steps":      100,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			complexity := tm.estimateComplexity(tt.task)
			// Should return a valid complexity level
			assert.Contains(t, []ComplexityLevel{ComplexityLow, ComplexityMedium, ComplexityHigh}, complexity)
		})
	}
}

// TestEstimateDataSize tests data size estimation
func TestEstimateDataSize(t *testing.T) {
	mockDB := database.NewMockDatabase()
	tm := NewTaskManager(mockDB, &redis.Client{})

	tests := []struct {
		name string
		task *Task
	}{
		{
			name: "small data",
			task: &Task{
				Data: map[string]interface{}{
					"value": "small",
				},
			},
		},
		{
			name: "large data",
			task: &Task{
				Data: map[string]interface{}{
					"dataSize":  1000000,
					"itemCount": 5000,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			size := tm.estimateDataSize(tt.task)
			assert.GreaterOrEqual(t, size, int64(0))
		})
	}
}

// TestCanWorkerHandleTask tests worker capability checking
func TestCanWorkerHandleTask(t *testing.T) {
	mockDB := database.NewMockDatabase()
	tm := NewTaskManager(mockDB, &redis.Client{})

	tests := []struct {
		name     string
		worker   *Worker
		task     *Task
		expected bool
	}{
		{
			name: "capable worker",
			worker: &Worker{
				// TaskTypePlanning requires "general_computation" (default case)
				Capabilities:       []string{"general_computation"},
				MaxConcurrentTasks: 5,
				CurrentTasksCount:  2,
			},
			task: &Task{
				Type: TaskTypePlanning,
				Data: map[string]interface{}{
					"memoryRequired": 4096,
				},
			},
			expected: true,
		},
		{
			name: "incapable worker - missing capability",
			worker: &Worker{
				// Worker lacks "general_computation" required for TaskTypePlanning
				Capabilities:       []string{"testing"},
				MaxConcurrentTasks: 5,
				CurrentTasksCount:  2,
			},
			task: &Task{
				Type: TaskTypePlanning,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			canHandle := tm.canWorkerHandleTask(tt.worker, tt.task)
			assert.Equal(t, tt.expected, canHandle)
		})
	}
}

// TestGetRequiredCapabilities tests capability extraction
func TestGetRequiredCapabilities(t *testing.T) {
	mockDB := database.NewMockDatabase()
	tm := NewTaskManager(mockDB, &redis.Client{})

	tests := []struct {
		name string
		task *Task
	}{
		{
			name: "planning task",
			task: &Task{
				Type: TaskTypePlanning,
			},
		},
		{
			name: "building task",
			task: &Task{
				Type: TaskTypeBuilding,
			},
		},
		{
			name: "testing task",
			task: &Task{
				Type: TaskTypeTesting,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			capabilities := tm.getRequiredCapabilities(tt.task.Type)
			assert.NotNil(t, capabilities)
			// Should return at least an empty slice
			assert.GreaterOrEqual(t, len(capabilities), 0)
		})
	}
}

// TestUpdateWorkerInDB tests worker database update
func TestUpdateWorkerInDB(t *testing.T) {
	mockDB := database.NewMockDatabase()
	tm := NewTaskManager(mockDB, &redis.Client{})

	w := &Worker{
		ID:                 uuid.New(),
		Hostname:           "test-worker",
		DisplayName:        "Test Worker",
		Capabilities:       []string{"planning"},
		MaxConcurrentTasks: 5,
		CurrentTasksCount:  2,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	// Mock database update
	mockDB.MockExecSuccess(1)

	err := tm.updateWorkerInDB(w)

	// Should complete without error
	assert.NoError(t, err)
}

// ========================================
// Anti-bluff regression tests (round-31 §11.4 audit, 2026-05-18)
// ========================================
//
// The three functions below were CRITICAL persistence bluffs prior to
// this audit — log-only stubs that returned nil success regardless of
// actual database state. These regressions assert two invariants:
//
//   (1) When tm.db is nil, the functions return the sentinel
//       ErrTaskPersistenceNotWired (loud failure mode, not silent
//       success); composes with CONST-035 / Article XI §11.9.
//
//   (2) When tm.db is wired, the functions issue a real Exec call
//       against the expected SQL statement (proves the wiring is
//       in place — not just a comment claiming intent).
//
// If any of these regressions ever fails, the persistence bluff has
// regressed and the fix in manager_methods.go must be re-applied.

// TestStoreTaskInDB_NilDB_ReturnsSentinel asserts the loud-failure path
// when no database backend is wired into the TaskManager.
func TestStoreTaskInDB_NilDB_ReturnsSentinel(t *testing.T) {
	tm := &TaskManager{} // bypass NewTaskManager — db deliberately nil
	err := tm.storeTaskInDB(&Task{ID: uuid.New()})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrTaskPersistenceNotWired,
		"storeTaskInDB on a nil-db TaskManager must surface the sentinel "+
			"(was: silent success that vanished tasks across restarts)")
}

// TestUpdateTaskInDB_NilDB_ReturnsSentinel asserts the loud-failure path
// for the update side.
func TestUpdateTaskInDB_NilDB_ReturnsSentinel(t *testing.T) {
	tm := &TaskManager{}
	err := tm.updateTaskInDB(&Task{ID: uuid.New()})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrTaskPersistenceNotWired,
		"updateTaskInDB on a nil-db TaskManager must surface the sentinel "+
			"(was: silent success that lost task-state updates)")
}

// TestUpdateWorkerInDB_NilDB_ReturnsSentinel asserts the loud-failure
// path for the worker update side.
func TestUpdateWorkerInDB_NilDB_ReturnsSentinel(t *testing.T) {
	tm := &TaskManager{}
	err := tm.updateWorkerInDB(&Worker{ID: uuid.New()})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrTaskPersistenceNotWired,
		"updateWorkerInDB on a nil-db TaskManager must surface the sentinel "+
			"(was: silent success that lost worker-state updates)")
}

// TestStoreTaskInDB_IssuesRealExecCall asserts the function actually
// invokes tm.db.Exec with an INSERT statement targeting distributed_tasks
// — proving the SQL wiring is in place, not just a comment promising it.
func TestStoreTaskInDB_IssuesRealExecCall(t *testing.T) {
	mockDB := database.NewMockDatabase()
	// Match Exec calls whose SQL contains "INSERT INTO distributed_tasks"
	// using a custom matcher so we verify the statement, not just any Exec.
	mockDB.On("Exec",
		mock.Anything,
		mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "INSERT INTO distributed_tasks")
		}),
		mock.Anything,
	).Return(pgconn.NewCommandTag("INSERT 0 1"), nil).Once()

	tm := NewTaskManager(mockDB, &redis.Client{})
	task := &Task{
		ID:        uuid.New(),
		Type:      TaskTypeBuilding,
		Status:    TaskStatusPending,
		Priority:  PriorityNormal,
		Data:      map[string]interface{}{"k": "v"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := tm.storeTaskInDB(task)
	require.NoError(t, err)
	mockDB.AssertExpectations(t)
}

// TestUpdateTaskInDB_IssuesRealExecCall mirrors the above for UPDATE
// statements against distributed_tasks.
func TestUpdateTaskInDB_IssuesRealExecCall(t *testing.T) {
	mockDB := database.NewMockDatabase()
	mockDB.On("Exec",
		mock.Anything,
		mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "UPDATE distributed_tasks")
		}),
		mock.Anything,
	).Return(pgconn.NewCommandTag("UPDATE 1"), nil).Once()

	tm := NewTaskManager(mockDB, &redis.Client{})
	task := &Task{
		ID:        uuid.New(),
		Type:      TaskTypeBuilding,
		Status:    TaskStatusRunning,
		Priority:  PriorityHigh,
		Data:      map[string]interface{}{"k": "v"},
		UpdatedAt: time.Now(),
	}

	err := tm.updateTaskInDB(task)
	require.NoError(t, err)
	mockDB.AssertExpectations(t)
}

// TestUpdateWorkerInDB_IssuesRealExecCall mirrors the above for UPDATE
// statements against the workers table.
func TestUpdateWorkerInDB_IssuesRealExecCall(t *testing.T) {
	mockDB := database.NewMockDatabase()
	mockDB.On("Exec",
		mock.Anything,
		mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "UPDATE workers")
		}),
		mock.Anything,
	).Return(pgconn.NewCommandTag("UPDATE 1"), nil).Once()

	tm := NewTaskManager(mockDB, &redis.Client{})
	w := &Worker{
		ID:                 uuid.New(),
		Hostname:           "test-worker",
		Status:             "active",
		HealthStatus:       "healthy",
		Capabilities:       []string{"general_computation"},
		MaxConcurrentTasks: 5,
		UpdatedAt:          time.Now(),
	}

	err := tm.updateWorkerInDB(w)
	require.NoError(t, err)
	mockDB.AssertExpectations(t)
}

// TestStoreTaskInDB_PropagatesDBError asserts that a database-side error
// is wrapped + returned to the caller (was: log-only stub that always
// returned nil regardless of any underlying failure).
func TestStoreTaskInDB_PropagatesDBError(t *testing.T) {
	mockDB := database.NewMockDatabase()
	dbErr := errors.New("simulated connection lost")
	mockDB.MockExecError(dbErr)

	tm := NewTaskManager(mockDB, &redis.Client{})
	err := tm.storeTaskInDB(&Task{ID: uuid.New(), Type: TaskTypeBuilding})
	require.Error(t, err)
	assert.ErrorIs(t, err, dbErr,
		"storeTaskInDB must propagate the underlying db error so callers can react "+
			"(was: silent nil-return that hid every DB failure)")
}

// TestNewTaskManager_TypedNilDB_NoPanic is the §11.4.134 regression guard for
// the degraded-mode typed-nil-interface crash: the TUI builds the manager via
// NewTaskManager(db, rds) where db has static type *database.Database and is a
// nil pointer when the DB is offline. A nil *database.Database assigned into the
// database.DatabaseInterface parameter is a NON-nil interface wrapping a typed
// nil, which (before the NewTaskManager normalization) defeated the
// `tm.db == nil` guard and PANICKED on the first DB call. This asserts the
// normalization makes persistence cleanly disabled (sentinel error, no panic) —
// matching the EXACT production construction pattern, unlike the &TaskManager{}
// tests above which build a true-nil field and so never reproduced the crash.
func TestNewTaskManager_TypedNilDB_NoPanic(t *testing.T) {
	var nilDB *database.Database // typed-nil pointer, exactly as main.go's `db` in degraded mode
	tm := NewTaskManager(nilDB, nil)

	// storeTaskInDB must surface the sentinel, not panic — proving the
	// normalization turned the typed-nil interface into a true nil so the guard
	// fires.
	err := tm.storeTaskInDB(&Task{ID: uuid.New()})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrTaskPersistenceNotWired)

	// CreateTask is the user-reachable path (the TUI "create task" form). In
	// degraded mode it must return an error WITHOUT crashing the app.
	require.NotPanics(t, func() {
		_, cerr := tm.CreateTask(TaskTypePlanning, map[string]interface{}{"x": 1}, PriorityNormal, CriticalityLow, nil)
		require.Error(t, cerr, "CreateTask in DB-degraded mode must error cleanly, not panic")
	})
}
