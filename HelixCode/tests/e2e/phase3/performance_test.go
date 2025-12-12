package phase3

import (
	"context"
	"fmt"
	"net/http"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"dev.helix.code/tests/e2e"
	"github.com/stretchr/testify/assert"
)

// PerformanceTestFramework extends testing for performance validation
type PerformanceTestFramework struct {
	*e2e.E2ETestFramework
	ConcurrentUsers int
	TestDuration    time.Duration
	Metrics         *PerformanceMetrics
}

// PerformanceMetrics tracks performance testing metrics
type PerformanceMetrics struct {
	TotalRequests      int64
	SuccessfulRequests int64
	FailedRequests     int64
	AverageResponseTime time.Duration
	P95ResponseTime     time.Duration
	P99ResponseTime     time.Duration
	MaxResponseTime     time.Duration
	MinResponseTime     time.Duration
	MemoryUsage         uint64
	CPUUsage            float64
	ErrorRate           float64
	Throughput          float64
}

// TestHighLoadAuthentication tests authentication under high load
func TestHighLoadAuthentication(t *testing.T) {
	t.Log("🚀 Testing authentication under high load...")
	
	framework := &PerformanceTestFramework{
		E2ETestFramework: e2e.NewE2ETestFramework(t),
		ConcurrentUsers:  100,
		TestDuration:     60 * time.Second,
		Metrics:          &PerformanceMetrics{},
	}
	defer framework.Cleanup(t)
	
	// Configure for real server
	framework.BaseURL = getProductionServerURL()
	
	// Run concurrent authentication tests
	ctx, cancel := context.WithTimeout(context.Background(), framework.TestDuration)
	defer cancel()
	
	var wg sync.WaitGroup
	successCount := int64(0)
	failureCount := int64(0)
	totalTime := int64(0)
	responseTimes := make([]time.Duration, 0, 1000)
	
	// Response time tracking
	var mu sync.Mutex
	
	// Launch concurrent users
	for i := 0; i < framework.ConcurrentUsers; i++ {
		wg.Add(1)
		go func(userID int) {
			defer wg.Done()
			
			for {
				select {
				case <-ctx.Done():
					return
				default:
					startTime := time.Now()
					
					// Test authentication
					authData := map[string]interface{}{
						"username": fmt.Sprintf("load_test_user_%d", userID),
						"email":    fmt.Sprintf("loadtest%d@helixcode.com", userID),
						"password": "LoadTestPass123!",
						"role":     "user",
					}
					
					resp, err := framework.POST(t, "/api/v1/auth/register", authData)
					duration := time.Since(startTime)
					
					if err != nil {
						atomic.AddInt64(&failureCount, 1)
						continue
					}
					defer resp.Body.Close()
					
					atomic.AddInt64(&framework.Metrics.TotalRequests, 1)
					
					// Track response times
					mu.Lock()
					responseTimes = append(responseTimes, duration)
					atomic.AddInt64(&totalTime, int64(duration))
					mu.Unlock()
					
					if resp.StatusCode == http.StatusCreated {
						atomic.AddInt64(&successCount, 1)
					} else if resp.StatusCode == http.StatusConflict {
						// User already exists, try login
						loginData := map[string]interface{}{
							"username": fmt.Sprintf("load_test_user_%d", userID),
							"password": "LoadTestPass123!",
						}
						
						loginStart := time.Now()
						loginResp, loginErr := framework.POST(t, "/api/v1/auth/login", loginData)
						loginDuration := time.Since(loginStart)
						
						if loginErr == nil && loginResp.StatusCode == http.StatusOK {
							atomic.AddInt64(&successCount, 1)
							loginResp.Body.Close()
							
							mu.Lock()
							responseTimes = append(responseTimes, loginDuration)
							atomic.AddInt64(&totalTime, int64(loginDuration))
							mu.Unlock()
						} else {
							atomic.AddInt64(&failureCount, 1)
							if loginResp != nil {
								loginResp.Body.Close()
							}
						}
					} else {
						atomic.AddInt64(&failureCount, 1)
					}
				}
			}
		}(i)
	}
	
	wg.Wait()
	
	// Calculate final metrics
	framework.Metrics.SuccessfulRequests = successCount
	framework.Metrics.FailedRequests = failureCount
	
	if successCount > 0 {
		framework.Metrics.AverageResponseTime = time.Duration(totalTime / successCount)
		
		// Calculate percentiles
		sort.Slice(responseTimes, func(i, j int) bool {
			return responseTimes[i] < responseTimes[j]
		})
		
		n := len(responseTimes)
		if n > 0 {
			framework.Metrics.P95ResponseTime = responseTimes[int(float64(n)*0.95)]
			framework.Metrics.P99ResponseTime = responseTimes[int(float64(n)*0.99)]
			framework.Metrics.MaxResponseTime = responseTimes[n-1]
			framework.Metrics.MinResponseTime = responseTimes[0]
		}
		
		// Calculate error rate and throughput
		totalRequests := successCount + failureCount
		if totalRequests > 0 {
			framework.Metrics.ErrorRate = float64(failureCount) / float64(totalRequests)
			framework.Metrics.Throughput = float64(totalRequests) / framework.TestDuration.Seconds()
		}
	}
	
	// Report results
	t.Logf("📊 Performance Metrics:")
	t.Logf("   Total Requests: %d", framework.Metrics.TotalRequests)
	t.Logf("   Successful Requests: %d (%.1f%%)", successCount, float64(successCount)/float64(framework.Metrics.TotalRequests)*100)
	t.Logf("   Failed Requests: %d (%.1f%%)", failureCount, framework.Metrics.ErrorRate*100)
	t.Logf("   Average Response Time: %v", framework.Metrics.AverageResponseTime)
	t.Logf("   P95 Response Time: %v", framework.Metrics.P95ResponseTime)
	t.Logf("   P99 Response Time: %v", framework.Metrics.P99ResponseTime)
	t.Logf("   Max Response Time: %v", framework.Metrics.MaxResponseTime)
	t.Logf("   Min Response Time: %v", framework.Metrics.MinResponseTime)
	t.Logf("   Error Rate: %.2f%%", framework.Metrics.ErrorRate*100)
	t.Logf("   Throughput: %.2f requests/second", framework.Metrics.Throughput)
	t.Logf("   Concurrent Users: %d", framework.ConcurrentUsers)
	t.Logf("   Test Duration: %v", framework.TestDuration)
	
	// Validate performance requirements
	assert.Greater(t, successCount, int64(0), "Should have successful requests")
	successRate := float64(successCount) / float64(framework.Metrics.TotalRequests)
	assert.Greater(t, successRate, 0.95, "Success rate should be > 95%")
	assert.Less(t, framework.Metrics.AverageResponseTime, 2*time.Second, "Average response time should be < 2s")
	assert.Less(t, framework.Metrics.P95ResponseTime, 5*time.Second, "P95 response time should be < 5s")
	assert.Less(t, framework.Metrics.ErrorRate, 0.05, "Error rate should be < 5%")
	
	t.Log("✅ High load authentication test completed successfully")
}

