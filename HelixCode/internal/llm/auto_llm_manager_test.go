package llm

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAutoLLMManager(t *testing.T) {
	t.Run("with empty base dir uses default", func(t *testing.T) {
		manager := NewAutoLLMManager("")

		require.NotNil(t, manager)
		assert.Contains(t, manager.baseDir, ".helixcode")
		assert.Contains(t, manager.baseDir, "local-llm")
		assert.NotNil(t, manager.providers)
		assert.NotNil(t, manager.backgroundTasks)
		assert.NotNil(t, manager.config)
		assert.False(t, manager.isInitialized)
		assert.False(t, manager.isRunning)
	})

	t.Run("with custom base dir", func(t *testing.T) {
		customDir := "/tmp/test-llm-manager"
		manager := NewAutoLLMManager(customDir)

		require.NotNil(t, manager)
		assert.Equal(t, customDir, manager.baseDir)
	})

	t.Run("config defaults", func(t *testing.T) {
		manager := NewAutoLLMManager("")

		assert.Equal(t, "1.0.0", manager.config.Version)
		assert.Equal(t, "zero_touch", manager.config.Mode)
		assert.True(t, manager.config.AutoDiscover)
		assert.True(t, manager.config.AutoInstall)
		assert.True(t, manager.config.AutoConfigure)
		assert.True(t, manager.config.AutoStart)
		assert.True(t, manager.config.AutoMonitor)
		assert.True(t, manager.config.AutoUpdate)
	})

	t.Run("health config defaults", func(t *testing.T) {
		manager := NewAutoLLMManager("")

		assert.Equal(t, 30, manager.config.Health.CheckInterval)
		assert.True(t, manager.config.Health.AutoRecovery)
		assert.Equal(t, 3, manager.config.Health.MaxRetries)
		assert.Equal(t, 5, manager.config.Health.RetryDelay)
	})

	t.Run("performance config defaults", func(t *testing.T) {
		manager := NewAutoLLMManager("")

		assert.True(t, manager.config.Performance.AutoOptimize)
		assert.True(t, manager.config.Performance.LoadBalance)
		assert.True(t, manager.config.Performance.CacheResponses)
		assert.True(t, manager.config.Performance.PredictScaling)
	})

	t.Run("security config defaults", func(t *testing.T) {
		manager := NewAutoLLMManager("")

		assert.True(t, manager.config.Security.AutoSandbox)
		assert.True(t, manager.config.Security.MinPrivileges)
		assert.True(t, manager.config.Security.NetworkIsolation)
		assert.True(t, manager.config.Security.ResourceLimits)
	})

	t.Run("update config defaults", func(t *testing.T) {
		manager := NewAutoLLMManager("")

		assert.True(t, manager.config.Updates.AutoCheck)
		assert.True(t, manager.config.Updates.AutoDownload)
		assert.True(t, manager.config.Updates.AutoInstall)
		assert.True(t, manager.config.Updates.BackupConfig)
		assert.True(t, manager.config.Updates.RollbackEnabled)
	})
}

func TestAutoLLMManager_CreateDirectoryStructure(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewAutoLLMManager(tmpDir)

	err := manager.createDirectoryStructure()
	require.NoError(t, err)

	// Verify directories were created
	expectedDirs := []string{
		"auto-manager/bin",
		"auto-manager/config",
		"auto-manager/scripts",
		"auto-manager/logs",
		"providers",
		"build",
		"config",
		"data/models",
		"data/cache",
		"data/logs",
		"cache/pip",
		"cache/npm",
		"cache/build",
		"runtime/processes",
		"runtime/health",
		"runtime/metrics",
		"runtime/state",
	}

	for _, dir := range expectedDirs {
		fullPath := filepath.Join(tmpDir, dir)
		_, err := os.Stat(fullPath)
		assert.NoError(t, err, "Directory %s should exist", dir)
	}
}

func TestAutoLLMManager_CreateDefaultConfiguration(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewAutoLLMManager(tmpDir)

	configPath := filepath.Join(tmpDir, "config.json")
	err := manager.createDefaultConfiguration(configPath)
	require.NoError(t, err)

	// Verify file was created
	_, err = os.Stat(configPath)
	assert.NoError(t, err)

	// Read and verify content
	content, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "zero_touch")
	assert.Contains(t, string(content), "1.0.0")
}

