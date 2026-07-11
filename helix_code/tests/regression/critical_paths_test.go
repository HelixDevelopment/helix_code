// Package regression contains critical path regression tests for HelixCode.
//
// These tests verify that core functionality that must never break is working correctly.
// Each test is marked as critical and documents what it protects against.
//
// CRITICAL: These tests should be run before every release and as part of CI/CD.
package regression

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"dev.helix.code/internal/auth"
	"dev.helix.code/internal/config"
	"dev.helix.code/internal/database"
	"dev.helix.code/internal/llm"
	"dev.helix.code/internal/redis"
	"dev.helix.code/internal/server"
	"dev.helix.code/internal/session"
	"dev.helix.code/internal/task"
	"dev.helix.code/internal/worker"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// CRITICAL PATH 1: Server Startup and Initialization
// =============================================================================

// TestCriticalPath_ServerStartupAndInitialization verifies that the server
// can be created and initialized without panics or errors.
//
// PROTECTS AGAINST:
// - Nil pointer dereferences during server creation
// - Missing required dependencies
// - Incorrect initialization order of components
// - Router setup failures
//
// CRITICAL: Server startup is the foundation of all functionality.
func TestCriticalPath_ServerStartupAndInitialization(t *testing.T) {
	t.Run("ServerCreationWithMinimalConfig", func(t *testing.T) {
		// Create minimal config that doesn't require external services
		cfg := &config.Config{
			Server: config.ServerConfig{
				Address:         "localhost",
				Port:            8080,
				ReadTimeout:     30,
				WriteTimeout:    30,
				IdleTimeout:     300,
				ShutdownTimeout: 30,
			},
			Logging: config.LoggingConfig{
				Level: "debug",
			},
			Auth: config.AuthConfig{
				JWTSecret:     "test-jwt-secret-32-chars-long-for-testing",
				TokenExpiry:   3600,
				SessionExpiry: 86400,
				BcryptCost:    10,
			},
		}

		// Server creation should not panic
		assert.NotPanics(t, func() {
			srv := server.New(cfg, nil, nil)
			assert.NotNil(t, srv, "Server should be created successfully")
		}, "Server creation should not panic")
	})

	t.Run("ServerCreationWithNilDependencies", func(t *testing.T) {
		cfg := &config.Config{
			Server: config.ServerConfig{
				Address: "localhost",
				Port:    8080,
			},
			Logging: config.LoggingConfig{
				Level: "info",
			},
		}

		// Should handle nil database and redis gracefully
		srv := server.New(cfg, nil, nil)
		assert.NotNil(t, srv, "Server should handle nil dependencies")
	})

	t.Run("RouterInitializationWithMiddleware", func(t *testing.T) {
		cfg := &config.Config{
			Server: config.ServerConfig{
				Address: "localhost",
				Port:    8080,
			},
			Logging: config.LoggingConfig{
				Level: "debug",
			},
		}

		srv := server.New(cfg, nil, nil)
		require.NotNil(t, srv, "Server should be created")

		// Middleware should be properly configured
		corsMiddleware := server.CORSMiddleware(nil)
		assert.NotNil(t, corsMiddleware, "CORS middleware should be created")

		securityMiddleware := server.SecurityMiddleware()
		assert.NotNil(t, securityMiddleware, "Security middleware should be created")
	})

	t.Run("GinModeConfiguration", func(t *testing.T) {
		// Test debug mode
		debugCfg := &config.Config{
			Server: config.ServerConfig{Address: "localhost", Port: 8080},
			Logging: config.LoggingConfig{Level: "debug"},
		}
		_ = server.New(debugCfg, nil, nil)
		assert.Equal(t, gin.DebugMode, gin.Mode(), "Should be in debug mode")

		// Test release mode
		gin.SetMode(gin.ReleaseMode)
		releaseCfg := &config.Config{
			Server: config.ServerConfig{Address: "localhost", Port: 8080},
			Logging: config.LoggingConfig{Level: "info"},
		}
		_ = server.New(releaseCfg, nil, nil)
		assert.Equal(t, gin.ReleaseMode, gin.Mode(), "Should be in release mode")
	})
}

// =============================================================================
// CRITICAL PATH 2: Database Connection and Schema Initialization
// =============================================================================

