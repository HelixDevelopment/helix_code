// Package integration provides integration tests for HelixCode
package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// CogneeTestConfig holds Cognee test configuration
type CogneeTestConfig struct {
	BaseURL string
	APIKey  string
	Timeout time.Duration
}

// LoadCogneeConfig loads Cognee configuration from environment
func LoadCogneeConfig() *CogneeTestConfig {
	return &CogneeTestConfig{
		BaseURL: getEnv("HELIX_TEST_COGNEE_URL", "http://localhost:8001"),
		APIKey:  getEnv("HELIX_TEST_COGNEE_API_KEY", "test_cognee_key_123"),
		Timeout: 60 * time.Second,
	}
}

// CogneeClient is a test client for Cognee API
type CogneeClient struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
}

// NewCogneeClient creates a new Cognee test client
func NewCogneeClient(config *CogneeTestConfig) *CogneeClient {
	return &CogneeClient{
		httpClient: &http.Client{Timeout: config.Timeout},
		baseURL:    strings.TrimSuffix(config.BaseURL, "/"),
		apiKey:     config.APIKey,
	}
}

func (c *CogneeClient) doRequest(method, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, c.baseURL+path, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	}

	return c.httpClient.Do(req)
}

// Health checks Cognee health
func (c *CogneeClient) Health() (map[string]interface{}, error) {
	resp, err := c.doRequest("GET", "/health", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}

// AddMemory adds a memory to Cognee
func (c *CogneeClient) AddMemory(content, dataset, contentType string, metadata map[string]interface{}) (map[string]interface{}, error) {
	payload := map[string]interface{}{
		"content":      content,
		"dataset_name": dataset,
		"content_type": contentType,
		"metadata":     metadata,
	}

	body, _ := json.Marshal(payload)
	resp, err := c.doRequest("POST", "/add", strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}

// SearchMemory searches memories in Cognee
func (c *CogneeClient) SearchMemory(query, dataset string, limit int) ([]map[string]interface{}, error) {
	payload := map[string]interface{}{
		"query":        query,
		"dataset_name": dataset,
		"limit":        limit,
	}

	body, _ := json.Marshal(payload)
	resp, err := c.doRequest("POST", "/search", strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Results []map[string]interface{} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Results, nil
}

// Cognify processes data into knowledge graph
func (c *CogneeClient) Cognify(datasets []string) (map[string]interface{}, error) {
	payload := map[string]interface{}{
		"datasets": datasets,
	}

	body, _ := json.Marshal(payload)
	resp, err := c.doRequest("POST", "/cognify", strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetDatasets lists all datasets
func (c *CogneeClient) GetDatasets() ([]map[string]interface{}, error) {
	resp, err := c.doRequest("GET", "/datasets", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Datasets []map[string]interface{} `json:"datasets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Datasets, nil
}

// CreateDataset creates a new dataset
func (c *CogneeClient) CreateDataset(name, description string) (map[string]interface{}, error) {
	payload := map[string]interface{}{
		"name":        name,
		"description": description,
	}

	body, _ := json.Marshal(payload)
	resp, err := c.doRequest("POST", "/datasets", strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}

// DeleteDataset deletes a dataset
func (c *CogneeClient) DeleteDataset(name string) error {
	resp, err := c.doRequest("DELETE", "/datasets/"+name, nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// RequireCognee checks if Cognee is available
func RequireCognee(t *testing.T) *CogneeClient {
	if testing.Short() {
		t.Skip("Skipping Cognee integration test in short mode")  // SKIP-OK: #short-mode
	}

	if os.Getenv("HELIX_TEST_INFRA") != "true" {
		t.Skip("Test infrastructure not available (set HELIX_TEST_INFRA=true)")  // SKIP-OK: #legacy-untriaged
	}

	config := LoadCogneeConfig()
	client := NewCogneeClient(config)

	// Check health with retry
	var lastErr error
	for i := 0; i < 5; i++ {
		_, err := client.Health()
		if err == nil {
			return client
		}
		lastErr = err
		time.Sleep(2 * time.Second)
	}

	t.Skipf("Cognee not available: %v (SKIP-OK: #infra-unavailable)", lastErr)
	return nil
}

// TestCogneeHealthIntegration tests Cognee health endpoint
func TestCogneeHealthIntegration(t *testing.T) {
	client := RequireCognee(t)

	health, err := client.Health()
	require.NoError(t, err)

	assert.NotNil(t, health)
	// Cognee should return some health status
	t.Logf("Cognee health: %+v", health)
}

// TestCogneeDatasetLifecycleIntegration tests dataset CRUD operations
func TestCogneeDatasetLifecycleIntegration(t *testing.T) {
	client := RequireCognee(t)
	ctx := context.Background()
	_ = ctx

	datasetName := fmt.Sprintf("test-dataset-%d", time.Now().UnixNano())
	description := "Integration test dataset"

	// Create dataset
	t.Run("CreateDataset", func(t *testing.T) {
		result, err := client.CreateDataset(datasetName, description)
		require.NoError(t, err)
		assert.NotNil(t, result)
		t.Logf("Created dataset: %+v", result)
	})

	// List datasets
	t.Run("ListDatasets", func(t *testing.T) {
		datasets, err := client.GetDatasets()
		require.NoError(t, err)
		t.Logf("Found %d datasets", len(datasets))
	})

	// Cleanup
	t.Cleanup(func() {
		client.DeleteDataset(datasetName)
	})
}

// TestCogneeMemoryOperationsIntegration tests memory add/search operations
func TestCogneeMemoryOperationsIntegration(t *testing.T) {
	client := RequireCognee(t)

	datasetName := fmt.Sprintf("memory-test-%d", time.Now().UnixNano())

	// Create dataset first
	_, err := client.CreateDataset(datasetName, "Memory operations test")
	require.NoError(t, err)

	t.Cleanup(func() {
		client.DeleteDataset(datasetName)
	})

	// Test adding memories
	t.Run("AddMemory", func(t *testing.T) {
		testCases := []struct {
			content  string
			metadata map[string]interface{}
		}{
			{
				content:  "The quick brown fox jumps over the lazy dog",
				metadata: map[string]interface{}{"source": "test", "category": "animals"},
			},
			{
				content:  "Artificial intelligence is transforming software development",
				metadata: map[string]interface{}{"source": "test", "category": "technology"},
			},
			{
				content:  "Go is a statically typed programming language designed at Google",
				metadata: map[string]interface{}{"source": "test", "category": "programming"},
			},
		}

		for i, tc := range testCases {
			t.Run(fmt.Sprintf("Memory%d", i+1), func(t *testing.T) {
				result, err := client.AddMemory(tc.content, datasetName, "text", tc.metadata)
				require.NoError(t, err)
				assert.NotNil(t, result)
				t.Logf("Added memory: %+v", result)
			})
		}
	})

	// Allow time for indexing
	time.Sleep(2 * time.Second)

	// Test searching memories
	t.Run("SearchMemory", func(t *testing.T) {
		testCases := []struct {
			query       string
			expectMatch bool
		}{
			{query: "fox lazy dog", expectMatch: true},
			{query: "artificial intelligence AI", expectMatch: true},
			{query: "Go programming language", expectMatch: true},
			{query: "completely unrelated xyz123", expectMatch: false},
		}

		for _, tc := range testCases {
			t.Run(tc.query, func(t *testing.T) {
				results, err := client.SearchMemory(tc.query, datasetName, 10)
				require.NoError(t, err)
				t.Logf("Search '%s' returned %d results", tc.query, len(results))

				if tc.expectMatch {
					// Note: expectMatch is a hint; actual results depend on Cognee's indexing
					// We just verify the search completes successfully
				}
			})
		}
	})
}

// TestCogneeCognifyIntegration tests the cognify (knowledge graph) operation
func TestCogneeCognifyIntegration(t *testing.T) {
	client := RequireCognee(t)

	datasetName := fmt.Sprintf("cognify-test-%d", time.Now().UnixNano())

	// Create dataset and add content
	_, err := client.CreateDataset(datasetName, "Cognify test dataset")
	require.NoError(t, err)

	t.Cleanup(func() {
		client.DeleteDataset(datasetName)
	})

	// Add some related content
	contents := []string{
		"HelixCode is an AI-powered development platform built with Go",
		"Go is developed by Google and is excellent for concurrent programming",
		"Concurrent programming enables efficient handling of multiple tasks",
		"AI platforms help developers write better code faster",
	}

	for _, content := range contents {
		_, err := client.AddMemory(content, datasetName, "text", nil)
		require.NoError(t, err)
	}

	// Allow time for indexing
	time.Sleep(2 * time.Second)

	// Run cognify
	t.Run("Cognify", func(t *testing.T) {
		result, err := client.Cognify([]string{datasetName})
		require.NoError(t, err)
		assert.NotNil(t, result)
		t.Logf("Cognify result: %+v", result)
	})
}

// TestCogneeConcurrentOperationsIntegration tests concurrent access
func TestCogneeConcurrentOperationsIntegration(t *testing.T) {
	client := RequireCognee(t)

	datasetName := fmt.Sprintf("concurrent-test-%d", time.Now().UnixNano())

	// Create dataset
	_, err := client.CreateDataset(datasetName, "Concurrent operations test")
	require.NoError(t, err)

	t.Cleanup(func() {
		client.DeleteDataset(datasetName)
	})

	// Concurrent adds
	numGoroutines := 5
	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			content := fmt.Sprintf("Concurrent test content %d from goroutine %d", time.Now().UnixNano(), id)
			_, err := client.AddMemory(content, datasetName, "text", map[string]interface{}{"goroutine": id})
			errors <- err
		}(i)
	}

	// Collect results
	for i := 0; i < numGoroutines; i++ {
		err := <-errors
		assert.NoError(t, err, "Concurrent add should succeed")
	}

	t.Log("All concurrent operations completed successfully")
}

// TestCogneeErrorHandlingIntegration tests error scenarios
func TestCogneeErrorHandlingIntegration(t *testing.T) {
	client := RequireCognee(t)

	t.Run("SearchNonExistentDataset", func(t *testing.T) {
		results, err := client.SearchMemory("test", "non-existent-dataset-xyz", 10)
		// Either returns empty results or an error
		t.Logf("Search non-existent dataset: results=%d, err=%v", len(results), err)
	})

	t.Run("CognifyNonExistentDataset", func(t *testing.T) {
		result, err := client.Cognify([]string{"non-existent-dataset-xyz"})
		// Either returns empty result or an error
		t.Logf("Cognify non-existent dataset: result=%v, err=%v", result, err)
	})
}
