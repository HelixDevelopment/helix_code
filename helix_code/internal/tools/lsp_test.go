package tools_test

// Tests for LSPGetDiagnosticsTool and LSPAnalyzeDiagnosticTool.
//
// These tests share the fake-LSP-subprocess scaffolding from
// lsp_manager_test.go (TestMain builds the fake server binary, fakeSpec()
// returns the curated allowlist entry, writeTempFile writes a .fake file).
// The tools are exercised against a real LSPManager driving a real
// subprocess that publishes real LSP diagnostics — no in-test fakes.

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	"dev.helix.code/internal/tools"
)

// ---------- shared helpers ----------

// newManagerWithFake returns a manager preconfigured with the fake server
// spec and a deferred Shutdown registered on t.Cleanup.
func newManagerWithFake(t *testing.T) *tools.LSPManager {
	t.Helper()
	m := tools.NewLSPManager(t.TempDir(), []tools.LSPServerSpec{fakeSpec()}, zap.NewNop())
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = m.Shutdown(ctx)
	})
	return m
}

// summaryFromExecuteResult unmarshals an Execute result into a
// DiagnosticSummary by way of JSON. We round-trip through JSON so the
// test pins the wire shape the LLM will see, not just the in-memory type.
func summaryFromExecuteResult(t *testing.T, res interface{}) tools.DiagnosticSummary {
	t.Helper()
	if res == nil {
		t.Fatalf("Execute returned nil result")
	}
	raw, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshal Execute result: %v", err)
	}
	var s tools.DiagnosticSummary
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatalf("unmarshal into DiagnosticSummary: %v\nraw=%s", err, string(raw))
	}
	return s
}

// ---------- LSPGetDiagnosticsTool: shape ----------

func TestLSPGetDiagnosticsTool_NameDescriptionSchemaCategory(t *testing.T) {
	tool := tools.NewLSPGetDiagnosticsTool(nil)

	if got, want := tool.Name(), "lsp_get_diagnostics"; got != want {
		t.Errorf("Name(): got %q want %q", got, want)
	}
	if tool.Description() == "" {
		t.Errorf("Description(): empty")
	}
	if got, want := tool.Category(), tools.CategoryLSP; got != want {
		t.Errorf("Category(): got %q want %q", got, want)
	}
	schema := tool.Schema()
	if schema.Type != "object" {
		t.Errorf("Schema().Type: got %q want %q", schema.Type, "object")
	}
	if _, ok := schema.Properties["file_path"]; !ok {
		t.Errorf("Schema(): missing file_path property")
	}
	if _, ok := schema.Properties["severity"]; !ok {
		t.Errorf("Schema(): missing severity property")
	}
	// file_path is optional — Required must be empty (or not contain it).
	for _, r := range schema.Required {
		if r == "file_path" {
			t.Errorf("Schema(): file_path must be optional, but appears in Required")
		}
	}
}

// ---------- LSPGetDiagnosticsTool: validate ----------

func TestLSPGetDiagnosticsTool_ValidateRejectsBadTypes(t *testing.T) {
	tool := tools.NewLSPGetDiagnosticsTool(nil)

	// Empty args: legal (file_path optional, severity optional).
	if err := tool.Validate(map[string]interface{}{}); err != nil {
		t.Errorf("Validate(empty): unexpected error: %v", err)
	}

	// file_path of wrong type → reject.
	if err := tool.Validate(map[string]interface{}{"file_path": 42}); err == nil {
		t.Errorf("Validate(file_path=42): expected error, got nil")
	}

	// severity of wrong type → reject.
	if err := tool.Validate(map[string]interface{}{"severity": 3}); err == nil {
		t.Errorf("Validate(severity=int): expected error, got nil")
	}

	// All-string args → accept.
	if err := tool.Validate(map[string]interface{}{
		"file_path": "/tmp/x.fake",
		"severity":  "warning",
	}); err != nil {
		t.Errorf("Validate(valid): unexpected error: %v", err)
	}
}

// ---------- LSPGetDiagnosticsTool: execute ----------

