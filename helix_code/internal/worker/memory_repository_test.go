package worker

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========================================
// InMemoryWorkerRepository Tests
// ========================================

func TestNewInMemoryWorkerRepository(t *testing.T) {
	repo := NewInMemoryWorkerRepository()

	assert.NotNil(t, repo)
	assert.NotNil(t, repo.workers)
	assert.NotNil(t, repo.metrics)
	assert.Equal(t, 0, repo.Count())
}

func TestInMemoryWorkerRepository_CreateWorker(t *testing.T) {
	repo := NewInMemoryWorkerRepository()
	ctx := context.Background()

	worker := &Worker{
		Hostname:    "test-worker-1",
		DisplayName: "Test Worker 1",
		Status:      WorkerStatusActive,
	}

	err := repo.CreateWorker(ctx, worker)
	require.NoError(t, err)

	// Worker should be assigned an ID
	assert.NotEqual(t, uuid.Nil, worker.ID)
	assert.False(t, worker.CreatedAt.IsZero())
	assert.False(t, worker.UpdatedAt.IsZero())

	// Count should be 1
	assert.Equal(t, 1, repo.Count())
}

func TestInMemoryWorkerRepository_CreateWorker_WithExistingID(t *testing.T) {
	repo := NewInMemoryWorkerRepository()
	ctx := context.Background()

	existingID := uuid.New()
	worker := &Worker{
		ID:          existingID,
		Hostname:    "test-worker-1",
		DisplayName: "Test Worker 1",
	}

	err := repo.CreateWorker(ctx, worker)
	require.NoError(t, err)

	// ID should not be overwritten
	assert.Equal(t, existingID, worker.ID)
}

func TestInMemoryWorkerRepository_GetWorker(t *testing.T) {
	repo := NewInMemoryWorkerRepository()
	ctx := context.Background()

	// Create a worker
	worker := &Worker{
		Hostname:    "test-worker-1",
		DisplayName: "Test Worker 1",
		Status:      WorkerStatusActive,
	}
	err := repo.CreateWorker(ctx, worker)
	require.NoError(t, err)

	// Retrieve the worker
	retrieved, err := repo.GetWorker(ctx, worker.ID)
	require.NoError(t, err)
	assert.Equal(t, worker.Hostname, retrieved.Hostname)
	assert.Equal(t, worker.DisplayName, retrieved.DisplayName)
}

func TestInMemoryWorkerRepository_GetWorker_NotFound(t *testing.T) {
	repo := NewInMemoryWorkerRepository()
	ctx := context.Background()

	_, err := repo.GetWorker(ctx, uuid.New())
	assert.Error(t, err)
	assert.Equal(t, ErrWorkerNotFound, err)
}

func TestInMemoryWorkerRepository_GetWorkerByHostname(t *testing.T) {
	repo := NewInMemoryWorkerRepository()
	ctx := context.Background()

	// Create workers
	worker1 := &Worker{
		Hostname: "worker-1",
		Status:   WorkerStatusActive,
	}
	worker2 := &Worker{
		Hostname: "worker-2",
		Status:   WorkerStatusActive,
	}
	err := repo.CreateWorker(ctx, worker1)
	require.NoError(t, err)
	err = repo.CreateWorker(ctx, worker2)
	require.NoError(t, err)

	// Retrieve by hostname
	retrieved, err := repo.GetWorkerByHostname(ctx, "worker-2")
	require.NoError(t, err)
	assert.Equal(t, worker2.ID, retrieved.ID)
	assert.Equal(t, "worker-2", retrieved.Hostname)
}

func TestInMemoryWorkerRepository_GetWorkerByHostname_NotFound(t *testing.T) {
	repo := NewInMemoryWorkerRepository()
	ctx := context.Background()

	_, err := repo.GetWorkerByHostname(ctx, "nonexistent")
	assert.Error(t, err)
	assert.Equal(t, ErrWorkerNotFound, err)
}

