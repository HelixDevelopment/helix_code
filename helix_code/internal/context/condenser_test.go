package context

// Unit tests for the history condenser (speed programme P3-T05).
//
// Coverage targets per the P3-T05 spec:
//   - the condenser preserves key facts (active task, decisions, files, open
//     questions, errors) for representative histories;
//   - sub-threshold history is left COMPLETELY untouched;
//   - the disabled config is a pure no-op;
//   - the recent N turns stay verbatim;
//   - the token-count trajectory is bounded after condensation (anti-bluff
//     proof — captured as test output).
//
// Mocks are permitted here (CONST-050(A) — unit-test sources only): the
// stubSummarizer below is a unit-test fake, never imported by production code.

import (
	stdctx "context"
	"strings"
	"testing"
)

// stubSummarizer is a unit-test fake Summarizer. It records its inputs and
// returns a fixed string so tests can assert the condenser wired it correctly.
type stubSummarizer struct {
	called     bool
	gotOlder   []HistoryTurn
	gotFacts   CriticalFacts
	returnText string
	returnErr  error
}

func (s *stubSummarizer) Summarize(_ stdctx.Context, older []HistoryTurn, facts CriticalFacts) (string, error) {
	s.called = true
	s.gotOlder = older
	s.gotFacts = facts
	return s.returnText, s.returnErr
}

// bigContent returns a string guaranteed to exceed n estimated tokens.
func bigContent(approxTokens int) string {
	return strings.Repeat("x", approxTokens*4+8)
}

// representativeHistory builds a long synthetic task-style conversation that
// carries every category of task-critical fact.
func representativeHistory() []HistoryTurn {
	h := []HistoryTurn{
		{Role: "user", Content: "Let's implement the P3-T05 history condenser."},
		{Role: "assistant", Content: "Working on: building the condenser in internal/context/condenser.go"},
		{Role: "assistant", Content: "I'll use a token threshold to trigger condensation. Decided to keep recent turns verbatim."},
		{Role: "tool", Content: "edited internal/context/critical_facts.go and internal/session/condense.go"},
		{Role: "assistant", Content: "Error: build failed because of an import cycle with internal/session"},
		{Role: "assistant", Content: "We will break the cycle with a package-local HistoryTurn type."},
		{Role: "user", Content: "Open question: should /compact be config-gated too?"},
		{Role: "assistant", Content: "Padding turn one " + bigContent(2000)},
		{Role: "assistant", Content: "Padding turn two " + bigContent(2000)},
		{Role: "assistant", Content: "Padding turn three " + bigContent(2000)},
		{Role: "user", Content: "Recent turn one — please continue."},
		{Role: "assistant", Content: "Recent turn two — continuing."},
		{Role: "user", Content: "Recent turn three — last verbatim turn."},
	}
	return h
}

func TestCondenser_Disabled_IsNoOp(t *testing.T) {
	cfg := CondenserConfig{Enabled: false, TokenThreshold: 1, KeepRecentTurns: 2}
	c := NewCondenser(cfg, nil, nil)

	in := representativeHistory()
	out, res := c.Condense(stdctx.Background(), in)

	if res.Condensed {
		t.Fatalf("disabled condenser must not condense")
	}
	if len(out) != len(in) {
		t.Fatalf("disabled condenser changed turn count: got %d want %d", len(out), len(in))
	}
	for i := range in {
		if out[i] != in[i] {
			t.Fatalf("disabled condenser mutated turn %d", i)
		}
	}
}

func TestCondenser_SubThreshold_Untouched(t *testing.T) {
	// Threshold far above the tiny history → nothing happens.
	cfg := CondenserConfig{Enabled: true, TokenThreshold: 1_000_000, KeepRecentTurns: 2}
	c := NewCondenser(cfg, nil, nil)

	in := []HistoryTurn{
		{Role: "user", Content: "short one"},
		{Role: "assistant", Content: "short two"},
		{Role: "user", Content: "short three"},
		{Role: "assistant", Content: "short four"},
	}
	out, res := c.Condense(stdctx.Background(), in)

	if res.Condensed {
		t.Fatalf("sub-threshold history must not be condensed (reason=%q)", res.SkipReason)
	}
	if len(out) != len(in) {
		t.Fatalf("sub-threshold history changed length")
	}
	for i := range in {
		if out[i] != in[i] {
			t.Fatalf("sub-threshold history mutated turn %d", i)
		}
	}
	if res.TokensBefore != res.TokensAfter {
		t.Fatalf("sub-threshold token count changed: before=%d after=%d", res.TokensBefore, res.TokensAfter)
	}
}

