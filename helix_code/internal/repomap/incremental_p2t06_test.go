package repomap

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	sitter "github.com/smacker/go-tree-sitter"
)

// incremental_p2t06_test.go — speed-programme Phase 2 task P2-T06.
//
// Proves incremental tree-sitter parsing via the edit API (R1 B06 extends, O9):
//
//   - the ANTI-BLUFF core: an incremental re-parse of an edited file produces a
//     syntax tree AND extracted symbol set IDENTICAL to a full re-parse of the
//     same final content — asserted across insert / delete / replace / multi-
//     line edit shapes, and across the whole supported-language matrix;
//   - a first parse with no prior tree still works (full parse) and a parse
//     after a language change falls back to a full parse;
//   - the parser pool + per-file tree retention has no cross-file state bleed
//     under `-race` (run with `go test -race`);
//   - benchmark: per-edit re-parse incremental vs full.
//
// Per CONST-035 / Article XI §11.9: every PASS here carries positive runtime
// evidence — actual AST/symbol equality, not metadata-only assertions.

// astSignature renders a syntax tree into a deterministic string capturing
// every node's type and byte span. Two trees with identical astSignature are
// structurally identical — this is the equality oracle for incremental vs full.
func astSignature(tree *sitter.Tree) string {
	if tree == nil {
		return "<nil-tree>"
	}
	var b strings.Builder
	var walk func(n *sitter.Node, depth int)
	walk = func(n *sitter.Node, depth int) {
		if n == nil {
			return
		}
		fmt.Fprintf(&b, "%s%s[%d:%d]\n",
			strings.Repeat(" ", depth), n.Type(), n.StartByte(), n.EndByte())
		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i), depth+1)
		}
	}
	walk(tree.RootNode(), 0)
	return b.String()
}

