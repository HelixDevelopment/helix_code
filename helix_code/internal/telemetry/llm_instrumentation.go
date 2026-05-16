// TracedLLMProvider — OpenTelemetry decorator for llm.Provider (P1-F16-T06).
//
// The decorator wraps an llm.Provider via Go struct embedding. 9 of the 11
// Provider methods are promoted directly from the embedded inner provider
// (zero overhead). Generate and GenerateStream are overridden to add a span
// + four metrics (call counter, latency histogram, prompt + completion token
// counters).
//
// Constitutional anchor: CONST-042 (No-Secret-Leak). The decorator NEVER
// emits prompt body, message contents, completion text, or API keys as span
// attributes. Only metadata flows through:
//   - llm.model            (string)
//   - llm.provider         (string, ProviderType.String())
//   - llm.message_count    (int — count, not contents)
//   - llm.max_tokens       (int)
//   - llm.usage.prompt_tokens     (int)
//   - llm.usage.completion_tokens (int)
//   - llm.finish_reason    (string)
//
// Defence-in-depth: every attribute slice passes through FilterAttributes
// against the operator-supplied BlockedAttributeKeys (additive over the
// CONST-042 default-deny list) before being attached to the span. Even if a
// future contributor adds an attribute that names a denied key, the floor
// strips it.
//
// Pragmatic naming: OTel semconv has gen_ai.* keys but they are still
// experimental and not in our pinned semconv v1.26.0. We use helixcode-
// internal keys (`llm.*`); migrating to gen_ai.* is a backwards-compatible
// append once semconv stabilises.
//
// Spec: docs/superpowers/specs/2026-05-06-p1-f16-telemetry-design.md §3 + §5.
// Plan: docs/superpowers/plans/2026-05-06-p1-f16-telemetry.md T06.
package telemetry

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"dev.helix.code/internal/llm"
)

// Instrument names. Lowercase snake_case with `helixcode_llm_` prefix so the
// OTel-to-Prometheus translation preserves the naming convention.
const (
	metricLLMCalls            = "helixcode_llm_calls_total"
	metricLLMLatencySeconds   = "helixcode_llm_latency_seconds"
	metricLLMPromptTokens     = "helixcode_llm_prompt_tokens_total"
	metricLLMCompletionTokens = "helixcode_llm_completion_tokens_total"
)

// Span attribute keys. Pragmatic helixcode-internal naming until semconv
// stabilises gen_ai.*.
const (
	attrLLMModel            = "llm.model"
	attrLLMProvider         = "llm.provider"
	attrLLMMessageCount     = "llm.message_count"
	attrLLMMaxTokens        = "llm.max_tokens"
	attrLLMPromptTokens     = "llm.usage.prompt_tokens"
	attrLLMCompletionTokens = "llm.usage.completion_tokens"
	attrLLMFinishReason     = "llm.finish_reason"
	attrLLMOutcome          = "outcome" // success | failure
)

// instrumentationScope is the tracer/meter name used by the decorator. Allows
// observers to filter telemetry produced by the LLM layer specifically.
const instrumentationScope = "dev.helix.code/internal/telemetry/llm"

// TracedLLMProvider wraps an llm.Provider with OTel instrumentation. Generate
// and GenerateStream calls produce spans with safe attributes (model, message
// count, finish reason, token counts) and metrics (call counter, latency
// histogram, prompt/completion token counters).
//
// CONST-042: prompt and completion BODIES are NEVER emitted as span
// attributes. Only metadata flows through, and every attribute is run through
// FilterAttributes against the effective deny-list before being attached.
//
// The 9 unmodified Provider methods (GetType, GetName, GetModels,
// GetCapabilities, IsAvailable, GetHealth, Close, GetContextWindow,
// CountTokens) are promoted via embedding for zero overhead.
type TracedLLMProvider struct {
	llm.Provider // embedded — promotes 9 methods.

	tracer trace.Tracer

	// Metrics instruments — created once at construction.
	callCounter             metric.Int64Counter
	latencyHistogram        metric.Float64Histogram
	promptTokensCounter     metric.Int64Counter
	completionTokensCounter metric.Int64Counter

	// blockedAttributeKeys is captured once at construction so the per-call
	// hot path does not need to re-read tp.Config(). Mirrors the operator-
	// supplied additions to DefaultBlockedAttributeKeys.
	blockedAttributeKeys []string
}