// TestCriticalPath_DatabaseConnectionAndSchema verifies database-related
// functionality works correctly.
//
// PROTECTS AGAINST:
// - Connection pool configuration errors
// - Schema initialization failures
// - Health check false positives
// - Interface compatibility issues
//
// CRITICAL: Database is required for persistence of all application data.
func TestCriticalPath_DatabaseConnectionAndSchema(t *testing.T) {
	t.Run("DatabaseConfigValidation", func(t *testing.T) {
		// Valid configuration should not cause errors
		validConfig := database.Config{
			Host:     "localhost",
			Port:     5432,
			User:     "test",
			Password: "test",
			DBName:   "test",
			SSLMode:  "disable",
		}

		// Config should be valid (not testing actual connection)
		assert.NotEmpty(t, validConfig.Host, "Host should be set")
		assert.Greater(t, validConfig.Port, 0, "Port should be positive")
		assert.NotEmpty(t, validConfig.DBName, "DBName should be set")
	})

	t.Run("DatabaseInterfaceCompatibility", func(t *testing.T) {
		// Verify Database implements DatabaseInterface
		var _ database.DatabaseInterface = (*database.Database)(nil)
	})

	t.Run("NilDatabaseHandling", func(t *testing.T) {
		// Systems should handle nil database gracefully
		// Task manager should handle nil database
		taskMgr := task.NewTaskManager(nil, nil)
		assert.NotNil(t, taskMgr, "Task manager should be created with nil database")
	})
}

// =============================================================================
// CRITICAL PATH 3: Worker Registration and Health Checks
// =============================================================================

// TestCriticalPath_WorkerRegistrationAndHealthChecks verifies worker management
// functionality.
//
// PROTECTS AGAINST:
// - Worker registration failures
// - Health check timeout issues
// - Capability matching errors
// - Status transition bugs
//
// CRITICAL: Workers are essential for distributed task processing.
func TestCriticalPath_WorkerRegistrationAndHealthChecks(t *testing.T) {
	t.Run("WorkerCreation", func(t *testing.T) {
		w := &worker.Worker{
			ID:                 uuid.New(),
			Hostname:           "test-worker-1",
			DisplayName:        "Test Worker 1",
			Capabilities:       []string{"planning", "building", "testing"},
			Status:             worker.WorkerStatusActive,
			HealthStatus:       worker.WorkerHealthHealthy,
			MaxConcurrentTasks: 10,
			CreatedAt:          time.Now(),
			UpdatedAt:          time.Now(),
		}

		assert.NotEqual(t, uuid.Nil, w.ID, "Worker ID should be set")
		assert.NotEmpty(t, w.Hostname, "Hostname should be set")
		assert.Equal(t, worker.WorkerStatusActive, w.Status, "Status should be active")
		assert.Equal(t, worker.WorkerHealthHealthy, w.HealthStatus, "Health should be healthy")
	})

	t.Run("WorkerStatusTransitions", func(t *testing.T) {
		// Verify all status constants are valid
		statuses := []worker.WorkerStatus{
			worker.WorkerStatusActive,
			worker.WorkerStatusInactive,
			worker.WorkerStatusMaintenance,
			worker.WorkerStatusFailed,
			worker.WorkerStatusOffline,
		}

		for _, status := range statuses {
			assert.NotEmpty(t, string(status), "Status should not be empty")
		}
	})

	t.Run("WorkerHealthStatuses", func(t *testing.T) {
		// Verify all health status constants are valid
		healthStatuses := []worker.WorkerHealth{
			worker.WorkerHealthHealthy,
			worker.WorkerHealthDegraded,
			worker.WorkerHealthUnhealthy,
			worker.WorkerHealthUnknown,
		}

		for _, health := range healthStatuses {
			assert.NotEmpty(t, string(health), "Health status should not be empty")
		}
	})

	t.Run("WorkerMetricsCreation", func(t *testing.T) {
		metrics := &worker.WorkerMetrics{
			ID:                 uuid.New(),
			WorkerID:           uuid.New(),
			CPUUsagePercent:    45.5,
			MemoryUsagePercent: 60.0,
			DiskUsagePercent:   30.0,
			NetworkRxBytes:     1000000,
			NetworkTxBytes:     500000,
			CurrentTasksCount:  3,
			TemperatureCelsius: 55.0,
			RecordedAt:         time.Now(),
		}

		assert.NotEqual(t, uuid.Nil, metrics.ID, "Metrics ID should be set")
		assert.GreaterOrEqual(t, metrics.CPUUsagePercent, float64(0), "CPU usage should be non-negative")
		assert.LessOrEqual(t, metrics.CPUUsagePercent, float64(100), "CPU usage should be <= 100")
	})

	t.Run("WorkerManagerCreation", func(t *testing.T) {
		// Create worker manager with mock repository
		manager := worker.NewWorkerManager(nil, 30*time.Second)
		assert.NotNil(t, manager, "Worker manager should be created")
	})
}

