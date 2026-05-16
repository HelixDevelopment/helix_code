package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"dev.helix.code/internal/logging"
	"dev.helix.code/internal/memory"
)

// ErrMem0OperationNotSupported is returned when an operation is not supported by the Mem0 API
var ErrMem0OperationNotSupported = fmt.Errorf("operation not supported by Mem0 API")

// Mem0Provider implements memory operations using Mem0.ai
type Mem0Provider struct {
	config  map[string]interface{}
	client  *http.Client
	baseURL string
	apiKey  string
	logger  *logging.Logger
	userID  string
	agentID string
	runID   string
}

// NewMem0Provider creates a new Mem0 provider instance
func NewMem0Provider(config map[string]interface{}) (*Mem0Provider, error) {
	provider := &Mem0Provider{
		config: config,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logging.NewLoggerWithName("mem0_provider"),
	}

	// Extract configuration
	if baseURL, ok := config["base_url"].(string); ok {
		provider.baseURL = baseURL
	} else {
		provider.baseURL = "https://api.mem0.ai"
	}

	if apiKey, ok := config["api_key"].(string); ok {
		provider.apiKey = apiKey
	}

	if userID, ok := config["user_id"].(string); ok {
		provider.userID = userID
	}

	if agentID, ok := config["agent_id"].(string); ok {
		provider.agentID = agentID
	}

	if runID, ok := config["run_id"].(string); ok {
		provider.runID = runID
	}

	return provider, nil
}

// GetType returns the provider type
func (p *Mem0Provider) GetType() string {
	return string(memory.ProviderTypeMem0)
}

// GetName returns the provider name
func (p *Mem0Provider) GetName() string {
	return "Mem0"
}

// GetCapabilities returns provider capabilities
func (p *Mem0Provider) GetCapabilities() []string {
	return []string{
		"memory_storage",
		"memory_retrieval",
		"memory_search",
		"context_management",
		"graph_memory",
		"vector_memory",
	}
}

// GetConfiguration returns provider configuration
func (p *Mem0Provider) GetConfiguration() interface{} {
	return p.config
}

// IsCloud returns whether this is a cloud provider
func (p *Mem0Provider) IsCloud() bool {
	return strings.Contains(p.baseURL, "mem0.ai") || strings.Contains(p.baseURL, "api.mem0")
}

// Store stores vectors in Mem0
func (p *Mem0Provider) Store(ctx context.Context, data []*VectorData) error {
	if len(data) == 0 {
		return nil
	}

	// Convert to Mem0 format
	memories := make([]map[string]interface{}, len(data))
	for i, item := range data {
		memories[i] = map[string]interface{}{
			"id":       item.ID,
			"text":     item.Metadata["content"],
			"user_id":  p.userID,
			"agent_id": p.agentID,
			"run_id":   p.runID,
			"metadata": item.Metadata,
		}
	}

	payload := map[string]interface{}{
		"memories": memories,
	}

	return p.makeRequest(ctx, "POST", "/memories/", payload, nil)
}

// Search searches for vectors in Mem0
func (p *Mem0Provider) Search(ctx context.Context, query *VectorQuery) (*VectorSearchResult, error) {
	// Prepare search payload
	payload := map[string]interface{}{
		"query":    query.Text,
		"user_id":  p.userID,
		"agent_id": p.agentID,
		"run_id":   p.runID,
		"limit":    query.TopK,
	}

	if query.Filters != nil {
		payload["filters"] = query.Filters
	}

	var response map[string]interface{}
	err := p.makeRequest(ctx, "POST", "/search/", payload, &response)
	if err != nil {
		return nil, err
	}

	// Parse response
	results := []*VectorSearchResultItem{}
	if memories, ok := response["memories"].([]interface{}); ok {
		for _, mem := range memories {
			if memMap, ok := mem.(map[string]interface{}); ok {
				item := &VectorSearchResultItem{
					ID:       getStringValue(memMap, "id"),
					Score:    getFloatValue(memMap, "score"),
					Metadata: memMap,
				}
				results = append(results, item)
			}
		}
	}

	return &VectorSearchResult{
		Results: results,
	}, nil
}

