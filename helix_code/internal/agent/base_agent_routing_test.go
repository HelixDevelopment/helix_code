// base_agent_routing_test.go — speed-programme Phase 3, task P3-T01.
//
// Unit tests for the small-model routing wiring in BaseAgent. Mocks are
// permitted here per CONST-050(A) — these are unit tests. The verifier model
// catalogue is supplied by an in-package mock VerifiedModelSource; the LLM
// calls go through the existing MockLLMProvider with a recording generateFunc.
package agent

import (
	"context"
	"testing"

	"dev.helix.code/internal/agent/task"
	"dev.helix.code/internal/llm"
	"dev.helix.code/internal/llm/routing"
)

// routingMockSource feeds the VerifierResolver a fixed verifier-style
// catalogue. The model list is verifier metadata — never hardcoded into
// routing logic (CONST-036/037).
type routingMockSource struct{}

func (routingMockSource) VerifiedModels(_ context.Context) ([]routing.TierModel, error) {
	return []routing.TierModel{
		{ID: "frontier-premium", VerifierTier: 1, Score: 9.4, Verified: true},
		{ID: "small-fast", VerifierTier: 3, Score: 7.1, Verified: true},
	}, nil
}

func newAgentRouter(t *testing.T, policy *routing.Policy) *routing.Router {
	t.Helper()
	r, err := routing.NewRouter(policy, routing.NewVerifierResolver(routingMockSource{}))
	if err != nil {
		t.Fatalf("NewRouter: %v", err)
	}
	return r
}

// TestAgentTaskClass_Mapping asserts the agent task-type → routing-class map:
// trivial subtasks route to cheap classes, reasoning-heavy tasks route to
// routing.TaskReasoning (frontier tier).
func TestAgentTaskClass_Mapping(t *testing.T) {
	trivial := []task.TaskType{
		task.TaskTypeReview, task.TaskTypeResearch,
		task.TaskTypeDocumentation, task.TaskTypeAnalysis,
	}
	for _, tt := range trivial {
		class, ok := agentTaskClass(tt)
		if !ok {
			t.Errorf("task type %s should map to a routing class", tt)
		}
		if routing.DefaultPolicy().InitialTier(class) != routing.TierSmall {
			t.Errorf("trivial task type %s mapped to class %s, which is not small-tier", tt, class)
		}
	}
	reasoning := []task.TaskType{
		task.TaskTypeCodeGeneration, task.TaskTypeCodeEdit,
		task.TaskTypePlanning, task.TaskTypeDebugging,
	}
	for _, tt := range reasoning {
		class, ok := agentTaskClass(tt)
		if !ok {
			t.Errorf("task type %s should map to a routing class", tt)
		}
		if routing.DefaultPolicy().InitialTier(class) != routing.TierFrontier {
			t.Errorf("reasoning task type %s mapped to class %s, which is not frontier-tier", tt, class)
		}
	}
}

// TestResponseConfidence asserts the finish-reason → confidence mapping that
// drives escalation.
func TestResponseConfidence(t *testing.T) {
	cases := []struct {
		finish string
		want   float64
	}{
		{"", 1.0}, {"stop", 1.0}, {"end_turn", 1.0},
		{"length", 0.0}, {"content_filter", 0.0}, {"unknown-reason", 0.0},
	}
	for _, c := range cases {
		got := responseConfidence(&llm.LLMResponse{FinishReason: c.finish})
		if got != c.want {
			t.Errorf("responseConfidence(%q) = %.1f, want %.1f", c.finish, got, c.want)
		}
	}
	if responseConfidence(nil) != 0.0 {
		t.Error("responseConfidence(nil) should be 0.0")
	}
}

// TestBaseAgent_RoutesTrivialTaskToSmallModel asserts that a trivial agent
// task (review) with a confident response runs on the small model when a
// router is wired.
func TestBaseAgent_RoutesTrivialTaskToSmallModel(t *testing.T) {
	var modelsSeen []string
	mp := NewMockLLMProvider()
	mp.generateFunc = func(_ context.Context, req *llm.LLMRequest) (*llm.LLMResponse, error) {
		modelsSeen = append(modelsSeen, req.Model)
		return &llm.LLMResponse{Content: `{"issues": []}`, FinishReason: "stop"}, nil
	}

	agent := NewBaseAgentFromConfig(&AgentConfig{
		ID: "routing-agent", Type: AgentTypeCoding, Name: "Routing Agent",
		Capabilities: []Capability{CapabilityCodeGeneration},
	})
	agent.SetLLMProvider(mp)
	agent.SetSubtaskRouter(newAgentRouter(t, routing.DefaultPolicy()))

	reviewTask := task.NewTask(task.TaskTypeReview, "Review", "Review code", task.PriorityNormal)
	reviewTask.Input = map[string]interface{}{"code": "func x() {}"}

	if _, err := agent.executeTaskWithLLM(context.Background(), reviewTask); err != nil {
		t.Fatalf("executeTaskWithLLM: %v", err)
	}
	if len(modelsSeen) != 1 || modelsSeen[0] != "small-fast" {
		t.Errorf("trivial review task models seen = %v, want [small-fast]", modelsSeen)
	}
}

