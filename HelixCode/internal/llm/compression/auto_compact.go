package compression

import (
	"context"
	"fmt"

	"dev.helix.code/internal/llm/compressioniface"
)

// ProviderTokenCounter is the subset of provider capability that AutoCompactor
// needs. Defined here to keep the dependency small and mock-friendly.
// Named to avoid collision with the compression-internal TokenCounter struct.
type ProviderTokenCounter interface {
	GetContextWindow() int
	CountTokens(text string) (int, error)
}

// AutoCompactionResult is the outcome of a MaybeCompact call.
type AutoCompactionResult struct {
	WasCompacted   bool
	TokensBefore   int
	// TokensAfter is computed from TokensBefore minus compressioniface.CompressionResult.TokensSaved.
	// (CompressionResult has no direct TokensAfter field; TokensAfter lives on CompressionRecord.)
	TokensAfter    int
	WindowSize     int
	ThresholdRatio float64
}

// AutoCompactor wraps an existing CompressionCoordinator with claude-code's
// 80%-window trigger and thrashing-detection semantics.
type AutoCompactor struct {
	tokens         ProviderTokenCounter
	coord          compressioniface.CompressionCoordinator
	guard          *ThrashingGuard
	thresholdRatio float64
}

// NewAutoCompactor returns an AutoCompactor configured with the given
// token-counter, compression coordinator, thrashing guard, and threshold
// ratio (e.g., 0.80 for claude-code's 80% trigger).
func NewAutoCompactor(
	tokens ProviderTokenCounter,
	coord compressioniface.CompressionCoordinator,
	guard *ThrashingGuard,
	thresholdRatio float64,
) *AutoCompactor {
	return &AutoCompactor{
		tokens:         tokens,
		coord:          coord,
		guard:          guard,
		thresholdRatio: thresholdRatio,
	}
}

// MaybeCompact checks whether the conversation has crossed the configured
// threshold and, if so, runs compression via the coordinator (gated by the
// thrashing guard). Returns ErrThrashing (unwrapped) if the guard rejects
// the call.
func (a *AutoCompactor) MaybeCompact(ctx context.Context, conv *compressioniface.Conversation) (*AutoCompactionResult, error) {
	if conv == nil || len(conv.Messages) == 0 {
		return &AutoCompactionResult{}, nil
	}

	total := 0
	for _, m := range conv.Messages {
		n, err := a.tokens.CountTokens(m.Content)
		if err != nil {
			return nil, fmt.Errorf("auto_compact: counting tokens: %w", err)
		}
		total += n
	}

	window := a.tokens.GetContextWindow()
	threshold := int(float64(window) * a.thresholdRatio)

	result := &AutoCompactionResult{
		TokensBefore:   total,
		TokensAfter:    total, // unchanged if no compaction occurs
		WindowSize:     window,
		ThresholdRatio: a.thresholdRatio,
	}

	if total < threshold {
		return result, nil
	}

	// Threshold crossed — check thrashing guard before compressing.
	if err := a.guard.RecordCompaction(); err != nil {
		return result, err
	}

	cr, err := a.coord.Compress(ctx, conv)
	if err != nil {
		return result, fmt.Errorf("auto_compact: compress: %w", err)
	}

	result.WasCompacted = true
	if cr != nil {
		// CompressionResult.TokensSaved is the delta; derive TokensAfter from it.
		result.TokensAfter = total - cr.TokensSaved
	}
	return result, nil
}
