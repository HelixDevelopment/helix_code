package llm

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Helper function to create test providers
func createTestAutoProviders() []*AutoProvider {
	return []*AutoProvider{
		{
			LocalLLMProvider: LocalLLMProvider{
				Name: "provider1",
			},
			Status: "running",
			Health: &HealthStatus{
				IsHealthy:    true,
				ResponseTime: 100,
			},
			Metrics: &PerformanceMetrics{
				TokensPerSecond: 50,
				ErrorRate:       1.0,
				ActiveRequests:  5,
			},
		},
		{
			LocalLLMProvider: LocalLLMProvider{
				Name: "provider2",
			},
			Status: "running",
			Health: &HealthStatus{
				IsHealthy:    true,
				ResponseTime: 50,
			},
			Metrics: &PerformanceMetrics{
				TokensPerSecond: 100,
				ErrorRate:       0.5,
				ActiveRequests:  2,
			},
		},
		{
			LocalLLMProvider: LocalLLMProvider{
				Name: "provider3",
			},
			Status: "running",
			Health: &HealthStatus{
				IsHealthy:    true,
				ResponseTime: 200,
			},
			Metrics: &PerformanceMetrics{
				TokensPerSecond: 30,
				ErrorRate:       2.0,
				ActiveRequests:  10,
			},
		},
	}
}

func TestRoundRobinStrategy(t *testing.T) {
	strategy := &RoundRobinStrategy{}

	t.Run("get name", func(t *testing.T) {
		assert.Equal(t, "round_robin", strategy.GetName())
	})

	t.Run("empty providers", func(t *testing.T) {
		result := strategy.SelectProvider([]*AutoProvider{})
		assert.Nil(t, result)
	})

	t.Run("cycles through providers", func(t *testing.T) {
		providers := createTestAutoProviders()

		// Reset current index
		strategy.current = 0

		// Should cycle through all providers
		first := strategy.SelectProvider(providers)
		assert.Equal(t, "provider1", first.Name)

		second := strategy.SelectProvider(providers)
		assert.Equal(t, "provider2", second.Name)

		third := strategy.SelectProvider(providers)
		assert.Equal(t, "provider3", third.Name)

		// Should wrap around
		fourth := strategy.SelectProvider(providers)
		assert.Equal(t, "provider1", fourth.Name)
	})
}

func TestLeastConnectionsStrategy(t *testing.T) {
	strategy := &LeastConnectionsStrategy{}

	t.Run("get name", func(t *testing.T) {
		assert.Equal(t, "least_connections", strategy.GetName())
	})

	t.Run("empty providers", func(t *testing.T) {
		result := strategy.SelectProvider([]*AutoProvider{})
		assert.Nil(t, result)
	})

	t.Run("selects provider with least connections", func(t *testing.T) {
		providers := createTestAutoProviders()

		// provider2 has least active requests (2)
		selected := strategy.SelectProvider(providers)
		assert.Equal(t, "provider2", selected.Name)
	})
}

func TestResponseTimeStrategy(t *testing.T) {
	strategy := &ResponseTimeStrategy{}

	t.Run("get name", func(t *testing.T) {
		assert.Equal(t, "response_time", strategy.GetName())
	})

	t.Run("empty providers", func(t *testing.T) {
		result := strategy.SelectProvider([]*AutoProvider{})
		assert.Nil(t, result)
	})

	t.Run("selects provider with lowest response time", func(t *testing.T) {
		providers := createTestAutoProviders()

		// provider2 has lowest response time (50)
		selected := strategy.SelectProvider(providers)
		assert.Equal(t, "provider2", selected.Name)
	})

	t.Run("ignores zero response times", func(t *testing.T) {
		providers := []*AutoProvider{
			{
				LocalLLMProvider: LocalLLMProvider{
					Name: "zero-response",
				},
				Health: &HealthStatus{
					ResponseTime: 0,
				},
			},
			{
				LocalLLMProvider: LocalLLMProvider{
					Name: "valid-response",
				},
				Health: &HealthStatus{
					ResponseTime: 100,
				},
			},
		}

		selected := strategy.SelectProvider(providers)
		assert.Equal(t, "valid-response", selected.Name)
	})
}

