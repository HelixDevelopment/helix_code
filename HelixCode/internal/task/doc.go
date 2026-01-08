// Package task provides distributed task management with checkpointing, dependencies, and priority queuing.
//
// The task package implements a comprehensive task management system for
// distributed AI development workflows. It supports task creation, assignment,
// checkpointing for failure recovery, dependency resolution, priority-based
// scheduling, and Redis caching for improved performance.
//
// # Key Components
//
// TaskManager coordinates all task operations:
//
//	db := database.Connect(cfg)
//	redis := redis.Connect(cfg)
//
//	manager := task.NewTaskManager(db, redis)
//
//	// Create a task
//	t, err := manager.CreateTask(
//	    task.TaskTypeBuilding,
//	    map[string]interface{}{"file": "main.go"},
//	    task.PriorityHigh,
//	    task.CriticalityNormal,
//	    []uuid.UUID{}, // dependencies
//	)
//
// # Task Types
//
// Tasks are categorized by development activity:
//
//	task.TaskTypePlanning    // Architecture and design tasks
//	task.TaskTypeBuilding    // Code development tasks
//	task.TaskTypeTesting     // Test execution tasks
//	task.TaskTypeRefactoring // Code improvement tasks
//	task.TaskTypeDebugging   // Bug investigation tasks
//	task.TaskTypeDesign      // UI/UX design tasks
//	task.TaskTypeDiagram     // Diagram generation tasks
//	task.TaskTypeDeployment  // Deployment tasks
//	task.TaskTypePorting     // Code porting tasks
//
// # Priority Levels
//
// Tasks have priority levels for scheduling:
//
//	task.PriorityLow      // Priority 1
//	task.PriorityNormal   // Priority 5
//	task.PriorityHigh     // Priority 10
//	task.PriorityCritical // Priority 20
//
// # Task Status
//
// Tasks progress through various statuses:
//
//	task.TaskStatusPending          // Waiting to be assigned
//	task.TaskStatusAssigned         // Assigned to a worker
//	task.TaskStatusRunning          // Currently executing
//	task.TaskStatusCompleted        // Successfully completed
//	task.TaskStatusFailed           // Execution failed
//	task.TaskStatusPaused           // Temporarily paused
//	task.TaskStatusWaitingForWorker // Needs worker assignment
//	task.TaskStatusWaitingForDeps   // Blocked by dependencies
//
// # Task Queue
//
// The TaskQueue provides priority-based scheduling:
//
//	queue := task.NewTaskQueue()
//
//	// Add task to appropriate priority queue
//	queue.AddTask(t)
//
//	// Get next task by priority
//	next := queue.GetNextTask()
//
// # Checkpointing
//
// CheckpointManager enables failure recovery:
//
//	checkpointMgr := task.NewCheckpointManager(db)
//
//	// Create a checkpoint
//	err := checkpointMgr.CreateCheckpoint(
//	    taskID,
//	    workerID,
//	    "step-3-complete",
//	    map[string]interface{}{
//	        "files_processed": 42,
//	        "current_state": "parsing",
//	    },
//	)
//
//	// Get latest checkpoint for recovery
//	checkpoint, err := checkpointMgr.GetLatestCheckpoint(taskID)
//
//	// Get all checkpoints
//	checkpoints, err := checkpointMgr.GetCheckpoints(taskID)
//
// # Dependency Management
//
// DependencyManager handles task dependencies:
//
//	depMgr := task.NewDependencyManager(db)
//
//	// Validate dependencies exist
//	err := depMgr.ValidateDependencies(dependencyIDs)
//
//	// Check if dependencies are completed
//	allDone, err := depMgr.CheckDependenciesCompleted(dependencyIDs)
//
//	// Get blocking (incomplete) dependencies
//	blocking, err := depMgr.GetBlockingDependencies(dependencyIDs)
//
//	// Detect circular dependencies
//	hasCircle, err := depMgr.DetectCircularDependencies(taskID, deps)
//
//	// Get full dependency chain
//	chain, err := depMgr.GetDependencyChain(taskID)
//
//	// Get tasks that depend on this task
//	dependents, err := depMgr.GetDependentTasks(taskID)
//
// # Redis Caching
//
// Tasks support Redis caching for performance:
//
//	ctx := context.Background()
//
//	// Get task with caching
//	t, err := manager.GetTaskWithCache(ctx, taskID)
//
//	// Update task (invalidates and refreshes cache)
//	err = manager.UpdateTaskWithCache(ctx, t)
//
// # Task Data
//
// Tasks carry arbitrary data for execution:
//
//	t := &task.Task{
//	    Type: task.TaskTypeBuilding,
//	    Data: map[string]interface{}{
//	        "source_files": []string{"main.go", "handler.go"},
//	        "target": "linux-amd64",
//	        "optimize": true,
//	    },
//	    ResultData: map[string]interface{}{},
//	    CheckpointData: map[string]interface{}{},
//	}
//
// # Task Splitting
//
// Large tasks can be split using strategies:
//
//	type SplitStrategy interface {
//	    GenerateSubtasks(parent *Task, analysis *TaskAnalysis) ([]SubtaskData, error)
//	}
//
//	analysis := &task.TaskAnalysis{
//	    TaskID:       taskID,
//	    TaskType:     task.TaskTypeBuilding,
//	    Complexity:   task.ComplexityHigh,
//	    DataSize:     1024 * 1024,
//	    Dependencies: 3,
//	}
//
// # Worker Association
//
// Tasks track their assigned workers:
//
//	t.AssignedWorker = &workerID  // Currently assigned
//	t.OriginalWorker = &workerID  // Initially assigned (for recovery)
//
// # Retry Handling
//
// Tasks support automatic retries:
//
//	t := &task.Task{
//	    RetryCount:   0,           // Current retry count
//	    MaxRetries:   3,           // Maximum retries allowed
//	    ErrorMessage: "",          // Last error if failed
//	}
//
// # Task Progress
//
// TaskProgress tracks execution progress:
//
//	progress := &task.TaskProgress{
//	    TaskID:    taskID,
//	    Status:    task.TaskStatusRunning,
//	    Progress:  0.75,  // 75% complete
//	    StartedAt: &startTime,
//	    UpdatedAt: time.Now(),
//	}
//
// # Thread Safety
//
// The TaskManager is thread-safe with internal mutex protection for
// concurrent access from multiple goroutines.
//
// # Database Integration
//
// Tasks are persisted to PostgreSQL and can be retrieved across server restarts.
// The DatabaseManager provides additional database-specific operations.
//
// # Example Workflow
//
//	// Create task with dependencies
//	planTask, _ := manager.CreateTask(TaskTypePlanning, planData, PriorityHigh, CriticalityNormal, nil)
//	buildTask, _ := manager.CreateTask(TaskTypeBuilding, buildData, PriorityNormal, CriticalityNormal, []uuid.UUID{planTask.ID})
//	testTask, _ := manager.CreateTask(TaskTypeTesting, testData, PriorityNormal, CriticalityNormal, []uuid.UUID{buildTask.ID})
//
//	// Tasks execute in dependency order: plan -> build -> test
package task
