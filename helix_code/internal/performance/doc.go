// Package performance provides production-grade performance optimization for HelixCode.
//
// This package implements a comprehensive performance optimization framework that
// analyzes, tunes, and monitors various aspects of the HelixCode system to achieve
// optimal throughput, latency, and resource utilization in production environments.
//
// # Architecture
//
// The performance system is centered around the PerformanceOptimizer which manages:
//
//   - Configuration-driven optimization targets
//   - Multiple optimization types (CPU, memory, GC, network, etc.)
//   - Real-time metrics collection and analysis
//   - Before/after improvement measurement
//   - Comprehensive reporting
//
// # Optimization Types
//
// The package supports the following optimization categories:
//
//   - CPU Optimization: Goroutine pools, benchmark-based tuning
//   - Memory Optimization: Object pooling, profiling-based improvements
//   - Garbage Collection: GC tuning, GOGC configuration, pause optimization
//   - Concurrency Optimization: Lock optimization, worker pool sizing
//   - Cache Optimization: Strategy tuning, cache warming
//   - Network Optimization: Connection pooling, compression
//   - Database Optimization: Connection pool sizing, query optimization
//   - Worker Optimization: Dynamic scaling, CPU affinity
//   - LLM Optimization: Request batching, response caching
//
// # Basic Usage
//
// Creating and running performance optimization:
//
//	config := performance.PerformanceConfig{
//	    CPUOptimization:         true,
//	    MemoryOptimization:      true,
//	    GarbageCollection:       true,
//	    ConcurrencyOptimization: true,
//	    TargetThroughput:        1000,
//	    TargetLatency:           "100ms",
//	    TargetCPUUtilization:    70.0,
//	    TargetMemoryUsage:       1024 * 1024 * 1024, // 1GB
//	}
//
//	optimizer, err := performance.NewPerformanceOptimizer(config)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	result, err := optimizer.StartProductionOptimization(ctx)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	log.Printf("Overall improvement: %.1f%%", result.OverallImprovement.OverallScore)
//
// # Configuration
//
// The PerformanceConfig structure controls which optimizations are enabled:
//
//	type PerformanceConfig struct {
//	    CPUOptimization         bool    // Enable CPU optimizations
//	    MemoryOptimization      bool    // Enable memory optimizations
//	    GarbageCollection       bool    // Enable GC tuning
//	    ConcurrencyOptimization bool    // Enable concurrency optimizations
//	    CacheOptimization       bool    // Enable cache optimizations
//	    NetworkOptimization     bool    // Enable network optimizations
//	    DatabaseOptimization    bool    // Enable database optimizations
//	    WorkerOptimization      bool    // Enable worker optimizations
//	    LLMOptimization         bool    // Enable LLM-specific optimizations
//	    TargetThroughput        int     // Target ops/sec
//	    TargetLatency           string  // Target latency
//	    TargetCPUUtilization    float64 // Target CPU percentage
//	    TargetMemoryUsage       int64   // Target memory in bytes
//	}
//
// # Metrics Collection
//
// The optimizer collects comprehensive runtime metrics:
//
//	type PerformanceMetrics struct {
//	    CPUUtilization    float64       // Current CPU usage percentage
//	    MemoryUsage       int64         // Heap allocation in bytes
//	    GCStats           GCStats       // Garbage collection statistics
//	    Throughput        int           // Operations per second
//	    AverageLatency    time.Duration // Mean response time
//	    P95Latency        time.Duration // 95th percentile latency
//	    P99Latency        time.Duration // 99th percentile latency
//	    CacheHitRate      float64       // Cache effectiveness
//	    ErrorRate         float64       // Request failure rate
//	    WorkerUtilization []float64     // Per-worker utilization
//	    LLMResponseTime   time.Duration // LLM response latency
//	    DatabaseQueryTime time.Duration // Database query latency
//	    NetworkLatency    time.Duration // Network round-trip time
//	}
//
// # GC Statistics
//
// Detailed garbage collection monitoring:
//
//	type GCStats struct {
//	    NumGC        uint32        // Number of completed GC cycles
//	    TotalGC      time.Duration // Total GC pause time
//	    PauseTotalNs uint64        // Total pause in nanoseconds
//	    HeapAlloc    uint64        // Bytes allocated on heap
//	    HeapSys      uint64        // Bytes obtained from system
//	    HeapIdle     uint64        // Bytes in idle spans
//	    HeapInuse    uint64        // Bytes in in-use spans
//	    StackInuse   uint64        // Bytes in stack spans
//	}
//
// # Optimization Results
//
// Each optimization produces measurable results:
//
//	type OptResult struct {
//	    Timestamp     time.Time // When optimization was applied
//	    Success       bool      // Whether optimization succeeded
//	    Improvement   float64   // Percentage improvement
//	    BeforeValue   float64   // Metric before optimization
//	    AfterValue    float64   // Metric after optimization
//	    MetricsChange string    // Description of changes
//	}
//
// # Overall Results
//
// The complete optimization session produces comprehensive results:
//
//	type OptimizationResult struct {
//	    StartTime          time.Time
//	    EndTime            time.Time
//	    Duration           time.Duration
//	    TotalApplied       int
//	    Successful         int
//	    Failed             int
//	    Baseline           *PerformanceMetrics
//	    PostOptimization   *PerformanceMetrics
//	    OverallImprovement *OverallImprovement
//	}
//
// # Improvement Tracking
//
// The optimizer calculates improvements across multiple dimensions:
//
//	type OverallImprovement struct {
//	    ThroughputImprovement float64 // Throughput % change
//	    LatencyImprovement    float64 // Latency % change
//	    MemoryImprovement     float64 // Memory usage % change
//	    CPUImprovement        float64 // CPU usage % change
//	    OverallScore          float64 // Average of all improvements
//	}
//
// # Reporting
//
// The optimizer generates detailed reports saved to reports/performance/:
//
//   - Baseline and post-optimization metrics
//   - Per-optimization results and improvements
//   - Recommendations for further optimization
//   - Production readiness assessment
//   - Key achievements summary
//
// # Production Readiness
//
// The optimizer evaluates whether targets are met:
//
//   - Throughput meets or exceeds target
//   - CPU utilization within target
//   - Memory usage within limits
//   - Cache hit rate above minimum
//   - Error rate below maximum
//
// # Thread Safety
//
// The PerformanceOptimizer uses atomic operations and proper synchronization
// to ensure thread-safe operation. Only one optimization session can run
// at a time, enforced by atomic state management.
package performance
