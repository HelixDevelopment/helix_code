package llm

import (
	"sync/atomic"
	"time"
)

// DefaultColdThreshold is the default time-since-last-completion threshold
// beyond which a prompt cache entry is considered "cold" (likely expired).
//
// Anthropic prompt-cache entries have a ~5-minute TTL. After this many
// seconds of inactivity the entry is almost certainly evicted on the
// provider side, so sending the same request pays the full uncached price.
// Other providers may need a different threshold via SetColdThreshold.
//
// Ported from gptme commit e896ed4ff (ANTHROPIC_CACHE_TTL_SECONDS = 300).
const DefaultColdThreshold = 5 * time.Minute

// CacheAwareness tracks the wall-clock time of the most recent successful
// LLM completion so callers can predict, BEFORE sending the next request,
// whether the provider's prompt cache entry is likely cold.
//
// The Anthropic prompt cache has an implicit TTL (~5 min). Without this
// signal, callers have no way to know whether the cache will hit until
// after the round-trip — by which point the cache-creation token cost has
// already been paid. CacheAwareness lets the request-build path decide
// whether to skip the explicit cache_control marker (saving a token and
// making block-priority decisions clearer) when the cache is predicted cold.
//
// Concurrency: the timestamp field is read/written via sync/atomic so
// callers may RecordCompletion from the response-handler goroutine while
// other goroutines call IsCacheLikelyCold or LastCompletionAt from the
// request-build path without explicit locking.
//
// Ported from gptme commit e896ed4ff (CacheState.last_call_completed_at
// + is_cache_likely_cold + get_elapsed_since_last_call).
type CacheAwareness struct {
	// lastCompletionUnixNano stores the timestamp of the most recent
	// completion as UnixNano. Zero == no completion recorded yet.
	// Read/written atomically.
	lastCompletionUnixNano atomic.Int64

	// coldThresholdNanos stores the cold-threshold duration in nanoseconds.
	// Read/written atomically so SetColdThreshold is safe to call from any
	// goroutine without blocking IsCacheLikelyCold.
	coldThresholdNanos atomic.Int64
}

// NewCacheAwareness returns a CacheAwareness with no completion recorded and
// the default 5-minute cold threshold (DefaultColdThreshold).
func NewCacheAwareness() *CacheAwareness {
	ca := &CacheAwareness{}
	ca.coldThresholdNanos.Store(int64(DefaultColdThreshold))
	return ca
}

// RecordCompletion stores t as the most recent completion time. Call this
// from the LLM response-handler path after a successful provider call.
// Subsequent invocations overwrite the prior value (most-recent-wins).
func (c *CacheAwareness) RecordCompletion(t time.Time) {
	c.lastCompletionUnixNano.Store(t.UnixNano())
}

// LastCompletionAt returns the wall-clock time of the most recent recorded
// completion, or a zero time.Time if no completion has been recorded.
func (c *CacheAwareness) LastCompletionAt() time.Time {
	n := c.lastCompletionUnixNano.Load()
	if n == 0 {
		return time.Time{}
	}
	return time.Unix(0, n)
}

// IsCacheLikelyCold reports whether the prompt cache is likely cold relative
// to the supplied "now" timestamp. It returns true when:
//
//   - no completion has been recorded yet (conservative: assume cold so the
//     caller doesn't burn a cache_creation token marker speculatively), OR
//   - the elapsed time since the last recorded completion is greater than
//     the configured cold threshold.
//
// The caller supplies "now" explicitly to keep this function deterministic
// and testable (no hidden time.Now() call).
//
// Note: the default threshold (DefaultColdThreshold = 5 min) is calibrated
// for Anthropic. For non-Anthropic providers, call SetColdThreshold first
// or treat the result as advisory if the provider has no prompt cache.
func (c *CacheAwareness) IsCacheLikelyCold(now time.Time) bool {
	last := c.LastCompletionAt()
	if last.IsZero() {
		return true
	}
	threshold := time.Duration(c.coldThresholdNanos.Load())
	return now.Sub(last) > threshold
}

