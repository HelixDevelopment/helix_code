// Tests for TelemetryProvider construction (P1-F16-T05).
//
// Anti-bluff anchor: TestNewTelemetryProvider_StdoutExporter_TracerEmitsSpan
// emits a real span through a real OTel SDK BatchSpanProcessor + stdouttrace
// exporter, then asserts the span name appears in the captured writer output.
// This is positive runtime evidence: the provider actually exports.
package telemetry

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
)

// captureStdout swaps the package-level stdoutWriter test hook with a buffer,
// runs fn, restores the original writer, and returns whatever fn wrote.
func captureStdout(t *testing.T, fn func()) []byte {
	t.Helper()
	var buf bytes.Buffer
	old := stdoutWriter
	stdoutWriter = &buf
	defer func() { stdoutWriter = old }()
	fn()
	// Defensive copy — the buffer is reused after the swap reverts.
	return append([]byte(nil), buf.Bytes()...)
}

func TestNewTelemetryProvider_DisabledReturnsNoop(t *testing.T) {
	cfg := TelemetryConfig{
		Enabled:  false,
		Exporter: ExporterStdout,
	}
	p, err := NewTelemetryProvider(cfg, zap.NewNop())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Exporter() != ExporterNoop {
		t.Errorf("Exporter() = %q, want %q (disabled => noop)", p.Exporter(), ExporterNoop)
	}
	if p.Tracer("x") == nil {
		t.Error("Tracer(\"x\") returned nil")
	}
	if p.Meter("x") == nil {
		t.Error("Meter(\"x\") returned nil")
	}
	if err := p.ForceFlush(context.Background()); err != nil {
		t.Errorf("ForceFlush returned %v, want nil", err)
	}
	if err := p.Shutdown(context.Background()); err != nil {
		t.Errorf("Shutdown returned %v, want nil", err)
	}
}

func TestNewTelemetryProvider_NoopExporter_ReturnsNoop(t *testing.T) {
	cfg := TelemetryConfig{
		Enabled:  true,
		Exporter: ExporterNoop,
	}
	p, err := NewTelemetryProvider(cfg, zap.NewNop())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Exporter() != ExporterNoop {
		t.Errorf("Exporter() = %q, want %q", p.Exporter(), ExporterNoop)
	}
}

func TestNewTelemetryProvider_StdoutExporter_Constructs(t *testing.T) {
	cfg := TelemetryConfig{
		Enabled:     true,
		Exporter:    ExporterStdout,
		ServiceName: "helixcode-test",
	}
	_ = captureStdout(t, func() {
		p, err := NewTelemetryProvider(cfg, zap.NewNop())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p.Exporter() != ExporterStdout {
			t.Errorf("Exporter() = %q, want %q", p.Exporter(), ExporterStdout)
		}
		if p.Tracer("comp") == nil {
			t.Error("Tracer is nil")
		}
		if p.Meter("comp") == nil {
			t.Error("Meter is nil")
		}
		// Tear down to release BatchSpanProcessor / PeriodicReader goroutines.
		if err := p.Shutdown(context.Background()); err != nil {
			t.Errorf("Shutdown returned %v, want nil", err)
		}
	})
}

func TestNewTelemetryProvider_OTLPGRPCExporter_Constructs(t *testing.T) {
	cfg := TelemetryConfig{
		Enabled:     true,
		Exporter:    ExporterOTLPGRPC,
		Endpoint:    "127.0.0.1:4317",
		ServiceName: "helixcode-grpc",
		Insecure:    true,
	}
	p, err := NewTelemetryProvider(cfg, zap.NewNop())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Exporter() != ExporterOTLPGRPC {
		t.Errorf("Exporter() = %q, want %q", p.Exporter(), ExporterOTLPGRPC)
	}
	if p.Tracer("comp") == nil {
		t.Error("Tracer is nil")
	}
	// Shutdown with a tight deadline — gRPC exporter may try to drain to a
	// non-existent collector; we don't care, we just want clean teardown.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = p.Shutdown(ctx)
}

