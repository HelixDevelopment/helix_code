package tools

// LSP-backed agent tools.
//
// This file wires the LSPManager (T05) into two Tool implementations the
// agent can call directly:
//
//   - LSPGetDiagnosticsTool  — returns a DiagnosticSummary, optionally
//     filtered to a single file and/or to a minimum severity level.
//   - LSPAnalyzeDiagnosticTool — looks up a single diagnostic by its
//     stable ID (assigned by LSPClient.onPublishDiagnostics) and returns
//     a small report with a human-readable summary string.
//
// Both tools are read-only; they never mutate manager state. They return
// errors only for missing-input or not-found cases — an empty diagnostic
// set is a successful response with TotalErrors=0.
//
// Note on omitted fields: the original spec sketched a SuggestedFix
// field on the analyze tool's report. F13 v1 has no fix-suggestion engine
// wired up yet (that lands in a follow-up feature); rather than expose a
// nullable JSON field that lies about future content, we omit it and
// surface only the data we actually have. When fix-suggestions land they
// will arrive as a new optional field with no breaking change to this
// shape.

import (
	"context"
	"fmt"
	"strings"

	"dev.helix.code/internal/approval"
)

// ---------- helpers ----------

// FilterDiagnosticsBySeverity returns the subset of in whose severity is
// at least as severe as min. The LSP severity scale is "lower number =
// more severe" (Error=1, Warning=2, Information=3, Hint=4) so "at least
// as severe as min" is `d.Severity <= min`.
//
// Order is preserved.
func FilterDiagnosticsBySeverity(in []Diagnostic, min DiagnosticSeverity) []Diagnostic {
	if min <= 0 {
		min = SeverityError
	}
	out := make([]Diagnostic, 0, len(in))
	for _, d := range in {
		if d.Severity <= min {
			out = append(out, d)
		}
	}
	return out
}

// parseSeverity maps the user-facing severity string ("error","warning",
// "information","hint", any case) onto a DiagnosticSeverity. Empty or
// unrecognised input falls back to SeverityError so the default behaviour
// is "show me errors only".
func parseSeverity(s string) DiagnosticSeverity {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "warning", "warn":
		return SeverityWarning
	case "information", "info":
		return SeverityInformation
	case "hint":
		return SeverityHint
	case "error", "":
		return SeverityError
	default:
		// Unrecognised → fall back to errors only. This matches the
		// task contract ("Default if unset OR unrecognised: SeverityError").
		return SeverityError
	}
}

// ---------- LSPGetDiagnosticsTool ----------

// LSPGetDiagnosticsTool is the agent-callable read of the LSPManager's
// diagnostic cache.
//
// Args:
//
//   - file_path string (optional) — when present, only that file's
//     diagnostics are returned; when absent, returns the union across
//     every running server.
//   - severity string (optional) — minimum severity filter
//     ("error","warning","information","hint", case-insensitive).
//     Default and fallback for unrecognised values is "error".
//
// Returns: a DiagnosticSummary with totals recomputed from the filtered
// diagnostic list.
type LSPGetDiagnosticsTool struct {
	manager *LSPManager
}

// NewLSPGetDiagnosticsTool returns a tool bound to the given manager.
// A nil manager is allowed (Execute will then return an empty summary)
// so the tool can be registered before the manager is wired in.
func NewLSPGetDiagnosticsTool(manager *LSPManager) *LSPGetDiagnosticsTool {
	return &LSPGetDiagnosticsTool{manager: manager}
}

func (t *LSPGetDiagnosticsTool) Name() string { return "lsp_get_diagnostics" }

// RequiresApproval — pure read of LSP server state (spec §3.6).
func (t *LSPGetDiagnosticsTool) RequiresApproval() approval.ApprovalLevel { return approval.LevelReadOnly }

func (t *LSPGetDiagnosticsTool) Description() string {
	return "Return diagnostics published by the LSP servers, optionally filtered to a single file or minimum severity."
}

func (t *LSPGetDiagnosticsTool) Category() ToolCategory { return CategoryLSP }

func (t *LSPGetDiagnosticsTool) Schema() ToolSchema {
	return ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"file_path": map[string]interface{}{
				"type":        "string",
				"description": "Optional. Absolute path of a single file to fetch diagnostics for. If omitted, returns diagnostics aggregated across all open files.",
			},
			"severity": map[string]interface{}{
				"type":        "string",
				"description": "Optional minimum severity to include (\"error\", \"warning\", \"information\", \"hint\"; case-insensitive). Default: \"error\".",
				"enum":        []string{"error", "warning", "information", "hint"},
			},
		},
		Required:    []string{},
		Description: "Fetch diagnostics from the LSP manager.",
	}
}