// SetColdThreshold replaces the cold-threshold duration. Safe to call from
// any goroutine; subsequent IsCacheLikelyCold calls will use the new value.
//
// Pass DefaultColdThreshold to restore Anthropic's 5-minute default.
func (c *CacheAwareness) SetColdThreshold(d time.Duration) {
	c.coldThresholdNanos.Store(int64(d))
}

// ColdThreshold returns the current cold-threshold duration.
func (c *CacheAwareness) ColdThreshold() time.Duration {
	return time.Duration(c.coldThresholdNanos.Load())
}

// CacheControl represents cache control directives for LLM providers
// Currently supported by Anthropic for prompt caching to reduce costs
type CacheControl struct {
	Type string `json:"type"` // "ephemeral" for Anthropic
}

// CacheableMessage represents a message that can be cached
type CacheableMessage struct {
	Message
	CacheControl *CacheControl `json:"cache_control,omitempty"`
}

// CacheStats tracks cache hit/miss statistics
type CacheStats struct {
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
}

// CacheStrategy defines when and what to cache
type CacheStrategy string

const (
	// CacheStrategyNone disables caching
	CacheStrategyNone CacheStrategy = "none"

	// CacheStrategySystem caches only system messages
	CacheStrategySystem CacheStrategy = "system"

	// CacheStrategyTools caches system messages and tool definitions
	CacheStrategyTools CacheStrategy = "tools"

	// CacheStrategyContext caches system, tools, and context prefix
	CacheStrategyContext CacheStrategy = "context"

	// CacheStrategyAggressive caches everything possible
	CacheStrategyAggressive CacheStrategy = "aggressive"
)

// CacheConfig holds configuration for prompt caching
type CacheConfig struct {
	// Enabled determines if caching is active
	Enabled bool `json:"enabled"`

	// Strategy determines what to cache
	Strategy CacheStrategy `json:"strategy"`

	// MinTokensForCache minimum tokens required to enable caching
	MinTokensForCache int `json:"min_tokens_for_cache"`

	// CacheTTL cache time-to-live in seconds (default: 300)
	CacheTTL int `json:"cache_ttl"`

	// ColdThreshold is the duration after which a prompt cache entry is
	// considered "cold" (likely expired on the provider side). Used by
	// CacheAwareness.IsCacheLikelyCold to predict implicit TTL-based expiry
	// before the next request. Defaults to DefaultColdThreshold (5 minutes,
	// Anthropic-aligned). Ported from gptme commit e896ed4ff.
	ColdThreshold time.Duration `json:"cold_threshold"`
}

// DefaultCacheConfig returns default caching configuration
func DefaultCacheConfig() CacheConfig {
	return CacheConfig{
		Enabled:           true,
		Strategy:          CacheStrategyTools,
		MinTokensForCache: 1024,                 // Anthropic's minimum for efficient caching
		CacheTTL:          300,                  // 5 minutes
		ColdThreshold:     DefaultColdThreshold, // 5 minutes — Anthropic prompt-cache TTL
	}
}

// ApplyCacheControl applies cache control to messages based on strategy
func ApplyCacheControl(messages []Message, tools []Tool, config CacheConfig) []CacheableMessage {
	if !config.Enabled || config.Strategy == CacheStrategyNone {
		return convertToCacheable(messages)
	}

	cacheableMessages := make([]CacheableMessage, len(messages))

	for i, msg := range messages {
		cacheableMessages[i] = CacheableMessage{
			Message: msg,
		}

		// Apply caching based on strategy
		switch config.Strategy {
		case CacheStrategySystem:
			// Cache only system message
			if i == 0 && msg.Role == "system" {
				cacheableMessages[i].CacheControl = &CacheControl{Type: "ephemeral"}
			}

		case CacheStrategyTools:
			// Cache system message if tools are present
			if i == 0 && msg.Role == "system" && len(tools) > 0 {
				cacheableMessages[i].CacheControl = &CacheControl{Type: "ephemeral"}
			}

		case CacheStrategyContext:
			// Cache system message and recent context
			if i == 0 && msg.Role == "system" {
				cacheableMessages[i].CacheControl = &CacheControl{Type: "ephemeral"}
			}
			// Cache the last few user messages to preserve context
			if msg.Role == "user" && i >= len(messages)-3 {
				cacheableMessages[i].CacheControl = &CacheControl{Type: "ephemeral"}
			}

		case CacheStrategyAggressive:
			// Cache system message
			if i == 0 && msg.Role == "system" {
				cacheableMessages[i].CacheControl = &CacheControl{Type: "ephemeral"}
			}
			// Cache all user messages (for multi-turn conversations)
			if msg.Role == "user" {
				cacheableMessages[i].CacheControl = &CacheControl{Type: "ephemeral"}
			}
		}
	}

	return cacheableMessages
}