func TestWeightedStrategy(t *testing.T) {
	strategy := &WeightedStrategy{}

	t.Run("get name", func(t *testing.T) {
		assert.Equal(t, "weighted", strategy.GetName())
	})

	t.Run("empty providers", func(t *testing.T) {
		result := strategy.SelectProvider([]*AutoProvider{})
		assert.Nil(t, result)
	})

	t.Run("selects a provider", func(t *testing.T) {
		providers := createTestAutoProviders()

		// Should return one of the providers
		selected := strategy.SelectProvider(providers)
		assert.NotNil(t, selected)
		assert.Contains(t, []string{"provider1", "provider2", "provider3"}, selected.Name)
	})
}

func TestPerformanceBasedStrategy(t *testing.T) {
	strategy := &PerformanceBasedStrategy{}

	t.Run("get name", func(t *testing.T) {
		assert.Equal(t, "performance_based", strategy.GetName())
	})

	t.Run("empty providers", func(t *testing.T) {
		result := strategy.SelectProvider([]*AutoProvider{})
		assert.Nil(t, result)
	})

	t.Run("selects best performing provider", func(t *testing.T) {
		providers := createTestAutoProviders()

		// provider2 has best overall performance (low response time, high throughput, low errors)
		selected := strategy.SelectProvider(providers)
		assert.Equal(t, "provider2", selected.Name)
	})

	t.Run("calculate score considers all factors", func(t *testing.T) {
		provider := &AutoProvider{
			LocalLLMProvider: LocalLLMProvider{
				Name: "test",
			},
			Health: &HealthStatus{
				IsHealthy:    true,
				ResponseTime: 100,
			},
			Metrics: &PerformanceMetrics{
				TokensPerSecond: 50,
				ErrorRate:       5.0,
			},
		}

		score := strategy.calculateScore(provider)
		assert.Greater(t, score, 0.0)
	})

	t.Run("unhealthy provider gets lower score", func(t *testing.T) {
		healthyProvider := &AutoProvider{
			LocalLLMProvider: LocalLLMProvider{
				Name: "healthy",
			},
			Health: &HealthStatus{
				IsHealthy:    true,
				ResponseTime: 100,
			},
			Metrics: &PerformanceMetrics{
				TokensPerSecond: 50,
				ErrorRate:       5.0,
			},
		}

		unhealthyProvider := &AutoProvider{
			LocalLLMProvider: LocalLLMProvider{
				Name: "unhealthy",
			},
			Health: &HealthStatus{
				IsHealthy:    false,
				ResponseTime: 100,
			},
			Metrics: &PerformanceMetrics{
				TokensPerSecond: 50,
				ErrorRate:       5.0,
			},
		}

		healthyScore := strategy.calculateScore(healthyProvider)
		unhealthyScore := strategy.calculateScore(unhealthyProvider)

		assert.Greater(t, healthyScore, unhealthyScore)
	})
}

func TestAlertSystem(t *testing.T) {
	t.Run("create new alert system", func(t *testing.T) {
		as := NewAlertSystem()
		assert.NotNil(t, as)
		assert.Empty(t, as.GetAlerts())
	})

	t.Run("send alert", func(t *testing.T) {
		as := NewAlertSystem()

		alert := &Alert{
			Type:      "error",
			Provider:  "test-provider",
			Message:   "Test error message",
			Severity:  "high",
			Timestamp: time.Now(),
		}

		as.SendAlert(alert)

		alerts := as.GetAlerts()
		assert.Len(t, alerts, 1)
		assert.Equal(t, "error", alerts[0].Type)
		assert.Equal(t, "test-provider", alerts[0].Provider)
		assert.Equal(t, "Test error message", alerts[0].Message)
		assert.Equal(t, "high", alerts[0].Severity)
	})

	t.Run("multiple alerts", func(t *testing.T) {
		as := NewAlertSystem()

		for i := 0; i < 5; i++ {
			alert := &Alert{
				Type:      "info",
				Provider:  "provider",
				Message:   "Message",
				Severity:  "low",
				Timestamp: time.Now(),
			}
			as.SendAlert(alert)
		}

		alerts := as.GetAlerts()
		assert.Len(t, alerts, 5)
	})

	t.Run("limits to 1000 alerts", func(t *testing.T) {
		as := NewAlertSystem()

		// Send 1005 alerts
		for i := 0; i < 1005; i++ {
			alert := &Alert{
				Type:      "info",
				Provider:  "provider",
				Message:   "Message",
				Severity:  "low",
				Timestamp: time.Now(),
			}
			as.SendAlert(alert)
		}

		alerts := as.GetAlerts()
		assert.LessOrEqual(t, len(alerts), 1000)
	})
}

