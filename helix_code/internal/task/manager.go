package task

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"reflect"
	"sync"
	"time"

	"dev.helix.code/internal/database"
	"dev.helix.code/internal/redis"
	goredis "github.com/redis/go-redis/v9"
	"github.com/google/uuid"
)

// TaskType represents different types of tasks
type TaskType string

const (
	TaskTypePlanning    TaskType = "planning"
	TaskTypeBuilding    TaskType = "building"
	TaskTypeTesting     TaskType = "testing"
	TaskTypeRefactoring TaskType = "refactoring"
	TaskTypeDebugging   TaskType = "debugging"
	TaskTypeDesign      TaskType = "design"
	TaskTypeDiagram     TaskType = "diagram"
	TaskTypeDeployment  TaskType = "deployment"
	TaskTypePorting     TaskType = "porting"
)

// TaskPriority represents task priority levels
type TaskPriority int

const (
	PriorityLow      TaskPriority = 1
	PriorityNormal   TaskPriority = 5
	PriorityHigh     TaskPriority = 10
	PriorityCritical TaskPriority = 20
)

// TaskCriticality represents task criticality levels
type TaskCriticality string

const (
	CriticalityLow      TaskCriticality = "low"
	CriticalityNormal   TaskCriticality = "normal"
	CriticalityHigh     TaskCriticality = "high"
	CriticalityCritical TaskCriticality = "critical"
)

// TaskStatus represents task status
type TaskStatus string

const (
	TaskStatusPending          TaskStatus = "pending"
	TaskStatusAssigned         TaskStatus = "assigned"
	TaskStatusRunning          TaskStatus = "running"
	TaskStatusCompleted        TaskStatus = "completed"
	TaskStatusFailed           TaskStatus = "failed"
	TaskStatusPaused           TaskStatus = "paused"
	TaskStatusWaitingForWorker TaskStatus = "waiting_for_worker"
	TaskStatusWaitingForDeps   TaskStatus = "waiting_for_deps"
)

// ComplexityLevel represents task complexity
type ComplexityLevel string

const (
	ComplexityLow    ComplexityLevel = "low"
	ComplexityMedium ComplexityLevel = "medium"
	ComplexityHigh   ComplexityLevel = "high"
)

// Task represents a distributed task
type Task struct {
	ID                uuid.UUID              `json:"id"`
	Type              TaskType               `json:"type"`
	Data              map[string]interface{} `json:"data"`
	Status            TaskStatus             `json:"status"`
	Priority          TaskPriority           `json:"priority"`
	Criticality       TaskCriticality        `json:"criticality"`
	AssignedWorker    *uuid.UUID             `json:"assigned_worker"`
	OriginalWorker    *uuid.UUID             `json:"original_worker"`
	Dependencies      []uuid.UUID            `json:"dependencies"`
	RetryCount        int                    `json:"retry_count"`
	MaxRetries        int                    `json:"max_retries"`
	ErrorMessage      string                 `json:"error_message"`
	ResultData        map[string]interface{} `json:"result_data"`
	CheckpointData    map[string]interface{} `json:"checkpoint_data"`
	EstimatedDuration time.Duration          `json:"estimated_duration"`
	StartedAt         *time.Time             `json:"started_at"`
	CompletedAt       *time.Time             `json:"completed_at"`
	CreatedAt         time.Time              `json:"created_at"`
	UpdatedAt         time.Time              `json:"updated_at"`
}

// TaskManager manages distributed tasks
type TaskManager struct {
	db            database.DatabaseInterface
	redis         *redis.Client
	mu            sync.RWMutex
	tasks         map[uuid.UUID]*Task
	workers       map[uuid.UUID]*Worker
	queue         *TaskQueue
	checkpointMgr *CheckpointManager
	dependencyMgr *DependencyManager
}

