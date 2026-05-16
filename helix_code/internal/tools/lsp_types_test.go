package tools

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

// TestDiagnosticSeverity_String covers each defined severity level.
func TestDiagnosticSeverity_String(t *testing.T) {
	cases := []struct {
		sev  DiagnosticSeverity
		want string
	}{
		{SeverityError, "error"},
		{SeverityWarning, "warning"},
		{SeverityInformation, "information"},
		{SeverityHint, "hint"},
	}
	for _, tc := range cases {
		if got := tc.sev.String(); got != tc.want {
			t.Errorf("DiagnosticSeverity(%d).String() = %q, want %q", tc.sev, got, tc.want)
		}
	}
}

// TestServerStatus_String covers all 7 status enum values.
func TestServerStatus_String(t *testing.T) {
	cases := []struct {
		st   ServerStatus
		want string
	}{
		{ServerStatusUnknown, "unknown"},
		{ServerStatusStarting, "starting"},
		{ServerStatusReady, "ready"},
		{ServerStatusIdle, "idle"},
		{ServerStatusStopping, "stopping"},
		{ServerStatusStopped, "stopped"},
		{ServerStatusCrashed, "crashed"},
	}
	for _, tc := range cases {
		if got := tc.st.String(); got != tc.want {
			t.Errorf("ServerStatus(%d).String() = %q, want %q", tc.st, got, tc.want)
		}
	}
}

func TestDiagnosticSummary_RecomputeFromEmpty(t *testing.T) {
	s := &DiagnosticSummary{}
	s.Recompute()
	if s.TotalErrors != 0 || s.TotalWarnings != 0 || s.TotalInformation != 0 || s.TotalHints != 0 {
		t.Errorf("empty summary totals not zero: %+v", s)
	}
	if s.Expandable {
		t.Errorf("empty summary should not be expandable")
	}
}

func TestDiagnosticSummary_RecomputeMixed(t *testing.T) {
	s := &DiagnosticSummary{
		Diagnostics: []Diagnostic{
			{ID: "d1", Severity: SeverityError, Source: "gopls", Message: "boom", FilePath: "/x.go"},
			{ID: "d2", Severity: SeverityWarning, Source: "gopls", Message: "hmm", FilePath: "/x.go"},
			{ID: "d3", Severity: SeverityWarning, Source: "gopls", Message: "hmm2", FilePath: "/x.go"},
			{ID: "d4", Severity: SeverityInformation, Source: "gopls", Message: "info", FilePath: "/x.go"},
			{ID: "d5", Severity: SeverityHint, Source: "gopls", Message: "hint", FilePath: "/x.go"},
			{ID: "d6", Severity: SeverityHint, Source: "gopls", Message: "hint2", FilePath: "/x.go"},
		},
	}
	s.Recompute()
	if s.TotalErrors != 1 {
		t.Errorf("TotalErrors = %d, want 1", s.TotalErrors)
	}
	if s.TotalWarnings != 2 {
		t.Errorf("TotalWarnings = %d, want 2", s.TotalWarnings)
	}
	if s.TotalInformation != 1 {
		t.Errorf("TotalInformation = %d, want 1", s.TotalInformation)
	}
	if s.TotalHints != 2 {
		t.Errorf("TotalHints = %d, want 2", s.TotalHints)
	}
	if !s.Expandable {
		t.Errorf("non-empty summary must be Expandable=true")
	}
}

func TestDiagnosticSummary_RecomputeIsIdempotent(t *testing.T) {
	s := &DiagnosticSummary{
		Diagnostics: []Diagnostic{
			{ID: "a", Severity: SeverityError, Source: "gopls", Message: "x"},
			{ID: "b", Severity: SeverityHint, Source: "gopls", Message: "y"},
		},
	}
	s.Recompute()
	first := *s
	s.Recompute()
	second := *s
	if !reflect.DeepEqual(first, second) {
		t.Errorf("Recompute not idempotent:\n first=%+v\nsecond=%+v", first, second)
	}
}

func TestDiagnosticSummary_JSONRoundTrip(t *testing.T) {
	original := DiagnosticSummary{
		Diagnostics: []Diagnostic{
			{
				ID:       "id-1",
				Severity: SeverityError,
				Code:     "E001",
				Source:   "gopls",
				Message:  "undefined: foo",
				Range: Range{
					Start: Position{Line: 1, Character: 4},
					End:   Position{Line: 1, Character: 7},
				},
				FilePath: "/abs/path/main.go",
			},
		},
	}
	original.Recompute()

	raw, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded DiagnosticSummary
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !reflect.DeepEqual(original, decoded) {
		t.Errorf("round-trip mismatch:\norig=%+v\ndecoded=%+v", original, decoded)
	}
}

func TestLSPServerSpec_Zero(t *testing.T) {
	var spec LSPServerSpec
	if spec.Name != "" || spec.Binary != "" || spec.LanguageID != "" {
		t.Errorf("zero spec has unexpected non-zero string fields: %+v", spec)
	}
	// nil slices must be range-iterable without panic.
	count := 0
	for range spec.FileExtensions {
		count++
	}
	for range spec.Args {
		count++
	}
	if count != 0 {
		t.Errorf("zero spec slices iterated %d times, want 0", count)
	}
	// nil map must be safe to read.
	if v, ok := spec.InitializationOpts["k"]; ok || v != nil {
		t.Errorf("zero InitializationOpts should be empty, got %v ok=%v", v, ok)
	}
}

func TestServerInfo_JSONShapeUsesStatusName(t *testing.T) {
	info := ServerInfo{
		Spec:       LSPServerSpec{Name: "gopls"},
		Name:       "gopls",
		Status:     ServerStatusReady,
		PID:        4242,
		Uptime:     3 * time.Second,
		OpenFiles:  2,
		LastActive: time.Unix(1700000000, 0).UTC(),
	}

	raw, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(raw)
	if !strings.Contains(s, `"status":"ready"`) {
		t.Errorf("expected JSON to contain \"status\":\"ready\", got: %s", s)
	}
	// status_name is the publicly serialised form too — should match.
	// The Spec field is intentionally not serialised (json:"-").
	if strings.Contains(s, `"Spec"`) || strings.Contains(s, `"spec"`) {
		t.Errorf("Spec must not appear in JSON output, got: %s", s)
	}
}
