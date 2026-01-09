package mapping

import (
	"fmt"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/bash"
	"github.com/smacker/go-tree-sitter/c"
	"github.com/smacker/go-tree-sitter/cpp"
	"github.com/smacker/go-tree-sitter/csharp"
	"github.com/smacker/go-tree-sitter/css"
	"github.com/smacker/go-tree-sitter/dockerfile"
	"github.com/smacker/go-tree-sitter/elixir"
	"github.com/smacker/go-tree-sitter/elm"
	"github.com/smacker/go-tree-sitter/golang"
	"github.com/smacker/go-tree-sitter/hcl"
	"github.com/smacker/go-tree-sitter/html"
	"github.com/smacker/go-tree-sitter/java"
	"github.com/smacker/go-tree-sitter/javascript"
	"github.com/smacker/go-tree-sitter/lua"
	"github.com/smacker/go-tree-sitter/ocaml"
	"github.com/smacker/go-tree-sitter/php"
	"github.com/smacker/go-tree-sitter/protobuf"
	"github.com/smacker/go-tree-sitter/python"
	"github.com/smacker/go-tree-sitter/ruby"
	"github.com/smacker/go-tree-sitter/rust"
	"github.com/smacker/go-tree-sitter/scala"
	"github.com/smacker/go-tree-sitter/svelte"
	"github.com/smacker/go-tree-sitter/swift"
	"github.com/smacker/go-tree-sitter/toml"
	"github.com/smacker/go-tree-sitter/typescript/tsx"
	"github.com/smacker/go-tree-sitter/typescript/typescript"
	"github.com/smacker/go-tree-sitter/yaml"
)

// TreeSitterLanguageParser implements LanguageParser using tree-sitter
type TreeSitterLanguageParser struct {
	language *sitter.Language
	name     string
	queries  *LanguageQueries
	parser   *sitter.Parser
}

// NewTreeSitterLanguageParser creates a new tree-sitter based language parser
func NewTreeSitterLanguageParser(lang *sitter.Language, name string, queries *LanguageQueries) *TreeSitterLanguageParser {
	parser := sitter.NewParser()
	parser.SetLanguage(lang)

	return &TreeSitterLanguageParser{
		language: lang,
		name:     name,
		queries:  queries,
		parser:   parser,
	}
}

// Parse parses source code into a ParsedTree using tree-sitter
func (p *TreeSitterLanguageParser) Parse(source []byte) (*ParsedTree, error) {
	tree, err := p.parser.ParseCtx(nil, nil, source)
	if err != nil {
		return nil, fmt.Errorf("tree-sitter parse failed: %w", err)
	}
	defer tree.Close()

	// Convert tree-sitter tree to our ParsedTree format
	root := tree.RootNode()
	parsedTree := &ParsedTree{
		Language:    p.name,
		Source:      source,
		Root:        convertNode(root, nil, source),
		ParseErrors: extractParseErrors(root, source),
	}

	return parsedTree, nil
}

// ExtractDefinitions extracts definitions from a parsed tree
func (p *TreeSitterLanguageParser) ExtractDefinitions(tree *ParsedTree) ([]*Definition, error) {
	var definitions []*Definition

	// Language-specific definition extraction
	switch p.name {
	case "go":
		definitions = p.extractGoDefinitions(tree)
	case "javascript", "typescript":
		definitions = p.extractJSDefinitions(tree)
	case "python":
		definitions = p.extractPythonDefinitions(tree)
	case "java":
		definitions = p.extractJavaDefinitions(tree)
	case "rust":
		definitions = p.extractRustDefinitions(tree)
	default:
		definitions = p.extractGenericDefinitions(tree)
	}

	return definitions, nil
}

// ExtractImports extracts imports from a parsed tree
func (p *TreeSitterLanguageParser) ExtractImports(tree *ParsedTree) ([]*Import, error) {
	var imports []*Import

	switch p.name {
	case "go":
		imports = p.extractGoImports(tree)
	case "javascript", "typescript":
		imports = p.extractJSImports(tree)
	case "python":
		imports = p.extractPythonImports(tree)
	case "java":
		imports = p.extractJavaImports(tree)
	case "rust":
		imports = p.extractRustImports(tree)
	default:
		imports = p.extractGenericImports(tree)
	}

	return imports, nil
}

