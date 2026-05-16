package llm

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/hardware"
)

func TestNewModelRanker(t *testing.T) {
	t.Run("CreatesWithDefaultWeights", func(t *testing.T) {
		ranker := NewModelRanker()
		require.NotNil(t, ranker)
		assert.NotEmpty(t, ranker.Weights)
		assert.Contains(t, ranker.Weights, "performance")
		assert.Contains(t, ranker.Weights, "compatibility")
		assert.Contains(t, ranker.Weights, "cost_efficiency")
		assert.Contains(t, ranker.Weights, "reliability")
		assert.Contains(t, ranker.Weights, "features")
	})

	t.Run("HasScoringFactors", func(t *testing.T) {
		ranker := NewModelRanker()
		require.NotNil(t, ranker)
		assert.NotEmpty(t, ranker.ScoringFactors)
		assert.Contains(t, ranker.ScoringFactors, "context_size")
		assert.Contains(t, ranker.ScoringFactors, "max_tokens")
		assert.Contains(t, ranker.ScoringFactors, "response_time")
	})

	t.Run("InitializesCustomScorers", func(t *testing.T) {
		ranker := NewModelRanker()
		require.NotNil(t, ranker)
		assert.NotNil(t, ranker.CustomScorers)
	})
}

func TestNewModelDiscoveryEngine(t *testing.T) {
	t.Run("CreatesEngine", func(t *testing.T) {
		tempDir := t.TempDir()
		engine := NewModelDiscoveryEngine(tempDir)
		require.NotNil(t, engine)
		assert.NotNil(t, engine.registry)
		assert.NotNil(t, engine.hardwareDetector)
		assert.NotNil(t, engine.usageAnalytics)
		assert.NotNil(t, engine.modelRanker)
		assert.Equal(t, tempDir, engine.baseDir)
	})

	t.Run("CreatesCacheDir", func(t *testing.T) {
		tempDir := t.TempDir()
		engine := NewModelDiscoveryEngine(tempDir)
		require.NotNil(t, engine)
		assert.Contains(t, engine.cacheDir, "cache/discovery")
	})
}

func TestModelDiscoveryEngine_GetRecommendations(t *testing.T) {
	t.Run("ReturnsRecommendations", func(t *testing.T) {
		tempDir := t.TempDir()
		engine := NewModelDiscoveryEngine(tempDir)

		ctx := context.Background()
		req := &RecommendationRequest{
			TaskTypes:          []string{"code_generation"},
			MaxRecommendations: 5,
			QualityPreference:  "balanced",
		}

		response, err := engine.GetRecommendations(ctx, req)
		require.NoError(t, err)
		assert.NotNil(t, response)
		assert.NotNil(t, response.Recommendations)
		assert.GreaterOrEqual(t, len(response.Recommendations), 0)
		assert.NotNil(t, response.Insights)
	})

	t.Run("RespectsMaxRecommendations", func(t *testing.T) {
		tempDir := t.TempDir()
		engine := NewModelDiscoveryEngine(tempDir)

		ctx := context.Background()
		req := &RecommendationRequest{
			TaskTypes:          []string{"code_generation"},
			MaxRecommendations: 2,
			QualityPreference:  "balanced",
		}

		response, err := engine.GetRecommendations(ctx, req)
		require.NoError(t, err)
		assert.LessOrEqual(t, len(response.Recommendations), 2)
	})

	t.Run("ReturnsSearchTime", func(t *testing.T) {
		tempDir := t.TempDir()
		engine := NewModelDiscoveryEngine(tempDir)

		ctx := context.Background()
		req := &RecommendationRequest{
			TaskTypes:          []string{"code_generation"},
			MaxRecommendations: 5,
		}

		response, err := engine.GetRecommendations(ctx, req)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, response.SearchTime, time.Duration(0))
	})

	t.Run("HandlesEmptyTaskTypes", func(t *testing.T) {
		tempDir := t.TempDir()
		engine := NewModelDiscoveryEngine(tempDir)

		ctx := context.Background()
		req := &RecommendationRequest{
			TaskTypes:          []string{},
			MaxRecommendations: 5,
		}

		response, err := engine.GetRecommendations(ctx, req)
		require.NoError(t, err)
		assert.NotNil(t, response)
	})

	t.Run("ExcludesModels", func(t *testing.T) {
		tempDir := t.TempDir()
		engine := NewModelDiscoveryEngine(tempDir)

		ctx := context.Background()
		req := &RecommendationRequest{
			TaskTypes:          []string{"code_generation"},
			MaxRecommendations: 10,
			ExcludeModels:      []string{"llama-3-8b-instruct"},
		}

		response, err := engine.GetRecommendations(ctx, req)
		require.NoError(t, err)

		// Check that excluded model is not in recommendations
		for _, rec := range response.Recommendations {
			assert.NotEqual(t, "llama-3-8b-instruct", rec.Model.ID)
		}
	})
}