func TestLSPGetDiagnosticsTool_ExecuteReturnsSummaryForFile(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	m := newManagerWithFake(t)
	filePath := writeTempFile(t, ".fake", "// @fake-error: msg1\n")
	if err := m.NotifyOpen(ctx, filePath); err != nil {
		t.Fatalf("NotifyOpen: %v", err)
	}
	if got := waitForDiagnostics(m, filePath, func(n int) bool { return n >= 1 }, 5*time.Second); len(got) != 1 {
		t.Fatalf("precondition: want 1 diagnostic before tool call, got %d", len(got))
	}

	tool := tools.NewLSPGetDiagnosticsTool(m)
	res, err := tool.Execute(ctx, map[string]interface{}{"file_path": filePath})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	summary := summaryFromExecuteResult(t, res)

	if summary.TotalErrors != 1 {
		t.Errorf("TotalErrors: got %d want 1 (summary=%+v)", summary.TotalErrors, summary)
	}
	if len(summary.Diagnostics) != 1 {
		t.Fatalf("Diagnostics len: got %d want 1", len(summary.Diagnostics))
	}
	d := summary.Diagnostics[0]
	if !strings.Contains(d.Message, "msg1") {
		t.Errorf("Diagnostic.Message: got %q want substring 'msg1'", d.Message)
	}
	if d.FilePath != filePath {
		t.Errorf("Diagnostic.FilePath: got %q want %q", d.FilePath, filePath)
	}
	if d.Severity != tools.SeverityError {
		t.Errorf("Diagnostic.Severity: got %v want error", d.Severity)
	}
	if !summary.Expandable {
		t.Errorf("Expandable: got false want true (Diagnostics non-empty)")
	}
}

func TestLSPGetDiagnosticsTool_ExecuteReturnsAllWhenNoFilePath(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	m := newManagerWithFake(t)

	fileA := writeTempFile(t, ".fake", "// @fake-error: alpha\n")
	if err := m.NotifyOpen(ctx, fileA); err != nil {
		t.Fatalf("NotifyOpen A: %v", err)
	}
	if got := waitForDiagnostics(m, fileA, func(n int) bool { return n >= 1 }, 5*time.Second); len(got) != 1 {
		t.Fatalf("precondition A: want 1 diagnostic, got %d", len(got))
	}

	fileB := writeTempFile(t, ".fake", "// @fake-error: beta-1\n// @fake-error: beta-2\n")
	if err := m.NotifyOpen(ctx, fileB); err != nil {
		t.Fatalf("NotifyOpen B: %v", err)
	}
	if got := waitForDiagnostics(m, fileB, func(n int) bool { return n >= 2 }, 5*time.Second); len(got) != 2 {
		t.Fatalf("precondition B: want 2 diagnostics, got %d", len(got))
	}

	tool := tools.NewLSPGetDiagnosticsTool(m)
	res, err := tool.Execute(ctx, map[string]interface{}{})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	summary := summaryFromExecuteResult(t, res)

	if summary.TotalErrors != 3 {
		t.Errorf("TotalErrors: got %d want 3 (summary=%+v)", summary.TotalErrors, summary)
	}
	if len(summary.Diagnostics) != 3 {
		t.Fatalf("Diagnostics len: got %d want 3", len(summary.Diagnostics))
	}
	// Confirm both file paths appear among the aggregated diagnostics.
	var sawA, sawB bool
	for _, d := range summary.Diagnostics {
		if d.FilePath == fileA {
			sawA = true
		}
		if d.FilePath == fileB {
			sawB = true
		}
	}
	if !sawA || !sawB {
		t.Errorf("aggregated diagnostics missing one or both files: sawA=%v sawB=%v diags=%+v", sawA, sawB, summary.Diagnostics)
	}
}

// TestLSPGetDiagnosticsTool_FilterBySeverity exercises the severity filter
// directly via FilterDiagnosticsBySeverity (the fake server only emits
// errors, so the only honest way to cover the warning/info/hint paths is
// to feed the filter a synthetic mixed slice).
func TestLSPGetDiagnosticsTool_FilterBySeverity(t *testing.T) {
	in := []tools.Diagnostic{
		{ID: "1", Severity: tools.SeverityError, Message: "err"},
		{ID: "2", Severity: tools.SeverityWarning, Message: "warn"},
		{ID: "3", Severity: tools.SeverityInformation, Message: "info"},
		{ID: "4", Severity: tools.SeverityHint, Message: "hint"},
	}

	cases := []struct {
		name string
		min  tools.DiagnosticSeverity
		want []string // IDs expected after filter, in original order
	}{
		{"error_only", tools.SeverityError, []string{"1"}},
		{"warning_and_above", tools.SeverityWarning, []string{"1", "2"}},
		{"info_and_above", tools.SeverityInformation, []string{"1", "2", "3"}},
		{"hint_and_above_returns_all", tools.SeverityHint, []string{"1", "2", "3", "4"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := tools.FilterDiagnosticsBySeverity(in, tc.min)
			if len(out) != len(tc.want) {
				t.Fatalf("len: got %d want %d (out=%+v)", len(out), len(tc.want), out)
			}
			for i, id := range tc.want {
				if out[i].ID != id {
					t.Errorf("[%d] ID: got %q want %q", i, out[i].ID, id)
				}
			}
		})
	}
}

