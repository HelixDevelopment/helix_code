package fastapply

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"time"

	"dev.helix.code/internal/llm/routing"
)

// Route records which path produced an apply result. It is the captured
// anti-bluff evidence for P3-T03: an [Outcome] always names the route that
// actually produced the bytes the caller received.
type Route int

const (
	// RouteReference means the reference apply produced the result —
	// either because fast-apply was config-disabled, or because it was
	// invoked as the verified fallback.
	RouteReference Route = iota

	// RouteFast means the fast route produced the result AND that result
	// was byte-verified equal to the reference apply. A result is NEVER
	// reported as RouteFast unless it passed byte-equality verification.
	RouteFast
)

// String renders a Route for logging and evidence capture.
func (r Route) String() string {
	switch r {
	case RouteReference:
		return "reference"
	case RouteFast:
		return "fast"
	default:
		return "unknown"
	}
}

// FallbackReason explains why a fast-apply attempt did not ship its bytes.
type FallbackReason int

const (
	// FallbackNone means no fallback occurred (fast route succeeded and
	// verified, or fast-apply was disabled so no attempt was made).
	FallbackNone FallbackReason = iota

	// FallbackDisabled means fast-apply is config-disabled; the reference
	// path was taken directly without attempting the fast route.
	FallbackDisabled

	// FallbackFastError means the fast route returned an error.
	FallbackFastError

	// FallbackByteMismatch means the fast route produced bytes that did
	// NOT match the reference apply — the verification core rejecting an
	// incorrect fast result. This is the central correctness guarantee.
	FallbackByteMismatch

	// FallbackNoFastFunc means no [FastEditFunc] was configured, so the
	// reference path was used.
	FallbackNoFastFunc
)

// String renders a FallbackReason for logging and evidence capture.
func (r FallbackReason) String() string {
	switch r {
	case FallbackNone:
		return "none"
	case FallbackDisabled:
		return "disabled"
	case FallbackFastError:
		return "fast_error"
	case FallbackByteMismatch:
		return "byte_mismatch"
	case FallbackNoFastFunc:
		return "no_fast_func"
	default:
		return "unknown"
	}
}

// FastEditFunc performs an edit via the fast route — a small/specialised
// apply model, or a speculative-edit strategy where the original file seeds
// the draft. It receives the original file bytes and the [Instruction] and
// returns the candidate edited file.
//
// The candidate is ALWAYS reference-verified before it is returned to the
// caller, so a FastEditFunc that hallucinates, drops bytes, or errors can
// only cost wasted latency — never a wrong file. modelID is the concrete
// model the routing layer selected for this apply, supplied for logging.
type FastEditFunc func(ctx context.Context, modelID string, instr *Instruction, original []byte) ([]byte, error)

// Outcome is the result of an [Applier.Apply] call. It always carries the
// final correct bytes plus the captured evidence — which route produced
// them, whether a fallback occurred and why, and the wall-clock split.
type Outcome struct {
	// Content is the final, correct edited file. It is byte-identical to
	// ReferenceApply(instr, original) on success — guaranteed, because a
	// fast result is only ever returned after passing byte verification.
	Content []byte

	// Route names the path that produced Content.
	Route Route

	// Fallback explains why fast-apply did not ship its bytes (FallbackNone
	// when it did, or when fast-apply was simply not attempted).
	Fallback FallbackReason

	// ModelID is the concrete fast model the routing layer selected, when
	// the fast route was attempted. Empty for a pure reference apply.
	ModelID string

	// FastDuration is the wall-clock the fast route took (zero when not
	// attempted). FastBytesVerified reports whether its output verified.
	FastDuration time.Duration

	// FastBytesVerified is true when the fast route ran AND its bytes
	// matched the reference apply byte-for-byte.
	FastBytesVerified bool

	// ReferenceDuration is the wall-clock the reference apply took. The
	// reference apply ALWAYS runs (it is the verification oracle), so this
	// is always populated on a successful Apply.
	ReferenceDuration time.Duration
}

// UsedFast reports whether the fast route's bytes were the ones shipped.
func (o Outcome) UsedFast() bool { return o.Route == RouteFast }