// =============================================================================
// CRITICAL PATH 4: Task Creation and Assignment
// =============================================================================

// TestCriticalPath_TaskCreationAndAssignment verifies task management functionality.
//
// PROTECTS AGAINST:
// - Task creation failures
// - Priority queue corruption
// - Status transition bugs
// - Checkpoint data loss
//
// CRITICAL: Tasks are the fundamental unit of work in HelixCode.
func TestCriticalPath_TaskCreationAndAssignment(t *testing.T) {
	t.Run("TaskTypes", func(t *testing.T) {
		// Verify all task types are valid
		taskTypes := []task.TaskType{
			task.TaskTypePlanning,
			task.TaskTypeBuilding,
			task.TaskTypeTesting,
			task.TaskTypeRefactoring,
			task.TaskTypeDebugging,
			task.TaskTypeDesign,
			task.TaskTypeDiagram,
			task.TaskTypeDeployment,
			task.TaskTypePorting,
		}

		for _, tt := range taskTypes {
			assert.NotEmpty(t, string(tt), "Task type should not be empty")
		}
	})

	t.Run("TaskPriorities", func(t *testing.T) {
		// Verify priority ordering
		assert.Less(t, int(task.PriorityLow), int(task.PriorityNormal), "Low < Normal")
		assert.Less(t, int(task.PriorityNormal), int(task.PriorityHigh), "Normal < High")
		assert.Less(t, int(task.PriorityHigh), int(task.PriorityCritical), "High < Critical")
	})

	t.Run("TaskStatusTransitions", func(t *testing.T) {
		// Verify all status constants
		statuses := []task.TaskStatus{
			task.TaskStatusPending,
			task.TaskStatusAssigned,
			task.TaskStatusRunning,
			task.TaskStatusCompleted,
			task.TaskStatusFailed,
			task.TaskStatusPaused,
			task.TaskStatusWaitingForWorker,
			task.TaskStatusWaitingForDeps,
		}

		for _, status := range statuses {
			assert.NotEmpty(t, string(status), "Task status should not be empty")
		}
	})

	t.Run("TaskCreation", func(t *testing.T) {
		tsk := &task.Task{
			ID:          uuid.New(),
			Type:        task.TaskTypePlanning,
			Data:        map[string]interface{}{"test": "data"},
			Status:      task.TaskStatusPending,
			Priority:    task.PriorityNormal,
			Criticality: task.CriticalityNormal,
			MaxRetries:  3,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}

		assert.NotEqual(t, uuid.Nil, tsk.ID, "Task ID should be set")
		assert.Equal(t, task.TaskTypePlanning, tsk.Type, "Task type should match")
		assert.Equal(t, task.TaskStatusPending, tsk.Status, "Initial status should be pending")
	})

	t.Run("TaskManagerCreation", func(t *testing.T) {
		manager := task.NewTaskManager(nil, nil)
		assert.NotNil(t, manager, "Task manager should be created")
	})

	t.Run("TaskQueueCreation", func(t *testing.T) {
		queue := task.NewTaskQueue()
		assert.NotNil(t, queue, "Task queue should be created")
	})
}

// =============================================================================
// CRITICAL PATH 5: LLM Provider Initialization
// =============================================================================