// CalculateComplexity calculates cyclomatic complexity
func (p *TreeSitterLanguageParser) CalculateComplexity(tree *ParsedTree) int {
	complexity := 1 // Base complexity

	// Count decision points
	decisionTypes := []string{
		"if_statement", "if_expression",
		"for_statement", "for_expression",
		"while_statement", "while_expression",
		"switch_statement", "switch_expression", "match_expression",
		"case_clause", "case",
		"catch_clause", "except_clause",
		"conditional_expression", "ternary_expression",
		"binary_expression", // For && and ||
	}

	for _, nodeType := range decisionTypes {
		nodes := tree.Query(nodeType)
		complexity += len(nodes)
	}

	return complexity
}

// GetQueries returns tree-sitter queries for this language
func (p *TreeSitterLanguageParser) GetQueries() *LanguageQueries {
	return p.queries
}

// convertNode converts a tree-sitter node to our Node format
func convertNode(node *sitter.Node, parent *Node, source []byte) *Node {
	if node == nil {
		return nil
	}

	n := &Node{
		Type:      node.Type(),
		StartByte: int(node.StartByte()),
		EndByte:   int(node.EndByte()),
		StartPoint: Point{
			Row:    int(node.StartPoint().Row),
			Column: int(node.StartPoint().Column),
		},
		EndPoint: Point{
			Row:    int(node.EndPoint().Row),
			Column: int(node.EndPoint().Column),
		},
		Parent: parent,
	}

	// Store text for small nodes (identifiers, literals, etc.)
	if n.EndByte-n.StartByte < 100 {
		n.Text = string(source[n.StartByte:n.EndByte])
	}

	// Convert children
	childCount := int(node.ChildCount())
	if childCount > 0 {
		n.Children = make([]*Node, 0, childCount)
		for i := 0; i < childCount; i++ {
			child := node.Child(i)
			if child != nil {
				n.Children = append(n.Children, convertNode(child, n, source))
			}
		}
	}

	return n
}

// extractParseErrors extracts parse errors from a tree-sitter tree
func extractParseErrors(root *sitter.Node, source []byte) []*ParseError {
	var errors []*ParseError

	// Walk tree looking for ERROR nodes
	var walk func(*sitter.Node)
	walk = func(node *sitter.Node) {
		if node == nil {
			return
		}

		if node.Type() == "ERROR" || node.IsMissing() {
			errors = append(errors, &ParseError{
				Message:   fmt.Sprintf("Parse error: unexpected %s", node.Type()),
				StartByte: int(node.StartByte()),
				EndByte:   int(node.EndByte()),
				StartLine: int(node.StartPoint().Row) + 1,
				EndLine:   int(node.EndPoint().Row) + 1,
			})
		}

		childCount := int(node.ChildCount())
		for i := 0; i < childCount; i++ {
			walk(node.Child(i))
		}
	}

	walk(root)
	return errors
}

// Language-specific extraction methods

func (p *TreeSitterLanguageParser) extractGoDefinitions(tree *ParsedTree) []*Definition {
	var definitions []*Definition

	// Extract function declarations
	for _, node := range tree.Query("function_declaration") {
		nameNode := node.FindChild("identifier")
		if nameNode != nil {
			definitions = append(definitions, &Definition{
				Name:      nameNode.Text,
				Type:      DefFunction,
				StartLine: node.StartPoint.Row + 1,
				EndLine:   node.EndPoint.Row + 1,
			})
		}
	}

	// Extract method declarations
	for _, node := range tree.Query("method_declaration") {
		nameNode := node.FindChild("field_identifier")
		if nameNode != nil {
			definitions = append(definitions, &Definition{
				Name:      nameNode.Text,
				Type:      DefMethod,
				StartLine: node.StartPoint.Row + 1,
				EndLine:   node.EndPoint.Row + 1,
			})
		}
	}

	// Extract type declarations (structs, interfaces)
	for _, node := range tree.Query("type_declaration") {
		for _, spec := range node.FindChildren("type_spec") {
			nameNode := spec.FindChild("type_identifier")
			if nameNode != nil {
				defType := DefType
				if spec.FindChild("struct_type") != nil {
					defType = DefStruct
				} else if spec.FindChild("interface_type") != nil {
					defType = DefInterface
				}
				definitions = append(definitions, &Definition{
					Name:      nameNode.Text,
					Type:      defType,
					StartLine: spec.StartPoint.Row + 1,
					EndLine:   spec.EndPoint.Row + 1,
				})
			}
		}
	}

	return definitions
}

