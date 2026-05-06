package smartedit

import (
	"bytes"
	"unicode/utf8"
)

// binarySampleBytes is the size of the prefix the heuristic inspects.
// Every modern source file has its declarative preamble (shebang, package
// statement, copyright header) inside the first 8 KiB; binary container
// formats (ELF, PE, ZIP, PDF, PNG, JPEG, classfiles) all carry their magic
// number and at least one NUL within the same window. 8 KiB is therefore
// the smallest sample size that captures the discriminating signal without
// reading the whole file end-to-end (which `os.ReadFile` will already have
// done at the caller, but the classifier should still be O(window) so we
// can reuse it on streamed input later).
const binarySampleBytes = 8 * 1024

// IsBinary reports whether content appears to be binary. Heuristic, in order:
//
//  1. If the first up-to-`binarySampleBytes` bytes contain any 0x00 byte,
//     classify as binary. NUL bytes are vanishingly rare in legitimate text
//     files (no UTF-8 / UTF-16-LE / UTF-16-BE / Windows-1252 / Latin-1
//     encoding produces a NUL for any character that appears in source code)
//     and pervasive in container formats.
//  2. Else, if `utf8.Valid` returns false on the same prefix, classify as
//     binary. Source files in HelixCode and every dependency are valid
//     UTF-8 by repo policy; invalid UTF-8 implies content the smart-edit
//     tool has no business mutating.
//  3. Otherwise the content is text.
//
// Pure function. Conservative: ambiguous inputs (tiny prefix, marginal byte
// distributions) tilt towards "text" so the applier remains usable on
// minimal source files. The ErrBinaryFile refusal path is the only place
// where this verdict is consumed.
//
// nil and empty slices are treated as text — a zero-byte file is a
// degenerate text file the applier may legitimately edit (though the
// parser will reject any non-empty SEARCH against it).
func IsBinary(content []byte) bool {
	if len(content) == 0 {
		return false
	}

	sample := content
	if len(sample) > binarySampleBytes {
		sample = sample[:binarySampleBytes]
	}

	if bytes.IndexByte(sample, 0x00) != -1 {
		return true
	}
	if !utf8.Valid(sample) {
		return true
	}
	return false
}
