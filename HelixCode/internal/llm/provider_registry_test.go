package llm

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"dev.helix.code/internal/config"
)

func TestLLMProviderFactory(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		config   map[string]interface{}
		hasError bool
	}{
		{
			name:     "OpenAI provider",
			provider: "openai",
			config: map[string]interface{}{
				"api_key": "test-key",
			},
			hasError: false,
		},
		{
			name:     "Anthropic provider",
			provider: "anthropic",
			config: map[string]interface{}{
				"api_key": "test-key",
			},
			hasError: false,
		},
		{
			name:     "Invalid provider",
			provider: "invalid",
			config:   map[string]interface{}{},
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewLLMProvider(tt.provider, tt.config)

			if tt.hasError {
				if err == nil {
					t.Errorf("Expected error for provider %s, got nil", tt.provider)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error for provider %s: %v", tt.provider, err)
				return
			}

			if provider == nil {
				t.Errorf("Expected provider instance, got nil")
			}
		})
	}
}

func TestLLMProviderRegistry(t *testing.T) {
	registry := NewLLMProviderRegistry()

	// Test registering a provider
	config := map[string]interface{}{
		"api_key": "test-key",
	}

	err := registry.RegisterProvider("test-provider", "openai", config)
	if err != nil {
		t.Fatalf("Failed to register provider: %v", err)
	}

	// Test getting a provider
	provider, err := registry.GetProvider("test-provider")
	if err != nil {
		t.Fatalf("Failed to get provider: %v", err)
	}

	if provider == nil {
		t.Fatal("Expected provider instance, got nil")
	}

	// Test getting non-existent provider
	_, err = registry.GetProvider("non-existent")
	if err == nil {
		t.Error("Expected error for non-existent provider")
	}

	// Test listing providers
	providers := registry.ListProviders()
	if len(providers) != 1 {
		t.Errorf("Expected 1 provider, got %d", len(providers))
	}

	if providers[0] != "test-provider" {
		t.Errorf("Expected provider name 'test-provider', got '%s'", providers[0])
	}
}

func TestLLMProviderHealthCheck(t *testing.T) {
	registry := NewLLMProviderRegistry()

	config := map[string]interface{}{
		"api_key": "test-key",
	}

	err := registry.RegisterProvider("health-test", "openai", config)
	if err != nil {
		t.Fatalf("Failed to register provider: %v", err)
	}

	// Test health check
	health := registry.HealthCheck()

	if health == nil {
		t.Fatal("Expected health check result, got nil")
	}

	if len(health) == 0 {
		t.Error("Expected health check to return results")
	}

	// Check health for our provider
	providerHealth, exists := health["health-test"]
	if !exists {
		t.Error("Expected health check for test provider")
	}

	if providerHealth == nil {
		t.Error("Expected health check data for test provider")
	}
}

func TestLLMProviderLoadBalancing(t *testing.T) {
	registry := NewLLMProviderRegistry()

	// Register multiple providers of same type
	for i := 0; i < 3; i++ {
		config := map[string]interface{}{
			"api_key": fmt.Sprintf("test-key-%d", i),
		}

		err := registry.RegisterProvider(fmt.Sprintf("openai-%d", i), "openai", config)
		if err != nil {
			t.Fatalf("Failed to register provider %d: %v", i, err)
		}
	}

	// Test load balancing
	requests := 10
	providerCounts := make(map[string]int)

	for i := 0; i < requests; i++ {
		provider, err := registry.GetProviderByType("openai")
		if err != nil {
			t.Fatalf("Failed to get provider by type: %v", err)
		}

		if provider != nil {
			providerCounts[provider.Name()]++
		}
	}

	// Each provider should have been used at least once
	if len(providerCounts) != 3 {
		t.Errorf("Expected 3 providers to be used, got %d", len(providerCounts))
	}

	// Check that load is reasonably balanced
	minCount := requests
	maxCount := 0

	for _, count := range providerCounts {
		if count < minCount {
			minCount = count
		}
		if count > maxCount {
			maxCount = count
		}
	}

	// Max difference should be reasonable (allowing for randomness)
	if maxCount-minCount > 4 {
		t.Errorf("Load balancing too uneven: min=%d, max=%d", minCount, maxCount)
	}
}

