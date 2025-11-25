# Memory Package

The `memory` package provides long-term memory integration for the HelixCode platform.

## Overview

This package handles:
- Multi-provider memory integration (Mem0, Zep, Memonto, BaseAI)
- Vector storage and retrieval
- Conversation persistence
- Context management
- Memory search and filtering

## Supported Providers

- **Mem0** - Memory management for AI agents
- **Zep** - Long-term memory for LLMs
- **Memonto** - Knowledge graph memory
- **BaseAI** - Local memory storage
- **Pinecone** - Vector database
- **Weaviate** - Vector search engine
- **ChromaDB** - Open-source embeddings database
- **Qdrant** - Vector similarity search

## Key Types

### MemoryManager

The main memory manager:

```go
type MemoryManager struct {
    providers map[string]MemoryProvider
    config    *Config
    logger    *logging.Logger
}
```

### MemoryProvider

```go
type MemoryProvider interface {
    Store(ctx context.Context, memory *Memory) error
    Retrieve(ctx context.Context, query *Query) ([]*Memory, error)
    Delete(ctx context.Context, id string) error
    Search(ctx context.Context, query string, limit int) ([]*Memory, error)
}
```

### Memory

```go
type Memory struct {
    ID        string
    Type      MemoryType
    Content   string
    Embedding []float64
    Metadata  map[string]interface{}
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

## Usage

### Creating the Memory Manager

```go
import "dev.helix.code/internal/memory"

config := &memory.Config{
    DefaultProvider: "mem0",
    Providers: map[string]*memory.ProviderConfig{
        "mem0": {
            Type:    "mem0",
            APIKey:  os.Getenv("MEM0_API_KEY"),
            Enabled: true,
        },
    },
}

manager := memory.NewManager(config)
err := manager.Initialize(ctx)
```

### Storing Memories

```go
mem := &memory.Memory{
    Type:    memory.TypeConversation,
    Content: "User prefers Python for scripting tasks",
    Metadata: map[string]interface{}{
        "user_id": userID,
        "topic":   "preferences",
    },
}

err := manager.Store(ctx, mem)
```

### Retrieving Memories

```go
// Retrieve by ID
mem, err := manager.Retrieve(ctx, memoryID)

// Search memories
results, err := manager.Search(ctx, "Python preferences", 10)

// Query with filters
query := &memory.Query{
    Filters: map[string]interface{}{
        "user_id": userID,
        "type":    "preference",
    },
    Limit: 20,
}
results, err := manager.Query(ctx, query)
```

### Conversation Memory

```go
// Store conversation
conv := &memory.Conversation{
    UserID:   userID,
    Messages: messages,
    Summary:  "Discussed Python best practices",
}
err := manager.StoreConversation(ctx, conv)

// Get recent conversations
convs, err := manager.GetRecentConversations(ctx, userID, 10)
```

### Vector Search

```go
// Generate embedding for query
embedding := manager.GenerateEmbedding(ctx, "How do I use Python async?")

// Search by vector similarity
results, err := manager.VectorSearch(ctx, embedding, 10, 0.8)
```

## Memory Types

```go
type MemoryType string

const (
    TypeConversation MemoryType = "conversation"
    TypeFact         MemoryType = "fact"
    TypePreference   MemoryType = "preference"
    TypeEpisodic     MemoryType = "episodic"
    TypeSemantic     MemoryType = "semantic"
    TypeProcedural   MemoryType = "procedural"
)
```

## Configuration

```yaml
memory:
  default_provider: "mem0"
  embedding_model: "text-embedding-ada-002"
  providers:
    mem0:
      enabled: true
      api_key: "${MEM0_API_KEY}"

    zep:
      enabled: true
      base_url: "http://localhost:8000"
      api_key: "${ZEP_API_KEY}"

    pinecone:
      enabled: true
      api_key: "${PINECONE_API_KEY}"
      environment: "us-west1-gcp"
      index_name: "helixcode"
```

## Provider Registry

```go
// Get available providers
providers := manager.ListProviders()

// Get provider by name
provider, err := manager.GetProvider("mem0")

// Health check all providers
status := manager.HealthCheck(ctx)
```

## Testing

```bash
go test -v ./internal/memory/...
```

## Notes

- Use environment variables for API keys
- Configure embedding model to match your LLM
- Implement memory cleanup for old entries
- Monitor storage usage for cost control