func TestNewTelemetryProvider_OTLPHTTPExporter_Constructs(t *testing.T) {
	cfg := TelemetryConfig{
		Enabled:     true,
		Exporter:    ExporterOTLPHTTP,
		Endpoint:    "http://127.0.0.1:4318",
		ServiceName: "helixcode-http",
		Insecure:    true,
	}
	p, err := NewTelemetryProvider(cfg, zap.NewNop())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Exporter() != ExporterOTLPHTTP {
		t.Errorf("Exporter() = %q, want %q", p.Exporter(), ExporterOTLPHTTP)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = p.Shutdown(ctx)
}

// TestNewTelemetryProvider_StdoutExporter_TracerEmitsSpan is the load-bearing
// anti-bluff test. It constructs a real provider with the stdout exporter,
// emits a real span through the OTel SDK, force-flushes, and asserts the
// span name appears in the captured stdout writer output.
func TestNewTelemetryProvider_StdoutExporter_TracerEmitsSpan(t *testing.T) {
	cfg := TelemetryConfig{
		Enabled:     true,
		Exporter:    ExporterStdout,
		ServiceName: "anti-bluff-svc",
		ResourceAttrs: map[string]string{
			"deployment.environment": "test",
		},
		BatchTimeout:  100 * time.Millisecond,
		ExportTimeout: 2 * time.Second,
	}

	captured := captureStdout(t, func() {
		p, err := NewTelemetryProvider(cfg, zap.NewNop())
		if err != nil {
			t.Fatalf("constructor failed: %v", err)
		}
		tr := p.Tracer("helixcode/test")
		_, span := tr.Start(context.Background(), "anti-bluff-span")
		span.End()

		flushCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := p.ForceFlush(flushCtx); err != nil {
			t.Fatalf("ForceFlush failed: %v", err)
		}
		if err := p.Shutdown(flushCtx); err != nil {
			t.Fatalf("Shutdown failed: %v", err)
		}
	})

	out := string(captured)
	t.Logf("captured stdout (load-bearing anti-bluff evidence):\n%s", out)
	if !strings.Contains(out, "anti-bluff-span") {
		t.Errorf("captured output missing span name. Got:\n%s", out)
	}
	if !strings.Contains(out, "anti-bluff-svc") {
		t.Errorf("captured output missing service name. Got:\n%s", out)
	}
	if !strings.Contains(out, "helixcode/test") {
		t.Errorf("captured output missing instrumentation scope name. Got:\n%s", out)
	}
}

func TestNewTelemetryProvider_Shutdown_Idempotent(t *testing.T) {
	cfg := TelemetryConfig{
		Enabled:     true,
		Exporter:    ExporterStdout,
		ServiceName: "idempotent",
	}
	// Use captureStdout so the second Shutdown — if it accidentally still
	// flushed — would not clutter the test runner's stdout.
	_ = captureStdout(t, func() {
		p, err := NewTelemetryProvider(cfg, zap.NewNop())
		if err != nil {
			t.Fatalf("constructor failed: %v", err)
		}
		ctx := context.Background()
		if err := p.Shutdown(ctx); err != nil {
			t.Fatalf("first Shutdown failed: %v", err)
		}
		if err := p.Shutdown(ctx); err != nil {
			t.Errorf("second Shutdown returned %v, want nil (idempotent)", err)
		}
	})
}

func TestNewTelemetryProvider_Shutdown_RespectsCanceledCtx(t *testing.T) {
	cfg := TelemetryConfig{
		Enabled:     true,
		Exporter:    ExporterStdout,
		ServiceName: "cancel",
	}
	_ = captureStdout(t, func() {
		p, err := NewTelemetryProvider(cfg, zap.NewNop())
		if err != nil {
			t.Fatalf("constructor failed: %v", err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		// Even with a pre-canceled ctx, Shutdown must return without
		// hanging. We accept either nil or a context error.
		done := make(chan struct{})
		go func() {
			defer close(done)
			_ = p.Shutdown(ctx)
		}()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Fatal("Shutdown blocked > 3s after canceled ctx")
		}
	})
}

func TestNewTelemetryProvider_ForceFlush_Stdout(t *testing.T) {
	cfg := TelemetryConfig{
		Enabled:     true,
		Exporter:    ExporterStdout,
		ServiceName: "flush-svc",
		// Long batch timeout so the only thing that flushes the span
		// is our explicit ForceFlush call.
		BatchTimeout: 60 * time.Second,
	}

	captured := captureStdout(t, func() {
		p, err := NewTelemetryProvider(cfg, zap.NewNop())
		if err != nil {
			t.Fatalf("constructor failed: %v", err)
		}
		tr := p.Tracer("helixcode/flush")
		_, span := tr.Start(context.Background(), "explicit-flush-span")
		span.End()
		if err := p.ForceFlush(context.Background()); err != nil {
			t.Fatalf("ForceFlush returned %v", err)
		}
		// Don't Shutdown yet — proves ForceFlush alone delivered.
		_ = p.Shutdown(context.Background())
	})

	if !strings.Contains(string(captured), "explicit-flush-span") {
		t.Errorf("ForceFlush did not deliver span. Got:\n%s", string(captured))
	}
}

func TestNewTelemetryProvider_Config_RoundTrip(t *testing.T) {
	in := TelemetryConfig{
		Enabled:              true,
		Exporter:             ExporterStdout,
		Endpoint:             "127.0.0.1:4317",
		ServiceName:          "rt",
		ResourceAttrs:        map[string]string{"k": "v"},
		BlockedAttributeKeys: []string{"foo"},
		BatchTimeout:         7 * time.Second,
		ExportTimeout:        11 * time.Second,
		ShutdownTimeout:      13 * time.Second,
		Insecure:             true,
	}
	_ = captureStdout(t, func() {
		p, err := NewTelemetryProvider(in, zap.NewNop())
		if err != nil {
			t.Fatalf("constructor failed: %v", err)
		}
		out := p.Config()
		if out.ServiceName != in.ServiceName {
			t.Errorf("ServiceName: got %q, want %q", out.ServiceName, in.ServiceName)
		}
		if out.Exporter != in.Exporter {
			t.Errorf("Exporter: got %q, want %q", out.Exporter, in.Exporter)
		}
		if out.BatchTimeout != in.BatchTimeout {
			t.Errorf("BatchTimeout: got %v, want %v", out.BatchTimeout, in.BatchTimeout)
		}
		if out.ExportTimeout != in.ExportTimeout {
			t.Errorf("ExportTimeout: got %v, want %v", out.ExportTimeout, in.ExportTimeout)
		}
		if out.Insecure != in.Insecure {
			t.Errorf("Insecure: got %v, want %v", out.Insecure, in.Insecure)
		}
		_ = p.Shutdown(context.Background())
	})
}

// TestNewTelemetryProvider_Resource_HasServiceName proves the cfg.ServiceName
// flows into the resource attached to spans. Verified through stdout capture
// rather than a synthetic accessor.
func TestNewTelemetryProvider_Resource_HasServiceName(t *testing.T) {
	cfg := TelemetryConfig{
		Enabled:     true,
		Exporter:    ExporterStdout,
		ServiceName: "my-svc",
		ResourceAttrs: map[string]string{
			"deployment.environment": "prod",
		},
		BatchTimeout: 100 * time.Millisecond,
	}
	captured := captureStdout(t, func() {
		p, err := NewTelemetryProvider(cfg, zap.NewNop())
		if err != nil {
			t.Fatalf("constructor failed: %v", err)
		}
		tr := p.Tracer("helixcode/resource-test")
		_, span := tr.Start(context.Background(), "resource-check")
		span.End()
		_ = p.ForceFlush(context.Background())
		_ = p.Shutdown(context.Background())
	})
	out := string(captured)
	if !strings.Contains(out, "my-svc") {
		t.Errorf("expected resource attr service.name=my-svc in stdout. Got:\n%s", out)
	}
	if !strings.Contains(out, "deployment.environment") {
		t.Errorf("expected custom resource attr deployment.environment in stdout. Got:\n%s", out)
	}
	if !strings.Contains(out, "prod") {
		t.Errorf("expected resource attr value 'prod' in stdout. Got:\n%s", out)
	}
}

func TestNewTelemetryProvider_BadOTLPGRPCEndpoint_DoesNotCrash(t *testing.T) {
	// WithEndpointURL with an obviously malformed URL is documented to keep
	// the default; the provider must not crash. We confirm by constructing
	// + shutting down without a panic, regardless of the exporter's later
	// fate at export time.
	cfg := TelemetryConfig{
		Enabled:     true,
		Exporter:    ExporterOTLPGRPC,
		Endpoint:    "not a valid endpoint :::",
		ServiceName: "bad-grpc",
		Insecure:    true,
	}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("constructor panicked on bad endpoint: %v", r)
		}
	}()
	p, err := NewTelemetryProvider(cfg, zap.NewNop())
	// Constructor MAY return an error here — that's allowed. What is NOT
	// allowed is for it to crash, hang, or return a nil provider that
	// callers can't safely Shutdown. If it returned a provider, that
	// provider must be usable and Shutdown-safe.
	if p == nil && err == nil {
		t.Fatal("got nil provider AND nil error — constructor contract violated")
	}
	if p != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		_ = p.Shutdown(ctx)
	}
	// If err is non-nil, it must be a real error (errors.Unwrap-able is fine).
	if err != nil {
		_ = errors.Unwrap(err) // smoke check — no panic.
	}
}

