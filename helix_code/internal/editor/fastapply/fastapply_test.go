package fastapply

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// corpusCase is one edit-corpus entry: an original file, an instruction,
// and the expected edited file. The corpus spans insert / delete / replace
// / multi-hunk / large-file shapes — the byte-equality anti-bluff core of
// P3-T03.
type corpusCase struct {
	name     string
	original string
	instr    *Instruction
	want     string
}

// editCorpus is the canonical apply corpus. Every fast-apply result MUST be
// byte-identical to the reference apply across every entry.
func editCorpus() []corpusCase {
	largeOriginal := strings.Repeat("line of unchanged content\n", 4000) +
		"// MARKER: edit me here\n" +
		strings.Repeat("more unchanged content\n", 4000)
	largeWant := strings.Repeat("line of unchanged content\n", 4000) +
		"// MARKER: EDITED\n" +
		strings.Repeat("more unchanged content\n", 4000)

	return []corpusCase{
		{
			name:     "replace_single_token",
			original: "package main\n\nfunc main() {\n\tprintln(\"hello\")\n}\n",
			instr: &Instruction{FilePath: "main.go", Hunks: []Hunk{
				{Kind: EditReplace, Search: "\"hello\"", Replace: "\"world\""},
			}},
			want: "package main\n\nfunc main() {\n\tprintln(\"world\")\n}\n",
		},
		{
			name:     "replace_multiline_block",
			original: "a\nold block line 1\nold block line 2\nb\n",
			instr: &Instruction{Hunks: []Hunk{
				{Kind: EditReplace, Search: "old block line 1\nold block line 2", Replace: "new block"},
			}},
			want: "a\nnew block\nb\n",
		},
		{
			name:     "delete_line",
			original: "keep 1\ndelete this line\nkeep 2\n",
			instr: &Instruction{Hunks: []Hunk{
				{Kind: EditDelete, Search: "delete this line\n"},
			}},
			want: "keep 1\nkeep 2\n",
		},
		{
			name:     "insert_before_anchor",
			original: "func b() {}\n",
			instr: &Instruction{Hunks: []Hunk{
				{Kind: EditInsertBefore, Anchor: "func b()", Replace: "func a() {}\n\n"},
			}},
			want: "func a() {}\n\nfunc b() {}\n",
		},
		{
			name:     "insert_after_anchor",
			original: "import \"fmt\"\n",
			instr: &Instruction{Hunks: []Hunk{
				{Kind: EditInsertAfter, Anchor: "import \"fmt\"\n", Replace: "import \"os\"\n"},
			}},
			want: "import \"fmt\"\nimport \"os\"\n",
		},
		{
			name:     "append_to_file",
			original: "first line\n",
			instr: &Instruction{Hunks: []Hunk{
				{Kind: EditAppend, Replace: "appended line\n"},
			}},
			want: "first line\nappended line\n",
		},
		{
			name:     "prepend_to_file",
			original: "body\n",
			instr: &Instruction{Hunks: []Hunk{
				{Kind: EditPrepend, Replace: "// header\n"},
			}},
			want: "// header\nbody\n",
		},
		{
			name:     "multi_hunk_replace_and_insert",
			original: "alpha\nbeta\ngamma\n",
			instr: &Instruction{Hunks: []Hunk{
				{Kind: EditReplace, Search: "beta", Replace: "BETA"},
				{Kind: EditInsertAfter, Anchor: "gamma\n", Replace: "delta\n"},
				{Kind: EditPrepend, Replace: "// top\n"},
			}},
			want: "// top\nalpha\nBETA\ngamma\ndelta\n",
		},
		{
			name:     "multi_hunk_sequential_dependence",
			original: "x\n",
			instr: &Instruction{Hunks: []Hunk{
				{Kind: EditReplace, Search: "x", Replace: "y"},
				{Kind: EditReplace, Search: "y", Replace: "z"},
			}},
			want: "z\n",
		},
		{
			name:     "large_file_single_hunk",
			original: largeOriginal,
			instr: &Instruction{Hunks: []Hunk{
				{Kind: EditReplace, Search: "// MARKER: edit me here", Replace: "// MARKER: EDITED"},
			}},
			want: largeWant,
		},
		{
			name:     "empty_file_append",
			original: "",
			instr: &Instruction{Hunks: []Hunk{
				{Kind: EditAppend, Replace: "new content\n"},
			}},
			want: "new content\n",
		},
		{
			name:     "no_op_equivalent_replace",
			original: "stable\n",
			instr: &Instruction{Hunks: []Hunk{
				{Kind: EditReplace, Search: "stable", Replace: "stable"},
			}},
			want: "stable\n",
		},
	}
}

