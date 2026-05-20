// routing_test.go — speed-programme Phase 3, task P3-T01.
//
// Unit tests for the small-model routing policy + cascade router. Mocks are
// permitted here per CONST-050(A) — these are unit tests. The verifier model
// catalogue is supplied by a mock VerifiedModelSource; the LLM calls are
// supplied by a mock GenerateFunc.
//
// Anti-bluff core (CONST-035): TestQualityParity_RoutedMatchesFrontierOnly
// asserts that a representative cheap-subtask set produces IDENTICAL output
// whether routed or run frontier-only — routing that silently degraded
// quality would fail that test.
package routing

import (
	"context"
	"errors"
	"testing"
)

// mockSource is a unit-test VerifiedModelSource — a fixed verifier catalogue.
type mockSource struct {
	models []TierModel
	err    error
	calls  int
}

func (m *mockSource) VerifiedModels(_ context.Context) ([]TierModel, error) {
	m.calls++
	if m.err != nil {
		return nil, m.err
	}
	return m.models, nil
}

// catalogue is a representative verifier catalogue: small (Tier 3/5) and
// frontier (Tier 1/2) models, plus an unverified and a deprecated model that
// MUST be ignored.
func catalogue() []TierModel {
	return []TierModel{
		{ID: "frontier-premium", VerifierTier: 1, Score: 9.4, Verified: true},
		{ID: "frontier-high", VerifierTier: 2, Score: 8.6, Verified: true},
		{ID: "small-fast", VerifierTier: 3, Score: 7.1, Verified: true},
		{ID: "small-free", VerifierTier: 5, Score: 6.2, Verified: true},
		{ID: "unverified-model", VerifierTier: 1, Score: 9.9, Verified: false},
		{ID: "deprecated-model", VerifierTier: 1, Score: 9.8, Verified: true, Deprecated: true},
	}
}

// TestPolicy_PicksSmallTierForTrivialClasses asserts the policy routes every
// trivial subtask class to the small tier and reasoning to the frontier tier.
func TestPolicy_PicksSmallTierForTrivialClasses(t *testing.T) {
	p := DefaultPolicy()
	trivial := []TaskClass{
		TaskClassification, TaskRanking, TaskCommitMessage, TaskAmbiguityDetection,
	}
	for _, c := range trivial {
		if got := p.InitialTier(c); got != TierSmall {
			t.Errorf("trivial class %s: InitialTier = %s, want small", c, got)
		}
	}
	if got := p.InitialTier(TaskReasoning); got != TierFrontier {
		t.Errorf("reasoning class: InitialTier = %s, want frontier", got)
	}
	// Unknown class must conservatively route frontier (never downgrade).
	if got := p.InitialTier(TaskClass("unknown")); got != TierFrontier {
		t.Errorf("unknown class: InitialTier = %s, want frontier", got)
	}
}

// TestPolicy_ForceFrontierOverridesEverything asserts the config-gated
// frontier-only switch routes every class to the frontier tier — the
// no-regression safety valve.
func TestPolicy_ForceFrontierOverridesEverything(t *testing.T) {
	p := FrontierOnlyPolicy()
	for _, c := range []TaskClass{TaskClassification, TaskRanking, TaskCommitMessage, TaskAmbiguityDetection, TaskReasoning} {
		if got := p.InitialTier(c); got != TierFrontier {
			t.Errorf("force-frontier class %s: InitialTier = %s, want frontier", c, got)
		}
	}
	// ShouldEscalate must be false under force-frontier (already on frontier).
	if p.ShouldEscalate(Result{Tier: TierSmall, Confidence: 0.0}) {
		t.Error("force-frontier policy must never escalate")
	}
}

// TestPolicy_EscalationThreshold asserts the escalate-on-low-confidence rule.
func TestPolicy_EscalationThreshold(t *testing.T) {
	p := NewPolicy(WithEscalationThreshold(0.7))
	cases := []struct {
		conf       float64
		tier       ModelTier
		wantEscala bool
	}{
		{conf: 0.69, tier: TierSmall, wantEscala: true},   // below threshold
		{conf: 0.70, tier: TierSmall, wantEscala: false},  // exactly at threshold
		{conf: 0.95, tier: TierSmall, wantEscala: false},  // confident
		{conf: 0.10, tier: TierFrontier, wantEscala: false}, // already frontier
	}
	for _, c := range cases {
		got := p.ShouldEscalate(Result{Tier: c.tier, Confidence: c.conf})
		if got != c.wantEscala {
			t.Errorf("ShouldEscalate(conf=%.2f tier=%s) = %v, want %v", c.conf, c.tier, got, c.wantEscala)
		}
	}
}

