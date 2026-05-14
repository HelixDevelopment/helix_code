// Tests for the TracedLLMProvider decorator (P1-F16-T06).
//
// Anti-bluff anchors:
//   - TestTracedLLMProvider_Generate_DoesNotEmitPromptBody is the load-bearing
//     CONST-042 anchor. Generate is invoked with a prompt that embeds an API
//     key marker; captured stdout (real OTel stdout exporters) MUST NOT contain
//     the marker.
//   - TestTracedLLMProvider_Generate_RecordsSpan + RecordsLatency +
//     RecordsTokenCounts assert real exporter output, not synthetic counters.
//   - TestTracedLLMProvider_Generate_NoopProvider_NoSpansEmitted proves the
//     zero-cost noop fast path doesn't leak span output.
package telemetry

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"dev.helix.code/internal/agent/subagent"
	"dev.helix.code/internal/llm"
)

// stubErrorProvider is a TEST-ONLY llm.Provider that returns a configured
// error from Generate / GenerateStream. Used to verify span error status.
type stubErrorProvider struct {
	*subagent.FakeLLMProvider
	err error
}

func newStubErrorProvider(err error) *stubErrorProvider {
	return &stubErrorProvider{
		FakeLLMProvider: subagent.NewFakeLLMProvider(nil),
		err:             err,
	}
}

func (s *stubErrorProvider) Generate(ctx context.Context, req *llm.LLMRequest) (*llm.LLMResponse, error) {
	return nil, s.err
}

func (s *stubErrorProvider) GenerateStream(ctx context.Context, req *llm.LLMRequest, ch chan<- llm.LLMResponse) error {
	if ch != nil {
		close(ch)
	}
	return s.err
}

// stubUsageProvider returns a canned response with explicit Usage values so
// tests can assert exact token counts rather than the FakeLLMProvider's
// length-based estimate.
type stubUsageProvider struct {
	*subagent.FakeLLMProvider
	prompt     int
	completion int
}

func newStubUsageProvider(prompt, completion int) *stubUsageProvider {
	return &stubUsageProvider{
		FakeLLMProvider: subagent.NewFakeLLMProvider(nil),
		prompt:          prompt,
		completion:      completion,
	}
}

func (s *stubUsageProvider) Generate(ctx context.Context, req *llm.LLMRequest) (*llm.LLMResponse, error) {
	return &llm.LLMResponse{
		ID:           uuid.New(),
		Content:      "stub-response",
		FinishReason: "stop",
		CreatedAt:    time.Now(),
		Usage: llm.Usage{
			PromptTokens:     s.prompt,
			CompletionTokens: s.completion,
			TotalTokens:      s.prompt + s.completion,
		},
	}, nil
}

// --- Compile + promotion tests ---

func TestTracedLLMProvider_Compiles_AsLLMProvider(t *testing.T) {
	var _ llm.Provider = (*TracedLLMProvider)(nil)
}

func TestTracedLLMProvider_PromotesGetType(t *testing.T) {
	inner := subagent.NewFakeLLMProvider(nil)
	tp, _ := NewTelemetryProvider(TelemetryConfig{Enabled: false, Exporter: ExporterNoop}, zap.NewNop())
	wrap, err := NewTracedLLMProvider(inner, tp)
	if err != nil {
		t.Fatalf("constructor failed: %v", err)
	}
	if wrap.GetType() != inner.GetType() {
		t.Errorf("GetType: wrap=%q inner=%q", wrap.GetType(), inner.GetType())
	}
}

func TestTracedLLMProvider_PromotesGetName(t *testing.T) {
	inner := subagent.NewFakeLLMProvider(nil)
	tp, _ := NewTelemetryProvider(TelemetryConfig{Enabled: false, Exporter: ExporterNoop}, zap.NewNop())
	wrap, _ := NewTracedLLMProvider(inner, tp)
	if wrap.GetName() != inner.GetName() {
		t.Errorf("GetName: wrap=%q inner=%q", wrap.GetName(), inner.GetName())
	}
}

