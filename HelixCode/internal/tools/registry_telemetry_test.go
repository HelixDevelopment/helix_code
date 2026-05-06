//go:build testing_export

package tools_test

// Tests for ToolRegistry.SetTelemetryInstrumentation + the F16 telemetry wrap
// inside Execute (P1-F16-T07).
//
// Build tag: `testing_export`. Required because these tests reach into the
// telemetry package's stdoutWriter (via SetStdoutWriterForTest in
// internal/telemetry/testing_export.go) to capture real exporter output.
// Run them with: go test -tags=testing_export ./internal/tools/...
//
// Anti-bluff anchors:
//   - TestToolRegistry_Execute_WithTelemetry_RecordsSpan emits a real span
//     through a real OTel SDK BatchSpanProcessor + stdouttrace exporter and
//     asserts the captured stdout contains the tool name. Positive runtime
//     evidence: the registry actually drives the telemetry decorator.
//   - TestToolRegistry_Execute_WithTelemetry_FailureStatusOnError verifies
//     that a tool error propagates to the span status (Error code +
//     description) — proves the finish closure is invoked on the failure path.

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	"dev.helix.code/internal/telemetry"
	"dev.helix.code/internal/tools"
)

// stubTelemetryTool is a tiny Tool registered into the registry for telemetry
// tests. Returns a configurable response or error from Execute.
type stubTelemetryTool struct {
	name     string
	category tools.ToolCategory
	err      error
	result   interface{}
	calls    int
}

func (s *stubTelemetryTool) Name() string                          { return s.name }
func (s *stubTelemetryTool) Description() string                   { return "stub tool for telemetry tests" }
func (s *stubTelemetryTool) Schema() tools.ToolSchema              { return tools.ToolSchema{Type: "object"} }
func (s *stubTelemetryTool) Category() tools.ToolCategory          { return s.category }
func (s *stubTelemetryTool) Validate(map[string]interface{}) error { return nil }
func (s *stubTelemetryTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	return s.result, nil
}

// captureTelemetryStdout swaps the telemetry package's stdoutWriter with a
// buffer (via the test-only SetStdoutWriterForTest hook gated by the
// `testing_export` build tag), runs fn, restores the original writer, and
// returns whatever fn wrote. Used in lieu of captureStdout (which is
// package-private to telemetry/_test.go).
func captureTelemetryStdout(t *testing.T, fn func()) []byte {
	t.Helper()
	var buf bytes.Buffer
	old := telemetry.SetStdoutWriterForTest(&buf)
	defer telemetry.SetStdoutWriterForTest(old)
	fn()
	return append([]byte(nil), buf.Bytes()...)
}

func newRegistryForTelemetry(t *testing.T) *tools.ToolRegistry {
	t.Helper()
	workspace := t.TempDir()
	cfg := tools.DefaultRegistryConfig()
	cfg.FileSystemConfig.WorkspaceRoot = workspace
	cfg.ShellConfig.WorkDir = workspace
	r, err := tools.NewToolRegistry(cfg)
	if err != nil {
		t.Fatalf("NewToolRegistry: %v", err)
	}
	t.Cleanup(func() { _ = r.Close() })
	return r
}

// SetTelemetryInstrumentation accepts nil and disables instrumentation.
func TestToolRegistry_SetTelemetryInstrumentation_AcceptsNilDisables(t *testing.T) {
	r := newRegistryForTelemetry(t)

	// Pre-set a real instrumentation, then clear with nil.
	tp, _ := telemetry.NewTelemetryProvider(telemetry.TelemetryConfig{Enabled: false, Exporter: telemetry.ExporterNoop}, zap.NewNop())
	ti, err := telemetry.NewToolInstrumentation(tp)
	if err != nil {
		t.Fatalf("NewToolInstrumentation: %v", err)
	}
	r.SetTelemetryInstrumentation(ti)
	r.SetTelemetryInstrumentation(nil) // must accept nil without panic

	// Register a stub tool and call Execute — must not panic.
	stub := &stubTelemetryTool{name: "tele_stub_a", category: tools.CategoryFileSystem, result: "ok"}
	r.Register(stub)

	res, err := r.Execute(context.Background(), "tele_stub_a", map[string]interface{}{})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res != "ok" {
		t.Errorf("Execute result = %v, want %q", res, "ok")
	}
	if stub.calls != 1 {
		t.Errorf("stub call count = %d, want 1", stub.calls)
	}
}

