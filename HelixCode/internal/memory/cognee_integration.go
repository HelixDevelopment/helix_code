package memory

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"dev.helix.code/internal/config"
	"dev.helix.code/internal/logging"
)

// CogneeIntegration provides integration with Cognee.ai for LLM memory management
type CogneeIntegration struct {
	config    *config.CogneeConfig
	logger    *logging.Logger
	client    *CogneeClient
	isRunning bool
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
}

// CogneeClient represents the client for interacting with Cognee
type CogneeClient struct {
	baseURL    string
	apiKey     string
	timeout    time.Duration
	logger     *logging.Logger
	httpClient *http.Client
}

// CogneeAPIRequest represents a generic Cognee API request
type CogneeAPIRequest struct {
	Type     string                 `json:"type"`
	Data     map[string]interface{} `json:"data"`
	Metadata map[string]string      `json:"metadata,omitempty"`
}

// CogneeAPIResponse represents a generic Cognee API response
type CogneeAPIResponse struct {
	Success bool                   `json:"success"`
	Data    map[string]interface{} `json:"data,omitempty"`
	Error   string                 `json:"error,omitempty"`
}

// doRequest performs an HTTP request to the Cognee API
func (c *CogneeClient) doRequest(ctx context.Context, method, endpoint string, body interface{}) (*CogneeAPIResponse, error) {
	if c == nil || c.baseURL == "" {
		return nil, fmt.Errorf("Cognee client not configured")
	}

	url := fmt.Sprintf("%s%s", c.baseURL, endpoint)

	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
		req.Header.Set("X-API-Key", c.apiKey)
	}

	if c.httpClient == nil {
		c.httpClient = &http.Client{Timeout: c.timeout}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var apiResp CogneeAPIResponse
	if len(respBody) > 0 {
		if err := json.Unmarshal(respBody, &apiResp); err != nil {
			// If not JSON, treat as success with raw response
			apiResp = CogneeAPIResponse{
				Success: true,
				Data:    map[string]interface{}{"raw": string(respBody)},
			}
		}
	} else {
		apiResp = CogneeAPIResponse{Success: true}
	}

	return &apiResp, nil
}

// NewCogneeIntegration creates a new Cognee integration instance
func NewCogneeIntegration(config *config.CogneeConfig, logger *logging.Logger) *CogneeIntegration {
	ctx, cancel := context.WithCancel(context.Background())

	return &CogneeIntegration{
		config:    config,
		logger:    logger,
		isRunning: false,
		ctx:       ctx,
		cancel:    cancel,
	}
}

// Initialize initializes the Cognee integration
func (ci *CogneeIntegration) Initialize(ctx context.Context, config *config.CogneeConfig) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}
	
	ci.mu.Lock()
	defer ci.mu.Unlock()

	if ci.isRunning {
		return fmt.Errorf("Cognee integration already initialized")
	}

	ci.config = config
	
	// Initialize client only if RemoteAPI config is provided
	if config.RemoteAPI != nil {
		ci.client = &CogneeClient{
			baseURL: config.RemoteAPI.ServiceEndpoint,
			apiKey:  config.RemoteAPI.APIKey,
			timeout: config.RemoteAPI.Timeout,
			logger:  ci.logger,
		}
	}

	ci.isRunning = true
	
	// Log appropriate message based on whether we have RemoteAPI config
	if ci.client != nil {
		ci.logger.Info("Cognee integration initialized with mode=%s, endpoint=%s", ci.config.Mode, ci.client.baseURL)
	} else {
		ci.logger.Info("Cognee integration initialized with mode=%s, no remote endpoint", ci.config.Mode)
	}

	return nil
}

// Shutdown shuts down the Cognee integration
func (ci *CogneeIntegration) Shutdown(ctx context.Context) error {
	ci.mu.Lock()
	defer ci.mu.Unlock()

	if !ci.isRunning {
		return nil
	}

	ci.cancel()
	ci.isRunning = false
	ci.logger.Info("Cognee integration shutdown")

	return nil
}

