// Package smartedit — fuzzy.go (speed programme P3-T02).
//
// Diff-style edits: instead of the LLM re-emitting an entire rewritten file,
// it emits ONLY the changed lines as a SEARCH/REPLACE block. The parser
// (parser.go) already accepts that minimal form; this file adds the
// whitespace-tolerant *applier* that locates a SEARCH block even when the
// model's reproduction of the surrounding context has drifted slightly
// (re-indentation, tab/space mix, trailing-whitespace churn).
//
// Why fuzzy matching matters for speed: a strict applier rejects a block the
// moment one space is off, forcing the agent into a costly retry round-trip
// (full-rewrite fallback, or a re-prompt). A whitespace-tolerant applier
// absorbs the most common LLM reproduction error and lets the ~30%-fewer-token
// diff format actually land — which is the whole point of P3-T02.
//
// SAFETY CONTRACT (Med-risk task — a malformed/mis-located diff MUST NOT
// silently corrupt a file):
//
//   - Tier 1 — exact match. Identical to findUnique in applier.go. If the
//     SEARCH text is byte-present exactly once, it is used verbatim. This is
//     attempted FIRST so a fuzzy apply is byte-identical to a strict apply
//     whenever the model got the context exactly right.
//   - Tier 2 — whitespace-normalised line-block match. The SEARCH and the
//     file are compared line-by-line after collapsing each line's internal
//     run of whitespace and trimming leading/trailing whitespace. A match is
//     accepted ONLY when it is UNIQUE under the normalised comparison; two or
//     more normalised matches → OutcomeAmbiguous, exactly as the strict
//     applier fails closed. The REPLACE text is spliced in at the byte span
//     of the matched ORIGINAL lines, so the file's own indentation around the
//     hunk is never disturbed.
//   - No tier silently picks "the first of several" — every tier fails closed
//     on ambiguity. A non-locating SEARCH yields OutcomeNotFound and ZERO
//     mutation; a structurally malformed prompt is rejected by Parse upstream
//     before the applier is ever reached.
//
// ApplyPlanToContentFuzzy is ADDITIVE: the strict ApplyPlanToContent in
// applier.go is unchanged and remains the default. Callers opt into fuzzy
// matching explicitly. The full-rewrite format and every existing strict
// test therefore cannot regress.
package smartedit

import "strings"

// ApplyPlanToContentFuzzy applies a sequence of EditBlocks with the two-tier
// (exact → whitespace-tolerant) location strategy described in the file
// header. Its signature, re-search semantics, per-block continuation, and
// no-mutation-on-total-failure guarantees are identical to the strict
// ApplyPlanToContent — only the SEARCH location step is more lenient.
//
// Re-search semantics: each block's SEARCH is located in the CURRENT
// (possibly already-mutated) content, so SEARCH/REPLACE sequences remain
// composable.
//
// Returns the same triple as ApplyPlanToContent:
//   - newContent: post-apply content (== input if every block failed).
//   - perBlock:   one EditResult per block, in source order.
//   - allApplied: true iff every block has Outcome == OutcomeApplied
//     (vacuously true for an empty slice).
//
// Pure function: does not touch disk and does not mutate the input slice.
func ApplyPlanToContentFuzzy(content []byte, blocks []EditBlock) (newContent []byte, perBlock []EditResult, allApplied bool) {
	working := string(content)

	if len(blocks) == 0 {
		return content, nil, true
	}

	results := make([]EditResult, 0, len(blocks))
	allOK := true

	for _, blk := range blocks {
		start, end, outcome := locateFuzzy(working, blk.Search)
		switch outcome {
		case OutcomeApplied:
			// Splice REPLACE in at the located [start,end) byte span. For an
			// exact (tier-1) hit, end == start+len(Search) so this is
			// byte-identical to the strict applier. For a tier-2 hit, the
			// span covers the file's ORIGINAL lines, so the surrounding
			// indentation is preserved untouched.
			working = working[:start] + blk.Replace + working[end:]
			results = append(results, EditResult{Block: blk, Outcome: OutcomeApplied})
		case OutcomeAmbiguous:
			results = append(results, EditResult{
				Block:   blk,
				Outcome: OutcomeAmbiguous,
				Error:   ErrSearchAmbiguous.Error(),
			})
			allOK = false
		default: // OutcomeNotFound
			results = append(results, EditResult{
				Block:   blk,
				Outcome: OutcomeNotFound,
				Error:   ErrSearchNotFound.Error(),
			})
			allOK = false
		}
	}

	return []byte(working), results, allOK
}

// locateFuzzy finds the byte span [start,end) in haystack that the needle
// identifies, using exact match first and a unique whitespace-normalised
// line-block match as a fallback.
//
// Returns:
//   - (idx, idx+len(needle), OutcomeApplied)         — exact unique match.
//   - (start, end,           OutcomeApplied)         — unique fuzzy match;
//     span covers the matched ORIGINAL lines.
//   - (-1, -1,               OutcomeAmbiguous)       — needle matches more
//     than one location at the FIRST tier that produced any match.
//   - (-1, -1,               OutcomeNotFound)        — no location at all,
//     or empty needle.
//
// Empty needle → not-found, matching findUnique and the parser's
// ErrSearchEmpty rejection. The applier should never see an empty needle.
func locateFuzzy(haystack, needle string) (start, end int, outcome EditOutcome) {
	if needle == "" {
		return -1, -1, OutcomeNotFound
	}

	// Tier 1: exact match. Reuse the strict semantics verbatim so a fuzzy
	// apply is provably identical to a strict apply when the model nailed
	// the context.
	switch count := strings.Count(haystack, needle); {
	case count == 1:
		idx := strings.Index(haystack, needle)
		return idx, idx + len(needle), OutcomeApplied
	case count > 1:
		// Exact-but-duplicate: ambiguous at tier 1. Do NOT fall through to
		// tier 2 — the duplicate is genuine and the user must widen.
		return -1, -1, OutcomeAmbiguous
	}

	// Tier 2: whitespace-normalised line-block match.
	return locateNormalised(haystack, needle)
}

