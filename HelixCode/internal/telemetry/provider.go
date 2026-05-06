// Provider construction for the telemetry package: wires the OTel SDK
// TracerProvider + MeterProvider against the exporter selected by
// TelemetryConfig and returns a TelemetryProvider implementation that
// callers can use end-to-end.
//
// Contract (T05):
//   - cfg.Enabled=false OR cfg.Exporter=ExporterNoop -> noopTelemetryProvider.
//   - cfg.Exporter=ExporterStdout -> real provider with stdouttrace+stdoutmetric.
//   - cfg.Exporter=ExporterOTLPGRPC -> real provider with otlptracegrpc+otlpmetricgrpc.
//   - cfg.Exporter=ExporterOTLPHTTP -> real provider with otlptracehttp+otlpmetrichttp.
//   - cfg.Exporter is anything else -> ErrUnsupportedExporter; fallback to noop.
//
// Resilience: if exporter construction fails (bad endpoint, transient
// collector failure during connect), NewTelemetryProvider logs a warning and
// returns a noop provider with a non-nil error so the agent keeps running.
// Telemetry must NEVER block the user's primary flow.
//
// Spec: docs/superpowers/specs/2026-05-06-p1-f16-telemetry-design.md §3.
// Plan: docs/superpowers/plans/2026-05-06-p1-f16-telemetry.md T05.
package telemetry

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/zap"
)

// stdoutWriter is the destination for the stdout exporters. Tests swap this
// with a *bytes.Buffer (see provider_test.go captureStdout) so they can
// inspect emitted spans without redirecting os.Stdout. Production keeps the
// default of os.Stdout.
var stdoutWriter io.Writer = os.Stdout

// realTelemetryProvider is the production *TelemetryProvider impl backed by
// the OTel SDK. Constructed via NewTelemetryProvider.
type realTelemetryProvider struct {
	cfg            TelemetryConfig
	log            *zap.Logger
	tracerProvider *sdktrace.TracerProvider
	meterProvider  *sdkmetric.MeterProvider

	closeOnce sync.Once
	closeErr  error
}

// NewTelemetryProvider constructs a TelemetryProvider per cfg.
//
// Failure mode: exporter construction errors return a noop provider (so the
// agent keeps running) AND the wrapping error so callers can log/observe the
// degradation. A nil logger is replaced with zap.NewNop so callers don't have
// to nil-check.
func NewTelemetryProvider(cfg TelemetryConfig, log *zap.Logger) (TelemetryProvider, error) {
	if log == nil {
		log = zap.NewNop()
	}

	// Disabled or noop exporter -> straight to noop.
	if !cfg.Enabled || cfg.Exporter == ExporterNoop {
		return newNoopTelemetryProvider(cfg), nil
	}

	// Reject unknown exporter kinds with a noop fallback so the agent keeps
	// running rather than crashing on a misconfiguration.
	if !cfg.Exporter.IsValid() {
		err := fmt.Errorf("%w: %q", ErrUnsupportedExporter, cfg.Exporter)
		log.Warn("telemetry: unsupported exporter, falling back to noop",
			zap.String("exporter", string(cfg.Exporter)),
			zap.Error(err),
		)
		return newNoopTelemetryProvider(cfg), err
	}

	cfg = applyDefaults(cfg)

	res, err := buildResource(cfg)
	if err != nil {
		log.Warn("telemetry: resource build failed, falling back to noop",
			zap.Error(err),
		)
		return newNoopTelemetryProvider(cfg), fmt.Errorf("build resource: %w", err)
	}

	ctx := context.Background()

	traceExp, err := buildTraceExporter(ctx, cfg)
	if err != nil {
		log.Warn("telemetry: trace exporter construction failed, falling back to noop",
			zap.String("exporter", string(cfg.Exporter)),
			zap.Error(err),
		)
		return newNoopTelemetryProvider(cfg), fmt.Errorf("build trace exporter: %w", err)
	}

	metricExp, err := buildMetricExporter(ctx, cfg)
	if err != nil {
		// Best-effort: tear down the already-constructed trace exporter.
		_ = traceExp.Shutdown(ctx)
		log.Warn("telemetry: metric exporter construction failed, falling back to noop",
			zap.String("exporter", string(cfg.Exporter)),
			zap.Error(err),
		)
		return newNoopTelemetryProvider(cfg), fmt.Errorf("build metric exporter: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExp,
			sdktrace.WithBatchTimeout(cfg.BatchTimeout),
			sdktrace.WithExportTimeout(cfg.ExportTimeout),
		),
		sdktrace.WithResource(res),
	)

	// PeriodicReader interval: reuse BatchTimeout so trace + metric flush
	// rhythms stay aligned. Operators tune both via OTEL_BSP_SCHEDULE_DELAY.
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExp,
			sdkmetric.WithInterval(cfg.BatchTimeout),
			sdkmetric.WithTimeout(cfg.ExportTimeout),
		)),
		sdkmetric.WithResource(res),
	)

	return &realTelemetryProvider{
		cfg:            cfg,
		log:            log,
		tracerProvider: tp,
		meterProvider:  mp,
	}, nil
}

