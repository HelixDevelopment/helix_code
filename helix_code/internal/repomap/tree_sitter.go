package repomap

import (
	"context"
	"fmt"
	"os"
	"sync"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/c"
	"github.com/smacker/go-tree-sitter/cpp"
	"github.com/smacker/go-tree-sitter/golang"
	"github.com/smacker/go-tree-sitter/java"
	"github.com/smacker/go-tree-sitter/javascript"
	"github.com/smacker/go-tree-sitter/python"
	"github.com/smacker/go-tree-sitter/ruby"
	"github.com/smacker/go-tree-sitter/rust"
	"github.com/smacker/go-tree-sitter/typescript/typescript"
)

// TreeSitterParser handles parsing of source code using Tree-sitter.
//
// R1 B06 / P2-T04: a *sitter.Parser is NOT goroutine-safe — concurrent calls
// against one parser corrupt its internal state. Allocating a fresh parser per
// file (the pre-P2-T04 behaviour) is correct but throws away the allocation on
// every call. The parserPool below recycles parsers: each worker in the
// repo-map worker pool takes a parser, sets its language, parses, and returns
// it. Because each goroutine holds exclusive ownership of its borrowed parser
// for the whole ParseFile call, the parser is never touched by two goroutines
// at once — the pool is the per-worker isolation the comment in repomap.go's
// task brief calls for.
type TreeSitterParser struct {
	languages  map[string]*sitter.Language
	parserPool sync.Pool
}

// NewTreeSitterParser creates a new Tree-sitter parser
func NewTreeSitterParser() *TreeSitterParser {
	return &TreeSitterParser{
		languages: map[string]*sitter.Language{
			"go":         golang.GetLanguage(),
			"python":     python.GetLanguage(),
			"javascript": javascript.GetLanguage(),
			"typescript": typescript.GetLanguage(),
			"java":       java.GetLanguage(),
			"c":          c.GetLanguage(),
			"cpp":        cpp.GetLanguage(),
			"rust":       rust.GetLanguage(),
			"ruby":       ruby.GetLanguage(),
		},
		parserPool: sync.Pool{
			New: func() interface{} {
				return sitter.NewParser()
			},
		},
	}
}

// ParseFile parses a source file and returns its syntax tree.
//
// R1 B06 / P2-T04: the *sitter.Parser is borrowed from parserPool, has its
// language set fresh for this file (the pool may hand back a parser last used
// for a different language), and is returned to the pool on the way out. The
// `oldTree == nil` argument to ParseCtx makes every parse a full parse with no
// state carried over from the parser's previous use — so a pooled parser yields
// byte-identical output to the pre-P2-T04 fresh-per-file parser.
func (tsp *TreeSitterParser) ParseFile(filePath string, language string) (*sitter.Tree, error) {
	lang, ok := tsp.languages[language]
	if !ok {
		return nil, fmt.Errorf("unsupported language: %s", language)
	}

	// Read file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Borrow a parser from the pool. The defer returns it so a panic or an
	// early error still recycles the parser rather than leaking it.
	parser := tsp.parserPool.Get().(*sitter.Parser)
	defer tsp.parserPool.Put(parser)

	// Reset the parser for this file: set the language (the pooled parser may
	// have been left configured for a different language) and parse with a nil
	// old tree so no incremental state bleeds in from a prior use.
	parser.SetLanguage(lang)

	// Parse content
	ctx := context.Background()
	tree, err := parser.ParseCtx(ctx, nil, content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse: %w", err)
	}

	return tree, nil
}

