package llm

import (
	"context"
	"testing"
	"time"

	"dev.helix.code/internal/verifier"
)

// ensemble_resolver_test.go — unit tests for the DYNAMIC, verifier-driven
// per-member model resolver (CONST-036/040). Mocks/in-memory fixtures are
// permitted here per CONST-050(A) — this is a *_test.go unit source. The
// real-wire proof (a live ensemble fanning to >1 cloud provider with no
// all-fail) is captured separately via the gated probe + the TUI evidence.
//
// These tests assert the production resolution path picks a VERIFIED,
// chat-capable, highest-scored model and NEVER an embedding/deprecated/
// unverified one — and that the selection contains ZERO hardcoded model names
// (every id comes from the seeded verifier catalogue, mirroring how the live
// verifier feeds the resolver).

// newEnabledVerifierAdapterWithModels builds a REAL enabled verifier.Adapter
// whose catalogue is the given seed list, served deterministically from an
// in-memory cache (no network). It returns the adapter; the caller wires it via
// SetVerifierAdapter and restores the prior adapter on cleanup.
func newEnabledVerifierAdapterWithModels(t *testing.T, models []*verifier.VerifiedModel) *verifier.Adapter {
	t.Helper()
	cache := verifier.NewCache(10*time.Minute, nil)
	cache.SetModels("all", models)
	adapter := verifier.NewAdapter(nil, cache, nil, &verifier.AdapterConfig{Enabled: true})
	return adapter
}

// withVerifierAdapter sets the package-level verifier adapter for the duration
// of the test and restores the previous value afterwards (the adapter is a
// process-global injected by SetVerifierAdapter / verifier_bridge.go).
func withVerifierAdapter(t *testing.T, a *verifier.Adapter) {
	t.Helper()
	prev := verifierAdapter
	SetVerifierAdapter(a)
	t.Cleanup(func() { SetVerifierAdapter(prev) })
}

// TestEnsembleVerifiedModelFor_PicksBestVerifiedChatModel proves the resolver
// returns the highest-scored VERIFIED, non-deprecated, non-embedding model for
// the requested provider — sourced entirely from the verifier catalogue.
func TestEnsembleVerifiedModelFor_PicksBestVerifiedChatModel(t *testing.T) {
	seed := []*verifier.VerifiedModel{
		// Embedding model for groq — MUST be skipped (SupportsEmbeddings).
		{ID: "groq-embed-x", Provider: "groq", Verified: true, OverallScore: 9.9, SupportsEmbeddings: true},
		// Deprecated high-score groq model — MUST be skipped (Deprecated).
		{ID: "groq-old-decommissioned", Provider: "groq", Verified: true, OverallScore: 9.5, Deprecated: true},
		// Unverified groq model — MUST be skipped (!Verified).
		{ID: "groq-unverified", Provider: "groq", Verified: false, OverallScore: 9.4},
		// Two eligible groq chat models — highest score wins.
		{ID: "groq-chat-mid", Provider: "groq", Verified: true, OverallScore: 7.0},
		{ID: "groq-chat-best", Provider: "groq", Verified: true, OverallScore: 8.8},
		// A different provider's best — MUST NOT be selected for groq.
		{ID: "deepseek-chat-best", Provider: "deepseek", Verified: true, OverallScore: 9.7},
	}
	withVerifierAdapter(t, newEnabledVerifierAdapterWithModels(t, seed))

	got := ensembleVerifiedModelFor(context.Background(), ProviderTypeGroq)
	if got != "groq-chat-best" {
		t.Fatalf("resolver picked %q, want the highest-scored verified non-embedding groq chat model %q", got, "groq-chat-best")
	}

	// Cross-provider isolation: deepseek resolves to its own best, never groq's.
	if got := ensembleVerifiedModelFor(context.Background(), ProviderTypeDeepSeek); got != "deepseek-chat-best" {
		t.Fatalf("deepseek resolver picked %q, want %q", got, "deepseek-chat-best")
	}
}