// convertToCacheable converts regular messages to cacheable messages without cache control
func convertToCacheable(messages []Message) []CacheableMessage {
	cacheable := make([]CacheableMessage, len(messages))
	for i, msg := range messages {
		cacheable[i] = CacheableMessage{Message: msg}
	}
	return cacheable
}

// CalculateCacheSavings calculates cost savings from caching
func CalculateCacheSavings(stats CacheStats, inputCostPer1K, cacheCostPer1K float64) CacheSavings {
	// Calculate tokens that would have been charged at full price
	fullPriceTokens := stats.InputTokens + stats.CacheCreationInputTokens

	// Calculate actual cost with caching
	cacheCost := float64(stats.CacheCreationInputTokens) / 1000.0 * inputCostPer1K
	cacheReadCost := float64(stats.CacheReadInputTokens) / 1000.0 * cacheCostPer1K
	regularCost := float64(stats.InputTokens-stats.CacheReadInputTokens) / 1000.0 * inputCostPer1K

	actualCost := cacheCost + cacheReadCost + regularCost

	// Calculate cost without caching
	costWithoutCache := float64(fullPriceTokens) / 1000.0 * inputCostPer1K

	// Calculate savings
	savings := costWithoutCache - actualCost
	savingsPercent := 0.0
	if costWithoutCache > 0 {
		savingsPercent = (savings / costWithoutCache) * 100.0
	}

	return CacheSavings{
		CostWithCache:    actualCost,
		CostWithoutCache: costWithoutCache,
		Savings:          savings,
		SavingsPercent:   savingsPercent,
		TokensCached:     stats.CacheCreationInputTokens,
		TokensRead:       stats.CacheReadInputTokens,
	}
}

// CacheSavings represents the cost savings from caching
type CacheSavings struct {
	CostWithCache    float64 `json:"cost_with_cache"`
	CostWithoutCache float64 `json:"cost_without_cache"`
	Savings          float64 `json:"savings"`
	SavingsPercent   float64 `json:"savings_percent"`
	TokensCached     int     `json:"tokens_cached"`
	TokensRead       int     `json:"tokens_read"`
}

// CacheMetrics tracks overall caching performance
type CacheMetrics struct {
	TotalRequests         int     `json:"total_requests"`
	RequestsWithCache     int     `json:"requests_with_cache"`
	CacheHitRate          float64 `json:"cache_hit_rate"`
	TotalTokensCached     int     `json:"total_tokens_cached"`
	TotalTokensRead       int     `json:"total_tokens_read"`
	TotalSavings          float64 `json:"total_savings"`
	AverageSavingsPercent float64 `json:"average_savings_percent"`
}

// UpdateMetrics updates cache metrics with new stats
func (cm *CacheMetrics) UpdateMetrics(stats CacheStats, savings CacheSavings) {
	cm.TotalRequests++

	if stats.CacheReadInputTokens > 0 || stats.CacheCreationInputTokens > 0 {
		cm.RequestsWithCache++
	}

	cm.TotalTokensCached += stats.CacheCreationInputTokens
	cm.TotalTokensRead += stats.CacheReadInputTokens
	cm.TotalSavings += savings.Savings

	// Update cache hit rate
	if cm.TotalRequests > 0 {
		cm.CacheHitRate = float64(cm.RequestsWithCache) / float64(cm.TotalRequests)
	}

	// Update average savings percent
	if cm.RequestsWithCache > 0 {
		cm.AverageSavingsPercent = (cm.AverageSavingsPercent*float64(cm.RequestsWithCache-1) + savings.SavingsPercent) / float64(cm.RequestsWithCache)
	}
}
