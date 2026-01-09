package llm

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockProvider implements Provider interface for testing
type MockTestProvider struct {
	name         string
	providerType ProviderType
	models       []ModelInfo
	available    bool
	health       *ProviderHealth
	healthErr    error
}

func (m *MockTestProvider) GetType() ProviderType {
	return m.providerType
}

func (m *MockTestProvider) GetName() string {
	return m.name
}

func (m *MockTestProvider) GetModels() []ModelInfo {
	return m.models
}

func (m *MockTestProvider) GetCapabilities() []ModelCapability {
	return []ModelCapability{CapabilityCodeGeneration}
}

func (m *MockTestProvider) Generate(ctx context.Context, req *LLMRequest) (*LLMResponse, error) {
	return nil, nil
}

func (m *MockTestProvider) GenerateStream(ctx context.Context, req *LLMRequest, ch chan<- LLMResponse) error {
	return nil
}

func (m *MockTestProvider) IsAvailable(ctx context.Context) bool {
	return m.available
}

func (m *MockTestProvider) GetHealth(ctx context.Context) (*ProviderHealth, error) {
	return m.health, m.healthErr
}

func (m *MockTestProvider) Close() error {
	return nil
}

// ========================================
// NewModelManager Tests
// ========================================

func TestNewModelManager(t *testing.T) {
	t.Run("creates new instance", func(t *testing.T) {
		mm := NewModelManager()
		require.NotNil(t, mm)
		assert.NotNil(t, mm.hardwareDetector)
		assert.NotNil(t, mm.providers)
		assert.NotNil(t, mm.modelRegistry)
	})
}

// ========================================
// RegisterProvider Tests
// ========================================

func TestModelManager_RegisterProvider(t *testing.T) {
	t.Run("registers provider successfully", func(t *testing.T) {
		mm := NewModelManager()
		provider := &MockTestProvider{
			name:         "test-provider",
			providerType: ProviderTypeOllama,
			models: []ModelInfo{
				{Name: "model1", Provider: ProviderTypeOllama},
				{Name: "model2", Provider: ProviderTypeOllama},
			},
		}

		err := mm.RegisterProvider(provider)
		assert.NoError(t, err)
	})

	t.Run("fails on duplicate provider", func(t *testing.T) {
		mm := NewModelManager()
		provider := &MockTestProvider{
			name:         "test-provider",
			providerType: ProviderTypeOllama,
			models:       []ModelInfo{},
		}

		err := mm.RegisterProvider(provider)
		require.NoError(t, err)

		// Try to register same provider type again
		err = mm.RegisterProvider(provider)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already registered")
	})
}

// ========================================
// GetAvailableModels Tests
// ========================================

func TestModelManager_GetAvailableModels(t *testing.T) {
	t.Run("returns empty for no models", func(t *testing.T) {
		mm := NewModelManager()
		models := mm.GetAvailableModels()
		assert.Empty(t, models)
	})

	t.Run("returns registered models", func(t *testing.T) {
		mm := NewModelManager()
		provider := &MockTestProvider{
			name:         "test-provider",
			providerType: ProviderTypeOllama,
			models: []ModelInfo{
				{Name: "model1", Provider: ProviderTypeOllama},
				{Name: "model2", Provider: ProviderTypeOllama},
			},
		}

		_ = mm.RegisterProvider(provider)
		models := mm.GetAvailableModels()
		assert.Len(t, models, 2)
	})
}

// ========================================
// GetModelsByCapability Tests
// ========================================

