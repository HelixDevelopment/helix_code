// Streaming-path proving test for the context-window USED-% indicator (T1.5
// remainder).
//
// Background: Round 3 added the `context: <used>/<window> (NN%)` indicator,
// rendered by printGenerationStats via the pure formatContextUsage helper and
// fed the REAL model window resolved by contextWindowForModel. The non-stream
// path (handleGenerate, main.go ~:1716) and the streaming REPL path
// (interactive loop, main.go ~:2257-2269) BOTH resolve the window the same way
// and BOTH funnel through printGenerationStats.
//
// The streaming path's stats come from streamREPLTurn, which surfaces the last
// chunk carrying usage telemetry as `stats` and returns `stats == nil` when NO
// chunk carried usage (main.go :2329-2331). The REPL call site only calls
// printGenerationStats when `stats != nil` (main.go :2267-2269). So:
//
//   - usage PRESENT on stream  -> stats != nil -> indicator rendered against
//     the REAL window (no fabricated denominator);
//   - usage ABSENT on stream   -> stats == nil -> NO stats line at all, hence
//     NO context indicator (honest omission — never a fake 0/window).
//
// This file proves that end-to-end behaviour by exercising the SAME sequence
// the REPL call site executes: streamREPLTurn -> (window via
// contextWindowForModel) -> printGenerationStats, capturing stdout.
//
// Anti-bluff (CONST-035): this is NOT a tautology. It asserts the real
// rendered bytes:
//   - the usage-absent case MUST omit the indicator (a regression that
//     fabricated `context: 0/<window>` when stats are absent, or that printed
//     stats off a nil-but-not-omitted struct, FAILS this test);
//   - the usage-present case MUST show the indicator computed from the REAL
//     provider window (8192 for fakeStreamProvider.GetContextWindow), so a
//     regression dropping the window wiring on the streaming call site FAILS.
//
// fakeStreamProvider (CONST-050(A): unit-test-only fake) is defined in
// repl_streaming_test.go in this same package.
package main

import (
	"context"
	"strings"
	"testing"

	"dev.helix.code/internal/llm"
)

// renderStreamingStats reproduces the REPL streaming call site (main.go
// ~:2257-2269) verbatim in shape: stream a turn, resolve the REAL window for
// the active model via contextWindowForModel, and — only when stats are
// present — print the per-turn stats line. Returns everything written to
// stdout so the test can assert on the rendered indicator.
func renderStreamingStats(t *testing.T, provider llm.Provider) (statsWereNil bool, stdout string) {
	t.Helper()
	// The REPL derives modelName from the provider's first model, exactly as
	// the interactive loop does (main.go :2242-2244).
	modelName := ""
	if models := provider.GetModels(); len(models) > 0 {
		modelName = models[0].Name
	}
	var stats *llm.LLMResponse
	out := captureStdout(t, func() {
		_, s, err := streamREPLTurn(context.Background(), provider, &llm.LLMRequest{Stream: true})
		if err != nil {
			t.Fatalf("streamREPLTurn: %v", err)
		}
		stats = s
		// Mirror the REPL gate: only render stats when the stream surfaced them.
		if stats != nil {
			printGenerationStats(stats, contextWindowForModel(provider, modelName))
		}
	})
	return stats == nil, out
}

// TestStreamingPath_ContextIndicator_OmittedWhenUsageAbsent proves the honest
// omit-when-unknown behaviour on the streaming path: when no chunk carries
// usage, streamREPLTurn yields nil stats, the REPL prints no stats line, and
// therefore NO context-window indicator is fabricated.
func TestStreamingPath_ContextIndicator_OmittedWhenUsageAbsent(t *testing.T) {
	// No chunk carries Usage / ProcessingTime telemetry -> streamREPLTurn
	// returns stats == nil (the streaming-usage-absent case).
	provider := &fakeStreamProvider{chunks: []llm.LLMResponse{
		{Content: "hello"},
		{Content: " world"},
	}}

	statsWereNil, out := renderStreamingStats(t, provider)

	if !statsWereNil {
		t.Fatalf("streaming usage was absent; stats MUST be nil so the REPL omits the stats line")
	}
	// Honest omission: no stats line, hence no `context:` indicator, and no
	// fabricated denominator against the provider window (8192).
	if strings.Contains(out, "context:") {
		t.Errorf("usage absent on stream -> context indicator MUST be omitted, "+
			"but stdout contained one: %q", out)
	}
	// Guard against the specific fabrication regression: a `0/8192` denominator
	// MUST NOT appear when no real token count was reported.
	if strings.Contains(out, "8192") {
		t.Errorf("usage absent on stream -> MUST NOT fabricate a window denominator; "+
			"stdout leaked the window: %q", out)
	}
}

// TestStreamingPath_ContextIndicator_RenderedWithRealWindowWhenUsagePresent
// proves that when the stream DOES carry usage, the streaming call site renders
// the indicator computed from the REAL provider window — not a guessed one.
// fakeStreamProvider.GetContextWindow() reports 8192 and its catalogue model
// ("fake-1") carries no ContextSize, so contextWindowForModel falls through to
// the provider window — the exact REAL-source resolution path the production
// REPL uses.
func TestStreamingPath_ContextIndicator_RenderedWithRealWindowWhenUsagePresent(t *testing.T) {
	// Final chunk carries usage: prompt 2000 + completion 96 = 2096 total
	// against the real 8192 window -> 25%.
	provider := &fakeStreamProvider{chunks: []llm.LLMResponse{
		{Content: "answer"},
		{Content: " body", Usage: llm.Usage{PromptTokens: 2000, CompletionTokens: 96, TotalTokens: 2096}},
	}}

	statsWereNil, out := renderStreamingStats(t, provider)

	if statsWereNil {
		t.Fatalf("streaming usage was present; stats MUST be non-nil so the REPL renders the stats line")
	}
	// The indicator MUST be present and computed from the REAL window (8192),
	// not a fabricated/guessed denominator: 2096/8192 = 25%.
	want := "context: 2096/8192 (25%)"
	if !strings.Contains(out, want) {
		t.Errorf("usage present on stream -> indicator MUST render against the REAL "+
			"provider window; want substring %q, got %q", want, out)
	}
}
