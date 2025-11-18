package challenges

import (
	"testing"
)

func TestGetSupportedModels(t *testing.T) {
	tests := []struct {
		provider      LLMProviderType
		expectedCount int
		expectedFirst string
	}{
		{
			provider:      ProviderXAI,
			expectedCount: 2,
			expectedFirst: "grok-beta",
		},
		{
			provider:      ProviderOpenAI,
			expectedCount: 6,
			expectedFirst: "gpt-4-turbo-preview",
		},
		{
			provider:      ProviderAnthropic,
			expectedCount: 6,
			expectedFirst: "claude-3-opus-20240229",
		},
		{
			provider:      ProviderGroq,
			expectedCount: 5,
			expectedFirst: "llama-3.1-405b-reasoning",
		},
		{
			provider:      ProviderGemini,
			expectedCount: 3,
			expectedFirst: "gemini-pro",
		},
		{
			provider:      ProviderMistral,
			expectedCount: 4,
			expectedFirst: "mistral-large-latest",
		},
		{
			provider:      ProviderCohere,
			expectedCount: 4,
			expectedFirst: "command-r-plus",
		},
		{
			provider:      ProviderDeepSeek,
			expectedCount: 3,
			expectedFirst: "deepseek-chat",
		},
		{
			provider:      ProviderOllama,
			expectedCount: 10,
			expectedFirst: "llama2",
		},
		{
			provider:      ProviderAzure,
			expectedCount: 2,
			expectedFirst: "gpt-4",
		},
	}

	for _, tt := range tests {
		t.Run(string(tt.provider), func(t *testing.T) {
			models := GetSupportedModels(tt.provider)

			if len(models) != tt.expectedCount {
				t.Errorf("Expected %d models for %s, got %d",
					tt.expectedCount, tt.provider, len(models))
			}

			if len(models) > 0 && models[0] != tt.expectedFirst {
				t.Errorf("Expected first model to be '%s', got '%s'",
					tt.expectedFirst, models[0])
			}

			// Verify no empty strings
			for i, model := range models {
				if model == "" {
					t.Errorf("Model at index %d is empty string", i)
				}
			}
		})
	}
}

func TestGetSupportedModels_UnsupportedProvider(t *testing.T) {
	models := GetSupportedModels(LLMProviderType("unsupported"))
	if len(models) != 0 {
		t.Errorf("Expected 0 models for unsupported provider, got %d", len(models))
	}
}

func TestGetDefaultModel(t *testing.T) {
	tests := []struct {
		provider LLMProviderType
		expected string
	}{
		{ProviderXAI, "grok-beta"},
		{ProviderOpenAI, "gpt-4-turbo-preview"},
		{ProviderAnthropic, "claude-3-opus-20240229"},
		{ProviderGroq, "llama-3.1-405b-reasoning"},
		{ProviderGemini, "gemini-pro"},
		{ProviderMistral, "mistral-large-latest"},
		{ProviderCohere, "command-r-plus"},
		{ProviderDeepSeek, "deepseek-chat"},
		{ProviderOllama, "llama2"},
		{ProviderAzure, "gpt-4"},
	}

	for _, tt := range tests {
		t.Run(string(tt.provider), func(t *testing.T) {
			model := GetDefaultModel(tt.provider)
			if model != tt.expected {
				t.Errorf("Expected default model '%s' for %s, got '%s'",
					tt.expected, tt.provider, model)
			}
		})
	}
}

func TestGetDefaultModel_UnsupportedProvider(t *testing.T) {
	model := GetDefaultModel(LLMProviderType("unsupported"))
	if model != "" {
		t.Errorf("Expected empty string for unsupported provider, got '%s'", model)
	}
}

