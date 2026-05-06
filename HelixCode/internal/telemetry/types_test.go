package telemetry

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/trace"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

// TestExporterKind_String — table-driven check that the typed constants
// stringify to their documented wire-protocol identifiers, exactly the
// values OTEL_EXPORTER_OTLP_PROTOCOL accepts (or "stdout"/"noop" sentinels).
func TestExporterKind_String(t *testing.T) {
	cases := []struct {
		k    ExporterKind
		want string
	}{
		{ExporterStdout, "stdout"},
		{ExporterOTLPGRPC, "grpc"},
		{ExporterOTLPHTTP, "http/protobuf"},
		{ExporterNoop, "noop"},
	}
	for _, c := range cases {
		if got := string(c.k); got != c.want {
			t.Errorf("ExporterKind %q = %q, want %q", c.k, got, c.want)
		}
	}
}

// TestExporterKind_IsValid_AcceptsKnown — every documented kind is valid.
func TestExporterKind_IsValid_AcceptsKnown(t *testing.T) {
	for _, k := range []ExporterKind{ExporterStdout, ExporterOTLPGRPC, ExporterOTLPHTTP, ExporterNoop} {
		if !k.IsValid() {
			t.Errorf("ExporterKind %q reported as invalid; want valid", k)
		}
	}
}

// TestExporterKind_IsValid_RejectsUnknown — unrecognised values are invalid.
func TestExporterKind_IsValid_RejectsUnknown(t *testing.T) {
	for _, k := range []ExporterKind{"", "jaeger", "zipkin", "OTLP", "STDOUT", "http"} {
		if k.IsValid() {
			t.Errorf("ExporterKind %q reported as valid; want invalid", k)
		}
	}
}

// TestTelemetryConfig_ZeroValueIsNoop — the zero-value config is "telemetry off".
// Production code that forgets to populate TelemetryConfig must NEVER accidentally
// enable export.
func TestTelemetryConfig_ZeroValueIsNoop(t *testing.T) {
	var cfg TelemetryConfig
	if cfg.Enabled {
		t.Fatalf("zero-value TelemetryConfig.Enabled = true; want false")
	}
	if cfg.Exporter != "" {
		t.Errorf("zero-value Exporter = %q; want empty", cfg.Exporter)
	}
	if cfg.ServiceName != "" {
		t.Errorf("zero-value ServiceName = %q; want empty", cfg.ServiceName)
	}
}

// TestDefaultBlockedAttributeKeys_NotEmpty — sanity floor: at least 15 entries.
// Catches accidental erasure of the deny-list.
func TestDefaultBlockedAttributeKeys_NotEmpty(t *testing.T) {
	if got := len(DefaultBlockedAttributeKeys); got < 15 {
		t.Errorf("DefaultBlockedAttributeKeys has %d entries; want >= 15", got)
	}
}

// TestDefaultBlockedAttributeKeys_ContainsCriticalKeys — table of must-have keys.
// Each key documented in CONST-042 § "default-deny posture" is present.
func TestDefaultBlockedAttributeKeys_ContainsCriticalKeys(t *testing.T) {
	required := []string{
		// Generic creds
		"api_key", "apikey", "api-key",
		"token", "auth", "authorization", "bearer",
		"password", "passwd", "secret",
		// Provider creds
		"anthropic_api_key", "openai_api_key", "google_api_key",
		"aws_access_key_id", "aws_secret_access_key", "azure_openai_api_key",
		// Prompt-body keys
		"prompt", "prompt_body", "request_body", "response_body",
		"messages", "completion",
	}
	have := make(map[string]bool, len(DefaultBlockedAttributeKeys))
	for _, k := range DefaultBlockedAttributeKeys {
		have[strings.ToLower(k)] = true
	}
	for _, want := range required {
		if !have[strings.ToLower(want)] {
			t.Errorf("DefaultBlockedAttributeKeys missing critical key %q", want)
		}
	}
}

// TestAttributeBlocked_DefaultMatches — a key in the default list is blocked
// even when the caller passes no extras.
func TestAttributeBlocked_DefaultMatches(t *testing.T) {
	if !AttributeBlocked("api_key", nil) {
		t.Errorf(`AttributeBlocked("api_key", nil) = false; want true`)
	}
}

// TestAttributeBlocked_CaseInsensitive — exact-match comparison must be
// case-insensitive so "API_KEY" and "Api-Key" both block.
func TestAttributeBlocked_CaseInsensitive(t *testing.T) {
	cases := []string{"API_KEY", "Api_Key", "AUTHORIZATION", "Authorization", "PASSWORD"}
	for _, k := range cases {
		if !AttributeBlocked(k, nil) {
			t.Errorf("AttributeBlocked(%q, nil) = false; want true (case-insensitive)", k)
		}
	}
}