// Worker represents a worker node
type Worker struct {
	ID                 uuid.UUID              `json:"id"`
	Hostname           string                 `json:"hostname"`
	DisplayName        string                 `json:"display_name"`
	SSHConfig          map[string]interface{} `json:"ssh_config"`
	Capabilities       []string               `json:"capabilities"`
	Resources          map[string]interface{} `json:"resources"`
	Status             string                 `json:"status"`
	HealthStatus       string                 `json:"health_status"`
	LastHeartbeat      *time.Time             `json:"last_heartbeat"`
	CPUUsagePercent    float64                `json:"cpu_usage_percent"`
	MemoryUsagePercent float64                `json:"memory_usage_percent"`
	DiskUsagePercent   float64                `json:"disk_usage_percent"`
	CurrentTasksCount  int                    `json:"current_tasks_count"`
	MaxConcurrentTasks int                    `json:"max_concurrent_tasks"`
	CreatedAt          time.Time              `json:"created_at"`
	UpdatedAt          time.Time              `json:"updated_at"`
}

// TaskQueue manages task prioritization
type TaskQueue struct {
	highPriority   []*Task
	normalPriority []*Task
	lowPriority    []*Task
	mu             sync.RWMutex
}

// CheckpointManager manages task checkpoints
type CheckpointManager struct {
	db database.DatabaseInterface
}

// DependencyManager manages task dependencies
type DependencyManager struct {
	db database.DatabaseInterface
}

// TaskAnalysis represents analysis of a task for splitting
type TaskAnalysis struct {
	TaskID       uuid.UUID
	TaskType     TaskType
	Complexity   ComplexityLevel
	DataSize     int64
	Dependencies int
}

// TaskProgress represents task progress information
type TaskProgress struct {
	TaskID    uuid.UUID
	Status    TaskStatus
	Progress  float64
	StartedAt *time.Time
	UpdatedAt time.Time
}

// SplitStrategy defines interface for task splitting strategies
type SplitStrategy interface {
	GenerateSubtasks(parent *Task, analysis *TaskAnalysis) ([]SubtaskData, error)
}

// SubtaskData represents data for a subtask
type SubtaskData struct {
	Data         map[string]interface{}
	Dependencies []uuid.UUID
}

// NewTaskManager creates a new task manager
func NewTaskManager(db database.DatabaseInterface, redisClient *redis.Client) *TaskManager {
	// Normalize a typed-nil interface to a TRUE nil. A caller that passes a
	// (*database.Database)(nil) (e.g. the TUI in DB-degraded mode) yields a
	// NON-nil interface wrapping a typed nil; that would defeat every
	// `db == nil` guard downstream (storeTaskInDB, CheckpointManager,
	// DependencyManager) and panic on the first DB call instead of cleanly
	// returning ErrTaskPersistenceNotWired. Persistence must be HONESTLY
	// disabled, never crash (§11.4).
	if db != nil {
		if rv := reflect.ValueOf(db); rv.Kind() == reflect.Ptr && rv.IsNil() {
			db = nil
		}
	}
	return &TaskManager{
		db:            db,
		redis:         redisClient,
		tasks:         make(map[uuid.UUID]*Task),
		workers:       make(map[uuid.UUID]*Worker),
		queue:         NewTaskQueue(),
		checkpointMgr: NewCheckpointManager(db),
		dependencyMgr: NewDependencyManager(db),
	}
}

// CreateTask creates a new task
func (tm *TaskManager) CreateTask(taskType TaskType, data map[string]interface{},
	priority TaskPriority, criticality TaskCriticality, dependencies []uuid.UUID) (*Task, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	return tm.createTaskUnsafe(taskType, data, priority, criticality, dependencies)
}

