# LLM Package

The `llm` package provides multi-provider Large Language Model (LLM) integration for the HelixCode platform.

## Overview

This package handles:
- Unified interface for multiple LLM providers
- Local inference servers (Llama.cpp, Ollama, vLLM, etc.)
- Cloud APIs (OpenAI, Anthropic, Gemini, etc.)
- Provider selection strategies
- Streaming responses
- Token counting and cost tracking

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
    Generate(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error)
    GenerateStream(ctx context.Context, req *GenerateRequest) (<-chan StreamChunk, error)
    GetCapabilities() *Capabilities
}
```

### GenerateRequest

```go
type GenerateRequest struct {
    Model       string
    Prompt      string
    Messages    []Message
    MaxTokens   int
    Temperature float64
    TopP        float64
    Stop        []string
    Stream      bool
}
```

### Message

```go
type Message struct {
    Role    string // "system", "user", "assistant"
    Content string
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
req := &llm.GenerateRequest{
    Prompt:      "Write a function that sorts an array",
    MaxTokens:   500,
    Temperature: 0.7,
}

resp, err := provider.Generate(ctx, req)
if err != nil {
    return err
}

fmt.Println(resp.Text)
fmt.Printf("Tokens used: %d\n", resp.Usage.TotalTokens)
```

### Streaming Responses

```go
req := &llm.GenerateRequest{
    Prompt: "Explain machine learning",
    Stream: true,
}

chunks, err := provider.GenerateStream(ctx, req)
if err != nil {
    return err
}

for chunk := range chunks {
    fmt.Print(chunk.Text)
}
```

### Using Multiple Providers

```go
manager := llm.NewProviderManager(config)

// Add providers
manager.AddProvider("openai", openaiProvider)
manager.AddProvider("anthropic", anthropicProvider)
manager.AddProvider("ollama", ollamaProvider)

// Use default provider
resp, err := manager.Generate(ctx, req)

// Use specific provider
resp, err := manager.GenerateWithProvider(ctx, "anthropic", req)
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

## Cost Tracking

```go
// Get cost information
cost := resp.Usage.Cost
currency := resp.Usage.Currency

// Get provider cost rates
rates := provider.GetCostRates()
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
