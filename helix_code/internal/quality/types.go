package quality

type ScoreResult struct {
	Overall      float64           `json:"overall"`
	Compilation  bool              `json:"compilation"`
	TestPassRate float64           `json:"test_pass_rate"`
	LintScore    float64           `json:"lint_score"`
	// Security is a COUNT of security findings (lower is better; 0 = clean).
	// computeOverall awards the security credit only when Security == 0.
	Security     int               `json:"security"`
	Details      map[string]string `json:"details"`
	Passed       bool              `json:"passed"`
}

type QualityGate struct {
	MinScore      float64 `yaml:"min_score"`
	RequireBuild  bool    `yaml:"require_build"`
	RequireTests  bool    `yaml:"require_tests"`
	RequireLint   bool    `yaml:"require_lint"`
}