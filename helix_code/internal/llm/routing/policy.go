package routing

// Policy is the routing decision table. It maps each [TaskClass] to a
// target [ModelTier] and carries the confidence threshold below which a
// small-tier result is escalated to the frontier tier.
//
// A Policy is immutable once constructed by [NewDefaultPolicy] or
// [DefaultPolicy]; callers tune behaviour through the constructor options
// rather than mutating fields, so a Policy is safe for concurrent use.
type Policy struct {
	// tierFor maps a TaskClass to its initial target tier.
	tierFor map[TaskClass]ModelTier

	// escalationThreshold is the confidence value in [0.0, 1.0] strictly
	// below which a small-tier result is escalated to the frontier tier.
	// A result with Confidence == escalationThreshold is NOT escalated.
	escalationThreshold float64

	// forceFrontier, when true, makes the Policy route EVERY task class to
	// TierFrontier — the config-gated no-routing safety valve. With it set
	// the Router behaves exactly as if routing did not exist (the
	// no-regression switch for the Med-risk quality constraint).
	forceFrontier bool
}

// DefaultEscalationThreshold is the confidence below which a small-model
// result is escalated to the frontier model. 0.7 is conservative: a small
// model must be clearly confident (>= 0.7) for its result to be accepted
// without a frontier double-check.
const DefaultEscalationThreshold = 0.7

// defaultTierTable is the canonical TaskClass→ModelTier mapping. Trivial
// subtasks route to the small tier; reasoning-heavy subtasks route straight
// to the frontier tier. It is copied into every Policy so the package-level
// map is never mutated.
func defaultTierTable() map[TaskClass]ModelTier {
	return map[TaskClass]ModelTier{
		TaskClassification:     TierSmall,
		TaskRanking:            TierSmall,
		TaskCommitMessage:      TierSmall,
		TaskAmbiguityDetection: TierSmall,
		TaskReasoning:          TierFrontier,
	}
}

// PolicyOption configures a [Policy] at construction time.
type PolicyOption func(*Policy)

// WithEscalationThreshold sets the confidence threshold below which a
// small-tier result is escalated. Values are clamped to [0.0, 1.0]. A
// threshold of 0.0 disables escalation (small results are always accepted);
// a threshold of 1.0 escalates every non-perfectly-confident small result.
func WithEscalationThreshold(threshold float64) PolicyOption {
	return func(p *Policy) {
		if threshold < 0 {
			threshold = 0
		}
		if threshold > 1 {
			threshold = 1
		}
		p.escalationThreshold = threshold
	}
}

// WithForceFrontier sets the frontier-only override. When force is true the
// Policy routes every task class to [TierFrontier]; this is the config-gated
// switch that turns routing off entirely (no-regression safety valve).
func WithForceFrontier(force bool) PolicyOption {
	return func(p *Policy) {
		p.forceFrontier = force
	}
}

// WithTierOverride overrides the target tier for a single task class. Useful
// for operators who want, say, commit-message generation always on the
// frontier tier while leaving the other cheap subtasks routed small.
func WithTierOverride(class TaskClass, tier ModelTier) PolicyOption {
	return func(p *Policy) {
		p.tierFor[class] = tier
	}
}

// NewPolicy builds a [Policy] from the default tier table and the supplied
// options. With no options it is identical to [DefaultPolicy].
func NewPolicy(opts ...PolicyOption) *Policy {
	p := &Policy{
		tierFor:             defaultTierTable(),
		escalationThreshold: DefaultEscalationThreshold,
		forceFrontier:       false,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// DefaultPolicy returns the canonical routing policy: trivial subtasks on
// the small tier, reasoning-heavy subtasks on the frontier tier, escalation
// threshold [DefaultEscalationThreshold], routing enabled.
func DefaultPolicy() *Policy {
	return NewPolicy()
}

// FrontierOnlyPolicy returns a [Policy] with the frontier-only override set —
// every subtask goes straight to the frontier tier. This is the explicit
// config value an operator selects to disable routing entirely.
func FrontierOnlyPolicy() *Policy {
	return NewPolicy(WithForceFrontier(true))
}

// InitialTier returns the tier a [TaskClass] is first routed to. When the
// frontier-only override is set every class returns [TierFrontier]. An
// unknown task class conservatively returns [TierFrontier] — never silently
// downgrade an unrecognised subtask to a cheap model.
func (p *Policy) InitialTier(class TaskClass) ModelTier {
	if p.forceFrontier {
		return TierFrontier
	}
	if tier, ok := p.tierFor[class]; ok {
		return tier
	}
	return TierFrontier
}

// ShouldEscalate reports whether a small-tier [Result] with the given
// confidence must be escalated to the frontier tier. It returns false when
// the frontier-only override is set (everything already ran on the frontier
// tier) and false for a result that did not run on the small tier.
func (p *Policy) ShouldEscalate(r Result) bool {
	if p.forceFrontier {
		return false
	}
	if r.Tier != TierSmall {
		return false
	}
	return r.Confidence < p.escalationThreshold
}

// EscalationThreshold returns the configured confidence threshold.
func (p *Policy) EscalationThreshold() float64 {
	return p.escalationThreshold
}

// ForceFrontier reports whether the frontier-only override is set.
func (p *Policy) ForceFrontier() bool {
	return p.forceFrontier
}
