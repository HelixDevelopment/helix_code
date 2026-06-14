// Package speckit_debate_adapter — round-71 wiring constructor tests.
//
// Per CONST-050(A) this *_test.go file MAY use a test-only invoker
// stub; wiring.go (production) imports only the real orchestrator.
package speckit_debate_adapter

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
)

func TestNewLLMBackedResponder_NilInvoker_ReturnsSentinel(t *testing.T) {
	t.Parallel()
	r, err := NewLLMBackedResponder(nil, []AgentSpec{{Provider: "openai", Model: "gpt-4o", Score: 0.9}})
	if r != nil {
		t.Fatalf("expected nil responder on nil invoker, got %+v", r)
	}
	if !errors.Is(err, ErrSpeckitDebateInvokerNotProvided) {
		t.Fatalf("expected ErrSpeckitDebateInvokerNotProvided, got %v", err)
	}
}

func TestNewLLMBackedResponder_NoAgents_ReturnsSentinel(t *testing.T) {
	t.Parallel()
	invoker := func(ctx context.Context, prompt string) (string, error) { return "ok", nil }
	r, err := NewLLMBackedResponder(invoker, nil)
	if r != nil {
		t.Fatalf("expected nil responder on empty agents, got %+v", r)
	}
	if !errors.Is(err, ErrSpeckitDebateNoAgents) {
		t.Fatalf("expected ErrSpeckitDebateNoAgents, got %v", err)
	}
}

func TestNewLLMBackedResponder_BadAgentScore_SurfacesError(t *testing.T) {
	t.Parallel()
	invoker := func(ctx context.Context, prompt string) (string, error) { return "ok", nil }
	// Score outside [0,1] is rejected by orchestrator.RegisterProvider —
	// the adapter must surface that error verbatim, not swallow it.
	_, err := NewLLMBackedResponder(invoker, []AgentSpec{{Provider: "openai", Model: "gpt-4o", Score: 2.0}})
	if err == nil {
		t.Fatal("expected RegisterProvider rejection for out-of-range score, got nil")
	}
	if !strings.Contains(err.Error(), "score") {
		t.Fatalf("expected score-related error surfaced verbatim, got %v", err)
	}
}

// TestNewLLMBackedResponder_RealInvokerDispatched proves the supplied
// ProviderInvoker is ACTUALLY called when a debate runs through the
// responder — i.e. the orchestrator is wired for real dispatch, not the
// synthesised-stub fallback. The stub invoker returns a unique marker
// string; the test asserts that marker surfaces in the formatted output
// AND that the invoker was invoked at least once.
func TestNewLLMBackedResponder_RealInvokerDispatched(t *testing.T) {
	t.Parallel()
	var calls atomic.Int64
	const marker = "REAL-INVOKER-CONTENT-7f3a"
	invoker := func(ctx context.Context, prompt string) (string, error) {
		calls.Add(1)
		return marker + " for prompt", nil
	}

	r, err := NewLLMBackedResponder(invoker,
		[]AgentSpec{
			{Provider: "openai", Model: "gpt-4o", Score: 0.9},
			{Provider: "anthropic", Model: "claude-3-5-sonnet", Score: 0.9},
		},
		WithMaxRounds(1),
	)
	if err != nil {
		t.Fatalf("constructor failed: %v", err)
	}

	out, err := r.Generate(context.Background(), "Is the round-71 wiring real?")
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if calls.Load() == 0 {
		t.Fatal("invoker was never called — orchestrator fell back to synthesised stub (anti-bluff failure)")
	}
	if !strings.Contains(out, marker) {
		t.Fatalf("real invoker content not present in output (got synthesised stub?):\n%s", out)
	}
	if strings.Contains(out, "synthesised") {
		t.Fatalf("output contains synthesised-stub marker — invoker not wired:\n%s", out)
	}
}