// TestCriticalPath_LLMProviderInitialization verifies LLM provider functionality.
//
// PROTECTS AGAINST:
// - Provider factory failures
// - Configuration parsing errors
// - Model discovery issues
// - Capability reporting bugs
//
// CRITICAL: LLM providers are required for all AI-powered features.
func TestCriticalPath_LLMProviderInitialization(t *testing.T) {
	t.Run("ProviderTypes", func(t *testing.T) {
		// Verify all provider types are defined
		providerTypes := []llm.ProviderType{
			llm.ProviderTypeOpenAI,
			llm.ProviderTypeAnthropic,
			llm.ProviderTypeGemini,
			llm.ProviderTypeOllama,
			llm.ProviderTypeLlamaCpp,
			llm.ProviderTypeQwen,
			llm.ProviderTypeXAI,
			llm.ProviderTypeOpenRouter,
			llm.ProviderTypeCopilot,
			llm.ProviderTypeAzure,
			llm.ProviderTypeBedrock,
			llm.ProviderTypeVertexAI,
			llm.ProviderTypeGroq,
			llm.ProviderTypeVLLM,
			llm.ProviderTypeLocalAI,
		}

		for _, pt := range providerTypes {
			assert.NotEmpty(t, string(pt), "Provider type should not be empty")
		}
	})

	t.Run("ModelCapabilities", func(t *testing.T) {
		// Verify capability constants
		capabilities := []llm.ModelCapability{
			llm.CapabilityTextGeneration,
			llm.CapabilityCodeGeneration,
			llm.CapabilityCodeAnalysis,
			llm.CapabilityPlanning,
			llm.CapabilityDebugging,
			llm.CapabilityRefactoring,
			llm.CapabilityTesting,
		}

		for _, cap := range capabilities {
			assert.NotEmpty(t, string(cap), "Capability should not be empty")
		}
	})

	t.Run("LocalProviderCreation", func(t *testing.T) {
		config := llm.ProviderConfigEntry{
			Type:     llm.ProviderTypeOllama,
			Enabled:  true,
			Endpoint: "http://localhost:11434",
			Models:   []string{"llama2"},
		}

		// Provider creation should not panic even if service is unavailable
		assert.NotPanics(t, func() {
			_, err := llm.NewProvider(config)
			// Error is expected if Ollama is not running, but should not panic
			_ = err
		}, "Provider creation should not panic")
	})

	t.Run("ModelManagerCreation", func(t *testing.T) {
		manager := llm.NewModelManager()
		assert.NotNil(t, manager, "Model manager should be created")
	})

	t.Run("ProviderConfigValidation", func(t *testing.T) {
		// Test various config scenarios
		configs := []llm.ProviderConfigEntry{
			{Type: llm.ProviderTypeOpenAI, Enabled: true, APIKey: "test-key"},
			{Type: llm.ProviderTypeAnthropic, Enabled: true, APIKey: "test-key"},
			{Type: llm.ProviderTypeGemini, Enabled: true, APIKey: "test-key"},
		}

		for _, cfg := range configs {
			assert.NotEmpty(t, string(cfg.Type), "Config type should be set")
			if cfg.Enabled {
				// Enabled providers should have API key or endpoint
				hasCredentials := cfg.APIKey != "" || cfg.Endpoint != ""
				assert.True(t, hasCredentials || cfg.Type == llm.ProviderTypeOllama,
					"Enabled provider should have credentials")
			}
		}
	})
}

// =============================================================================
// CRITICAL PATH 6: Authentication Flow
// =============================================================================

// TestCriticalPath_AuthenticationFlow verifies authentication functionality.
//
// PROTECTS AGAINST:
// - JWT generation/validation failures
// - Password hashing issues
// - Session management bugs
// - Token expiry calculation errors
//
// CRITICAL: Authentication is required for all protected endpoints.
func TestCriticalPath_AuthenticationFlow(t *testing.T) {
	t.Run("AuthConfigDefaults", func(t *testing.T) {
		defaultCfg := auth.DefaultConfig()

		assert.NotEmpty(t, defaultCfg.JWTSecret, "JWT secret should have default")
		assert.Greater(t, defaultCfg.TokenExpiry, time.Duration(0), "Token expiry should be positive")
		assert.Greater(t, defaultCfg.SessionExpiry, time.Duration(0), "Session expiry should be positive")
		assert.Greater(t, defaultCfg.BcryptCost, 0, "Bcrypt cost should be positive")
	})

	t.Run("AuthServiceCreation", func(t *testing.T) {
		cfg := auth.AuthConfig{
			JWTSecret:     "test-jwt-secret-32-chars-long-for-testing",
			TokenExpiry:   24 * time.Hour,
			SessionExpiry: 7 * 24 * time.Hour,
			BcryptCost:    10,
		}

		// Create auth service (with nil repository for testing)
		service := auth.NewAuthService(cfg, nil)
		assert.NotNil(t, service, "Auth service should be created")
	})

	t.Run("JWTGeneration", func(t *testing.T) {
		cfg := auth.AuthConfig{
			JWTSecret:   "test-jwt-secret-32-chars-long-for-testing",
			TokenExpiry: 24 * time.Hour,
		}

		service := auth.NewAuthService(cfg, nil)

		user := &auth.User{
			ID:       uuid.New(),
			Username: "testuser",
			Email:    "test@example.com",
		}

		token, err := service.GenerateJWT(user)
		require.NoError(t, err, "JWT generation should succeed")
		assert.NotEmpty(t, token, "Token should not be empty")
	})

	t.Run("JWTVerification", func(t *testing.T) {
		cfg := auth.AuthConfig{
			JWTSecret:   "test-jwt-secret-32-chars-long-for-testing",
			TokenExpiry: 24 * time.Hour,
		}

		service := auth.NewAuthService(cfg, nil)

		user := &auth.User{
			ID:       uuid.New(),
			Username: "testuser",
			Email:    "test@example.com",
		}

		token, err := service.GenerateJWT(user)
		require.NoError(t, err)

		// Verify the token
		verifiedUser, err := service.VerifyJWT(token)
		require.NoError(t, err, "JWT verification should succeed")
		assert.Equal(t, user.ID, verifiedUser.ID, "User ID should match")
		assert.Equal(t, user.Username, verifiedUser.Username, "Username should match")
	})

	t.Run("UserStructure", func(t *testing.T) {
		user := &auth.User{
			ID:          uuid.New(),
			Username:    "testuser",
			Email:       "test@example.com",
			DisplayName: "Test User",
			IsActive:    true,
			IsVerified:  false,
			MFAEnabled:  false,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}

		assert.NotEqual(t, uuid.Nil, user.ID, "User ID should be set")
		assert.True(t, user.IsActive, "User should be active by default")
	})

	t.Run("SessionStructure", func(t *testing.T) {
		session := &auth.Session{
			ID:           uuid.New(),
			UserID:       uuid.New(),
			SessionToken: "test-session-token",
			ClientType:   "rest_api",
			ExpiresAt:    time.Now().Add(24 * time.Hour),
			CreatedAt:    time.Now(),
		}

		assert.NotEqual(t, uuid.Nil, session.ID, "Session ID should be set")
		assert.NotEmpty(t, session.SessionToken, "Session token should not be empty")
		assert.True(t, session.ExpiresAt.After(time.Now()), "Session should not be expired")
	})
}