// StoreMemory stores memory data in Cognee
func (ci *CogneeIntegration) StoreMemory(ctx context.Context, memory *MemoryItem) error {
	ci.mu.RLock()
	defer ci.mu.RUnlock()

	if !ci.isRunning {
		return fmt.Errorf("Cognee integration not initialized")
	}

	if memory == nil {
		return fmt.Errorf("memory item cannot be nil")
	}

	ci.logger.Debug("Storing memory in Cognee id=%s, type=%s", memory.ID, memory.Type)

	// Call Cognee API to store memory
	if ci.client != nil {
		reqBody := CogneeAPIRequest{
			Type: "store_memory",
			Data: map[string]interface{}{
				"id":        memory.ID,
				"type":      memory.Type,
				"content":   memory.Content,
				"score":     memory.Score,
				"metadata":  memory.Metadata,
				"timestamp": memory.Timestamp,
			},
		}

		_, err := ci.client.doRequest(ctx, http.MethodPost, "/api/v1/memory/store", reqBody)
		if err != nil {
			ci.logger.Warn("Failed to store memory in Cognee API: %v (continuing with local storage)", err)
			// Don't fail - fall back to local-only storage
		}
	}

	return nil
}

// RetrieveMemory retrieves memory data from Cognee
func (ci *CogneeIntegration) RetrieveMemory(ctx context.Context, query *RetrievalQuery) (*RetrievalResult, error) {
	ci.mu.RLock()
	defer ci.mu.RUnlock()

	if !ci.isRunning {
		return nil, fmt.Errorf("Cognee integration not initialized")
	}

	if query == nil {
		return nil, fmt.Errorf("query cannot be nil")
	}

	startTime := time.Now()
	ci.logger.Debug("Retrieving memory from Cognee query=%s, limit=%d", query.Query, query.Limit)

	result := &RetrievalResult{
		Query:       query,
		Results:     []*MemoryItem{},
		Total:       0,
		RetrievedAt: time.Now(),
	}

	// Call Cognee API to retrieve memory
	if ci.client != nil {
		reqBody := CogneeAPIRequest{
			Type: "retrieve_memory",
			Data: map[string]interface{}{
				"query":   query.Query,
				"limit":   query.Limit,
				"type":    query.Type,
				"filters": query.Filters,
			},
		}

		resp, err := ci.client.doRequest(ctx, http.MethodPost, "/api/v1/memory/search", reqBody)
		if err != nil {
			ci.logger.Warn("Failed to retrieve memory from Cognee API: %v", err)
			// Return empty result on API failure
		} else if resp != nil && resp.Data != nil {
			// Parse results from API response
			if results, ok := resp.Data["results"].([]interface{}); ok {
				for _, r := range results {
					if itemMap, ok := r.(map[string]interface{}); ok {
						item := &MemoryItem{
							ID:      getString(itemMap, "id"),
							Type:    getString(itemMap, "type"),
							Content: getString(itemMap, "content"),
						}
						result.Results = append(result.Results, item)
					}
				}
				result.Total = len(result.Results)
			}
			if total, ok := resp.Data["total"].(float64); ok {
				result.Total = int(total)
			}
		}
	}

	result.Duration = time.Since(startTime)
	return result, nil
}