// TestEnsembleVerifiedModelFor_NeverPicksEmbeddingOrDeprecated is the §1.1
// paired-mutation guard: if the resolver's skip filters were removed (mutation),
// it would return the 9.9-score embedding or 9.5-score deprecated model. This
// test asserts it returns NEITHER — so the mutation that drops either filter
// makes this test FAIL.
func TestEnsembleVerifiedModelFor_NeverPicksEmbeddingOrDeprecated(t *testing.T) {
	seed := []*verifier.VerifiedModel{
		{ID: "groq-embed-x", Provider: "groq", Verified: true, OverallScore: 9.9, SupportsEmbeddings: true},
		{ID: "groq-old-decommissioned", Provider: "groq", Verified: true, OverallScore: 9.5, Deprecated: true},
		{ID: "groq-capembed-only", Provider: "groq", Verified: true, OverallScore: 9.4, Capabilities: []string{"embedding"}},
		{ID: "groq-chat-best", Provider: "groq", Verified: true, OverallScore: 8.0},
	}
	withVerifierAdapter(t, newEnabledVerifierAdapterWithModels(t, seed))

	got := ensembleVerifiedModelFor(context.Background(), ProviderTypeGroq)
	switch got {
	case "groq-embed-x":
		t.Fatalf("resolver picked an embedding model (SupportsEmbeddings) — must be excluded")
	case "groq-old-decommissioned":
		t.Fatalf("resolver picked a deprecated model — must be excluded")
	case "groq-capembed-only":
		t.Fatalf("resolver picked an embedding-capability-only model — must be excluded")
	case "groq-chat-best":
		// correct
	default:
		t.Fatalf("resolver returned unexpected id %q", got)
	}
}

// TestEnsembleVerifiedModelFor_DisabledVerifierYieldsEmpty proves the resolver
// returns "" (→ the caller's catalogue fallback) when the verifier is not wired
// — never a guessed/hardcoded name (§11.4.6).
func TestEnsembleVerifiedModelFor_DisabledVerifierYieldsEmpty(t *testing.T) {
	withVerifierAdapter(t, nil)
	if got := ensembleVerifiedModelFor(context.Background(), ProviderTypeGroq); got != "" {
		t.Fatalf("disabled verifier must yield empty resolution, got %q", got)
	}

	// Enabled-but-empty catalogue ⇒ still empty (no eligible groq model).
	withVerifierAdapter(t, newEnabledVerifierAdapterWithModels(t, []*verifier.VerifiedModel{
		{ID: "deepseek-only", Provider: "deepseek", Verified: true, OverallScore: 9.0},
	}))
	if got := ensembleVerifiedModelFor(context.Background(), ProviderTypeGroq); got != "" {
		t.Fatalf("no eligible groq model must yield empty resolution, got %q", got)
	}
}

// TestEnsembleOrderedCandidates_VerifierFirst proves the live verifier-resolved
// model is tried BEFORE the provider's own catalogue entries (so a wired
// verifier means a single successful call per member, not a probe loop), and
// that the cached working model still takes absolute priority.
func TestEnsembleOrderedCandidates_VerifierFirst(t *testing.T) {
	seed := []*verifier.VerifiedModel{
		{ID: "groq-verified-best", Provider: "groq", Verified: true, OverallScore: 9.0},
	}
	withVerifierAdapter(t, newEnabledVerifierAdapterWithModels(t, seed))

	// The member's own catalogue leads with a (would-be) dead model + an
	// embedding model; the verifier model must come first regardless.
	stub := &modelAwareStub{
		ptype:      ProviderTypeGroq,
		name:       "Groq",
		ids:        []string{"catalogue-a", "catalogue-embed"},
		embedModel: "catalogue-embed",
	}
	ens := NewEnsembleProvider(EnsembleProviderConfig{Members: []Provider{stub}})

	got := ens.orderedCandidates(context.Background(), stub)
	if len(got) == 0 || got[0] != "groq-verified-best" {
		t.Fatalf("verifier-resolved model must be first candidate, got %v", got)
	}
	// The embedding catalogue entry must not appear at all (capability-driven
	// exclusion — not name-driven).
	for _, id := range got {
		if id == "catalogue-embed" {
			t.Fatalf("embedding catalogue entry must be excluded, got %v", got)
		}
	}

	// Cached working model wins absolutely.
	ens.rememberWorkingModel(stub.GetName(), "cached-winner")
	got = ens.orderedCandidates(context.Background(), stub)
	if got[0] != "cached-winner" {
		t.Fatalf("cached working model must be first, got %v", got)
	}
	if got[1] != "groq-verified-best" {
		t.Fatalf("verifier model must be second after cache, got %v", got)
	}
}