// Tracer returns an OTel Tracer named for the given component.
func (r *realTelemetryProvider) Tracer(name string) trace.Tracer {
	return r.tracerProvider.Tracer(name)
}

// Meter returns an OTel Meter for the given component.
func (r *realTelemetryProvider) Meter(name string) metric.Meter {
	return r.meterProvider.Meter(name)
}

// Config returns a snapshot of the active configuration. Mutations to the
// returned value do NOT affect provider state (slices/maps are intentionally
// shared since TelemetryConfig is treated as read-only after construction —
// the contract is "snapshot", not "deep copy").
func (r *realTelemetryProvider) Config() TelemetryConfig {
	return r.cfg
}

// Exporter returns the exporter kind currently wired.
func (r *realTelemetryProvider) Exporter() ExporterKind {
	return r.cfg.Exporter
}

// ForceFlush flushes both the tracer and meter providers. Returns the first
// non-nil error encountered.
func (r *realTelemetryProvider) ForceFlush(ctx context.Context) error {
	var firstErr error
	if err := r.tracerProvider.ForceFlush(ctx); err != nil {
		firstErr = fmt.Errorf("tracer flush: %w", err)
	}
	if err := r.meterProvider.ForceFlush(ctx); err != nil && firstErr == nil {
		firstErr = fmt.Errorf("meter flush: %w", err)
	}
	return firstErr
}

// Shutdown flushes + tears down both providers. Idempotent via sync.Once: a
// second call returns the first call's error (nil after a clean shutdown).
func (r *realTelemetryProvider) Shutdown(ctx context.Context) error {
	r.closeOnce.Do(func() {
		var firstErr error
		if err := r.tracerProvider.Shutdown(ctx); err != nil {
			firstErr = fmt.Errorf("tracer shutdown: %w", err)
		}
		if err := r.meterProvider.Shutdown(ctx); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("meter shutdown: %w", err)
		}
		r.closeErr = firstErr
	})
	return r.closeErr
}

// noopTelemetryProvider is returned when telemetry is disabled or an exporter
// could not be constructed. Every method is safe for concurrent use and never
// errors.
type noopTelemetryProvider struct {
	cfg            TelemetryConfig
	tracerProvider trace.TracerProvider
	meterProvider  metric.MeterProvider
}

func newNoopTelemetryProvider(cfg TelemetryConfig) *noopTelemetryProvider {
	// Force the Exporter field to ExporterNoop so callers querying
	// p.Exporter() get an honest answer ("we are disabled") regardless of
	// what was originally requested.
	cfg.Exporter = ExporterNoop
	return &noopTelemetryProvider{
		cfg:            cfg,
		tracerProvider: tracenoop.NewTracerProvider(),
		meterProvider:  metricnoop.NewMeterProvider(),
	}
}

func (n *noopTelemetryProvider) Tracer(name string) trace.Tracer {
	return n.tracerProvider.Tracer(name)
}

func (n *noopTelemetryProvider) Meter(name string) metric.Meter {
	return n.meterProvider.Meter(name)
}

func (n *noopTelemetryProvider) Config() TelemetryConfig          { return n.cfg }
func (n *noopTelemetryProvider) Exporter() ExporterKind           { return ExporterNoop }
func (n *noopTelemetryProvider) ForceFlush(_ context.Context) error { return nil }
func (n *noopTelemetryProvider) Shutdown(_ context.Context) error   { return nil }

