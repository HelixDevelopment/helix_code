package providers

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// AIIntegration Tests
// =============================================================================

func TestNewAIIntegration_WithNilConfig(t *testing.T) {
	ai := NewAIIntegration(nil)

	require.NotNil(t, ai)
	assert.NotNil(t, ai.config)
	assert.Equal(t, "openai", ai.config.DefaultLLM)
	assert.Equal(t, "memgpt", ai.config.DefaultMemory)
	assert.True(t, ai.config.CacheEnabled)
	assert.NotNil(t, ai.providers)
	assert.NotNil(t, ai.vector)
	assert.NotNil(t, ai.memory)
}

func TestNewAIIntegration_WithCustomConfig(t *testing.T) {
	config := &AIConfig{
		DefaultLLM:    "anthropic",
		DefaultMemory: "custom",
		Providers: map[string]*AIProviderConfig{
			"test": {
				Enabled:     true,
				Model:       "test-model",
				MaxTokens:   1000,
				Temperature: 0.5,
			},
		},
		CacheEnabled: false,
		CacheSize:    500,
	}

	ai := NewAIIntegration(config)

	require.NotNil(t, ai)
	assert.Equal(t, "anthropic", ai.config.DefaultLLM)
	assert.Equal(t, "custom", ai.config.DefaultMemory)
	assert.False(t, ai.config.CacheEnabled)
	assert.Equal(t, 500, ai.config.CacheSize)
}

func TestAIIntegration_ListProviders_Empty(t *testing.T) {
	ai := NewAIIntegration(&AIConfig{
		DefaultLLM: "test",
		Providers:  make(map[string]*AIProviderConfig),
	})

	providers := ai.ListProviders()
	assert.Empty(t, providers)
}

func TestAIIntegration_GetProvider_NotFound(t *testing.T) {
	ai := NewAIIntegration(&AIConfig{
		DefaultLLM: "test",
		Providers:  make(map[string]*AIProviderConfig),
	})

	provider, err := ai.GetProvider("nonexistent")
	assert.Error(t, err)
	assert.Nil(t, provider)
	assert.Contains(t, err.Error(), "AI provider not found")
}

func TestAIIntegration_GetVector(t *testing.T) {
	ai := NewAIIntegration(nil)

	vector := ai.GetVector()
	assert.NotNil(t, vector)
}

func TestAIIntegration_GetMemory(t *testing.T) {
	ai := NewAIIntegration(nil)

	memory := ai.GetMemory()
	assert.NotNil(t, memory)
}

func TestAIIntegration_GenerateTextWithProvider_NotFound(t *testing.T) {
	ai := NewAIIntegration(&AIConfig{
		DefaultLLM: "test",
		Providers:  make(map[string]*AIProviderConfig),
	})

	ctx := context.Background()
	result, err := ai.GenerateTextWithProvider(ctx, "nonexistent", "test prompt", nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "AI provider not found")
}

func TestAIIntegration_GenerateChatWithProvider_NotFound(t *testing.T) {
	ai := NewAIIntegration(&AIConfig{
		DefaultLLM: "test",
		Providers:  make(map[string]*AIProviderConfig),
	})

	ctx := context.Background()
	messages := []*ChatMessage{{Role: "user", Content: "Hello"}}
	result, err := ai.GenerateChatWithProvider(ctx, "nonexistent", messages, nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "AI provider not found")
}

func TestAIIntegration_GenerateEmbeddingWithProvider_NotFound(t *testing.T) {
	ai := NewAIIntegration(&AIConfig{
		DefaultLLM: "test",
		Providers:  make(map[string]*AIProviderConfig),
	})

	ctx := context.Background()
	embedding, err := ai.GenerateEmbeddingWithProvider(ctx, "nonexistent", "test text")

	assert.Error(t, err)
	assert.Nil(t, embedding)
	assert.Contains(t, err.Error(), "AI provider not found")
}

