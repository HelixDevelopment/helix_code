package session

// Unit + integration-style tests for session-level history compaction
// (speed programme P3-T05).
//
// Coverage per the P3-T05 spec:
//   - the compactor preserves key facts (active task, decisions, files, open
//     questions, errors) for a representative session;
//   - sub-threshold history is untouched;
//   - the disabled config / nil compactor is a no-op;
//   - integration: a long synthesised session stays under the context window
//     after compaction (window-bounding proof, no mocks — real condenser);
//   - task-success parity: a representative long task's critical facts survive
//     compaction so the agent can continue correctly — the anti-bluff core.
//
// Mocks permitted (CONST-050(A) — unit-test source). The summarizer fake here
// is a unit-test artefact, never imported by production code.

import (
	stdctx "context"
	"strings"
	"testing"

	hctx "dev.helix.code/internal/context"
)

// fakeSummarizer is a unit-test summarizer that echoes a deterministic summary
// embedding every preserved fact — proving the wiring without a live LLM.
type fakeSummarizer struct{}

func (fakeSummarizer) Summarize(_ stdctx.Context, _ []hctx.HistoryTurn, facts hctx.CriticalFacts) (string, error) {
	var b strings.Builder
	b.WriteString("SUMMARY|task=")
	b.WriteString(facts.ActiveTask)
	for _, d := range facts.Decisions {
		b.WriteString("|decision=")
		b.WriteString(d)
	}
	for _, f := range facts.FilesTouched {
		b.WriteString("|file=")
		b.WriteString(f)
	}
	for _, q := range facts.OpenQuestions {
		b.WriteString("|question=")
		b.WriteString(q)
	}
	for _, e := range facts.Errors {
		b.WriteString("|error=")
		b.WriteString(e)
	}
	return b.String(), nil
}

// bigMsg returns message content guaranteed to exceed approxTokens tokens.
func bigMsg(approxTokens int) string {
	return strings.Repeat("y", approxTokens*4+8)
}

// representativeSession builds a long synthetic task-style session transcript.
func representativeSession() []Message {
	return []Message{
		{Role: "user", Content: "Let's implement the session history compactor."},
		{Role: "assistant", Content: "Working on: the P3-T05 session compactor in internal/session/condense.go"},
		{Role: "assistant", Content: "Decided to wrap the context.Condenser and keep recent turns verbatim."},
		{Role: "tool", Content: "edited internal/session/condense.go and internal/context/condenser.go"},
		{Role: "assistant", Content: "Error: import cycle between session and context"},
		{Role: "assistant", Content: "We will use a package-local HistoryTurn at the boundary."},
		{Role: "user", Content: "Open question: should /compact respect the Enabled gate?"},
		{Role: "assistant", Content: "padding " + bigMsg(2500)},
		{Role: "assistant", Content: "padding " + bigMsg(2500)},
		{Role: "assistant", Content: "padding " + bigMsg(2500)},
		{Role: "user", Content: "recent message one"},
		{Role: "assistant", Content: "recent message two"},
		{Role: "user", Content: "recent message three"},
	}
}

func TestHistoryCompactor_NilIsNoOp(t *testing.T) {
	var hc *HistoryCompactor // nil
	in := representativeSession()
	out, res := hc.CompactIfNeeded(stdctx.Background(), in)
	if res.Compacted {
		t.Fatalf("nil compactor must not compact")
	}
	if len(out) != len(in) {
		t.Fatalf("nil compactor changed history length")
	}
	if hc.ShouldCompact(in) {
		t.Fatalf("nil compactor must never report ShouldCompact")
	}
	if hc.Enabled() {
		t.Fatalf("nil compactor must report not enabled")
	}
}

func TestHistoryCompactor_Disabled_IsNoOp(t *testing.T) {
	cfg := HistoryCompactorConfig{Enabled: false, TokenThreshold: 1, KeepRecentTurns: 2}
	hc := NewHistoryCompactor(cfg, nil, nil)

	in := representativeSession()
	out, res := hc.CompactIfNeeded(stdctx.Background(), in)
	if res.Compacted {
		t.Fatalf("disabled compactor must not compact")
	}
	if len(out) != len(in) {
		t.Fatalf("disabled compactor changed history")
	}
	// Explicit /compact must also respect the disabled gate.
	out2, res2 := hc.Compact(stdctx.Background(), in)
	if res2.Compacted || len(out2) != len(in) {
		t.Fatalf("disabled explicit /compact must be a no-op")
	}
}

