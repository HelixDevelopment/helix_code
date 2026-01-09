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
// NotImplementedProvider Tests
// =============================================================================
// These tests verify that NotImplementedProvider returns proper errors
// instead of fake mock data, preventing mock responses from reaching production.

func TestNotImplementedProvider_GenerateText(t *testing.T) {
	provider := newNotImplementedProvider("TestProvider")
	ctx := context.Background()

	result, err := provider.GenerateText(ctx, "test prompt", nil)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "TestProvider")
	assert.Contains(t, err.Error(), "not yet integrated")
}

func TestNotImplementedProvider_GenerateEmbedding(t *testing.T) {
	provider := newNotImplementedProvider("TestProvider")
	ctx := context.Background()

	embedding, err := provider.GenerateEmbedding(ctx, "test text")

	require.Error(t, err)
	assert.Nil(t, embedding)
	assert.Contains(t, err.Error(), "not yet integrated")
}

func TestNotImplementedProvider_GenerateChat(t *testing.T) {
	provider := newNotImplementedProvider("TestProvider")
	ctx := context.Background()

	messages := []*ChatMessage{{Role: "user", Content: "Hello"}}
	result, err := provider.GenerateChat(ctx, messages, nil)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not yet integrated")
}

func TestNotImplementedProvider_ClassifyText(t *testing.T) {
	provider := newNotImplementedProvider("TestProvider")
	ctx := context.Background()

	result, err := provider.ClassifyText(ctx, "test text", []string{"cat1", "cat2"})

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not yet integrated")
}

func TestNotImplementedProvider_ExtractEntities(t *testing.T) {
	provider := newNotImplementedProvider("TestProvider")
	ctx := context.Background()

	entities, err := provider.ExtractEntities(ctx, "John Doe works at Company")

	require.Error(t, err)
	assert.Nil(t, entities)
	assert.Contains(t, err.Error(), "not yet integrated")
}

func TestNotImplementedProvider_GetCapabilities(t *testing.T) {
	provider := newNotImplementedProvider("TestProvider")

	capabilities := provider.GetCapabilities()

	assert.Empty(t, capabilities)
}

func TestNotImplementedProvider_GetCostInfo(t *testing.T) {
	provider := newNotImplementedProvider("TestProvider")

	cost := provider.GetCostInfo()

	assert.NotNil(t, cost)
	assert.Equal(t, 0, cost.TotalTokens)
	assert.Equal(t, 0.0, cost.Cost)
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
	// Using NotImplementedProvider to test wrapper behavior
	// The wrapper should propagate the error from the underlying provider
	notImplProvider := newNotImplementedProvider("TestProvider")
	wrapper := &LLMProviderWrapper{provider: notImplProvider}
	ctx := context.Background()

	result, err := wrapper.Generate(ctx, "test prompt")

	// NotImplementedProvider returns an error, wrapper should propagate it
	require.Error(t, err)
	assert.Empty(t, result)
	assert.Contains(t, err.Error(), "not yet integrated")
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

// =============================================================================
// LLMProviderAdapter Tests
// =============================================================================

func TestNewLLMProviderAdapter(t *testing.T) {
	adapter := NewLLMProviderAdapter(nil, "TestProvider")

	require.NotNil(t, adapter)
	assert.Equal(t, "TestProvider", adapter.providerName)
	assert.NotNil(t, adapter.lastCostInfo)
	assert.Equal(t, "USD", adapter.lastCostInfo.Currency)
}

func TestLLMProviderAdapter_GetCapabilities_NilProvider(t *testing.T) {
	adapter := NewLLMProviderAdapter(nil, "Test")

	caps := adapter.GetCapabilities()
	assert.Empty(t, caps)
}

func TestLLMProviderAdapter_GetCostInfo(t *testing.T) {
	adapter := NewLLMProviderAdapter(nil, "Test")

	costInfo := adapter.GetCostInfo()
	require.NotNil(t, costInfo)
	assert.Equal(t, "USD", costInfo.Currency)
	assert.Equal(t, 0, costInfo.InputTokens)
	assert.Equal(t, 0, costInfo.OutputTokens)
}

func TestLLMProviderAdapter_GenerateText_NilProvider(t *testing.T) {
	adapter := NewLLMProviderAdapter(nil, "Test")

	ctx := context.Background()
	result, err := adapter.GenerateText(ctx, "test prompt", nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "provider not initialized")
}

func TestLLMProviderAdapter_GenerateChat_NilProvider(t *testing.T) {
	adapter := NewLLMProviderAdapter(nil, "Test")

	ctx := context.Background()
	messages := []*ChatMessage{{Role: "user", Content: "Hello"}}
	result, err := adapter.GenerateChat(ctx, messages, nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "provider not initialized")
}

func TestLLMProviderAdapter_GenerateEmbedding(t *testing.T) {
	adapter := NewLLMProviderAdapter(nil, "Test")

	ctx := context.Background()
	result, err := adapter.GenerateEmbedding(ctx, "test text")

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "embedding generation requires dedicated embedding model")
}