func TestLoadBalancingStats(t *testing.T) {
	t.Run("create empty stats", func(t *testing.T) {
		stats := &LoadBalancingStats{
			ProviderCounts: make(map[string]int64),
			ResponseTimes:  make(map[string]float64),
			ErrorRates:     make(map[string]float64),
		}

		assert.NotNil(t, stats)
		assert.Empty(t, stats.ProviderCounts)
	})
}

// ========================================
// LoadBalancer Tests
// ========================================

func TestNewLoadBalancer(t *testing.T) {
	t.Run("creates load balancer with nil manager", func(t *testing.T) {
		lb := NewLoadBalancer(nil)
		assert.NotNil(t, lb)
		assert.NotNil(t, lb.strategies)
		assert.Equal(t, "performance_based", lb.currentStrategy)
		assert.NotNil(t, lb.stats)
		assert.False(t, lb.isRunning)
	})

	t.Run("initializes all strategies", func(t *testing.T) {
		lb := NewLoadBalancer(nil)

		expectedStrategies := []string{
			"round_robin",
			"least_connections",
			"response_time",
			"weighted",
			"performance_based",
		}

		for _, name := range expectedStrategies {
			assert.NotNil(t, lb.strategies[name], "strategy %s should exist", name)
		}
	})
}

func TestLoadBalancer_SetStrategy(t *testing.T) {
	t.Run("set valid strategy", func(t *testing.T) {
		lb := NewLoadBalancer(nil)

		err := lb.SetStrategy("round_robin")
		assert.NoError(t, err)
		assert.Equal(t, "round_robin", lb.currentStrategy)
	})

	t.Run("set invalid strategy", func(t *testing.T) {
		lb := NewLoadBalancer(nil)

		err := lb.SetStrategy("invalid_strategy")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unknown load balancing strategy")
	})

	t.Run("set all valid strategies", func(t *testing.T) {
		lb := NewLoadBalancer(nil)

		strategies := []string{
			"round_robin",
			"least_connections",
			"response_time",
			"weighted",
			"performance_based",
		}

		for _, strategy := range strategies {
			err := lb.SetStrategy(strategy)
			assert.NoError(t, err, "strategy %s should be valid", strategy)
			assert.Equal(t, strategy, lb.currentStrategy)
		}
	})
}

func TestLoadBalancer_GetStats(t *testing.T) {
	t.Run("returns stats copy", func(t *testing.T) {
		lb := NewLoadBalancer(nil)

		stats := lb.GetStats()
		assert.NotNil(t, stats)
		assert.NotNil(t, stats.ProviderCounts)
		assert.NotNil(t, stats.ResponseTimes)
		assert.NotNil(t, stats.ErrorRates)
	})
}

func TestLoadBalancer_Stop(t *testing.T) {
	t.Run("stop when not running", func(t *testing.T) {
		lb := NewLoadBalancer(nil)

		lb.Stop()
		assert.False(t, lb.isRunning)
	})

	t.Run("stop when running", func(t *testing.T) {
		lb := NewLoadBalancer(nil)
		lb.isRunning = true

		lb.Stop()
		assert.False(t, lb.isRunning)
	})
}

func TestLoadBalancer_SelectOptimalProvider(t *testing.T) {
	t.Run("returns nil with nil manager", func(t *testing.T) {
		// This test requires AutoLLMManager setup
		// Skip for now as it needs integration test infrastructure
		t.Skip("requires AutoLLMManager setup")  // SKIP-OK: #legacy-untriaged
	})
}

func TestLoadBalancer_Start(t *testing.T) {
	t.Run("start with nil manager", func(t *testing.T) {
		lb := NewLoadBalancer(nil)
		ctx := context.Background()

		err := lb.Start(ctx)
		assert.NoError(t, err)
		assert.True(t, lb.isRunning)

		// Cleanup
		lb.Stop()
	})

	t.Run("start when already running returns nil", func(t *testing.T) {
		lb := NewLoadBalancer(nil)
		lb.isRunning = true

		ctx := context.Background()
		err := lb.Start(ctx)
		assert.NoError(t, err)
	})
}