// With telemetry wired, Execute records a real span carrying the tool name.
func TestToolRegistry_Execute_WithTelemetry_RecordsSpan(t *testing.T) {
	cfg := telemetry.TelemetryConfig{
		Enabled:      true,
		Exporter:     telemetry.ExporterStdout,
		ServiceName:  "registry-tele",
		BatchTimeout: 50 * time.Millisecond,
	}
	captured := captureTelemetryStdout(t, func() {
		tp, err := telemetry.NewTelemetryProvider(cfg, zap.NewNop())
		if err != nil {
			t.Fatalf("provider: %v", err)
		}
		ti, err := telemetry.NewToolInstrumentation(tp)
		if err != nil {
			t.Fatalf("NewToolInstrumentation: %v", err)
		}

		r := newRegistryForTelemetry(t)
		r.SetTelemetryInstrumentation(ti)

		stub := &stubTelemetryTool{name: "tele_stub_b", category: tools.CategoryFileSystem, result: "ok"}
		r.Register(stub)

		_, err = r.Execute(context.Background(), "tele_stub_b", map[string]interface{}{})
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = tp.ForceFlush(ctx)
		_ = tp.Shutdown(ctx)
	})

	out := string(captured)
	if !strings.Contains(out, "tool.tele_stub_b") {
		t.Errorf("captured stdout missing span name 'tool.tele_stub_b'. Got:\n%s", out)
	}
	if !strings.Contains(out, "filesystem") {
		t.Errorf("captured stdout missing tool.category 'filesystem'. Got:\n%s", out)
	}
	if !strings.Contains(out, "Ok") {
		t.Errorf("captured stdout missing Ok status for successful Execute. Got:\n%s", out)
	}
}

// On tool error, the span status is Error and the description carries the
// error message.
func TestToolRegistry_Execute_WithTelemetry_FailureStatusOnError(t *testing.T) {
	cfg := telemetry.TelemetryConfig{
		Enabled:      true,
		Exporter:     telemetry.ExporterStdout,
		ServiceName:  "registry-tele-err",
		BatchTimeout: 50 * time.Millisecond,
	}
	wantErr := errors.New("tool went boom")

	captured := captureTelemetryStdout(t, func() {
		tp, _ := telemetry.NewTelemetryProvider(cfg, zap.NewNop())
		ti, _ := telemetry.NewToolInstrumentation(tp)

		r := newRegistryForTelemetry(t)
		r.SetTelemetryInstrumentation(ti)

		stub := &stubTelemetryTool{name: "tele_stub_c", category: tools.CategoryShell, err: wantErr}
		r.Register(stub)

		_, err := r.Execute(context.Background(), "tele_stub_c", map[string]interface{}{})
		if err == nil || err.Error() != wantErr.Error() {
			t.Errorf("Execute err = %v, want %v", err, wantErr)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = tp.ForceFlush(ctx)
		_ = tp.Shutdown(ctx)
	})

	out := string(captured)
	if !strings.Contains(out, "tool.tele_stub_c") {
		t.Errorf("captured stdout missing span name 'tool.tele_stub_c'. Got:\n%s", out)
	}
	if !strings.Contains(out, "Error") {
		t.Errorf("captured stdout missing Error status. Got:\n%s", out)
	}
	if !strings.Contains(out, "tool went boom") {
		t.Errorf("captured stdout missing error description 'tool went boom'. Got:\n%s", out)
	}
}