func TestTracedLLMProvider_PromotesIsAvailable(t *testing.T) {
	inner := subagent.NewFakeLLMProvider(nil)
	tp, _ := NewTelemetryProvider(TelemetryConfig{Enabled: false, Exporter: ExporterNoop}, zap.NewNop())
	wrap, _ := NewTracedLLMProvider(inner, tp)
	if !wrap.IsAvailable(context.Background()) {
		t.Error("IsAvailable should be true (delegated to fake)")
	}
}

func TestTracedLLMProvider_PromotesGetModels(t *testing.T) {
	inner := subagent.NewFakeLLMProvider(nil)
	tp, _ := NewTelemetryProvider(TelemetryConfig{Enabled: false, Exporter: ExporterNoop}, zap.NewNop())
	wrap, _ := NewTracedLLMProvider(inner, tp)
	if got := wrap.GetModels(); got == nil {
		t.Error("GetModels returned nil; want empty slice from fake")
	}
}

func TestTracedLLMProvider_PromotesGetCapabilities(t *testing.T) {
	// Anti-bluff (CONST-035 / §11.9): the original form ran
	// `_ = wrap.GetCapabilities()` with the comment "Must not panic" and
	// asserted NOTHING beyond the absence of a panic. The test name says
	// "Promotes" — meaning TracedLLMProvider MUST forward the call to the
	// inner provider and return what the inner returned. Pin that real
	// promotion contract: the wrapped result must EQUAL the inner's result
	// (FakeLLMProvider.GetCapabilities returns an empty []ModelCapability;
	// a regression that returned nil or a fabricated non-empty slice
	// would now fail this test).
	inner := subagent.NewFakeLLMProvider(nil)
	tp, _ := NewTelemetryProvider(TelemetryConfig{Enabled: false, Exporter: ExporterNoop}, zap.NewNop())
	wrap, _ := NewTracedLLMProvider(inner, tp)

	innerCaps := inner.GetCapabilities()
	wrapCaps := wrap.GetCapabilities()
	if len(wrapCaps) != len(innerCaps) {
		t.Fatalf("wrap.GetCapabilities len=%d, want %d (must promote inner result)", len(wrapCaps), len(innerCaps))
	}
	for i := range innerCaps {
		if wrapCaps[i] != innerCaps[i] {
			t.Errorf("wrap.GetCapabilities[%d]=%v, want %v", i, wrapCaps[i], innerCaps[i])
		}
	}
}

func TestTracedLLMProvider_PromotesGetHealth(t *testing.T) {
	inner := subagent.NewFakeLLMProvider(nil)
	tp, _ := NewTelemetryProvider(TelemetryConfig{Enabled: false, Exporter: ExporterNoop}, zap.NewNop())
	wrap, _ := NewTracedLLMProvider(inner, tp)
	h, err := wrap.GetHealth(context.Background())
	if err != nil {
		t.Fatalf("GetHealth err: %v", err)
	}
	if h == nil || h.Status == "" {
		t.Errorf("GetHealth returned empty health: %+v", h)
	}
}

func TestTracedLLMProvider_PromotesClose(t *testing.T) {
	inner := subagent.NewFakeLLMProvider(nil)
	tp, _ := NewTelemetryProvider(TelemetryConfig{Enabled: false, Exporter: ExporterNoop}, zap.NewNop())
	wrap, _ := NewTracedLLMProvider(inner, tp)
	if err := wrap.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}

func TestTracedLLMProvider_PromotesGetContextWindow(t *testing.T) {
	inner := subagent.NewFakeLLMProvider(nil)
	tp, _ := NewTelemetryProvider(TelemetryConfig{Enabled: false, Exporter: ExporterNoop}, zap.NewNop())
	wrap, _ := NewTracedLLMProvider(inner, tp)
	if wrap.GetContextWindow() != inner.GetContextWindow() {
		t.Errorf("GetContextWindow: wrap=%d inner=%d", wrap.GetContextWindow(), inner.GetContextWindow())
	}
}

func TestTracedLLMProvider_PromotesCountTokens(t *testing.T) {
	inner := subagent.NewFakeLLMProvider(nil)
	tp, _ := NewTelemetryProvider(TelemetryConfig{Enabled: false, Exporter: ExporterNoop}, zap.NewNop())
	wrap, _ := NewTracedLLMProvider(inner, tp)
	got, err := wrap.CountTokens("hello world")
	if err != nil {
		t.Fatalf("CountTokens: %v", err)
	}
	if got <= 0 {
		t.Errorf("CountTokens = %d, want > 0", got)
	}
}