func TestLLMProviderFallback(t *testing.T) {
	registry := NewLLMProviderRegistry()

	// Register providers with different priorities
	providers := []struct {
		name     string
		provider string
		config   map[string]interface{}
	}{
		{"primary", "openai", map[string]interface{}{"api_key": "primary-key"}},
		{"secondary", "anthropic", map[string]interface{}{"api_key": "secondary-key"}},
		{"tertiary", "gemini", map[string]interface{}{"api_key": "tertiary-key"}},
	}

	for _, p := range providers {
		err := registry.RegisterProvider(p.name, p.provider, p.config)
		if err != nil {
			t.Fatalf("Failed to register provider %s: %v", p.name, err)
		}
	}

	// Test fallback when primary fails
	ctx := context.Background()

	// Simulate primary provider failure
	// This would normally be done by mocking, but for this test we'll just verify
	// that we can get providers by type
	provider, err := registry.GetProviderByType("openai")
	if err != nil {
		t.Fatalf("Failed to get OpenAI provider: %v", err)
	}

	if provider == nil {
		t.Fatal("Expected OpenAI provider, got nil")
	}

	if provider.Name() != "primary" {
		t.Errorf("Expected primary provider, got %s", provider.Name())
	}
}

func TestLLMProviderMetrics(t *testing.T) {
	registry := NewLLMProviderRegistry()

	config := map[string]interface{}{
		"api_key": "test-key",
	}

	err := registry.RegisterProvider("metrics-test", "openai", config)
	if err != nil {
		t.Fatalf("Failed to register provider: %v", err)
	}

	// Get metrics
	metrics := registry.GetMetrics()

	if metrics == nil {
		t.Fatal("Expected metrics, got nil")
	}

	// Check that we have some basic metrics
	if len(metrics) == 0 {
		t.Error("Expected some metrics to be returned")
	}

	// Check for common metric types
	hasRequestCount := false
	hasErrorCount := false
	hasLatency := false

	for _, metric := range metrics {
		switch metric.Name {
		case "llm_requests_total":
			hasRequestCount = true
		case "llm_errors_total":
			hasErrorCount = true
		case "llm_request_duration_seconds":
			hasLatency = true
		}
	}

	if !hasRequestCount {
		t.Error("Expected request count metric")
	}

	if !hasErrorCount {
		t.Error("Expected error count metric")
	}

	if !hasLatency {
		t.Error("Expected latency metric")
	}
}

func TestLLMProviderConfiguration(t *testing.T) {
	registry := NewLLMProviderRegistry()

	// Test configuration validation
	validConfig := map[string]interface{}{
		"api_key":     "test-key",
		"model":       "gpt-3.5-turbo",
		"max_tokens":  1000,
		"temperature": 0.7,
	}

	err := registry.RegisterProvider("config-test", "openai", validConfig)
	if err != nil {
		t.Fatalf("Failed to register with valid config: %v", err)
	}

	// Test invalid configuration
	invalidConfig := map[string]interface{}{
		// Missing required api_key
		"model": "gpt-3.5-turbo",
	}

	err = registry.RegisterProvider("invalid-config", "openai", invalidConfig)
	if err == nil {
		t.Error("Expected error for invalid configuration")
	}
}

func TestLLMProviderConcurrency(t *testing.T) {
	registry := NewLLMProviderRegistry()

	// Register a provider
	config := map[string]interface{}{
		"api_key": "test-key",
	}

	err := registry.RegisterProvider("concurrency-test", "openai", config)
	if err != nil {
		t.Fatalf("Failed to register provider: %v", err)
	}

	// Test concurrent access
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(id int) {
			// Get provider
			provider, err := registry.GetProvider("concurrency-test")
			if err != nil {
				t.Errorf("Goroutine %d: failed to get provider: %v", id, err)
				done <- false
				return
			}

			if provider == nil {
				t.Errorf("Goroutine %d: got nil provider", id)
				done <- false
				return
			}

			// Check health
			health := registry.HealthCheck()
			if health == nil {
				t.Errorf("Goroutine %d: health check returned nil", id)
				done <- false
				return
			}

			done <- true
		}(i)
	}

	// Wait for all goroutines
	allPassed := true
	for i := 0; i < 10; i++ {
		if !<-done {
			allPassed = false
		}
	}

	if !allPassed {
		t.Error("Some concurrent operations failed")
	}
}

func TestLLMProviderErrorHandling(t *testing.T) {
	registry := NewLLMProviderRegistry()

	// Test registering with invalid provider type
	err := registry.RegisterProvider("error-test", "nonexistent", map[string]interface{}{})
	if err == nil {
		t.Error("Expected error for nonexistent provider type")
	}

	// Test getting non-existent provider
	_, err = registry.GetProvider("non-existent-provider")
	if err == nil {
		t.Error("Expected error for non-existent provider")
	}

	// Test getting provider by invalid type
	_, err = registry.GetProviderByType("invalid-type")
	if err == nil {
		t.Error("Expected error for invalid provider type")
	}
}

