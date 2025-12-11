package task

import (
	"context"
	"testing"
	"time"

	"dev.helix.code/internal/config"
)

func TestNewTask(t *testing.T) {
	task := NewTask("test-task", "test-type", "Test Task")

	if task == nil {
		t.Fatal("NewTask returned nil")
	}

	if task.ID != "test-task" {
		t.Errorf("Expected ID 'test-task', got '%s'", task.ID)
	}

	if task.Type != "test-type" {
		t.Errorf("Expected type 'test-type', got '%s'", task.Type)
	}

	if task.Name != "Test Task" {
		t.Errorf("Expected name 'Test Task', got '%s'", task.Name)
	}

	if task.Status != StatusPending {
		t.Errorf("Expected status Pending, got %v", task.Status)
	}

	if task.Priority != PriorityNormal {
		t.Errorf("Expected priority Normal, got %v", task.Priority)
	}

	if task.Progress != 0 {
		t.Errorf("Expected progress 0, got %d", task.Progress)
	}

	if task.MaxRetries != 3 {
		t.Errorf("Expected max retries 3, got %d", task.MaxRetries)
	}
}

func TestTaskUpdateStatus(t *testing.T) {
	task := NewTask("test-task", "test-type", "Test Task")

	// Test status update
	task.UpdateStatus(StatusRunning)

	if task.Status != StatusRunning {
		t.Errorf("Expected status Running, got %v", task.Status)
	}

	if task.StartedAt == nil {
		t.Error("StartedAt should be set when status changes to Running")
	}

	// Test completion
	task.UpdateStatus(StatusCompleted)

	if task.Status != StatusCompleted {
		t.Errorf("Expected status Completed, got %v", task.Status)
	}

	if task.CompletedAt == nil {
		t.Error("CompletedAt should be set when status changes to Completed")
	}
}

func TestTaskSetProgress(t *testing.T) {
	task := NewTask("test-task", "test-type", "Test Task")

	// Test normal progress
	task.SetProgress(50)

	if task.Progress != 50 {
		t.Errorf("Expected progress 50, got %d", task.Progress)
	}

	// Test progress bounds
	task.SetProgress(-10)
	if task.Progress != 0 {
		t.Errorf("Expected progress 0 for negative value, got %d", task.Progress)
	}

	task.SetProgress(150)
	if task.Progress != 100 {
		t.Errorf("Expected progress 100 for value > 100, got %d", task.Progress)
	}
}

func TestTaskIncrementRetries(t *testing.T) {
	task := NewTask("test-task", "test-type", "Test Task")

	// Test increment retries
	canRetry := task.IncrementRetries()
	if !canRetry {
		t.Error("Should be able to retry initially")
	}

	if task.Retries != 1 {
		t.Errorf("Expected retries 1, got %d", task.Retries)
	}

	// Test max retries
	task.MaxRetries = 2
	task.Retries = 2

	canRetry = task.IncrementRetries()
	if canRetry {
		t.Error("Should not be able to retry beyond max retries")
	}

	if task.Retries != 3 {
		t.Errorf("Expected retries 3, got %d", task.Retries)
	}
}

func TestTaskCanRetry(t *testing.T) {
	task := NewTask("test-task", "test-type", "Test Task")

	// Initially can retry
	if !task.CanRetry() {
		t.Error("Should be able to retry initially")
	}

	// Set retries to max
	task.Retries = task.MaxRetries

	if task.CanRetry() {
		t.Error("Should not be able to retry at max retries")
	}
}

func TestTaskIsExpired(t *testing.T) {
	task := NewTask("test-task", "test-type", "Test Task")

	// No deadline - not expired
	if task.IsExpired() {
		t.Error("Task without deadline should not be expired")
	}

	// Past deadline - expired
	pastDeadline := time.Now().Add(-1 * time.Hour)
	task.Deadline = &pastDeadline

	if !task.IsExpired() {
		t.Error("Task with past deadline should be expired")
	}

	// Future deadline - not expired
	futureDeadline := time.Now().Add(1 * time.Hour)
	task.Deadline = &futureDeadline

	if task.IsExpired() {
		t.Error("Task with future deadline should not be expired")
	}
}

