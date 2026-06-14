//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/adapters/speckit_debate_adapter"
	"dev.helix.code/internal/llm"
)

// debate_e2e_test.go — REAL end-to-end exercise of the production
// speckit_debate_adapter multi-agent debate pipeline against a LIVE local
// Ollama, asserting a genuine multi-agent debate transcript reaches the caller.
//
// Anti-bluff (CONST-035 / Article XI §11.9 / CONST-050(A) / §11.4.107):
// this test makes NO stub, NO fake provider, NO canned response, and NO mock.
// The path exercised is the exact production wiring the /debate CLI command
// uses:
//
//   speckit_debate_adapter.NewLLMBackedResponder(invoker, []AgentSpec{...})
//     -> orchestrator.NewOrchestrator(WithProviderInvoker(invoker))
//     -> ConductDebate (real 8-phase MASTER protocol)
//     -> per agent / per round: invoker(ctx, builtPrompt)
//        -> llm.OllamaProvider.Generate -> REAL HTTP POST to
//           http://localhost:11434/api/chat -> qwen2.5:0.5b produces real tokens
//     -> formatDebateResponse -> a single real transcript string.
//
// The `invoker` here is a closure over a REAL *llm.OllamaProvider's Generate
// (mirroring how llm_generate_e2e_test.go constructs a real provider and how
// the /debate command wraps llm.Provider.Generate into a ProviderInvoker).
//
// Anti-bluff assertions: the returned transcript MUST be non-empty, MUST
// reference the debate topic, MUST carry the real per-agent provider/model
// framing, and MUST NOT be the orchestrator's self-labelled
// "[synthesised ... awaiting provider wiring]" deterministic-stub string (the
// ACK-STUB path that fires when NO invoker is wired). It must also NOT carry an
// "[invoker-error" marker (which would mean the real LLM call failed).
//
// Run:
//   go test -tags=integration -run TestDebateE2E ./tests/integration/ -count=1 -v
//
// Per CONST-050(A) integration tests exercise the real system. If Ollama is not
// reachable OR the qwen2.5:0.5b model is not installed, the test SKIPs with an
// explicit reason (SKIP-OK) rather than bluffing a PASS — an honest documented
// absence, never a fabricated debate.

// debateModel is the small model this test drives. liveDebateModel verifies it
// is genuinely installed on the live Ollama before the debate runs.
const debateModel = "qwen2.5:0.5b"

// liveDebateModel reports whether the live Ollama at ollamaEndpoint is reachable
// AND has debateModel installed. (ollamaEndpoint + ollamaTags are declared in
// llm_generate_e2e_test.go within this same package.)
func liveDebateModel(t *testing.T) (reachable, hasModel bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ollamaEndpoint+"/api/tags", nil)
	if err != nil {
		return false, false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, false
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return false, false
	}
	var tags ollamaTags
	if decErr := json.NewDecoder(resp.Body).Decode(&tags); decErr != nil {
		return true, false
	}
	for _, m := range tags.Models {
		if m.Name == debateModel {
			return true, true
		}
	}
	return true, false
}

