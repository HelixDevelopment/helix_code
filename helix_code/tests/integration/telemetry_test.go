//go:build integration && testing_export

// Package-level test file: P1-F16-T10 telemetry integration tests.
//
// Build tag rationale:
//   - `integration` — these tests are part of the integration test surface
//     gated by the existing test-infra harness (make test-infra-up etc.).
//     Without this tag the file does not compile into the default `go test`
//     run, mirroring tests/integration/sandbox_test.go and subagent_test.go.
//   - `testing_export` — required so the telemetry package's
//     SetStdoutWriterForTest seam is reachable. Without it the seam is
//     compiled out and we'd have no way to capture real stdout exporter
//     bytes from a sibling test package.
//
// Run with:
//   cd HelixCode && go test -tags="integration testing_export" -run TestTelemetry_ ./tests/integration/...
//
// Anti-bluff anchors carried by these tests:
//   - Every PASS line below comes from a REAL OTel SDK pipeline emitting
//     spans / metrics through real exporters (stdout or OTLP). No fake
//     tracer, no fake exporter.
//   - TestTelemetry_TracedLLMProvider_DoesNotLeakPromptInExport is the
//     load-bearing CONST-042 (No-Secret-Leak) certification. A regression
//     that allowed prompt body or API key strings to flow into spans would
//     fail this test before the binary ever ships.
//   - The OTLP-gRPC/HTTP tests only run when a real reachable collector is
//     advertised via OTEL_EXPORTER_OTLP_ENDPOINT. They are SKIPPED with a
//     SKIP-OK marker otherwise — never falsely-green.
package integration

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"dev.helix.code/internal/agent/subagent"
	"dev.helix.code/internal/llm"
	"dev.helix.code/internal/telemetry"
)

// captureTelemetryStdout swaps the telemetry package's stdoutWriter with a
// buffer (via the test-only SetStdoutWriterForTest hook gated by the
// `testing_export` build tag) and returns whatever the OTel stdout exporters
// wrote during fn. Mirrors the helper in internal/tools/registry_telemetry_test.go.
func captureTelemetryStdout(t *testing.T, fn func()) []byte {
	t.Helper()
	var buf bytes.Buffer
	old := telemetry.SetStdoutWriterForTest(&buf)
	defer telemetry.SetStdoutWriterForTest(old)
	fn()
	return append([]byte(nil), buf.Bytes()...)
}

// withCleanOTELEnv unsets the standard OTEL_* env vars + HelixCode override
// so a parent shell that has them set doesn't poison a "no telemetry" test.
// t.Setenv("", "") is not allowed; we use os.Setenv and t.Cleanup to restore.
// t.Setenv with empty value works fine on modern Go but Unsetenv is the
// canonical "no value" form for our resolver.
func withCleanOTELEnv(t *testing.T) {
	t.Helper()
	keys := []string{
		"HELIXCODE_OTEL_EXPORTER",
		"OTEL_EXPORTER_OTLP_PROTOCOL",
		"OTEL_EXPORTER_OTLP_ENDPOINT",
		"OTEL_EXPORTER_OTLP_INSECURE",
		"OTEL_SERVICE_NAME",
		"OTEL_RESOURCE_ATTRIBUTES",
		"OTEL_TRACES_EXPORTER",
		"OTEL_BSP_SCHEDULE_DELAY",
		"OTEL_BSP_EXPORT_TIMEOUT",
	}
	saved := make(map[string]string, len(keys))
	for _, k := range keys {
		if v, ok := os.LookupEnv(k); ok {
			saved[k] = v
		}
		os.Unsetenv(k)
	}
	t.Cleanup(func() {
		for _, k := range keys {
			os.Unsetenv(k)
		}
		for k, v := range saved {
			os.Setenv(k, v)
		}
	})
}

// envLookupFromMap returns a closure that mimics os.Getenv against a fixed
// map. Used to drive LoadConfigFromEnv deterministically without touching
// process state.
func envLookupFromMap(m map[string]string) func(string) string {
	return func(k string) string {
		return m[k]
	}
}