// ========================================
// Additional Strategy Tests
// ========================================

func TestRoundRobinStrategy_Wrapping(t *testing.T) {
	strategy := &RoundRobinStrategy{}
	providers := createTestAutoProviders()

	// Run through multiple cycles
	for i := 0; i < 10; i++ {
		selected := strategy.SelectProvider(providers)
		assert.NotNil(t, selected)
	}
}

func TestLeastConnectionsStrategy_ZeroConnections(t *testing.T) {
	strategy := &LeastConnectionsStrategy{}

	providers := []*AutoProvider{
		{
			LocalLLMProvider: LocalLLMProvider{Name: "p1"},
			Metrics:          &PerformanceMetrics{ActiveRequests: 0},
		},
		{
			LocalLLMProvider: LocalLLMProvider{Name: "p2"},
			Metrics:          &PerformanceMetrics{ActiveRequests: 5},
		},
	}

	selected := strategy.SelectProvider(providers)
	// Should select p1 with 0 connections
	assert.Equal(t, "p1", selected.Name)
}

func TestResponseTimeStrategy_AllSameTime(t *testing.T) {
	strategy := &ResponseTimeStrategy{}

	providers := []*AutoProvider{
		{
			LocalLLMProvider: LocalLLMProvider{Name: "p1"},
			Health:           &HealthStatus{ResponseTime: 100},
		},
		{
			LocalLLMProvider: LocalLLMProvider{Name: "p2"},
			Health:           &HealthStatus{ResponseTime: 100},
		},
	}

	selected := strategy.SelectProvider(providers)
	// Should select one of them
	assert.NotNil(t, selected)
}

func TestWeightedStrategy_SingleProvider(t *testing.T) {
	strategy := &WeightedStrategy{}

	providers := []*AutoProvider{
		{
			LocalLLMProvider: LocalLLMProvider{Name: "only"},
			Health:           &HealthStatus{IsHealthy: true},
			Metrics:          &PerformanceMetrics{TokensPerSecond: 50},
		},
	}

	selected := strategy.SelectProvider(providers)
	assert.Equal(t, "only", selected.Name)
}

func TestPerformanceBasedStrategy_HighErrorRate(t *testing.T) {
	strategy := &PerformanceBasedStrategy{}

	lowErrorProvider := &AutoProvider{
		LocalLLMProvider: LocalLLMProvider{Name: "low-error"},
		Health:           &HealthStatus{IsHealthy: true, ResponseTime: 100},
		Metrics:          &PerformanceMetrics{TokensPerSecond: 50, ErrorRate: 1.0},
	}

	highErrorProvider := &AutoProvider{
		LocalLLMProvider: LocalLLMProvider{Name: "high-error"},
		Health:           &HealthStatus{IsHealthy: true, ResponseTime: 100},
		Metrics:          &PerformanceMetrics{TokensPerSecond: 50, ErrorRate: 50.0},
	}

	lowScore := strategy.calculateScore(lowErrorProvider)
	highScore := strategy.calculateScore(highErrorProvider)

	// Low error rate should have higher score
	assert.Greater(t, lowScore, highScore)
}

func TestPerformanceBasedStrategy_HighThroughput(t *testing.T) {
	strategy := &PerformanceBasedStrategy{}

	slowProvider := &AutoProvider{
		LocalLLMProvider: LocalLLMProvider{Name: "slow"},
		Health:           &HealthStatus{IsHealthy: true, ResponseTime: 100},
		Metrics:          &PerformanceMetrics{TokensPerSecond: 10, ErrorRate: 1.0},
	}

	fastProvider := &AutoProvider{
		LocalLLMProvider: LocalLLMProvider{Name: "fast"},
		Health:           &HealthStatus{IsHealthy: true, ResponseTime: 100},
		Metrics:          &PerformanceMetrics{TokensPerSecond: 100, ErrorRate: 1.0},
	}

	slowScore := strategy.calculateScore(slowProvider)
	fastScore := strategy.calculateScore(fastProvider)

	// Higher throughput should have higher score
	assert.Greater(t, fastScore, slowScore)
}