func TestGetProviderAPIEndpoint(t *testing.T) {
	tests := []struct {
		provider LLMProviderType
		expected string
	}{
		{ProviderXAI, "https://api.x.ai/v1"},
		{ProviderOpenAI, "https://api.openai.com/v1"},
		{ProviderAnthropic, "https://api.anthropic.com/v1"},
		{ProviderGemini, "https://generativelanguage.googleapis.com/v1"},
		{ProviderGroq, "https://api.groq.com/openai/v1"},
		{ProviderMistral, "https://api.mistral.ai/v1"},
		{ProviderCohere, "https://api.cohere.ai/v1"},
		{ProviderDeepSeek, "https://api.deepseek.com/v1"},
		{ProviderOllama, "http://localhost:11434"},
	}

	for _, tt := range tests {
		t.Run(string(tt.provider), func(t *testing.T) {
			endpoint := GetProviderAPIEndpoint(tt.provider)
			if endpoint != tt.expected {
				t.Errorf("Expected endpoint '%s' for %s, got '%s'",
					tt.expected, tt.provider, endpoint)
			}
		})
	}
}

func TestGetProviderAPIEndpoint_UnsupportedProvider(t *testing.T) {
	endpoint := GetProviderAPIEndpoint(LLMProviderType("unsupported"))
	if endpoint != "" {
		t.Errorf("Expected empty string for unsupported provider, got '%s'", endpoint)
	}
}

func TestGetProviderRateLimits(t *testing.T) {
	tests := []struct {
		provider        LLMProviderType
		expectedReqMin  int
		expectedTokMin  int
	}{
		{ProviderXAI, 60, 100000},
		{ProviderOpenAI, 3500, 90000},
		{ProviderAnthropic, 1000, 100000},
		{ProviderGroq, 30, 14400},
		{ProviderDeepSeek, 60, 100000},
	}

	for _, tt := range tests {
		t.Run(string(tt.provider), func(t *testing.T) {
			limits := GetProviderRateLimits(tt.provider)

			reqPerMin, hasReq := limits["requests_per_minute"]
			if !hasReq {
				t.Error("Expected 'requests_per_minute' in rate limits")
			} else if reqPerMin != tt.expectedReqMin {
				t.Errorf("Expected %d requests/min, got %d",
					tt.expectedReqMin, reqPerMin)
			}

			tokPerMin, hasTok := limits["tokens_per_minute"]
			if !hasTok {
				t.Error("Expected 'tokens_per_minute' in rate limits")
			} else if tokPerMin != tt.expectedTokMin {
				t.Errorf("Expected %d tokens/min, got %d",
					tt.expectedTokMin, tokPerMin)
			}
		})
	}
}

func TestGetProviderRateLimits_DefaultLimits(t *testing.T) {
	// Providers without specific limits should get defaults
	providers := []LLMProviderType{
		ProviderGemini,
		ProviderMistral,
		ProviderCohere,
		ProviderOllama,
	}

	for _, provider := range providers {
		t.Run(string(provider), func(t *testing.T) {
			limits := GetProviderRateLimits(provider)

			// Should have default limits
			if len(limits) == 0 {
				t.Error("Expected default rate limits, got empty map")
			}

			reqPerMin, hasReq := limits["requests_per_minute"]
			if !hasReq {
				t.Error("Expected 'requests_per_minute' in default limits")
			} else if reqPerMin != 60 {
				t.Errorf("Expected default 60 req/min, got %d", reqPerMin)
			}

			tokPerMin, hasTok := limits["tokens_per_minute"]
			if !hasTok {
				t.Error("Expected 'tokens_per_minute' in default limits")
			} else if tokPerMin != 100000 {
				t.Errorf("Expected default 100K tok/min, got %d", tokPerMin)
			}
		})
	}
}

func TestAllProvidersHaveModels(t *testing.T) {
	// Verify all defined providers have at least one model
	providers := []LLMProviderType{
		ProviderXAI,
		ProviderOpenAI,
		ProviderAnthropic,
		ProviderGemini,
		ProviderGroq,
		ProviderMistral,
		ProviderCohere,
		ProviderDeepSeek,
		ProviderOllama,
		ProviderAzure,
	}

	for _, provider := range providers {
		t.Run(string(provider), func(t *testing.T) {
			models := GetSupportedModels(provider)
			if len(models) == 0 {
				t.Errorf("Provider %s has no models defined", provider)
			}
		})
	}
}

