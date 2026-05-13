package worker

import (
	"context"
	"fmt"
	"time"

	"dev.helix.code/internal/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// DatabaseManager handles worker lifecycle and operations with database persistence
type DatabaseManager struct {
	db database.DatabaseInterface
}

// NewDatabaseManager creates a new worker manager with database persistence
func NewDatabaseManager(db database.DatabaseInterface) *DatabaseManager {
	return &DatabaseManager{
		db: db,
	}
}

// GetWorker retrieves a worker by ID from database
func (m *DatabaseManager) GetWorker(ctx context.Context, id string) (*Worker, error) {
	workerID, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("invalid worker ID: %v", err)
	}

	query := `
		SELECT 
			id, hostname, display_name, ssh_config, capabilities, resources,
			status, health_status, last_heartbeat, cpu_usage_percent,
			memory_usage_percent, disk_usage_percent, current_tasks_count,
			max_concurrent_tasks, created_at, updated_at
		FROM workers
		WHERE id = $1
	`

	var (
		dbID               uuid.UUID
		hostname           string
		displayName        string
		sshConfig          map[string]interface{}
		capabilities       []string
		resources          map[string]interface{}
		status             string
		healthStatus       string
		lastHeartbeat      *time.Time
		cpuUsagePercent    *float64
		memoryUsagePercent *float64
		diskUsagePercent   *float64
		currentTasksCount  int
		maxConcurrentTasks int
		createdAt          time.Time
		updatedAt          time.Time
	)

	err = m.db.QueryRow(ctx, query, workerID).Scan(
		&dbID, &hostname, &displayName, &sshConfig, &capabilities, &resources,
		&status, &healthStatus, &lastHeartbeat, &cpuUsagePercent,
		&memoryUsagePercent, &diskUsagePercent, &currentTasksCount,
		&maxConcurrentTasks, &createdAt, &updatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("worker not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get worker from database: %v", err)
	}

	// Convert nullable fields
	var lastHeartbeatTime time.Time
	if lastHeartbeat != nil {
		lastHeartbeatTime = *lastHeartbeat
	}

	var cpuUsage float64
	if cpuUsagePercent != nil {
		cpuUsage = *cpuUsagePercent
	}

	var memoryUsage float64
	if memoryUsagePercent != nil {
		memoryUsage = *memoryUsagePercent
	}

	var diskUsage float64
	if diskUsagePercent != nil {
		diskUsage = *diskUsagePercent
	}

	worker := &Worker{
		ID:                 dbID,
		Hostname:           hostname,
		DisplayName:        displayName,
		SSHConfig:          sshConfig,
		Capabilities:       capabilities,
		Resources:          parseResources(resources),
		Status:             WorkerStatus(status),
		HealthStatus:       WorkerHealth(healthStatus),
		LastHeartbeat:      lastHeartbeatTime,
		CPUUsagePercent:    cpuUsage,
		MemoryUsagePercent: memoryUsage,
		DiskUsagePercent:   diskUsage,
		CurrentTasksCount:  currentTasksCount,
		MaxConcurrentTasks: maxConcurrentTasks,
		CreatedAt:          createdAt,
		UpdatedAt:          updatedAt,
	}

	return worker, nil
}

