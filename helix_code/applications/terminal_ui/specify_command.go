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

// buildSpeckitResponder wraps the TUI's REAL llm.Provider into the speckit
// debate adapter's responder, mirroring cmd/cli/main.go handleDebate /
// handleSpecify verbatim. This is the single honest seam shared by /specify
// and /debate: every phase/debate turn round-trips through a real
// provider.Generate call against tui.llmProvider — real provider output only,
// never fabricated.
//
// Anti-bluff (§11.4 / CONST-035): when no provider is configured OR the active
// provider advertises no models, this returns a non-nil error and an honest
// human-readable reason; callers surface it rather than fabricating output.
// The adapter itself refuses a nil invoker; the speckit engine refuses a nil
// DebateFunc (speckit.ErrDebateFuncNotConfigured). Nothing here invents text.
func (tui *TerminalUI) buildSpeckitResponder() (*speckit_debate_adapter.DebateOrchestratorResponder, string, error) {
	provider := tui.llmProvider
	if provider == nil {
		return nil, "", fmt.Errorf("%s", tui.t("terminal_ui_chat_no_provider_error"))
	}

	// Resolve the provider's first advertised model. The adapter's
	// RegisterProvider + the provider's Generate both require a non-empty
	// model name, so a provider with zero models cannot drive a debate —
	// refuse cleanly rather than hand the adapter an empty spec (§11.4.6
	// no-guessing). Prefer the model the user actually selected when it is one
	// of the provider's advertised models; otherwise fall back to the first.
	models := provider.GetModels()
	if len(models) == 0 {
		return nil, "", fmt.Errorf("active provider %q advertises no models", provider.GetName())
	}
	modelName := models[0].Name
	if sel := strings.TrimSpace(tui.selectedModel); sel != "" {
		for _, m := range models {
			if m.Name == sel {
				modelName = sel
				break
			}
		}
	}
	if strings.TrimSpace(modelName) == "" {
		return nil, "", fmt.Errorf("active provider %q advertises no usable model name", provider.GetName())
	}

	// Wrap the REAL provider into the adapter's ProviderInvoker shape.
	// provider.Generate is (ctx, *LLMRequest) (*LLMResponse, error); the
	// orchestrator wants (ctx, prompt) (string, error). This closure is the
	// honest seam — identical to cmd/cli/main.go.
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
		// Two agents (same provider/model, distinct scores) — the orchestrator
		// requires >=2 participants; a single agent fails at runtime with
		// "insufficient agents (have 1, need 2)" (HXC-080).
		[]speckit_debate_adapter.AgentSpec{
			{Provider: provider.GetName(), Model: modelName, Score: 0.9},
			{Provider: provider.GetName(), Model: modelName, Score: 0.85},
		},
	)
	if err != nil {
		return nil, "", err
	}
	return responder, modelName, nil
}

// runSpecify drives HelixSpecifier's speckit Specify phase against the TUI's
// REAL llm provider, mirroring cmd/cli/main.go handleSpecify. It returns the
// real phase header + Output (never fabricated). Errors — including
// speckit.ErrDebateFuncNotConfigured and any provider/debate failure — are
// returned verbatim.
//
// This function performs BLOCKING LLM calls and so MUST NOT be invoked on the
// tview event loop; callers run it in a goroutine and marshal the result back
// through QueueUpdateDraw (see handleSpecifyCommand).
func (tui *TerminalUI) runSpecify(ctx context.Context, request string) (string, error) {
	responder, _, err := tui.buildSpeckitResponder()
	if err != nil {
		return "", err
	}

	// Build the real speckit pillar and wire the REAL debate responder via the
	// canonical SetDebateFunc(LLMBackedDebateFunc(responder)) path. A nil
	// DebateFunc makes ExecutePhase return ErrDebateFuncNotConfigured.
	pillar := speckit.NewPillar(speckitconfig.DefaultConfig(), logrus.New())
	pillar.SetDebateFunc(speckit.LLMBackedDebateFunc(responder))

	result, err := pillar.ExecutePhase(ctx, speckittypes.PhaseSpecify, &speckittypes.PhaseInput{
		UserRequest: request,
	})
	if err != nil {
		return "", err
	}
	if result == nil {
		return "", fmt.Errorf("speckit ExecutePhase returned nil result")
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "# Specify phase (quality score %.3f, debate %s)\n",
		result.QualityScore, result.DebateID)
	sb.WriteString(result.Output)
	if !strings.HasSuffix(result.Output, "\n") {
		sb.WriteString("\n")
	}
	return sb.String(), nil
}