func TestTaskDuration(t *testing.T) {
	task := NewTask("test-task", "test-type", "Test Task")

	// No start/completion time
	if task.Duration() != 0 {
		t.Errorf("Expected duration 0, got %v", task.Duration())
	}

	// Set start and completion times
	startTime := time.Now().Add(-1 * time.Hour)
	completionTime := time.Now()

	task.StartedAt = &startTime
	task.CompletedAt = &completionTime

	duration := task.Duration()
	expectedDuration := completionTime.Sub(startTime)

	if duration != expectedDuration {
		t.Errorf("Expected duration %v, got %v", expectedDuration, duration)
	}
}

func TestTaskAssignToWorker(t *testing.T) {
	task := NewTask("test-task", "test-type", "Test Task")

	task.AssignToWorker("worker-123")

	if task.WorkerID != "worker-123" {
		t.Errorf("Expected worker ID 'worker-123', got '%s'", task.WorkerID)
	}

	if task.Status != StatusAssigned {
		t.Errorf("Expected status Assigned, got %v", task.Status)
	}
}

func TestTaskSetResult(t *testing.T) {
	task := NewTask("test-task", "test-type", "Test Task")

	result := map[string]interface{}{
		"output": "test result",
		"count":  42,
	}

	task.SetResult(result)

	if task.Result != result {
		t.Errorf("Expected result %v, got %v", result, task.Result)
	}

	if task.Progress != 100 {
		t.Errorf("Expected progress 100, got %d", task.Progress)
	}
}

func TestTaskSetError(t *testing.T) {
	task := NewTask("test-task", "test-type", "Test Task")

	testErr := fmt.Errorf("test error")
	task.SetError(testErr)

	if task.Error != "test error" {
		t.Errorf("Expected error 'test error', got '%s'", task.Error)
	}

	// Test nil error
	task.SetError(nil)
	if task.Error != "" {
		t.Errorf("Expected empty error for nil, got '%s'", task.Error)
	}
}

func TestTaskGetInfo(t *testing.T) {
	task := NewTask("test-task", "test-type", "Test Task")
	task.Description = "Test description"
	task.Priority = PriorityHigh
	task.Tags = []string{"tag1", "tag2"}

	info := task.GetInfo()

	if info["id"] != "test-task" {
		t.Errorf("Expected ID 'test-task', got %v", info["id"])
	}

	if info["type"] != "test-type" {
		t.Errorf("Expected type 'test-type', got %v", info["type"])
	}

	if info["description"] != "Test description" {
		t.Errorf("Expected description 'Test description', got %v", info["description"])
	}

	if info["priority"] != PriorityHigh {
		t.Errorf("Expected priority %v, got %v", PriorityHigh, info["priority"])
	}

	if len(info["tags"].([]string)) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(info["tags"].([]string)))
	}
}

func TestNewTaskManager(t *testing.T) {
	config := &config.TasksConfig{}
	manager := NewTaskManager(config)

	if manager == nil {
		t.Fatal("NewTaskManager returned nil")
	}

	if len(manager.tasks) != 0 {
		t.Error("New manager should have no tasks")
	}

	if len(manager.workers) != 0 {
		t.Error("New manager should have no workers")
	}

	if manager.queue.Size() != 0 {
		t.Error("New manager should have empty queue")
	}
}

func TestTaskManagerSubmitTask(t *testing.T) {
	manager := NewTaskManager(&config.TasksConfig{})
	task := NewTask("test-task", "test-type", "Test Task")

	err := manager.SubmitTask(task)
	if err != nil {
		t.Fatalf("Failed to submit task: %v", err)
	}

	// Check task was added
	retrieved, exists := manager.GetTask("test-task")
	if !exists {
		t.Fatal("Task should exist after submission")
	}

	if retrieved != task {
		t.Error("Retrieved task should be the same as submitted")
	}

	// Check task was queued
	if manager.queue.Size() != 1 {
		t.Errorf("Expected queue size 1, got %d", manager.queue.Size())
	}

	// Test duplicate submission
	err = manager.SubmitTask(task)
	if err == nil {
		t.Error("Expected error for duplicate task submission")
	}
}

func TestTaskManagerGetTask(t *testing.T) {
	manager := NewTaskManager(&config.TasksConfig{})
	task := NewTask("test-task", "test-type", "Test Task")

	manager.SubmitTask(task)

	// Get existing task
	retrieved, exists := manager.GetTask("test-task")
	if !exists {
		t.Fatal("Task should exist")
	}

	if retrieved != task {
		t.Error("Retrieved task should be the same")
	}

	// Get non-existent task
	_, exists = manager.GetTask("non-existent")
	if exists {
		t.Error("Non-existent task should not exist")
	}
}

