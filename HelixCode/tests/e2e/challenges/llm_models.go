package challenges

// GetSupportedModels returns all supported models for a given provider
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
	default:
		return map[string]int{
			"requests_per_minute": 60,
			"tokens_per_minute":   100000,
		}
	}
}