// locateNormalised performs the tier-2 whitespace-tolerant line-block search.
//
// Both haystack and needle are split into lines; each line is normalised by
// trimming leading/trailing whitespace and collapsing every internal run of
// spaces/tabs to a single space. The needle's normalised line slice is then
// searched for as a contiguous sublist of the haystack's normalised line
// slice. The match is accepted ONLY if it is unique; >1 match → ambiguous.
//
// On a unique match the returned span covers the ORIGINAL (un-normalised)
// haystack lines so the REPLACE text is spliced in without disturbing the
// file's real indentation outside the hunk.
func locateNormalised(haystack, needle string) (start, end int, outcome EditOutcome) {
	hLines, hStarts := splitLinesWithOffsets(haystack)
	nLines, _ := splitLinesWithOffsets(needle)

	// Drop a trailing empty needle line caused by the needle ending in '\n'
	// (SEARCH bodies always do — parser.go appends '\n' per line). The
	// haystack keeps its trailing-empty line so an offset is always valid.
	nNorm := normaliseLines(nLines)
	for len(nNorm) > 0 && nNorm[len(nNorm)-1] == "" {
		nNorm = nNorm[:len(nNorm)-1]
		nLines = nLines[:len(nLines)-1]
	}
	if len(nNorm) == 0 {
		// Needle was all-whitespace — refuse to match anything (a
		// whitespace-only SEARCH would be unboundedly ambiguous).
		return -1, -1, OutcomeNotFound
	}

	hNorm := normaliseLines(hLines)

	matchCount := 0
	var firstStart, firstEnd int
	for i := 0; i+len(nNorm) <= len(hNorm); i++ {
		ok := true
		for j := 0; j < len(nNorm); j++ {
			if hNorm[i+j] != nNorm[j] {
				ok = false
				break
			}
		}
		if !ok {
			continue
		}
		matchCount++
		if matchCount == 1 {
			firstStart = hStarts[i]
			// End is the byte offset just past the last matched original
			// line INCLUDING its trailing '\n'. Each parser-produced SEARCH
			// line carries an appended '\n', and the REPLACE text likewise
			// carries its own line terminators, so the span we excise must
			// cover the original lines' '\n's too — otherwise the file's
			// terminator and the REPLACE's terminator both survive and the
			// hunk gains a spurious blank line.
			lastLine := i + len(nNorm) - 1
			firstEnd = hStarts[lastLine] + len(hLines[lastLine])
			// Consume the trailing '\n' of the last matched line if present
			// (every line except a no-final-newline EOF line has one).
			if firstEnd < len(haystack) && haystack[firstEnd] == '\n' {
				firstEnd++
			}
		}
		if matchCount > 1 {
			return -1, -1, OutcomeAmbiguous
		}
	}

	if matchCount == 1 {
		return firstStart, firstEnd, OutcomeApplied
	}
	return -1, -1, OutcomeNotFound
}

// splitLinesWithOffsets splits s into lines, each WITHOUT its trailing '\n',
// and returns a parallel slice of byte offsets for each line's first byte in
// s. A string ending in '\n' yields a trailing empty line whose offset is
// len(s), so callers can always compute a valid splice boundary.
func splitLinesWithOffsets(s string) (lines []string, offsets []int) {
	if s == "" {
		return []string{""}, []int{0}
	}
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			offsets = append(offsets, start)
			start = i + 1
		}
	}
	// Final segment after the last '\n' (empty if s ended with '\n').
	lines = append(lines, s[start:])
	offsets = append(offsets, start)
	return lines, offsets
}

// normaliseLines returns a copy of lines with each entry whitespace-normalised
// per normaliseLine.
func normaliseLines(lines []string) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = normaliseLine(l)
	}
	return out
}

// normaliseLine collapses a single line for whitespace-tolerant comparison:
// leading/trailing whitespace is trimmed and every internal run of spaces and
// tabs is collapsed to exactly one space. Two lines that differ only in
// indentation depth, tab-vs-space, or trailing whitespace normalise equal.
//
// This is deliberately conservative — it tolerates whitespace drift but NOT
// content drift: a single changed identifier still makes two lines differ, so
// a fuzzy match can never silently apply a REPLACE to the wrong code.
func normaliseLine(line string) string {
	var b strings.Builder
	b.Grow(len(line))
	inSpace := false
	started := false
	for i := 0; i < len(line); i++ {
		c := line[i]
		if c == ' ' || c == '\t' || c == '\r' {
			inSpace = true
			continue
		}
		if inSpace && started {
			b.WriteByte(' ')
		}
		b.WriteByte(c)
		inSpace = false
		started = true
	}
	return b.String()
}
