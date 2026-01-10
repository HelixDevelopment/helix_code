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

func TestNewUsageAnalytics(t *testing.T) {
	t.Run("CreatesAnalytics", func(t *testing.T) {
		tempDir := t.TempDir()
		analytics := NewUsageAnalytics(tempDir)
		require.NotNil(t, analytics)
		assert.NotNil(t, analytics.ModelUsageStats)
		assert.NotNil(t, analytics.TaskPatterns)
		assert.NotNil(t, analytics.UserPreferences)
		assert.NotNil(t, analytics.PerformanceHistory)
		assert.Contains(t, analytics.analyticsDir, "analytics")
	})

	t.Run("CreatesAnalyticsDir", func(t *testing.T) {
		tempDir := t.TempDir()
		analytics := NewUsageAnalytics(tempDir)
		require.NotNil(t, analytics)

		// Directory should exist
		_, err := os.Stat(analytics.analyticsDir)
		assert.NoError(t, err)
	})

	t.Run("LoadsExistingData", func(t *testing.T) {
		tempDir := t.TempDir()
		analyticsDir := filepath.Join(tempDir, "analytics")
		os.MkdirAll(analyticsDir, 0755)

		// Create some existing data
		data := `{"test-model": {"model_id": "test-model", "total_requests": 100}}`
		os.WriteFile(filepath.Join(analyticsDir, "model_usage_stats.json"), []byte(data), 0644)

		analytics := NewUsageAnalytics(tempDir)
		require.NotNil(t, analytics)
		// Should load existing data
		_, exists := analytics.ModelUsageStats["test-model"]
		assert.True(t, exists)
	})
}

func TestUsageAnalytics_RecordModelUsage(t *testing.T) {
	t.Run("RecordsNewModel", func(t *testing.T) {
		tempDir := t.TempDir()
		analytics := NewUsageAnalytics(tempDir)

		ctx := context.Background()
		metrics := &UsageMetrics{
			Timestamp:   time.Now(),
			LatencyMs:   100.0,
			Success:     true,
			UserRating:  4.5,
			TaskType:    "code_generation",
			InputTokens: 100,
			Provider:    "ollama",
		}

		err := analytics.RecordModelUsage(ctx, "llama-7b", "ollama", "user1", metrics)
		require.NoError(t, err)

		// Check stats were recorded
		stats, exists := analytics.ModelUsageStats["llama-7b"]
		assert.True(t, exists)
		assert.Equal(t, int64(1), stats.TotalRequests)
		assert.Contains(t, stats.PreferredBy, "user1")
		assert.Contains(t, stats.CommonTasks, "code_generation")
	})

	t.Run("UpdatesExistingModel", func(t *testing.T) {
		tempDir := t.TempDir()
		analytics := NewUsageAnalytics(tempDir)

		ctx := context.Background()
		metrics := &UsageMetrics{
			Timestamp:   time.Now(),
			LatencyMs:   100.0,
			Success:     true,
			UserRating:  4.0,
			TaskType:    "code_generation",
			InputTokens: 100,
		}

		// First usage
		err := analytics.RecordModelUsage(ctx, "llama-7b", "ollama", "user1", metrics)
		require.NoError(t, err)

		// Second usage
		err = analytics.RecordModelUsage(ctx, "llama-7b", "ollama", "user2", metrics)
		require.NoError(t, err)

		stats := analytics.ModelUsageStats["llama-7b"]
		assert.Equal(t, int64(2), stats.TotalRequests)
		assert.Contains(t, stats.PreferredBy, "user1")
		assert.Contains(t, stats.PreferredBy, "user2")
	})

	t.Run("UpdatesSuccessRate", func(t *testing.T) {
		tempDir := t.TempDir()
		analytics := NewUsageAnalytics(tempDir)

		ctx := context.Background()

		// Success
		metrics := &UsageMetrics{
			Timestamp: time.Now(),
			LatencyMs: 100.0,
			Success:   true,
		}
		analytics.RecordModelUsage(ctx, "llama-7b", "ollama", "user1", metrics)

		// Failure
		metrics.Success = false
		analytics.RecordModelUsage(ctx, "llama-7b", "ollama", "user2", metrics)

		stats := analytics.ModelUsageStats["llama-7b"]
		assert.Greater(t, stats.SuccessRate, 0.0)
		assert.Less(t, stats.SuccessRate, 1.0)
	})
}