// TestReferenceApply_Corpus proves the reference apply is correct across the
// full corpus — it is the trusted oracle, so it must itself be exact.
func TestReferenceApply_Corpus(t *testing.T) {
	for _, tc := range editCorpus() {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ReferenceApply(tc.instr, []byte(tc.original))
			if err != nil {
				t.Fatalf("ReferenceApply error: %v", err)
			}
			if string(got) != tc.want {
				t.Fatalf("reference apply mismatch\n want: %q\n got:  %q", tc.want, string(got))
			}
		})
	}
}

// TestFastApply_ByteEqualsReference is the ANTI-BLUFF CORE of P3-T03.
//
// For every corpus entry it runs the full Applier with the speculative fast
// route and asserts (a) the fast route was actually taken, (b) the shipped
// bytes are byte-identical to a fresh independent ReferenceApply. A fast
// apply that ships even one differing byte fails here.
func TestFastApply_ByteEqualsReference(t *testing.T) {
	a := NewApplier(DefaultConfig(), SpeculativeFastEditFunc())
	ctx := context.Background()

	for _, tc := range editCorpus() {
		t.Run(tc.name, func(t *testing.T) {
			refBytes, refErr := ReferenceApply(tc.instr, []byte(tc.original))
			if refErr != nil {
				t.Fatalf("reference apply error: %v", refErr)
			}

			out, err := a.Apply(ctx, tc.instr, []byte(tc.original))
			if err != nil {
				t.Fatalf("Apply error: %v", err)
			}

			// Byte-equality: shipped bytes == independent reference apply.
			if string(out.Content) != string(refBytes) {
				t.Fatalf("BYTE MISMATCH fast vs reference\n ref:  %q\n fast: %q", string(refBytes), string(out.Content))
			}
			// And byte-equality with the declared expected output.
			if string(out.Content) != tc.want {
				t.Fatalf("output mismatch\n want: %q\n got:  %q", tc.want, string(out.Content))
			}
			// The fast route must actually have been taken and verified.
			if !out.UsedFast() {
				t.Fatalf("expected fast route, got %s (fallback=%s)", out.Route, out.Fallback)
			}
			if !out.FastBytesVerified {
				t.Fatal("fast bytes were not marked verified")
			}
			if out.Fallback != FallbackNone {
				t.Fatalf("unexpected fallback %s", out.Fallback)
			}
		})
	}
}

// TestFastApply_FallbackOnWrongResult proves the central correctness
// guarantee: when the fast route produces WRONG bytes the Applier rejects
// them and ships the reference bytes. A fast-apply that produces a wrong
// file must NEVER reach the caller.
func TestFastApply_FallbackOnWrongResult(t *testing.T) {
	// A deliberately broken fast route that corrupts the file.
	brokenFast := FastEditFunc(func(_ context.Context, _ string, _ *Instruction, original []byte) ([]byte, error) {
		return append([]byte("CORRUPTED "), original...), nil
	})
	a := NewApplier(DefaultConfig(), brokenFast)

	original := []byte("package main\n\nfunc main() {}\n")
	instr := &Instruction{Hunks: []Hunk{
		{Kind: EditReplace, Search: "func main()", Replace: "func run()"},
	}}
	want, _ := ReferenceApply(instr, original)

	out, err := a.Apply(context.Background(), instr, original)
	if err != nil {
		t.Fatalf("Apply error: %v", err)
	}
	if string(out.Content) != string(want) {
		t.Fatalf("wrong file shipped despite broken fast route\n want: %q\n got:  %q", string(want), string(out.Content))
	}
	if out.UsedFast() {
		t.Fatal("broken fast result must NOT be shipped — expected reference route")
	}
	if out.Fallback != FallbackByteMismatch {
		t.Fatalf("expected FallbackByteMismatch, got %s", out.Fallback)
	}
	if out.FastBytesVerified {
		t.Fatal("corrupted fast bytes must not be marked verified")
	}
}

