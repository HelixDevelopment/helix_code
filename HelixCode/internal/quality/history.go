package quality

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type History struct {
	Entries []ScoreResult `json:"entries"`
	Path    string
}

func NewHistory(path string) *History {
	return &History{Path: path}
}

func (h *History) Append(r ScoreResult) error {
	h.Entries = append(h.Entries, r)
	if h.Path == "" {
		return nil
	}
	dir := filepath.Dir(h.Path)
	os.MkdirAll(dir, 0755)
	f, err := os.OpenFile(h.Path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	data, _ := json.Marshal(r)
	data = append(data, '\n')
	_, err = f.Write(data)
	return err
}

func (h *History) Average() ScoreResult {
	if len(h.Entries) == 0 {
		return ScoreResult{}
	}
	var sum ScoreResult
	var securitySum float64
	count := float64(len(h.Entries))
	for _, e := range h.Entries {
		sum.Overall += e.Overall
		sum.LintScore += e.LintScore
		sum.TestPassRate += e.TestPassRate
		securitySum += float64(e.Security)
	}
	return ScoreResult{
		Overall:      sum.Overall / count,
		LintScore:    sum.LintScore / count,
		TestPassRate: sum.TestPassRate / count,
		Security:     int(securitySum / count),
	}
}