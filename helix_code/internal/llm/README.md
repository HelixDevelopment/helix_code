# LLM Package

The `llm` package provides multi-provider Large Language Model (LLM) integration for the HelixCode platform.

## Overview

This package handles:
- Unified interface for multiple LLM providers
- Local inference servers (Llama.cpp, Ollama, vLLM, etc.)
- Cloud APIs (OpenAI, Anthropic, Gemini, etc.)
- Provider selection strategies
- Streaming responses
- Token counting and usage tracking

## Supported Providers

### Local Providers
- Llama.cpp
- Ollama
- vLLM
- LocalAI
- LM Studio
- Jan
- GPT4All
- KoboldAI
- TabbyAPI
- MLX
- MistralRS

### Cloud Providers
- OpenAI
- Anthropic (Claude)
- Google Gemini
- Vertex AI
- Qwen
- xAI (Grok)
- Groq
- OpenRouter
- GitHub Copilot
- AWS Bedrock
- Azure OpenAI

## Key Types

### Provider Interface

```go
type Provider interface {
    GetType() ProviderType
    GetName() string
    GetModels() []ModelInfo
    GetCapabilities() []ModelCapability
    Generate(ctx context.Context, request *LLMRequest) (*LLMResponse, error)
    GenerateStream(ctx context.Context, request *LLMRequest, ch chan<- LLMResponse) error
    IsAvailable(ctx context.Context) bool
    GetHealth(ctx context.Context) (*ProviderHealth, error)
    Close() error
}
```

### LLMRequest

```go
type LLMRequest struct {
    ID               uuid.UUID              `json:"id"`
    Model            string                 `json:"model"`
    Messages         []Message              `json:"messages"`
    MaxTokens        int                    `json:"max_tokens"`
    Temperature      float64                `json:"temperature"`
    TopP             float64                `json:"top_p"`
    Stream           bool                   `json:"stream"`
    Tools            []Tool                 `json:"tools"`
    ToolChoice       interface{}            `json:"tool_choice"`
    Stop             []string               `json:"stop"`
    ThinkingBudget   int                    `json:"thinking_budget"`
    CacheConfig      *CacheConfig           `json:"cache_config"`
    Reasoning        *ReasoningConfig       `json:"reasoning"`
    ProviderMetadata map[string]interface{} `json:"provider_metadata"`
}
```

### LLMResponse

```go
type LLMResponse struct {
    ID               uuid.UUID              `json:"id"`
    RequestID        uuid.UUID              `json:"request_id"`
    Content          string                 `json:"content"`
    ToolCalls        []ToolCall             `json:"tool_calls"`
    Usage            Usage                  `json:"usage"`
    FinishReason     string                 `json:"finish_reason"`
    ProcessingTime   time.Duration          `json:"processing_time"`
    CreatedAt        time.Time              `json:"created_at"`
    ProviderMetadata map[string]interface{} `json:"provider_metadata"`
}
```

### Usage

```go
type Usage struct {
    PromptTokens     int `json:"prompt_tokens"`
    CompletionTokens int `json:"completion_tokens"`
    TotalTokens      int `json:"total_tokens"`
}
```

### Message

```go
type Message struct {
    Role    string // "system", "user", "assistant"
    Content string
    Name    string // optional
}
```

## Usage

### Creating a Provider

```go
import "dev.helix.code/internal/llm"

// Create OpenAI provider
config := &llm.ProviderConfig{
    Type:    llm.ProviderTypeOpenAI,
    APIKey:  os.Getenv("OPENAI_API_KEY"),
    Model:   "gpt-4",
}
provider, err := llm.NewProvider(config)
```

### Generating Text

```go
req := &llm.LLMRequest{
    Model:       "gpt-4",
    Messages:    []llm.Message{{Role: "user", Content: "Write a function that sorts an array"}},
    MaxTokens:   500,
    Temperature: 0.7,
}

resp, err := provider.Generate(ctx, req)
if err != nil {
    return err
}

fmt.Println(resp.Content)
fmt.Printf("Tokens used: %d\n", resp.Usage.TotalTokens)
```

### Streaming Responses

```go
req := &llm.LLMRequest{
    Model:    "gpt-4",
    Messages: []llm.Message{{Role: "user", Content: "Explain machine learning"}},
    Stream:   true,
}

responseChan := make(chan llm.LLMResponse, 100)
go func() {
    err := provider.GenerateStream(ctx, req, responseChan)
    if err != nil {
        log.Printf("Stream error: %v", err)
    }
}()

for chunk := range responseChan {
    fmt.Print(chunk.Content)
}
```

### Using ModelManager

```go
// Create and configure ModelManager
manager := llm.NewModelManager()

// Register providers
manager.RegisterProvider(openaiProvider)
manager.RegisterProvider(anthropicProvider)
manager.RegisterProvider(ollamaProvider)

// Select optimal model based on criteria
criteria := llm.ModelSelectionCriteria{
    TaskType:        "code_generation",
    RequiredCaps:    []llm.ModelCapability{llm.CapabilityCodeGeneration},
    MaxContextSize:  8192,
    QualityPref:     "balanced",
}
model, err := manager.SelectOptimalModel(criteria)

// Get available models
models := manager.GetAvailableModels()

// Get models by capability
codeModels := manager.GetModelsByCapability([]llm.ModelCapability{llm.CapabilityCodeGeneration})

// Health check all providers
healthMap := manager.HealthCheck(ctx)
```

### Using InitializeModelManager

```go
// Initialize from configuration
configs := []llm.ProviderConfigEntry{
    {Type: llm.ProviderTypeOpenAI, APIKey: apiKey, Enabled: true},
    {Type: llm.ProviderTypeOllama, Endpoint: "http://localhost:11434", Enabled: true},
}
manager, err := llm.InitializeModelManager(configs)
```

## Provider Selection Strategies

Configure provider selection via config:

```yaml
llm:
  selection:
    strategy: "performance"  # performance, cost, availability, round-robin
    fallback_enabled: true
```

### Strategies

- **performance**: Select provider with lowest latency
- **cost**: Select cheapest provider for the request
- **availability**: Select first available provider
- **round-robin**: Distribute requests evenly

## Configuration

```yaml
llm:
  providers:
    openai:
      enabled: true
      api_key: "${OPENAI_API_KEY}"
      model: "gpt-4"
      max_tokens: 4096

    anthropic:
      enabled: true
      api_key: "${ANTHROPIC_API_KEY}"
      model: "claude-3-sonnet-20240229"

    ollama:
      enabled: true
      base_url: "http://localhost:11434"
      model: "llama2"
```

## Token Usage Tracking

```go
// Get token usage from response
resp, _ := provider.Generate(ctx, req)
fmt.Printf("Prompt tokens: %d\n", resp.Usage.PromptTokens)
fmt.Printf("Completion tokens: %d\n", resp.Usage.CompletionTokens)
fmt.Printf("Total tokens: %d\n", resp.Usage.TotalTokens)
```

## Testing

```bash
go test -v ./internal/llm/...
```

## Notes

- Use environment variables for API keys
- Enable fallback for production reliability
- Monitor token usage to control costs
- Use streaming for long responses
