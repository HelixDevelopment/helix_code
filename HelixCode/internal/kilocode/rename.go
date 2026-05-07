package kilocode

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

type RenameEngine struct {
	rootDir string
}

func NewRenameEngine(rootDir string) *RenameEngine {
	return &RenameEngine{rootDir: rootDir}
}

func (e *RenameEngine) Rename(ctx context.Context, oldName, newName string) (*RenameResult, error) {
	if oldName == "" || newName == "" {
		return nil, fmt.Errorf("old and new name required")
	}

	var goFiles []string
	filepath.Walk(e.rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			goFiles = append(goFiles, path)
		}
		return nil
	})

	if len(goFiles) == 0 {
		return nil, ErrSymbolNotFound
	}

	result := &RenameResult{NewName: newName}
	occurrences := 0
	filesModified := 0

	fset := token.NewFileSet()
	for _, file := range goFiles {
		src, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		f, err := parser.ParseFile(fset, file, src, 0)
		if err != nil {
			continue
		}

		fileModified := false
		ast.Inspect(f, func(n ast.Node) bool {
			ident, ok := n.(*ast.Ident)
			if !ok {
				return true
			}
			if ident.Name == oldName && ident.Obj != nil {
				occurrences++
				fileModified = true
			}
			return true
		})

		if fileModified {
			filesModified++
			newSrc := strings.ReplaceAll(string(src), oldName, newName)
			if err := os.WriteFile(file, []byte(newSrc), 0644); err != nil {
				return nil, fmt.Errorf("write %s: %w", file, err)
			}
		}
	}

	if occurrences == 0 {
		return nil, ErrSymbolNotFound
	}

	result.Occurrences = occurrences
	result.FilesModified = filesModified
	result.Symbol.Name = oldName

	return result, nil
}
