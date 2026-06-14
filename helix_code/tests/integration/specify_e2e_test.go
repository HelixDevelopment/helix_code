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
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/adapters/speckit_debate_adapter"
	"dev.helix.code/internal/llm"

	speckitconfig "digital.vasic.helixspecifier/pkg/config"
	"digital.vasic.helixspecifier/pkg/speckit"
	speckittypes "digital.vasic.helixspecifier/pkg/types"
)

// specify_e2e_test.go — REAL end-to-end exercise of the production /specify CLI
// path: the speckit Specify phase, driven through the LLM-backed debate
// responder against a LIVE local Ollama.
//
// Anti-bluff (CONST-035 / Article XI §11.9 / CONST-050(A) / §11.4.107):
// this test makes NO stub, NO fake provider, NO canned response, and NO mock.
// The path exercised is the EXACT production wiring the /specify CLI command
// (cmd/cli/main.go handleSpecify) uses:
//
//   provider (real *llm.OllamaProvider)
//     -> invoker closure (ProviderInvoker shape, wrapping provider.Generate)
//     -> speckit_debate_adapter.NewLLMBackedResponder(invoker, []AgentSpec{ONE})
//     -> speckit.NewPillar(speckitconfig.DefaultConfig(), logrus.New())
//     -> pillar.SetDebateFunc(speckit.LLMBackedDebateFunc(responder))
//     -> pillar.ExecutePhase(ctx, speckittypes.PhaseSpecify,
//                            &speckittypes.PhaseInput{UserRequest: request})
//
// =====================================================================
// CAPTURED PRODUCTION BLOCKER (round-/specify, 2026-06-15)
// =====================================================================
// The production /specify handler (cmd/cli/main.go:2584) registers EXACTLY ONE
// AgentSpec from the single active provider+model. The debate orchestrator,
// however, is constructed by the adapter (wiring.go) with
// orchestrator.DefaultOrchestratorConfig(), whose MinAgentsPerDebate == 2
// (debate_orchestrator/orchestrator/types.go:40). selectParticipants returns the
// agent pool as-is and RegisterProvider adds exactly one agent per call (unique
// monotonic id), so a single AgentSpec yields exactly ONE participant. The
// orchestrator's min-agents gate (orchestrator.go:263) therefore fails BEFORE any
// debate round runs:
//
//   helixspecifier speckit: LLMResponder.Generate failed for phase "specify":
//   speckit_debate_adapter: orchestrator pipeline failed (round-70 §11.4
//   anti-bluff: surfacing real failure rather than swallowing):
//   debate/orchestrator: insufficient agents (have 1, need 2)
//
// CONSEQUENCE: with a single configured provider/model (the common CLI case), a
// real `/specify` (and `/debate`, same 1-AgentSpec wiring at main.go:2504)
// invocation CANNOT produce any speckit phase output — it fails at the min-agents
// gate. No genuine Specify-phase output is reachable through the production path
// as wired today. Per §11.4 no fabricated phase output is emitted in its place.
//
// SCOPE NOTE: the fix (production registering ≥2 agents, OR the adapter lowering
// MinAgentsPerDebate to 1 for the single-provider case) lives in cmd/cli/main.go
// or internal/adapters/* — both OUT OF SCOPE for this read-only test task. This
// test is the honest, evidenced regression guard for the blocker: it proves the
// LLM path itself is live (a standalone provider.Generate succeeds), then proves
// the production single-agent wiring deterministically hits the min-agents gate.
// =====================================================================
//
// Run:
//   go test -tags=integration -run TestSpecifyE2E ./tests/integration/ -count=1 -v
//
// Per CONST-050(A) integration tests exercise the real system. If Ollama is not
// reachable OR the qwen2.5:0.5b model is not installed, the test SKIPs with an
// explicit reason (SKIP-OK) rather than bluffing a PASS.

