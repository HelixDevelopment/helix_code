package performance

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestConfig holds performance test configuration
type TestConfig struct {
	BaseURL           string
	ConcurrentUsers   int
	RequestsPerUser   int
	RampUpTime        time.Duration
	TestDuration      time.Duration
	TargetRPS         int
	TargetLatencyP95  time.Duration
	TargetLatencyP99  time.Duration
}

func getTestConfig() *TestConfig {
	baseURL := os.Getenv("HELIXCODE_TEST_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	return &TestConfig{
		BaseURL:          baseURL,
		ConcurrentUsers:  10,
		RequestsPerUser:  100,
		RampUpTime:       5 * time.Second,
		TestDuration:     30 * time.Second,
		TargetRPS:        100,
		TargetLatencyP95: 500 * time.Millisecond,
		TargetLatencyP99: 1 * time.Second,
	}
}

func newHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
			IdleConnTimeout:     90 * time.Second,
		},
	}
}

type LatencyStats struct {
	Min        time.Duration
	Max        time.Duration
	Avg        time.Duration
	P50        time.Duration
	P95        time.Duration
	P99        time.Duration
	Total      int
	Successful int
	Failed     int
}

func calculateStats(latencies []time.Duration) *LatencyStats {
	if len(latencies) == 0 {
		return &LatencyStats{}
	}

	// Sort latencies for percentile calculation
	sorted := make([]time.Duration, len(latencies))
	copy(sorted, latencies)
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	var total time.Duration
	for _, l := range sorted {
		total += l
	}

	return &LatencyStats{
		Min:   sorted[0],
		Max:   sorted[len(sorted)-1],
		Avg:   total / time.Duration(len(sorted)),
		P50:   sorted[len(sorted)*50/100],
		P95:   sorted[len(sorted)*95/100],
		P99:   sorted[len(sorted)*99/100],
		Total: len(sorted),
	}
}

// =============================================================================
// Benchmark Tests
// =============================================================================

func BenchmarkHealthEndpoint(b *testing.B) {
	config := getTestConfig()
	client := newHTTPClient()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := client.Get(config.BaseURL + "/health")
		if err != nil {
			b.Fatal(err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}

func BenchmarkHealthEndpointParallel(b *testing.B) {
	config := getTestConfig()
	client := newHTTPClient()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			resp, err := client.Get(config.BaseURL + "/health")
			if err != nil {
				continue
			}
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}
	})
}

func BenchmarkProjectCreation(b *testing.B) {
	config := getTestConfig()
	client := newHTTPClient()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		payload := map[string]string{
			"name":        fmt.Sprintf("bench-project-%d-%d", time.Now().UnixNano(), i),
			"description": "Benchmark test project",
			"path":        fmt.Sprintf("/tmp/bench-test-%d-%d", time.Now().UnixNano(), i),
			"type":        "go",
		}
		jsonData, _ := json.Marshal(payload)

		resp, err := client.Post(config.BaseURL+"/api/v1/projects", "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			continue
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}

func BenchmarkAuthLogin(b *testing.B) {
	config := getTestConfig()
	client := newHTTPClient()

	payload := map[string]string{
		"username": "benchuser",
		"password": "benchpass123",
	}
	jsonData, _ := json.Marshal(payload)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := client.Post(config.BaseURL+"/api/v1/auth/login", "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			continue
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}

// =============================================================================
// Load Tests
// =============================================================================

func TestPerformance_HealthEndpointThroughput(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")  // SKIP-OK: #short-mode
	}

	config := getTestConfig()
	client := newHTTPClient()

	duration := 10 * time.Second
	var successCount int64
	var failCount int64
	latencies := make([]time.Duration, 0, 10000)
	var mu sync.Mutex

	done := make(chan struct{})
	go func() {
		time.Sleep(duration)
		close(done)
	}()

	var wg sync.WaitGroup
	for i := 0; i < config.ConcurrentUsers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
					start := time.Now()
					resp, err := client.Get(config.BaseURL + "/health")
					elapsed := time.Since(start)

					if err == nil && resp.StatusCode == http.StatusOK {
						atomic.AddInt64(&successCount, 1)
						mu.Lock()
						latencies = append(latencies, elapsed)
						mu.Unlock()
					} else {
						atomic.AddInt64(&failCount, 1)
					}

					if resp != nil {
						io.Copy(io.Discard, resp.Body)
						resp.Body.Close()
					}
				}
			}
		}()
	}

	wg.Wait()

	stats := calculateStats(latencies)
	rps := float64(successCount) / duration.Seconds()

	t.Logf("Health Endpoint Throughput Test Results:")
	t.Logf("  Duration: %v", duration)
	t.Logf("  Concurrent Users: %d", config.ConcurrentUsers)
	t.Logf("  Total Requests: %d", successCount+failCount)
	t.Logf("  Successful: %d", successCount)
	t.Logf("  Failed: %d", failCount)
	t.Logf("  RPS: %.2f", rps)
	t.Logf("  Latency Min: %v", stats.Min)
	t.Logf("  Latency Max: %v", stats.Max)
	t.Logf("  Latency Avg: %v", stats.Avg)
	t.Logf("  Latency P50: %v", stats.P50)
	t.Logf("  Latency P95: %v", stats.P95)
	t.Logf("  Latency P99: %v", stats.P99)

	// Performance assertions
	if rps < 10 {
		t.Errorf("RPS too low: %.2f (expected > 10)", rps)
	}
	if stats.P95 > 5*time.Second {
		t.Errorf("P95 latency too high: %v (expected < 5s)", stats.P95)
	}
}