// symbolSignature renders an extracted symbol slice into a deterministic,
// order-independent string for equality comparison.
func symbolSignature(syms []Symbol) string {
	lines := make([]string, 0, len(syms))
	for _, s := range syms {
		lines = append(lines, fmt.Sprintf("%s|%v|%d|%d|%s",
			s.Name, s.Type, s.LineStart, s.LineEnd, s.Parent))
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

// editCase describes one before/after content pair for the equality matrix.
type editCase struct {
	name     string
	language string
	ext      string
	before   string
	after    string
}

func incrementalEditCases() []editCase {
	const goBefore = `package demo

import "fmt"

// Greeter prints greetings.
type Greeter struct {
	Name string
}

func (g *Greeter) Hello() string {
	return fmt.Sprintf("hello %s", g.Name)
}

func main() {
	g := &Greeter{Name: "world"}
	fmt.Println(g.Hello())
}
`
	return []editCase{
		{
			name:     "go_insert_function",
			language: "go", ext: ".go",
			before: goBefore,
			after: strings.Replace(goBefore,
				"func main() {",
				"func Goodbye() string {\n\treturn \"bye\"\n}\n\nfunc main() {", 1),
		},
		{
			name:     "go_delete_function",
			language: "go", ext: ".go",
			before: goBefore,
			after: strings.Replace(goBefore,
				"func (g *Greeter) Hello() string {\n\treturn fmt.Sprintf(\"hello %s\", g.Name)\n}\n\n",
				"", 1),
		},
		{
			name:     "go_replace_identifier",
			language: "go", ext: ".go",
			before: goBefore,
			after:  strings.Replace(goBefore, "Greeter", "Welcomer", -1),
		},
		{
			name:     "go_multiline_body_change",
			language: "go", ext: ".go",
			before: goBefore,
			after: strings.Replace(goBefore,
				"\treturn fmt.Sprintf(\"hello %s\", g.Name)\n",
				"\tprefix := \"hi\"\n\treturn fmt.Sprintf(\"%s %s\", prefix, g.Name)\n", 1),
		},
		{
			name:     "python_insert_method",
			language: "python", ext: ".py",
			before: "class Calc:\n    def add(self, a, b):\n        return a + b\n",
			after:  "class Calc:\n    def add(self, a, b):\n        return a + b\n\n    def sub(self, a, b):\n        return a - b\n",
		},
		{
			name:     "javascript_replace_value",
			language: "javascript", ext: ".js",
			before: "function area(r) {\n  return 3.14 * r * r;\n}\n",
			after:  "function area(r) {\n  return Math.PI * r * r;\n}\n",
		},
		{
			name:     "go_append_at_eof",
			language: "go", ext: ".go",
			before: goBefore,
			after:  goBefore + "\nfunc Extra() {}\n",
		},
		{
			name:     "go_insert_at_bof",
			language: "go", ext: ".go",
			before: goBefore,
			after:  "// new top comment\n" + goBefore,
		},
	}
}

// TestIncrementalParse_EqualsFullParse is the anti-bluff core: for every edit
// shape, an incremental re-parse of the edited file must produce a tree and
// symbol set byte-identical to a FULL re-parse of the same final content.
func TestIncrementalParse_EqualsFullParse(t *testing.T) {
	ctx := context.Background()
	tsp := NewTreeSitterParser()

	for _, tc := range incrementalEditCases() {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			file := filepath.Join(dir, "src"+tc.ext)

			// 1. First parse (full) of the BEFORE content via the incremental
			// parser — establishes the retained tree.
			if err := os.WriteFile(file, []byte(tc.before), 0o644); err != nil {
				t.Fatalf("write before: %v", err)
			}
			ip := NewIncrementalParser(tsp)
			firstTree, firstWasIncremental, err := ip.ParseFile(ctx, file, tc.language)
			if err != nil {
				t.Fatalf("first parse: %v", err)
			}
			if firstWasIncremental {
				t.Fatalf("first parse must be a FULL parse (no prior tree), got incremental")
			}
			firstTree.Close()

			// 2. Write the AFTER content and re-parse — this MUST be incremental.
			if err := os.WriteFile(file, []byte(tc.after), 0o644); err != nil {
				t.Fatalf("write after: %v", err)
			}
			incTree, wasIncremental, err := ip.ParseFile(ctx, file, tc.language)
			if err != nil {
				t.Fatalf("incremental re-parse: %v", err)
			}
			defer incTree.Close()
			if !wasIncremental {
				t.Fatalf("re-parse after edit must be incremental, got full parse")
			}

			// 3. Independently full-parse the AFTER content.
			fullTree, err := tsp.ParseContent(ctx, []byte(tc.after), tc.language, nil)
			if err != nil {
				t.Fatalf("full parse of after: %v", err)
			}
			defer fullTree.Close()

			// 4. AST equality — the anti-bluff proof.
			incAST := astSignature(incTree)
			fullAST := astSignature(fullTree)
			if incAST != fullAST {
				t.Fatalf("AST MISMATCH incremental vs full\n--- incremental ---\n%s\n--- full ---\n%s",
					incAST, fullAST)
			}

			// 5. Symbol-set equality — extracted symbols must also match.
			incSyms, err := tsp.ExtractSymbols(incTree, file, tc.language)
			if err != nil {
				t.Fatalf("extract incremental symbols: %v", err)
			}
			fullSyms, err := tsp.ExtractSymbols(fullTree, file, tc.language)
			if err != nil {
				t.Fatalf("extract full symbols: %v", err)
			}
			if symbolSignature(incSyms) != symbolSignature(fullSyms) {
				t.Fatalf("SYMBOL MISMATCH incremental vs full\n--- incremental ---\n%s\n--- full ---\n%s",
					symbolSignature(incSyms), symbolSignature(fullSyms))
			}
			t.Logf("OK %s: AST + %d symbols identical (incremental == full)", tc.name, len(incSyms))
		})
	}
}

// TestIncrementalParse_RepeatedEdits proves a chain of edits stays correct —
// each re-parse builds on the previous retained tree and still equals a full
// parse of the final content.
func TestIncrementalParse_RepeatedEdits(t *testing.T) {
	ctx := context.Background()
	tsp := NewTreeSitterParser()
	ip := NewIncrementalParser(tsp)

	dir := t.TempDir()
	file := filepath.Join(dir, "chain.go")

	versions := []string{
		"package p\n\nfunc A() {}\n",
		"package p\n\nfunc A() {}\n\nfunc B() {}\n",
		"package p\n\nfunc A() int { return 1 }\n\nfunc B() {}\n",
		"package p\n\nfunc A() int { return 1 }\n",
	}

	for i, content := range versions {
		if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
			t.Fatalf("write v%d: %v", i, err)
		}
		tree, wasIncremental, err := ip.ParseFile(ctx, file, "go")
		if err != nil {
			t.Fatalf("parse v%d: %v", i, err)
		}
		if i == 0 && wasIncremental {
			t.Fatalf("v0 must be full parse")
		}
		if i > 0 && !wasIncremental {
			t.Fatalf("v%d must be incremental", i)
		}
		full, err := tsp.ParseContent(ctx, []byte(content), "go", nil)
		if err != nil {
			t.Fatalf("full parse v%d: %v", i, err)
		}
		if astSignature(tree) != astSignature(full) {
			t.Fatalf("v%d AST mismatch incremental vs full", i)
		}
		tree.Close()
		full.Close()
	}
}