// TestTelemetry_NoopByDefault — with no OTEL_* env set, LoadConfigFromEnv
// must yield Enabled=false / Exporter=Noop and NewTelemetryProvider must
// return a noop provider whose Exporter() reports ExporterNoop.
//
// Anti-bluff: we DO NOT just assert config equality; we construct the real
// provider and call Exporter() / Tracer() / Meter() on it.
func TestTelemetry_NoopByDefault(t *testing.T) {
	withCleanOTELEnv(t)

	cfg, err := telemetry.LoadConfigFromEnv(os.Getenv)
	require.NoError(t, err)
	require.False(t, cfg.Enabled, "default config must be Enabled=false when no env is set")
	require.Equal(t, telemetry.ExporterNoop, cfg.Exporter)

	tp, err := telemetry.NewTelemetryProvider(cfg, zap.NewNop())
	require.NoError(t, err)
	require.NotNil(t, tp, "noop provider must not be nil")
	require.Equal(t, telemetry.ExporterNoop, tp.Exporter())
	require.NotNil(t, tp.Tracer("default-test"), "noop tracer must be a real (no-op) tracer")
	require.NotNil(t, tp.Meter("default-test"), "noop meter must be a real (no-op) meter")

	require.NoError(t, tp.ForceFlush(context.Background()))
	require.NoError(t, tp.Shutdown(context.Background()))
}

// TestTelemetry_StdoutEndToEnd — end-to-end flow through a real stdout
// exporter pipeline:
//
//   1. Set HELIXCODE_OTEL_EXPORTER=stdout via the env-lookup map.
//   2. LoadConfigFromEnv -> NewTelemetryProvider -> NewTracedLLMProvider.
//   3. Wrap a subagent.FakeLLMProvider (real llm.Provider impl) and call
//      Generate.
//   4. ForceFlush + Shutdown to drain pending spans.
//   5. Assert the captured stdout contains the model name and the
//      "llm.Generate" span name.
//
// Always runs (no gate). Proves the full main.go wiring path: env -> config
// -> provider -> decorator -> Generate -> stdout exporter.
func TestTelemetry_StdoutEndToEnd(t *testing.T) {
	envMap := map[string]string{
		"HELIXCODE_OTEL_EXPORTER": "stdout",
		"OTEL_SERVICE_NAME":       "helixcode-tele-stdout-e2e",
		"OTEL_BSP_SCHEDULE_DELAY": "50",
	}
	cfg, err := telemetry.LoadConfigFromEnv(envLookupFromMap(envMap))
	require.NoError(t, err)
	require.True(t, cfg.Enabled)
	require.Equal(t, telemetry.ExporterStdout, cfg.Exporter)

	captured := captureTelemetryStdout(t, func() {
		tp, err := telemetry.NewTelemetryProvider(cfg, zap.NewNop())
		require.NoError(t, err)
		require.Equal(t, telemetry.ExporterStdout, tp.Exporter())

		fake := subagent.NewFakeLLMProvider(map[string]string{
			"hello-telemetry": "stdout-end-to-end-canned",
		})
		traced, err := telemetry.NewTracedLLMProvider(fake, tp)
		require.NoError(t, err)

		req := &llm.LLMRequest{
			Model:     "stdout-test-model",
			MaxTokens: 64,
			Messages:  []llm.Message{{Role: "user", Content: "hello-telemetry"}},
		}
		resp, err := traced.Generate(context.Background(), req)
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.Equal(t, "stdout-end-to-end-canned", resp.Content)

		flushCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		require.NoError(t, tp.ForceFlush(flushCtx))
		require.NoError(t, tp.Shutdown(flushCtx))
	})

	out := string(captured)
	require.Contains(t, out, "llm.Generate", "captured stdout must contain the LLM span name. Got:\n%s", out)
	require.Contains(t, out, "stdout-test-model",
		"captured stdout must contain the model attribute. Got:\n%s", out)
}

// TestTelemetry_OTLPGRPCExporter_Gated — exercises the OTLP gRPC exporter
// against a real reachable collector. SKIPPED with a SKIP-OK marker when no
// collector is advertised so a green CI never lies about coverage.
func TestTelemetry_OTLPGRPCExporter_Gated(t *testing.T) {
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		t.Skip("SKIP-OK: P1-F16-T10 — OTEL_EXPORTER_OTLP_ENDPOINT not set; gRPC OTLP collector unavailable")
	}

	cfg := telemetry.TelemetryConfig{
		Enabled:      true,
		Exporter:     telemetry.ExporterOTLPGRPC,
		Endpoint:     endpoint,
		ServiceName:  "helixcode-tele-otlp-grpc",
		Insecure:     true,
		BatchTimeout: 100 * time.Millisecond,
	}
	tp, err := telemetry.NewTelemetryProvider(cfg, zap.NewNop())
	require.NoError(t, err)
	require.Equal(t, telemetry.ExporterOTLPGRPC, tp.Exporter())

	tracer := tp.Tracer("integration-test-grpc")
	_, span := tracer.Start(context.Background(), "otlp_grpc_smoke_span")
	span.End()

	flushCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := tp.ForceFlush(flushCtx); err != nil {
		t.Logf("ForceFlush returned %v (non-fatal — collector may have rejected; smoke is span emission)", err)
	}
	require.NoError(t, tp.Shutdown(flushCtx))
}