// TestAttributeBlocked_BenignNotMatched — keys that are NOT in the deny-list
// pass through. Catches a regression where the matcher becomes a substring
// or prefix match.
func TestAttributeBlocked_BenignNotMatched(t *testing.T) {
	for _, k := range []string{"model", "duration_ms", "tool_name", "agent_id", ""} {
		if AttributeBlocked(k, nil) {
			t.Errorf("AttributeBlocked(%q, nil) = true; want false (benign)", k)
		}
	}
}

// TestAttributeBlocked_PartialMatchNotMatched — exact-match only; substring
// matches must NOT fire. "my_api_key_thing" contains "api_key" but is not
// equal to it, so it is allowed. (Operators who want it blocked add it
// explicitly via TelemetryConfig.BlockedAttributeKeys.)
func TestAttributeBlocked_PartialMatchNotMatched(t *testing.T) {
	if AttributeBlocked("my_api_key_thing", nil) {
		t.Errorf(`AttributeBlocked("my_api_key_thing", nil) = true; want false (no substring match)`)
	}
	if AttributeBlocked("not_a_secret_field", nil) {
		t.Errorf(`AttributeBlocked("not_a_secret_field", nil) = true; want false (no substring match)`)
	}
}

// TestAttributeBlocked_ExtraKeyAccepted — caller-supplied extra keys are
// honoured (case-insensitive exact match).
func TestAttributeBlocked_ExtraKeyAccepted(t *testing.T) {
	extra := []string{"custom_secret", "internal_token_v2"}
	if !AttributeBlocked("custom_secret", extra) {
		t.Errorf(`AttributeBlocked("custom_secret", extra) = false; want true`)
	}
	if !AttributeBlocked("CUSTOM_SECRET", extra) {
		t.Errorf(`AttributeBlocked("CUSTOM_SECRET", extra) = false; want true (case-insensitive over extras)`)
	}
	if AttributeBlocked("model", extra) {
		t.Errorf(`AttributeBlocked("model", extra) = true; want false (model not in extras and not in defaults)`)
	}
}

// TestAttributeBlocked_UserCannotSubtract — the load-bearing CONST-042 test:
// even when extra is nil, every default key still blocks. There is no API
// to remove an entry from DefaultBlockedAttributeKeys at runtime.
func TestAttributeBlocked_UserCannotSubtract(t *testing.T) {
	for _, k := range DefaultBlockedAttributeKeys {
		if !AttributeBlocked(k, nil) {
			t.Errorf("default key %q is unblocked when extra=nil; CONST-042 violation", k)
		}
		// And still blocked even when the user passes a (different) extra slice.
		if !AttributeBlocked(k, []string{"unrelated_field"}) {
			t.Errorf("default key %q is unblocked when extra=[unrelated_field]; CONST-042 violation", k)
		}
	}
}

// TestFilterAttributes_DropsBlockedKeys — denied keys are removed; allowed
// keys are preserved.
func TestFilterAttributes_DropsBlockedKeys(t *testing.T) {
	in := []attribute.KeyValue{
		attribute.String("api_key", "sk-redacted"),
		attribute.String("model", "claude"),
		attribute.String("authorization", "Bearer xyz"),
		attribute.Int("duration_ms", 42),
	}
	out := FilterAttributes(in, nil)
	if len(out) != 2 {
		t.Fatalf("FilterAttributes len = %d; want 2 (kept: model, duration_ms)", len(out))
	}
	keys := make([]string, 0, len(out))
	for _, kv := range out {
		keys = append(keys, string(kv.Key))
	}
	wantKeep := map[string]bool{"model": true, "duration_ms": true}
	for _, k := range keys {
		if !wantKeep[k] {
			t.Errorf("FilterAttributes kept unexpected key %q", k)
		}
	}
}

// TestFilterAttributes_PreservesOrder — relative order of surviving
// attributes is unchanged.
func TestFilterAttributes_PreservesOrder(t *testing.T) {
	in := []attribute.KeyValue{
		attribute.String("a", "1"),
		attribute.String("api_key", "drop"),
		attribute.String("b", "2"),
		attribute.String("token", "drop"),
		attribute.String("c", "3"),
	}
	out := FilterAttributes(in, nil)
	if len(out) != 3 {
		t.Fatalf("FilterAttributes len = %d; want 3", len(out))
	}
	wantOrder := []string{"a", "b", "c"}
	for i, kv := range out {
		if string(kv.Key) != wantOrder[i] {
			t.Errorf("FilterAttributes[%d].Key = %q; want %q", i, kv.Key, wantOrder[i])
		}
	}
}