// createTaskUnsafe creates a new task without acquiring the lock.
// Caller must hold tm.mu.Lock() before calling this method.
func (tm *TaskManager) createTaskUnsafe(taskType TaskType, data map[string]interface{},
	priority TaskPriority, criticality TaskCriticality, dependencies []uuid.UUID) (*Task, error) {
	task := &Task{
		ID:           uuid.New(),
		Type:         taskType,
		Data:         data,
		Status:       TaskStatusPending,
		Priority:     priority,
		Criticality:  criticality,
		Dependencies: dependencies,
		MaxRetries:   3,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// Validate dependencies
	if err := tm.dependencyMgr.ValidateDependencies(dependencies); err != nil {
		return nil, fmt.Errorf("%s", tr(context.Background(), "internal_task_invalid_dependencies", map[string]any{"Err": err.Error()}))
	}

	// Store in memory
	tm.tasks[task.ID] = task

	// Add to database
	if err := tm.storeTaskInDB(task); err != nil {
		delete(tm.tasks, task.ID)
		return nil, fmt.Errorf("%s", tr(context.Background(), "internal_task_store_in_db_failed", map[string]any{"Err": err.Error()}))
	}

	// Add to appropriate queue
	tm.queue.AddTask(task)

	log.Printf("✅ Task created: %s (type: %s, priority: %d)", task.ID, taskType, priority)
	return task, nil
}

// Redis Caching Methods

// cacheTask caches a task in Redis
func (tm *TaskManager) cacheTask(ctx context.Context, task *Task) error {
	if tm.redis == nil || !tm.redis.IsEnabled() {
		return nil // Redis not available, skip caching
	}

	key := fmt.Sprintf("task:%s", task.ID)
	data, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("%s", tr(ctx, "internal_task_marshal_failed", map[string]any{"Err": err.Error()}))
	}

	// Cache for 1 hour
	return tm.redis.Set(ctx, key, string(data), time.Hour)
}

// getCachedTask retrieves a task from Redis cache
func (tm *TaskManager) getCachedTask(ctx context.Context, taskID uuid.UUID) (*Task, error) {
	if tm.redis == nil || !tm.redis.IsEnabled() {
		return nil, nil // Redis not available, graceful degradation
	}

	key := fmt.Sprintf("task:%s", taskID)
	data, err := tm.redis.Get(ctx, key)
	if err != nil {
		// Check if it's a cache miss (key not found) - this is expected
		if errors.Is(err, goredis.Nil) {
			return nil, nil // Cache miss, no error
		}
		// Log unexpected errors but still allow fallback
		log.Printf("Warning: Redis cache error for task %s: %v", taskID, err)
		return nil, nil // Allow graceful degradation
	}

	var task Task
	if err := json.Unmarshal([]byte(data), &task); err != nil {
		log.Printf("Warning: failed to unmarshal cached task %s: %v", taskID, err)
		return nil, nil // Allow graceful degradation on unmarshal error
	}

	return &task, nil
}

// invalidateTaskCache removes a task from cache
func (tm *TaskManager) invalidateTaskCache(ctx context.Context, taskID uuid.UUID) error {
	if tm.redis == nil || !tm.redis.IsEnabled() {
		return nil // Redis not available
	}

	key := fmt.Sprintf("task:%s", taskID)
	return tm.redis.Del(ctx, key)
}

// cacheTaskStats caches task statistics
func (tm *TaskManager) cacheTaskStats(ctx context.Context, stats map[string]interface{}) error {
	if tm.redis == nil || !tm.redis.IsEnabled() {
		return nil // Redis not available
	}

	data, err := json.Marshal(stats)
	if err != nil {
		return fmt.Errorf("%s", tr(ctx, "internal_task_marshal_stats_failed", map[string]any{"Err": err.Error()}))
	}

	// Cache for 5 minutes
	return tm.redis.Set(ctx, "task:stats", string(data), 5*time.Minute)
}