func TestAllProvidersHaveDefaultModel(t *testing.T) {
	// Verify all providers with models have a default model
	providers := []LLMProviderType{
		ProviderXAI,
		ProviderOpenAI,
		ProviderAnthropic,
		ProviderGemini,
		ProviderGroq,
		ProviderMistral,
		ProviderCohere,
		ProviderDeepSeek,
		ProviderOllama,
		ProviderAzure,
	}

	for _, provider := range providers {
		t.Run(string(provider), func(t *testing.T) {
			defaultModel := GetDefaultModel(provider)
			if defaultModel == "" {
				t.Errorf("Provider %s has no default model", provider)
			}

			// Verify default model is in supported models
			models := GetSupportedModels(provider)
			found := false
			for _, model := range models {
				if model == defaultModel {
					found = true
					break
				}
			}

			if !found {
				t.Errorf("Default model '%s' not found in supported models for %s",
					defaultModel, provider)
			}
		})
	}
}

func TestCloudProvidersHaveEndpoints(t *testing.T) {
	// All cloud providers should have API endpoints
	cloudProviders := []LLMProviderType{
		ProviderXAI,
		ProviderOpenAI,
		ProviderAnthropic,
		ProviderGemini,
		ProviderGroq,
		ProviderMistral,
		ProviderCohere,
		ProviderDeepSeek,
	}

	for _, provider := range cloudProviders {
		t.Run(string(provider), func(t *testing.T) {
			endpoint := GetProviderAPIEndpoint(provider)
			if endpoint == "" {
				t.Errorf("Cloud provider %s has no API endpoint", provider)
			}

			// Should be HTTPS for cloud providers
			if provider != ProviderOllama && len(endpoint) > 0 {
				if endpoint[:5] != "https" {
					t.Errorf("Expected HTTPS endpoint for %s, got '%s'",
						provider, endpoint)
				}
			}
		})
	}
}

func TestXAIModels_Specific(t *testing.T) {
	// Test xAI-specific models in detail
	models := GetSupportedModels(ProviderXAI)

	expectedModels := []string{"grok-beta", "grok-vision-beta"}

	if len(models) != len(expectedModels) {
		t.Errorf("Expected %d xAI models, got %d", len(expectedModels), len(models))
	}

	for i, expected := range expectedModels {
		if i >= len(models) {
			t.Errorf("Missing expected model '%s'", expected)
			continue
		}
		if models[i] != expected {
			t.Errorf("Expected model '%s' at index %d, got '%s'",
				expected, i, models[i])
		}
	}
}

func TestOllamaModels_Specific(t *testing.T) {
	// Test Ollama-specific models
	models := GetSupportedModels(ProviderOllama)

	// Should include common models
	expectedModels := []string{
		"llama2",
		"llama2:13b",
		"llama2:70b",
		"codellama",
		"mistral",
		"mixtral",
	}

	for _, expected := range expectedModels {
		found := false
		for _, model := range models {
			if model == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected Ollama model '%s' not found", expected)
		}
	}
}

func TestAnthropicModels_Specific(t *testing.T) {
	// Test Anthropic Claude models
	models := GetSupportedModels(ProviderAnthropic)

	// Should include Claude 3 family
	expectedModels := []string{
		"claude-3-opus-20240229",
		"claude-3-sonnet-20240229",
		"claude-3-haiku-20240307",
	}

	for _, expected := range expectedModels {
		found := false
		for _, model := range models {
			if model == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected Anthropic model '%s' not found", expected)
		}
	}
}

func TestDeepSeekModels_Specific(t *testing.T) {
	// Test DeepSeek-specific models
	models := GetSupportedModels(ProviderDeepSeek)

	expectedModels := []string{"deepseek-chat", "deepseek-coder", "deepseek-reasoner"}

	if len(models) != len(expectedModels) {
		t.Errorf("Expected %d DeepSeek models, got %d", len(expectedModels), len(models))
	}

	for i, expected := range expectedModels {
		if i >= len(models) {
			t.Errorf("Missing expected model '%s'", expected)
			continue
		}
		if models[i] != expected {
			t.Errorf("Expected model '%s' at index %d, got '%s'",
				expected, i, models[i])
		}
	}
}
