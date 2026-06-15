package cognee

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"

	"dev.helix.code/internal/config"
	"dev.helix.code/internal/logging"
)

// Client represents a Cognee API client
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	logger     *logging.Logger
	mu         sync.RWMutex
	connected  bool
	lastCheck  time.Time
}

// NewClient creates a new Cognee API client
func NewClient(cfg *config.CogneeConfig) *Client {
	baseURL := fmt.Sprintf("http://%s:%d", cfg.Host, cfg.Port)
	if cfg.RemoteAPI != nil && cfg.RemoteAPI.ServiceEndpoint != "" && cfg.Mode == "cloud" {
		baseURL = cfg.RemoteAPI.ServiceEndpoint
	}

	timeout := 30 * time.Second
	if cfg.RemoteAPI != nil && cfg.RemoteAPI.Timeout > 0 {
		timeout = cfg.RemoteAPI.Timeout
	}

	apiKey := ""
	if cfg.RemoteAPI != nil {
		apiKey = cfg.RemoteAPI.APIKey
	}

	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		logger: logging.NewLoggerWithName("cognee_client"),
	}
}

// IsConnected returns whether the client is connected to Cognee
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// GetBaseURL returns the base URL of the Cognee API. The read is guarded by
// c.mu because SetBaseURL mutates baseURL under the same lock — an unguarded
// read here races a concurrent SetBaseURL.
func (c *Client) GetBaseURL() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.baseURL
}

// getBaseURL is the lock-guarded internal accessor every request builder MUST
// use to read baseURL. baseURL is mutable (SetBaseURL writes it under c.mu), so
// reading c.baseURL directly while a request is in flight is a data race.
func (c *Client) getBaseURL() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.baseURL
}

// getAPIKey is the lock-guarded internal accessor for apiKey. apiKey is mutable
// (SetAPIKey writes it under c.mu), so reading c.apiKey directly in setHeaders
// while SetAPIKey runs concurrently is a data race.
func (c *Client) getAPIKey() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.apiKey
}

// TestConnection tests the connection to Cognee
func (c *Client) TestConnection(ctx context.Context) bool {
	url := fmt.Sprintf("%s/health", c.getBaseURL())

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		c.logger.Debug("Failed to create health check request: %v", err)
		return false
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		c.logger.Debug("Health check failed: %v", err)
		c.mu.Lock()
		c.connected = false
		c.lastCheck = time.Now()
		c.mu.Unlock()
		return false
	}
	defer resp.Body.Close()

	connected := resp.StatusCode == http.StatusOK
	c.mu.Lock()
	c.connected = connected
	c.lastCheck = time.Now()
	c.mu.Unlock()

	return connected
}

// AddMemory adds a memory entry to Cognee
func (c *Client) AddMemory(ctx context.Context, req *AddMemoryRequest) (*AddMemoryResponse, error) {
	url := fmt.Sprintf("%s/api/memory", c.getBaseURL())

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("cognee API error: %s, body: %s", resp.Status, string(body))
	}

	var response AddMemoryResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response, nil
}

// SearchMemory searches for memories in Cognee
func (c *Client) SearchMemory(ctx context.Context, req *SearchMemoryRequest) (*SearchMemoryResponse, error) {
	url := fmt.Sprintf("%s/api/search", c.getBaseURL())

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(httpReq)

	startTime := time.Now()
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("cognee API error: %s, body: %s", resp.Status, string(body))
	}

	var response SearchMemoryResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	response.Duration = time.Since(startTime)
	response.Query = req.Query

	return &response, nil
}

// Cognify processes data into knowledge graphs
func (c *Client) Cognify(ctx context.Context, req *CognifyRequest) (*CognifyResponse, error) {
	url := fmt.Sprintf("%s/api/cognify", c.getBaseURL())

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("cognee API error: %s, body: %s", resp.Status, string(body))
	}

	var response CognifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	response.StartedAt = time.Now()

	return &response, nil
}

