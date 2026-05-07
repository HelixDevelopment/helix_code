package continua

import (
	"context"
	"fmt"
	"os"
	"strings"
)

type WorkspaceEditor struct{}

func NewWorkspaceEditor() *WorkspaceEditor { return &WorkspaceEditor{} }

func (e *WorkspaceEditor) Open(ctx context.Context, filePath string) (*EditResult, error) {
	src, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	lines := strings.Split(strings.TrimRight(string(src), "\n"), "\n")
	return &EditResult{FilePath: filePath, Content: string(src), Lines: len(lines)}, nil
}

func (e *WorkspaceEditor) Edit(ctx context.Context, filePath, content string) (*EditResult, error) {
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return nil, fmt.Errorf("edit file: %w", err)
	}
	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	return &EditResult{FilePath: filePath, Content: content, Lines: len(lines)}, nil
}

func (e *WorkspaceEditor) Save(ctx context.Context, filePath string, content []string) (*EditResult, error) {
	joined := strings.Join(content, "\n") + "\n"
	return e.Edit(ctx, filePath, joined)
}
