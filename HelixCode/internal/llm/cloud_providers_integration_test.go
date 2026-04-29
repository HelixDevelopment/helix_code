//go:build integration

package llm

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Cloud Provider Integration Tests
// These tests make REAL API calls to cloud providers.
// They require valid API keys in environment variables.
//
// To run:
//   export OPENAI_API_KEY=sk-...
//   export ANTHROPIC_API_KEY=sk-ant-...
//   export GEMINI_API_KEY=...
//   export GROQ_API_KEY=gsk_...
//   go test -v -tags=integration ./internal/llm/... -run "CloudProvider"

// =============================================================================
// OpenAI Provider Integration Tests
// =============================================================================

func TestCloudProvider_OpenAI_Generate(t *testing.T) {
	apiKey := getEnvOrSkip(t, "OPENAI_API_KEY", "OPENAI_API_KEY not set, skipping OpenAI integration test")

	// Skip if it's a mock key
	if strings.Contains(apiKey, "mock") {
		t.Skip("Skipping OpenAI test - mock API key detected")  // SKIP-OK: #requires-upstream-key
	}

	config := ProviderConfigEntry{
		Type:     ProviderTypeOpenAI,
		Endpoint: "https://api.openai.com/v1",
		APIKey:   apiKey,
	}

	provider, err := NewOpenAIProvider(config)
	require.NoError(t, err)
	defer provider.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Test basic generation
	request := &LLMRequest{
		ID:    uuid.New(),
		Model: "gpt-4o-mini", // Use cost-effective model for testing
		Messages: []Message{
			{Role: "user", Content: "Respond with exactly: 'Hello from OpenAI!'"},
		},
		MaxTokens:   50,
		Temperature: 0.0, // Deterministic for testing
	}

	response, err := provider.Generate(ctx, request)
	require.NoError(t, err, "OpenAI Generate should succeed")

	assert.NotEmpty(t, response.Content, "Response content should not be empty")
	assert.Greater(t, response.Usage.TotalTokens, 0, "Usage should report tokens")

	t.Logf("✅ OpenAI Generate test passed")
	t.Logf("   Model: %s", request.Model)
	t.Logf("   Response: %s", truncateString(response.Content, 100))
	t.Logf("   Tokens: prompt=%d, completion=%d, total=%d",
		response.Usage.PromptTokens, response.Usage.CompletionTokens, response.Usage.TotalTokens)
}

func TestCloudProvider_OpenAI_Health(t *testing.T) {
	apiKey := getEnvOrSkip(t, "OPENAI_API_KEY", "OPENAI_API_KEY not set, skipping OpenAI health test")

	if strings.Contains(apiKey, "mock") {
		t.Skip("Skipping OpenAI test - mock API key detected")  // SKIP-OK: #requires-upstream-key
	}

	config := ProviderConfigEntry{
		Type:     ProviderTypeOpenAI,
		Endpoint: "https://api.openai.com/v1",
		APIKey:   apiKey,
	}

	provider, err := NewOpenAIProvider(config)
	require.NoError(t, err)
	defer provider.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	health, err := provider.GetHealth(ctx)
	require.NoError(t, err, "OpenAI GetHealth should succeed")

	assert.Equal(t, "healthy", health.Status, "OpenAI should be healthy")
	assert.Greater(t, health.Latency, time.Duration(0), "Latency should be measured")
	assert.Greater(t, health.ModelCount, 0, "Should have models available")

	t.Logf("✅ OpenAI Health test passed: status=%s, latency=%v, models=%d",
		health.Status, health.Latency, health.ModelCount)
}

// =============================================================================
// Anthropic Provider Integration Tests
// =============================================================================

