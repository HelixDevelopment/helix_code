package compression

import (
	"context"
	"fmt"
	"sync"

	"dev.helix.code/internal/hooks"
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
	mu             sync.Mutex
	hooksManager   *hooks.Manager
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

// SetHooksManager wires a hooks.Manager so MaybeCompact can fire
// HookTypeOnCompaction after a successful compaction. A nil manager disables
// hook firing. Blocking hooks surface as MaybeCompact's return error so the
// caller can react.
func (a *AutoCompactor) SetHooksManager(m *hooks.Manager) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.hooksManager = m
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

	// Fire OnCompaction; a blocking hook surfaces as the function's error.
	a.mu.Lock()
	mgr := a.hooksManager
	a.mu.Unlock()
	if mgr != nil {
		event := hooks.NewEventWithContext(ctx, hooks.HookTypeOnCompaction)
		event.Source = "auto_compactor"
		event.SetData("before_size", result.TokensBefore)
		event.SetData("after_size", result.TokensAfter)
		event.SetData("messages_compacted", len(conv.Messages))
		results := mgr.TriggerEventAndWait(event)
		if blockers := hooks.Blockers(results); len(blockers) > 0 {
			return result, fmt.Errorf("compaction blocked by hook(s): %v", blockers[0])
		}
	}

	return result, nil
}
