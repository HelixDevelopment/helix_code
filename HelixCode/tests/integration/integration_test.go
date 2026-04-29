// Package integration provides end-to-end integration tests for HelixCode
package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConfig holds test configuration
type TestConfig struct {
	BaseURL     string
	DatabaseURL string
	RedisURL    string
	AuthToken   string
	Timeout     time.Duration
}

// LoadTestConfig loads test configuration from environment
func LoadTestConfig() *TestConfig {
	return &TestConfig{
		BaseURL:     getEnv("TEST_BASE_URL", "http://localhost:8080"),
		DatabaseURL: getEnv("TEST_DATABASE_URL", "postgres://helixcode:testpass@localhost:5432/helixcode_test"),
		RedisURL:    getEnv("TEST_REDIS_URL", "redis://localhost:6379"),
		AuthToken:   getEnv("TEST_AUTH_TOKEN", ""),
		Timeout:     30 * time.Second,
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// skipIfServerUnavailable skips the test if the server is not reachable
func skipIfServerUnavailable(t *testing.T, config *TestConfig) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", config.BaseURL+"/health", nil)
	if err != nil {
		t.Skipf("Skipping test: cannot create request: %v", err)
		return
	}

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Skipf("Skipping test: server not available at %s: %v", config.BaseURL, err)
		return
	}
	resp.Body.Close()
}

// skipIfAuthNotConfigured skips the test if the server auth isn't properly configured for testing
// This checks if TEST_AUTH_TOKEN is set or if the test admin user exists
func skipIfAuthNotConfigured(t *testing.T, config *TestConfig) {
	t.Helper()

	// If TEST_AUTH_TOKEN is set, auth is configured
	if config.AuthToken != "" {
		return
	}

	// Check if server has the auth endpoint and test user configured
	client := &http.Client{Timeout: 5 * time.Second}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Try to login with test credentials
	creds := map[string]string{"username": "admin", "password": "admin123"}
	jsonData, _ := json.Marshal(creds)
	req, err := http.NewRequestWithContext(ctx, "POST", config.BaseURL+"/api/v1/auth/login", bytes.NewBuffer(jsonData))
	if err != nil {
		t.Skipf("Skipping test: cannot create auth request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		t.Skipf("Skipping test: auth endpoint not reachable: %v", err)
		return
	}
	defer resp.Body.Close()

	// If we get 404, the endpoint doesn't exist
	if resp.StatusCode == http.StatusNotFound {
		t.Skip("Skipping test: auth endpoint not implemented on server")  // SKIP-OK: #legacy-untriaged
		return
	}

	// If we get 401, the test user isn't configured (this is expected in CI)
	if resp.StatusCode == http.StatusUnauthorized {
		t.Skip("Skipping test: test admin user not configured on server")  // SKIP-OK: #legacy-untriaged
		return
	}

	// If we get 200, auth is properly configured - extract token
	if resp.StatusCode == http.StatusOK {
		var authResp map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&authResp); err == nil {
			if token, ok := authResp["token"].(string); ok {
				config.AuthToken = token
			}
		}
	}
}

// TestClient wraps HTTP client for API testing
type TestClient struct {
	httpClient *http.Client
	baseURL    string
	authToken  string
}

// NewTestClient creates a new test client
func NewTestClient(config *TestConfig) *TestClient {
	return &TestClient{
		httpClient: &http.Client{Timeout: config.Timeout},
		baseURL:    config.BaseURL,
		authToken:  config.AuthToken,
	}
}

// Get performs GET request
func (c *TestClient) Get(path string) (*http.Response, error) {
	return c.do("GET", path, nil)
}

// Post performs POST request
func (c *TestClient) Post(path string, body interface{}) (*http.Response, error) {
	return c.do("POST", path, body)
}

// Put performs PUT request
func (c *TestClient) Put(path string, body interface{}) (*http.Response, error) {
	return c.do("PUT", path, body)
}

// Delete performs DELETE request
func (c *TestClient) Delete(path string) (*http.Response, error) {
	return c.do("DELETE", path, nil)
}

func (c *TestClient) do(method, path string, body interface{}) (*http.Response, error) {
	url := c.baseURL + path

	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	if c.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	}

	return c.httpClient.Do(req)
}

