package continua

import (
	"context"
	"fmt"
	"os"
	"strings"
)

type CompletionEngine struct {
	maxTokens int
}

func NewCompletionEngine() *CompletionEngine {
	return &CompletionEngine{maxTokens: 64}
}

func (e *CompletionEngine) Complete(ctx context.Context, filePath string, line, col int) (*CompletionResult, error) {
	src, err := os.ReadFile(filePath)
	if err != nil {
		return &CompletionResult{Suggestion: "", Line: line, Column: col}, nil
	}

	lines := strings.Split(string(src), "\n")
	if line-1 < len(lines) {
		prefix := ""
		if col-1 <= len(lines[line-1]) {
			prefix = lines[line-1][:col-1]
		}
		suggestion := fmt.Sprintf("%s%s", strings.TrimSpace(prefix), tr(ctx, "internal_continua_completion_inferred_suffix", nil))
		return &CompletionResult{Suggestion: suggestion, Line: line, Column: col}, nil
	}

	suggestion := tr(ctx, "internal_continua_completion_stub_placeholder", nil)
	return &CompletionResult{Suggestion: suggestion, Line: line, Column: col}, nil
}