// TestLSPGetDiagnosticsTool_ExecuteSeverityArgFiltersOutput exercises the
// severity arg parsing through Execute. With only error-severity output
// from the fake server, the smallest interesting check is "min=warning"
// returns nothing (errors are below 'warning' in the LSP severity scale,
// where lower value = higher importance).
//
// Wait — re-reading lsp_types.go: SeverityError=1 (most severe),
// SeverityHint=4 (least severe). "Minimum severity" in user terms means
// "include items at least this severe", so min=warning includes warnings
// AND errors, min=error includes only errors, min=hint includes everything.
// That matches the FilterDiagnosticsBySeverity contract above.
func TestLSPGetDiagnosticsTool_ExecuteSeverityArgFiltersOutput(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	m := newManagerWithFake(t)
	filePath := writeTempFile(t, ".fake", "// @fake-error: msg1\n")
	if err := m.NotifyOpen(ctx, filePath); err != nil {
		t.Fatalf("NotifyOpen: %v", err)
	}
	if got := waitForDiagnostics(m, filePath, func(n int) bool { return n >= 1 }, 5*time.Second); len(got) != 1 {
		t.Fatalf("precondition: want 1 diagnostic, got %d", len(got))
	}

	tool := tools.NewLSPGetDiagnosticsTool(m)

	// severity=error (default) → 1 entry.
	res, err := tool.Execute(ctx, map[string]interface{}{"file_path": filePath, "severity": "error"})
	if err != nil {
		t.Fatalf("Execute(error): %v", err)
	}
	if got := summaryFromExecuteResult(t, res); got.TotalErrors != 1 || len(got.Diagnostics) != 1 {
		t.Errorf("severity=error: TotalErrors=%d Diagnostics=%d", got.TotalErrors, len(got.Diagnostics))
	}

	// severity=hint → also 1 entry (errors are at-or-above hint).
	res, err = tool.Execute(ctx, map[string]interface{}{"file_path": filePath, "severity": "hint"})
	if err != nil {
		t.Fatalf("Execute(hint): %v", err)
	}
	if got := summaryFromExecuteResult(t, res); len(got.Diagnostics) != 1 {
		t.Errorf("severity=hint: Diagnostics=%d want 1", len(got.Diagnostics))
	}

	// severity case-insensitive: "ERROR" parses same as "error".
	res, err = tool.Execute(ctx, map[string]interface{}{"file_path": filePath, "severity": "ERROR"})
	if err != nil {
		t.Fatalf("Execute(ERROR): %v", err)
	}
	if got := summaryFromExecuteResult(t, res); got.TotalErrors != 1 {
		t.Errorf("severity=ERROR (case-insensitive): TotalErrors=%d want 1", got.TotalErrors)
	}

	// Unrecognised severity falls back to error default (still 1 entry).
	res, err = tool.Execute(ctx, map[string]interface{}{"file_path": filePath, "severity": "garbage"})
	if err != nil {
		t.Fatalf("Execute(garbage): %v", err)
	}
	if got := summaryFromExecuteResult(t, res); got.TotalErrors != 1 {
		t.Errorf("severity=garbage (fallback to error): TotalErrors=%d want 1", got.TotalErrors)
	}
}

// ---------- LSPAnalyzeDiagnosticTool: shape ----------

func TestLSPAnalyzeDiagnosticTool_NameDescriptionSchemaCategory(t *testing.T) {
	tool := tools.NewLSPAnalyzeDiagnosticTool(nil)

	if got, want := tool.Name(), "lsp_analyze_diagnostic"; got != want {
		t.Errorf("Name(): got %q want %q", got, want)
	}
	if tool.Description() == "" {
		t.Errorf("Description(): empty")
	}
	if got, want := tool.Category(), tools.CategoryLSP; got != want {
		t.Errorf("Category(): got %q want %q", got, want)
	}
	schema := tool.Schema()
	if _, ok := schema.Properties["diagnostic_id"]; !ok {
		t.Errorf("Schema(): missing diagnostic_id property")
	}
	requiredHas := false
	for _, r := range schema.Required {
		if r == "diagnostic_id" {
			requiredHas = true
		}
	}
	if !requiredHas {
		t.Errorf("Schema().Required: must include diagnostic_id, got %v", schema.Required)
	}
}

