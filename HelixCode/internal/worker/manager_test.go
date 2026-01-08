package worker

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockWorkerRepository is a mock implementation of WorkerRepository
type MockWorkerRepository struct {
	workers map[uuid.UUID]*Worker
	mutex   sync.RWMutex
}

func NewMockWorkerRepository() *MockWorkerRepository {
	return &MockWorkerRepository{
		workers: make(map[uuid.UUID]*Worker),
	}
}

func (m *MockWorkerRepository) CreateWorker(ctx context.Context, worker *Worker) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.workers[worker.ID] = worker
	return nil
}

func (m *MockWorkerRepository) GetWorker(ctx context.Context, id uuid.UUID) (*Worker, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	if worker, ok := m.workers[id]; ok {
		return worker, nil
	}
	return nil, fmt.Errorf("worker not found")
}

func (m *MockWorkerRepository) GetWorkerByHostname(ctx context.Context, hostname string) (*Worker, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	for _, worker := range m.workers {
		if worker.Hostname == hostname {
			return worker, nil
		}
	}
	return nil, fmt.Errorf("worker not found")
}

func (m *MockWorkerRepository) ListWorkers(ctx context.Context, status WorkerStatus) ([]*Worker, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	var result []*Worker
	for _, worker := range m.workers {
		if status == "" || worker.Status == status {
			result = append(result, worker)
		}
	}
	return result, nil
}

func (m *MockWorkerRepository) UpdateWorker(ctx context.Context, worker *Worker) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if _, ok := m.workers[worker.ID]; !ok {
		return fmt.Errorf("worker not found")
	}
	m.workers[worker.ID] = worker
	return nil
}

func (m *MockWorkerRepository) DeleteWorker(ctx context.Context, id uuid.UUID) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	delete(m.workers, id)
	return nil
}

func (m *MockWorkerRepository) RecordMetrics(ctx context.Context, metrics *WorkerMetrics) error {
	return nil
}

func (m *MockWorkerRepository) GetWorkerMetrics(ctx context.Context, workerID uuid.UUID, since time.Time) ([]*WorkerMetrics, error) {
	return nil, nil
}

// ========================================
// WorkerManager Tests
// ========================================

func TestNewWorkerManager(t *testing.T) {
	repo := NewMockWorkerRepository()
	manager := NewWorkerManager(repo, 30*time.Second)

	assert.NotNil(t, manager)
	assert.NotNil(t, manager.workers)
	assert.Equal(t, 30*time.Second, manager.healthTTL)
}

func TestWorkerManager_RegisterWorker(t *testing.T) {
	repo := NewMockWorkerRepository()
	manager := NewWorkerManager(repo, 30*time.Second)

	ctx := context.Background()
	worker := &Worker{
		Hostname:    "test-worker-1",
		DisplayName: "Test Worker 1",
		Resources: Resources{
			CPUCount:    4,
			TotalMemory: 8 * 1024 * 1024 * 1024, // 8GB
		},
		MaxConcurrentTasks: 10,
	}

	err := manager.RegisterWorker(ctx, worker)
	require.NoError(t, err)

	// Worker should be assigned an ID
	assert.NotEqual(t, uuid.Nil, worker.ID)
	assert.Equal(t, WorkerStatusActive, worker.Status)
	assert.Equal(t, WorkerHealthHealthy, worker.HealthStatus)
}

func TestWorkerManager_RegisterWorker_ExistingHostname(t *testing.T) {
	repo := NewMockWorkerRepository()
	manager := NewWorkerManager(repo, 30*time.Second)

	ctx := context.Background()

	// Register first worker
	worker1 := &Worker{
		Hostname:    "test-worker-1",
		DisplayName: "Test Worker 1",
	}
	err := manager.RegisterWorker(ctx, worker1)
	require.NoError(t, err)

	firstID := worker1.ID

	// Register second worker with same hostname
	worker2 := &Worker{
		Hostname:    "test-worker-1",
		DisplayName: "Test Worker 1 Updated",
	}
	err = manager.RegisterWorker(ctx, worker2)
	require.NoError(t, err)

	// Should reuse the same ID
	assert.Equal(t, firstID, worker2.ID)
}