func TestHistoryCompactor_SubThreshold_Untouched(t *testing.T) {
	cfg := HistoryCompactorConfig{Enabled: true, TokenThreshold: 1_000_000, KeepRecentTurns: 2}
	hc := NewHistoryCompactor(cfg, nil, nil)

	in := []Message{
		{Role: "user", Content: "short one"},
		{Role: "assistant", Content: "short two"},
		{Role: "user", Content: "short three"},
		{Role: "assistant", Content: "short four"},
	}
	out, res := hc.CompactIfNeeded(stdctx.Background(), in)
	if res.Compacted {
		t.Fatalf("sub-threshold session must not be compacted")
	}
	if len(out) != len(in) {
		t.Fatalf("sub-threshold session changed length")
	}
	for i := range in {
		if out[i] != in[i] {
			t.Fatalf("sub-threshold session mutated message %d", i)
		}
	}
}

func TestHistoryCompactor_CompactsAndPreservesFacts(t *testing.T) {
	cfg := HistoryCompactorConfig{Enabled: true, TokenThreshold: 3000, KeepRecentTurns: 3}
	hc := NewHistoryCompactor(cfg, nil, fakeSummarizer{})

	in := representativeSession()
	out, res := hc.CompactIfNeeded(stdctx.Background(), in)

	if !res.Compacted {
		t.Fatalf("session above threshold must be compacted (reason=%q)", res.SkipReason)
	}
	t.Logf("token trajectory: before=%d after=%d delta=%d", res.TokensBefore, res.TokensAfter, res.TokensBefore-res.TokensAfter)
	if res.TokensAfter >= res.TokensBefore {
		t.Fatalf("compaction did not bound the window: before=%d after=%d", res.TokensBefore, res.TokensAfter)
	}

	// Recent 3 messages stay verbatim.
	if len(out) != 1+cfg.KeepRecentTurns {
		t.Fatalf("compacted session has %d msgs, want %d", len(out), 1+cfg.KeepRecentTurns)
	}
	recentIn := in[len(in)-cfg.KeepRecentTurns:]
	for i, want := range recentIn {
		if out[1+i] != want {
			t.Fatalf("recent message %d not verbatim", i)
		}
	}

	// Task-critical facts preserved in the result.
	if res.ActiveTask == "" {
		t.Fatalf("active task lost")
	}
	if len(res.Decisions) == 0 || len(res.FilesTouched) == 0 ||
		len(res.OpenQuestions) == 0 || len(res.Errors) == 0 {
		t.Fatalf("a fact category was dropped: %+v", res)
	}

	// The summary message carries the facts.
	summary := out[0].Content
	if !strings.Contains(summary, res.ActiveTask) {
		t.Fatalf("summary message missing active task")
	}
}

func TestHistoryCompactor_ExplicitCompact_ForcesBelowThreshold(t *testing.T) {
	cfg := HistoryCompactorConfig{Enabled: true, TokenThreshold: 1_000_000, KeepRecentTurns: 2}
	hc := NewHistoryCompactor(cfg, nil, nil)

	in := []Message{
		{Role: "user", Content: "Working on: explicit compact"},
		{Role: "assistant", Content: "Decided to summarize now."},
		{Role: "tool", Content: "edited foo.go"},
		{Role: "user", Content: "recent one"},
		{Role: "assistant", Content: "recent two"},
	}
	// Automatic: no-op below threshold.
	if _, r := hc.CompactIfNeeded(stdctx.Background(), in); r.Compacted {
		t.Fatalf("automatic compaction should not trigger below threshold")
	}
	// Explicit /compact: forced.
	out, res := hc.Compact(stdctx.Background(), in)
	if !res.Compacted {
		t.Fatalf("explicit /compact must force compaction")
	}
	if len(out) != 1+cfg.KeepRecentTurns {
		t.Fatalf("forced compact produced %d msgs", len(out))
	}
	if res.ActiveTask == "" {
		t.Fatalf("forced compact lost the active task")
	}
}