// applyDefaults fills any zero-value timeout fields with the package defaults
// declared in types.go. Mutates a local copy; the input is unchanged.
func applyDefaults(cfg TelemetryConfig) TelemetryConfig {
	if cfg.BatchTimeout <= 0 {
		cfg.BatchTimeout = DefaultBatchTimeout
	}
	if cfg.ExportTimeout <= 0 {
		cfg.ExportTimeout = DefaultExportTimeout
	}
	if cfg.ShutdownTimeout <= 0 {
		cfg.ShutdownTimeout = DefaultShutdownTimeout
	}
	if cfg.ServiceName == "" {
		cfg.ServiceName = DefaultServiceName
	}
	return cfg
}

// buildResource composes the OTel *resource.Resource attached to every span
// and metric stream. Includes the service name and any operator-supplied
// resource attributes.
//
// We deliberately use resource.NewSchemaless rather than merging with
// resource.Default() because the SDK's default resource embeds a different
// semconv schema URL (currently v1.40.0) which conflicts with our pinned
// semconv import. The merge would error out with "conflicting Schema URL".
// Returning a schemaless resource with our own attributes keeps the wiring
// stable across SDK upgrades.
func buildResource(cfg TelemetryConfig) (*resource.Resource, error) {
	attrs := make([]attribute.KeyValue, 0, 1+len(cfg.ResourceAttrs))
	attrs = append(attrs, semconv.ServiceName(cfg.ServiceName))
	for k, v := range cfg.ResourceAttrs {
		// Skip empty keys defensively — config.parseResourceAttrs already
		// filters these out, but guard against direct struct construction.
		if k == "" {
			continue
		}
		attrs = append(attrs, attribute.String(k, v))
	}
	return resource.NewSchemaless(attrs...), nil
}

// buildTraceExporter constructs the SpanExporter for the configured exporter
// kind. Returns ErrUnsupportedExporter for unknown kinds (defence in depth —
// the caller already validates via IsValid).
func buildTraceExporter(ctx context.Context, cfg TelemetryConfig) (sdktrace.SpanExporter, error) {
	switch cfg.Exporter {
	case ExporterStdout:
		return stdouttrace.New(
			stdouttrace.WithWriter(stdoutWriter),
		)
	case ExporterOTLPGRPC:
		opts := []otlptracegrpc.Option{
			otlptracegrpc.WithTimeout(cfg.ExportTimeout),
		}
		if cfg.Endpoint != "" {
			opts = append(opts, otlptracegrpc.WithEndpoint(cfg.Endpoint))
		}
		if cfg.Insecure {
			opts = append(opts, otlptracegrpc.WithInsecure())
		}
		return otlptracegrpc.New(ctx, opts...)
	case ExporterOTLPHTTP:
		opts := []otlptracehttp.Option{
			otlptracehttp.WithTimeout(cfg.ExportTimeout),
		}
		if cfg.Endpoint != "" {
			opts = append(opts, otlptracehttp.WithEndpointURL(cfg.Endpoint))
		}
		if cfg.Insecure {
			opts = append(opts, otlptracehttp.WithInsecure())
		}
		return otlptracehttp.New(ctx, opts...)
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnsupportedExporter, cfg.Exporter)
	}
}

// buildMetricExporter is the metric-side mirror of buildTraceExporter.
func buildMetricExporter(ctx context.Context, cfg TelemetryConfig) (sdkmetric.Exporter, error) {
	switch cfg.Exporter {
	case ExporterStdout:
		return stdoutmetric.New(
			stdoutmetric.WithWriter(stdoutWriter),
		)
	case ExporterOTLPGRPC:
		opts := []otlpmetricgrpc.Option{
			otlpmetricgrpc.WithTimeout(cfg.ExportTimeout),
		}
		if cfg.Endpoint != "" {
			opts = append(opts, otlpmetricgrpc.WithEndpoint(cfg.Endpoint))
		}
		if cfg.Insecure {
			opts = append(opts, otlpmetricgrpc.WithInsecure())
		}
		return otlpmetricgrpc.New(ctx, opts...)
	case ExporterOTLPHTTP:
		opts := []otlpmetrichttp.Option{
			otlpmetrichttp.WithTimeout(cfg.ExportTimeout),
		}
		if cfg.Endpoint != "" {
			opts = append(opts, otlpmetrichttp.WithEndpointURL(cfg.Endpoint))
		}
		if cfg.Insecure {
			opts = append(opts, otlpmetrichttp.WithInsecure())
		}
		return otlpmetrichttp.New(ctx, opts...)
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnsupportedExporter, cfg.Exporter)
	}
}

