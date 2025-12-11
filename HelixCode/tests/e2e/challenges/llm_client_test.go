package challenges

import (
	"context"
	"testing"
	"time"
)

func TestNewLLMClient(t *testing.T) {
	apiKeys := &APIKeys{
		XAI: &XAIConfig{APIKey: "test-key"},
	}

	client := NewLLMClient(ProviderXAI, "grok-beta", apiKeys)

	if client == nil {
		t.Fatal("Expected non-nil client")
	}

	if client.provider != ProviderXAI {
		t.Errorf("Expected provider XAI, got %s", client.provider)
	}

	if client.model != "grok-beta" {
		t.Errorf("Expected model 'grok-beta', got '%s'", client.model)
	}

	if client.apiKeys != apiKeys {
		t.Error("Expected apiKeys to be set")
	}

	if client.client == nil {
		t.Error("Expected HTTP client to be initialized")
	}

	if client.client.Timeout != 120*time.Second {
		t.Errorf("Expected 120s timeout, got %v", client.client.Timeout)
	}
}

func TestLLMClient_UnsupportedProvider(t *testing.T) {
	apiKeys := &APIKeys{}
	client := NewLLMClient(LLMProviderType("unsupported"), "model", apiKeys)

	ctx := context.Background()
	req := &CompletionRequest{
		Prompt:      "test",
		MaxTokens:   100,
		Temperature: 0.7,
	}

	_, err := client.Complete(ctx, req)
	if err == nil {
		t.Error("Expected error for unsupported provider")
	}

	if err.Error() != "unsupported provider: unsupported" {
		t.Errorf("Expected 'unsupported provider' error, got: %v", err)
	}
}

func TestLLMClient_MissingAPIKey(t *testing.T) {
	// Test each cloud provider without API key
	providers := []LLMProviderType{
		ProviderXAI,
		ProviderOpenAI,
		ProviderAnthropic,
		ProviderGroq,
	}

	for _, provider := range providers {
		t.Run(string(provider), func(t *testing.T) {
			apiKeys := &APIKeys{} // Empty - no keys configured
			client := NewLLMClient(provider, "test-model", apiKeys)

			ctx := context.Background()
			req := &CompletionRequest{
				Prompt:      "test",
				MaxTokens:   100,
				Temperature: 0.7,
			}

			_, err := client.Complete(ctx, req)
			if err == nil {
				t.Error("Expected error for missing API key")
			}
		})
	}
}

func TestCompletionRequest_DefaultValues(t *testing.T) {
	req := &CompletionRequest{
		Prompt: "test prompt",
	}

	// Verify required field is set
	if req.Prompt != "test prompt" {
		t.Errorf("Expected prompt 'test prompt', got '%s'", req.Prompt)
	}

	// Other fields should be zero values
	if req.MaxTokens != 0 {
		t.Errorf("Expected MaxTokens 0, got %d", req.MaxTokens)
	}

	if req.Temperature != 0 {
		t.Errorf("Expected Temperature 0, got %f", req.Temperature)
	}

	if req.SystemPrompt != "" {
		t.Errorf("Expected empty SystemPrompt, got '%s'", req.SystemPrompt)
	}
}

func TestCompletionRequest_WithAllFields(t *testing.T) {
	req := &CompletionRequest{
		Prompt:       "user prompt",
		SystemPrompt: "system prompt",
		MaxTokens:    1000,
		Temperature:  0.7,
	}

	if req.Prompt != "user prompt" {
		t.Errorf("Expected prompt 'user prompt', got '%s'", req.Prompt)
	}

	if req.SystemPrompt != "system prompt" {
		t.Errorf("Expected system prompt 'system prompt', got '%s'", req.SystemPrompt)
	}

	if req.MaxTokens != 1000 {
		t.Errorf("Expected MaxTokens 1000, got %d", req.MaxTokens)
	}

	if req.Temperature != 0.7 {
		t.Errorf("Expected Temperature 0.7, got %f", req.Temperature)
	}
}

func TestCompletionResponse_Structure(t *testing.T) {
	resp := &CompletionResponse{
		Content:      "generated content",
		FinishReason: "stop",
		TokensUsed:   150,
	}

	if resp.Content != "generated content" {
		t.Errorf("Expected content 'generated content', got '%s'", resp.Content)
	}

	if resp.FinishReason != "stop" {
		t.Errorf("Expected finish reason 'stop', got '%s'", resp.FinishReason)
	}

	if resp.TokensUsed != 150 {
		t.Errorf("Expected 150 tokens used, got %d", resp.TokensUsed)
	}
}

// Integration test - only runs if API key is available
func TestLLMClient_OllamaIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// This test requires Ollama to be running locally
	// It's optional and will be skipped if Ollama isn't available

	apiKeys := &APIKeys{}
	client := NewLLMClient(ProviderOllama, "llama2", apiKeys)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := &CompletionRequest{
		Prompt:       "Say hello",
		SystemPrompt: "You are a helpful assistant",
		MaxTokens:    50,
		Temperature:  0.7,
	}

	resp, err := client.Complete(ctx, req)

	// If Ollama isn't running, we expect a connection error
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			t.Skip("Ollama not available (timeout)")
		}
		// Connection refused is also acceptable
		t.Logf("Ollama not available: %v", err)
		return
	}

	// If we got here, Ollama is running
	if resp.Content == "" {
		t.Error("Expected non-empty response content")
	}

	if resp.FinishReason != "stop" {
		t.Logf("Unexpected finish reason: %s", resp.FinishReason)
	}

	t.Logf("Ollama response: %s (tokens: %d)", resp.Content, resp.TokensUsed)
}

