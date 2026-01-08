// Package worker provides distributed worker pool management with SSH-based task execution.
//
// The worker package implements a comprehensive distributed worker management system
// that supports SSH-based remote execution, health monitoring, capability detection,
// resource tracking, and secure host key verification. It includes support for
// worker isolation through sandboxing and consensus protocols for leader election.
//
// # Key Components
//
// WorkerManager manages workers with a repository backend:
//
//	repo := worker.NewMemoryRepository() // or DatabaseRepository
//	manager := worker.NewWorkerManager(repo, 30*time.Second)
//
//	// Register a worker
//	w := &worker.Worker{
//	    Hostname:           "worker-01.example.com",
//	    DisplayName:        "Worker 01",
//	    Capabilities:       []string{"build", "test"},
//	    MaxConcurrentTasks: 4,
//	}
//	err := manager.RegisterWorker(ctx, w)
//
// # SSH Worker Pool
//
// SSHWorkerPool manages SSH-accessible distributed workers:
//
//	pool := worker.NewSSHWorkerPool(true) // auto-install CLI
//
//	// Add a worker
//	w := &worker.SSHWorker{
//	    Hostname:    "remote-01.example.com",
//	    DisplayName: "Remote Worker 01",
//	    SSHConfig: &worker.SSHWorkerConfig{
//	        Host:     "remote-01.example.com",
//	        Port:     22,
//	        Username: "helix",
//	        KeyPath:  "/home/helix/.ssh/id_rsa",
//	    },
//	}
//	err := pool.AddWorker(ctx, w)
//
//	// Execute commands on worker
//	output, err := pool.ExecuteCommand(ctx, workerID, "go build ./...")
//
// # Worker Status
//
// Workers have defined statuses:
//
//	worker.WorkerStatusActive      // Ready for tasks
//	worker.WorkerStatusInactive    // Not accepting tasks
//	worker.WorkerStatusMaintenance // Under maintenance
//	worker.WorkerStatusFailed      // Failed health check
//	worker.WorkerStatusOffline     // Not reachable
//
// # Worker Health
//
// Worker health is tracked separately:
//
//	worker.WorkerHealthHealthy   // All metrics normal
//	worker.WorkerHealthDegraded  // High resource usage (>70%)
//	worker.WorkerHealthUnhealthy // Critical resource usage (>90%)
//	worker.WorkerHealthUnknown   // Unable to determine
//
// # Resource Tracking
//
// Workers track their hardware resources:
//
//	resources := worker.Resources{
//	    CPUCount:    8,
//	    TotalMemory: 16 * 1024 * 1024 * 1024, // 16GB
//	    TotalDisk:   500 * 1024 * 1024 * 1024, // 500GB
//	    GPUCount:    2,
//	    GPUModel:    "NVIDIA RTX 4090",
//	    GPUMemory:   24 * 1024 * 1024 * 1024, // 24GB
//	}
//
// # Health Monitoring
//
// Perform health checks on workers:
//
//	// Manager-based health check
//	err := manager.HealthCheck(ctx)
//
//	// Pool-based health check
//	err = pool.HealthCheck(ctx)
//
//	// Workers exceeding healthTTL are marked unhealthy
//
// # Heartbeat Updates
//
// Workers send heartbeat updates with metrics:
//
//	metrics := &worker.WorkerMetrics{
//	    CPUUsagePercent:    45.5,
//	    MemoryUsagePercent: 62.0,
//	    DiskUsagePercent:   35.0,
//	    CurrentTasksCount:  2,
//	}
//	err := manager.UpdateWorkerHeartbeat(ctx, workerID, metrics)
//
// # Task Assignment
//
// Assign tasks to workers:
//
//	// Get available workers with required capabilities
//	workers, err := manager.GetAvailableWorkers(ctx, []string{"build", "docker"})
//
//	// Assign a task
//	err = manager.AssignTask(ctx, workerID)
//
//	// Complete a task
//	err = manager.CompleteTask(ctx, workerID)
//
// # SSH Security
//
// The SSH pool implements secure host key verification:
//
//	hostKeys := worker.NewHostKeyManager("/home/user/.ssh/known_hosts")
//	err := hostKeys.LoadKnownHosts()
//
//	// Add new host key
//	hostKeys.AddHostKey(hostname, publicKey)
//
//	// Get fingerprint
//	fingerprint := hostKeys.GetHostKeyFingerprint(publicKey)
//
// SSH connections use:
//   - Known hosts verification
//   - Public key or password authentication
//   - Strong cipher suites (AES-CTR)
//   - Strong MACs (HMAC-SHA2-256)
//
// # Worker Isolation
//
// Execute commands in isolated sandboxes:
//
//	isolation := worker.NewWorkerIsolationManager()
//
//	// Create sandbox
//	sandbox, err := isolation.CreateSandbox(ctx, workerID, resources)
//
//	// Execute in sandbox
//	stdout, stderr, err := isolation.ExecuteInSandbox(ctx, sandboxID, client, command)
//
//	// Cleanup
//	isolation.CleanupSandbox(ctx, sandboxID)
//
// # Consensus Protocol
//
// Leader election for distributed coordination:
//
//	consensus := worker.NewConsensusManager(worker.ConsensusConfig{
//	    NodeID: "node-1",
//	    Peers:  []string{"node-2", "node-3"},
//	    OnLeaderElected: func(leaderID string) {
//	        log.Printf("New leader: %s", leaderID)
//	    },
//	})
//	err := consensus.Start(ctx)
//
// # Capability Detection
//
// SSH workers auto-detect capabilities:
//
//	// Detected capabilities include:
//	// - ssh-execution (always)
//	// - remote-computation (always)
//	// - python-execution (if python3 available)
//	// - docker-execution (if docker available)
//	// - cuda-computation (if nvcc available)
//
// # CLI Auto-Installation
//
// SSH pool can auto-install Helix CLI on workers:
//
//	pool := worker.NewSSHWorkerPool(true) // Enable auto-install
//
//	// Custom download URL
//	pool := worker.NewSSHWorkerPoolWithConfig(true, "https://custom.url/helix")
//
//	// Environment variable override
//	// HELIX_CLI_DOWNLOAD_URL=https://custom.url/helix
//
// # Statistics
//
// Get worker pool statistics:
//
//	stats, err := manager.GetWorkerStats(ctx)
//	fmt.Printf("Total: %d, Active: %d, Healthy: %d\n",
//	    stats.TotalWorkers, stats.ActiveWorkers, stats.HealthyWorkers)
//	fmt.Printf("Avg CPU: %.1f%%, Avg Memory: %.1f%%\n",
//	    stats.AverageCPUUsage, stats.AverageMemoryUsage)
//
//	poolStats := pool.GetWorkerStats(ctx)
//	fmt.Printf("Total CPU: %d, Total Memory: %dGB, Total GPU: %d\n",
//	    poolStats.TotalCPU, poolStats.TotalMemory/1024/1024/1024, poolStats.TotalGPU)
//
// # Repository Interface
//
// Implement custom storage backends:
//
//	type WorkerRepository interface {
//	    CreateWorker(ctx context.Context, worker *Worker) error
//	    GetWorker(ctx context.Context, id uuid.UUID) (*Worker, error)
//	    GetWorkerByHostname(ctx context.Context, hostname string) (*Worker, error)
//	    ListWorkers(ctx context.Context, status WorkerStatus) ([]*Worker, error)
//	    UpdateWorker(ctx context.Context, worker *Worker) error
//	    DeleteWorker(ctx context.Context, id uuid.UUID) error
//	    RecordMetrics(ctx context.Context, metrics *WorkerMetrics) error
//	    GetWorkerMetrics(ctx context.Context, workerID uuid.UUID, since time.Time) ([]*WorkerMetrics, error)
//	}
//
// # Thread Safety
//
// All worker management operations are thread-safe through internal mutex
// protection, allowing concurrent access from multiple goroutines.
//
// # Database Integration
//
// Use DatabaseManager for PostgreSQL persistence:
//
//	dbManager := worker.NewDatabaseManager(db)
package worker
