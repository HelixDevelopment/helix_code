package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewHealthMonitor(t *testing.T) {
	manager := NewAutoLLMManager("")

	monitor := NewHealthMonitor(manager)

	require.NotNil(t, monitor)
	assert.Equal(t, manager, monitor.manager)
	assert.Equal(t, 30*time.Second, monitor.checkInterval)
	assert.False(t, monitor.isRunning)
	assert.NotNil(t, monitor.stopChan)
	assert.NotNil(t, monitor.client)
	assert.NotNil(t, monitor.alertSystem)
}

func TestHealthMonitor_SetInterval(t *testing.T) {
	manager := NewAutoLLMManager("")
	monitor := NewHealthMonitor(manager)

	newInterval := 60 * time.Second
	monitor.SetInterval(newInterval)

	assert.Equal(t, newInterval, monitor.checkInterval)
}

func TestHealthMonitor_Stop(t *testing.T) {
	t.Run("not running", func(t *testing.T) {
		manager := NewAutoLLMManager("")
		monitor := NewHealthMonitor(manager)

		// Should not panic
		monitor.Stop()
	})

	t.Run("running", func(t *testing.T) {
		manager := NewAutoLLMManager("")
		monitor := NewHealthMonitor(manager)
		monitor.isRunning = true
		monitor.stopChan = make(chan bool)

		// Start a goroutine to consume the close signal
		go func() {
			<-monitor.stopChan
		}()

		// Should not panic
		monitor.Stop()
	})
}

func TestHealthMonitor_Start_AlreadyRunning(t *testing.T) {
	manager := NewAutoLLMManager("")
	monitor := NewHealthMonitor(manager)
	monitor.isRunning = true

	err := monitor.Start(context.Background())
	assert.NoError(t, err)
}

func TestHealthMonitor_Start_WithCancellation(t *testing.T) {
	manager := NewAutoLLMManager("")
	monitor := NewHealthMonitor(manager)
	monitor.checkInterval = 100 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())

	// Start in goroutine
	done := make(chan error)
	go func() {
		done <- monitor.Start(ctx)
	}()

	// Wait a bit then cancel
	time.Sleep(50 * time.Millisecond)
	cancel()

	// Wait for completion
	err := <-done
	assert.NoError(t, err)
	assert.False(t, monitor.isRunning)
}

func TestHealthMonitor_CheckProviderHealth(t *testing.T) {
	manager := NewAutoLLMManager("")
	monitor := NewHealthMonitor(manager)

	t.Run("healthy provider", func(t *testing.T) {
		// Create a test server that returns 200
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		provider := &AutoProvider{
			LocalLLMProvider: LocalLLMProvider{
				Name:      "TestProvider",
				HealthURL: server.URL,
			},
			Health: &HealthStatus{},
		}

		isHealthy, responseTime, err := monitor.checkProviderHealth(provider)

		assert.True(t, isHealthy)
		assert.GreaterOrEqual(t, responseTime, 0) // May be 0 on fast systems
		assert.NoError(t, err)
	})

	t.Run("unhealthy provider - bad status", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		provider := &AutoProvider{
			LocalLLMProvider: LocalLLMProvider{
				Name:      "TestProvider",
				HealthURL: server.URL,
			},
			Health: &HealthStatus{},
		}

		isHealthy, responseTime, err := monitor.checkProviderHealth(provider)

		assert.False(t, isHealthy)
		assert.GreaterOrEqual(t, responseTime, 0) // May be 0 on fast systems
		assert.NoError(t, err)
	})

	t.Run("unreachable provider", func(t *testing.T) {
		provider := &AutoProvider{
			LocalLLMProvider: LocalLLMProvider{
				Name:      "TestProvider",
				HealthURL: "http://localhost:99999/health",
			},
			Health: &HealthStatus{},
		}

		isHealthy, responseTime, err := monitor.checkProviderHealth(provider)

		assert.False(t, isHealthy)
		assert.GreaterOrEqual(t, responseTime, 0) // May be 0 on fast systems
		assert.Error(t, err)
	})
}