// TestIncrementalParse_FirstParseNoPriorTree confirms a brand-new file parses
// as a full parse and produces a valid tree.
func TestIncrementalParse_FirstParseNoPriorTree(t *testing.T) {
	ctx := context.Background()
	ip := NewIncrementalParser(NewTreeSitterParser())
	dir := t.TempDir()
	file := filepath.Join(dir, "fresh.go")
	if err := os.WriteFile(file, []byte("package fresh\n\nfunc Solo() {}\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	tree, wasIncremental, err := ip.ParseFile(ctx, file, "go")
	if err != nil {
		t.Fatalf("first parse: %v", err)
	}
	defer tree.Close()
	if wasIncremental {
		t.Fatalf("first parse of a file with no prior tree must be full, got incremental")
	}
	if tree.RootNode() == nil || tree.RootNode().HasError() {
		t.Fatalf("first parse produced an invalid tree")
	}
}

// TestIncrementalParse_IdenticalContentReparse confirms a re-parse of byte-
// identical content reuses the retained tree and still equals a full parse.
func TestIncrementalParse_IdenticalContentReparse(t *testing.T) {
	ctx := context.Background()
	tsp := NewTreeSitterParser()
	ip := NewIncrementalParser(tsp)
	content := []byte("package p\n\nfunc Same() {}\n")

	first, _, err := ip.ParseContent(ctx, "x.go", "go", content)
	if err != nil {
		t.Fatalf("first parse: %v", err)
	}
	first.Close()

	second, _, err := ip.ParseContent(ctx, "x.go", "go", content)
	if err != nil {
		t.Fatalf("identical re-parse: %v", err)
	}
	defer second.Close()

	full, err := tsp.ParseContent(ctx, content, "go", nil)
	if err != nil {
		t.Fatalf("full parse: %v", err)
	}
	defer full.Close()

	if astSignature(second) != astSignature(full) {
		t.Fatalf("identical-content re-parse AST mismatch vs full")
	}
}

// TestIncrementalParse_LanguageChangeFallsBackToFull confirms that reusing a
// path for a different language discards the stale tree and full-parses.
func TestIncrementalParse_LanguageChangeFallsBackToFull(t *testing.T) {
	ctx := context.Background()
	ip := NewIncrementalParser(NewTreeSitterParser())

	first, _, err := ip.ParseContent(ctx, "ambiguous", "go", []byte("package p\nfunc A(){}\n"))
	if err != nil {
		t.Fatalf("go parse: %v", err)
	}
	first.Close()

	// Same path key, different language — must NOT reuse the Go tree.
	second, wasIncremental, err := ip.ParseContent(ctx, "ambiguous", "python",
		[]byte("def a():\n    pass\n"))
	if err != nil {
		t.Fatalf("python parse: %v", err)
	}
	defer second.Close()
	if wasIncremental {
		t.Fatalf("language change must force a full parse, got incremental")
	}
	if second.RootNode().HasError() {
		t.Fatalf("python parse after language change produced an invalid tree")
	}
}

// TestIncrementalParse_NoCrossFileStateBleed is the `-race` guard: many
// goroutines each repeatedly edit-and-reparse their OWN distinct file through a
// SHARED IncrementalParser + shared parser pool. A correct implementation keeps
// each file's retained tree isolated; any cross-file bleed corrupts a result.
//
// Run with: go test -race -run TestIncrementalParse_NoCrossFileStateBleed
func TestIncrementalParse_NoCrossFileStateBleed(t *testing.T) {
	ctx := context.Background()
	tsp := NewTreeSitterParser()
	ip := NewIncrementalParser(tsp)
	dir := t.TempDir()

	const goroutines = 12
	const iterations = 8

	var wg sync.WaitGroup
	errCh := make(chan error, goroutines*iterations)

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			file := filepath.Join(dir, fmt.Sprintf("worker_%d.go", id))
			for it := 0; it < iterations; it++ {
				// Each iteration produces content unique to (id, it) so a bleed
				// from another file would yield a wrong AST.
				content := fmt.Sprintf(
					"package w%d\n\nfunc F%d_%d() int { return %d }\n",
					id, id, it, id*1000+it)
				if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
					errCh <- fmt.Errorf("g%d it%d write: %w", id, it, err)
					return
				}
				tree, _, err := ip.ParseFile(ctx, file, "go")
				if err != nil {
					errCh <- fmt.Errorf("g%d it%d parse: %w", id, it, err)
					return
				}
				full, err := tsp.ParseContent(ctx, []byte(content), "go", nil)
				if err != nil {
					tree.Close()
					errCh <- fmt.Errorf("g%d it%d full: %w", id, it, err)
					return
				}
				if astSignature(tree) != astSignature(full) {
					errCh <- fmt.Errorf("g%d it%d: cross-file bleed — AST mismatch", id, it)
				}
				syms, err := tsp.ExtractSymbols(tree, file, "go")
				if err == nil {
					want := fmt.Sprintf("F%d_%d", id, it)
					found := false
					for _, s := range syms {
						if s.Name == want {
							found = true
							break
						}
					}
					if !found {
						errCh <- fmt.Errorf("g%d it%d: expected symbol %s missing — state bleed", id, it, want)
					}
				}
				tree.Close()
				full.Close()
			}
		}(g)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Error(err)
	}
}

