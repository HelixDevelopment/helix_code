package challenges

import (
	"context"
	"time"
)

// GetSupportedModels returns all supported models for a given provider (static list)
// Use GetSupportedModelsWithDiscovery for dynamic model discovery
func GetSupportedModels(provider LLMProviderType) []string {
	switch provider {
	case ProviderXAI:
		return []string{
			"grok-beta",
			"grok-vision-beta",
		}
	case ProviderOpenAI:
		return []string{
			"gpt-4-turbo-preview",
			"gpt-4-turbo",
			"gpt-4",
			"gpt-4-32k",
			"gpt-3.5-turbo",
			"gpt-3.5-turbo-16k",
		}
	case ProviderAnthropic:
		return []string{
			"claude-3-opus-20240229",
			"claude-3-sonnet-20240229",
			"claude-3-haiku-20240307",
			"claude-2.1",
			"claude-2.0",
			"claude-instant-1.2",
		}
	case ProviderGemini:
		return []string{
			"gemini-pro",
			"gemini-pro-vision",
			"gemini-ultra",
		}
	case ProviderGroq:
		return []string{
			"llama-3.1-405b-reasoning",
			"llama-3.1-70b-versatile",
			"llama-3.1-8b-instant",
			"mixtral-8x7b-32768",
			"gemma-7b-it",
		}
	case ProviderMistral:
		return []string{
			"mistral-large-latest",
			"mistral-medium-latest",
			"mistral-small-latest",
			"mistral-tiny",
		}
	case ProviderCohere:
		return []string{
			"command-r-plus",
			"command-r",
			"command",
			"command-light",
		}
	case ProviderDeepSeek:
		return []string{
			"deepseek-chat",
			"deepseek-coder",
			"deepseek-reasoner",
		}
	case ProviderHuggingFace:
		// Free stable coding models on Hugging Face
		return []string{
			"bigcode/starcoder",
			"Salesforce/codegen-2B-mono",
			"codellama/CodeLlama-7b-hf",
		}
	case ProviderOpenCode:
		// OpenCode free coding models
		return []string{
			"opencode-7b",
			"opencode-13b",
		}
	case ProviderOpenRouter:
		// Free/cheap coding models on OpenRouter
		return []string{
			"meta-llama/codellama-34b-instruct:free",
			"phind/phind-codellama-34b-v2",
			"deepseek/deepseek-coder-6.7b-instruct:free",
		}
	case ProviderOllama:
		// Common Ollama models - user may have others installed
		return []string{
			"llama2",
			"llama2:13b",
			"llama2:70b",
			"codellama",
			"codellama:13b",
			"mistral",
			"mixtral",
			"qwen",
			"deepseek-coder",
			"phi",
		}
	case ProviderAzure:
		// Azure uses deployment names - these are examples
		return []string{
			"gpt-4",
			"gpt-35-turbo",
		}
	default:
		return []string{}
	}
}

// GetDefaultModel returns the default model for a provider
func GetDefaultModel(provider LLMProviderType) string {
	models := GetSupportedModels(provider)
	if len(models) > 0 {
		return models[0]
	}
	return ""
}

// GetProviderAPIEndpoint returns the API endpoint for a provider
func GetProviderAPIEndpoint(provider LLMProviderType) string {
	switch provider {
	case ProviderXAI:
		return "https://api.x.ai/v1"
	case ProviderOpenAI:
		return "https://api.openai.com/v1"
	case ProviderAnthropic:
		return "https://api.anthropic.com/v1"
	case ProviderGemini:
		return "https://generativelanguage.googleapis.com/v1"
	case ProviderGroq:
		return "https://api.groq.com/openai/v1"
	case ProviderMistral:
		return "https://api.mistral.ai/v1"
	case ProviderCohere:
		return "https://api.cohere.ai/v1"
	case ProviderDeepSeek:
		return "https://api.deepseek.com/v1"
	case ProviderHuggingFace:
		return "https://api-inference.huggingface.co/models"
	case ProviderOpenCode:
		return "https://api.opencode.com/v1"
	case ProviderOpenRouter:
		return "https://openrouter.ai/api/v1"
	case ProviderOllama:
		return "http://localhost:11434"
	default:
		return ""
	}
}

// GetProviderRateLimits returns rate limit info for a provider
func GetProviderRateLimits(provider LLMProviderType) map[string]int {
	switch provider {
	case ProviderXAI:
		return map[string]int{
			"requests_per_minute": 60,
			"tokens_per_minute":   100000,
		}
	case ProviderOpenAI:
		return map[string]int{
			"requests_per_minute": 3500,
			"tokens_per_minute":   90000,
		}
	case ProviderAnthropic:
		return map[string]int{
			"requests_per_minute": 1000,
			"tokens_per_minute":   100000,
		}
	case ProviderGroq:
		return map[string]int{
			"requests_per_minute": 30,
			"tokens_per_minute":   14400,
		}
	case ProviderDeepSeek:
		return map[string]int{
			"requests_per_minute": 60,
			"tokens_per_minute":   100000,
		}
	case ProviderHuggingFace:
		return map[string]int{
			"requests_per_minute": 1000, // Free tier is generous
			"tokens_per_minute":   100000,
		}
	case ProviderOpenCode:
		return map[string]int{
			"requests_per_minute": 100,
			"tokens_per_minute":   100000,
		}
	case ProviderOpenRouter:
		return map[string]int{
			"requests_per_minute": 200, // Depends on model
			"tokens_per_minute":   100000,
		}
	default:
		return map[string]int{
			"requests_per_minute": 60,
			"tokens_per_minute":   100000,
		}
	}
}

// GetSupportedModelsWithDiscovery attempts to fetch models dynamically from provider APIs
// Falls back to static list if discovery fails or API keys are not available
func GetSupportedModelsWithDiscovery(provider LLMProviderType, apiKeys *APIKeys) []string {
	// Try dynamic discovery first for supported providers
	if supportsDiscovery(provider) && apiKeys != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		discovery := NewModelDiscovery(apiKeys)
		models, err := discovery.DiscoverModels(ctx, provider)
		if err == nil && len(models) > 0 {
			return GetModelIDs(models)
		}
		// If discovery fails, fall back to static list
	}

	// Fall back to static list
	return GetSupportedModels(provider)
}

// supportsDiscovery returns true if the provider supports dynamic model discovery
func supportsDiscovery(provider LLMProviderType) bool {
	supportedProviders := map[LLMProviderType]bool{
		ProviderOpenAI:   true,
		ProviderXAI:      true,
		ProviderDeepSeek: true,
		ProviderGroq:     true,
		ProviderOllama:   true,
	}
	return supportedProviders[provider]
}

// GetModelDetails fetches detailed information about available models
func GetModelDetails(provider LLMProviderType, apiKeys *APIKeys) ([]ModelInfo, error) {
	if !supportsDiscovery(provider) {
		return nil, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	discovery := NewModelDiscovery(apiKeys)
	return discovery.DiscoverModels(ctx, provider)
}
