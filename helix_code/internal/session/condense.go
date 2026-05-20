package session

// Session-side history compaction (speed programme P3-T05).
//
// This file wires the task-aware history condenser (internal/context) into the
// session layer. It exposes compaction two ways, mirroring OpenHands' condenser
// + a `/compact`-style explicit operation:
//
//   - automatically — CompactIfNeeded summarises history once it crosses the
//     configured token threshold;
//   - explicitly — Compact forces a one-shot summarisation regardless of size
//     (backing a `/compact` / `/condense` command).
//
// Both paths operate on []Message — the session-native transcript turn type —
// and convert to/from the condenser's package-local context.HistoryTurn at the
// boundary, so internal/session does not create an import cycle with
// internal/context.
//
// # Hard no-regression contract (P3-T05 Med-risk)
//
// Compaction MUST NOT drop task-critical context. The condenser preserves the
// active task, decisions, files touched, open questions and errors by
// construction (see internal/context/critical_facts.go). On top of that this
// file guarantees:
//
//   - config-gated: a HistoryCompactor built from a disabled config is a pure
//     no-op — Compact and CompactIfNeeded return history unchanged;
//   - sub-threshold histories are returned byte-for-byte unchanged by
//     CompactIfNeeded — a short session behaves exactly as it does today;
//   - the most recent N turns always stay verbatim;
//   - a nil compactor (the default — compaction off unless explicitly wired)
//     is a no-op via the nil-safe methods, so existing callers are unaffected.

import (
	stdctx "context"

	hctx "dev.helix.code/internal/context"
)

// HistoryCompactorConfig configures session-level compaction. It is a thin
// session-facing wrapper over context.CondenserConfig so callers configure
// compaction without importing internal/context directly.
type HistoryCompactorConfig struct {
	// Enabled gates the whole mechanism. False ⇒ every method is a no-op.
	Enabled bool
	// TokenThreshold is the history token count above which CompactIfNeeded
	// triggers. A history at or below this size is left untouched.
	TokenThreshold int
	// KeepRecentTurns is how many of the most recent turns stay verbatim.
	KeepRecentTurns int
}

// DefaultHistoryCompactorConfig returns a disabled-by-default config —
// compaction never changes behaviour unless explicitly opted in.
func DefaultHistoryCompactorConfig() HistoryCompactorConfig {
	d := hctx.DefaultCondenserConfig()
	return HistoryCompactorConfig{
		Enabled:         d.Enabled,
		TokenThreshold:  d.TokenThreshold,
		KeepRecentTurns: d.KeepRecentTurns,
	}
}

// toCondenserConfig converts the session-facing config to the condenser's.
func (c HistoryCompactorConfig) toCondenserConfig() hctx.CondenserConfig {
	return hctx.CondenserConfig{
		Enabled:         c.Enabled,
		TokenThreshold:  c.TokenThreshold,
		KeepRecentTurns: c.KeepRecentTurns,
	}
}

// CompactionResult reports the outcome of a session compaction. It re-exposes
// the condenser result in session terms so callers (and tests) get the
// before/after trajectory and the preserved facts without importing
// internal/context.
type CompactionResult struct {
	// Compacted is true when history was actually summarised.
	Compacted bool
	// SkipReason explains a non-Compacted result.
	SkipReason string
	// TokensBefore / TokensAfter are history token counts; the delta is the
	// anti-bluff proof that the window was bounded.
	TokensBefore int
	TokensAfter  int
	// TurnsBefore / TurnsAfter are turn counts.
	TurnsBefore int
	TurnsAfter  int
	// ActiveTask / Decisions / FilesTouched / OpenQuestions / Errors are the
	// task-critical facts the condenser preserved into the summary. Exposed so
	// the no-regression / task-success-parity tests can assert nothing was lost.
	ActiveTask    string
	Decisions     []string
	FilesTouched  []string
	OpenQuestions []string
	Errors        []string
	// Summary is the generated compact summary text.
	Summary string
}

// HistoryCompactor compacts a session's []Message transcript. It owns a
// context.Condenser and translates between the session and condenser types.
type HistoryCompactor struct {
	cfg       HistoryCompactorConfig
	condenser *hctx.Condenser
}

