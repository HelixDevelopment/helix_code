# Monitoring Package

The `monitoring` package provides metrics and monitoring for the HelixCode platform.

## Overview

This package handles:
- Metrics collection
- Health monitoring
- Performance tracking
- Resource usage monitoring
- Alerting integration

## Key Types

### Monitor

```go
type Monitor struct {
    metrics   *MetricsCollector
    health    *HealthChecker
    alerter   *Alerter
    config    *Config
}
```

### Metrics

```go
type Metrics struct {
    Counters   map[string]*Counter
    Gauges     map[string]*Gauge
    Histograms map[string]*Histogram
}
```

## Usage

### Creating the Monitor

```go
import "dev.helix.code/internal/monitoring"

monitor := monitoring.NewMonitor(config)
err := monitor.Start(ctx)
```

### Recording Metrics

```go
// Counter (cumulative)
monitor.IncrementCounter("requests_total")
monitor.AddCounter("bytes_processed", 1024)

// Gauge (current value)
monitor.SetGauge("active_connections", 42)
monitor.SetGauge("memory_usage_bytes", memStats.Alloc)

// Histogram (distribution)
monitor.RecordHistogram("request_duration_ms", duration.Milliseconds())
```

### Labels

```go
// Metrics with labels
monitor.IncrementCounterWithLabels("requests_total", map[string]string{
    "method": "GET",
    "status": "200",
    "path":   "/api/v1/users",
})
```

### Health Checks

```go
// Register health check
monitor.RegisterHealthCheck("database", func(ctx context.Context) error {
    return db.Ping(ctx)
})

monitor.RegisterHealthCheck("redis", func(ctx context.Context) error {
    return redis.Ping(ctx)
})

// Run health checks
status := monitor.CheckHealth(ctx)
// Returns: {"database": "healthy", "redis": "healthy", "status": "healthy"}
```

### Resource Monitoring

```go
// CPU usage
cpu := monitor.GetCPUUsage()

// Memory usage
mem := monitor.GetMemoryUsage()

// Goroutine count
goroutines := monitor.GetGoroutineCount()

// System stats
stats := monitor.GetSystemStats()
```

### Alerting

```go
// Configure alert
alert := &monitoring.Alert{
    Name:      "high_memory",
    Condition: "memory_usage_percent > 90",
    Severity:  monitoring.SeverityCritical,
    Channels:  []string{"slack", "email"},
}

monitor.RegisterAlert(alert)

// Manual alert
monitor.TriggerAlert(ctx, "high_memory", "Memory usage at 95%")
```

## Prometheus Integration

```go
// Export Prometheus metrics
handler := monitor.PrometheusHandler()
http.Handle("/metrics", handler)
```

Example metrics output:
```
# HELP requests_total Total number of requests
# TYPE requests_total counter
requests_total{method="GET",status="200"} 1234

# HELP active_connections Current active connections
# TYPE active_connections gauge
active_connections 42

# HELP request_duration_ms Request duration histogram
# TYPE request_duration_ms histogram
request_duration_ms_bucket{le="10"} 100
request_duration_ms_bucket{le="50"} 200
request_duration_ms_bucket{le="100"} 250
```

## Configuration

```yaml
monitoring:
  enabled: true
  metrics_interval: 10s
  health_check_interval: 30s

  prometheus:
    enabled: true
    path: "/metrics"

  alerts:
    - name: high_cpu
      condition: "cpu_usage > 80"
      severity: warning
      channels: ["slack"]

    - name: high_memory
      condition: "memory_usage > 90"
      severity: critical
      channels: ["slack", "email"]
```

## Middleware

```go
// Add monitoring middleware
router.Use(monitoring.Middleware(monitor))

// Automatically records:
// - Request count
// - Request duration
// - Response status codes
// - Active requests
```

## Testing

```bash
go test -v ./internal/monitoring/...
```

## Notes

- Use counters for cumulative values
- Use gauges for current state
- Use histograms for latency distribution
- Configure meaningful alerts
- Export metrics for external monitoring