// ListWorkers returns all workers from database
func (m *DatabaseManager) ListWorkers(ctx context.Context) ([]*Worker, error) {
	query := `
		SELECT 
			id, hostname, display_name, ssh_config, capabilities, resources,
			status, health_status, last_heartbeat, cpu_usage_percent,
			memory_usage_percent, disk_usage_percent, current_tasks_count,
			max_concurrent_tasks, created_at, updated_at
		FROM workers
		ORDER BY created_at DESC
	`

	rows, err := m.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query workers: %v", err)
	}
	defer rows.Close()

	var workers []*Worker
	for rows.Next() {
		var (
			dbID               uuid.UUID
			hostname           string
			displayName        string
			sshConfig          map[string]interface{}
			capabilities       []string
			resources          map[string]interface{}
			status             string
			healthStatus       string
			lastHeartbeat      *time.Time
			cpuUsagePercent    *float64
			memoryUsagePercent *float64
			diskUsagePercent   *float64
			currentTasksCount  int
			maxConcurrentTasks int
			createdAt          time.Time
			updatedAt          time.Time
		)

		if err := rows.Scan(
			&dbID, &hostname, &displayName, &sshConfig, &capabilities, &resources,
			&status, &healthStatus, &lastHeartbeat, &cpuUsagePercent,
			&memoryUsagePercent, &diskUsagePercent, &currentTasksCount,
			&maxConcurrentTasks, &createdAt, &updatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan worker row: %v", err)
		}

		// Convert nullable fields
		var lastHeartbeatTime time.Time
		if lastHeartbeat != nil {
			lastHeartbeatTime = *lastHeartbeat
		}

		var cpuUsage float64
		if cpuUsagePercent != nil {
			cpuUsage = *cpuUsagePercent
		}

		var memoryUsage float64
		if memoryUsagePercent != nil {
			memoryUsage = *memoryUsagePercent
		}

		var diskUsage float64
		if diskUsagePercent != nil {
			diskUsage = *diskUsagePercent
		}

		worker := &Worker{
			ID:                 dbID,
			Hostname:           hostname,
			DisplayName:        displayName,
			SSHConfig:          sshConfig,
			Capabilities:       capabilities,
			Resources:          parseResources(resources),
			Status:             WorkerStatus(status),
			HealthStatus:       WorkerHealth(healthStatus),
			LastHeartbeat:      lastHeartbeatTime,
			CPUUsagePercent:    cpuUsage,
			MemoryUsagePercent: memoryUsage,
			DiskUsagePercent:   diskUsage,
			CurrentTasksCount:  currentTasksCount,
			MaxConcurrentTasks: maxConcurrentTasks,
			CreatedAt:          createdAt,
			UpdatedAt:          updatedAt,
		}

		workers = append(workers, worker)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating worker rows: %v", err)
	}

	return workers, nil
}