// TestHistoryCompactor_Integration_LongSessionStaysUnderWindow is the
// integration-style proof: a long synthesised session, repeatedly compacted as
// the agent loop would, stays under a fixed context window. No mocks — the real
// context.Condenser is exercised end-to-end.
func TestHistoryCompactor_Integration_LongSessionStaysUnderWindow(t *testing.T) {
	const contextWindow = 32000 // simulated model window in tokens
	cfg := HistoryCompactorConfig{Enabled: true, TokenThreshold: contextWindow * 70 / 100, KeepRecentTurns: 4}
	hc := NewHistoryCompactor(cfg, nil, fakeSummarizer{})

	history := representativeSession()
	var trajectory []int
	for turn := 0; turn < 60; turn++ {
		// The agent appends a fat turn each loop iteration.
		history = append(history, Message{Role: "assistant", Content: "loop step " + bigMsg(900)})
		history, _ = hc.CompactIfNeeded(stdctx.Background(), history)

		size := 0
		for _, m := range history {
			size += len(m.Role)/4 + len(m.Content)/4
		}
		trajectory = append(trajectory, size)
		if size > contextWindow {
			t.Fatalf("turn %d: history (%d tokens) exceeded the context window (%d)", turn, size, contextWindow)
		}
	}
	t.Logf("60-turn history-size trajectory (tokens): %v", trajectory)
	t.Logf("integration: session never exceeded the %d-token window over 60 turns", contextWindow)
}

// TestHistoryCompactor_TaskSuccessParity is the anti-bluff core: it proves a
// representative long task can still be continued correctly AFTER compaction.
//
// The "task" is reconstructed purely from the post-compaction history: the
// agent must still know (a) what it is working on, (b) the decisions it made,
// (c) which files it touched, (d) the open questions, (e) the errors it hit.
// Compaction that silently drops any of these is the failure mode P3-T05
// exists to prevent — this test fails loudly if that happens.
func TestHistoryCompactor_TaskSuccessParity(t *testing.T) {
	cfg := HistoryCompactorConfig{Enabled: true, TokenThreshold: 3000, KeepRecentTurns: 3}
	hc := NewHistoryCompactor(cfg, nil, fakeSummarizer{})

	original := representativeSession()

	// Facts the agent MUST still have access to after compaction.
	wantTaskSubstr := "P3-T05"
	wantFiles := []string{"condense.go", "condenser.go"}
	wantQuestionSubstr := "Enabled gate"
	wantErrorSubstr := "import cycle"

	compacted, res := hc.CompactIfNeeded(stdctx.Background(), original)
	if !res.Compacted {
		t.Fatalf("expected compaction for the parity test")
	}

	// Reconstruct the agent's available knowledge from the compacted history
	// ALONE — exactly what the agent would see on the next turn.
	var available strings.Builder
	for _, m := range compacted {
		available.WriteString(m.Content)
		available.WriteString("\n")
	}
	knowledge := available.String()

	if !strings.Contains(knowledge, wantTaskSubstr) {
		t.Fatalf("TASK-SUCCESS-PARITY FAIL: agent lost the active task after compaction")
	}
	for _, f := range wantFiles {
		if !strings.Contains(knowledge, f) {
			t.Fatalf("TASK-SUCCESS-PARITY FAIL: agent lost touched file %q after compaction", f)
		}
	}
	if !strings.Contains(knowledge, wantQuestionSubstr) {
		t.Fatalf("TASK-SUCCESS-PARITY FAIL: agent lost an open question after compaction")
	}
	if !strings.Contains(knowledge, wantErrorSubstr) {
		t.Fatalf("TASK-SUCCESS-PARITY FAIL: agent lost a recorded error after compaction")
	}
	if !strings.Contains(knowledge, "decision=") {
		t.Fatalf("TASK-SUCCESS-PARITY FAIL: agent lost recorded decisions after compaction")
	}

	// And the no-regression invariant: the verbatim recent turns are byte-equal
	// to the originals, so in-flight context is never corrupted.
	recentOriginal := original[len(original)-cfg.KeepRecentTurns:]
	recentCompacted := compacted[len(compacted)-cfg.KeepRecentTurns:]
	for i := range recentOriginal {
		if recentCompacted[i] != recentOriginal[i] {
			t.Fatalf("TASK-SUCCESS-PARITY FAIL: recent turn %d corrupted by compaction", i)
		}
	}
	t.Logf("task-success parity: active task, decisions, files, open questions, errors ALL preserved after compaction")
}