func TestCondenser_TooFewTurns_NotCondensed(t *testing.T) {
	cfg := CondenserConfig{Enabled: true, TokenThreshold: 1, KeepRecentTurns: 5}
	c := NewCondenser(cfg, nil, nil)
	in := []HistoryTurn{
		{Role: "user", Content: bigContent(100)},
		{Role: "assistant", Content: bigContent(100)},
	}
	_, res := c.Condense(stdctx.Background(), in)
	if res.Condensed {
		t.Fatalf("history with too few turns must not be condensed")
	}
}

func TestCondenser_Condenses_AboveThreshold_PreservesFacts(t *testing.T) {
	cfg := CondenserConfig{Enabled: true, TokenThreshold: 2000, KeepRecentTurns: 3}
	c := NewCondenser(cfg, nil, nil) // nil summarizer → deterministic structured summary

	in := representativeHistory()
	out, res := c.Condense(stdctx.Background(), in)

	if !res.Condensed {
		t.Fatalf("history above threshold must be condensed (reason=%q)", res.SkipReason)
	}

	// --- token-count trajectory proof (anti-bluff) ---
	t.Logf("token trajectory: before=%d after=%d (delta=%d)", res.TokensBefore, res.TokensAfter, res.TokensBefore-res.TokensAfter)
	if res.TokensAfter >= res.TokensBefore {
		t.Fatalf("condensation did not bound the window: before=%d after=%d", res.TokensBefore, res.TokensAfter)
	}

	// --- recent N turns stay verbatim ---
	if len(out) != 1+cfg.KeepRecentTurns {
		t.Fatalf("condensed history has %d turns, want %d (1 summary + %d recent)", len(out), 1+cfg.KeepRecentTurns, cfg.KeepRecentTurns)
	}
	recentIn := in[len(in)-cfg.KeepRecentTurns:]
	recentOut := out[1:]
	for i := range recentIn {
		if recentOut[i] != recentIn[i] {
			t.Fatalf("recent turn %d not verbatim: got %+v want %+v", i, recentOut[i], recentIn[i])
		}
	}

	// --- task-critical facts preserved ---
	f := res.Facts
	if f.ActiveTask == "" || !strings.Contains(f.ActiveTask, "condenser") {
		t.Fatalf("active task not preserved: %q", f.ActiveTask)
	}
	if len(f.Decisions) == 0 {
		t.Fatalf("decisions not preserved")
	}
	if len(f.FilesTouched) == 0 {
		t.Fatalf("files touched not preserved")
	}
	if !containsSubstr(f.FilesTouched, "condense.go") {
		t.Fatalf("expected condense.go among preserved files, got %v", f.FilesTouched)
	}
	if len(f.OpenQuestions) == 0 {
		t.Fatalf("open questions not preserved")
	}
	if len(f.Errors) == 0 {
		t.Fatalf("errors not preserved")
	}

	// --- the summary turn actually carries the facts ---
	summary := out[0].Content
	if !strings.Contains(summary, f.ActiveTask) {
		t.Fatalf("summary turn missing active task")
	}
	for _, fp := range f.FilesTouched {
		if !strings.Contains(summary, fp) {
			t.Fatalf("summary turn missing preserved file %q", fp)
		}
	}
}

func TestCondenser_CondenseNow_ForcesBelowThreshold(t *testing.T) {
	// High threshold so automatic condensation would NOT trigger.
	cfg := CondenserConfig{Enabled: true, TokenThreshold: 1_000_000, KeepRecentTurns: 2}
	c := NewCondenser(cfg, nil, nil)

	in := []HistoryTurn{
		{Role: "user", Content: "Working on: explicit compact test"},
		{Role: "assistant", Content: "I will summarize older turns now."},
		{Role: "assistant", Content: "edited internal/context/condenser.go"},
		{Role: "user", Content: "recent one"},
		{Role: "assistant", Content: "recent two"},
	}

	// Automatic path: no-op (under threshold).
	_, autoRes := c.Condense(stdctx.Background(), in)
	if autoRes.Condensed {
		t.Fatalf("automatic Condense should not trigger below threshold")
	}

	// Explicit /compact path: forces condensation.
	out, res := c.CondenseNow(stdctx.Background(), in)
	if !res.Condensed {
		t.Fatalf("CondenseNow must force condensation regardless of threshold (reason=%q)", res.SkipReason)
	}
	if len(out) != 1+cfg.KeepRecentTurns {
		t.Fatalf("forced condense produced %d turns, want %d", len(out), 1+cfg.KeepRecentTurns)
	}
	if res.Facts.ActiveTask == "" {
		t.Fatalf("forced condense lost the active task")
	}
}