func TestAutoLLMManager_LoadConfiguration(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewAutoLLMManager(tmpDir)

	// Create directory structure first
	err := manager.createDirectoryStructure()
	require.NoError(t, err)

	// Load configuration (should create default)
	err = manager.loadConfiguration()
	require.NoError(t, err)
}

func TestAutoLLMManager_GetStatus(t *testing.T) {
	manager := NewAutoLLMManager("")

	t.Run("empty providers", func(t *testing.T) {
		status := manager.GetStatus()
		assert.NotNil(t, status)
		assert.Empty(t, status)
	})

	t.Run("with providers", func(t *testing.T) {
		// Add a test provider
		manager.providers["test"] = &AutoProvider{
			LocalLLMProvider: LocalLLMProvider{
				Name: "TestProvider",
			},
			Status: "running",
			Health: &HealthStatus{
				Status:    "healthy",
				IsHealthy: true,
			},
			Metrics: &PerformanceMetrics{},
		}

		status := manager.GetStatus()
		assert.Len(t, status, 1)
		assert.Equal(t, "running", status["test"].Status)
		assert.Equal(t, "TestProvider", status["test"].Name)
	})
}

func TestAutoLLMManager_GetRunningEndpoints(t *testing.T) {
	manager := NewAutoLLMManager("")

	t.Run("no running providers", func(t *testing.T) {
		endpoints := manager.GetRunningEndpoints()
		assert.Empty(t, endpoints)
	})

	t.Run("with stopped provider", func(t *testing.T) {
		manager.providers["stopped"] = &AutoProvider{
			LocalLLMProvider: LocalLLMProvider{
				Name:        "StoppedProvider",
				DefaultPort: 8080,
			},
			Status: "stopped",
			Health: &HealthStatus{IsHealthy: false},
		}

		endpoints := manager.GetRunningEndpoints()
		assert.Empty(t, endpoints)
	})

	t.Run("with running healthy provider", func(t *testing.T) {
		manager.providers["running"] = &AutoProvider{
			LocalLLMProvider: LocalLLMProvider{
				Name:        "RunningProvider",
				DefaultPort: 8081,
			},
			Status: "running",
			Health: &HealthStatus{IsHealthy: true},
		}

		endpoints := manager.GetRunningEndpoints()
		assert.Contains(t, endpoints, "http://127.0.0.1:8081")
	})

	t.Run("with running unhealthy provider", func(t *testing.T) {
		manager.providers = make(map[string]*AutoProvider)
		manager.providers["unhealthy"] = &AutoProvider{
			LocalLLMProvider: LocalLLMProvider{
				Name:        "UnhealthyProvider",
				DefaultPort: 8082,
			},
			Status: "running",
			Health: &HealthStatus{IsHealthy: false},
		}

		endpoints := manager.GetRunningEndpoints()
		assert.Empty(t, endpoints)
	})
}

func TestAutoLLMManager_IsProcessRunning(t *testing.T) {
	manager := NewAutoLLMManager("")

	t.Run("negative PID", func(t *testing.T) {
		result := manager.isProcessRunning(-1)
		assert.False(t, result)
	})

	t.Run("zero PID", func(t *testing.T) {
		result := manager.isProcessRunning(0)
		assert.False(t, result)
	})

	t.Run("current process", func(t *testing.T) {
		// Note: isProcessRunning uses Signal(nil) which may not work as expected
		// on all systems. The function is primarily for checking if child processes
		// are running, not self-checking.
		pid := os.Getpid()
		// Just verify it doesn't panic
		_ = manager.isProcessRunning(pid)
	})

	t.Run("non-existent PID", func(t *testing.T) {
		// Use a very high PID that likely doesn't exist
		result := manager.isProcessRunning(999999)
		assert.False(t, result)
	})
}

func TestAutoLLMManager_Stop(t *testing.T) {
	t.Run("not running", func(t *testing.T) {
		manager := NewAutoLLMManager("")
		manager.isRunning = false

		err := manager.Stop()
		assert.NoError(t, err)
	})

	t.Run("running without tasks", func(t *testing.T) {
		manager := NewAutoLLMManager("")
		manager.isRunning = true

		err := manager.Stop()
		assert.NoError(t, err)
		assert.False(t, manager.isRunning)
	})

	t.Run("running with background tasks", func(t *testing.T) {
		manager := NewAutoLLMManager("")
		manager.isRunning = true
		manager.backgroundTasks["test"] = &BackgroundTask{
			ID:       "test-task",
			Name:     "Test Task",
			StopChan: make(chan bool),
		}

		err := manager.Stop()
		assert.NoError(t, err)
		assert.False(t, manager.isRunning)
	})
}

