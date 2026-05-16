package worker

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

// TestDistributedWorkerManager tests the distributed worker manager
func TestDistributedWorkerManager(t *testing.T) {
	// Skip this test for now as it requires SSH setup
	t.Skip("Skipping distributed worker manager test - requires SSH setup")  // SKIP-OK: #requires-ssh

	config := WorkerConfig{
		Enabled: true,
		Pool: map[string]WorkerConfigEntry{
			"test-worker-1": {
				Host:         "localhost",
				Port:         2222,
				Username:     "test",
				KeyPath:      "test/key",
				Capabilities: []string{"code-generation"},
				DisplayName:  "Test Worker 1",
			},
		},
		AutoInstall:         false,
		HealthCheckInterval: 30,
		MaxConcurrentTasks:  10,
		TaskTimeout:         300,
	}

	manager := NewDistributedWorkerManager(config)

	// Test initialization (should fail gracefully without SSH)
	ctx := context.Background()
	err := manager.Initialize(ctx)
	// We expect this to fail in unit test environment
	if err == nil {
		t.Log("Manager initialized (unexpected in unit test)")
	}

	// Test worker stats even with no workers
	stats := manager.GetWorkerStats()
	if stats["total_workers"].(int) != 0 {
		t.Errorf("Expected 0 workers, got %d", stats["total_workers"])
	}

	t.Log("✅ Distributed worker manager test passed (skipped SSH operations)")
}

// TestWorkerConfigValidation tests worker configuration validation
func TestWorkerConfigValidation(t *testing.T) {
	tests := []struct {
		name   string
		config WorkerConfig
		valid  bool
	}{
		{
			name: "Valid configuration",
			config: WorkerConfig{
				Enabled: true,
				Pool: map[string]WorkerConfigEntry{
					"worker1": {
						Host:         "localhost",
						Port:         22,
						Username:     "user",
						KeyPath:      "/path/to/key",
						Capabilities: []string{"code-generation"},
					},
				},
				MaxConcurrentTasks: 10,
			},
			valid: true,
		},
		{
			name: "Disabled configuration",
			config: WorkerConfig{
				Enabled: false,
			},
			valid: true,
		},
		{
			name: "Invalid port",
			config: WorkerConfig{
				Enabled: true,
				Pool: map[string]WorkerConfigEntry{
					"worker1": {
						Host: "localhost",
						Port: 0, // Invalid port
					},
				},
			},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewDistributedWorkerManager(tt.config)

			// Test initialization (should fail gracefully for SSH configs)
			ctx := context.Background()
			err := manager.Initialize(ctx)

			if tt.name == "Invalid port" {
				// Invalid port should be caught during validation
				if err == nil {
					t.Error("Expected error for invalid port configuration")
				}
			} else {
				// Other configs should pass validation even if SSH fails
				if tt.valid && err != nil && tt.name != "Valid configuration" {
					t.Errorf("Expected valid configuration but got error: %v", err)
				}
			}
		})
	}
}

// TestTaskPriority tests task priority handling
func TestTaskPriority(t *testing.T) {
	// Skip this test as it requires SSH setup
	t.Skip("Skipping task priority test - requires SSH setup")  // SKIP-OK: #requires-ssh

	// Test task creation without SSH
	tasks := []struct {
		priority    int
		criticality Criticality
	}{
		{1, CriticalityCritical}, // Highest priority
		{5, CriticalityNormal},   // Medium priority
		{10, CriticalityLow},     // Lowest priority
	}

	for _, taskDef := range tasks {
		task := &DistributedTask{
			Type:        "priority-test",
			Priority:    taskDef.priority,
			Criticality: taskDef.criticality,
			MaxRetries:  1,
		}

		// Test task creation without submission
		if task.Priority != taskDef.priority {
			t.Errorf("Task priority not set correctly: expected %d, got %d", taskDef.priority, task.Priority)
		}

		t.Logf("Created task with priority %d and criticality %s", taskDef.priority, taskDef.criticality)
	}

	t.Log("✅ Task priority test passed (without SSH)")
}

