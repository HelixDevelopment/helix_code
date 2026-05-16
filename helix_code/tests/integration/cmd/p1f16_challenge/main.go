// p1f16_challenge runs the F16 OpenTelemetry integration end-to-end against
// real OTel SDK exporters: a real stdout exporter (Phase A + Phase C), a real
// OTLP/HTTP exporter pointed at an in-process httptest receiver (Phase B), the
// real noop fast path (Phase D), and an optional real collector if the
// operator points OTEL_EXPORTER_OTLP_ENDPOINT at one (Phase E). Runtime-
// evidence harness for the P1-F16 Challenge per Article XI §11.9.
//
// Phases:
//
//	A. STDOUT exporter end-to-end (always runs) — LoadConfigFromEnv +
//	   real TelemetryProvider + real TracedLLMProvider over a
//	   subagent.FakeLLMProvider; assert captured stdout contains the
//	   "llm.Generate" span name + the configured llm.model attribute +
//	   the helixcode_llm_calls_total metric.
//	B. FAKE OTLP/HTTP receiver (always runs) — start an httptest.Server
//	   that records POSTs to /v1/traces and /v1/metrics; build a real
//	   OTLP/HTTP TelemetryProvider against that endpoint; force-flush;
//	   assert at least one /v1/traces POST with non-empty body landed.
//	C. SECRET-ATTRIBUTE FILTER (always runs) — stdout exporter; Generate
//	   with a prompt containing the marker `API_KEY=sk-CHALLENGE-12345`;
//	   force-flush; assert the captured stdout contains the
//	   "llm.Generate" span (proves export happened) AND does NOT contain
//	   the secret marker (proves CONST-042 anti-leak).
//	D. NOOP zero-cost (always runs) — telemetry disabled (no env vars);
//	   construct provider, wrap, call Generate 100 times; assert
//	   provider.Exporter()==ExporterNoop and no stdout was written.
//	E. REAL collector (gated) — if OTEL_EXPORTER_OTLP_ENDPOINT is set
//	   AND reachable (TCP dial test), build OTLP/HTTP provider, emit a
//	   span, ForceFlush, print "real collector phase: span dispatched
//	   to <endpoint>". Else: print the gated-skip line.
//
// Build-tag note: the harness binary MUST be built with
// `-tags=testing_export` so it compiles against telemetry's
// SetStdoutWriterForTest seam (Phase A + Phase C + Phase D rely on a
// captured stdout buffer rather than redirecting os.Stdout itself).
//
// Exit code 0 on success; exit 1 with a diagnostic on any check failure.
//
//go:build testing_export
package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"dev.helix.code/internal/agent/subagent"
	"dev.helix.code/internal/llm"
	"dev.helix.code/internal/telemetry"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
}

func run() error {
	fmt.Println("==> P1-F16 challenge harness pid:", os.Getpid())

	if err := phaseA(); err != nil {
		return fmt.Errorf("phase A: %w", err)
	}
	if err := phaseB(); err != nil {
		return fmt.Errorf("phase B: %w", err)
	}
	if err := phaseC(); err != nil {
		return fmt.Errorf("phase C: %w", err)
	}
	if err := phaseD(); err != nil {
		return fmt.Errorf("phase D: %w", err)
	}
	if err := phaseE(); err != nil {
		return fmt.Errorf("phase E: %w", err)
	}

	fmt.Println("==> ALL CHECKS PASSED")
	fmt.Println("==> P1-F16 challenge harness PASS")
	return nil
}