func (t *LSPGetDiagnosticsTool) Validate(params map[string]interface{}) error {
	if v, ok := params["file_path"]; ok {
		if _, isString := v.(string); !isString {
			return fmt.Errorf("file_path must be a string, got %T", v)
		}
	}
	if v, ok := params["severity"]; ok {
		if _, isString := v.(string); !isString {
			return fmt.Errorf("severity must be a string, got %T", v)
		}
	}
	return nil
}

func (t *LSPGetDiagnosticsTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	_ = ctx // diagnostic reads are local cache lookups; ctx not needed today.

	filePath, _ := params["file_path"].(string)
	severityArg, _ := params["severity"].(string)
	min := parseSeverity(severityArg)

	var diags []Diagnostic
	if t.manager != nil {
		if strings.TrimSpace(filePath) == "" {
			diags = t.manager.AllDiagnostics()
		} else {
			diags = t.manager.GetDiagnostics(filePath)
		}
	}
	filtered := FilterDiagnosticsBySeverity(diags, min)

	summary := DiagnosticSummary{Diagnostics: filtered}
	summary.Recompute()
	return summary, nil
}

// ---------- LSPAnalyzeDiagnosticTool ----------

// LSPAnalyzeDiagnosticTool returns extra context for a single diagnostic
// identified by its stable LSPClient-assigned ID.
//
// Args:
//
//   - diagnostic_id string (required) — the ID returned by
//     LSPGetDiagnostics on a previous call.
//
// Returns: a struct with the matched Diagnostic plus a human-readable
// summary message ("<severity> at <file>:<line>:<col>: <message>"). If
// no diagnostic with that ID is currently cached, Execute returns an
// error wrapping the missing ID — this is the honest signal to the
// agent that the diagnostic has been superseded by a more recent
// publishDiagnostics or never existed.
type LSPAnalyzeDiagnosticTool struct {
	manager *LSPManager
}

// NewLSPAnalyzeDiagnosticTool returns a tool bound to the given manager.
// A nil manager is allowed but Execute will then always return
// not-found.
func NewLSPAnalyzeDiagnosticTool(manager *LSPManager) *LSPAnalyzeDiagnosticTool {
	return &LSPAnalyzeDiagnosticTool{manager: manager}
}

func (t *LSPAnalyzeDiagnosticTool) Name() string { return "lsp_analyze_diagnostic" }

// RequiresApproval — pure read of a single diagnostic record (spec §3.6).
func (t *LSPAnalyzeDiagnosticTool) RequiresApproval() approval.ApprovalLevel { return approval.LevelReadOnly }

func (t *LSPAnalyzeDiagnosticTool) Description() string {
	return "Look up a single LSP diagnostic by its stable ID and return a structured analysis report."
}

func (t *LSPAnalyzeDiagnosticTool) Category() ToolCategory { return CategoryLSP }

func (t *LSPAnalyzeDiagnosticTool) Schema() ToolSchema {
	return ToolSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"diagnostic_id": map[string]interface{}{
				"type":        "string",
				"description": "The diagnostic ID returned by lsp_get_diagnostics.",
			},
		},
		Required:    []string{"diagnostic_id"},
		Description: "Analyse a single LSP diagnostic by ID.",
	}
}

func (t *LSPAnalyzeDiagnosticTool) Validate(params map[string]interface{}) error {
	v, ok := params["diagnostic_id"]
	if !ok {
		return fmt.Errorf("diagnostic_id is required")
	}
	s, isString := v.(string)
	if !isString {
		return fmt.Errorf("diagnostic_id must be a string, got %T", v)
	}
	if strings.TrimSpace(s) == "" {
		return fmt.Errorf("diagnostic_id must not be empty")
	}
	return nil
}

// lspAnalyzeReport is the JSON shape returned by LSPAnalyzeDiagnosticTool.
// It is exported as a package-private struct because the public surface
// is the JSON wire shape, not a Go type — agents see only the marshalled
// form.
type lspAnalyzeReport struct {
	Diagnostic     Diagnostic `json:"diagnostic"`
	SummaryMessage string     `json:"summary_message"`
}

func (t *LSPAnalyzeDiagnosticTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	_ = ctx
	id, _ := params["diagnostic_id"].(string)
	id = strings.TrimSpace(id)
	if id == "" {
		// Defence in depth: Validate already covers this, but a direct
		// Execute caller (bypassing the registry) might skip validation.
		return nil, fmt.Errorf("diagnostic_id is required")
	}

	if t.manager == nil {
		return nil, fmt.Errorf("diagnostic id %q not found", id)
	}
	for _, d := range t.manager.AllDiagnostics() {
		if d.ID == id {
			return lspAnalyzeReport{
				Diagnostic: d,
				SummaryMessage: fmt.Sprintf("%s at %s:%d:%d: %s",
					d.Severity.String(),
					d.FilePath,
					d.Range.Start.Line+1,
					d.Range.Start.Character+1,
					d.Message,
				),
			}, nil
		}
	}
	return nil, fmt.Errorf("diagnostic id %q not found", id)
}