func TestAIIntegration_ClassifyTextWithProvider_NotFound(t *testing.T) {
	ai := NewAIIntegration(&AIConfig{
		DefaultLLM: "test",
		Providers:  make(map[string]*AIProviderConfig),
	})

	ctx := context.Background()
	result, err := ai.ClassifyTextWithProvider(ctx, "nonexistent", "test", []string{"cat1", "cat2"})

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "AI provider not found")
}

func TestAIIntegration_ExtractEntitiesWithProvider_NotFound(t *testing.T) {
	ai := NewAIIntegration(&AIConfig{
		DefaultLLM: "test",
		Providers:  make(map[string]*AIProviderConfig),
	})

	ctx := context.Background()
	entities, err := ai.ExtractEntitiesWithProvider(ctx, "nonexistent", "test text")

	assert.Error(t, err)
	assert.Nil(t, entities)
	assert.Contains(t, err.Error(), "AI provider not found")
}

func TestAIIntegration_GetStats(t *testing.T) {
	ai := NewAIIntegration(nil)

	ctx := context.Background()
	stats, err := ai.GetStats(ctx)

	require.NoError(t, err)
	assert.NotNil(t, stats)
	assert.NotNil(t, stats.Providers)
}

// =============================================================================
// AIConfig Tests
// =============================================================================

func TestAIConfig_Defaults(t *testing.T) {
	ai := NewAIIntegration(nil)
	config := ai.config

	assert.Equal(t, "openai", config.DefaultLLM)
	assert.Equal(t, "memgpt", config.DefaultMemory)
	assert.True(t, config.CacheEnabled)
	assert.Equal(t, 1000, config.CacheSize)
	assert.Equal(t, int64(300000), config.CacheTTL)
	assert.False(t, config.ProfilingEnabled)
}

func TestAIProviderConfig_Fields(t *testing.T) {
	config := &AIProviderConfig{
		Enabled:          true,
		Model:            "gpt-4",
		MaxTokens:        4096,
		Temperature:      0.7,
		TopP:             1.0,
		FrequencyPenalty: 0.0,
		PresencePenalty:  0.0,
		Config: map[string]interface{}{
			"api_key": "test-key",
		},
	}

	assert.True(t, config.Enabled)
	assert.Equal(t, "gpt-4", config.Model)
	assert.Equal(t, 4096, config.MaxTokens)
	assert.Equal(t, 0.7, config.Temperature)
	assert.Equal(t, 1.0, config.TopP)
	assert.Equal(t, "test-key", config.Config["api_key"])
}

// =============================================================================
// GenerationOptions Tests
// =============================================================================

func TestGenerationOptions_Fields(t *testing.T) {
	options := &GenerationOptions{
		MaxTokens:        1000,
		Temperature:      0.8,
		TopP:             0.95,
		FrequencyPenalty: 0.1,
		PresencePenalty:  0.2,
		Stop:             []string{"\n", "END"},
		Stream:           true,
	}

	assert.Equal(t, 1000, options.MaxTokens)
	assert.Equal(t, 0.8, options.Temperature)
	assert.Equal(t, 0.95, options.TopP)
	assert.Equal(t, 0.1, options.FrequencyPenalty)
	assert.Equal(t, 0.2, options.PresencePenalty)
	assert.Len(t, options.Stop, 2)
	assert.True(t, options.Stream)
}

// =============================================================================
// ChatMessage Tests
// =============================================================================

func TestChatMessage_Fields(t *testing.T) {
	msg := &ChatMessage{
		Role:    "user",
		Content: "Hello, world!",
		Name:    "testuser",
		Tokens:  5,
		Metadata: map[string]interface{}{
			"timestamp": time.Now(),
		},
	}

	assert.Equal(t, "user", msg.Role)
	assert.Equal(t, "Hello, world!", msg.Content)
	assert.Equal(t, "testuser", msg.Name)
	assert.Equal(t, 5, msg.Tokens)
	assert.NotNil(t, msg.Metadata)
}