func TestUsageAnalytics_UpdateUsageTrend(t *testing.T) {
	tempDir := t.TempDir()
	analytics := NewUsageAnalytics(tempDir)

	t.Run("StableForLowRequests", func(t *testing.T) {
		stats := &ModelUsageStats{
			TotalRequests:    5,
			UserSatisfaction: 4.5,
			SuccessRate:      0.95,
		}
		analytics.updateUsageTrend("test-model", stats)
		assert.Equal(t, "stable", stats.UsageTrend)
	})

	t.Run("IncreasingForHighSatisfaction", func(t *testing.T) {
		stats := &ModelUsageStats{
			TotalRequests:    100,
			UserSatisfaction: 4.5,
			SuccessRate:      0.95,
		}
		analytics.updateUsageTrend("test-model", stats)
		assert.Equal(t, "increasing", stats.UsageTrend)
	})

	t.Run("DecreasingForLowSatisfaction", func(t *testing.T) {
		stats := &ModelUsageStats{
			TotalRequests:    100,
			UserSatisfaction: 2.5,
			SuccessRate:      0.6,
		}
		analytics.updateUsageTrend("test-model", stats)
		assert.Equal(t, "decreasing", stats.UsageTrend)
	})
}

func TestUsageAnalytics_RecordTaskPattern(t *testing.T) {
	tempDir := t.TempDir()
	analytics := NewUsageAnalytics(tempDir)

	ctx := context.Background()
	metrics := &UsageMetrics{
		Timestamp: time.Now(),
		LatencyMs: 150.0,
	}

	t.Run("CreatesNewPattern", func(t *testing.T) {
		err := analytics.RecordTaskPattern(ctx, "code_generation", "llama-7b", 0.7, metrics)
		require.NoError(t, err)

		pattern, exists := analytics.TaskPatterns["code_generation"]
		assert.True(t, exists)
		assert.Equal(t, "code_generation", pattern.TaskType)
		assert.Contains(t, pattern.CommonModels, "llama-7b")
	})

	t.Run("UpdatesExistingPattern", func(t *testing.T) {
		err := analytics.RecordTaskPattern(ctx, "code_generation", "codellama", 0.8, metrics)
		require.NoError(t, err)

		pattern := analytics.TaskPatterns["code_generation"]
		assert.Contains(t, pattern.CommonModels, "llama-7b")
		assert.Contains(t, pattern.CommonModels, "codellama")
	})

	t.Run("UpdatesPeakHours", func(t *testing.T) {
		err := analytics.RecordTaskPattern(ctx, "debugging", "llama-7b", 0.5, metrics)
		require.NoError(t, err)

		pattern := analytics.TaskPatterns["debugging"]
		assert.NotEmpty(t, pattern.PeakHours)
	})
}

func TestUsageAnalytics_UserPreferences(t *testing.T) {
	tempDir := t.TempDir()
	analytics := NewUsageAnalytics(tempDir)
	ctx := context.Background()

	t.Run("SetAndGetPreferences", func(t *testing.T) {
		prefs := &UserPreferences{
			UserID:             "user1",
			PreferredProviders: []string{"ollama", "vllm"},
			QualityPreference:  "balanced",
		}

		err := analytics.SetUserPreferences(ctx, prefs)
		require.NoError(t, err)

		retrieved, err := analytics.GetUserPreferences(ctx, "user1")
		require.NoError(t, err)
		assert.Equal(t, "user1", retrieved.UserID)
		assert.Equal(t, "balanced", retrieved.QualityPreference)
		assert.Contains(t, retrieved.PreferredProviders, "ollama")
	})

	t.Run("GetDefaultPreferences", func(t *testing.T) {
		prefs, err := analytics.GetUserPreferences(ctx, "unknown-user")
		require.NoError(t, err)
		assert.Equal(t, "unknown-user", prefs.UserID)
		assert.Equal(t, "balanced", prefs.QualityPreference)
	})
}