// TestFastApply_FallbackOnFastError proves a fast route that ERRORS falls
// back to the reference apply rather than failing the whole edit.
func TestFastApply_FallbackOnFastError(t *testing.T) {
	errFast := FastEditFunc(func(_ context.Context, _ string, _ *Instruction, _ []byte) ([]byte, error) {
		return nil, errors.New("apply model unavailable")
	})
	a := NewApplier(DefaultConfig(), errFast)

	original := []byte("hello\n")
	instr := &Instruction{Hunks: []Hunk{{Kind: EditReplace, Search: "hello", Replace: "world"}}}
	want, _ := ReferenceApply(instr, original)

	out, err := a.Apply(context.Background(), instr, original)
	if err != nil {
		t.Fatalf("Apply error: %v", err)
	}
	if string(out.Content) != string(want) {
		t.Fatalf("reference fallback wrong\n want: %q\n got:  %q", string(want), string(out.Content))
	}
	if out.UsedFast() {
		t.Fatal("errored fast route must fall back to reference")
	}
	if out.Fallback != FallbackFastError {
		t.Fatalf("expected FallbackFastError, got %s", out.Fallback)
	}
}

// TestFastApply_DisabledIsPureReference proves the no-regression safety
// valve: with Config.Enabled == false the fast route is never invoked and
// the output is byte-identical to the reference apply.
func TestFastApply_DisabledIsPureReference(t *testing.T) {
	fastInvoked := false
	fast := FastEditFunc(func(_ context.Context, _ string, _ *Instruction, original []byte) ([]byte, error) {
		fastInvoked = true
		return original, nil
	})
	a := NewApplier(DisabledConfig(), fast)

	for _, tc := range editCorpus() {
		t.Run(tc.name, func(t *testing.T) {
			refBytes, refErr := ReferenceApply(tc.instr, []byte(tc.original))
			if refErr != nil {
				t.Fatalf("reference apply error: %v", refErr)
			}
			out, err := a.Apply(context.Background(), tc.instr, []byte(tc.original))
			if err != nil {
				t.Fatalf("Apply error: %v", err)
			}
			if string(out.Content) != string(refBytes) {
				t.Fatalf("disabled apply not byte-identical to reference\n want: %q\n got:  %q", string(refBytes), string(out.Content))
			}
			if out.Route != RouteReference {
				t.Fatalf("disabled config must use reference route, got %s", out.Route)
			}
			if out.Fallback != FallbackDisabled {
				t.Fatalf("expected FallbackDisabled, got %s", out.Fallback)
			}
		})
	}
	if fastInvoked {
		t.Fatal("fast route was invoked despite Config.Enabled == false")
	}
}

// TestFastApply_NoFastFunc proves a nil fast route yields a pure reference
// apply (the Applier is valid as a reference-only applier).
func TestFastApply_NoFastFunc(t *testing.T) {
	a := NewApplier(DefaultConfig(), nil)
	original := []byte("a\nb\n")
	instr := &Instruction{Hunks: []Hunk{{Kind: EditReplace, Search: "a", Replace: "A"}}}
	want, _ := ReferenceApply(instr, original)

	out, err := a.Apply(context.Background(), instr, original)
	if err != nil {
		t.Fatalf("Apply error: %v", err)
	}
	if string(out.Content) != string(want) {
		t.Fatalf("no-fast-func output wrong: got %q want %q", string(out.Content), string(want))
	}
	if out.Fallback != FallbackNoFastFunc {
		t.Fatalf("expected FallbackNoFastFunc, got %s", out.Fallback)
	}
}

// TestReferenceApply_UnsatisfiableInstruction proves an instruction whose
// target is not present is an honest hard error — never a guessed result.
func TestReferenceApply_UnsatisfiableInstruction(t *testing.T) {
	tests := []struct {
		name string
		hunk Hunk
		want error
	}{
		{"missing_search", Hunk{Kind: EditReplace, Search: "not present", Replace: "x"}, ErrSearchNotFound},
		{"missing_anchor", Hunk{Kind: EditInsertBefore, Anchor: "no anchor", Replace: "x"}, ErrSearchNotFound},
		{"ambiguous_search", Hunk{Kind: EditReplace, Search: "dup", Replace: "x"}, ErrAmbiguousSearch},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := []byte("dup\nsome content\ndup\n")
			_, err := ReferenceApply(&Instruction{Hunks: []Hunk{tt.hunk}}, original)
			if !errors.Is(err, tt.want) {
				t.Fatalf("expected %v, got %v", tt.want, err)
			}
		})
	}
}

