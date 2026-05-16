# Providers Package

The `providers` package provides unified AI and vector database integration for the HelixCode platform.

## Overview

This package handles:
- Unified AI provider interface
- Vector database integration
- Memory management integration
- Conversation management
- Personality management

## Key Components

### AIIntegration

Unified interface for AI systems:

```go
type AIIntegration struct {
    providers       map[string]AIProvider
    vector          *VectorIntegration
    memory          *MemoryIntegration
    conversationMgr *ConversationManager
    personalityMgr  *PersonalityManager
}
```

### VectorIntegration

Vector database operations:

```go
type VectorIntegration struct {
    providers map[string]VectorProvider
    manager   *ProviderManager
}
```

## AI Providers

Supported AI providers:
- OpenAI
- Anthropic (Claude)
- Cohere
- HuggingFace
- Mistral
- Gemini
- Gemma
- LlamaIndex
- MemGPT
- CrewAI
- CharacterAI
- Replika
- Anima

## Usage

### Creating AI Integration

```go
import "dev.helix.code/internal/providers"

config := &providers.AIConfig{
    DefaultLLM:    "openai",
    DefaultMemory: "memgpt",
    Providers: map[string]*providers.AIProviderConfig{
        "openai": {
            Enabled:     true,
            Model:       "gpt-4",
            MaxTokens:   4096,
            Temperature: 0.7,
        },
    },
}

ai := providers.NewAIIntegration(config)
err := ai.Initialize(ctx)
```

### Text Generation

```go
// Generate text
result, err := ai.GenerateText(ctx, "Write a function", &providers.GenerationOptions{
    MaxTokens:   500,
    Temperature: 0.7,
})

// Generate with specific provider
result, err := ai.GenerateTextWithProvider(ctx, "anthropic", prompt, options)
```

### Chat Generation

```go
messages := []*providers.ChatMessage{
    {Role: "user", Content: "Hello!"},
}

result, err := ai.GenerateChat(ctx, messages, &providers.ChatOptions{
    Model:       "gpt-4",
    MaxTokens:   1000,
    Temperature: 0.7,
})
```

### Embeddings

```go
// Generate embedding
embedding, err := ai.GenerateEmbedding(ctx, "Hello world")

// Store in vector database
vectorData := &providers.VectorData{
    ID:        "vec-1",
    Embedding: embedding,
    Metadata:  map[string]interface{}{"type": "greeting"},
}
err := ai.GetVector().StoreVector(ctx, vectorData)
```

### Vector Search

```go
// Search for similar vectors
results, err := ai.GetVector().FindSimilarVectors(ctx, embedding, 10, nil)

for _, result := range results {
    fmt.Printf("ID: %s, Score: %f\n", result.ID, result.Score)
}
```

### Conversation Management

```go
// Create conversation
conv, err := ai.GetConversation().CreateConversation(ctx, "conv-1")

// Add message
msg := &providers.ChatMessage{Role: "user", Content: "Hello"}
err := ai.GetConversation().AddMessage(ctx, "conv-1", msg)

// Get conversation
conv, err := ai.GetConversation().GetConversation(ctx, "conv-1")
```

### Personality Management

```go
// Get personality
personality, err := ai.GetPersonality().GetPersonality("technical")

// Set active personality
err := ai.GetPersonality().SetActivePersonality("creative")

// Add custom personality
newPersonality := &providers.Personality{
    ID:           "custom",
    Name:         "Custom Assistant",
    SystemPrompt: "You are a custom assistant...",
    Temperature:  0.8,
    Enabled:      true,
}
err := ai.GetPersonality().AddPersonality(newPersonality)
```

## Configuration

```yaml
providers:
  ai:
    default_llm: "openai"
    default_memory: "memgpt"
    providers:
      openai:
        enabled: true
        api_key: "${OPENAI_API_KEY}"
        model: "gpt-4"

  vector:
    default_provider: "pinecone"
    providers:
      pinecone:
        enabled: true
        api_key: "${PINECONE_API_KEY}"
```

## Health Check

```go
status, err := ai.HealthCheck(ctx)

fmt.Printf("Status: %s\n", status.Status)
fmt.Printf("Healthy Providers: %d/%d\n", status.HealthyProviders, status.TotalProviders)
```

## Testing

```bash
go test -v ./internal/providers/...
```

## Notes

- Use environment variables for API keys
- Configure fallback providers for reliability
- Monitor provider health and costs
- Use appropriate models for tasks