// TestVerifierResolver_SelectsByTier asserts the resolver picks the
// highest-scored verified, non-deprecated model in each tier's verifier-tier
// set, sourcing the model list ENTIRELY from verifier metadata (CONST-036/037).
func TestVerifierResolver_SelectsByTier(t *testing.T) {
	src := &mockSource{models: catalogue()}
	r := NewVerifierResolver(src)

	small, err := r.ResolveModel(context.Background(), TierSmall)
	if err != nil {
		t.Fatalf("ResolveModel(small): %v", err)
	}
	if small != "small-fast" {
		t.Errorf("small tier resolved %q, want small-fast (highest-scored Tier 3/5)", small)
	}

	frontier, err := r.ResolveModel(context.Background(), TierFrontier)
	if err != nil {
		t.Fatalf("ResolveModel(frontier): %v", err)
	}
	if frontier != "frontier-premium" {
		t.Errorf("frontier tier resolved %q, want frontier-premium (highest-scored Tier 1/2)", frontier)
	}
}

// TestVerifierResolver_IgnoresUnverifiedAndDeprecated asserts the higher-scored
// unverified-model and deprecated-model are NOT selected even though they
// out-score the verified models — CONST-037 (verifier-checked only).
func TestVerifierResolver_IgnoresUnverifiedAndDeprecated(t *testing.T) {
	src := &mockSource{models: catalogue()}
	r := NewVerifierResolver(src)
	frontier, err := r.ResolveModel(context.Background(), TierFrontier)
	if err != nil {
		t.Fatalf("ResolveModel(frontier): %v", err)
	}
	if frontier == "unverified-model" || frontier == "deprecated-model" {
		t.Errorf("resolver picked ineligible model %q", frontier)
	}
}

// TestVerifierResolver_PropagatesSourceError asserts a verifier-source error
// is surfaced, not swallowed.
func TestVerifierResolver_PropagatesSourceError(t *testing.T) {
	src := &mockSource{err: errors.New("verifier unreachable")}
	r := NewVerifierResolver(src)
	if _, err := r.ResolveModel(context.Background(), TierSmall); err == nil {
		t.Error("ResolveModel should propagate verifier-source error")
	}
}

// TestRouter_RoutesTrivialToSmall asserts a trivial subtask runs on the small
// tier when the small model is confident — no escalation.
func TestRouter_RoutesTrivialToSmall(t *testing.T) {
	r, err := NewRouter(DefaultPolicy(), NewVerifierResolver(&mockSource{models: catalogue()}))
	if err != nil {
		t.Fatalf("NewRouter: %v", err)
	}
	gen := func(_ context.Context, modelID string, tier ModelTier) (Result, error) {
		return Result{Content: "trivial-answer", Confidence: 0.95}, nil
	}
	res, err := r.Route(context.Background(), TaskClassification, gen)
	if err != nil {
		t.Fatalf("Route: %v", err)
	}
	if res.Tier != TierSmall {
		t.Errorf("trivial confident subtask ran on %s, want small", res.Tier)
	}
	if res.ModelID != "small-fast" {
		t.Errorf("trivial subtask used model %q, want small-fast", res.ModelID)
	}
	if res.Escalated {
		t.Error("confident small result must not escalate")
	}
}

// TestRouter_RoutesReasoningToFrontier asserts a reasoning-heavy subtask
// goes straight to the frontier tier — no quality risk.
func TestRouter_RoutesReasoningToFrontier(t *testing.T) {
	r, _ := NewRouter(DefaultPolicy(), NewVerifierResolver(&mockSource{models: catalogue()}))
	gen := func(_ context.Context, modelID string, tier ModelTier) (Result, error) {
		return Result{Content: "deep-answer", Confidence: 0.99}, nil
	}
	res, err := r.Route(context.Background(), TaskReasoning, gen)
	if err != nil {
		t.Fatalf("Route: %v", err)
	}
	if res.Tier != TierFrontier {
		t.Errorf("reasoning subtask ran on %s, want frontier", res.Tier)
	}
}

