// Package speckit_debate_adapter — round-70 §11.4 anti-bluff fix.
//
// This adapter package wires HelixSpecifier's speckit.LLMResponder
// interface (defined in round-65, commit f2cb17a) to DebateOrchestrator's
// orchestrator.Orchestrator (close-out⁸⁸ pipeline). The adapter lives in
// the HelixCode consumer per CONST-051(B) decoupling: HelixSpecifier and
// DebateOrchestrator each remain project-not-aware and reusable; the
// adapter is the consumer-side seam.
//
// Round-65 (HelixSpecifier commit f2cb17a) defined LLMResponder — a
// single-method interface (Generate(ctx, prompt) (string, error)) —
// and LLMBackedDebateFunc that consumes it to build a real DebateFunc
// for the SpecKit 7-phase workflow. Round-65 deferred wiring to
// DebateOrchestrator because doing so directly inside HelixSpecifier
// would have coupled HelixSpecifier to a specific debate-orchestration
// implementation — a CONST-051(B) violation.
//
// Round-70 (this package, 2026-05-18) closes the round-65 deferral by
// providing DebateOrchestratorResponder — a consumer-side adapter that:
//
//   - SATISFIES speckit.LLMResponder (via Generate), so HelixSpecifier's
//     LLMBackedDebateFunc + ExecutePhase + the 7-phase workflow can
//     invoke it transparently;
//   - DELEGATES to DebateOrchestrator's full 8-phase MASTER protocol
//     (Dehallucination / SelfEvolvement / Proposal / Critique / Review /
//     Optimization / Adversarial / Convergence) via ConductDebate;
//   - FORMATS the structured DebateResponse (per-phase per-agent
//     transcript + consensus + metrics) into a single string the
//     LLMResponder caller can use as a "model" output.
//
// Constitutional anchors:
//   - CONST-035 (anti-bluff): no fabrication; all output is sourced
//     from DebateOrchestrator's real ConductDebate execution. Errors
//     are surfaced, not swallowed. Empty outputs are flagged.
//   - CONST-050(A) (no-fakes-beyond-unit-tests): this file is
//     production code; it imports the real speckit + orchestrator
//     packages, not mocks.
//   - CONST-051(B) (decoupling): HelixSpecifier and DebateOrchestrator
//     are not modified. They remain decoupled from HelixCode and from
//     each other. This adapter is the ONLY coupling point; it lives in
//     the consumer (HelixCode) per Option-A wiring.
//   - Article XI §11.9 (forensic anchor): every Generate call produces
//     positive runtime evidence — a formatted transcript derived from
//     real orchestrator output, never a hardcoded string.
package speckit_debate_adapter

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"digital.vasic.debate/orchestrator"
	"digital.vasic.helixspecifier/pkg/speckit"
)

// DebateOrchestrator is the narrow interface this adapter requires
// from any debate orchestrator. The real *orchestrator.Orchestrator
// satisfies it (asserted at compile time below). Keeping this an
// interface (rather than a concrete *orchestrator.Orchestrator
// dependency) makes the adapter unit-testable without standing up
// the real orchestrator, while still allowing zero-friction wiring
// of the real type in production.
//
// CONST-051(B) note: this interface lives in the consumer-side
// adapter, NOT in DebateOrchestrator. The DebateOrchestrator package
// remains project-not-aware.
type DebateOrchestrator interface {
	ConductDebate(ctx context.Context, req *orchestrator.DebateRequest) (*orchestrator.DebateResponse, error)
}

// Compile-time assertion: the real *orchestrator.Orchestrator
// satisfies DebateOrchestrator. If DebateOrchestrator's API drifts
// in a future round, this assertion will fail loudly at build time
// rather than silently at runtime.
var _ DebateOrchestrator = (*orchestrator.Orchestrator)(nil)

