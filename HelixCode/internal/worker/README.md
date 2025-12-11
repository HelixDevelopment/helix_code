# Worker Package

The `worker` package provides distributed worker management via SSH for the HelixCode platform.

## Overview

This package handles:
- SSH-based worker pool management
- Automatic Helix CLI installation on workers
- Health monitoring and heartbeats
- Resource tracking (CPU, memory, GPU)
- Capability-based task assignment
- Connection pooling and retry logic

## Key Types

### SSHWorkerPool

The main worker pool manager:

```go
type SSHWorkerPool struct {
    workers    map[string]*Worker
    config     *PoolConfig
    mu         sync.RWMutex
    healthChan chan *HealthReport
}
```

### Worker

Represents a distributed worker:

```go
type Worker struct {
    ID           string
    Host         string
    User         string
    Port         int
    Status       WorkerStatus
    Capabilities []string
    Resources    *ResourceInfo
    LastSeen     time.Time
}
```

### ResourceInfo

Worker resource information:

```go
type ResourceInfo struct {
    CPUCores     int
    MemoryTotal  int64
    MemoryFree   int64
    GPUAvailable bool
    GPUMemory    int64
    DiskFree     int64
}
```

## Usage

### Creating a Worker Pool

```go
import "dev.helix.code/internal/worker"

config := &worker.PoolConfig{
    MaxWorkers:          10,
    HealthCheckInterval: 30 * time.Second,
    ConnectionTimeout:   10 * time.Second,
}

pool := worker.NewSSHWorkerPool(config)
```

### Adding Workers

```go
// Add worker via SSH
workerConfig := &worker.WorkerConfig{
    Host:       "worker-host.example.com",
    User:       "helix",
    Port:       22,
    PrivateKey: privateKeyPath,
}

err := pool.AddWorker(ctx, workerConfig)
if err != nil {
    return err
}
```

### Auto-Installation

Workers automatically get the Helix CLI installed:

```go
// This happens automatically when adding a worker
// The pool detects if Helix is installed and installs if needed
pool.AddWorker(ctx, workerConfig)
```

### Health Monitoring

```go
// Start health monitoring
pool.StartHealthCheck(ctx)

// Get worker health status
health := pool.GetWorkerHealth(workerID)

// Subscribe to health reports
reports := pool.HealthReports()
for report := range reports {
    if report.Status == worker.StatusUnhealthy {
        log.Warn("Worker unhealthy: %s", report.WorkerID)
    }
}
```

### Task Assignment

```go
// Get available worker with specific capability
worker, err := pool.GetAvailableWorker(ctx, []string{"gpu", "python"})
if err != nil {
    return err
}

// Assign task to worker
err = pool.AssignTask(ctx, worker.ID, task)
```

### Resource Tracking

```go
// Get worker resources
resources := pool.GetWorkerResources(workerID)
fmt.Printf("CPU: %d cores, Memory: %d GB\n",
    resources.CPUCores,
    resources.MemoryTotal/(1024*1024*1024))

// Get pool-wide resources
poolResources := pool.GetPoolResources()
```

## Configuration

```yaml
workers:
  max_workers: 10
  health_check_interval: 30s
  connection_timeout: 10s
  retry_attempts: 3
  retry_backoff: 5s
  auto_install: true
```

## Worker Status

Workers can have the following statuses:

| Status | Description |
|--------|-------------|
| `idle` | Worker available for tasks |
| `busy` | Worker executing a task |
| `unhealthy` | Health check failed |
| `offline` | Cannot connect to worker |
| `installing` | Installing Helix CLI |

## SSH Configuration

Workers connect via SSH with these authentication methods:

1. SSH key (recommended)
2. SSH agent forwarding
3. Password (not recommended for production)

```go
config := &worker.WorkerConfig{
    Host:       "worker.example.com",
    User:       "helix",
    PrivateKey: "~/.ssh/id_rsa",  // or use SSHAgent: true
}
```

## Testing

```bash
go test -v ./internal/worker/...
```

## Notes

- Workers are health-checked every 30s by default
- Unhealthy workers are automatically removed from active pool
- Use SSH keys for secure worker authentication
- Monitor worker resources to prevent overloading
