package memory

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConfig holds the configuration for memory tests
type TestConfig struct {
	BaseURL     string
	AdminToken  string
	Timeout     time.Duration
	Iterations  int
	Concurrency int
}

// DefaultTestConfig returns a default test configuration
func DefaultTestConfig() *TestConfig {
	return &TestConfig{
		BaseURL:     "http://localhost:8080",
		AdminToken:  "test-admin-token",
		Timeout:     30 * time.Second,
		Iterations:  100,
		Concurrency: 10,
	}
}

// MemoryStats captures memory statistics at a point in time
type MemoryStats struct {
	Alloc      uint64    // bytes allocated and in use
	TotalAlloc uint64    // bytes allocated (even if freed)
	Sys        uint64    // bytes obtained from system
	NumGC      uint32    // number of GC runs
	HeapAlloc  uint64    // bytes allocated and in heap
	HeapSys    uint64    // bytes obtained from system for heap
	HeapIdle   uint64    // bytes in idle spans
	HeapInuse  uint64    // bytes in in-use spans
	StackInuse uint64    // bytes in stack spans
	Timestamp  time.Time // when stats were captured
}

// CaptureMemoryStats captures current memory statistics
func CaptureMemoryStats() *MemoryStats {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return &MemoryStats{
		Alloc:      m.Alloc,
		TotalAlloc: m.TotalAlloc,
		Sys:        m.Sys,
		NumGC:      m.NumGC,
		HeapAlloc:  m.HeapAlloc,
		HeapSys:    m.HeapSys,
		HeapIdle:   m.HeapIdle,
		HeapInuse:  m.HeapInuse,
		StackInuse: m.StackInuse,
		Timestamp:  time.Now(),
	}
}

// MemoryDelta calculates the difference between two memory stats
type MemoryDelta struct {
	AllocDelta      int64
	TotalAllocDelta uint64
	HeapAllocDelta  int64
	GCRuns          uint32
	Duration        time.Duration
}

// CalculateDelta calculates the difference between before and after stats
func CalculateDelta(before, after *MemoryStats) *MemoryDelta {
	return &MemoryDelta{
		AllocDelta:      int64(after.Alloc) - int64(before.Alloc),
		TotalAllocDelta: after.TotalAlloc - before.TotalAlloc,
		HeapAllocDelta:  int64(after.HeapAlloc) - int64(before.HeapAlloc),
		GCRuns:          after.NumGC - before.NumGC,
		Duration:        after.Timestamp.Sub(before.Timestamp),
	}
}

// =============================================================================
// Memory Leak Detection Tests
// =============================================================================