func TestTaskManagerCancelTask(t *testing.T) {
	manager := NewTaskManager(&config.TasksConfig{})
	task := NewTask("test-task", "test-type", "Test Task")

	manager.SubmitTask(task)

	// Cancel task
	err := manager.CancelTask("test-task")
	if err != nil {
		t.Fatalf("Failed to cancel task: %v", err)
	}

	if task.Status != StatusCancelled {
		t.Errorf("Expected status Cancelled, got %v", task.Status)
	}

	// Try to cancel completed task
	task.UpdateStatus(StatusCompleted)
	err = manager.CancelTask("test-task")
	if err == nil {
		t.Error("Expected error when cancelling completed task")
	}
}

func TestTaskManagerRetryTask(t *testing.T) {
	manager := NewTaskManager(&config.TasksConfig{})
	task := NewTask("test-task", "test-type", "Test Task")

	manager.SubmitTask(task)

	// Mark as failed
	task.UpdateStatus(StatusFailed)

	// Retry task
	err := manager.RetryTask("test-task")
	if err != nil {
		t.Fatalf("Failed to retry task: %v", err)
	}

	if task.Status != StatusPending {
		t.Errorf("Expected status Pending after retry, got %v", task.Status)
	}

	if task.Retries != 0 {
		t.Error("Retries should be reset on retry")
	}

	// Test retry non-failed task
	task.UpdateStatus(StatusCompleted)
	err = manager.RetryTask("test-task")
	if err == nil {
		t.Error("Expected error when retrying non-failed task")
	}
}

func TestTaskManagerGetTasksByStatus(t *testing.T) {
	manager := NewTaskManager(&config.TasksConfig{})

	// Create tasks with different statuses
	pendingTask := NewTask("pending", "test", "Pending Task")
	runningTask := NewTask("running", "test", "Running Task")
	runningTask.UpdateStatus(StatusRunning)
	completedTask := NewTask("completed", "test", "Completed Task")
	completedTask.UpdateStatus(StatusCompleted)

	manager.SubmitTask(pendingTask)
	manager.tasks["running"] = runningTask
	manager.tasks["completed"] = completedTask

	// Test getting tasks by status
	pending := manager.GetTasksByStatus(StatusPending)
	if len(pending) != 1 {
		t.Errorf("Expected 1 pending task, got %d", len(pending))
	}

	running := manager.GetTasksByStatus(StatusRunning)
	if len(running) != 1 {
		t.Errorf("Expected 1 running task, got %d", len(running))
	}

	completed := manager.GetTasksByStatus(StatusCompleted)
	if len(completed) != 1 {
		t.Errorf("Expected 1 completed task, got %d", len(completed))
	}
}

func TestTaskManagerRegisterWorker(t *testing.T) {
	manager := NewTaskManager(&config.TasksConfig{})

	// Register worker
	manager.RegisterWorker("worker-1")

	workers := manager.GetAvailableWorkers()
	if len(workers) != 1 {
		t.Errorf("Expected 1 available worker, got %d", len(workers))
	}

	if workers[0] != "worker-1" {
		t.Errorf("Expected worker 'worker-1', got '%s'", workers[0])
	}

	// Unregister worker
	manager.UnregisterWorker("worker-1")

	workers = manager.GetAvailableWorkers()
	if len(workers) != 0 {
		t.Errorf("Expected 0 available workers after unregister, got %d", len(workers))
	}
}

func TestTaskManagerAssignTask(t *testing.T) {
	manager := NewTaskManager(&config.TasksConfig{})

	// Register worker
	manager.RegisterWorker("worker-1")

	// Submit task
	task := NewTask("test-task", "test-type", "Test Task")
	manager.SubmitTask(task)

	// Assign task
	ctx := context.Background()
	assignedTask, err := manager.AssignTask(ctx, "worker-1")
	if err != nil {
		t.Fatalf("Failed to assign task: %v", err)
	}

	if assignedTask != task {
		t.Error("Assigned task should be the submitted task")
	}

	if assignedTask.WorkerID != "worker-1" {
		t.Errorf("Expected worker ID 'worker-1', got '%s'", assignedTask.WorkerID)
	}

	if assignedTask.Status != StatusAssigned {
		t.Errorf("Expected status Assigned, got %v", assignedTask.Status)
	}

	// Worker should not be available
	workers := manager.GetAvailableWorkers()
	if len(workers) != 0 {
		t.Errorf("Expected 0 available workers after assignment, got %d", len(workers))
	}
}

