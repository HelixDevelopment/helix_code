package kilocode

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

func BuildCallGraph(rootDir string) (*CallGraph, error) {
	cg := &CallGraph{
		Nodes: make(map[string]SymbolRef),
	}

	var goFiles []string
	filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			goFiles = append(goFiles, path)
		}
		return nil
	})

	fset := token.NewFileSet()
	for _, file := range goFiles {
		f, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			continue
		}

		pkgName := f.Name.Name

		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}

			funcName := fn.Name.Name
			if fn.Recv != nil && len(fn.Recv.List) > 0 {
				recvType := typeExprString(fn.Recv.List[0].Type)
				funcName = recvType + "." + funcName
			} else {
				funcName = pkgName + "." + funcName
			}

			pos := fset.Position(fn.Pos())
			ref := SymbolRef{
				Name:     funcName,
				Kind:     KindFunction,
				FilePath: file,
				Line:     pos.Line,
				Column:   pos.Column,
			}
			cg.Nodes[funcName] = ref

			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				calleeName := extractCalleeName(call)
				if calleeName != "" {
					cg.Edges = append(cg.Edges, CallEdge{
						Caller: ref,
						Callee: SymbolRef{Name: calleeName, FilePath: file},
					})
				}
				return true
			})
		}
	}

	return cg, nil
}

func (cg *CallGraph) FindCallers(symbolName string) []SymbolRef {
	var callers []SymbolRef
	seen := make(map[string]bool)
	for _, edge := range cg.Edges {
		if edge.Callee.Name == symbolName && !seen[edge.Caller.Name] {
			callers = append(callers, edge.Caller)
			seen[edge.Caller.Name] = true
		}
	}
	return callers
}

func (cg *CallGraph) FindCallees(symbolName string) []SymbolRef {
	var callees []SymbolRef
	seen := make(map[string]bool)
	for _, edge := range cg.Edges {
		if edge.Caller.Name == symbolName && !seen[edge.Callee.Name] {
			callees = append(callees, edge.Callee)
			seen[edge.Callee.Name] = true
		}
	}
	return callees
}

func (cg *CallGraph) NodeCount() int {
	return len(cg.Nodes)
}

func (cg *CallGraph) EdgeCount() int {
	return len(cg.Edges)
}

func extractCalleeName(call *ast.CallExpr) string {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		return fun.Name
	case *ast.SelectorExpr:
		if x, ok := fun.X.(*ast.Ident); ok {
			return x.Name + "." + fun.Sel.Name
		}
		return fun.Sel.Name
	}
	return ""
}

func typeExprString(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.StarExpr:
		return "*" + typeExprString(e.X)
	default:
		return fmt.Sprintf("%T", expr)
	}
}
