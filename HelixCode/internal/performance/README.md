# Performance Package

The `performance` package provides performance profiling and optimization for the HelixCode platform.

## Overview

This package handles:
- Performance profiling
- Benchmarking
- Memory analysis
- CPU profiling
- Bottleneck detection

## Key Types

### Profiler

```go
type Profiler struct {
    config    *Config
    metrics   *MetricsCollector
    analyzer  *Analyzer
}
```

### Profile

```go
type Profile struct {
    Type      ProfileType
    StartTime time.Time
    EndTime   time.Time
    Data      []byte
    Analysis  *Analysis
}
```

## Usage

### Profiling

```go
import "dev.helix.code/internal/performance"

profiler := performance.NewProfiler(config)

// CPU profiling
profile, err := profiler.CPUProfile(ctx, 30*time.Second)

// Memory profiling
profile, err := profiler.MemoryProfile(ctx)

// Block profiling
profile, err := profiler.BlockProfile(ctx)
```

### Benchmarking

```go
result, err := profiler.Benchmark(ctx, func() {
    // Code to benchmark
})

fmt.Printf("Duration: %v\n", result.Duration)
fmt.Printf("Memory: %d bytes\n", result.MemoryUsed)
```

### Analysis

```go
analysis := profiler.Analyze(ctx, profile)
for _, issue := range analysis.Issues {
    fmt.Printf("Issue: %s\n", issue.Description)
}
```

## Configuration

```yaml
performance:
  profiling:
    enabled: true
    cpu_rate: 100
  benchmarking:
    iterations: 1000
```

## Testing

```bash
go test -v ./internal/performance/...
```
