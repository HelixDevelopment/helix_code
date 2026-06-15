//go:build nogui

package main

import (
	"context"
	"fmt"
	"strings"

	"dev.helix.code/internal/adapters/speckit_debate_adapter"
	"dev.helix.code/internal/llm"
	speckitconfig "digital.vasic.helixspecifier/pkg/config"
	"digital.vasic.helixspecifier/pkg/speckit"
	speckittypes "digital.vasic.helixspecifier/pkg/types"
	"github.com/sirupsen/logrus"
)

// Specify runs HelixSpecifier's speckit Specify phase against the headless
// desktop (`nogui`) client's REAL llm provider. It mirrors the CLI's
// (*CLI).handleSpecify wiring (cmd/cli/main.go:2546) so the desktop client
// reaches the speckit Specify capability through the SAME canonical real path
// as cmd/cli — provider → ProviderInvoker → LLMBackedResponder (TWO agents) →
// speckit.Pillar.SetDebateFunc(LLMBackedDebateFunc) → ExecutePhase(PhaseSpecify).
//
// The signature uses only simple types (string in, string + error out),
// matching the flat-parity Generate method (generate_nogui.go) and the mobile
// gomobile core so an embedder/binding consumer reaches the Specify phase
// identically.
//
// Anti-bluff (§11.4 / CONST-035): the responder round-trips every phase debate
// turn through a REAL provider.Generate call — no simulated/fabricated phase
// output. The speckit engine REQUIRES a real DebateFunc: a nil DebateFunc makes
// ExecutePhase return speckit.ErrDebateFuncNotConfigured. The debate
// orchestrator requires >=2 participants (MinAgentsPerDebate); a single agent
// fails at runtime with "insufficient agents (have 1, need 2)" (HXC-080), which
// is why TWO AgentSpec entries (scores 0.9 / 0.85) are registered. When no
// provider/model is reachable, OR the engine/debate/provider returns an error,
// the REAL error is surfaced verbatim — never swallowed into a fake success.
func (cliApp *CLIApp) Specify(request string) (string, error) {
	return cliApp.specifyInternal(context.Background(), request)
}

// specifyInternal performs the real speckit Specify phase. Split out from
// Specify so a context can be supplied by callers/tests.
func (cliApp *CLIApp) specifyInternal(ctx context.Context, request string) (string, error) {
	request = strings.TrimSpace(request)
	if request == "" {
		return "", fmt.Errorf("specify: request must not be empty")
	}

	// Resolve a REAL llm.Provider via the SAME path as Generate
	// (buildDesktopLLMProvider: cloud via HELIX_LLM_PROVIDER → local Ollama
	// fallback). Never a stub/fake provider.
	provider := buildDesktopLLMProvider(ctx)
	if provider == nil {
		return "", fmt.Errorf("specify: no LLM provider available (cloud unconfigured and local Ollama unreachable)")
	}

	// Resolve the provider's first advertised model — the responder's
	// RegisterProvider and the provider's Generate both require a non-empty
	// model name (§11.4.6 no-guessing), identical guard to handleSpecify.
	modelName := ""
	if models := provider.GetModels(); len(models) > 0 {
		modelName = models[0].Name
	}
	if strings.TrimSpace(modelName) == "" {
		return "", fmt.Errorf("specify: active provider advertises no models; cannot run the specify phase")
	}

	// Wrap the REAL provider into the adapter's ProviderInvoker shape — the
	// identical honest seam handleSpecify uses.
	invoker := func(ictx context.Context, prompt string) (string, error) {
		resp, err := provider.Generate(ictx, &llm.LLMRequest{
			Model:       modelName,
			MaxTokens:   1000,
			Temperature: 0.7,
			Messages:    []llm.Message{{Role: "user", Content: prompt}},
		})
		if err != nil {
			return "", err
		}
		if resp == nil {
			return "", fmt.Errorf("provider returned nil response")
		}
		return resp.Content, nil
	}

	responder, err := speckit_debate_adapter.NewLLMBackedResponder(
		invoker,
		// Two agents (same provider/model, distinct scores) — the debate
		// orchestrator requires >=2 participants (HXC-080); one agent fails at
		// runtime with "insufficient agents (have 1, need 2)".
		[]speckit_debate_adapter.AgentSpec{
			{Provider: provider.GetName(), Model: modelName, Score: 0.9},
			{Provider: provider.GetName(), Model: modelName, Score: 0.85},
		},
	)
	if err != nil {
		return "", fmt.Errorf("specify: setup failed: %w", err)
	}

	// Build the real speckit pillar and wire the REAL debate responder into it
	// via the canonical SetDebateFunc(LLMBackedDebateFunc(responder)) path. A
	// nil DebateFunc would make ExecutePhase return ErrDebateFuncNotConfigured.
	pillar := speckit.NewPillar(speckitconfig.DefaultConfig(), logrus.New())
	pillar.SetDebateFunc(speckit.LLMBackedDebateFunc(responder))

	result, err := pillar.ExecutePhase(ctx, speckittypes.PhaseSpecify, &speckittypes.PhaseInput{
		UserRequest: request,
	})
	if err != nil {
		// Surfaces the REAL error verbatim — including
		// speckit.ErrDebateFuncNotConfigured and any debate/provider failure.
		// Never a fabricated phase output (§11.4 / CONST-035).
		return "", fmt.Errorf("specify: %w", err)
	}
	if result == nil {
		return "", fmt.Errorf("specify: speckit ExecutePhase returned nil result")
	}

	// Compose the same header the CLI prints, then the REAL phase output.
	var b strings.Builder
	fmt.Fprintf(&b, "# Specify phase (quality score %.3f, debate %s)\n",
		result.QualityScore, result.DebateID)
	b.WriteString(result.Output)
	if !strings.HasSuffix(result.Output, "\n") {
		b.WriteString("\n")
	}
	return b.String(), nil
}
