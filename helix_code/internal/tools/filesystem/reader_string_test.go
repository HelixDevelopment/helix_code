package filesystem

import (
	"strings"
	"testing"
)

// TestFileContent_String_RendersText is the RED→GREEN guard for the agent
// tool-loop bug where fs_read's *FileContent reached the model as a decimal
// byte array (e.g. "&{/path [35 32 65 ...]}") instead of the file text,
// because the value was rendered with fmt.Sprintf("%v", v) and Content is a
// []byte. The fix gives *FileContent a String() method (fmt.Stringer) that
// renders the readable file text. RED on the pre-fix artifact: no String()
// method ⇒ this test does not compile / the byte-array assertion holds.
func TestFileContent_String_RendersText(t *testing.T) {
	fc := &FileContent{
		Path:    "/x/AGENTS.md",
		Content: []byte("# AGENTS.md\nhello world"),
	}
	got := fc.String()

	if !strings.Contains(got, "# AGENTS.md") {
		t.Fatalf("String() must contain the file text header line; got:\n%q", got)
	}
	if !strings.Contains(got, "hello world") {
		t.Fatalf("String() must contain the file body text; got:\n%q", got)
	}
	// The defect signature: a decimal byte array. "# " is bytes 35 32.
	if strings.Contains(got, "[35 32") {
		t.Fatalf("String() must NOT render Content as a decimal byte array; got:\n%q", got)
	}
	if !strings.Contains(got, "/x/AGENTS.md") {
		t.Fatalf("String() should name the file path; got:\n%q", got)
	}
}

// TestFileContent_String_PartialHeader verifies the partial-read header form.
func TestFileContent_String_PartialHeader(t *testing.T) {
	fc := &FileContent{
		Path:       "/x/big.go",
		Content:    []byte("line10\nline11\nline12"),
		IsPartial:  true,
		StartLine:  10,
		EndLine:    12,
		TotalLines: 100,
	}
	got := fc.String()
	for _, want := range []string{"/x/big.go", "10", "12", "100", "line10", "line12"} {
		if !strings.Contains(got, want) {
			t.Fatalf("partial String() must contain %q; got:\n%q", want, got)
		}
	}
	if strings.Contains(got, "[108 105 110 101") { // "line" as bytes
		t.Fatalf("partial String() must NOT render Content as a byte array; got:\n%q", got)
	}
}
