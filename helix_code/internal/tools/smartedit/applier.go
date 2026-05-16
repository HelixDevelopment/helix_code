package smartedit

import (
	"strings"
)

// ApplyPlanToContent applies a sequence of EditBlocks to a single file's
// content using lenient re-search semantics.
//
// Re-search semantics: each block's SEARCH text is re-located in the
// CURRENT (possibly already-mutated by earlier blocks) content. This makes
// SEARCH/REPLACE sequences composable — a later block may legitimately
// match against text that an earlier block introduced via REPLACE.
// `TestApplyPlanToContent_MultipleBlocks_Composable` is the canonical
// anchor for this guarantee.
//
// Ambiguity: if SEARCH appears more than once in the running content, the
// block fails with `OutcomeAmbiguous`. The user must widen the SEARCH with
// surrounding context until it identifies a single hunk uniquely. Failing
// closed (rather than picking the first match) is what prevents accidental
// edits to the wrong call site in repos that contain duplicated boilerplate.
//
// Per-block continuation: a block that fails (not-found OR ambiguous) does
// NOT halt the loop. The applier still attempts every subsequent block and
// reports the per-block outcome. The caller (smart_edit_tool, T06) is
// responsible for the higher-tier whole-prompt atomicity gate per spec
// §4.2: if any block on any file failed, no file is committed. The
// applier itself is purely informational — its returned `newContent` is
// what the file would look like if it were committed; whether it actually
// gets committed is decided one tier up.
//
// Returns:
//   - newContent: the post-apply content. Equal to input if every block
//     failed; otherwise reflects every successfully applied block's REPLACE.
//   - perBlock: one EditResult per input block, in source order. Only
//     `Outcome` and `Error` (and `Block` for context) are populated; `Diff`
//     is left empty for T05 to compute.
//   - allApplied: true iff every block has Outcome=OutcomeApplied. Vacuously
//     true for an empty `blocks` slice.
//
// Pure function: does NOT touch disk and does NOT mutate the input slice
// (a working copy via `string(content)` insulates the caller).
func ApplyPlanToContent(content []byte, blocks []EditBlock) (newContent []byte, perBlock []EditResult, allApplied bool) {
	// Working copy as a Go string so we can use strings.Count / strings.Index
	// directly. The applier is byte-faithful: \r, BOMs, trailing spaces are
	// preserved exactly because Go strings are immutable byte sequences.
	working := string(content)

	if len(blocks) == 0 {
		// Vacuous success: no blocks attempted, so every block trivially
		// applied. Caller distinguishes "no work" from "all done" via the
		// empty perBlock slice.
		return content, nil, true
	}

	results := make([]EditResult, 0, len(blocks))
	allOK := true

	for _, blk := range blocks {
		idx, ambiguous := findUnique(working, blk.Search)
		switch {
		case ambiguous:
			results = append(results, EditResult{
				Block:   blk,
				Outcome: OutcomeAmbiguous,
				Error:   ErrSearchAmbiguous.Error(),
			})
			allOK = false
		case idx == -1:
			results = append(results, EditResult{
				Block:   blk,
				Outcome: OutcomeNotFound,
				Error:   ErrSearchNotFound.Error(),
			})
			allOK = false
		default:
			// Single-occurrence replacement. We splice the REPLACE text in
			// at the located index rather than calling strings.Replace,
			// because Replace(_, _, _, 1) re-runs the search and would
			// duplicate the work we already did. Slicing also makes the
			// uniqueness invariant locally provable.
			working = working[:idx] + blk.Replace + working[idx+len(blk.Search):]
			results = append(results, EditResult{
				Block:   blk,
				Outcome: OutcomeApplied,
			})
		}
	}

	return []byte(working), results, allOK
}

// findUnique locates needle in haystack with strict uniqueness:
//
//   - Empty needle → (-1, false). An empty needle has unbounded match count
//     under strings.Count semantics; rather than collapse it into the
//     ambiguous bucket (which would also be a defensible choice, see
//     `TestFindUnique_EmptyNeedle`), we treat it as not-found. This keeps
//     the ambiguous signal reserved for genuine duplicate-match conditions
//     and matches the parser's ErrSearchEmpty rejection at parse time —
//     the applier should never legitimately see an empty needle.
//   - Needle absent → (-1, false).
//   - Needle exactly once → (firstIndex, false).
//   - Needle more than once → (-1, true).
//
// Counting before locating costs one extra pass over the haystack but
// guarantees the ambiguity check is performed on the SAME content the
// caller will later splice into. The two-pass cost is negligible at the
// MaxFileBytes cap (10 MiB) compared to the simplicity it buys.
func findUnique(haystack, needle string) (idx int, ambiguous bool) {
	if needle == "" {
		return -1, false
	}
	count := strings.Count(haystack, needle)
	switch {
	case count == 0:
		return -1, false
	case count > 1:
		return -1, true
	default:
		return strings.Index(haystack, needle), false
	}
}