func (p *TreeSitterLanguageParser) extractJSDefinitions(tree *ParsedTree) []*Definition {
	var definitions []*Definition

	// Function declarations
	for _, node := range tree.Query("function_declaration") {
		nameNode := node.FindChild("identifier")
		if nameNode != nil {
			definitions = append(definitions, &Definition{
				Name:      nameNode.Text,
				Type:      DefFunction,
				StartLine: node.StartPoint.Row + 1,
				EndLine:   node.EndPoint.Row + 1,
			})
		}
	}

	// Arrow functions assigned to variables
	for _, node := range tree.Query("variable_declarator") {
		if node.FindChild("arrow_function") != nil || node.FindChild("function") != nil {
			nameNode := node.FindChild("identifier")
			if nameNode != nil {
				definitions = append(definitions, &Definition{
					Name:      nameNode.Text,
					Type:      DefFunction,
					StartLine: node.StartPoint.Row + 1,
					EndLine:   node.EndPoint.Row + 1,
				})
			}
		}
	}

	// Class declarations
	for _, node := range tree.Query("class_declaration") {
		nameNode := node.FindChild("identifier")
		if nameNode != nil {
			definitions = append(definitions, &Definition{
				Name:      nameNode.Text,
				Type:      DefClass,
				StartLine: node.StartPoint.Row + 1,
				EndLine:   node.EndPoint.Row + 1,
			})
		}
	}

	return definitions
}

func (p *TreeSitterLanguageParser) extractPythonDefinitions(tree *ParsedTree) []*Definition {
	var definitions []*Definition

	// Function definitions
	for _, node := range tree.Query("function_definition") {
		nameNode := node.FindChild("identifier")
		if nameNode != nil {
			definitions = append(definitions, &Definition{
				Name:      nameNode.Text,
				Type:      DefFunction,
				StartLine: node.StartPoint.Row + 1,
				EndLine:   node.EndPoint.Row + 1,
			})
		}
	}

	// Class definitions
	for _, node := range tree.Query("class_definition") {
		nameNode := node.FindChild("identifier")
		if nameNode != nil {
			definitions = append(definitions, &Definition{
				Name:      nameNode.Text,
				Type:      DefClass,
				StartLine: node.StartPoint.Row + 1,
				EndLine:   node.EndPoint.Row + 1,
			})
		}
	}

	return definitions
}

func (p *TreeSitterLanguageParser) extractJavaDefinitions(tree *ParsedTree) []*Definition {
	var definitions []*Definition

	// Class declarations
	for _, node := range tree.Query("class_declaration") {
		nameNode := node.FindChild("identifier")
		if nameNode != nil {
			definitions = append(definitions, &Definition{
				Name:      nameNode.Text,
				Type:      DefClass,
				StartLine: node.StartPoint.Row + 1,
				EndLine:   node.EndPoint.Row + 1,
			})
		}
	}

	// Interface declarations
	for _, node := range tree.Query("interface_declaration") {
		nameNode := node.FindChild("identifier")
		if nameNode != nil {
			definitions = append(definitions, &Definition{
				Name:      nameNode.Text,
				Type:      DefInterface,
				StartLine: node.StartPoint.Row + 1,
				EndLine:   node.EndPoint.Row + 1,
			})
		}
	}

	// Method declarations
	for _, node := range tree.Query("method_declaration") {
		nameNode := node.FindChild("identifier")
		if nameNode != nil {
			definitions = append(definitions, &Definition{
				Name:      nameNode.Text,
				Type:      DefMethod,
				StartLine: node.StartPoint.Row + 1,
				EndLine:   node.EndPoint.Row + 1,
			})
		}
	}

	return definitions
}