// Config tunes an [Applier].
type Config struct {
	// Enabled is the no-regression safety valve. When false the Applier
	// performs the reference apply ONLY — the fast route is never invoked
	// and output is byte-identical to the pre-existing reference behaviour.
	Enabled bool

	// VerifyAlways, when true (the default and the safe setting), compares
	// every fast result against the reference apply before shipping it.
	// It exists as an explicit field so the verification step is visible
	// and auditable; setting it false is NOT supported for production use
	// and the Applier treats false as true to preserve correctness.
	VerifyAlways bool
}

// DefaultConfig returns the safe default: fast-apply enabled with mandatory
// byte verification of every fast result against the reference apply.
func DefaultConfig() Config {
	return Config{Enabled: true, VerifyAlways: true}
}

// DisabledConfig returns a Config with fast-apply off — the reference path
// only. This is the explicit config value an operator selects to take the
// pure no-regression behaviour.
func DisabledConfig() Config {
	return Config{Enabled: false, VerifyAlways: true}
}

// Applier executes file edits through the fast-apply path with mandatory
// reference verification. It is safe for concurrent use; its stats are
// mutex-guarded.
type Applier struct {
	cfg      Config
	fastFn   FastEditFunc
	router   *routing.Router
	applyGen routing.GenerateFunc

	mu    sync.Mutex
	stats Stats
}

// Stats summarises an Applier's apply history — the aggregate anti-bluff
// evidence: how many applies took the fast route vs fell back, and why.
type Stats struct {
	Total            int
	FastShipped      int
	ReferenceShipped int
	ByteMismatch     int
	FastError        int
	Disabled         int

	// FastDurationSum / ReferenceDurationSum accumulate wall-clock across
	// applies that ran each route, for throughput-delta evidence.
	FastDurationSum      time.Duration
	ReferenceDurationSum time.Duration
}

// NewApplier builds an [Applier]. cfg tunes behaviour ([DefaultConfig] /
// [DisabledConfig]). fastFn is the fast route; a nil fastFn makes every
// apply take the reference path (FallbackNoFastFunc). The Applier is valid
// with a nil fastFn — it then behaves as a pure reference applier.
func NewApplier(cfg Config, fastFn FastEditFunc) *Applier {
	return &Applier{cfg: cfg, fastFn: fastFn}
}

// WithRouting wires the [Applier] to a P3-T01 [routing.Router] so the fast
// apply is dispatched to the routing-selected model tier. gen is the
// [routing.GenerateFunc] that performs one apply-model call; the Applier
// routes it through the router under [routing.TaskClass] "fast_apply" so
// the apply runs on the small/specialised tier and escalates on low
// confidence exactly like every other routed subtask.
//
// When a router is wired, the [FastEditFunc] passed to [NewApplier] may be
// nil — the routed generate path supersedes it. WithRouting returns the
// Applier for chaining.
func (a *Applier) WithRouting(router *routing.Router, gen routing.GenerateFunc) *Applier {
	a.router = router
	a.applyGen = gen
	return a
}

// TaskClassFastApply is the routing task class file-apply subtasks run
// under. File apply is a mechanical transformation well-suited to the
// small/specialised tier — exactly the cheap-subtask shape P3-T01 routes.
const TaskClassFastApply routing.TaskClass = "fast_apply"

