package smartedit

import (
	"errors"
	"strings"
	"testing"
)

// TestParse_EmptyPromptReturnsEmptyPlan — empty prompt yields an empty plan
// with no error. The plan's SourceBytes must reflect 0 and PerFile must be a
// non-nil (but empty) map so callers can range over it without nil-checking.
func TestParse_EmptyPromptReturnsEmptyPlan(t *testing.T) {
	plan, err := Parse("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan == nil {
		t.Fatal("plan is nil")
	}
	if len(plan.Blocks) != 0 {
		t.Fatalf("Blocks len = %d, want 0", len(plan.Blocks))
	}
	if plan.PerFile == nil {
		t.Fatal("PerFile is nil; expected initialised empty map")
	}
	if len(plan.PerFile) != 0 {
		t.Fatalf("PerFile len = %d, want 0", len(plan.PerFile))
	}
	if plan.SourceBytes != 0 {
		t.Fatalf("SourceBytes = %d, want 0", plan.SourceBytes)
	}
}

// TestParse_SingleBlock_HappyPath — one block with a path line; the parser
// must produce a single EditBlock with the expected path/search/replace and
// must record the block in PerFile under the same path key.
func TestParse_SingleBlock_HappyPath(t *testing.T) {
	prompt := "src/foo.go\n" +
		"<<<<<<< SEARCH\n" +
		"old line\n" +
		"=======\n" +
		"new line\n" +
		">>>>>>> REPLACE\n"
	plan, err := Parse(prompt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(plan.Blocks) != 1 {
		t.Fatalf("Blocks len = %d, want 1", len(plan.Blocks))
	}
	b := plan.Blocks[0]
	if b.Path != "src/foo.go" {
		t.Errorf("Path = %q, want %q", b.Path, "src/foo.go")
	}
	if b.Search != "old line\n" {
		t.Errorf("Search = %q, want %q", b.Search, "old line\n")
	}
	if b.Replace != "new line\n" {
		t.Errorf("Replace = %q, want %q", b.Replace, "new line\n")
	}
	if got := plan.PerFile["src/foo.go"]; len(got) != 1 {
		t.Errorf("PerFile[src/foo.go] len = %d, want 1", len(got))
	}
}

// TestParse_MultipleBlocksOnDifferentFiles — two blocks each with their own
// path; the parser must group them in PerFile under their respective keys
// while preserving source order in Blocks.
func TestParse_MultipleBlocksOnDifferentFiles(t *testing.T) {
	prompt := "a.go\n" +
		"<<<<<<< SEARCH\n" +
		"alpha\n" +
		"=======\n" +
		"ALPHA\n" +
		">>>>>>> REPLACE\n" +
		"\n" +
		"b.go\n" +
		"<<<<<<< SEARCH\n" +
		"beta\n" +
		"=======\n" +
		"BETA\n" +
		">>>>>>> REPLACE\n"
	plan, err := Parse(prompt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(plan.Blocks) != 2 {
		t.Fatalf("Blocks len = %d, want 2", len(plan.Blocks))
	}
	if plan.Blocks[0].Path != "a.go" || plan.Blocks[1].Path != "b.go" {
		t.Errorf("paths = [%q, %q], want [a.go, b.go]", plan.Blocks[0].Path, plan.Blocks[1].Path)
	}
	if len(plan.PerFile["a.go"]) != 1 || len(plan.PerFile["b.go"]) != 1 {
		t.Errorf("PerFile grouping wrong: a.go=%d b.go=%d", len(plan.PerFile["a.go"]), len(plan.PerFile["b.go"]))
	}
}

// TestParse_PathStickiness_SecondBlockInheritsPath — when a second block
// follows a first block without its own path line, the path from the first
// block sticks. Both blocks target the same file.
func TestParse_PathStickiness_SecondBlockInheritsPath(t *testing.T) {
	prompt := "a/b/c.go\n" +
		"<<<<<<< SEARCH\n" +
		"x\n" +
		"=======\n" +
		"y\n" +
		">>>>>>> REPLACE\n" +
		"\n" +
		"<<<<<<< SEARCH\n" +
		"p\n" +
		"=======\n" +
		"q\n" +
		">>>>>>> REPLACE\n"
	plan, err := Parse(prompt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(plan.Blocks) != 2 {
		t.Fatalf("Blocks len = %d, want 2", len(plan.Blocks))
	}
	if plan.Blocks[0].Path != "a/b/c.go" || plan.Blocks[1].Path != "a/b/c.go" {
		t.Errorf("paths = [%q, %q], want both a/b/c.go", plan.Blocks[0].Path, plan.Blocks[1].Path)
	}
	if len(plan.PerFile["a/b/c.go"]) != 2 {
		t.Errorf("PerFile[a/b/c.go] len = %d, want 2", len(plan.PerFile["a/b/c.go"]))
	}
}

// TestParse_PathStickyResetsOnNewPath — block1 path A; block2 path B (a new
// path line appears between them). Each block must target its declared path.
func TestParse_PathStickyResetsOnNewPath(t *testing.T) {
	prompt := "A.go\n" +
		"<<<<<<< SEARCH\n" +
		"a\n" +
		"=======\n" +
		"AA\n" +
		">>>>>>> REPLACE\n" +
		"B.go\n" +
		"<<<<<<< SEARCH\n" +
		"b\n" +
		"=======\n" +
		"BB\n" +
		">>>>>>> REPLACE\n"
	plan, err := Parse(prompt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(plan.Blocks) != 2 {
		t.Fatalf("Blocks len = %d, want 2", len(plan.Blocks))
	}
	if plan.Blocks[0].Path != "A.go" {
		t.Errorf("block[0].Path = %q, want A.go", plan.Blocks[0].Path)
	}
	if plan.Blocks[1].Path != "B.go" {
		t.Errorf("block[1].Path = %q, want B.go", plan.Blocks[1].Path)
	}
}

// TestParse_FirstBlockWithoutPath_Errors — a SEARCH marker arrives before
// any path line has been declared. The parser must return ErrPathRequired.
func TestParse_FirstBlockWithoutPath_Errors(t *testing.T) {
	prompt := "<<<<<<< SEARCH\n" +
		"orphan\n" +
		"=======\n" +
		"replacement\n" +
		">>>>>>> REPLACE\n"
	plan, err := Parse(prompt)
	if !errors.Is(err, ErrPathRequired) {
		t.Fatalf("err = %v, want ErrPathRequired", err)
	}
	if plan != nil {
		t.Errorf("plan = %#v, want nil on error", plan)
	}
}

// TestParse_LineNumbersTracked — LineStart of the first block must equal the
// 1-indexed line number of its `<<<<<<< SEARCH` marker, and LineEnd must
// equal the 1-indexed line number of its `>>>>>>> REPLACE` marker.
func TestParse_LineNumbersTracked(t *testing.T) {
	// Line 1: pre-comment (BLANK)
	// Line 2: pre-comment (text)
	// Line 3: path
	// Line 4: SEARCH marker
	// Line 5: search content
	// Line 6: divider
	// Line 7: replace content
	// Line 8: REPLACE marker
	prompt := "\n" +
		"# leading comment line is ignored as it is not adjacent to SEARCH\n" +
		"path/to/file.go\n" +
		"<<<<<<< SEARCH\n" +
		"hello\n" +
		"=======\n" +
		"world\n" +
		">>>>>>> REPLACE\n"
	plan, err := Parse(prompt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(plan.Blocks) != 1 {
		t.Fatalf("Blocks len = %d, want 1", len(plan.Blocks))
	}
	if plan.Blocks[0].LineStart != 4 {
		t.Errorf("LineStart = %d, want 4", plan.Blocks[0].LineStart)
	}
	if plan.Blocks[0].LineEnd != 8 {
		t.Errorf("LineEnd = %d, want 8", plan.Blocks[0].LineEnd)
	}
}

// TestParse_TooManyBlocks — generate MaxBlocksPerPrompt + 1 blocks; the
// parser must return ErrTooManyBlocks.
func TestParse_TooManyBlocks(t *testing.T) {
	var b strings.Builder
	b.WriteString("file.go\n")
	for i := 0; i < MaxBlocksPerPrompt+1; i++ {
		b.WriteString("<<<<<<< SEARCH\n")
		b.WriteString("s\n")
		b.WriteString("=======\n")
		b.WriteString("r\n")
		b.WriteString(">>>>>>> REPLACE\n")
	}
	plan, err := Parse(b.String())
	if !errors.Is(err, ErrTooManyBlocks) {
		t.Fatalf("err = %v, want ErrTooManyBlocks", err)
	}
	if plan != nil {
		t.Errorf("plan = %#v, want nil on error", plan)
	}
}

// TestParse_PromptTooLarge — prompts larger than MaxPromptBytes must be
// rejected with ErrPromptTooLarge before any line-based parsing happens.
func TestParse_PromptTooLarge(t *testing.T) {
	huge := strings.Repeat("a", MaxPromptBytes+1)
	plan, err := Parse(huge)
	if !errors.Is(err, ErrPromptTooLarge) {
		t.Fatalf("err = %v, want ErrPromptTooLarge", err)
	}
	if plan != nil {
		t.Errorf("plan = %#v, want nil on error", plan)
	}
}

// TestParse_SearchTooLarge — a single SEARCH section larger than
// MaxSearchBytes must trigger ErrSearchTooLarge.
func TestParse_SearchTooLarge(t *testing.T) {
	big := strings.Repeat("x", MaxSearchBytes+1)
	prompt := "f.go\n" +
		"<<<<<<< SEARCH\n" +
		big + "\n" +
		"=======\n" +
		"y\n" +
		">>>>>>> REPLACE\n"
	plan, err := Parse(prompt)
	if !errors.Is(err, ErrSearchTooLarge) {
		t.Fatalf("err = %v, want ErrSearchTooLarge", err)
	}
	if plan != nil {
		t.Errorf("plan = %#v, want nil on error", plan)
	}
}

// TestParse_ReplaceTooLarge — a single REPLACE section larger than
// MaxReplaceBytes must trigger ErrReplaceTooLarge.
func TestParse_ReplaceTooLarge(t *testing.T) {
	big := strings.Repeat("x", MaxReplaceBytes+1)
	prompt := "f.go\n" +
		"<<<<<<< SEARCH\n" +
		"y\n" +
		"=======\n" +
		big + "\n" +
		">>>>>>> REPLACE\n"
	plan, err := Parse(prompt)
	if !errors.Is(err, ErrReplaceTooLarge) {
		t.Fatalf("err = %v, want ErrReplaceTooLarge", err)
	}
	if plan != nil {
		t.Errorf("plan = %#v, want nil on error", plan)
	}
}

// TestParse_EmptySearchErrors — an empty SEARCH section (divider immediately
// after the SEARCH marker) must be rejected with ErrSearchEmpty.
func TestParse_EmptySearchErrors(t *testing.T) {
	prompt := "f.go\n" +
		"<<<<<<< SEARCH\n" +
		"=======\n" +
		"new\n" +
		">>>>>>> REPLACE\n"
	plan, err := Parse(prompt)
	if !errors.Is(err, ErrSearchEmpty) {
		t.Fatalf("err = %v, want ErrSearchEmpty", err)
	}
	if plan != nil {
		t.Errorf("plan = %#v, want nil on error", plan)
	}
}

// TestParse_MissingDivider — REPLACE marker arrives without an intervening
// `=======` divider. Must error as invalid block structure.
func TestParse_MissingDivider(t *testing.T) {
	prompt := "f.go\n" +
		"<<<<<<< SEARCH\n" +
		"old\n" +
		">>>>>>> REPLACE\n"
	plan, err := Parse(prompt)
	if !errors.Is(err, ErrInvalidBlockStructure) {
		t.Fatalf("err = %v, want ErrInvalidBlockStructure", err)
	}
	if plan != nil {
		t.Errorf("plan = %#v, want nil on error", plan)
	}
}

// TestParse_DanglingMarker — SEARCH marker with no closing REPLACE marker
// before EOF must error as invalid block structure.
func TestParse_DanglingMarker(t *testing.T) {
	prompt := "f.go\n" +
		"<<<<<<< SEARCH\n" +
		"old\n" +
		"=======\n" +
		"new\n"
	plan, err := Parse(prompt)
	if !errors.Is(err, ErrInvalidBlockStructure) {
		t.Fatalf("err = %v, want ErrInvalidBlockStructure", err)
	}
	if plan != nil {
		t.Errorf("plan = %#v, want nil on error", plan)
	}
}

// TestParse_DuplicateSEARCH — two SEARCH markers without an intervening
// REPLACE marker must error as invalid block structure (a SEARCH inside the
// REPLACE section without the indented-marker exemption is malformed).
func TestParse_DuplicateSEARCH(t *testing.T) {
	prompt := "f.go\n" +
		"<<<<<<< SEARCH\n" +
		"old\n" +
		"<<<<<<< SEARCH\n" +
		"=======\n" +
		"new\n" +
		">>>>>>> REPLACE\n"
	plan, err := Parse(prompt)
	if !errors.Is(err, ErrInvalidBlockStructure) {
		t.Fatalf("err = %v, want ErrInvalidBlockStructure", err)
	}
	if plan != nil {
		t.Errorf("plan = %#v, want nil on error", plan)
	}
}

// TestParse_IndentedMarkerNotTreatedAsMarker — a marker triplet inside a
// SEARCH section is allowed PROVIDED every nested marker line has at least
// one leading whitespace byte. The parser MUST treat indented markers as
// content. This is the load-bearing edge-case anchor for column-0 strictness.
func TestParse_IndentedMarkerNotTreatedAsMarker(t *testing.T) {
	prompt := "a.go\n" +
		"<<<<<<< SEARCH\n" +
		"  <<<<<<< SEARCH\n" +
		"=======\n" +
		"  >>>>>>> REPLACE\n" +
		">>>>>>> REPLACE\n"
	plan, err := Parse(prompt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(plan.Blocks) != 1 {
		t.Fatalf("Blocks len = %d, want 1", len(plan.Blocks))
	}
	wantSearch := "  <<<<<<< SEARCH\n"
	wantReplace := "  >>>>>>> REPLACE\n"
	if plan.Blocks[0].Search != wantSearch {
		t.Errorf("Search = %q, want %q", plan.Blocks[0].Search, wantSearch)
	}
	if plan.Blocks[0].Replace != wantReplace {
		t.Errorf("Replace = %q, want %q", plan.Blocks[0].Replace, wantReplace)
	}
}

// TestParse_TrailingWhitespaceOnMarkerOK — a marker line with trailing
// spaces / tabs is still recognised as a marker after TrimRight.
func TestParse_TrailingWhitespaceOnMarkerOK(t *testing.T) {
	prompt := "f.go\n" +
		"<<<<<<< SEARCH \t\n" +
		"old\n" +
		"======= \t\n" +
		"new\n" +
		">>>>>>> REPLACE \t\n"
	plan, err := Parse(prompt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(plan.Blocks) != 1 {
		t.Fatalf("Blocks len = %d, want 1", len(plan.Blocks))
	}
	if plan.Blocks[0].Search != "old\n" || plan.Blocks[0].Replace != "new\n" {
		t.Errorf("block = %+v", plan.Blocks[0])
	}
}

// TestParse_BlankLineBetweenBlocks_OK — blank lines between blocks must be
// tolerated and not interpreted as path lines.
func TestParse_BlankLineBetweenBlocks_OK(t *testing.T) {
	prompt := "x.go\n" +
		"<<<<<<< SEARCH\n" +
		"a\n" +
		"=======\n" +
		"A\n" +
		">>>>>>> REPLACE\n" +
		"\n" +
		"\n" +
		"<<<<<<< SEARCH\n" +
		"b\n" +
		"=======\n" +
		"B\n" +
		">>>>>>> REPLACE\n"
	plan, err := Parse(prompt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(plan.Blocks) != 2 {
		t.Fatalf("Blocks len = %d, want 2", len(plan.Blocks))
	}
	if plan.Blocks[0].Path != "x.go" || plan.Blocks[1].Path != "x.go" {
		t.Errorf("paths = [%q, %q], want [x.go, x.go]", plan.Blocks[0].Path, plan.Blocks[1].Path)
	}
}

// TestParse_PathLineWithLeadingDot_OK — a relative path beginning with `./`
// must be accepted verbatim.
func TestParse_PathLineWithLeadingDot_OK(t *testing.T) {
	prompt := "./file.go\n" +
		"<<<<<<< SEARCH\n" +
		"a\n" +
		"=======\n" +
		"b\n" +
		">>>>>>> REPLACE\n"
	plan, err := Parse(prompt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(plan.Blocks) != 1 {
		t.Fatalf("Blocks len = %d, want 1", len(plan.Blocks))
	}
	if plan.Blocks[0].Path != "./file.go" {
		t.Errorf("Path = %q, want ./file.go", plan.Blocks[0].Path)
	}
}

// TestParse_PathLineWithSlash_OK — a relative path with subdirectories
// must be accepted verbatim.
func TestParse_PathLineWithSlash_OK(t *testing.T) {
	prompt := "subdir/file.go\n" +
		"<<<<<<< SEARCH\n" +
		"a\n" +
		"=======\n" +
		"b\n" +
		">>>>>>> REPLACE\n"
	plan, err := Parse(prompt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan.Blocks[0].Path != "subdir/file.go" {
		t.Errorf("Path = %q, want subdir/file.go", plan.Blocks[0].Path)
	}
}

// TestParse_BlockGroupingByFile — three blocks targeting two distinct files
// (A, B, A in source order). PerFile must group them by file while Blocks
// preserves source order.
func TestParse_BlockGroupingByFile(t *testing.T) {
	prompt := "A.go\n" +
		"<<<<<<< SEARCH\n" +
		"a1\n" +
		"=======\n" +
		"A1\n" +
		">>>>>>> REPLACE\n" +
		"B.go\n" +
		"<<<<<<< SEARCH\n" +
		"b1\n" +
		"=======\n" +
		"B1\n" +
		">>>>>>> REPLACE\n" +
		"A.go\n" +
		"<<<<<<< SEARCH\n" +
		"a2\n" +
		"=======\n" +
		"A2\n" +
		">>>>>>> REPLACE\n"
	plan, err := Parse(prompt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(plan.Blocks) != 3 {
		t.Fatalf("Blocks len = %d, want 3", len(plan.Blocks))
	}
	if got := plan.PerFile["A.go"]; len(got) != 2 {
		t.Errorf("PerFile[A.go] len = %d, want 2", len(got))
	}
	if got := plan.PerFile["B.go"]; len(got) != 1 {
		t.Errorf("PerFile[B.go] len = %d, want 1", len(got))
	}
	// Source-order preservation in Blocks.
	if plan.Blocks[0].Path != "A.go" ||
		plan.Blocks[1].Path != "B.go" ||
		plan.Blocks[2].Path != "A.go" {
		t.Errorf("source order broken: %q %q %q",
			plan.Blocks[0].Path, plan.Blocks[1].Path, plan.Blocks[2].Path)
	}
}

// TestParse_SourceBytesField — the parser must record len(prompt) verbatim
// in plan.SourceBytes for the applier's defence-in-depth size check.
func TestParse_SourceBytesField(t *testing.T) {
	prompt := "f.go\n" +
		"<<<<<<< SEARCH\n" +
		"x\n" +
		"=======\n" +
		"y\n" +
		">>>>>>> REPLACE\n"
	plan, err := Parse(prompt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan.SourceBytes != len(prompt) {
		t.Errorf("SourceBytes = %d, want %d", plan.SourceBytes, len(prompt))
	}
}