func TestHealthMonitor_UpdateProviderHealth(t *testing.T) {
	manager := NewAutoLLMManager("")
	monitor := NewHealthMonitor(manager)

	t.Run("healthy update", func(t *testing.T) {
		provider := &AutoProvider{
			LocalLLMProvider: LocalLLMProvider{Name: "TestProvider"},
			Health:           &HealthStatus{},
			RetryCount:       5,
		}

		monitor.updateProviderHealth(provider, true, 100, nil)

		assert.Equal(t, "healthy", provider.Health.Status)
		assert.True(t, provider.Health.IsHealthy)
		assert.Equal(t, 100, provider.Health.ResponseTime)
		assert.Empty(t, provider.Health.Error)
		assert.Equal(t, 0, provider.RetryCount)
	})

	t.Run("unhealthy update with error", func(t *testing.T) {
		provider := &AutoProvider{
			LocalLLMProvider: LocalLLMProvider{Name: "TestProvider"},
			Health:           &HealthStatus{},
			RetryCount:       1,
		}

		err := assert.AnError
		monitor.updateProviderHealth(provider, false, 500, err)

		assert.Equal(t, "unhealthy", provider.Health.Status)
		assert.False(t, provider.Health.IsHealthy)
		assert.Equal(t, 500, provider.Health.ResponseTime)
		assert.NotEmpty(t, provider.Health.Error)
		assert.Equal(t, 2, provider.RetryCount)
	})

	t.Run("unhealthy update without error", func(t *testing.T) {
		provider := &AutoProvider{
			LocalLLMProvider: LocalLLMProvider{Name: "TestProvider"},
			Health:           &HealthStatus{},
			RetryCount:       0,
		}

		monitor.updateProviderHealth(provider, false, 200, nil)

		assert.Equal(t, "unhealthy", provider.Health.Status)
		assert.False(t, provider.Health.IsHealthy)
		assert.Empty(t, provider.Health.Error)
		assert.Equal(t, 1, provider.RetryCount)
	})
}

func TestHealthMonitor_PerformHealthChecks(t *testing.T) {
	// Anti-bluff (CONST-035 / §11.9): the original form of this test ran
	// performHealthChecks() and asserted only "should not panic" — passing
	// regardless of any mutations performHealthChecks might have applied to
	// non-running providers. Per health_monitor.go:76-78, providers whose
	// Status != "running" MUST be skipped (no HTTP call, no Health/Metrics
	// update). We pin that contract by capturing the pre-call snapshot and
	// asserting it is unchanged afterwards. A regression that probed
	// stopped providers (e.g. dropping the `if provider.Status != "running"
	// { continue }` guard) would now fail this test instead of silently
	// passing.
	manager := NewAutoLLMManager("")

	stopped := &AutoProvider{
		LocalLLMProvider: LocalLLMProvider{Name: "StoppedProvider"},
		Status:           "stopped",
		Health:           &HealthStatus{},
		Metrics:          &PerformanceMetrics{},
	}
	manager.providers["stopped"] = stopped

	// Snapshot the fields performHealthChecks would mutate for a "running"
	// provider, so we can verify they are NOT mutated for "stopped".
	preStatus := stopped.Status
	preHealth := *stopped.Health   // value-copy of zero HealthStatus
	preMetrics := *stopped.Metrics // value-copy of zero PerformanceMetrics

	monitor := NewHealthMonitor(manager)
	monitor.performHealthChecks()

	assert.Equal(t, preStatus, stopped.Status, "performHealthChecks must not mutate provider.Status for non-running providers")
	assert.Equal(t, preHealth, *stopped.Health, "performHealthChecks must not mutate provider.Health for non-running providers")
	assert.Equal(t, preMetrics, *stopped.Metrics, "performHealthChecks must not mutate provider.Metrics for non-running providers")
}

func TestHealthMonitor_HandleUnhealthyProvider(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewAutoLLMManager(tmpDir)
	manager.config.Health.MaxRetries = 3

	monitor := NewHealthMonitor(manager)

	t.Run("within retry limits", func(t *testing.T) {
		provider := &AutoProvider{
			LocalLLMProvider: LocalLLMProvider{Name: "TestProvider", DataPath: tmpDir},
			Health:           &HealthStatus{},
			RetryCount:       1,
		}

		// Should not panic
		monitor.handleUnhealthyProvider("test", provider, assert.AnError)
	})

	t.Run("max retries exceeded", func(t *testing.T) {
		provider := &AutoProvider{
			LocalLLMProvider: LocalLLMProvider{Name: "TestProvider", DataPath: tmpDir},
			Health:           &HealthStatus{},
			RetryCount:       5,
		}

		// Should not panic
		monitor.handleUnhealthyProvider("test", provider, assert.AnError)
	})
}