// TestRouter_EscalatesLowConfidenceToFrontier is the core escalation test:
// a low-confidence small-model result MUST escalate to the frontier model.
func TestRouter_EscalatesLowConfidenceToFrontier(t *testing.T) {
	r, _ := NewRouter(DefaultPolicy(), NewVerifierResolver(&mockSource{models: catalogue()}))

	var seen []ModelTier
	gen := func(_ context.Context, modelID string, tier ModelTier) (Result, error) {
		seen = append(seen, tier)
		if tier == TierSmall {
			return Result{Content: "uncertain", Confidence: 0.3}, nil // low confidence
		}
		return Result{Content: "frontier-quality-answer", Confidence: 1.0}, nil
	}
	res, err := r.Route(context.Background(), TaskRanking, gen)
	if err != nil {
		t.Fatalf("Route: %v", err)
	}
	if res.Tier != TierFrontier {
		t.Errorf("escalated result tier = %s, want frontier", res.Tier)
	}
	if !res.Escalated {
		t.Error("low-confidence small result must be marked Escalated")
	}
	if res.Content != "frontier-quality-answer" {
		t.Errorf("escalated content = %q, want frontier answer", res.Content)
	}
	if len(seen) != 2 || seen[0] != TierSmall || seen[1] != TierFrontier {
		t.Errorf("escalation call order = %v, want [small frontier]", seen)
	}
}

// TestRouter_SmallTierResolutionFailureFallsThroughToFrontier asserts that a
// missing cheap model never blocks a subtask — it falls through to frontier.
func TestRouter_SmallTierResolutionFailureFallsThroughToFrontier(t *testing.T) {
	// Catalogue with only frontier models — small tier cannot resolve to a
	// Tier 3/5 model, but the resolver's fallback still yields a frontier
	// model. Verify the subtask still completes.
	frontierOnly := []TierModel{
		{ID: "frontier-premium", VerifierTier: 1, Score: 9.4, Verified: true},
	}
	r, _ := NewRouter(DefaultPolicy(), NewVerifierResolver(&mockSource{models: frontierOnly}))
	gen := func(_ context.Context, modelID string, tier ModelTier) (Result, error) {
		return Result{Content: "answer", Confidence: 0.99}, nil
	}
	res, err := r.Route(context.Background(), TaskClassification, gen)
	if err != nil {
		t.Fatalf("Route should not fail when small tier is unavailable: %v", err)
	}
	if res.ModelID != "frontier-premium" {
		t.Errorf("fallback model = %q, want frontier-premium", res.ModelID)
	}
}

// TestRouter_ForceFrontierDisablesRouting asserts the config-gated switch
// sends every subtask to the frontier model — the no-regression mode.
func TestRouter_ForceFrontierDisablesRouting(t *testing.T) {
	r, _ := NewRouter(FrontierOnlyPolicy(), NewVerifierResolver(&mockSource{models: catalogue()}))
	gen := func(_ context.Context, modelID string, tier ModelTier) (Result, error) {
		return Result{Content: "x", Confidence: 0.1}, nil // would escalate if routed
	}
	for _, c := range []TaskClass{TaskClassification, TaskRanking, TaskCommitMessage, TaskAmbiguityDetection} {
		res, err := r.Route(context.Background(), c, gen)
		if err != nil {
			t.Fatalf("Route(%s): %v", c, err)
		}
		if res.Tier != TierFrontier {
			t.Errorf("force-frontier: class %s ran on %s, want frontier", c, res.Tier)
		}
		if res.Escalated {
			t.Errorf("force-frontier: class %s must not record an escalation", c)
		}
	}
}

// TestRouter_LogCapturesPerSubtaskModel asserts the routing log captures the
// per-subtask model-used evidence — the anti-bluff proof artefact.
func TestRouter_LogCapturesPerSubtaskModel(t *testing.T) {
	r, _ := NewRouter(DefaultPolicy(), NewVerifierResolver(&mockSource{models: catalogue()}))
	gen := func(_ context.Context, modelID string, tier ModelTier) (Result, error) {
		if tier == TierSmall {
			return Result{Content: "lo", Confidence: 0.2}, nil
		}
		return Result{Content: "hi", Confidence: 1.0}, nil
	}
	if _, err := r.Route(context.Background(), TaskClassification, gen); err != nil {
		t.Fatalf("Route: %v", err)
	}
	log := r.Log()
	if len(log) != 2 {
		t.Fatalf("routing log has %d entries, want 2 (small attempt + frontier escalation)", len(log))
	}
	if log[0].Tier != TierSmall || log[0].ModelID != "small-fast" {
		t.Errorf("log[0] = %+v, want small/small-fast", log[0])
	}
	if log[1].Tier != TierFrontier || !log[1].Escalated {
		t.Errorf("log[1] = %+v, want frontier escalated", log[1])
	}
	stats := r.Stats()
	if stats.Total != 2 || stats.SmallTier != 1 || stats.FrontierTier != 1 || stats.Escalations != 1 {
		t.Errorf("Stats = %+v, want Total=2 Small=1 Frontier=1 Escalations=1", stats)
	}
}