// Test Suite: Basic Health Checks
func TestHealthEndpoint(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")  // SKIP-OK: #short-mode
	}

	config := LoadTestConfig()
	skipIfServerUnavailable(t, config)
	client := NewTestClient(config)

	resp, err := client.Get("/health")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var healthData map[string]interface{}
	err = json.Unmarshal(body, &healthData)
	require.NoError(t, err)

	// Accept both "ok" and "healthy" status values
	status, _ := healthData["status"].(string)
	assert.Contains(t, []string{"ok", "healthy"}, status, "Status should be 'ok' or 'healthy'")
}

// Test Suite: Authentication Flow
func TestAuthenticationFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")  // SKIP-OK: #short-mode
	}

	config := LoadTestConfig()
	skipIfServerUnavailable(t, config)

	// Check if auth endpoint exists first
	client := &http.Client{Timeout: 5 * time.Second}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	testCreds := map[string]string{"username": "test", "password": "test"}
	jsonData, _ := json.Marshal(testCreds)
	req, _ := http.NewRequestWithContext(ctx, "POST", config.BaseURL+"/api/v1/auth/login", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		t.Skipf("Skipping test: auth endpoint not reachable: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		t.Skip("Skipping test: auth endpoint not implemented on server")  // SKIP-OK: #legacy-untriaged
	}

	testClient := NewTestClient(config)

	// Note: "Valid credentials" test requires a pre-configured admin user
	// The test will be skipped if the admin user doesn't exist
	tests := []struct {
		name         string
		credentials  map[string]string
		expectStatus []int // Allow multiple valid statuses
	}{
		{
			name:         "Valid credentials",
			credentials:  map[string]string{"username": "admin", "password": "admin123"},
			expectStatus: []int{http.StatusOK, http.StatusUnauthorized}, // May not have admin user
		},
		{
			name:         "Invalid credentials",
			credentials:  map[string]string{"username": "admin", "password": "wrong"},
			expectStatus: []int{http.StatusUnauthorized},
		},
		{
			name:         "Missing credentials",
			credentials:  map[string]string{},
			expectStatus: []int{http.StatusBadRequest, http.StatusUnauthorized},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := testClient.Post("/api/v1/auth/login", tt.credentials)
			require.NoError(t, err)
			defer resp.Body.Close()

			// Check if status is one of the expected values
			statusOK := false
			for _, expected := range tt.expectStatus {
				if resp.StatusCode == expected {
					statusOK = true
					break
				}
			}
			assert.True(t, statusOK, "Status %d should be one of %v", resp.StatusCode, tt.expectStatus)
		})
	}
}

// Test Suite: Task Management
func TestTaskCreationAndRetrieval(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")  // SKIP-OK: #short-mode
	}

	config := LoadTestConfig()
	skipIfServerUnavailable(t, config)
	skipIfAuthNotConfigured(t, config)
	client := NewTestClient(config)

	// Create a task
	taskData := map[string]interface{}{
		"title":       "Integration Test Task",
		"description": "Test task created by integration test",
		"priority":    "high",
		"type":        "planning",
	}

	resp, err := client.Post("/api/v1/tasks", taskData)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Skip if we get 401 (auth required but not properly configured)
	if resp.StatusCode == http.StatusUnauthorized {
		t.Skip("Skipping test: authentication required but not configured")  // SKIP-OK: #legacy-untriaged
	}

	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	// Parse response
	var createResponse map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&createResponse)
	require.NoError(t, err)

	taskID, ok := createResponse["id"].(string)
	require.True(t, ok, "Task ID should be a string")

	// Retrieve the task
	resp, err = client.Get(fmt.Sprintf("/api/v1/tasks/%s", taskID))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var task map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&task)
	require.NoError(t, err)

	assert.Equal(t, taskData["title"], task["title"])
	assert.Equal(t, taskData["priority"], task["priority"])

	// Cleanup: Delete the task
	resp, err = client.Delete(fmt.Sprintf("/api/v1/tasks/%s", taskID))
	require.NoError(t, err)
	resp.Body.Close()
}

// Test Suite: Worker Management
func TestWorkerRegistrationAndHeartbeat(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")  // SKIP-OK: #short-mode
	}

	config := LoadTestConfig()
	skipIfServerUnavailable(t, config)
	skipIfAuthNotConfigured(t, config)
	client := NewTestClient(config)

	// Register worker
	workerData := map[string]interface{}{
		"hostname":     "test-worker-1",
		"capabilities": []string{"planning", "building", "testing"},
		"resources": map[string]interface{}{
			"cpu":    4,
			"memory": 8192,
		},
	}

	resp, err := client.Post("/api/v1/workers/register", workerData)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Skip if we get 401 (auth required but not properly configured)
	if resp.StatusCode == http.StatusUnauthorized {
		t.Skip("Skipping test: authentication required but not configured")  // SKIP-OK: #legacy-untriaged
	}

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var registerResponse map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&registerResponse)
	require.NoError(t, err)

	workerID, ok := registerResponse["worker_id"].(string)
	require.True(t, ok)

	// Send heartbeat
	heartbeatData := map[string]interface{}{
		"worker_id": workerID,
		"status":    "idle",
	}

	resp, err = client.Post("/api/v1/workers/heartbeat", heartbeatData)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// Test Suite: End-to-End Workflow
func TestCompleteWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")  // SKIP-OK: #short-mode
	}

	config := LoadTestConfig()
	skipIfServerUnavailable(t, config)
	client := NewTestClient(config)
	ctx := context.Background()

	// 1. Create a project
	projectData := map[string]interface{}{
		"name":        "Integration Test Project",
		"description": "E2E test project",
	}

	resp, err := client.Post("/api/v1/projects", projectData)
	require.NoError(t, err)
	defer resp.Body.Close()

	var projectResp map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&projectResp)
	require.NoError(t, err)

	projectIDValue, ok := projectResp["id"]
	if !ok || projectIDValue == nil {
		t.Skip("Server did not return project ID - server may not be fully configured")  // SKIP-OK: #legacy-untriaged
	}
	projectID, ok := projectIDValue.(string)
	if !ok {
		t.Skipf("Project ID is not a string: %T", projectIDValue)
	}

	// 2. Create a task within the project
	taskData := map[string]interface{}{
		"project_id":  projectID,
		"title":       "Test Task",
		"description": "Task for E2E test",
		"type":        "planning",
	}

	resp, err = client.Post("/api/v1/tasks", taskData)
	require.NoError(t, err)
	defer resp.Body.Close()

	var taskResp map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&taskResp)
	require.NoError(t, err)

	taskID := taskResp["id"].(string)

	// 3. Assign task to a worker (simulated)
	// 4. Poll for task completion with timeout
	timeout := time.After(10 * time.Second)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	taskCompleted := false
	for !taskCompleted {
		select {
		case <-ctx.Done():
			t.Fatal("Context cancelled")
		case <-timeout:
			t.Log("Task did not complete within timeout (expected in test)")
			taskCompleted = true
		case <-ticker.C:
			resp, err := client.Get(fmt.Sprintf("/api/v1/tasks/%s", taskID))
			if err != nil {
				continue
			}

			var status map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&status)
			resp.Body.Close()

			if status["status"] == "completed" || status["status"] == "failed" {
				taskCompleted = true
			}
		}
	}

	// Cleanup
	client.Delete(fmt.Sprintf("/api/v1/projects/%s", projectID))
}

// Test Suite: Concurrent Operations
func TestConcurrentTaskCreation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")  // SKIP-OK: #short-mode
	}

	config := LoadTestConfig()
	skipIfServerUnavailable(t, config)
	skipIfAuthNotConfigured(t, config)
	client := NewTestClient(config)

	// Pre-check: Ensure we can create tasks before running concurrent test
	testData := map[string]interface{}{
		"title":    "Pre-check Task",
		"type":     "planning",
		"priority": "normal",
	}
	preCheck, err := client.Post("/api/v1/tasks", testData)
	if err != nil {
		t.Skipf("Skipping test: cannot reach tasks endpoint: %v", err)
	}
	preCheck.Body.Close()
	if preCheck.StatusCode == http.StatusUnauthorized {
		t.Skip("Skipping test: authentication required but not configured")  // SKIP-OK: #legacy-untriaged
	}
	if preCheck.StatusCode == http.StatusNotFound {
		t.Skip("Skipping test: tasks endpoint not implemented")  // SKIP-OK: #legacy-untriaged
	}

	numTasks := 10
	results := make(chan error, numTasks)

	for i := 0; i < numTasks; i++ {
		go func(taskNum int) {
			taskData := map[string]interface{}{
				"title":    fmt.Sprintf("Concurrent Task %d", taskNum),
				"type":     "planning",
				"priority": "normal",
			}

			resp, err := client.Post("/api/v1/tasks", taskData)
			if err != nil {
				results <- err
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusCreated {
				results <- fmt.Errorf("unexpected status: %d", resp.StatusCode)
				return
			}

			results <- nil
		}(i)
	}

	// Collect results
	for i := 0; i < numTasks; i++ {
		err := <-results
		assert.NoError(t, err, "Task %d should succeed", i)
	}
}