// SearchInsights performs insight-based search using graph reasoning
func (c *Client) SearchInsights(ctx context.Context, req *InsightsRequest) (*InsightsResponse, error) {
	url := fmt.Sprintf("%s/api/search", c.getBaseURL())

	insightsReq := map[string]interface{}{
		"query":       req.Query,
		"datasets":    req.Datasets,
		"limit":       req.Limit,
		"search_type": "INSIGHTS",
	}

	data, err := json.Marshal(insightsReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(httpReq)

	startTime := time.Now()
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("cognee API error: %s, body: %s", resp.Status, string(body))
	}

	var response InsightsResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	response.Query = req.Query
	response.Duration = time.Since(startTime)

	return &response, nil
}

// SearchGraphCompletion performs LLM-powered graph completion search
func (c *Client) SearchGraphCompletion(ctx context.Context, query string, datasets []string, limit int) (*SearchMemoryResponse, error) {
	url := fmt.Sprintf("%s/api/search", c.getBaseURL())

	searchReq := map[string]interface{}{
		"query":       query,
		"datasets":    datasets,
		"limit":       limit,
		"search_type": "GRAPH_COMPLETION",
	}

	data, err := json.Marshal(searchReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(httpReq)

	startTime := time.Now()
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("cognee API error: %s, body: %s", resp.Status, string(body))
	}

	var response SearchMemoryResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	response.Query = query
	response.Duration = time.Since(startTime)

	return &response, nil
}

// ProcessCodePipeline processes code through Cognee's code understanding pipeline
func (c *Client) ProcessCodePipeline(ctx context.Context, req *CodePipelineRequest) (*CodePipelineResponse, error) {
	url := fmt.Sprintf("%s/api/code-pipeline/index", c.getBaseURL())

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("cognee API error: %s, body: %s", resp.Status, string(body))
	}

	var response CodePipelineResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response, nil
}

// CreateDataset creates a new dataset
func (c *Client) CreateDataset(ctx context.Context, req *CreateDatasetRequest) (*DatasetResponse, error) {
	url := fmt.Sprintf("%s/api/datasets", c.getBaseURL())

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("cognee API error: %s, body: %s", resp.Status, string(body))
	}

	var response DatasetResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response, nil
}

// ListDatasets retrieves all datasets
func (c *Client) ListDatasets(ctx context.Context) (*DatasetsResponse, error) {
	url := fmt.Sprintf("%s/api/datasets", c.getBaseURL())

	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("cognee API error: %s, body: %s", resp.Status, string(body))
	}

	var response DatasetsResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response, nil
}

// GetDataset retrieves a specific dataset
func (c *Client) GetDataset(ctx context.Context, name string) (*Dataset, error) {
	url := fmt.Sprintf("%s/api/datasets/%s", c.getBaseURL(), name)

	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("cognee API error: %s, body: %s", resp.Status, string(body))
	}

	var dataset Dataset
	if err := json.NewDecoder(resp.Body).Decode(&dataset); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &dataset, nil
}

// DeleteDataset deletes a dataset
func (c *Client) DeleteDataset(ctx context.Context, name string) error {
	url := fmt.Sprintf("%s/api/datasets/%s", c.getBaseURL(), name)

	httpReq, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("cognee API error: %s, body: %s", resp.Status, string(body))
	}

	return nil
}

// VisualizeGraph retrieves graph visualization data
func (c *Client) VisualizeGraph(ctx context.Context, req *GraphVisualizationRequest) (*GraphVisualizationResponse, error) {
	url := fmt.Sprintf("%s/api/visualize", c.getBaseURL())

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("cognee API error: %s, body: %s", resp.Status, string(body))
	}

	var response GraphVisualizationResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response, nil
}

// DeleteData removes data from a dataset
func (c *Client) DeleteData(ctx context.Context, req *DeleteDataRequest) (*DeleteDataResponse, error) {
	url := fmt.Sprintf("%s/api/delete", c.getBaseURL())

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "DELETE", url, bytes.NewBuffer(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("cognee API error: %s, body: %s", resp.Status, string(body))
	}

	var response DeleteDataResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response, nil
}