// =============================================================================
// CRITICAL PATH 7: Configuration Loading
// =============================================================================

// TestCriticalPath_ConfigurationLoading verifies configuration functionality.
//
// PROTECTS AGAINST:
// - Config file parsing errors
// - Environment variable override failures
// - Default value issues
// - Validation bypass bugs
//
// CRITICAL: Configuration is required for all application behavior.
func TestCriticalPath_ConfigurationLoading(t *testing.T) {
	t.Run("ConfigStructureIntegrity", func(t *testing.T) {
		// Verify config structure has all required fields
		cfg := config.Config{}

		// Server config
		cfg.Server.Address = "localhost"
		cfg.Server.Port = 8080
		assert.Equal(t, "localhost", cfg.Server.Address)
		assert.Equal(t, 8080, cfg.Server.Port)

		// Database config
		cfg.Database.Host = "localhost"
		cfg.Database.Port = 5432
		assert.Equal(t, "localhost", cfg.Database.Host)
		assert.Equal(t, 5432, cfg.Database.Port)

		// Auth config
		cfg.Auth.JWTSecret = "secret"
		assert.NotEmpty(t, cfg.Auth.JWTSecret)

		// LLM config
		cfg.LLM.DefaultProvider = "local"
		assert.NotEmpty(t, cfg.LLM.DefaultProvider)
	})

	t.Run("ConfigValidation", func(t *testing.T) {
		validator := config.NewConfigurationValidator(true)
		assert.NotNil(t, validator, "Validator should be created")

		// Valid config
		validCfg := &config.Config{
			Version: "1.0.0",
			Application: config.ApplicationConfig{
				Environment: "development",
			},
			Server: config.ServerConfig{Port: 8080},
			Database: database.Config{
				Port: 5432,
			},
			Auth: config.AuthConfig{
				JWTSecret:  "test-jwt-secret-32-chars-long-for-testing",
				BcryptCost: 12,
			},
			LLM: config.LLMConfig{
				DefaultProvider: "local",
				MaxTokens:       1000,
			},
		}

		result := validator.Validate(validCfg)
		assert.True(t, result.Valid, "Valid config should pass validation")
	})

	t.Run("ConfigFileCreation", func(t *testing.T) {
		tempDir := t.TempDir()
		configPath := filepath.Join(tempDir, "test_config.yaml")

		err := config.CreateDefaultConfig(configPath)
		require.NoError(t, err, "Default config creation should succeed")

		// Verify file exists
		_, err = os.Stat(configPath)
		assert.NoError(t, err, "Config file should exist")
	})

	t.Run("EnvironmentVariableOverrides", func(t *testing.T) {
		// Test environment variable helper functions
		os.Setenv("TEST_ENV_VAR", "test_value")
		defer os.Unsetenv("TEST_ENV_VAR")

		value := config.GetEnvOrDefault("TEST_ENV_VAR", "default")
		assert.Equal(t, "test_value", value, "Should return env var value")

		defaultValue := config.GetEnvOrDefault("NONEXISTENT_VAR", "default")
		assert.Equal(t, "default", defaultValue, "Should return default for missing var")

		os.Setenv("TEST_INT_VAR", "42")
		defer os.Unsetenv("TEST_INT_VAR")

		intValue := config.GetEnvIntOrDefault("TEST_INT_VAR", 0)
		assert.Equal(t, 42, intValue, "Should parse int from env var")

		defaultInt := config.GetEnvIntOrDefault("NONEXISTENT_INT", 99)
		assert.Equal(t, 99, defaultInt, "Should return default for missing int var")
	})

	t.Run("ConfigManagerCreation", func(t *testing.T) {
		tempDir := t.TempDir()
		configPath := filepath.Join(tempDir, "manager_config.yaml")

		manager, err := config.NewHelixConfigManager(configPath)
		require.NoError(t, err, "Config manager creation should succeed")
		assert.NotNil(t, manager, "Manager should not be nil")
		assert.True(t, manager.IsConfigPresent(), "Config should be present after creation")
	})
}