// TestTelemetry_OTLPHTTPExporter_Gated — exercises the OTLP HTTP/protobuf
// exporter against a real reachable collector. Same gating as the gRPC test.
func TestTelemetry_OTLPHTTPExporter_Gated(t *testing.T) {
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		t.Skip("SKIP-OK: P1-F16-T10 — OTEL_EXPORTER_OTLP_ENDPOINT not set; HTTP OTLP collector unavailable")
	}

	cfg := telemetry.TelemetryConfig{
		Enabled:      true,
		Exporter:     telemetry.ExporterOTLPHTTP,
		Endpoint:     endpoint,
		ServiceName:  "helixcode-tele-otlp-http",
		Insecure:     true,
		BatchTimeout: 100 * time.Millisecond,
	}
	tp, err := telemetry.NewTelemetryProvider(cfg, zap.NewNop())
	require.NoError(t, err)
	require.Equal(t, telemetry.ExporterOTLPHTTP, tp.Exporter())

	tracer := tp.Tracer("integration-test-http")
	_, span := tracer.Start(context.Background(), "otlp_http_smoke_span")
	span.End()

	flushCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := tp.ForceFlush(flushCtx); err != nil {
		t.Logf("ForceFlush returned %v (non-fatal — collector may have rejected; smoke is span emission)", err)
	}
	require.NoError(t, tp.Shutdown(flushCtx))
}

// TestTelemetry_TracedLLMProvider_DoesNotLeakPromptInExport — load-bearing
// CONST-042 (No-Secret-Leak) test.
//
// Threat model: a future contributor adds a span attribute that records the
// prompt body, or accidentally promotes message content to an attribute. The
// FilterAttributes seam is intended to catch that — this test proves it does.
//
// Method:
//   1. Real stdout exporter pipeline.
//   2. Prompt embeds a sentinel "API_KEY=sk-test-LEAK-CANARY-1234" — both an
//      api-key-like substring (denied by DefaultBlockedAttributeKeys) AND a
//      prompt body (denied because TracedLLMProvider does not record prompt
//      contents at all).
//   3. After Generate + ForceFlush, the captured stdout MUST NOT contain the
//      sentinel. Assertion is exact substring search.
//
// If this test ever fails, do not skip it. The decorator has regressed and
// the binary is leaking secrets. CONST-042 anchor.
func TestTelemetry_TracedLLMProvider_DoesNotLeakPromptInExport(t *testing.T) {
	const sentinel = "API_KEY=sk-test-LEAK-CANARY-1234"
	envMap := map[string]string{
		"HELIXCODE_OTEL_EXPORTER": "stdout",
		"OTEL_BSP_SCHEDULE_DELAY": "50",
	}
	cfg, err := telemetry.LoadConfigFromEnv(envLookupFromMap(envMap))
	require.NoError(t, err)
	require.True(t, cfg.Enabled)

	captured := captureTelemetryStdout(t, func() {
		tp, err := telemetry.NewTelemetryProvider(cfg, zap.NewNop())
		require.NoError(t, err)

		fake := subagent.NewFakeLLMProvider(nil)
		traced, err := telemetry.NewTracedLLMProvider(fake, tp)
		require.NoError(t, err)

		req := &llm.LLMRequest{
			Model:     "leak-canary-model",
			MaxTokens: 32,
			Messages: []llm.Message{
				{Role: "user", Content: "please summarise this token: " + sentinel},
			},
		}
		_, err = traced.Generate(context.Background(), req)
		require.NoError(t, err)

		flushCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		require.NoError(t, tp.ForceFlush(flushCtx))
		require.NoError(t, tp.Shutdown(flushCtx))
	})

	out := string(captured)
	require.NotContains(t, out, sentinel,
		"CONST-042 violation: prompt-borne sentinel %q leaked into exporter output. Captured:\n%s",
		sentinel, out)

	// Sanity check: the export pipeline DID emit *something* — otherwise the
	// "no leak" assertion above is vacuously true.
	require.Contains(t, out, "llm.Generate",
		"sanity: the exporter must have emitted the llm.Generate span (otherwise the leak check is vacuous). Got:\n%s",
		out)
}