// TestFilterAttributes_EmptyInputReturnsEmpty — empty slice in, non-nil empty
// slice out (no panics, no surprise nil).
func TestFilterAttributes_EmptyInputReturnsEmpty(t *testing.T) {
	out := FilterAttributes([]attribute.KeyValue{}, nil)
	if len(out) != 0 {
		t.Errorf("FilterAttributes(empty, nil) len = %d; want 0", len(out))
	}
}

// TestFilterAttributes_NilInputReturnsEmpty — nil slice in, non-nil empty
// slice out. Documented behaviour: callers can always range over the result.
func TestFilterAttributes_NilInputReturnsEmpty(t *testing.T) {
	out := FilterAttributes(nil, nil)
	if len(out) != 0 {
		t.Errorf("FilterAttributes(nil, nil) len = %d; want 0", len(out))
	}
	// Iteration must be safe.
	for range out {
		t.Errorf("FilterAttributes(nil, nil) yielded an element on range")
	}
}

// TestSentinelErrors_Distinct — the three sentinels are not aliases of each
// other; errors.Is comparisons stay precise.
func TestSentinelErrors_Distinct(t *testing.T) {
	if errors.Is(ErrTelemetryDisabled, ErrUnsupportedExporter) {
		t.Errorf("ErrTelemetryDisabled aliases ErrUnsupportedExporter")
	}
	if errors.Is(ErrTelemetryDisabled, ErrTelemetryNotInitialised) {
		t.Errorf("ErrTelemetryDisabled aliases ErrTelemetryNotInitialised")
	}
	if errors.Is(ErrUnsupportedExporter, ErrTelemetryNotInitialised) {
		t.Errorf("ErrUnsupportedExporter aliases ErrTelemetryNotInitialised")
	}
	// Each error is itself.
	if !errors.Is(ErrTelemetryDisabled, ErrTelemetryDisabled) {
		t.Errorf("ErrTelemetryDisabled is not errors.Is itself (stdlib invariant violated)")
	}
}

// TestDefaultTimeouts — defaults match the values documented in spec §3.
func TestDefaultTimeouts(t *testing.T) {
	if DefaultBatchTimeout != 5*time.Second {
		t.Errorf("DefaultBatchTimeout = %v; want 5s", DefaultBatchTimeout)
	}
	if DefaultExportTimeout != 30*time.Second {
		t.Errorf("DefaultExportTimeout = %v; want 30s", DefaultExportTimeout)
	}
	if DefaultShutdownTimeout != 5*time.Second {
		t.Errorf("DefaultShutdownTimeout = %v; want 5s", DefaultShutdownTimeout)
	}
	if DefaultServiceName != "helixcode" {
		t.Errorf("DefaultServiceName = %q; want %q", DefaultServiceName, "helixcode")
	}
}

// stubProvider is a no-op TelemetryProvider used solely to confirm at compile
// time that the interface signature is satisfiable from outside the package.
// If this struct stops compiling, the TelemetryProvider contract has changed
// in a breaking way and downstream tasks (T05/T06/T07) need to be updated.
type stubProvider struct {
	cfg TelemetryConfig
}

func (s *stubProvider) Tracer(name string) trace.Tracer       { return tracenoop.NewTracerProvider().Tracer(name) }
func (s *stubProvider) Meter(name string) metric.Meter        { return metricnoop.NewMeterProvider().Meter(name) }
func (s *stubProvider) Config() TelemetryConfig               { return s.cfg }
func (s *stubProvider) Exporter() ExporterKind                { return ExporterNoop }
func (s *stubProvider) ForceFlush(ctx context.Context) error  { return nil }
func (s *stubProvider) Shutdown(ctx context.Context) error    { return nil }

// TestTelemetryProvider_InterfaceCompiles — compile-time + runtime sanity:
// stubProvider implements TelemetryProvider, the typed Tracer/Meter return
// values are usable, and the no-op exporter sentinel round-trips.
func TestTelemetryProvider_InterfaceCompiles(t *testing.T) {
	var p TelemetryProvider = &stubProvider{cfg: TelemetryConfig{Enabled: false}}
	if p.Exporter() != ExporterNoop {
		t.Errorf("stubProvider.Exporter() = %q; want %q", p.Exporter(), ExporterNoop)
	}
	if p.Config().Enabled {
		t.Errorf("stubProvider.Config().Enabled = true; want false")
	}
	if p.Tracer("test") == nil {
		t.Errorf("stubProvider.Tracer returned nil; want non-nil noop tracer")
	}
	if p.Meter("test") == nil {
		t.Errorf("stubProvider.Meter returned nil; want non-nil noop meter")
	}
	if err := p.ForceFlush(context.Background()); err != nil {
		t.Errorf("stubProvider.ForceFlush = %v; want nil", err)
	}
	if err := p.Shutdown(context.Background()); err != nil {
		t.Errorf("stubProvider.Shutdown = %v; want nil", err)
	}
}
