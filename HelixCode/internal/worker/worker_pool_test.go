package worker

import (
	"context"
	"testing"
	"time"

	"dev.helix.code/internal/config"
)

func TestNewPoolWorker(t *testing.T) {
	capabilities := WorkerCapabilities{
		CPUCores: 4,
		MemoryGB: 8,
		DiskGB:   100,
		OS:       "linux",
		Arch:     "amd64",
		Tags:     []string{"general"},
	}

	worker := NewPoolWorker("test-worker", "Test Worker", "localhost:8080", capabilities)

	if worker == nil {
		t.Fatal("NewPoolWorker returned nil")
	}

	if worker.ID != "test-worker" {
		t.Errorf("Expected ID 'test-worker', got '%s'", worker.ID)
	}

	if worker.Name != "Test Worker" {
		t.Errorf("Expected name 'Test Worker', got '%s'", worker.Name)
	}

	if worker.Status != StatusAvailable {
		t.Errorf("Expected status Available, got %v", worker.Status)
	}

	if worker.Capabilities.CPUCores != 4 {
		t.Errorf("Expected 4 CPU cores, got %d", worker.Capabilities.CPUCores)
	}
}

func TestWorkerUpdateStatus(t *testing.T) {
	worker := NewPoolWorker("test-worker", "Test Worker", "localhost:8080", WorkerCapabilities{})

	worker.UpdateStatus(StatusBusy)
	if worker.Status != StatusBusy {
		t.Errorf("Expected status Busy, got %v", worker.Status)
	}

	worker.UpdateStatus(StatusError)
	if worker.Status != StatusError {
		t.Errorf("Expected status Error, got %v", worker.Status)
	}
}

func TestWorkerIsAvailable(t *testing.T) {
	worker := NewPoolWorker("test-worker", "Test Worker", "localhost:8080", WorkerCapabilities{})

	// Initially available
	if !worker.IsAvailable() {
		t.Error("Worker should be available initially")
	}

	// Not available when busy
	worker.UpdateStatus(StatusBusy)
	if worker.IsAvailable() {
		t.Error("Worker should not be available when busy")
	}

	// Not available when offline
	worker.UpdateStatus(StatusOffline)
	if worker.IsAvailable() {
		t.Error("Worker should not be available when offline")
	}

	// Not available when in error
	worker.UpdateStatus(StatusError)
	if worker.IsAvailable() {
		t.Error("Worker should not be available when in error")
	}
}

