# Performance Package

The `performance` package provides production-grade performance optimization and profiling capabilities for the HelixCode platform. It implements a comprehensive framework for analyzing, tuning, and monitoring various aspects of the system to achieve optimal throughput, latency, and resource utilization in production environments.

## Overview

This package handles:
- Comprehensive performance profiling across multiple dimensions
- Production-grade optimization with measurable improvements
- Real-time metrics collection and analysis
- Before/after improvement measurement with detailed reporting
- Multi-dimensional optimization (CPU, memory, GC, network, database, cache, workers, LLM)

## Architecture

The performance system is centered around the `PerformanceOptimizer` which manages:

1. **Configuration-driven optimization targets** - Define goals for throughput, latency, CPU, and memory
2. **Multiple optimization types** - Nine distinct optimization categories
3. **Real-time metrics collection** - Continuous monitoring of system performance
4. **Before/after measurement** - Quantifiable improvement tracking
5. **Comprehensive reporting** - Detailed reports saved to `reports/performance/`

## Key Types

### PerformanceOptimizer

The main orchestrator for all performance optimization activities:

```go
type PerformanceOptimizer struct {
    config        PerformanceConfig
    metrics       *PerformanceMetrics
    optimizations map[string]Optimization
    mutex         sync.RWMutex
    running       atomic.Bool
}
```

The optimizer uses atomic operations and proper synchronization to ensure thread-safe operation. Only one optimization session can run at a time, enforced by atomic state management.

### PerformanceConfig

Configuration structure controlling which optimizations are enabled and their targets:

```go
type PerformanceConfig struct {
    CPUOptimization         bool    `json:"cpu_optimization"`
    MemoryOptimization      bool    `json:"memory_optimization"`
    GarbageCollection       bool    `json:"garbage_collection"`
    ConcurrencyOptimization bool    `json:"concurrency_optimization"`
    CacheOptimization       bool    `json:"cache_optimization"`
    NetworkOptimization     bool    `json:"network_optimization"`
    DatabaseOptimization    bool    `json:"database_optimization"`
    WorkerOptimization      bool    `json:"worker_optimization"`
    LLMOptimization         bool    `json:"llm_optimization"`
    TargetThroughput        int     `json:"target_throughput"`
    TargetLatency           string  `json:"target_latency"`
    TargetCPUUtilization    float64 `json:"target_cpu_utilization"`
    TargetMemoryUsage       int64   `json:"target_memory_usage"`
    MaxResponseTime         string  `json:"max_response_time"`
    MinCacheHitRate         float64 `json:"min_cache_hit_rate"`
    MaxErrorRate            float64 `json:"max_error_rate"`
}
```

### PerformanceMetrics

Comprehensive metrics structure tracking all performance dimensions:

```go
type PerformanceMetrics struct {
    Timestamp         time.Time     `json:"timestamp"`
    CPUUtilization    float64       `json:"cpu_utilization"`
    MemoryUsage       int64         `json:"memory_usage_bytes"`
    GCStats           GCStats       `json:"gc_stats"`
    Throughput        int           `json:"throughput_per_second"`
    AverageLatency    time.Duration `json:"average_latency"`
    P95Latency        time.Duration `json:"p95_latency"`
    P99Latency        time.Duration `json:"p99_latency"`
    CacheHitRate      float64       `json:"cache_hit_rate"`
    ErrorRate         float64       `json:"error_rate"`
    WorkerUtilization []float64     `json:"worker_utilization"`
    LLMResponseTime   time.Duration `json:"llm_response_time"`
    DatabaseQueryTime time.Duration `json:"database_query_time"`
    NetworkLatency    time.Duration `json:"network_latency"`
}
```

### GCStats

Detailed garbage collection statistics from Go runtime:

```go
type GCStats struct {
    NumGC        uint32        `json:"num_gc"`
    TotalGC      time.Duration `json:"total_gc"`
    PauseTotalNs uint64        `json:"pause_total_ns"`
    PauseNs      [256]uint64   `json:"pause_ns"`
    HeapAlloc    uint64        `json:"heap_alloc_bytes"`
    HeapSys      uint64        `json:"heap_sys_bytes"`
    HeapIdle     uint64        `json:"heap_idle_bytes"`
    HeapInuse    uint64        `json:"heap_inuse_bytes"`
    StackInuse   uint64        `json:"stack_inuse_bytes"`
}
```

### Optimization Types

The package supports nine optimization categories:

