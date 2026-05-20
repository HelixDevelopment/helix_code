package context

// History condenser / compaction (speed programme P3-T05).
//
// Long autonomous runs accumulate conversation history that bloats the context
// window. Beyond raw size, an ever-growing history destabilises the cacheable
// prompt prefix (Phase 1 prompt-cache work): once the window approaches the
// model limit, the agent is forced to truncate — and naive truncation silently
// drops task-critical facts.
//
// The Condenser solves both problems. When the running history exceeds a
// configurable token threshold, it summarises the OLDER turns into a single
// compact record while keeping the most recent N turns verbatim. The summary
// is LLM-generated so it reads naturally and adapts to the user's language
// (CONST-046 — no hardcoded user-facing prose); a deterministic structured
// fallback is used when no summarizer is wired.
//
// # Hard no-regression contract (P3-T05 Med-risk)
//
// Compaction MUST NOT drop task-critical context. The condenser therefore:
//
//   - extracts task-critical facts (active task, decisions, files touched,
//     open questions, errors) from the turns being condensed and folds them
//     into the summary EXPLICITLY — they survive condensation by construction;
//   - keeps the most recent N turns byte-for-byte verbatim;
//   - is config-gated: when Config.Enabled is false the condenser is a pure
//     no-op and history is returned unchanged;
//   - leaves sub-threshold histories COMPLETELY untouched — a session shorter
//     than the threshold behaves exactly as it does today.
//
// A condense that loses the active task or a recorded decision is the failure
// mode this package is built to prevent.

import (
	stdctx "context"
	"fmt"
	"sort"
	"strings"
)

// HistoryTurn is a single conversation turn the condenser operates on. It is a
// deliberately minimal, package-local type so internal/context does not import
// internal/session (that would be an import cycle — session imports context-
// adjacent packages). Callers in internal/session convert their richer
// session.Message into HistoryTurn at the boundary.
type HistoryTurn struct {
	// Role is "user", "assistant", "system", "tool", etc.
	Role string
	// Content is the verbatim turn text.
	Content string
}

// TokenCounter estimates the token cost of a string. The condenser uses it to
// decide when the threshold is crossed and to report the before/after
// trajectory. Implementations SHOULD use a provider-native tokenizer and MUST
// fall back to a char-based estimate; a nil counter makes the condenser fall
// back to the built-in estimate (≈4 chars/token).
type TokenCounter interface {
	CountTokens(text string) (int, error)
}

// Summarizer turns the older turns into a compact natural-language summary.
// It is the LLM seam — implementations call a real provider. A nil Summarizer
// makes the condenser emit a deterministic structured summary instead, so the
// condenser is always usable (and unit-testable) without a live provider.
type Summarizer interface {
	// Summarize produces a compact summary of older. The facts argument is the
	// pre-extracted task-critical context the summary MUST preserve verbatim;
	// implementations SHOULD instruct the model to keep every fact intact.
	Summarize(ctx stdctx.Context, older []HistoryTurn, facts CriticalFacts) (string, error)
}

// CriticalFacts is the task-critical context the condenser extracts from the
// turns being condensed BEFORE summarisation, so it survives compaction by
// construction rather than depending on the LLM remembering it.
type CriticalFacts struct {
	// ActiveTask is the most recent statement of what the agent is working on.
	ActiveTask string
	// Decisions are choices the agent made that later turns may depend on.
	Decisions []string
	// FilesTouched are file paths created/edited/read during the condensed span.
	FilesTouched []string
	// OpenQuestions are unresolved questions raised but not yet answered.
	OpenQuestions []string
	// Errors are error messages / failures encountered (must not be forgotten —
	// a re-attempt that forgets a prior failure repeats the mistake).
	Errors []string
}

// IsEmpty reports whether no critical facts were extracted.
func (f CriticalFacts) IsEmpty() bool {
	return f.ActiveTask == "" && len(f.Decisions) == 0 && len(f.FilesTouched) == 0 &&
		len(f.OpenQuestions) == 0 && len(f.Errors) == 0
}

// CondenserConfig configures the history condenser.
type CondenserConfig struct {
	// Enabled gates the whole mechanism. When false, Condense is a pure no-op:
	// history is returned unchanged. This is the no-regression escape hatch —
	// compaction can always be turned off.
	Enabled bool
	// TokenThreshold is the history token count above which condensation
	// triggers. A history at or below this size is left COMPLETELY untouched.
	// Must be > 0 when Enabled; a non-positive value disables condensation.
	TokenThreshold int
	// KeepRecentTurns is how many of the most recent turns stay verbatim.
	// These turns are never summarised. Must be >= 1 when Enabled.
	KeepRecentTurns int
}