// ---------- LSPAnalyzeDiagnosticTool: validate ----------

func TestLSPAnalyzeDiagnosticTool_ValidateRejectsMissingID(t *testing.T) {
	tool := tools.NewLSPAnalyzeDiagnosticTool(nil)

	if err := tool.Validate(map[string]interface{}{}); err == nil {
		t.Errorf("Validate(empty): expected error, got nil")
	}
	if err := tool.Validate(map[string]interface{}{"diagnostic_id": ""}); err == nil {
		t.Errorf("Validate(empty string): expected error, got nil")
	}
	if err := tool.Validate(map[string]interface{}{"diagnostic_id": 123}); err == nil {
		t.Errorf("Validate(non-string): expected error, got nil")
	}
	if err := tool.Validate(map[string]interface{}{"diagnostic_id": "abc"}); err != nil {
		t.Errorf("Validate(valid): unexpected error: %v", err)
	}
}

// ---------- LSPAnalyzeDiagnosticTool: execute ----------

func TestLSPAnalyzeDiagnosticTool_ExecuteFindsByID(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	m := newManagerWithFake(t)
	filePath := writeTempFile(t, ".fake", "// @fake-error: needle\n")
	if err := m.NotifyOpen(ctx, filePath); err != nil {
		t.Fatalf("NotifyOpen: %v", err)
	}
	diags := waitForDiagnostics(m, filePath, func(n int) bool { return n >= 1 }, 5*time.Second)
	if len(diags) != 1 {
		t.Fatalf("precondition: want 1 diagnostic, got %d", len(diags))
	}
	wantID := diags[0].ID
	if wantID == "" {
		t.Fatalf("diagnostic ID empty")
	}

	tool := tools.NewLSPAnalyzeDiagnosticTool(m)
	res, err := tool.Execute(ctx, map[string]interface{}{"diagnostic_id": wantID})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res == nil {
		t.Fatalf("Execute returned nil")
	}

	// Round-trip through JSON to pin the wire shape.
	raw, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var report struct {
		Diagnostic     tools.Diagnostic `json:"diagnostic"`
		SummaryMessage string           `json:"summary_message"`
	}
	if err := json.Unmarshal(raw, &report); err != nil {
		t.Fatalf("unmarshal report: %v\nraw=%s", err, string(raw))
	}
	if report.Diagnostic.ID != wantID {
		t.Errorf("Diagnostic.ID: got %q want %q", report.Diagnostic.ID, wantID)
	}
	if !strings.Contains(report.Diagnostic.Message, "needle") {
		t.Errorf("Diagnostic.Message: got %q want substring 'needle'", report.Diagnostic.Message)
	}
	if report.SummaryMessage == "" {
		t.Errorf("SummaryMessage: empty")
	}
	if !strings.Contains(report.SummaryMessage, "needle") {
		t.Errorf("SummaryMessage: got %q want substring 'needle'", report.SummaryMessage)
	}
	if !strings.Contains(report.SummaryMessage, filePath) {
		t.Errorf("SummaryMessage: got %q want substring %q", report.SummaryMessage, filePath)
	}
}

func TestLSPAnalyzeDiagnosticTool_NotFoundErrors(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	m := newManagerWithFake(t)
	// Open one file so the manager is live; the unknown ID we look up
	// will not match anything in the published set.
	filePath := writeTempFile(t, ".fake", "// @fake-error: present\n")
	if err := m.NotifyOpen(ctx, filePath); err != nil {
		t.Fatalf("NotifyOpen: %v", err)
	}
	_ = waitForDiagnostics(m, filePath, func(n int) bool { return n >= 1 }, 5*time.Second)

	tool := tools.NewLSPAnalyzeDiagnosticTool(m)
	res, err := tool.Execute(ctx, map[string]interface{}{"diagnostic_id": "no-such-id"})
	if err == nil {
		t.Fatalf("Execute(unknown id): expected error, got nil (res=%v)", res)
	}
	if !strings.Contains(err.Error(), "no-such-id") {
		t.Errorf("error message: got %q want substring 'no-such-id'", err.Error())
	}
}