func TestModelManager_GetModelsByCapability(t *testing.T) {
	t.Run("returns matching models", func(t *testing.T) {
		mm := NewModelManager()
		provider := &MockTestProvider{
			name:         "test-provider",
			providerType: ProviderTypeOllama,
			models: []ModelInfo{
				{
					Name:         "code-model",
					Provider:     ProviderTypeOllama,
					Capabilities: []ModelCapability{CapabilityCodeGeneration},
				},
				{
					Name:         "chat-model",
					Provider:     ProviderTypeOllama,
					Capabilities: []ModelCapability{CapabilityTextGeneration},
				},
			},
		}

		_ = mm.RegisterProvider(provider)
		matching := mm.GetModelsByCapability([]ModelCapability{CapabilityCodeGeneration})
		assert.Len(t, matching, 1)
		assert.Equal(t, "code-model", matching[0].Name)
	})

	t.Run("returns models with multiple capabilities", func(t *testing.T) {
		mm := NewModelManager()
		provider := &MockTestProvider{
			name:         "test-provider",
			providerType: ProviderTypeOllama,
			models: []ModelInfo{
				{
					Name:         "full-model",
					Provider:     ProviderTypeOllama,
					Capabilities: []ModelCapability{CapabilityCodeGeneration, CapabilityTextGeneration, CapabilityDebugging},
				},
				{
					Name:         "partial-model",
					Provider:     ProviderTypeOllama,
					Capabilities: []ModelCapability{CapabilityCodeGeneration},
				},
			},
		}

		_ = mm.RegisterProvider(provider)
		matching := mm.GetModelsByCapability([]ModelCapability{CapabilityCodeGeneration, CapabilityTextGeneration})
		assert.Len(t, matching, 1)
		assert.Equal(t, "full-model", matching[0].Name)
	})

	t.Run("returns empty for no matches", func(t *testing.T) {
		mm := NewModelManager()
		provider := &MockTestProvider{
			name:         "test-provider",
			providerType: ProviderTypeOllama,
			models: []ModelInfo{
				{
					Name:         "model",
					Provider:     ProviderTypeOllama,
					Capabilities: []ModelCapability{CapabilityTextGeneration},
				},
			},
		}

		_ = mm.RegisterProvider(provider)
		matching := mm.GetModelsByCapability([]ModelCapability{CapabilityVision})
		assert.Empty(t, matching)
	})
}

// ========================================
// GetProviderForModel Tests
// ========================================

