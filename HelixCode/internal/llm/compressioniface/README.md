# Compressioniface Package

The `compressioniface` package provides interface definitions and shared types for the compression subsystem in HelixCode. It enables cross-package dependency management by defining contracts that the compression implementation must fulfill.

## Overview

This package exists to solve a common Go dependency problem: allowing other packages to depend on compression functionality without creating import cycles. By separating interface definitions from implementations, packages can depend on `compressioniface` while the actual `compression` package implements these interfaces.

## Architecture

```
compressioniface/
├── interface.go        # Interface and type definitions
└── interface_test.go   # Comprehensive tests for all types
```

### Design Pattern

This package follows the **interface segregation principle**:

```
                    ┌─────────────────────┐
                    │  compressioniface   │
                    │  (interfaces only)  │
                    └──────────┬──────────┘
                               │
           ┌───────────────────┴───────────────────┐
           │                                       │
           ▼                                       ▼
┌─────────────────────┐                 ┌─────────────────────┐
│    compression      │                 │  other packages     │
│  (implementation)   │                 │  (consumers)        │
└─────────────────────┘                 └─────────────────────┘
```

## Key Interfaces

### CompressionCoordinator

The main interface for compression operations:

```go
type CompressionCoordinator interface {
    // Compress compresses a conversation using the configured strategy
    Compress(ctx context.Context, conv *Conversation) (*CompressionResult, error)

    // ShouldCompress determines if compression is needed
    ShouldCompress(conv *Conversation) (bool, string)

    // EstimateCompression estimates the result of compression without executing it
    EstimateCompression(conv *Conversation) (*CompressionEstimate, error)

    // GetStats returns compression statistics
    GetStats() *CompressionStats

    // GetConfig returns the current configuration
    GetConfig() *Config

    // UpdateConfig updates the configuration
    UpdateConfig(config *Config)
}
```

### Factory Pattern

```go
// Factory creates compression coordinators
type Factory interface {
    NewCoordinator(provider Provider, config *Config) (CompressionCoordinator, error)
}

// NewCoordinatorFactory is registered by the compression package at init time
var NewCoordinatorFactory func(provider Provider, config *Config) (CompressionCoordinator, error)
```

## Type Definitions

### Conversation

```go
type Conversation struct {
    ID                 string
    Messages           []*Message
    Metadata           map[string]interface{}
    CreatedAt          time.Time
    UpdatedAt          time.Time
    TokenCount         int
    Compressed         bool
    CompressionHistory []*CompressionRecord
}
```

### Message

```go
type Message struct {
    ID         string
    Role       MessageRole      // system, user, assistant
    Content    string
    Timestamp  time.Time
    TokenCount int
    Metadata   MessageMetadata
    Pinned     bool
    Important  bool
}
```

### Message Roles

```go
type MessageRole string

const (
    RoleSystem    MessageRole = "system"
    RoleUser      MessageRole = "user"
    RoleAssistant MessageRole = "assistant"
)
```

### Message Types

```go
type MessageType string

const (
    TypeNormal     MessageType = "normal"
    TypeCommand    MessageType = "command"
    TypeToolCall   MessageType = "tool_call"
    TypeToolResult MessageType = "tool_result"
    TypeError      MessageType = "error"
)
```

### Message Metadata

```go
type MessageMetadata struct {
    Type       MessageType
    Context    []string     // Related context tags
    References []string     // References to other messages or resources
    Tools      []string     // Tools used in the message
    FilePaths  []string     // File paths mentioned
    CodeBlocks int          // Number of code blocks
    HasError   bool         // Whether message contains an error
}
```

### Compression Strategy

```go
type CompressionStrategy int

const (
    StrategySlidingWindow CompressionStrategy = iota
    StrategySemanticSummarization
    StrategyHybrid
)

func (cs CompressionStrategy) String() string {
    switch cs {
    case StrategySlidingWindow:
        return "sliding_window"
    case StrategySemanticSummarization:
        return "semantic_summarization"
    case StrategyHybrid:
        return "hybrid"
    default:
        return "unknown"
    }
}
```

### Configuration

```go
type Config struct {
    Enabled              bool
    DefaultStrategy      CompressionStrategy
    TokenBudget          int               // Maximum token budget
    WarningThreshold     int               // Warning threshold
    CompressionThreshold int               // Compression trigger threshold
    AutoCompressEnabled  bool              // Enable auto-compression
    AutoCompressInterval time.Duration     // Auto-compress interval
}
```

### Compression Result

```go
type CompressionResult struct {
    Original        *Conversation       // Original conversation
    Compressed      *Conversation       // Compressed result
    Strategy        CompressionStrategy // Strategy used
    TokensSaved     int                 // Tokens saved by compression
    MessagesRemoved int                 // Messages removed
    Summary         string              // Summary of compression
    Timestamp       time.Time           // When compression occurred
}
```

### Compression Estimate

```go
type CompressionEstimate struct {
    TokensSaved     int     // Estimated tokens to save
    MessagesRemoved int     // Messages that would be removed
    MessagesKept    int     // Messages that would be kept
    EstimatedRatio  float64 // Compression ratio (0-1)
}
```

### Compression Statistics

```go
type CompressionStats struct {
    TotalCompressions    int       // Total number of compressions
    TotalTokensSaved     int       // Cumulative tokens saved
    TotalMessagesRemoved int       // Cumulative messages removed
    LastCompression      time.Time // When last compression occurred
    AverageRatio         float64   // Average compression ratio
}
```