func TestTaskManagerCompleteTask(t *testing.T) {
	manager := NewTaskManager(&config.TasksConfig{})

	// Register worker and submit task
	manager.RegisterWorker("worker-1")
	task := NewTask("test-task", "test-type", "Test Task")
	manager.SubmitTask(task)

	// Assign task
	ctx := context.Background()
	_, err := manager.AssignTask(ctx, "worker-1")
	if err != nil {
		t.Fatalf("Failed to assign task: %v", err)
	}

	// Complete task
	result := "task completed successfully"
	err = manager.CompleteTask("test-task", result)
	if err != nil {
		t.Fatalf("Failed to complete task: %v", err)
	}

	if task.Status != StatusCompleted {
		t.Errorf("Expected status Completed, got %v", task.Status)
	}

	if task.Result != result {
		t.Errorf("Expected result %v, got %v", result, task.Result)
	}

	// Worker should be available again
	workers := manager.GetAvailableWorkers()
	if len(workers) != 1 {
		t.Errorf("Expected 1 available worker after completion, got %d", len(workers))
	}
}

func TestTaskManagerFailTask(t *testing.T) {
	manager := NewTaskManager(&config.TasksConfig{})

	// Register worker and submit task
	manager.RegisterWorker("worker-1")
	task := NewTask("test-task", "test-type", "Test Task")
	manager.SubmitTask(task)

	// Assign task
	ctx := context.Background()
	_, err := manager.AssignTask(ctx, "worker-1")
	if err != nil {
		t.Fatalf("Failed to assign task: %v", err)
	}

	// Fail task
	testErr := fmt.Errorf("task failed")
	err = manager.FailTask("test-task", testErr)
	if err != nil {
		t.Fatalf("Failed to fail task: %v", err)
	}

	if task.Status != StatusFailed {
		t.Errorf("Expected status Failed, got %v", task.Status)
	}

	if task.Error != "task failed" {
		t.Errorf("Expected error 'task failed', got '%s'", task.Error)
	}

	// Worker should be available again
	workers := manager.GetAvailableWorkers()
	if len(workers) != 1 {
		t.Errorf("Expected 1 available worker after failure, got %d", len(workers))
	}
}

func TestTaskManagerGetStatistics(t *testing.T) {
	manager := NewTaskManager(&config.TasksConfig{})

	// Add tasks with different statuses
	pendingTask := NewTask("pending", "test", "Pending")
	runningTask := NewTask("running", "test", "Running")
	runningTask.UpdateStatus(StatusRunning)
	completedTask := NewTask("completed", "test", "Completed")
	completedTask.UpdateStatus(StatusCompleted)
	failedTask := NewTask("failed", "test", "Failed")
	failedTask.UpdateStatus(StatusFailed)

	manager.tasks["pending"] = pendingTask
	manager.tasks["running"] = runningTask
	manager.tasks["completed"] = completedTask
	manager.tasks["failed"] = failedTask

	// Add worker
	manager.RegisterWorker("worker-1")

	stats := manager.GetStatistics()

	if stats["total_tasks"] != 4 {
		t.Errorf("Expected 4 total tasks, got %v", stats["total_tasks"])
	}

	if stats["pending_tasks"] != 1 {
		t.Errorf("Expected 1 pending task, got %v", stats["pending_tasks"])
	}

	if stats["running_tasks"] != 1 {
		t.Errorf("Expected 1 running task, got %v", stats["running_tasks"])
	}

	if stats["completed_tasks"] != 1 {
		t.Errorf("Expected 1 completed task, got %v", stats["completed_tasks"])
	}

	if stats["failed_tasks"] != 1 {
		t.Errorf("Expected 1 failed task, got %v", stats["failed_tasks"])
	}

	if stats["available_workers"] != 1 {
		t.Errorf("Expected 1 available worker, got %v", stats["available_workers"])
	}
}