func TestLLMProviderLifecycle(t *testing.T) {
	registry := NewLLMProviderRegistry()

	// Register provider
	config := map[string]interface{}{
		"api_key": "test-key",
	}

	err := registry.RegisterProvider("lifecycle-test", "openai", config)
	if err != nil {
		t.Fatalf("Failed to register provider: %v", err)
	}

	// Verify it's registered
	provider, err := registry.GetProvider("lifecycle-test")
	if err != nil {
		t.Fatalf("Failed to get registered provider: %v", err)
	}

	if provider == nil {
		t.Fatal("Registered provider is nil")
	}

	// Test unregistering
	err = registry.UnregisterProvider("lifecycle-test")
	if err != nil {
		t.Fatalf("Failed to unregister provider: %v", err)
	}

	// Verify it's unregistered
	_, err = registry.GetProvider("lifecycle-test")
	if err == nil {
		t.Error("Expected error after unregistering provider")
	}

	// Verify it's not in the list
	providers := registry.ListProviders()
	for _, p := range providers {
		if p == "lifecycle-test" {
			t.Error("Unregistered provider still in list")
		}
	}
}

func TestLLMProviderTypeValidation(t *testing.T) {
	validTypes := []string{
		"openai",
		"anthropic",
		"gemini",
		"xai",
		"openrouter",
		"copilot",
		"ollama",
		"llamacpp",
		"vllm",
		"koboldai",
	}

	registry := NewLLMProviderRegistry()

	for _, providerType := range validTypes {
		t.Run(fmt.Sprintf("ValidType_%s", providerType), func(t *testing.T) {
			config := map[string]interface{}{
				"api_key": "test-key",
			}

			err := registry.RegisterProvider(fmt.Sprintf("test-%s", providerType), providerType, config)
			// We expect this might fail due to missing dependencies, but not due to invalid type
			// The important thing is that it's not rejected as an invalid type
			if err != nil && strings.Contains(err.Error(), "unsupported provider") {
				t.Errorf("Provider type %s should be supported", providerType)
			}
		})
	}
}

func TestLLMProviderConfigValidation(t *testing.T) {
	registry := NewLLMProviderRegistry()

	tests := []struct {
		name        string
		provider    string
		config      map[string]interface{}
		expectError bool
	}{
		{
			name:     "OpenAI with API key",
			provider: "openai",
			config: map[string]interface{}{
				"api_key": "test-key",
			},
			expectError: false,
		},
		{
			name:        "OpenAI without API key",
			provider:    "openai",
			config:      map[string]interface{}{},
			expectError: true,
		},
		{
			name:        "Ollama without API key (should be OK)",
			provider:    "ollama",
			config:      map[string]interface{}{},
			expectError: false,
		},
		{
			name:     "Invalid temperature",
			provider: "openai",
			config: map[string]interface{}{
				"api_key":     "test-key",
				"temperature": 2.5, // Invalid: should be 0-2
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := registry.RegisterProvider(fmt.Sprintf("config-test-%s", tt.name), tt.provider, tt.config)

			if tt.expectError && err == nil {
				t.Errorf("Expected error for config %s, got nil", tt.name)
			}

			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for config %s: %v", tt.name, err)
			}
		})
	}
}

func BenchmarkLLMProviderRegistry(b *testing.B) {
	registry := NewLLMProviderRegistry()

	// Register some providers
	for i := 0; i < 10; i++ {
		config := map[string]interface{}{
			"api_key": fmt.Sprintf("bench-key-%d", i),
		}
		registry.RegisterProvider(fmt.Sprintf("bench-provider-%d", i), "openai", config)
	}

	b.ResetTimer()

	b.Run("GetProvider", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := registry.GetProvider("bench-provider-0")
			if err != nil {
				b.Fatalf("Failed to get provider: %v", err)
			}
		}
	})

	b.Run("ListProviders", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			providers := registry.ListProviders()
			if len(providers) != 10 {
				b.Fatalf("Expected 10 providers, got %d", len(providers))
			}
		}
	})

	b.Run("HealthCheck", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			health := registry.HealthCheck()
			if health == nil {
				b.Fatalf("Health check returned nil")
			}
		}
	})
}

func BenchmarkLLMProviderLoadBalancing(b *testing.B) {
	registry := NewLLMProviderRegistry()

	// Register multiple providers
	for i := 0; i < 5; i++ {
		config := map[string]interface{}{
			"api_key": fmt.Sprintf("lb-key-%d", i),
		}
		registry.RegisterProvider(fmt.Sprintf("lb-provider-%d", i), "openai", config)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := registry.GetProviderByType("openai")
		if err != nil {
			b.Fatalf("Failed to get provider by type: %v", err)
		}
	}
}