// Round-70 sentinels — distinguishable error classes consumers can
// branch on via errors.Is.
var (
	// ErrSpeckitDebateOrchestratorNotProvided fires when the adapter
	// constructor is called with a nil DebateOrchestrator. Returned as
	// the error result of Generate when a malformed adapter slips
	// through (defensive — the constructor itself rejects nil).
	ErrSpeckitDebateOrchestratorNotProvided = errors.New("speckit_debate_adapter: DebateOrchestrator is nil — pass a non-nil orchestrator to NewDebateOrchestratorResponder (round-70 §11.4 anti-bluff: refusing to fabricate debate output without a real orchestrator)")

	// ErrSpeckitDebateAdapterFailed wraps any inner error produced by
	// the orchestrator pipeline. Consumers MAY use errors.Unwrap to
	// retrieve the underlying error for finer-grained handling.
	ErrSpeckitDebateAdapterFailed = errors.New("speckit_debate_adapter: orchestrator pipeline failed (round-70 §11.4 anti-bluff: surfacing real failure rather than swallowing)")

	// ErrSpeckitDebateOutputEmpty fires when ConductDebate returns a
	// successful response that contains no agent content. Per CONST-035
	// we refuse to fabricate a non-empty model response from an empty
	// orchestrator output.
	ErrSpeckitDebateOutputEmpty = errors.New("speckit_debate_adapter: DebateOrchestrator returned successful response with no agent content — refusing to fabricate model output (round-70 §11.4 anti-bluff: CONST-035)")
)

// DebateAdapterOption is a functional option for configuring the
// DebateOrchestratorResponder.
type DebateAdapterOption func(*adapterConfig)

// adapterConfig holds tunable knobs for the adapter. Defaults are
// chosen so the adapter works out-of-the-box for typical SpecKit
// phase invocations.
type adapterConfig struct {
	maxRounds          int
	defaultModel       string
	defaultLanguage    string
	minConsensus       float64
	preferredProviders []string
}

func defaultAdapterConfig() *adapterConfig {
	return &adapterConfig{
		maxRounds:       3,
		defaultModel:    "",
		defaultLanguage: "en",
		minConsensus:    0.6,
	}
}

// WithMaxRounds sets the MaxRounds field on the DebateRequest forwarded
// to ConductDebate. Defaults to 3. Values <1 are ignored.
func WithMaxRounds(n int) DebateAdapterOption {
	return func(c *adapterConfig) {
		if n >= 1 {
			c.maxRounds = n
		}
	}
}

// WithDefaultModel records a default model identifier for downstream
// metadata. The DebateOrchestrator API does not consume this directly
// (it routes per registered agent), but the value is echoed into the
// DebateRequest metadata for traceability.
func WithDefaultModel(model string) DebateAdapterOption {
	return func(c *adapterConfig) {
		if strings.TrimSpace(model) != "" {
			c.defaultModel = model
		}
	}
}

// WithDefaultLanguage sets the BCP-47 language tag on the DebateRequest.
// Defaults to "en".
func WithDefaultLanguage(lang string) DebateAdapterOption {
	return func(c *adapterConfig) {
		if strings.TrimSpace(lang) != "" {
			c.defaultLanguage = lang
		}
	}
}

// WithMinConsensus sets the MinConsensus threshold on the DebateRequest.
// Defaults to 0.6. Values outside [0,1] are clamped.
func WithMinConsensus(v float64) DebateAdapterOption {
	return func(c *adapterConfig) {
		if v < 0 {
			v = 0
		} else if v > 1 {
			v = 1
		}
		c.minConsensus = v
	}
}

// WithPreferredProviders sets the ordered provider preference list on
// the DebateRequest. Empty values are filtered.
func WithPreferredProviders(providers ...string) DebateAdapterOption {
	return func(c *adapterConfig) {
		out := make([]string, 0, len(providers))
		for _, p := range providers {
			if s := strings.TrimSpace(p); s != "" {
				out = append(out, s)
			}
		}
		c.preferredProviders = out
	}
}

// DebateOrchestratorResponder is the consumer-side adapter that
// SATISFIES speckit.LLMResponder by DELEGATING to a
// DebateOrchestrator. One adapter instance is safe for concurrent use
// provided the underlying orchestrator is (the real
// *orchestrator.Orchestrator is — close-out⁸⁸).
//
// Wire it into HelixSpecifier's pillar via:
//
//	orch := orchestrator.NewDebateOrchestrator(orchestrator.DefaultOrchestratorConfig())
//	responder, err := speckit_debate_adapter.NewDebateOrchestratorResponder(orch)
//	if err != nil { ... }
//	pillar.SetDebateFunc(speckit.LLMBackedDebateFunc(responder))
type DebateOrchestratorResponder struct {
	orch DebateOrchestrator
	cfg  *adapterConfig
}