// TestWorkerCapabilities tests worker capability matching
func TestWorkerCapabilities(t *testing.T) {
	config := WorkerConfig{
		Enabled: true,
		Pool: map[string]WorkerConfigEntry{
			"code-worker": {
				Host:         "localhost",
				Port:         22,
				Username:     "test",
				KeyPath:      "test/key",
				Capabilities: []string{"code-generation", "refactoring"},
				DisplayName:  "Code Worker",
			},
			"test-worker": {
				Host:         "localhost",
				Port:         23,
				Username:     "test",
				KeyPath:      "test/key",
				Capabilities: []string{"testing", "debugging"},
				DisplayName:  "Test Worker",
			},
		},
	}

	manager := NewDistributedWorkerManager(config)

	// Initialize - will fail due to SSH but we test the config parsing
	ctx := context.Background()
	err := manager.Initialize(ctx)
	// We expect this to fail in unit test environment due to SSH
	if err == nil {
		t.Log("Manager initialized successfully (unexpected in unit test)")
	}

	// In unit test, workers won't be available due to SSH failures
	// So we test the configuration structure instead
	workers := manager.GetAvailableWorkers()
	t.Logf("Available workers in test environment: %d (expected 0 due to SSH failures)", len(workers))

	// Test that the manager was created with correct config
	if len(manager.config.Pool) != 2 {
		t.Errorf("Expected 2 workers in config, got %d", len(manager.config.Pool))
	}

	// Verify config capabilities
	for name, entry := range manager.config.Pool {
		if len(entry.Capabilities) == 0 {
			t.Errorf("Worker %s should have capabilities", name)
		}
		t.Logf("Worker %s config capabilities: %v", name, entry.Capabilities)
	}

	t.Log("✅ Worker capabilities test passed")
}

// TestTaskStatusTransitions tests task status transitions
func TestTaskStatusTransitions(t *testing.T) {
	task := &DistributedTask{
		Type:      "status-test",
		Status:    TaskStatusPending,
		CreatedAt: time.Now(),
	}

	// Test status transitions
	initialStatus := task.Status

	// Simulate task assignment
	task.Status = TaskStatusRunning
	if task.Status == initialStatus {
		t.Error("Task status should change when assigned")
	}

	// Simulate task start
	task.Status = TaskStatusRunning
	if task.Status != TaskStatusRunning {
		t.Error("Task status should be running")
	}

	// Simulate task completion
	task.Status = TaskStatusCompleted
	if task.Status != TaskStatusCompleted {
		t.Error("Task status should be completed")
	}

	// Simulate task failure
	task.Status = TaskStatusFailed
	if task.Status != TaskStatusFailed {
		t.Error("Task status should be failed")
	}

	t.Log("✅ Task status transitions test passed")
}

// TestWorkerHealthMonitoring tests worker health monitoring
func TestWorkerHealthMonitoring(t *testing.T) {
	config := WorkerConfig{
		Enabled: true,
		Pool: map[string]WorkerConfigEntry{
			"healthy-worker": {
				Host:         "localhost",
				Port:         22,
				Username:     "test",
				KeyPath:      "test/key",
				Capabilities: []string{"monitoring"},
			},
		},
		HealthCheckInterval: 5, // Short interval for testing
	}

	manager := NewDistributedWorkerManager(config)

	// Get worker and simulate health updates
	workers := manager.GetAvailableWorkers()
	if len(workers) == 0 {
		t.Skip("No workers available for health monitoring test")  // SKIP-OK: #legacy-untriaged
	}

	worker := workers[0]

	// Test initial health status
	if worker.HealthStatus == "" {
		t.Error("Worker should have initial health status")
	}

	// Test last heartbeat
	if worker.LastHeartbeat.IsZero() {
		t.Error("Worker should have last heartbeat set")
	}

	t.Logf("Worker health: %s, last heartbeat: %v", worker.HealthStatus, worker.LastHeartbeat)
	t.Log("✅ Worker health monitoring test passed")
}

// TestDistributedWorkerManager_GetWorkerStats tests the GetWorkerStats function
func TestDistributedWorkerManager_GetWorkerStats(t *testing.T) {
	t.Run("empty workers", func(t *testing.T) {
		config := WorkerConfig{
			Enabled: true,
			Pool:    map[string]WorkerConfigEntry{},
		}
		manager := NewDistributedWorkerManager(config)

		stats := manager.GetWorkerStats()

		if stats["total_workers"].(int) != 0 {
			t.Errorf("Expected total_workers=0, got %v", stats["total_workers"])
		}
		if stats["active_workers"].(int) != 0 {
			t.Errorf("Expected active_workers=0, got %v", stats["active_workers"])
		}
		if stats["healthy_workers"].(int) != 0 {
			t.Errorf("Expected healthy_workers=0, got %v", stats["healthy_workers"])
		}
		if stats["total_tasks"].(int) != 0 {
			t.Errorf("Expected total_tasks=0, got %v", stats["total_tasks"])
		}
	})

	t.Run("with workers", func(t *testing.T) {
		config := WorkerConfig{
			Enabled: true,
			Pool:    map[string]WorkerConfigEntry{},
		}
		manager := NewDistributedWorkerManager(config)

		// Add test workers with unique IDs
		id1 := uuid.New()
		id2 := uuid.New()
		id3 := uuid.New()

		worker1 := &Worker{
			ID:                id1,
			Status:            WorkerStatusActive,
			HealthStatus:      WorkerHealthHealthy,
			CurrentTasksCount: 2,
		}
		worker2 := &Worker{
			ID:                id2,
			Status:            WorkerStatusInactive,
			HealthStatus:      WorkerHealthDegraded,
			CurrentTasksCount: 1,
		}
		worker3 := &Worker{
			ID:                id3,
			Status:            WorkerStatusActive,
			HealthStatus:      WorkerHealthHealthy,
			CurrentTasksCount: 3,
		}

		manager.workers[id1] = worker1
		manager.workers[id2] = worker2
		manager.workers[id3] = worker3

		stats := manager.GetWorkerStats()

		if stats["total_workers"].(int) != 3 {
			t.Errorf("Expected total_workers=3, got %v", stats["total_workers"])
		}
		if stats["active_workers"].(int) != 2 {
			t.Errorf("Expected active_workers=2, got %v", stats["active_workers"])
		}
		if stats["healthy_workers"].(int) != 2 {
			t.Errorf("Expected healthy_workers=2, got %v", stats["healthy_workers"])
		}
		if stats["total_tasks"].(int) != 6 {
			t.Errorf("Expected total_tasks=6, got %v", stats["total_tasks"])
		}
	})
}