// TestBaseAgent_EscalatesLowConfidenceToFrontier asserts that a truncated
// small-model response escalates the trivial task to the frontier model — the
// final response comes from the frontier model (no quality regression).
func TestBaseAgent_EscalatesLowConfidenceToFrontier(t *testing.T) {
	var modelsSeen []string
	mp := NewMockLLMProvider()
	mp.generateFunc = func(_ context.Context, req *llm.LLMRequest) (*llm.LLMResponse, error) {
		modelsSeen = append(modelsSeen, req.Model)
		if req.Model == "small-fast" {
			return &llm.LLMResponse{Content: `{"partial":`, FinishReason: "length"}, nil
		}
		return &llm.LLMResponse{Content: `{"issues": ["full review"]}`, FinishReason: "stop"}, nil
	}

	agent := NewBaseAgentFromConfig(&AgentConfig{
		ID: "routing-agent", Type: AgentTypeCoding, Name: "Routing Agent",
		Capabilities: []Capability{CapabilityCodeGeneration},
	})
	agent.SetLLMProvider(mp)
	agent.SetSubtaskRouter(newAgentRouter(t, routing.DefaultPolicy()))

	reviewTask := task.NewTask(task.TaskTypeReview, "Review", "Review code", task.PriorityNormal)
	reviewTask.Input = map[string]interface{}{"code": "func x() {}"}

	if _, err := agent.executeTaskWithLLM(context.Background(), reviewTask); err != nil {
		t.Fatalf("executeTaskWithLLM: %v", err)
	}
	if len(modelsSeen) != 2 || modelsSeen[0] != "small-fast" || modelsSeen[1] != "frontier-premium" {
		t.Errorf("escalation models seen = %v, want [small-fast frontier-premium]", modelsSeen)
	}
}

// TestBaseAgent_NoRouterRunsUnchanged asserts the no-regression default: with
// no router wired the request runs on the provider's first model exactly as
// before.
func TestBaseAgent_NoRouterRunsUnchanged(t *testing.T) {
	var modelsSeen []string
	mp := NewMockLLMProvider()
	mp.generateFunc = func(_ context.Context, req *llm.LLMRequest) (*llm.LLMResponse, error) {
		modelsSeen = append(modelsSeen, req.Model)
		return &llm.LLMResponse{Content: `{"issues": []}`, FinishReason: "stop"}, nil
	}

	agent := NewBaseAgentFromConfig(&AgentConfig{
		ID: "plain-agent", Type: AgentTypeCoding, Name: "Plain Agent",
		Capabilities: []Capability{CapabilityCodeGeneration},
	})
	agent.SetLLMProvider(mp)
	// No SetSubtaskRouter call — routing disabled.

	reviewTask := task.NewTask(task.TaskTypeReview, "Review", "Review code", task.PriorityNormal)
	reviewTask.Input = map[string]interface{}{"code": "func x() {}"}

	if _, err := agent.executeTaskWithLLM(context.Background(), reviewTask); err != nil {
		t.Fatalf("executeTaskWithLLM: %v", err)
	}
	if len(modelsSeen) != 1 || modelsSeen[0] != "mock-model" {
		t.Errorf("no-router models seen = %v, want [mock-model] (provider's first model)", modelsSeen)
	}
}

// TestBaseAgent_ForceFrontierDisablesRouting asserts the config-gated
// frontier-only policy sends every subtask straight to the frontier model.
func TestBaseAgent_ForceFrontierDisablesRouting(t *testing.T) {
	var modelsSeen []string
	mp := NewMockLLMProvider()
	mp.generateFunc = func(_ context.Context, req *llm.LLMRequest) (*llm.LLMResponse, error) {
		modelsSeen = append(modelsSeen, req.Model)
		return &llm.LLMResponse{Content: `{"issues": []}`, FinishReason: "length"}, nil // would escalate if routed
	}

	agent := NewBaseAgentFromConfig(&AgentConfig{
		ID: "ff-agent", Type: AgentTypeCoding, Name: "FF Agent",
		Capabilities: []Capability{CapabilityCodeGeneration},
	})
	agent.SetLLMProvider(mp)
	agent.SetSubtaskRouter(newAgentRouter(t, routing.FrontierOnlyPolicy()))

	reviewTask := task.NewTask(task.TaskTypeReview, "Review", "Review code", task.PriorityNormal)
	reviewTask.Input = map[string]interface{}{"code": "func x() {}"}

	if _, err := agent.executeTaskWithLLM(context.Background(), reviewTask); err != nil {
		t.Fatalf("executeTaskWithLLM: %v", err)
	}
	if len(modelsSeen) != 1 || modelsSeen[0] != "frontier-premium" {
		t.Errorf("force-frontier models seen = %v, want [frontier-premium]", modelsSeen)
	}
}