func TestUsageAnalytics_GetModelUsageStats(t *testing.T) {
	tempDir := t.TempDir()
	analytics := NewUsageAnalytics(tempDir)
	ctx := context.Background()

	t.Run("ReturnsExistingStats", func(t *testing.T) {
		// Record some usage first
		metrics := &UsageMetrics{
			Timestamp: time.Now(),
			LatencyMs: 100.0,
			Success:   true,
		}
		analytics.RecordModelUsage(ctx, "llama-7b", "ollama", "user1", metrics)

		stats, err := analytics.GetModelUsageStats(ctx, "llama-7b")
		require.NoError(t, err)
		assert.Equal(t, "llama-7b", stats.ModelID)
	})

	t.Run("ErrorsForUnknownModel", func(t *testing.T) {
		stats, err := analytics.GetModelUsageStats(ctx, "unknown-model")
		assert.Error(t, err)
		assert.Nil(t, stats)
	})
}

func TestUsageAnalytics_GetTopModelsByUsage(t *testing.T) {
	tempDir := t.TempDir()
	analytics := NewUsageAnalytics(tempDir)
	ctx := context.Background()

	// Record usage for multiple models
	metrics := &UsageMetrics{
		Timestamp: time.Now(),
		LatencyMs: 100.0,
		Success:   true,
	}

	// Model with 3 requests
	analytics.RecordModelUsage(ctx, "model-a", "ollama", "user1", metrics)
	analytics.RecordModelUsage(ctx, "model-a", "ollama", "user2", metrics)
	analytics.RecordModelUsage(ctx, "model-a", "ollama", "user3", metrics)

	// Model with 1 request
	analytics.RecordModelUsage(ctx, "model-b", "ollama", "user1", metrics)

	t.Run("ReturnsTopModels", func(t *testing.T) {
		models, err := analytics.GetTopModelsByUsage(ctx, 5)
		require.NoError(t, err)
		assert.Len(t, models, 2)
		// First should have more requests
		assert.GreaterOrEqual(t, models[0].TotalRequests, models[1].TotalRequests)
	})

	t.Run("RespectsLimit", func(t *testing.T) {
		models, err := analytics.GetTopModelsByUsage(ctx, 1)
		require.NoError(t, err)
		assert.Len(t, models, 1)
	})
}

func TestUsageAnalytics_GetTaskPatterns(t *testing.T) {
	tempDir := t.TempDir()
	analytics := NewUsageAnalytics(tempDir)
	ctx := context.Background()

	metrics := &UsageMetrics{Timestamp: time.Now()}
	analytics.RecordTaskPattern(ctx, "code_generation", "llama-7b", 0.7, metrics)
	analytics.RecordTaskPattern(ctx, "debugging", "codellama", 0.5, metrics)

	patterns, err := analytics.GetTaskPatterns(ctx)
	require.NoError(t, err)
	assert.Len(t, patterns, 2)
	assert.NotNil(t, patterns["code_generation"])
	assert.NotNil(t, patterns["debugging"])
}

func TestUsageAnalytics_GetPerformanceHistory(t *testing.T) {
	tempDir := t.TempDir()
	analytics := NewUsageAnalytics(tempDir)
	ctx := context.Background()

	t.Run("ReturnsExistingHistory", func(t *testing.T) {
		// Record some usage to create performance history
		metrics := &UsageMetrics{
			Timestamp:    time.Now(),
			LatencyMs:    100.0,
			Success:      true,
			InputTokens:  100,
			OutputTokens: 50,
			MemoryUsage:  4096,
		}
		analytics.RecordModelUsage(ctx, "llama-7b", "ollama", "user1", metrics)

		history, err := analytics.GetPerformanceHistory(ctx, "llama-7b")
		require.NoError(t, err)
		assert.Equal(t, "llama-7b", history.ModelID)
		assert.NotEmpty(t, history.TimeSeries)
	})

	t.Run("ErrorsForUnknownModel", func(t *testing.T) {
		history, err := analytics.GetPerformanceHistory(ctx, "unknown-model")
		assert.Error(t, err)
		assert.Nil(t, history)
	})
}