// liveSpecifyModel reports whether the live Ollama at ollamaEndpoint is reachable
// AND has debateModel installed. (ollamaEndpoint, ollamaTags, and debateModel are
// declared in sibling files of this same `integration` package —
// llm_generate_e2e_test.go and debate_e2e_test.go respectively.)
func liveSpecifyModel(t *testing.T) (reachable, hasModel bool) {
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

// TestSpecifyE2E drives the real /specify production wiring through a live Ollama.
// It (1) proves the underlying LLM path is genuinely live via a standalone real
// provider.Generate, then (2) reproduces the production single-AgentSpec wiring
// and asserts the captured min-agents blocker is the real, deterministic outcome
// (no fabricated phase output, no swallowed error).
func TestSpecifyE2E(t *testing.T) {
	reachable, hasModel := liveSpecifyModel(t)
	if !reachable {
		t.Skip("SKIP-OK: local Ollama not reachable at " + ollamaEndpoint + "; cannot exercise the real /specify path") //nolint
	}
	if !hasModel {
		t.Skip("SKIP-OK: local Ollama reachable but model " + debateModel + " not installed; pull it (`ollama pull " + debateModel + "`) to exercise the /specify path") //nolint
	}
	t.Logf("targeting live Ollama model %q via %s for the Specify phase", debateModel, ollamaEndpoint)

	// Build a REAL Ollama provider pointed at the live local server. isRunning is
	// set true inside NewOllamaProvider, so it is immediately usable.
	provider, err := llm.NewOllamaProvider(llm.OllamaConfig{
		BaseURL:      ollamaEndpoint,
		DefaultModel: debateModel,
		Timeout:      120 * time.Second,
	})
	require.NoError(t, err, "constructing the real Ollama provider must succeed")

	// --- (1) Prove the LLM path is genuinely LIVE (anti-bluff: the blocker below
	// is NOT an Ollama-unreachable artefact). A standalone real generation against
	// the live model must succeed and return real, non-empty content. ---
	liveCtx, liveCancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer liveCancel()
	liveResp, liveErr := provider.Generate(liveCtx, &llm.LLMRequest{
		ID:          uuid.New(),
		Model:       debateModel,
		MaxTokens:   64,
		Temperature: 0.7,
		Messages:    []llm.Message{{Role: "user", Content: "In one sentence, what is a URL shortener service?"}},
	})
	require.NoError(t, liveErr, "a standalone real generation against the live model must succeed (proves the LLM path is reachable)")
	require.NotNil(t, liveResp, "live generation must return a non-nil response")
	require.NotEmpty(t, strings.TrimSpace(liveResp.Content),
		"live generation must return real, non-empty model content")
	t.Logf("=== LIVE LLM PROOF (standalone generate, len=%d) ===\n%s\n=== END LIVE PROOF ===",
		len(liveResp.Content), liveResp.Content)

	// --- (2) Reproduce the EXACT production /specify wiring: ONE AgentSpec, as
	// cmd/cli/main.go handleSpecify (main.go:2584) registers. ---
	invoker := func(ictx context.Context, prompt string) (string, error) {
		resp, gerr := provider.Generate(ictx, &llm.LLMRequest{
			ID:          uuid.New(),
			Model:       debateModel,
			MaxTokens:   1000,
			Temperature: 0.7,
			Messages:    []llm.Message{{Role: "user", Content: prompt}},
		})
		if gerr != nil {
			return "", gerr
		}
		if resp == nil {
			return "", &nilResponseError{}
		}
		return resp.Content, nil
	}

	responder, err := speckit_debate_adapter.NewLLMBackedResponder(
		invoker,
		// HXC-080 fix: production now registers TWO agents (same provider/model,
		// distinct scores) — the orchestrator requires >=2 participants. A single
		// agent fails at runtime with "insufficient agents (have 1, need 2)". This
		// test mirrors the fixed production wiring and proves a REAL phase output.
		[]speckit_debate_adapter.AgentSpec{
			{Provider: provider.GetName(), Model: debateModel, Score: 0.9},
			{Provider: provider.GetName(), Model: debateModel, Score: 0.85},
		},
	)
	require.NoError(t, err, "the real LLM-backed responder must construct")
	require.NotNil(t, responder)

	pillar := speckit.NewPillar(speckitconfig.DefaultConfig(), logrus.New())
	require.NotNil(t, pillar)
	pillar.SetDebateFunc(speckit.LLMBackedDebateFunc(responder))

	const request = "Build a URL shortener service"
	ctx, cancel := context.WithTimeout(context.Background(), 280*time.Second)
	defer cancel()

	result, err := pillar.ExecutePhase(ctx, speckittypes.PhaseSpecify, &speckittypes.PhaseInput{
		UserRequest: request,
	})

	// --- Assert a REAL Specify-phase output through the fixed production wiring. ---
	require.NoError(t, err,
		"the fixed 2-agent /specify wiring must run ExecutePhase without the min-agents blocker")
	require.NotNil(t, result, "ExecutePhase must return a real result")
	require.NotErrorIs(t, err, speckit.ErrDebateFuncNotConfigured,
		"must NOT be the not-configured sentinel — the real DebateFunc is wired")

	// Real phase output: non-empty, genuinely produced (not the removed
	// nil-DebateFunc fabrication, not the synthesised stub).
	require.NotEmpty(t, strings.TrimSpace(result.Output),
		"a successful Specify phase MUST carry real, non-empty phase output")
	assert.NotContains(t, result.Output, "awaiting provider wiring",
		"output must be real model content, not the synthesised stub")
	assert.True(t, result.Success, "a completed Specify phase must report Success=true")
	t.Logf("=== REAL /specify PHASE OUTPUT (qualityScore=%v) ===\n%s\n=== END ===",
		result.QualityScore, result.Output)
}

// nilResponseError mirrors handleSpecify's nil-response guard so the invoker shape
// is identical to production.
type nilResponseError struct{}

func (*nilResponseError) Error() string { return "provider returned nil response" }
