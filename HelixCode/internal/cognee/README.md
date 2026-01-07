# Cognee Integration Package

This package provides comprehensive integration with [Cognee](https://cognee.ai), an AI-powered knowledge graph and memory system for building intelligent applications with enhanced retrieval and reasoning capabilities.

## Overview

The cognee package implements:

- **CogneeService**: Main service for managing Cognee operations
- **CogneeClient**: HTTP client for Cognee API communication
- **CogneeManager**: High-level manager for backward compatibility
- **Handler**: REST API handlers for exposing Cognee endpoints
- **PerformanceOptimizer**: Research-based performance optimization

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                      CogneeManager                               │
│  (High-level API for knowledge processing and search)           │
└───────────────────────────┬─────────────────────────────────────┘
                            │
┌───────────────────────────▼─────────────────────────────────────┐
│                      CogneeService                               │
│  - Memory management (add, search, delete)                      │
│  - Dataset operations (create, list, delete)                    │
│  - Cognify (knowledge graph processing)                         │
│  - Insights extraction                                          │
│  - Code pipeline processing                                     │
│  - Graph visualization                                          │
│  - Background health checks                                     │
│  - Event handling                                               │
│  - Caching                                                      │
└───────────────────────────┬─────────────────────────────────────┘
                            │
┌───────────────────────────▼─────────────────────────────────────┐
│                      CogneeClient                                │
│  - HTTP communication with Cognee API                           │
│  - Auto-containerization (Docker/Podman)                        │
│  - Connection management                                        │
│  - Authentication                                               │
└───────────────────────────┬─────────────────────────────────────┘
                            │
                            ▼
                    Cognee API Server
```

## Usage

### Basic Usage

```go
import (
    "context"
    "dev.helix.code/internal/cognee"
    "dev.helix.code/internal/config"
    "dev.helix.code/internal/hardware"
)

// Create configuration
cfg := config.DefaultCogneeConfig()

// Create service
service, err := cognee.NewCogneeService(cfg, hwProfile)
if err != nil {
    return err
}

// Start the service
ctx := context.Background()
if err := service.Start(ctx); err != nil {
    return err
}
defer service.Stop(ctx)

// Add memory
resp, err := service.AddMemory(ctx, &cognee.AddMemoryRequest{
    Content:     "Important information to remember",
    DatasetName: "my-dataset",
    ContentType: "text",
})

// Search memory
results, err := service.SearchMemory(ctx, &cognee.SearchMemoryRequest{
    Query:       "important information",
    DatasetName: "my-dataset",
    Limit:       10,
})

// Process data into knowledge graphs
cognifyResp, err := service.Cognify(ctx, &cognee.CognifyRequest{
    Datasets: []string{"my-dataset"},
})
```

### Using CogneeManager (Backward Compatible)

```go
// Create manager with full Helix configuration
manager, err := cognee.NewCogneeManager(helixConfig, hwProfile)
if err != nil {
    return err
}
defer manager.Close()

// Process knowledge
err = manager.ProcessKnowledge(ctx, "Content to process")

// Search knowledge
results, err := manager.SearchKnowledge(ctx, "search query")
```

### REST API Handler

```go
import "github.com/gin-gonic/gin"

router := gin.Default()
api := router.Group("/api/v1")

// Register Cognee routes
handler := cognee.NewHandler(service)
handler.RegisterRoutes(api)
```

## API Endpoints

The handler exposes the following REST endpoints:

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/cognee/health` | Health check |
| GET | `/cognee/stats` | Service statistics |
| POST | `/cognee/memory` | Add memory entry |
| POST | `/cognee/memory/batch` | Add multiple memories |
| POST | `/cognee/search` | Search memories |
| DELETE | `/cognee/memory` | Delete data |
| POST | `/cognee/cognify` | Process into knowledge graph |
| POST | `/cognee/insights` | Get insights |
| POST | `/cognee/graph/complete` | LLM-powered graph completion |
| POST | `/cognee/code` | Process code |
| GET | `/cognee/datasets` | List datasets |
| POST | `/cognee/datasets` | Create dataset |
| GET | `/cognee/datasets/:name` | Get dataset |
| DELETE | `/cognee/datasets/:name` | Delete dataset |
| POST | `/cognee/visualize` | Get graph visualization |
| POST | `/cognee/feedback` | Submit feedback |

## Configuration

Configure Cognee in your `config.yaml`:

```yaml
cognee:
  enabled: true
  auto_start: true
  host: localhost
  port: 8000
  mode: local  # local, cloud, hybrid

  remote_api:
    service_endpoint: https://api.cognee.ai
    api_key: ""
    timeout: 30s

  optimization:
    host_aware: true
    cpu_optimization: true
    gpu_optimization: true
    memory_optimization: true

  features:
    knowledge_graph: true
    semantic_search: true
    real_time_processing: true
    multi_modal_support: true
    graph_analytics: true
    advanced_insights: true
    auto_optimization: true

  performance:
    workers: 4
    queue_size: 1000
    batch_size: 32
    flush_interval: 5s
    optimization_level: high

  cache:
    enabled: true
    type: redis
    host: localhost
    port: 6379
    ttl: 1h
    compression: true

  monitoring:
    enabled: true
    metrics_port: 9090
    health_check: 30s
    log_level: info
    trace_enabled: true
```

## Data Models

### CogneeMemory

Represents a memory entry stored in Cognee:

```go
type CogneeMemory struct {
    ID          string                 `json:"id"`
    VectorID    string                 `json:"vector_id"`
    Content     string                 `json:"content"`
    ContentType string                 `json:"content_type"`
    DatasetName string                 `json:"dataset_name"`
    Metadata    map[string]interface{} `json:"metadata"`
    GraphNodes  map[string]interface{} `json:"graph_nodes"`
    Embedding   []float32              `json:"embedding"`
    Score       float64                `json:"score"`
    CreatedAt   time.Time              `json:"created_at"`
    UpdatedAt   time.Time              `json:"updated_at"`
}
```

### Dataset

Represents a Cognee dataset:

```go
type Dataset struct {
    ID          string                 `json:"id"`
    Name        string                 `json:"name"`
    Description string                 `json:"description"`
    Metadata    map[string]interface{} `json:"metadata"`
    CreatedAt   time.Time              `json:"created_at"`
    UpdatedAt   time.Time              `json:"updated_at"`
    MemoryCount int                    `json:"memory_count"`
}
```

## Performance Optimization

The package includes research-based performance optimization:

- **Neural-Symbolic Integration**: Hybrid indexing and reasoning
- **Graph Compression**: Adaptive compression algorithms
- **Batch Processing**: Dynamic batch sizing
- **Parallel Processing**: CPU/GPU optimization
- **Memory Management**: Adaptive memory allocation

```go
// Initialize performance optimizer
optimizer, err := cognee.NewPerformanceOptimizer(cfg, hwProfile)
if err != nil {
    return err
}

ctx := context.Background()
optimizer.Initialize(ctx)
optimizer.Start(ctx)
defer optimizer.Stop(ctx)

// Run optimization
result, err := optimizer.Optimize(ctx)
if err != nil {
    return err
}
fmt.Printf("Improvement: %.2f%%\n", result.Improvement*100)
```

## Auto-Containerization

The client supports automatic container startup using Docker or Podman:

```go
client := cognee.NewClient(cfg)

// Auto-start Cognee container if not running
if err := client.AutoContainerize(ctx); err != nil {
    log.Printf("Failed to auto-start: %v", err)
}
```

## Event Handling

Register handlers for Cognee events:

```go
service.RegisterEventHandler(func(event *cognee.CogneeEvent) {
    log.Printf("Cognee event: type=%s, action=%s", event.Type, event.Action)
})
```

## Testing

Run the cognee package tests:

```bash
cd HelixCode
go test -v ./internal/cognee/...
```

With coverage:

```bash
go test -cover ./internal/cognee/...
```

## File Structure

```
internal/cognee/
├── README.md              # This documentation
├── models.go              # Data models and request/response types
├── client.go              # Cognee API HTTP client
├── service.go             # Main CogneeService implementation
├── cognee_manager.go      # High-level CogneeManager
├── handler.go             # REST API handlers
├── host_optimizer.go      # Host-aware optimization
├── performance_optimizer.go # Performance optimization
└── cognee_test.go         # Unit tests
```

## Dependencies

- `github.com/gin-gonic/gin`: HTTP router for handlers
- `github.com/google/uuid`: UUID generation
- `dev.helix.code/internal/config`: Configuration
- `dev.helix.code/internal/hardware`: Hardware profiling
- `dev.helix.code/internal/logging`: Logging

## Related Resources

- [Cognee Documentation](https://docs.cognee.ai)
- [Cognee GitHub](https://github.com/cognee-ai/cognee)
- [HelixCode Configuration Guide](../config/README.md)
