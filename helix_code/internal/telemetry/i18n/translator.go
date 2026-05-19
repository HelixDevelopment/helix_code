// Package i18n declares internal/telemetry's hardcoded-content
// abstraction per CONST-046 (round-237 §11.4 anti-bluff sweep,
// 2026-05-19). Mirrors the "consumer defines its own Translator
// interface" pattern of rounds 93..235 (most recently
// internal/projectmemory 235 NO-OP, internal/plantree 233,
// internal/voice 226, internal/secrets 225, internal/render 224,
// internal/quality 223 NO-OP, internal/clarification 222,
// internal/approval 221).
//
// Wire-in path at boot: consuming binary builds an *i18n.Localizer
// (loaded with active.en.yaml from internal/telemetry/i18n/bundles),
// wraps it in *i18nadapter.Translator, stores via
// telemetry.SetTranslator. The package-level tr() helper falls back
// to NoopTranslator{} when not wired — loud message-ID echo, never
// silent swallow (which would be a §11.4 PASS-bluff at the i18n
// layer).
//
// Round-237 status: NO-OP INFRA — the audit gate
// (scripts/audit-const046-hardcoded-content.sh) reports zero
// hardcoded-content violations in helix_code/internal/telemetry/ at
// HEAD. The package's surface is entirely OTLP / OpenTelemetry
// instrumentation plumbing:
//
//   - OTLP wire-protocol metric names (e.g. "helixcode_llm_calls_total",
//     "helixcode_llm_latency_seconds", "helixcode_llm_prompt_tokens_total",
//     "helixcode_llm_completion_tokens_total",
//     "helixcode_tool_calls_total", "helixcode_tool_latency_seconds",
//     "helixcode_agent_iterations_total",
//     "helixcode_agent_iteration_latency_seconds") — load-bearing
//     identifiers consumed by Prometheus / Grafana / OTLP backends.
//     Translating them would silently break every downstream dashboard
//     + alerting rule that joins on the metric name. Per round-158's
//     hardware identifier-token precedent, identity keys consumed by
//     downstream parsers / equality checks are explicitly OUT of
//     CONST-046 scope.
//
//   - OTLP attribute / span-attribute keys (e.g. "llm.model",
//     "llm.provider", "llm.message_count", "llm.max_tokens",
//     "llm.usage.prompt_tokens", "llm.usage.completion_tokens",
//     "llm.finish_reason", "tool.category", "tool.outcome",
//     "agent.iteration", "agent.outcome") — OpenTelemetry semantic
//     convention keys, namespaced wire-format identifiers, NOT
//     user-display content.
//
//   - Instrumentation scope identifiers ("dev.helix.code/internal/
//     telemetry/llm", "dev.helix.code/internal/telemetry/tool",
//     "dev.helix.code/internal/telemetry/agent") — module-path
//     identity used by OTLP collectors to route + filter spans /
//     metrics by source. Translating them would break source
//     attribution at the observability backend.
//
//   - OpenTelemetry metric.WithDescription strings ("Total LLM
//     Generate / GenerateStream calls.", "LLM end-to-end latency in
//     seconds.", "Total prompt tokens consumed across LLM calls.",
//     "Total completion tokens emitted across LLM calls.", "Total
//     tool dispatch calls.", "Tool dispatch latency in seconds.",
//     "Total agent iterations.", "Agent iteration latency in
//     seconds.") — metric-metadata text surfaced ONLY to operators in
//     OTLP backend UIs (Prometheus /metrics endpoint, Grafana metric
//     browser, OpenTelemetry Collector exporter logs). NOT
//     end-user-product narrative. The OpenTelemetry spec treats
//     WithDescription as a machine-consumable descriptor, not
//     i18n-localised UI text; per round-235's logger-WARN precedent,
//     ops/debug surfaces are explicitly OUT of CONST-046 scope.
//
//   - Outcome tag literals ("success", "failure", "stream_success",
//     "stream_failure") — closed-set attribute values used for
//     cardinality-bounded metric grouping. Translating them would
//     destroy join semantics across exporters (every dashboard that
//     filters by outcome="success" would break for non-English
//     locales). Identity-tag precedent applies.
//
//   - Sentinel-error static fragments ("telemetry disabled",
//     "unsupported exporter kind", "telemetry provider not
//     initialised") + constructor-error wrapped messages ("telemetry:
//     NewTracedLLMProvider: inner provider must not be nil",
//     "telemetry: NewTracedLLMProvider: telemetry provider must not
//     be nil", "telemetry: NewToolInstrumentation: telemetry provider
//     must not be nil", "telemetry: NewAgentInstrumentation:
//     telemetry provider must not be nil", "telemetry: build call
//     counter: %w", "telemetry: build latency histogram: %w",
//     "telemetry: build prompt tokens counter: %w", "telemetry: build
//     completion tokens counter: %w", "telemetry: build tool call
//     counter: %w", "telemetry: build tool latency histogram: %w",
//     "telemetry: build agent iteration counter: %w", "telemetry:
//     build agent iteration latency histogram: %w", "build resource:
//     %w", "build trace exporter: %w", "build metric exporter: %w",
//     "tracer flush: %w", "meter flush: %w", "tracer shutdown: %w",
//     "meter shutdown: %w") — surfaced to logs / errors.Is
//     comparisons / error chains for developers + ops, not end-user
//     UI narrative. Constructor-boot errors only surface during
//     misconfiguration before any user interaction has begun.
//
//   - Environment-variable name tokens ("HELIXCODE_OTEL_EXPORTER",
//     "OTEL_TRACES_EXPORTER", "OTEL_EXPORTER_OTLP_PROTOCOL") —
//     filesystem-style environment-variable identifiers consumed by
//     os.Getenv. Translating them would silently disconnect the
//     config-resolution pipeline from the documented operator
//     contract.
//
// Per round-158's hardware identifier-token precedent + round-235's
// log-message ops-surface precedent, identity keys + ops-surface
// descriptors consumed by downstream parsers / observability backends
// are explicitly OUT of CONST-046 scope. Translating any metric name,
// attribute key, or WithDescription string would silently rewrite the
// contract with OTLP collectors / Prometheus / Grafana.
//
// The scaffold lands now so any FUTURE user-facing string added to
// internal/telemetry/ (e.g. a /telemetry status TUI banner, a CLI
// summary line surfacing exporter state to end users, a TUI startup
// hint about telemetry being disabled) inherits the standard
// migration pattern without further infra work.
package i18n

import "context"

// Translator is the contract internal/telemetry uses for every
// CONST-046-migrated user-facing string.
type Translator interface {
	// T resolves messageID against the active locale. templateData
	// supplies named placeholders for go-i18n style interpolation;
	// pass nil when the message has no placeholders.
	T(ctx context.Context, messageID string, templateData map[string]any) (string, error)

	// TPlural resolves messageID with plural-form selection driven
	// by count. templateData carries any non-count placeholders.
	TPlural(ctx context.Context, messageID string, count int, templateData map[string]any) (string, error)
}

// NoopTranslator returns the messageID verbatim. SAFETY default for
// unit tests within this package + backward-compat for callers who
// have not yet wired a real Translator. Production paths MUST inject
// a real Translator (helix_code wires *i18nadapter.Translator at
// boot).
type NoopTranslator struct{}

// T returns id unchanged (loud echo). Never returns an error.
func (NoopTranslator) T(_ context.Context, id string, _ map[string]any) (string, error) {
	return id, nil
}

// TPlural returns id unchanged (loud echo). Never returns an error.
func (NoopTranslator) TPlural(_ context.Context, id string, _ int, _ map[string]any) (string, error) {
	return id, nil
}