// Delete deletes vectors from Mem0
func (p *Mem0Provider) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	payload := map[string]interface{}{
		"memory_ids": ids,
		"user_id":    p.userID,
		"agent_id":   p.agentID,
		"run_id":     p.runID,
	}

	return p.makeRequest(ctx, "DELETE", "/memories/", payload, nil)
}

// GetStats returns provider statistics
func (p *Mem0Provider) GetStats(ctx context.Context) (*ProviderStats, error) {
	var response map[string]interface{}
	err := p.makeRequest(ctx, "GET", "/stats/", nil, &response)
	if err != nil {
		return nil, err
	}

	return &ProviderStats{
		TotalVectors:   int64(getFloatValue(response, "total_memories")),
		TotalSize:      int64(getFloatValue(response, "total_size_bytes")),
		AverageLatency: time.Duration(getFloatValue(response, "avg_latency_ms")) * time.Millisecond,
	}, nil
}

// Health checks provider health
func (p *Mem0Provider) Health(ctx context.Context) (*HealthStatus, error) {
	var response map[string]interface{}
	err := p.makeRequest(ctx, "GET", "/health/", nil, &response)
	if err != nil {
		return &HealthStatus{
			Status:    "unhealthy",
			Message:   err.Error(),
			Timestamp: time.Now(),
		}, nil
	}

	status := "healthy"
	if healthStatus, ok := response["status"].(string); ok && healthStatus != "ok" {
		status = "unhealthy"
	}

	return &HealthStatus{
		Status:    status,
		Timestamp: time.Now(),
	}, nil
}

// Close closes the provider
func (p *Mem0Provider) Close(ctx context.Context) error {
	// Cleanup resources if needed
	return nil
}

// AddMetadata adds metadata to a vector
func (p *Mem0Provider) AddMetadata(ctx context.Context, id string, metadata map[string]interface{}) error {
	// Mem0 doesn't have direct metadata addition, update the memory instead
	return p.UpdateMetadata(ctx, id, metadata)
}

// UpdateMetadata updates metadata for a vector
func (p *Mem0Provider) UpdateMetadata(ctx context.Context, id string, metadata map[string]interface{}) error {
	// For Mem0, this would require updating the memory entry
	// This is a simplified implementation
	payload := map[string]interface{}{
		"memory_id": id,
		"metadata":  metadata,
		"user_id":   p.userID,
		"agent_id":  p.agentID,
		"run_id":    p.runID,
	}

	return p.makeRequest(ctx, "PUT", "/memories/"+id+"/", payload, nil)
}

// GetMetadata retrieves metadata for vectors by fetching each memory individually
func (p *Mem0Provider) GetMetadata(ctx context.Context, ids []string) (map[string]map[string]interface{}, error) {
	if len(ids) == 0 {
		return make(map[string]map[string]interface{}), nil
	}

	result := make(map[string]map[string]interface{})

	// Mem0 requires fetching each memory by ID individually
	for _, id := range ids {
		var response map[string]interface{}
		err := p.makeRequest(ctx, "GET", "/memories/"+id+"/", nil, &response)
		if err != nil {
			// If the memory doesn't exist, continue with empty metadata
			p.logger.Debug("Failed to get metadata for memory %s: %v", id, err)
			result[id] = make(map[string]interface{})
			continue
		}

		// Extract metadata from the response
		if metadata, ok := response["metadata"].(map[string]interface{}); ok {
			result[id] = metadata
		} else {
			// Return the full response as metadata if no explicit metadata field
			result[id] = response
		}
	}

	return result, nil
}