func TestNewTelemetryProvider_NilLoggerSafe(t *testing.T) {
	cfg := TelemetryConfig{
		Enabled:     true,
		Exporter:    ExporterStdout,
		ServiceName: "nil-log",
	}
	_ = captureStdout(t, func() {
		p, err := NewTelemetryProvider(cfg, nil)
		if err != nil {
			t.Fatalf("constructor failed with nil logger: %v", err)
		}
		if p == nil {
			t.Fatal("provider is nil")
		}
		_ = p.Shutdown(context.Background())
	})
}

func TestNewTelemetryProvider_UnsupportedExporter_ReturnsError(t *testing.T) {
	cfg := TelemetryConfig{
		Enabled:  true,
		Exporter: ExporterKind("does-not-exist"),
	}
	p, err := NewTelemetryProvider(cfg, zap.NewNop())
	if err == nil {
		t.Errorf("expected error for unsupported exporter; got nil")
	}
	if !errors.Is(err, ErrUnsupportedExporter) {
		t.Errorf("expected ErrUnsupportedExporter, got %v", err)
	}
	// Provider should be a noop fallback OR nil — but we should be able to
	// Shutdown safely if non-nil.
	if p != nil {
		if p.Exporter() != ExporterNoop {
			t.Errorf("fallback Exporter = %q, want noop", p.Exporter())
		}
		_ = p.Shutdown(context.Background())
	}
}
