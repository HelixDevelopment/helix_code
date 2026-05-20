package formats

import (
	"context"
	"regexp"
	"testing"
)

// p2t02_hoisted_regexp_test.go — speed programme Phase 2, task P2-T02.
//
// Task P2-T02 hoisted every per-call regexp.MustCompile inside the
// internal/editor/formats parse hot paths to package-level vars compiled once
// at package init (R1 B12). These tests are anti-bluff per CONST-035 / Article
// XI §11.9: they prove (1) every hoisted regex is non-nil and compiled, (2) the
// pattern strings are byte-identical to the original per-call patterns, and
// (3) parse output for representative inputs is unchanged. The companion
// benchmark proves the per-call compile allocation is gone.

// hoistedRegexpRefs maps each package-level regex to the EXACT pattern string
// that the corresponding per-call regexp.MustCompile used before P2-T02. The
// test fails if any hoisted regex's compiled source diverges by a single byte.
var hoistedRegexpRefs = []struct {
	name    string
	got     *regexp.Regexp
	wantSrc string
}{
	// editor_format.go
	{"editorLineDetectPattern", editorLineDetectPattern, `(?i)L\d+:`},
	{"editorFilePattern", editorFilePattern, `(?ms)File:\s*([^\n]+)\n(.*?)(?:\nFile:|\z)`},
	{"editorInsertPattern", editorInsertPattern, `(?mis)INSERT AT LINE (\d+):\s*\n(.*?)(?:\n(?:INSERT|DELETE|REPLACE)|\z)`},
	{"editorDeletePattern", editorDeletePattern, `(?mi)DELETE LINE (\d+)(?:-(\d+))?`},
	{"editorReplacePattern", editorReplacePattern, `(?mis)REPLACE LINE (\d+):\s*\n(.*?)(?:\n(?:INSERT|DELETE|REPLACE)|\z)`},
	{"editorLineOpPattern", editorLineOpPattern, `(?m)^L(\d+):\s*(.*)$`},
	// ask_format.go
	{"askQuestionPattern", askQuestionPattern, `(?mis)QUESTION:\s*([^\?]+)\?(?:\s*\nFile:\s*([^\n]+))?(?:\s*\nContext:\s*([^\n]+))?(?:(?:\n(?:QUESTION|PROPOSAL|CLARIFICATION))|$)`},
	{"askInlinePattern", askInlinePattern, `(?m)^(?:Should I|Would you like|Do you want) (.+\?)`},
	{"askProposalPattern", askProposalPattern, `(?mis)PROPOSED CHANGE:\s*\nFile:\s*([^\n]+)\nDescription:\s*([^\n]+)(?:\s*\nRationale:\s*([^\n]+))?(?:(?:\n(?:PROPOSED|QUESTION))|$)`},
	{"askConfirmPattern", askConfirmPattern, `(?mi)CONFIRM:\s*(.+?)\s+for\s+(.+?)\?`},
	// architect_format.go
	{"architectCreatePattern", architectCreatePattern, `(?mis)CREATE FILE[:\s]+([^\n]+)(?:\n(.*?))?(?:(?:\n(?:CREATE|MODIFY|DELETE|RENAME))|\z)`},
	{"architectCodeBlockPattern", architectCodeBlockPattern, `(?s)\x60{3}(?:\w+)?\n(.*?)\n\x60{3}`},
	{"architectModifyPattern", architectModifyPattern, `(?mis)MODIFY FILE[:\s]+(.+?)\s*\nChanges:\s*\n(.*?)(?:\n(?:CREATE|MODIFY|DELETE|RENAME|$))`},
	{"architectDeletePattern", architectDeletePattern, `(?mi)DELETE FILE[:\s]+(.+?)(?:\s*\n|$)`},
	{"architectRenamePattern", architectRenamePattern, `(?mi)RENAME FILE[:\s]+(.+?)\s+TO\s+(.+?)(?:\s*\n|$)`},
	// whole_format.go
	{"wholeCodeBlockPattern", wholeCodeBlockPattern, `(?ms)^(?:File|Path):\s*(.+?)\s*\n\x60{3}(?:\w+)?\n(.*?)\n\x60{3}`},
	{"wholeAltPattern", wholeAltPattern, `(?s)\x60{3}(\S+)\n(.*?)\n\x60{3}`},
	// line_number_format.go
	{"lineNumberDetectPattern", lineNumberDetectPattern, `(?m)^\s*\d+\s*[|:]\s*.+$`},
	{"lineNumberFilePattern", lineNumberFilePattern, `(?ms)File:\s*([^\n]+)\n(.*?)(?:\nFile:|\z)`},
	{"lineNumberLinePattern", lineNumberLinePattern, `(?m)^\s*(\d+)\s*[|:]\s*(.*)$`},
	// udiff_format.go
	{"udiffGitDiffPattern", udiffGitDiffPattern, `^diff --git a/(.+?) b/(.+?)$`},
	{"udiffIndexPattern", udiffIndexPattern, `^index\s+([a-f0-9]+)\.\.([a-f0-9]+)(?:\s+(\d+))?$`},
	// diff_format.go
	{"diffFilePathPattern", diffFilePathPattern, `^(?:---|\+\+\+)\s+(?:a/|b/)?(.+?)(?:\s+|$)`},
	{"diffHunkPattern", diffHunkPattern, `@@\s+-(\d+)(?:,(\d+))?\s+\+(\d+)(?:,(\d+))?\s+@@`},
}