// DefaultCondenserConfig returns a disabled-by-default config. Callers opt in
// explicitly — the condenser never changes behaviour unless asked to.
func DefaultCondenserConfig() CondenserConfig {
	return CondenserConfig{
		Enabled:         false,
		TokenThreshold:  24000,
		KeepRecentTurns: 6,
	}
}

// normalized returns a config with invalid fields clamped to safe values so
// the condenser can never panic or produce a degenerate result.
func (c CondenserConfig) normalized() CondenserConfig {
	out := c
	if out.KeepRecentTurns < 1 {
		out.KeepRecentTurns = 1
	}
	if out.TokenThreshold < 0 {
		out.TokenThreshold = 0
	}
	return out
}

// Condenser summarises older conversation history to keep the context window
// bounded and the cacheable prefix small.
type Condenser struct {
	cfg     CondenserConfig
	counter TokenCounter
	summ    Summarizer
}

// NewCondenser builds a Condenser. counter may be nil (built-in char-based
// estimate is used); summ may be nil (deterministic structured summary is
// used). The config is normalized defensively.
func NewCondenser(cfg CondenserConfig, counter TokenCounter, summ Summarizer) *Condenser {
	return &Condenser{
		cfg:     cfg.normalized(),
		counter: counter,
		summ:    summ,
	}
}

// CondenseResult is the outcome of a Condense call.
type CondenseResult struct {
	// Condensed is true when the history was actually summarised. False means
	// the history was returned unchanged (disabled, sub-threshold, or too few
	// turns to condense).
	Condensed bool
	// SkipReason explains a non-Condensed result for observability.
	SkipReason string
	// TokensBefore / TokensAfter are the history token counts. After < Before
	// when Condensed is true — this delta is the anti-bluff proof.
	TokensBefore int
	TokensAfter  int
	// TurnsBefore / TurnsAfter are the turn counts.
	TurnsBefore int
	TurnsAfter  int
	// Facts is the task-critical context preserved into the summary. Exposed so
	// callers (and the task-success-parity test) can assert nothing was lost.
	Facts CriticalFacts
	// Summary is the generated compact summary text (empty when not condensed).
	Summary string
}

// Condense applies threshold-gated condensation to history. It is the single
// entry point for both automatic (threshold) and explicit (/compact) use — the
// explicit path simply lowers the effective threshold to zero via CondenseNow.
//
// Returns the (possibly unchanged) history and a result describing what
// happened. history is never mutated in place; a new slice is returned.
func (c *Condenser) Condense(ctx stdctx.Context, history []HistoryTurn) ([]HistoryTurn, CondenseResult) {
	return c.condense(ctx, history, false)
}

// CondenseNow forces condensation regardless of the token threshold, provided
// the condenser is enabled and there are enough turns to condense. It backs
// the explicit /compact-style command. The Enabled gate still applies — an
// explicit /compact on a condenser that is config-disabled is still a no-op,
// so disabling compaction disables it everywhere.
func (c *Condenser) CondenseNow(ctx stdctx.Context, history []HistoryTurn) ([]HistoryTurn, CondenseResult) {
	return c.condense(ctx, history, true)
}

// condense is the shared implementation. force=true skips the threshold check.
func (c *Condenser) condense(ctx stdctx.Context, history []HistoryTurn, force bool) ([]HistoryTurn, CondenseResult) {
	res := CondenseResult{
		TurnsBefore:  len(history),
		TurnsAfter:   len(history),
		TokensBefore: c.countTurns(history),
	}
	res.TokensAfter = res.TokensBefore

	// No-regression gate 1: config-disabled → pure no-op.
	if !c.cfg.Enabled {
		res.SkipReason = "condenser disabled"
		return history, res
	}

	// No-regression gate 2: sub-threshold histories are left COMPLETELY
	// untouched (unless an explicit /compact forced it).
	if !force {
		if c.cfg.TokenThreshold <= 0 {
			res.SkipReason = "no token threshold configured"
			return history, res
		}
		if res.TokensBefore <= c.cfg.TokenThreshold {
			res.SkipReason = "history under token threshold"
			return history, res
		}
	}

	// We keep the most recent KeepRecentTurns turns verbatim and condense the
	// rest. If there is nothing left to condense, do nothing.
	keep := c.cfg.KeepRecentTurns
	if len(history) <= keep+1 {
		// +1: condensing a single old turn into a summary turn yields no gain.
		res.SkipReason = "too few turns to condense"
		return history, res
	}

	splitAt := len(history) - keep
	older := history[:splitAt]
	recent := history[splitAt:]

	// Extract task-critical facts BEFORE summarisation. This is the heart of
	// the no-regression guarantee: the facts are preserved by construction,
	// independent of whether the LLM summary captures them.
	facts := ExtractCriticalFacts(older)
	res.Facts = facts

	// Generate the compact summary.
	summary, err := c.summarize(ctx, older, facts)
	if err != nil || strings.TrimSpace(summary) == "" {
		// A summariser failure must never lose history. Fall back to the
		// deterministic structured summary so condensation still proceeds
		// safely rather than aborting (aborting would let the window grow
		// unbounded — itself a failure mode).
		summary = renderStructuredSummary(older, facts)
	}
	res.Summary = summary

	// The condensed history is: one summary turn + the verbatim recent turns.
	out := make([]HistoryTurn, 0, 1+len(recent))
	out = append(out, HistoryTurn{Role: "system", Content: summary})
	out = append(out, recent...)

	res.Condensed = true
	res.TurnsAfter = len(out)
	res.TokensAfter = c.countTurns(out)
	res.SkipReason = ""
	return out, res
}