// Apply applies instr to original and returns the final, correct edited
// file plus captured evidence.
//
// Guarantee: Outcome.Content is ALWAYS byte-identical to
// ReferenceApply(instr, original). The fast route's bytes are shipped only
// when they pass byte-for-byte verification against the reference apply;
// on any mismatch, fast error, disabled config, or missing fast route the
// reference apply's own bytes are shipped. A wrong file is never returned.
//
// If the reference apply itself fails (an unsatisfiable instruction), Apply
// returns that error — an honest failure, never a guessed result.
func (a *Applier) Apply(ctx context.Context, instr *Instruction, original []byte) (Outcome, error) {
	if instr == nil {
		return Outcome{}, ErrNilInstruction
	}

	// The reference apply ALWAYS runs: it is both the correctness oracle
	// the fast result is verified against and the guaranteed fallback.
	refStart := time.Now()
	refBytes, refErr := ReferenceApply(instr, original)
	refDur := time.Since(refStart)
	if refErr != nil {
		// The instruction is unsatisfiable. Do not attempt the fast route —
		// there is nothing correct to ship. Return the honest error.
		return Outcome{}, fmt.Errorf("fastapply: reference apply failed: %w", refErr)
	}

	out := Outcome{
		Content:           refBytes,
		Route:             RouteReference,
		Fallback:          FallbackNone,
		ReferenceDuration: refDur,
	}

	// Config-gated no-regression valve: disabled → reference path only.
	if !a.cfg.Enabled {
		out.Fallback = FallbackDisabled
		a.record(out)
		return out, nil
	}

	// No fast route configured → reference path.
	if a.fastFn == nil && (a.router == nil || a.applyGen == nil) {
		out.Fallback = FallbackNoFastFunc
		a.record(out)
		return out, nil
	}

	// Attempt the fast route.
	fastStart := time.Now()
	fastBytes, modelID, fastErr := a.runFast(ctx, instr, original)
	out.FastDuration = time.Since(fastStart)
	out.ModelID = modelID

	if fastErr != nil {
		// Fast route errored — fall back to the verified reference bytes.
		out.Fallback = FallbackFastError
		a.record(out)
		return out, nil
	}

	// MANDATORY verification: the fast bytes must equal the reference
	// bytes exactly. This is the correctness core of P3-T03 — a fast
	// result that differs by even one byte is rejected.
	if !bytes.Equal(fastBytes, refBytes) {
		out.Fallback = FallbackByteMismatch
		a.record(out)
		return out, nil
	}

	// Fast route produced byte-identical output: ship the fast result.
	// (Content is already refBytes, which equals fastBytes — identical.)
	out.Route = RouteFast
	out.FastBytesVerified = true
	a.record(out)
	return out, nil
}

// runFast invokes the fast route. When a router is wired the apply is
// dispatched through it (so it runs on the routing-selected tier and the
// routing log captures the model used); otherwise the direct FastEditFunc
// is called. It returns the candidate bytes and the model that produced
// them.
func (a *Applier) runFast(ctx context.Context, instr *Instruction, original []byte) ([]byte, string, error) {
	if a.router != nil && a.applyGen != nil {
		// Route through the P3-T01 router. The router resolves the model
		// tier and runs applyGen; we wrap applyGen so the actual file
		// transformation happens via the fast function while the router
		// records the model-used evidence.
		var fastBytes []byte
		gen := func(gctx context.Context, modelID string, tier routing.ModelTier) (routing.Result, error) {
			res, err := a.applyGen(gctx, modelID, tier)
			if err != nil {
				return routing.Result{}, err
			}
			// If a direct fast function is also configured, use it for the
			// byte transformation seeded by the routed model; otherwise the
			// applyGen Result.Content carries the edited file directly.
			if a.fastFn != nil {
				b, ferr := a.fastFn(gctx, modelID, instr, original)
				if ferr != nil {
					return routing.Result{}, ferr
				}
				fastBytes = b
			} else {
				fastBytes = []byte(res.Content)
			}
			return res, nil
		}
		res, err := a.router.Route(ctx, TaskClassFastApply, gen)
		if err != nil {
			return nil, "", err
		}
		return fastBytes, res.ModelID, nil
	}

	// No router — direct fast function.
	b, err := a.fastFn(ctx, "", instr, original)
	if err != nil {
		return nil, "", err
	}
	return b, "", nil
}

// record folds an Outcome into the Applier's aggregate Stats.
func (a *Applier) record(o Outcome) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.stats.Total++
	a.stats.ReferenceDurationSum += o.ReferenceDuration
	a.stats.FastDurationSum += o.FastDuration
	switch o.Route {
	case RouteFast:
		a.stats.FastShipped++
	case RouteReference:
		a.stats.ReferenceShipped++
	}
	switch o.Fallback {
	case FallbackByteMismatch:
		a.stats.ByteMismatch++
	case FallbackFastError:
		a.stats.FastError++
	case FallbackDisabled:
		a.stats.Disabled++
	}
}

// Stats returns a snapshot of the Applier's aggregate apply history — the
// captured anti-bluff evidence for the route split and fallback counts.
func (a *Applier) Stats() Stats {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.stats
}

// ResetStats clears the aggregate Stats. Useful between benchmark runs.
func (a *Applier) ResetStats() {
	a.mu.Lock()
	a.stats = Stats{}
	a.mu.Unlock()
}