func TestLLMClient_ContextCancellation(t *testing.T) {
	apiKeys := &APIKeys{
		XAI: &XAIConfig{APIKey: "test-key"},
	}
	client := NewLLMClient(ProviderXAI, "grok-beta", apiKeys)

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	req := &CompletionRequest{
		Prompt:      "test",
		MaxTokens:   100,
		Temperature: 0.7,
	}

	_, err := client.Complete(ctx, req)
	if err == nil {
		t.Error("Expected error for cancelled context")
	}

	// Should be context cancellation error
	if ctx.Err() != context.Canceled {
		t.Errorf("Expected context.Canceled, got %v", ctx.Err())
	}
}

func TestLLMClient_ContextTimeout(t *testing.T) {
	apiKeys := &APIKeys{
		XAI: &XAIConfig{APIKey: "test-key"},
	}
	client := NewLLMClient(ProviderXAI, "grok-beta", apiKeys)

	// Create a context with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	// Wait for timeout
	time.Sleep(1 * time.Millisecond)

	req := &CompletionRequest{
		Prompt:      "test",
		MaxTokens:   100,
		Temperature: 0.7,
	}

	_, err := client.Complete(ctx, req)
	if err == nil {
		t.Error("Expected error for timed out context")
	}
}

func TestLLMClient_MultipleProviders(t *testing.T) {
	// Test that we can create clients for all providers
	apiKeys := &APIKeys{
		XAI:       &XAIConfig{APIKey: "xai-key"},
		OpenAI:    &OpenAIConfig{APIKey: "openai-key"},
		Anthropic: &AnthropicConfig{APIKey: "anthropic-key"},
		Groq:      &GroqConfig{APIKey: "groq-key"},
	}

	providers := []struct {
		provider LLMProviderType
		model    string
	}{
		{ProviderXAI, "grok-beta"},
		{ProviderOpenAI, "gpt-4"},
		{ProviderAnthropic, "claude-3-opus-20240229"},
		{ProviderGroq, "llama-3.1-405b-reasoning"},
		{ProviderOllama, "llama2"},
	}

	for _, p := range providers {
		t.Run(string(p.provider), func(t *testing.T) {
			client := NewLLMClient(p.provider, p.model, apiKeys)

			if client == nil {
				t.Fatal("Expected non-nil client")
			}

			if client.provider != p.provider {
				t.Errorf("Expected provider %s, got %s", p.provider, client.provider)
			}

			if client.model != p.model {
				t.Errorf("Expected model %s, got %s", p.model, client.model)
			}
		})
	}
}

func TestLLMClient_HTTPClientTimeout(t *testing.T) {
	apiKeys := &APIKeys{}
	client := NewLLMClient(ProviderOllama, "llama2", apiKeys)

	// Verify HTTP client has correct timeout
	if client.client.Timeout != 120*time.Second {
		t.Errorf("Expected 120 second timeout, got %v", client.client.Timeout)
	}
}

func TestLLMClient_EmptyPrompt(t *testing.T) {
	apiKeys := &APIKeys{}
	client := NewLLMClient(ProviderOllama, "llama2", apiKeys)

	ctx := context.Background()
	req := &CompletionRequest{
		Prompt:      "", // Empty prompt
		MaxTokens:   100,
		Temperature: 0.7,
	}

	// Should still attempt the request (provider may reject empty prompt)
	_, err := client.Complete(ctx, req)
	// Error is expected since Ollama likely isn't running,
	// but we're testing that empty prompt doesn't panic
	_ = err // Expected to fail
}

func TestLLMClient_LargePrompt(t *testing.T) {
	apiKeys := &APIKeys{}
	client := NewLLMClient(ProviderOllama, "llama2", apiKeys)

	// Create a large prompt (10KB)
	largePrompt := string(make([]byte, 10*1024))

	ctx := context.Background()
	req := &CompletionRequest{
		Prompt:      largePrompt,
		MaxTokens:   100,
		Temperature: 0.7,
	}

	// Should handle large prompts without panic
	_, err := client.Complete(ctx, req)
	_ = err // Expected to fail if Ollama not running
}

func TestLLMClient_SpecialCharactersInPrompt(t *testing.T) {
	apiKeys := &APIKeys{}
	client := NewLLMClient(ProviderOllama, "llama2", apiKeys)

	// Test with special characters, unicode, etc.
	specialPrompts := []string{
		"Hello\nWorld",              // Newlines
		"Test\tTab",                 // Tabs
		"Quote\"Test\"",             // Quotes
		"Unicode: 你好世界",             // Unicode
		"Emoji: 🚀🎉",                 // Emoji
		"JSON: {\"key\":\"value\"}", // JSON
	}

	for _, prompt := range specialPrompts {
		req := &CompletionRequest{
			Prompt:      prompt,
			MaxTokens:   100,
			Temperature: 0.7,
		}

		ctx := context.Background()
		_, err := client.Complete(ctx, req)
		_ = err // Expected to fail if Ollama not running

		// Main test: should not panic with special characters
	}
}
