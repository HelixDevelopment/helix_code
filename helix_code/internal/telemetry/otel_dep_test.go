package telemetry_test

import (
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

// TestOTelDepsImportable proves the OpenTelemetry v1.30.0 dependencies are
// wired so subsequent F16 tasks can build against them. P1-F16-T02 deliverable.
func TestOTelDepsImportable(t *testing.T) {
	_ = otel.Tracer
	_ = sdk.Version
	_ = sdkmetric.NewMeterProvider
	_ = otlptracegrpc.NewClient
	_ = otlptracehttp.NewClient
	_ = otlpmetricgrpc.New
	_ = otlpmetrichttp.New
	_ = stdouttrace.New
	_ = stdoutmetric.New
}