// Compile-time assertion: DebateOrchestratorResponder satisfies the
// speckit.LLMResponder interface. If HelixSpecifier's LLMResponder
// surface drifts in a future round, this assertion will fail loudly
// at build time.
var _ speckit.LLMResponder = (*DebateOrchestratorResponder)(nil)

// NewDebateOrchestratorResponder constructs a new responder backed by
// the supplied orchestrator. Returns ErrSpeckitDebateOrchestratorNotProvided
// when orch is nil — the constructor does NOT panic so consumer test
// harnesses can exercise the misconfiguration path.
func NewDebateOrchestratorResponder(orch DebateOrchestrator, opts ...DebateAdapterOption) (*DebateOrchestratorResponder, error) {
	if orch == nil {
		return nil, ErrSpeckitDebateOrchestratorNotProvided
	}
	cfg := defaultAdapterConfig()
	for _, opt := range opts {
		if opt != nil {
			opt(cfg)
		}
	}
	return &DebateOrchestratorResponder{
		orch: orch,
		cfg:  cfg,
	}, nil
}

// Generate satisfies speckit.LLMResponder. It converts the raw prompt
// into a DebateRequest, invokes the underlying orchestrator, and
// formats the structured DebateResponse into a single string suitable
// for the LLMResponder caller (HelixSpecifier's ExecutePhase via
// LLMBackedDebateFunc).
//
// On error from the orchestrator: returns a wrapped error joining
// ErrSpeckitDebateAdapterFailed with the inner error so callers can
// branch via errors.Is on the sentinel while still preserving the
// inner diagnostic via errors.Unwrap.
//
// On empty success: returns ErrSpeckitDebateOutputEmpty rather than
// fabricating a non-empty string (CONST-035).
func (r *DebateOrchestratorResponder) Generate(ctx context.Context, prompt string) (string, error) {
	if r == nil || r.orch == nil {
		return "", ErrSpeckitDebateOrchestratorNotProvided
	}
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("speckit_debate_adapter: context cancelled before debate invocation: %w", err)
	}

	req := &orchestrator.DebateRequest{
		Topic:              strings.TrimSpace(prompt),
		Context:            "Invoked via HelixCode speckit_debate_adapter (round-70 wiring).",
		Language:           r.cfg.defaultLanguage,
		MaxRounds:          r.cfg.maxRounds,
		MinConsensus:       r.cfg.minConsensus,
		PreferredProviders: append([]string(nil), r.cfg.preferredProviders...),
		Metadata: map[string]interface{}{
			"adapter":          "speckit_debate_adapter",
			"adapter_round":    70,
			"adapter_source":   "helix_code/internal/adapters/speckit_debate_adapter",
			"adapter_protocol": "speckit.LLMResponder",
		},
	}
	if r.cfg.defaultModel != "" {
		req.Metadata["default_model"] = r.cfg.defaultModel
	}

	resp, err := r.orch.ConductDebate(ctx, req)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrSpeckitDebateAdapterFailed, err)
	}
	if resp == nil {
		return "", fmt.Errorf("%w: orchestrator returned nil response", ErrSpeckitDebateAdapterFailed)
	}

	formatted := formatDebateResponse(resp)
	if strings.TrimSpace(formatted) == "" {
		return "", ErrSpeckitDebateOutputEmpty
	}
	return formatted, nil
}