func TestWorkerManager_UpdateWorkerHeartbeat(t *testing.T) {
	repo := NewMockWorkerRepository()
	manager := NewWorkerManager(repo, 30*time.Second)

	ctx := context.Background()

	// Register a worker first
	worker := &Worker{
		Hostname: "test-worker-1",
	}
	err := manager.RegisterWorker(ctx, worker)
	require.NoError(t, err)

	// Update heartbeat
	metrics := &WorkerMetrics{
		WorkerID:           worker.ID,
		CPUUsagePercent:    50.0,
		MemoryUsagePercent: 60.0,
		DiskUsagePercent:   70.0,
	}

	err = manager.UpdateWorkerHeartbeat(ctx, worker.ID, metrics)
	require.NoError(t, err)
}

func TestWorkerManager_UpdateWorkerHeartbeat_NotFound(t *testing.T) {
	repo := NewMockWorkerRepository()
	manager := NewWorkerManager(repo, 30*time.Second)

	ctx := context.Background()

	// Try to update non-existent worker
	metrics := &WorkerMetrics{
		WorkerID:        uuid.New(),
		CPUUsagePercent: 50.0,
	}

	err := manager.UpdateWorkerHeartbeat(ctx, uuid.New(), metrics)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "worker not found")
}

// ========================================
// Worker Types Tests
// ========================================

func TestWorkerStatus_Constants(t *testing.T) {
	assert.Equal(t, WorkerStatus("active"), WorkerStatusActive)
	assert.Equal(t, WorkerStatus("inactive"), WorkerStatusInactive)
	assert.Equal(t, WorkerStatus("maintenance"), WorkerStatusMaintenance)
	assert.Equal(t, WorkerStatus("failed"), WorkerStatusFailed)
	assert.Equal(t, WorkerStatus("offline"), WorkerStatusOffline)
}

func TestWorkerHealth_Constants(t *testing.T) {
	assert.Equal(t, WorkerHealth("healthy"), WorkerHealthHealthy)
	assert.Equal(t, WorkerHealth("degraded"), WorkerHealthDegraded)
	assert.Equal(t, WorkerHealth("unhealthy"), WorkerHealthUnhealthy)
	assert.Equal(t, WorkerHealth("unknown"), WorkerHealthUnknown)
}

func TestResources_Structure(t *testing.T) {
	resources := Resources{
		CPUCount:    8,
		TotalMemory: 16 * 1024 * 1024 * 1024, // 16GB
		TotalDisk:   500 * 1024 * 1024 * 1024, // 500GB
		GPUCount:    1,
		GPUModel:    "NVIDIA RTX 3080",
		GPUMemory:   10 * 1024 * 1024 * 1024, // 10GB
	}

	assert.Equal(t, 8, resources.CPUCount)
	assert.Equal(t, int64(16*1024*1024*1024), resources.TotalMemory)
	assert.Equal(t, 1, resources.GPUCount)
	assert.Equal(t, "NVIDIA RTX 3080", resources.GPUModel)
}

func TestWorker_Structure(t *testing.T) {
	id := uuid.New()
	worker := Worker{
		ID:                 id,
		Hostname:           "worker-1",
		DisplayName:        "Worker 1",
		Status:             WorkerStatusActive,
		HealthStatus:       WorkerHealthHealthy,
		CPUUsagePercent:    45.5,
		MemoryUsagePercent: 60.0,
		DiskUsagePercent:   30.0,
		CurrentTasksCount:  5,
		MaxConcurrentTasks: 10,
		Resources: Resources{
			CPUCount:    4,
			TotalMemory: 8 * 1024 * 1024 * 1024,
		},
	}

	assert.Equal(t, id, worker.ID)
	assert.Equal(t, "worker-1", worker.Hostname)
	assert.Equal(t, WorkerStatusActive, worker.Status)
	assert.Equal(t, 45.5, worker.CPUUsagePercent)
	assert.Equal(t, 5, worker.CurrentTasksCount)
}

