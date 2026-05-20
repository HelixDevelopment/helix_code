// Package routing implements small-model routing for cheap LLM subtasks —
// the "model cascade" lever of the HelixCode speed programme (Phase 3,
// task P3-T01).
//
// Multi-step agent loops run many cheap LLM subtasks — task classification,
// candidate ranking, commit-message generation, ambiguity detection — on the
// same frontier model used for hard reasoning. Routing those trivial subtasks
// to a fast/cheap model and escalating to the frontier model only when the
// cheap model's result is low-confidence cuts agent-loop wall-clock without
// degrading output quality.
//
// # Design
//
//   - A [Policy] maps a [TaskClass] to a target [ModelTier] and the
//     confidence threshold below which the cheap result is escalated.
//   - A [ModelResolver] turns a [ModelTier] into a concrete model ID. The
//     concrete model is ALWAYS sourced from LLMsVerifier metadata
//     (CONST-036 / CONST-037) — this package never hardcodes a model list.
//   - A [Router] executes a generation through the policy: it runs the cheap
//     tier first, inspects the result's confidence, and escalates to the
//     frontier tier when confidence is below the policy threshold.
//
// # Decoupling (CONST-051(B))
//
// This package is project-not-aware and reusable. It depends only on the
// standard library and the [ModelResolver] / [GenerateFunc] interfaces it
// defines. The verifier and llm.Provider wiring lives in the caller, not
// here.
//
// # No-regression guarantee
//
// The escalate-on-low-confidence path guarantees a hard task still reaches
// the frontier model. Routing is config-gated: a [Policy] with
// ForceFrontier set sends EVERY subtask straight to the frontier tier,
// behaving exactly as if no routing existed. That switch is the
// no-regression safety valve for the Med-risk quality constraint.
package routing

import (
	"context"
	"errors"
	"fmt"
)

// ModelTier identifies a class of model by capability/cost rather than by a
// concrete model ID. The concrete model for a tier is resolved at runtime
// from LLMsVerifier metadata (CONST-036/037) via a [ModelResolver].
type ModelTier int

const (
	// TierSmall is the fast/cheap tier — small models suitable for trivial
	// subtasks (classification, ranking, short generation). Maps to verifier
	// model Tier 3 (Fast) / Tier 5 (Free) metadata.
	TierSmall ModelTier = iota

	// TierFrontier is the high-capability tier — the frontier models used
	// for hard reasoning. Maps to verifier model Tier 1 (Premium) / Tier 2
	// (High-quality) metadata. This is the escalation target.
	TierFrontier
)

// String renders a ModelTier for logging and evidence capture.
func (t ModelTier) String() string {
	switch t {
	case TierSmall:
		return "small"
	case TierFrontier:
		return "frontier"
	default:
		return fmt.Sprintf("ModelTier(%d)", int(t))
	}
}

// TaskClass identifies a category of LLM subtask. Trivial classes are
// routed to the small tier; reasoning-heavy classes go straight to the
// frontier tier. The classes below cover the cheap-subtask call sites named
// in speed-programme task P3-T01.
type TaskClass string

const (
	// TaskClassification is a short classification subtask — e.g. deciding
	// which TaskType an agent request belongs to. Trivial; routes small.
	TaskClassification TaskClass = "classification"

	// TaskRanking is a candidate-ranking subtask — ordering a small set of
	// options. Trivial; routes small.
	TaskRanking TaskClass = "ranking"

	// TaskCommitMessage is commit-message generation from a diff summary.
	// Trivial; routes small.
	TaskCommitMessage TaskClass = "commit_message"

	// TaskAmbiguityDetection decides whether a user prompt is ambiguous and
	// what clarifying questions to ask. Trivial; routes small.
	TaskAmbiguityDetection TaskClass = "ambiguity_detection"

	// TaskReasoning is a reasoning-heavy subtask — code generation, planning,
	// debugging. Routes straight to the frontier tier, no escalation needed.
	TaskReasoning TaskClass = "reasoning"
)

// Sentinel errors for routing operations.
var (
	// ErrNoModelForTier indicates the [ModelResolver] could not produce a
	// concrete model for the requested tier. The Router treats a small-tier
	// resolution failure as a reason to fall through to the frontier tier;
	// a frontier-tier resolution failure is fatal.
	ErrNoModelForTier = errors.New("routing: no model available for tier")

	// ErrNilGenerateFunc is returned by Route when no GenerateFunc was
	// supplied.
	ErrNilGenerateFunc = errors.New("routing: generate function must not be nil")

	// ErrNilResolver is returned by NewRouter when no ModelResolver was
	// supplied.
	ErrNilResolver = errors.New("routing: model resolver must not be nil")
)

// ModelResolver resolves a [ModelTier] to a concrete model ID using
// authoritative model metadata. Implementations MUST source model metadata
// from LLMsVerifier (CONST-036/037) and MUST NOT hardcode model lists.
//
// ResolveModel returns the model ID to use for the tier, or an error
// (wrapping [ErrNoModelForTier]) when no suitable model is known.
type ModelResolver interface {
	ResolveModel(ctx context.Context, tier ModelTier) (string, error)
}

// Result is the outcome of one LLM subtask call. The Confidence field drives
// the escalation decision — a small-model Result with Confidence below the
// policy threshold is escalated to the frontier tier.
type Result struct {
	// ModelID is the concrete model that produced this Result.
	ModelID string

	// Tier is the tier the model belongs to.
	Tier ModelTier

	// Content is the textual output of the subtask.
	Content string

	// Confidence is a [0.0, 1.0] self-reported or caller-computed confidence
	// in Content. A GenerateFunc that cannot compute a confidence should
	// return 1.0 (treat as confident) so routing never escalates blindly.
	Confidence float64

	// Escalated is true when this Result came from the frontier tier after a
	// low-confidence small-tier attempt.
	Escalated bool
}

// GenerateFunc performs one LLM subtask against a concrete model. The Router
// calls it once for the small tier and, on low confidence, once more for the
// frontier tier. Implementations wrap a real llm.Provider.Generate call.
//
// The returned Result.Confidence is what the Router compares against the
// policy threshold. The ModelID and Tier fields of the returned Result are
// overwritten by the Router with the values it routed to, so a GenerateFunc
// need not populate them.
type GenerateFunc func(ctx context.Context, modelID string, tier ModelTier) (Result, error)