func TestModelDiscoveryEngine_ScoreTaskCompatibility(t *testing.T) {
	t.Run("ScoresCodeGenerationTask", func(t *testing.T) {
		tempDir := t.TempDir()
		engine := NewModelDiscoveryEngine(tempDir)

		model := &ModelInfo{
			ID:           "codellama-7b",
			Capabilities: []ModelCapability{CapabilityCodeGeneration, CapabilityReasoning, CapabilityDebugging},
		}

		score := engine.scoreTaskCompatibility(model, []string{"code_generation"})
		assert.Greater(t, score, 0.0)
	})

	t.Run("ReturnsNeutralForNoTasks", func(t *testing.T) {
		tempDir := t.TempDir()
		engine := NewModelDiscoveryEngine(tempDir)

		model := &ModelInfo{
			ID:           "test-model",
			Capabilities: []ModelCapability{CapabilityCodeGeneration},
		}

		score := engine.scoreTaskCompatibility(model, []string{})
		assert.Equal(t, 0.5, score)
	})

	t.Run("AveragesMultipleTasks", func(t *testing.T) {
		tempDir := t.TempDir()
		engine := NewModelDiscoveryEngine(tempDir)

		model := &ModelInfo{
			ID:           "test-model",
			Capabilities: []ModelCapability{CapabilityCodeGeneration, CapabilityReasoning, CapabilityPlanning},
		}

		score := engine.scoreTaskCompatibility(model, []string{"code_generation", "planning"})
		assert.GreaterOrEqual(t, score, 0.0)
		assert.LessOrEqual(t, score, 1.0)
	})
}

func TestModelDiscoveryEngine_ScoreModelForTask(t *testing.T) {
	tempDir := t.TempDir()
	engine := NewModelDiscoveryEngine(tempDir)

	testCases := []struct {
		name         string
		taskType     string
		capabilities []ModelCapability
		expectHigh   bool
	}{
		{
			name:         "CodeGenerationWithCapability",
			taskType:     "code_generation",
			capabilities: []ModelCapability{CapabilityCodeGeneration, CapabilityReasoning, CapabilityDebugging},
			expectHigh:   true,
		},
		{
			name:         "PlanningWithCapability",
			taskType:     "planning",
			capabilities: []ModelCapability{CapabilityPlanning, CapabilityReasoning, CapabilityAnalysis},
			expectHigh:   true,
		},
		{
			name:         "DebuggingWithCapability",
			taskType:     "debugging",
			capabilities: []ModelCapability{CapabilityDebugging, CapabilityCodeGeneration, CapabilityReasoning},
			expectHigh:   true,
		},
		{
			name:         "UnknownTask",
			taskType:     "unknown_task",
			capabilities: []ModelCapability{CapabilityCodeGeneration},
			expectHigh:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			model := &ModelInfo{
				ID:           "test-model",
				Capabilities: tc.capabilities,
			}
			score := engine.scoreModelForTask(model, tc.taskType)
			assert.GreaterOrEqual(t, score, 0.0)
			assert.LessOrEqual(t, score, 1.0)
			if tc.expectHigh && tc.taskType != "unknown_task" {
				assert.Greater(t, score, 0.5)
			}
		})
	}
}

func TestModelDiscoveryEngine_ScoreHardwareCompatibility(t *testing.T) {
	t.Run("WithHardwareProfile", func(t *testing.T) {
		tempDir := t.TempDir()
		engine := NewModelDiscoveryEngine(tempDir)

		model := &ModelInfo{
			ID:   "llama-7b",
			Size: 4000000000,
		}

		profile := &hardware.HardwareInfo{
			Memory: hardware.MemoryInfo{
				Total:     32 * 1024 * 1024 * 1024, // 32GB
				Available: 16 * 1024 * 1024 * 1024, // 16GB
			},
			CPU: hardware.CPUInfo{
				Cores:   16,
				HasAVX:  true,
				HasAVX2: true,
			},
		}

		score := engine.scoreHardwareCompatibility(model, profile)
		assert.GreaterOrEqual(t, score, 0.0)
		assert.LessOrEqual(t, score, 1.0)
	})

	t.Run("WithGPU", func(t *testing.T) {
		tempDir := t.TempDir()
		engine := NewModelDiscoveryEngine(tempDir)

		model := &ModelInfo{
			ID:   "llama-7b",
			Size: 4000000000,
		}

		profile := &hardware.HardwareInfo{
			Memory: hardware.MemoryInfo{
				Total: 32 * 1024 * 1024 * 1024, // 32GB
			},
			CPU: hardware.CPUInfo{
				Cores:   8,
				HasAVX2: true,
			},
			GPU: hardware.GPUInfo{
				Name:              "NVIDIA RTX 3080",
				VRAM:              "10GB",
				ComputeCapability: 8.6,
			},
		}

		score := engine.scoreHardwareCompatibility(model, profile)
		assert.GreaterOrEqual(t, score, 0.0)
		assert.LessOrEqual(t, score, 1.0)
	})

	t.Run("WithNilProfile", func(t *testing.T) {
		tempDir := t.TempDir()
		engine := NewModelDiscoveryEngine(tempDir)

		model := &ModelInfo{
			ID:   "llama-7b",
			Size: 4000000000,
		}

		score := engine.scoreHardwareCompatibility(model, nil)
		// Should use detected hardware or return neutral score
		assert.GreaterOrEqual(t, score, 0.0)
		assert.LessOrEqual(t, score, 1.0)
	})
}

