package smartedit

import (
	"bytes"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Tier-1 parity: ApplyPlanToContentFuzzy must be byte-identical to the strict
// ApplyPlanToContent whenever the SEARCH text matches exactly. This is the
// no-regression anchor — fuzzy is additive, never a behaviour change for
// exact diffs.
// ---------------------------------------------------------------------------

// TestFuzzy_ExactMatch_IdenticalToStrict applies the same exact-match block
// through both appliers and asserts the outputs are byte-equal.
func TestFuzzy_ExactMatch_IdenticalToStrict(t *testing.T) {
	content := []byte("alpha\nbeta\ngamma\n")
	blocks := []EditBlock{{Path: "x.txt", Search: "beta\n", Replace: "BETA\n"}}

	strictOut, _, strictOK := ApplyPlanToContent(content, blocks)
	fuzzyOut, results, fuzzyOK := ApplyPlanToContentFuzzy(content, blocks)

	if !strictOK || !fuzzyOK {
		t.Fatalf("both appliers must succeed on exact match: strict=%v fuzzy=%v", strictOK, fuzzyOK)
	}
	if !bytes.Equal(strictOut, fuzzyOut) {
		t.Errorf("fuzzy diverged from strict on exact match:\nstrict %q\nfuzzy  %q", strictOut, fuzzyOut)
	}
	if string(fuzzyOut) != "alpha\nBETA\ngamma\n" {
		t.Errorf("wrong output: %q", fuzzyOut)
	}
	if results[0].Outcome != OutcomeApplied {
		t.Errorf("want OutcomeApplied, got %q", results[0].Outcome)
	}
}

// TestFuzzy_RoundTrip_KnownChange emits a diff for a known change, applies it,
// and asserts the file equals the intended target — the core round-trip proof.
func TestFuzzy_RoundTrip_KnownChange(t *testing.T) {
	original := "func add(a, b int) int {\n\treturn a + b\n}\n"
	intended := "func add(a, b int) int {\n\treturn a + b + 1\n}\n"

	// The diff a model would emit: ONLY the changed line, with one context-free
	// SEARCH/REPLACE block — the changed-lines-only form of P3-T02.
	blocks := []EditBlock{{
		Path:    "math.go",
		Search:  "\treturn a + b\n",
		Replace: "\treturn a + b + 1\n",
	}}

	out, _, ok := ApplyPlanToContentFuzzy([]byte(original), blocks)
	if !ok {
		t.Fatalf("round-trip diff did not apply")
	}
	if string(out) != intended {
		t.Errorf("round-trip mismatch:\nwant %q\ngot  %q", intended, out)
	}
}

// ---------------------------------------------------------------------------
// Tier-2 fuzzy: whitespace / indentation drift is tolerated.
// ---------------------------------------------------------------------------

// TestFuzzy_IndentationDrift_TabVsSpace: the file is tab-indented but the
// model emitted a space-indented SEARCH. Strict match fails; fuzzy succeeds
// and the file's ORIGINAL tab indentation outside the hunk is preserved.
func TestFuzzy_IndentationDrift_TabVsSpace(t *testing.T) {
	file := "func f() {\n\tx := 1\n\treturn x\n}\n"
	// Model emitted spaces instead of tabs in the SEARCH context.
	blocks := []EditBlock{{
		Path:    "f.go",
		Search:  "    x := 1\n    return x\n",
		Replace: "\tx := 2\n\treturn x\n",
	}}

	// Strict must fail (proves the drift is real).
	if _, _, strictOK := ApplyPlanToContent([]byte(file), blocks); strictOK {
		t.Fatalf("strict applier unexpectedly matched space-vs-tab drift")
	}

	out, results, ok := ApplyPlanToContentFuzzy([]byte(file), blocks)
	if !ok {
		t.Fatalf("fuzzy applier failed on tab/space drift: %+v", results)
	}
	want := "func f() {\n\tx := 2\n\treturn x\n}\n"
	if string(out) != want {
		t.Errorf("fuzzy tab/space result wrong:\nwant %q\ngot  %q", want, out)
	}
}

// TestFuzzy_TrailingWhitespace_Tolerated: SEARCH lines carry trailing spaces
// the file does not have. Fuzzy match absorbs it.
func TestFuzzy_TrailingWhitespace_Tolerated(t *testing.T) {
	file := "line one\nline two\nline three\n"
	blocks := []EditBlock{{
		Path:    "t.txt",
		Search:  "line two   \n",
		Replace: "LINE TWO\n",
	}}

	out, _, ok := ApplyPlanToContentFuzzy([]byte(file), blocks)
	if !ok {
		t.Fatalf("fuzzy did not tolerate trailing whitespace")
	}
	if string(out) != "line one\nLINE TWO\nline three\n" {
		t.Errorf("wrong output: %q", out)
	}
}

// TestFuzzy_InternalWhitespaceRun_Collapsed: SEARCH has multiple spaces where
// the file has one (or vice versa). Fuzzy collapses internal runs.
func TestFuzzy_InternalWhitespaceRun_Collapsed(t *testing.T) {
	file := "result = a + b\n"
	blocks := []EditBlock{{
		Path:    "e.go",
		Search:  "result   =    a  +  b\n",
		Replace: "result = a - b\n",
	}}

	out, _, ok := ApplyPlanToContentFuzzy([]byte(file), blocks)
	if !ok {
		t.Fatalf("fuzzy did not collapse internal whitespace runs")
	}
	if string(out) != "result = a - b\n" {
		t.Errorf("wrong output: %q", out)
	}
}

// TestFuzzy_PreservesSurroundingIndentation: a deeply-indented block is
// replaced; the indentation of lines OUTSIDE the matched span is untouched.
func TestFuzzy_PreservesSurroundingIndentation(t *testing.T) {
	file := "package x\n\nfunc outer() {\n\t\tif cond {\n\t\t\tdoThing()\n\t\t}\n}\n"
	// Model dropped the leading indentation entirely in its SEARCH.
	blocks := []EditBlock{{
		Path:    "x.go",
		Search:  "if cond {\ndoThing()\n}\n",
		Replace: "\t\tif cond {\n\t\t\tdoOtherThing()\n\t\t}\n",
	}}

	out, _, ok := ApplyPlanToContentFuzzy([]byte(file), blocks)
	if !ok {
		t.Fatalf("fuzzy failed on de-indented SEARCH")
	}
	want := "package x\n\nfunc outer() {\n\t\tif cond {\n\t\t\tdoOtherThing()\n\t\t}\n}\n"
	if string(out) != want {
		t.Errorf("surrounding indentation disturbed:\nwant %q\ngot  %q", want, out)
	}
}

// ---------------------------------------------------------------------------
// Malformed / non-locating diffs MUST be rejected with ZERO file mutation.
// ---------------------------------------------------------------------------

// TestFuzzy_NonLocatingSearch_Rejected: a SEARCH that matches nothing (even
// fuzzily — the content genuinely differs) yields OutcomeNotFound and no
// mutation.
func TestFuzzy_NonLocatingSearch_Rejected(t *testing.T) {
	original := []byte("the quick brown fox\n")
	blocks := []EditBlock{{
		Path:    "f.txt",
		Search:  "a totally different sentence\n",
		Replace: "should never apply\n",
	}}

	out, results, ok := ApplyPlanToContentFuzzy(original, blocks)
	if ok {
		t.Errorf("non-locating SEARCH must NOT apply")
	}
	if !bytes.Equal(out, original) {
		t.Errorf("file mutated despite non-locating SEARCH:\nwant %q\ngot  %q", original, out)
	}
	if results[0].Outcome != OutcomeNotFound {
		t.Errorf("want OutcomeNotFound, got %q", results[0].Outcome)
	}
	if results[0].Error == "" {
		t.Errorf("expected a non-empty error for the rejected block")
	}
}

// TestFuzzy_ContentDrift_NotSilentlyApplied is the core safety test: fuzzy
// tolerates WHITESPACE drift but NOT content drift. A SEARCH whose code text
// genuinely differs from the file must be rejected — never silently applied
// to the nearest-looking line.
func TestFuzzy_ContentDrift_NotSilentlyApplied(t *testing.T) {
	original := []byte("\tcounter += 1\n")
	// One identifier changed — this is content drift, not whitespace drift.
	blocks := []EditBlock{{
		Path:    "c.go",
		Search:  "\tcounter += 2\n",
		Replace: "\tcounter += 99\n",
	}}

	out, results, ok := ApplyPlanToContentFuzzy(original, blocks)
	if ok {
		t.Fatalf("fuzzy silently applied a content-drifted SEARCH — CORRECTNESS LOSS")
	}
	if !bytes.Equal(out, original) {
		t.Errorf("file corrupted by content-drifted SEARCH:\nwant %q\ngot  %q", original, out)
	}
	if results[0].Outcome != OutcomeNotFound {
		t.Errorf("want OutcomeNotFound for content drift, got %q", results[0].Outcome)
	}
}

// TestFuzzy_AmbiguousFuzzyMatch_Rejected: when tier 2 runs (no exact match
// exists) and the normalised SEARCH locates TWO or more distinct file spans,
// the block is rejected as ambiguous — NOT applied to the first one. Both
// file occurrences use whitespace that differs from the SEARCH, so neither is
// an exact match and tier 2 is forced to run.
func TestFuzzy_AmbiguousFuzzyMatch_Rejected(t *testing.T) {
	// SEARCH "x = 1" has NO exact match; both file lines ("  x  =  1" and
	// "\t x  = 1") normalise to "x = 1" → tier-2 ambiguity.
	original := []byte("a\n  x  =  1\nb\n\t x  = 1\nc\n")
	blocks := []EditBlock{{
		Path:    "a.go",
		Search:  "x = 1\n",
		Replace: "x = 2\n",
	}}

	// Confirm the precondition: no exact match exists, so tier 2 must run.
	if c := bytes.Count(original, []byte("x = 1\n")); c != 0 {
		t.Fatalf("test precondition broken: exact match exists (%d)", c)
	}

	out, results, ok := ApplyPlanToContentFuzzy(original, blocks)
	if ok {
		t.Fatalf("ambiguous fuzzy match must NOT apply")
	}
	if !bytes.Equal(out, original) {
		t.Errorf("file mutated despite ambiguous fuzzy match:\nwant %q\ngot  %q", original, out)
	}
	if results[0].Outcome != OutcomeAmbiguous {
		t.Errorf("want OutcomeAmbiguous, got %q", results[0].Outcome)
	}
}

// TestFuzzy_ExactDuplicate_AmbiguousNotFuzzed: an exact byte-duplicate must
// be reported ambiguous at tier 1 — fuzzy must NOT fall through to tier 2 and
// pick one.
func TestFuzzy_ExactDuplicate_AmbiguousNotFuzzed(t *testing.T) {
	original := []byte("dup\nmiddle\ndup\n")
	blocks := []EditBlock{{Path: "d.txt", Search: "dup\n", Replace: "DUP\n"}}

	out, results, ok := ApplyPlanToContentFuzzy(original, blocks)
	if ok {
		t.Fatalf("exact duplicate must be ambiguous")
	}
	if !bytes.Equal(out, original) {
		t.Errorf("file mutated on ambiguous exact duplicate")
	}
	if results[0].Outcome != OutcomeAmbiguous {
		t.Errorf("want OutcomeAmbiguous, got %q", results[0].Outcome)
	}
}

// TestFuzzy_WhitespaceOnlySearch_Rejected: a SEARCH that normalises to
// nothing (all whitespace) must not match anything.
func TestFuzzy_WhitespaceOnlySearch_Rejected(t *testing.T) {
	original := []byte("real content\n")
	blocks := []EditBlock{{Path: "w.txt", Search: "   \n\t\n", Replace: "x\n"}}

	out, results, ok := ApplyPlanToContentFuzzy(original, blocks)
	if ok {
		t.Fatalf("whitespace-only SEARCH must not match")
	}
	if !bytes.Equal(out, original) {
		t.Errorf("file mutated on whitespace-only SEARCH")
	}
	if results[0].Outcome != OutcomeNotFound {
		t.Errorf("want OutcomeNotFound, got %q", results[0].Outcome)
	}
}

// TestFuzzy_EmptyNeedle_NotFound pins the empty-needle decision.
func TestFuzzy_EmptyNeedle_NotFound(t *testing.T) {
	start, end, outcome := locateFuzzy("hello world", "")
	if outcome != OutcomeNotFound || start != -1 || end != -1 {
		t.Errorf("empty needle: want (-1,-1,not-found), got (%d,%d,%q)", start, end, outcome)
	}
}

// ---------------------------------------------------------------------------
// Parser-level malformed-prompt rejection (the diff format's structural
// guard) — bad markers must never reach the applier.
// ---------------------------------------------------------------------------

// TestFuzzy_MalformedPrompt_RejectedByParser exercises the structural guard:
// a prompt with bad / missing markers is rejected at Parse time, so the
// applier is never given a half-formed plan.
func TestFuzzy_MalformedPrompt_RejectedByParser(t *testing.T) {
	cases := []struct {
		name   string
		prompt string
	}{
		{
			name:   "missing_divider",
			prompt: "f.go\n<<<<<<< SEARCH\nold\n>>>>>>> REPLACE\n",
		},
		{
			name:   "replace_before_search",
			prompt: "f.go\n>>>>>>> REPLACE\nnew\n",
		},
		{
			name:   "stray_divider",
			prompt: "f.go\n=======\nnew\n",
		},
		{
			name:   "empty_search",
			prompt: "f.go\n<<<<<<< SEARCH\n=======\nnew\n>>>>>>> REPLACE\n",
		},
		{
			name:   "unterminated_block",
			prompt: "f.go\n<<<<<<< SEARCH\nold\n=======\nnew\n",
		},
		{
			name:   "missing_path",
			prompt: "<<<<<<< SEARCH\nold\n=======\nnew\n>>>>>>> REPLACE\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			plan, err := Parse(tc.prompt)
			if err == nil {
				t.Fatalf("malformed prompt %q accepted by parser; plan=%+v", tc.name, plan)
			}
			if plan != nil {
				t.Errorf("parser returned a non-nil plan for malformed input")
			}
		})
	}
}

