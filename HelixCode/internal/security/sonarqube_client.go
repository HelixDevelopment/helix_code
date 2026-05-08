package security

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"time"
)

type SonarQubeScanner struct {
	cfg    ScannerConfig
	client *http.Client
}

func NewSonarQubeScanner(cfg ScannerConfig) *SonarQubeScanner {
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	return &SonarQubeScanner{cfg: cfg, client: &http.Client{Timeout: cfg.Timeout}}
}

func (s *SonarQubeScanner) Name() string { return "SonarQube" }

func (s *SonarQubeScanner) IsAvailable(ctx context.Context) bool {
	if s.cfg.SonarQubeURL == "" || s.cfg.SonarQubeToken == "" {
		return false
	}
	req, err := http.NewRequestWithContext(ctx, "GET", s.cfg.SonarQubeURL+"/api/system/health", nil)
	if err != nil {
		return false
	}
	req.SetBasicAuth(s.cfg.SonarQubeToken, "")
	resp, err := s.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func (s *SonarQubeScanner) Scan(ctx context.Context, target string) (*ScanResult, error) {
	start := time.Now()
	cmd := exec.CommandContext(ctx, "sonar-scanner",
		"-Dsonar.host.url="+s.cfg.SonarQubeURL,
		"-Dsonar.login="+s.cfg.SonarQubeToken,
		"-Dsonar.projectKey="+target,
		"-Dsonar.sources=.",
	)
	output, err := cmd.CombinedOutput()
	result := &ScanResult{
		ScannerName: s.Name(),
		Duration:    time.Since(start),
		RawOutput:   string(output),
		Success:     err == nil,
	}
	if err != nil {
		result.ErrText = fmt.Sprintf("sonar-scanner: %v", err)
		result.Score = 100
		return result, nil
	}
	issues, err := s.fetchIssues(ctx, target)
	if err != nil {
		result.ErrText = err.Error()
		result.Score = 100
		return result, nil
	}
	result.Issues = issues
	result.Score = calculateScore(issues)
	return result, nil
}

func (s *SonarQubeScanner) fetchIssues(ctx context.Context, projectKey string) ([]SecurityIssue, error) {
	max := s.cfg.MaxIssues
	if max == 0 {
		max = 100
	}
	url := fmt.Sprintf("%s/api/issues/search?componentKeys=%s&statuses=OPEN&ps=%d",
		s.cfg.SonarQubeURL, projectKey, max)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(s.cfg.SonarQubeToken, "")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var sr struct {
		Issues []struct {
			Severity  string `json:"severity"`
			Message   string `json:"message"`
			Rule      string `json:"rule"`
			Component string `json:"component"`
			Line      int    `json:"line"`
		} `json:"issues"`
	}
	if err := json.Unmarshal(body, &sr); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	issues := make([]SecurityIssue, 0, len(sr.Issues))
	for _, i := range sr.Issues {
		issues = append(issues, SecurityIssue{
			Severity:    i.Severity,
			Title:       i.Message,
			Description: i.Message,
			FilePath:    i.Component,
			LineNumber:  i.Line,
			RuleID:      i.Rule,
		})
	}
	return issues, nil
}

func (s *SonarQubeScanner) Close() error {
	s.client.CloseIdleConnections()
	return nil
}
