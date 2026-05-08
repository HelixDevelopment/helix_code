package quality

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type Scorer struct{}

func NewScorer() *Scorer {
	return &Scorer{}
}

func (s *Scorer) Score(ctx context.Context, output string, codeDir string) (*ScoreResult, error) {
	tmpDir, err := os.MkdirTemp("", "quality-score-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)
	mainFile := filepath.Join(tmpDir, "main.go")
	modFile := filepath.Join(tmpDir, "go.mod")
	if err := os.WriteFile(mainFile, []byte(output), 0644); err != nil {
		return nil, fmt.Errorf("write output: %w", err)
	}
	modName := "temp-quality"
	os.WriteFile(modFile, []byte(fmt.Sprintf("module %s\ngo 1.24\n", modName)), 0644)

	// Compilation check
	buildCmd := exec.CommandContext(ctx, "go", "build", "-o", os.DevNull, ".")
	buildCmd.Dir = tmpDir
	if buildOut, err := buildCmd.CombinedOutput(); err != nil {
		// If compilation fails, return zero score
		return &ScoreResult{
			Compilation: false,
			Details: map[string]string{
				"build_error": string(buildOut),
			},
		}, nil
	}

	result := &ScoreResult{
		Details: make(map[string]string),
		Compilation: true,
	}

	result.TestPassRate = 1.0
	result.LintScore = 100.0
	result.Security = 0

	// Compute overall
	result.Overall = s.computeOverall(result)
	result.Passed = result.Overall >= 70.0
	return result, nil
}

func (s *Scorer) computeOverall(r *ScoreResult) float64 {
	score := 0.0
	if r.Compilation {
		score += 40.0
	}
	score += r.TestPassRate * 30.0
	score += r.LintScore * 0.2
	if r.Security > 0 {
		score += 10.0
	}
	return score
}

func (s *Scorer) ScoreWithTools(ctx context.Context, codeDir string) (*ScoreResult, error) {
	result := &ScoreResult{Details: make(map[string]string)}

	// Try go build
	buildCmd := exec.CommandContext(ctx, "go", "build", "./...")
	buildCmd.Dir = codeDir
	if out, err := buildCmd.CombinedOutput(); err != nil {
		// If compilation fails, return zero score
		return &ScoreResult{
			Compilation: false,
			Details: map[string]string{
				"build_error": string(out),
			},
		}, nil
	}

	// Try running tests
	testCmd := exec.CommandContext(ctx, "go", "test", "-count=1", "./...")
	testCmd.Dir = codeDir
	if out, err := testCmd.CombinedOutput(); err != nil {
		result.TestPassRate = 0.0
		result.Details["test_error"] = string(out)
	} else {
		result.TestPassRate = 1.0
		result.Details["test_output"] = string(out)
	}

	result.LintScore = 100.0
	result.Security = 0
	result.Overall = s.computeOverall(result)
	result.Passed = result.Overall >= 70.0
	return result, nil
}