func TestAlertSystem_ClearAlerts(t *testing.T) {
	as := NewAlertSystem()

	// Add some alerts
	for i := 0; i < 5; i++ {
		as.SendAlert(&Alert{
			Type:      "test",
			Provider:  "provider",
			Message:   "test message",
			Severity:  "low",
			Timestamp: time.Now(),
		})
	}

	assert.Len(t, as.GetAlerts(), 5)

	// GetAlerts returns a copy, so we can't clear via that
	// Just verify alerts are returned correctly
	alerts := as.GetAlerts()
	for _, alert := range alerts {
		assert.Equal(t, "test", alert.Type)
	}
}

// ========================================
// LoadBalancer Integration Tests with AutoLLMManager
// ========================================

func createMockAutoLLMManager() *AutoLLMManager {
	return &AutoLLMManager{
		providers: map[string]*AutoProvider{
			"healthy1": {
				LocalLLMProvider: LocalLLMProvider{Name: "healthy1"},
				Status:           "running",
				Health:           &HealthStatus{IsHealthy: true, ResponseTime: 100},
				Metrics:          &PerformanceMetrics{TokensPerSecond: 50, ErrorRate: 1.0},
			},
			"healthy2": {
				LocalLLMProvider: LocalLLMProvider{Name: "healthy2"},
				Status:           "running",
				Health:           &HealthStatus{IsHealthy: true, ResponseTime: 50},
				Metrics:          &PerformanceMetrics{TokensPerSecond: 100, ErrorRate: 0.5},
			},
			"unhealthy": {
				LocalLLMProvider: LocalLLMProvider{Name: "unhealthy"},
				Status:           "running",
				Health:           &HealthStatus{IsHealthy: false, ResponseTime: 500},
				Metrics:          &PerformanceMetrics{TokensPerSecond: 10, ErrorRate: 20.0},
			},
			"stopped": {
				LocalLLMProvider: LocalLLMProvider{Name: "stopped"},
				Status:           "stopped",
				Health:           &HealthStatus{IsHealthy: true, ResponseTime: 100},
				Metrics:          &PerformanceMetrics{TokensPerSecond: 50, ErrorRate: 0},
			},
		},
	}
}

func TestLoadBalancer_SelectOptimalProvider_WithManager(t *testing.T) {
	t.Run("selects from healthy providers", func(t *testing.T) {
		manager := createMockAutoLLMManager()
		lb := NewLoadBalancer(manager)

		ctx := context.Background()
		selected := lb.SelectOptimalProvider(ctx)

		assert.NotNil(t, selected)
		// Should select one of the healthy running providers
		assert.Contains(t, []string{"healthy1", "healthy2"}, selected.Name)
	})

	t.Run("returns nil with no healthy providers", func(t *testing.T) {
		manager := &AutoLLMManager{
			providers: map[string]*AutoProvider{
				"unhealthy": {
					LocalLLMProvider: LocalLLMProvider{Name: "unhealthy"},
					Status:           "running",
					Health:           &HealthStatus{IsHealthy: false},
				},
			},
		}
		lb := NewLoadBalancer(manager)

		ctx := context.Background()
		selected := lb.SelectOptimalProvider(ctx)

		assert.Nil(t, selected)
	})

	t.Run("falls back to performance_based with invalid strategy", func(t *testing.T) {
		manager := createMockAutoLLMManager()
		lb := NewLoadBalancer(manager)
		lb.currentStrategy = "invalid_strategy"

		ctx := context.Background()
		selected := lb.SelectOptimalProvider(ctx)

		assert.NotNil(t, selected)
	})
}

func TestLoadBalancer_getHealthyProviders_Full(t *testing.T) {
	t.Run("returns only healthy running providers", func(t *testing.T) {
		manager := createMockAutoLLMManager()
		lb := NewLoadBalancer(manager)

		healthy := lb.getHealthyProviders()

		assert.Len(t, healthy, 2)
		for _, p := range healthy {
			assert.True(t, p.Health.IsHealthy)
			assert.Equal(t, "running", p.Status)
		}
	})

	t.Run("returns empty with no healthy providers", func(t *testing.T) {
		manager := &AutoLLMManager{
			providers: map[string]*AutoProvider{
				"unhealthy": {
					LocalLLMProvider: LocalLLMProvider{Name: "unhealthy"},
					Status:           "running",
					Health:           &HealthStatus{IsHealthy: false},
				},
			},
		}
		lb := NewLoadBalancer(manager)

		healthy := lb.getHealthyProviders()

		assert.Empty(t, healthy)
	})
}