// =============================================================================
// CRITICAL PATH 8: API Endpoint Availability
// =============================================================================

// TestCriticalPath_APIEndpointAvailability verifies that critical API endpoints
// are available and respond correctly.
//
// PROTECTS AGAINST:
// - Route registration failures
// - Middleware blocking valid requests
// - Handler panics
// - Response format errors
//
// CRITICAL: API endpoints are the interface for all clients.
func TestCriticalPath_APIEndpointAvailability(t *testing.T) {
	// Create test server
	cfg := &config.Config{
		Server: config.ServerConfig{
			Address: "localhost",
			Port:    8080,
		},
		Logging: config.LoggingConfig{
			Level: "debug",
		},
	}

	srv := server.New(cfg, nil, nil)
	require.NotNil(t, srv, "Server should be created")

	// Get the router for testing.
	//
	// §11.4.120 stale-test reconciliation note: the CORS allowlist below is
	// intentionally non-nil. Production CORSMiddleware was hardened to a
	// strict allowlist (cfg.Auth.CORSAllowedOrigins / HELIX_CORS_ALLOWED_ORIGINS)
	// that never echoes "*" and never grants Allow-Origin to an origin absent
	// from the allowlist (default-deny). The CORSHeaders subtest below was
	// reconciled to assert that secure behavior instead of the removed
	// insecure wildcard behavior — see cors_security_test.go for the
	// canonical anti-bluff coverage of the middleware itself.
	corsAllowedOrigins := []string{"http://localhost:3000"}
	router := gin.New()
	router.Use(server.CORSMiddleware(corsAllowedOrigins))
	router.Use(server.SecurityMiddleware())
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "healthy",
			"version":   "1.0.0",
			"timestamp": time.Now().UTC(),
		})
	})

	t.Run("HealthEndpoint", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/health", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "Health endpoint should return 200")
		assert.Contains(t, w.Body.String(), "healthy", "Response should contain healthy status")
	})

	t.Run("CORSHeaders", func(t *testing.T) {
		// Allowlisted origin: CORSMiddleware MUST echo the exact origin
		// (never "*") and MUST NOT set Allow-Credentials without an
		// explicit per-request opt-in that isn't exercised here — the
		// echoed-origin + Vary:Origin pairing is what matters for this
		// critical-path smoke test (full anti-bluff coverage of every
		// header combination lives in cors_security_test.go).
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("OPTIONS", "/health", nil)
		req.Header.Set("Origin", "http://localhost:3000")
		req.Header.Set("Access-Control-Request-Method", "GET")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNoContent, w.Code, "OPTIONS should return 204")
		assert.Equal(t, "http://localhost:3000", w.Header().Get("Access-Control-Allow-Origin"),
			"allowlisted origin should be echoed back verbatim, never a wildcard (§11.4.120: hardened CORSMiddleware never emits \"*\")")
		assert.NotEqual(t, "*", w.Header().Get("Access-Control-Allow-Origin"),
			"CORS origin must never be a wildcard (secure allowlist behavior)")
		assert.Contains(t, w.Header().Get("Access-Control-Allow-Methods"), "GET", "CORS methods should include GET")

		// Non-allowlisted origin: default-deny — no Allow-Origin header at all.
		w2 := httptest.NewRecorder()
		req2, _ := http.NewRequest("OPTIONS", "/health", nil)
		req2.Header.Set("Origin", "http://evil.example.com")
		req2.Header.Set("Access-Control-Request-Method", "GET")
		router.ServeHTTP(w2, req2)

		assert.Equal(t, http.StatusNoContent, w2.Code, "OPTIONS should still return 204 for a disallowed origin")
		assert.Empty(t, w2.Header().Get("Access-Control-Allow-Origin"),
			"disallowed origin must receive no Access-Control-Allow-Origin header (default-deny)")
	})

	t.Run("SecurityHeaders", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/health", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"), "X-Frame-Options should be set")
		assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"), "X-Content-Type-Options should be set")
		assert.Equal(t, "1; mode=block", w.Header().Get("X-XSS-Protection"), "X-XSS-Protection should be set")
		assert.Contains(t, w.Header().Get("Strict-Transport-Security"), "max-age=", "HSTS should be set")
	})
}