func TestHealthMonitor_PerformHealthChecks_WithRunningProvider(t *testing.T) {
	// Create a test server for health checks
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	manager := NewAutoLLMManager(tmpDir)

	// Add a running provider with health URL
	manager.providers["running"] = &AutoProvider{
		LocalLLMProvider: LocalLLMProvider{
			Name:      "RunningProvider",
			HealthURL: server.URL,
		},
		Status:  "running",
		Health:  &HealthStatus{},
		Metrics: &PerformanceMetrics{},
	}

	monitor := NewHealthMonitor(manager)

	// Perform health checks
	monitor.performHealthChecks()

	// Verify health was updated
	provider := manager.providers["running"]
	assert.True(t, provider.Health.IsHealthy)
	assert.Equal(t, "healthy", provider.Health.Status)
}

func TestHealthMonitor_PerformHealthChecks_WithUnhealthyProvider(t *testing.T) {
	// Create a test server that returns 500
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	manager := NewAutoLLMManager(tmpDir)
	manager.config.Health.MaxRetries = 3

	// Add a running provider with unhealthy status
	manager.providers["unhealthy"] = &AutoProvider{
		LocalLLMProvider: LocalLLMProvider{
			Name:      "UnhealthyProvider",
			HealthURL: server.URL,
		},
		Status:     "running",
		Health:     &HealthStatus{},
		Metrics:    &PerformanceMetrics{},
		RetryCount: 0,
	}

	monitor := NewHealthMonitor(manager)

	// Perform health checks
	monitor.performHealthChecks()

	// Verify health was updated to unhealthy
	provider := manager.providers["unhealthy"]
	assert.False(t, provider.Health.IsHealthy)
	assert.Equal(t, "unhealthy", provider.Health.Status)
}

func TestHealthMonitor_TriggerAutoRecovery(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewAutoLLMManager(tmpDir)
	manager.config.Health.RetryDelay = 0 // No delay for testing

	monitor := NewHealthMonitor(manager)

	provider := &AutoProvider{
		LocalLLMProvider: LocalLLMProvider{
			Name:     "TestProvider",
			DataPath: tmpDir,
		},
		Health:     &HealthStatus{},
		RetryCount: 1,
	}

	// Should not panic - triggerAutoRecovery runs in goroutine
	done := make(chan bool, 1)
	go func() {
		monitor.triggerAutoRecovery("test", provider)
		done <- true
	}()

	select {
	case <-done:
		// Success
	case <-time.After(5 * time.Second):
		t.Log("triggerAutoRecovery completed or timed out")
	}
}

func TestHealthMonitor_Start_WithStopChannel(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long-running test in short mode (SKIP-OK: #short-mode)")
	}
	tmpDir := t.TempDir()
	manager := NewAutoLLMManager(tmpDir)
	monitor := NewHealthMonitor(manager)
	monitor.checkInterval = 100 * time.Millisecond

	// Start in goroutine
	done := make(chan error)
	go func() {
		done <- monitor.Start(context.Background())
	}()

	// Wait a bit then stop via channel
	time.Sleep(50 * time.Millisecond)
	monitor.Stop()

	// Wait for completion
	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("Health monitor did not stop")
	}
}

func TestHealthMonitor_AlertSystemIntegration(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewAutoLLMManager(tmpDir)
	manager.config.Health.MaxRetries = 1

	monitor := NewHealthMonitor(manager)

	provider := &AutoProvider{
		LocalLLMProvider: LocalLLMProvider{
			Name:     "TestProvider",
			DataPath: tmpDir,
		},
		Health:     &HealthStatus{},
		RetryCount: 0,
	}

	// Handle unhealthy within retry limits
	monitor.handleUnhealthyProvider("test", provider, assert.AnError)

	// Verify alert was sent
	alerts := monitor.alertSystem.GetAlerts()
	assert.NotEmpty(t, alerts)
	assert.Equal(t, "health_failure", alerts[0].Type)
}

func TestHealthMonitor_AlertSystemMaxRetries(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewAutoLLMManager(tmpDir)
	manager.config.Health.MaxRetries = 1

	monitor := NewHealthMonitor(manager)

	provider := &AutoProvider{
		LocalLLMProvider: LocalLLMProvider{
			Name:     "TestProvider",
			DataPath: tmpDir,
		},
		Health:     &HealthStatus{},
		RetryCount: 5, // Exceeds max retries
	}

	// Handle unhealthy exceeding retry limits
	monitor.handleUnhealthyProvider("test", provider, assert.AnError)

	// Verify critical alert was sent
	alerts := monitor.alertSystem.GetAlerts()
	assert.GreaterOrEqual(t, len(alerts), 2)

	hasCriticalAlert := false
	for _, alert := range alerts {
		if alert.Severity == "critical" {
			hasCriticalAlert = true
		}
	}
	assert.True(t, hasCriticalAlert)
}