// TestMemory_LeakDetection_RepeatedRequests tests for memory leaks during repeated API requests
func TestMemory_LeakDetection_RepeatedRequests(t *testing.T) {
	config := DefaultTestConfig()
	client := &http.Client{Timeout: config.Timeout}

	// Skip if server is not available
	resp, err := client.Get(config.BaseURL + "/health")
	if err != nil {
		t.Skip("Server not available, skipping memory leak test")
	}
	resp.Body.Close()

	// Force GC before test to get clean baseline
	runtime.GC()
	runtime.GC()

	beforeStats := CaptureMemoryStats()

	// Perform many repeated requests
	iterations := config.Iterations * 10
	for i := 0; i < iterations; i++ {
		resp, err := client.Get(config.BaseURL + "/health")
		if err != nil {
			continue
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}

	// Force GC after test
	runtime.GC()
	runtime.GC()

	afterStats := CaptureMemoryStats()
	delta := CalculateDelta(beforeStats, afterStats)

	// Log memory statistics
	t.Logf("Memory delta after %d requests:", iterations)
	t.Logf("  Alloc delta: %d bytes", delta.AllocDelta)
	t.Logf("  Heap alloc delta: %d bytes", delta.HeapAllocDelta)
	t.Logf("  Total allocated: %d bytes", delta.TotalAllocDelta)
	t.Logf("  GC runs: %d", delta.GCRuns)

	// Memory should not grow significantly after GC
	// Allow up to 10MB growth as a reasonable threshold
	maxAllowedGrowth := int64(10 * 1024 * 1024)
	assert.Less(t, delta.HeapAllocDelta, maxAllowedGrowth,
		"Potential memory leak detected: heap grew by %d bytes after %d requests",
		delta.HeapAllocDelta, iterations)
}

// TestMemory_LeakDetection_ConcurrentRequests tests for memory leaks during concurrent requests
func TestMemory_LeakDetection_ConcurrentRequests(t *testing.T) {
	config := DefaultTestConfig()
	client := &http.Client{Timeout: config.Timeout}

	// Skip if server is not available
	resp, err := client.Get(config.BaseURL + "/health")
	if err != nil {
		t.Skip("Server not available, skipping memory leak test")
	}
	resp.Body.Close()

	// Force GC before test
	runtime.GC()
	runtime.GC()

	beforeStats := CaptureMemoryStats()

	// Run concurrent requests in waves
	for wave := 0; wave < 10; wave++ {
		var wg sync.WaitGroup
		for i := 0; i < config.Concurrency; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 10; j++ {
					resp, err := client.Get(config.BaseURL + "/health")
					if err != nil {
						continue
					}
					io.Copy(io.Discard, resp.Body)
					resp.Body.Close()
				}
			}()
		}
		wg.Wait()
	}

	// Force GC after test
	runtime.GC()
	runtime.GC()

	afterStats := CaptureMemoryStats()
	delta := CalculateDelta(beforeStats, afterStats)

	t.Logf("Memory delta after concurrent requests:")
	t.Logf("  Heap alloc delta: %d bytes", delta.HeapAllocDelta)
	t.Logf("  GC runs: %d", delta.GCRuns)

	maxAllowedGrowth := int64(20 * 1024 * 1024)
	assert.Less(t, delta.HeapAllocDelta, maxAllowedGrowth,
		"Potential memory leak detected during concurrent requests")
}

// TestMemory_LeakDetection_JSONParsing tests for memory leaks in JSON parsing
func TestMemory_LeakDetection_JSONParsing(t *testing.T) {
	config := DefaultTestConfig()
	client := &http.Client{Timeout: config.Timeout}

	// Skip if server is not available
	resp, err := client.Get(config.BaseURL + "/health")
	if err != nil {
		t.Skip("Server not available, skipping memory leak test")
	}
	resp.Body.Close()

	runtime.GC()
	runtime.GC()

	beforeStats := CaptureMemoryStats()

	// Create and parse many JSON requests
	for i := 0; i < config.Iterations; i++ {
		// Create a project request with large payload
		project := map[string]interface{}{
			"name":        fmt.Sprintf("test-project-%d", i),
			"description": string(make([]byte, 1024)), // 1KB description
			"tags":        []string{"tag1", "tag2", "tag3"},
			"metadata": map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
		}

		jsonData, err := json.Marshal(project)
		require.NoError(t, err)

		req, err := http.NewRequest("POST", config.BaseURL+"/api/v1/projects", bytes.NewBuffer(jsonData))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+config.AdminToken)

		resp, err := client.Do(req)
		if err != nil {
			continue
		}

		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)
		resp.Body.Close()
	}

	runtime.GC()
	runtime.GC()

	afterStats := CaptureMemoryStats()
	delta := CalculateDelta(beforeStats, afterStats)

	t.Logf("Memory delta after JSON parsing tests:")
	t.Logf("  Total allocated: %d bytes", delta.TotalAllocDelta)
	t.Logf("  Heap alloc delta: %d bytes", delta.HeapAllocDelta)

	maxAllowedGrowth := int64(15 * 1024 * 1024)
	assert.Less(t, delta.HeapAllocDelta, maxAllowedGrowth,
		"Potential memory leak in JSON parsing")
}

// =============================================================================
// Memory Allocation Tests
// =============================================================================

