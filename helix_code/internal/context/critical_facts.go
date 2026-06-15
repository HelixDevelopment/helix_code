package context

// Task-critical fact extraction for the history condenser (speed programme
// P3-T05).
//
// Before any history span is summarised, this file mines it for the facts the
// agent MUST NOT forget to continue the task correctly: the active task, the
// decisions made, the files touched, the open questions, and the errors
// encountered. These facts are folded into the condensed summary EXPLICITLY so
// they survive compaction by construction — the no-regression guarantee does
// not depend on the LLM summariser remembering them.
//
// Extraction is deterministic, dependency-free pattern matching. It is
// intentionally conservative: a false positive (an extra preserved fact) is
// harmless; a false negative (a dropped task-critical fact) is the failure
// mode. When in doubt, the extractor keeps the fact.

import (
	"regexp"
	"strings"
)

// Pre-compiled patterns (hoisted to package level so the regex engine compiles
// them once, not per Condense call — mirrors the P2-T02 regex-hoist work).
var (
	// reActiveTask matches an explicit statement of the task being worked on.
	reActiveTask = regexp.MustCompile(`(?i)\b(?:working on|task is|current task|the task|implementing|i(?:'m| am) (?:working on|implementing|building)|let'?s (?:implement|build|fix))\b[:\s]+(.+)`)

	// reDecision matches a recorded decision / choice.
	reDecision = regexp.MustCompile(`(?i)\b(?:decided to|i(?:'ll| will)|we(?:'ll| will)|chose to|going with|opting for|the approach is|decision[:\s])\b[:\s]*(.+)`)

	// reOpenQuestion matches an unresolved question.
	reOpenQuestion = regexp.MustCompile(`(?i)\b(?:open question|still need to (?:know|decide|clarify)|unclear|not sure (?:whether|if|how)|need to (?:confirm|check)|TBD|to be (?:decided|determined))\b[:\s]*(.*)`)

	// reError matches an error / failure mention.
	reError = regexp.MustCompile(`(?i)\b(?:error|failed|failure|panic|exception|cannot|could not|unable to|broke|broken)\b[:\s]*(.*)`)

	// reFilePath matches a plausible source-file path with a known extension.
	reFilePath = regexp.MustCompile(`(?:[\w./\-]+/)?[\w.\-]+\.(?:go|py|js|ts|tsx|jsx|java|kt|rs|c|h|cpp|cc|rb|sh|yaml|yml|json|toml|md|sql|proto|html|css)\b`)
)

// maxFactsPerCategory caps how many distinct facts of each kind are preserved.
// A cap keeps the condensed summary itself bounded — an unbounded fact list
// would re-bloat the very window the condenser exists to shrink. The cap is
// generous enough that real task-critical context is never lost for normal
// runs; the most recent matches win when the cap is hit (recency = relevance).
const maxFactsPerCategory = 24

// ExtractCriticalFacts mines turns for the task-critical context that must
// survive condensation. It is deterministic and never panics on any input.
//
// The returned CriticalFacts is the contract the condensed summary preserves:
// every field here is rendered verbatim into the summary by
// renderStructuredSummary (and an LLM Summarizer is instructed to do the same).
func ExtractCriticalFacts(turns []HistoryTurn) CriticalFacts {
	var (
		activeTask string
		decisions  []string
		files      []string
		questions  []string
		errs       []string
	)

	for _, t := range turns {
		// Split into lines so a single multi-line tool result does not collapse
		// into one giant "fact".
		for _, raw := range strings.Split(t.Content, "\n") {
			line := strings.TrimSpace(raw)
			if line == "" {
				continue
			}

			// Active task: the LAST match wins — the most recent statement of
			// what the agent is doing is the authoritative one.
			if m := reActiveTask.FindStringSubmatch(line); m != nil {
				if cand := cleanFact(m[1]); cand != "" {
					activeTask = cand
				}
			}

			if m := reDecision.FindStringSubmatch(line); m != nil {
				if cand := cleanFact(m[1]); cand != "" {
					decisions = append(decisions, cand)
				}
			}

			if m := reOpenQuestion.FindStringSubmatch(line); m != nil {
				// The matched tail may be empty (e.g. a bare "TBD"); fall back
				// to the whole line so the question is not lost.
				cand := cleanFact(m[1])
				if cand == "" {
					cand = cleanFact(line)
				}
				if cand != "" {
					questions = append(questions, cand)
				}
			}

			if reError.MatchString(line) {
				if cand := cleanFact(line); cand != "" {
					errs = append(errs, cand)
				}
			}

			// File paths can appear anywhere on a line and several per line.
			for _, fp := range reFilePath.FindAllString(line, -1) {
				files = append(files, fp)
			}
		}
	}

	return CriticalFacts{
		ActiveTask: activeTask,
		Decisions:  capFacts(dedupePreserveOrder(decisions)),
		// FilesTouched MUST honour the recency contract just like the other
		// categories: dedupe preserving recency (insertion) order, cap to keep
		// the most-recently-touched tail, and only THEN sort a COPY for stable
		// display. Sorting BEFORE the cap (the historical bug) made the
		// alpha-last 24 win instead of the most-recent 24, silently dropping
		// the active-work file from the condensed summary.
		FilesTouched:  sortedCopy(capFacts(dedupePreserveOrder(files))),
		OpenQuestions: capFacts(dedupePreserveOrder(questions)),
		Errors:        capFacts(dedupePreserveOrder(errs)),
	}
}

// cleanFact trims a captured fact and bounds its length so one runaway line
// cannot dominate the summary. Returns "" for empty / whitespace-only input.
func cleanFact(s string) string {
	s = strings.TrimSpace(s)
	// Strip a trailing sentence terminator group for tidier digests.
	s = strings.TrimRight(s, " \t")
	const maxFactLen = 280
	if len(s) > maxFactLen {
		// Trim on a rune boundary to avoid splitting a multi-byte rune.
		cut := maxFactLen
		for cut > 0 && !isRuneStart(s[cut]) {
			cut--
		}
		s = strings.TrimSpace(s[:cut]) + "…"
	}
	return s
}

// isRuneStart reports whether b is a UTF-8 leading byte (not a continuation
// byte). Continuation bytes have the top two bits set to 10.
func isRuneStart(b byte) bool {
	return b&0xC0 != 0x80
}

// capFacts truncates a fact list to maxFactsPerCategory, keeping the MOST
// RECENT entries (the tail) when the cap is exceeded — recent facts are the
// most likely to still be relevant to the continuing task.
func capFacts(items []string) []string {
	if len(items) <= maxFactsPerCategory {
		return items
	}
	return items[len(items)-maxFactsPerCategory:]
}
