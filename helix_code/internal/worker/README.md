# Worker Package

The `worker` package provides distributed worker management via SSH for the HelixCode platform.

## Overview

This package handles:
- SSH-based worker pool management
- Automatic Helix CLI installation on workers
- Health monitoring and heartbeats
- Resource tracking (CPU, memory, GPU)
- Capability-based task assignment
- Connection pooling with host key verification
- Worker isolation and sandboxing
- Consensus management for distributed coordination

## Key Types

### SSHWorkerPool

The main SSH worker pool manager:

```go
type SSHWorkerPool struct {
    workers     map[uuid.UUID]*SSHWorker
    mutex       sync.RWMutex
    autoInstall bool
    hostKeys    *HostKeyManager
    isolation   *WorkerIsolationManager
    consensus   *ConsensusManager
}
```

### SSHWorker

Represents an SSH-accessible worker node:

```go
type SSHWorker struct {
    ID           uuid.UUID
    Hostname     string
    DisplayName  string
    SSHConfig    *SSHWorkerConfig
    Capabilities []string
    Resources    Resources
    Status       WorkerStatus
    HealthStatus WorkerHealth
    LastCheck    time.Time
    CreatedAt    time.Time
    UpdatedAt    time.Time
}
```

### Worker

Represents a distributed worker (used by WorkerManager):

```go
type Worker struct {
    ID                 uuid.UUID              `json:"id"`
    Hostname           string                 `json:"hostname"`
    DisplayName        string                 `json:"display_name"`
    SSHConfig          map[string]interface{} `json:"ssh_config"`
    Capabilities       []string               `json:"capabilities"`
    Resources          Resources              `json:"resources"`
    Status             WorkerStatus           `json:"status"`
    HealthStatus       WorkerHealth           `json:"health_status"`
    LastHeartbeat      time.Time              `json:"last_heartbeat"`
    CPUUsagePercent    float64                `json:"cpu_usage_percent"`
    MemoryUsagePercent float64                `json:"memory_usage_percent"`
    DiskUsagePercent   float64                `json:"disk_usage_percent"`
    CurrentTasksCount  int                    `json:"current_tasks_count"`
    MaxConcurrentTasks int                    `json:"max_concurrent_tasks"`
    CreatedAt          time.Time              `json:"created_at"`
    UpdatedAt          time.Time              `json:"updated_at"`
}
```

### Resources

Worker hardware resources:

```go
type Resources struct {
    CPUCount    int    `json:"cpu_count"`
    TotalMemory int64  `json:"total_memory"` // in bytes
    TotalDisk   int64  `json:"total_disk"`   // in bytes
    GPUCount    int    `json:"gpu_count"`
    GPUModel    string `json:"gpu_model"`
    GPUMemory   int64  `json:"gpu_memory"` // in bytes
}
```

### SSHWorkerConfig

SSH connection configuration:

```go
type SSHWorkerConfig struct {
    Host                  string
    Port                  int
    Username              string
    PrivateKey            string
    Password              string
    KeyPath               string
    KnownHostsPath        string
    HostKeyFingerprint    string
    StrictHostKeyChecking bool
}
```

## Usage

### Creating a Worker Pool

```go
import "dev.helix.code/internal/worker"

// Create pool with auto-install enabled
pool := worker.NewSSHWorkerPool(true)
```

### Adding Workers

```go
// Create SSH worker configuration
sshWorker := &worker.SSHWorker{
    Hostname:    "worker-host.example.com",
    DisplayName: "Build Worker 1",
    SSHConfig: &worker.SSHWorkerConfig{
        Host:     "worker-host.example.com",
        Port:     22,
        Username: "helix",
        KeyPath:  "/path/to/private/key",
    },
    Capabilities: []string{"docker-execution", "python-execution"},
}

err := pool.AddWorker(ctx, sshWorker)
if err != nil {
    return err
}
```

### Auto-Installation

Workers automatically get the Helix CLI installed when `autoInstall` is enabled:

```go
pool := worker.NewSSHWorkerPool(true) // Enable auto-install
pool.AddWorker(ctx, worker)           // CLI is installed if missing
```

### Executing Commands

```go
// Execute command on worker (with sandbox isolation)
output, err := pool.ExecuteCommand(ctx, workerID, "ls -la /tmp")
if err != nil {
    return err
}
fmt.Println(output)
```

### Health Monitoring

```go
// Perform health check on all workers
err := pool.HealthCheck(ctx)

// Get worker statistics
stats := pool.GetWorkerStats(ctx)
fmt.Printf("Active: %d, Healthy: %d\n", stats.ActiveWorkers, stats.HealthyWorkers)
```

### Using WorkerManager

```go
// Create worker manager with repository
manager := worker.NewWorkerManager(repo, 30*time.Second) // 30s health TTL

// Register a worker
err := manager.RegisterWorker(ctx, worker)

// Get available workers with capabilities
workers, err := manager.GetAvailableWorkers(ctx, []string{"gpu", "python"})

// Update heartbeat with metrics
err := manager.UpdateWorkerHeartbeat(ctx, workerID, metrics)
```

## Configuration

```yaml
workers:
  enabled: true
  auto_install: true
  health_check_interval: 30
  max_concurrent_tasks: 5
  task_timeout: 3600
  pool:
    worker1:
      host: "worker1.example.com"
      port: 22
      username: "helix"
      key_path: "~/.ssh/id_rsa"
      display_name: "Build Worker 1"
      capabilities: ["docker-execution", "python-execution"]
```

## Worker Status

Workers can have the following statuses:

| Status | Description |
|--------|-------------|
| `active` | Worker is active and available |
| `inactive` | Worker is inactive |
| `maintenance` | Worker is under maintenance |
| `failed` | Worker has failed |
| `offline` | Cannot connect to worker |

## Worker Health

Health statuses for workers:

| Health Status | Description |
|---------------|-------------|
| `healthy` | Worker is healthy |
| `degraded` | Worker has high resource usage (>70%) |
| `unhealthy` | Worker has critical resource usage (>90%) |
| `unknown` | Health status unknown |

## SSH Security

Workers connect via SSH with proper security:

1. **Host Key Verification**: Uses `HostKeyManager` with known_hosts file
2. **SSH Keys**: Private key authentication (recommended)
3. **Key File**: Load key from file path
4. **Password**: Password authentication (not recommended)

```go
config := &worker.SSHWorkerConfig{
    Host:                  "worker.example.com",
    Port:                  22,
    Username:              "helix",
    KeyPath:               "~/.ssh/id_rsa",
    StrictHostKeyChecking: true,
}
```

## Worker Isolation

Commands are executed in sandboxed environments for security:

- Namespace isolation (mount, PID, network, IPC, UTS)
- Resource limits (CPU, memory)
- Temporary workspaces
- Automatic cleanup of expired sandboxes (24 hours)

## Testing

```bash
go test -v ./internal/worker/...
```

## Notes

- Workers are health-checked based on configured interval
- Unhealthy workers are automatically marked as offline
- Use SSH keys for secure worker authentication
- Monitor worker resources to prevent overloading
- Host key verification prevents MITM attacks
- Sandboxing provides process isolation for security