// ParseContent parses already-loaded source bytes. It is the shared parse
// primitive behind both ParseFile (which reads the file first) and the
// incremental path. `oldTree` is an EDIT-DESCRIBED tree for the SAME file (i.e.
// a tree on which Tree.Edit has already been called to mark the changed region)
// or nil for a full parse. tree-sitter reuses the unchanged subtrees of an
// edited oldTree, re-parsing only the regions the edit touched.
//
// R1 B06 / P2-T06: passing a non-nil oldTree is what makes the parse
// incremental. tree-sitter's contract guarantees that an incremental re-parse
// yields a syntax tree IDENTICAL to a full parse of the same final content —
// provided the oldTree was correctly edited to describe the change. Callers
// that cannot describe the change precisely MUST pass nil (full parse) — see
// IncrementalParser.Parse for the safety fallbacks.
func (tsp *TreeSitterParser) ParseContent(ctx context.Context, content []byte, language string, oldTree *sitter.Tree) (*sitter.Tree, error) {
	lang, ok := tsp.languages[language]
	if !ok {
		return nil, fmt.Errorf("unsupported language: %s", language)
	}

	// Borrow a parser from the pool. The defer returns it so a panic or an
	// early error still recycles the parser rather than leaking it.
	parser := tsp.parserPool.Get().(*sitter.Parser)
	defer tsp.parserPool.Put(parser)

	// Reset the parser for this file: set the language (the pooled parser may
	// have been left configured for a different language). The oldTree (if any)
	// carries no parser-internal state — it is an immutable syntax tree — so a
	// pooled parser handing back an oldTree from this same file is safe; a
	// pooled parser is never asked to reuse a tree from a DIFFERENT file
	// because IncrementalParser keys retained trees by file path.
	parser.SetLanguage(lang)

	tree, err := parser.ParseCtx(ctx, oldTree, content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse: %w", err)
	}
	return tree, nil
}

// computeEditInput derives the sitter.EditInput describing the single
// contiguous change between oldContent and newContent. It finds the common
// prefix and common suffix; everything between is treated as the edited span.
//
// This is exact for any single-region edit (insert, delete, replace,
// multi-line) and is a CONSERVATIVE over-approximation for multi-region edits:
// a too-wide span still produces a CORRECT incremental re-parse (tree-sitter
// simply re-parses more than strictly necessary) — never an incorrect tree.
// The returned bool is false when the two contents are byte-identical (no edit
// — caller should reuse the old tree unchanged).
func computeEditInput(oldContent, newContent []byte) (sitter.EditInput, bool) {
	if len(oldContent) == len(newContent) {
		identical := true
		for i := range oldContent {
			if oldContent[i] != newContent[i] {
				identical = false
				break
			}
		}
		if identical {
			return sitter.EditInput{}, false
		}
	}

	// Common prefix length.
	maxPrefix := len(oldContent)
	if len(newContent) < maxPrefix {
		maxPrefix = len(newContent)
	}
	prefix := 0
	for prefix < maxPrefix && oldContent[prefix] == newContent[prefix] {
		prefix++
	}

	// Common suffix length, not overlapping the prefix already consumed.
	maxSuffix := maxPrefix - prefix
	suffix := 0
	for suffix < maxSuffix &&
		oldContent[len(oldContent)-1-suffix] == newContent[len(newContent)-1-suffix] {
		suffix++
	}

	startByte := prefix
	oldEndByte := len(oldContent) - suffix
	newEndByte := len(newContent) - suffix

	return sitter.EditInput{
		StartIndex:  uint32(startByte),
		OldEndIndex: uint32(oldEndByte),
		NewEndIndex: uint32(newEndByte),
		StartPoint:  byteOffsetToPoint(oldContent, startByte),
		OldEndPoint: byteOffsetToPoint(oldContent, oldEndByte),
		NewEndPoint: byteOffsetToPoint(newContent, newEndByte),
	}, true
}

// byteOffsetToPoint converts a byte offset into a tree-sitter Point (0-based
// row, 0-based column-in-bytes). tree-sitter measures columns in bytes, so this
// counts bytes since the last newline rather than runes.
func byteOffsetToPoint(content []byte, offset int) sitter.Point {
	if offset > len(content) {
		offset = len(content)
	}
	row := uint32(0)
	col := uint32(0)
	for i := 0; i < offset; i++ {
		if content[i] == '\n' {
			row++
			col = 0
		} else {
			col++
		}
	}
	return sitter.Point{Row: row, Column: col}
}

// IncrementalParser wraps a TreeSitterParser with per-file retention of the
// previous syntax tree and its source bytes. When the same file is re-parsed
// after an edit, it edits the retained tree to describe the change and asks
// tree-sitter to re-parse incrementally — re-parsing only the changed regions.
//
// R1 B06 / P2-T06 safety contract:
//   - A retained tree is ONLY ever paired with the SAME file path (the cache is
//     keyed by path) — never with an unrelated file. This is the cross-file
//     state-bleed guard the task brief requires.
//   - The first parse of a file (no prior tree) is a full parse.
//   - If the language changed between parses, the prior tree is discarded and a
//     full parse is done (a tree from a different grammar cannot be reused).
//   - All access is mutex-guarded so the type is safe for concurrent use across
//     the repo-map worker pool.
//
// Correctness guarantee: an incremental re-parse produces a syntax tree (and
// therefore an extracted symbol set) IDENTICAL to a full re-parse of the same
// final content — proven by the incremental-vs-full equality tests.
type IncrementalParser struct {
	tsp *TreeSitterParser

	mu      sync.Mutex
	entries map[string]*incrementalEntry
}