// TestApply_UnsatisfiableInstructionReturnsError proves the Applier returns
// an honest error (not a wrong file) when the instruction is unsatisfiable.
func TestApply_UnsatisfiableInstructionReturnsError(t *testing.T) {
	a := NewApplier(DefaultConfig(), SpeculativeFastEditFunc())
	instr := &Instruction{Hunks: []Hunk{{Kind: EditReplace, Search: "absent", Replace: "x"}}}
	_, err := a.Apply(context.Background(), instr, []byte("present content\n"))
	if err == nil {
		t.Fatal("expected error for unsatisfiable instruction, got nil")
	}
	if !errors.Is(err, ErrSearchNotFound) {
		t.Fatalf("expected ErrSearchNotFound, got %v", err)
	}
}

// TestApply_NilInstruction proves nil-instruction guarding.
func TestApply_NilInstruction(t *testing.T) {
	a := NewApplier(DefaultConfig(), SpeculativeFastEditFunc())
	if _, err := a.Apply(context.Background(), nil, []byte("x")); !errors.Is(err, ErrNilInstruction) {
		t.Fatalf("expected ErrNilInstruction, got %v", err)
	}
	if _, err := ReferenceApply(&Instruction{}, []byte("x")); !errors.Is(err, ErrNoHunks) {
		t.Fatalf("expected ErrNoHunks, got %v", err)
	}
}

// TestSpeculativeApply_ByteEqualsReference proves the speculative strategy
// itself is byte-identical to the reference apply across the corpus, and
// that the accepted-ratio (verbatim reuse) is high for small edits in
// large files — the property that makes speculative editing fast.
func TestSpeculativeApply_ByteEqualsReference(t *testing.T) {
	for _, tc := range editCorpus() {
		t.Run(tc.name, func(t *testing.T) {
			refBytes, refErr := ReferenceApply(tc.instr, []byte(tc.original))
			if refErr != nil {
				t.Fatalf("reference apply error: %v", refErr)
			}
			draft, err := SpeculativeApply(tc.instr, []byte(tc.original))
			if err != nil {
				t.Fatalf("SpeculativeApply error: %v", err)
			}
			if string(draft.Bytes()) != string(refBytes) {
				t.Fatalf("speculative draft != reference\n ref:   %q\n draft: %q", string(refBytes), string(draft.Bytes()))
			}
		})
	}
}

// TestSpeculativeApply_HighAcceptanceOnLargeFile proves the speculative
// strategy reuses the overwhelming majority of a large file verbatim — the
// reason a speculative apply is an order of magnitude faster than a full
// re-emit.
func TestSpeculativeApply_HighAcceptanceOnLargeFile(t *testing.T) {
	original := strings.Repeat("unchanged\n", 5000) + "TARGET\n" + strings.Repeat("unchanged\n", 5000)
	instr := &Instruction{Hunks: []Hunk{{Kind: EditReplace, Search: "TARGET", Replace: "REPLACED"}}}
	draft, err := SpeculativeApply(instr, []byte(original))
	if err != nil {
		t.Fatalf("SpeculativeApply error: %v", err)
	}
	ratio := draft.AcceptedRatio()
	if ratio < 0.99 {
		t.Fatalf("expected >=99%% verbatim reuse for a small edit in a large file, got %.4f", ratio)
	}
	t.Logf("speculative accepted ratio (verbatim reuse) = %.6f", ratio)
}

// TestApply_StatsAccumulate proves the aggregate Stats evidence is recorded.
func TestApply_StatsAccumulate(t *testing.T) {
	a := NewApplier(DefaultConfig(), SpeculativeFastEditFunc())
	ctx := context.Background()
	corpus := editCorpus()
	for _, tc := range corpus {
		if _, err := a.Apply(ctx, tc.instr, []byte(tc.original)); err != nil {
			t.Fatalf("Apply error: %v", err)
		}
	}
	s := a.Stats()
	if s.Total != len(corpus) {
		t.Fatalf("expected %d total applies, got %d", len(corpus), s.Total)
	}
	if s.FastShipped != len(corpus) {
		t.Fatalf("expected all %d applies to ship fast, got %d", len(corpus), s.FastShipped)
	}
	if s.ByteMismatch != 0 || s.FastError != 0 {
		t.Fatalf("unexpected fallbacks: mismatch=%d error=%d", s.ByteMismatch, s.FastError)
	}
	fmt.Printf("[evidence] applies=%d fast-shipped=%d ref-shipped=%d byte-mismatch=%d\n",
		s.Total, s.FastShipped, s.ReferenceShipped, s.ByteMismatch)
}