func (p *TreeSitterLanguageParser) extractRustDefinitions(tree *ParsedTree) []*Definition {
	var definitions []*Definition

	// Function items
	for _, node := range tree.Query("function_item") {
		nameNode := node.FindChild("identifier")
		if nameNode != nil {
			definitions = append(definitions, &Definition{
				Name:      nameNode.Text,
				Type:      DefFunction,
				StartLine: node.StartPoint.Row + 1,
				EndLine:   node.EndPoint.Row + 1,
			})
		}
	}

	// Struct items
	for _, node := range tree.Query("struct_item") {
		nameNode := node.FindChild("type_identifier")
		if nameNode != nil {
			definitions = append(definitions, &Definition{
				Name:      nameNode.Text,
				Type:      DefStruct,
				StartLine: node.StartPoint.Row + 1,
				EndLine:   node.EndPoint.Row + 1,
			})
		}
	}

	// Trait items
	for _, node := range tree.Query("trait_item") {
		nameNode := node.FindChild("type_identifier")
		if nameNode != nil {
			definitions = append(definitions, &Definition{
				Name:      nameNode.Text,
				Type:      DefInterface,
				StartLine: node.StartPoint.Row + 1,
				EndLine:   node.EndPoint.Row + 1,
			})
		}
	}

	// Impl blocks
	for _, node := range tree.Query("impl_item") {
		nameNode := node.FindChild("type_identifier")
		if nameNode != nil {
			definitions = append(definitions, &Definition{
				Name:      nameNode.Text,
				Type:      DefStruct,
				StartLine: node.StartPoint.Row + 1,
				EndLine:   node.EndPoint.Row + 1,
			})
		}
	}

	return definitions
}

func (p *TreeSitterLanguageParser) extractGenericDefinitions(tree *ParsedTree) []*Definition {
	var definitions []*Definition

	// Look for common definition patterns
	definitionTypes := []string{
		"function_definition", "function_declaration", "function_item",
		"method_definition", "method_declaration",
		"class_definition", "class_declaration",
		"interface_declaration", "trait_item",
		"struct_item", "struct_declaration",
	}

	for _, defType := range definitionTypes {
		for _, node := range tree.Query(defType) {
			// Try to find an identifier child
			nameNode := node.FindChild("identifier")
			if nameNode == nil {
				nameNode = node.FindChild("type_identifier")
			}
			if nameNode == nil {
				nameNode = node.FindChild("field_identifier")
			}

			if nameNode != nil {
				definitions = append(definitions, &Definition{
					Name:      nameNode.Text,
					Type:      DefFunction, // Generic fallback
					StartLine: node.StartPoint.Row + 1,
					EndLine:   node.EndPoint.Row + 1,
				})
			}
		}
	}

	return definitions
}

// Import extraction methods

func (p *TreeSitterLanguageParser) extractGoImports(tree *ParsedTree) []*Import {
	var imports []*Import

	for _, node := range tree.Query("import_declaration") {
		for _, spec := range node.FindChildren("import_spec") {
			pathNode := spec.FindChild("interpreted_string_literal")
			if pathNode != nil {
				path := pathNode.Text
				// Remove quotes
				if len(path) > 2 {
					path = path[1 : len(path)-1]
				}
				imports = append(imports, &Import{
					Path:      path,
					StartLine: spec.StartPoint.Row + 1,
				})
			}
		}
	}

	return imports
}

func (p *TreeSitterLanguageParser) extractJSImports(tree *ParsedTree) []*Import {
	var imports []*Import

	for _, node := range tree.Query("import_statement") {
		sourceNode := node.FindChild("string")
		if sourceNode != nil {
			path := sourceNode.Text
			// Remove quotes
			if len(path) > 2 {
				path = path[1 : len(path)-1]
			}
			imports = append(imports, &Import{
				Path: path,
				StartLine: node.StartPoint.Row + 1,
			})
		}
	}

	return imports
}

func (p *TreeSitterLanguageParser) extractPythonImports(tree *ParsedTree) []*Import {
	var imports []*Import

	// import statements
	for _, node := range tree.Query("import_statement") {
		nameNode := node.FindChild("dotted_name")
		if nameNode != nil {
			imports = append(imports, &Import{
				Path: nameNode.Text,
				StartLine: node.StartPoint.Row + 1,
			})
		}
	}

	// from imports
	for _, node := range tree.Query("import_from_statement") {
		nameNode := node.FindChild("dotted_name")
		if nameNode != nil {
			imports = append(imports, &Import{
				Path: nameNode.Text,
				StartLine: node.StartPoint.Row + 1,
			})
		}
	}

	return imports
}

func (p *TreeSitterLanguageParser) extractJavaImports(tree *ParsedTree) []*Import {
	var imports []*Import

	for _, node := range tree.Query("import_declaration") {
		// Get the full import path
		text := node.GetText(tree.Source)
		imports = append(imports, &Import{
			Path: text,
			StartLine: node.StartPoint.Row + 1,
		})
	}

	return imports
}