func TestCondenser_DisabledIgnoresExplicitCompact(t *testing.T) {
	cfg := CondenserConfig{Enabled: false, TokenThreshold: 1, KeepRecentTurns: 1}
	c := NewCondenser(cfg, nil, nil)
	in := representativeHistory()
	out, res := c.CondenseNow(stdctx.Background(), in)
	if res.Condensed {
		t.Fatalf("explicit compact must still respect the Enabled gate")
	}
	if len(out) != len(in) {
		t.Fatalf("disabled explicit compact changed history")
	}
}

func TestCondenser_UsesWiredSummarizer(t *testing.T) {
	stub := &stubSummarizer{returnText: "LLM SUMMARY: task preserved"}
	cfg := CondenserConfig{Enabled: true, TokenThreshold: 2000, KeepRecentTurns: 3}
	c := NewCondenser(cfg, nil, stub)

	out, res := c.Condense(stdctx.Background(), representativeHistory())
	if !res.Condensed {
		t.Fatalf("expected condensation")
	}
	if !stub.called {
		t.Fatalf("condenser did not call the wired summarizer")
	}
	if out[0].Content != "LLM SUMMARY: task preserved" {
		t.Fatalf("summary turn did not use the summarizer output: %q", out[0].Content)
	}
	// The summarizer must receive the pre-extracted facts so it can be
	// instructed to preserve them.
	if stub.gotFacts.IsEmpty() {
		t.Fatalf("summarizer received empty facts — facts must be pre-extracted")
	}
}

func TestCondenser_SummarizerFailure_FallsBackSafely(t *testing.T) {
	stub := &stubSummarizer{returnErr: errFake}
	cfg := CondenserConfig{Enabled: true, TokenThreshold: 2000, KeepRecentTurns: 3}
	c := NewCondenser(cfg, nil, stub)

	out, res := c.Condense(stdctx.Background(), representativeHistory())
	if !res.Condensed {
		t.Fatalf("summarizer failure must not abort condensation (history must not be lost)")
	}
	// Fallback structured summary must still carry the active task.
	if !strings.Contains(out[0].Content, res.Facts.ActiveTask) {
		t.Fatalf("fallback summary lost the active task")
	}
}

func TestCondenser_ShouldCondense(t *testing.T) {
	cfg := CondenserConfig{Enabled: true, TokenThreshold: 2000, KeepRecentTurns: 3}
	c := NewCondenser(cfg, nil, nil)
	if !c.ShouldCondense(representativeHistory()) {
		t.Fatalf("ShouldCondense should be true for a large history")
	}
	small := []HistoryTurn{{Role: "user", Content: "hi"}, {Role: "assistant", Content: "hello"}}
	if c.ShouldCondense(small) {
		t.Fatalf("ShouldCondense should be false for a small history")
	}
	disabled := NewCondenser(CondenserConfig{Enabled: false}, nil, nil)
	if disabled.ShouldCondense(representativeHistory()) {
		t.Fatalf("disabled condenser must never report ShouldCondense")
	}
}

func TestCondenser_RepeatedCondensation_StaysBounded(t *testing.T) {
	// Anti-bluff: a long autonomous run condenses repeatedly. After each
	// condensation the window must stay bounded — never grow unbounded.
	cfg := CondenserConfig{Enabled: true, TokenThreshold: 3000, KeepRecentTurns: 3}
	c := NewCondenser(cfg, nil, nil)

	history := representativeHistory()
	var trajectory []int
	for turn := 0; turn < 40; turn++ {
		// Simulate the agent adding a fat turn each iteration.
		history = append(history, HistoryTurn{Role: "assistant", Content: "step result " + bigContent(700)})
		var res CondenseResult
		history, res = c.Condense(stdctx.Background(), history)
		trajectory = append(trajectory, c.countTurns(history))
		_ = res
	}
	t.Logf("history-size trajectory over 40 turns: %v", trajectory)

	maxSize := 0
	for _, s := range trajectory {
		if s > maxSize {
			maxSize = s
		}
	}
	// The window must never exceed threshold + one fat turn + keep-recent
	// turns' worth of content. A loose-but-real upper bound proves boundedness.
	upper := cfg.TokenThreshold * 4
	if maxSize > upper {
		t.Fatalf("history size grew unbounded: max=%d upper-bound=%d", maxSize, upper)
	}
}

// containsSubstr reports whether any element of items contains sub.
func containsSubstr(items []string, sub string) bool {
	for _, it := range items {
		if strings.Contains(it, sub) {
			return true
		}
	}
	return false
}

// errFake is a sentinel error for the summarizer-failure test.
var errFake = stubErr("fake summarizer failure")

type stubErr string

func (e stubErr) Error() string { return string(e) }
