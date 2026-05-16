package roocode

import (
	"context"
	"fmt"
	"os"
	"strings"
)

type CodeReviewer struct{}

func NewCodeReviewer() *CodeReviewer {
	return &CodeReviewer{}
}

func (r *CodeReviewer) Review(ctx context.Context, filePath string) (*ReviewResult, error) {
	src, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	content := string(src)
	var issues []string
	var suggestions []string

	lines := strings.Split(content, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, "TODO") {
			issues = append(issues, fmt.Sprintf("TODO comment at line %d", i+1))
			suggestions = append(suggestions, fmt.Sprintf("Consider implementing at line %d", i+1))
		}
		if strings.Contains(trimmed, "FIXME") {
			issues = append(issues, fmt.Sprintf("FIXME marker at line %d", i+1))
		}
	}

	if len(lines) > 300 {
		issues = append(issues, "File exceeds 300 lines")
		suggestions = append(suggestions, "Consider splitting into smaller files")
	}

	approved := len(issues) == 0

	return &ReviewResult{
		File:        filePath,
		Issues:      issues,
		Suggestions: suggestions,
		Approved:    approved,
	}, nil
}