// phaseA exercises the real OTel stdout exporter end-to-end. We swap the
// telemetry package's stdoutWriter for a captured buffer (via the
// testing_export build-tag seam), construct a TelemetryProvider from the
// env-var path (HELIXCODE_OTEL_EXPORTER=stdout), wrap a FakeLLMProvider in
// TracedLLMProvider, run Generate, ForceFlush, and assert the captured
// buffer contains the "llm.Generate" span name + the request's model
// attribute + the calls_total metric. The buffer evidence is the load-
// bearing positive-runtime-evidence anchor for this phase.
func phaseA() error {
	fmt.Println("==> phase A: STDOUT exporter end-to-end (always runs)")

	envs := map[string]string{
		"HELIXCODE_OTEL_EXPORTER": "stdout",
		"OTEL_SERVICE_NAME":       "p1f16-phase-a",
		"OTEL_BSP_SCHEDULE_DELAY": "50",
	}
	cfg, err := telemetry.LoadConfigFromEnv(envFromMap(envs))
	if err != nil {
		return fmt.Errorf("LoadConfigFromEnv: %w", err)
	}
	if cfg.Exporter != telemetry.ExporterStdout {
		return fmt.Errorf("LoadConfigFromEnv: cfg.Exporter=%q want %q", cfg.Exporter, telemetry.ExporterStdout)
	}
	if !cfg.Enabled {
		return fmt.Errorf("LoadConfigFromEnv: cfg.Enabled=false; want true for stdout exporter")
	}

	captured, restore := captureTelemetryStdout()
	defer restore()

	tp, err := telemetry.NewTelemetryProvider(cfg, zap.NewNop())
	if err != nil {
		return fmt.Errorf("NewTelemetryProvider: %w", err)
	}
	defer tp.Shutdown(context.Background())

	inner := subagent.NewFakeLLMProvider(map[string]string{
		"phase-A-prompt": "phase-A-output",
	})
	wrap, err := telemetry.NewTracedLLMProvider(inner, tp)
	if err != nil {
		return fmt.Errorf("NewTracedLLMProvider: %w", err)
	}

	const phaseAModel = "phase-A-model"
	resp, err := wrap.Generate(context.Background(), &llm.LLMRequest{
		Model: phaseAModel,
		Messages: []llm.Message{
			{Role: "user", Content: "phase-A-prompt"},
		},
	})
	if err != nil {
		return fmt.Errorf("Generate: %w", err)
	}
	if resp == nil || resp.Content != "phase-A-output" {
		return fmt.Errorf("Generate response unexpected: %+v", resp)
	}
	if got := inner.GenerateCallCount(); got != 1 {
		return fmt.Errorf("inner FakeLLMProvider.GenerateCallCount=%d want 1", got)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := tp.ForceFlush(ctx); err != nil {
		return fmt.Errorf("ForceFlush: %w", err)
	}

	out := captured.String()
	if !strings.Contains(out, "llm.Generate") {
		return fmt.Errorf("captured stdout missing span name 'llm.Generate'.\n--- captured ---\n%s", out)
	}
	if !strings.Contains(out, "llm.model") {
		return fmt.Errorf("captured stdout missing 'llm.model' attribute key.\n--- captured ---\n%s", out)
	}
	if !strings.Contains(out, phaseAModel) {
		return fmt.Errorf("captured stdout missing model value %q.\n--- captured ---\n%s", phaseAModel, out)
	}
	if !strings.Contains(out, "helixcode_llm_calls_total") {
		return fmt.Errorf("captured stdout missing metric 'helixcode_llm_calls_total'.\n--- captured ---\n%s", out)
	}

	snippet := firstLineContaining(out, "llm.Generate", 240)
	fmt.Printf("    exporter         : stdout\n")
	fmt.Printf("    captured_bytes   : %d\n", len(out))
	fmt.Printf("    span_evidence    : %s\n", snippet)
	fmt.Printf("    metric_evidence  : present (helixcode_llm_calls_total)\n")
	return nil
}

// phaseB exercises the real otlptracehttp + otlpmetrichttp exporters against
// an in-process httptest.Server. The receiver records every POST it sees so
// the harness can assert at least one /v1/traces POST with a non-empty body
// landed, which is REAL HTTP round-trip evidence (Article XI §11.9) — the
// OTel SDK actually serialised + dispatched protobuf bytes over a TCP socket.
func phaseB() error {
	fmt.Println("==> phase B: real OTLP/HTTP exporter into in-process fake receiver (always runs)")

	rec := newFakeOTLPReceiver()
	server := httptest.NewServer(rec)
	defer server.Close()

	parsed, err := url.Parse(server.URL)
	if err != nil {
		return fmt.Errorf("parse server URL: %w", err)
	}

	cfg := telemetry.TelemetryConfig{
		Enabled:       true,
		Exporter:      telemetry.ExporterOTLPHTTP,
		Endpoint:      server.URL,
		Insecure:      true,
		ServiceName:   "p1f16-phase-b",
		BatchTimeout:  50 * time.Millisecond,
		ExportTimeout: 5 * time.Second,
	}

	tp, err := telemetry.NewTelemetryProvider(cfg, zap.NewNop())
	if err != nil {
		return fmt.Errorf("NewTelemetryProvider: %w", err)
	}
	defer tp.Shutdown(context.Background())
	if got := tp.Exporter(); got != telemetry.ExporterOTLPHTTP {
		return fmt.Errorf("provider exporter=%q want %q", got, telemetry.ExporterOTLPHTTP)
	}

	inner := subagent.NewFakeLLMProvider(map[string]string{
		"phase-B-prompt": "phase-B-output",
	})
	wrap, err := telemetry.NewTracedLLMProvider(inner, tp)
	if err != nil {
		return fmt.Errorf("NewTracedLLMProvider: %w", err)
	}
	resp, err := wrap.Generate(context.Background(), &llm.LLMRequest{
		Model: "phase-B-model",
		Messages: []llm.Message{
			{Role: "user", Content: "phase-B-prompt"},
		},
	})
	if err != nil {
		return fmt.Errorf("Generate: %w", err)
	}
	if resp == nil || resp.Content != "phase-B-output" {
		return fmt.Errorf("Generate response unexpected: %+v", resp)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := tp.ForceFlush(ctx); err != nil {
		return fmt.Errorf("ForceFlush: %w", err)
	}
	// Shutdown the provider while the receiver is still alive so the metric
	// PeriodicReader gets a chance to drain its final reading.
	if err := tp.Shutdown(ctx); err != nil {
		return fmt.Errorf("Shutdown: %w", err)
	}

	traces := rec.snapshot("/v1/traces")
	metrics := rec.snapshot("/v1/metrics")

	if len(traces) == 0 {
		return fmt.Errorf("fake OTLP/HTTP receiver got 0 POSTs to /v1/traces; expected at least 1")
	}
	if traces[0].bodyLen == 0 {
		return fmt.Errorf("fake OTLP/HTTP receiver got /v1/traces POST with empty body")
	}

	fmt.Printf("    receiver_addr    : %s\n", parsed.Host)
	fmt.Printf("    traces_posts     : %d (first body bytes: %d)\n", len(traces), traces[0].bodyLen)
	fmt.Printf("    metrics_posts    : %d", len(metrics))
	if len(metrics) > 0 {
		fmt.Printf(" (first body bytes: %d)", metrics[0].bodyLen)
	}
	fmt.Println()
	return nil
}

// phaseC asserts the CONST-042 anti-leak path. We Generate with a prompt body
// containing a unique marker; after ForceFlush, the captured stdout buffer
// MUST NOT contain the marker (proving the prompt body never reached the
// exporter), AND MUST contain the "llm.Generate" span name (proving the
// export actually happened — i.e. the absence of the marker is filtering, not
// a no-op). This is the load-bearing CONST-042 evidence for the challenge.
func phaseC() error {
	fmt.Println("==> phase C: secret-attribute filter (always runs)")

	cfg := telemetry.TelemetryConfig{
		Enabled:      true,
		Exporter:     telemetry.ExporterStdout,
		ServiceName:  "p1f16-phase-c",
		BatchTimeout: 50 * time.Millisecond,
	}

	captured, restore := captureTelemetryStdout()
	defer restore()

	tp, err := telemetry.NewTelemetryProvider(cfg, zap.NewNop())
	if err != nil {
		return fmt.Errorf("NewTelemetryProvider: %w", err)
	}
	defer tp.Shutdown(context.Background())

	inner := subagent.NewFakeLLMProvider(nil)
	wrap, err := telemetry.NewTracedLLMProvider(inner, tp)
	if err != nil {
		return fmt.Errorf("NewTracedLLMProvider: %w", err)
	}

	const secretMarker = "API_KEY=sk-CHALLENGE-12345"
	if _, err := wrap.Generate(context.Background(), &llm.LLMRequest{
		Model: "phase-C-model",
		Messages: []llm.Message{
			{Role: "system", Content: "system message containing " + secretMarker + " inline"},
			{Role: "user", Content: "user message also referencing " + secretMarker},
		},
	}); err != nil {
		return fmt.Errorf("Generate: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := tp.ForceFlush(ctx); err != nil {
		return fmt.Errorf("ForceFlush: %w", err)
	}

	out := captured.String()
	if strings.Contains(out, secretMarker) {
		return fmt.Errorf("CONST-042 LEAK: captured stdout contains secret marker %q.\n--- captured ---\n%s", secretMarker, out)
	}
	if !strings.Contains(out, "llm.Generate") {
		return fmt.Errorf("captured stdout missing span name 'llm.Generate' — export must have happened to prove the marker absence is filtering, not silence.\n--- captured ---\n%s", out)
	}

	fmt.Printf("    captured_bytes   : %d\n", len(out))
	fmt.Printf("    span_present     : true (llm.Generate exported)\n")
	fmt.Printf("    secret_present   : false (marker %q absent)\n", secretMarker)
	fmt.Println("    secret-leak prevention verified")
	return nil
}

// phaseD verifies the noop fast path is genuinely zero-cost: an empty env
// resolves to ExporterNoop, the provider reports ExporterNoop back, and 100
// Generate calls write zero bytes to the (still-captured) stdout writer.
// Anti-bluff: the captured-buffer assertion proves no span object accidentally
// reached an exporter — a regression to "always-on stdout" would dump a
// flood into the buffer.
func phaseD() error {
	fmt.Println("==> phase D: noop zero-cost (always runs)")

	cfg, err := telemetry.LoadConfigFromEnv(envFromMap(nil))
	if err != nil {
		return fmt.Errorf("LoadConfigFromEnv (empty env): %w", err)
	}
	if cfg.Enabled {
		return fmt.Errorf("LoadConfigFromEnv (empty env): cfg.Enabled=true want false")
	}
	if cfg.Exporter != telemetry.ExporterNoop {
		return fmt.Errorf("LoadConfigFromEnv (empty env): cfg.Exporter=%q want %q", cfg.Exporter, telemetry.ExporterNoop)
	}

	captured, restore := captureTelemetryStdout()
	defer restore()

	tp, err := telemetry.NewTelemetryProvider(cfg, zap.NewNop())
	if err != nil {
		return fmt.Errorf("NewTelemetryProvider: %w", err)
	}
	defer tp.Shutdown(context.Background())
	if got := tp.Exporter(); got != telemetry.ExporterNoop {
		return fmt.Errorf("provider exporter=%q want %q", got, telemetry.ExporterNoop)
	}

	inner := subagent.NewFakeLLMProvider(nil)
	wrap, err := telemetry.NewTracedLLMProvider(inner, tp)
	if err != nil {
		return fmt.Errorf("NewTracedLLMProvider: %w", err)
	}

	const calls = 100
	start := time.Now()
	for i := 0; i < calls; i++ {
		if _, err := wrap.Generate(context.Background(), &llm.LLMRequest{
			Model: "phase-D-model",
			Messages: []llm.Message{
				{Role: "user", Content: "noop-call"},
			},
		}); err != nil {
			return fmt.Errorf("Generate #%d: %w", i, err)
		}
	}
	elapsed := time.Since(start)

	if got := inner.GenerateCallCount(); got != int64(calls) {
		return fmt.Errorf("inner Generate call count=%d want %d", got, calls)
	}

	if n := captured.Len(); n != 0 {
		return fmt.Errorf("noop provider unexpectedly wrote %d bytes to stdout writer:\n%s", n, captured.String())
	}

	fmt.Printf("    exporter         : %s\n", tp.Exporter())
	fmt.Printf("    calls            : %d\n", calls)
	fmt.Printf("    captured_bytes   : 0\n")
	fmt.Printf("    elapsed          : %s\n", elapsed)
	fmt.Println("    noop fast path: 100 calls completed without telemetry overhead")
	return nil
}

// phaseE attempts a real OTLP/HTTP collector round-trip when the operator
// points OTEL_EXPORTER_OTLP_ENDPOINT at one. Reachability is probed by a TCP
// dial against the endpoint's host:port; if the dial fails (or the env var
// is unset), we skip honestly per F11/F12/F13/F14/F15 precedent.
func phaseE() error {
	fmt.Println("==> phase E: real OTLP/HTTP collector round-trip (gated)")

	endpoint := strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"))
	if endpoint == "" {
		fmt.Println("    [skipped: OTEL_EXPORTER_OTLP_ENDPOINT not set]")
		return nil
	}

	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Host == "" {
		fmt.Printf("    [skipped: OTEL_EXPORTER_OTLP_ENDPOINT=%q is not a parseable URL: %v]\n", endpoint, err)
		return nil
	}
	host := parsed.Host
	if !strings.Contains(host, ":") {
		switch parsed.Scheme {
		case "http":
			host += ":80"
		case "https":
			host += ":443"
		default:
			host += ":4318"
		}
	}
	dialer := net.Dialer{Timeout: 2 * time.Second}
	conn, err := dialer.Dial("tcp", host)
	if err != nil {
		fmt.Printf("    [skipped: TCP dial %s failed: %v]\n", host, err)
		return nil
	}
	_ = conn.Close()

	cfg := telemetry.TelemetryConfig{
		Enabled:       true,
		Exporter:      telemetry.ExporterOTLPHTTP,
		Endpoint:      endpoint,
		Insecure:      parsed.Scheme != "https",
		ServiceName:   "p1f16-phase-e",
		BatchTimeout:  50 * time.Millisecond,
		ExportTimeout: 5 * time.Second,
	}
	tp, err := telemetry.NewTelemetryProvider(cfg, zap.NewNop())
	if err != nil {
		return fmt.Errorf("NewTelemetryProvider: %w", err)
	}
	defer tp.Shutdown(context.Background())

	tracer := tp.Tracer("p1f16/phase-e")
	_, span := tracer.Start(context.Background(), "p1f16.phase_e.real_collector")
	span.End()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := tp.ForceFlush(ctx); err != nil {
		return fmt.Errorf("ForceFlush: %w", err)
	}

	fmt.Printf("    real collector phase: span dispatched to %s\n", endpoint)
	return nil
}

// fakeOTLPReceiver is an in-process httptest handler that records each POST
// it receives, so the harness can prove the OTel SDK actually performed an
// HTTP round-trip (real evidence per Article XI §11.9).
type fakeOTLPReceiver struct {
	mu    sync.Mutex
	posts map[string][]recordedPost
}

type recordedPost struct {
	at      time.Time
	bodyLen int
}

func newFakeOTLPReceiver() *fakeOTLPReceiver {
	return &fakeOTLPReceiver{posts: make(map[string][]recordedPost)}
}

func (r *fakeOTLPReceiver) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	body, _ := io.ReadAll(req.Body)
	_ = req.Body.Close()
	r.mu.Lock()
	r.posts[req.URL.Path] = append(r.posts[req.URL.Path], recordedPost{
		at:      time.Now(),
		bodyLen: len(body),
	})
	r.mu.Unlock()
	w.Header().Set("Content-Type", "application/x-protobuf")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(nil)
}

func (r *fakeOTLPReceiver) snapshot(path string) []recordedPost {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]recordedPost, len(r.posts[path]))
	copy(out, r.posts[path])
	return out
}