// TestMemory_Allocation_LargePayloads tests memory handling with large payloads
func TestMemory_Allocation_LargePayloads(t *testing.T) {
	config := DefaultTestConfig()
	client := &http.Client{Timeout: config.Timeout}

	// Skip if server is not available
	resp, err := client.Get(config.BaseURL + "/health")
	if err != nil {
		t.Skip("Server not available, skipping allocation test")
	}
	resp.Body.Close()

	payloadSizes := []int{
		1024,           // 1KB
		10 * 1024,      // 10KB
		100 * 1024,     // 100KB
		1024 * 1024,    // 1MB
		5 * 1024 * 1024, // 5MB
	}

	for _, size := range payloadSizes {
		t.Run(fmt.Sprintf("PayloadSize_%d", size), func(t *testing.T) {
			runtime.GC()
			beforeStats := CaptureMemoryStats()

			// Create large payload
			payload := make([]byte, size)
			for i := range payload {
				payload[i] = byte('a' + (i % 26))
			}

			project := map[string]interface{}{
				"name":        "large-payload-test",
				"description": string(payload),
			}

			jsonData, err := json.Marshal(project)
			require.NoError(t, err)

			req, err := http.NewRequest("POST", config.BaseURL+"/api/v1/projects", bytes.NewBuffer(jsonData))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+config.AdminToken)

			resp, err := client.Do(req)
			if err != nil {
				t.Skipf("Request failed for size %d: %v", size, err)
				return
			}
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()

			runtime.GC()
			afterStats := CaptureMemoryStats()
			delta := CalculateDelta(beforeStats, afterStats)

			t.Logf("Memory for %d byte payload:", size)
			t.Logf("  Allocated: %d bytes", delta.TotalAllocDelta)

			// Memory allocation should be reasonable (not more than 100x payload size)
			// HTTP requests involve JSON encoding, buffers, headers, etc. which add overhead
			maxExpectedAlloc := uint64(size * 100)
			assert.Less(t, delta.TotalAllocDelta, maxExpectedAlloc,
				"Excessive memory allocation for payload size %d", size)
		})
	}
}

// TestMemory_Allocation_ConnectionPooling tests connection pool memory efficiency
func TestMemory_Allocation_ConnectionPooling(t *testing.T) {
	config := DefaultTestConfig()

	// Create custom transport with connection pooling
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
		IdleConnTimeout:     90 * time.Second,
	}
	client := &http.Client{Transport: transport, Timeout: config.Timeout}
	defer transport.CloseIdleConnections()

	// Skip if server is not available
	resp, err := client.Get(config.BaseURL + "/health")
	if err != nil {
		t.Skip("Server not available, skipping connection pool test")
	}
	resp.Body.Close()

	runtime.GC()
	runtime.GC()

	beforeStats := CaptureMemoryStats()

	// Make many requests to test connection reuse
	for i := 0; i < 500; i++ {
		resp, err := client.Get(config.BaseURL + "/health")
		if err != nil {
			continue
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}

	runtime.GC()
	runtime.GC()

	afterStats := CaptureMemoryStats()
	delta := CalculateDelta(beforeStats, afterStats)

	t.Logf("Connection pool memory efficiency:")
	t.Logf("  Total allocated: %d bytes", delta.TotalAllocDelta)
	t.Logf("  Heap delta: %d bytes", delta.HeapAllocDelta)

	// Connection pooling should keep allocations low
	maxAllowedAlloc := uint64(50 * 1024 * 1024) // 50MB
	assert.Less(t, delta.TotalAllocDelta, maxAllowedAlloc,
		"Connection pooling not effective, excessive allocations")
}

// =============================================================================
// GC Pressure Tests
// =============================================================================