// DeleteMetadata deletes specific metadata keys from memories
// Note: Mem0 doesn't support selective metadata deletion directly,
// so we fetch, remove keys, and update the memory
func (p *Mem0Provider) DeleteMetadata(ctx context.Context, ids []string, keys []string) error {
	if len(ids) == 0 || len(keys) == 0 {
		return nil
	}

	for _, id := range ids {
		// Fetch current metadata
		var response map[string]interface{}
		err := p.makeRequest(ctx, "GET", "/memories/"+id+"/", nil, &response)
		if err != nil {
			p.logger.Debug("Failed to get memory %s for metadata deletion: %v", id, err)
			continue
		}

		// Get current metadata
		metadata, ok := response["metadata"].(map[string]interface{})
		if !ok {
			metadata = make(map[string]interface{})
		}

		// Remove specified keys
		for _, key := range keys {
			delete(metadata, key)
		}

		// Update the memory with modified metadata
		err = p.UpdateMetadata(ctx, id, metadata)
		if err != nil {
			return fmt.Errorf("failed to update metadata after key deletion for memory %s: %w", id, err)
		}
	}

	return nil
}

// Retrieve retrieves memories by their IDs from Mem0
func (p *Mem0Provider) Retrieve(ctx context.Context, ids []string) ([]*VectorData, error) {
	if len(ids) == 0 {
		return []*VectorData{}, nil
	}

	results := make([]*VectorData, 0, len(ids))

	for _, id := range ids {
		var response map[string]interface{}
		err := p.makeRequest(ctx, "GET", "/memories/"+id+"/", nil, &response)
		if err != nil {
			// Log and continue if specific memory not found
			p.logger.Debug("Failed to retrieve memory %s: %v", id, err)
			continue
		}

		// Convert response to VectorData
		vectorData := &VectorData{
			ID:        getStringValue(response, "id"),
			Metadata:  make(map[string]interface{}),
			Timestamp: time.Now(),
		}

		// Extract metadata
		if metadata, ok := response["metadata"].(map[string]interface{}); ok {
			vectorData.Metadata = metadata
		}

		// Include text/content as metadata
		if text, ok := response["text"].(string); ok {
			vectorData.Metadata["content"] = text
		}
		if memory, ok := response["memory"].(string); ok {
			vectorData.Metadata["memory"] = memory
		}

		results = append(results, vectorData)
	}

	return results, nil
}

// Update updates a vector
func (p *Mem0Provider) Update(ctx context.Context, id string, vector *VectorData) error {
	// For Mem0, update the memory
	return p.UpdateMetadata(ctx, id, vector.Metadata)
}

// FindSimilar finds similar vectors
func (p *Mem0Provider) FindSimilar(ctx context.Context, embedding []float64, k int, filters map[string]interface{}) ([]*VectorSimilarityResult, error) {
	// Use search with vector
	query := &VectorQuery{
		Vector:  embedding,
		TopK:    k,
		Filters: filters,
	}
	result, err := p.Search(ctx, query)
	if err != nil {
		return nil, err
	}

	similar := make([]*VectorSimilarityResult, len(result.Results))
	for i, item := range result.Results {
		similar[i] = &VectorSimilarityResult{
			ID:       item.ID,
			Score:    item.Score,
			Distance: 1.0 - item.Score, // Convert score to distance
			Metadata: item.Metadata,
		}
	}
	return similar, nil
}

// BatchFindSimilar performs batch similarity search
func (p *Mem0Provider) BatchFindSimilar(ctx context.Context, queries [][]float64, k int) ([][]*VectorSimilarityResult, error) {
	results := make([][]*VectorSimilarityResult, len(queries))
	for i, query := range queries {
		similar, err := p.FindSimilar(ctx, query, k, nil)
		if err != nil {
			return nil, err
		}
		results[i] = similar
	}
	return results, nil
}