// GetHealth retrieves health status
func (c *Client) GetHealth(ctx context.Context) (*HealthStatus, error) {
	url := fmt.Sprintf("%s/health", c.getBaseURL())

	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	startTime := time.Now()
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return &HealthStatus{
			Status:       "unhealthy",
			Timestamp:    time.Now(),
			LastCheck:    time.Now(),
			ResponseTime: time.Since(startTime),
			Components:   map[string]string{"connection": "failed"},
		}, nil
	}
	defer resp.Body.Close()

	responseTime := time.Since(startTime)

	var health HealthStatus
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		health = HealthStatus{
			Status:    "unknown",
			Timestamp: time.Now(),
		}
	}

	health.LastCheck = time.Now()
	health.ResponseTime = responseTime

	if resp.StatusCode == http.StatusOK {
		health.Status = "healthy"
		c.mu.Lock()
		c.connected = true
		c.lastCheck = time.Now()
		c.mu.Unlock()
	} else {
		health.Status = "unhealthy"
		c.mu.Lock()
		c.connected = false
		c.lastCheck = time.Now()
		c.mu.Unlock()
	}

	return &health, nil
}

// GetStatistics retrieves Cognee statistics
func (c *Client) GetStatistics(ctx context.Context) (*CogneeStatistics, error) {
	url := fmt.Sprintf("%s/api/stats", c.getBaseURL())

	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("cognee API error: %s, body: %s", resp.Status, string(body))
	}

	var stats CogneeStatistics
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	stats.LastUpdated = time.Now()

	return &stats, nil
}

// AddBatchMemory adds multiple memories in batch
func (c *Client) AddBatchMemory(ctx context.Context, req *BatchMemoryRequest) (*BatchMemoryResponse, error) {
	url := fmt.Sprintf("%s/api/memory/batch", c.getBaseURL())

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("cognee API error: %s, body: %s", resp.Status, string(body))
	}

	var response BatchMemoryResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response, nil
}

// SubmitFeedback submits feedback on search results
func (c *Client) SubmitFeedback(ctx context.Context, req *FeedbackRequest) (*FeedbackResponse, error) {
	url := fmt.Sprintf("%s/api/feedback", c.getBaseURL())

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("cognee API error: %s, body: %s", resp.Status, string(body))
	}

	var response FeedbackResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response, nil
}

// AutoContainerize starts Cognee in a container if not running
func (c *Client) AutoContainerize(ctx context.Context) error {
	if c.TestConnection(ctx) {
		c.logger.Info("Cognee is already running")
		return nil
	}

	c.logger.Info("Cognee not running, attempting to start container...")

	return c.startCogneeContainer(ctx)
}

// startCogneeContainer starts Cognee using Docker or Podman
func (c *Client) startCogneeContainer(ctx context.Context) error {
	var containerRuntime string
	var composeCmd []string

	if _, err := exec.LookPath("docker"); err == nil {
		containerRuntime = "docker"
		composeCmd = []string{"compose", "up", "-d", "cognee"}
	} else if _, err := exec.LookPath("podman"); err == nil {
		containerRuntime = "podman"
		composeCmd = []string{"compose", "up", "-d", "cognee"}
	} else {
		return fmt.Errorf("neither docker nor podman found in PATH")
	}

	c.logger.Info("Starting Cognee using %s...", containerRuntime)

	cmd := exec.CommandContext(ctx, containerRuntime, composeCmd...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to start Cognee container: %w, output: %s", err, string(output))
	}

	c.logger.Info("Cognee container start initiated, waiting for service to be ready...")

	for i := 0; i < 30; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
			if c.TestConnection(ctx) {
				c.logger.Info("Cognee service is now ready")
				return nil
			}
			c.logger.Debug("Waiting for Cognee service... attempt %d/30", i+1)
		}
	}

	return fmt.Errorf("cognee container started but service is not responding after 60 seconds")
}

// setHeaders sets common headers for API requests
func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if key := c.getAPIKey(); key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
}

// Close closes the client
func (c *Client) Close() error {
	c.httpClient.CloseIdleConnections()
	c.mu.Lock()
	c.connected = false
	c.mu.Unlock()
	return nil
}

// SetAPIKey updates the API key
func (c *Client) SetAPIKey(apiKey string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.apiKey = apiKey
}

// SetBaseURL updates the base URL
func (c *Client) SetBaseURL(baseURL string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.baseURL = strings.TrimSuffix(baseURL, "/")
}

// SetTimeout updates the HTTP client timeout
func (c *Client) SetTimeout(timeout time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.httpClient.Timeout = timeout
}