func TestCloudProvider_Anthropic_Generate(t *testing.T) {
	apiKey := getEnvOrSkip(t, "ANTHROPIC_API_KEY", "ANTHROPIC_API_KEY not set, skipping Anthropic integration test")

	if strings.Contains(apiKey, "mock") {
		t.Skip("Skipping Anthropic test - mock API key detected")  // SKIP-OK: #requires-upstream-key
	}

	config := ProviderConfigEntry{
		Type:     ProviderTypeAnthropic,
		Endpoint: "https://api.anthropic.com",
		APIKey:   apiKey,
	}

	provider, err := NewAnthropicProvider(config)
	require.NoError(t, err)
	defer provider.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	request := &LLMRequest{
		ID:    uuid.New(),
		Model: "claude-3-5-haiku-latest", // Use cost-effective model
		Messages: []Message{
			{Role: "user", Content: "Respond with exactly: 'Hello from Anthropic!'"},
		},
		MaxTokens:   50,
		Temperature: 0.0,
	}

	response, err := provider.Generate(ctx, request)
	require.NoError(t, err, "Anthropic Generate should succeed")

	assert.NotEmpty(t, response.Content, "Response content should not be empty")
	assert.Greater(t, response.Usage.TotalTokens, 0, "Usage should report tokens")

	t.Logf("✅ Anthropic Generate test passed")
	t.Logf("   Model: %s", request.Model)
	t.Logf("   Response: %s", truncateString(response.Content, 100))
	t.Logf("   Tokens: prompt=%d, completion=%d, total=%d",
		response.Usage.PromptTokens, response.Usage.CompletionTokens, response.Usage.TotalTokens)
}

func TestCloudProvider_Anthropic_Health(t *testing.T) {
	apiKey := getEnvOrSkip(t, "ANTHROPIC_API_KEY", "ANTHROPIC_API_KEY not set, skipping Anthropic health test")

	if strings.Contains(apiKey, "mock") {
		t.Skip("Skipping Anthropic test - mock API key detected")  // SKIP-OK: #requires-upstream-key
	}

	config := ProviderConfigEntry{
		Type:     ProviderTypeAnthropic,
		Endpoint: "https://api.anthropic.com",
		APIKey:   apiKey,
	}

	provider, err := NewAnthropicProvider(config)
	require.NoError(t, err)
	defer provider.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	health, err := provider.GetHealth(ctx)
	require.NoError(t, err, "Anthropic GetHealth should succeed")

	assert.Equal(t, "healthy", health.Status, "Anthropic should be healthy")
	assert.Greater(t, health.Latency, time.Duration(0), "Latency should be measured")

	t.Logf("✅ Anthropic Health test passed: status=%s, latency=%v, models=%d",
		health.Status, health.Latency, health.ModelCount)
}

// =============================================================================
// Gemini Provider Integration Tests
// =============================================================================

func TestCloudProvider_Gemini_Generate(t *testing.T) {
	apiKey := getEnvOrSkip(t, "GEMINI_API_KEY", "GEMINI_API_KEY not set, skipping Gemini integration test")

	if strings.Contains(apiKey, "mock") {
		t.Skip("Skipping Gemini test - mock API key detected")  // SKIP-OK: #requires-upstream-key
	}

	config := ProviderConfigEntry{
		Type:     ProviderTypeGemini,
		Endpoint: "https://generativelanguage.googleapis.com/v1beta",
		APIKey:   apiKey,
	}

	provider, err := NewGeminiProvider(config)
	require.NoError(t, err)
	defer provider.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	request := &LLMRequest{
		ID:    uuid.New(),
		Model: "gemini-2.0-flash", // Use stable flash model
		Messages: []Message{
			{Role: "user", Content: "Respond with exactly: 'Hello from Gemini!'"},
		},
		MaxTokens:   50,
		Temperature: 0.0,
	}

	response, err := provider.Generate(ctx, request)
	require.NoError(t, err, "Gemini Generate should succeed")

	assert.NotEmpty(t, response.Content, "Response content should not be empty")
	assert.Greater(t, response.Usage.TotalTokens, 0, "Usage should report tokens")

	t.Logf("✅ Gemini Generate test passed")
	t.Logf("   Model: %s", request.Model)
	t.Logf("   Response: %s", truncateString(response.Content, 100))
	t.Logf("   Tokens: prompt=%d, completion=%d, total=%d",
		response.Usage.PromptTokens, response.Usage.CompletionTokens, response.Usage.TotalTokens)
}