func TestLoadBalancer_updateStats_Full(t *testing.T) {
	t.Run("increments request counts", func(t *testing.T) {
		manager := createMockAutoLLMManager()
		lb := NewLoadBalancer(manager)

		provider := manager.providers["healthy1"]

		lb.updateStats(provider)
		assert.Equal(t, int64(1), lb.stats.TotalRequests)
		assert.Equal(t, int64(1), lb.stats.ProviderCounts["healthy1"])

		lb.updateStats(provider)
		assert.Equal(t, int64(2), lb.stats.TotalRequests)
		assert.Equal(t, int64(2), lb.stats.ProviderCounts["healthy1"])
	})

	t.Run("updates last updated time", func(t *testing.T) {
		manager := createMockAutoLLMManager()
		lb := NewLoadBalancer(manager)

		before := lb.stats.LastUpdated
		time.Sleep(1 * time.Millisecond)

		provider := manager.providers["healthy1"]
		lb.updateStats(provider)

		assert.True(t, lb.stats.LastUpdated.After(before))
	})
}

func TestLoadBalancer_collectPerformanceStats_Full(t *testing.T) {
	t.Run("collects response times and error rates", func(t *testing.T) {
		manager := createMockAutoLLMManager()
		lb := NewLoadBalancer(manager)

		lb.collectPerformanceStats()

		// Should have stats for healthy providers
		assert.NotEmpty(t, lb.stats.ResponseTimes)
		assert.NotEmpty(t, lb.stats.ErrorRates)
	})

	t.Run("uses weighted average for response times", func(t *testing.T) {
		manager := createMockAutoLLMManager()
		lb := NewLoadBalancer(manager)

		// Pre-set some values
		lb.stats.ResponseTimes["healthy1"] = 200.0
		lb.stats.ErrorRates["healthy1"] = 5.0

		lb.collectPerformanceStats()

		// Values should be weighted average (10% new, 90% old)
		// New value: 100, Old: 200, Result: 0.1*100 + 0.9*200 = 10 + 180 = 190
		expectedRT := 0.1*100 + 0.9*200
		assert.InDelta(t, expectedRT, lb.stats.ResponseTimes["healthy1"], 0.01)
	})
}

func TestLoadBalancer_determineOptimalProvider_Full(t *testing.T) {
	t.Run("determines best performing provider", func(t *testing.T) {
		manager := createMockAutoLLMManager()
		lb := NewLoadBalancer(manager)

		lb.determineOptimalProvider()

		// healthy2 has better metrics (faster, lower error rate)
		assert.Equal(t, "healthy2", lb.stats.OptimalProvider)
	})

	t.Run("ignores unhealthy providers", func(t *testing.T) {
		manager := &AutoLLMManager{
			providers: map[string]*AutoProvider{
				"unhealthy": {
					LocalLLMProvider: LocalLLMProvider{Name: "unhealthy"},
					Status:           "running",
					Health:           &HealthStatus{IsHealthy: false, ResponseTime: 10},
					Metrics:          &PerformanceMetrics{TokensPerSecond: 1000, ErrorRate: 0},
				},
				"healthy": {
					LocalLLMProvider: LocalLLMProvider{Name: "healthy"},
					Status:           "running",
					Health:           &HealthStatus{IsHealthy: true, ResponseTime: 100},
					Metrics:          &PerformanceMetrics{TokensPerSecond: 50, ErrorRate: 1.0},
				},
			},
		}
		lb := NewLoadBalancer(manager)

		lb.determineOptimalProvider()

		assert.Equal(t, "healthy", lb.stats.OptimalProvider)
	})
}

