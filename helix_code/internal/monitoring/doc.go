// Package monitoring provides metrics collection and system monitoring for the HelixCode platform.
//
// The monitoring package offers a collector-based architecture for gathering system metrics,
// health checks, and operational data. It supports periodic collection and aggregation of
// metrics from multiple sources.
//
// # Architecture
//
// The package uses a collector pattern:
//   - Monitor: Central component that manages collectors and stores metrics
//   - Collector: Interface for metric sources that gather specific data
//
// # Collector Interface
//
// Custom collectors implement the Collector interface:
//
//	type Collector interface {
//	    Name() string
//	    Collect() (map[string]interface{}, error)
//	}
//
// # Basic Usage
//
// Creating a monitor and collecting metrics:
//
//	monitor := monitoring.NewMonitor()
//
//	// Add collectors
//	monitor.AddCollector(cpuCollector)
//	monitor.AddCollector(memoryCollector)
//	monitor.AddCollector(goroutineCollector)
//
//	// Collect metrics once
//	err := monitor.CollectMetrics(ctx)
//
//	// Get all metrics
//	metrics := monitor.GetAllMetrics()
//
// # Custom Collectors
//
// Create collectors for specific metric sources:
//
//	type CPUCollector struct{}
//
//	func (c *CPUCollector) Name() string {
//	    return "cpu"
//	}
//
//	func (c *CPUCollector) Collect() (map[string]interface{}, error) {
//	    return map[string]interface{}{
//	        "cpu_usage_percent": getCPUUsage(),
//	        "cpu_count":         runtime.NumCPU(),
//	    }, nil
//	}
//
//	monitor.AddCollector(&CPUCollector{})
//
// # Periodic Collection
//
// Start automatic periodic metric collection:
//
//	ctx, cancel := context.WithCancel(context.Background())
//	defer cancel()
//
//	// Collect metrics every 30 seconds
//	go monitor.StartPeriodicCollection(ctx, 30*time.Second)
//
// # Retrieving Metrics
//
// Get individual or all metrics:
//
//	// Get specific metric
//	value, exists := monitor.GetMetric("cpu_usage_percent")
//	if exists {
//	    fmt.Printf("CPU Usage: %v%%\n", value)
//	}
//
//	// Get all metrics
//	allMetrics := monitor.GetAllMetrics()
//	for key, value := range allMetrics {
//	    fmt.Printf("%s: %v\n", key, value)
//	}
//
// # Health Checks
//
// Perform system health checks:
//
//	err := monitor.HealthCheck()
//	if err != nil {
//	    log.Printf("Health check failed: %v", err)
//	}
//
// # Example Metrics
//
// Common metrics that can be collected:
//
//   - cpu_usage_percent: Current CPU utilization
//   - memory_used_bytes: Memory in use
//   - memory_available_bytes: Available memory
//   - goroutine_count: Number of active goroutines
//   - heap_alloc_bytes: Heap memory allocated
//   - gc_pause_ns: Last GC pause duration
//   - open_files: Number of open file descriptors
//   - active_connections: Current network connections
//
// # Integration with HTTP
//
// Expose metrics via HTTP endpoint:
//
//	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
//	    metrics := monitor.GetAllMetrics()
//	    json.NewEncoder(w).Encode(metrics)
//	})
//
// # Example: Memory Collector
//
//	type MemoryCollector struct{}
//
//	func (c *MemoryCollector) Name() string {
//	    return "memory"
//	}
//
//	func (c *MemoryCollector) Collect() (map[string]interface{}, error) {
//	    var m runtime.MemStats
//	    runtime.ReadMemStats(&m)
//
//	    return map[string]interface{}{
//	        "heap_alloc_bytes":   m.HeapAlloc,
//	        "heap_sys_bytes":     m.HeapSys,
//	        "heap_objects":       m.HeapObjects,
//	        "gc_cycles":          m.NumGC,
//	        "gc_pause_total_ns":  m.PauseTotalNs,
//	    }, nil
//	}
//
// # Example: Goroutine Collector
//
//	type GoroutineCollector struct{}
//
//	func (c *GoroutineCollector) Name() string {
//	    return "goroutines"
//	}
//
//	func (c *GoroutineCollector) Collect() (map[string]interface{}, error) {
//	    return map[string]interface{}{
//	        "goroutine_count": runtime.NumGoroutine(),
//	    }, nil
//	}
//
// # Thread Safety
//
// The Monitor is thread-safe. Metrics can be collected and read concurrently
// from multiple goroutines. The periodic collection runs in its own goroutine
// and safely updates the shared metrics map.
package monitoring