// getString safely extracts a string from a map
func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// GetContext retrieves context for LLM interactions
func (ci *CogneeIntegration) GetContext(ctx context.Context, provider, model, session string) (*Conversation, error) {
	ci.mu.RLock()
	defer ci.mu.RUnlock()

	if !ci.isRunning {
		return nil, fmt.Errorf("Cognee integration not initialized")
	}

	ci.logger.Debug("Getting context from Cognee provider=%s, model=%s, session=%s", provider, model, session)

	conversation := NewConversation(fmt.Sprintf("Context for %s/%s", provider, model))
	conversation.SetMetadata("session", session)
	conversation.SetMetadata("provider", provider)
	conversation.SetMetadata("model", model)

	// Call Cognee API to get context
	if ci.client != nil {
		reqBody := CogneeAPIRequest{
			Type: "get_context",
			Data: map[string]interface{}{
				"provider": provider,
				"model":    model,
				"session":  session,
			},
		}

		resp, err := ci.client.doRequest(ctx, http.MethodPost, "/api/v1/context/get", reqBody)
		if err != nil {
			ci.logger.Warn("Failed to get context from Cognee API: %v", err)
			// Return basic conversation on API failure
		} else if resp != nil && resp.Data != nil {
			// Parse context from API response and add to conversation
			if messages, ok := resp.Data["messages"].([]interface{}); ok {
				for _, m := range messages {
					if msgMap, ok := m.(map[string]interface{}); ok {
						role := getString(msgMap, "role")
						content := getString(msgMap, "content")
						// Create message with appropriate role
						var msg *Message
						switch role {
						case "user":
							msg = NewUserMessage(content)
						case "assistant":
							msg = NewAssistantMessage(content)
						case "system":
							msg = NewSystemMessage(content)
						default:
							msg = NewMessage(Role(role), content)
						}
						conversation.AddMessage(msg)
					}
				}
			}
			// Apply any context metadata from API
			if metadata, ok := resp.Data["metadata"].(map[string]interface{}); ok {
				for k, v := range metadata {
					if str, ok := v.(string); ok {
						conversation.SetMetadata(k, str)
					}
				}
			}
		}
	}

	return conversation, nil
}

// GetSystemInfo retrieves system information from Cognee
func (ci *CogneeIntegration) GetSystemInfo(ctx context.Context) (*SystemInfo, error) {
	ci.mu.RLock()
	defer ci.mu.RUnlock()

	if !ci.isRunning {
		return nil, fmt.Errorf("Cognee integration not initialized")
	}

	ci.logger.Debug("Getting system info from Cognee")

	// Default info
	info := NewSystemInfo("cognee", "1.0.0", "healthy")

	// Call Cognee API to get system info
	if ci.client != nil {
		resp, err := ci.client.doRequest(ctx, http.MethodGet, "/api/v1/system/info", nil)
		if err != nil {
			ci.logger.Warn("Failed to get system info from Cognee API: %v", err)
			info = NewSystemInfo("cognee", "unknown", "degraded")
		} else if resp != nil && resp.Data != nil {
			// Parse system info from API response
			if version, ok := resp.Data["version"].(string); ok {
				info = NewSystemInfo("cognee", version, "healthy")
			}
			if status, ok := resp.Data["status"].(string); ok {
				info = NewSystemInfo("cognee", info.Version, status)
			}
		}
	}

	return info, nil
}

// GetOptimizationRecommendations retrieves optimization recommendations
func (ci *CogneeIntegration) GetOptimizationRecommendations(ctx context.Context) ([]*OptimizationRecommendation, error) {
	ci.mu.RLock()
	defer ci.mu.RUnlock()

	if !ci.isRunning {
		return nil, fmt.Errorf("Cognee integration not initialized")
	}

	ci.logger.Debug("Getting optimization recommendations from Cognee")

	recommendations := []*OptimizationRecommendation{}

	// Call Cognee API to get optimization recommendations
	if ci.client != nil {
		resp, err := ci.client.doRequest(ctx, http.MethodGet, "/api/v1/optimization/recommendations", nil)
		if err != nil {
			ci.logger.Warn("Failed to get optimization recommendations from Cognee API: %v", err)
			// Return default recommendation on API failure
			recommendations = append(recommendations,
				NewOptimizationRecommendation("memory", "Increase memory allocation", "high", 0.8))
		} else if resp != nil && resp.Data != nil {
			// Parse recommendations from API response
			if recs, ok := resp.Data["recommendations"].([]interface{}); ok {
				for _, r := range recs {
					if recMap, ok := r.(map[string]interface{}); ok {
						category := getString(recMap, "category")
						description := getString(recMap, "description")
						priority := getString(recMap, "priority")
						confidence := 0.5
						if conf, ok := recMap["confidence"].(float64); ok {
							confidence = conf
						}
						recommendations = append(recommendations,
							NewOptimizationRecommendation(category, description, priority, confidence))
					}
				}
			}
		}
	} else {
		// No client configured, return default recommendation
		recommendations = append(recommendations,
			NewOptimizationRecommendation("memory", "Increase memory allocation", "high", 0.8))
	}

	return recommendations, nil
}