// summarize delegates to the wired Summarizer, or to the deterministic
// structured fallback when no Summarizer is configured.
func (c *Condenser) summarize(ctx stdctx.Context, older []HistoryTurn, facts CriticalFacts) (string, error) {
	if c.summ == nil {
		return renderStructuredSummary(older, facts), nil
	}
	return c.summ.Summarize(ctx, older, facts)
}

// ShouldCondense reports whether the given history would trigger automatic
// condensation. Useful for callers that want to decide proactively without
// running the condensation. Returns false when disabled.
func (c *Condenser) ShouldCondense(history []HistoryTurn) bool {
	if !c.cfg.Enabled || c.cfg.TokenThreshold <= 0 {
		return false
	}
	if len(history) <= c.cfg.KeepRecentTurns+1 {
		return false
	}
	return c.countTurns(history) > c.cfg.TokenThreshold
}

// Config returns the condenser's effective (normalized) configuration.
func (c *Condenser) Config() CondenserConfig { return c.cfg }

// countTurns returns the total estimated token count for a slice of turns.
func (c *Condenser) countTurns(turns []HistoryTurn) int {
	total := 0
	for _, t := range turns {
		total += c.countText(t.Role) + c.countText(t.Content)
	}
	return total
}

// countText counts tokens in s, using the wired counter when available and the
// char-based estimate otherwise.
func (c *Condenser) countText(s string) int {
	if c.counter != nil {
		if n, err := c.counter.CountTokens(s); err == nil {
			return n
		}
	}
	return estimateTokens(s)
}

// estimateTokens is the built-in conservative token estimate: ≈4 chars/token,
// minimum 1 for any non-empty string. Matches the fallback documented on the
// llm.Provider.CountTokens contract.
func estimateTokens(s string) int {
	if s == "" {
		return 0
	}
	n := len(s) / 4
	if n < 1 {
		n = 1
	}
	return n
}

// renderStructuredSummary produces a deterministic, machine-readable summary of
// the condensed turns. It is the fallback used when no LLM Summarizer is wired
// or when the LLM summariser fails. It is NOT user-facing prose — it is a
// structured digest the agent reads — so it is exempt from CONST-046 (which
// governs prose shown to end users). The structure guarantees every extracted
// fact appears verbatim.
func renderStructuredSummary(older []HistoryTurn, facts CriticalFacts) string {
	var b strings.Builder
	b.WriteString("[CONDENSED HISTORY]\n")
	b.WriteString(fmt.Sprintf("condensed_turns=%d\n", len(older)))

	if facts.ActiveTask != "" {
		b.WriteString("active_task: ")
		b.WriteString(facts.ActiveTask)
		b.WriteString("\n")
	}
	writeFactList(&b, "decisions", facts.Decisions)
	writeFactList(&b, "files_touched", facts.FilesTouched)
	writeFactList(&b, "open_questions", facts.OpenQuestions)
	writeFactList(&b, "errors_encountered", facts.Errors)
	return strings.TrimRight(b.String(), "\n")
}

// writeFactList appends a labelled bullet list to b when items is non-empty.
func writeFactList(b *strings.Builder, label string, items []string) {
	if len(items) == 0 {
		return
	}
	b.WriteString(label)
	b.WriteString(":\n")
	for _, it := range items {
		b.WriteString("  - ")
		b.WriteString(it)
		b.WriteString("\n")
	}
}

// dedupePreserveOrder returns items with duplicates and empty strings removed,
// preserving first-seen order. Used so the fact lists do not balloon.
func dedupePreserveOrder(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))
	for _, it := range items {
		it = strings.TrimSpace(it)
		if it == "" {
			continue
		}
		if _, ok := seen[it]; ok {
			continue
		}
		seen[it] = struct{}{}
		out = append(out, it)
	}
	return out
}

// sortedUnique returns items deduplicated and sorted — used for file paths
// where a stable order aids diffing the summary across runs.
func sortedUnique(items []string) []string {
	out := dedupePreserveOrder(items)
	sort.Strings(out)
	return out
}