// =============================================================================
// ChatResult Tests
// =============================================================================

func TestChatResult_Fields(t *testing.T) {
	result := &ChatResult{
		Message: &ChatMessage{
			Role:    "assistant",
			Content: "Hello!",
		},
		Messages:     []*ChatMessage{},
		Tokens:       10,
		FinishReason: "stop",
		Metadata:     map[string]interface{}{"model": "gpt-4"},
		Cost:         0.002,
		Duration:     100 * time.Millisecond,
	}

	assert.NotNil(t, result.Message)
	assert.Equal(t, "assistant", result.Message.Role)
	assert.Equal(t, 10, result.Tokens)
	assert.Equal(t, "stop", result.FinishReason)
	assert.Equal(t, 0.002, result.Cost)
}

// =============================================================================
// Entity Tests
// =============================================================================

func TestEntity_Fields(t *testing.T) {
	entity := &Entity{
		Type:       "PERSON",
		Text:       "John Doe",
		Confidence: 0.95,
		Start:      0,
		End:        8,
		Metadata:   map[string]interface{}{"source": "NER"},
	}

	assert.Equal(t, "PERSON", entity.Type)
	assert.Equal(t, "John Doe", entity.Text)
	assert.Equal(t, 0.95, entity.Confidence)
	assert.Equal(t, 0, entity.Start)
	assert.Equal(t, 8, entity.End)
}

// =============================================================================
// CostInfo Tests
// =============================================================================

func TestCostInfo_Fields(t *testing.T) {
	cost := &CostInfo{
		InputTokens:  100,
		OutputTokens: 50,
		TotalTokens:  150,
		Cost:         0.003,
		Currency:     "USD",
	}

	assert.Equal(t, 100, cost.InputTokens)
	assert.Equal(t, 50, cost.OutputTokens)
	assert.Equal(t, 150, cost.TotalTokens)
	assert.Equal(t, 0.003, cost.Cost)
	assert.Equal(t, "USD", cost.Currency)
}

// =============================================================================
// MemoryIntegration Tests
// =============================================================================

func TestNewMemoryIntegration_WithNilConfig(t *testing.T) {
	mi := NewMemoryIntegration(nil)

	require.NotNil(t, mi)
	assert.NotNil(t, mi.config)
	assert.True(t, mi.config.Enabled)
	assert.Equal(t, 10000, mi.config.MaxGenerations)
	assert.Equal(t, 1000, mi.config.MaxConversations)
	assert.NotNil(t, mi.generations)
	assert.NotNil(t, mi.conversations)
}

func TestNewMemoryIntegration_WithCustomConfig(t *testing.T) {
	config := &MemoryConfig{
		Enabled:          true,
		Provider:         "custom",
		MaxGenerations:   5000,
		MaxConversations: 500,
		TTL:              12 * time.Hour,
	}

	mi := NewMemoryIntegration(config)

	require.NotNil(t, mi)
	assert.Equal(t, 5000, mi.config.MaxGenerations)
	assert.Equal(t, 500, mi.config.MaxConversations)
	assert.Equal(t, 12*time.Hour, mi.config.TTL)
}

func TestMemoryIntegration_Initialize(t *testing.T) {
	mi := NewMemoryIntegration(nil)
	ctx := context.Background()

	err := mi.Initialize(ctx)
	assert.NoError(t, err)
}

func TestMemoryIntegration_StoreGeneration(t *testing.T) {
	mi := NewMemoryIntegration(nil)
	ctx := context.Background()
	_ = mi.Initialize(ctx)

	generation := &GenerationResult{
		Text:   "Generated text",
		Tokens: 10,
		Cost:   0.001,
	}

	err := mi.StoreGeneration(ctx, "test-provider", "test prompt", generation)
	assert.NoError(t, err)

	// Check statistics were updated
	stats, err := mi.GetMemoryStats(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), stats.TotalGenerations)
	assert.Equal(t, int64(10), stats.TotalTokens)
}

