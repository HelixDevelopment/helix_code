package kilocode

type ImpactAnalyzer struct {
	cg      *CallGraph
	rootDir string
}

func NewImpactAnalyzer(rootDir string) (*ImpactAnalyzer, error) {
	cg, err := BuildCallGraph(rootDir)
	if err != nil {
		return nil, err
	}
	return &ImpactAnalyzer{cg: cg, rootDir: rootDir}, nil
}

func (ia *ImpactAnalyzer) Analyze(symbolName string) (*ImpactResult, error) {
	node, ok := ia.cg.Nodes[symbolName]
	if !ok {
		return &ImpactResult{
			Symbol:      SymbolRef{Name: symbolName},
			Callers:     nil,
			Callees:     nil,
			BlastRadius: 0,
			RiskScore:   0,
		}, nil
	}

	callers := ia.cg.FindCallers(symbolName)
	callees := ia.cg.FindCallees(symbolName)

	affectedFiles := make(map[string]bool)
	for _, c := range callers {
		affectedFiles[c.FilePath] = true
	}
	for _, c := range callees {
		affectedFiles[c.FilePath] = true
	}
	affectedFiles[node.FilePath] = true

	var files []string
	for f := range affectedFiles {
		files = append(files, f)
	}

	blastRadius := len(files)
	riskScore := float64(len(callers)) * 0.6 + float64(len(callees)) * 0.4

	if blastRadius == 0 {
		blastRadius = 1
	}
	if riskScore == 0 {
		riskScore = 0.1
	}

	return &ImpactResult{
		Symbol:        node,
		Callers:       callers,
		Callees:       callees,
		AffectedFiles: files,
		BlastRadius:   blastRadius,
		RiskScore:     riskScore,
	}, nil
}
