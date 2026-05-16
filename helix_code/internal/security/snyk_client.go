package security

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"time"
)

type SnykScanner struct {
	cfg ScannerConfig
}

func NewSnykScanner(cfg ScannerConfig) *SnykScanner {
	return &SnykScanner{cfg: cfg}
}

func (s *SnykScanner) Name() string { return "Snyk" }

func (s *SnykScanner) IsAvailable(ctx context.Context) bool {
	if s.cfg.SnykToken == "" {
		return false
	}
	_, err := exec.LookPath("snyk")
	return err == nil
}

func (s *SnykScanner) Scan(ctx context.Context, target string) (*ScanResult, error) {
	start := time.Now()
	cmd := exec.CommandContext(ctx, "snyk", "code", "test", "--json", "--severity-threshold=low")
	cmd.Env = append(cmd.Environ(), "SNYK_TOKEN="+s.cfg.SnykToken)
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	err := cmd.Run()
	output := out.String()
	if output == "" {
		output = errOut.String()
	}
	result := &ScanResult{
		ScannerName: s.Name(),
		Duration:    time.Since(start),
		RawOutput:   output,
		Success:     err == nil,
	}
	if output == "" {
		result.Score = 100
		return result, nil
	}
	type snykIssue struct {
		Issue struct {
			Severity string `json:"severity"`
			Title    string `json:"title"`
			Message  string `json:"message"`
		} `json:"issue"`
		Position struct {
			FilePath string `json:"filePath"`
			Line     int    `json:"line"`
		} `json:"position"`
		RuleID string `json:"ruleId"`
	}
	var snykResp struct {
		Results []snykIssue `json:"runs"`
	}
	if err := json.Unmarshal([]byte(output), &snykResp); err != nil {
		result.Score = 100
		return result, nil
	}
	issues := make([]SecurityIssue, 0, len(snykResp.Results))
	for _, r := range snykResp.Results {
		issues = append(issues, SecurityIssue{
			Severity:    r.Issue.Severity,
			Title:       r.Issue.Title,
			Description: r.Issue.Message,
			FilePath:    r.Position.FilePath,
			LineNumber:  r.Position.Line,
			RuleID:      r.RuleID,
		})
	}
	result.Issues = issues
	result.Score = calculateScore(issues)
	return result, nil
}

func (s *SnykScanner) Close() error { return nil }