func TestMemoryIntegration_StoreConversation(t *testing.T) {
	mi := NewMemoryIntegration(nil)
	ctx := context.Background()
	_ = mi.Initialize(ctx)

	messages := []*ChatMessage{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there!"},
	}
	result := &ChatResult{
		Tokens: 20,
		Cost:   0.002,
	}

	err := mi.StoreConversation(ctx, "test-provider", messages, result)
	assert.NoError(t, err)

	stats, err := mi.GetMemoryStats(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), stats.TotalConversations)
}

func TestMemoryIntegration_GetMemoryStats(t *testing.T) {
	mi := NewMemoryIntegration(nil)
	ctx := context.Background()

	stats, err := mi.GetMemoryStats(ctx)

	require.NoError(t, err)
	assert.NotNil(t, stats)
	assert.Equal(t, int64(0), stats.TotalGenerations)
	assert.Equal(t, int64(0), stats.TotalConversations)
}

func TestMemoryIntegration_Stop(t *testing.T) {
	mi := NewMemoryIntegration(nil)
	ctx := context.Background()
	_ = mi.Initialize(ctx)

	// Store some data
	_ = mi.StoreGeneration(ctx, "test", "prompt", &GenerationResult{Tokens: 1})

	err := mi.Stop(ctx)
	assert.NoError(t, err)

	// Verify storage is cleared
	assert.Empty(t, mi.generations)
	assert.Empty(t, mi.conversations)
}

// =============================================================================
// ConversationManager Tests
// =============================================================================

func TestConversationManager_CreateConversation(t *testing.T) {
	ai := NewAIIntegration(nil)
	cm := NewConversationManager(ai, ai.config)
	ctx := context.Background()

	conv, err := cm.CreateConversation(ctx, "conv-1")

	require.NoError(t, err)
	assert.NotNil(t, conv)
	assert.Equal(t, "conv-1", conv.ID)
	assert.Empty(t, conv.Messages)
}

func TestConversationManager_AddMessage(t *testing.T) {
	ai := NewAIIntegration(nil)
	cm := NewConversationManager(ai, ai.config)
	ctx := context.Background()

	_, _ = cm.CreateConversation(ctx, "conv-1")

	msg := &ChatMessage{
		Role:    "user",
		Content: "Hello",
		Tokens:  5,
	}

	err := cm.AddMessage(ctx, "conv-1", msg)
	assert.NoError(t, err)

	conv, _ := cm.GetConversation(ctx, "conv-1")
	assert.Len(t, conv.Messages, 1)
	assert.Equal(t, 5, conv.TotalTokens)
}

func TestConversationManager_AddMessage_NotFound(t *testing.T) {
	ai := NewAIIntegration(nil)
	cm := NewConversationManager(ai, ai.config)
	ctx := context.Background()

	msg := &ChatMessage{Role: "user", Content: "Hello"}
	err := cm.AddMessage(ctx, "nonexistent", msg)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "conversation not found")
}

func TestConversationManager_GetConversation(t *testing.T) {
	ai := NewAIIntegration(nil)
	cm := NewConversationManager(ai, ai.config)
	ctx := context.Background()

	_, _ = cm.CreateConversation(ctx, "conv-1")

	conv, err := cm.GetConversation(ctx, "conv-1")
	require.NoError(t, err)
	assert.Equal(t, "conv-1", conv.ID)
}

func TestConversationManager_GetConversation_NotFound(t *testing.T) {
	ai := NewAIIntegration(nil)
	cm := NewConversationManager(ai, ai.config)
	ctx := context.Background()

	conv, err := cm.GetConversation(ctx, "nonexistent")

	assert.Error(t, err)
	assert.Nil(t, conv)
	assert.Contains(t, err.Error(), "conversation not found")
}