// TestComputeEditInput_Shapes unit-tests the edit-descriptor computation across
// edit shapes, asserting the changed span the descriptor reports actually
// covers the differing bytes.
func TestComputeEditInput_Shapes(t *testing.T) {
	cases := []struct {
		name      string
		before    string
		after     string
		wantEdit  bool
	}{
		{"identical", "abcdef", "abcdef", false},
		{"pure_insert_middle", "abcdef", "abcXYZdef", true},
		{"pure_delete_middle", "abcXYZdef", "abcdef", true},
		{"replace_middle", "abcXYZdef", "abc123def", true},
		{"append_eof", "abcdef", "abcdefghi", true},
		{"prepend_bof", "abcdef", "XYZabcdef", true},
		{"full_replace", "abcdef", "zzzzzz", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			edit, changed := computeEditInput([]byte(tc.before), []byte(tc.after))
			if changed != tc.wantEdit {
				t.Fatalf("changed=%v want %v", changed, tc.wantEdit)
			}
			if !changed {
				return
			}
			// The reported span must be self-consistent.
			if edit.StartIndex > edit.OldEndIndex {
				t.Fatalf("StartIndex %d > OldEndIndex %d", edit.StartIndex, edit.OldEndIndex)
			}
			if edit.StartIndex > uint32(len(tc.before)) || edit.OldEndIndex > uint32(len(tc.before)) {
				t.Fatalf("old span out of bounds for before len %d: %+v", len(tc.before), edit)
			}
			if edit.NewEndIndex > uint32(len(tc.after)) {
				t.Fatalf("NewEndIndex %d out of bounds for after len %d", edit.NewEndIndex, len(tc.after))
			}
			// Bytes before StartIndex must be a common prefix.
			for i := uint32(0); i < edit.StartIndex; i++ {
				if tc.before[i] != tc.after[i] {
					t.Fatalf("byte %d before StartIndex differs — not a common prefix", i)
				}
			}
		})
	}
}

// TestByteOffsetToPoint verifies row/column derivation against known offsets.
func TestByteOffsetToPoint(t *testing.T) {
	content := []byte("ab\ncde\nf")
	cases := []struct {
		offset   int
		wantRow  uint32
		wantCol  uint32
	}{
		{0, 0, 0},
		{2, 0, 2},   // before first newline
		{3, 1, 0},   // start of line 1
		{6, 1, 3},   // before second newline
		{7, 2, 0},   // start of line 2
		{8, 2, 1},   // EOF
	}
	for _, tc := range cases {
		p := byteOffsetToPoint(content, tc.offset)
		if p.Row != tc.wantRow || p.Column != tc.wantCol {
			t.Errorf("offset %d: got (%d,%d) want (%d,%d)",
				tc.offset, p.Row, p.Column, tc.wantRow, tc.wantCol)
		}
	}
}