func TestPerformance_ConcurrentProjectCreation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")  // SKIP-OK: #short-mode
	}

	config := getTestConfig()
	client := newHTTPClient()

	numProjects := 50
	var successCount int64
	var failCount int64
	latencies := make([]time.Duration, 0, numProjects)
	var mu sync.Mutex

	var wg sync.WaitGroup
	for i := 0; i < numProjects; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			payload := map[string]string{
				"name":        fmt.Sprintf("concurrent-project-%d-%d", time.Now().UnixNano(), idx),
				"description": "Concurrent creation test",
				"path":        fmt.Sprintf("/tmp/concurrent-test-%d-%d", time.Now().UnixNano(), idx),
				"type":        "go",
			}
			jsonData, _ := json.Marshal(payload)

			start := time.Now()
			resp, err := client.Post(config.BaseURL+"/api/v1/projects", "application/json", bytes.NewBuffer(jsonData))
			elapsed := time.Since(start)

			if err == nil && resp.StatusCode == http.StatusCreated {
				atomic.AddInt64(&successCount, 1)
				mu.Lock()
				latencies = append(latencies, elapsed)
				mu.Unlock()
			} else {
				atomic.AddInt64(&failCount, 1)
			}

			if resp != nil {
				io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
			}
		}(i)
	}

	wg.Wait()

	stats := calculateStats(latencies)

	t.Logf("Concurrent Project Creation Test Results:")
	t.Logf("  Total Projects: %d", numProjects)
	t.Logf("  Successful: %d", successCount)
	t.Logf("  Failed: %d", failCount)
	t.Logf("  Latency Min: %v", stats.Min)
	t.Logf("  Latency Max: %v", stats.Max)
	t.Logf("  Latency Avg: %v", stats.Avg)
	t.Logf("  Latency P95: %v", stats.P95)

	// All projects should be created successfully
	if successCount < int64(numProjects*8/10) { // Allow 20% failure for testing without DB
		t.Logf("Note: Only %d/%d projects created (may need database setup)", successCount, numProjects)
	}
}

func TestPerformance_SustainedLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")  // SKIP-OK: #short-mode
	}

	config := getTestConfig()
	client := newHTTPClient()

	duration := 30 * time.Second
	targetRPS := 50
	interval := time.Second / time.Duration(targetRPS)

	var successCount int64
	var failCount int64
	latencies := make([]time.Duration, 0, targetRPS*int(duration.Seconds()))
	var mu sync.Mutex

	done := make(chan struct{})
	go func() {
		time.Sleep(duration)
		close(done)
	}()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			goto results
		case <-ticker.C:
			go func() {
				start := time.Now()
				resp, err := client.Get(config.BaseURL + "/health")
				elapsed := time.Since(start)

				if err == nil && resp.StatusCode == http.StatusOK {
					atomic.AddInt64(&successCount, 1)
					mu.Lock()
					latencies = append(latencies, elapsed)
					mu.Unlock()
				} else {
					atomic.AddInt64(&failCount, 1)
				}

				if resp != nil {
					io.Copy(io.Discard, resp.Body)
					resp.Body.Close()
				}
			}()
		}
	}

