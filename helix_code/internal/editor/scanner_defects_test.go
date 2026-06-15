package editor

// scanner_defects_test.go — RED→GREEN regression guards for two scanner-based
// editor defects (DEFECT-A: bufio.Scanner default 64KB token cap; DEFECT-B:
// trailing-newline state dropped on read → silent newline-addition on write).
//
// §11.4.115 polarity switch: set RED_MODE=1 in the environment to run the tests
// in REPRODUCE mode against a PRE-FIX artifact — they then ASSERT the defect is
// PRESENT (a guard against authoring a synthetic RED that merely agrees with the
// fix). With RED_MODE unset / 0 (default) the SAME sources are the standing
// GREEN regression guards asserting the defects are ABSENT.
//
// §11.4.135: these are permanent regression guards registered in the same change
// as the fix.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// redMode reports whether the polarity switch is engaged (reproduce-the-defect).
func redMode() bool {
	v := os.Getenv("RED_MODE")
	return v == "1" || strings.EqualFold(v, "true")
}

// bigLine returns a single line whose length exceeds bufio.Scanner's default
// 64KiB (65536) token limit, so a default-configured scanner fails on it.
func bigLine() string {
	// 100 KiB of 'a' — comfortably over the 64 KiB default cap.
	return strings.Repeat("a", 100*1024)
}

// writeTemp writes content to a fresh temp file and returns its path.
func writeTemp(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "subject.txt")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("setup: write temp file: %v", err)
	}
	return p
}

// ---------------------------------------------------------------------------
// DEFECT-A — large single line ("token too long") on scanner-read editors.
// ---------------------------------------------------------------------------

// TestDefectA_LineEditor_BigLine exercises LineEditor.Apply on a file whose
// first line is >64KiB. Pre-fix: readFile's default scanner returns
// "bufio.Scanner: token too long" and Apply fails. Post-fix: the edit succeeds
// and the (untouched) big line is preserved byte-exact.
func TestDefectA_LineEditor_BigLine(t *testing.T) {
	big := bigLine()
	// Two lines: a huge first line, a small second line we will replace.
	original := big + "\nsecond\n"
	p := writeTemp(t, original)

	le := NewLineEditor()
	err := le.Apply(Edit{
		FilePath: p,
		Format:   EditFormatLines,
		Content:  []LineEdit{{StartLine: 2, EndLine: 2, NewContent: "SECOND"}},
	})

	if redMode() {
		// REPRODUCE: the defect must be present on the pre-fix artifact.
		if err == nil {
			t.Fatal("RED_MODE: expected token-too-long failure on big line, got nil — defect not reproduced")
		}
		if !strings.Contains(err.Error(), "token too long") {
			t.Fatalf("RED_MODE: expected 'token too long' error, got: %v", err)
		}
		return
	}

	// GREEN guard: edit must succeed and preserve the big line byte-exact.
	if err != nil {
		t.Fatalf("LineEditor.Apply on big-line file failed: %v", err)
	}
	got, rerr := os.ReadFile(p)
	if rerr != nil {
		t.Fatalf("read back: %v", rerr)
	}
	want := big + "\nSECOND\n"
	if string(got) != want {
		t.Fatalf("big-line round-trip mismatch:\n  got len=%d (first line preserved=%v)\n  want len=%d",
			len(got), strings.HasPrefix(string(got), big), len(want))
	}
}

// TestDefectA_SearchReplaceEditor_ApplyToLines_BigLine exercises the
// scanner-read ApplyToLines path (NOT the whole-file Apply path).
func TestDefectA_SearchReplaceEditor_ApplyToLines_BigLine(t *testing.T) {
	big := bigLine()
	original := big + "\nfoo\n"
	p := writeTemp(t, original)

	sre := NewSearchReplaceEditor()
	err := sre.ApplyToLines(Edit{
		FilePath: p,
		Format:   EditFormatSearchReplace,
		Content:  []SearchReplace{{Search: "foo", Replace: "bar", Count: -1}},
	})

	if redMode() {
		if err == nil {
			t.Fatal("RED_MODE: expected token-too-long failure on big line, got nil — defect not reproduced")
		}
		if !strings.Contains(err.Error(), "token too long") {
			t.Fatalf("RED_MODE: expected 'token too long' error, got: %v", err)
		}
		return
	}

	if err != nil {
		t.Fatalf("SearchReplaceEditor.ApplyToLines on big-line file failed: %v", err)
	}
	got, rerr := os.ReadFile(p)
	if rerr != nil {
		t.Fatalf("read back: %v", rerr)
	}
	want := big + "\nbar\n"
	if string(got) != want {
		t.Fatalf("big-line round-trip mismatch: got len=%d want len=%d (first line preserved=%v)",
			len(got), len(want), strings.HasPrefix(string(got), big))
	}
}