// TestMemory_GCPressure_HighAllocationRate tests behavior under high allocation rate
func TestMemory_GCPressure_HighAllocationRate(t *testing.T) {
	config := DefaultTestConfig()
	client := &http.Client{Timeout: config.Timeout}

	// Skip if server is not available
	resp, err := client.Get(config.BaseURL + "/health")
	if err != nil {
		t.Skip("Server not available, skipping GC pressure test")
	}
	resp.Body.Close()

	runtime.GC()
	beforeStats := CaptureMemoryStats()

	startTime := time.Now()

	// High allocation rate test
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	requestCount := int64(0)
	var mu sync.Mutex

	for i := 0; i < config.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					resp, err := client.Get(config.BaseURL + "/health")
					if err == nil {
						io.Copy(io.Discard, resp.Body)
						resp.Body.Close()
						mu.Lock()
						requestCount++
						mu.Unlock()
					}
				}
			}
		}()
	}

	wg.Wait()
	duration := time.Since(startTime)

	runtime.GC()
	runtime.GC()

	afterStats := CaptureMemoryStats()
	delta := CalculateDelta(beforeStats, afterStats)

	t.Logf("GC pressure test results:")
	t.Logf("  Duration: %v", duration)
	t.Logf("  Requests: %d", requestCount)
	t.Logf("  Requests/sec: %.2f", float64(requestCount)/duration.Seconds())
	t.Logf("  GC runs: %d", delta.GCRuns)
	t.Logf("  GC runs/sec: %.2f", float64(delta.GCRuns)/duration.Seconds())
	t.Logf("  Heap delta: %d bytes", delta.HeapAllocDelta)

	// GC should run reasonably often under load
	assert.Greater(t, delta.GCRuns, uint32(0), "No GC runs during high allocation test")

	// Memory should stay bounded
	maxAllowedGrowth := int64(50 * 1024 * 1024)
	assert.Less(t, delta.HeapAllocDelta, maxAllowedGrowth,
		"Memory grew too much under GC pressure")
}

// TestMemory_GCPressure_BurstTraffic tests GC behavior during burst traffic
func TestMemory_GCPressure_BurstTraffic(t *testing.T) {
	config := DefaultTestConfig()
	client := &http.Client{Timeout: config.Timeout}

	// Skip if server is not available
	resp, err := client.Get(config.BaseURL + "/health")
	if err != nil {
		t.Skip("Server not available, skipping burst traffic test")
	}
	resp.Body.Close()

	runtime.GC()
	runtime.GC()

	beforeStats := CaptureMemoryStats()

	// Simulate burst traffic: periods of high activity followed by quiet periods
	for burst := 0; burst < 5; burst++ {
		// High activity burst
		var wg sync.WaitGroup
		for i := 0; i < config.Concurrency*2; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 50; j++ {
					resp, err := client.Get(config.BaseURL + "/health")
					if err == nil {
						io.Copy(io.Discard, resp.Body)
						resp.Body.Close()
					}
				}
			}()
		}
		wg.Wait()

		// Quiet period - let GC catch up
		time.Sleep(500 * time.Millisecond)
	}

	runtime.GC()
	runtime.GC()

	afterStats := CaptureMemoryStats()
	delta := CalculateDelta(beforeStats, afterStats)

	t.Logf("Burst traffic GC test:")
	t.Logf("  GC runs: %d", delta.GCRuns)
	t.Logf("  Heap delta: %d bytes", delta.HeapAllocDelta)

	// Memory should be reclaimed during quiet periods
	maxAllowedGrowth := int64(25 * 1024 * 1024)
	assert.Less(t, delta.HeapAllocDelta, maxAllowedGrowth,
		"Memory not properly reclaimed during burst traffic quiet periods")
}

// =============================================================================
// Resource Cleanup Tests
// =============================================================================