// CreateCollection creates a new collection
// Note: Mem0 API does not support traditional collection management.
// Memories are organized by user_id, agent_id, and run_id instead of collections.
// This method returns an error explaining the limitation.
func (p *Mem0Provider) CreateCollection(ctx context.Context, name string, config *CollectionConfig) error {
	return fmt.Errorf("%w: Mem0 does not support traditional collections; memories are organized by user_id, agent_id, and run_id. Configure these in the provider settings instead", ErrMem0OperationNotSupported)
}

// DeleteCollection deletes a collection
// Note: Mem0 API does not support collection deletion.
// To delete all memories for a user/agent, use Delete with the appropriate filters.
func (p *Mem0Provider) DeleteCollection(ctx context.Context, name string) error {
	return fmt.Errorf("%w: Mem0 does not support collection deletion; use Delete to remove memories by user_id, agent_id, or run_id", ErrMem0OperationNotSupported)
}

// ListCollections lists all collections
// Note: Mem0 API does not have a traditional collection concept.
// Returns information about the current user/agent/run configuration.
func (p *Mem0Provider) ListCollections(ctx context.Context) ([]*CollectionInfo, error) {
	// Return a virtual collection representing the current configuration
	collections := []*CollectionInfo{}

	// Create a virtual collection for the current user_id if set
	if p.userID != "" {
		collections = append(collections, &CollectionInfo{
			Name:      fmt.Sprintf("user_%s", p.userID),
			Status:    "active",
			CreatedAt: time.Now(),
			Metadata: map[string]interface{}{
				"type":    "user_memories",
				"user_id": p.userID,
			},
		})
	}

	// Create a virtual collection for the current agent_id if set
	if p.agentID != "" {
		collections = append(collections, &CollectionInfo{
			Name:      fmt.Sprintf("agent_%s", p.agentID),
			Status:    "active",
			CreatedAt: time.Now(),
			Metadata: map[string]interface{}{
				"type":     "agent_memories",
				"agent_id": p.agentID,
			},
		})
	}

	// Create a virtual collection for the current run_id if set
	if p.runID != "" {
		collections = append(collections, &CollectionInfo{
			Name:      fmt.Sprintf("run_%s", p.runID),
			Status:    "active",
			CreatedAt: time.Now(),
			Metadata: map[string]interface{}{
				"type":   "run_memories",
				"run_id": p.runID,
			},
		})
	}

	// If no configuration is set, return an empty collection representing all memories
	if len(collections) == 0 {
		collections = append(collections, &CollectionInfo{
			Name:      "all_memories",
			Status:    "active",
			CreatedAt: time.Now(),
			Metadata: map[string]interface{}{
				"type": "all_memories",
				"note": "No user_id, agent_id, or run_id configured",
			},
		})
	}

	return collections, nil
}

// GetCollection gets collection information
// Note: Mem0 API does not have traditional collections.
// This returns information about the virtual collection based on configuration.
func (p *Mem0Provider) GetCollection(ctx context.Context, name string) (*CollectionInfo, error) {
	collections, err := p.ListCollections(ctx)
	if err != nil {
		return nil, err
	}

	for _, col := range collections {
		if col.Name == name {
			return col, nil
		}
	}

	return nil, fmt.Errorf("collection '%s' not found: %w; Mem0 uses user_id, agent_id, and run_id for organization instead of collections", name, ErrMem0OperationNotSupported)
}

// CreateIndex creates an index
// Note: Mem0 is a managed service that handles indexing automatically.
// Users cannot create custom indexes as the vector storage is managed internally.
func (p *Mem0Provider) CreateIndex(ctx context.Context, collection string, config *IndexConfig) error {
	return fmt.Errorf("%w: Mem0 is a managed service that handles indexing automatically; custom index creation is not supported", ErrMem0OperationNotSupported)
}

// DeleteIndex deletes an index
// Note: Mem0 is a managed service that handles indexing automatically.
// Users cannot delete indexes as the vector storage is managed internally.
func (p *Mem0Provider) DeleteIndex(ctx context.Context, collection, name string) error {
	return fmt.Errorf("%w: Mem0 is a managed service that handles indexing automatically; custom index deletion is not supported", ErrMem0OperationNotSupported)
}

