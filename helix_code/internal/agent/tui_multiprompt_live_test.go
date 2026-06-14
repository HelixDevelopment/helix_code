//go:build helixagentlive

// LIVE reproduction of the REAL multi-prompt TUI flow (systematic-debugging
// STEP 1). Drives RunToolLoop against the running HelixAgent (:7061) with a real
// read-only tool registry, EXACTLY as applications/terminal_ui/main.go does, and
// reproduces the prompt-2 HTTP 400 by replaying the TUI's chatHistory append
// logic (final-content assistant turn + trace assistant turn).
//
//	go test -tags helixagentlive -run TestLive_TUIMultiPrompt -count=1 ./internal/agent/
package agent

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"dev.helix.code/internal/llm"
	"dev.helix.code/internal/llm/providers/helixagent"
	"dev.helix.code/internal/tools"
	"dev.helix.code/internal/tools/git"
)

func liveBaseURL() string {
	if v := os.Getenv("HELIXAGENT_BASE_URL"); v != "" {
		return v
	}
	return helixagent.DefaultBaseURL
}

func buildSystemPrompt(registry *tools.ToolRegistry) string {
	names := make([]string, 0)
	for _, t := range registry.List() {
		names = append(names, t.Name())
	}
	sort.Strings(names)
	return "You are the Helix coding agent, operating INSIDE the user's real codebase at the current working directory. " +
		"You have these tools available: " + strings.Join(names, ", ") + ". " +
		"These tools give you genuine read access to the user's files and git state — you CAN see the codebase. " +
		"When the user asks whether you can see or access their codebase, you MUST call a tool FIRST and then answer from what it returned."
}

func dumpHistory(t *testing.T, label string, msgs []llm.Message) {
	t.Logf("=== %s (%d messages) ===", label, len(msgs))
	for i, m := range msgs {
		t.Logf("  [%d] role=%-9s content-len=%-4d has-tool-calls=%v tool_call_id=%q",
			i, m.Role, len(m.Content), len(m.ToolCalls) > 0, m.ToolCallID)
	}
}

func TestLive_TUIMultiPrompt(t *testing.T) {
	provider := helixagent.New(liveBaseURL())
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	if !provider.IsAvailable(ctx) {
		t.Skipf("SKIP-OK: HelixAgent not reachable at %s", liveBaseURL())
	}

	reg, err := tools.NewToolRegistry(nil)
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	wd, _ := os.Getwd()
	reg.Register(git.NewGitStatusTool(wd))
	sysPrompt := buildSystemPrompt(reg)

	opts := ToolLoopOptions{
		Model:              "helixagent-llm",
		MaxTurns:           6,
		SystemPrompt:       sysPrompt,
		MaxToolResultChars: 800,
		ReadOnlyOnly:       true,
	}

	// chatHistory is the OUTER conversation the TUI stores (no system prompt;
	// RunToolLoop prepends that). We replay the TUI's append logic exactly.
	var chatHistory []llm.Message

	prompts := []string{
		"Do you see my codebase?",
		"Do you need an AGENTS.md to understand the project better?",
		"Check git status of all work done.",
	}

	for turn, prompt := range prompts {
		// TUI sendChatMessage: append user turn.
		chatHistory = append(chatHistory, llm.Message{Role: "user", Content: prompt})
		// history = snapshot WITHOUT the placeholder assistant turn.
		history := append([]llm.Message(nil), chatHistory...)

		dumpHistory(t, fmt.Sprintf("PROMPT %d (%q) — sent into RunToolLoop", turn+1, prompt), history)

		result, loopErr := RunToolLoop(ctx, provider, reg, history, opts)
		if loopErr != nil {
			t.Fatalf("PROMPT %d RunToolLoop FAILED: %v", turn+1, loopErr)
		}
		if strings.TrimSpace(result.FinalContent) == "" {
			t.Fatalf("PROMPT %d returned EMPTY FinalContent", turn+1)
		}
		t.Logf("PROMPT %d FinalContent (len=%d): %.200q", turn+1, len(result.FinalContent), result.FinalContent)

		// TUI append logic (main.go ~1672-1692): set placeholder assistant content,
		// then append a SECOND assistant turn for the trace, then ensemble panel.
		chatHistory = append(chatHistory, llm.Message{Role: "assistant", Content: result.FinalContent})
		if len(result.Trace) > 0 {
			// The TUI builds trace lines; here we just append a representative
			// trace assistant message (always non-empty when Trace>0).
			traceText := fmt.Sprintf("tool trace: %d call(s)", len(result.Trace))
			chatHistory = append(chatHistory, llm.Message{Role: "assistant", Content: traceText})
		}
		// FinalMetadata → ensemble panel: helixagent-llm carries only
		// {"helixagent_model":...}; FormatEnsemblePanel returns empty → not appended.
	}

	dumpHistory(t, "FINAL chatHistory", chatHistory)
	t.Logf("ALL %d PROMPTS SUCCEEDED", len(prompts))
}

// TestLive_TUIMultiPrompt_EmptyAssistantInHistory reproduces the EXACT reported
// failure: an empty assistant turn stored in the chat history (as the real TUI
// can store when a streamed/ensemble turn produced no text) replayed into the
// NEXT RunToolLoop call. Before the adapter sanitiser this 400s on prompt 2;
// after, the whole conversation succeeds.
func TestLive_TUIMultiPrompt_EmptyAssistantInHistory(t *testing.T) {
	provider := helixagent.New(liveBaseURL())
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	if !provider.IsAvailable(ctx) {
		t.Skipf("SKIP-OK: HelixAgent not reachable at %s", liveBaseURL())
	}
	reg, err := tools.NewToolRegistry(nil)
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	wd, _ := os.Getwd()
	reg.Register(git.NewGitStatusTool(wd))
	opts := ToolLoopOptions{Model: "helixagent-llm", MaxTurns: 6, SystemPrompt: buildSystemPrompt(reg), MaxToolResultChars: 800, ReadOnlyOnly: true}

	// Hand-construct the precise failing history: an EMPTY assistant turn sits at
	// what becomes messages[3] (after RunToolLoop prepends the system prompt and
	// before the new user turn). This is the reported "assistant message must
	// have content or tool_calls" shape.
	history := []llm.Message{
		{Role: "user", Content: "Do you see my codebase?"},
		{Role: "assistant", Content: ""}, // <-- the empty assistant the TUI stored
		{Role: "user", Content: "Do you need an AGENTS.md to understand the project better?"},
	}
	dumpHistory(t, "REPLAY with empty assistant at history[1] → messages[3] post-system", history)

	result, loopErr := RunToolLoop(ctx, provider, reg, history, opts)
	if loopErr != nil {
		t.Fatalf("RunToolLoop FAILED (the reported 400 if unfixed): %v", loopErr)
	}
	if strings.TrimSpace(result.FinalContent) == "" {
		t.Fatalf("empty FinalContent")
	}
	t.Logf("SUCCESS — prompt-2 over empty-assistant history answered (len=%d): %.200q", len(result.FinalContent), result.FinalContent)
}