func TestConversationManager_Stop(t *testing.T) {
	ai := NewAIIntegration(nil)
	cm := NewConversationManager(ai, ai.config)
	ctx := context.Background()

	_, _ = cm.CreateConversation(ctx, "conv-1")

	err := cm.Stop(ctx)
	assert.NoError(t, err)

	// Verify conversations cleared
	assert.Empty(t, cm.conversations)
}

// =============================================================================
// PersonalityManager Tests
// =============================================================================

func TestNewPersonalityManager(t *testing.T) {
	ai := NewAIIntegration(nil)
	pm := NewPersonalityManager(ai, ai.config)

	require.NotNil(t, pm)
	assert.NotNil(t, pm.personalities)
	assert.Len(t, pm.personalities, 3) // default, technical, creative
	assert.NotNil(t, pm.activePersonality)
	assert.Equal(t, "default", pm.activePersonality.ID)
}

func TestPersonalityManager_GetPersonality(t *testing.T) {
	ai := NewAIIntegration(nil)
	pm := NewPersonalityManager(ai, ai.config)

	personality, err := pm.GetPersonality("default")

	require.NoError(t, err)
	assert.Equal(t, "default", personality.ID)
	assert.Equal(t, "Default Assistant", personality.Name)
}

func TestPersonalityManager_GetPersonality_NotFound(t *testing.T) {
	ai := NewAIIntegration(nil)
	pm := NewPersonalityManager(ai, ai.config)

	personality, err := pm.GetPersonality("nonexistent")

	assert.Error(t, err)
	assert.Nil(t, personality)
	assert.Contains(t, err.Error(), "personality not found")
}

func TestPersonalityManager_SetActivePersonality(t *testing.T) {
	ai := NewAIIntegration(nil)
	pm := NewPersonalityManager(ai, ai.config)

	err := pm.SetActivePersonality("technical")
	assert.NoError(t, err)

	active := pm.GetActivePersonality()
	assert.Equal(t, "technical", active.ID)
}

func TestPersonalityManager_SetActivePersonality_NotFound(t *testing.T) {
	ai := NewAIIntegration(nil)
	pm := NewPersonalityManager(ai, ai.config)

	err := pm.SetActivePersonality("nonexistent")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "personality not found")
}

func TestPersonalityManager_SetActivePersonality_Disabled(t *testing.T) {
	ai := NewAIIntegration(nil)
	pm := NewPersonalityManager(ai, ai.config)

	// Disable a personality
	pm.personalities["technical"].Enabled = false

	err := pm.SetActivePersonality("technical")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "personality is disabled")
}

func TestPersonalityManager_AddPersonality(t *testing.T) {
	ai := NewAIIntegration(nil)
	pm := NewPersonalityManager(ai, ai.config)

	newPersonality := &Personality{
		ID:          "custom",
		Name:        "Custom Personality",
		Description: "A custom personality",
		Enabled:     true,
	}

	err := pm.AddPersonality(newPersonality)
	assert.NoError(t, err)

	personality, err := pm.GetPersonality("custom")
	require.NoError(t, err)
	assert.Equal(t, "custom", personality.ID)
}

func TestPersonalityManager_AddPersonality_EmptyID(t *testing.T) {
	ai := NewAIIntegration(nil)
	pm := NewPersonalityManager(ai, ai.config)

	newPersonality := &Personality{
		ID:   "",
		Name: "No ID",
	}

	err := pm.AddPersonality(newPersonality)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ID cannot be empty")
}

func TestPersonalityManager_AddPersonality_Duplicate(t *testing.T) {
	ai := NewAIIntegration(nil)
	pm := NewPersonalityManager(ai, ai.config)

	newPersonality := &Personality{
		ID:   "default",
		Name: "Duplicate",
	}

	err := pm.AddPersonality(newPersonality)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "personality already exists")
}