func TestCloudProvider_Gemini_Health(t *testing.T) {
	apiKey := getEnvOrSkip(t, "GEMINI_API_KEY", "GEMINI_API_KEY not set, skipping Gemini health test")

	if strings.Contains(apiKey, "mock") {
		t.Skip("Skipping Gemini test - mock API key detected")  // SKIP-OK: #requires-upstream-key
	}

	config := ProviderConfigEntry{
		Type:     ProviderTypeGemini,
		Endpoint: "https://generativelanguage.googleapis.com/v1beta",
		APIKey:   apiKey,
	}

	provider, err := NewGeminiProvider(config)
	require.NoError(t, err)
	defer provider.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	health, err := provider.GetHealth(ctx)
	require.NoError(t, err, "Gemini GetHealth should succeed")

	assert.Equal(t, "healthy", health.Status, "Gemini should be healthy")
	assert.Greater(t, health.Latency, time.Duration(0), "Latency should be measured")

	t.Logf("✅ Gemini Health test passed: status=%s, latency=%v, models=%d",
		health.Status, health.Latency, health.ModelCount)
}

// =============================================================================
// Groq Provider Integration Tests
// =============================================================================

func TestCloudProvider_Groq_Generate(t *testing.T) {
	apiKey := getEnvOrSkip(t, "GROQ_API_KEY", "GROQ_API_KEY not set, skipping Groq integration test")

	if strings.Contains(apiKey, "mock") {
		t.Skip("Skipping Groq test - mock API key detected")  // SKIP-OK: #requires-upstream-key
	}

	config := ProviderConfigEntry{
		Type:     ProviderTypeGroq,
		Endpoint: "https://api.groq.com",
		APIKey:   apiKey,
	}

	provider, err := NewGroqProvider(config)
	require.NoError(t, err)
	defer provider.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	request := &LLMRequest{
		ID:    uuid.New(),
		Model: "llama-3.1-8b-instant", // Fast and free model
		Messages: []Message{
			{Role: "user", Content: "Respond with exactly: 'Hello from Groq!'"},
		},
		MaxTokens:   50,
		Temperature: 0.0,
	}

	response, err := provider.Generate(ctx, request)
	require.NoError(t, err, "Groq Generate should succeed")

	assert.NotEmpty(t, response.Content, "Response content should not be empty")
	assert.Greater(t, response.Usage.TotalTokens, 0, "Usage should report tokens")

	t.Logf("✅ Groq Generate test passed")
	t.Logf("   Model: %s", request.Model)
	t.Logf("   Response: %s", truncateString(response.Content, 100))
	t.Logf("   Tokens: prompt=%d, completion=%d, total=%d",
		response.Usage.PromptTokens, response.Usage.CompletionTokens, response.Usage.TotalTokens)
}

func TestCloudProvider_Groq_Health(t *testing.T) {
	apiKey := getEnvOrSkip(t, "GROQ_API_KEY", "GROQ_API_KEY not set, skipping Groq health test")

	if strings.Contains(apiKey, "mock") {
		t.Skip("Skipping Groq test - mock API key detected")  // SKIP-OK: #requires-upstream-key
	}

	config := ProviderConfigEntry{
		Type:     ProviderTypeGroq,
		Endpoint: "https://api.groq.com",
		APIKey:   apiKey,
	}

	provider, err := NewGroqProvider(config)
	require.NoError(t, err)
	defer provider.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	health, err := provider.GetHealth(ctx)
	require.NoError(t, err, "Groq GetHealth should succeed")

	assert.Equal(t, "healthy", health.Status, "Groq should be healthy")
	assert.Greater(t, health.Latency, time.Duration(0), "Latency should be measured")

	t.Logf("✅ Groq Health test passed: status=%s, latency=%v, models=%d",
		health.Status, health.Latency, health.ModelCount)
}