// incrementalEntry is the retained state for one file.
type incrementalEntry struct {
	tree     *sitter.Tree
	content  []byte
	language string
}

// NewIncrementalParser creates an IncrementalParser over the given underlying
// TreeSitterParser. The TreeSitterParser (and its parser pool) is shared, so an
// IncrementalParser composes with the P2-T04 parser pool.
func NewIncrementalParser(tsp *TreeSitterParser) *IncrementalParser {
	return &IncrementalParser{
		tsp:     tsp,
		entries: make(map[string]*incrementalEntry),
	}
}

// ParseFile reads the file and parses it, reusing the previously retained tree
// for this same file path when one exists (incremental re-parse). The first
// call for a path — or any call after a language change — is a full parse.
//
// The returned tree is also retained (replacing any prior retention for the
// path) so the NEXT edit of the file can itself be incremental. The boolean
// reports whether this particular parse was incremental (true) or full (false)
// — used by tests and benchmarks to prove the incremental path was taken.
func (ip *IncrementalParser) ParseFile(ctx context.Context, filePath, language string) (*sitter.Tree, bool, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, false, fmt.Errorf("failed to read file: %w", err)
	}
	return ip.ParseContent(ctx, filePath, language, content)
}

// ParseContent parses already-loaded bytes for filePath, reusing the retained
// tree for that path when safe. Exposed separately from ParseFile so callers
// that already hold the content (e.g. an editor buffer) avoid a re-read.
func (ip *IncrementalParser) ParseContent(ctx context.Context, filePath, language string, content []byte) (*sitter.Tree, bool, error) {
	ip.mu.Lock()
	prev := ip.entries[filePath]
	ip.mu.Unlock()

	var oldTree *sitter.Tree
	incremental := false

	// Incremental is only attempted when there is a prior tree for THIS file
	// AND the language is unchanged. A different grammar cannot reuse subtrees.
	if prev != nil && prev.tree != nil && prev.language == language {
		edit, changed := computeEditInput(prev.content, content)
		if !changed {
			// Content is byte-identical — the retained tree is already correct.
			// Hand back a copy so the caller owns an independent tree and the
			// retained one stays valid for the next call.
			ip.mu.Lock()
			ip.entries[filePath] = &incrementalEntry{
				tree:     prev.tree.Copy(),
				content:  append([]byte(nil), content...),
				language: language,
			}
			retained := ip.entries[filePath]
			ip.mu.Unlock()
			return retained.tree.Copy(), true, nil
		}
		// Work on a copy: Tree.Edit mutates the tree in place, and we must not
		// corrupt the retained tree if the parse below fails.
		oldTree = prev.tree.Copy()
		oldTree.Edit(edit)
		incremental = true
	}

	tree, err := ip.tsp.ParseContent(ctx, content, language, oldTree)
	if err != nil {
		// Incremental parse failed — fall back to a clean full parse so a
		// caller never sees a transient incremental failure as a hard error.
		if incremental {
			tree, err = ip.tsp.ParseContent(ctx, content, language, nil)
			incremental = false
		}
		if err != nil {
			return nil, false, err
		}
	}

	// Retain a copy for the next edit; hand the caller its own copy. Two
	// independent trees means neither side's later Edit/Close touches the other.
	retainCopy := tree.Copy()
	ip.mu.Lock()
	ip.entries[filePath] = &incrementalEntry{
		tree:     retainCopy,
		content:  append([]byte(nil), content...),
		language: language,
	}
	ip.mu.Unlock()

	return tree, incremental, nil
}

// Forget drops any retained tree for the given file path (e.g. when a file is
// deleted). The next parse of that path becomes a full parse again.
func (ip *IncrementalParser) Forget(filePath string) {
	ip.mu.Lock()
	delete(ip.entries, filePath)
	ip.mu.Unlock()
}

// Reset drops all retained trees. After Reset every file's next parse is a full
// parse.
func (ip *IncrementalParser) Reset() {
	ip.mu.Lock()
	ip.entries = make(map[string]*incrementalEntry)
	ip.mu.Unlock()
}

