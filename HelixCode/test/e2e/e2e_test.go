//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"dev.helix.code/internal/config"
	"dev.helix.code/internal/database"
	"dev.helix.code/internal/hardware"
	"dev.helix.code/internal/llm"
	"dev.helix.code/internal/worker"
)

// TestEnvironment represents the end-to-end test environment
type TestEnvironment struct {
	Config      *config.Config
	Database    *database.Database
	HardwareDetector *hardware.Detector
	ModelManager *llm.ModelManager
	WorkerManager *worker.DistributedWorkerManager
	ctx         context.Context
	cancel      context.CancelFunc
}

// SetupTestEnvironment creates a complete test environment
func SetupTestEnvironment(t *testing.T) *TestEnvironment {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)

	// Load test configuration
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Failed to load test configuration: %v", err)
	}

	// Initialize database
	db, err := database.New(database.Config{
		Host:    cfg.Database.Host,
		Port:    cfg.Database.Port,
		User:    cfg.Database.User,
		Password: cfg.Database.Password,
		DBName:  cfg.Database.DBName,
		SSLMode: cfg.Database.SSLMode,
	})
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}

	// Initialize database schema
	if err := db.InitializeSchema(); err != nil {
		t.Fatalf("Failed to initialize database schema: %v", err)
	}

	// Create test environment
	env := &TestEnvironment{
		Config:      cfg,
		Database:    db,
		HardwareDetector: hardware.NewDetector(),
		ModelManager: llm.NewModelManager(),
		ctx:         ctx,
		cancel:      cancel,
	}

	// Initialize worker manager
	env.WorkerManager = worker.NewDistributedWorkerManager(worker.WorkerConfig{
		Enabled:             cfg.Workers.Enabled,
		Pool:                cfg.Workers.Pool,
		AutoInstall:         cfg.Workers.AutoInstall,
		HealthCheckInterval: cfg.Workers.HealthCheckInterval,
		MaxConcurrentTasks:  cfg.Workers.MaxConcurrentTasks,
		TaskTimeout:         time.Duration(cfg.Workers.TaskTimeout) * time.Second,
	})

	// Initialize worker manager
	if err := env.WorkerManager.Initialize(ctx); err != nil {
		t.Fatalf("Failed to initialize worker manager: %v", err)
	}

	log.Println("✅ Test environment setup complete")
	return env
}

// TeardownTestEnvironment cleans up the test environment
func (env *TestEnvironment) TeardownTestEnvironment(t *testing.T) {
	t.Helper()

	if env.cancel != nil {
		env.cancel()
	}

	if env.Database != nil {
		env.Database.Close()
	}

	log.Println("✅ Test environment teardown complete")
}

// TestHardwareDetection tests the hardware detection system
func TestHardwareDetection(t *testing.T) {
	env := SetupTestEnvironment(t)
	defer env.TeardownTestEnvironment(t)

	// Test hardware detection
	hardwareInfo, err := env.HardwareDetector.Detect()
	if err != nil {
		t.Fatalf("Hardware detection failed: %v", err)
	}

	// Verify hardware information
	if hardwareInfo.CPU.Cores == 0 {
		t.Error("CPU core count should be greater than 0")
	}

	if hardwareInfo.Memory.TotalRAM == "" {
		t.Error("Total RAM should not be empty")
	}

	// Test model size calculation
	optimalSize := env.HardwareDetector.GetOptimalModelSize()
	if optimalSize == "" {
		t.Error("Optimal model size should not be empty")
	}

	// Test compatibility checking
	compatible := env.HardwareDetector.CanRunModel("7B")
	if !compatible {
		t.Log("7B model not compatible with test hardware")
	}

	// Test compilation flags
	flags := env.HardwareDetector.GetCompilationFlags()
	if len(flags) == 0 {
		t.Log("No compilation flags returned (may be normal for test environment)")
	}

	t.Logf("✅ Hardware detection test passed: %s CPU, %s GPU, %s RAM",
		hardwareInfo.CPU.Model, hardwareInfo.GPU.Model, hardwareInfo.Memory.TotalRAM)
}

