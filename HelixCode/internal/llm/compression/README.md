# Compression Package

The `compression` package provides conversation context compression functionality for HelixCode, enabling intelligent management of conversation history by automatically summarizing and compressing older messages to stay within token budgets while preserving semantic meaning and important context.

## Overview

As AI conversations grow longer, they can exceed the context window limits of LLM models. This package solves that problem by implementing multiple compression strategies that reduce token count while maintaining conversation coherence and preserving critical information.

## Architecture

The package is organized into several key components:

```
compression/
├── compressor.go      # CompressionCoordinator, TokenCounter, type conversions
├── strategies.go      # Strategy interface and implementations
├── retention.go       # RetentionPolicy and rules
├── doc.go             # Package documentation
└── compression_test.go # Comprehensive tests
```

### Core Components

- **CompressionCoordinator**: Main entry point that orchestrates compression operations, tracks statistics, and manages configuration
- **CompressionEngine**: Executes compression strategies and manages strategy registration
- **Strategy implementations**: SlidingWindowStrategy, SemanticSummarizationStrategy, HybridStrategy
- **RetentionPolicy**: Defines rules for which messages to retain during compression
- **TokenCounter**: Counts and caches token counts for messages with hash-based caching

## Key Types and Interfaces

### CompressionCoordinator

```go
type CompressionCoordinator struct {
    engine          *CompressionEngine
    tokenCounter    *TokenCounter
    retentionPolicy *RetentionPolicy
    config          *Config
    // ... internal fields
}

// Primary methods
func (cc *CompressionCoordinator) Compress(ctx context.Context, conv *Conversation) (*CompressionResult, error)
func (cc *CompressionCoordinator) ShouldCompress(conv *Conversation) (bool, string)
func (cc *CompressionCoordinator) EstimateCompression(conv *Conversation) (*CompressionEstimate, error)
func (cc *CompressionCoordinator) GetStats() *CompressionStats
func (cc *CompressionCoordinator) GetConfig() *Config
func (cc *CompressionCoordinator) UpdateConfig(config *Config)
```

### Compression Strategies

```go
type CompressionStrategy int

const (
    StrategySlidingWindow           CompressionStrategy = iota  // Keeps last N messages
    StrategySemanticSummarization                               // LLM-based summarization
    StrategyHybrid                                              // Combines both approaches
    StrategyCustom                                              // Custom compression logic
)
```

### Strategy Interface

```go
type Strategy interface {
    Execute(ctx context.Context, conv *Conversation, policy *RetentionPolicy) (*CompressionResult, error)
    Estimate(conv *Conversation, policy *RetentionPolicy) (*CompressionEstimate, error)
    Name() string
}
```

### Message Types

```go
type Message struct {
    ID         string
    Role       MessageRole      // system, user, assistant
    Content    string
    Timestamp  time.Time
    TokenCount int
    Metadata   MessageMetadata
    Pinned     bool            // Always retained
    Important  bool            // High priority retention
}

type MessageType string

const (
    TypeNormal     MessageType = "normal"
    TypeCommand    MessageType = "command"
    TypeToolCall   MessageType = "tool_call"
    TypeToolResult MessageType = "tool_result"
    TypeError      MessageType = "error"
)
```

## Usage Examples

### Basic Compression

```go
// Create a compression coordinator
provider := getYourLLMProvider() // Any llm.Provider implementation
coordinator := compression.NewCompressionCoordinator(provider)

// Create a conversation
conversation := &compression.Conversation{
    ID:       "conv-123",
    Messages: messages,
}

// Check if compression is needed
if shouldCompress, reason := coordinator.ShouldCompress(conversation); shouldCompress {
    result, err := coordinator.Compress(context.Background(), conversation)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Saved %d tokens, removed %d messages\n",
        result.TokensSaved, result.MessagesRemoved)
}
```

### Using Different Strategies

```go
// Sliding Window - keeps last N messages
coordinator := compression.NewCompressionCoordinator(
    provider,
    compression.WithStrategy(compression.StrategySlidingWindow),
)

// Semantic Summarization - uses LLM to summarize older messages
coordinator := compression.NewCompressionCoordinator(
    provider,
    compression.WithStrategy(compression.StrategySemanticSummarization),
)

// Hybrid - combines both for optimal results (recommended)
coordinator := compression.NewCompressionCoordinator(
    provider,
    compression.WithStrategy(compression.StrategyHybrid),
)
```

### Custom Retention Policies

```go
// Use preset policies
policy := compression.GetPolicyByPreset(compression.PresetConservative) // Retains more
policy := compression.GetPolicyByPreset(compression.PresetBalanced)     // Default
policy := compression.GetPolicyByPreset(compression.PresetAggressive)   // Maximum compression

coordinator := compression.NewCompressionCoordinator(
    provider,
    compression.WithRetentionPolicy(policy),
)

// Build custom policies
policy := compression.NewPolicyBuilder().
    WithRecentCount(15).
    WithMinAge(1 * time.Hour).
    WithDefaultRules().
    AddRule(compression.RetentionRule{
        Priority: 11,
        Match: func(msg *compression.Message, pos compression.MessagePosition) bool {
            return msg.Metadata.Type == compression.TypeCommand
        },
        Action: compression.ActionRetain,
        Reason: "preserve_commands",
    }).
    Build()
```