// TestDefectA_DiffEditor_BigLine exercises DiffEditor.Apply (scanner readFile)
// on a context line >64KiB.
func TestDefectA_DiffEditor_BigLine(t *testing.T) {
	big := bigLine()
	original := big + "\nold\n"
	p := writeTemp(t, original)

	// Diff: keep the big context line, replace "old" with "new".
	// No trailing "\n" — see DiffEditor empty-context-line note below.
	diff := "@@ -1,2 +1,2 @@\n " + big + "\n-old\n+new"

	de := NewDiffEditor()
	err := de.Apply(Edit{
		FilePath: p,
		Format:   EditFormatDiff,
		Content:  diff,
	})

	if redMode() {
		if err == nil {
			t.Fatal("RED_MODE: expected token-too-long failure on big line, got nil — defect not reproduced")
		}
		if !strings.Contains(err.Error(), "token too long") {
			t.Fatalf("RED_MODE: expected 'token too long' error, got: %v", err)
		}
		return
	}

	if err != nil {
		t.Fatalf("DiffEditor.Apply on big-line file failed: %v", err)
	}
	got, rerr := os.ReadFile(p)
	if rerr != nil {
		t.Fatalf("read back: %v", rerr)
	}
	want := big + "\nnew\n"
	if string(got) != want {
		t.Fatalf("big-line diff round-trip mismatch: got len=%d want len=%d (first line preserved=%v)",
			len(got), len(want), strings.HasPrefix(string(got), big))
	}
}

// ---------------------------------------------------------------------------
// DEFECT-B — trailing-newline state corruption on scanner-read editors.
// A file ending WITHOUT a newline must NOT gain one; a file ending WITH one
// must keep exactly one.
// ---------------------------------------------------------------------------

// TestDefectB_LineEditor_NoTrailingNewline_Preserved edits a file that does not
// end with a newline and asserts the edit does not introduce one.
func TestDefectB_LineEditor_NoTrailingNewline_Preserved(t *testing.T) {
	original := "alpha\nbeta" // no trailing newline
	p := writeTemp(t, original)

	le := NewLineEditor()
	if err := le.Apply(Edit{
		FilePath: p,
		Format:   EditFormatLines,
		Content:  []LineEdit{{StartLine: 1, EndLine: 1, NewContent: "ALPHA"}},
	}); err != nil {
		t.Fatalf("LineEditor.Apply failed: %v", err)
	}

	got, rerr := os.ReadFile(p)
	if rerr != nil {
		t.Fatalf("read back: %v", rerr)
	}
	want := "ALPHA\nbeta" // still no trailing newline

	if redMode() {
		// REPRODUCE: pre-fix the file gains a trailing newline.
		if string(got) == want {
			t.Fatal("RED_MODE: expected trailing-newline corruption, but output was correct — defect not reproduced")
		}
		if string(got) != want+"\n" {
			t.Fatalf("RED_MODE: expected corrupted output %q, got %q", want+"\n", string(got))
		}
		return
	}

	// GREEN guard: byte-exact, no added newline.
	if string(got) != want {
		t.Fatalf("trailing-newline NOT preserved: got %q, want %q", string(got), want)
	}
}

// TestDefectB_LineEditor_WithTrailingNewline_KeepsExactlyOne edits a file that
// DOES end with a single newline and asserts exactly one remains.
func TestDefectB_LineEditor_WithTrailingNewline_KeepsExactlyOne(t *testing.T) {
	original := "alpha\nbeta\n" // exactly one trailing newline
	p := writeTemp(t, original)

	le := NewLineEditor()
	if err := le.Apply(Edit{
		FilePath: p,
		Format:   EditFormatLines,
		Content:  []LineEdit{{StartLine: 1, EndLine: 1, NewContent: "ALPHA"}},
	}); err != nil {
		t.Fatalf("LineEditor.Apply failed: %v", err)
	}

	got, rerr := os.ReadFile(p)
	if rerr != nil {
		t.Fatalf("read back: %v", rerr)
	}
	want := "ALPHA\nbeta\n"
	// This invariant holds both pre- and post-fix; it is a GREEN-only guard
	// (no RED branch) that protects against a fix that drops the newline.
	if string(got) != want {
		t.Fatalf("single trailing newline NOT preserved: got %q, want %q", string(got), want)
	}
}

