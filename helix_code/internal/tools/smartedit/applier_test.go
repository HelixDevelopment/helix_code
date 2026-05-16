package smartedit

import (
	"bytes"
	"testing"
)

// TestApplyPlanToContent_SingleBlock_Applied asserts the happy path: a
// single block whose SEARCH matches exactly once is applied and the
// content is mutated in place.
func TestApplyPlanToContent_SingleBlock_Applied(t *testing.T) {
	content := []byte("alpha\nbeta\ngamma\n")
	blocks := []EditBlock{{
		Path:    "x.txt",
		Search:  "beta\n",
		Replace: "BETA\n",
	}}

	got, results, allApplied := ApplyPlanToContent(content, blocks)

	if !allApplied {
		t.Fatalf("expected allApplied=true, got false; results=%+v", results)
	}
	if string(got) != "alpha\nBETA\ngamma\n" {
		t.Errorf("content mismatch:\nwant %q\ngot  %q", "alpha\nBETA\ngamma\n", string(got))
	}
	if len(results) != 1 || results[0].Outcome != OutcomeApplied {
		t.Errorf("expected one OutcomeApplied result, got %+v", results)
	}
	if results[0].Error != "" {
		t.Errorf("expected empty Error on applied block, got %q", results[0].Error)
	}
}

// TestApplyPlanToContent_SearchNotFound_OutcomeNotFound asserts that a
// SEARCH absent from content yields OutcomeNotFound, the content is
// unchanged, and allApplied is false.
func TestApplyPlanToContent_SearchNotFound_OutcomeNotFound(t *testing.T) {
	content := []byte("hello world\n")
	blocks := []EditBlock{{
		Path:    "x.txt",
		Search:  "missing",
		Replace: "found",
	}}

	got, results, allApplied := ApplyPlanToContent(content, blocks)

	if allApplied {
		t.Errorf("expected allApplied=false when SEARCH not found")
	}
	if !bytes.Equal(got, content) {
		t.Errorf("content mutated despite not-found block:\nwant %q\ngot  %q", string(content), string(got))
	}
	if len(results) != 1 || results[0].Outcome != OutcomeNotFound {
		t.Errorf("expected one OutcomeNotFound result, got %+v", results)
	}
	if results[0].Error == "" {
		t.Errorf("expected non-empty Error on not-found block")
	}
}

// TestApplyPlanToContent_AmbiguousMatch_OutcomeAmbiguous asserts that a
// SEARCH appearing multiple times yields OutcomeAmbiguous with the content
// unchanged. This is the semantics-defining test for the ambiguity rule.
func TestApplyPlanToContent_AmbiguousMatch_OutcomeAmbiguous(t *testing.T) {
	content := []byte("foo\nbar\nfoo\n")
	blocks := []EditBlock{{
		Path:    "x.txt",
		Search:  "foo\n",
		Replace: "FOO\n",
	}}

	got, results, allApplied := ApplyPlanToContent(content, blocks)

	if allApplied {
		t.Errorf("expected allApplied=false on ambiguous match")
	}
	if !bytes.Equal(got, content) {
		t.Errorf("content mutated despite ambiguous block:\nwant %q\ngot  %q", string(content), string(got))
	}
	if len(results) != 1 || results[0].Outcome != OutcomeAmbiguous {
		t.Errorf("expected one OutcomeAmbiguous result, got %+v", results)
	}
	if results[0].Error == "" {
		t.Errorf("expected non-empty Error on ambiguous block")
	}
}