func TestWorkerMetrics_Structure(t *testing.T) {
	id := uuid.New()
	workerID := uuid.New()
	now := time.Now()

	metrics := WorkerMetrics{
		ID:                 id,
		WorkerID:           workerID,
		CPUUsagePercent:    75.0,
		MemoryUsagePercent: 80.0,
		DiskUsagePercent:   50.0,
		NetworkRxBytes:     1024 * 1024 * 100,
		NetworkTxBytes:     1024 * 1024 * 50,
		CurrentTasksCount:  3,
		TemperatureCelsius: 65.0,
		RecordedAt:         now,
	}

	assert.Equal(t, id, metrics.ID)
	assert.Equal(t, workerID, metrics.WorkerID)
	assert.Equal(t, 75.0, metrics.CPUUsagePercent)
	assert.Equal(t, int64(1024*1024*100), metrics.NetworkRxBytes)
}

// ========================================
// MockWorkerRepository Tests
// ========================================

func TestMockWorkerRepository(t *testing.T) {
	repo := NewMockWorkerRepository()
	ctx := context.Background()

	// Test Create
	worker := &Worker{
		ID:       uuid.New(),
		Hostname: "test-worker",
		Status:   WorkerStatusActive,
	}
	err := repo.CreateWorker(ctx, worker)
	require.NoError(t, err)

	// Test GetWorker
	retrieved, err := repo.GetWorker(ctx, worker.ID)
	require.NoError(t, err)
	assert.Equal(t, worker.Hostname, retrieved.Hostname)

	// Test GetWorkerByHostname
	byHostname, err := repo.GetWorkerByHostname(ctx, "test-worker")
	require.NoError(t, err)
	assert.Equal(t, worker.ID, byHostname.ID)

	// Test ListWorkers
	workers, err := repo.ListWorkers(ctx, WorkerStatusActive)
	require.NoError(t, err)
	assert.Len(t, workers, 1)

	// Test UpdateWorker
	worker.Status = WorkerStatusMaintenance
	err = repo.UpdateWorker(ctx, worker)
	require.NoError(t, err)

	// Test DeleteWorker
	err = repo.DeleteWorker(ctx, worker.ID)
	require.NoError(t, err)

	// Verify deleted
	_, err = repo.GetWorker(ctx, worker.ID)
	assert.Error(t, err)
}

// ========================================
// WorkerManager Additional Tests
// ========================================

func TestWorkerManager_GetAvailableWorkers(t *testing.T) {
	repo := NewMockWorkerRepository()
	manager := NewWorkerManager(repo, 30*time.Second)
	ctx := context.Background()

	// Create workers with different states
	healthyWorker := &Worker{
		ID:                 uuid.New(),
		Hostname:           "healthy-worker",
		Status:             WorkerStatusActive,
		HealthStatus:       WorkerHealthHealthy,
		CurrentTasksCount:  2,
		MaxConcurrentTasks: 10,
		LastHeartbeat:      time.Now(),
		Capabilities:       []string{"gpu", "docker"},
	}
	unhealthyWorker := &Worker{
		ID:                 uuid.New(),
		Hostname:           "unhealthy-worker",
		Status:             WorkerStatusActive,
		HealthStatus:       WorkerHealthUnhealthy,
		CurrentTasksCount:  0,
		MaxConcurrentTasks: 10,
		LastHeartbeat:      time.Now(),
	}
	fullWorker := &Worker{
		ID:                 uuid.New(),
		Hostname:           "full-worker",
		Status:             WorkerStatusActive,
		HealthStatus:       WorkerHealthHealthy,
		CurrentTasksCount:  10,
		MaxConcurrentTasks: 10,
		LastHeartbeat:      time.Now(),
	}

	repo.CreateWorker(ctx, healthyWorker)
	repo.CreateWorker(ctx, unhealthyWorker)
	repo.CreateWorker(ctx, fullWorker)

	// Get available workers with no capabilities required
	available, err := manager.GetAvailableWorkers(ctx, nil)
	require.NoError(t, err)
	assert.Len(t, available, 1)
	assert.Equal(t, "healthy-worker", available[0].Hostname)
}