// safeBuffer is a goroutine-safe writer that accumulates bytes for later
// inspection. The OTel SDK's stdouttrace exporter writes from a background
// flush goroutine, so a plain bytes.Buffer would race against the harness
// reading captured.String() in the assertion phase.
type safeBuffer struct {
	mu  sync.Mutex
	buf []byte
	// closed is set when restore() is called; further writes are dropped
	// rather than panicking, which protects against late SDK shutdown writes
	// arriving after the next phase swaps in its own buffer.
	closed atomic.Bool
}

func (b *safeBuffer) Write(p []byte) (int, error) {
	if b.closed.Load() {
		return len(p), nil
	}
	b.mu.Lock()
	b.buf = append(b.buf, p...)
	b.mu.Unlock()
	return len(p), nil
}

func (b *safeBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(b.buf)
}

func (b *safeBuffer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.buf)
}

// captureTelemetryStdout swaps the telemetry package's stdoutWriter (via the
// testing_export build-tag seam) for a goroutine-safe buffer. Returns the
// buffer + a restore func the caller must defer to put the original writer
// back. Multiple phases may stack captures; restore order is LIFO via defer.
func captureTelemetryStdout() (*safeBuffer, func()) {
	buf := &safeBuffer{}
	prev := telemetry.SetStdoutWriterForTest(buf)
	return buf, func() {
		buf.closed.Store(true)
		telemetry.SetStdoutWriterForTest(prev)
	}
}

// envFromMap returns an envLookup func that resolves keys from m (returning
// "" for misses). nil m yields a function that always returns "" — useful for
// the noop-path test where every OTEL_/HELIXCODE_ var must be absent.
func envFromMap(m map[string]string) func(string) string {
	return func(k string) string {
		if m == nil {
			return ""
		}
		return m[k]
	}
}

// firstLineContaining returns the first line of s that contains needle,
// truncated to maxLen bytes. Used to keep the on-screen evidence one-liner
// readable when the underlying span JSON is multi-kilobyte.
func firstLineContaining(s, needle string, maxLen int) string {
	for _, line := range strings.Split(s, "\n") {
		if strings.Contains(line, needle) {
			line = strings.TrimSpace(line)
			if len(line) > maxLen {
				return line[:maxLen] + "...(truncated)"
			}
			return line
		}
	}
	return "(no line containing " + needle + ")"
}