func (p *TreeSitterLanguageParser) extractRustImports(tree *ParsedTree) []*Import {
	var imports []*Import

	for _, node := range tree.Query("use_declaration") {
		text := node.GetText(tree.Source)
		imports = append(imports, &Import{
			Path: text,
			StartLine: node.StartPoint.Row + 1,
		})
	}

	return imports
}

func (p *TreeSitterLanguageParser) extractGenericImports(tree *ParsedTree) []*Import {
	var imports []*Import

	importTypes := []string{
		"import_statement", "import_declaration",
		"import_from_statement", "use_declaration",
		"require_expression",
	}

	for _, importType := range importTypes {
		for _, node := range tree.Query(importType) {
			text := node.GetText(tree.Source)
			imports = append(imports, &Import{
				Path: text,
				StartLine: node.StartPoint.Row + 1,
			})
		}
	}

	return imports
}

// GetTreeSitterLanguage returns the tree-sitter language for a given language name
func GetTreeSitterLanguage(name string) (*sitter.Language, error) {
	switch name {
	case "go", "golang":
		return golang.GetLanguage(), nil
	case "javascript", "js":
		return javascript.GetLanguage(), nil
	case "typescript", "ts":
		return typescript.GetLanguage(), nil
	case "tsx":
		return tsx.GetLanguage(), nil
	case "python", "py":
		return python.GetLanguage(), nil
	case "rust":
		return rust.GetLanguage(), nil
	case "java":
		return java.GetLanguage(), nil
	case "c":
		return c.GetLanguage(), nil
	case "cpp", "c++":
		return cpp.GetLanguage(), nil
	case "csharp", "c#", "cs":
		return csharp.GetLanguage(), nil
	case "ruby", "rb":
		return ruby.GetLanguage(), nil
	case "php":
		return php.GetLanguage(), nil
	case "swift":
		return swift.GetLanguage(), nil
	case "scala":
		return scala.GetLanguage(), nil
	case "elixir":
		return elixir.GetLanguage(), nil
	case "lua":
		return lua.GetLanguage(), nil
	case "ocaml":
		return ocaml.GetLanguage(), nil
	case "elm":
		return elm.GetLanguage(), nil
	case "bash", "sh":
		return bash.GetLanguage(), nil
	case "html":
		return html.GetLanguage(), nil
	case "css":
		return css.GetLanguage(), nil
	case "yaml":
		return yaml.GetLanguage(), nil
	case "toml":
		return toml.GetLanguage(), nil
	case "dockerfile":
		return dockerfile.GetLanguage(), nil
	case "protobuf", "proto":
		return protobuf.GetLanguage(), nil
	case "hcl", "terraform":
		return hcl.GetLanguage(), nil
	case "svelte":
		return svelte.GetLanguage(), nil
	default:
		return nil, fmt.Errorf("unsupported tree-sitter language: %s", name)
	}
}

// RegisterTreeSitterParsers registers all available tree-sitter parsers with the registry
func RegisterTreeSitterParsers(registry *DefaultLanguageRegistry) error {
	languages := []string{
		"go", "javascript", "typescript", "python", "rust",
		"java", "c", "cpp", "csharp", "ruby", "php",
		"swift", "scala", "elixir", "lua", "ocaml", "elm",
		"bash", "html", "css", "yaml", "toml", "dockerfile",
		"protobuf", "hcl", "svelte",
	}

	for _, lang := range languages {
		tsLang, err := GetTreeSitterLanguage(lang)
		if err != nil {
			continue // Skip unsupported languages
		}

		parser := NewTreeSitterLanguageParser(tsLang, lang, getDefaultQueries(lang))
		if err := registry.Register(lang, parser); err != nil {
			return fmt.Errorf("failed to register %s parser: %w", lang, err)
		}
	}

	return nil
}

// getDefaultQueries returns default tree-sitter queries for a language
func getDefaultQueries(lang string) *LanguageQueries {
	// These are simplified queries - in production you'd load proper tree-sitter query files
	return &LanguageQueries{
		Functions: fmt.Sprintf("(function_declaration) @function (function_definition) @function"),
		Classes:   fmt.Sprintf("(class_declaration) @class (class_definition) @class"),
		Methods:   fmt.Sprintf("(method_declaration) @method (method_definition) @method"),
		Imports:   fmt.Sprintf("(import_statement) @import (import_declaration) @import"),
		Exports:   fmt.Sprintf("(export_statement) @export"),
		Comments:  fmt.Sprintf("(comment) @comment"),
	}
}