func TestUsageAnalytics_RecordOptimization(t *testing.T) {
	tempDir := t.TempDir()
	analytics := NewUsageAnalytics(tempDir)
	ctx := context.Background()

	before := &PerformanceEstimate{TokensPerSecond: 20.0}
	after := &PerformanceEstimate{TokensPerSecond: 30.0}

	t.Run("RecordsOptimization", func(t *testing.T) {
		err := analytics.RecordOptimization(ctx, "llama-7b", "ollama", "quantization", before, after, true, "GPTQ")
		require.NoError(t, err)

		history := analytics.PerformanceHistory["llama-7b"]
		assert.NotNil(t, history)
		assert.Len(t, history.OptimizationHistory, 1)
		assert.Equal(t, 50.0, history.OptimizationHistory[0].Improvement) // 50% improvement
		assert.True(t, history.OptimizationHistory[0].Success)
	})

	t.Run("HandlesNilMetrics", func(t *testing.T) {
		err := analytics.RecordOptimization(ctx, "model-b", "ollama", "quantization", nil, nil, false, "failed")
		require.NoError(t, err)

		history := analytics.PerformanceHistory["model-b"]
		assert.NotNil(t, history)
		assert.Equal(t, 0.0, history.OptimizationHistory[0].Improvement)
	})
}

func TestUsageAnalytics_GenerateUsageReport(t *testing.T) {
	tempDir := t.TempDir()
	analytics := NewUsageAnalytics(tempDir)
	ctx := context.Background()

	// Record some usage data
	metrics := &UsageMetrics{
		Timestamp:    time.Now(),
		LatencyMs:    100.0,
		Success:      true,
		UserRating:   4.5,
		TaskType:     "code_generation",
		InputTokens:  100,
		OutputTokens: 50,
	}
	analytics.RecordModelUsage(ctx, "llama-7b", "ollama", "user1", metrics)
	analytics.RecordTaskPattern(ctx, "code_generation", "llama-7b", 0.7, metrics)

	prefs := &UserPreferences{
		UserID:            "user1",
		QualityPreference: "balanced",
	}
	analytics.SetUserPreferences(ctx, prefs)

	t.Run("GeneratesReport", func(t *testing.T) {
		timeRange := TimeRange{
			Start: time.Now().Add(-24 * time.Hour),
			End:   time.Now(),
		}

		report, err := analytics.GenerateUsageReport(ctx, timeRange)
		require.NoError(t, err)
		assert.NotNil(t, report)
		assert.NotNil(t, report.Summary)
		assert.NotNil(t, report.TopModels)
		assert.NotNil(t, report.TaskAnalysis)
		assert.NotNil(t, report.PerformanceAnalysis)
		assert.NotNil(t, report.UserAnalysis)
		assert.NotZero(t, report.GeneratedAt)
	})
}

func TestUsageAnalytics_CalculateUsageSummary(t *testing.T) {
	tempDir := t.TempDir()
	analytics := NewUsageAnalytics(tempDir)
	ctx := context.Background()

	// Record some usage data
	metrics := &UsageMetrics{
		Timestamp:  time.Now(),
		LatencyMs:  100.0,
		Success:    true,
		UserRating: 4.5,
	}
	analytics.RecordModelUsage(ctx, "llama-7b", "ollama", "user1", metrics)

	// Set trending model
	analytics.ModelUsageStats["llama-7b"].UsageTrend = "increasing"
	analytics.ModelUsageStats["llama-7b"].TotalRequests = 100

	summary := analytics.calculateUsageSummary()
	assert.Equal(t, 1, summary.TotalModels)
	assert.Equal(t, int64(100), summary.TotalRequests)
	assert.Contains(t, summary.TrendingModels, "llama-7b")
}