func TestModelManager_GetProviderForModel(t *testing.T) {
	t.Run("returns provider for existing model", func(t *testing.T) {
		mm := NewModelManager()
		provider := &MockTestProvider{
			name:         "test-provider",
			providerType: ProviderTypeOllama,
			models: []ModelInfo{
				{Name: "test-model", Provider: ProviderTypeOllama},
			},
		}

		_ = mm.RegisterProvider(provider)
		p, err := mm.GetProviderForModel("test-model", ProviderTypeOllama)
		assert.NoError(t, err)
		assert.NotNil(t, p)
		assert.Equal(t, "test-provider", p.GetName())
	})

	t.Run("fails for non-existent model", func(t *testing.T) {
		mm := NewModelManager()
		provider := &MockTestProvider{
			name:         "test-provider",
			providerType: ProviderTypeOllama,
			models:       []ModelInfo{},
		}

		_ = mm.RegisterProvider(provider)
		_, err := mm.GetProviderForModel("non-existent", ProviderTypeOllama)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("fails for non-existent provider", func(t *testing.T) {
		mm := NewModelManager()
		_, err := mm.GetProviderForModel("model", ProviderTypeOllama)
		assert.Error(t, err)
	})
}

// ========================================
// SelectOptimalModel Tests
// ========================================

func TestModelManager_SelectOptimalModel(t *testing.T) {
	t.Run("fails with no models", func(t *testing.T) {
		mm := NewModelManager()
		criteria := ModelSelectionCriteria{}
		_, err := mm.SelectOptimalModel(criteria)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no models available")
	})

	t.Run("selects model with matching capabilities", func(t *testing.T) {
		mm := NewModelManager()
		provider := &MockTestProvider{
			name:         "test-provider",
			providerType: ProviderTypeOllama,
			available:    true,
			models: []ModelInfo{
				{
					Name:         "code-model",
					Provider:     ProviderTypeOllama,
					Capabilities: []ModelCapability{CapabilityCodeGeneration},
					ContextSize:  4096,
				},
			},
		}

		_ = mm.RegisterProvider(provider)

		criteria := ModelSelectionCriteria{
			RequiredCapabilities: []ModelCapability{CapabilityCodeGeneration},
			MaxTokens:            1000,
		}

		model, err := mm.SelectOptimalModel(criteria)
		assert.NoError(t, err)
		assert.NotNil(t, model)
		assert.Equal(t, "code-model", model.Name)
	})

	t.Run("fails when no model meets criteria", func(t *testing.T) {
		mm := NewModelManager()
		provider := &MockTestProvider{
			name:         "test-provider",
			providerType: ProviderTypeOllama,
			available:    false, // Provider not available
			models: []ModelInfo{
				{
					Name:         "model",
					Provider:     ProviderTypeOllama,
					Capabilities: []ModelCapability{CapabilityTextGeneration},
					ContextSize:  4096,
				},
			},
		}

		_ = mm.RegisterProvider(provider)

		criteria := ModelSelectionCriteria{
			RequiredCapabilities: []ModelCapability{CapabilityVision},
			MaxTokens:            1000,
		}

		_, err := mm.SelectOptimalModel(criteria)
		assert.Error(t, err)
	})

	t.Run("fails when context size insufficient", func(t *testing.T) {
		mm := NewModelManager()
		provider := &MockTestProvider{
			name:         "test-provider",
			providerType: ProviderTypeOllama,
			available:    true,
			models: []ModelInfo{
				{
					Name:        "small-context-model",
					Provider:    ProviderTypeOllama,
					ContextSize: 1000,
				},
			},
		}

		_ = mm.RegisterProvider(provider)

		criteria := ModelSelectionCriteria{
			MaxTokens: 5000, // More than model's context size
		}

		_, err := mm.SelectOptimalModel(criteria)
		assert.Error(t, err)
	})
}

// ========================================
// HealthCheck Tests
// ========================================

func TestModelManager_HealthCheck(t *testing.T) {
	t.Run("returns empty for no providers", func(t *testing.T) {
		mm := NewModelManager()
		ctx := context.Background()
		health := mm.HealthCheck(ctx)
		assert.Empty(t, health)
	})

	t.Run("returns health status for all providers", func(t *testing.T) {
		mm := NewModelManager()
		provider := &MockTestProvider{
			name:         "test-provider",
			providerType: ProviderTypeOllama,
			health: &ProviderHealth{
				Status: "healthy",
			},
		}

		_ = mm.RegisterProvider(provider)

		ctx := context.Background()
		health := mm.HealthCheck(ctx)

		assert.Len(t, health, 1)
		assert.NotNil(t, health[ProviderTypeOllama])
		assert.Equal(t, "healthy", health[ProviderTypeOllama].Status)
	})

	t.Run("handles provider health error", func(t *testing.T) {
		mm := NewModelManager()
		provider := &MockTestProvider{
			name:         "failing-provider",
			providerType: ProviderTypeOllama,
			healthErr:    assert.AnError,
		}

		_ = mm.RegisterProvider(provider)

		ctx := context.Background()
		health := mm.HealthCheck(ctx)

		assert.Len(t, health, 1)
		assert.Equal(t, "unhealthy", health[ProviderTypeOllama].Status)
		// ErrorCount should be >= 1 when there's an error
		assert.GreaterOrEqual(t, health[ProviderTypeOllama].ErrorCount, 1)
	})
}

// ========================================
// calculateCapabilityScore Tests
// ========================================

func TestCalculateCapabilityScore(t *testing.T) {
	mm := NewModelManager()

	t.Run("returns 1.0 for no required capabilities", func(t *testing.T) {
		score := mm.calculateCapabilityScore([]ModelCapability{CapabilityTextGeneration}, []ModelCapability{})
		assert.Equal(t, 1.0, score)
	})

	t.Run("returns 1.0 for all capabilities matched", func(t *testing.T) {
		available := []ModelCapability{CapabilityTextGeneration, CapabilityCodeGeneration}
		required := []ModelCapability{CapabilityTextGeneration, CapabilityCodeGeneration}
		score := mm.calculateCapabilityScore(available, required)
		assert.Equal(t, 1.0, score)
	})

	t.Run("returns 0.5 for half capabilities matched", func(t *testing.T) {
		available := []ModelCapability{CapabilityTextGeneration}
		required := []ModelCapability{CapabilityTextGeneration, CapabilityCodeGeneration}
		score := mm.calculateCapabilityScore(available, required)
		assert.Equal(t, 0.5, score)
	})

	t.Run("returns 0.0 for no capabilities matched", func(t *testing.T) {
		available := []ModelCapability{CapabilityTextGeneration}
		required := []ModelCapability{CapabilityVision}
		score := mm.calculateCapabilityScore(available, required)
		assert.Equal(t, 0.0, score)
	})
}

// ========================================
// calculateTaskSuitability Tests
// ========================================

func TestCalculateTaskSuitability(t *testing.T) {
	mm := NewModelManager()

	testCases := []struct {
		name         string
		taskType     string
		capabilities []ModelCapability
		expectBonus  bool
	}{
		{"planning task with capability", "planning", []ModelCapability{CapabilityPlanning}, true},
		{"planning task without capability", "planning", []ModelCapability{CapabilityTextGeneration}, false},
		{"code_generation task with capability", "code_generation", []ModelCapability{CapabilityCodeGeneration}, true},
		{"debugging task with capability", "debugging", []ModelCapability{CapabilityDebugging}, true},
		{"testing task with capability", "testing", []ModelCapability{CapabilityTesting}, true},
		{"refactoring task with capability", "refactoring", []ModelCapability{CapabilityRefactoring}, true},
		{"unknown task type", "unknown", []ModelCapability{CapabilityTextGeneration}, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			model := &ModelInfo{Capabilities: tc.capabilities}
			score := mm.calculateTaskSuitability(model, tc.taskType)
			if tc.expectBonus {
				assert.Greater(t, score, 1.0)
			} else {
				assert.Equal(t, 1.0, score)
			}
		})
	}
}

// ========================================
// calculateQualityScore Tests
// ========================================

func TestCalculateQualityScore(t *testing.T) {
	mm := NewModelManager()

	t.Run("larger model gets higher quality estimate with quality preference", func(t *testing.T) {
		largeModel := &ModelInfo{Name: "llama-70b"}
		smallModel := &ModelInfo{Name: "llama-7b"}

		// Use "quality" preference since "balanced" returns 1.0 regardless of model size
		largeScore := mm.calculateQualityScore(largeModel, "quality")
		smallScore := mm.calculateQualityScore(smallModel, "quality")

		assert.Greater(t, largeScore, smallScore)
	})

	t.Run("balanced preference returns 1.0 regardless of model size", func(t *testing.T) {
		largeModel := &ModelInfo{Name: "llama-70b"}
		smallModel := &ModelInfo{Name: "llama-7b"}

		largeScore := mm.calculateQualityScore(largeModel, "balanced")
		smallScore := mm.calculateQualityScore(smallModel, "balanced")

		assert.Equal(t, 1.0, largeScore)
		assert.Equal(t, 1.0, smallScore)
	})

	t.Run("quality preference increases score", func(t *testing.T) {
		model := &ModelInfo{Name: "llama-13b"}

		qualityScore := mm.calculateQualityScore(model, "quality")
		balancedScore := mm.calculateQualityScore(model, "balanced")

		assert.Greater(t, qualityScore, balancedScore)
	})

	t.Run("fast preference adjusts score", func(t *testing.T) {
		largeModel := &ModelInfo{Name: "llama-70b"}
		smallModel := &ModelInfo{Name: "llama-3b"}

		largeScore := mm.calculateQualityScore(largeModel, "fast")
		smallScore := mm.calculateQualityScore(smallModel, "fast")

		// Fast preference inverts quality - smaller models score higher
		assert.Greater(t, smallScore, largeScore)
	})
}

// ========================================
// estimateModelSize Tests
// ========================================

func TestEstimateModelSize(t *testing.T) {
	mm := NewModelManager()

	testCases := []struct {
		modelName    string
		expectedSize string
	}{
		{"llama-70b", "70B"},
		{"Llama-70B", "70B"},
		{"codellama-34b", "34B"},
		{"mistral-13b", "13B"},
		{"phi-7b", "7B"},
		{"gemma-3b", "3B"},
		{"gpt-4", ""},      // Unknown size
		{"claude-3", ""},   // Unknown size
		{"some-model", ""}, // Unknown size
	}

	for _, tc := range testCases {
		t.Run(tc.modelName, func(t *testing.T) {
			size := mm.estimateModelSize(tc.modelName)
			assert.Equal(t, tc.expectedSize, size)
		})
	}
}

// ========================================
// hasAllCapabilities Tests
// ========================================

func TestHasAllCapabilities(t *testing.T) {
	mm := NewModelManager()

	t.Run("returns true for all capabilities present", func(t *testing.T) {
		available := []ModelCapability{CapabilityTextGeneration, CapabilityCodeGeneration, CapabilityVision}
		required := []ModelCapability{CapabilityTextGeneration, CapabilityCodeGeneration}
		assert.True(t, mm.hasAllCapabilities(available, required))
	})

	t.Run("returns false when capability missing", func(t *testing.T) {
		available := []ModelCapability{CapabilityTextGeneration}
		required := []ModelCapability{CapabilityTextGeneration, CapabilityCodeGeneration}
		assert.False(t, mm.hasAllCapabilities(available, required))
	})

	t.Run("returns true for empty required", func(t *testing.T) {
		available := []ModelCapability{CapabilityTextGeneration}
		required := []ModelCapability{}
		assert.True(t, mm.hasAllCapabilities(available, required))
	})
}

// ========================================
// hasCapability Tests
// ========================================

func TestHasCapability(t *testing.T) {
	mm := NewModelManager()

	t.Run("returns true when capability exists", func(t *testing.T) {
		caps := []ModelCapability{CapabilityTextGeneration, CapabilityCodeGeneration}
		assert.True(t, mm.hasCapability(caps, CapabilityTextGeneration))
	})

	t.Run("returns false when capability missing", func(t *testing.T) {
		caps := []ModelCapability{CapabilityTextGeneration}
		assert.False(t, mm.hasCapability(caps, CapabilityVision))
	})

	t.Run("returns false for empty capabilities", func(t *testing.T) {
		caps := []ModelCapability{}
		assert.False(t, mm.hasCapability(caps, CapabilityTextGeneration))
	})
}

// ========================================
// getModelKey Tests
// ========================================

func TestGetModelKey(t *testing.T) {
	mm := NewModelManager()

	key := mm.getModelKey(ProviderTypeOllama, "llama-7b")
	assert.Equal(t, "ollama::llama-7b", key)

	key = mm.getModelKey(ProviderTypeAnthropic, "claude-3")
	assert.Equal(t, "anthropic::claude-3", key)
}

// ========================================
// calculateConfidence Tests
// ========================================

func TestCalculateConfidence(t *testing.T) {
	mm := NewModelManager()

	t.Run("base confidence is 0.5", func(t *testing.T) {
		model := &ModelInfo{Name: "test"}
		criteria := ModelSelectionCriteria{}
		conf := mm.calculateConfidence(model, criteria)
		assert.Equal(t, 0.5, conf)
	})

	t.Run("capability match increases confidence", func(t *testing.T) {
		model := &ModelInfo{
			Name:         "test",
			Capabilities: []ModelCapability{CapabilityCodeGeneration},
		}
		criteria := ModelSelectionCriteria{
			RequiredCapabilities: []ModelCapability{CapabilityCodeGeneration},
		}
		conf := mm.calculateConfidence(model, criteria)
		assert.Greater(t, conf, 0.5)
	})

	t.Run("task type increases confidence", func(t *testing.T) {
		model := &ModelInfo{
			Name:         "test",
			Capabilities: []ModelCapability{CapabilityCodeGeneration},
		}
		criteria := ModelSelectionCriteria{
			TaskType: "code_generation",
		}
		conf := mm.calculateConfidence(model, criteria)
		assert.Greater(t, conf, 0.5)
	})

	t.Run("confidence caps at 1.0", func(t *testing.T) {
		model := &ModelInfo{
			Name:         "test",
			Capabilities: []ModelCapability{CapabilityCodeGeneration, CapabilityPlanning, CapabilityDebugging},
		}
		criteria := ModelSelectionCriteria{
			TaskType:             "code_generation",
			RequiredCapabilities: []ModelCapability{CapabilityCodeGeneration, CapabilityPlanning, CapabilityDebugging},
		}
		conf := mm.calculateConfidence(model, criteria)
		assert.LessOrEqual(t, conf, 1.0)
	})
}