// NewTracedLLMProvider wraps inner with telemetry. Returns an error if either
// inner or tp is nil, or if any metric instrument fails to construct
// (extremely rare; OTel Int64Counter can fail on invalid name only).
//
// When tp is the noop provider (telemetry off), the returned wrapper is
// effectively a thin pass-through: the OTel noop tracer/meter return no-op
// spans/instruments, so neither stdout nor an OTLP collector receive
// anything.
func NewTracedLLMProvider(inner llm.Provider, tp TelemetryProvider) (*TracedLLMProvider, error) {
	if inner == nil {
		return nil, errors.New("telemetry: NewTracedLLMProvider: inner provider must not be nil")
	}
	if tp == nil {
		return nil, errors.New("telemetry: NewTracedLLMProvider: telemetry provider must not be nil")
	}

	meter := tp.Meter(instrumentationScope)

	callCounter, err := meter.Int64Counter(metricLLMCalls,
		metric.WithDescription("Total LLM Generate / GenerateStream calls."),
	)
	if err != nil {
		return nil, fmt.Errorf("telemetry: build call counter: %w", err)
	}
	latencyHistogram, err := meter.Float64Histogram(metricLLMLatencySeconds,
		metric.WithDescription("LLM Generate / GenerateStream latency in seconds."),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, fmt.Errorf("telemetry: build latency histogram: %w", err)
	}
	promptTokensCounter, err := meter.Int64Counter(metricLLMPromptTokens,
		metric.WithDescription("Total prompt tokens consumed by LLM calls."),
	)
	if err != nil {
		return nil, fmt.Errorf("telemetry: build prompt tokens counter: %w", err)
	}
	completionTokensCounter, err := meter.Int64Counter(metricLLMCompletionTokens,
		metric.WithDescription("Total completion tokens emitted by LLM calls."),
	)
	if err != nil {
		return nil, fmt.Errorf("telemetry: build completion tokens counter: %w", err)
	}

	return &TracedLLMProvider{
		Provider:                inner,
		tracer:                  tp.Tracer(instrumentationScope),
		callCounter:             callCounter,
		latencyHistogram:        latencyHistogram,
		promptTokensCounter:     promptTokensCounter,
		completionTokensCounter: completionTokensCounter,
		blockedAttributeKeys:    append([]string(nil), tp.Config().BlockedAttributeKeys...),
	}, nil
}

// Generate wraps the inner Generate call with a span + metrics. Returns the
// inner result + error verbatim — telemetry NEVER alters the caller-observed
// behaviour.
func (t *TracedLLMProvider) Generate(ctx context.Context, req *llm.LLMRequest) (*llm.LLMResponse, error) {
	ctx, span := t.tracer.Start(ctx, "llm.Generate")
	defer span.End()

	t.setRequestAttributes(span, req)

	start := time.Now()
	resp, err := t.Provider.Generate(ctx, req)
	elapsed := time.Since(start).Seconds()

	outcome := "success"
	if err != nil {
		outcome = "failure"
		span.SetStatus(codes.Error, err.Error())
	} else {
		t.setResponseAttributes(span, resp)
		t.recordTokenCounts(ctx, resp)
		span.SetStatus(codes.Ok, "")
	}

	t.latencyHistogram.Record(ctx, elapsed,
		metric.WithAttributes(t.outcomeAttr(outcome)),
	)
	t.callCounter.Add(ctx, 1,
		metric.WithAttributes(t.outcomeAttr(outcome)),
	)

	return resp, err
}