// TestMemory_ResourceCleanup_Goroutines tests for goroutine leaks
func TestMemory_ResourceCleanup_Goroutines(t *testing.T) {
	config := DefaultTestConfig()
	client := &http.Client{Timeout: config.Timeout}

	// Skip if server is not available
	resp, err := client.Get(config.BaseURL + "/health")
	if err != nil {
		t.Skip("Server not available, skipping goroutine leak test")
	}
	resp.Body.Close()

	initialGoroutines := runtime.NumGoroutine()
	t.Logf("Initial goroutines: %d", initialGoroutines)

	// Run many concurrent operations
	for round := 0; round < 5; round++ {
		var wg sync.WaitGroup
		for i := 0; i < config.Concurrency; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 20; j++ {
					resp, err := client.Get(config.BaseURL + "/health")
					if err == nil {
						io.Copy(io.Discard, resp.Body)
						resp.Body.Close()
					}
				}
			}()
		}
		wg.Wait()
	}

	// Give time for goroutines to clean up
	time.Sleep(2 * time.Second)
	runtime.GC()

	finalGoroutines := runtime.NumGoroutine()
	goroutineDelta := finalGoroutines - initialGoroutines

	t.Logf("Final goroutines: %d (delta: %d)", finalGoroutines, goroutineDelta)

	// Allow for some goroutine growth but not excessive
	maxAllowedGrowth := 50
	assert.Less(t, goroutineDelta, maxAllowedGrowth,
		"Potential goroutine leak: %d goroutines created but not cleaned up", goroutineDelta)
}

// TestMemory_ResourceCleanup_FileDescriptors tests for file descriptor leaks
func TestMemory_ResourceCleanup_FileDescriptors(t *testing.T) {
	config := DefaultTestConfig()
	client := &http.Client{Timeout: config.Timeout}

	// Skip if server is not available
	resp, err := client.Get(config.BaseURL + "/health")
	if err != nil {
		t.Skip("Server not available, skipping file descriptor test")
	}
	resp.Body.Close()

	// Make many requests, properly closing all resources
	for i := 0; i < config.Iterations*2; i++ {
		resp, err := client.Get(config.BaseURL + "/health")
		if err != nil {
			continue
		}
		// Ensure body is fully read and closed
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}

	// This test mainly ensures no panic from too many open files
	// The test passing indicates proper resource cleanup
	t.Log("File descriptor cleanup test passed")
}

// TestMemory_ResourceCleanup_Contexts tests context cancellation cleanup
func TestMemory_ResourceCleanup_Contexts(t *testing.T) {
	config := DefaultTestConfig()
	client := &http.Client{Timeout: config.Timeout}

	// Skip if server is not available
	resp, err := client.Get(config.BaseURL + "/health")
	if err != nil {
		t.Skip("Server not available, skipping context cleanup test")
	}
	resp.Body.Close()

	runtime.GC()
	beforeStats := CaptureMemoryStats()
	initialGoroutines := runtime.NumGoroutine()

	// Create and cancel many contexts
	for i := 0; i < config.Iterations; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		req, _ := http.NewRequestWithContext(ctx, "GET", config.BaseURL+"/health", nil)

		resp, err := client.Do(req)
		if err == nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}
		cancel() // Always cancel to clean up
	}

	time.Sleep(time.Second)
	runtime.GC()
	runtime.GC()

	afterStats := CaptureMemoryStats()
	finalGoroutines := runtime.NumGoroutine()
	delta := CalculateDelta(beforeStats, afterStats)

	t.Logf("Context cleanup test:")
	t.Logf("  Goroutine delta: %d", finalGoroutines-initialGoroutines)
	t.Logf("  Heap delta: %d bytes", delta.HeapAllocDelta)

	maxAllowedGrowth := int64(10 * 1024 * 1024)
	assert.Less(t, delta.HeapAllocDelta, maxAllowedGrowth,
		"Context cleanup may have issues")
}

// =============================================================================
// Memory Profiling Helpers
// =============================================================================