```go
const (
    CPUOpt           OptType = "cpu"
    MemoryOpt        OptType = "memory"
    GCOpt            OptType = "garbage_collection"
    ConcurrencyOpt   OptType = "concurrency"
    CacheOpt         OptType = "cache"
    NetworkOpt       OptType = "network"
    DatabaseOpt      OptType = "database"
    WorkerOpt        OptType = "worker"
    LLMOpt           OptType = "llm"
    ComprehensiveOpt OptType = "comprehensive"
)
```

## Usage Examples

### Basic Production Optimization

```go
import "dev.helix.code/internal/performance"

config := performance.PerformanceConfig{
    CPUOptimization:         true,
    MemoryOptimization:      true,
    GarbageCollection:       true,
    ConcurrencyOptimization: true,
    CacheOptimization:       true,
    NetworkOptimization:     true,
    DatabaseOptimization:    true,
    WorkerOptimization:      true,
    LLMOptimization:         true,
    TargetThroughput:        1000,
    TargetLatency:           "100ms",
    TargetCPUUtilization:    70.0,
    TargetMemoryUsage:       1024 * 1024 * 1024, // 1GB
    MinCacheHitRate:         0.95,
    MaxErrorRate:            0.01,
}

optimizer, err := performance.NewPerformanceOptimizer(config)
if err != nil {
    log.Fatal(err)
}

ctx := context.Background()
result, err := optimizer.StartProductionOptimization(ctx)
if err != nil {
    log.Fatal(err)
}

log.Printf("Overall improvement: %.1f%%", result.OverallImprovement.OverallScore)
log.Printf("Throughput improvement: %.1f%%", result.OverallImprovement.ThroughputImprovement)
log.Printf("Latency improvement: %.1f%%", result.OverallImprovement.LatencyImprovement)
```

### Selective Optimization

Enable only specific optimization categories:

```go
config := performance.PerformanceConfig{
    CPUOptimization:    true,
    MemoryOptimization: true,
    CacheOptimization:  true,
    TargetThroughput:   500,
}

optimizer, _ := performance.NewPerformanceOptimizer(config)
result, _ := optimizer.StartProductionOptimization(ctx)

// Check specific optimization results
for name, opt := range result.Optimizations {
    if opt.Results != nil && opt.Results.Success {
        log.Printf("%s: %.1f%% improvement", name, opt.Results.Improvement)
    }
}
```

### Analyzing Optimization Results

```go
result, _ := optimizer.StartProductionOptimization(ctx)

// Access baseline metrics
log.Printf("Baseline CPU: %.1f%%", result.Baseline.CPUUtilization)
log.Printf("Baseline Memory: %d MB", result.Baseline.MemoryUsage/(1024*1024))
log.Printf("Baseline Throughput: %d ops/sec", result.Baseline.Throughput)

// Access post-optimization metrics
log.Printf("Final CPU: %.1f%%", result.PostOptimization.CPUUtilization)
log.Printf("Final Memory: %d MB", result.PostOptimization.MemoryUsage/(1024*1024))
log.Printf("Final Throughput: %d ops/sec", result.PostOptimization.Throughput)

// Check production readiness
if result.PostOptimization.Throughput >= config.TargetThroughput {
    log.Println("Production ready: throughput target met")
}
```

## Optimization Categories

### CPU Optimization

- **Goroutine Pool**: Implements goroutine pools for CPU-intensive operations
- **Benchmark Optimization**: Optimizes code paths based on benchmark results
- Configuration: `pool_size` based on `runtime.NumCPU() * 2`

### Memory Optimization

- **Memory Pool**: Object pooling to reduce allocations
- **Memory Profiling**: Profiling-based improvements with configurable intervals
- Configuration: `pool_size: 1000`, `max_allocations: 100000`

### Garbage Collection Optimization

- **GC Tuning**: Optimizes GOGC and GOMAXPROCS settings
- **GC Monitoring**: Continuous monitoring with alerting
- Configuration: `GOGC: 100`, `TargetPauseTime: 10ms`

### Concurrency Optimization

- **Concurrency Patterns**: Optimizes patterns for resource utilization
- **Lock Optimization**: Reduces lock contention with lock-free patterns
- Configuration: `max_concurrent: 1000`, `worker_pool_size: 100`

### Cache Optimization

- **Cache Strategy**: LRU and Redis caching strategies
- **Cache Warming**: Pre-populates frequently accessed data
- Configuration: `lru_cache_size: 10000`, `cache_ttl: 1h`, `cache_hit_rate_target: 0.95`

