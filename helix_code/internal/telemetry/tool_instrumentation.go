// ToolInstrumentation — OpenTelemetry helper for ToolRegistry.Execute (P1-F16-T07).
//
// ToolRegistry.Execute calls Begin(ctx, name, category) before invoking the
// underlying tool and invokes the returned finish closure on its return path.
// The closure handles latency measurement, call-counter increment, span status,
// and span end so the registry's hook only adds ~5 lines.
//
// Spec: docs/superpowers/specs/2026-05-06-p1-f16-telemetry-design.md §3
// (in-place tool instrumentation).
//
// Constitutional anchor: CONST-042 (No-Secret-Leak). Span attributes are
// constrained to tool.name + tool.category — both metadata, never tool
// arguments or output. Every attribute slice still passes through
// FilterAttributes against the operator-supplied BlockedAttributeKeys before
// being attached to the span. Tool arguments / results / errors-other-than-
// .Error() ARE NEVER recorded as attributes.
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
)

// Instrument names. Lowercase snake_case with `helixcode_tool_` prefix so the
// OTel-to-Prometheus translation preserves the naming convention.
const (
	metricToolCalls          = "helixcode_tool_calls_total"
	metricToolLatencySeconds = "helixcode_tool_latency_seconds"
)

// Span attribute keys for tool dispatch. Pragmatic helixcode-internal naming
// (no semconv equivalent yet; gen_ai.* is LLM-only).
const (
	attrToolName     = "tool.name"
	attrToolCategory = "tool.category"
	attrToolOutcome  = "outcome" // success | failure
)

// toolInstrumentationScope is the tracer/meter name used for tool dispatch
// telemetry. Lets observers filter spans/metrics produced by the tool layer.
const toolInstrumentationScope = "dev.helix.code/internal/telemetry/tool"

// ToolInstrumentation provides span+metric instruments for tool dispatch.
// ToolRegistry.Execute calls Begin around each tool execution and invokes
// the returned finish closure on its return path.
type ToolInstrumentation struct {
	tracer trace.Tracer

	// Metrics instruments — created once at construction.
	callCounter      metric.Int64Counter
	latencyHistogram metric.Float64Histogram

	// blockedAttributeKeys captures TelemetryConfig.BlockedAttributeKeys at
	// construction time so the per-call hot path doesn't re-read tp.Config().
	blockedAttributeKeys []string
}

// NewToolInstrumentation builds the helper. Returns an error if tp is nil or
// any metric instrument fails to construct (extremely rare). Returns a no-op-
// equivalent helper when tp is the noop provider (the OTel noop tracer/meter
// emit nothing).
func NewToolInstrumentation(tp TelemetryProvider) (*ToolInstrumentation, error) {
	if tp == nil {
		return nil, errors.New("telemetry: NewToolInstrumentation: telemetry provider must not be nil")
	}

	meter := tp.Meter(toolInstrumentationScope)

	callCounter, err := meter.Int64Counter(metricToolCalls,
		metric.WithDescription("Total tool dispatch calls."),
	)
	if err != nil {
		return nil, fmt.Errorf("telemetry: build tool call counter: %w", err)
	}
	latencyHistogram, err := meter.Float64Histogram(metricToolLatencySeconds,
		metric.WithDescription("Tool dispatch latency in seconds."),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, fmt.Errorf("telemetry: build tool latency histogram: %w", err)
	}

	return &ToolInstrumentation{
		tracer:               tp.Tracer(toolInstrumentationScope),
		callCounter:          callCounter,
		latencyHistogram:     latencyHistogram,
		blockedAttributeKeys: append([]string(nil), tp.Config().BlockedAttributeKeys...),
	}, nil
}

// Begin starts a span for the given tool. Returns the new ctx + a finish
// closure that the caller MUST invoke when execution completes.
//
// The closure takes (err error) and:
//   - Records latency since Begin into the helixcode_tool_latency_seconds
//     histogram, labelled with outcome.
//   - Increments helixcode_tool_calls_total with outcome={success|failure}.
//   - Sets span status (Ok or Error with err.Error() as the description).
//   - Ends the span.
//
// Span name is "tool." + toolName. Attributes attached: tool.name and
// tool.category (when non-empty). Both attributes pass through
// FilterAttributes so operator-blocked keys are stripped before being
// recorded.
func (i *ToolInstrumentation) Begin(ctx context.Context, toolName, toolCategory string) (context.Context, func(err error)) {
	ctx, span := i.tracer.Start(ctx, "tool."+toolName)

	attrs := make([]attribute.KeyValue, 0, 2)
	attrs = append(attrs, attribute.String(attrToolName, toolName))
	if toolCategory != "" {
		attrs = append(attrs, attribute.String(attrToolCategory, toolCategory))
	}
	span.SetAttributes(FilterAttributes(attrs, i.blockedAttributeKeys)...)

	start := time.Now()

	finish := func(err error) {
		elapsed := time.Since(start).Seconds()

		outcome := "success"
		if err != nil {
			outcome = "failure"
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetStatus(codes.Ok, "")
		}

		i.latencyHistogram.Record(ctx, elapsed,
			metric.WithAttributes(attribute.String(attrToolOutcome, outcome)),
		)
		i.callCounter.Add(ctx, 1,
			metric.WithAttributes(attribute.String(attrToolOutcome, outcome)),
		)

		span.End()
	}

	return ctx, finish
}