// ListIndexes lists indexes
// Note: Mem0 is a managed service that handles indexing automatically.
// Returns information about the internal indexing used by Mem0.
func (p *Mem0Provider) ListIndexes(ctx context.Context, collection string) ([]*IndexInfo, error) {
	// Return a virtual index representing Mem0's internal indexing
	return []*IndexInfo{
		{
			Name:      "mem0_managed_index",
			Type:      "managed",
			State:     "active",
			CreatedAt: time.Now(),
			Metadata: map[string]interface{}{
				"provider":    "mem0",
				"managed":     true,
				"description": "Mem0 manages vector indexing internally; this is a virtual representation",
			},
		},
	}, nil
}

// Optimize optimizes the provider
// Note: Mem0 is a managed service that handles optimization automatically.
// This method is a no-op for Mem0 as optimization is performed internally.
func (p *Mem0Provider) Optimize(ctx context.Context) error {
	p.logger.Debug("Optimize called on Mem0 provider - Mem0 handles optimization automatically")
	// Mem0 handles optimization internally; this is a no-op
	return nil
}

// Backup creates a backup
// Note: Mem0 is a managed cloud service that does not expose backup functionality.
// For data export, use the Mem0 dashboard or export memories via the API.
func (p *Mem0Provider) Backup(ctx context.Context, path string) error {
	return fmt.Errorf("%w: Mem0 is a managed cloud service; backup functionality is not exposed via API. For data export, use the Mem0 dashboard or export memories programmatically using Search/Retrieve operations", ErrMem0OperationNotSupported)
}

// Restore restores from backup
// Note: Mem0 is a managed cloud service that does not expose restore functionality.
// To restore data, import memories using the Store operation.
func (p *Mem0Provider) Restore(ctx context.Context, path string) error {
	return fmt.Errorf("%w: Mem0 is a managed cloud service; restore functionality is not exposed via API. To import data, use the Store operation to add memories", ErrMem0OperationNotSupported)
}

// Initialize initializes the provider
func (p *Mem0Provider) Initialize(ctx context.Context, config interface{}) error {
	// Already initialized in NewMem0Provider
	return nil
}

// Start starts the provider
func (p *Mem0Provider) Start(ctx context.Context) error {
	// HTTP client is ready
	return nil
}

// Stop stops the provider
func (p *Mem0Provider) Stop(ctx context.Context) error {
	return p.Close(ctx)
}

// GetCostInfo returns cost information
func (p *Mem0Provider) GetCostInfo() *CostInfo {
	return &CostInfo{
		Currency:      "USD",
		ComputeCost:   0.0,
		TransferCost:  0.0,
		StorageCost:   0.0,
		TotalCost:     0.0,
		BillingPeriod: "monthly",
		FreeTierUsed:  0.0,
		FreeTierLimit: 0.0,
	}
}

// makeRequest makes an HTTP request to Mem0 API
func (p *Mem0Provider) makeRequest(ctx context.Context, method, endpoint string, payload interface{}, response interface{}) error {
	url := p.baseURL + endpoint

	var body io.Reader
	if payload != nil {
		jsonData, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("failed to marshal payload: %w", err)
		}
		body = strings.NewReader(string(jsonData))
	}

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	// Make request
	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse response if needed
	if response != nil {
		if err := json.Unmarshal(respBody, response); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}
	}

	return nil
}

// Helper functions
func getStringValue(data map[string]interface{}, key string) string {
	if val, ok := data[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func getFloatValue(data map[string]interface{}, key string) float64 {
	if val, ok := data[key]; ok {
		if num, ok := val.(float64); ok {
			return num
		}
		if num, ok := val.(int); ok {
			return float64(num)
		}
	}
	return 0.0
}