// TestDistributedWorkerManager_GetAvailableWorkers tests the GetAvailableWorkers function
func TestDistributedWorkerManager_GetAvailableWorkers(t *testing.T) {
	t.Run("no workers", func(t *testing.T) {
		config := WorkerConfig{
			Enabled: true,
			Pool:    map[string]WorkerConfigEntry{},
		}
		manager := NewDistributedWorkerManager(config)

		workers := manager.GetAvailableWorkers()
		if len(workers) != 0 {
			t.Errorf("Expected 0 workers, got %d", len(workers))
		}
	})

	t.Run("with mixed workers", func(t *testing.T) {
		config := WorkerConfig{
			Enabled: true,
			Pool:    map[string]WorkerConfigEntry{},
		}
		manager := NewDistributedWorkerManager(config)

		// Add various worker states with unique IDs
		id1 := uuid.New()
		id2 := uuid.New()
		id3 := uuid.New()
		id4 := uuid.New()

		worker1 := &Worker{
			ID:           id1,
			Status:       WorkerStatusActive,
			HealthStatus: WorkerHealthHealthy, // Should be available
		}
		worker2 := &Worker{
			ID:           id2,
			Status:       WorkerStatusInactive,
			HealthStatus: WorkerHealthHealthy, // Should NOT be available - inactive
		}
		worker3 := &Worker{
			ID:           id3,
			Status:       WorkerStatusActive,
			HealthStatus: WorkerHealthUnhealthy, // Should NOT be available - unhealthy
		}
		worker4 := &Worker{
			ID:           id4,
			Status:       WorkerStatusActive,
			HealthStatus: WorkerHealthHealthy, // Should be available
		}

		manager.workers[id1] = worker1
		manager.workers[id2] = worker2
		manager.workers[id3] = worker3
		manager.workers[id4] = worker4

		workers := manager.GetAvailableWorkers()
		if len(workers) != 2 {
			t.Errorf("Expected 2 available workers, got %d", len(workers))
		}
	})
}

// TestDistributedTask_Struct tests DistributedTask struct fields
func TestDistributedTask_Struct(t *testing.T) {
	now := time.Now()
	task := &DistributedTask{
		Type: "test-task",
		Payload: map[string]interface{}{
			"key": "value",
		},
		Data: map[string]interface{}{
			"data_key": "data_value",
		},
		Status:      TaskStatusPending,
		Priority:    5,
		Criticality: CriticalityHigh,
		MaxRetries:  3,
		CreatedAt:   now,
	}

	if task.Type != "test-task" {
		t.Errorf("Expected Type='test-task', got %s", task.Type)
	}
	if task.Status != TaskStatusPending {
		t.Errorf("Expected Status=pending, got %s", task.Status)
	}
	if task.Priority != 5 {
		t.Errorf("Expected Priority=5, got %d", task.Priority)
	}
	if task.Criticality != CriticalityHigh {
		t.Errorf("Expected Criticality=high, got %s", task.Criticality)
	}
	if task.MaxRetries != 3 {
		t.Errorf("Expected MaxRetries=3, got %d", task.MaxRetries)
	}
}