// TestDebateE2E drives the real speckit_debate_adapter LLM-backed responder
// through a live Ollama and asserts a genuine multi-agent debate transcript.
func TestDebateE2E(t *testing.T) {
	reachable, hasModel := liveDebateModel(t)
	if !reachable {
		t.Skip("SKIP-OK: local Ollama not reachable at " + ollamaEndpoint + "; cannot exercise the real debate path") //nolint
	}
	if !hasModel {
		t.Skip("SKIP-OK: local Ollama reachable but model " + debateModel + " not installed; pull it (`ollama pull " + debateModel + "`) to exercise the debate path") //nolint
	}
	t.Logf("targeting live Ollama model %q via %s for multi-agent debate", debateModel, ollamaEndpoint)

	// Build a REAL Ollama provider pointed at the live local server. isRunning is
	// set true inside NewOllamaProvider, so it is immediately usable.
	provider, err := llm.NewOllamaProvider(llm.OllamaConfig{
		BaseURL:      ollamaEndpoint,
		DefaultModel: debateModel,
		Timeout:      120 * time.Second,
	})
	require.NoError(t, err, "constructing the real Ollama provider must succeed")

	// Wrap the real provider's Generate into a ProviderInvoker closure — the
	// exact shape the orchestrator drives per agent / per round. This mirrors the
	// /debate CLI command wrapping llm.Provider.Generate. NO fabrication: every
	// returned string is the model's genuine completion text.
	invoker := func(ctx context.Context, prompt string) (string, error) {
		resp, gerr := provider.Generate(ctx, &llm.LLMRequest{
			ID:    uuid.New(),
			Model: debateModel,
			Messages: []llm.Message{
				{Role: "user", Content: prompt},
			},
			MaxTokens:   192,
			Temperature: 0.7,
			TopP:        0.9,
		})
		if gerr != nil {
			return "", gerr
		}
		return resp.Content, nil
	}

	// Construct the production responder via the real adapter constructor with
	// TWO debate agents, both driven by the live qwen model. Keep rounds low so
	// the 0.5b model finishes within the test budget.
	responder, err := speckit_debate_adapter.NewLLMBackedResponder(
		invoker,
		[]speckit_debate_adapter.AgentSpec{
			{Provider: "ollama", Model: debateModel, Score: 0.9},
			{Provider: "ollama", Model: debateModel, Score: 0.85},
		},
		speckit_debate_adapter.WithMaxRounds(1),
		speckit_debate_adapter.WithDefaultLanguage("en"),
	)
	require.NoError(t, err, "the real LLM-backed responder must construct")
	require.NotNil(t, responder)

	const topic = "Should a cache use LRU or LFU eviction?"
	ctx, cancel := context.WithTimeout(context.Background(), 280*time.Second)
	defer cancel()

	transcript, err := responder.Generate(ctx, topic)
	require.NoError(t, err, "the real debate must complete without error against live Ollama")

	t.Logf("=== REAL DEBATE TRANSCRIPT (len=%d) ===\n%s\n=== END TRANSCRIPT ===", len(transcript), transcript)

	// --- Anti-bluff assertions on the REAL transcript ---

	require.NotEmpty(t, strings.TrimSpace(transcript),
		"the real debate transcript must be non-empty")

	// It must NOT be the orchestrator's ACK-STUB path (fires only when NO invoker
	// is wired). Its presence would prove the LLM was never actually called.
	require.NotContains(t, transcript, "[synthesised",
		"transcript must NOT contain the synthesised-stub marker — that path means the real LLM was never invoked; got the deterministic stub instead of real model output")
	require.NotContains(t, transcript, "awaiting provider wiring",
		"transcript must NOT carry the 'awaiting provider wiring' stub phrase")

	// It must NOT carry an invoker-error marker — that would mean the real LLM
	// call failed and the orchestrator continued with an error placeholder.
	require.NotContains(t, transcript, "[invoker-error",
		"transcript must NOT carry an invoker-error marker; the real provider calls must have succeeded, got: %s", transcript)

	// The transcript must reference the real topic (the orchestrator echoes it
	// into the formatted output header).
	assert.Contains(t, transcript, "LRU",
		"the real debate transcript must reference the debate topic (LRU); got %q", transcript)

	// The real per-agent framing proves agent content was sourced from the wired
	// invoker, with the real provider+model echoed.
	assert.Contains(t, transcript, "provider=ollama",
		"transcript must carry the real per-agent provider framing")
	assert.Contains(t, transcript, "model="+debateModel,
		"transcript must carry the real per-agent model framing")

	// A genuine multi-agent debate over 2 agents must show the FOR: agent markers
	// the adapter emits per captured agent response.
	assert.GreaterOrEqual(t, strings.Count(transcript, "FOR: agent"), 2,
		"a real 2-agent debate must capture at least 2 agent responses; got transcript: %s", transcript)

	// The pipeline must reach a real CONCLUSION marker (earned by completing the
	// orchestrator pipeline end-to-end).
	assert.Contains(t, transcript, "CONCLUSION:",
		"the real debate must reach the orchestrator's CONCLUSION marker")
}
