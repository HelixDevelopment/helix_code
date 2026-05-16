package tools

import (
	"encoding/json"
	"time"
)

// DiagnosticSeverity mirrors the LSP severity scale (1=Error..4=Hint).
//
// The numeric values intentionally match the LSP wire protocol so the
// mapping layer in lsp_client.go (T04) is a straight cast.
type DiagnosticSeverity int

const (
	SeverityError       DiagnosticSeverity = 1
	SeverityWarning     DiagnosticSeverity = 2
	SeverityInformation DiagnosticSeverity = 3
	SeverityHint        DiagnosticSeverity = 4
)

// String returns the lowercase severity name.
//
// Unknown values fall back to "unknown" so log/error messages stay safe.
func (s DiagnosticSeverity) String() string {
	switch s {
	case SeverityError:
		return "error"
	case SeverityWarning:
		return "warning"
	case SeverityInformation:
		return "information"
	case SeverityHint:
		return "hint"
	default:
		return "unknown"
	}
}

// Position is a 0-indexed line/character pair (UTF-16 code-unit offsets per LSP).
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// Range is an LSP range with start and end Positions.
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Diagnostic is HelixCode's stable wrapped form of an LSP diagnostic.
//
// We deliberately wrap protocol.Diagnostic from go.lsp.dev because:
//   - we need a stable ID field so LSPAnalyzeDiagnostic can look it up
//     across tool invocations without re-querying the server,
//   - we need FilePath on every record (LSP only carries it on the
//     publishDiagnostics envelope), and
//   - we want a stable JSON schema for the LLM that does not churn with
//     upstream go.lsp.dev/protocol bumps.
type Diagnostic struct {
	ID       string             `json:"id"`
	Severity DiagnosticSeverity `json:"severity"`
	Code     string             `json:"code,omitempty"`
	Source   string             `json:"source"`
	Message  string             `json:"message"`
	Range    Range              `json:"range"`
	FilePath string             `json:"file_path"`
}

// DiagnosticSummary aggregates a list of diagnostics with severity
// totals suitable for inlining into Edit/Write tool results.
type DiagnosticSummary struct {
	TotalErrors      int          `json:"total_errors"`
	TotalWarnings    int          `json:"total_warnings"`
	TotalInformation int          `json:"total_information"`
	TotalHints       int          `json:"total_hints"`
	Diagnostics      []Diagnostic `json:"diagnostics"`
	Expandable       bool         `json:"expandable"`
}

// Recompute (re)derives the totals from Diagnostics and the Expandable
// flag. Idempotent: callers may call it any number of times after
// mutating Diagnostics.
func (s *DiagnosticSummary) Recompute() {
	s.TotalErrors = 0
	s.TotalWarnings = 0
	s.TotalInformation = 0
	s.TotalHints = 0
	for _, d := range s.Diagnostics {
		switch d.Severity {
		case SeverityError:
			s.TotalErrors++
		case SeverityWarning:
			s.TotalWarnings++
		case SeverityInformation:
			s.TotalInformation++
		case SeverityHint:
			s.TotalHints++
		}
	}
	s.Expandable = len(s.Diagnostics) > 0
}

// LSPServerSpec describes a curated allowlist entry: which binary to
// spawn for which file extensions, with which initialisation options.
type LSPServerSpec struct {
	Name               string
	Binary             string
	Args               []string
	FileExtensions     []string
	LanguageID         string
	InitializationOpts map[string]any
}

// ServerStatus is the lifecycle state of a managed LSP server process.
type ServerStatus int

const (
	ServerStatusUnknown ServerStatus = iota
	ServerStatusStarting
	ServerStatusReady
	ServerStatusIdle
	ServerStatusStopping
	ServerStatusStopped
	ServerStatusCrashed
)

// String returns the lowercase status name.
func (s ServerStatus) String() string {
	switch s {
	case ServerStatusUnknown:
		return "unknown"
	case ServerStatusStarting:
		return "starting"
	case ServerStatusReady:
		return "ready"
	case ServerStatusIdle:
		return "idle"
	case ServerStatusStopping:
		return "stopping"
	case ServerStatusStopped:
		return "stopped"
	case ServerStatusCrashed:
		return "crashed"
	default:
		return "unknown"
	}
}

// ServerInfo is the public read-only snapshot of a managed server's state.
//
// JSON output uses StatusName (the Status enum's String() form) instead
// of the numeric Status, so external consumers see "ready" rather than 2.
// The Spec field is intentionally excluded from JSON output: it is held
// by reference for in-process callers and would just bloat the JSON
// payload sent to the LLM.
type ServerInfo struct {
	Spec       LSPServerSpec `json:"-"`
	Name       string        `json:"name"`
	Status     ServerStatus  `json:"-"`
	StatusName string        `json:"status"`
	PID        int           `json:"pid,omitempty"`
	Uptime     time.Duration `json:"uptime,omitempty"`
	OpenFiles  int           `json:"open_files"`
	LastActive time.Time     `json:"last_active"`
}

// MarshalJSON renders ServerInfo with StatusName auto-derived from
// Status, so callers that mutate Status directly do not have to
// remember to keep StatusName in sync.
func (i ServerInfo) MarshalJSON() ([]byte, error) {
	type alias ServerInfo
	clone := alias(i)
	clone.StatusName = i.Status.String()
	return json.Marshal(clone)
}