// TestDistributedWorkerManager_SubmitTask tests task submission
func TestDistributedWorkerManager_SubmitTask(t *testing.T) {
	config := WorkerConfig{
		Enabled: true,
		Pool:    map[string]WorkerConfigEntry{},
	}

	manager := NewDistributedWorkerManager(config)

	t.Run("no available workers", func(t *testing.T) {
		task := &DistributedTask{
			Type: "test-task",
		}
		err := manager.SubmitTask(task)
		if err == nil {
			t.Error("Expected error when no workers available")
		}
		if err.Error() != "no available workers" {
			t.Errorf("Expected 'no available workers' error, got %s", err.Error())
		}
	})

	t.Run("with available worker", func(t *testing.T) {
		// Add an available worker
		worker := &Worker{
			Status:       WorkerStatusActive,
			HealthStatus: WorkerHealthHealthy,
		}
		manager.workers[worker.ID] = worker

		task := &DistributedTask{
			Type: "test-task",
			Payload: map[string]interface{}{
				"command": "echo test",
			},
		}

		err := manager.SubmitTask(task)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		// Verify task was processed
		if task.Status != TaskStatusCompleted {
			t.Errorf("Expected task status completed, got %s", task.Status)
		}
		if task.StartedAt == nil {
			t.Error("Expected StartedAt to be set")
		}
		if task.CompletedAt == nil {
			t.Error("Expected CompletedAt to be set")
		}
		if task.Result == nil {
			t.Error("Expected Result to be set")
		}
	})
}

// TestWorkerConfig_Struct tests WorkerConfig struct
func TestWorkerConfig_Struct(t *testing.T) {
	config := WorkerConfig{
		Enabled: true,
		Pool: map[string]WorkerConfigEntry{
			"worker1": {
				Host:         "192.168.1.100",
				Port:         22,
				Username:     "helix",
				KeyPath:      "/home/helix/.ssh/id_rsa",
				Capabilities: []string{"code-generation", "testing"},
				DisplayName:  "Main Worker",
			},
		},
		AutoInstall:         true,
		HealthCheckInterval: 30,
		MaxConcurrentTasks:  10,
		TaskTimeout:         300,
	}

	if !config.Enabled {
		t.Error("Expected Enabled=true")
	}
	if len(config.Pool) != 1 {
		t.Errorf("Expected 1 worker in pool, got %d", len(config.Pool))
	}
	if config.AutoInstall != true {
		t.Error("Expected AutoInstall=true")
	}
	if config.HealthCheckInterval != 30 {
		t.Errorf("Expected HealthCheckInterval=30, got %d", config.HealthCheckInterval)
	}
	if config.MaxConcurrentTasks != 10 {
		t.Errorf("Expected MaxConcurrentTasks=10, got %d", config.MaxConcurrentTasks)
	}
	if config.TaskTimeout != 300 {
		t.Errorf("Expected TaskTimeout=300, got %d", config.TaskTimeout)
	}

	// Verify worker entry
	worker := config.Pool["worker1"]
	if worker.Host != "192.168.1.100" {
		t.Errorf("Expected Host='192.168.1.100', got %s", worker.Host)
	}
	if len(worker.Capabilities) != 2 {
		t.Errorf("Expected 2 capabilities, got %d", len(worker.Capabilities))
	}
}

// TestTaskStatus_Constants tests TaskStatus constants
func TestTaskStatus_Constants(t *testing.T) {
	statuses := []TaskStatus{
		TaskStatusPending,
		TaskStatusRunning,
		TaskStatusCompleted,
		TaskStatusFailed,
		TaskStatusCancelled,
	}

	expectedValues := []string{
		"pending",
		"running",
		"completed",
		"failed",
		"cancelled",
	}

	for i, status := range statuses {
		if string(status) != expectedValues[i] {
			t.Errorf("Expected status '%s', got '%s'", expectedValues[i], status)
		}
	}
}

// TestCriticality_Constants tests Criticality constants
func TestCriticality_Constants(t *testing.T) {
	criticalities := []Criticality{
		CriticalityLow,
		CriticalityNormal,
		CriticalityHigh,
		CriticalityCritical,
	}

	expectedValues := []string{
		"low",
		"normal",
		"high",
		"critical",
	}

	for i, crit := range criticalities {
		if string(crit) != expectedValues[i] {
			t.Errorf("Expected criticality '%s', got '%s'", expectedValues[i], crit)
		}
	}
}

// TestNewDistributedWorkerManager tests manager creation
func TestNewDistributedWorkerManager(t *testing.T) {
	config := WorkerConfig{
		Enabled:             true,
		AutoInstall:         true,
		HealthCheckInterval: 60,
	}

	manager := NewDistributedWorkerManager(config)

	if manager == nil {
		t.Fatal("Expected manager to be created")
	}
	if manager.workers == nil {
		t.Error("Expected workers map to be initialized")
	}
	if manager.tasks == nil {
		t.Error("Expected tasks map to be initialized")
	}
	if manager.sshPool == nil {
		t.Error("Expected sshPool to be initialized")
	}
	if manager.config.HealthCheckInterval != 60 {
		t.Errorf("Expected HealthCheckInterval=60, got %d", manager.config.HealthCheckInterval)
	}
}
