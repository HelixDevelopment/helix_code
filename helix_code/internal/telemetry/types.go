// Package telemetry defines the foundational types, default attribute
// deny-list, and provider contract for HelixCode's OpenTelemetry-based
// observability feature (P1-F16).
//
// This file is type-only: every consumer (env-var config parser, exporter
// constructors, tracer/meter providers, LLM/tool/agent decorators, and the
// `/telemetry` slash command) imports the types declared here. Behaviour
// (env parsing, exporter construction, decorators) lives in sibling files
// added by later T04–T09 tasks.
//
// Constitutional anchor: CONST-042 (No-Secret-Leak). DefaultBlockedAttributeKeys
// is the canonical floor of attribute keys whose values MUST NOT flow into
// spans, metrics, or logs. The deny-list is additive: TelemetryConfig.
// BlockedAttributeKeys lets operators ADD entries; there is no API to remove
// a default. Callers route attributes through FilterAttributes (and the
// AttributeBlocked predicate) before recording them on any span/meter so the
// secret floor cannot be bypassed by accident.
//
// Spec: docs/superpowers/specs/2026-05-06-p1-f16-telemetry-design.md
// Plan: docs/superpowers/plans/2026-05-06-p1-f16-telemetry.md
package telemetry

import (
	"context"
	"errors"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// ExporterKind identifies which exporter implementation is selected at
// runtime. The string values for ExporterOTLPGRPC and ExporterOTLPHTTP match
// the documented values of OTEL_EXPORTER_OTLP_PROTOCOL ("grpc" and
// "http/protobuf"); ExporterStdout and ExporterNoop are HelixCode-internal
// sentinels (the OTel spec has no env-var value for them).
type ExporterKind string

const (
	// ExporterStdout writes spans/metrics to stdout via the OTel
	// stdout{trace,metric} exporters. Always available; the safe default
	// when an operator wants to "see something" without standing up a
	// collector.
	ExporterStdout ExporterKind = "stdout"
	// ExporterOTLPGRPC selects the OTLP exporter over gRPC. Matches
	// OTEL_EXPORTER_OTLP_PROTOCOL=grpc.
	ExporterOTLPGRPC ExporterKind = "grpc"
	// ExporterOTLPHTTP selects the OTLP exporter over HTTP/protobuf.
	// Matches OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf.
	ExporterOTLPHTTP ExporterKind = "http/protobuf"
	// ExporterNoop is the sentinel used when telemetry is disabled. The
	// provider returns no-op tracers/meters and no exporter is wired.
	ExporterNoop ExporterKind = "noop"
)

// IsValid reports whether k is one of the four documented exporter kinds.
// Consumers (T04 env parser, T05 provider constructor) reject invalid kinds
// with ErrUnsupportedExporter rather than silently falling back.
func (k ExporterKind) IsValid() bool {
	switch k {
	case ExporterStdout, ExporterOTLPGRPC, ExporterOTLPHTTP, ExporterNoop:
		return true
	}
	return false
}

// DefaultServiceName is the fallback OTEL_SERVICE_NAME value when no
// environment override is supplied. Distinct from any deployed service name
// so that "helixcode" in a backend always means "the CLI itself".
const DefaultServiceName = "helixcode"

// Default timeouts wired through TelemetryConfig when the operator does not
// override via env vars. These values match the spec §3 defaults and the
// upstream OTel SDK's documented recommended ranges.
const (
	// DefaultBatchTimeout is the maximum time a span batch may sit in the
	// BatchSpanProcessor before being flushed.
	DefaultBatchTimeout = 5 * time.Second
	// DefaultExportTimeout is the per-export RPC deadline for OTLP/stdout
	// exporters.
	DefaultExportTimeout = 30 * time.Second
	// DefaultShutdownTimeout is the wall-clock budget the shutdown path
	// gives ForceFlush + provider Shutdown to drain.
	DefaultShutdownTimeout = 5 * time.Second
)

// TelemetryConfig describes the runtime telemetry setup. Populated by parsing
// OTEL_* env vars in T04. The zero value is intentionally "no-op telemetry"
// (Enabled=false, Exporter=""): code paths that forget to populate it MUST
// NOT accidentally start exporting anything.
//
// BlockedAttributeKeys is additive on top of DefaultBlockedAttributeKeys. The
// effective deny-list at runtime is the union of the two; no env var or
// configuration value can subtract from the default floor (CONST-042).
type TelemetryConfig struct {
	// Enabled is the master switch. When false, the provider returns
	// no-op tracers/meters regardless of the other fields.
	Enabled bool
	// Exporter selects the exporter implementation. Validated via
	// ExporterKind.IsValid before the provider is constructed.
	Exporter ExporterKind
	// Endpoint is the OTLP collector endpoint
	// (OTEL_EXPORTER_OTLP_ENDPOINT). Ignored for stdout/noop.
	Endpoint string
	// ServiceName is the OTel resource service.name attribute. Defaults
	// to DefaultServiceName when unset.
	ServiceName string
	// ResourceAttrs are extra OTel resource attributes parsed from
	// OTEL_RESOURCE_ATTRIBUTES (e.g. "deployment.environment=prod").
	ResourceAttrs map[string]string
	// BlockedAttributeKeys are operator-supplied additions to the
	// default deny-list. Compared case-insensitively, exact match.
	BlockedAttributeKeys []string
	// BatchTimeout is the span-batch flush interval. Falls back to
	// DefaultBatchTimeout when zero.
	BatchTimeout time.Duration
	// ExportTimeout is the per-export RPC deadline. Falls back to
	// DefaultExportTimeout when zero.
	ExportTimeout time.Duration
	// ShutdownTimeout is the budget given to ForceFlush + Shutdown
	// during teardown. Falls back to DefaultShutdownTimeout when zero.
	ShutdownTimeout time.Duration
	// Insecure disables TLS for OTLP exporters when true (mirrors
	// OTEL_EXPORTER_OTLP_INSECURE=true). Has no effect on stdout/noop.
	Insecure bool
}

// DefaultBlockedAttributeKeys is the seed list of attribute keys that MUST
// NOT flow into spans, metrics, or logs under any circumstance. CONST-042
// enforces a default-deny posture for credentials and prompt bodies.
//
// Comparison is case-insensitive exact match (handled by AttributeBlocked).
// Operators can ADD additional keys via TelemetryConfig.BlockedAttributeKeys
// but no env-var, config, or runtime call can remove an entry from this slice.
//
// CONST-042 anchor: this list is the floor; the union of the floor + any
// operator additions is what FilterAttributes enforces at the call site.
// Tests in types_test.go ("TestAttributeBlocked_UserCannotSubtract") prove
// the no-subtract invariant.
var DefaultBlockedAttributeKeys = []string{
	// Generic credential keys
	"api_key", "apikey", "api-key",
	"token", "auth", "authorization", "bearer",
	"password", "passwd", "secret",
	// Provider-specific credential keys
	"anthropic_api_key", "openai_api_key", "google_api_key",
	"aws_access_key_id", "aws_secret_access_key",
	"azure_openai_api_key",
	// Prompt-body / completion-body keys (privacy + token-volume risk)
	"prompt", "prompt_body",
	"request_body", "response_body",
	"messages", "completion",
}

// TelemetryProvider is the contract for the central telemetry orchestrator.
// Implemented by *RealTelemetryProvider in T05; tests and the explicit
// "telemetry off" path use a no-op implementation. Methods return the OTel
// SDK's own typed values (trace.Tracer / metric.Meter) so callers can use
// the OTel Go API directly without a thin shim.
type TelemetryProvider interface {
	// Tracer returns an OTel Tracer named for the given component.
	// When the provider is in noop mode (telemetry off), the returned
	// Tracer is the OTel no-op tracer — every span is a stub.
	Tracer(name string) trace.Tracer
	// Meter returns an OTel Meter for the given component.
	// In noop mode the returned Meter is the OTel no-op meter.
	Meter(name string) metric.Meter
	// Config returns a snapshot of the active configuration. The returned
	// value is a copy: mutating it does NOT change provider state.
	Config() TelemetryConfig
	// Exporter returns the exporter kind currently wired. ExporterNoop
	// indicates telemetry is disabled.
	Exporter() ExporterKind
	// ForceFlush flushes any buffered spans/metrics. Honours ctx
	// deadline; returns the underlying provider error on failure.
	ForceFlush(ctx context.Context) error
	// Shutdown flushes + tears down the provider. Idempotent: a second
	// call after a successful Shutdown returns nil.
	Shutdown(ctx context.Context) error
}

// Sentinel errors for the telemetry package. Callers compare via errors.Is.
var (
	// ErrTelemetryDisabled is returned by operations that require an
	// active provider when the runtime configuration has telemetry off.
	ErrTelemetryDisabled = errors.New("telemetry disabled")
	// ErrUnsupportedExporter is returned by the env parser and provider
	// constructor when ExporterKind.IsValid is false.
	ErrUnsupportedExporter = errors.New("unsupported exporter kind")
	// ErrTelemetryNotInitialised is returned by package-level helpers
	// invoked before main.go has wired a provider.
	ErrTelemetryNotInitialised = errors.New("telemetry provider not initialised")
)

// AttributeBlocked reports whether key is in the effective deny-list.
// The effective deny-list is the union of DefaultBlockedAttributeKeys and
// extra; comparison is case-insensitive exact match (no substring or
// prefix matching, to avoid blocking benign keys like "tool_name" just
// because they share characters with a denied key).
//
// Pure function: safe for concurrent use, no allocations on the hot path
// beyond the single ToLower of the input.
func AttributeBlocked(key string, extra []string) bool {
	if key == "" {
		return false
	}
	lowered := strings.ToLower(key)
	for _, d := range DefaultBlockedAttributeKeys {
		if strings.ToLower(d) == lowered {
			return true
		}
	}
	for _, d := range extra {
		if strings.ToLower(d) == lowered {
			return true
		}
	}
	return false
}

// FilterAttributes returns a new slice containing only the KVs whose keys
// are NOT blocked by the effective deny-list. The relative order of
// surviving entries is preserved. A nil or empty input yields a non-nil
// empty slice — callers can always range over the result without a nil
// check.
//
// This is the function every instrumented decorator (T06/T07/T08) must use
// before recording attributes on a span or meter. Bypassing this filter is
// a CONST-042 violation flagged by the bluff scanner.
func FilterAttributes(attrs []attribute.KeyValue, extra []string) []attribute.KeyValue {
	out := make([]attribute.KeyValue, 0, len(attrs))
	for _, kv := range attrs {
		if AttributeBlocked(string(kv.Key), extra) {
			continue
		}
		out = append(out, kv)
	}
	return out
}