func TestWorkerCanHandleTask(t *testing.T) {
	capabilities := WorkerCapabilities{
		CPUCores:    8,
		MemoryGB:    16,
		DiskGB:      200,
		GPUs:        1,
		Tags:        []string{"gpu", "high-memory"},
		Specialized: []string{"gpu", "high-memory"},
	}

	worker := NewPoolWorker("test-worker", "Test Worker", "localhost:8080", capabilities)

	tests := []struct {
		name         string
		requirements map[string]interface{}
		expected     bool
	}{
		{
			name:         "Basic task",
			requirements: map[string]interface{}{},
			expected:     true,
		},
		{
			name: "CPU requirements met",
			requirements: map[string]interface{}{
				"cpu_cores": 4,
			},
			expected: true,
		},
		{
			name: "CPU requirements not met",
			requirements: map[string]interface{}{
				"cpu_cores": 16,
			},
			expected: false,
		},
		{
			name: "Memory requirements met",
			requirements: map[string]interface{}{
				"memory_gb": 8,
			},
			expected: true,
		},
		{
			name: "Memory requirements not met",
			requirements: map[string]interface{}{
				"memory_gb": 32,
			},
			expected: false,
		},
		{
			name: "GPU requirements met",
			requirements: map[string]interface{}{
				"gpus": 1,
			},
			expected: true,
		},
		{
			name: "GPU requirements not met",
			requirements: map[string]interface{}{
				"gpus": 2,
			},
			expected: false,
		},
		{
			name: "Tag requirements met",
			requirements: map[string]interface{}{
				"tags": []string{"gpu"},
			},
			expected: true,
		},
		{
			name: "Tag requirements not met",
			requirements: map[string]interface{}{
				"tags": []string{"cpu-only"},
			},
			expected: false,
		},
		{
			name: "Specialized requirements met",
			requirements: map[string]interface{}{
				"specialized": []string{"gpu", "high-memory"},
			},
			expected: true,
		},
		{
			name: "Specialized requirements partially met",
			requirements: map[string]interface{}{
				"specialized": []string{"gpu", "cpu-only"},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := worker.CanHandleTask("test-task", tt.requirements)
			if result != tt.expected {
				t.Errorf("CanHandleTask() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestWorkerCanHandleTaskWhenBusy(t *testing.T) {
	worker := NewPoolWorker("test-worker", "Test Worker", "localhost:8080", WorkerCapabilities{})

	// Initially can handle
	if !worker.CanHandleTask("test-task", map[string]interface{}{}) {
		t.Error("Worker should be able to handle task when available")
	}

	// Cannot handle when busy
	worker.UpdateStatus(StatusBusy)
	if worker.CanHandleTask("test-task", map[string]interface{}{}) {
		t.Error("Worker should not be able to handle task when busy")
	}

	// Cannot handle when offline
	worker.UpdateStatus(StatusOffline)
	if worker.CanHandleTask("test-task", map[string]interface{}{}) {
		t.Error("Worker should not be able to handle task when offline")
	}
}

func TestWorkerUpdateStats(t *testing.T) {
	worker := NewPoolWorker("test-worker", "Test Worker", "localhost:8080", WorkerCapabilities{})

	// Initial stats
	if worker.TasksProcessed != 0 {
		t.Errorf("Expected 0 tasks processed initially, got %d", worker.TasksProcessed)
	}

	// Update stats
	worker.UpdateStats(5, 4, 1, 2.5)

	if worker.TasksProcessed != 5 {
		t.Errorf("Expected 5 tasks processed, got %d", worker.TasksProcessed)
	}

	if worker.TasksSucceeded != 4 {
		t.Errorf("Expected 4 tasks succeeded, got %d", worker.TasksSucceeded)
	}

	if worker.TasksFailed != 1 {
		t.Errorf("Expected 1 task failed, got %d", worker.TasksFailed)
	}

	if worker.LoadAverage != 2.5 {
		t.Errorf("Expected load average 2.5, got %f", worker.LoadAverage)
	}
}

func TestWorkerGetHealth(t *testing.T) {
	capabilities := WorkerCapabilities{
		CPUCores: 4,
		MemoryGB: 8,
		Tags:     []string{"test"},
	}

	worker := NewPoolWorker("test-worker", "Test Worker", "localhost:8080", capabilities)
	worker.UpdateStats(10, 8, 2, 1.5)

	health := worker.GetHealth()

	if health["id"] != "test-worker" {
		t.Errorf("Expected ID 'test-worker', got %v", health["id"])
	}

	if health["status"] != "available" {
		t.Errorf("Expected status 'available', got %v", health["status"])
	}

	if health["tasks_processed"] != 10 {
		t.Errorf("Expected 10 tasks processed, got %v", health["tasks_processed"])
	}

	if health["success_rate"] != 80.0 {
		t.Errorf("Expected 80%% success rate, got %v", health["success_rate"])
	}

	if health["load_average"] != 1.5 {
		t.Errorf("Expected load average 1.5, got %v", health["load_average"])
	}
}

func TestNewPoolWorkerPool(t *testing.T) {
	config := &config.WorkersConfig{}
	pool := NewPoolWorkerPool(config)

	if pool == nil {
		t.Fatal("NewPoolWorkerPool returned nil")
	}

	if len(pool.workers) != 0 {
		t.Error("New pool should have no workers")
	}
}

func TestWorkerPoolRegisterUnregister(t *testing.T) {
	pool := NewPoolWorkerPool(&config.WorkersConfig{})
	worker := NewPoolWorker("test-worker", "Test Worker", "localhost:8080", WorkerCapabilities{})

	// Register
	pool.RegisterWorker(worker)

	if len(pool.workers) != 1 {
		t.Errorf("Expected 1 worker after register, got %d", len(pool.workers))
	}

	// Get worker
	retrieved, exists := pool.GetWorker("test-worker")
	if !exists {
		t.Fatal("Worker should exist")
	}

	if retrieved != worker {
		t.Error("Retrieved worker is not the same as registered")
	}

	// Unregister
	pool.UnregisterWorker("test-worker")

	if len(pool.workers) != 0 {
		t.Errorf("Expected 0 workers after unregister, got %d", len(pool.workers))
	}
}

func TestWorkerPoolGetAvailableWorkers(t *testing.T) {
	pool := NewPoolWorkerPool(&config.WorkersConfig{})

	// Add available worker
	worker1 := NewPoolWorker("worker1", "Worker 1", "localhost:8080", WorkerCapabilities{})
	pool.RegisterWorker(worker1)

	// Add busy worker
	worker2 := NewPoolWorker("worker2", "Worker 2", "localhost:8081", WorkerCapabilities{})
	worker2.UpdateStatus(StatusBusy)
	pool.RegisterWorker(worker2)

	// Add offline worker
	worker3 := NewPoolWorker("worker3", "Worker 3", "localhost:8082", WorkerCapabilities{})
	worker3.UpdateStatus(StatusOffline)
	pool.RegisterWorker(worker3)

	available := pool.GetAvailableWorkers()

	if len(available) != 1 {
		t.Errorf("Expected 1 available worker, got %d", len(available))
	}

	if available[0].ID != "worker1" {
		t.Errorf("Expected available worker ID 'worker1', got '%s'", available[0].ID)
	}
}

func TestWorkerPoolAssignTask(t *testing.T) {
	pool := NewPoolWorkerPool(&config.WorkersConfig{})

	// Add worker with capabilities
	capabilities := WorkerCapabilities{
		CPUCores: 4,
		MemoryGB: 8,
		Tags:     []string{"general"},
	}

	worker := NewPoolWorker("test-worker", "Test Worker", "localhost:8080", capabilities)
	pool.RegisterWorker(worker)

	ctx := context.Background()

	// Assign task
	assignedWorker, err := pool.AssignTask(ctx, "test-task", map[string]interface{}{
		"cpu_cores": 2,
		"memory_gb": 4,
	})

	if err != nil {
		t.Fatalf("Failed to assign task: %v", err)
	}

	if assignedWorker == nil {
		t.Fatal("Assigned worker is nil")
	}

	if assignedWorker.ID != "test-worker" {
		t.Errorf("Expected assigned worker ID 'test-worker', got '%s'", assignedWorker.ID)
	}

	if assignedWorker.Status != StatusBusy {
		t.Errorf("Assigned worker should be busy, got status %v", assignedWorker.Status)
	}

	// Worker should not be available for another task
	available := pool.GetAvailableWorkers()
	if len(available) != 0 {
		t.Errorf("Expected 0 available workers after assignment, got %d", len(available))
	}
}

func TestWorkerPoolAssignTaskNoAvailableWorkers(t *testing.T) {
	pool := NewPoolWorkerPool(&config.WorkersConfig{})

	ctx := context.Background()

	// Try to assign task with no workers
	_, err := pool.AssignTask(ctx, "test-task", map[string]interface{}{})
	if err == nil {
		t.Error("Expected error when no workers available")
	}
}

func TestWorkerPoolAssignTaskNoSuitableWorkers(t *testing.T) {
	pool := NewPoolWorkerPool(&config.WorkersConfig{})

	// Add worker without required capabilities
	worker := NewPoolWorker("worker1", "Worker 1", "localhost:8080", WorkerCapabilities{
		CPUCores: 2, // Less than required
	})
	pool.RegisterWorker(worker)

	ctx := context.Background()

	// Try to assign task requiring more CPU than available
	_, err := pool.AssignTask(ctx, "test-task", map[string]interface{}{
		"cpu_cores": 4,
	})
	if err == nil {
		t.Error("Expected error when no suitable workers available")
	}
}

func TestWorkerPoolReleaseWorker(t *testing.T) {
	pool := NewPoolWorkerPool(&config.WorkersConfig{})
	worker := NewPoolWorker("test-worker", "Test Worker", "localhost:8080", WorkerCapabilities{})
	pool.RegisterWorker(worker)

	// Mark as busy
	worker.UpdateStatus(StatusBusy)

	// Release
	pool.ReleaseWorker("test-worker")

	if worker.Status != StatusAvailable {
		t.Errorf("Worker should be available after release, got status %v", worker.Status)
	}

	// Should be in available list
	available := pool.GetAvailableWorkers()
	if len(available) != 1 {
		t.Errorf("Expected 1 available worker after release, got %d", len(available))
	}
}

func TestWorkerPoolGetPoolStats(t *testing.T) {
	pool := NewPoolWorkerPool(&config.WorkersConfig{})

	// Add workers with different statuses
	worker1 := NewPoolWorker("worker1", "Worker 1", "localhost:8080", WorkerCapabilities{})
	pool.RegisterWorker(worker1) // Available

	worker2 := NewPoolWorker("worker2", "Worker 2", "localhost:8081", WorkerCapabilities{})
	worker2.UpdateStatus(StatusBusy)
	pool.RegisterWorker(worker2) // Busy

	worker3 := NewPoolWorker("worker3", "Worker 3", "localhost:8082", WorkerCapabilities{})
	worker3.UpdateStatus(StatusOffline)
	pool.RegisterWorker(worker3) // Offline

	worker4 := NewPoolWorker("worker4", "Worker 4", "localhost:8083", WorkerCapabilities{})
	worker4.UpdateStatus(StatusError)
	pool.RegisterWorker(worker4) // Error

	stats := pool.GetPoolStats()

	if stats["total_workers"] != 4 {
		t.Errorf("Expected 4 total workers, got %v", stats["total_workers"])
	}

	if stats["available_workers"] != 1 {
		t.Errorf("Expected 1 available worker, got %v", stats["available_workers"])
	}

	if stats["busy_workers"] != 1 {
		t.Errorf("Expected 1 busy worker, got %v", stats["busy_workers"])
	}

	if stats["offline_workers"] != 1 {
		t.Errorf("Expected 1 offline worker, got %v", stats["offline_workers"])
	}

	if stats["error_workers"] != 1 {
		t.Errorf("Expected 1 error worker, got %v", stats["error_workers"])
	}

	if stats["utilization_rate"] != 25.0 {
		t.Errorf("Expected 25%% utilization rate, got %v", stats["utilization_rate"])
	}
}

func TestWorkerPoolHealthCheck(t *testing.T) {
	pool := NewPoolWorkerPool(&config.WorkersConfig{})

	// No workers
	err := pool.HealthCheck()
	if err == nil {
		t.Error("Expected error when no workers registered")
	}

	// Add offline worker
	worker := NewPoolWorker("worker1", "Worker 1", "localhost:8080", WorkerCapabilities{})
	worker.UpdateStatus(StatusOffline)
	pool.RegisterWorker(worker)

	err = pool.HealthCheck()
	if err == nil {
		t.Error("Expected error when no available workers")
	}

	// Add available worker
	worker2 := NewPoolWorker("worker2", "Worker 2", "localhost:8081", WorkerCapabilities{})
	pool.RegisterWorker(worker2)

	err = pool.HealthCheck()
	if err != nil {
		t.Errorf("Expected no error when available workers exist, got: %v", err)
	}
}

func TestDefaultScheduler(t *testing.T) {
	scheduler := NewDefaultScheduler()

	workers := []*PoolWorker{
		NewPoolWorker("worker1", "Worker 1", "localhost:8080", WorkerCapabilities{}),
		NewPoolWorker("worker2", "Worker 2", "localhost:8081", WorkerCapabilities{}),
		NewPoolWorker("worker3", "Worker 3", "localhost:8082", WorkerCapabilities{}),
	}

	// Select workers multiple times to test round-robin
	selected := make(map[string]int)

	for i := 0; i < 6; i++ {
		worker := scheduler.SelectWorker(workers, "test-task", map[string]interface{}{})
		if worker != nil {
			selected[worker.ID]++
		}
	}

	// Each worker should be selected twice
	for _, count := range selected {
		if count != 2 {
			t.Errorf("Expected each worker to be selected 2 times, got counts: %v", selected)
		}
	}
}

func TestDefaultSchedulerNoSuitableWorkers(t *testing.T) {
	scheduler := NewDefaultScheduler()

	workers := []*PoolWorker{
		NewPoolWorker("worker1", "Worker 1", "localhost:8080", WorkerCapabilities{}),
	}

	// All workers are busy
	for _, worker := range workers {
		worker.UpdateStatus(StatusBusy)
	}

	selected := scheduler.SelectWorker(workers, "test-task", map[string]interface{}{})
	if selected != nil {
		t.Error("Expected no worker selected when all are busy")
	}
}

func TestPerformanceScheduler(t *testing.T) {
	scheduler := NewPerformanceScheduler()

	// Create workers with different performance metrics
	worker1 := NewPoolWorker("worker1", "Worker 1", "localhost:8080", WorkerCapabilities{})
	worker1.UpdateStats(100, 95, 5, 10.0) // 95% success rate, low load

	worker2 := NewPoolWorker("worker2", "Worker 2", "localhost:8081", WorkerCapabilities{})
	worker2.UpdateStats(100, 80, 20, 50.0) // 80% success rate, high load

	worker3 := NewPoolWorker("worker3", "Worker 3", "localhost:8082", WorkerCapabilities{})
	worker3.UpdateStats(0, 0, 0, 0.0) // New worker

	workers := []*PoolWorker{worker1, worker2, worker3}

	selected := scheduler.SelectWorker(workers, "test-task", map[string]interface{}{})

	if selected == nil {
		t.Fatal("Expected a worker to be selected")
	}

	// Should select worker1 (best performance)
	if selected.ID != "worker1" {
		t.Errorf("Expected worker1 to be selected (best performance), got %s", selected.ID)
	}
}

func TestGlobalPool(t *testing.T) {
	// Initialize global pool
	config := &config.WorkersConfig{}
	InitializeGlobalPool(config)

	pool := GetGlobalPool()
	if pool == nil {
		t.Fatal("Global pool not initialized")
	}

	// Test global functions
	ctx := context.Background()

	// Should fail with no workers
	_, err := AssignTaskGlobal(ctx, "test-task", map[string]interface{}{})
	if err == nil {
		t.Error("Expected error when no workers in global pool")
	}

	// Add a worker
	worker := NewPoolWorker("global-worker", "Global Worker", "localhost:8080", WorkerCapabilities{})
	pool.RegisterWorker(worker)

	// Should work now
	assigned, err := AssignTaskGlobal(ctx, "test-task", map[string]interface{}{})
	if err != nil {
		t.Fatalf("Failed to assign task globally: %v", err)
	}

	if assigned.ID != "global-worker" {
		t.Errorf("Expected global-worker to be assigned, got %s", assigned.ID)
	}

	// Release worker
	ReleaseWorkerGlobal("global-worker")

	if worker.Status != StatusAvailable {
		t.Error("Worker should be available after global release")
	}
}

func BenchmarkWorkerCanHandleTask(b *testing.B) {
	capabilities := WorkerCapabilities{
		CPUCores:    8,
		MemoryGB:    16,
		GPUs:        2,
		Tags:        []string{"gpu", "high-memory", "fast-storage"},
		Specialized: []string{"gpu", "high-memory", "fast-storage"},
	}

	worker := NewPoolWorker("bench-worker", "Bench Worker", "localhost:8080", capabilities)

	requirements := map[string]interface{}{
		"cpu_cores":   4,
		"memory_gb":   8,
		"gpus":        1,
		"tags":        []string{"gpu", "high-memory"},
		"specialized": []string{"gpu"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		worker.CanHandleTask("bench-task", requirements)
	}
}

func BenchmarkWorkerPoolAssignTask(b *testing.B) {
	pool := NewPoolWorkerPool(&config.WorkersConfig{})

	// Add multiple workers
	for i := 0; i < 10; i++ {
		worker := NewPoolWorker(fmt.Sprintf("worker%d", i), fmt.Sprintf("Worker %d", i),
			fmt.Sprintf("localhost:808%d", i), WorkerCapabilities{
				CPUCores: 4,
				MemoryGB: 8,
			})
		pool.RegisterWorker(worker)
	}

	ctx := context.Background()
	requirements := map[string]interface{}{
		"cpu_cores": 2,
		"memory_gb": 4,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		worker, err := pool.AssignTask(ctx, "bench-task", requirements)
		if err != nil {
			b.Fatalf("Failed to assign task: %v", err)
		}
		pool.ReleaseWorker(worker.ID)
	}
}

func BenchmarkDefaultScheduler(b *testing.B) {
	scheduler := NewDefaultScheduler()

	workers := make([]*PoolWorker, 10)
	for i := 0; i < 10; i++ {
		workers[i] = NewPoolWorker(fmt.Sprintf("worker%d", i), fmt.Sprintf("Worker %d", i),
			fmt.Sprintf("localhost:808%d", i), WorkerCapabilities{})
	}

	requirements := map[string]interface{}{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scheduler.SelectWorker(workers, "bench-task", requirements)
	}
}
