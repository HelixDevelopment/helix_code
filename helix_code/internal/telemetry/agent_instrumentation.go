// AgentInstrumentation — OpenTelemetry helper for the agent loop's per-
// iteration span boundary (P1-F16-T08).
//
// The agent loop iterates: receive task → call LLM → maybe call tool → loop.
// Each iteration is a span boundary. BaseAgent.executeTaskWithLLM (the LLM-
// driven loop body) calls BeginIteration before invoking the LLM and invokes
// the returned finish closure on its return path. The closure handles latency
// measurement, iteration-counter increment, span status, and span end so the
// agent's hook only adds ~5 lines.
//
// Spec: docs/superpowers/specs/2026-05-06-p1-f16-telemetry-design.md §3
// (agent-loop in-place instrumentation).
//
// Constitutional anchor: CONST-042 (No-Secret-Leak). Span attributes are
// constrained to agent.iteration_index + agent.task_id — both metadata, never
// task input, prompt body, or tool arguments. Every attribute slice still
// passes through FilterAttributes against the operator-supplied
// BlockedAttributeKeys before being attached to the span.
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

// Instrument names. Lowercase snake_case with `helixcode_agent_` prefix so the
// OTel-to-Prometheus translation preserves the naming convention.
const (
	metricAgentIterations       = "helixcode_agent_iterations_total"
	metricAgentIterationSeconds = "helixcode_agent_iteration_seconds"
)

// Span attribute keys for agent loop iterations. Pragmatic helixcode-internal
// naming (no semconv equivalent yet).
const (
	attrAgentIterationIndex = "agent.iteration_index"
	attrAgentTaskID         = "agent.task_id"
	attrAgentOutcome        = "outcome" // success | failure
)

// agentInstrumentationScope is the tracer/meter name used for agent-loop
// telemetry. Lets observers filter spans/metrics produced by the agent layer.
const agentInstrumentationScope = "dev.helix.code/internal/telemetry/agent"

// AgentInstrumentation provides span+metric instruments for agent loop
// iterations. BaseAgent calls BeginIteration around each iteration and
// invokes the returned finish closure on its return path.
type AgentInstrumentation struct {
	tracer trace.Tracer

	// Metrics instruments — created once at construction.
	iterationCounter metric.Int64Counter
	iterationLatency metric.Float64Histogram

	// blockedAttributeKeys captures TelemetryConfig.BlockedAttributeKeys at
	// construction time so the per-call hot path doesn't re-read tp.Config().
	blockedAttributeKeys []string
}

// NewAgentInstrumentation builds the helper. Returns an error if tp is nil or
// any metric instrument fails to construct (extremely rare). Returns a no-op-
// equivalent helper when tp is the noop provider (the OTel noop tracer/meter
// emit nothing).
func NewAgentInstrumentation(tp TelemetryProvider) (*AgentInstrumentation, error) {
	if tp == nil {
		return nil, errors.New("telemetry: NewAgentInstrumentation: telemetry provider must not be nil")
	}

	meter := tp.Meter(agentInstrumentationScope)

	iterationCounter, err := meter.Int64Counter(metricAgentIterations,
		metric.WithDescription("Total agent loop iterations."),
	)
	if err != nil {
		return nil, fmt.Errorf("telemetry: build agent iteration counter: %w", err)
	}
	iterationLatency, err := meter.Float64Histogram(metricAgentIterationSeconds,
		metric.WithDescription("Agent loop iteration latency in seconds."),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, fmt.Errorf("telemetry: build agent iteration latency histogram: %w", err)
	}

	return &AgentInstrumentation{
		tracer:               tp.Tracer(agentInstrumentationScope),
		iterationCounter:     iterationCounter,
		iterationLatency:     iterationLatency,
		blockedAttributeKeys: append([]string(nil), tp.Config().BlockedAttributeKeys...),
	}, nil
}

// BeginIteration starts a span for agent loop iteration iterIndex. Returns the
// new ctx + a finish closure that the caller MUST invoke when the iteration
// completes.
//
// The closure takes (err error) and:
//   - Records latency since BeginIteration into the
//     helixcode_agent_iteration_seconds histogram, labelled with outcome.
//   - Increments helixcode_agent_iterations_total with outcome={success|failure}.
//   - Sets span status (Ok or Error with err.Error() as the description).
//   - Ends the span.
//
// Span name is "agent.iteration". Attributes attached: agent.iteration_index
// and agent.task_id (when non-empty). Both attributes pass through
// FilterAttributes so operator-blocked keys are stripped before being
// recorded.
func (a *AgentInstrumentation) BeginIteration(ctx context.Context, iterIndex int, taskID string) (context.Context, func(err error)) {
	ctx, span := a.tracer.Start(ctx, "agent.iteration")

	attrs := make([]attribute.KeyValue, 0, 2)
	attrs = append(attrs, attribute.Int(attrAgentIterationIndex, iterIndex))
	if taskID != "" {
		attrs = append(attrs, attribute.String(attrAgentTaskID, taskID))
	}
	span.SetAttributes(FilterAttributes(attrs, a.blockedAttributeKeys)...)

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

		a.iterationLatency.Record(ctx, elapsed,
			metric.WithAttributes(attribute.String(attrAgentOutcome, outcome)),
		)
		a.iterationCounter.Add(ctx, 1,
			metric.WithAttributes(attribute.String(attrAgentOutcome, outcome)),
		)

		span.End()
	}

	return ctx, finish
}