func TestPersonalityManager_UpdatePersonality(t *testing.T) {
	ai := NewAIIntegration(nil)
	pm := NewPersonalityManager(ai, ai.config)

	updates := map[string]interface{}{
		"name":        "Updated Default",
		"description": "An updated description",
		"temperature": 0.5,
	}

	err := pm.UpdatePersonality("default", updates)
	assert.NoError(t, err)

	personality, _ := pm.GetPersonality("default")
	assert.Equal(t, "Updated Default", personality.Name)
	assert.Equal(t, "An updated description", personality.Description)
	assert.Equal(t, 0.5, personality.Temperature)
}

func TestPersonalityManager_UpdatePersonality_NotFound(t *testing.T) {
	ai := NewAIIntegration(nil)
	pm := NewPersonalityManager(ai, ai.config)

	err := pm.UpdatePersonality("nonexistent", map[string]interface{}{})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "personality not found")
}

func TestPersonalityManager_RemovePersonality(t *testing.T) {
	ai := NewAIIntegration(nil)
	pm := NewPersonalityManager(ai, ai.config)

	err := pm.RemovePersonality("technical")
	assert.NoError(t, err)

	_, err = pm.GetPersonality("technical")
	assert.Error(t, err)
}

func TestPersonalityManager_RemovePersonality_Default(t *testing.T) {
	ai := NewAIIntegration(nil)
	pm := NewPersonalityManager(ai, ai.config)

	err := pm.RemovePersonality("default")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot remove default personality")
}

func TestPersonalityManager_RemovePersonality_ActiveSwitchesToDefault(t *testing.T) {
	ai := NewAIIntegration(nil)
	pm := NewPersonalityManager(ai, ai.config)

	_ = pm.SetActivePersonality("technical")

	err := pm.RemovePersonality("technical")
	assert.NoError(t, err)

	active := pm.GetActivePersonality()
	assert.Equal(t, "default", active.ID)
}

func TestPersonalityManager_ListPersonalities(t *testing.T) {
	ai := NewAIIntegration(nil)
	pm := NewPersonalityManager(ai, ai.config)

	personalities := pm.ListPersonalities()

	assert.Len(t, personalities, 3)
}

func TestPersonalityManager_IncrementUsage(t *testing.T) {
	ai := NewAIIntegration(nil)
	pm := NewPersonalityManager(ai, ai.config)

	personality, _ := pm.GetPersonality("default")
	initialCount := personality.UsageCount

	pm.IncrementUsage("default")

	personality, _ = pm.GetPersonality("default")
	assert.Equal(t, initialCount+1, personality.UsageCount)
}

func TestPersonalityManager_Stop(t *testing.T) {
	ai := NewAIIntegration(nil)
	pm := NewPersonalityManager(ai, ai.config)
	ctx := context.Background()

	err := pm.Stop(ctx)
	assert.NoError(t, err)

	// Should only have default personality left
	assert.Len(t, pm.personalities, 1)
	assert.Equal(t, "default", pm.activePersonality.ID)
}

// =============================================================================
// MockAIProvider Tests
// =============================================================================

func TestMockAIProvider_GenerateText(t *testing.T) {
	provider := &MockAIProvider{}
	ctx := context.Background()

	result, err := provider.GenerateText(ctx, "test prompt", nil)

	require.NoError(t, err)
	assert.NotEmpty(t, result.Text)
	assert.Greater(t, result.Tokens, 0)
	assert.Equal(t, "stop", result.FinishReason)
}

func TestMockAIProvider_GenerateEmbedding(t *testing.T) {
	provider := &MockAIProvider{}
	ctx := context.Background()

	embedding, err := provider.GenerateEmbedding(ctx, "test text")

	require.NoError(t, err)
	assert.Len(t, embedding, 1536)
}

