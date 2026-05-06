package smartedit

import (
	"bytes"
	"testing"
)

// TestIsBinary_PlainText asserts that plain ASCII text is classified as
// non-binary. This is the dominant input class for the smart-edit tool: any
// hand-authored source file the agent will ever attempt to edit must pass
// this check.
func TestIsBinary_PlainText(t *testing.T) {
	content := []byte("Hello, world!\nThis is plain ASCII text.\nNothing fancy here.\n")
	if IsBinary(content) {
		t.Errorf("plain ASCII text mis-classified as binary")
	}
}

// TestIsBinary_UTF8Text asserts that valid multi-byte UTF-8 (Cyrillic, CJK,
// emoji) is classified as non-binary. The applier MUST handle non-ASCII
// source files; otherwise localised comments and identifiers would trigger
// false binary refusals.
func TestIsBinary_UTF8Text(t *testing.T) {
	content := []byte("Здравствуй мир\n你好世界\nこんにちは世界\n🚀 Hello 🌍\n")
	if IsBinary(content) {
		t.Errorf("valid multi-byte UTF-8 text mis-classified as binary")
	}
}

// TestIsBinary_NULByteInFirst8K asserts that the presence of any 0x00 byte
// in the sampled prefix triggers binary classification. This is the strongest
// single signal of binary content in practice (object files, images, native
// executables, compiled JVM/Wasm modules all contain NULs near the start).
func TestIsBinary_NULByteInFirst8K(t *testing.T) {
	content := []byte("some text\x00with a NUL byte embedded\n")
	if !IsBinary(content) {
		t.Errorf("content with NUL byte in first 8KB mis-classified as text")
	}
}

// TestIsBinary_NULByteAfter8K asserts that the heuristic samples only the
// first 8 KiB. A NUL byte beyond that window is intentionally NOT detected;
// an attacker concealing binary data past 8 KiB is a non-goal of this
// conservative classifier (the broader caller protections — MaxFileBytes,
// path validation — handle that threat).
func TestIsBinary_NULByteAfter8K(t *testing.T) {
	// 9 KiB of ASCII text followed by a NUL byte; only the first 8 KiB is
	// sampled, so the NUL must NOT be detected.
	prefix := bytes.Repeat([]byte("a"), 9*1024)
	content := append(prefix, 0x00)
	if IsBinary(content) {
		t.Errorf("NUL byte beyond 8KB sampling window must not trigger binary detection")
	}
}

// TestIsBinary_InvalidUTF8 asserts that content containing invalid UTF-8
// byte sequences (lone continuation bytes, truncated sequences, illegal
// surrogates encoded in UTF-8 form) is classified as binary. Source files
// in any modern repo are valid UTF-8; invalid UTF-8 implies the content is
// either binary or in a legacy encoding the applier has no business editing.
func TestIsBinary_InvalidUTF8(t *testing.T) {
	// 0x80 alone is an invalid UTF-8 sequence (continuation byte without a
	// preceding lead byte). 0xFF is never legal in UTF-8.
	content := []byte{0x48, 0x65, 0x6C, 0x6C, 0x6F, 0xFF, 0x80, 0x0A}
	if !IsBinary(content) {
		t.Errorf("invalid UTF-8 byte sequences must be classified as binary")
	}
}

// TestIsBinary_EmptyContent asserts that an empty byte slice is classified
// as non-binary. An empty file is a degenerate but valid text file; the
// applier should be free to add content to it (though the parser will
// reject any non-empty SEARCH against it).
func TestIsBinary_EmptyContent(t *testing.T) {
	if IsBinary(nil) {
		t.Errorf("nil content mis-classified as binary")
	}
	if IsBinary([]byte{}) {
		t.Errorf("empty byte slice mis-classified as binary")
	}
}

// TestIsBinary_LargePlainText asserts that 100 KiB of pure ASCII (well
// beyond the 8 KiB sampling limit) is still classified as text. Confirms
// the sampling-prefix optimisation does not produce false positives on
// large but valid source files.
func TestIsBinary_LargePlainText(t *testing.T) {
	content := bytes.Repeat([]byte("The quick brown fox jumps over the lazy dog.\n"), 2500)
	if len(content) < 100*1024 {
		t.Fatalf("test fixture too small: %d bytes", len(content))
	}
	if IsBinary(content) {
		t.Errorf("100KB of plain ASCII mis-classified as binary")
	}
}