### Compression Record

```go
type CompressionRecord struct {
    Timestamp        time.Time
    Strategy         CompressionStrategy
    MessagesBefore   int
    MessagesAfter    int
    TokensBefore     int
    TokensAfter      int
    CompressionRatio float64
}
```

## Usage Examples

### Using the Factory Pattern

```go
import "dev.helix.code/internal/llm/compressioniface"

// The compression package registers its factory at init time
// Consumers can use it like this:

config := &compressioniface.Config{
    Enabled:              true,
    DefaultStrategy:      compressioniface.StrategyHybrid,
    TokenBudget:          200000,
    WarningThreshold:     150000,
    CompressionThreshold: 180000,
    AutoCompressEnabled:  true,
    AutoCompressInterval: 5 * time.Minute,
}

coordinator, err := compressioniface.NewCoordinatorFactory(provider, config)
if err != nil {
    log.Fatal(err)
}
```

### Creating Conversations

```go
conversation := &compressioniface.Conversation{
    ID: "conv-123",
    Messages: []*compressioniface.Message{
        {
            ID:        "msg-1",
            Role:      compressioniface.RoleSystem,
            Content:   "You are a helpful assistant.",
            Timestamp: time.Now(),
            Metadata: compressioniface.MessageMetadata{
                Type: compressioniface.TypeNormal,
            },
        },
        {
            ID:        "msg-2",
            Role:      compressioniface.RoleUser,
            Content:   "Hello!",
            Timestamp: time.Now(),
        },
    },
    CreatedAt:  time.Now(),
    UpdatedAt:  time.Now(),
    TokenCount: 0, // Will be calculated
}
```

### Checking Compression Need

```go
shouldCompress, reason := coordinator.ShouldCompress(conversation)
if shouldCompress {
    fmt.Printf("Compression needed: %s\n", reason)

    result, err := coordinator.Compress(ctx, conversation)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Compressed: saved %d tokens\n", result.TokensSaved)
}
```

### Working with Message Metadata

```go
message := &compressioniface.Message{
    ID:      "msg-tool",
    Role:    compressioniface.RoleAssistant,
    Content: "Executing tool...",
    Metadata: compressioniface.MessageMetadata{
        Type:       compressioniface.TypeToolCall,
        Tools:      []string{"bash", "read"},
        FilePaths:  []string{"/path/to/file.go"},
        CodeBlocks: 1,
        HasError:   false,
    },
    Pinned:    false,
    Important: true,
}
```

## Configuration Best Practices

### Recommended Default Configuration

```go
config := &compressioniface.Config{
    Enabled:              true,
    DefaultStrategy:      compressioniface.StrategyHybrid, // Best balance
    TokenBudget:          200000,                          // Claude's context window
    WarningThreshold:     150000,                          // 75% of budget
    CompressionThreshold: 180000,                          // 90% of budget
    AutoCompressEnabled:  true,
    AutoCompressInterval: 5 * time.Minute,
}
```

### Strategy Selection Guide

| Strategy | Use Case | Pros | Cons |
|----------|----------|------|------|
| `StrategySlidingWindow` | Simple conversations | Fast, predictable | Loses semantic context |
| `StrategySemanticSummarization` | Complex discussions | Preserves meaning | Requires LLM calls |
| `StrategyHybrid` | General use | Best of both | Slightly more complex |

## Thread Safety

All types in this package are designed for concurrent use:
- Immutable type definitions
- No shared state in interface package
- Implementation is responsible for thread safety

## Testing

The package includes comprehensive tests for all types:

```bash
cd HelixCode
go test -v ./internal/llm/compressioniface

# Run specific test categories
go test -v ./internal/llm/compressioniface -run TestCompressionStrategy
go test -v ./internal/llm/compressioniface -run TestMessage
go test -v ./internal/llm/compressioniface -run TestConfig
```

### Test Coverage

Tests cover:
- All constant values and string representations
- Type uniqueness (no duplicate enum values)
- Struct field assignments and zero values
- Edge cases (empty arrays, nil values)
- Type conversions

## Integration with Other Packages

### For Consumers

```go
// In a package that needs compression
import "dev.helix.code/internal/llm/compressioniface"

type MyService struct {
    compressor compressioniface.CompressionCoordinator
}

func NewMyService(compressor compressioniface.CompressionCoordinator) *MyService {
    return &MyService{compressor: compressor}
}
```

### For the Implementation Package

```go
// In the compression package
import "dev.helix.code/internal/llm/compressioniface"

// Ensure implementation satisfies interface
var _ compressioniface.CompressionCoordinator = (*CompressionCoordinator)(nil)

func init() {
    // Register factory for consumers
    compressioniface.NewCoordinatorFactory = func(provider compressioniface.Provider, config *compressioniface.Config) (compressioniface.CompressionCoordinator, error) {
        // Create and return implementation
        return NewCompressionCoordinator(provider, config), nil
    }
}
```

## Why This Package Exists

1. **Avoid import cycles**: Other packages can depend on interfaces without importing the full implementation
2. **Testability**: Consumers can mock the interface easily
3. **Flexibility**: Implementation can change without affecting consumers
4. **Clean architecture**: Separates contracts from implementations
5. **Dependency injection**: Enables proper DI patterns in Go