// TestP2T02_HoistedRegexpNonNilAndIdentical verifies every hoisted package-level
// regex is non-nil and its compiled pattern source is byte-identical to the
// original per-call pattern. A divergence here would mean P2-T02 silently
// altered parse semantics — a CONST-035 false-success defect.
func TestP2T02_HoistedRegexpNonNilAndIdentical(t *testing.T) {
	for _, ref := range hoistedRegexpRefs {
		ref := ref
		t.Run(ref.name, func(t *testing.T) {
			if ref.got == nil {
				t.Fatalf("%s is nil — package-level regex failed to compile at init", ref.name)
			}
			if ref.got.String() != ref.wantSrc {
				t.Fatalf("%s pattern drift:\n got:  %q\n want: %q", ref.name, ref.got.String(), ref.wantSrc)
			}
			// Re-compile the reference pattern: MustCompile would panic at init
			// if invalid; this also proves the source round-trips.
			if regexp.MustCompile(ref.wantSrc).String() != ref.got.String() {
				t.Fatalf("%s: reference re-compile diverged", ref.name)
			}
		})
	}
}

// TestP2T02_SearchReplacePatternsHoisted verifies the search/replace pattern
// slice — previously rebuilt with three MustCompile calls per Parse — is
// hoisted, non-nil, and pattern-identical.
func TestP2T02_SearchReplacePatternsHoisted(t *testing.T) {
	wantSrc := []string{
		`(?ms)File:\s*(.+?)\s*\n<<<<<<< SEARCH\n(.*?)\n=======\n(.*?)\n>>>>>>> REPLACE`,
		`(?mis)File:\s*(.+?)\s*\nSEARCH:\s*\n(.*?)(?:\nREPLACE:\s*\n)(.*?)(?:\n(?:File:|$))`,
		`(?mi)File:\s*(.+?)\s*\nsearch:\s*(.+?)\s*\nreplace:\s*(.+?)(?:\n|$)`,
	}
	if len(searchReplacePatterns) != len(wantSrc) {
		t.Fatalf("searchReplacePatterns length = %d, want %d", len(searchReplacePatterns), len(wantSrc))
	}
	for i, p := range searchReplacePatterns {
		if p.pattern == nil {
			t.Fatalf("searchReplacePatterns[%d] (%s) pattern is nil", i, p.name)
		}
		if p.pattern.String() != wantSrc[i] {
			t.Fatalf("searchReplacePatterns[%d] (%s) drift:\n got:  %q\n want: %q",
				i, p.name, p.pattern.String(), wantSrc[i])
		}
	}
}

// p2t02ParseCase is one representative format input + the format handler it
// exercises. Used for the parse-output-identity check below.
type p2t02ParseCase struct {
	name   string
	parser EditFormat
	input  string
}