func TestLLMProviderAdapter_ClassifyText_NilProvider(t *testing.T) {
	adapter := NewLLMProviderAdapter(nil, "Test")

	ctx := context.Background()
	result, err := adapter.ClassifyText(ctx, "test text", []string{"cat1", "cat2"})

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "provider not initialized")
}

func TestLLMProviderAdapter_ExtractEntities_NilProvider(t *testing.T) {
	adapter := NewLLMProviderAdapter(nil, "Test")

	ctx := context.Background()
	result, err := adapter.ExtractEntities(ctx, "John works at Acme Corp")

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "provider not initialized")
}

// =============================================================================
// Helper Function Tests
// =============================================================================

func TestContainsIgnoreCase(t *testing.T) {
	tests := []struct {
		s      string
		substr string
		want   bool
	}{
		{"Hello World", "hello", true},
		{"Hello World", "WORLD", true},
		{"Hello World", "xyz", false},
		{"Test", "test", true},
		{"", "", true},
		{"abc", "abcd", false},
	}

	for _, tt := range tests {
		t.Run(tt.s+"_"+tt.substr, func(t *testing.T) {
			got := containsIgnoreCase(tt.s, tt.substr)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestToLower(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Hello", "hello"},
		{"WORLD", "world"},
		{"MixedCase", "mixedcase"},
		{"", ""},
		{"123", "123"},
		{"abc", "abc"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := toLower(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFindSubstring(t *testing.T) {
	tests := []struct {
		s      string
		substr string
		want   bool
	}{
		{"hello world", "world", true},
		{"hello world", "hello", true},
		{"hello world", "xyz", false},
		{"test", "test", true},
		{"test", "testing", false},
		{"", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.s+"_"+tt.substr, func(t *testing.T) {
			got := findSubstring(tt.s, tt.substr)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		s      string
		substr string
		want   bool
	}{
		{"hello world", "world", true},
		{"hello world", "hello", true},
		{"hello world", "xyz", false},
		{"test", "test", true},
		{"", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.s+"_"+tt.substr, func(t *testing.T) {
			got := contains(tt.s, tt.substr)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSplitLines(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"a\nb\nc", []string{"a", "b", "c"}},
		{"single", []string{"single"}},
		{"a\n", []string{"a"}},    // trailing newline does not add empty element
		{"", nil},                  // empty string returns nil
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := splitLines(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTrimSpace(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"  hello  ", "hello"},
		{"\t\ttab\t\t", "tab"},
		{"\n\nnewline\n\n", "newline"},
		{"notrim", "notrim"},
		{"", ""},
		{"   ", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := trimSpace(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFindChar(t *testing.T) {
	tests := []struct {
		s    string
		c    byte
		want int
	}{
		{"hello", 'e', 1},
		{"hello", 'o', 4},
		{"hello", 'x', -1},
		{"", 'a', -1},
	}

	for _, tt := range tests {
		t.Run(string(tt.c), func(t *testing.T) {
			got := findChar(tt.s, tt.c)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFindSubstringIndex(t *testing.T) {
	tests := []struct {
		s      string
		substr string
		want   int
	}{
		{"hello world", "world", 6},
		{"hello world", "hello", 0},
		{"hello world", "xyz", -1},
		{"test", "test", 0},
		{"", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.s+"_"+tt.substr, func(t *testing.T) {
			got := findSubstringIndex(tt.s, tt.substr)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseEntities(t *testing.T) {
	input := `PERSON: "John Doe"
ORGANIZATION: "Acme Corp"
LOCATION: "New York"`
	originalText := "John Doe works at Acme Corp in New York"

	entities := parseEntities(input, originalText)

	require.Len(t, entities, 3)

	assert.Equal(t, "PERSON", entities[0].Type)
	assert.Equal(t, "John Doe", entities[0].Text)

	assert.Equal(t, "ORGANIZATION", entities[1].Type)
	assert.Equal(t, "Acme Corp", entities[1].Text)

	assert.Equal(t, "LOCATION", entities[2].Type)
	assert.Equal(t, "New York", entities[2].Text)
}

// =============================================================================
// Provider Factory Tests
// =============================================================================

func TestNewOpenAIProvider_NilConfig(t *testing.T) {
	provider := NewOpenAIProvider(nil)

	// Should return NotImplementedProvider with error message
	require.NotNil(t, provider)

	ctx := context.Background()
	_, err := provider.GenerateText(ctx, "test", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing config")
}

func TestNewAnthropicProvider_NilConfig(t *testing.T) {
	provider := NewAnthropicProvider(nil)

	require.NotNil(t, provider)

	ctx := context.Background()
	_, err := provider.GenerateText(ctx, "test", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing config")
}

func TestNewGeminiProvider_NilConfig(t *testing.T) {
	provider := NewGeminiProvider(nil)

	require.NotNil(t, provider)

	ctx := context.Background()
	_, err := provider.GenerateText(ctx, "test", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing config")
}

func TestNewCohereProvider(t *testing.T) {
	provider := NewCohereProvider(nil)

	require.NotNil(t, provider)

	ctx := context.Background()
	_, err := provider.GenerateText(ctx, "test", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not implemented")
}

func TestNewHuggingFaceProvider(t *testing.T) {
	provider := NewHuggingFaceProvider(nil)

	require.NotNil(t, provider)

	ctx := context.Background()
	_, err := provider.GenerateText(ctx, "test", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not implemented")
}

func TestNewMistralProvider(t *testing.T) {
	provider := NewMistralProvider(nil)

	require.NotNil(t, provider)

	ctx := context.Background()
	_, err := provider.GenerateText(ctx, "test", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not implemented")
}

func TestNewGemmaProvider(t *testing.T) {
	provider := NewGemmaProvider(nil)

	require.NotNil(t, provider)

	ctx := context.Background()
	_, err := provider.GenerateText(ctx, "test", nil)
	assert.Error(t, err)
}

func TestNewLlamaIndexProvider(t *testing.T) {
	provider := NewLlamaIndexProvider(nil)

	require.NotNil(t, provider)

	ctx := context.Background()
	_, err := provider.GenerateText(ctx, "test", nil)
	assert.Error(t, err)
}

func TestNewMemGPTAIProvider(t *testing.T) {
	provider := NewMemGPTAIProvider(nil)

	require.NotNil(t, provider)

	ctx := context.Background()
	_, err := provider.GenerateText(ctx, "test", nil)
	assert.Error(t, err)
}

func TestNewCrewAIProvider(t *testing.T) {
	provider := NewCrewAIProvider(nil)

	require.NotNil(t, provider)

	ctx := context.Background()
	_, err := provider.GenerateText(ctx, "test", nil)
	assert.Error(t, err)
}

func TestNewCharacterAIProvider_FromProviders(t *testing.T) {
	provider := NewCharacterAIProvider(nil)

	require.NotNil(t, provider)

	ctx := context.Background()
	_, err := provider.GenerateText(ctx, "test", nil)
	assert.Error(t, err)
}

func TestNewReplikaAIProvider(t *testing.T) {
	provider := NewReplikaAIProvider(nil)

	require.NotNil(t, provider)

	ctx := context.Background()
	_, err := provider.GenerateText(ctx, "test", nil)
	assert.Error(t, err)
}

func TestNewAnimaAIProvider(t *testing.T) {
	provider := NewAnimaAIProvider(nil)

	require.NotNil(t, provider)

	ctx := context.Background()
	_, err := provider.GenerateText(ctx, "test", nil)
	assert.Error(t, err)
}
