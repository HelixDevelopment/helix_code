package routing

import (
	"context"
	"fmt"
	"sort"
)

// TierModel is the minimal model metadata the routing resolver needs to pick
// a concrete model for a [ModelTier]. It is a decoupled projection of the
// verifier's VerifiedModel — this package never imports the verifier
// package, keeping it project-not-aware and reusable (CONST-051(B)).
//
// The caller (the agent / commands wiring) projects the authoritative
// LLMsVerifier metadata into a []TierModel; this package never invents model
// data and never hardcodes a model list (CONST-036 / CONST-037).
type TierModel struct {
	// ID is the concrete model identifier passed to a provider.
	ID string

	// VerifierTier is the verifier's 1-5 tier classification
	// (1=Premium, 2=High-quality, 3=Fast, 4=Aggregator, 5=Free).
	VerifierTier int

	// Score is the verifier's overall quality score; used to pick the best
	// model within a routing tier.
	Score float64

	// Verified is the verifier's verification flag. Only verified models are
	// eligible (CONST-037 — every model shown/used must be verifier-checked).
	Verified bool

	// Deprecated marks a model the verifier flags as retired; never selected.
	Deprecated bool
}

// VerifiedModelSource supplies the authoritative model catalogue from
// LLMsVerifier. The agent / commands wiring implements this against the
// verifier adapter; unit tests implement it with a mock catalogue (mocks are
// permitted in unit tests per CONST-050(A)).
type VerifiedModelSource interface {
	// VerifiedModels returns the current verifier model catalogue.
	VerifiedModels(ctx context.Context) ([]TierModel, error)
}

// VerifierResolver is a [ModelResolver] that picks the concrete model for a
// routing tier from live LLMsVerifier metadata. It satisfies CONST-036/037:
// the model list is never hardcoded — it always comes from the verifier
// catalogue via the [VerifiedModelSource].
//
// Tier mapping:
//
//   - [TierSmall]    → verifier tiers 3 (Fast) and 5 (Free); highest-scored
//     such model wins.
//   - [TierFrontier] → verifier tiers 1 (Premium) and 2 (High-quality);
//     highest-scored such model wins.
//
// When no model matches a tier's verifier-tier set, the resolver returns the
// highest-scored verified model overall — a conservative fallback that
// never blocks a subtask and never silently downgrades quality (a missing
// small model resolves to the best available; a missing frontier model
// resolves to the best available).
type VerifierResolver struct {
	source VerifiedModelSource
}

// smallVerifierTiers and frontierVerifierTiers are the verifier tier numbers
// each routing tier draws from. These are routing policy, not a model list —
// the concrete models are always discovered from verifier metadata.
var (
	smallVerifierTiers    = map[int]bool{3: true, 5: true}
	frontierVerifierTiers = map[int]bool{1: true, 2: true}
)

// NewVerifierResolver builds a [VerifierResolver] over the given verifier
// model source. A nil source makes every ResolveModel call fail with
// [ErrNoModelForTier].
func NewVerifierResolver(source VerifiedModelSource) *VerifierResolver {
	return &VerifierResolver{source: source}
}

// ResolveModel implements [ModelResolver]. It fetches the live verifier
// catalogue and returns the highest-scored verified, non-deprecated model
// whose verifier tier belongs to the requested routing tier.
func (r *VerifierResolver) ResolveModel(ctx context.Context, tier ModelTier) (string, error) {
	if r.source == nil {
		return "", fmt.Errorf("%w: %s (no verifier source)", ErrNoModelForTier, tier)
	}

	models, err := r.source.VerifiedModels(ctx)
	if err != nil {
		return "", fmt.Errorf("routing: fetch verifier catalogue: %w", err)
	}

	eligible := make([]TierModel, 0, len(models))
	for _, m := range models {
		if m.Deprecated || !m.Verified || m.ID == "" {
			continue
		}
		eligible = append(eligible, m)
	}
	if len(eligible) == 0 {
		return "", fmt.Errorf("%w: %s (verifier catalogue has no eligible models)", ErrNoModelForTier, tier)
	}

	var want map[int]bool
	switch tier {
	case TierSmall:
		want = smallVerifierTiers
	case TierFrontier:
		want = frontierVerifierTiers
	default:
		return "", fmt.Errorf("%w: unknown tier %s", ErrNoModelForTier, tier)
	}

	// Prefer models in the routing tier's verifier-tier set; fall back to the
	// best verified model overall when the set is empty.
	matched := make([]TierModel, 0, len(eligible))
	for _, m := range eligible {
		if want[m.VerifierTier] {
			matched = append(matched, m)
		}
	}
	pool := matched
	if len(pool) == 0 {
		pool = eligible
	}

	// Highest score wins; ties broken by ID for deterministic resolution.
	sort.Slice(pool, func(i, j int) bool {
		if pool[i].Score != pool[j].Score {
			return pool[i].Score > pool[j].Score
		}
		return pool[i].ID < pool[j].ID
	})
	return pool[0].ID, nil
}