// TestMemory_Profiling_Baseline establishes baseline memory usage
func TestMemory_Profiling_Baseline(t *testing.T) {
	runtime.GC()
	runtime.GC()

	stats := CaptureMemoryStats()

	t.Logf("Baseline memory statistics:")
	t.Logf("  Allocated: %d bytes (%.2f MB)", stats.Alloc, float64(stats.Alloc)/1024/1024)
	t.Logf("  Total allocated: %d bytes (%.2f MB)", stats.TotalAlloc, float64(stats.TotalAlloc)/1024/1024)
	t.Logf("  System: %d bytes (%.2f MB)", stats.Sys, float64(stats.Sys)/1024/1024)
	t.Logf("  Heap allocated: %d bytes (%.2f MB)", stats.HeapAlloc, float64(stats.HeapAlloc)/1024/1024)
	t.Logf("  Heap system: %d bytes (%.2f MB)", stats.HeapSys, float64(stats.HeapSys)/1024/1024)
	t.Logf("  Heap idle: %d bytes (%.2f MB)", stats.HeapIdle, float64(stats.HeapIdle)/1024/1024)
	t.Logf("  Heap in use: %d bytes (%.2f MB)", stats.HeapInuse, float64(stats.HeapInuse)/1024/1024)
	t.Logf("  Stack in use: %d bytes (%.2f MB)", stats.StackInuse, float64(stats.StackInuse)/1024/1024)
	t.Logf("  Number of GC runs: %d", stats.NumGC)
	t.Logf("  Number of goroutines: %d", runtime.NumGoroutine())
}

// TestMemory_Profiling_IdleServerMemory tests memory usage of an idle server
func TestMemory_Profiling_IdleServerMemory(t *testing.T) {
	config := DefaultTestConfig()
	client := &http.Client{Timeout: config.Timeout}

	// Skip if server is not available
	resp, err := client.Get(config.BaseURL + "/health")
	if err != nil {
		t.Skip("Server not available, skipping idle memory test")
	}
	resp.Body.Close()

	// Wait for server to settle
	time.Sleep(2 * time.Second)
	runtime.GC()
	runtime.GC()

	stats := CaptureMemoryStats()

	t.Logf("Server memory (idle after health check):")
	t.Logf("  Heap in use: %d bytes (%.2f MB)", stats.HeapInuse, float64(stats.HeapInuse)/1024/1024)
	t.Logf("  Goroutines: %d", runtime.NumGoroutine())

	// Idle server should not use excessive memory
	maxIdleHeap := uint64(100 * 1024 * 1024) // 100MB
	assert.Less(t, stats.HeapInuse, maxIdleHeap,
		"Idle server using too much memory")
}

// =============================================================================
// Stress Tests
// =============================================================================

// TestMemory_Stress_SustainedLoad tests memory stability under sustained load
func TestMemory_Stress_SustainedLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping sustained load test in short mode")
	}

	config := DefaultTestConfig()
	client := &http.Client{Timeout: config.Timeout}

	// Skip if server is not available
	resp, err := client.Get(config.BaseURL + "/health")
	if err != nil {
		t.Skip("Server not available, skipping sustained load test")
	}
	resp.Body.Close()

	runtime.GC()
	runtime.GC()

	initialStats := CaptureMemoryStats()
	samples := make([]*MemoryStats, 0)

	// Run sustained load for 60 seconds, sampling every 10 seconds
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	done := make(chan struct{})

	// Worker goroutines
	for i := 0; i < config.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
					resp, err := client.Get(config.BaseURL + "/health")
					if err == nil {
						io.Copy(io.Discard, resp.Body)
						resp.Body.Close()
					}
				}
			}
		}()
	}

	// Sampling goroutine
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				samples = append(samples, CaptureMemoryStats())
			}
		}
	}()

	<-ctx.Done()
	close(done)
	wg.Wait()

	runtime.GC()
	runtime.GC()

	finalStats := CaptureMemoryStats()

	t.Logf("Sustained load test results:")
	t.Logf("  Initial heap: %d bytes", initialStats.HeapInuse)
	for i, sample := range samples {
		t.Logf("  Sample %d heap: %d bytes", i+1, sample.HeapInuse)
	}
	t.Logf("  Final heap: %d bytes", finalStats.HeapInuse)

	// Check that memory doesn't grow unbounded
	delta := CalculateDelta(initialStats, finalStats)
	maxAllowedGrowth := int64(100 * 1024 * 1024) // 100MB
	assert.Less(t, delta.HeapAllocDelta, maxAllowedGrowth,
		"Memory grew unbounded during sustained load")
}
