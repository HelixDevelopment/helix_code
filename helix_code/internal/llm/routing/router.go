package routing

import (
	"context"
	"fmt"
	"sync"
)

// RouteEvent is one entry in a Router's per-subtask routing log. It is the
// captured anti-bluff evidence required by speed-programme task P3-T01: a
// per-subtask model-used record proving which model actually served each
// call and whether an escalation occurred.
type RouteEvent struct {
	// Class is the task class that was routed.
	Class TaskClass

	// Tier is the tier the call actually ran on.
	Tier ModelTier

	// ModelID is the concrete model that served the call.
	ModelID string

	// Confidence is the confidence the call reported.
	Confidence float64

	// Escalated is true when this event is the frontier escalation of a
	// low-confidence small-tier attempt.
	Escalated bool
}

// Router executes LLM subtasks through a [Policy]: it resolves the policy's
// target tier to a concrete model via a [ModelResolver], runs the supplied
// [GenerateFunc] on that model, and — for small-tier results below the
// policy's confidence threshold — escalates to the frontier tier.
//
// A Router is safe for concurrent use; its routing log is mutex-guarded.
type Router struct {
	policy   *Policy
	resolver ModelResolver

	mu  sync.Mutex
	log []RouteEvent
}

// NewRouter builds a [Router] from a [Policy] and a [ModelResolver]. A nil
// policy defaults to [DefaultPolicy]. A nil resolver is an error — the
// Router cannot turn a tier into a concrete model without one.
func NewRouter(policy *Policy, resolver ModelResolver) (*Router, error) {
	if resolver == nil {
		return nil, ErrNilResolver
	}
	if policy == nil {
		policy = DefaultPolicy()
	}
	return &Router{policy: policy, resolver: resolver}, nil
}

// Policy returns the Router's policy.
func (r *Router) Policy() *Policy { return r.policy }

// Route executes one LLM subtask of the given [TaskClass] through the policy.
//
// Behaviour:
//
//  1. Resolve the policy's initial tier for the class to a concrete model.
//  2. Run gen on that model.
//  3. If the result ran on the small tier and its confidence is below the
//     policy threshold, resolve the frontier model and re-run gen on it;
//     return the frontier result (Escalated=true).
//  4. If the small tier cannot be resolved at all, fall through to the
//     frontier tier (a missing cheap model must never block a subtask).
//
// Every call appended to the routing log is retrievable via [Router.Log] —
// the per-subtask model-used evidence for the anti-bluff proof.
//
// Route never silently degrades quality: a hard task (TaskReasoning, an
// unknown class, or a low-confidence small result) always reaches the
// frontier model. With a [FrontierOnlyPolicy] every call goes straight to
// the frontier tier with zero behavioural difference from un-routed code.
func (r *Router) Route(ctx context.Context, class TaskClass, gen GenerateFunc) (Result, error) {
	if gen == nil {
		return Result{}, ErrNilGenerateFunc
	}

	initialTier := r.policy.InitialTier(class)

	// Resolve the initial tier. A small-tier resolution failure is not
	// fatal — fall through to the frontier tier so the subtask still runs.
	modelID, err := r.resolver.ResolveModel(ctx, initialTier)
	if err != nil {
		if initialTier == TierSmall {
			return r.runFrontier(ctx, class, gen, false)
		}
		return Result{}, fmt.Errorf("routing: resolve %s tier: %w", initialTier, err)
	}

	res, err := gen(ctx, modelID, initialTier)
	if err != nil {
		return Result{}, fmt.Errorf("routing: %s subtask on %s (%s): %w", class, modelID, initialTier, err)
	}
	res.ModelID = modelID
	res.Tier = initialTier
	res.Escalated = false
	r.record(class, res)

	// Escalate a low-confidence small-tier result to the frontier tier.
	if r.policy.ShouldEscalate(res) {
		return r.runFrontier(ctx, class, gen, true)
	}

	return res, nil
}

// runFrontier resolves and runs the subtask on the frontier tier. escalated
// records whether this run is the escalation of a prior small-tier attempt.
func (r *Router) runFrontier(ctx context.Context, class TaskClass, gen GenerateFunc, escalated bool) (Result, error) {
	modelID, err := r.resolver.ResolveModel(ctx, TierFrontier)
	if err != nil {
		return Result{}, fmt.Errorf("routing: resolve frontier tier: %w", err)
	}
	res, err := gen(ctx, modelID, TierFrontier)
	if err != nil {
		return Result{}, fmt.Errorf("routing: %s subtask on frontier %s: %w", class, modelID, err)
	}
	res.ModelID = modelID
	res.Tier = TierFrontier
	res.Escalated = escalated
	r.record(class, res)
	return res, nil
}

// record appends a RouteEvent to the routing log.
func (r *Router) record(class TaskClass, res Result) {
	r.mu.Lock()
	r.log = append(r.log, RouteEvent{
		Class:      class,
		Tier:       res.Tier,
		ModelID:    res.ModelID,
		Confidence: res.Confidence,
		Escalated:  res.Escalated,
	})
	r.mu.Unlock()
}

// Log returns a copy of the per-subtask routing log — the captured
// model-used evidence for the anti-bluff proof. The slice is a snapshot;
// concurrent Route calls do not mutate the returned slice.
func (r *Router) Log() []RouteEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]RouteEvent, len(r.log))
	copy(out, r.log)
	return out
}

// Stats summarises the routing log: how many subtasks ran on each tier and
// how many escalated. Used by benchmarks and the loop wall-clock evidence.
type Stats struct {
	Total        int
	SmallTier    int
	FrontierTier int
	Escalations  int
}

// Stats computes a [Stats] summary of the routing log.
func (r *Router) Stats() Stats {
	r.mu.Lock()
	defer r.mu.Unlock()
	var s Stats
	for _, e := range r.log {
		s.Total++
		switch e.Tier {
		case TierSmall:
			s.SmallTier++
		case TierFrontier:
			s.FrontierTier++
		}
		if e.Escalated {
			s.Escalations++
		}
	}
	return s
}

// ResetLog clears the routing log. Useful between benchmark iterations.
func (r *Router) ResetLog() {
	r.mu.Lock()
	r.log = nil
	r.mu.Unlock()
}