func TestWorkerManager_GetAvailableWorkers_WithCapabilities(t *testing.T) {
	repo := NewMockWorkerRepository()
	manager := NewWorkerManager(repo, 30*time.Second)
	ctx := context.Background()

	// Create workers with different capabilities
	gpuWorker := &Worker{
		ID:                 uuid.New(),
		Hostname:           "gpu-worker",
		Status:             WorkerStatusActive,
		HealthStatus:       WorkerHealthHealthy,
		CurrentTasksCount:  0,
		MaxConcurrentTasks: 10,
		LastHeartbeat:      time.Now(),
		Capabilities:       []string{"gpu", "cuda"},
	}
	cpuWorker := &Worker{
		ID:                 uuid.New(),
		Hostname:           "cpu-worker",
		Status:             WorkerStatusActive,
		HealthStatus:       WorkerHealthHealthy,
		CurrentTasksCount:  0,
		MaxConcurrentTasks: 10,
		LastHeartbeat:      time.Now(),
		Capabilities:       []string{"cpu"},
	}

	repo.CreateWorker(ctx, gpuWorker)
	repo.CreateWorker(ctx, cpuWorker)

	// Get workers with GPU capability
	available, err := manager.GetAvailableWorkers(ctx, []string{"gpu"})
	require.NoError(t, err)
	assert.Len(t, available, 1)
	assert.Equal(t, "gpu-worker", available[0].Hostname)

	// Get workers with GPU and CUDA capabilities
	available, err = manager.GetAvailableWorkers(ctx, []string{"gpu", "cuda"})
	require.NoError(t, err)
	assert.Len(t, available, 1)

	// Get workers with nonexistent capability
	available, err = manager.GetAvailableWorkers(ctx, []string{"nonexistent"})
	require.NoError(t, err)
	assert.Len(t, available, 0)
}

func TestWorkerManager_AssignTask(t *testing.T) {
	repo := NewMockWorkerRepository()
	manager := NewWorkerManager(repo, 30*time.Second)
	ctx := context.Background()

	worker := &Worker{
		ID:                 uuid.New(),
		Hostname:           "test-worker",
		Status:             WorkerStatusActive,
		CurrentTasksCount:  0,
		MaxConcurrentTasks: 5,
	}
	repo.CreateWorker(ctx, worker)

	// Assign task
	err := manager.AssignTask(ctx, worker.ID)
	require.NoError(t, err)

	// Check task count increased
	updated, err := repo.GetWorker(ctx, worker.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, updated.CurrentTasksCount)
}

func TestWorkerManager_AssignTask_AtCapacity(t *testing.T) {
	repo := NewMockWorkerRepository()
	manager := NewWorkerManager(repo, 30*time.Second)
	ctx := context.Background()

	worker := &Worker{
		ID:                 uuid.New(),
		Hostname:           "full-worker",
		Status:             WorkerStatusActive,
		CurrentTasksCount:  5,
		MaxConcurrentTasks: 5,
	}
	repo.CreateWorker(ctx, worker)

	// Try to assign task to full worker
	err := manager.AssignTask(ctx, worker.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "maximum capacity")
}

func TestWorkerManager_AssignTask_NotFound(t *testing.T) {
	repo := NewMockWorkerRepository()
	manager := NewWorkerManager(repo, 30*time.Second)
	ctx := context.Background()

	err := manager.AssignTask(ctx, uuid.New())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "worker not found")
}

func TestWorkerManager_CompleteTask(t *testing.T) {
	repo := NewMockWorkerRepository()
	manager := NewWorkerManager(repo, 30*time.Second)
	ctx := context.Background()

	worker := &Worker{
		ID:                 uuid.New(),
		Hostname:           "test-worker",
		Status:             WorkerStatusActive,
		CurrentTasksCount:  3,
		MaxConcurrentTasks: 5,
	}
	repo.CreateWorker(ctx, worker)

	// Complete task
	err := manager.CompleteTask(ctx, worker.ID)
	require.NoError(t, err)

	// Check task count decreased
	updated, err := repo.GetWorker(ctx, worker.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, updated.CurrentTasksCount)
}