// GenerateStream wraps the inner streaming call. Records the same start-time-
// to-completion span. v1 records latency + call count; per-token streaming
// metrics may be added once stream chunks consistently carry usage data
// across providers.
func (t *TracedLLMProvider) GenerateStream(ctx context.Context, req *llm.LLMRequest, ch chan<- llm.LLMResponse) error {
	ctx, span := t.tracer.Start(ctx, "llm.GenerateStream")
	defer span.End()

	t.setRequestAttributes(span, req)

	start := time.Now()
	err := t.Provider.GenerateStream(ctx, req, ch)
	elapsed := time.Since(start).Seconds()

	outcome := "success"
	if err != nil {
		outcome = "failure"
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}

	t.latencyHistogram.Record(ctx, elapsed,
		metric.WithAttributes(t.outcomeAttr(outcome)),
	)
	t.callCounter.Add(ctx, 1,
		metric.WithAttributes(t.outcomeAttr(outcome)),
	)

	return err
}

// setRequestAttributes assembles the safe request-side attribute set and
// applies it to the span after running through FilterAttributes against the
// effective deny-list. NEVER includes prompt body or message contents — only
// metadata (model, count, max_tokens, provider type).
func (t *TracedLLMProvider) setRequestAttributes(span trace.Span, req *llm.LLMRequest) {
	attrs := make([]attribute.KeyValue, 0, 4)
	attrs = append(attrs, attribute.String(attrLLMProvider, string(t.Provider.GetType())))
	if req != nil {
		attrs = append(attrs,
			attribute.String(attrLLMModel, req.Model),
			attribute.Int(attrLLMMessageCount, len(req.Messages)),
		)
		if req.MaxTokens > 0 {
			attrs = append(attrs, attribute.Int(attrLLMMaxTokens, req.MaxTokens))
		}
	}
	span.SetAttributes(FilterAttributes(attrs, t.blockedAttributeKeys)...)
}

// setResponseAttributes attaches token counts + finish reason to the span,
// after running through FilterAttributes. NEVER includes completion text.
func (t *TracedLLMProvider) setResponseAttributes(span trace.Span, resp *llm.LLMResponse) {
	if resp == nil {
		return
	}
	attrs := []attribute.KeyValue{
		attribute.Int(attrLLMPromptTokens, resp.Usage.PromptTokens),
		attribute.Int(attrLLMCompletionTokens, resp.Usage.CompletionTokens),
	}
	if resp.FinishReason != "" {
		attrs = append(attrs, attribute.String(attrLLMFinishReason, resp.FinishReason))
	}
	span.SetAttributes(FilterAttributes(attrs, t.blockedAttributeKeys)...)
}

// recordTokenCounts increments the prompt + completion token counters when
// the response carries non-zero usage data. Skips silently when usage is
// missing — better to under-report than to fabricate zeros that pollute the
// metric stream.
func (t *TracedLLMProvider) recordTokenCounts(ctx context.Context, resp *llm.LLMResponse) {
	if resp == nil {
		return
	}
	modelAttr := metric.WithAttributes(
		attribute.String(attrLLMProvider, string(t.Provider.GetType())),
	)
	if resp.Usage.PromptTokens > 0 {
		t.promptTokensCounter.Add(ctx, int64(resp.Usage.PromptTokens), modelAttr)
	}
	if resp.Usage.CompletionTokens > 0 {
		t.completionTokensCounter.Add(ctx, int64(resp.Usage.CompletionTokens), modelAttr)
	}
}

// outcomeAttr returns the attribute KV used to label call/latency metrics so
// success vs failure rates can be split in the backend.
func (t *TracedLLMProvider) outcomeAttr(outcome string) attribute.KeyValue {
	return attribute.String(attrLLMOutcome, outcome)
}
