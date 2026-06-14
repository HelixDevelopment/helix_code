// Package speckit_debate_adapter — round-71 consumer-side wiring helper.
//
// adapter.go (round-70) defined DebateOrchestratorResponder, which
// satisfies speckit.LLMResponder by delegating to a DebateOrchestrator
// interface. That constructor accepts an ALREADY-BUILT orchestrator, so
// it cannot itself inject a ProviderInvoker (the orchestrator's
// WithProviderInvoker is an Option on the *concrete* *orchestrator.
// Orchestrator type, applied at construction time).
//
// Round-71 (this file) closes the remaining reachability gap WITHOUT
// modifying the decoupled DebateOrchestrator submodule (CONST-051(B)):
// it provides NewLLMBackedResponder, a consumer-side constructor that
// builds a REAL, invoker-wired *orchestrator.Orchestrator from a
// ProviderInvoker the caller supplies, then wraps it in a
// DebateOrchestratorResponder.
//
// Why this is the honest seam (§11.4 / CONST-035 anti-bluff):
//   - Without a ProviderInvoker, orchestrator.NewDebateOrchestrator
//     runs in deterministic mode and emits self-labelled
//     "[synthesised ... awaiting provider wiring]" stub content
//     (orchestrator.go synthesiseContent). That is an ACKNOWLEDGED-STUB
//     path — useful for tests, NOT real model output.
//   - The orchestrator's own registry-based NewProviderInvoker is
//     hardcoded NotYetImplemented (orchestrator/api.go), so it cannot
//     be used to reach a real LLM.
//   - The ONLY way to make the orchestrator produce real LLM output is
//     to inject a ProviderInvoker via WithProviderInvoker. HelixCode's
//     provider layer exposes exactly that shape — e.g.
//     (*providers.LLMProviderWrapper).Generate(ctx, prompt) (string,
//     error). The caller passes that method value here.
//
// This file imports ONLY the real orchestrator package (no mocks,
// CONST-050(A)) and adds no project-specific context to the submodule.
package speckit_debate_adapter

import (
	"context"
	"errors"

	"digital.vasic.debate/orchestrator"
)

// ProviderInvoker is the consumer-facing callback shape the caller
// supplies to drive real LLM dispatch. It is identical to
// orchestrator.ProviderInvoker (func(ctx, prompt) (string, error)) and
// matches HelixCode's (*providers.LLMProviderWrapper).Generate method
// value, so a caller can pass that method directly:
//
//	wrapper := /* a *providers.LLMProviderWrapper backed by a real AIProvider */
//	responder, err := speckit_debate_adapter.NewLLMBackedResponder(
//	    wrapper.Generate,
//	    []AgentSpec{
//	        {Provider: "openai", Model: "gpt-4o", Score: 0.9},
//	        {Provider: "anthropic", Model: "claude-3-5-sonnet", Score: 0.9},
//	    },
//	)
//	if err != nil { return err }
//	pillar.SetDebateFunc(speckit.LLMBackedDebateFunc(responder))
//
// Keeping a local alias (rather than re-exporting the orchestrator
// type) lets consumers depend on this adapter package alone for the
// callback shape.
type ProviderInvoker = orchestrator.ProviderInvoker

// ErrSpeckitDebateInvokerNotProvided fires when NewLLMBackedResponder
// is called with a nil ProviderInvoker. Per CONST-035 the adapter
// refuses to construct an LLM-backed responder that would silently fall
// back to the orchestrator's synthesised-stub path.
var ErrSpeckitDebateInvokerNotProvided = errors.New("speckit_debate_adapter: ProviderInvoker is nil — pass a real provider callback (e.g. (*providers.LLMProviderWrapper).Generate) to NewLLMBackedResponder (round-71 §11.4 anti-bluff: refusing to fall back to synthesised-stub orchestrator output)")

// ErrSpeckitDebateNoAgents fires when NewLLMBackedResponder is called
// with no AgentSpec entries. A debate with zero registered agents
// produces no agent content, so the adapter rejects it up-front rather
// than letting ConductDebate emit an empty response later.
var ErrSpeckitDebateNoAgents = errors.New("speckit_debate_adapter: at least one AgentSpec is required — a debate with no agents produces no model output (round-71 §11.4 anti-bluff)")

// AgentSpec describes one debate participant to register on the
// orchestrator before any debate runs. Provider+Model are passed
// through to the wired ProviderInvoker's prompt framing; Score
// (clamped to [0,1]) weights the agent in selection/aggregation.
type AgentSpec struct {
	Provider string
	Model    string
	Score    float64
}

// NewLLMBackedResponder builds a real, invoker-wired orchestrator from
// the supplied ProviderInvoker + agent roster and returns a
// DebateOrchestratorResponder around it. Unlike
// NewDebateOrchestratorResponder (which accepts an externally-built
// orchestrator), this constructor guarantees the orchestrator is wired
// for REAL LLM dispatch — there is no synthesised-stub fallback path.
//
// invoker MUST be non-nil (a real provider callback). agents MUST
// contain at least one spec. opts are forwarded to the underlying
// DebateOrchestratorResponder (WithMaxRounds, WithDefaultLanguage, …).
//
// On nil invoker → ErrSpeckitDebateInvokerNotProvided.
// On empty agents → ErrSpeckitDebateNoAgents.
// On a RegisterProvider rejection (bad name/model/score) → that error,
// surfaced verbatim (never swallowed, §11.4).
func NewLLMBackedResponder(invoker ProviderInvoker, agents []AgentSpec, opts ...DebateAdapterOption) (*DebateOrchestratorResponder, error) {
	if invoker == nil {
		return nil, ErrSpeckitDebateInvokerNotProvided
	}
	if len(agents) == 0 {
		return nil, ErrSpeckitDebateNoAgents
	}

	orch := orchestrator.NewOrchestrator(
		nil, // no registry — agents registered explicitly below
		nil, // no lesson bank
		orchestrator.DefaultOrchestratorConfig(),
		orchestrator.WithProviderInvoker(invoker),
	)

	for _, a := range agents {
		if err := orch.RegisterProvider(a.Provider, a.Model, a.Score); err != nil {
			return nil, err
		}
	}

	return NewDebateOrchestratorResponder(orch, opts...)
}

// touch keeps context imported for the doc-comment example's signature
// even if a future edit drops the only direct use; harmless no-op.
var _ = context.Background
