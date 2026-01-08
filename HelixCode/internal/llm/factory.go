package llm

import (
	"fmt"
	"time"
)

// NewProvider creates a new provider instance based on the configuration
func NewProvider(config ProviderConfigEntry) (Provider, error) {
	switch config.Type {
	case ProviderTypeOpenAI:
		return NewOpenAIProvider(config)
	case ProviderTypeAnthropic:
		return NewAnthropicProvider(config)
	case ProviderTypeGemini:
		return NewGeminiProvider(config)
	case ProviderTypeOllama:
		ollamaConfig := OllamaConfig{
			BaseURL:       config.Endpoint,
			DefaultModel:  "llama2", // Default
			Timeout:       120 * time.Second,
			StreamEnabled: true,
		}
		if len(config.Models) > 0 {
			ollamaConfig.DefaultModel = config.Models[0]
		}
		// Map parameters if available
		if val, ok := config.Parameters["timeout"].(float64); ok {
			ollamaConfig.Timeout = time.Duration(val) * time.Second
		}
		return NewOllamaProvider(ollamaConfig)
	case ProviderTypeLlamaCpp:
		llamaConfig := LlamaConfig{
			ModelPath:     "", // Needs to be set from config
			ContextSize:   4096,
			ServerTimeout: 120 * time.Second,
		}
		if len(config.Models) > 0 {
			llamaConfig.ModelPath = config.Models[0]
		}
		// Map parameters
		if val, ok := config.Parameters["context_size"].(float64); ok {
			llamaConfig.ContextSize = int(val)
		}
		if val, ok := config.Parameters["gpu_enabled"].(bool); ok {
			llamaConfig.GPUEnabled = val
		}
		return NewLlamaCPPProvider(llamaConfig)
	case ProviderTypeQwen:
		return NewQwenProvider(config)
	case ProviderTypeXAI:
		return NewXAIProvider(config)
	case ProviderTypeOpenRouter:
		return NewOpenRouterProvider(config)
	case ProviderTypeCopilot:
		return NewCopilotProvider(config)
	case ProviderTypeAzure:
		return NewAzureProvider(config)
	case ProviderTypeBedrock:
		return NewBedrockProvider(config)
	case ProviderTypeVertexAI:
		return NewVertexAIProvider(config)
	case ProviderTypeGroq:
		return NewGroqProvider(config)
	case ProviderTypeVLLM:
		return NewVLLMProvider(config)
	case ProviderTypeLocalAI:
		return NewLocalAIProvider(config)
	case ProviderTypeFastChat:
		return NewFastChatProvider(config)
	case ProviderTypeTextGen:
		return NewTextGenProvider(config)
	case ProviderTypeLMStudio:
		return NewLMStudioProvider(config)
	case ProviderTypeJan:
		return NewJanProvider(config)
	case ProviderTypeGPT4All:
		return NewGPT4AllProvider(config)
	case ProviderTypeTabbyAPI:
		return NewTabbyAPIProvider(config)
	case ProviderTypeMLX:
		return NewMLXProvider(config)
	case ProviderTypeMistralRS:
		return NewMistralRSProvider(config)
	case ProviderTypeKoboldAI:
		koboldConfig := KoboldAIConfig{
			BaseURL: config.Endpoint,
			APIKey:  config.APIKey,
			Timeout: 120 * time.Second,
		}
		if len(config.Models) > 0 {
			koboldConfig.DefaultModel = config.Models[0]
		}
		if val, ok := config.Parameters["timeout"].(float64); ok {
			koboldConfig.Timeout = time.Duration(val) * time.Second
		}
		return NewKoboldAIProvider(koboldConfig)
	default:
		return nil, fmt.Errorf("unsupported provider type: %s", config.Type)
	}
}

// InitializeModelManager initializes a ModelManager with providers from configuration
func InitializeModelManager(configs []ProviderConfigEntry) (*ModelManager, error) {
	manager := NewModelManager()

	for _, config := range configs {
		if !config.Enabled {
			continue
		}

		provider, err := NewProvider(config)
		if err != nil {
			return nil, fmt.Errorf("failed to create provider %s: %w", config.Type, err)
		}

		if err := manager.RegisterProvider(provider); err != nil {
			return nil, fmt.Errorf("failed to register provider %s: %w", config.Type, err)
		}
	}

	return manager, nil
}