// TestConcurrentProjectOperations tests project operations under concurrent load
func TestConcurrentProjectOperations(t *testing.T) {
	t.Log("🏗️ Testing concurrent project operations...")
	
	framework := NewPhase3Framework(t)
	defer framework.Cleanup(t)
	
	// Set up authenticated user for project operations
	if len(framework.TestUsers) > 0 && framework.TestUsers[0].Token != "" {
		framework.TestUser = &e2e.TestUser{
			Token: framework.TestUsers[0].Token,
		}
	}
	
	// Test concurrent project creation
	projectCount := 50
	var wg sync.WaitGroup
	errors := make(chan error, projectCount)
	successCount := int64(0)
	
	for i := 0; i < projectCount; i++ {
		wg.Add(1)
		go func(projectID int) {
			defer wg.Done()
			
			projectData := map[string]interface{}{
				"name":        fmt.Sprintf("LoadTestProject_%d_%d", projectID, time.Now().UnixNano()),
				"description": "Project created during concurrent load testing",
				"type":        "go",
				"template":    "basic",
			}
			
			startTime := time.Now()
			resp, err := framework.POST(t, "/api/v1/projects", projectData)
			duration := time.Since(startTime)
			
			if err != nil {
				errors <- fmt.Errorf("project %d: creation failed: %v", projectID, err)
				return
			}
			defer resp.Body.Close()
			
			if resp.StatusCode == http.StatusCreated {
				atomic.AddInt64(&successCount, 1)
				t.Logf("✅ Project %d: Created successfully in %v", projectID, duration)
			} else if resp.StatusCode == http.StatusConflict {
				t.Logf("ℹ️ Project %d: Already exists (expected)", projectID)
			} else {
				errors <- fmt.Errorf("project %d: unexpected status %d", projectID, resp.StatusCode)
			}
		}(i)
	}
	
	wg.Wait()
	close(errors)
	
	// Check for errors
	errorCount := 0
	for err := range errors {
		t.Logf("⚠️ %v", err)
		errorCount++
	}
	
	successRate := float64(successCount) / float64(projectCount)
	t.Logf("📊 Concurrent Project Results:")
	t.Logf("   Total Projects: %d", projectCount)
	t.Logf("   Successful: %d (%.1f%%)", successCount, successRate*100)
	t.Logf("   Errors: %d (%.1f%%)", errorCount, float64(errorCount)/float64(projectCount)*100)
	
	assert.Greater(t, successRate, 0.90, "Success rate should be > 90%")
	assert.Less(t, errorCount, projectCount/10, "Error rate should be < 10%")
	
	t.Log("✅ Concurrent project operations test completed successfully")
}

