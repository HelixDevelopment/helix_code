package challenges

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v2"
)

// APIKeys holds all API key configurations
type APIKeys struct {
	OpenAI      *OpenAIConfig      `yaml:"openai,omitempty"`
	Anthropic   *AnthropicConfig   `yaml:"anthropic,omitempty"`
	XAI         *XAIConfig         `yaml:"xai,omitempty"`
	Gemini      *GeminiConfig      `yaml:"gemini,omitempty"`
	Groq        *GroqConfig        `yaml:"groq,omitempty"`
	Mistral     *MistralConfig     `yaml:"mistral,omitempty"`
	Cohere      *CohereConfig      `yaml:"cohere,omitempty"`
	DeepSeek    *DeepSeekConfig    `yaml:"deepseek,omitempty"`
	HuggingFace *HuggingFaceConfig `yaml:"huggingface,omitempty"`
	OpenCode    *OpenCodeConfig    `yaml:"opencode,omitempty"`
	OpenRouter  *OpenRouterConfig  `yaml:"openrouter,omitempty"`
	Vertex      *VertexConfig      `yaml:"vertex,omitempty"`
	Azure       *AzureConfig       `yaml:"azure,omitempty"`
	Bedrock     *BedrockConfig     `yaml:"bedrock,omitempty"`
}

// OpenAIConfig holds OpenAI API configuration
type OpenAIConfig struct {
	APIKey       string `yaml:"api_key"`
	Organization string `yaml:"organization,omitempty"`
}

// AnthropicConfig holds Anthropic Claude API configuration
type AnthropicConfig struct {
	APIKey string `yaml:"api_key"`
}

// XAIConfig holds xAI (Grok) API configuration
type XAIConfig struct {
	APIKey string `yaml:"api_key"`
}

// GeminiConfig holds Google Gemini API configuration
type GeminiConfig struct {
	APIKey string `yaml:"api_key"`
}

// GroqConfig holds Groq API configuration
type GroqConfig struct {
	APIKey string `yaml:"api_key"`
}

// MistralConfig holds Mistral AI API configuration
type MistralConfig struct {
	APIKey string `yaml:"api_key"`
}

// CohereConfig holds Cohere API configuration
type CohereConfig struct {
	APIKey string `yaml:"api_key"`
}

// DeepSeekConfig holds DeepSeek API configuration
type DeepSeekConfig struct {
	APIKey string `yaml:"api_key"`
}

// HuggingFaceConfig holds Hugging Face API configuration
type HuggingFaceConfig struct {
	APIKey string `yaml:"api_key"`
}

// OpenCodeConfig holds OpenCode API configuration
type OpenCodeConfig struct {
	APIKey string `yaml:"api_key"`
}

// OpenRouterConfig holds OpenRouter API configuration
type OpenRouterConfig struct {
	APIKey string `yaml:"api_key"`
}

// VertexConfig holds Google Vertex AI configuration
type VertexConfig struct {
	ProjectID       string `yaml:"project_id"`
	Location        string `yaml:"location"`
	CredentialsFile string `yaml:"credentials_file"`
}

// AzureConfig holds Azure OpenAI configuration
type AzureConfig struct {
	APIKey         string `yaml:"api_key"`
	Endpoint       string `yaml:"endpoint"`
	DeploymentName string `yaml:"deployment_name"`
}

// BedrockConfig holds AWS Bedrock configuration
type BedrockConfig struct {
	Region          string `yaml:"region"`
	AccessKeyID     string `yaml:"access_key_id"`
	SecretAccessKey string `yaml:"secret_access_key"`
}

// LoadAPIKeys loads API keys from file
func LoadAPIKeys(configPath string) (*APIKeys, error) {
	// Default path if not specified
	if configPath == "" {
		configPath = "api-keys.yaml"
	}

	// Read the file
	data, err := os.ReadFile(configPath)
	if err != nil {
		// If file doesn't exist, return empty config (will use local providers only)
		if os.IsNotExist(err) {
			return &APIKeys{}, nil
		}
		return nil, fmt.Errorf("failed to read API keys file: %w", err)
	}

	// Parse YAML
	var keys APIKeys
	if err := yaml.Unmarshal(data, &keys); err != nil {
		return nil, fmt.Errorf("failed to parse API keys: %w", err)
	}

	return &keys, nil
}

// GetAPIKey returns the API key for a specific provider
func (k *APIKeys) GetAPIKey(provider LLMProviderType) (string, error) {
	switch provider {
	case ProviderOpenAI:
		if k.OpenAI != nil {
			return k.OpenAI.APIKey, nil
		}
	case ProviderAnthropic:
		if k.Anthropic != nil {
			return k.Anthropic.APIKey, nil
		}
	case ProviderXAI:
		if k.XAI != nil {
			return k.XAI.APIKey, nil
		}
	case ProviderGemini:
		if k.Gemini != nil {
			return k.Gemini.APIKey, nil
		}
	case ProviderGroq:
		if k.Groq != nil {
			return k.Groq.APIKey, nil
		}
	case ProviderMistral:
		if k.Mistral != nil {
			return k.Mistral.APIKey, nil
		}
	case ProviderCohere:
		if k.Cohere != nil {
			return k.Cohere.APIKey, nil
		}
	case ProviderDeepSeek:
		if k.DeepSeek != nil {
			return k.DeepSeek.APIKey, nil
		}
	case ProviderHuggingFace:
		if k.HuggingFace != nil {
			return k.HuggingFace.APIKey, nil
		}
	case ProviderOpenCode:
		if k.OpenCode != nil {
			return k.OpenCode.APIKey, nil
		}
	case ProviderOpenRouter:
		if k.OpenRouter != nil {
			return k.OpenRouter.APIKey, nil
		}
	case ProviderAzure:
		if k.Azure != nil {
			return k.Azure.APIKey, nil
		}
	case ProviderBedrock:
		if k.Bedrock != nil {
			return k.Bedrock.AccessKeyID, nil // Primary credential
		}
	}

	return "", fmt.Errorf("no API key configured for provider: %s", provider)
}

