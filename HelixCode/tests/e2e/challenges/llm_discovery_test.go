package challenges

import (
	"context"
	"testing"
	"time"
)

func TestModelDiscovery_CacheExpiration(t *testing.T) {
	apiKeys := &APIKeys{}
	discovery := NewModelDiscovery(apiKeys)

	// Manually add cached data with short TTL
	discovery.mu.Lock()
	discovery.cache[ProviderAnthropic] = &ModelCache{
		Models: []ModelInfo{
			{ID: "test-model"},
		},
		Timestamp: time.Now().Add(-25 * time.Hour), // Expired (TTL is 24 hours)
		TTL:       24 * time.Hour,
	}
	discovery.mu.Unlock()

	// Should not use expired cache
	ctx := context.Background()
	models, err := discovery.DiscoverModels(ctx, ProviderAnthropic)

	// Anthropic returns hardcoded list, so should succeed
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if len(models) == 0 {
		t.Error("Expected models to be returned")
	}
}

func TestModelDiscovery_CacheValid(t *testing.T) {
	apiKeys := &APIKeys{}
	discovery := NewModelDiscovery(apiKeys)

	// Add fresh cached data
	cachedModels := []ModelInfo{
		{ID: "cached-model-1"},
		{ID: "cached-model-2"},
	}

	discovery.mu.Lock()
	discovery.cache[ProviderAnthropic] = &ModelCache{
		Models:    cachedModels,
		Timestamp: time.Now(),
		TTL:       24 * time.Hour,
	}
	discovery.mu.Unlock()

	// Should use valid cache
	ctx := context.Background()
	models, err := discovery.DiscoverModels(ctx, ProviderAnthropic)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if len(models) != len(cachedModels) {
		t.Errorf("Expected %d cached models, got %d", len(cachedModels), len(models))
	}

	if models[0].ID != "cached-model-1" {
		t.Errorf("Expected first model ID 'cached-model-1', got '%s'", models[0].ID)
	}
}

func TestModelDiscovery_UnsupportedProvider(t *testing.T) {
	apiKeys := &APIKeys{}
	discovery := NewModelDiscovery(apiKeys)

	ctx := context.Background()
	_, err := discovery.DiscoverModels(ctx, ProviderMistral)

	if err == nil {
		t.Error("Expected error for unsupported provider, got nil")
	}
}

func TestGetModelIDs(t *testing.T) {
	models := []ModelInfo{
		{ID: "model-1", Name: "Model One"},
		{ID: "model-2", Name: "Model Two"},
		{ID: "model-3", Name: "Model Three"},
	}

	ids := GetModelIDs(models)

	if len(ids) != 3 {
		t.Errorf("Expected 3 IDs, got %d", len(ids))
	}

	expected := []string{"model-1", "model-2", "model-3"}
	for i, id := range ids {
		if id != expected[i] {
			t.Errorf("Expected ID '%s' at position %d, got '%s'", expected[i], i, id)
		}
	}
}

func TestModelDiscovery_ClearCache(t *testing.T) {
	apiKeys := &APIKeys{}
	discovery := NewModelDiscovery(apiKeys)

	// Add cache for multiple providers
	discovery.mu.Lock()
	discovery.cache[ProviderOpenAI] = &ModelCache{
		Models:    []ModelInfo{{ID: "gpt-4"}},
		Timestamp: time.Now(),
		TTL:       24 * time.Hour,
	}
	discovery.cache[ProviderXAI] = &ModelCache{
		Models:    []ModelInfo{{ID: "grok-beta"}},
		Timestamp: time.Now(),
		TTL:       24 * time.Hour,
	}
	discovery.mu.Unlock()

	// Clear specific provider cache
	provider := ProviderOpenAI
	discovery.ClearCache(&provider)

	discovery.mu.RLock()
	if _, exists := discovery.cache[ProviderOpenAI]; exists {
		t.Error("Expected OpenAI cache to be cleared")
	}
	if _, exists := discovery.cache[ProviderXAI]; !exists {
		t.Error("Expected xAI cache to still exist")
	}
	discovery.mu.RUnlock()

	// Clear all caches
	discovery.ClearCache(nil)

	discovery.mu.RLock()
	if len(discovery.cache) != 0 {
		t.Errorf("Expected all caches to be cleared, got %d entries", len(discovery.cache))
	}
	discovery.mu.RUnlock()
}

func TestGetSupportedModelsWithDiscovery_Fallback(t *testing.T) {
	// Test with nil API keys - should fall back to static list
	models := GetSupportedModelsWithDiscovery(ProviderOpenAI, nil)

	if len(models) == 0 {
		t.Error("Expected static models to be returned when API keys are nil")
	}

	// Verify we got the static list
	staticModels := GetSupportedModels(ProviderOpenAI)
	if len(models) != len(staticModels) {
		t.Errorf("Expected fallback to static list with %d models, got %d", len(staticModels), len(models))
	}
}

func TestGetSupportedModelsWithDiscovery_UnsupportedProvider(t *testing.T) {
	apiKeys := &APIKeys{}

	// Provider that doesn't support discovery should return static list
	models := GetSupportedModelsWithDiscovery(ProviderMistral, apiKeys)
	staticModels := GetSupportedModels(ProviderMistral)

	if len(models) != len(staticModels) {
		t.Errorf("Expected static list for unsupported provider, got different length")
	}
}

func TestSupportsDiscovery(t *testing.T) {
	tests := []struct {
		provider LLMProviderType
		expected bool
	}{
		{ProviderOpenAI, true},
		{ProviderXAI, true},
		{ProviderDeepSeek, true},
		{ProviderGroq, true},
		{ProviderOllama, true},
		{ProviderAnthropic, false},
		{ProviderMistral, false},
		{ProviderCohere, false},
		{ProviderGemini, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.provider), func(t *testing.T) {
			result := supportsDiscovery(tt.provider)
			if result != tt.expected {
				t.Errorf("Expected supportsDiscovery(%s) = %v, got %v", tt.provider, tt.expected, result)
			}
		})
	}
}

func TestModelDiscovery_Anthropic_Hardcoded(t *testing.T) {
	// Anthropic doesn't have a models endpoint, so it returns hardcoded list
	apiKeys := &APIKeys{}
	discovery := NewModelDiscovery(apiKeys)

	ctx := context.Background()
	models, err := discovery.DiscoverModels(ctx, ProviderAnthropic)

	if err != nil {
		t.Errorf("Expected no error for Anthropic, got: %v", err)
	}

	if len(models) == 0 {
		t.Error("Expected Anthropic to return hardcoded models")
	}

	// Check for expected Claude models
	foundClaude3 := false
	for _, m := range models {
		if m.ID == "claude-3-opus-20240229" {
			foundClaude3 = true
			break
		}
	}

	if !foundClaude3 {
		t.Error("Expected to find Claude 3 Opus in Anthropic models")
	}
}

func TestGetModelDetails_UnsupportedProvider(t *testing.T) {
	apiKeys := &APIKeys{}

	models, err := GetModelDetails(ProviderMistral, apiKeys)

	if err != nil {
		t.Errorf("Expected no error for unsupported provider, got: %v", err)
	}

	if models != nil {
		t.Error("Expected nil models for unsupported provider")
	}
}

func TestGetModelDetails_SupportedProvider(t *testing.T) {
	apiKeys := &APIKeys{}

	// Anthropic doesn't support discovery via API, should return nil
	models, err := GetModelDetails(ProviderAnthropic, apiKeys)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Anthropic doesn't support discovery, so should return nil
	if models != nil {
		t.Error("Expected nil models for Anthropic (doesn't support discovery)")
	}
}