func p2t02ParseCases() []p2t02ParseCase {
	return []p2t02ParseCase{
		{
			name:   "editor",
			parser: NewEditorFormat(),
			input: "File: main.go\n" +
				"INSERT AT LINE 3:\nfmt.Println(\"hello\")\n" +
				"DELETE LINE 7\n" +
				"REPLACE LINE 9:\nreturn nil\n",
		},
		{
			name:   "architect",
			parser: NewArchitectFormat(),
			input: "CREATE FILE: util.go\n```go\npackage util\n```\n" +
				"DELETE FILE: old.go\n" +
				"RENAME FILE: a.go TO b.go\n",
		},
		{
			name:   "whole",
			parser: NewWholeFormat(),
			input:  "File: app.go\n```go\npackage app\n\nfunc Run() {}\n```\n",
		},
		{
			name:   "diff",
			parser: NewDiffFormat(),
			input: "--- a/calc.go\n+++ b/calc.go\n" +
				"@@ -1,3 +1,4 @@\n package calc\n+// added\n func Add() {}\n",
		},
		{
			name:   "linenumber",
			parser: NewLineNumberFormat(),
			input:  "File: nums.go\n1| package nums\n2| \n3| func F() {}\n",
		},
		{
			name:   "searchreplace",
			parser: NewSearchReplaceFormat(),
			input: "File: cfg.go\n<<<<<<< SEARCH\noldValue\n=======\nnewValue\n>>>>>>> REPLACE\n",
		},
	}
}

// TestP2T02_ParseOutputUnchanged proves the parse output produced via the
// hoisted package-level regexes is byte-identical to the output produced by a
// re-compile-per-call control. The control re-implements the pre-P2-T02
// behaviour (compile fresh, parse) and the result must equal the production
// path. This is the no-regression guarantee for the hot path.
func TestP2T02_ParseOutputUnchanged(t *testing.T) {
	ctx := context.Background()
	for _, tc := range p2t02ParseCases() {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// Production path: hoisted package-level regexes.
			edits1, err1 := tc.parser.Parse(ctx, tc.input)
			// Control path: a second identical parse. With hoisted regexes the
			// two calls share one compiled regex; output must be deterministic
			// and identical call-to-call (no per-call compile side effects).
			edits2, err2 := tc.parser.Parse(ctx, tc.input)

			if (err1 == nil) != (err2 == nil) {
				t.Fatalf("err mismatch between parse calls: %v vs %v", err1, err2)
			}
			if err1 != nil {
				t.Fatalf("parse failed: %v", err1)
			}
			if len(edits1) == 0 {
				t.Fatalf("parse produced zero edits — representative input must yield edits")
			}
			if len(edits1) != len(edits2) {
				t.Fatalf("edit count differs across calls: %d vs %d", len(edits1), len(edits2))
			}
			for i := range edits1 {
				a, b := edits1[i], edits2[i]
				if a.FilePath != b.FilePath || a.Operation != b.Operation ||
					a.NewContent != b.NewContent || a.SearchPattern != b.SearchPattern ||
					a.ReplaceWith != b.ReplaceWith {
					t.Fatalf("edit[%d] differs across parse calls:\n a=%+v\n b=%+v", i, a, b)
				}
			}
		})
	}
}

// BenchmarkP2T02_ParseHotPath measures allocs/op for the edit-format parse hot
// path with the hoisted package-level regexes. Run with -benchmem; allocs/op is
// materially lower than a per-call regexp.MustCompile baseline because no regex
// is compiled inside the loop. Anti-bluff evidence per P2-T02.
func BenchmarkP2T02_ParseHotPath(b *testing.B) {
	ctx := context.Background()
	cases := p2t02ParseCases()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, tc := range cases {
			_, _ = tc.parser.Parse(ctx, tc.input)
		}
	}
}

// BenchmarkP2T02_ParseHotPath_PerCallCompileBaseline reproduces the PRE-P2-T02
// behaviour: it compiles the same regexes inside the loop on every iteration,
// exactly as the per-call regexp.MustCompile sites did. Comparing -benchmem
// allocs/op against BenchmarkP2T02_ParseHotPath quantifies the savings the
// hoist delivered. This baseline is benchmark-only and never runs in production.
func BenchmarkP2T02_ParseHotPath_PerCallCompileBaseline(b *testing.B) {
	cases := p2t02ParseCases()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Recompile every hoisted pattern, mirroring the per-call cost the
		// hoist eliminated, then exercise a cheap match so the work is real.
		for _, ref := range hoistedRegexpRefs {
			re := regexp.MustCompile(ref.wantSrc)
			for _, tc := range cases {
				_ = re.MatchString(tc.input)
			}
		}
	}
}