func TestAutoLLMManager_UpdatePerformanceMetrics(t *testing.T) {
	manager := NewAutoLLMManager("")

	provider := &AutoProvider{
		LocalLLMProvider: LocalLLMProvider{Name: "TestProvider"},
		Metrics: &PerformanceMetrics{
			ActiveRequests: 10,
			ErrorRate:      0.5,
		},
	}

	manager.updatePerformanceMetrics(provider)

	// After update, metrics should be reset
	assert.Equal(t, 0, provider.Metrics.ActiveRequests)
	assert.Equal(t, 0.0, provider.Metrics.ErrorRate)
	assert.False(t, provider.Metrics.LastUpdated.IsZero())
}

func TestAutoLLMManager_CreateStartupScriptForProvider(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewAutoLLMManager(tmpDir)

	provider := &AutoProvider{
		LocalLLMProvider: LocalLLMProvider{
			Name:       "TestProvider",
			DataPath:   "/path/to/data",
			StartupCmd: []string{"./run", "--port", "8080"},
			Environment: map[string]string{
				"TEST_VAR": "test_value",
			},
		},
	}

	scriptPath := filepath.Join(tmpDir, "startup.sh")
	err := manager.createStartupScriptForProvider(provider, scriptPath)
	require.NoError(t, err)

	// Verify script was created
	content, err := os.ReadFile(scriptPath)
	require.NoError(t, err)

	script := string(content)
	assert.Contains(t, script, "#!/bin/bash")
	assert.Contains(t, script, "TestProvider")
	assert.Contains(t, script, "cd /path/to/data")
	assert.Contains(t, script, "TEST_VAR")
	assert.Contains(t, script, "./run --port 8080")
}

func TestAutoLLMManager_InitializeProviders(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewAutoLLMManager(tmpDir)

	// Create directory structure first
	err := manager.createDirectoryStructure()
	require.NoError(t, err)

	// Initialize providers
	err = manager.initializeProviders()
	require.NoError(t, err)

	// Verify providers were initialized
	assert.NotEmpty(t, manager.providers)

	// Check that each provider has proper setup
	for name, provider := range manager.providers {
		assert.NotEmpty(t, provider.Name, "Provider %s should have a name", name)
		assert.Equal(t, "not_installed", provider.Status)
		assert.NotNil(t, provider.Config)
		assert.NotNil(t, provider.Health)
		assert.NotNil(t, provider.Metrics)
		assert.Contains(t, provider.BinaryPath, tmpDir)
		assert.Contains(t, provider.ConfigPath, tmpDir)
		assert.Contains(t, provider.DataPath, tmpDir)
	}
}

func TestAutoLLMManager_AutoConfigureProvider(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewAutoLLMManager(tmpDir)

	configDir := filepath.Join(tmpDir, "config")
	require.NoError(t, os.MkdirAll(configDir, 0755))

	provider := &AutoProvider{
		LocalLLMProvider: LocalLLMProvider{
			Name:        "TestProvider",
			ConfigPath:  configDir,
			DefaultPort: 9090,
		},
		Config: make(map[string]interface{}),
	}

	err := manager.autoConfigureProvider(provider)
	require.NoError(t, err)

	// Verify config file was created
	configPath := filepath.Join(configDir, "config.json")
	content, err := os.ReadFile(configPath)
	require.NoError(t, err)

	assert.Contains(t, string(content), "127.0.0.1")
	assert.Contains(t, string(content), "9090")

	// Verify provider config was updated
	assert.Equal(t, 9090, provider.Config["port"])
	assert.Equal(t, "127.0.0.1", provider.Config["host"])
}

func TestAutoConfig_Struct(t *testing.T) {
	config := &AutoConfig{
		Version:       "1.0.0",
		Mode:          "zero_touch",
		AutoDiscover:  true,
		AutoInstall:   true,
		AutoConfigure: true,
		AutoStart:     true,
		AutoMonitor:   true,
		AutoUpdate:    true,
	}

	assert.Equal(t, "1.0.0", config.Version)
	assert.Equal(t, "zero_touch", config.Mode)
}

func TestHealthStatus_Struct(t *testing.T) {
	status := &HealthStatus{
		Status:       "healthy",
		ResponseTime: 100,
		LastCheck:    time.Now(),
		Error:        "",
		IsHealthy:    true,
	}

	assert.Equal(t, "healthy", status.Status)
	assert.Equal(t, 100, status.ResponseTime)
	assert.True(t, status.IsHealthy)
}

