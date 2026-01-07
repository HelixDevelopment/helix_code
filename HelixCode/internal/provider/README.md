# Provider Package

The provider package defines the core `Provider` interface used across HelixCode for LLM integration. It establishes a unified contract that all LLM providers must implement.

## Overview

This package provides:
- The `Provider` interface for LLM operations
- `ProviderType` constants for all supported providers
- Common methods for generation, streaming, health checks, and Cognee integration

## Provider Types

### Cloud Providers

| Type | Constant | Description |
|------|----------|-------------|
| OpenAI | `ProviderTypeOpenAI` | OpenAI GPT models |
| Anthropic | `ProviderTypeAnthropic` | Claude models |
| Gemini | `ProviderTypeGemini` | Google Gemini |
| VertexAI | `ProviderTypeVertexAI` | Google Cloud Vertex AI |
| Azure | `ProviderTypeAzure` | Azure OpenAI |
| Bedrock | `ProviderTypeBedrock` | AWS Bedrock |
| Groq | `ProviderTypeGroq` | Groq inference |
| Qwen | `ProviderTypeQwen` | Alibaba Qwen |
| Copilot | `ProviderTypeCopilot` | GitHub Copilot |
| OpenRouter | `ProviderTypeOpenRouter` | OpenRouter multi-provider |
| xAI | `ProviderTypeXAI` | xAI Grok |

### Local Providers

| Type | Constant | Description |
|------|----------|-------------|
| Ollama | `ProviderTypeOllama` | Ollama local models |
| Local | `ProviderTypeLocal` | Generic local provider |
| LlamaCpp | `ProviderTypeLlamaCpp` | Llama.cpp inference |
| VLLM | `ProviderTypeVLLM` | vLLM server |
| LocalAI | `ProviderTypeLocalAI` | LocalAI server |

## Provider Interface

All providers must implement:

```go
type Provider interface {
    // Type and identification
    GetType() ProviderType
    GetName() string

    // Model information
    GetModels() []llm.ModelInfo
    GetCapabilities() []llm.ModelCapability
    GetModelName() string
    GetModelInfo() *llm.ModelInfo

    // Generation
    Generate(ctx context.Context, request *llm.LLMRequest) (*llm.LLMResponse, error)
    GenerateStream(ctx context.Context, request *llm.LLMRequest, ch chan<- llm.LLMResponse) error

    // Health and availability
    IsAvailable(ctx context.Context) bool
    GetHealth(ctx context.Context) (*llm.ProviderHealth, error)

    // Lifecycle
    Close() error

    // Cognee integration
    SupportsCognee() bool
    InitializeCognee(config interface{}, options interface{}) error

    // Hardware profiling
    GetHardwareProfile() *hardware.HardwareProfile
}
```

## Usage

### Checking Provider Type

```go
func handleProvider(p provider.Provider) {
    switch p.GetType() {
    case provider.ProviderTypeOpenAI:
        // Handle OpenAI-specific features
    case provider.ProviderTypeOllama:
        // Handle local Ollama
    default:
        // Generic handling
    }
}
```

### Converting Type to String

```go
providerType := provider.ProviderTypeAnthropic
fmt.Println(providerType.String()) // Output: "anthropic"
```

### Implementing a Provider

```go
type MyProvider struct {
    // Provider-specific fields
}

func (p *MyProvider) GetType() provider.ProviderType {
    return provider.ProviderTypeLocal
}

func (p *MyProvider) GetName() string {
    return "MyCustomProvider"
}

func (p *MyProvider) Generate(ctx context.Context, req *llm.LLMRequest) (*llm.LLMResponse, error) {
    // Implementation
}

// ... implement remaining interface methods
```

## Integration Points

### LLM Package

The provider interface integrates with `internal/llm`:
- `llm.LLMRequest` - Request structure
- `llm.LLMResponse` - Response structure
- `llm.ModelInfo` - Model metadata
- `llm.ModelCapability` - Model capabilities
- `llm.ProviderHealth` - Health status

### Hardware Package

For local providers:
- `hardware.HardwareProfile` - CPU/GPU/memory info
- Used for model selection and optimization

### Cognee Integration

Providers supporting Cognee implement:
- `SupportsCognee()` - Returns true if Cognee-compatible
- `InitializeCognee()` - Configure Cognee integration

## Best Practices

1. **Check availability before use**: Call `IsAvailable()` before generation
2. **Handle streaming errors**: Monitor the channel for errors during streaming
3. **Close providers properly**: Always call `Close()` when done
4. **Check capabilities**: Use `GetCapabilities()` to verify feature support
5. **Use context for cancellation**: Pass context for timeout/cancellation