func TestUsageAnalytics_AnalyzeTasks(t *testing.T) {
	tempDir := t.TempDir()
	analytics := NewUsageAnalytics(tempDir)
	ctx := context.Background()

	metrics := &UsageMetrics{
		Timestamp: time.Now(),
		LatencyMs: 150.0,
	}
	analytics.RecordTaskPattern(ctx, "code_generation", "llama-7b", 0.7, metrics)

	analysis := analytics.analyzeTasks()
	assert.NotNil(t, analysis["code_generation"])
	assert.Equal(t, "code_generation", analysis["code_generation"].TaskType)
}

func TestUsageAnalytics_AnalyzePerformance(t *testing.T) {
	tempDir := t.TempDir()
	analytics := NewUsageAnalytics(tempDir)
	ctx := context.Background()

	// Record some usage to create performance history
	metrics := &UsageMetrics{
		Timestamp:    time.Now(),
		LatencyMs:    100.0,
		Success:      true,
		InputTokens:  100,
		OutputTokens: 50,
	}
	analytics.RecordModelUsage(ctx, "llama-7b", "ollama", "user1", metrics)

	analysis := analytics.analyzePerformance()
	assert.NotNil(t, analysis)
	assert.NotNil(t, analysis.BottleneckAnalysis)
	assert.NotNil(t, analysis.OptimizationImpact)
	assert.NotNil(t, analysis.OptimalProviders)
}

func TestUsageAnalytics_AnalyzeUsers(t *testing.T) {
	tempDir := t.TempDir()
	analytics := NewUsageAnalytics(tempDir)
	ctx := context.Background()

	// Add user preferences
	prefs := &UserPreferences{
		UserID:             "user1",
		PreferredProviders: []string{"ollama"},
		QualityPreference:  "fast",
	}
	analytics.SetUserPreferences(ctx, prefs)

	analysis := analytics.analyzeUsers()
	assert.Equal(t, int64(1), analysis.TotalUsers)
	assert.NotNil(t, analysis.UserRetention)
	assert.NotEmpty(t, analysis.BehavioralTrends)
}

func TestUsageAnalytics_GenerateRecommendations(t *testing.T) {
	tempDir := t.TempDir()
	analytics := NewUsageAnalytics(tempDir)

	t.Run("RecommendsForLowTPS", func(t *testing.T) {
		report := &UsageReport{
			Summary: &UsageSummary{},
			PerformanceAnalysis: &PerformanceAnalysis{
				AverageTPS:         5.0, // Low TPS
				OptimizationImpact: &OptimizationImpact{},
			},
			UserAnalysis: &UserAnalysis{
				UserRetention: &UserRetention{MonthlyRetention: 0.6},
			},
			TaskAnalysis: make(map[string]*TaskAnalysis),
		}

		recommendations := analytics.generateRecommendations(report)
		assert.Contains(t, recommendations, "Consider optimizing providers for better throughput")
	})

	t.Run("RecommendsForLowRetention", func(t *testing.T) {
		report := &UsageReport{
			Summary: &UsageSummary{},
			PerformanceAnalysis: &PerformanceAnalysis{
				AverageTPS:         50.0,
				OptimizationImpact: &OptimizationImpact{},
			},
			UserAnalysis: &UserAnalysis{
				UserRetention: &UserRetention{MonthlyRetention: 0.3}, // Low retention
			},
			TaskAnalysis: make(map[string]*TaskAnalysis),
		}

		recommendations := analytics.generateRecommendations(report)
		found := false
		for _, rec := range recommendations {
			if rec == "Focus on improving user experience to increase retention" {
				found = true
				break
			}
		}
		assert.True(t, found)
	})

	t.Run("RecommendsForHighLatency", func(t *testing.T) {
		report := &UsageReport{
			Summary: &UsageSummary{},
			PerformanceAnalysis: &PerformanceAnalysis{
				AverageTPS:         50.0,
				OptimizationImpact: &OptimizationImpact{},
			},
			UserAnalysis: &UserAnalysis{
				UserRetention: &UserRetention{MonthlyRetention: 0.6},
			},
			TaskAnalysis: map[string]*TaskAnalysis{
				"slow_task": {
					TaskType:       "slow_task",
					AverageLatency: 2000.0, // High latency
				},
			},
		}

		recommendations := analytics.generateRecommendations(report)
		found := false
		for _, rec := range recommendations {
			if rec == "Task 'slow_task' has high latency (2000ms), consider using faster providers or optimizing" {
				found = true
				break
			}
		}
		assert.True(t, found)
	})
}

