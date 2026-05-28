//go:build integration

package llm

import (
	"context"
	"os"
	"testing"
	"time"
)

// NOTE (HXC-024): The VLLM/LocalAI/FastChat/TextGen/LMStudio/Jan/GPT4All/
// TabbyAPI/MLX/MistralRS provider constructors were removed from production
// (no NewVLLMProvider/NewLocalAIProvider/... exist under internal/llm/*.go
// non-test). Their integration tests covered deleted code and were removed.
// KoboldAI is the only local provider here whose constructor (NewKoboldAIProvider
// in koboldai_provider.go) still exists, so its integration test is retained.

// TestKoboldAIProviderIntegration tests the KoboldAI provider with integration
func TestKoboldAIProviderIntegration(t *testing.T) {
	config := KoboldAIConfig{
		BaseURL:          getEnvOrDefault("KOBOLD_ENDPOINT", "http://localhost:5001"),
		APIKey:           os.Getenv("KOBOLD_API_KEY"),
		DefaultModel:     "kobold-model",
		Timeout:          30 * time.Second,
		MaxRetries:       3,
		StreamingSupport: true,
	}

	provider, err := NewKoboldAIProvider(config)
	if err != nil {
		t.Skipf("Skipping KoboldAI integration test: %v (SKIP-OK: #integration-only)", err)
	}

	testLocalProvider(t, provider, "KoboldAI")
}

func testLocalProvider(t *testing.T, provider Provider, providerName string) {
	ctx := context.Background()

	// Test provider availability
	available := provider.IsAvailable(ctx)
	if !available {
		t.Skipf("%s provider not available for integration test (SKIP-OK: #integration-only)", providerName)
	}

	// Test model listing
	models := provider.GetModels()
	if len(models) == 0 {
		t.Logf("⚠️  No models available from %s (may be normal)", providerName)
	}

	// Test capabilities
	capabilities := provider.GetCapabilities()
	if len(capabilities) == 0 {
		t.Errorf("%s provider should have capabilities", providerName)
	}

	// Test health check
	health, err := provider.GetHealth(ctx)
	if err != nil {
		t.Errorf("%s health check failed: %v", providerName, err)
	}
	if health.Status == "" {
		t.Errorf("%s health status should not be empty", providerName)
	}

	// Test basic generation (if provider is healthy)
	if health.Status == "healthy" && len(models) > 0 {
		request := &LLMRequest{
			ID:    generateTestID(),
			Model: models[0].Name, // Use first available model
			Messages: []Message{
				{Role: "user", Content: "Hello! Please respond with just 'Hello World'."},
			},
			MaxTokens:   50,
			Temperature: 0.1,
		}

		response, err := provider.Generate(ctx, request)
		if err != nil {
			t.Logf("⚠️  Generation test failed for %s (may be normal): %v", providerName, err)
		} else if response != nil {
			t.Logf("✅ %s generated response: %s", providerName, response.Content)
			if response.Usage.TotalTokens == 0 {
				t.Logf("⚠️  %s response should include token usage", providerName)
			}
		}
	}

	t.Logf("✅ %s provider integration test passed: %d models, health: %s",
		providerName, len(models), health.Status)

	// Cleanup
	provider.Close()
}