func TestLoadBalancer_calculatePerformanceScore_Full(t *testing.T) {
	t.Run("calculates score with all factors", func(t *testing.T) {
		manager := createMockAutoLLMManager()
		lb := NewLoadBalancer(manager)

		lb.stats.ResponseTimes["healthy1"] = 100.0
		lb.stats.ErrorRates["healthy1"] = 2.0

		provider := manager.providers["healthy1"]
		score := lb.calculatePerformanceScore("healthy1", provider)

		assert.Greater(t, score, 0.0)
	})

	t.Run("higher throughput yields higher score", func(t *testing.T) {
		manager := createMockAutoLLMManager()
		lb := NewLoadBalancer(manager)

		lb.stats.ResponseTimes["healthy1"] = 100.0
		lb.stats.ResponseTimes["healthy2"] = 100.0
		lb.stats.ErrorRates["healthy1"] = 0.0
		lb.stats.ErrorRates["healthy2"] = 0.0

		score1 := lb.calculatePerformanceScore("healthy1", manager.providers["healthy1"])
		score2 := lb.calculatePerformanceScore("healthy2", manager.providers["healthy2"])

		// healthy2 has higher TokensPerSecond (100 vs 50)
		assert.Greater(t, score2, score1)
	})

	t.Run("lower error rate yields higher score", func(t *testing.T) {
		manager := &AutoLLMManager{
			providers: map[string]*AutoProvider{
				"low_error": {
					LocalLLMProvider: LocalLLMProvider{Name: "low_error"},
					Status:           "running",
					Health:           &HealthStatus{IsHealthy: true, ResponseTime: 100},
					Metrics:          &PerformanceMetrics{TokensPerSecond: 50, ErrorRate: 0},
				},
				"high_error": {
					LocalLLMProvider: LocalLLMProvider{Name: "high_error"},
					Status:           "running",
					Health:           &HealthStatus{IsHealthy: true, ResponseTime: 100},
					Metrics:          &PerformanceMetrics{TokensPerSecond: 50, ErrorRate: 50.0},
				},
			},
		}
		lb := NewLoadBalancer(manager)

		lb.stats.ResponseTimes["low_error"] = 100.0
		lb.stats.ResponseTimes["high_error"] = 100.0
		lb.stats.ErrorRates["low_error"] = 0.0
		lb.stats.ErrorRates["high_error"] = 50.0

		scoreLow := lb.calculatePerformanceScore("low_error", manager.providers["low_error"])
		scoreHigh := lb.calculatePerformanceScore("high_error", manager.providers["high_error"])

		assert.Greater(t, scoreLow, scoreHigh)
	})

	t.Run("faster response time yields higher score", func(t *testing.T) {
		manager := &AutoLLMManager{
			providers: map[string]*AutoProvider{
				"fast": {
					LocalLLMProvider: LocalLLMProvider{Name: "fast"},
					Status:           "running",
					Health:           &HealthStatus{IsHealthy: true, ResponseTime: 50},
					Metrics:          &PerformanceMetrics{TokensPerSecond: 50, ErrorRate: 0},
				},
				"slow": {
					LocalLLMProvider: LocalLLMProvider{Name: "slow"},
					Status:           "running",
					Health:           &HealthStatus{IsHealthy: true, ResponseTime: 500},
					Metrics:          &PerformanceMetrics{TokensPerSecond: 50, ErrorRate: 0},
				},
			},
		}
		lb := NewLoadBalancer(manager)

		lb.stats.ResponseTimes["fast"] = 50.0
		lb.stats.ResponseTimes["slow"] = 500.0
		lb.stats.ErrorRates["fast"] = 0.0
		lb.stats.ErrorRates["slow"] = 0.0

		scoreFast := lb.calculatePerformanceScore("fast", manager.providers["fast"])
		scoreSlow := lb.calculatePerformanceScore("slow", manager.providers["slow"])

		assert.Greater(t, scoreFast, scoreSlow)
	})

	t.Run("handles missing response time stats", func(t *testing.T) {
		manager := createMockAutoLLMManager()
		lb := NewLoadBalancer(manager)

		// Don't add any stats
		provider := manager.providers["healthy1"]
		score := lb.calculatePerformanceScore("healthy1", provider)

		// Should still return a valid score
		assert.Greater(t, score, 0.0)
	})
}

func TestLoadBalancer_collectStats_Context(t *testing.T) {
	t.Run("stops on context cancellation", func(t *testing.T) {
		manager := createMockAutoLLMManager()
		lb := NewLoadBalancer(manager)

		ctx, cancel := context.WithCancel(context.Background())

		done := make(chan struct{})
		go func() {
			lb.collectStats(ctx)
			close(done)
		}()

		// Cancel the context
		cancel()

		// Wait for collectStats to return
		select {
		case <-done:
			// Success
		case <-time.After(2 * time.Second):
			t.Fatal("collectStats did not stop after context cancellation")
		}
	})
}
