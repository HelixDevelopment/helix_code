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
	// Guard against out-of-range line/column. line and col are 1-based and
	// flow from tool input (continua_tools.go casts JSON numbers), so an LLM
	// or user can supply 0 or negative values. Without these guards line<=0
	// indexes lines[-1] (index out of range) and col<=0 slices [: -1]
	// (slice bounds out of range) — both panic the request goroutine.
	if line >= 1 && line-1 < len(lines) {
		prefix := ""
		lineText := lines[line-1]
		// Clamp the column into [1, len(lineText)+1] so prefix is always a
		// valid 0..len slice bound, regardless of caller-supplied col.
		c := col
		if c < 1 {
			c = 1
		}
		if c-1 > len(lineText) {
			c = len(lineText) + 1
		}
		prefix = lineText[:c-1]
		suggestion := fmt.Sprintf("%s%s", strings.TrimSpace(prefix), tr(ctx, "internal_continua_completion_inferred_suffix", nil))
		return &CompletionResult{Suggestion: suggestion, Line: line, Column: col}, nil
	}

	suggestion := tr(ctx, "internal_continua_completion_stub_placeholder", nil)
	return &CompletionResult{Suggestion: suggestion, Line: line, Column: col}, nil
}