func TestInMemoryWorkerRepository_ListWorkers(t *testing.T) {
	repo := NewInMemoryWorkerRepository()
	ctx := context.Background()

	// Create workers with different statuses
	worker1 := &Worker{
		Hostname: "worker-1",
		Status:   WorkerStatusActive,
	}
	worker2 := &Worker{
		Hostname: "worker-2",
		Status:   WorkerStatusInactive,
	}
	worker3 := &Worker{
		Hostname: "worker-3",
		Status:   WorkerStatusActive,
	}
	repo.CreateWorker(ctx, worker1)
	repo.CreateWorker(ctx, worker2)
	repo.CreateWorker(ctx, worker3)

	// List all workers
	all, err := repo.ListWorkers(ctx, "")
	require.NoError(t, err)
	assert.Len(t, all, 3)

	// List only active workers
	active, err := repo.ListWorkers(ctx, WorkerStatusActive)
	require.NoError(t, err)
	assert.Len(t, active, 2)

	// List only inactive workers
	inactive, err := repo.ListWorkers(ctx, WorkerStatusInactive)
	require.NoError(t, err)
	assert.Len(t, inactive, 1)
}

func TestInMemoryWorkerRepository_UpdateWorker(t *testing.T) {
	repo := NewInMemoryWorkerRepository()
	ctx := context.Background()

	// Create a worker
	worker := &Worker{
		Hostname: "test-worker",
		Status:   WorkerStatusActive,
	}
	err := repo.CreateWorker(ctx, worker)
	require.NoError(t, err)

	originalUpdatedAt := worker.UpdatedAt

	// Wait a bit to ensure UpdatedAt changes
	time.Sleep(10 * time.Millisecond)

	// Update the worker
	worker.Status = WorkerStatusMaintenance
	worker.DisplayName = "Updated Worker"
	err = repo.UpdateWorker(ctx, worker)
	require.NoError(t, err)

	// Verify update
	retrieved, err := repo.GetWorker(ctx, worker.ID)
	require.NoError(t, err)
	assert.Equal(t, WorkerStatusMaintenance, retrieved.Status)
	assert.Equal(t, "Updated Worker", retrieved.DisplayName)
	assert.True(t, retrieved.UpdatedAt.After(originalUpdatedAt))
}

func TestInMemoryWorkerRepository_UpdateWorker_NotFound(t *testing.T) {
	repo := NewInMemoryWorkerRepository()
	ctx := context.Background()

	worker := &Worker{
		ID:       uuid.New(),
		Hostname: "nonexistent",
	}

	err := repo.UpdateWorker(ctx, worker)
	assert.Error(t, err)
	assert.Equal(t, ErrWorkerNotFound, err)
}

func TestInMemoryWorkerRepository_DeleteWorker(t *testing.T) {
	repo := NewInMemoryWorkerRepository()
	ctx := context.Background()

	// Create a worker
	worker := &Worker{
		Hostname: "test-worker",
	}
	err := repo.CreateWorker(ctx, worker)
	require.NoError(t, err)

	assert.Equal(t, 1, repo.Count())

	// Delete the worker
	err = repo.DeleteWorker(ctx, worker.ID)
	require.NoError(t, err)

	assert.Equal(t, 0, repo.Count())

	// Verify it's gone
	_, err = repo.GetWorker(ctx, worker.ID)
	assert.Equal(t, ErrWorkerNotFound, err)
}

func TestInMemoryWorkerRepository_DeleteWorker_NotFound(t *testing.T) {
	repo := NewInMemoryWorkerRepository()
	ctx := context.Background()

	err := repo.DeleteWorker(ctx, uuid.New())
	assert.Error(t, err)
	assert.Equal(t, ErrWorkerNotFound, err)
}

func TestInMemoryWorkerRepository_RecordMetrics(t *testing.T) {
	repo := NewInMemoryWorkerRepository()
	ctx := context.Background()

	workerID := uuid.New()
	metrics := &WorkerMetrics{
		WorkerID:           workerID,
		CPUUsagePercent:    50.0,
		MemoryUsagePercent: 60.0,
		DiskUsagePercent:   30.0,
	}

	err := repo.RecordMetrics(ctx, metrics)
	require.NoError(t, err)

	// Metrics should be assigned an ID and timestamp
	assert.NotEqual(t, uuid.Nil, metrics.ID)
	assert.False(t, metrics.RecordedAt.IsZero())

	// Retrieve metrics
	retrieved, err := repo.GetWorkerMetrics(ctx, workerID, time.Time{})
	require.NoError(t, err)
	assert.Len(t, retrieved, 1)
	assert.Equal(t, 50.0, retrieved[0].CPUUsagePercent)
}

