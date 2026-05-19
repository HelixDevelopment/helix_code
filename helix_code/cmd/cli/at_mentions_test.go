package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAtMentionTokens(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"none", "What does this code do?", nil},
		{"single", "explain @README.md please", []string{"@README.md"}},
		{"at-start", "@cmd/cli/main.go is the entry point", []string{"@cmd/cli/main.go"}},
		{"at-end-with-period", "look at @docs/CONTINUATION.md.", []string{"@docs/CONTINUATION.md"}},
		{"with-comma", "compare @a.go, @b.go", []string{"@a.go", "@b.go"}},
		{"email-not-mention", "drop us a note at user@example.com", nil},
		{"struct-tag-not-mention", "json:\"foo,omitempty\" with @path/x.go", []string{"@path/x.go"}},
		{"too-short", "@ @a", nil},
		{"trailing-quote", "see (@docs/foo.md) for the spec", []string{"@docs/foo.md"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := atMentionTokens(tt.in)
			if len(got) != len(tt.want) {
				t.Fatalf("atMentionTokens(%q) = %v, want %v", tt.in, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("atMentionTokens(%q)[%d] = %q, want %q", tt.in, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestExpandAtMentions_AttachesRealFiles(t *testing.T) {
	dir := t.TempDir()
	// Create two real files; both should be embedded.
	a := filepath.Join(dir, "a.txt")
	b := filepath.Join(dir, "sub", "b.go")
	if err := os.WriteFile(a, []byte("alpha"), 0o644); err != nil {
		t.Fatalf("write a: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(b), 0o755); err != nil {
		t.Fatalf("mkdir sub: %v", err)
	}
	if err := os.WriteFile(b, []byte("package main\nfunc Beta() {}"), 0o644); err != nil {
		t.Fatalf("write b: %v", err)
	}

	// chdir so the relative @ paths in the prompt resolve.
	old, _ := os.Getwd()
	defer os.Chdir(old)
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	prompt := "explain @a.txt and @sub/b.go please"
	attached := expandAtMentions(&prompt)
	if len(attached) != 2 {
		t.Fatalf("attached %v, want 2 paths", attached)
	}
	if !strings.Contains(prompt, "<attached_files>") {
		t.Fatalf("prompt missing attached_files block: %q", prompt)
	}
	if !strings.Contains(prompt, "alpha") {
		t.Errorf("prompt missing a.txt content")
	}
	if !strings.Contains(prompt, "func Beta() {}") {
		t.Errorf("prompt missing b.go content")
	}
}

func TestExpandAtMentions_MissingFileStaysVerbatim(t *testing.T) {
	prompt := "explain @does/not/exist.go please"
	original := prompt
	attached := expandAtMentions(&prompt)
	if attached != nil {
		t.Errorf("attached %v, want nil for missing file", attached)
	}
	if prompt != original {
		t.Errorf("prompt was modified for missing file: %q", prompt)
	}
}

func TestExpandAtMentions_SkipsOversizedFile(t *testing.T) {
	dir := t.TempDir()
	huge := filepath.Join(dir, "huge.bin")
	// 257 KiB > 256 KiB cap.
	if err := os.WriteFile(huge, make([]byte, 257*1024), 0o644); err != nil {
		t.Fatalf("write huge: %v", err)
	}
	old, _ := os.Getwd()
	defer os.Chdir(old)
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	// Round-311: expandAtMentions's oversize-skip strings route through
	// the i18n seam. Wire a translator resolving the round-311 IDs so the
	// assertions below check the real user-facing text.
	prevTr := translator
	SetTranslator(round311TestTranslator{})
	defer func() { translator = prevTr }()

	prompt := "look at @huge.bin"
	attached := expandAtMentions(&prompt)
	if len(attached) != 1 {
		t.Fatalf("attached %v, want 1 path", attached)
	}
	if !strings.Contains(attached[0], "skipped: too large") {
		t.Errorf("attached path missing skip marker: %q", attached[0])
	}
	if !strings.Contains(prompt, "skipped: file exceeds 256 KiB") {
		t.Errorf("prompt missing oversize-skip block: %q", prompt)
	}
}

func TestExpandAtMentions_DirectoryNotAttached(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "subdir")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	old, _ := os.Getwd()
	defer os.Chdir(old)
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	prompt := "look at @subdir please"
	attached := expandAtMentions(&prompt)
	if attached != nil {
		t.Errorf("attached %v, want nil for directory", attached)
	}
}

func TestExpandAtMentions_NoMentionsReturnsNil(t *testing.T) {
	prompt := "just a normal question with no at-mentions"
	original := prompt
	attached := expandAtMentions(&prompt)
	if attached != nil {
		t.Errorf("attached %v, want nil", attached)
	}
	if prompt != original {
		t.Errorf("prompt was modified: %q", prompt)
	}
}

func TestExpandAtMentions_DeduplicatesRepeatedPaths(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "x.go")
	if err := os.WriteFile(f, []byte("only once"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	old, _ := os.Getwd()
	defer os.Chdir(old)
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	prompt := "@x.go compared to @x.go"
	attached := expandAtMentions(&prompt)
	if len(attached) != 1 {
		t.Fatalf("attached %v, want 1 (dedup)", attached)
	}
	// The content should appear exactly once in the attached_files block.
	if strings.Count(prompt, "only once") != 1 {
		t.Errorf("content appears %d times, want exactly 1", strings.Count(prompt, "only once"))
	}
}
