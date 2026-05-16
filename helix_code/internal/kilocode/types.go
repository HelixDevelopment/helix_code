package kilocode

import "errors"

type SymbolKind int

const (
	KindFunction  SymbolKind = iota
	KindMethod
	KindVariable
	KindClass
	KindInterface
)

type SymbolRef struct {
	Name     string     `json:"name"`
	Kind     SymbolKind `json:"kind"`
	FilePath string     `json:"file_path"`
	Line     int        `json:"line"`
	Column   int        `json:"column"`
}

type CallEdge struct {
	Caller SymbolRef `json:"caller"`
	Callee SymbolRef `json:"callee"`
}

type CallGraph struct {
	Nodes map[string]SymbolRef `json:"nodes"`
	Edges []CallEdge           `json:"edges"`
}

type ImpactResult struct {
	Symbol         SymbolRef   `json:"symbol"`
	Callers        []SymbolRef `json:"callers"`
	Callees        []SymbolRef `json:"callees"`
	AffectedFiles  []string    `json:"affected_files"`
	BlastRadius    int         `json:"blast_radius"`
	RiskScore      float64     `json:"risk_score"`
}

type RenameResult struct {
	Symbol        SymbolRef `json:"symbol"`
	NewName       string    `json:"new_name"`
	FilesModified int       `json:"files_modified"`
	Occurrences   int       `json:"occurrences"`
}

var (
	ErrSymbolNotFound    = errors.New("symbol not found in codebase")
	ErrRenameConflict    = errors.New("rename would create a naming conflict")
	ErrRefactorNotSafe   = errors.New("refactoring is not safe — impact analysis required")
	ErrNoCallGraph       = errors.New("no call graph available — run impact analysis first")
)

const (
	MaxRenameFiles      = 500
	MaxRefactorFileSize = 1 * 1024 * 1024
)