// RegisterWorker registers a new worker in the system
func (m *DatabaseManager) RegisterWorker(ctx context.Context, hostname, displayName string, sshConfig map[string]interface{}, capabilities []string, resources map[string]interface{}) (*Worker, error) {
	// ssh_config is JSONB NOT NULL in the workers schema. When the caller
	// omits ssh_config (or sends an empty body), it arrives as a nil map,
	// which pgx serializes to SQL NULL — triggering a "null value in
	// column ssh_config violates not-null constraint" 500 at INSERT
	// time. Same pattern as the task_data NOT NULL bug in
	// internal/task/manager_db.go:CreateTask. Default to empty JSON
	// object at the persistence constructor so the schema invariant
	// always holds. capabilities is a text[] column that pgx handles
	// nil-as-empty-array correctly, so no equivalent fix needed there.
	if sshConfig == nil {
		sshConfig = map[string]interface{}{}
	}
	// capabilities is TEXT[] NOT NULL DEFAULT '{}' — but the INSERT
	// explicitly passes the value, so the column DEFAULT never applies.
	// A nil []string in Go marshals to SQL NULL via pgx, violating the
	// NOT NULL constraint. Default to empty slice.
	if capabilities == nil {
		capabilities = []string{}
	}

	worker := &Worker{
		ID:                 uuid.New(),
		Hostname:           hostname,
		DisplayName:        displayName,
		SSHConfig:          sshConfig,
		Capabilities:       capabilities,
		Resources:          parseResources(resources),
		Status:             "active",
		HealthStatus:       "healthy",
		CurrentTasksCount:  0,
		MaxConcurrentTasks: 10,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	query := `
		INSERT INTO workers (
			id, hostname, display_name, ssh_config, capabilities, resources,
			status, health_status, current_tasks_count, max_concurrent_tasks,
			created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING created_at, updated_at
	`

	var createdAt, updatedAt time.Time
	err := m.db.QueryRow(ctx, query,
		worker.ID, worker.Hostname, worker.DisplayName, worker.SSHConfig,
		worker.Capabilities, worker.Resources, worker.Status, worker.HealthStatus,
		worker.CurrentTasksCount, worker.MaxConcurrentTasks,
		worker.CreatedAt, worker.UpdatedAt,
	).Scan(&createdAt, &updatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to register worker in database: %v", err)
	}

	worker.CreatedAt = createdAt
	worker.UpdatedAt = updatedAt

	return worker, nil
}

// UpdateWorkerHeartbeat updates worker heartbeat and metrics.
//
// Anti-bluff (CONST-035): the previous version updated only
// `workers.last_heartbeat` + appended a row to the `worker_metrics`
// time-series table — but NEVER updated the worker's
// cpu_usage_percent / memory_usage_percent / disk_usage_percent
// snapshot columns. Result: GET /workers/:id always returned
// `"cpu_usage_percent": 0, "memory_usage_percent": 0,
//  "disk_usage_percent": 0` regardless of how many heartbeats had
// landed. The worker record SAID it was the current-state snapshot
// (`workers.cpu_usage_percent` exists as a column) but actually
// reflected only the initial-create zero values forever.
//
// Now the heartbeat also writes the current values into the
// workers row, so GET /workers/:id reflects the latest heartbeat.
func (m *DatabaseManager) UpdateWorkerHeartbeat(ctx context.Context, id string, metrics map[string]interface{}) error {
	workerID, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid worker ID: %v", err)
	}
	if metrics == nil {
		metrics = map[string]interface{}{}
	}

	cpuUsage, _ := metrics["cpu_usage_percent"].(float64)
	memoryUsage, _ := metrics["memory_usage_percent"].(float64)
	diskUsage, _ := metrics["disk_usage_percent"].(float64)
	networkRx, _ := metrics["network_rx_bytes"].(int64)
	networkTx, _ := metrics["network_tx_bytes"].(int64)
	currentTasks, _ := metrics["current_tasks_count"].(int)
	temperature, _ := metrics["temperature_celsius"].(float64)

	// Update the worker's snapshot columns AND the heartbeat timestamp.
	// COALESCE(NULLIF) preserves existing values when the caller omits
	// a field (sends 0.0 as the JSON zero-value). The previous query
	// updated only last_heartbeat — turning the snapshot columns into
	// a "current state" bluff.
	if _, err = m.db.Exec(ctx, `
		UPDATE workers
		SET last_heartbeat       = NOW(),
		    cpu_usage_percent    = CASE WHEN $2 > 0 THEN $2 ELSE cpu_usage_percent END,
		    memory_usage_percent = CASE WHEN $3 > 0 THEN $3 ELSE memory_usage_percent END,
		    disk_usage_percent   = CASE WHEN $4 > 0 THEN $4 ELSE disk_usage_percent END,
		    updated_at           = NOW()
		WHERE id = $1
	`, workerID, cpuUsage, memoryUsage, diskUsage); err != nil {
		return fmt.Errorf("failed to update worker heartbeat: %v", err)
	}

	// Store time-series row for history queries.
	if _, err = m.db.Exec(ctx, `
		INSERT INTO worker_metrics (
			worker_id, cpu_usage_percent, memory_usage_percent, disk_usage_percent,
			network_rx_bytes, network_tx_bytes, current_tasks_count, temperature_celsius
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, workerID, cpuUsage, memoryUsage, diskUsage,
		networkRx, networkTx, currentTasks, temperature,
	); err != nil {
		return fmt.Errorf("failed to store worker metrics: %v", err)
	}

	return nil
}

// UpdateWorker updates an existing worker in the database.
//
// Anti-bluff (CONST-035): the previous version unconditionally
// replaced every column with the request value — passing
// capabilities=nil ([]string nil) marshaled to SQL NULL via pgx,
// violating the TEXT[] NOT NULL constraint and returning a 500.
// Same pattern as RegisterWorker (round 5). And: passing an empty
// hostname would have overwritten the existing hostname with ""
// (a partial update should preserve unmentioned fields, not blank
// them). The fix mirrors UpdateProject: COALESCE+NULLIF preserves
// the existing column when the input is the zero value, and
// capabilities defaults to an empty slice when nil so the column
// invariant always holds.
func (m *DatabaseManager) UpdateWorker(ctx context.Context, id string, hostname, displayName string, capabilities []string, maxConcurrentTasks int) (*Worker, error) {
	workerID, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("invalid worker ID: %v", err)
	}
	if capabilities == nil {
		capabilities = []string{}
	}

	query := `
		UPDATE workers
		SET hostname             = COALESCE(NULLIF($1, ''), hostname),
		    display_name         = COALESCE(NULLIF($2, ''), display_name),
		    capabilities         = $3,
		    max_concurrent_tasks = CASE WHEN $4 > 0 THEN $4 ELSE max_concurrent_tasks END,
		    updated_at           = NOW()
		WHERE id = $5
		RETURNING id, hostname, display_name, ssh_config, capabilities, resources,
			status, health_status, last_heartbeat, cpu_usage_percent,
			memory_usage_percent, disk_usage_percent, current_tasks_count,
			max_concurrent_tasks, created_at, updated_at
	`

	var (
		dbID                uuid.UUID
		returnedHostname    string
		returnedDisplayName string
		sshConfig           map[string]interface{}
		returnedCaps        []string
		resources           map[string]interface{}
		status              string
		healthStatus        string
		lastHeartbeat       *time.Time
		cpuUsagePercent     *float64
		memoryUsagePercent  *float64
		diskUsagePercent    *float64
		currentTasksCount   int
		returnedMaxTasks    int
		createdAt           time.Time
		updatedAt           time.Time
	)

	err = m.db.QueryRow(ctx, query, hostname, displayName, capabilities, maxConcurrentTasks, workerID).Scan(
		&dbID, &returnedHostname, &returnedDisplayName, &sshConfig, &returnedCaps, &resources,
		&status, &healthStatus, &lastHeartbeat, &cpuUsagePercent,
		&memoryUsagePercent, &diskUsagePercent, &currentTasksCount,
		&returnedMaxTasks, &createdAt, &updatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to update worker: %v", err)
	}

	// Convert nullable fields
	var lastHeartbeatTime time.Time
	if lastHeartbeat != nil {
		lastHeartbeatTime = *lastHeartbeat
	}

	var cpuUsage float64
	if cpuUsagePercent != nil {
		cpuUsage = *cpuUsagePercent
	}

	var memoryUsage float64
	if memoryUsagePercent != nil {
		memoryUsage = *memoryUsagePercent
	}

	var diskUsage float64
	if diskUsagePercent != nil {
		diskUsage = *diskUsagePercent
	}

	worker := &Worker{
		ID:                 dbID,
		Hostname:           returnedHostname,
		DisplayName:        returnedDisplayName,
		SSHConfig:          sshConfig,
		Capabilities:       returnedCaps,
		Resources:          parseResources(resources),
		Status:             WorkerStatus(status),
		HealthStatus:       WorkerHealth(healthStatus),
		LastHeartbeat:      lastHeartbeatTime,
		CPUUsagePercent:    cpuUsage,
		MemoryUsagePercent: memoryUsage,
		DiskUsagePercent:   diskUsage,
		CurrentTasksCount:  currentTasksCount,
		MaxConcurrentTasks: returnedMaxTasks,
		CreatedAt:          createdAt,
		UpdatedAt:          updatedAt,
	}

	return worker, nil
}

// DeleteWorker removes a worker from the database
func (m *DatabaseManager) DeleteWorker(ctx context.Context, id string) error {
	workerID, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid worker ID: %v", err)
	}

	query := `DELETE FROM workers WHERE id = $1`

	result, err := m.db.Exec(ctx, query, workerID)
	if err != nil {
		return fmt.Errorf("failed to delete worker: %v", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("worker not found: %s", id)
	}

	return nil
}

// GetWorkerMetrics retrieves metrics for a worker
func (m *DatabaseManager) GetWorkerMetrics(ctx context.Context, id string, since time.Time) ([]*WorkerMetrics, error) {
	workerID, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("invalid worker ID: %v", err)
	}

	query := `
		SELECT
			id, worker_id, cpu_usage_percent, memory_usage_percent, disk_usage_percent,
			network_rx_bytes, network_tx_bytes, current_tasks_count, temperature_celsius, recorded_at
		FROM worker_metrics
		WHERE worker_id = $1 AND recorded_at >= $2
		ORDER BY recorded_at DESC
		LIMIT 100
	`

	rows, err := m.db.Query(ctx, query, workerID, since)
	if err != nil {
		return nil, fmt.Errorf("failed to query worker metrics: %v", err)
	}
	defer rows.Close()

	var metrics []*WorkerMetrics
	for rows.Next() {
		var (
			metricsID          uuid.UUID
			returnedWorkerID   uuid.UUID
			cpuUsagePercent    float64
			memoryUsagePercent float64
			diskUsagePercent   float64
			networkRxBytes     int64
			networkTxBytes     int64
			currentTasksCount  int
			temperatureCelsius *float64
			recordedAt         time.Time
		)

		if err := rows.Scan(
			&metricsID, &returnedWorkerID, &cpuUsagePercent, &memoryUsagePercent, &diskUsagePercent,
			&networkRxBytes, &networkTxBytes, &currentTasksCount, &temperatureCelsius, &recordedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan metrics row: %v", err)
		}

		var temp float64
		if temperatureCelsius != nil {
			temp = *temperatureCelsius
		}

		metric := &WorkerMetrics{
			ID:                 metricsID,
			WorkerID:           returnedWorkerID,
			CPUUsagePercent:    cpuUsagePercent,
			MemoryUsagePercent: memoryUsagePercent,
			DiskUsagePercent:   diskUsagePercent,
			NetworkRxBytes:     networkRxBytes,
			NetworkTxBytes:     networkTxBytes,
			CurrentTasksCount:  currentTasksCount,
			TemperatureCelsius: temp,
			RecordedAt:         recordedAt,
		}

		metrics = append(metrics, metric)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating metrics rows: %v", err)
	}

	return metrics, nil
}

// Helper functions for parsing

func getStringDB(m map[string]interface{}, key, defaultValue string) string {
	if val, ok := m[key].(string); ok {
		return val
	}
	return defaultValue
}

func getIntDB(m map[string]interface{}, key string, defaultValue int) int {
	if val, ok := m[key].(float64); ok {
		return int(val)
	}
	return defaultValue
}

func getInt64DB(m map[string]interface{}, key string, defaultValue int64) int64 {
	if val, ok := m[key].(float64); ok {
		return int64(val)
	}
	return defaultValue
}

// parseResources converts a map to Resources struct
func parseResources(resources map[string]interface{}) Resources {
	return Resources{
		CPUCount:    getIntDB(resources, "cpu_count", 0),
		TotalMemory: getInt64DB(resources, "total_memory", 0),
		TotalDisk:   getInt64DB(resources, "total_disk", 0),
		GPUCount:    getIntDB(resources, "gpu_count", 0),
		GPUModel:    getStringDB(resources, "gpu_model", ""),
		GPUMemory:   getInt64DB(resources, "gpu_memory", 0),
	}
}