func TestWorkerManager_CompleteTask_AtZero(t *testing.T) {
	repo := NewMockWorkerRepository()
	manager := NewWorkerManager(repo, 30*time.Second)
	ctx := context.Background()

	worker := &Worker{
		ID:                 uuid.New(),
		Hostname:           "idle-worker",
		Status:             WorkerStatusActive,
		CurrentTasksCount:  0,
		MaxConcurrentTasks: 5,
	}
	repo.CreateWorker(ctx, worker)

	// Complete task on idle worker should not go negative
	err := manager.CompleteTask(ctx, worker.ID)
	require.NoError(t, err)

	// Task count should remain 0
	updated, err := repo.GetWorker(ctx, worker.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, updated.CurrentTasksCount)
}

func TestWorkerManager_CompleteTask_NotFound(t *testing.T) {
	repo := NewMockWorkerRepository()
	manager := NewWorkerManager(repo, 30*time.Second)
	ctx := context.Background()

	err := manager.CompleteTask(ctx, uuid.New())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "worker not found")
}

func TestWorkerManager_HealthCheck(t *testing.T) {
	repo := NewMockWorkerRepository()
	manager := NewWorkerManager(repo, 30*time.Second)
	ctx := context.Background()

	// Create a worker with old heartbeat
	staleWorker := &Worker{
		ID:            uuid.New(),
		Hostname:      "stale-worker",
		Status:        WorkerStatusActive,
		HealthStatus:  WorkerHealthHealthy,
		LastHeartbeat: time.Now().Add(-1 * time.Hour),
	}
	repo.CreateWorker(ctx, staleWorker)

	// Create a healthy worker
	healthyWorker := &Worker{
		ID:            uuid.New(),
		Hostname:      "healthy-worker",
		Status:        WorkerStatusActive,
		HealthStatus:  WorkerHealthHealthy,
		LastHeartbeat: time.Now(),
	}
	repo.CreateWorker(ctx, healthyWorker)

	// Run health check
	err := manager.HealthCheck(ctx)
	require.NoError(t, err)

	// Stale worker should be marked unhealthy
	updatedStale, err := repo.GetWorker(ctx, staleWorker.ID)
	require.NoError(t, err)
	assert.Equal(t, WorkerHealthUnhealthy, updatedStale.HealthStatus)
	assert.Equal(t, WorkerStatusOffline, updatedStale.Status)

	// Healthy worker should remain healthy
	updatedHealthy, err := repo.GetWorker(ctx, healthyWorker.ID)
	require.NoError(t, err)
	assert.Equal(t, WorkerHealthHealthy, updatedHealthy.HealthStatus)
	assert.Equal(t, WorkerStatusActive, updatedHealthy.Status)
}

func TestWorkerManager_GetWorkerStats(t *testing.T) {
	repo := NewMockWorkerRepository()
	manager := NewWorkerManager(repo, 30*time.Second)
	ctx := context.Background()

	// Create workers with various states
	worker1 := &Worker{
		ID:                 uuid.New(),
		Hostname:           "worker-1",
		Status:             WorkerStatusActive,
		HealthStatus:       WorkerHealthHealthy,
		CurrentTasksCount:  3,
		MaxConcurrentTasks: 10,
		CPUUsagePercent:    40.0,
		MemoryUsagePercent: 50.0,
	}
	worker2 := &Worker{
		ID:                 uuid.New(),
		Hostname:           "worker-2",
		Status:             WorkerStatusActive,
		HealthStatus:       WorkerHealthDegraded,
		CurrentTasksCount:  7,
		MaxConcurrentTasks: 10,
		CPUUsagePercent:    80.0,
		MemoryUsagePercent: 70.0,
	}
	worker3 := &Worker{
		ID:                 uuid.New(),
		Hostname:           "worker-3",
		Status:             WorkerStatusOffline,
		HealthStatus:       WorkerHealthUnhealthy,
		CurrentTasksCount:  0,
		MaxConcurrentTasks: 10,
		CPUUsagePercent:    0.0,
		MemoryUsagePercent: 0.0,
	}

	repo.CreateWorker(ctx, worker1)
	repo.CreateWorker(ctx, worker2)
	repo.CreateWorker(ctx, worker3)

	stats, err := manager.GetWorkerStats(ctx)
	require.NoError(t, err)

	assert.Equal(t, 3, stats.TotalWorkers)
	assert.Equal(t, 2, stats.ActiveWorkers)
	assert.Equal(t, 1, stats.HealthyWorkers)
	assert.Equal(t, 10, stats.TotalTasks)                // 3 + 7 + 0
	assert.Equal(t, 20, stats.AvailableTasks)            // (10-3) + (10-7) + (10-0)
	assert.Equal(t, 40.0, stats.AverageCPUUsage)         // (40+80+0)/3
	assert.Equal(t, 40.0, stats.AverageMemoryUsage)      // (50+70+0)/3
}