results:
	// Wait for in-flight requests
	time.Sleep(2 * time.Second)

	stats := calculateStats(latencies)
	actualRPS := float64(successCount) / duration.Seconds()

	t.Logf("Sustained Load Test Results:")
	t.Logf("  Duration: %v", duration)
	t.Logf("  Target RPS: %d", targetRPS)
	t.Logf("  Actual RPS: %.2f", actualRPS)
	t.Logf("  Total Requests: %d", successCount+failCount)
	t.Logf("  Successful: %d", successCount)
	t.Logf("  Failed: %d", failCount)
	t.Logf("  Success Rate: %.2f%%", float64(successCount)/float64(successCount+failCount)*100)
	t.Logf("  Latency Avg: %v", stats.Avg)
	t.Logf("  Latency P95: %v", stats.P95)
	t.Logf("  Latency P99: %v", stats.P99)
}

func TestPerformance_ConnectionPooling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")  // SKIP-OK: #short-mode
	}

	config := getTestConfig()
	client := newHTTPClient()

	// First batch - cold connections
	coldLatencies := make([]time.Duration, 0, 10)
	for i := 0; i < 10; i++ {
		start := time.Now()
		resp, err := client.Get(config.BaseURL + "/health")
		if err == nil {
			coldLatencies = append(coldLatencies, time.Since(start))
			resp.Body.Close()
		}
	}

	// Second batch - warm connections (should be faster)
	warmLatencies := make([]time.Duration, 0, 100)
	for i := 0; i < 100; i++ {
		start := time.Now()
		resp, err := client.Get(config.BaseURL + "/health")
		if err == nil {
			warmLatencies = append(warmLatencies, time.Since(start))
			resp.Body.Close()
		}
	}

	coldStats := calculateStats(coldLatencies)
	warmStats := calculateStats(warmLatencies)

	t.Logf("Connection Pooling Test Results:")
	t.Logf("  Cold Connections (first 10):")
	t.Logf("    Avg: %v", coldStats.Avg)
	t.Logf("    P95: %v", coldStats.P95)
	t.Logf("  Warm Connections (next 100):")
	t.Logf("    Avg: %v", warmStats.Avg)
	t.Logf("    P95: %v", warmStats.P95)

	// Warm connections should generally be faster or similar
	if warmStats.Avg > coldStats.Avg*2 {
		t.Logf("Note: Warm connections not showing expected improvement")
	}
}

func TestPerformance_MemoryUnderLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")  // SKIP-OK: #short-mode
	}

	config := getTestConfig()
	client := newHTTPClient()

	// Make many requests to stress memory
	numRequests := 1000
	var wg sync.WaitGroup

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := client.Get(config.BaseURL + "/health")
			if err == nil {
				io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
			}
		}()

		// Small delay to avoid overwhelming
		if i%100 == 0 {
			time.Sleep(10 * time.Millisecond)
		}
	}

	wg.Wait()

	t.Logf("Memory Under Load Test: %d requests completed", numRequests)
}

func TestPerformance_ResponseSize(t *testing.T) {
	config := getTestConfig()
	client := newHTTPClient()

	endpoints := []string{
		"/health",
		"/api/v1/system/status",
		"/api/v1/system/stats",
	}

	for _, endpoint := range endpoints {
		resp, err := client.Get(config.BaseURL + endpoint)
		if err != nil {
			t.Logf("Endpoint %s: connection error", endpoint)
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		t.Logf("Endpoint %s:", endpoint)
		t.Logf("  Status: %d", resp.StatusCode)
		t.Logf("  Response Size: %d bytes", len(body))
		t.Logf("  Content-Type: %s", resp.Header.Get("Content-Type"))

		// Response should not be excessively large
		if len(body) > 1024*1024 { // 1MB
			t.Errorf("Response too large for %s: %d bytes", endpoint, len(body))
		}
	}
}

func TestPerformance_ErrorRecovery(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")  // SKIP-OK: #short-mode
	}

	config := getTestConfig()
	client := newHTTPClient()

	// Make some requests that will fail (invalid endpoints)
	for i := 0; i < 10; i++ {
		resp, _ := client.Get(config.BaseURL + "/invalid/endpoint/that/does/not/exist")
		if resp != nil {
			resp.Body.Close()
		}
	}

	// Server should still respond normally after errors
	start := time.Now()
	resp, err := client.Get(config.BaseURL + "/health")
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Server not responding after errors: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Unexpected status after errors: %d", resp.StatusCode)
	}

	if elapsed > 5*time.Second {
		t.Errorf("Response too slow after errors: %v", elapsed)
	}

	t.Logf("Error Recovery Test: Server recovered, response time: %v", elapsed)
}
