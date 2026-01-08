package llm

import (
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