func TestInMemoryWorkerRepository_RecordMetrics_LimitSize(t *testing.T) {
	repo := NewInMemoryWorkerRepository()
	ctx := context.Background()

	workerID := uuid.New()

	// Record more than 1000 metrics
	for i := 0; i < 1050; i++ {
		metrics := &WorkerMetrics{
			WorkerID:        workerID,
			CPUUsagePercent: float64(i),
			RecordedAt:      time.Now(),
		}
		err := repo.RecordMetrics(ctx, metrics)
		require.NoError(t, err)
	}

	// Should only keep the last 1000
	retrieved, err := repo.GetWorkerMetrics(ctx, workerID, time.Time{})
	require.NoError(t, err)
	assert.Len(t, retrieved, 1000)

	// First one should be 50 (since we kept last 1000 of 0-1049)
	assert.Equal(t, 50.0, retrieved[0].CPUUsagePercent)
}

func TestInMemoryWorkerRepository_GetWorkerMetrics_Empty(t *testing.T) {
	repo := NewInMemoryWorkerRepository()
	ctx := context.Background()

	metrics, err := repo.GetWorkerMetrics(ctx, uuid.New(), time.Time{})
	require.NoError(t, err)
	assert.Empty(t, metrics)
}

func TestInMemoryWorkerRepository_GetWorkerMetrics_FilterBySince(t *testing.T) {
	repo := NewInMemoryWorkerRepository()
	ctx := context.Background()

	workerID := uuid.New()
	now := time.Now()

	// Record old metrics
	oldMetrics := &WorkerMetrics{
		WorkerID:        workerID,
		CPUUsagePercent: 10.0,
		RecordedAt:      now.Add(-1 * time.Hour),
	}
	repo.RecordMetrics(ctx, oldMetrics)

	// Record recent metrics
	recentMetrics := &WorkerMetrics{
		WorkerID:        workerID,
		CPUUsagePercent: 90.0,
		RecordedAt:      now,
	}
	repo.RecordMetrics(ctx, recentMetrics)

	// Get metrics since 30 minutes ago
	retrieved, err := repo.GetWorkerMetrics(ctx, workerID, now.Add(-30*time.Minute))
	require.NoError(t, err)
	assert.Len(t, retrieved, 1)
	assert.Equal(t, 90.0, retrieved[0].CPUUsagePercent)
}

func TestInMemoryWorkerRepository_Clear(t *testing.T) {
	repo := NewInMemoryWorkerRepository()
	ctx := context.Background()

	// Add workers and metrics
	worker := &Worker{Hostname: "test"}
	repo.CreateWorker(ctx, worker)

	metrics := &WorkerMetrics{WorkerID: worker.ID}
	repo.RecordMetrics(ctx, metrics)

	assert.Equal(t, 1, repo.Count())

	// Clear
	repo.Clear()

	assert.Equal(t, 0, repo.Count())

	// Metrics should also be cleared
	retrieved, err := repo.GetWorkerMetrics(ctx, worker.ID, time.Time{})
	require.NoError(t, err)
	assert.Empty(t, retrieved)
}

func TestInMemoryWorkerRepository_ConcurrentAccess(t *testing.T) {
	repo := NewInMemoryWorkerRepository()
	ctx := context.Background()

	done := make(chan bool, 20)

	// Concurrent writers
	for i := 0; i < 10; i++ {
		go func(idx int) {
			worker := &Worker{
				Hostname: "worker-" + string(rune('a'+idx)),
				Status:   WorkerStatusActive,
			}
			repo.CreateWorker(ctx, worker)
			done <- true
		}(i)
	}

	// Concurrent readers
	for i := 0; i < 10; i++ {
		go func() {
			_, _ = repo.ListWorkers(ctx, "")
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 20; i++ {
		<-done
	}

	// Should have 10 workers
	assert.Equal(t, 10, repo.Count())
}