// TestMemoryOptimization validates memory usage optimization
func TestMemoryOptimization(t *testing.T) {
	t.Log("🧠 Testing memory usage optimization...")
	
	// Record initial memory state
	var m1 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)
	initialMemory := m1.Alloc
	
	// Create framework and perform operations
	framework := NewPhase3Framework(t)
	defer framework.Cleanup(t)
	
	// Perform multiple test operations
	for i := 0; i < 100; i++ {
		resp, err := framework.GET(t, "/health")
		if err == nil {
			resp.Body.Close()
		}
	}
	
	// Force garbage collection and measure final state
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)
	finalMemory := m2.Alloc
	
	// Calculate metrics
	memoryIncrease := finalMemory - initialMemory
	memoryEfficiency := float64(initialMemory) / float64(finalMemory) * 100
	
	t.Logf("📊 Memory Usage Analysis:")
	t.Logf("   Initial Memory: %d bytes", initialMemory)
	t.Logf("   Final Memory: %d bytes", finalMemory)
	t.Logf("   Memory Increase: %d bytes", memoryIncrease)
	t.Logf("   Memory Efficiency: %.2f%%", memoryEfficiency)
	t.Logf("   Heap Objects: %d → %d", m1.HeapObjects, m2.HeapObjects)
	t.Logf("   GC Runs: %d", m2.NumGC)
	
	// Validate memory efficiency
	assert.Less(t, memoryIncrease, uint64(10*1024*1024), "Memory increase should be < 10MB")
	assert.Less(t, m2.HeapObjects, uint64(5000), "Should not have excessive heap objects")
	
	t.Log("✅ Memory optimization validated")
}

// TestResourceCleanup validates proper resource cleanup
func TestResourceCleanup(t *testing.T) {
	t.Log("🧹 Testing resource cleanup...")
	
	// Test multiple framework creations and cleanups
	for i := 0; i < 10; i++ {
		framework := NewPhase3Framework(t)
		
		// Perform some operations
		resp, err := framework.GET(t, "/health")
		if err == nil {
			resp.Body.Close()
		}
		
		// Cleanup
		framework.Cleanup(t)
	}
	
	// Force garbage collection and check for leaks
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	t.Logf("📊 Resource Cleanup Analysis:")
	t.Logf("   Heap Objects: %d", m.HeapObjects)
	t.Logf("   Heap Allocated: %d bytes", m.HeapAlloc)
	t.Logf("   Heap In Use: %d bytes", m.HeapInuse)
	t.Logf("   GC Runs: %d", m.NumGC)
	
	assert.Less(t, m.HeapObjects, uint64(10000), "Should not have excessive heap objects after cleanup")
	assert.Less(t, m.HeapAlloc, uint64(50*1024*1024), "Heap allocation should be reasonable")
	
	t.Log("✅ Resource cleanup validated")
}

// TestThroughputScalability tests system throughput under scaling
func TestThroughputScalability(t *testing.T) {
	t.Log("📈 Testing system throughput scalability...")
	
	framework := NewPhase3Framework(t)
	defer framework.Cleanup(t)
	
	// Test different load levels
	loadLevels := []struct {
		name            string
		requestsPerSecond int
		duration        time.Duration
	}{
		{
			name:            "Light Load",
			requestsPerSecond: 10,
			duration:        30 * time.Second,
		},
		{
			name:            "Medium Load",
			requestsPerSecond: 50,
			duration:        30 * time.Second,
		},
		{
			name:            "Heavy Load",
			requestsPerSecond: 100,
			duration:        30 * time.Second,
		},
	}
	
	for _, load := range loadLevels {
		t.Run(load.name, func(t *testing.T) {
			t.Logf("📈 Testing %s (%.0f req/s for %v)...", load.name, float64(load.requestsPerSecond), load.duration)
			
			startTime := time.Now()
			requestCount := 0
			successCount := 0
			var totalResponseTime time.Duration
			
			ticker := time.NewTicker(time.Second / time.Duration(load.requestsPerSecond))
			defer ticker.Stop()
			
			timeout := time.After(load.duration)
			
			for {
				select {
				case <-timeout:
					goto done
				case <-ticker.C:
					requestStart := time.Now()
					
					resp, err := framework.GET(t, "/health")
					if err == nil {
						resp.Body.Close()
						successCount++
					}
					
					requestCount++
					totalResponseTime += time.Since(requestStart)
				}
			}
			
		done:
			actualDuration := time.Since(startTime)
			actualThroughput := float64(requestCount) / actualDuration.Seconds()
			avgResponseTime := totalResponseTime / time.Duration(requestCount)
			successRate := float64(successCount) / float64(requestCount)
			
			t.Logf("📊 %s Results:", load.name)
			t.Logf("   Target Throughput: %.0f req/s", float64(load.requestsPerSecond))
			t.Logf("   Actual Throughput: %.2f req/s", actualThroughput)
			t.Logf("   Total Requests: %d", requestCount)
			t.Logf("   Successful Requests: %d (%.1f%%)", successCount, successRate*100)
			t.Logf("   Average Response Time: %v", avgResponseTime)
			t.Logf("   Test Duration: %v", actualDuration)
			
			assert.Greater(t, actualThroughput, float64(load.requestsPerSecond)*0.8, "Should achieve >80% of target throughput")
			assert.Greater(t, successRate, 0.95, "Success rate should be >95%")
			assert.Less(t, avgResponseTime, 2*time.Second, "Average response time should be <2s")
		})
	}
	
	t.Log("✅ Throughput scalability test completed successfully")
}