// TestFuzzy_NoMutationOnAnyFailure: across a multi-block plan where one block
// fails (fuzzy or otherwise), allApplied is false so the higher tier discards
// everything. The applier itself must not mutate the input slice.
func TestFuzzy_NoMutationOnAnyFailure(t *testing.T) {
	original := []byte("\talpha\n\tbeta\n")
	input := make([]byte, len(original))
	copy(input, original)

	blocks := []EditBlock{
		{Path: "x.go", Search: "    alpha\n", Replace: "\tALPHA\n"}, // fuzzy hit
		{Path: "x.go", Search: "does not exist\n", Replace: "x\n"},  // fails
	}

	_, results, allApplied := ApplyPlanToContentFuzzy(input, blocks)
	if allApplied {
		t.Errorf("allApplied must be false when any block fails")
	}
	if !bytes.Equal(input, original) {
		t.Errorf("applier mutated the input slice:\nwant %q\ngot  %q", original, input)
	}
	if results[1].Outcome != OutcomeNotFound {
		t.Errorf("block[1] want OutcomeNotFound, got %q", results[1].Outcome)
	}
}

// TestFuzzy_Composable: fuzzy re-search must remain composable — block 2's
// SEARCH only exists after block 1's REPLACE runs. Block 1 fuzzy-matches a
// space-indented SEARCH against the tab-indented file; block 2 then exact-
// matches text block 1 introduced. Block 2's SEARCH reproduces the leading
// tab so the whole line is excised — the result is a single clean line.
func TestFuzzy_Composable(t *testing.T) {
	content := []byte("\toldName()\n")
	blocks := []EditBlock{
		{Path: "c.go", Search: "  oldName()\n", Replace: "\tnewName()\n"},   // fuzzy (space vs tab)
		{Path: "c.go", Search: "\tnewName()\n", Replace: "\tfinalName()\n"}, // exact on block-1 result
	}

	out, results, ok := ApplyPlanToContentFuzzy(content, blocks)
	if !ok {
		t.Fatalf("composable fuzzy blocks did not both apply: %+v", results)
	}
	if string(out) != "\tfinalName()\n" {
		t.Errorf("composability wrong: %q", out)
	}
}