### Estimating Compression Impact

```go
estimate, err := coordinator.EstimateCompression(conversation)
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Would save %d tokens, remove %d messages (ratio: %.2f)\n",
    estimate.TokensSaved, estimate.MessagesRemoved, estimate.EstimatedRatio)
```

### Analyzing Policy Effects

```go
stats := compression.AnalyzePolicy(policy, messages)
fmt.Printf("Retention rate: %.2f%%\n", stats.RetentionRate*100)
for rule, count := range stats.RuleMatches {
    fmt.Printf("  %s: %d messages\n", rule, count)
}
```

## Configuration Options

### Config Structure

```go
type Config struct {
    Enabled              bool              // Enable compression
    DefaultStrategy      CompressionStrategy
    TokenBudget          int               // Maximum token budget (default: 200000)
    WarningThreshold     int               // Warning at this token count (default: 150000)
    CompressionThreshold int               // Compress above this count (default: 180000)
    AutoCompressEnabled  bool              // Enable automatic compression
    AutoCompressInterval time.Duration     // Auto-compress interval (default: 5 min)
}
```

### Functional Options

```go
coordinator := compression.NewCompressionCoordinator(
    provider,
    compression.WithConfig(customConfig),
    compression.WithStrategy(compression.StrategyHybrid),
    compression.WithThreshold(180000),
    compression.WithAutoCompress(true),
    compression.WithRetentionPolicy(customPolicy),
)
```

## Retention Policy Presets

| Preset | Recent Messages | Min Age | Description |
|--------|-----------------|---------|-------------|
| Conservative | 30 | 1 hour | Retains more messages for complex workflows |
| Balanced | 10 | 30 min | Default balanced approach |
| Aggressive | 5 | 10 min | Maximum compression, minimal retention |

### Default Retention Rules (by priority)

1. **System messages** (Priority 10) - Always retained
2. **Pinned messages** (Priority 9) - Always retained
3. **Important messages** (Priority 8) - Always retained
4. **Command messages** (Priority 7) - Preserved for context
5. **Recent messages** (Priority 6) - Based on recentCount
6. **Error messages** (Priority 5) - Preserved for debugging
7. **Tool messages** (Priority 4) - Tool calls and results
8. **File references** (Priority 3) - Messages with file paths
9. **Code blocks** (Priority 2) - Messages containing code

## Message Conversion

Convert between LLM and compression message formats:

```go
// LLM message to compression message
llmMsg := llm.Message{Role: "user", Content: "Hello"}
compMsg := compression.ConvertLLMMessage(llmMsg)

// Compression message back to LLM message
llmMsg = compression.ConvertToLLMMessage(compMsg)

// Batch conversion
llmMessages := compression.ConvertToLLMMessages(compMessages)
```

## Best Practices

1. **Use StrategyHybrid** for most use cases - it balances compression ratio and semantic preservation
2. **Set compression threshold** to 80-85% of your model's context window
3. **Always keep system messages** (handled automatically by default rules)
4. **Pin critical messages** that must never be removed
5. **Use Conservative policy** for complex, multi-step workflows
6. **Use Aggressive policy** for simple Q&A conversations
7. **Monitor compression statistics** to tune configuration over time
8. **Test compression** with representative conversations before deploying

## Performance Considerations

- **Token count caching**: Results are cached using content hashes to avoid repeated calculations
- **Efficient filtering**: Messages are filtered with priority-sorted rules
- **Chunk processing**: Semantic summarization processes messages in configurable chunks (default 5000 tokens)
- **Minimal allocations**: Slice operations minimize memory allocations

## Thread Safety

The `CompressionCoordinator` is fully thread-safe:
- Uses `sync.RWMutex` for concurrent access protection
- Safe to use from multiple goroutines simultaneously
- Token counter caching is also thread-safe

## Error Handling

```go
result, err := coordinator.Compress(ctx, conversation)
if err != nil {
    // Handle error - possibly fall back to sliding window only
    log.Printf("Compression failed: %v", err)
    return
}
```

## Interface Compatibility

The package implements `compressioniface.CompressionCoordinator` for cross-package use:

```go
var _ compressioniface.CompressionCoordinator = (*CompressionCoordinator)(nil)
```

This allows the compression coordinator to be used by other packages that only depend on the interface definition.

## Statistics Tracking

```go
stats := coordinator.GetStats()
fmt.Printf("Total compressions: %d\n", stats.TotalCompressions)
fmt.Printf("Total tokens saved: %d\n", stats.TotalTokensSaved)
fmt.Printf("Total messages removed: %d\n", stats.TotalMessagesRemoved)
fmt.Printf("Average compression ratio: %.2f\n", stats.AverageRatio)
fmt.Printf("Last compression: %v\n", stats.LastCompression)
```

## Testing

The package includes comprehensive tests covering all strategies, retention policies, and edge cases:

```bash
cd HelixCode
go test -v ./internal/llm/compression

# With coverage
go test -cover ./internal/llm/compression

# Run benchmarks
go test -bench=. ./internal/llm/compression
```
