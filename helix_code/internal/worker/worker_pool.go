package worker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"dev.helix.code/internal/config"
)

// PoolWorkerStatus represents the status of a pool worker
type PoolWorkerStatus string

const (
	// StatusAvailable indicates the worker is available for tasks
	StatusAvailable PoolWorkerStatus = "available"
	// StatusBusy indicates the worker is currently executing a task
	StatusBusy PoolWorkerStatus = "busy"
	// StatusOffline indicates the worker is offline
	StatusOffline PoolWorkerStatus = "offline"
	// StatusError indicates the worker encountered an error
	StatusError PoolWorkerStatus = "error"
)

// WorkerCapabilities represents the capabilities of a worker
type WorkerCapabilities struct {
	CPUCores    int      `json:"cpu_cores"`
	MemoryGB    int      `json:"memory_gb"`
	DiskGB      int      `json:"disk_gb"`
	GPUs        int      `json:"gpus"`
	OS          string   `json:"os"`
	Arch        string   `json:"arch"`
	Tags        []string `json:"tags"`
	Specialized []string `json:"specialized"` // e.g., "gpu", "high-memory", "fast-storage"
}

// PoolWorker represents a distributed worker node in the pool
type PoolWorker struct {
	ID             string             `json:"id"`
	Name           string             `json:"name"`
	Address        string             `json:"address"`
	Status         PoolWorkerStatus   `json:"status"`
	Capabilities   WorkerCapabilities `json:"capabilities"`
	LastSeen       time.Time          `json:"last_seen"`
	TasksProcessed int                `json:"tasks_processed"`
	TasksSucceeded int                `json:"tasks_succeeded"`
	TasksFailed    int                `json:"tasks_failed"`
	LoadAverage    float64            `json:"load_average"`
	mu             sync.RWMutex
}

// NewPoolWorker creates a new pool worker
func NewPoolWorker(id, name, address string, capabilities WorkerCapabilities) *PoolWorker {
	return &PoolWorker{
		ID:           id,
		Name:         name,
		Address:      address,
		Status:       StatusAvailable,
		Capabilities: capabilities,
		LastSeen:     time.Now(),
	}
}

// UpdateStatus updates the worker status
func (w *PoolWorker) UpdateStatus(status PoolWorkerStatus) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.Status = status
	w.LastSeen = time.Now()
}

// IsAvailable checks if the worker is available
func (w *PoolWorker) IsAvailable() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.Status == StatusAvailable
}

// GetStatus returns the worker's current status under the worker's own lock.
// PoolWorker.Status is guarded by w.mu (UpdateStatus writes it under w.mu.Lock);
// any reader outside the PoolWorker MUST go through this accessor rather than
// touching the field directly, otherwise it races with UpdateStatus. This race
// was surfaced by the §11.4.85 concurrent-assign stress test (GetPoolStats read
// worker.Status directly while ReleaseWorker -> UpdateStatus wrote it).
func (w *PoolWorker) GetStatus() PoolWorkerStatus {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.Status
}