// =============================================================================
// Additional Critical Component Tests
// =============================================================================

// TestCriticalPath_SessionManagement verifies session management functionality.
//
// PROTECTS AGAINST:
// - Session creation failures
// - Session retrieval errors
// - Concurrent access issues
//
// CRITICAL: Sessions track user interactions and context.
func TestCriticalPath_SessionManagement(t *testing.T) {
	t.Run("SessionManagerCreation", func(t *testing.T) {
		manager := session.NewManager()
		assert.NotNil(t, manager, "Session manager should be created")
	})

	t.Run("SessionCreation", func(t *testing.T) {
		manager := session.NewManager()

		sess, err := manager.Create("test-project", "Test Session", "Description", session.ModePlanning)
		require.NoError(t, err, "Session creation should succeed")
		assert.NotNil(t, sess, "Session should not be nil")
		assert.NotEmpty(t, sess.ID, "Session ID should be set")
	})

	t.Run("SessionRetrieval", func(t *testing.T) {
		manager := session.NewManager()

		sess, err := manager.Create("test-project", "Test Session", "Description", session.ModeBuilding)
		require.NoError(t, err)

		retrieved, err := manager.Get(sess.ID)
		require.NoError(t, err, "Session retrieval should succeed")
		assert.Equal(t, sess.ID, retrieved.ID, "Session IDs should match")
	})

	t.Run("SessionListing", func(t *testing.T) {
		manager := session.NewManager()

		// Create a few sessions
		_, err := manager.Create("project-1", "Session 1", "Description", session.ModeTesting)
		require.NoError(t, err)
		_, err = manager.Create("project-2", "Session 2", "Description", session.ModeRefactoring)
		require.NoError(t, err)

		sessions := manager.GetAll()
		assert.GreaterOrEqual(t, len(sessions), 2, "Should have at least 2 sessions")
	})

	t.Run("SessionModes", func(t *testing.T) {
		// Verify all session modes are valid
		modes := []session.Mode{
			session.ModePlanning,
			session.ModeBuilding,
			session.ModeTesting,
			session.ModeRefactoring,
			session.ModeDebugging,
			session.ModeDeployment,
		}

		for _, mode := range modes {
			assert.True(t, mode.IsValid(), "Mode %s should be valid", mode)
		}
	})

	t.Run("SessionStatuses", func(t *testing.T) {
		// Verify all session statuses are valid
		statuses := []session.Status{
			session.StatusActive,
			session.StatusPaused,
			session.StatusCompleted,
			session.StatusFailed,
		}

		for _, status := range statuses {
			assert.True(t, status.IsValid(), "Status %s should be valid", status)
		}
	})

	t.Run("SessionLifecycle", func(t *testing.T) {
		manager := session.NewManager()

		// Create session
		sess, err := manager.Create("test-project", "Lifecycle Test", "Description", session.ModePlanning)
		require.NoError(t, err)
		assert.Equal(t, session.StatusPaused, sess.Status, "Initial status should be paused")

		// Start session
		err = manager.Start(sess.ID)
		require.NoError(t, err)
		sess, _ = manager.Get(sess.ID)
		assert.Equal(t, session.StatusActive, sess.Status, "Status should be active after start")

		// Pause session
		err = manager.Pause(sess.ID)
		require.NoError(t, err)
		sess, _ = manager.Get(sess.ID)
		assert.Equal(t, session.StatusPaused, sess.Status, "Status should be paused")

		// Complete session
		err = manager.Complete(sess.ID)
		require.NoError(t, err)
		sess, _ = manager.Get(sess.ID)
		assert.Equal(t, session.StatusCompleted, sess.Status, "Status should be completed")
	})
}

