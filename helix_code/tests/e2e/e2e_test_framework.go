package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// E2ETestFramework provides infrastructure for end-to-end testing
type E2ETestFramework struct {
	Server     *httptest.Server
	HTTPClient *http.Client
	BaseURL    string
	TestUser   *TestUser
}

// TestUser represents a test user for E2E tests
type TestUser struct {
	ID       string
	Username string
	Email    string
	Token    string
}

// NewE2ETestFramework creates a new E2E test framework
func NewE2ETestFramework(t *testing.T) *E2ETestFramework {
	// Create HTTP client with timeout
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	// For now, we'll use a mock server approach
	// In a real implementation, this would start the actual HelixCode server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Mock server responses for E2E testing
		switch r.URL.Path {
		case "/health":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{"status": "healthy"})
		case "/api/v1/auth/register":
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"user_id": "test-user-123",
				"message": "User registered successfully",
			})
		case "/api/v1/auth/login":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"token":      "mock-jwt-token-for-testing",
				"expires_in": 3600,
			})
		case "/api/v1/auth/profile":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"username": "testuser",
				"email":    "test@example.com",
			})
		case "/api/v1/auth/me":
			if r.Header.Get("Authorization") == "" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"username": "testuser",
				"email":    "test@example.com",
			})
		case "/api/v1/auth/logout":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{"message": "Logged out successfully"})
		case "/api/v1/admin/users":
			if r.Header.Get("Authorization") == "" {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode([]map[string]interface{}{{"username": "admin"}})
		case "/api/v1/projects":
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"project_id": "test-project-123",
				"name":       "Test Project",
				"message":    "Project created successfully",
			})
		case "/api/v1/projects/test-project-123":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"project_id": "test-project-123",
				"name":       "Test Project",
			})
		case "/api/v1/projects/test-project-123/files":
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"file_id": "test-file-123",
				"path":    "test_file.txt",
			})
		case "/api/v1/projects/test-project-123/files/test_file.txt":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"path":    "test_file.txt",
				"content": "This is a test file for E2E testing",
			})
		default:
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": "Not found"})
		}
	}))

	framework := &E2ETestFramework{
		Server:     server,
		HTTPClient: httpClient,
		BaseURL:    server.URL,
	}

	// Setup test user
	framework.setupTestUser(t)

	return framework
}

// Cleanup cleans up test resources
func (f *E2ETestFramework) Cleanup(t *testing.T) {
	if f.Server != nil {
		f.Server.Close()
	}
}

// setupTestUser creates a test user for authentication tests
func (f *E2ETestFramework) setupTestUser(t *testing.T) {
	// For now, we'll create a mock test user
	// In real implementation, this would register a user via API
	f.TestUser = &TestUser{
		ID:       "test-user-123",
		Username: "testuser",
		Email:    "test@example.com",
		Token:    "mock-jwt-token-for-testing",
	}
}

// WaitForServer waits for server to be ready
func WaitForServer(t *testing.T, framework *E2ETestFramework, timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			t.Fatal("Timeout waiting for server to be ready")
		default:
			resp, err := framework.GET(t, "/health")
			if err == nil && resp.StatusCode == http.StatusOK {
				resp.Body.Close()
				return
			}
			if resp != nil {
				resp.Body.Close()
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// HTTP Request Helpers

// GET performs a GET request
func (f *E2ETestFramework) GET(t *testing.T, path string) (*http.Response, error) {
	url := f.BaseURL + path
	req, err := http.NewRequest("GET", url, nil)
	require.NoError(t, err)

	if f.TestUser != nil && f.TestUser.Token != "" {
		req.Header.Set("Authorization", "Bearer "+f.TestUser.Token)
	}

	return f.HTTPClient.Do(req)
}

// POST performs a POST request with JSON body
func (f *E2ETestFramework) POST(t *testing.T, path string, body interface{}) (*http.Response, error) {
	url := f.BaseURL + path

	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		require.NoError(t, err)
		bodyReader = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequest("POST", url, bodyReader)
	require.NoError(t, err)

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if f.TestUser != nil && f.TestUser.Token != "" {
		req.Header.Set("Authorization", "Bearer "+f.TestUser.Token)
	}

	return f.HTTPClient.Do(req)
}

// PUT performs a PUT request with JSON body
func (f *E2ETestFramework) PUT(t *testing.T, path string, body interface{}) (*http.Response, error) {
	url := f.BaseURL + path

	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		require.NoError(t, err)
		bodyReader = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequest("PUT", url, bodyReader)
	require.NoError(t, err)

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if f.TestUser != nil && f.TestUser.Token != "" {
		req.Header.Set("Authorization", "Bearer "+f.TestUser.Token)
	}

	return f.HTTPClient.Do(req)
}

// DELETE performs a DELETE request
func (f *E2ETestFramework) DELETE(t *testing.T, path string) (*http.Response, error) {
	url := f.BaseURL + path
	req, err := http.NewRequest("DELETE", url, nil)
	require.NoError(t, err)

	if f.TestUser != nil && f.TestUser.Token != "" {
		req.Header.Set("Authorization", "Bearer "+f.TestUser.Token)
	}

	return f.HTTPClient.Do(req)
}

// Response Helpers

// ParseJSON parses JSON response
func ParseJSON(t *testing.T, resp *http.Response, target interface{}) {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	err = json.Unmarshal(body, target)
	require.NoError(t, err)
}

// AssertStatus asserts HTTP status code
func AssertStatus(t *testing.T, resp *http.Response, expectedStatus int) {
	assert.Equal(t, expectedStatus, resp.StatusCode,
		"Expected status %d, got %d", expectedStatus, resp.StatusCode)
}

// AssertJSONResponse asserts JSON response structure
func AssertJSONResponse(t *testing.T, resp *http.Response, expectedStatus int, target interface{}) {
	AssertStatus(t, resp, expectedStatus)

	if target != nil {
		ParseJSON(t, resp, target)
	}
}

// CleanupTestData cleans up test data
func CleanupTestData(t *testing.T, data interface{}) {
	// Clean up test users, projects, etc.
	// This would be implemented based on specific test requirements
	t.Log("Cleaning up test data...")
}

// SkipIfCI skips test if running in CI environment
func SkipIfCI(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("Skipping test in CI environment")  // SKIP-OK: #legacy-untriaged
	}
}

// SkipIfShort skips test if running with -short flag
func SkipIfShort(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping test in short mode")  // SKIP-OK: #short-mode
	}
}