// TestRouter_NilGuards asserts the constructor and Route reject nil inputs.
func TestRouter_NilGuards(t *testing.T) {
	if _, err := NewRouter(DefaultPolicy(), nil); !errors.Is(err, ErrNilResolver) {
		t.Errorf("NewRouter(nil resolver) err = %v, want ErrNilResolver", err)
	}
	r, _ := NewRouter(DefaultPolicy(), NewVerifierResolver(&mockSource{models: catalogue()}))
	if _, err := r.Route(context.Background(), TaskClassification, nil); !errors.Is(err, ErrNilGenerateFunc) {
		t.Errorf("Route(nil gen) err = %v, want ErrNilGenerateFunc", err)
	}
}

// TestQualityParity_RoutedMatchesFrontierOnly is the anti-bluff core of
// P3-T01 (CONST-035): for a representative cheap-subtask set, the routed
// output MUST match the frontier-only output. Routing that silently degraded
// quality is the failure mode — this test catches it.
//
// The model harness here returns a quality-graded answer: the small model
// gives a low-confidence answer for hard inputs, the frontier model always
// gives the reference answer. Because the router escalates every
// low-confidence small result, the routed final answer must equal the
// frontier-only answer for every subtask in the set.
func TestQualityParity_RoutedMatchesFrontierOnly(t *testing.T) {
	src := &mockSource{models: catalogue()}

	// Representative cheap-subtask set: (class, input, reference frontier
	// answer, whether the small model handles it confidently).
	type subtask struct {
		class     TaskClass
		input     string
		reference string
		smallOK   bool // true: small model is confident & correct
	}
	set := []subtask{
		{TaskClassification, "is this a bug report?", "bug", true},
		{TaskClassification, "subtle multi-intent prompt", "feature+bug", false},
		{TaskRanking, "rank 3 obvious options", "1,2,3", true},
		{TaskRanking, "rank 12 near-tied options", "ref-ranking", false},
		{TaskCommitMessage, "one-line trivial diff", "fix: typo", true},
		{TaskCommitMessage, "sprawling refactor diff", "refactor: split module", false},
		{TaskAmbiguityDetection, "clearly specified prompt", "no-clarification", true},
		{TaskAmbiguityDetection, "vague underspecified prompt", "ask: which file?", false},
	}

	// frontierOnlyAnswer simulates the un-routed path: every subtask always
	// runs on the frontier model and returns the reference answer.
	frontierOnlyAnswer := func(s subtask) string { return s.reference }

	// routedAnswer simulates the routed path. The small model returns the
	// reference answer with high confidence only when smallOK; otherwise it
	// returns a degraded answer with low confidence, which the router MUST
	// escalate to the frontier model (which returns the reference answer).
	routedAnswer := func(t *testing.T, s subtask) (string, bool) {
		r, _ := NewRouter(DefaultPolicy(), NewVerifierResolver(src))
		gen := func(_ context.Context, modelID string, tier ModelTier) (Result, error) {
			if tier == TierSmall {
				if s.smallOK {
					return Result{Content: s.reference, Confidence: 0.92}, nil
				}
				// degraded small-model answer — must NOT be the final result
				return Result{Content: "DEGRADED:" + s.input, Confidence: 0.25}, nil
			}
			// frontier always returns the reference answer
			return Result{Content: s.reference, Confidence: 1.0}, nil
		}
		res, err := r.Route(context.Background(), s.class, gen)
		if err != nil {
			t.Fatalf("routed Route(%s): %v", s.class, err)
		}
		return res.Content, res.Escalated
	}

	escalations := 0
	for _, s := range set {
		want := frontierOnlyAnswer(s)
		got, escalated := routedAnswer(t, s)
		if got != want {
			t.Errorf("QUALITY REGRESSION for %s/%q: routed=%q frontier-only=%q",
				s.class, s.input, got, want)
		}
		if got == "DEGRADED:"+s.input {
			t.Errorf("QUALITY BLUFF: degraded small-model output reached the final result for %q", s.input)
		}
		if !s.smallOK && !escalated {
			t.Errorf("hard subtask %q did not escalate — quality not guaranteed", s.input)
		}
		if escalated {
			escalations++
		}
	}

	// Half the representative set is hard; all hard subtasks must have
	// escalated. This proves the cascade actually exercised both paths.
	if escalations != 4 {
		t.Errorf("expected 4 escalations for the 4 hard subtasks, got %d", escalations)
	}
	t.Logf("quality-parity: %d/%d subtasks routed, %d escalated to frontier, "+
		"routed output == frontier-only output for ALL subtasks", len(set), len(set), escalations)
}