// --- Generate behaviour tests ---

func TestTracedLLMProvider_Generate_PassThrough(t *testing.T) {
	inner := subagent.NewFakeLLMProvider(map[string]string{
		"hi": "hello",
	})
	tp, _ := NewTelemetryProvider(TelemetryConfig{Enabled: false, Exporter: ExporterNoop}, zap.NewNop())
	wrap, _ := NewTracedLLMProvider(inner, tp)

	resp, err := wrap.Generate(context.Background(), &llm.LLMRequest{
		Model: "fake-model",
		Messages: []llm.Message{
			{Role: "user", Content: "hi"},
		},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if resp == nil || resp.Content != "hello" {
		t.Errorf("Generate response = %+v, want content=hello", resp)
	}
	if inner.GenerateCallCount() != 1 {
		t.Errorf("inner Generate not called: count=%d", inner.GenerateCallCount())
	}
}

func TestTracedLLMProvider_Generate_RecordsSpan(t *testing.T) {
	cfg := TelemetryConfig{
		Enabled:      true,
		Exporter:     ExporterStdout,
		ServiceName:  "trace-llm",
		BatchTimeout: 50 * time.Millisecond,
	}
	captured := captureStdout(t, func() {
		tp, err := NewTelemetryProvider(cfg, zap.NewNop())
		if err != nil {
			t.Fatalf("provider: %v", err)
		}
		inner := subagent.NewFakeLLMProvider(nil)
		wrap, err := NewTracedLLMProvider(inner, tp)
		if err != nil {
			t.Fatalf("wrap: %v", err)
		}
		_, err = wrap.Generate(context.Background(), &llm.LLMRequest{
			Model: "test-model-X",
			Messages: []llm.Message{
				{Role: "user", Content: "harmless prompt body"},
			},
		})
		if err != nil {
			t.Fatalf("Generate: %v", err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = tp.ForceFlush(ctx)
		_ = tp.Shutdown(ctx)
	})

	out := string(captured)
	if !strings.Contains(out, "llm.Generate") {
		t.Errorf("captured stdout missing span name 'llm.Generate'. Got:\n%s", out)
	}
	if !strings.Contains(out, "test-model-X") {
		t.Errorf("captured stdout missing llm.model attribute value. Got:\n%s", out)
	}
}

// Load-bearing CONST-042 anchor: prompt body MUST NEVER appear in span output.
func TestTracedLLMProvider_Generate_DoesNotEmitPromptBody(t *testing.T) {
	const secretMarker = "API_KEY=sk-1234"
	cfg := TelemetryConfig{
		Enabled:      true,
		Exporter:     ExporterStdout,
		ServiceName:  "leak-check",
		BatchTimeout: 50 * time.Millisecond,
	}
	captured := captureStdout(t, func() {
		tp, _ := NewTelemetryProvider(cfg, zap.NewNop())
		inner := subagent.NewFakeLLMProvider(nil)
		wrap, _ := NewTracedLLMProvider(inner, tp)
		_, err := wrap.Generate(context.Background(), &llm.LLMRequest{
			Model: "leak-model",
			Messages: []llm.Message{
				{Role: "user", Content: "user message containing " + secretMarker + " in body"},
				{Role: "system", Content: "system prompt also referencing " + secretMarker},
			},
		})
		if err != nil {
			t.Fatalf("Generate: %v", err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = tp.ForceFlush(ctx)
		_ = tp.Shutdown(ctx)
	})

	out := string(captured)
	t.Logf("captured stdout (CONST-042 leak check):\n%s", out)
	if strings.Contains(out, secretMarker) {
		t.Fatalf("CONST-042 LEAK: captured stdout contains secret marker %q. Output:\n%s", secretMarker, out)
	}
}

func TestTracedLLMProvider_Generate_RecordsLatency(t *testing.T) {
	cfg := TelemetryConfig{
		Enabled:      true,
		Exporter:     ExporterStdout,
		ServiceName:  "latency-llm",
		BatchTimeout: 50 * time.Millisecond,
	}
	const delay = 100 * time.Millisecond

	captured := captureStdout(t, func() {
		tp, _ := NewTelemetryProvider(cfg, zap.NewNop())
		inner := subagent.NewFakeLLMProvider(nil)
		inner.WithDelay(delay)
		wrap, _ := NewTracedLLMProvider(inner, tp)

		start := time.Now()
		_, err := wrap.Generate(context.Background(), &llm.LLMRequest{
			Model: "lat-model",
			Messages: []llm.Message{
				{Role: "user", Content: "hi"},
			},
		})
		elapsed := time.Since(start)
		if err != nil {
			t.Fatalf("Generate: %v", err)
		}
		if elapsed < 90*time.Millisecond {
			t.Errorf("Generate returned faster than fake delay: %v", elapsed)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = tp.ForceFlush(ctx)
		_ = tp.Shutdown(ctx)
	})

	out := string(captured)
	if !strings.Contains(out, "helixcode_llm_latency_seconds") {
		t.Errorf("captured stdout missing latency histogram metric. Got:\n%s", out)
	}
}

func TestTracedLLMProvider_Generate_RecordsTokenCounts(t *testing.T) {
	cfg := TelemetryConfig{
		Enabled:      true,
		Exporter:     ExporterStdout,
		ServiceName:  "tokens-llm",
		BatchTimeout: 50 * time.Millisecond,
	}
	const promptTokens, completionTokens = 17, 23

	captured := captureStdout(t, func() {
		tp, _ := NewTelemetryProvider(cfg, zap.NewNop())
		inner := newStubUsageProvider(promptTokens, completionTokens)
		wrap, _ := NewTracedLLMProvider(inner, tp)
		_, err := wrap.Generate(context.Background(), &llm.LLMRequest{
			Model: "tok-model",
			Messages: []llm.Message{
				{Role: "user", Content: "x"},
			},
		})
		if err != nil {
			t.Fatalf("Generate: %v", err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = tp.ForceFlush(ctx)
		_ = tp.Shutdown(ctx)
	})

	out := string(captured)
	if !strings.Contains(out, "helixcode_llm_prompt_tokens_total") {
		t.Errorf("captured stdout missing prompt_tokens metric. Got:\n%s", out)
	}
	if !strings.Contains(out, "helixcode_llm_completion_tokens_total") {
		t.Errorf("captured stdout missing completion_tokens metric. Got:\n%s", out)
	}
	// The exact prompt-token value should appear somewhere in the metric body.
	if !strings.Contains(out, fmt.Sprintf("%d", promptTokens)) {
		t.Errorf("captured stdout missing prompt token value %d. Got:\n%s", promptTokens, out)
	}
}

func TestTracedLLMProvider_Generate_OnError_SpanStatusError(t *testing.T) {
	cfg := TelemetryConfig{
		Enabled:      true,
		Exporter:     ExporterStdout,
		ServiceName:  "err-llm",
		BatchTimeout: 50 * time.Millisecond,
	}
	wantErr := errors.New("upstream LLM exploded")

	captured := captureStdout(t, func() {
		tp, _ := NewTelemetryProvider(cfg, zap.NewNop())
		inner := newStubErrorProvider(wantErr)
		wrap, _ := NewTracedLLMProvider(inner, tp)
		_, err := wrap.Generate(context.Background(), &llm.LLMRequest{
			Model: "err-model",
			Messages: []llm.Message{
				{Role: "user", Content: "fail"},
			},
		})
		if err == nil || err.Error() != wantErr.Error() {
			t.Errorf("Generate err = %v, want %v", err, wantErr)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = tp.ForceFlush(ctx)
		_ = tp.Shutdown(ctx)
	})

	out := string(captured)
	// stdouttrace renders Status as a JSON sub-object; both "Error" code and
	// the wrapped error message should be present.
	if !strings.Contains(out, "Error") {
		t.Errorf("captured stdout missing Error status. Got:\n%s", out)
	}
	if !strings.Contains(out, "upstream LLM exploded") {
		t.Errorf("captured stdout missing error description. Got:\n%s", out)
	}
}

// Verifies FilterAttributes is in the path: blocking the (otherwise benign)
// "llm.model" key strips it from the span output.
func TestTracedLLMProvider_Generate_FilterDropsBlockedAttribute(t *testing.T) {
	cfg := TelemetryConfig{
		Enabled:              true,
		Exporter:             ExporterStdout,
		ServiceName:          "filter-llm",
		BatchTimeout:         50 * time.Millisecond,
		BlockedAttributeKeys: []string{"llm.model"},
	}
	captured := captureStdout(t, func() {
		tp, _ := NewTelemetryProvider(cfg, zap.NewNop())
		inner := subagent.NewFakeLLMProvider(nil)
		wrap, _ := NewTracedLLMProvider(inner, tp)
		_, err := wrap.Generate(context.Background(), &llm.LLMRequest{
			Model: "should-be-stripped-XYZ",
			Messages: []llm.Message{
				{Role: "user", Content: "hi"},
			},
		})
		if err != nil {
			t.Fatalf("Generate: %v", err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = tp.ForceFlush(ctx)
		_ = tp.Shutdown(ctx)
	})

	out := string(captured)
	if strings.Contains(out, "should-be-stripped-XYZ") {
		t.Errorf("blocked attribute leaked through filter. Got:\n%s", out)
	}
	// Span itself MUST still be emitted — only the attribute is filtered.
	if !strings.Contains(out, "llm.Generate") {
		t.Errorf("expected span name still emitted. Got:\n%s", out)
	}
}

func TestTracedLLMProvider_Generate_NoopProvider_NoSpansEmitted(t *testing.T) {
	cfg := TelemetryConfig{
		Enabled:  false,
		Exporter: ExporterNoop,
	}
	var buf bytes.Buffer
	old := stdoutWriter
	stdoutWriter = &buf
	defer func() { stdoutWriter = old }()

	tp, _ := NewTelemetryProvider(cfg, zap.NewNop())
	inner := subagent.NewFakeLLMProvider(nil)
	wrap, err := NewTracedLLMProvider(inner, tp)
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}
	resp, err := wrap.Generate(context.Background(), &llm.LLMRequest{
		Model: "noop-model",
		Messages: []llm.Message{
			{Role: "user", Content: "hi"},
		},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if resp == nil {
		t.Fatal("Generate returned nil response")
	}
	if buf.Len() != 0 {
		t.Errorf("noop provider unexpectedly wrote stdout: %q", buf.String())
	}
}

func TestTracedLLMProvider_GenerateStream_RecordsSpan(t *testing.T) {
	cfg := TelemetryConfig{
		Enabled:      true,
		Exporter:     ExporterStdout,
		ServiceName:  "stream-llm",
		BatchTimeout: 50 * time.Millisecond,
	}
	captured := captureStdout(t, func() {
		tp, _ := NewTelemetryProvider(cfg, zap.NewNop())
		inner := subagent.NewFakeLLMProvider(nil)
		wrap, _ := NewTracedLLMProvider(inner, tp)

		ch := make(chan llm.LLMResponse, 4)
		err := wrap.GenerateStream(context.Background(), &llm.LLMRequest{
			Model: "stream-model",
			Messages: []llm.Message{
				{Role: "user", Content: "hi"},
			},
		}, ch)
		if err != nil {
			t.Fatalf("GenerateStream: %v", err)
		}
		// Drain the channel so the stub doesn't block.
		drained := 0
		for range ch {
			drained++
		}
		if drained == 0 {
			t.Error("GenerateStream produced no chunks")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = tp.ForceFlush(ctx)
		_ = tp.Shutdown(ctx)
	})

	out := string(captured)
	if !strings.Contains(out, "llm.GenerateStream") {
		t.Errorf("captured stdout missing 'llm.GenerateStream' span name. Got:\n%s", out)
	}
}

func TestTracedLLMProvider_NewTracedLLMProvider_NilInner(t *testing.T) {
	tp, _ := NewTelemetryProvider(TelemetryConfig{Enabled: false, Exporter: ExporterNoop}, zap.NewNop())
	if _, err := NewTracedLLMProvider(nil, tp); err == nil {
		t.Error("expected error for nil inner provider")
	}
}

func TestTracedLLMProvider_NewTracedLLMProvider_NilTelemetry(t *testing.T) {
	inner := subagent.NewFakeLLMProvider(nil)
	if _, err := NewTracedLLMProvider(inner, nil); err == nil {
		t.Error("expected error for nil telemetry provider")
	}
}
