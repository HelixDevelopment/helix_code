//go:build helixagentlive

// LIVE anti-bluff test (CONST-035 / §11.4.5 captured-evidence): exercises the
// HelixAgent adapter against the REAL running HelixAgent server. Build-tagged
// so it runs ONLY when explicitly requested:
//
//	go test -tags helixagentlive -run Live ./internal/llm/providers/helixagent/
//
// Base URL is resolved from HELIXAGENT_BASE_URL (default DefaultBaseURL). If
// the agent is not reachable the test SKIPs with a reason (§11.4.3) — the
// non-live unit tests still prove the mapping.
package helixagent

import (
	"context"
	"os"
	"testing"
	"time"

	"dev.helix.code/internal/llm"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func liveBaseURL() string {
	if v := os.Getenv("HELIXAGENT_BASE_URL"); v != "" {
		return v
	}
	return DefaultBaseURL
}

func TestLive_GenerateAgainstRunningAgent(t *testing.T) {
	p := New(liveBaseURL())

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	if !p.IsAvailable(ctx) {
		t.Skipf("SKIP-OK: HelixAgent not reachable at %s — start the agent on :7061 to run the live test", p.baseURL)
	}

	// Real model discovery from the live engine.
	models := p.GetModels()
	require.NotEmpty(t, models, "live /v1/models returned at least one model")
	t.Logf("LIVE models (%d): %+v", len(models), modelIDs(models))

	// Real chat completion through the real 25-provider engine.
	req := &llm.LLMRequest{
		ID:        uuid.New(),
		Model:     "helixagent-llm",
		Messages:  []llm.Message{{Role: "user", Content: "Reply with exactly: PONG"}},
		MaxTokens: 64,
	}
	resp, err := p.Generate(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.NotEmpty(t, resp.Content, "live HelixAgent returned non-empty content")
	t.Logf("LIVE HelixAgent response: %q (finish=%s, usage=%+v, model=%v)",
		resp.Content, resp.FinishReason, resp.Usage, resp.ProviderMetadata["helixagent_model"])
}

// TestLive_ToolCalling_RealEngine sends a REAL OpenAI tool-calling request to
// the running HelixAgent server and asserts the engine returned either a real
// tool_call request OR a real textual answer — proving the adapter's
// tool-forwarding + tool_call parsing works end-to-end against the actual
// engine, not a fake. An empty content + empty tool_calls + clean finish is a
// non-working result and FAILs (CONST-035 anti-bluff).
func TestLive_ToolCalling_RealEngine(t *testing.T) {
	p := New(liveBaseURL())

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	if !p.IsAvailable(ctx) {
		t.Skipf("SKIP-OK: HelixAgent not reachable at %s — start the agent on :7061 to run the live tool-calling test", p.baseURL)
	}

	tools := []llm.Tool{{
		Type: "function",
		Function: llm.ToolFunction{
			Name:        "get_current_time",
			Description: "Return the current server time as an ISO-8601 string",
			Parameters: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
	}}

	resp, err := p.Generate(ctx, &llm.LLMRequest{
		ID:         uuid.New(),
		Messages:   []llm.Message{{Role: "user", Content: "What time is it right now? Use the get_current_time tool."}},
		Tools:      tools,
		ToolChoice: "auto",
	})
	require.NoError(t, err)
	require.NotNil(t, resp)

	t.Logf("LIVE tool-calling base=%s finish_reason=%q tool_calls=%d content_len=%d model=%v",
		p.baseURL, resp.FinishReason, len(resp.ToolCalls), len(resp.Content), resp.ProviderMetadata["helixagent_model"])
	for i, tc := range resp.ToolCalls {
		t.Logf("LIVE tool_call[%d]: id=%q name=%q args=%v", i, tc.ID, tc.Function.Name, tc.Function.Arguments)
	}

	// Anti-bluff: a real engine MUST return a tool_call request OR a real answer.
	if len(resp.ToolCalls) == 0 && resp.Content == "" {
		t.Fatalf("LIVE result empty: no tool_calls AND no content (finish_reason=%q) — feature not working", resp.FinishReason)
	}
	if len(resp.ToolCalls) > 0 {
		assert.NotEmpty(t, resp.ToolCalls[0].Function.Name, "live tool_call has a function name (parse OK)")
		assert.NotNil(t, resp.ToolCalls[0].Function.Arguments, "live tool_call arguments decoded into a map (parse OK)")
	}
	t.Logf("LIVE PASS: HelixAgent returned a usable result (tool_calls=%d, content_len=%d)", len(resp.ToolCalls), len(resp.Content))
}

func modelIDs(models []llm.ModelInfo) []string {
	out := make([]string, 0, len(models))
	for _, m := range models {
		out = append(out, m.ID)
	}
	return out
}