// NewHistoryCompactor builds a HistoryCompactor.
//
// counter and summarizer may both be nil: a nil counter falls back to the
// built-in char-based token estimate, and a nil summarizer falls back to the
// condenser's deterministic structured summary. This keeps the compactor fully
// usable (and unit-testable) without a live LLM provider, while production
// callers wire a real provider-backed summarizer for natural-language,
// locale-aware summaries (CONST-046).
func NewHistoryCompactor(cfg HistoryCompactorConfig, counter hctx.TokenCounter, summarizer hctx.Summarizer) *HistoryCompactor {
	return &HistoryCompactor{
		cfg:       cfg,
		condenser: hctx.NewCondenser(cfg.toCondenserConfig(), counter, summarizer),
	}
}

// messagesToTurns converts session messages to condenser turns.
func messagesToTurns(msgs []Message) []hctx.HistoryTurn {
	turns := make([]hctx.HistoryTurn, len(msgs))
	for i, m := range msgs {
		turns[i] = hctx.HistoryTurn{Role: m.Role, Content: m.Content}
	}
	return turns
}

// turnsToMessages converts condenser turns back to session messages. Timestamps
// are not carried on a synthesised summary turn — it is a fresh artefact.
func turnsToMessages(turns []hctx.HistoryTurn) []Message {
	msgs := make([]Message, len(turns))
	for i, t := range turns {
		msgs[i] = Message{Role: t.Role, Content: t.Content}
	}
	return msgs
}

// toCompactionResult adapts a condenser result to the session-facing shape.
func toCompactionResult(r hctx.CondenseResult) CompactionResult {
	return CompactionResult{
		Compacted:     r.Condensed,
		SkipReason:    r.SkipReason,
		TokensBefore:  r.TokensBefore,
		TokensAfter:   r.TokensAfter,
		TurnsBefore:   r.TurnsBefore,
		TurnsAfter:    r.TurnsAfter,
		ActiveTask:    r.Facts.ActiveTask,
		Decisions:     r.Facts.Decisions,
		FilesTouched:  r.Facts.FilesTouched,
		OpenQuestions: r.Facts.OpenQuestions,
		Errors:        r.Facts.Errors,
		Summary:       r.Summary,
	}
}

// CompactIfNeeded applies threshold-gated automatic compaction to history. A
// history at or below the configured token threshold is returned unchanged.
// This is the path the agent loop calls after each turn.
//
// Nil-safe: a nil *HistoryCompactor returns history unchanged with a no-op
// result, so callers can hold a nil compactor when compaction is disabled.
func (h *HistoryCompactor) CompactIfNeeded(ctx stdctx.Context, history []Message) ([]Message, CompactionResult) {
	if h == nil {
		return history, CompactionResult{
			SkipReason:   "compactor not configured",
			TokensBefore: 0,
			TokensAfter:  0,
			TurnsBefore:  len(history),
			TurnsAfter:   len(history),
		}
	}
	out, res := h.condenser.Condense(ctx, messagesToTurns(history))
	cr := toCompactionResult(res)
	if !res.Condensed {
		// Unchanged — return the original slice so byte-identity is preserved
		// (the no-regression guarantee for sub-threshold sessions).
		return history, cr
	}
	return turnsToMessages(out), cr
}

// Compact forces a one-shot compaction regardless of the token threshold. It
// backs an explicit `/compact`-style command. The Enabled gate still applies —
// an explicit /compact while compaction is config-disabled is still a no-op,
// so disabling compaction disables it everywhere.
//
// Nil-safe like CompactIfNeeded.
func (h *HistoryCompactor) Compact(ctx stdctx.Context, history []Message) ([]Message, CompactionResult) {
	if h == nil {
		return history, CompactionResult{
			SkipReason:  "compactor not configured",
			TurnsBefore: len(history),
			TurnsAfter:  len(history),
		}
	}
	out, res := h.condenser.CondenseNow(ctx, messagesToTurns(history))
	cr := toCompactionResult(res)
	if !res.Condensed {
		return history, cr
	}
	return turnsToMessages(out), cr
}

// ShouldCompact reports whether history would trigger automatic compaction.
// Nil-safe — a nil compactor never triggers.
func (h *HistoryCompactor) ShouldCompact(history []Message) bool {
	if h == nil {
		return false
	}
	return h.condenser.ShouldCondense(messagesToTurns(history))
}

// Enabled reports whether this compactor is active. Nil-safe.
func (h *HistoryCompactor) Enabled() bool {
	return h != nil && h.cfg.Enabled
}