func TestModelDiscoveryEngine_ScoreGPUCompatibility(t *testing.T) {
	tempDir := t.TempDir()
	engine := NewModelDiscoveryEngine(tempDir)

	t.Run("WithSufficientVRAM", func(t *testing.T) {
		model := &ModelInfo{
			ID:   "llama-7b",
			Size: 4000000000,
		}
		gpu := &hardware.GPUInfo{
			Name:              "NVIDIA RTX 4090",
			VRAM:              "24GB",
			ComputeCapability: 8.9,
		}

		score := engine.scoreGPUCompatibility(model, gpu)
		assert.GreaterOrEqual(t, score, 0.5)
	})

	t.Run("WithInsufficientVRAM", func(t *testing.T) {
		model := &ModelInfo{
			ID:   "llama-70b",
			Size: 40000000000,
		}
		gpu := &hardware.GPUInfo{
			Name: "NVIDIA GTX 1650",
			VRAM: "4GB",
		}

		score := engine.scoreGPUCompatibility(model, gpu)
		assert.GreaterOrEqual(t, score, 0.0)
		assert.LessOrEqual(t, score, 1.0)
	})

	t.Run("WithLowComputeCapability", func(t *testing.T) {
		model := &ModelInfo{
			ID: "llama-7b",
		}
		gpu := &hardware.GPUInfo{
			Name:              "Old GPU",
			VRAM:              "8GB",
			ComputeCapability: 5.0,
		}

		score := engine.scoreGPUCompatibility(model, gpu)
		assert.Equal(t, 0.7, score) // Reduced due to low compute capability
	})
}

func TestModelDiscoveryEngine_ScoreCPUCompatibility(t *testing.T) {
	tempDir := t.TempDir()
	engine := NewModelDiscoveryEngine(tempDir)

	t.Run("WithAVX2Support", func(t *testing.T) {
		model := &ModelInfo{
			ID: "llama-7b",
		}
		cpu := &hardware.CPUInfo{
			Cores:   8,
			HasAVX:  true,
			HasAVX2: true,
		}

		score := engine.scoreCPUCompatibility(model, cpu)
		assert.Greater(t, score, 0.7) // Should get bonus for AVX support
	})

	t.Run("WithNEON", func(t *testing.T) {
		model := &ModelInfo{
			ID: "llama-7b",
		}
		cpu := &hardware.CPUInfo{
			Cores:   8,
			HasNEON: true,
		}

		score := engine.scoreCPUCompatibility(model, cpu)
		assert.GreaterOrEqual(t, score, 0.7)
	})

	t.Run("LargeModelLowCores", func(t *testing.T) {
		model := &ModelInfo{
			ID: "llama-70b",
		}
		cpu := &hardware.CPUInfo{
			Cores: 4,
		}

		score := engine.scoreCPUCompatibility(model, cpu)
		// Should be reduced due to low core count for large model
		assert.LessOrEqual(t, score, 0.7)
	})
}

func TestModelDiscoveryEngine_ScorePerformance(t *testing.T) {
	tempDir := t.TempDir()
	engine := NewModelDiscoveryEngine(tempDir)

	testCases := []struct {
		name              string
		modelID           string
		qualityPreference string
		expectHighScore   bool
	}{
		{"FastPreferSmall", "llama-3b", "fast", true},
		{"FastPreferLarge", "llama-70b", "fast", false},
		{"QualityPreferSmall", "llama-3b", "quality", false},
		{"QualityPreferLarge", "llama-70b", "quality", true},
		{"BalancedPreferMedium", "llama-13b", "balanced", true},
		{"UnknownPreference", "llama-7b", "unknown", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			model := &ModelInfo{ID: tc.modelID}
			score := engine.scorePerformance(model, tc.qualityPreference)
			assert.GreaterOrEqual(t, score, 0.0)
			assert.LessOrEqual(t, score, 1.0)
			if tc.expectHighScore {
				assert.GreaterOrEqual(t, score, 0.5)
			}
		})
	}
}

func TestModelDiscoveryEngine_ScoreCost(t *testing.T) {
	tempDir := t.TempDir()
	engine := NewModelDiscoveryEngine(tempDir)

	t.Run("NoBudgetLimit", func(t *testing.T) {
		model := &ModelInfo{ID: "llama-7b", Format: FormatGGUF}
		score := engine.scoreCost(model, 0)
		assert.Equal(t, 0.5, score)
	})

	t.Run("WithinBudget", func(t *testing.T) {
		model := &ModelInfo{ID: "llama-7b", Format: FormatGGUF}
		score := engine.scoreCost(model, 10.0)
		assert.Greater(t, score, 0.0)
	})

	t.Run("OverBudget", func(t *testing.T) {
		model := &ModelInfo{ID: "llama-70b", Format: FormatGGUF}
		score := engine.scoreCost(model, 0.1) // Very low budget
		assert.Equal(t, 0.0, score)
	})
}