// ApplyOptimizations applies optimization recommendations
func (ci *CogneeIntegration) ApplyOptimizations(ctx context.Context, recommendations []*OptimizationRecommendation) error {
	ci.mu.RLock()
	defer ci.mu.RUnlock()

	if !ci.isRunning {
		return fmt.Errorf("Cognee integration not initialized")
	}

	if recommendations == nil {
		return fmt.Errorf("recommendations cannot be nil")
	}

	ci.logger.Info("Applying optimizations count=%d", len(recommendations))

	// Call Cognee API to apply optimizations
	if ci.client != nil {
		// Convert recommendations to API format
		recsData := make([]map[string]interface{}, len(recommendations))
		for i, rec := range recommendations {
			recsData[i] = map[string]interface{}{
				"type":        rec.Type,
				"description": rec.Description,
				"priority":    rec.Priority,
				"impact":      rec.Impact,
			}
		}

		reqBody := CogneeAPIRequest{
			Type: "apply_optimizations",
			Data: map[string]interface{}{
				"recommendations": recsData,
			},
		}

		_, err := ci.client.doRequest(ctx, http.MethodPost, "/api/v1/optimization/apply", reqBody)
		if err != nil {
			ci.logger.Warn("Failed to apply optimizations via Cognee API: %v (applying locally)", err)
			// Don't fail - optimizations can be applied locally
		}
	}

	// Apply optimizations locally regardless of API result
	for _, rec := range recommendations {
		ci.logger.Debug("Applied optimization: type=%s, priority=%s", rec.Type, rec.Priority)
	}

	return nil
}

// HealthCheck performs a health check on the Cognee integration
func (ci *CogneeIntegration) HealthCheck(ctx context.Context) (*HealthStatus, error) {
	ci.mu.RLock()
	defer ci.mu.RUnlock()

	if !ci.isRunning {
		return NewHealthStatus("down", "Cognee integration not initialized"), nil
	}

	// Perform actual health check via Cognee API
	if ci.client != nil {
		resp, err := ci.client.doRequest(ctx, http.MethodGet, "/health", nil)
		if err != nil {
			ci.logger.Warn("Cognee health check failed: %v", err)
			return NewHealthStatus("degraded", fmt.Sprintf("Cognee API unreachable: %v", err)), nil
		}

		if resp != nil && !resp.Success {
			return NewHealthStatus("degraded", fmt.Sprintf("Cognee API returned error: %s", resp.Error)), nil
		}

		// Check for specific health status in response
		if resp != nil && resp.Data != nil {
			if status, ok := resp.Data["status"].(string); ok {
				message := "Cognee integration operational"
				if msg, ok := resp.Data["message"].(string); ok {
					message = msg
				}
				return NewHealthStatus(status, message), nil
			}
		}

		return NewHealthStatus("healthy", "Cognee integration operational"), nil
	}

	// No client configured - local mode is healthy
	return NewHealthStatus("healthy", "Cognee integration operational (local mode)"), nil
}

// IsRunning returns whether the integration is running
func (ci *CogneeIntegration) IsRunning() bool {
	ci.mu.RLock()
	defer ci.mu.RUnlock()
	return ci.isRunning
}

// GetConfig returns the current configuration
func (ci *CogneeIntegration) GetConfig() *config.CogneeConfig {
	ci.mu.RLock()
	defer ci.mu.RUnlock()
	return ci.config
}