// IsCloudProvider returns true if the provider requires API keys
func IsCloudProvider(provider LLMProviderType) bool {
	cloudProviders := map[LLMProviderType]bool{
		ProviderOpenAI:      true,
		ProviderAnthropic:   true,
		ProviderXAI:         true,
		ProviderGemini:      true,
		ProviderGroq:        true,
		ProviderMistral:     true,
		ProviderCohere:      true,
		ProviderDeepSeek:    true,
		ProviderHuggingFace: true,
		ProviderOpenCode:    true,
		ProviderAzure:       true,
		ProviderBedrock:     true,
		ProviderVertexAI:    true,
		ProviderOpenRouter:  true,
	}
	return cloudProviders[provider]
}

// MaskAPIKey masks an API key for safe logging
func MaskAPIKey(apiKey string) string {
	if len(apiKey) <= 8 {
		return "***"
	}
	// Show first 4 and last 4 characters
	return apiKey[:4] + "..." + apiKey[len(apiKey)-4:]
}

// SanitizeForLogging removes API keys from text
func SanitizeForLogging(text string, apiKeys *APIKeys) string {
	sanitized := text

	// Replace all API keys with masked versions
	if apiKeys.OpenAI != nil && apiKeys.OpenAI.APIKey != "" {
		sanitized = strings.ReplaceAll(sanitized, apiKeys.OpenAI.APIKey, MaskAPIKey(apiKeys.OpenAI.APIKey))
	}
	if apiKeys.Anthropic != nil && apiKeys.Anthropic.APIKey != "" {
		sanitized = strings.ReplaceAll(sanitized, apiKeys.Anthropic.APIKey, MaskAPIKey(apiKeys.Anthropic.APIKey))
	}
	if apiKeys.XAI != nil && apiKeys.XAI.APIKey != "" {
		sanitized = strings.ReplaceAll(sanitized, apiKeys.XAI.APIKey, MaskAPIKey(apiKeys.XAI.APIKey))
	}
	if apiKeys.Gemini != nil && apiKeys.Gemini.APIKey != "" {
		sanitized = strings.ReplaceAll(sanitized, apiKeys.Gemini.APIKey, MaskAPIKey(apiKeys.Gemini.APIKey))
	}
	if apiKeys.Groq != nil && apiKeys.Groq.APIKey != "" {
		sanitized = strings.ReplaceAll(sanitized, apiKeys.Groq.APIKey, MaskAPIKey(apiKeys.Groq.APIKey))
	}
	if apiKeys.Mistral != nil && apiKeys.Mistral.APIKey != "" {
		sanitized = strings.ReplaceAll(sanitized, apiKeys.Mistral.APIKey, MaskAPIKey(apiKeys.Mistral.APIKey))
	}
	if apiKeys.Cohere != nil && apiKeys.Cohere.APIKey != "" {
		sanitized = strings.ReplaceAll(sanitized, apiKeys.Cohere.APIKey, MaskAPIKey(apiKeys.Cohere.APIKey))
	}
	if apiKeys.DeepSeek != nil && apiKeys.DeepSeek.APIKey != "" {
		sanitized = strings.ReplaceAll(sanitized, apiKeys.DeepSeek.APIKey, MaskAPIKey(apiKeys.DeepSeek.APIKey))
	}
	if apiKeys.HuggingFace != nil && apiKeys.HuggingFace.APIKey != "" {
		sanitized = strings.ReplaceAll(sanitized, apiKeys.HuggingFace.APIKey, MaskAPIKey(apiKeys.HuggingFace.APIKey))
	}
	if apiKeys.OpenCode != nil && apiKeys.OpenCode.APIKey != "" {
		sanitized = strings.ReplaceAll(sanitized, apiKeys.OpenCode.APIKey, MaskAPIKey(apiKeys.OpenCode.APIKey))
	}
	if apiKeys.OpenRouter != nil && apiKeys.OpenRouter.APIKey != "" {
		sanitized = strings.ReplaceAll(sanitized, apiKeys.OpenRouter.APIKey, MaskAPIKey(apiKeys.OpenRouter.APIKey))
	}
	if apiKeys.Azure != nil && apiKeys.Azure.APIKey != "" {
		sanitized = strings.ReplaceAll(sanitized, apiKeys.Azure.APIKey, MaskAPIKey(apiKeys.Azure.APIKey))
	}
	if apiKeys.Bedrock != nil {
		if apiKeys.Bedrock.AccessKeyID != "" {
			sanitized = strings.ReplaceAll(sanitized, apiKeys.Bedrock.AccessKeyID, MaskAPIKey(apiKeys.Bedrock.AccessKeyID))
		}
		if apiKeys.Bedrock.SecretAccessKey != "" {
			sanitized = strings.ReplaceAll(sanitized, apiKeys.Bedrock.SecretAccessKey, MaskAPIKey(apiKeys.Bedrock.SecretAccessKey))
		}
	}

	return sanitized
}
