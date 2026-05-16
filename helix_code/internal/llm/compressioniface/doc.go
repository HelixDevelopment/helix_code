// Package compressioniface defines interfaces for conversation compression.
//
// Long conversations accumulate tokens that can exceed LLM context limits.
// This package defines the CompressionCoordinator interface and related types
// for compressing conversations while preserving important information.
//
// # Compression Strategies
//
// Three strategies are available:
//
// StrategySlidingWindow removes older messages while keeping recent ones.
// Fast and predictable but may lose important early context.
//
// StrategySemanticSummarization uses LLM to summarize older messages into
// a condensed form. Preserves meaning better but requires LLM calls.
//
// StrategyHybrid combines both approaches: summarizes very old messages
// and uses sliding window for moderately old ones.
//
// # Interface Usage
//
// Implementations are created through factories:
//
//	coordinator, err := NewCoordinatorFactory(provider, &Config{
//	    Enabled: true,
//	    DefaultStrategy: StrategyHybrid,
//	    TokenBudget: 8000,
//	    AutoCompressEnabled: true,
//	})
//
//	if should, reason := coordinator.ShouldCompress(conv); should {
//	    result, err := coordinator.Compress(ctx, conv)
//	    // result.Compressed contains the compressed conversation
//	}
//
// # Message Types
//
// Messages are categorized for compression decisions:
//   - TypeNormal: Regular conversation messages
//   - TypeCommand: Slash command invocations
//   - TypeToolCall: Tool usage requests
//   - TypeToolResult: Tool execution results
//   - TypeError: Error messages
//
// Messages can be marked as Pinned or Important to prevent compression.
//
// # Statistics
//
// The coordinator tracks compression statistics including total compressions,
// tokens saved, and average compression ratios for monitoring and optimization.
package compressioniface
