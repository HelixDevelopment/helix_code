package kilocode

import (
	"fmt"
	"os"
	"strings"
)

type Refactorer struct {
	rootDir string
}

func NewRefactorer(rootDir string) *Refactorer {
	return &Refactorer{rootDir: rootDir}
}

func (r *Refactorer) ExtractMethod(sourceFile, funcName string, startLine, endLine int) error {
	if _, err := os.Stat(sourceFile); err != nil {
		return fmt.Errorf("source file: %w", err)
	}

	src, err := os.ReadFile(sourceFile)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	lines := strings.Split(string(src), "\n")
	if startLine < 1 || endLine > len(lines) || startLine > endLine {
		return fmt.Errorf("invalid line range %d-%d (file has %d lines)", startLine, endLine, len(lines))
	}

	extracted := strings.Join(lines[startLine-1:endLine], "\n")
	newFunc := fmt.Sprintf("\nfunc %s() {\n%s\n}\n", funcName, extracted)

	indent := detectIndent(lines[startLine-1])
	replacement := indent + funcName + "()"

	for i := startLine - 1; i < endLine; i++ {
		lines[i] = ""
	}
	lines[startLine-1] = replacement

	newSrc := ""
	for _, line := range lines {
		if line == "" && newSrc == "" {
			continue
		}
		newSrc += line + "\n"
	}
	newSrc += newFunc

	return os.WriteFile(sourceFile, []byte(newSrc), 0644)
}

func (r *Refactorer) InlineCall(sourceFile, funcName string) error {
	src, err := os.ReadFile(sourceFile)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	content := string(src)
	newContent := strings.ReplaceAll(content, funcName+"()", "/* inlined "+funcName+" */")
	if newContent == content {
		return fmt.Errorf("no calls to %s() found in %s", funcName, sourceFile)
	}

	return os.WriteFile(sourceFile, []byte(newContent), 0644)
}

func detectIndent(line string) string {
	for i, c := range line {
		if c != ' ' && c != '\t' {
			return line[:i]
		}
	}
	return ""
}
