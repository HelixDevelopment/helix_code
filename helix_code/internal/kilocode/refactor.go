package kilocode

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Refactorer struct {
	rootDir string
}

func NewRefactorer(rootDir string) *Refactorer {
	return &Refactorer{rootDir: rootDir}
}

// resolveWithinRoot validates that sourceFile resolves to a location
// inside the Refactorer's rootDir and returns the cleaned absolute path.
// Any path that escapes the root (via "..", an absolute path pointing
// elsewhere, or a symlink traversal) is rejected with ErrPathOutsideRoot.
// Without this guard a caller-supplied "file" param could read/overwrite
// arbitrary files on disk (path traversal). rootDir == "" means no root
// is configured, which we treat as a hard rejection rather than an open
// door (a refactorer with no scope must not write anywhere).
func (r *Refactorer) resolveWithinRoot(sourceFile string) (string, error) {
	if r.rootDir == "" {
		return "", ErrPathOutsideRoot
	}
	rootAbs, err := filepath.Abs(r.rootDir)
	if err != nil {
		return "", fmt.Errorf("resolve root: %w", err)
	}
	// Resolve symlinks on the root so the prefix comparison is against
	// the real directory (e.g. macOS /var -> /private/var).
	if resolved, err := filepath.EvalSymlinks(rootAbs); err == nil {
		rootAbs = resolved
	}

	target := sourceFile
	if !filepath.IsAbs(target) {
		target = filepath.Join(rootAbs, target)
	}
	target = filepath.Clean(target)
	// If the target exists, resolve its symlinks too so a symlinked file
	// inside the root that points outside is caught.
	if resolved, err := filepath.EvalSymlinks(target); err == nil {
		target = resolved
	}

	rel, err := filepath.Rel(rootAbs, target)
	if err != nil {
		return "", ErrPathOutsideRoot
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", ErrPathOutsideRoot
	}
	return target, nil
}

func (r *Refactorer) ExtractMethod(sourceFile, funcName string, startLine, endLine int) error {
	sourceFile, err := r.resolveWithinRoot(sourceFile)
	if err != nil {
		return fmt.Errorf("source file: %w", err)
	}
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
	sourceFile, err := r.resolveWithinRoot(sourceFile)
	if err != nil {
		return fmt.Errorf("source file: %w", err)
	}
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