func TestCloudProvider_Groq_Streaming(t *testing.T) {
	apiKey := getEnvOrSkip(t, "GROQ_API_KEY", "GROQ_API_KEY not set, skipping Groq streaming test")

	if strings.Contains(apiKey, "mock") {
		t.Skip("Skipping Groq test - mock API key detected")  // SKIP-OK: #requires-upstream-key
	}

	config := ProviderConfigEntry{
		Type:     ProviderTypeGroq,
		Endpoint: "https://api.groq.com",
		APIKey:   apiKey,
	}

	provider, err := NewGroqProvider(config)
	require.NoError(t, err)
	defer provider.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	request := &LLMRequest{
		ID:    uuid.New(),
		Model: "llama-3.1-8b-instant",
		Messages: []Message{
			{Role: "user", Content: "Count from 1 to 5, each number on a new line."},
		},
		MaxTokens:   100,
		Temperature: 0.0,
		Stream:      true,
	}

	responseCh := make(chan LLMResponse, 100)
	var responses []LLMResponse

	go func() {
		err := provider.GenerateStream(ctx, request, responseCh)
		if err != nil {
			t.Errorf("GenerateStream error: %v", err)
		}
	}()

	for response := range responseCh {
		responses = append(responses, response)
	}

	assert.NotEmpty(t, responses, "Should receive streaming responses")

	var fullContent strings.Builder
	for _, resp := range responses {
		fullContent.WriteString(resp.Content)
	}

	content := fullContent.String()
	assert.NotEmpty(t, content, "Full content should not be empty")
	assert.Contains(t, content, "1", "Content should contain '1'")

	t.Logf("✅ Groq Streaming test passed")
	t.Logf("   Chunks received: %d", len(responses))
	t.Logf("   Full content: %s", truncateString(content, 100))
}

// =============================================================================
// XAI Provider Integration Tests
// =============================================================================

func TestCloudProvider_XAI_Generate(t *testing.T) {
	apiKey := getEnvOrSkip(t, "XAI_API_KEY", "XAI_API_KEY not set, skipping XAI integration test")

	if strings.Contains(apiKey, "mock") {
		t.Skip("Skipping XAI test - mock API key detected")  // SKIP-OK: #requires-upstream-key
	}

	config := ProviderConfigEntry{
		Type:     ProviderTypeXAI,
		Endpoint: "https://api.x.ai/v1",
		APIKey:   apiKey,
	}

	provider, err := NewXAIProvider(config)
	require.NoError(t, err)
	defer provider.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	request := &LLMRequest{
		ID:    uuid.New(),
		Model: "grok-beta",
		Messages: []Message{
			{Role: "user", Content: "Respond with exactly: 'Hello from xAI!'"},
		},
		MaxTokens:   50,
		Temperature: 0.0,
	}

	response, err := provider.Generate(ctx, request)
	require.NoError(t, err, "XAI Generate should succeed")

	assert.NotEmpty(t, response.Content, "Response content should not be empty")

	t.Logf("✅ XAI Generate test passed")
	t.Logf("   Model: %s", request.Model)
	t.Logf("   Response: %s", truncateString(response.Content, 100))
}

// =============================================================================
// OpenRouter Provider Integration Tests
// =============================================================================

func TestCloudProvider_OpenRouter_Generate(t *testing.T) {
	apiKey := getEnvOrSkip(t, "OPENROUTER_API_KEY", "OPENROUTER_API_KEY not set, skipping OpenRouter integration test")

	if strings.Contains(apiKey, "mock") {
		t.Skip("Skipping OpenRouter test - mock API key detected")  // SKIP-OK: #requires-upstream-key
	}

	config := ProviderConfigEntry{
		Type:     ProviderTypeOpenRouter,
		Endpoint: "https://openrouter.ai/api/v1",
		APIKey:   apiKey,
	}

	provider, err := NewOpenRouterProvider(config)
	require.NoError(t, err)
	defer provider.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	request := &LLMRequest{
		ID:    uuid.New(),
		Model: "openai/gpt-4o-mini", // OpenRouter model format
		Messages: []Message{
			{Role: "user", Content: "Respond with exactly: 'Hello from OpenRouter!'"},
		},
		MaxTokens:   50,
		Temperature: 0.0,
	}

	response, err := provider.Generate(ctx, request)
	require.NoError(t, err, "OpenRouter Generate should succeed")

	assert.NotEmpty(t, response.Content, "Response content should not be empty")

	t.Logf("✅ OpenRouter Generate test passed")
	t.Logf("   Model: %s", request.Model)
	t.Logf("   Response: %s", truncateString(response.Content, 100))
}

