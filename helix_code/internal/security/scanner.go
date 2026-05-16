package security

import (
	"context"
	"time"
)

type ScanResult struct {
	ScannerName string
	Issues      []SecurityIssue
	Score       int
	Duration    time.Duration
	RawOutput   string
	Success     bool
	ErrText     string
}

type SecurityIssue struct {
	Severity    string
	Title       string
	Description string
	FilePath    string
	LineNumber  int
	RuleID      string
}

type Scanner interface {
	Name() string
	IsAvailable(ctx context.Context) bool
	Scan(ctx context.Context, target string) (*ScanResult, error)
	Close() error
}

type ScannerConfig struct {
	SonarQubeURL   string
	SonarQubeToken string
	SnykToken      string
	Timeout        time.Duration
	MaxIssues      int
}

func calculateScore(issues []SecurityIssue) int {
	if len(issues) == 0 {
		return 100
	}
	p := 0
	for _, i := range issues {
		switch i.Severity {
		case "BLOCKER":
			p += 20
		case "CRITICAL":
			p += 10
		case "MAJOR":
			p += 5
		case "MINOR":
			p += 2
		default:
			p += 1
		}
	}
	s := 100 - p
	if s < 0 {
		return 0
	}
	return s
}