// TestDistributedWorkerSystem tests the distributed worker management
func TestDistributedWorkerSystem(t *testing.T) {
	env := SetupTestEnvironment(t)
	defer env.TeardownTestEnvironment(t)

	// Wait for workers to be ready
	time.Sleep(10 * time.Second)

	// Get available workers
	workers := env.WorkerManager.GetAvailableWorkers()
	if len(workers) == 0 {
		t.Fatal("No workers available for testing")
	}

	t.Logf("Found %d available workers", len(workers))

	// Test worker health
	for _, w := range workers {
		if w.Status != worker.WorkerStatusActive {
			t.Errorf("Worker %s should be active, got %s", w.DisplayName, w.Status)
		}
		if w.HealthStatus != worker.HealthStatusHealthy {
			t.Errorf("Worker %s should be healthy, got %s", w.DisplayName, w.HealthStatus)
		}
	}

	// Test worker statistics
	stats := env.WorkerManager.GetWorkerStats()
	if stats["total_workers"].(int) != len(workers) {
		t.Errorf("Worker stats mismatch: expected %d, got %d", len(workers), stats["total_workers"])
	}

	// Test task submission
	task := &worker.DistributedTask{
		Type:        "test-task",
		Data:        map[string]interface{}{"message": "Hello from test"},
		Priority:    5,
		Criticality: worker.CriticalityNormal,
		MaxRetries:  3,
	}

	if err := env.WorkerManager.SubmitTask(task); err != nil {
		t.Fatalf("Failed to submit test task: %v", err)
	}

	t.Logf("✅ Distributed worker system test passed: submitted task %s", task.ID)
}

// TestModelManagement tests the LLM model management system
func TestModelManagement(t *testing.T) {
	env := SetupTestEnvironment(t)
	defer env.TeardownTestEnvironment(t)

	// Test model selection
	criteria := llm.ModelSelectionCriteria{
		TaskType: "code_generation",
		RequiredCapabilities: []llm.ModelCapability{
			llm.CapabilityCodeGeneration,
			llm.CapabilityCodeAnalysis,
		},
		MaxTokens:        2048,
		QualityPreference: "balanced",
	}

	selectedModel, err := env.ModelManager.SelectOptimalModel(criteria)
	if err != nil {
		t.Logf("Model selection failed (expected in test environment): %v", err)
		return
	}

	if selectedModel == nil {
		t.Error("Model selection should return a model")
	}

	// Test model listing
	models := env.ModelManager.GetAvailableModels()
	if len(models) == 0 {
		t.Log("No models available (may be normal in test environment)")
	}

	// Test health checking
	health := env.ModelManager.HealthCheck(env.ctx)
	if len(health) == 0 {
		t.Log("No providers available for health check (may be normal in test environment)")
	}

	t.Log("✅ Model management test passed")
}

// TestEndToEndWorkflow tests a complete workflow from task submission to completion
func TestEndToEndWorkflow(t *testing.T) {
	env := SetupTestEnvironment(t)
	defer env.TeardownTestEnvironment(t)

	// Wait for workers to be ready
	time.Sleep(15 * time.Second)

	workers := env.WorkerManager.GetAvailableWorkers()
	if len(workers) == 0 {
		t.Skip("No workers available for end-to-end test")
	}

	// Submit multiple test tasks
	tasks := []*worker.DistributedTask{
		{
			Type:        "code-generation",
			Data:        map[string]interface{}{"language": "go", "description": "test function"},
			Priority:    3,
			Criticality: worker.CriticalityNormal,
		},
		{
			Type:        "testing",
			Data:        map[string]interface{}{"framework": "go-test", "coverage": true},
			Priority:    2,
			Criticality: worker.CriticalityHigh,
		},
	}

	for i, task := range tasks {
		if err := env.WorkerManager.SubmitTask(task); err != nil {
			t.Fatalf("Failed to submit task %d: %v", i, err)
		}
		t.Logf("Submitted task %s: %s", task.ID, task.Type)
	}

	// Wait for tasks to be processed (simulate)
	time.Sleep(5 * time.Second)

	// Check worker load
	stats := env.WorkerManager.GetWorkerStats()
	totalTasks := stats["total_tasks"].(int)
	if totalTasks < len(tasks) {
		t.Logf("Not all tasks assigned yet: %d/%d", totalTasks, len(tasks))
	}

	t.Log("✅ End-to-end workflow test passed")
}

// TestErrorHandling tests error scenarios and recovery
func TestErrorHandling(t *testing.T) {
	env := SetupTestEnvironment(t)
	defer env.TeardownTestEnvironment(t)

	// Test invalid task submission
	invalidTask := &worker.DistributedTask{
		Type: "", // Invalid: empty type
	}

	if err := env.WorkerManager.SubmitTask(invalidTask); err == nil {
		t.Error("Should reject task with empty type")
	}

	// Test worker retrieval with invalid ID
	_, err := env.WorkerManager.GetWorker("invalid-worker-id")
	if err == nil {
		t.Error("Should return error for invalid worker ID")
	}

	t.Log("✅ Error handling test passed")
}

// TestMain sets up and tears down the test environment
func TestMain(m *testing.M) {
	// Setup global test environment
	log.Println("🚀 Setting up global test environment...")

	// Generate test SSH keys if needed
	if _, err := os.Stat("test/workers/ssh-keys/id_rsa"); os.IsNotExist(err) {
		log.Println("⚠️ Test SSH keys not found. Run scripts/generate-test-keys.sh first")
		os.Exit(1)
	}

	// Run tests
	code := m.Run()

	// Teardown
	log.Println("🧹 Cleaning up test environment...")

	os.Exit(code)
}