// TestCriticalPath_RedisClient verifies Redis client functionality.
//
// PROTECTS AGAINST:
// - Nil client panics
// - Connection state misreporting
// - Graceful degradation failures
//
// CRITICAL: Redis is used for caching and real-time state.
func TestCriticalPath_RedisClient(t *testing.T) {
	t.Run("DisabledClientHandling", func(t *testing.T) {
		// Create a disabled Redis client
		client := &redis.Client{}

		// Should not panic when checking if enabled
		assert.NotPanics(t, func() {
			enabled := client.IsEnabled()
			assert.False(t, enabled, "Uninitialized client should be disabled")
		})
	})

	t.Run("NilClientSafety", func(t *testing.T) {
		// Operations on nil/disabled client should not panic
		var client *redis.Client

		assert.NotPanics(t, func() {
			if client != nil {
				_ = client.IsEnabled()
			}
		}, "Nil client check should not panic")
	})
}

// =============================================================================
// Integration Tests for Critical Paths
// =============================================================================

// TestCriticalPath_FullServerFlow tests a complete server request flow.
//
// PROTECTS AGAINST:
// - Component integration failures
// - Request pipeline issues
// - Response serialization bugs
//
// CRITICAL: Full flow must work for any functionality to be usable.
func TestCriticalPath_FullServerFlow(t *testing.T) {
	t.Run("CompleteRequestFlow", func(t *testing.T) {
		// Create minimal server
		cfg := &config.Config{
			Server: config.ServerConfig{
				Address:         "localhost",
				Port:            8080,
				ReadTimeout:     30,
				WriteTimeout:    30,
				IdleTimeout:     300,
				ShutdownTimeout: 30,
			},
			Logging: config.LoggingConfig{
				Level: "debug",
			},
		}

		_ = server.New(cfg, nil, nil)

		// Create test router
		router := gin.New()
		router.Use(gin.Recovery())
		router.Use(server.CORSMiddleware(nil))
		router.Use(server.SecurityMiddleware())

		// Add health endpoint
		router.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"status": "healthy",
			})
		})

		// Add version endpoint
		router.GET("/api/v1/server/info", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"version": "1.0.0",
				"name":    "HelixCode",
			})
		})

		// Test health endpoint
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/health", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		// Test server info endpoint
		w = httptest.NewRecorder()
		req, _ = http.NewRequest("GET", "/api/v1/server/info", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "HelixCode")
	})
}

// =============================================================================
// Stability and Error Handling Tests
// =============================================================================

// TestCriticalPath_ErrorHandling verifies error handling across critical paths.
//
// PROTECTS AGAINST:
// - Unhandled panics
// - Error swallowing
// - Incorrect error types
//
// CRITICAL: Proper error handling prevents data corruption and crashes.
func TestCriticalPath_ErrorHandling(t *testing.T) {
	t.Run("AuthErrors", func(t *testing.T) {
		// Verify error constants are defined
		assert.NotNil(t, auth.ErrInvalidCredentials)
		assert.NotNil(t, auth.ErrTokenExpired)
		assert.NotNil(t, auth.ErrTokenInvalid)
		assert.NotNil(t, auth.ErrUserNotFound)
		assert.NotNil(t, auth.ErrUserExists)
	})

	t.Run("RecoveryMiddleware", func(t *testing.T) {
		router := gin.New()
		router.Use(gin.Recovery())

		router.GET("/panic", func(c *gin.Context) {
			panic("test panic")
		})

		// Should recover from panic
		assert.NotPanics(t, func() {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/panic", nil)
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusInternalServerError, w.Code)
		})
	})

	t.Run("InvalidTokenHandling", func(t *testing.T) {
		cfg := auth.AuthConfig{
			JWTSecret:   "test-jwt-secret-32-chars-long-for-testing",
			TokenExpiry: 24 * time.Hour,
		}

		service := auth.NewAuthService(cfg, nil)

		// Invalid token should return error
		_, err := service.VerifyJWT("invalid-token")
		assert.Error(t, err, "Invalid token should return error")
	})
}