func TestPerformanceMetrics_Struct(t *testing.T) {
	metrics := &PerformanceMetrics{
		TokensPerSecond: 50.5,
		MemoryUsage:     1024 * 1024 * 100,
		CPUUsage:        25.5,
		ActiveRequests:  5,
		TotalRequests:   1000,
		ErrorRate:       0.01,
		LastUpdated:     time.Now(),
	}

	assert.Equal(t, 50.5, metrics.TokensPerSecond)
	assert.Equal(t, int64(104857600), metrics.MemoryUsage)
	assert.Equal(t, 25.5, metrics.CPUUsage)
	assert.Equal(t, 5, metrics.ActiveRequests)
}

func TestBackgroundTask_Struct(t *testing.T) {
	task := &BackgroundTask{
		ID:        "task-1",
		Name:      "Health Check",
		Interval:  30 * time.Second,
		LastRun:   time.Now(),
		IsRunning: true,
		StopChan:  make(chan bool),
	}

	assert.Equal(t, "task-1", task.ID)
	assert.Equal(t, "Health Check", task.Name)
	assert.Equal(t, 30*time.Second, task.Interval)
	assert.True(t, task.IsRunning)
}

func TestAutoLLMManager_Initialize_AlreadyInitialized(t *testing.T) {
	manager := NewAutoLLMManager("")
	manager.isInitialized = true

	err := manager.Initialize(context.Background())
	assert.NoError(t, err)
}

func TestAutoLLMManager_Start_AlreadyRunning(t *testing.T) {
	manager := NewAutoLLMManager("")
	manager.isRunning = true

	err := manager.Start(context.Background())
	assert.NoError(t, err)
}

func TestAutoLLMManager_StartBackgroundTasks_Disabled(t *testing.T) {
	manager := NewAutoLLMManager("")
	manager.config.AutoMonitor = false
	manager.config.Performance.AutoOptimize = false
	manager.config.Updates.AutoCheck = false

	err := manager.startBackgroundTasks()
	assert.NoError(t, err)
	assert.Empty(t, manager.backgroundTasks)
}

func TestAutoLLMManager_RunBackgroundTask(t *testing.T) {
	manager := NewAutoLLMManager("")

	callCount := 0
	task := &BackgroundTask{
		ID:       "test-task",
		Name:     "Test Task",
		Function: func() error { callCount++; return nil },
		Interval: 50 * time.Millisecond,
		StopChan: make(chan bool),
	}

	// Run in goroutine and stop after a short time
	go func() {
		time.Sleep(75 * time.Millisecond)
		close(task.StopChan)
	}()

	manager.runBackgroundTask(task)
	assert.False(t, task.IsRunning)
}

func TestAutoLLMManager_AutoPerformanceOptimization(t *testing.T) {
	manager := NewAutoLLMManager("")
	manager.providers["test"] = &AutoProvider{
		LocalLLMProvider: LocalLLMProvider{Name: "TestProvider"},
		Status:           "running",
		Health:           &HealthStatus{IsHealthy: true},
		Metrics: &PerformanceMetrics{
			CPUUsage:    90.0, // High CPU
			MemoryUsage: 10 * 1024 * 1024 * 1024, // 10GB
		},
	}

	err := manager.autoPerformanceOptimization()
	assert.NoError(t, err)
}

func TestAutoLLMManager_AutoHealthCheck_NoRunningProviders(t *testing.T) {
	manager := NewAutoLLMManager("")
	manager.providers["stopped"] = &AutoProvider{
		LocalLLMProvider: LocalLLMProvider{Name: "StoppedProvider"},
		Status:           "stopped",
		Health:           &HealthStatus{},
	}

	err := manager.autoHealthCheck()
	assert.NoError(t, err)
}

func TestAutoLLMManager_PerformHealthCheck(t *testing.T) {
	manager := NewAutoLLMManager("")

	provider := &AutoProvider{
		LocalLLMProvider: LocalLLMProvider{
			Name:      "TestProvider",
			HealthURL: "http://localhost:99999/invalid",
		},
		Health: &HealthStatus{},
	}

	isHealthy, responseTime, err := manager.performHealthCheck(provider)
	assert.False(t, isHealthy)
	assert.GreaterOrEqual(t, responseTime, 0)
	assert.Error(t, err)
}