func TestUsageAnalytics_SaveAndLoad(t *testing.T) {
	tempDir := t.TempDir()

	t.Run("SavesData", func(t *testing.T) {
		analytics := NewUsageAnalytics(tempDir)
		ctx := context.Background()

		// Record some data
		metrics := &UsageMetrics{
			Timestamp: time.Now(),
			LatencyMs: 100.0,
			Success:   true,
		}
		analytics.RecordModelUsage(ctx, "llama-7b", "ollama", "user1", metrics)

		// Check files exist
		_, err := os.Stat(filepath.Join(analytics.analyticsDir, "model_usage_stats.json"))
		assert.NoError(t, err)
	})

	t.Run("LoadsData", func(t *testing.T) {
		// Create a new analytics instance that should load existing data
		analytics := NewUsageAnalytics(tempDir)

		// Should have loaded the previous data
		_, exists := analytics.ModelUsageStats["llama-7b"]
		assert.True(t, exists)
	})
}

func TestUsageMetrics_Struct(t *testing.T) {
	metrics := &UsageMetrics{
		Timestamp:      time.Now(),
		LatencyMs:      150.5,
		Success:        true,
		UserRating:     4.5,
		TaskType:       "code_generation",
		InputTokens:    1000,
		OutputTokens:   500,
		MemoryUsage:    4096,
		GPUUtilization: 85.5,
		CPUUtilization: 45.2,
		Provider:       "ollama",
		ModelVersion:   "1.0",
		ErrorType:      "",
		ErrorCode:      "",
	}

	assert.Equal(t, 150.5, metrics.LatencyMs)
	assert.True(t, metrics.Success)
	assert.Equal(t, 4.5, metrics.UserRating)
	assert.Equal(t, "code_generation", metrics.TaskType)
	assert.Equal(t, int64(1000), metrics.InputTokens)
	assert.Equal(t, "ollama", metrics.Provider)
}

func TestUsageReport_Struct(t *testing.T) {
	report := &UsageReport{
		GeneratedAt: time.Now(),
		TimeRange: TimeRange{
			Start: time.Now().Add(-24 * time.Hour),
			End:   time.Now(),
		},
		Summary:             &UsageSummary{TotalModels: 5},
		TopModels:           []*ModelUsageStats{},
		TaskAnalysis:        make(map[string]*TaskAnalysis),
		PerformanceAnalysis: &PerformanceAnalysis{AverageTPS: 25.0},
		UserAnalysis:        &UserAnalysis{TotalUsers: 10},
		Recommendations:     []string{"Test recommendation"},
	}

	assert.NotZero(t, report.GeneratedAt)
	assert.Equal(t, 5, report.Summary.TotalModels)
	assert.Equal(t, 25.0, report.PerformanceAnalysis.AverageTPS)
	assert.Equal(t, int64(10), report.UserAnalysis.TotalUsers)
}

func TestUsageSummary_Struct(t *testing.T) {
	summary := &UsageSummary{
		TotalModels:         10,
		TotalRequests:       1000,
		AverageLatency:      150.0,
		OverallSuccessRate:  0.95,
		AverageSatisfaction: 4.5,
		MostUsedProviders:   []string{"ollama", "vllm"},
		TrendingModels:      []string{"llama-7b", "codellama"},
		PerformanceTrends:   map[string]float64{"tps": 25.0},
	}

	assert.Equal(t, 10, summary.TotalModels)
	assert.Equal(t, int64(1000), summary.TotalRequests)
	assert.Equal(t, 0.95, summary.OverallSuccessRate)
	assert.Contains(t, summary.MostUsedProviders, "ollama")
}