### Network Optimization

- **Connection Pooling**: HTTP connection pool management
- **Network Compression**: Configurable compression levels
- Configuration: `max_connections: 100`, `compression_level: 6`

### Database Optimization

- **Connection Pool**: Database connection pool sizing
- **Query Optimization**: Query timeout and index recommendations
- Configuration: `max_connections: 50`, `query_timeout: 30s`

### Worker Optimization

- **Worker Scaling**: Dynamic scaling based on workload
- **CPU Affinity**: Optional CPU affinity for worker processes
- Configuration: `min_workers: 10`, `max_workers: 100`, `scale_factor: 0.8`

### LLM Optimization

- **Request Batching**: Batches LLM requests for efficiency
- **Response Caching**: Caches LLM responses with LRU strategy
- Configuration: `batch_size: 10`, `cache_ttl: 24h`, `cache_size: 10000`

## Configuration

### YAML Configuration

```yaml
performance:
  profiling:
    enabled: true
    cpu_rate: 100
  benchmarking:
    iterations: 1000
  optimization:
    cpu: true
    memory: true
    gc: true
    concurrency: true
    cache: true
    network: true
    database: true
    worker: true
    llm: true
  targets:
    throughput: 1000
    latency: "100ms"
    cpu_utilization: 70.0
    memory_usage: 1073741824
    cache_hit_rate: 0.95
    max_error_rate: 0.01
```

## Optimization Result Structure

### OptimizationResult

Complete results from an optimization session:

```go
type OptimizationResult struct {
    StartTime          time.Time               `json:"start_time"`
    EndTime            time.Time               `json:"end_time"`
    Duration           time.Duration           `json:"duration"`
    TotalApplied       int                     `json:"total_applied"`
    Successful         int                     `json:"successful"`
    Failed             int                     `json:"failed"`
    Baseline           *PerformanceMetrics     `json:"baseline"`
    PostOptimization   *PerformanceMetrics     `json:"post_optimization"`
    Optimizations      map[string]Optimization `json:"optimizations"`
    OverallImprovement *OverallImprovement     `json:"overall_improvement"`
}
```

### OverallImprovement

Aggregate improvement metrics:

```go
type OverallImprovement struct {
    ThroughputImprovement float64 `json:"throughput_improvement_percent"`
    LatencyImprovement    float64 `json:"latency_improvement_percent"`
    MemoryImprovement     float64 `json:"memory_improvement_percent"`
    CPUImprovement        float64 `json:"cpu_improvement_percent"`
    OverallScore          float64 `json:"overall_improvement_score"`
}
```

## Best Practices

### 1. Start with Baseline Measurement

Always collect baseline metrics before optimization to measure actual improvements.

### 2. Enable Optimizations Incrementally

Start with high-impact optimizations (CPU, Memory, Cache) before enabling all.

### 3. Set Realistic Targets

Configure targets based on actual production requirements, not arbitrary numbers.

### 4. Monitor Production Readiness

Use the built-in production readiness evaluation to determine deployment readiness.

### 5. Review Generated Reports

Check `reports/performance/production_optimization_report.txt` for detailed analysis.

## Integration Patterns

### With Monitoring Systems

```go
result, _ := optimizer.StartProductionOptimization(ctx)

// Export to Prometheus
prometheus.MustRegister(prometheus.NewGaugeFunc(
    prometheus.GaugeOpts{
        Name: "helix_cpu_utilization",
        Help: "Current CPU utilization percentage",
    },
    func() float64 { return result.PostOptimization.CPUUtilization },
))
```

### With Alerting

```go
if result.PostOptimization.ErrorRate > config.MaxErrorRate {
    alertManager.Send("Performance: Error rate exceeded threshold")
}

if result.PostOptimization.CacheHitRate < config.MinCacheHitRate {
    alertManager.Send("Performance: Cache hit rate below minimum")
}
```

## Testing

```bash
# Run all performance tests
go test -v ./internal/performance/...

# Run with coverage
go test -cover ./internal/performance/...

# Run benchmarks
go test -bench=. ./internal/performance/...
```

## Thread Safety

The `PerformanceOptimizer` uses:
- `sync.RWMutex` for protecting optimization state
- `atomic.Bool` for running state management
- Only one optimization session can run at a time

## Notes

- Optimization reports are saved to `reports/performance/` directory
- Each optimization type creates two optimizations (18 total when all enabled)
- Production readiness is evaluated against configured targets
- Failed optimizations are logged but do not stop the optimization process
