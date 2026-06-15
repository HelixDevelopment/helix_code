package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"dev.helix.code/internal/adapters/speckit_debate_adapter"
	"dev.helix.code/internal/llm"
	speckitconfig "digital.vasic.helixspecifier/pkg/config"
	"digital.vasic.helixspecifier/pkg/speckit"
	speckittypes "digital.vasic.helixspecifier/pkg/types"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// specify.go — real HelixSpecifier "Specify" phase surface over HTTP.
//
// Anti-bluff (CONST-035 / BLUFF-001 / Article XI §11.9): this handler drives
// the REAL speckit engine. It mirrors cmd/cli/main.go's handleSpecify exactly
// (cmd/cli/main.go:2546) — there is NO fabricated phase output, NO canned
// result, NO simulation. The flow is:
//
//	provider := resolveLLMProvider(...)                          // real provider
//	invoker  := func(ctx, prompt) (string, error){ provider.Generate(...) }
//	responder, _ := speckit_debate_adapter.NewLLMBackedResponder(invoker, 2 agents)
//	pillar := speckit.NewPillar(speckitconfig.DefaultConfig(), logrus.New())
//	pillar.SetDebateFunc(speckit.LLMBackedDebateFunc(responder))
//	result, err := pillar.ExecutePhase(ctx, speckittypes.PhaseSpecify, &PhaseInput{UserRequest})
//
// Every phase debate turn round-trips through a REAL provider.Generate call.
// The speckit engine REQUIRES a real DebateFunc: a nil DebateFunc makes
// ExecutePhase return speckit.ErrDebateFuncNotConfigured (the prior fabricating
// branch was removed). When no provider/model is configured, OR the
// engine/debate returns an error, the handler surfaces the REAL error
// (502/500) rather than fabricating any phase output.
//
// Provider resolution reuses resolveLLMProvider (llm_generate.go) verbatim, so
// the exact same flag/env/local-Ollama-default precedence applies and no key
// value is ever read, logged, or persisted here (CONST-042 / §12.1).

// specifyRequest is the JSON body accepted by POST /api/v1/specify.
type specifyRequest struct {
	// Request is the user's natural-language feature/spec request. Required.
	Request string `json:"request"`
	// Provider optionally names the provider to use (e.g. "anthropic",
	// "ollama"). When empty, HELIX_LLM_PROVIDER / local-Ollama default apply —
	// identical to /api/v1/llm/generate.
	Provider string `json:"provider"`
	// Model is the model id to target. Optional — when empty the provider's
	// first advertised model is used (the same guard the CLI applies).
	Model string `json:"model"`
}

// specifyHandler handles POST /api/v1/specify — a real speckit Specify-phase
// run. On success it returns {output, qualityScore, debateID, success}; on
// failure it surfaces the real error as a 502/500, never a fabricated 200.
func (s *Server) specifyHandler(c *gin.Context) {
	var req specifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  fmt.Sprintf("invalid request body: %v", err),
		})
		return
	}

	request := strings.TrimSpace(req.Request)
	if request == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "request must include a non-empty 'request'",
		})
		return
	}

	// Resolve a REAL provider via the shared resolveLLMProvider path. Closed by
	// this handler once the phase completes.
	provider, err := resolveLLMProvider(req.Provider, req.Model)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "error",
			"error":  err.Error(),
		})
		return
	}
	defer func() { _ = provider.Close() }()

	// Resolve a concrete model name. The adapter's RegisterProvider + the
	// provider's Generate both require a non-empty model (§11.4.6 no-guessing):
	// honour an explicit request model, else the provider's first advertised
	// model. A provider advertising none cannot drive the phase — refuse
	// cleanly (502) rather than hand the adapter an empty spec.
	modelName := strings.TrimSpace(req.Model)
	if modelName == "" {
		if models := provider.GetModels(); len(models) > 0 {
			modelName = models[0].Name
		}
	}
	if strings.TrimSpace(modelName) == "" {
		c.JSON(http.StatusBadGateway, gin.H{
			"status":   "error",
			"error":    "active provider advertises no models; cannot run the specify phase",
			"provider": provider.GetName(),
		})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 180*time.Second)
	defer cancel()

	// Honest seam: every debate agent turn round-trips through a real
	// provider.Generate call against the resolved provider — identical to
	// handleSpecify's invoker in cmd/cli/main.go.
	invoker := func(ictx context.Context, prompt string) (string, error) {
		resp, gErr := provider.Generate(ictx, &llm.LLMRequest{
			Model:       modelName,
			MaxTokens:   1000,
			Temperature: 0.7,
			Messages:    []llm.Message{{Role: "user", Content: prompt}},
		})
		if gErr != nil {
			return "", gErr
		}
		if resp == nil {
			return "", fmt.Errorf("provider returned nil response")
		}
		return resp.Content, nil
	}

	responder, err := speckit_debate_adapter.NewLLMBackedResponder(
		invoker,
		// Two agents (same provider/model, distinct scores) — the debate
		// orchestrator requires >=2 participants (MinAgentsPerDebate); a single
		// agent fails at runtime with "insufficient agents (have 1, need 2)"
		// (HXC-080).
		[]speckit_debate_adapter.AgentSpec{
			{Provider: provider.GetName(), Model: modelName, Score: 0.9},
			{Provider: provider.GetName(), Model: modelName, Score: 0.85},
		},
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":   "error",
			"error":    fmt.Sprintf("specify setup failed: %v", err),
			"provider": provider.GetName(),
		})
		return
	}

	// Build the real speckit pillar and wire the REAL debate responder via the
	// canonical SetDebateFunc(LLMBackedDebateFunc(responder)) path. A nil
	// DebateFunc would make ExecutePhase return ErrDebateFuncNotConfigured.
	pillar := speckit.NewPillar(speckitconfig.DefaultConfig(), logrus.New())
	pillar.SetDebateFunc(speckit.LLMBackedDebateFunc(responder))

	result, err := pillar.ExecutePhase(ctx, speckittypes.PhaseSpecify, &speckittypes.PhaseInput{
		UserRequest: request,
	})
	if err != nil {
		// Real engine/debate/provider error surfaced verbatim — including
		// speckit.ErrDebateFuncNotConfigured and any provider failure. Never a
		// fabricated phase output (§11.4 / CONST-035).
		c.JSON(http.StatusBadGateway, gin.H{
			"status":   "error",
			"error":    fmt.Sprintf("specify phase failed: %v", err),
			"provider": provider.GetName(),
		})
		return
	}
	if result == nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"status":   "error",
			"error":    "speckit ExecutePhase returned nil result",
			"provider": provider.GetName(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":       "success",
		"output":       result.Output,
		"qualityScore": result.QualityScore,
		"debateID":     result.DebateID,
		"success":      result.Success,
		"provider":     provider.GetName(),
		"model":        modelName,
	})
}
