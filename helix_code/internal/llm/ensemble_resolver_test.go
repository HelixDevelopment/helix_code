package llm

import (
	"context"
	"os"
	"sort"
	"testing"
	"time"

	"dev.helix.code/internal/verifier"
)

// resolverRedMode reports whether the §11.4.115 RED polarity switch is active for
// the native-ordering regression guard.
//
//	RED_MODE=1 (default): reproduce the DEFECT — assert that ALPHABETICAL sorting
//	            of the catalogue surfaces the dead model first (the pre-fix
//	            catalogueChatCandidatesFor did `sort.Strings`, which led with the
//	            decommissioned/paid entry → discovery burst → free-tier rate-limit
//	            → ensemble all-fail in the TUI).
//	RED_MODE=0: the standing GREEN regression guard asserts the defect is ABSENT —
//	            catalogueChatCandidatesFor preserves the provider's native
//	            working-model-first order so the lead candidate is the working one.
func resolverRedMode() bool {
	v := os.Getenv("RED_MODE")
	return v == "" || v == "1"
}

// TestCatalogueChatCandidates_PreservesProviderNativeOrder is the permanent
// regression guard (§11.4.135) for the live TUI ensemble all-fail defect: the
// provider's GetModels() leads with its WORKING chat model (real providers curate
// it: deepseek-chat, llama-3.3-70b-versatile, mistral-small-latest, the live
// OpenRouter free-tier re-order) and the catalogue's last entry is a
// decommissioned/paid one. catalogueChatCandidatesFor MUST preserve that native
// order — NOT alphabetically sort, which would surface the dead entry first.
//
// The fixture mirrors the captured live bug: "zzz-decommissioned" sorts FIRST
// alphabetically but is the catalogue's LAST (worst) entry; the real working
// model "aaa-working" is the catalogue's FIRST. The provider here puts the
// working model first (real-provider behaviour) but in a non-alphabetical order.
func TestCatalogueChatCandidates_PreservesProviderNativeOrder(t *testing.T) {
	// Native catalogue order: working chat model FIRST, dead model LAST —
	// exactly how the real cloud providers order their live catalogues. Note the
	// ids are NOT in alphabetical order: "aaa-working" < "mmm-mid" <
	// "zzz-decommissioned" alphabetically would coincidentally match here, so we
	// pick ids whose native order DIFFERS from alphabetical to make the test
	// discriminate: native = [working, dead, mid]; alphabetical = [dead?, mid, working].
	stub := &modelAwareStub{
		ptype: ProviderTypeGroq,
		name:  "Groq",
		// native order: the WORKING model leads; a paid/dead model and a mid model
		// follow. Alphabetically this list would reorder to
		// [a-paid-402, m-mid, z-working] — surfacing the dead "a-paid-402" first.
		ids: []string{"z-working-chat", "a-paid-402", "m-mid-chat"},
	}

	got := catalogueChatCandidatesFor(stub)
	if len(got) == 0 {
		t.Fatalf("no candidates returned")
	}

	if resolverRedMode() {
		// RED: prove the DEFECT — alphabetical sorting surfaces the paid/dead
		// model first. (This is what the pre-fix code did and what tripped the
		// burst.) We compute the alphabetical order the old code produced and
		// assert it leads with the dead entry, demonstrating the defect is real.
		alpha := append([]string(nil), got...)
		sort.Strings(alpha)
		if alpha[0] != "a-paid-402" {
			t.Fatalf("RED expectation broken: alphabetical lead = %q, want the dead model %q", alpha[0], "a-paid-402")
		}
		// And prove the dead model leading is DIFFERENT from the native lead, i.e.
		// the alphabetical-sort defect genuinely mis-orders.
		if alpha[0] == got[0] {
			t.Fatalf("RED expectation broken: alphabetical lead == native lead %q (fixture not discriminating)", got[0])
		}
		return
	}

	// GREEN: the production resolver preserves native order — the working model
	// leads, the paid/dead model never leads. This is the standing guard.
	if got[0] != "z-working-chat" {
		t.Fatalf("native order not preserved: lead candidate = %q, want the provider's working-first model %q (alphabetical sort regression)", got[0], "z-working-chat")
	}
	// The dead/paid entry must NOT be first (it is the burst trigger).
	if got[0] == "a-paid-402" {
		t.Fatalf("dead/paid model surfaced first — the alphabetical-sort burst defect has regressed")
	}
}

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
