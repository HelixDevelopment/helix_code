package quality

func (g *QualityGate) Check(result *ScoreResult) bool {
	if g.RequireBuild && !result.Compilation {
		return false
	}
	if g.RequireTests && result.TestPassRate < 1.0 {
		return false
	}
	if g.RequireLint && result.LintScore < 80.0 {
		return false
	}
	if result.Overall < g.MinScore {
		return false
	}
	return true
}

func DefaultGate() *QualityGate {
	return &QualityGate{
		MinScore:     70.0,
		RequireBuild: true,
		RequireTests: false,
		RequireLint:  false,
	}
}

func StrictGate() *QualityGate {
	return &QualityGate{
		MinScore:     90.0,
		RequireBuild: true,
		RequireTests: true,
		RequireLint:  true,
	}
}