// ExtractSymbols extracts symbols from a parsed syntax tree
func (tsp *TreeSitterParser) ExtractSymbols(tree *sitter.Tree, filePath string, language string) ([]Symbol, error) {
	if tree == nil {
		return nil, fmt.Errorf("tree is nil")
	}

	extractor := NewTagExtractor(language)
	symbols := extractor.Extract(tree, filePath)

	return symbols, nil
}

// SupportedLanguages returns the list of supported languages
func (tsp *TreeSitterParser) SupportedLanguages() []string {
	langs := make([]string, 0, len(tsp.languages))
	for lang := range tsp.languages {
		langs = append(langs, lang)
	}
	return langs
}

// Query helpers for different language constructs

// goQueries defines Tree-sitter queries for Go
var goQueries = map[string]string{
	"functions": `
		(function_declaration
			name: (identifier) @name) @definition
	`,
	"methods": `
		(method_declaration
			receiver: (parameter_list) @receiver
			name: (field_identifier) @name) @definition
	`,
	"types": `
		(type_declaration
			(type_spec
				name: (type_identifier) @name)) @definition
	`,
	"interfaces": `
		(type_declaration
			(type_spec
				name: (type_identifier) @name
				type: (interface_type))) @definition
	`,
	"structs": `
		(type_declaration
			(type_spec
				name: (type_identifier) @name
				type: (struct_type))) @definition
	`,
}

// pythonQueries defines Tree-sitter queries for Python
var pythonQueries = map[string]string{
	"functions": `
		(function_definition
			name: (identifier) @name) @definition
	`,
	"classes": `
		(class_definition
			name: (identifier) @name) @definition
	`,
	"methods": `
		(class_definition
			body: (block
				(function_definition
					name: (identifier) @name))) @definition
	`,
}

// javascriptQueries defines Tree-sitter queries for JavaScript/TypeScript
var javascriptQueries = map[string]string{
	"functions": `
		(function_declaration
			name: (identifier) @name) @definition
	`,
	"classes": `
		(class_declaration
			name: (type_identifier) @name) @definition
	`,
	"methods": `
		(method_definition
			name: (property_identifier) @name) @definition
	`,
	"arrow_functions": `
		(variable_declarator
			name: (identifier) @name
			value: (arrow_function)) @definition
	`,
}

// javaQueries defines Tree-sitter queries for Java
var javaQueries = map[string]string{
	"classes": `
		(class_declaration
			name: (identifier) @name) @definition
	`,
	"methods": `
		(method_declaration
			name: (identifier) @name) @definition
	`,
	"interfaces": `
		(interface_declaration
			name: (identifier) @name) @definition
	`,
}

// cppQueries defines Tree-sitter queries for C/C++
var cppQueries = map[string]string{
	"functions": `
		(function_definition
			declarator: (function_declarator
				declarator: (identifier) @name)) @definition
	`,
	"classes": `
		(class_specifier
			name: (type_identifier) @name) @definition
	`,
	"structs": `
		(struct_specifier
			name: (type_identifier) @name) @definition
	`,
}

// rustQueries defines Tree-sitter queries for Rust
var rustQueries = map[string]string{
	"functions": `
		(function_item
			name: (identifier) @name) @definition
	`,
	"structs": `
		(struct_item
			name: (type_identifier) @name) @definition
	`,
	"enums": `
		(enum_item
			name: (type_identifier) @name) @definition
	`,
	"traits": `
		(trait_item
			name: (type_identifier) @name) @definition
	`,
	"impls": `
		(impl_item
			type: (type_identifier) @name) @definition
	`,
}

// rubyQueries defines Tree-sitter queries for Ruby
var rubyQueries = map[string]string{
	"methods": `
		(method
			name: (identifier) @name) @definition
	`,
	"classes": `
		(class
			name: (constant) @name) @definition
	`,
	"modules": `
		(module
			name: (constant) @name) @definition
	`,
}

// GetLanguageQueries returns the appropriate queries for a language
func GetLanguageQueries(language string) map[string]string {
	switch language {
	case "go":
		return goQueries
	case "python":
		return pythonQueries
	case "javascript", "typescript":
		return javascriptQueries
	case "java":
		return javaQueries
	case "c", "cpp":
		return cppQueries
	case "rust":
		return rustQueries
	case "ruby":
		return rubyQueries
	default:
		return nil
	}
}
