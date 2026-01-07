package worker

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Common errors for worker operations
var (
	// ErrWorkerNotFound is returned when a worker cannot be found
	ErrWorkerNotFound = errors.New("worker not found")
)

// InMemoryWorkerRepository provides an in-memory implementation of WorkerRepository
// suitable for development, testing, and standalone UI applications
type InMemoryWorkerRepository struct {
	mu      sync.RWMutex
	workers map[uuid.UUID]*Worker
	metrics map[uuid.UUID][]*WorkerMetrics
}

// NewInMemoryWorkerRepository creates a new in-memory worker repository
func NewInMemoryWorkerRepository() *InMemoryWorkerRepository {
	return &InMemoryWorkerRepository{
		workers: make(map[uuid.UUID]*Worker),
		metrics: make(map[uuid.UUID][]*WorkerMetrics),
	}
}

// CreateWorker creates a new worker
func (r *InMemoryWorkerRepository) CreateWorker(ctx context.Context, worker *Worker) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if worker.ID == uuid.Nil {
		worker.ID = uuid.New()
	}

	if worker.CreatedAt.IsZero() {
		worker.CreatedAt = time.Now()
	}
	worker.UpdatedAt = time.Now()

	r.workers[worker.ID] = worker
	return nil
}

// GetWorker retrieves a worker by ID
func (r *InMemoryWorkerRepository) GetWorker(ctx context.Context, id uuid.UUID) (*Worker, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	worker, ok := r.workers[id]
	if !ok {
		return nil, ErrWorkerNotFound
	}

	return worker, nil
}

// GetWorkerByHostname retrieves a worker by hostname
func (r *InMemoryWorkerRepository) GetWorkerByHostname(ctx context.Context, hostname string) (*Worker, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, worker := range r.workers {
		if worker.Hostname == hostname {
			return worker, nil
		}
	}

	return nil, ErrWorkerNotFound
}

// ListWorkers lists workers, optionally filtered by status
func (r *InMemoryWorkerRepository) ListWorkers(ctx context.Context, status WorkerStatus) ([]*Worker, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*Worker
	for _, worker := range r.workers {
		if status == "" || worker.Status == status {
			result = append(result, worker)
		}
	}

	return result, nil
}

// UpdateWorker updates an existing worker
func (r *InMemoryWorkerRepository) UpdateWorker(ctx context.Context, worker *Worker) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.workers[worker.ID]; !ok {
		return ErrWorkerNotFound
	}

	worker.UpdatedAt = time.Now()
	r.workers[worker.ID] = worker
	return nil
}

// DeleteWorker deletes a worker by ID
func (r *InMemoryWorkerRepository) DeleteWorker(ctx context.Context, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.workers[id]; !ok {
		return ErrWorkerNotFound
	}

	delete(r.workers, id)
	delete(r.metrics, id)
	return nil
}

// RecordMetrics records worker metrics
func (r *InMemoryWorkerRepository) RecordMetrics(ctx context.Context, metrics *WorkerMetrics) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if metrics.ID == uuid.Nil {
		metrics.ID = uuid.New()
	}

	if metrics.RecordedAt.IsZero() {
		metrics.RecordedAt = time.Now()
	}

	r.metrics[metrics.WorkerID] = append(r.metrics[metrics.WorkerID], metrics)

	// Keep only last 1000 metrics per worker to prevent memory growth
	if len(r.metrics[metrics.WorkerID]) > 1000 {
		r.metrics[metrics.WorkerID] = r.metrics[metrics.WorkerID][len(r.metrics[metrics.WorkerID])-1000:]
	}

	return nil
}

// GetWorkerMetrics retrieves metrics for a worker since a given time
func (r *InMemoryWorkerRepository) GetWorkerMetrics(ctx context.Context, workerID uuid.UUID, since time.Time) ([]*WorkerMetrics, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	allMetrics, ok := r.metrics[workerID]
	if !ok {
		return []*WorkerMetrics{}, nil
	}

	var result []*WorkerMetrics
	for _, m := range allMetrics {
		if m.RecordedAt.After(since) || m.RecordedAt.Equal(since) {
			result = append(result, m)
		}
	}

	return result, nil
}

// Count returns the number of workers
func (r *InMemoryWorkerRepository) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.workers)
}

// Clear removes all workers and metrics
func (r *InMemoryWorkerRepository) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.workers = make(map[uuid.UUID]*Worker)
	r.metrics = make(map[uuid.UUID][]*WorkerMetrics)
}

// Ensure InMemoryWorkerRepository implements WorkerRepository
var _ WorkerRepository = (*InMemoryWorkerRepository)(nil)
