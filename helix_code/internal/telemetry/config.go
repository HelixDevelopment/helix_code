// Config parsing for the telemetry package: takes the OTel-spec environment
// variables (and HelixCode-specific overrides) and produces a TelemetryConfig
// the provider in T05 can consume directly.
//
// Pure / side-effect-free: every entry point takes envLookup as a parameter so
// tests don't have to mutate process state. main.go (T09) wires os.Getenv as
// the lookup at startup.
//
// Spec: docs/superpowers/specs/2026-05-06-p1-f16-telemetry-design.md §3, §4.
// Plan: docs/superpowers/plans/2026-05-06-p1-f16-telemetry.md T04.
package telemetry

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Environment variable names recognised by the parser. Centralised here so
// every consumer (parser, tests, /telemetry slash command in T08) references
// the same canonical strings.
const (
	envOTELProtocol      = "OTEL_EXPORTER_OTLP_PROTOCOL"
	envOTELEndpoint      = "OTEL_EXPORTER_OTLP_ENDPOINT"
	envOTELInsecure      = "OTEL_EXPORTER_OTLP_INSECURE"
	envOTELServiceName   = "OTEL_SERVICE_NAME"
	envOTELResourceAttrs = "OTEL_RESOURCE_ATTRIBUTES"
	envOTELTracesExp     = "OTEL_TRACES_EXPORTER"
	envOTELBSPSchedule   = "OTEL_BSP_SCHEDULE_DELAY"
	envOTELBSPExportTO   = "OTEL_BSP_EXPORT_TIMEOUT"
	envHelixCodeExporter = "HELIXCODE_OTEL_EXPORTER"
)

// LoadConfigFromEnv parses OTEL_* env vars (and the HelixCode-specific
// override) into a TelemetryConfig.
//
// Returns:
//   - cfg.Enabled = false, cfg.Exporter = ExporterNoop when no telemetry env
//     vars are set or OTEL_TRACES_EXPORTER=none is supplied (explicit-disable).
//   - cfg.Enabled = true with the resolved exporter kind otherwise.
//   - ErrUnsupportedExporter wrapping the offending value when
//     OTEL_EXPORTER_OTLP_PROTOCOL is set to an unrecognised value such as
//     "http/json" or "thrift" — fail-fast rather than silently picking
//     a default (a "fail-open" bluff).
//
// Other parse failures (negative or non-numeric durations, malformed resource
// attribute entries) are tolerated: the field receives its default and the
// rest of the config is honoured. This mirrors the OTel SDK behaviour.
func LoadConfigFromEnv(envLookup func(string) string) (TelemetryConfig, error) {
	if envLookup == nil {
		envLookup = func(string) string { return "" }
	}

	exporter, err := resolveExporterKind(envLookup)
	if err != nil {
		return TelemetryConfig{}, err
	}

	serviceName := envLookup(envOTELServiceName)
	if serviceName == "" {
		serviceName = DefaultServiceName
	}

	cfg := TelemetryConfig{
		Enabled:              exporter != ExporterNoop,
		Exporter:             exporter,
		Endpoint:             envLookup(envOTELEndpoint),
		ServiceName:          serviceName,
		ResourceAttrs:        parseResourceAttrs(envLookup(envOTELResourceAttrs)),
		BlockedAttributeKeys: append([]string(nil), DefaultBlockedAttributeKeys...),
		BatchTimeout:         parseDurationMillis(envLookup(envOTELBSPSchedule), DefaultBatchTimeout),
		ExportTimeout:        parseDurationMillis(envLookup(envOTELBSPExportTO), DefaultExportTimeout),
		ShutdownTimeout:      DefaultShutdownTimeout,
		Insecure:             parseBool(envLookup(envOTELInsecure), false),
	}

	return cfg, nil
}