func TestMockAIProvider_GenerateChat(t *testing.T) {
	provider := &MockAIProvider{}
	ctx := context.Background()

	messages := []*ChatMessage{{Role: "user", Content: "Hello"}}
	result, err := provider.GenerateChat(ctx, messages, nil)

	require.NoError(t, err)
	assert.NotNil(t, result.Message)
	assert.Equal(t, "assistant", result.Message.Role)
}

func TestMockAIProvider_ClassifyText(t *testing.T) {
	provider := &MockAIProvider{}
	ctx := context.Background()

	result, err := provider.ClassifyText(ctx, "test text", []string{"cat1", "cat2"})

	require.NoError(t, err)
	assert.Equal(t, "cat1", result.Category)
	assert.Greater(t, result.Confidence, 0.0)
}

func TestMockAIProvider_ExtractEntities(t *testing.T) {
	provider := &MockAIProvider{}
	ctx := context.Background()

	entities, err := provider.ExtractEntities(ctx, "John Doe works at Company")

	require.NoError(t, err)
	assert.NotEmpty(t, entities)
	assert.Equal(t, "PERSON", entities[0].Type)
}

func TestMockAIProvider_GetCapabilities(t *testing.T) {
	provider := &MockAIProvider{}

	capabilities := provider.GetCapabilities()

	assert.NotEmpty(t, capabilities)
	assert.Contains(t, capabilities, "text_generation")
	assert.Contains(t, capabilities, "chat")
}

func TestMockAIProvider_GetCostInfo(t *testing.T) {
	provider := &MockAIProvider{}

	cost := provider.GetCostInfo()

	assert.NotNil(t, cost)
	assert.Greater(t, cost.TotalTokens, 0)
	assert.Equal(t, "USD", cost.Currency)
}

// =============================================================================
// AIHealthStatus Tests
// =============================================================================

func TestAIHealthStatus_Fields(t *testing.T) {
	status := &AIHealthStatus{
		Status:           "healthy",
		TotalProviders:   3,
		HealthyProviders: 3,
		ProviderStatuses: map[string]string{
			"openai":    "healthy",
			"anthropic": "healthy",
			"gemini":    "healthy",
		},
		LastCheck: time.Now(),
	}

	assert.Equal(t, "healthy", status.Status)
	assert.Equal(t, 3, status.TotalProviders)
	assert.Equal(t, 3, status.HealthyProviders)
	assert.Len(t, status.ProviderStatuses, 3)
}

// =============================================================================
// AIStats Tests
// =============================================================================

func TestAIStats_Fields(t *testing.T) {
	stats := &AIStats{
		Providers: map[string]*AIProviderStats{
			"openai": {
				Name:      "openai",
				Requests:  100,
				Successes: 98,
				Failures:  2,
			},
		},
		TotalCost:   1.50,
		TotalTokens: 50000,
		LastUpdate:  time.Now(),
	}

	assert.NotNil(t, stats.Providers["openai"])
	assert.Equal(t, int64(100), stats.Providers["openai"].Requests)
	assert.Equal(t, 1.50, stats.TotalCost)
}

// =============================================================================
// LLMProviderWrapper Tests
// =============================================================================

func TestLLMProviderWrapper_Generate(t *testing.T) {
	mockProvider := &MockAIProvider{}
	wrapper := &LLMProviderWrapper{provider: mockProvider}
	ctx := context.Background()

	result, err := wrapper.Generate(ctx, "test prompt")

	require.NoError(t, err)
	assert.NotEmpty(t, result)
}

// =============================================================================
// AIIntegration Additional Tests
// =============================================================================

func TestAIIntegration_Initialize_Skipped(t *testing.T) {
	// Note: Initialize requires valid vector/memory configurations
	// which are not available in unit tests without external services.
	// This test is skipped to prevent nil pointer panics.
	t.Skip("Initialize requires external services (vector DB)")
}