func TestTaskQueue(t *testing.T) {
	queue := NewTaskQueue()

	if !queue.IsEmpty() {
		t.Error("New queue should be empty")
	}

	if queue.Size() != 0 {
		t.Error("New queue should have size 0")
	}

	// Enqueue tasks with different priorities
	lowPriority := NewTask("low", "test", "Low Priority")
	lowPriority.Priority = PriorityLow

	normalPriority := NewTask("normal", "test", "Normal Priority")
	normalPriority.Priority = PriorityNormal

	highPriority := NewTask("high", "test", "High Priority")
	highPriority.Priority = PriorityHigh

	queue.Enqueue(lowPriority)
	queue.Enqueue(normalPriority)
	queue.Enqueue(highPriority)

	if queue.Size() != 3 {
		t.Errorf("Expected queue size 3, got %d", queue.Size())
	}

	// Dequeue should return highest priority first (high)
	first := queue.Dequeue()
	if first.ID != "high" {
		t.Errorf("Expected first task 'high', got '%s'", first.ID)
	}

	// Then normal
	second := queue.Dequeue()
	if second.ID != "normal" {
		t.Errorf("Expected second task 'normal', got '%s'", second.ID)
	}

	// Then low
	third := queue.Dequeue()
	if third.ID != "low" {
		t.Errorf("Expected third task 'low', got '%s'", third.ID)
	}

	if !queue.IsEmpty() {
		t.Error("Queue should be empty after dequeuing all tasks")
	}
}

func TestTaskQueuePeek(t *testing.T) {
	queue := NewTaskQueue()

	task := NewTask("test", "test", "Test Task")
	queue.Enqueue(task)

	// Peek should return task without removing
	peeked := queue.Peek()
	if peeked != task {
		t.Error("Peek should return the task")
	}

	if queue.Size() != 1 {
		t.Error("Queue size should remain 1 after peek")
	}

	// Dequeue should still work
	dequeued := queue.Dequeue()
	if dequeued != task {
		t.Error("Dequeue should return the same task")
	}
}

func TestTaskQueueClear(t *testing.T) {
	queue := NewTaskQueue()

	queue.Enqueue(NewTask("test1", "test", "Test 1"))
	queue.Enqueue(NewTask("test2", "test", "Test 2"))

	if queue.Size() != 2 {
		t.Errorf("Expected queue size 2, got %d", queue.Size())
	}

	queue.Clear()

	if queue.Size() != 0 {
		t.Errorf("Expected queue size 0 after clear, got %d", queue.Size())
	}

	if !queue.IsEmpty() {
		t.Error("Queue should be empty after clear")
	}
}

func TestGlobalManager(t *testing.T) {
	// Initialize global manager
	config := &config.TasksConfig{}
	InitializeGlobalManager(config)

	manager := GetGlobalManager()
	if manager == nil {
		t.Fatal("Global manager not initialized")
	}

	// Test global functions
	task := NewTask("global-test", "test", "Global Test Task")

	err := SubmitTaskGlobal(task)
	if err != nil {
		t.Fatalf("Failed to submit task globally: %v", err)
	}

	// Get task
	retrieved, exists := GetTaskGlobal("global-test")
	if !exists {
		t.Fatal("Task should exist globally")
	}

	if retrieved != task {
		t.Error("Retrieved task should be the same")
	}

	// Assign task
	ctx := context.Background()
	manager.RegisterWorker("global-worker")

	assigned, err := AssignTaskGlobal(ctx, "global-worker")
	if err != nil {
		t.Fatalf("Failed to assign task globally: %v", err)
	}

	if assigned != task {
		t.Error("Assigned task should be the submitted task")
	}
}

func BenchmarkTaskManagerSubmitTask(b *testing.B) {
	manager := NewTaskManager(&config.TasksConfig{})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		task := NewTask(fmt.Sprintf("bench-task-%d", i), "bench", "Benchmark Task")
		manager.SubmitTask(task)
	}
}

func BenchmarkTaskManagerGetTask(b *testing.B) {
	manager := NewTaskManager(&config.TasksConfig{})

	// Pre-populate with tasks
	for i := 0; i < 1000; i++ {
		task := NewTask(fmt.Sprintf("task-%d", i), "test", "Test Task")
		manager.SubmitTask(task)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		taskID := fmt.Sprintf("task-%d", i%1000)
		manager.GetTask(taskID)
	}
}

func BenchmarkTaskQueueEnqueue(b *testing.B) {
	queue := NewTaskQueue()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		task := NewTask(fmt.Sprintf("queue-task-%d", i), "test", "Queue Task")
		queue.Enqueue(task)
	}
}

func BenchmarkTaskQueueDequeue(b *testing.B) {
	queue := NewTaskQueue()

	// Pre-populate queue
	for i := 0; i < 1000; i++ {
		task := NewTask(fmt.Sprintf("dequeue-task-%d", i), "test", "Dequeue Task")
		queue.Enqueue(task)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if queue.Size() > 0 {
			queue.Dequeue()
		}
	}
}