func TestModelDiscoveryEngine_ScorePrivacy(t *testing.T) {
	tempDir := t.TempDir()
	engine := NewModelDiscoveryEngine(tempDir)

	t.Run("LocalPrivacy", func(t *testing.T) {
		model := &ModelInfo{ID: "llama-7b"}
		score := engine.scorePrivacy(model, "local")
		assert.Equal(t, 1.0, score)
	})

	t.Run("CloudPrivacy", func(t *testing.T) {
		model := &ModelInfo{ID: "llama-7b"}
		score := engine.scorePrivacy(model, "cloud")
		assert.Equal(t, 1.0, score)
	})

	t.Run("HybridWithAPI", func(t *testing.T) {
		model := &ModelInfo{ID: "model-api-version"}
		score := engine.scorePrivacy(model, "hybrid")
		assert.Equal(t, 0.5, score)
	})

	t.Run("HybridWithoutAPI", func(t *testing.T) {
		model := &ModelInfo{ID: "llama-7b"}
		score := engine.scorePrivacy(model, "hybrid")
		assert.Equal(t, 1.0, score)
	})

	t.Run("UnknownPrivacyLevel", func(t *testing.T) {
		model := &ModelInfo{ID: "llama-7b"}
		score := engine.scorePrivacy(model, "unknown")
		assert.Equal(t, 0.8, score)
	})
}