// CanHandleTask checks if the worker can handle a specific task
func (w *PoolWorker) CanHandleTask(taskType string, requirements map[string]interface{}) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()

	// Check basic availability
	if w.Status != StatusAvailable {
		return false
	}

	// Check CPU requirements
	if cpuReq, ok := requirements["cpu_cores"].(int); ok {
		if w.Capabilities.CPUCores < cpuReq {
			return false
		}
	}

	// Check memory requirements
	if memReq, ok := requirements["memory_gb"].(int); ok {
		if w.Capabilities.MemoryGB < memReq {
			return false
		}
	}

	// Check GPU requirements
	if gpuReq, ok := requirements["gpus"].(int); ok {
		if w.Capabilities.GPUs < gpuReq {
			return false
		}
	}

	// Check specialized capabilities
	if specReq, ok := requirements["specialized"].([]string); ok {
		for _, req := range specReq {
			found := false
			for _, cap := range w.Capabilities.Specialized {
				if cap == req {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
	}

	// Check tags
	if tagReq, ok := requirements["tags"].([]string); ok {
		for _, req := range tagReq {
			found := false
			for _, tag := range w.Capabilities.Tags {
				if tag == req {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
	}

	return true
}

// UpdateStats updates worker statistics
func (w *PoolWorker) UpdateStats(processed, succeeded, failed int, loadAvg float64) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.TasksProcessed += processed
	w.TasksSucceeded += succeeded
	w.TasksFailed += failed
	w.LoadAverage = loadAvg
	w.LastSeen = time.Now()
}

// GetHealth returns worker health information
func (w *PoolWorker) GetHealth() map[string]interface{} {
	w.mu.RLock()
	defer w.mu.RUnlock()

	totalTasks := w.TasksProcessed
	successRate := float64(0)
	if totalTasks > 0 {
		successRate = float64(w.TasksSucceeded) / float64(totalTasks) * 100
	}

	return map[string]interface{}{
		"id":              w.ID,
		"name":            w.Name,
		"status":          string(w.Status),
		"last_seen":       w.LastSeen.Format(time.RFC3339),
		"tasks_processed": w.TasksProcessed,
		"tasks_succeeded": w.TasksSucceeded,
		"tasks_failed":    w.TasksFailed,
		"success_rate":    successRate,
		"load_average":    w.LoadAverage,
		"capabilities":    w.Capabilities,
	}
}

// WorkerPool manages a pool of workers
type WorkerPool struct {
	workers   map[string]*PoolWorker
	scheduler Scheduler
	config    *config.WorkersConfig
	stopChan  chan struct{}
	wg        sync.WaitGroup
	mu        sync.RWMutex
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(config *config.WorkersConfig) *WorkerPool {
	return &WorkerPool{
		workers:   make(map[string]*PoolWorker),
		scheduler: NewDefaultScheduler(),
		config:    config,
		stopChan:  make(chan struct{}),
	}
}

// RegisterWorker registers a worker with the pool
func (wp *WorkerPool) RegisterWorker(worker *PoolWorker) {
	wp.mu.Lock()
	defer wp.mu.Unlock()
	wp.workers[worker.ID] = worker
}

// UnregisterWorker removes a worker from the pool
func (wp *WorkerPool) UnregisterWorker(workerID string) {
	wp.mu.Lock()
	defer wp.mu.Unlock()
	delete(wp.workers, workerID)
}

// GetWorker gets a worker by ID
func (wp *WorkerPool) GetWorker(workerID string) (*PoolWorker, bool) {
	wp.mu.RLock()
	defer wp.mu.RUnlock()
	worker, exists := wp.workers[workerID]
	return worker, exists
}

// GetAvailableWorkers returns all available workers
func (wp *WorkerPool) GetAvailableWorkers() []*PoolWorker {
	wp.mu.RLock()
	defer wp.mu.RUnlock()

	available := make([]*PoolWorker, 0)
	for _, worker := range wp.workers {
		if worker.IsAvailable() {
			available = append(available, worker)
		}
	}
	return available
}

// AssignTask assigns a task to an appropriate worker.
//
// NOTE: GetAvailableWorkers acquires wp.mu (RLock) itself. This method MUST NOT
// wrap that call in its own RLock — sync.RWMutex is not reentrant, so a same-
// goroutine double-RLock deadlocks the instant a writer (RegisterWorker /
// UnregisterWorker via wp.mu.Lock) is pending between the two acquisitions
// (Go's RWMutex blocks new readers while a writer waits). That deadlock was
// surfaced by the §11.4.85 register/unregister chaos test; the fix is to let
// GetAvailableWorkers do its own single locking.
func (wp *WorkerPool) AssignTask(ctx context.Context, taskType string, requirements map[string]interface{}) (*PoolWorker, error) {
	availableWorkers := wp.GetAvailableWorkers()

	if len(availableWorkers) == 0 {
		return nil, fmt.Errorf("no available workers")
	}

	// Use scheduler to select worker
	selectedWorker := wp.scheduler.SelectWorker(availableWorkers, taskType, requirements)
	if selectedWorker == nil {
		return nil, fmt.Errorf("no suitable worker found for task type: %s", taskType)
	}

	// Mark worker as busy
	selectedWorker.UpdateStatus(StatusBusy)

	return selectedWorker, nil
}

// ReleaseWorker releases a worker back to available status
func (wp *WorkerPool) ReleaseWorker(workerID string) {
	if worker, exists := wp.GetWorker(workerID); exists {
		worker.UpdateStatus(StatusAvailable)
	}
}

// GetPoolStats returns pool statistics
func (wp *WorkerPool) GetPoolStats() map[string]interface{} {
	wp.mu.RLock()
	defer wp.mu.RUnlock()

	totalWorkers := len(wp.workers)
	availableWorkers := 0
	busyWorkers := 0
	offlineWorkers := 0
	errorWorkers := 0

	for _, worker := range wp.workers {
		// Read each worker's status through its own lock (see GetStatus) — the
		// pool-level RLock guards the map, NOT each worker's mutable fields.
		switch worker.GetStatus() {
		case StatusAvailable:
			availableWorkers++
		case StatusBusy:
			busyWorkers++
		case StatusOffline:
			offlineWorkers++
		case StatusError:
			errorWorkers++
		}
	}

	return map[string]interface{}{
		"total_workers":     totalWorkers,
		"available_workers": availableWorkers,
		"busy_workers":      busyWorkers,
		"offline_workers":   offlineWorkers,
		"error_workers":     errorWorkers,
		"utilization_rate":  float64(busyWorkers) / float64(totalWorkers) * 100,
	}
}

// HealthCheck performs a health check on the worker pool
func (wp *WorkerPool) HealthCheck() error {
	wp.mu.RLock()
	defer wp.mu.RUnlock()

	if len(wp.workers) == 0 {
		return fmt.Errorf("no workers registered")
	}

	availableCount := 0
	for _, worker := range wp.workers {
		if worker.IsAvailable() {
			availableCount++
		}
	}

	if availableCount == 0 {
		return fmt.Errorf("no available workers")
	}

	return nil
}

// Start starts the worker pool
func (wp *WorkerPool) Start(ctx context.Context) error {
	wp.wg.Add(1)
	go wp.healthCheckLoop(ctx)
	return nil
}

// Stop stops the worker pool
func (wp *WorkerPool) Stop() {
	close(wp.stopChan)
	wp.wg.Wait()
}

// healthCheckLoop runs periodic health checks
func (wp *WorkerPool) healthCheckLoop(ctx context.Context) {
	defer wp.wg.Done()

	ticker := time.NewTicker(30 * time.Second) // Health check every 30 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-wp.stopChan:
			return
		case <-ticker.C:
			wp.performHealthChecks()
		}
	}
}

// performHealthChecks performs health checks on all workers
func (wp *WorkerPool) performHealthChecks() {
	wp.mu.RLock()
	workers := make([]*PoolWorker, 0, len(wp.workers))
	for _, worker := range wp.workers {
		workers = append(workers, worker)
	}
	wp.mu.RUnlock()

	for _, worker := range workers {
		// Check if worker has been seen recently
		if time.Since(worker.LastSeen) > time.Duration(wp.config.HealthTTL)*time.Second {
			worker.UpdateStatus(StatusOffline)
		}
	}
}

// Scheduler defines the interface for worker selection
type Scheduler interface {
	SelectWorker(workers []*PoolWorker, taskType string, requirements map[string]interface{}) *PoolWorker
}

// DefaultScheduler implements a simple round-robin scheduler
type DefaultScheduler struct {
	lastSelected int
	mu           sync.Mutex
}

// NewDefaultScheduler creates a new default scheduler
func NewDefaultScheduler() *DefaultScheduler {
	return &DefaultScheduler{}
}

// SelectWorker selects a worker using round-robin scheduling
func (ds *DefaultScheduler) SelectWorker(workers []*PoolWorker, taskType string, requirements map[string]interface{}) *PoolWorker {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	if len(workers) == 0 {
		return nil
	}

	// Find suitable workers
	suitableWorkers := make([]*PoolWorker, 0)
	for _, worker := range workers {
		if worker.CanHandleTask(taskType, requirements) {
			suitableWorkers = append(suitableWorkers, worker)
		}
	}

	if len(suitableWorkers) == 0 {
		return nil
	}

	// Round-robin selection
	selected := suitableWorkers[ds.lastSelected%len(suitableWorkers)]
	ds.lastSelected++

	return selected
}

// PerformanceScheduler selects workers based on performance metrics
type PerformanceScheduler struct{}

// NewPerformanceScheduler creates a new performance-based scheduler
func NewPerformanceScheduler() *PerformanceScheduler {
	return &PerformanceScheduler{}
}

// SelectWorker selects the worker with the best performance metrics
func (ps *PerformanceScheduler) SelectWorker(workers []*PoolWorker, taskType string, requirements map[string]interface{}) *PoolWorker {
	var bestWorker *PoolWorker
	bestScore := -1.0

	for _, worker := range workers {
		if !worker.CanHandleTask(taskType, requirements) {
			continue
		}

		// Calculate performance score based on success rate and load
		totalTasks := worker.TasksProcessed
		if totalTasks == 0 {
			// New worker gets high score
			score := 1.0
			if score > bestScore {
				bestScore = score
				bestWorker = worker
			}
			continue
		}

		successRate := float64(worker.TasksSucceeded) / float64(totalTasks)
		loadFactor := 1.0 - (worker.LoadAverage / 100.0) // Lower load is better

		score := successRate*0.7 + loadFactor*0.3

		if score > bestScore {
			bestScore = score
			bestWorker = worker
		}
	}

	return bestWorker
}

// Global worker pool instance
var globalPool *WorkerPool

// GetGlobalPool returns the global worker pool
func GetGlobalPool() *WorkerPool {
	return globalPool
}

// SetGlobalPool sets the global worker pool
func SetGlobalPool(pool *WorkerPool) {
	globalPool = pool
}

// InitializeGlobalPool initializes the global worker pool
func InitializeGlobalPool(config *config.WorkersConfig) {
	globalPool = NewWorkerPool(config)
}

// AssignTaskGlobal assigns a task using the global pool
func AssignTaskGlobal(ctx context.Context, taskType string, requirements map[string]interface{}) (*PoolWorker, error) {
	if globalPool == nil {
		return nil, fmt.Errorf("global worker pool not initialized")
	}
	return globalPool.AssignTask(ctx, taskType, requirements)
}

// ReleaseWorkerGlobal releases a worker using the global pool
func ReleaseWorkerGlobal(workerID string) {
	if globalPool != nil {
		globalPool.ReleaseWorker(workerID)
	}
}