// TestIncrementalParse_AllLanguages exercises the incremental==full guarantee
// across every supported language with a representative edit.
func TestIncrementalParse_AllLanguages(t *testing.T) {
	ctx := context.Background()
	tsp := NewTreeSitterParser()

	samples := map[string]struct {
		before, after string
	}{
		"go":     {"package p\nfunc A() {}\n", "package p\nfunc A() {}\nfunc B() {}\n"},
		"python": {"def a():\n    return 1\n", "def a():\n    return 2\n"},
		"javascript": {"function f() { return 1; }\n", "function f() { return 42; }\n"},
		"typescript": {"function f(): number { return 1; }\n", "function f(): number { return 9; }\n"},
		"java":   {"class C { void m() {} }\n", "class C { void m() {} void n() {} }\n"},
		"c":      {"int main() { return 0; }\n", "int main() { return 1; }\n"},
		"cpp":    {"int main() { return 0; }\n", "int main() { return 7; }\n"},
		"rust":   {"fn main() { let x = 1; }\n", "fn main() { let x = 2; }\n"},
		"ruby":   {"def a\n  1\nend\n", "def a\n  2\nend\n"},
	}

	for lang, s := range samples {
		t.Run(lang, func(t *testing.T) {
			ip := NewIncrementalParser(tsp)
			key := "sample." + lang
			first, _, err := ip.ParseContent(ctx, key, lang, []byte(s.before))
			if err != nil {
				t.Fatalf("%s first parse: %v", lang, err)
			}
			first.Close()

			inc, wasIncremental, err := ip.ParseContent(ctx, key, lang, []byte(s.after))
			if err != nil {
				t.Fatalf("%s incremental parse: %v", lang, err)
			}
			defer inc.Close()
			if !wasIncremental {
				t.Fatalf("%s re-parse must be incremental", lang)
			}
			full, err := tsp.ParseContent(ctx, []byte(s.after), lang, nil)
			if err != nil {
				t.Fatalf("%s full parse: %v", lang, err)
			}
			defer full.Close()
			if astSignature(inc) != astSignature(full) {
				t.Fatalf("%s: incremental AST != full AST", lang)
			}
		})
	}
}

// --- benchmarks: per-edit re-parse incremental vs full ---

// largeGoSource builds a Go file with n functions for a realistic re-parse cost.
func largeGoSource(n int) string {
	var b strings.Builder
	b.WriteString("package big\n\nimport \"fmt\"\n\n")
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "func Fn%d(x int) int {\n\ty := x * %d\n\tfmt.Println(y)\n\treturn y + %d\n}\n\n", i, i+1, i)
	}
	return b.String()
}

// BenchmarkReparse_Full re-parses the whole file from scratch on every edit.
func BenchmarkReparse_Full(b *testing.B) {
	ctx := context.Background()
	tsp := NewTreeSitterParser()
	base := largeGoSource(400)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Simulate a single small edit: change one return value.
		edited := strings.Replace(base, "return y + 0", fmt.Sprintf("return y + %d", i+1), 1)
		tree, err := tsp.ParseContent(ctx, []byte(edited), "go", nil)
		if err != nil {
			b.Fatal(err)
		}
		tree.Close()
	}
}

// BenchmarkReparse_Incremental re-parses via the edit API, reusing the prior
// tree — only the changed region is re-parsed.
func BenchmarkReparse_Incremental(b *testing.B) {
	ctx := context.Background()
	tsp := NewTreeSitterParser()
	ip := NewIncrementalParser(tsp)
	base := largeGoSource(400)

	// Prime the retained tree with a first (full) parse.
	first, _, err := ip.ParseContent(ctx, "bench.go", "go", []byte(base))
	if err != nil {
		b.Fatal(err)
	}
	first.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		edited := strings.Replace(base, "return y + 0", fmt.Sprintf("return y + %d", i+1), 1)
		tree, wasIncremental, err := ip.ParseContent(ctx, "bench.go", "go", []byte(edited))
		if err != nil {
			b.Fatal(err)
		}
		if !wasIncremental {
			b.Fatal("expected incremental re-parse")
		}
		tree.Close()
	}
}