func TestWorkerManager_GetWorkerStats_Empty(t *testing.T) {
	repo := NewMockWorkerRepository()
	manager := NewWorkerManager(repo, 30*time.Second)
	ctx := context.Background()

	stats, err := manager.GetWorkerStats(ctx)
	require.NoError(t, err)

	assert.Equal(t, 0, stats.TotalWorkers)
	assert.Equal(t, 0, stats.ActiveWorkers)
	assert.Equal(t, 0.0, stats.AverageCPUUsage)
	assert.Equal(t, 0.0, stats.AverageMemoryUsage)
}

func TestParseSSHConfig(t *testing.T) {
	configJSON := `{
		"host": "192.168.1.100",
		"port": 2222,
		"username": "admin",
		"private_key": "/path/to/key",
		"password": "secret123"
	}`

	config, err := ParseSSHConfig(configJSON)
	require.NoError(t, err)

	assert.Equal(t, "192.168.1.100", config.Host)
	assert.Equal(t, 2222, config.Port)
	assert.Equal(t, "admin", config.Username)
	assert.Equal(t, "/path/to/key", config.PrivateKey)
	assert.Equal(t, "secret123", config.Password)
}

func TestParseSSHConfig_DefaultPort(t *testing.T) {
	configJSON := `{
		"host": "192.168.1.100",
		"username": "admin"
	}`

	config, err := ParseSSHConfig(configJSON)
	require.NoError(t, err)

	assert.Equal(t, 22, config.Port)
}

func TestParseSSHConfig_InvalidJSON(t *testing.T) {
	configJSON := `invalid json`

	_, err := ParseSSHConfig(configJSON)
	assert.Error(t, err)
}

func TestCalculateHealthStatus(t *testing.T) {
	repo := NewMockWorkerRepository()
	manager := NewWorkerManager(repo, 30*time.Second)

	tests := []struct {
		name           string
		cpuUsage       float64
		memoryUsage    float64
		diskUsage      float64
		expectedHealth WorkerHealth
	}{
		{"healthy", 50.0, 50.0, 50.0, WorkerHealthHealthy},
		{"degraded_cpu", 75.0, 50.0, 50.0, WorkerHealthDegraded},
		{"degraded_memory", 50.0, 75.0, 50.0, WorkerHealthDegraded},
		{"degraded_disk", 50.0, 50.0, 75.0, WorkerHealthDegraded},
		{"unhealthy_cpu", 95.0, 50.0, 50.0, WorkerHealthUnhealthy},
		{"unhealthy_memory", 50.0, 95.0, 50.0, WorkerHealthUnhealthy},
		{"unhealthy_disk", 50.0, 50.0, 95.0, WorkerHealthUnhealthy},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			worker := &Worker{}
			metrics := &WorkerMetrics{
				CPUUsagePercent:    tt.cpuUsage,
				MemoryUsagePercent: tt.memoryUsage,
				DiskUsagePercent:   tt.diskUsage,
			}

			health := manager.calculateHealthStatus(worker, metrics)
			assert.Equal(t, tt.expectedHealth, health)
		})
	}
}