// TestTelemetry_ToolInstrumentation_RecordsSpan — wire ToolInstrumentation
// against a stdout-backed provider, run a Begin/finish round-trip, and assert
// the span name "tool.fake_tool" appears in captured output.
//
// We do NOT go through ToolRegistry.Execute here (that surface is already
// covered by internal/tools/registry_telemetry_test.go). The point of THIS
// test is to prove the helper produced by NewToolInstrumentation actually
// emits spans through the production exporter pipeline when wired into the
// real provider.
func TestTelemetry_ToolInstrumentation_RecordsSpan(t *testing.T) {
	cfg := telemetry.TelemetryConfig{
		Enabled:      true,
		Exporter:     telemetry.ExporterStdout,
		ServiceName:  "helixcode-tele-tool-int",
		BatchTimeout: 50 * time.Millisecond,
	}

	captured := captureTelemetryStdout(t, func() {
		tp, err := telemetry.NewTelemetryProvider(cfg, zap.NewNop())
		require.NoError(t, err)

		ti, err := telemetry.NewToolInstrumentation(tp)
		require.NoError(t, err)

		_, finish := ti.Begin(context.Background(), "fake_tool", "fake_category")
		finish(nil)

		flushCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		require.NoError(t, tp.ForceFlush(flushCtx))
		require.NoError(t, tp.Shutdown(flushCtx))
	})

	out := string(captured)
	require.Contains(t, out, "tool.fake_tool",
		"captured stdout must contain the tool span name. Got:\n%s", out)
	require.Contains(t, out, "fake_category",
		"captured stdout must contain the tool.category attribute. Got:\n%s", out)
}

// TestTelemetry_AgentInstrumentation_RecordsSpan — wire AgentInstrumentation
// against a stdout-backed provider, drive BeginIteration/finish, and assert
// the span name "agent.iteration" appears in captured output.
func TestTelemetry_AgentInstrumentation_RecordsSpan(t *testing.T) {
	cfg := telemetry.TelemetryConfig{
		Enabled:      true,
		Exporter:     telemetry.ExporterStdout,
		ServiceName:  "helixcode-tele-agent-int",
		BatchTimeout: 50 * time.Millisecond,
	}

	captured := captureTelemetryStdout(t, func() {
		tp, err := telemetry.NewTelemetryProvider(cfg, zap.NewNop())
		require.NoError(t, err)

		ai, err := telemetry.NewAgentInstrumentation(tp)
		require.NoError(t, err)

		_, finish := ai.BeginIteration(context.Background(), 7, "task-tele-agent-int")
		finish(nil)

		flushCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		require.NoError(t, tp.ForceFlush(flushCtx))
		require.NoError(t, tp.Shutdown(flushCtx))
	})

	out := string(captured)
	require.Contains(t, out, "agent.iteration",
		"captured stdout must contain the agent.iteration span name. Got:\n%s", out)
	require.Contains(t, out, "task-tele-agent-int",
		"captured stdout must contain the agent.task_id attribute. Got:\n%s", out)
}

// TestTelemetry_Shutdown_FlushesPendingSpans — emit a span, then call ONLY
// Shutdown (no explicit ForceFlush). The captured stdout must still contain
// the span — proving Shutdown internally flushes pending batches.
//
// Anti-bluff: this test deliberately avoids the explicit ForceFlush call
// because in production main.go uses ONLY a deferred Shutdown. If Shutdown
// did not flush, every CLI run would silently lose its tail spans.
func TestTelemetry_Shutdown_FlushesPendingSpans(t *testing.T) {
	cfg := telemetry.TelemetryConfig{
		Enabled:      true,
		Exporter:     telemetry.ExporterStdout,
		ServiceName:  "helixcode-tele-shutdown-flush",
		BatchTimeout: 30 * time.Second, // long enough that auto-flush won't fire
	}

	captured := captureTelemetryStdout(t, func() {
		tp, err := telemetry.NewTelemetryProvider(cfg, zap.NewNop())
		require.NoError(t, err)

		tracer := tp.Tracer("shutdown-flush-test")
		_, span := tracer.Start(context.Background(), "pending_until_shutdown")
		span.End()

		// NOTE: NO ForceFlush here. Shutdown alone must drain.
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		require.NoError(t, tp.Shutdown(shutCtx))
	})

	out := string(captured)
	require.Contains(t, out, "pending_until_shutdown",
		"Shutdown must internally flush pending spans before tearing down. Got:\n%s", out)

	// Sanity: prove the test would catch a real flush regression by ensuring
	// the buffer is non-trivial (the OTel stdouttrace exporter writes a
	// fully-formed JSON envelope for each span).
	require.NotEqual(t, "", strings.TrimSpace(out),
		"captured stdout must be non-empty when a span has been emitted")
}