// resolveExporterKind applies the documented precedence chain:
//
//  1. HELIXCODE_OTEL_EXPORTER (=stdout) — HelixCode-specific dev shortcut.
//  2. OTEL_TRACES_EXPORTER (=console -> stdout, =none -> noop, =otlp -> step 3).
//  3. OTEL_EXPORTER_OTLP_PROTOCOL (grpc | http/protobuf).
//  4. nothing set -> ExporterNoop (silent default; telemetry off unless opted in).
//
// Step 3 returns ErrUnsupportedExporter when the protocol is set to an
// unrecognised value. This is the only fatal case — every other source of
// ambiguity falls through to the next tier.
func resolveExporterKind(envLookup func(string) string) (ExporterKind, error) {
	if v := strings.ToLower(strings.TrimSpace(envLookup(envHelixCodeExporter))); v != "" {
		switch v {
		case "stdout", "console":
			return ExporterStdout, nil
		case "none", "noop", "off":
			return ExporterNoop, nil
		case "grpc":
			return ExporterOTLPGRPC, nil
		case "http/protobuf", "http":
			return ExporterOTLPHTTP, nil
		default:
			return "", fmt.Errorf("%w: HELIXCODE_OTEL_EXPORTER=%q", ErrUnsupportedExporter, v)
		}
	}

	if v := strings.ToLower(strings.TrimSpace(envLookup(envOTELTracesExp))); v != "" {
		switch v {
		case "console":
			return ExporterStdout, nil
		case "none":
			return ExporterNoop, nil
		case "otlp":
			// Falls through to OTEL_EXPORTER_OTLP_PROTOCOL handling below.
		default:
			return "", fmt.Errorf("%w: OTEL_TRACES_EXPORTER=%q", ErrUnsupportedExporter, v)
		}
	}

	if v := strings.ToLower(strings.TrimSpace(envLookup(envOTELProtocol))); v != "" {
		switch v {
		case "grpc":
			return ExporterOTLPGRPC, nil
		case "http/protobuf":
			return ExporterOTLPHTTP, nil
		default:
			return "", fmt.Errorf("%w: OTEL_EXPORTER_OTLP_PROTOCOL=%q", ErrUnsupportedExporter, v)
		}
	}

	// OTEL_TRACES_EXPORTER=otlp without a protocol is ambiguous; treat as
	// noop rather than guessing. Operators must set OTEL_EXPORTER_OTLP_PROTOCOL.
	return ExporterNoop, nil
}

// parseResourceAttrs splits "k1=v1,k2=v2,k3=v3" into a map[string]string.
//
// Tolerates whitespace around keys/values, empty values ("k1=,k2=v2" yields
// k1=""), and skips malformed entries (entries without an "=", or with empty
// keys after trimming). An empty input yields an empty (non-nil) map so
// callers can range over it safely.
func parseResourceAttrs(s string) map[string]string {
	out := make(map[string]string)
	s = strings.TrimSpace(s)
	if s == "" {
		return out
	}
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		idx := strings.Index(part, "=")
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(part[:idx])
		val := strings.TrimSpace(part[idx+1:])
		if key == "" {
			continue
		}
		out[key] = val
	}
	return out
}

// parseDurationMillis parses s as an integer number of milliseconds. Returns
// def for: empty input, non-numeric input, or negative values. The OTel spec
// expresses BSP timeouts in milliseconds, so we keep the unit explicit here
// rather than trying to interpret Go duration strings.
func parseDurationMillis(s string, def time.Duration) time.Duration {
	s = strings.TrimSpace(s)
	if s == "" {
		return def
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n < 0 {
		return def
	}
	return time.Duration(n) * time.Millisecond
}

// parseBool parses "true"/"false"/"1"/"0" (case-insensitive). Returns def for
// empty or unrecognised input — there is intentionally no "yes/no" support
// here so the documented OTel spec values stay the only happy path.
func parseBool(s string, def bool) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "":
		return def
	case "true", "1":
		return true
	case "false", "0":
		return false
	default:
		return def
	}
}