// TestDefectB_SearchReplaceEditor_ApplyToLines_NoTrailingNewline_Preserved
// covers the scanner-read ApplyToLines path.
func TestDefectB_SearchReplaceEditor_ApplyToLines_NoTrailingNewline_Preserved(t *testing.T) {
	original := "alpha\nbeta" // no trailing newline
	p := writeTemp(t, original)

	sre := NewSearchReplaceEditor()
	if err := sre.ApplyToLines(Edit{
		FilePath: p,
		Format:   EditFormatSearchReplace,
		Content:  []SearchReplace{{Search: "alpha", Replace: "ALPHA", Count: -1}},
	}); err != nil {
		t.Fatalf("SearchReplaceEditor.ApplyToLines failed: %v", err)
	}

	got, rerr := os.ReadFile(p)
	if rerr != nil {
		t.Fatalf("read back: %v", rerr)
	}
	want := "ALPHA\nbeta"

	if redMode() {
		if string(got) == want {
			t.Fatal("RED_MODE: expected trailing-newline corruption, but output was correct — defect not reproduced")
		}
		if string(got) != want+"\n" {
			t.Fatalf("RED_MODE: expected corrupted output %q, got %q", want+"\n", string(got))
		}
		return
	}

	if string(got) != want {
		t.Fatalf("trailing-newline NOT preserved: got %q, want %q", string(got), want)
	}
}

// TestDefectB_DiffEditor_NoTrailingNewline_Preserved covers DiffEditor.
func TestDefectB_DiffEditor_NoTrailingNewline_Preserved(t *testing.T) {
	original := "alpha\nbeta" // no trailing newline
	p := writeTemp(t, original)

	// No trailing newline in the diff string itself: a trailing "\n" would
	// split into a spurious empty context line that DiffEditor reads as an
	// extra expected line at EOF.
	diff := "@@ -1,2 +1,2 @@\n-alpha\n+ALPHA\n beta"

	de := NewDiffEditor()
	if err := de.Apply(Edit{
		FilePath: p,
		Format:   EditFormatDiff,
		Content:  diff,
	}); err != nil {
		t.Fatalf("DiffEditor.Apply failed: %v", err)
	}

	got, rerr := os.ReadFile(p)
	if rerr != nil {
		t.Fatalf("read back: %v", rerr)
	}
	want := "ALPHA\nbeta"

	if redMode() {
		if string(got) == want {
			t.Fatal("RED_MODE: expected trailing-newline corruption, but output was correct — defect not reproduced")
		}
		if string(got) != want+"\n" {
			t.Fatalf("RED_MODE: expected corrupted output %q, got %q", want+"\n", string(got))
		}
		return
	}

	if string(got) != want {
		t.Fatalf("trailing-newline NOT preserved: got %q, want %q", string(got), want)
	}
}

// TestDefectB_DiffEditor_WithTrailingNewline_KeepsExactlyOne guards against a
// fix that drops the newline on already-newline-terminated files.
func TestDefectB_DiffEditor_WithTrailingNewline_KeepsExactlyOne(t *testing.T) {
	original := "alpha\nbeta\n"
	p := writeTemp(t, original)

	diff := "@@ -1,2 +1,2 @@\n-alpha\n+ALPHA\n beta"

	de := NewDiffEditor()
	if err := de.Apply(Edit{
		FilePath: p,
		Format:   EditFormatDiff,
		Content:  diff,
	}); err != nil {
		t.Fatalf("DiffEditor.Apply failed: %v", err)
	}

	got, rerr := os.ReadFile(p)
	if rerr != nil {
		t.Fatalf("read back: %v", rerr)
	}
	want := "ALPHA\nbeta\n"
	if string(got) != want {
		t.Fatalf("single trailing newline NOT preserved: got %q, want %q", string(got), want)
	}
}
