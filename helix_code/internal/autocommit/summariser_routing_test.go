// summariser_routing_test.go — speed-programme Phase 3, task P3-T01.
//
// Unit tests for the small-model routing wiring in LLMSummariser. Mocks are
// permitted here per CONST-050(A) — these are unit tests. The verifier model
// catalogue is supplied by an in-package mock VerifiedModelSource; the
// llm.Provider is the existing in-package fakeProvider.
package autocommit

import (
	"context"
	"testing"

	"dev.helix.code/internal/llm"
	"dev.helix.code/internal/llm/routing"
	"github.com/stretchr/testify/require"
)

// routingTestCatalogue is a small verifier catalogue: one small-tier and one
// frontier-tier model. The routing model list comes ENTIRELY from this
// verifier-style metadata — never hardcoded (CONST-036/037).
type routingTestSource struct{}

func (routingTestSource) VerifiedModels(_ context.Context) ([]routing.TierModel, error) {
	return []routing.TierModel{
		{ID: "frontier-premium", VerifierTier: 1, Score: 9.4, Verified: true},
		{ID: "small-fast", VerifierTier: 3, Score: 7.1, Verified: true},
	}, nil
}

// finishReasonProvider is a fakeProvider variant that records the model it
// was asked to generate with and returns a configurable finish reason — so
// the test can drive the escalate-on-low-confidence path.
type finishReasonProvider struct {
	modelsSeen   []string
	finishBySmall string // finish reason returned when small-fast model is used
	contentSmall  string
	contentFront  string
}

func (p *finishReasonProvider) GetType() llm.ProviderType { return "fake" }
func (p *finishReasonProvider) GetName() string           { return "fake" }
func (p *finishReasonProvider) GetModels() []llm.ModelInfo {
	return []llm.ModelInfo{{ID: "default-model", Name: "default", ContextSize: 4096}}
}
func (p *finishReasonProvider) GetCapabilities() []llm.ModelCapability { return nil }
func (p *finishReasonProvider) Generate(_ context.Context, req *llm.LLMRequest) (*llm.LLMResponse, error) {
	p.modelsSeen = append(p.modelsSeen, req.Model)
	if req.Model == "small-fast" {
		return &llm.LLMResponse{Content: p.contentSmall, FinishReason: p.finishBySmall}, nil
	}
	return &llm.LLMResponse{Content: p.contentFront, FinishReason: "stop"}, nil
}
func (p *finishReasonProvider) GenerateStream(_ context.Context, _ *llm.LLMRequest, _ chan<- llm.LLMResponse) error {
	return nil
}
func (p *finishReasonProvider) IsAvailable(_ context.Context) bool { return true }
func (p *finishReasonProvider) GetHealth(_ context.Context) (*llm.ProviderHealth, error) {
	return &llm.ProviderHealth{Status: "ok"}, nil
}
func (p *finishReasonProvider) Close() error                      { return nil }
func (p *finishReasonProvider) GetContextWindow() int             { return 4096 }
func (p *finishReasonProvider) CountTokens(s string) (int, error) { return len(s) / 4, nil }

func newTestRouter(t *testing.T, policy *routing.Policy) *routing.Router {
	t.Helper()
	r, err := routing.NewRouter(policy, routing.NewVerifierResolver(routingTestSource{}))
	require.NoError(t, err)
	return r
}

// TestRoutedSummariser_TrivialCommitMsgRoutesToSmallModel asserts that a
// commit-message subtask with a confident small-model response is served by
// the small/cheap model — no escalation, no frontier call.
func TestRoutedSummariser_TrivialCommitMsgRoutesToSmallModel(t *testing.T) {
	p := &finishReasonProvider{
		finishBySmall: "stop", // confident
		contentSmall:  "fix: trivial typo",
		contentFront:  "frontier answer",
	}
	s := NewRoutedSummariser(p, newTestRouter(t, routing.DefaultPolicy()))
	got := s.Summarise(context.Background(), "diff body", "fs_edit", []string{"x.go"})
	require.Equal(t, "fix: trivial typo", got)
	require.Equal(t, []string{"small-fast"}, p.modelsSeen,
		"trivial confident commit-msg subtask must run only on the small model")
}

// TestRoutedSummariser_LowConfidenceEscalatesToFrontier asserts that a
// truncated small-model commit-message escalates to the frontier model — the
// final commit subject comes from the frontier model (no quality regression).
func TestRoutedSummariser_LowConfidenceEscalatesToFrontier(t *testing.T) {
	p := &finishReasonProvider{
		finishBySmall: "length", // truncated → low confidence → escalate
		contentSmall:  "partial degr",
		contentFront:  "refactor: split oversized module",
	}
	s := NewRoutedSummariser(p, newTestRouter(t, routing.DefaultPolicy()))
	got := s.Summarise(context.Background(), "sprawling diff", "fs_edit", []string{"big.go"})
	require.Equal(t, "refactor: split oversized module", got,
		"low-confidence small result must escalate; final subject from the frontier model")
	require.Equal(t, []string{"small-fast", "frontier-premium"}, p.modelsSeen,
		"escalation must call small then frontier")
}

// TestRoutedSummariser_ForceFrontierDisablesRouting asserts the config-gated
// frontier-only policy sends commit-message generation straight to the
// frontier model — the no-regression switch.
func TestRoutedSummariser_ForceFrontierDisablesRouting(t *testing.T) {
	p := &finishReasonProvider{
		finishBySmall: "length", // would escalate if routed small
		contentSmall:  "small",
		contentFront:  "frontier-only answer",
	}
	s := NewRoutedSummariser(p, newTestRouter(t, routing.FrontierOnlyPolicy()))
	got := s.Summarise(context.Background(), "diff", "fs_edit", []string{"x.go"})
	require.Equal(t, "frontier-only answer", got)
	require.Equal(t, []string{"frontier-premium"}, p.modelsSeen,
		"force-frontier policy must route every commit-msg subtask to the frontier model")
}

// TestRoutedSummariser_NilRouterBehavesLikePlainSummariser asserts that a nil
// router yields exactly the un-routed behaviour (no-regression default).
func TestRoutedSummariser_NilRouterBehavesLikePlainSummariser(t *testing.T) {
	p := &finishReasonProvider{contentFront: "x", contentSmall: "x"}
	s := NewRoutedSummariser(p, nil)
	got := s.Summarise(context.Background(), "diff", "fs_edit", []string{"x.go"})
	require.NotEmpty(t, got)
	// With a nil router the request runs on the provider's first model.
	require.Equal(t, []string{"default-model"}, p.modelsSeen)
}