// TestNormaliseLine pins the line-normalisation contract directly.
func TestNormaliseLine(t *testing.T) {
	cases := []struct{ in, want string }{
		{"  hello  ", "hello"},
		{"\tx := 1", "x := 1"},
		{"a   b\t\tc", "a b c"},
		{"   ", ""},
		{"", ""},
		{"no-change", "no-change"},
		{"trailing\t", "trailing"},
	}
	for _, c := range cases {
		if got := normaliseLine(c.in); got != c.want {
			t.Errorf("normaliseLine(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestSplitLinesWithOffsets pins the offset bookkeeping the splice relies on.
func TestSplitLinesWithOffsets(t *testing.T) {
	lines, offsets := splitLinesWithOffsets("ab\ncd\n")
	if len(lines) != 3 || len(offsets) != 3 {
		t.Fatalf("want 3 lines+offsets, got %d/%d", len(lines), len(offsets))
	}
	if lines[0] != "ab" || lines[1] != "cd" || lines[2] != "" {
		t.Errorf("lines wrong: %#v", lines)
	}
	if offsets[0] != 0 || offsets[1] != 3 || offsets[2] != 6 {
		t.Errorf("offsets wrong: %#v", offsets)
	}
}

// TestFuzzy_EditFormatCompliance_Corpus exercises a small corpus of model-style
// diffs and reports the apply-success rate — the anti-bluff compliance metric
// for P3-T02. Every entry is a realistic changed-lines-only diff; the corpus
// mixes exact and whitespace-drifted SEARCH blocks. All MUST apply.
func TestFuzzy_EditFormatCompliance_Corpus(t *testing.T) {
	type corpusEntry struct {
		name      string
		original  string
		search    string
		replace   string
		wantFinal string
	}
	corpus := []corpusEntry{
		{
			name:      "exact-single-line",
			original:  "const limit = 10\n",
			search:    "const limit = 10\n",
			replace:   "const limit = 20\n",
			wantFinal: "const limit = 20\n",
		},
		{
			name:      "tab-drift",
			original:  "func g() {\n\treturn nil\n}\n",
			search:    "    return nil\n",
			replace:   "\treturn errors.New(\"x\")\n",
			wantFinal: "func g() {\n\treturn errors.New(\"x\")\n}\n",
		},
		{
			name:      "trailing-ws-drift",
			original:  "value := compute()\n",
			search:    "value := compute()  \n",
			replace:   "value := computeFast()\n",
			wantFinal: "value := computeFast()\n",
		},
		{
			name:      "multiline-exact",
			original:  "a\nb\nc\nd\n",
			search:    "b\nc\n",
			replace:   "B\nC\n",
			wantFinal: "a\nB\nC\nd\n",
		},
		{
			name:      "internal-ws-drift",
			original:  "x = y + z\n",
			search:    "x  =  y  +  z\n",
			replace:   "x = y - z\n",
			wantFinal: "x = y - z\n",
		},
	}

	applied := 0
	for _, e := range corpus {
		t.Run(e.name, func(t *testing.T) {
			blocks := []EditBlock{{Path: "f", Search: e.search, Replace: e.replace}}
			out, _, ok := ApplyPlanToContentFuzzy([]byte(e.original), blocks)
			if !ok {
				t.Errorf("corpus entry %q did not apply", e.name)
				return
			}
			if string(out) != e.wantFinal {
				t.Errorf("corpus entry %q mismatch:\nwant %q\ngot  %q", e.name, e.wantFinal, out)
				return
			}
			applied++
		})
	}
	rate := float64(applied) / float64(len(corpus)) * 100
	t.Logf("edit-format apply-success rate: %d/%d = %.1f%%", applied, len(corpus), rate)
	if applied != len(corpus) {
		t.Errorf("compliance rate %.1f%% — every corpus diff MUST apply", rate)
	}
}

// sanity: keep strings import used even if a future edit drops a case.
var _ = strings.Count