// =============================================================================
// Multi-Provider Integration Tests
// =============================================================================

func TestCloudProvider_MultiProvider_Comparison(t *testing.T) {
	providers := []struct {
		name    string
		envKey  string
		factory func(apiKey string) (Provider, error)
		model   string
	}{
		{
			name:   "OpenAI",
			envKey: "OPENAI_API_KEY",
			factory: func(apiKey string) (Provider, error) {
				return NewOpenAIProvider(ProviderConfigEntry{
					Type:     ProviderTypeOpenAI,
					Endpoint: "https://api.openai.com/v1",
					APIKey:   apiKey,
				})
			},
			model: "gpt-4o-mini",
		},
		{
			name:   "Anthropic",
			envKey: "ANTHROPIC_API_KEY",
			factory: func(apiKey string) (Provider, error) {
				return NewAnthropicProvider(ProviderConfigEntry{
					Type:     ProviderTypeAnthropic,
					Endpoint: "https://api.anthropic.com",
					APIKey:   apiKey,
				})
			},
			model: "claude-3-5-haiku-latest",
		},
		{
			name:   "Gemini",
			envKey: "GEMINI_API_KEY",
			factory: func(apiKey string) (Provider, error) {
				return NewGeminiProvider(ProviderConfigEntry{
					Type:     ProviderTypeGemini,
					Endpoint: "https://generativelanguage.googleapis.com/v1beta",
					APIKey:   apiKey,
				})
			},
			model: "gemini-2.0-flash",
		},
		{
			name:   "Groq",
			envKey: "GROQ_API_KEY",
			factory: func(apiKey string) (Provider, error) {
				return NewGroqProvider(ProviderConfigEntry{
					Type:     ProviderTypeGroq,
					Endpoint: "https://api.groq.com",
					APIKey:   apiKey,
				})
			},
			model: "llama-3.1-8b-instant",
		},
	}

	var testedProviders int

	for _, p := range providers {
		t.Run(p.name, func(t *testing.T) {
			apiKey := os.Getenv(p.envKey)
			if apiKey == "" || strings.Contains(apiKey, "mock") {
				t.Skipf("%s not configured for testing (SKIP-OK: #unmarked-skip-needs-ticket)", p.name)
				return
			}

			provider, err := p.factory(apiKey)
			require.NoError(t, err)
			defer provider.Close()

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			// Test simple math to compare responses
			request := &LLMRequest{
				ID:    uuid.New(),
				Model: p.model,
				Messages: []Message{
					{Role: "user", Content: "What is 2 + 2? Answer with just the number."},
				},
				MaxTokens:   10,
				Temperature: 0.0,
			}

			start := time.Now()
			response, err := provider.Generate(ctx, request)
			latency := time.Since(start)

			require.NoError(t, err, "%s Generate should succeed", p.name)
			assert.Contains(t, response.Content, "4", "%s should answer correctly", p.name)

			t.Logf("✅ %s: response=%q, latency=%v, tokens=%d",
				p.name, truncateString(response.Content, 50), latency, response.Usage.TotalTokens)

			testedProviders++
		})
	}

	if testedProviders == 0 {
		t.Skip("No cloud providers configured for multi-provider comparison test")  // SKIP-OK: #legacy-untriaged
	}

	t.Logf("✅ Multi-provider comparison complete: %d providers tested", testedProviders)
}

// =============================================================================
// Helper Functions
// =============================================================================

func truncateString(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}