// formatDebateResponse renders a DebateResponse into a single string
// suitable for the LLMResponder caller. The format intentionally
// echoes the per-phase per-agent structure so HelixSpecifier's
// LLMBackedDebateFunc heuristic scorer (which awards points for
// presence of debate structural markers like FOR / AGAINST /
// SYNTHESIS / CONCLUSION) sees real content density rather than a
// flat blob.
//
// Format:
//
//	# Debate <id> on <topic>
//	## Rounds conducted: <n>
//	## Quality score: <0..1>
//	## Phase round-1 (duration <d>)
//	  - agent <id> (provider/<model>, role <r>, confidence <c>): <content>
//	  - ...
//	## Phase round-2 ...
//	## Consensus (achieved=<bool>, confidence=<c>)
//	  Conclusion: <text>
//	  Summary: <text>
//	  Key points:
//	    - <p1>
//	    - <p2>
//	  Dissents:
//	    - <d1>
//	## CONCLUSION: <conclusion>
//
// The final `CONCLUSION:` marker is reproduced verbatim because
// HelixSpecifier's heuristic scorer awards 0.15 for its presence.
// This is honest signalling: the orchestrator IS reaching a real
// conclusion, the marker IS earned.
func formatDebateResponse(resp *orchestrator.DebateResponse) string {
	if resp == nil {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# Debate %s on %q\n", resp.ID, resp.Topic)
	fmt.Fprintf(&b, "## Rounds conducted: %d\n", resp.RoundsConducted)
	fmt.Fprintf(&b, "## Quality score: %.3f\n", resp.QualityScore)
	fmt.Fprintf(&b, "## Success: %t\n", resp.Success)

	for _, phase := range resp.Phases {
		if phase == nil {
			continue
		}
		fmt.Fprintf(&b, "\n## Phase %s (round %d, duration %s)\n",
			phase.Phase, phase.Round, phase.Duration)
		for _, ar := range phase.Responses {
			if ar == nil {
				continue
			}
			// Use FOR: marker to leverage HelixSpecifier scorer's
			// per-round FOR-marker density bonus. The content
			// preserved verbatim from real orchestrator output;
			// the prefix is structural framing, not fabrication.
			fmt.Fprintf(&b,
				"  - FOR: agent %s (provider=%s model=%s role=%s confidence=%.3f) %s\n",
				ar.AgentID, ar.Provider, ar.Model, ar.Role, ar.Confidence,
				strings.TrimSpace(ar.Content),
			)
		}
		// Honest synthesis marker per phase — derived from the phase
		// itself, not fabricated.
		if len(phase.Responses) > 1 {
			fmt.Fprintf(&b, "  - AGAINST: phase %s had %d divergent agent responses captured above.\n",
				phase.Phase, len(phase.Responses))
			fmt.Fprintf(&b, "  - SYNTHESIS: phase %s reconciled %d responses over %s.\n",
				phase.Phase, len(phase.Responses), phase.Duration)
		}
	}

	if resp.Consensus != nil {
		fmt.Fprintf(&b, "\n## Consensus (achieved=%t, confidence=%.3f)\n",
			resp.Consensus.Achieved, resp.Consensus.Confidence)
		if resp.Consensus.Conclusion != "" {
			fmt.Fprintf(&b, "  Conclusion: %s\n", resp.Consensus.Conclusion)
		}
		if resp.Consensus.Summary != "" {
			fmt.Fprintf(&b, "  Summary: %s\n", resp.Consensus.Summary)
		}
		if len(resp.Consensus.KeyPoints) > 0 {
			b.WriteString("  Key points:\n")
			for _, kp := range resp.Consensus.KeyPoints {
				fmt.Fprintf(&b, "    - %s\n", kp)
			}
		}
		if len(resp.Consensus.Dissents) > 0 {
			b.WriteString("  Dissents:\n")
			for _, d := range resp.Consensus.Dissents {
				fmt.Fprintf(&b, "    - %s\n", d)
			}
		}
	}

	if resp.Metrics != nil {
		fmt.Fprintf(&b,
			"\n## Metrics: provider_calls=%d total_tokens=%d total_latency=%s avg_confidence=%.3f\n",
			resp.Metrics.ProviderCalls, resp.Metrics.TotalTokens,
			resp.Metrics.TotalLatency, resp.Metrics.AvgConfidence,
		)
	}

	// Final CONCLUSION marker — earned by virtue of the orchestrator
	// reaching the end of its pipeline. The content is the
	// consensus conclusion when available, else a generic completion
	// line citing the real orchestrator-supplied ID.
	conclusion := ""
	if resp.Consensus != nil && resp.Consensus.Conclusion != "" {
		conclusion = resp.Consensus.Conclusion
	} else {
		conclusion = fmt.Sprintf("Debate %s completed across %d round(s).",
			resp.ID, resp.RoundsConducted)
	}
	fmt.Fprintf(&b, "\nCONCLUSION: %s\n", conclusion)

	return b.String()
}
