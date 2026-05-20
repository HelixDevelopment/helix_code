package session

// Benchmarks for history compaction (speed programme P3-T05).
//
// BenchmarkLongRunTurn_NoCompaction vs BenchmarkLongRunTurn_WithCompaction
// measure per-turn cost over a long autonomous run. The point P3-T05 makes is
// NOT that compaction makes a single turn faster — it makes the WINDOW
// bounded, which keeps every downstream cost (token counting, serialisation,
// prompt-cache prefix hashing, the provider request itself) bounded too. The
// no-compaction benchmark's history grows without limit; the with-compaction
// benchmark's history stays bounded. The captured allocs/op + ns/op delta is
// the anti-bluff proof of that boundedness.

import (
	stdctx "context"
	"strings"
	"testing"
)

// benchTurn returns a synthetic agent turn of roughly the given token size.
func benchTurn(approxTokens int) Message {
	return Message{Role: "assistant", Content: "agent step output " + strings.Repeat("z", approxTokens*4)}
}

// BenchmarkLongRunTurn_NoCompaction simulates a long run with NO compaction:
// the history grows unbounded, so each turn's per-turn processing cost
// (token counting over the whole history) grows linearly.
func BenchmarkLongRunTurn_NoCompaction(b *testing.B) {
	ctx := stdctx.Background()
	cfg := HistoryCompactorConfig{Enabled: false} // compaction OFF
	hc := NewHistoryCompactor(cfg, nil, nil)

	history := representativeSession()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		history = append(history, benchTurn(600))
		// CompactIfNeeded is a no-op here, but we still measure the cost of the
		// per-turn token accounting the agent loop performs.
		history, _ = hc.CompactIfNeeded(ctx, history)
	}
	b.StopTimer()
	totalTokens := 0
	for _, m := range history {
		totalTokens += len(m.Content) / 4
	}
	b.Logf("no-compaction: final history %d turns, ~%d tokens (UNBOUNDED)", len(history), totalTokens)
}

// BenchmarkLongRunTurn_WithCompaction simulates the same long run WITH
// threshold-triggered compaction: the history stays bounded, so per-turn cost
// stays bounded too.
func BenchmarkLongRunTurn_WithCompaction(b *testing.B) {
	ctx := stdctx.Background()
	cfg := HistoryCompactorConfig{Enabled: true, TokenThreshold: 12000, KeepRecentTurns: 4}
	hc := NewHistoryCompactor(cfg, nil, fakeSummarizer{})

	history := representativeSession()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		history = append(history, benchTurn(600))
		history, _ = hc.CompactIfNeeded(ctx, history)
	}
	b.StopTimer()
	totalTokens := 0
	for _, m := range history {
		totalTokens += len(m.Content) / 4
	}
	b.Logf("with-compaction: final history %d turns, ~%d tokens (BOUNDED)", len(history), totalTokens)
}

// BenchmarkCompactionCall measures the cost of one compaction operation on a
// large history — the one-time cost paid each time the threshold is crossed.
func BenchmarkCompactionCall(b *testing.B) {
	ctx := stdctx.Background()
	cfg := HistoryCompactorConfig{Enabled: true, TokenThreshold: 1, KeepRecentTurns: 4}
	hc := NewHistoryCompactor(cfg, nil, fakeSummarizer{})
	history := representativeSession()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = hc.Compact(ctx, history)
	}
}