// TestApplyPlanToContent_MultipleBlocks_AllApplied asserts two disjoint
// blocks both apply against the same file and mutate it correctly.
func TestApplyPlanToContent_MultipleBlocks_AllApplied(t *testing.T) {
	content := []byte("alpha\nbeta\ngamma\ndelta\n")
	blocks := []EditBlock{
		{Path: "x.txt", Search: "alpha\n", Replace: "ALPHA\n"},
		{Path: "x.txt", Search: "gamma\n", Replace: "GAMMA\n"},
	}

	got, results, allApplied := ApplyPlanToContent(content, blocks)

	if !allApplied {
		t.Fatalf("expected allApplied=true, got false; results=%+v", results)
	}
	if string(got) != "ALPHA\nbeta\nGAMMA\ndelta\n" {
		t.Errorf("content mismatch:\nwant %q\ngot  %q", "ALPHA\nbeta\nGAMMA\ndelta\n", string(got))
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	for i, r := range results {
		if r.Outcome != OutcomeApplied {
			t.Errorf("result[%d] outcome=%q, want OutcomeApplied", i, r.Outcome)
		}
	}
}

// TestApplyPlanToContent_MultipleBlocks_Composable is the load-bearing
// composability anchor referenced by spec §4.2: block 2's SEARCH text
// only appears AFTER block 1's REPLACE has executed. Both blocks must
// apply because the applier re-searches the running mutated content.
//
// This test is what differentiates a lenient re-search applier from a
// snapshot applier; if it fails, the implementation is wrong even if
// every other test passes.
func TestApplyPlanToContent_MultipleBlocks_Composable(t *testing.T) {
	content := []byte("foo\nbaseline\n")
	blocks := []EditBlock{
		// Block 1: introduces "bar" by replacing "foo".
		{Path: "x.txt", Search: "foo\n", Replace: "bar\n"},
		// Block 2: SEARCH text "bar" only exists post-block-1.
		{Path: "x.txt", Search: "bar\n", Replace: "BAR_FINAL\n"},
	}

	got, results, allApplied := ApplyPlanToContent(content, blocks)

	if !allApplied {
		t.Fatalf("expected both composable blocks to apply; results=%+v", results)
	}
	if string(got) != "BAR_FINAL\nbaseline\n" {
		t.Errorf("composability final state wrong:\nwant %q\ngot  %q", "BAR_FINAL\nbaseline\n", string(got))
	}
	for i, r := range results {
		if r.Outcome != OutcomeApplied {
			t.Errorf("result[%d] outcome=%q, want OutcomeApplied", i, r.Outcome)
		}
	}
}

// TestApplyPlanToContent_MultipleBlocks_FirstFailsContinue asserts that the
// applier does not short-circuit on the first failed block. It records the
// per-block outcome and proceeds; the higher-level smart_edit_tool decides
// whether to commit any of them. allApplied must be false because at least
// one block failed.
func TestApplyPlanToContent_MultipleBlocks_FirstFailsContinue(t *testing.T) {
	content := []byte("alpha\nbeta\n")
	blocks := []EditBlock{
		{Path: "x.txt", Search: "missing\n", Replace: "anything\n"},
		{Path: "x.txt", Search: "beta\n", Replace: "BETA\n"},
	}

	got, results, allApplied := ApplyPlanToContent(content, blocks)

	if allApplied {
		t.Errorf("expected allApplied=false when any block fails")
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Outcome != OutcomeNotFound {
		t.Errorf("result[0] outcome=%q, want OutcomeNotFound", results[0].Outcome)
	}
	if results[1].Outcome != OutcomeApplied {
		t.Errorf("result[1] outcome=%q, want OutcomeApplied (must continue past failure)", results[1].Outcome)
	}
	// The mutated content reflects block 2's success even though block 1
	// failed; the higher tier (smart_edit_tool, T06) is responsible for
	// discarding this content if any block on any file failed.
	if string(got) != "alpha\nBETA\n" {
		t.Errorf("unexpected content:\nwant %q\ngot  %q", "alpha\nBETA\n", string(got))
	}
}

// TestApplyPlanToContent_EmptyBlocks asserts that zero blocks is a no-op:
// content is returned unchanged, perBlock is empty, and allApplied is true
// (vacuously — no failures occurred).
func TestApplyPlanToContent_EmptyBlocks(t *testing.T) {
	content := []byte("unchanged\n")
	got, results, allApplied := ApplyPlanToContent(content, []EditBlock{})

	if !allApplied {
		t.Errorf("empty blocks must yield allApplied=true (vacuous)")
	}
	if !bytes.Equal(got, content) {
		t.Errorf("empty blocks must not mutate content")
	}
	if len(results) != 0 {
		t.Errorf("empty blocks must yield empty results, got %+v", results)
	}
}

// TestApplyPlanToContent_PreservesOrder asserts that the perBlock slice is
// indexed in source-block order regardless of which blocks succeed. This is
// the guarantee the slash command relies on when rendering per-block status
// next to the originating SEARCH/REPLACE hunk.
func TestApplyPlanToContent_PreservesOrder(t *testing.T) {
	content := []byte("a\nb\nc\nd\n")
	blocks := []EditBlock{
		{Path: "x.txt", Search: "a\n", Replace: "A\n"},
		{Path: "x.txt", Search: "missing\n", Replace: "x\n"},
		{Path: "x.txt", Search: "c\n", Replace: "C\n"},
	}

	_, results, _ := ApplyPlanToContent(content, blocks)

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	wantOutcomes := []EditOutcome{OutcomeApplied, OutcomeNotFound, OutcomeApplied}
	for i, want := range wantOutcomes {
		if results[i].Outcome != want {
			t.Errorf("result[%d] outcome=%q, want %q", i, results[i].Outcome, want)
		}
		if results[i].Block.Search != blocks[i].Search {
			t.Errorf("result[%d] block.Search=%q, want %q (order broken)", i, results[i].Block.Search, blocks[i].Search)
		}
	}
}

// TestApplyPlanToContent_MultilineSearch asserts that SEARCH bodies
// spanning multiple lines are matched and replaced as a single literal
// substring, preserving whatever surrounds them.
func TestApplyPlanToContent_MultilineSearch(t *testing.T) {
	content := []byte("header\nfunc Foo() {\n    return 1\n}\nfooter\n")
	blocks := []EditBlock{{
		Path:    "x.go",
		Search:  "func Foo() {\n    return 1\n}\n",
		Replace: "func Foo() {\n    return 42\n}\n",
	}}

	got, results, allApplied := ApplyPlanToContent(content, blocks)

	if !allApplied {
		t.Fatalf("multi-line block did not apply; results=%+v", results)
	}
	want := "header\nfunc Foo() {\n    return 42\n}\nfooter\n"
	if string(got) != want {
		t.Errorf("multi-line replace mismatch:\nwant %q\ngot  %q", want, string(got))
	}
}

// TestApplyPlanToContent_NoMutationOnFailure asserts byte-equality between
// input and output content when the only block fails. This is the safety
// guarantee callers depend on: a failed apply must NEVER produce partially
// edited content.
func TestApplyPlanToContent_NoMutationOnFailure(t *testing.T) {
	original := []byte("line1\nline2\nline3\n")
	// Defensive copy — the applier MUST NOT mutate the input slice.
	input := make([]byte, len(original))
	copy(input, original)

	blocks := []EditBlock{{
		Path:    "x.txt",
		Search:  "absent\n",
		Replace: "anything\n",
	}}

	got, _, allApplied := ApplyPlanToContent(input, blocks)
	if allApplied {
		t.Errorf("expected failure")
	}
	if !bytes.Equal(got, original) {
		t.Errorf("output mutated despite failure:\nwant %q\ngot  %q", string(original), string(got))
	}
	if !bytes.Equal(input, original) {
		t.Errorf("input slice mutated by applier:\nwant %q\ngot  %q", string(original), string(input))
	}
}

// TestFindUnique_Found asserts the canonical unambiguous lookup path.
func TestFindUnique_Found(t *testing.T) {
	idx, ambiguous := findUnique("alpha beta gamma", "beta")
	if ambiguous {
		t.Errorf("expected non-ambiguous match")
	}
	if idx != 6 {
		t.Errorf("expected idx=6, got %d", idx)
	}
}

// TestFindUnique_NotFound asserts (-1, false) for absent needles.
func TestFindUnique_NotFound(t *testing.T) {
	idx, ambiguous := findUnique("alpha beta gamma", "delta")
	if ambiguous {
		t.Errorf("expected ambiguous=false for not-found")
	}
	if idx != -1 {
		t.Errorf("expected idx=-1, got %d", idx)
	}
}

// TestFindUnique_Ambiguous asserts the canonical ambiguity signal: needle
// appears more than once → (-1, true).
func TestFindUnique_Ambiguous(t *testing.T) {
	idx, ambiguous := findUnique("foo bar foo", "foo")
	if !ambiguous {
		t.Errorf("expected ambiguous=true for duplicate match")
	}
	if idx != -1 {
		t.Errorf("expected idx=-1 on ambiguous, got %d", idx)
	}
}

// TestFindUnique_EmptyNeedle pins the empty-needle decision: an empty
// needle is treated as NOT FOUND ((-1, false)). Rationale: strings.Count
// reports utf8.RuneCountInString(haystack)+1 occurrences of "", which
// would always be ambiguous and thus always reject empty-needle blocks
// anyway. Returning not-found is equally safe and matches the parser's
// ErrSearchEmpty rejection at parse time — the applier should never
// receive an empty-needle block in practice, so either signal is
// defensive. We choose not-found because (a) it is the lighter classification
// and (b) it keeps the (-1, ambiguous=true) sentinel reserved for true
// duplicate matches.
func TestFindUnique_EmptyNeedle(t *testing.T) {
	idx, ambiguous := findUnique("hello world", "")
	if ambiguous {
		t.Errorf("expected ambiguous=false for empty needle (chosen semantics: not-found)")
	}
	if idx != -1 {
		t.Errorf("expected idx=-1 for empty needle, got %d", idx)
	}
}