// runDebate runs the given topic through the debate responder wired to the
// TUI's REAL llm provider, mirroring cmd/cli/main.go handleDebate. Returns the
// real debate output (never fabricated); provider/debate errors are returned
// verbatim. BLOCKING — run off the event loop.
func (tui *TerminalUI) runDebate(ctx context.Context, topic string) (string, error) {
	responder, _, err := tui.buildSpeckitResponder()
	if err != nil {
		return "", err
	}
	out, err := responder.Generate(ctx, topic)
	if err != nil {
		return "", err
	}
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	return out, nil
}

// handleSpecifyCommand wires the `/specify <request>` chat command. It is
// invoked from handleChatCommand (which runs on the tview event loop), so the
// blocking speckit run is dispatched on its own goroutine; the user message,
// the in-progress status, and the final result/error are all funnelled back
// through QueueUpdateDraw because tview is not goroutine-safe.
//
// Anti-bluff: an empty request prints usage; a missing provider/model surfaces
// the honest builder error; a real run renders result.Output; any engine error
// (incl. the ErrDebateFuncNotConfigured sentinel) is surfaced verbatim.
func (tui *TerminalUI) handleSpecifyCommand(request string) {
	tui.runPhaseCommand("/specify", request,
		"terminal_ui_chat_specify_usage", "terminal_ui_chat_specify_running",
		tui.runSpecify)
}

// handleDebateCommand wires the `/debate <topic>` chat command (same threading
// + anti-bluff discipline as handleSpecifyCommand).
func (tui *TerminalUI) handleDebateCommand(topic string) {
	tui.runPhaseCommand("/debate", topic,
		"terminal_ui_chat_debate_usage", "terminal_ui_chat_debate_running",
		tui.runDebate)
}

// runPhaseCommand is the shared driver for /specify and /debate. run is the
// blocking real-provider call (runSpecify / runDebate). usageKey/runningKey are
// i18n message IDs (CONST-046 — no hardcoded user-facing strings).
func (tui *TerminalUI) runPhaseCommand(
	name, arg, usageKey, runningKey string,
	run func(context.Context, string) (string, error),
) {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		tui.chatHistory = append(tui.chatHistory, llm.Message{
			Role:    "system",
			Content: tui.t(usageKey),
		})
		tui.chatOutput.SetText(tui.formatChatHistory())
		tui.chatOutput.ScrollToEnd()
		return
	}

	// Honest no-provider guard up-front so the user gets the same clear
	// message the plain chat path gives, without spawning a goroutine.
	if tui.llmProvider == nil {
		tui.chatHistory = append(tui.chatHistory, llm.Message{
			Role:    "assistant",
			Content: tui.t("terminal_ui_chat_no_provider_error"),
		})
		tui.chatOutput.SetText(tui.formatChatHistory())
		tui.chatOutput.ScrollToEnd()
		tui.statusBar.SetText("[red]" + tui.t("terminal_ui_chat_no_provider_status"))
		return
	}

	// Echo the command as a user turn + an in-progress status, then run the
	// real (blocking) phase off the event loop.
	tui.chatHistory = append(tui.chatHistory, llm.Message{
		Role:    "user",
		Content: name + " " + arg,
	})
	tui.chatOutput.SetText(tui.formatChatHistory())
	tui.chatOutput.ScrollToEnd()
	tui.statusBar.SetText("[yellow]" + tui.t(runningKey))

	go func() {
		out, err := run(context.Background(), arg)
		tui.app.QueueUpdateDraw(func() {
			if err != nil {
				tui.chatHistory = append(tui.chatHistory, llm.Message{
					Role:    "system",
					Content: tui.td("terminal_ui_chat_phase_failed", map[string]any{
						"Command": name,
						"Error":   err.Error(),
					}),
				})
				tui.statusBar.SetText("[red]" + tui.t("terminal_ui_status_bar_default"))
			} else {
				tui.chatHistory = append(tui.chatHistory, llm.Message{
					Role:    "assistant",
					Content: out,
				})
				tui.statusBar.SetText("[green]" + tui.t("terminal_ui_status_bar_default"))
			}
			tui.chatOutput.SetText(tui.formatChatHistory())
			tui.chatOutput.ScrollToEnd()
		})
	}()
}