// getCachedTaskStats retrieves cached task statistics
func (tm *TaskManager) getCachedTaskStats(ctx context.Context) (map[string]interface{}, error) {
	if tm.redis == nil || !tm.redis.IsEnabled() {
		return nil, nil // Redis not available, graceful degradation
	}

	data, err := tm.redis.Get(ctx, "task:stats")
	if err != nil {
		// Check if it's a cache miss (key not found) - this is expected
		if errors.Is(err, goredis.Nil) {
			return nil, nil // Cache miss, no error
		}
		// Log unexpected errors but still allow fallback
		log.Printf("Warning: Redis cache error for task stats: %v", err)
		return nil, nil // Allow graceful degradation
	}

	var stats map[string]interface{}
	if err := json.Unmarshal([]byte(data), &stats); err != nil {
		log.Printf("Warning: failed to unmarshal cached stats: %v", err)
		return nil, nil // Allow graceful degradation on unmarshal error
	}

	return stats, nil
}

// cacheWorkerTasks caches tasks assigned to a worker
func (tm *TaskManager) cacheWorkerTasks(ctx context.Context, workerID uuid.UUID, taskIDs []uuid.UUID) error {
	if tm.redis == nil || !tm.redis.IsEnabled() {
		return nil // Redis not available
	}

	key := fmt.Sprintf("worker:%s:tasks", workerID)
	data, err := json.Marshal(taskIDs)
	if err != nil {
		return fmt.Errorf("%s", tr(ctx, "internal_task_marshal_ids_failed", map[string]any{"Err": err.Error()}))
	}

	// Cache for 10 minutes
	return tm.redis.Set(ctx, key, string(data), 10*time.Minute)
}

// getCachedWorkerTasks retrieves cached worker tasks
func (tm *TaskManager) getCachedWorkerTasks(ctx context.Context, workerID uuid.UUID) ([]uuid.UUID, error) {
	if tm.redis == nil || !tm.redis.IsEnabled() {
		return nil, nil // Redis not available, graceful degradation
	}

	key := fmt.Sprintf("worker:%s:tasks", workerID)
	data, err := tm.redis.Get(ctx, key)
	if err != nil {
		// Check if it's a cache miss (key not found) - this is expected
		if errors.Is(err, goredis.Nil) {
			return nil, nil // Cache miss, no error
		}
		// Log unexpected errors but still allow fallback
		log.Printf("Warning: Redis cache error for worker tasks %s: %v", workerID, err)
		return nil, nil // Allow graceful degradation
	}

	var taskIDs []uuid.UUID
	if err := json.Unmarshal([]byte(data), &taskIDs); err != nil {
		log.Printf("Warning: failed to unmarshal cached task IDs for worker %s: %v", workerID, err)
		return nil, nil // Allow graceful degradation on unmarshal error
	}

	return taskIDs, nil
}

// GetTaskWithCache retrieves a task with caching
func (tm *TaskManager) GetTaskWithCache(ctx context.Context, taskID uuid.UUID) (*Task, error) {
	// Try cache first
	if cachedTask, err := tm.getCachedTask(ctx, taskID); err == nil && cachedTask != nil {
		return cachedTask, nil
	}

	tm.mu.RLock()
	task, exists := tm.tasks[taskID]
	tm.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("%s", tr(ctx, "internal_task_not_found", map[string]any{"ID": taskID.String()}))
	}

	// Cache the result
	if task != nil {
		if cacheErr := tm.cacheTask(ctx, task); cacheErr != nil {
			log.Printf("⚠️  Failed to cache task %s: %v", taskID, cacheErr)
		}
	}

	return task, nil
}

// UpdateTaskWithCache updates a task and invalidates cache
func (tm *TaskManager) UpdateTaskWithCache(ctx context.Context, task *Task) error {
	// Update database
	if err := tm.updateTaskInDB(task); err != nil {
		return err
	}

	// Invalidate cache
	if err := tm.invalidateTaskCache(ctx, task.ID); err != nil {
		log.Printf("⚠️  Failed to invalidate cache for task %s: %v", task.ID, err)
	}

	// Cache updated task
	if err := tm.cacheTask(ctx, task); err != nil {
		log.Printf("⚠️  Failed to cache updated task %s: %v", task.ID, err)
	}

	return nil
}
