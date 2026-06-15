package roocode

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrPathTraversal is returned when a caller-supplied name or output
// directory would resolve to a filesystem path outside the generator's
// configured outputDir. Roo-code-driven generation MUST stay sandboxed
// inside outputDir: spec.Name / spec.OutputDir are external (LLM- or
// user-supplied) inputs, so a ".."-bearing or absolute value that
// escaped the base would let an external agent plant or overwrite files
// anywhere on the host (a real path-traversal write primitive).
var ErrPathTraversal = errors.New("roocode: resolved path escapes output directory")

type CodeGenerator struct {
	outputDir string
}

func NewCodeGenerator(outputDir string) *CodeGenerator {
	return &CodeGenerator{outputDir: outputDir}
}

// withinBase reports the cleaned absolute path of candidate when (and
// only when) it resolves inside base; otherwise it returns
// ErrPathTraversal. Comparison is done on absolute, lexically-cleaned
// paths with a trailing separator on base so that a sibling sharing a
// prefix (e.g. base "out", candidate "outside") cannot masquerade as
// being within base.
//
// NOTE (§11.4.6 honest boundary): containment is LEXICAL (filepath.Abs /
// filepath.Clean), not symlink-resolved. A symlink planted INSIDE base that
// points outside it would pass this check and the write would land at the
// link target. base (the generator's outputDir) is agent-configured, not
// attacker-controlled, so this is low-risk here; a caller needing
// symlink-proof containment must filepath.EvalSymlinks the candidate before
// withinBase (as internal/kilocode's resolveWithinRoot does).
func withinBase(base, candidate string) (string, error) {
	absBase, err := filepath.Abs(base)
	if err != nil {
		return "", fmt.Errorf("resolve base dir: %w", err)
	}
	absCand, err := filepath.Abs(candidate)
	if err != nil {
		return "", fmt.Errorf("resolve candidate path: %w", err)
	}
	if absCand != absBase && !strings.HasPrefix(absCand, absBase+string(os.PathSeparator)) {
		return "", fmt.Errorf("%w: %q not within %q", ErrPathTraversal, absCand, absBase)
	}
	return absCand, nil
}

func (g *CodeGenerator) Generate(ctx context.Context, spec GenerateSpec) (string, error) {
	if err := os.MkdirAll(g.outputDir, 0755); err != nil {
		return "", fmt.Errorf("create output dir: %w", err)
	}

	content := buildContent(spec)
	fileName := spec.Name + fileExtension(spec.Type)
	filePath, err := withinBase(g.outputDir, filepath.Join(g.outputDir, fileName))
	if err != nil {
		return "", err
	}

	// The cleaned filePath may sit in a (validated, within-base)
	// subdirectory if spec.Name contained separators — ensure it exists.
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return "", fmt.Errorf("create file dir: %w", err)
	}

	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("write file: %w", err)
	}

	return filePath, nil
}

func (g *CodeGenerator) Bootstrap(ctx context.Context, spec BootstrapSpec) ([]string, error) {
	dir := spec.OutputDir
	if dir == "" {
		dir = filepath.Join(g.outputDir, spec.Name)
	}
	// spec.OutputDir / spec.Name are external inputs — confine the
	// resolved project directory to outputDir before any mkdir/write.
	dir, err := withinBase(g.outputDir, dir)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create project dir: %w", err)
	}

	var files []string

	switch spec.ProjectType {
	case "go":
		mainFile := filepath.Join(dir, "main.go")
		content := fmt.Sprintf("package main\n\nfunc main() {\n\t// %s\n}\n", spec.Name)
		os.WriteFile(mainFile, []byte(content), 0644)
		files = append(files, mainFile)

		goMod := filepath.Join(dir, "go.mod")
		os.WriteFile(goMod, []byte(fmt.Sprintf("module %s\n\ngo 1.24\n", spec.Name)), 0644)
		files = append(files, goMod)

	case "python":
		mainFile := filepath.Join(dir, "main.py")
		content := fmt.Sprintf("#!/usr/bin/env python3\n\"\"\"%s\"\"\"\n\ndef main():\n    pass\n\nif __name__ == \"__main__\":\n    main()\n", spec.Name)
		os.WriteFile(mainFile, []byte(content), 0644)
		files = append(files, mainFile)

	case "node":
		mainFile := filepath.Join(dir, "index.js")
		content := fmt.Sprintf("// %s\n\nfunction main() {\n  console.log(\"Hello\");\n}\n\nmain();\n", spec.Name)
		os.WriteFile(mainFile, []byte(content), 0644)
		files = append(files, mainFile)
	}

	return files, nil
}

func buildContent(spec GenerateSpec) string {
	var b strings.Builder

	switch spec.Type {
	case "go":
		b.WriteString(fmt.Sprintf("package %s\n\n", spec.Template))
		if spec.Prompt != "" {
			b.WriteString(fmt.Sprintf("// %s\n", spec.Prompt))
		}
		b.WriteString(fmt.Sprintf("func %s() {\n\t// TODO: implement\n}\n", spec.Name))
	case "python":
		if spec.Prompt != "" {
			b.WriteString(fmt.Sprintf("# %s\n", spec.Prompt))
		}
		b.WriteString(fmt.Sprintf("def %s():\n    pass\n", spec.Name))
	case "js":
		if spec.Prompt != "" {
			b.WriteString(fmt.Sprintf("// %s\n", spec.Prompt))
		}
		b.WriteString(fmt.Sprintf("function %s() {\n  // TODO: implement\n}\n", spec.Name))
	default:
		b.WriteString(fmt.Sprintf("// %s - generated by Roo-code\n", spec.Name))
	}
	return b.String()
}

func fileExtension(lang string) string {
	switch lang {
	case "go":
		return ".go"
	case "python":
		return ".py"
	case "js", "node":
		return ".js"
	default:
		return ".txt"
	}
}
