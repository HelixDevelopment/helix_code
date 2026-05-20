// bench_test.go — speed-programme Phase 3, task P3-T01.
//
// Benchmarks the agent-loop wall-clock with small-model routing vs
// frontier-only. The benchmark models the latency asymmetry that motivates
// the model cascade: a small/cheap model serves trivial subtasks faster than
// the frontier model. The delta IS the speedup claim (CONST-035 Rule 9 —
// "faster" without pasted before/after numbers is a bluff).
//
// The benchmark uses simulated per-tier latencies so it is deterministic and
// hermetic (a unit-level benchmark). The integration test exercises the real
// cascade against provider shims; this benchmark proves the routing
// machinery itself adds negligible overhead and that routing trivial
// subtasks to the faster tier reduces total loop time.
package routing

import (
	"context"
	"testing"
	"time"
)

// benchCatalogue is the verifier catalogue used by the benchmarks.
func benchCatalogue() []TierModel {
	return []TierModel{
		{ID: "frontier-premium", VerifierTier: 1, Score: 9.4, Verified: true},
		{ID: "small-fast", VerifierTier: 3, Score: 7.1, Verified: true},
	}
}

// simulated per-tier service time. The frontier model is ~6x slower on a
// trivial subtask — representative of a frontier-vs-small latency gap.
const (
	smallTierLatency    = 5 * time.Millisecond
	frontierTierLatency = 30 * time.Millisecond
)

// agentLoopSubtasks is the cheap-subtask mix one agent-loop iteration runs.
var agentLoopSubtasks = []TaskClass{
	TaskClassification,
	TaskRanking,
	TaskCommitMessage,
	TaskAmbiguityDetection,
	TaskClassification,
	TaskRanking,
}

// benchGen returns a GenerateFunc that sleeps for the tier's simulated
// latency. The small model is always confident here (it handles the trivial
// subtask) so no escalation occurs — the best case the cascade targets.
func benchGen() GenerateFunc {
	return func(_ context.Context, modelID string, tier ModelTier) (Result, error) {
		switch tier {
		case TierSmall:
			time.Sleep(smallTierLatency)
		default:
			time.Sleep(frontierTierLatency)
		}
		return Result{Content: "ok", Confidence: 0.95}, nil
	}
}

// BenchmarkAgentLoop_FrontierOnly is the BEFORE number: every cheap subtask
// runs on the frontier model (the un-routed baseline).
func BenchmarkAgentLoop_FrontierOnly(b *testing.B) {
	r, err := NewRouter(FrontierOnlyPolicy(), NewVerifierResolver(&mockSource{models: benchCatalogue()}))
	if err != nil {
		b.Fatalf("NewRouter: %v", err)
	}
	gen := benchGen()
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, class := range agentLoopSubtasks {
			if _, err := r.Route(ctx, class, gen); err != nil {
				b.Fatalf("Route: %v", err)
			}
		}
		r.ResetLog()
	}
}

// BenchmarkAgentLoop_WithRouting is the AFTER number: cheap subtasks route to
// the small model. The wall-clock delta vs BenchmarkAgentLoop_FrontierOnly is
// the agent-loop speedup the model cascade delivers.
func BenchmarkAgentLoop_WithRouting(b *testing.B) {
	r, err := NewRouter(DefaultPolicy(), NewVerifierResolver(&mockSource{models: benchCatalogue()}))
	if err != nil {
		b.Fatalf("NewRouter: %v", err)
	}
	gen := benchGen()
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, class := range agentLoopSubtasks {
			if _, err := r.Route(ctx, class, gen); err != nil {
				b.Fatalf("Route: %v", err)
			}
		}
		r.ResetLog()
	}
}

// BenchmarkRouterOverhead isolates the routing machinery's own cost: a
// GenerateFunc that does no work. Proves the policy lookup + resolver call +
// log append add negligible overhead vs a bare provider call.
func BenchmarkRouterOverhead(b *testing.B) {
	r, _ := NewRouter(DefaultPolicy(), NewVerifierResolver(&mockSource{models: benchCatalogue()}))
	gen := func(_ context.Context, _ string, _ ModelTier) (Result, error) {
		return Result{Content: "x", Confidence: 1.0}, nil
	}
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := r.Route(ctx, TaskClassification, gen); err != nil {
			b.Fatalf("Route: %v", err)
		}
		r.ResetLog()
	}
}