func TestAIIntegration_GenerateText(t *testing.T) {
	ai := NewAIIntegration(&AIConfig{
		DefaultLLM: "nonexistent",
		Providers:  make(map[string]*AIProviderConfig),
	})

	ctx := context.Background()
	result, err := ai.GenerateText(ctx, "test prompt", nil)

	// Should fail because no provider exists
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestAIIntegration_GenerateChat(t *testing.T) {
	ai := NewAIIntegration(&AIConfig{
		DefaultLLM: "nonexistent",
		Providers:  make(map[string]*AIProviderConfig),
	})

	ctx := context.Background()
	messages := []*ChatMessage{{Role: "user", Content: "Hello"}}
	result, err := ai.GenerateChat(ctx, messages, nil)

	// Should fail because no provider exists
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestAIIntegration_GenerateEmbedding(t *testing.T) {
	ai := NewAIIntegration(&AIConfig{
		DefaultLLM: "nonexistent",
		Providers:  make(map[string]*AIProviderConfig),
	})

	ctx := context.Background()
	embedding, err := ai.GenerateEmbedding(ctx, "test text")

	// Should fail because no provider exists
	assert.Error(t, err)
	assert.Nil(t, embedding)
}

func TestAIIntegration_ClassifyText(t *testing.T) {
	ai := NewAIIntegration(&AIConfig{
		DefaultLLM: "nonexistent",
		Providers:  make(map[string]*AIProviderConfig),
	})

	ctx := context.Background()
	result, err := ai.ClassifyText(ctx, "test text", []string{"cat1", "cat2"})

	// Should fail because no provider exists
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestAIIntegration_ExtractEntities(t *testing.T) {
	ai := NewAIIntegration(&AIConfig{
		DefaultLLM: "nonexistent",
		Providers:  make(map[string]*AIProviderConfig),
	})

	ctx := context.Background()
	entities, err := ai.ExtractEntities(ctx, "John works at Google")

	// Should fail because no provider exists
	assert.Error(t, err)
	assert.Nil(t, entities)
}

func TestAIIntegration_GetConversationManager(t *testing.T) {
	ai := NewAIIntegration(nil)

	// ConversationManager is created on-demand or via Initialize
	// Without Initialize, it may be nil
	convMgr := ai.GetConversation()
	// May be nil without full initialization
	_ = convMgr
}

func TestAIIntegration_GetPersonalityManager(t *testing.T) {
	ai := NewAIIntegration(nil)

	// PersonalityManager is created on-demand or via Initialize
	// Without Initialize, it may be nil
	personalityMgr := ai.GetPersonality()
	// May be nil without full initialization
	_ = personalityMgr
}

func TestAIIntegration_HealthCheck(t *testing.T) {
	ai := NewAIIntegration(&AIConfig{
		DefaultLLM: "test",
		Providers:  make(map[string]*AIProviderConfig),
	})

	ctx := context.Background()
	health, err := ai.HealthCheck(ctx)

	require.NoError(t, err)
	assert.NotNil(t, health)
	assert.Equal(t, 0, health.TotalProviders)
}

func TestAIIntegration_Stop(t *testing.T) {
	ai := NewAIIntegration(&AIConfig{
		DefaultLLM: "test",
		Providers:  make(map[string]*AIProviderConfig),
	})

	ctx := context.Background()
	err := ai.Stop(ctx)

	require.NoError(t, err)
}

func TestAIIntegration_ListProviders_WithConfigs(t *testing.T) {
	// Note: ListProviders returns created provider instances, not configs.
	// Without Initialize, providers are not created.
	ai := NewAIIntegration(&AIConfig{
		DefaultLLM: "test",
		Providers: map[string]*AIProviderConfig{
			"provider1": {Enabled: true},
			"provider2": {Enabled: true},
		},
	})

	// ListProviders returns empty without Initialize
	providers := ai.ListProviders()
	// Config is stored but providers aren't created until Initialize
	_ = providers
}