func TestTaskAnalysis_Struct(t *testing.T) {
	analysis := &TaskAnalysis{
		TaskType:          "code_generation",
		Frequency:         50,
		AverageComplexity: 0.7,
		PreferredModels:   []string{"codellama"},
		AverageLatency:    200.0,
		SuccessRate:       0.9,
		PeakHours:         []string{"09:00", "14:00"},
		Trends:            []string{"increasing"},
	}

	assert.Equal(t, "code_generation", analysis.TaskType)
	assert.Equal(t, int64(50), analysis.Frequency)
	assert.Equal(t, 0.7, analysis.AverageComplexity)
}

func TestPerformanceAnalysis_Struct(t *testing.T) {
	analysis := &PerformanceAnalysis{
		AverageTPS: 25.0,
		OptimalProviders: map[string]float64{
			"vllm": 30.0,
		},
		BottleneckAnalysis: &BottleneckAnalysis{
			MemoryBottleneck: true,
			AffectedModels:   []string{"llama-70b"},
		},
		OptimizationImpact: &OptimizationImpact{
			TotalOptimizations: 10,
			AverageImprovement: 25.0,
		},
		Recommendations: []string{"Add more RAM"},
	}

	assert.Equal(t, 25.0, analysis.AverageTPS)
	assert.True(t, analysis.BottleneckAnalysis.MemoryBottleneck)
	assert.Equal(t, int64(10), analysis.OptimizationImpact.TotalOptimizations)
}

func TestUserAnalysis_Struct(t *testing.T) {
	analysis := &UserAnalysis{
		TotalUsers:             100,
		AverageRequestsPerUser: 10.5,
		UserSegments: map[string]int64{
			"power_users": 20,
		},
		PreferredProviders: map[string][]string{
			"ollama": {"user1", "user2"},
		},
		UserRetention: &UserRetention{
			DailyRetention:   0.9,
			WeeklyRetention:  0.8,
			MonthlyRetention: 0.6,
			ChurnRate:        0.05,
		},
		BehavioralTrends: []string{"increasing usage"},
	}

	assert.Equal(t, int64(100), analysis.TotalUsers)
	assert.Equal(t, 10.5, analysis.AverageRequestsPerUser)
	assert.Equal(t, 0.9, analysis.UserRetention.DailyRetention)
}

func TestContainsHelper(t *testing.T) {
	t.Run("FindsItem", func(t *testing.T) {
		slice := []string{"a", "b", "c"}
		assert.True(t, contains(slice, "b"))
	})

	t.Run("DoesNotFindItem", func(t *testing.T) {
		slice := []string{"a", "b", "c"}
		assert.False(t, contains(slice, "d"))
	})

	t.Run("EmptySlice", func(t *testing.T) {
		slice := []string{}
		assert.False(t, contains(slice, "a"))
	})
}

func TestUsageAnalytics_UpdateAverageMetrics(t *testing.T) {
	tempDir := t.TempDir()
	analytics := NewUsageAnalytics(tempDir)

	t.Run("UpdatesAverages", func(t *testing.T) {
		history := &PerformanceHistory{
			TimeSeries: []PerformanceDataPoint{
				{TokensPerSecond: 20.0, MemoryUsage: 4000, Latency: 100, SuccessRate: 1.0, UserRating: 4.0},
				{TokensPerSecond: 30.0, MemoryUsage: 5000, Latency: 150, SuccessRate: 1.0, UserRating: 5.0},
			},
		}

		analytics.updateAverageMetrics(history)

		assert.NotNil(t, history.AverageMetrics)
		assert.Equal(t, 25.0, history.AverageMetrics.TokensPerSecond)
		assert.Equal(t, int64(4500), history.AverageMetrics.MemoryUsage)
		assert.Equal(t, int64(125), history.AverageMetrics.Latency)
	})

	t.Run("HandlesEmptyTimeSeries", func(t *testing.T) {
		history := &PerformanceHistory{
			TimeSeries: []PerformanceDataPoint{},
		}

		analytics.updateAverageMetrics(history)
		assert.Nil(t, history.AverageMetrics)
	})
}