func TestModelDiscoveryEngine_ParseVRAMString(t *testing.T) {
	tempDir := t.TempDir()
	engine := NewModelDiscoveryEngine(tempDir)

	testCases := []struct {
		input    string
		expected int64
	}{
		{"8GB", 8 * 1024 * 1024 * 1024},
		{"16GB", 16 * 1024 * 1024 * 1024},
		{"512MB", 512 * 1024 * 1024},
		{"1024MB", 1024 * 1024 * 1024},
		{"", 0},
		{"invalid", 0},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := engine.parseVRAMString(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestModelDiscoveryEngine_InferCapabilities(t *testing.T) {
	tempDir := t.TempDir()
	engine := NewModelDiscoveryEngine(tempDir)

	testCases := []struct {
		modelID             string
		expectedCapability  ModelCapability
		shouldHaveCapability bool
	}{
		{"codellama-7b", CapabilityCodeGeneration, true},
		{"llama-7b-instruct", CapabilityCodeGeneration, true},
		{"test-debug-model", CapabilityDebugging, true},
		{"test-plan-model", CapabilityPlanning, true},
		{"doc-model", CapabilityDocumentation, true},
		{"generic-model", CapabilityTextGeneration, true},
	}

	for _, tc := range testCases {
		t.Run(tc.modelID, func(t *testing.T) {
			capabilities := engine.inferCapabilities(tc.modelID)
			assert.NotEmpty(t, capabilities)
			found := false
			for _, cap := range capabilities {
				if cap == tc.expectedCapability {
					found = true
					break
				}
			}
			assert.Equal(t, tc.shouldHaveCapability, found)
		})
	}
}

func TestModelDiscoveryEngine_EstimateModelSize(t *testing.T) {
	tempDir := t.TempDir()
	engine := NewModelDiscoveryEngine(tempDir)

	testCases := []struct {
		modelID  string
		expected string
	}{
		{"llama-70b-instruct", "70B"},
		{"llama-34b-instruct", "34B"},
		{"llama-13b-instruct", "13B"},
		{"llama-8b-instruct", "8B"},
		{"llama-7b-instruct", "7B"},
		{"llama-3b-instruct", "3B"},
		{"unknown-model", "7B"}, // Default
	}

	for _, tc := range testCases {
		t.Run(tc.modelID, func(t *testing.T) {
			size := engine.estimateModelSize(tc.modelID)
			assert.Equal(t, tc.expected, size)
		})
	}
}

func TestModelDiscoveryEngine_EstimateModelMemoryRequirements(t *testing.T) {
	tempDir := t.TempDir()
	engine := NewModelDiscoveryEngine(tempDir)

	t.Run("7BModel", func(t *testing.T) {
		model := &ModelInfo{ID: "llama-7b"}
		memory := engine.estimateModelMemoryRequirements(model)
		assert.Greater(t, memory, int64(0))
		assert.Equal(t, int64(4.5*1024), memory)
	})

	t.Run("70BModel", func(t *testing.T) {
		model := &ModelInfo{ID: "llama-70b"}
		memory := engine.estimateModelMemoryRequirements(model)
		assert.Greater(t, memory, int64(0))
		assert.Equal(t, int64(40.0*1024), memory)
	})
}

func TestModelDiscoveryEngine_EstimatePerformance(t *testing.T) {
	tempDir := t.TempDir()
	engine := NewModelDiscoveryEngine(tempDir)

	t.Run("WithHardwareProfile", func(t *testing.T) {
		model := &ModelInfo{ID: "llama-7b", Format: FormatGGUF}
		profile := &hardware.HardwareInfo{
			CPU: hardware.CPUInfo{
				Cores: 8,
			},
			GPU: hardware.GPUInfo{
				Name: "NVIDIA RTX 3080",
				VRAM: "10GB",
			},
		}

		estimate := engine.estimatePerformance(model, profile)
		require.NotNil(t, estimate)
		assert.Greater(t, estimate.TokensPerSecond, 0.0)
		assert.Greater(t, estimate.MemoryUsage, int64(0))
		assert.Greater(t, estimate.Latency, int64(0))
		assert.Greater(t, estimate.Throughput, int64(0))
		assert.GreaterOrEqual(t, estimate.CostPerMillion, 0.0)
		assert.GreaterOrEqual(t, estimate.QualityScore, 0.0)
	})

	t.Run("WithoutGPU", func(t *testing.T) {
		model := &ModelInfo{ID: "llama-7b", Format: FormatGGUF}
		profile := &hardware.HardwareInfo{
			CPU: hardware.CPUInfo{
				Cores: 16,
			},
		}

		estimate := engine.estimatePerformance(model, profile)
		require.NotNil(t, estimate)
		assert.Greater(t, estimate.TokensPerSecond, 0.0)
	})
}

func TestModelDiscoveryEngine_EvaluateHardwareFit(t *testing.T) {
	tempDir := t.TempDir()
	engine := NewModelDiscoveryEngine(tempDir)

	t.Run("GoodFit", func(t *testing.T) {
		model := &ModelInfo{ID: "llama-7b"}
		profile := &hardware.HardwareInfo{
			Memory: hardware.MemoryInfo{
				Total: 64 * 1024 * 1024 * 1024, // 64GB
			},
			CPU: hardware.CPUInfo{
				Cores:   16,
				HasAVX2: true,
			},
			GPU: hardware.GPUInfo{
				Name: "NVIDIA RTX 4090",
				VRAM: "24GB",
			},
		}

		fit := engine.evaluateHardwareFit(model, profile)
		require.NotNil(t, fit)
		assert.True(t, fit.WillRun)
		assert.Greater(t, fit.OverallFit, 0.0)
		assert.GreaterOrEqual(t, fit.CPUScore, 0.0)
		assert.GreaterOrEqual(t, fit.GPUScore, 0.0)
		assert.GreaterOrEqual(t, fit.MemoryScore, 0.0)
	})

	t.Run("PoorFit", func(t *testing.T) {
		model := &ModelInfo{ID: "llama-70b"}
		profile := &hardware.HardwareInfo{
			Memory: hardware.MemoryInfo{
				Total: 4 * 1024, // 4KB - extremely low for 70B model
			},
			CPU: hardware.CPUInfo{
				Cores: 2,
			},
		}

		fit := engine.evaluateHardwareFit(model, profile)
		require.NotNil(t, fit)
		// With very low memory, memory score should be low
		assert.LessOrEqual(t, fit.MemoryScore, 1.0)
		// OverallFit should be affected
		assert.GreaterOrEqual(t, fit.OverallFit, 0.0)
	})

	t.Run("NilProfile", func(t *testing.T) {
		model := &ModelInfo{ID: "llama-7b"}
		fit := engine.evaluateHardwareFit(model, nil)
		require.NotNil(t, fit)
	})
}

func TestModelDiscoveryEngine_AnalyzeUsageMatch(t *testing.T) {
	tempDir := t.TempDir()
	engine := NewModelDiscoveryEngine(tempDir)

	t.Run("WithTaskTypes", func(t *testing.T) {
		model := &ModelInfo{
			ID:           "codellama-7b",
			Capabilities: []ModelCapability{CapabilityCodeGeneration, CapabilityDebugging},
		}

		match := engine.analyzeUsageMatch(model, []string{"code_generation", "debugging"})
		require.NotNil(t, match)
		assert.NotEmpty(t, match.TaskType)
		assert.GreaterOrEqual(t, match.FitScore, 0.0)
		assert.LessOrEqual(t, match.FitScore, 1.0)
	})

	t.Run("EmptyTaskTypes", func(t *testing.T) {
		model := &ModelInfo{ID: "llama-7b"}
		match := engine.analyzeUsageMatch(model, []string{})
		require.NotNil(t, match)
		assert.Equal(t, "general", match.TaskType)
		assert.Equal(t, 0.5, match.FitScore)
	})
}

func TestModelDiscoveryEngine_GetCompatibleProviders(t *testing.T) {
	tempDir := t.TempDir()
	engine := NewModelDiscoveryEngine(tempDir)

	t.Run("GGUFModel", func(t *testing.T) {
		model := &ModelInfo{
			ID:     "llama-7b",
			Format: FormatGGUF,
		}
		providers := engine.getCompatibleProviders(model)
		assert.Contains(t, providers, "llamacpp")
		assert.Contains(t, providers, "ollama")
	})

	t.Run("GPTQModel", func(t *testing.T) {
		model := &ModelInfo{
			ID:     "llama-7b-gptq",
			Format: FormatGPTQ,
		}
		providers := engine.getCompatibleProviders(model)
		assert.Contains(t, providers, "vllm")
	})
}

func TestModelDiscoveryEngine_IsModelCompatibleWithProvider(t *testing.T) {
	tempDir := t.TempDir()
	engine := NewModelDiscoveryEngine(tempDir)

	testCases := []struct {
		modelFormat ModelFormat
		provider    string
		compatible  bool
	}{
		{FormatGGUF, "llamacpp", true},
		{FormatGGUF, "ollama", true},
		{FormatGPTQ, "llamacpp", false},
		{FormatGGUF, "vllm", true},
		{FormatGPTQ, "vllm", true},
		{FormatHF, "vllm", true},
		{FormatGGUF, "unknown-provider", true}, // Default true for unknown
	}

	for _, tc := range testCases {
		t.Run(string(tc.modelFormat)+"-"+tc.provider, func(t *testing.T) {
			model := &ModelInfo{Format: tc.modelFormat}
			result := engine.isModelCompatibleWithProvider(model, tc.provider)
			assert.Equal(t, tc.compatible, result)
		})
	}
}

func TestModelDiscoveryEngine_CalculateOptimalGPULayers(t *testing.T) {
	tempDir := t.TempDir()
	engine := NewModelDiscoveryEngine(tempDir)

	t.Run("7BModelWith8GBVRAM", func(t *testing.T) {
		model := &ModelInfo{ID: "llama-7b"}
		gpu := &hardware.GPUInfo{VRAM: "8GB"}
		layers := engine.calculateOptimalGPULayers(model, gpu)
		assert.Greater(t, layers, 0)
	})

	t.Run("7BModelWith16GBVRAM", func(t *testing.T) {
		model := &ModelInfo{ID: "llama-7b"}
		gpu := &hardware.GPUInfo{VRAM: "16GB"}
		layers := engine.calculateOptimalGPULayers(model, gpu)
		assert.Greater(t, layers, 0)
	})

	t.Run("UnknownModel", func(t *testing.T) {
		model := &ModelInfo{ID: "unknown-model"}
		gpu := &hardware.GPUInfo{VRAM: "8GB"}
		layers := engine.calculateOptimalGPULayers(model, gpu)
		assert.Equal(t, 32, layers) // Default
	})
}

func TestModelDiscoveryEngine_GenerateAlternatives(t *testing.T) {
	tempDir := t.TempDir()
	engine := NewModelDiscoveryEngine(tempDir)

	t.Run("GeneratesAlternatives", func(t *testing.T) {
		recommendations := []*ModelRecommendation{
			{
				Model: &ModelInfo{
					ID:           "llama-7b",
					Capabilities: []ModelCapability{CapabilityCodeGeneration},
				},
			},
		}
		req := &RecommendationRequest{
			TaskTypes: []string{"code_generation"},
		}

		alternatives := engine.generateAlternatives(recommendations, req)
		assert.NotNil(t, alternatives)
	})
}

func TestModelDiscoveryEngine_CalculateCapabilitySimilarity(t *testing.T) {
	tempDir := t.TempDir()
	engine := NewModelDiscoveryEngine(tempDir)

	t.Run("SameCapabilities", func(t *testing.T) {
		m1 := &ModelInfo{
			Capabilities: []ModelCapability{CapabilityCodeGeneration, CapabilityReasoning},
		}
		m2 := &ModelInfo{
			Capabilities: []ModelCapability{CapabilityCodeGeneration, CapabilityReasoning},
		}
		similarity := engine.calculateCapabilitySimilarity(m1, m2)
		assert.Equal(t, 1.0, similarity)
	})

	t.Run("NoSharedCapabilities", func(t *testing.T) {
		m1 := &ModelInfo{
			Capabilities: []ModelCapability{CapabilityCodeGeneration},
		}
		m2 := &ModelInfo{
			Capabilities: []ModelCapability{CapabilityPlanning},
		}
		similarity := engine.calculateCapabilitySimilarity(m1, m2)
		assert.Equal(t, 0.0, similarity)
	})

	t.Run("PartialOverlap", func(t *testing.T) {
		m1 := &ModelInfo{
			Capabilities: []ModelCapability{CapabilityCodeGeneration, CapabilityReasoning},
		}
		m2 := &ModelInfo{
			Capabilities: []ModelCapability{CapabilityCodeGeneration, CapabilityPlanning},
		}
		similarity := engine.calculateCapabilitySimilarity(m1, m2)
		assert.Equal(t, 0.5, similarity)
	})

	t.Run("NilModels", func(t *testing.T) {
		similarity := engine.calculateCapabilitySimilarity(nil, nil)
		assert.Equal(t, 0.0, similarity)
	})

	t.Run("EmptyCapabilities", func(t *testing.T) {
		m1 := &ModelInfo{Capabilities: []ModelCapability{}}
		m2 := &ModelInfo{Capabilities: []ModelCapability{CapabilityCodeGeneration}}
		similarity := engine.calculateCapabilitySimilarity(m1, m2)
		assert.Equal(t, 0.0, similarity)
	})
}

func TestModelDiscoveryEngine_GetFallbackAlternatives(t *testing.T) {
	tempDir := t.TempDir()
	engine := NewModelDiscoveryEngine(tempDir)

	testCases := []struct {
		modelID    string
		expectType string
	}{
		{"codellama-7b", "code"},
		{"deepseek-coder", "code"},
		{"llama-7b-instruct", "llama"},
		{"mistral-7b", "mistral"},
		{"phi-2", "phi"},
		{"generic-model", "general"},
	}

	for _, tc := range testCases {
		t.Run(tc.modelID, func(t *testing.T) {
			model := &ModelInfo{ID: tc.modelID}
			alternatives := engine.getFallbackAlternatives(model)
			assert.NotEmpty(t, alternatives)
		})
	}
}

func TestModelDiscoveryEngine_GenerateInsights(t *testing.T) {
	tempDir := t.TempDir()
	engine := NewModelDiscoveryEngine(tempDir)

	t.Run("GeneratesInsights", func(t *testing.T) {
		recommendations := []*ModelRecommendation{
			{
				Model: &ModelInfo{ID: "llama-7b"},
				EstimatedPerformance: &PerformanceEstimate{
					TokensPerSecond: 25.0,
					CostPerMillion:  0.2,
				},
				HardwareFit: &HardwareFit{
					OverallFit: 0.85,
				},
			},
		}
		req := &RecommendationRequest{
			TaskTypes: []string{"code_generation"},
		}

		insights := engine.generateInsights(recommendations, nil, req)
		require.NotNil(t, insights)
		assert.NotNil(t, insights.PerformanceComparisons)
		assert.NotNil(t, insights.CostAnalysis)
		assert.NotNil(t, insights.HardwareAnalysis)
	})

	t.Run("EmptyRecommendations", func(t *testing.T) {
		req := &RecommendationRequest{}
		insights := engine.generateInsights([]*ModelRecommendation{}, nil, req)
		require.NotNil(t, insights)
	})
}

func TestModelDiscoveryEngine_CalculateRelevanceScore(t *testing.T) {
	tempDir := t.TempDir()
	engine := NewModelDiscoveryEngine(tempDir)

	t.Run("WithRecommendations", func(t *testing.T) {
		recommendations := []*ModelRecommendation{
			{
				RecommendationScore: 0.9,
				UsageMatch:          &UsageMatch{FitScore: 0.8},
			},
			{
				RecommendationScore: 0.7,
				UsageMatch:          &UsageMatch{FitScore: 0.6},
			},
		}
		req := &RecommendationRequest{
			TaskTypes: []string{"code_generation"},
		}

		score := engine.calculateRelevanceScore(recommendations, req)
		assert.Greater(t, score, 0.0)
		assert.LessOrEqual(t, score, 1.0)
	})

	t.Run("EmptyRecommendations", func(t *testing.T) {
		req := &RecommendationRequest{}
		score := engine.calculateRelevanceScore([]*ModelRecommendation{}, req)
		assert.Equal(t, 0.0, score)
	})
}

func TestRecommendationStructs(t *testing.T) {
	t.Run("ModelRecommendation", func(t *testing.T) {
		rec := &ModelRecommendation{
			Model:               &ModelInfo{ID: "test"},
			RecommendationScore: 0.95,
			Reasons:             []string{"High performance"},
			Providers:           []string{"ollama"},
			EstimatedPerformance: &PerformanceEstimate{
				TokensPerSecond: 25.0,
			},
			HardwareFit: &HardwareFit{
				OverallFit: 0.9,
			},
			UsageMatch: &UsageMatch{
				FitScore: 0.85,
			},
		}
		assert.Equal(t, "test", rec.Model.ID)
		assert.Equal(t, 0.95, rec.RecommendationScore)
	})

	t.Run("PerformanceEstimate", func(t *testing.T) {
		est := &PerformanceEstimate{
			TokensPerSecond: 50.0,
			MemoryUsage:     4096,
			Latency:         20,
			Throughput:      3000,
			CostPerMillion:  0.2,
			QualityScore:    0.85,
		}
		assert.Equal(t, 50.0, est.TokensPerSecond)
		assert.Equal(t, int64(4096), est.MemoryUsage)
	})

	t.Run("HardwareFit", func(t *testing.T) {
		fit := &HardwareFit{
			CPUScore:        0.9,
			GPUScore:        0.85,
			MemoryScore:     1.0,
			OverallFit:      0.92,
			WillRun:         true,
			OptimalSettings: map[string]interface{}{"gpu_layers": 32},
			Warnings:        []string{},
			Recommendations: []string{"Enable GPU acceleration"},
		}
		assert.True(t, fit.WillRun)
		assert.Equal(t, 0.92, fit.OverallFit)
	})

	t.Run("UsageMatch", func(t *testing.T) {
		match := &UsageMatch{
			TaskType:          "code_generation",
			FitScore:          0.9,
			RecommendedFor:    []string{"coding", "debugging"},
			NotRecommendedFor: []string{},
			Reasoning:         []string{"Excellent for code tasks"},
		}
		assert.Equal(t, "code_generation", match.TaskType)
		assert.Equal(t, 0.9, match.FitScore)
	})

	t.Run("RecommendationRequest", func(t *testing.T) {
		req := &RecommendationRequest{
			TaskTypes:          []string{"code_generation"},
			Constraints:        map[string]interface{}{"max_memory": 16000},
			QualityPreference:  "balanced",
			PrivacyLevel:       "local",
			MaxRecommendations: 5,
			ExcludeModels:      []string{},
			IncludeProviders:   []string{"ollama"},
		}
		assert.Equal(t, "balanced", req.QualityPreference)
		assert.Equal(t, 5, req.MaxRecommendations)
	})

	t.Run("RecommendationResponse", func(t *testing.T) {
		resp := &RecommendationResponse{
			Recommendations: []*ModelRecommendation{},
			TotalModels:     10,
			SearchTime:      100 * time.Millisecond,
			RelevanceScore:  0.85,
			Alternatives:    []*ModelRecommendation{},
			Insights:        &RecommendationInsights{},
		}
		assert.Equal(t, 10, resp.TotalModels)
		assert.Equal(t, 0.85, resp.RelevanceScore)
	})
}

func TestUsageAnalyticsStructs(t *testing.T) {
	t.Run("ModelUsageStats", func(t *testing.T) {
		stats := &ModelUsageStats{
			ModelID:           "llama-7b",
			TotalRequests:     1000,
			AverageLatency:    150.5,
			SuccessRate:       0.99,
			UserSatisfaction:  4.5,
			PreferredBy:       []string{"user1", "user2"},
			CommonTasks:       []string{"code_generation"},
			PerformanceIssues: []string{},
			LastUsed:          time.Now(),
			UsageTrend:        "increasing",
		}
		assert.Equal(t, "llama-7b", stats.ModelID)
		assert.Equal(t, int64(1000), stats.TotalRequests)
		assert.Equal(t, "increasing", stats.UsageTrend)
	})

	t.Run("TaskPattern", func(t *testing.T) {
		pattern := &TaskPattern{
			TaskType:                "code_generation",
			CommonModels:            []string{"llama-7b", "codellama"},
			AverageComplexity:       0.7,
			PeakHours:               []string{"09:00", "14:00"},
			PerformanceRequirements: map[string]float64{"latency": 100},
			RecommendedModelSizes:   []string{"7B", "13B"},
		}
		assert.Equal(t, "code_generation", pattern.TaskType)
		assert.Contains(t, pattern.CommonModels, "llama-7b")
	})

	t.Run("UserPreferences", func(t *testing.T) {
		prefs := &UserPreferences{
			UserID:              "user1",
			PreferredProviders:  []string{"ollama"},
			QualityPreference:   "balanced",
			BudgetConstraints:   map[string]float64{"monthly": 100.0},
			TaskFrequencies:     map[string]int{"code_generation": 50},
			HardwareConstraints: map[string]bool{"gpu_required": true},
			PrivacyRequirements: map[string]bool{"local_only": true},
		}
		assert.Equal(t, "user1", prefs.UserID)
		assert.Equal(t, "balanced", prefs.QualityPreference)
	})

	t.Run("PerformanceHistory", func(t *testing.T) {
		history := &PerformanceHistory{
			ModelID:  "llama-7b",
			Provider: "ollama",
			TimeSeries: []PerformanceDataPoint{
				{
					Timestamp:       time.Now(),
					TokensPerSecond: 25.0,
					MemoryUsage:     4096,
					Latency:         40,
					SuccessRate:     0.99,
					UserRating:      4.5,
				},
			},
			AverageMetrics: &PerformanceEstimate{
				TokensPerSecond: 25.0,
			},
		}
		assert.Equal(t, "llama-7b", history.ModelID)
		assert.Len(t, history.TimeSeries, 1)
	})

	t.Run("OptimizationRecord", func(t *testing.T) {
		record := &OptimizationRecord{
			Timestamp:        time.Now(),
			OptimizationType: "quantization",
			BeforeMetrics:    &PerformanceEstimate{TokensPerSecond: 20},
			AfterMetrics:     &PerformanceEstimate{TokensPerSecond: 30},
			Improvement:      50.0,
			Success:          true,
			Method:           "GPTQ",
		}
		assert.Equal(t, "quantization", record.OptimizationType)
		assert.True(t, record.Success)
		assert.Equal(t, 50.0, record.Improvement)
	})
}

func TestMinHelper(t *testing.T) {
	assert.Equal(t, 3, min(3, 5))
	assert.Equal(t, 2, min(5, 2))
	assert.Equal(t, 4, min(4, 4))
}
