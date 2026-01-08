package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"dev.helix.code/internal/logging"
	"dev.helix.code/internal/memory"
)

// BaseAIProvider implements memory operations using BaseAI API
type BaseAIProvider struct {
	config          map[string]interface{}
	logger          *logging.Logger
	apiKey          string
	baseURL         string
	httpClient      *http.Client
	mu              sync.RWMutex
	localCache      map[string][]byte
	collections     map[string]*CollectionInfo
	stats           *ProviderStats
	costTracker     *BaseAICostTracker
	lastHealthCheck time.Time
}

// BaseAI API structures
type BaseAICostTracker struct {
	TotalCost    float64   `json:"total_cost"`
	Operations   int       `json:"operations"`
	RequestCount int       `json:"request_count"`
	LastUpdated  time.Time `json:"last_updated"`
}

type BaseAIRequest struct {
	Action     string                 `json:"action"`
	Collection string                 `json:"collection,omitempty"`
	Data       []BaseAIVectorData     `json:"data,omitempty"`
	Query      BaseAIQuery            `json:"query,omitempty"`
	Options    map[string]interface{} `json:"options,omitempty"`
}

type BaseAIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Meta    *BaseAIMeta `json:"meta,omitempty"`
}

type BaseAIMeta struct {
	Cost    float64   `json:"cost"`
	Tokens  int       `json:"tokens"`
	Latency float64   `json:"latency"`
	Time    time.Time `json:"time"`
}

type BaseAIVectorData struct {
	ID       string                 `json:"id"`
	Vector   []float64              `json:"vector"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	Text     string                 `json:"text,omitempty"`
}

type BaseAIQuery struct {
	Vector  []float64              `json:"vector,omitempty"`
	Text    string                 `json:"text,omitempty"`
	TopK    int                    `json:"top_k"`
	Filters map[string]interface{} `json:"filters,omitempty"`
}

type BaseAICollection struct {
	Name        string                 `json:"name"`
	Config      map[string]interface{} `json:"config"`
	Created     time.Time              `json:"created"`
	Updated     time.Time              `json:"updated"`
	VectorCount int                    `json:"vector_count"`
	Size        int64                  `json:"size"`
}

// NewBaseAIProvider creates a new BaseAI provider instance
func NewBaseAIProvider(config map[string]interface{}) (*BaseAIProvider, error) {
	provider := &BaseAIProvider{
		config:      config,
		logger:      logging.NewLoggerWithName("baseai_provider"),
		httpClient:  &http.Client{Timeout: 30 * time.Second},
		localCache:  make(map[string][]byte),
		collections: make(map[string]*CollectionInfo),
		stats: &ProviderStats{
			Name:             "BaseAI",
			Type:             "baseai",
			Status:           "initializing",
			TotalOperations:  0,
			SuccessfulOps:    0,
			FailedOps:        0,
			AverageLatency:   0,
			TotalVectors:     0,
			TotalCollections: 0,
			TotalSize:        0,
			LastHealthCheck:  time.Now(),
		},
		costTracker: &BaseAICostTracker{},
	}

	// Extract configuration
	if apiKey, ok := config["api_key"].(string); ok {
		provider.apiKey = apiKey
	} else {
		return nil, fmt.Errorf("baseai api_key is required")
	}

	if baseURL, ok := config["base_url"].(string); ok {
		provider.baseURL = strings.TrimSuffix(baseURL, "/")
	} else {
		provider.baseURL = "https://api.langbase.com/v1"
	}

	// Initialize with default collection
	defaultCollection := &CollectionInfo{
		Name:        "default",
		VectorCount: 0,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Config: &CollectionConfig{
			Dimension: 1536, // Default OpenAI embedding dimension
			Metric:    "cosine",
		},
	}
	provider.collections["default"] = defaultCollection
	provider.stats.TotalCollections = 1

	return provider, nil
}

// GetType returns the provider type
func (p *BaseAIProvider) GetType() string {
	return string(memory.ProviderTypeBaseAI)
}

// GetName returns the provider name
func (p *BaseAIProvider) GetName() string {
	return "BaseAI"
}

// GetCapabilities returns provider capabilities
func (p *BaseAIProvider) GetCapabilities() []string {
	return []string{
		"memory_storage",
		"memory_retrieval",
		"memory_search",
		"context_management",
		"rag_memory",
		"document_memory",
		"agent_memory",
		"metadata_operations",
		"cost_tracking",
	}
}

// GetConfiguration returns provider configuration
func (p *BaseAIProvider) GetConfiguration() interface{} {
	return p.config
}

// IsCloud returns whether this is a cloud provider
func (p *BaseAIProvider) IsCloud() bool {
	return true
}

// Store stores memory data in BaseAI
func (p *BaseAIProvider) Store(ctx context.Context, data []*VectorData) error {
	if len(data) == 0 {
		return nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	start := time.Now()
	p.stats.TotalOperations++

	// Convert to BaseAI format
	baseAIData := make([]BaseAIVectorData, len(data))
	for i, item := range data {
		baseAIData[i] = BaseAIVectorData{
			ID:       item.ID,
			Vector:   item.Vector,
			Metadata: item.Metadata,
			Text:     getStringFromMetadata(item.Metadata, "text"),
		}
	}

	// Prepare API request
	req := BaseAIRequest{
		Action:     "store",
		Collection: "default",
		Data:       baseAIData,
		Options:    map[string]interface{}{},
	}

	// Call BaseAI API
	resp, cost, err := p.callAPI(ctx, req)
	if err != nil {
		p.stats.FailedOps++
		p.stats.Status = "error"
		p.logger.Error("BaseAI Store failed: %v", err)
		return fmt.Errorf("BaseAI Store failed: %v", err)
	}

	// Update local cache and stats
	if resp.Success {
		p.stats.SuccessfulOps++
		p.stats.TotalVectors += int64(len(data))
		p.stats.AverageLatency = time.Duration(float64(p.stats.AverageLatency) + float64(time.Since(start))/2)
		p.costTracker.TotalCost += cost
		p.costTracker.Operations++
		p.stats.Status = "active"

		// Update collection info
		if coll, exists := p.collections["default"]; exists {
			coll.VectorCount += int64(len(data))
			coll.UpdatedAt = time.Now()
		}

		p.logger.Info("Successfully stored %d vectors to BaseAI", len(data))
		return nil
	} else {
		p.stats.FailedOps++
		p.stats.Status = "error"
		return fmt.Errorf("BaseAI API error: %s", resp.Error)
	}
}

// Search searches for memory in BaseAI
func (p *BaseAIProvider) Search(ctx context.Context, query *VectorQuery) (*VectorSearchResult, error) {
	if query == nil {
		return nil, fmt.Errorf("query cannot be nil")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	start := time.Now()
	p.stats.TotalOperations++

	// Prepare BaseAI query
	baseAIQuery := BaseAIQuery{
		Text:    query.Text,
		TopK:    query.TopK,
		Filters: query.Filters,
	}

	if query.Vector != nil && len(query.Vector) > 0 {
		baseAIQuery.Vector = query.Vector
	}

	// Prepare API request
	req := BaseAIRequest{
		Action:     "search",
		Collection: "default",
		Query:      baseAIQuery,
	}

	// Call BaseAI API
	resp, cost, err := p.callAPI(ctx, req)
	if err != nil {
		p.stats.FailedOps++
		p.logger.Error("BaseAI Search failed: %v", err)
		return &VectorSearchResult{Results: []*VectorSearchResultItem{}}, nil
	}

	p.costTracker.TotalCost += cost
	p.costTracker.Operations++

	// Convert results
	result := &VectorSearchResult{
		Results: []*VectorSearchResultItem{},
	}

	if resp.Success && resp.Data != nil {
		if data, ok := resp.Data.([]interface{}); ok {
			for _, item := range data {
				if resultMap, ok := item.(map[string]interface{}); ok {
					resultItem := &VectorSearchResultItem{
						ID:       getStringFromMap(resultMap, "id"),
						Score:    getFloatFromMap(resultMap, "score"),
						Metadata: getMapFromMap(resultMap, "metadata"),
					}
					result.Results = append(result.Results, resultItem)
				}
			}
		}
	}

	p.stats.SuccessfulOps++
	p.stats.AverageLatency = time.Duration(float64(p.stats.AverageLatency) + float64(time.Since(start))/2)
	p.logger.Info("BaseAI Search returned %d results", len(result.Results))

	return result, nil
}

// Retrieve retrieves vectors by IDs from BaseAI
func (p *BaseAIProvider) Retrieve(ctx context.Context, ids []string) ([]*VectorData, error) {
	if len(ids) == 0 {
		return []*VectorData{}, nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	start := time.Now()
	p.stats.TotalOperations++

	// Prepare API request
	req := BaseAIRequest{
		Action:     "retrieve",
		Collection: "default",
		Options:    map[string]interface{}{"ids": ids},
	}

	// Call BaseAI API
	resp, cost, err := p.callAPI(ctx, req)
	if err != nil {
		p.stats.FailedOps++
		p.logger.Error("BaseAI Retrieve failed: %v", err)
		return []*VectorData{}, nil
	}

	p.costTracker.TotalCost += cost
	p.costTracker.Operations++

	// Convert results
	results := []*VectorData{}
	if resp.Success && resp.Data != nil {
		if data, ok := resp.Data.([]interface{}); ok {
			for _, item := range data {
				if resultMap, ok := item.(map[string]interface{}); ok {
					vectorData := &VectorData{
						ID:       getStringFromMap(resultMap, "id"),
						Vector:   getFloatSliceFromMap(resultMap, "vector"),
						Metadata: getMapFromMap(resultMap, "metadata"),
					}
					results = append(results, vectorData)
				}
			}
		}
	}

	p.stats.SuccessfulOps++
	p.stats.AverageLatency = time.Duration(float64(p.stats.AverageLatency) + float64(time.Since(start))/2)

	return results, nil
}

// Update updates a vector in BaseAI
func (p *BaseAIProvider) Update(ctx context.Context, id string, vector *VectorData) error {
	if vector == nil {
		return fmt.Errorf("vector data cannot be nil")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	start := time.Now()
	p.stats.TotalOperations++

	// Convert to BaseAI format
	baseAIData := BaseAIVectorData{
		ID:       vector.ID,
		Vector:   vector.Vector,
		Metadata: vector.Metadata,
		Text:     getStringFromMetadata(vector.Metadata, "text"),
	}

	// Prepare API request
	req := BaseAIRequest{
		Action:     "update",
		Collection: "default",
		Data:       []BaseAIVectorData{baseAIData},
		Options:    map[string]interface{}{"id": id},
	}

	// Call BaseAI API
	resp, cost, err := p.callAPI(ctx, req)
	if err != nil {
		p.stats.FailedOps++
		p.logger.Error("BaseAI Update failed: %v", err)
		return fmt.Errorf("BaseAI Update failed: %v", err)
	}

	p.costTracker.TotalCost += cost
	p.costTracker.Operations++

	if resp.Success {
		p.stats.SuccessfulOps++
		p.stats.AverageLatency = time.Duration(float64(p.stats.AverageLatency) + float64(time.Since(start))/2)
		p.logger.Info("Successfully updated vector %s in BaseAI", id)
		return nil
	} else {
		p.stats.FailedOps++
		return fmt.Errorf("BaseAI API error: %s", resp.Error)
	}
}

// Delete deletes memory from BaseAI
func (p *BaseAIProvider) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	start := time.Now()
	p.stats.TotalOperations++

	// Prepare API request
	req := BaseAIRequest{
		Action:     "delete",
		Collection: "default",
		Options:    map[string]interface{}{"ids": ids},
	}

	// Call BaseAI API
	resp, cost, err := p.callAPI(ctx, req)
	if err != nil {
		p.stats.FailedOps++
		p.logger.Error("BaseAI Delete failed: %v", err)
		return fmt.Errorf("BaseAI Delete failed: %v", err)
	}

	p.costTracker.TotalCost += cost
	p.costTracker.Operations++

	if resp.Success {
		p.stats.SuccessfulOps++
		p.stats.AverageLatency = time.Duration(float64(p.stats.AverageLatency) + float64(time.Since(start))/2)
		p.stats.TotalVectors -= int64(len(ids))

		// Update collection info
		if coll, exists := p.collections["default"]; exists {
			coll.VectorCount -= int64(len(ids))
			coll.UpdatedAt = time.Now()
		}

		p.logger.Info("Successfully deleted %d vectors from BaseAI", len(ids))
		return nil
	} else {
		p.stats.FailedOps++
		return fmt.Errorf("BaseAI API error: %s", resp.Error)
	}
}

// FindSimilar finds similar vectors in BaseAI
func (p *BaseAIProvider) FindSimilar(ctx context.Context, embedding []float64, k int, filters map[string]interface{}) ([]*VectorSimilarityResult, error) {
	if len(embedding) == 0 {
		return []*VectorSimilarityResult{}, nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	start := time.Now()
	p.stats.TotalOperations++

	// Prepare API request
	req := BaseAIRequest{
		Action:     "similarity",
		Collection: "default",
		Query: BaseAIQuery{
			Vector:  embedding,
			TopK:    k,
			Filters: filters,
		},
	}

	// Call BaseAI API
	resp, cost, err := p.callAPI(ctx, req)
	if err != nil {
		p.stats.FailedOps++
		p.logger.Error("BaseAI FindSimilar failed: %v", err)
		return []*VectorSimilarityResult{}, nil
	}

	p.costTracker.TotalCost += cost
	p.costTracker.Operations++

	// Convert results
	results := []*VectorSimilarityResult{}
	if resp.Success && resp.Data != nil {
		if data, ok := resp.Data.([]interface{}); ok {
			for _, item := range data {
				if resultMap, ok := item.(map[string]interface{}); ok {
					similarityResult := &VectorSimilarityResult{
						ID:       getStringFromMap(resultMap, "id"),
						Score:    getFloatFromMap(resultMap, "score"),
						Metadata: getMapFromMap(resultMap, "metadata"),
					}
					results = append(results, similarityResult)
				}
			}
		}
	}

	p.stats.SuccessfulOps++
	p.stats.AverageLatency = time.Duration(float64(p.stats.AverageLatency) + float64(time.Since(start))/2)

	return results, nil
}

// BatchFindSimilar finds similar vectors for multiple queries in BaseAI
func (p *BaseAIProvider) BatchFindSimilar(ctx context.Context, queries [][]float64, k int) ([][]*VectorSimilarityResult, error) {
	if len(queries) == 0 {
		return [][]*VectorSimilarityResult{}, nil
	}

	results := make([][]*VectorSimilarityResult, len(queries))
	for i, query := range queries {
		similar, err := p.FindSimilar(ctx, query, k, nil)
		if err != nil {
			p.logger.Error("BatchFindSimilar query %d failed: %v", i, err)
			results[i] = []*VectorSimilarityResult{}
		} else {
			results[i] = similar
		}
	}

	return results, nil
}

// CreateCollection creates a collection in BaseAI
func (p *BaseAIProvider) CreateCollection(ctx context.Context, name string, config *CollectionConfig) error {
	if name == "" {
		return fmt.Errorf("collection name cannot be empty")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	start := time.Now()
	p.stats.TotalOperations++

	// Prepare API request
	req := BaseAIRequest{
		Action:     "create_collection",
		Collection: name,
		Options: map[string]interface{}{
			"dimension": config.Dimension,
			"metric":    config.Metric,
		},
	}

	// Call BaseAI API
	resp, cost, err := p.callAPI(ctx, req)
	if err != nil {
		p.stats.FailedOps++
		p.logger.Error("BaseAI CreateCollection failed: %v", err)
		return fmt.Errorf("BaseAI CreateCollection failed: %v", err)
	}

	p.costTracker.TotalCost += cost
	p.costTracker.Operations++

	if resp.Success {
		// Create local collection info
		collectionInfo := &CollectionInfo{
			Name:        name,
			VectorCount: 0,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
			Config:      config,
		}
		p.collections[name] = collectionInfo
		p.stats.TotalCollections++

		p.stats.SuccessfulOps++
		p.stats.AverageLatency = time.Duration(float64(p.stats.AverageLatency) + float64(time.Since(start))/2)
		p.logger.Info("Successfully created collection %s in BaseAI", name)
		return nil
	} else {
		p.stats.FailedOps++
		return fmt.Errorf("BaseAI API error: %s", resp.Error)
	}
}

// DeleteCollection deletes a collection in BaseAI
func (p *BaseAIProvider) DeleteCollection(ctx context.Context, name string) error {
	if name == "" {
		return fmt.Errorf("collection name cannot be empty")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	start := time.Now()
	p.stats.TotalOperations++

	// Prepare API request
	req := BaseAIRequest{
		Action:     "delete_collection",
		Collection: name,
	}

	// Call BaseAI API
	resp, cost, err := p.callAPI(ctx, req)
	if err != nil {
		p.stats.FailedOps++
		p.logger.Error("BaseAI DeleteCollection failed: %v", err)
		return fmt.Errorf("BaseAI DeleteCollection failed: %v", err)
	}

	p.costTracker.TotalCost += cost
	p.costTracker.Operations++

	if resp.Success {
		// Remove from local collections
		delete(p.collections, name)
		p.stats.TotalCollections--

		p.stats.SuccessfulOps++
		p.stats.AverageLatency = time.Duration(float64(p.stats.AverageLatency) + float64(time.Since(start))/2)
		p.logger.Info("Successfully deleted collection %s from BaseAI", name)
		return nil
	} else {
		p.stats.FailedOps++
		return fmt.Errorf("BaseAI API error: %s", resp.Error)
	}
}

// ListCollections lists collections in BaseAI
func (p *BaseAIProvider) ListCollections(ctx context.Context) ([]*CollectionInfo, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	start := time.Now()
	p.stats.TotalOperations++

	// Prepare API request
	req := BaseAIRequest{
		Action: "list_collections",
	}

	// Call BaseAI API
	resp, cost, err := p.callAPI(ctx, req)
	if err != nil {
		p.stats.FailedOps++
		p.logger.Error("BaseAI ListCollections failed: %v", err)
		return []*CollectionInfo{}, nil
	}

	p.costTracker.TotalCost += cost
	p.costTracker.Operations++

	// Convert results
	results := []*CollectionInfo{}
	if resp.Success && resp.Data != nil {
		if data, ok := resp.Data.([]interface{}); ok {
			for _, item := range data {
				if resultMap, ok := item.(map[string]interface{}); ok {
					collectionInfo := &CollectionInfo{
						Name:        getStringFromMap(resultMap, "name"),
						VectorCount: int64(getIntFromMap(resultMap, "vector_count")),
						CreatedAt:   getTimeFromMap(resultMap, "created"),
						UpdatedAt:   getTimeFromMap(resultMap, "updated"),
					}
					results = append(results, collectionInfo)
				}
			}
		}
	}

	p.stats.SuccessfulOps++
	p.stats.AverageLatency = time.Duration(float64(p.stats.AverageLatency) + float64(time.Since(start))/2)

	return results, nil
}

// GetCollection gets collection info in BaseAI
func (p *BaseAIProvider) GetCollection(ctx context.Context, name string) (*CollectionInfo, error) {
	if name == "" {
		return nil, fmt.Errorf("collection name cannot be empty")
	}

	p.mu.RLock()
	if coll, exists := p.collections[name]; exists {
		p.mu.RUnlock()
		return coll, nil
	}
	p.mu.RUnlock()

	start := time.Now()
	p.stats.TotalOperations++

	// Prepare API request
	req := BaseAIRequest{
		Action:     "get_collection",
		Collection: name,
	}

	// Call BaseAI API
	resp, cost, err := p.callAPI(ctx, req)
	if err != nil {
		p.stats.FailedOps++
		p.logger.Error("BaseAI GetCollection failed: %v", err)
		return nil, fmt.Errorf("BaseAI GetCollection failed: %v", err)
	}

	p.costTracker.TotalCost += cost
	p.costTracker.Operations++

	if resp.Success && resp.Data != nil {
		if resultMap, ok := resp.Data.(map[string]interface{}); ok {
			collectionInfo := &CollectionInfo{
				Name:        getStringFromMap(resultMap, "name"),
				VectorCount: int64(getIntFromMap(resultMap, "vector_count")),
				CreatedAt:   getTimeFromMap(resultMap, "created"),
				UpdatedAt:   getTimeFromMap(resultMap, "updated"),
			}

			// Cache locally
			p.mu.Lock()
			p.collections[name] = collectionInfo
			p.mu.Unlock()

			p.stats.SuccessfulOps++
			p.stats.AverageLatency = time.Duration(float64(p.stats.AverageLatency) + float64(time.Since(start))/2)

			return collectionInfo, nil
		}
	}

	p.stats.FailedOps++
	return nil, fmt.Errorf("collection %s not found", name)
}

// CreateIndex creates an index in BaseAI
func (p *BaseAIProvider) CreateIndex(ctx context.Context, collection string, config *IndexConfig) error {
	if collection == "" {
		return fmt.Errorf("collection name cannot be empty")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	start := time.Now()
	p.stats.TotalOperations++

	// Prepare API request
	req := BaseAIRequest{
		Action:     "create_index",
		Collection: collection,
		Options: map[string]interface{}{
			"field":  getStringFromConfig(config, "field"),
			"index":  getStringFromConfig(config, "type"),
			"metric": config.Metric,
		},
	}

	// Call BaseAI API
	resp, cost, err := p.callAPI(ctx, req)
	if err != nil {
		p.stats.FailedOps++
		p.logger.Error("BaseAI CreateIndex failed: %v", err)
		return fmt.Errorf("BaseAI CreateIndex failed: %v", err)
	}

	p.costTracker.TotalCost += cost
	p.costTracker.Operations++

	if resp.Success {
		p.stats.SuccessfulOps++
		p.stats.AverageLatency = time.Duration(float64(p.stats.AverageLatency) + float64(time.Since(start))/2)
		p.logger.Info("Successfully created index in collection %s", collection)
		return nil
	} else {
		p.stats.FailedOps++
		return fmt.Errorf("BaseAI API error: %s", resp.Error)
	}
}

// DeleteIndex deletes an index in BaseAI
func (p *BaseAIProvider) DeleteIndex(ctx context.Context, collection, name string) error {
	if collection == "" || name == "" {
		return fmt.Errorf("collection name and index name cannot be empty")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	start := time.Now()
	p.stats.TotalOperations++

	// Prepare API request
	req := BaseAIRequest{
		Action:     "delete_index",
		Collection: collection,
		Options:    map[string]interface{}{"index": name},
	}

	// Call BaseAI API
	resp, cost, err := p.callAPI(ctx, req)
	if err != nil {
		p.stats.FailedOps++
		p.logger.Error("BaseAI DeleteIndex failed: %v", err)
		return fmt.Errorf("BaseAI DeleteIndex failed: %v", err)
	}

	p.costTracker.TotalCost += cost
	p.costTracker.Operations++

	if resp.Success {
		p.stats.SuccessfulOps++
		p.stats.AverageLatency = time.Duration(float64(p.stats.AverageLatency) + float64(time.Since(start))/2)
		p.logger.Info("Successfully deleted index %s from collection %s", name, collection)
		return nil
	} else {
		p.stats.FailedOps++
		return fmt.Errorf("BaseAI API error: %s", resp.Error)
	}
}

// ListIndexes lists indexes in BaseAI
func (p *BaseAIProvider) ListIndexes(ctx context.Context, collection string) ([]*IndexInfo, error) {
	if collection == "" {
		return []*IndexInfo{}, nil
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	start := time.Now()
	p.stats.TotalOperations++

	// Prepare API request
	req := BaseAIRequest{
		Action:     "list_indexes",
		Collection: collection,
	}

	// Call BaseAI API
	resp, cost, err := p.callAPI(ctx, req)
	if err != nil {
		p.stats.FailedOps++
		p.logger.Error("BaseAI ListIndexes failed: %v", err)
		return []*IndexInfo{}, nil
	}

	p.costTracker.TotalCost += cost
	p.costTracker.Operations++

	// Convert results
	results := []*IndexInfo{}
	if resp.Success && resp.Data != nil {
		if data, ok := resp.Data.([]interface{}); ok {
			for _, item := range data {
				if resultMap, ok := item.(map[string]interface{}); ok {
					indexInfo := &IndexInfo{
						Name:      getStringFromMap(resultMap, "name"),
						Type:      getStringFromMap(resultMap, "type"),
						State:     getStringFromMap(resultMap, "state"),
						CreatedAt: getTimeFromMap(resultMap, "created"),
						UpdatedAt: getTimeFromMap(resultMap, "updated"),
					}
					results = append(results, indexInfo)
				}
			}
		}
	}

	p.stats.SuccessfulOps++
	p.stats.AverageLatency = time.Duration(float64(p.stats.AverageLatency) + float64(time.Since(start))/2)

	return results, nil
}

// AddMetadata adds metadata to a vector in BaseAI
func (p *BaseAIProvider) AddMetadata(ctx context.Context, id string, metadata map[string]interface{}) error {
	if id == "" || len(metadata) == 0 {
		return fmt.Errorf("id and metadata cannot be empty")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	start := time.Now()
	p.stats.TotalOperations++

	// Prepare API request
	req := BaseAIRequest{
		Action:     "add_metadata",
		Collection: "default",
		Options: map[string]interface{}{
			"id":       id,
			"metadata": metadata,
		},
	}

	// Call BaseAI API
	resp, cost, err := p.callAPI(ctx, req)
	if err != nil {
		p.stats.FailedOps++
		p.logger.Error("BaseAI AddMetadata failed: %v", err)
		return fmt.Errorf("BaseAI AddMetadata failed: %v", err)
	}

	p.costTracker.TotalCost += cost
	p.costTracker.Operations++

	if resp.Success {
		p.stats.SuccessfulOps++
		p.stats.AverageLatency = time.Duration(float64(p.stats.AverageLatency) + float64(time.Since(start))/2)
		p.logger.Info("Successfully added metadata to vector %s in BaseAI", id)
		return nil
	} else {
		p.stats.FailedOps++
		return fmt.Errorf("BaseAI API error: %s", resp.Error)
	}
}

// UpdateMetadata updates metadata for a vector in BaseAI
func (p *BaseAIProvider) UpdateMetadata(ctx context.Context, id string, metadata map[string]interface{}) error {
	if id == "" || len(metadata) == 0 {
		return fmt.Errorf("id and metadata cannot be empty")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	start := time.Now()
	p.stats.TotalOperations++

	// Prepare API request
	req := BaseAIRequest{
		Action:     "update_metadata",
		Collection: "default",
		Options: map[string]interface{}{
			"id":       id,
			"metadata": metadata,
		},
	}

	// Call BaseAI API
	resp, cost, err := p.callAPI(ctx, req)
	if err != nil {
		p.stats.FailedOps++
		p.logger.Error("BaseAI UpdateMetadata failed: %v", err)
		return fmt.Errorf("BaseAI UpdateMetadata failed: %v", err)
	}

	p.costTracker.TotalCost += cost
	p.costTracker.Operations++

	if resp.Success {
		p.stats.SuccessfulOps++
		p.stats.AverageLatency = time.Duration(float64(p.stats.AverageLatency) + float64(time.Since(start))/2)
		p.logger.Info("Successfully updated metadata for vector %s in BaseAI", id)
		return nil
	} else {
		p.stats.FailedOps++
		return fmt.Errorf("BaseAI API error: %s", resp.Error)
	}
}

// GetMetadata gets metadata for vectors in BaseAI
func (p *BaseAIProvider) GetMetadata(ctx context.Context, ids []string) (map[string]map[string]interface{}, error) {
	if len(ids) == 0 {
		return map[string]map[string]interface{}{}, nil
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	start := time.Now()
	p.stats.TotalOperations++

	// Prepare API request
	req := BaseAIRequest{
		Action:     "get_metadata",
		Collection: "default",
		Options:    map[string]interface{}{"ids": ids},
	}

	// Call BaseAI API
	resp, cost, err := p.callAPI(ctx, req)
	if err != nil {
		p.stats.FailedOps++
		p.logger.Error("BaseAI GetMetadata failed: %v", err)
		return map[string]map[string]interface{}{}, nil
	}

	p.costTracker.TotalCost += cost
	p.costTracker.Operations++

	// Convert results
	results := map[string]map[string]interface{}{}
	if resp.Success && resp.Data != nil {
		if data, ok := resp.Data.(map[string]interface{}); ok {
			for id, metadata := range data {
				if metadataMap, ok := metadata.(map[string]interface{}); ok {
					results[id] = metadataMap
				}
			}
		}
	}

	p.stats.SuccessfulOps++
	p.stats.AverageLatency = time.Duration(float64(p.stats.AverageLatency) + float64(time.Since(start))/2)

	return results, nil
}

// DeleteMetadata deletes metadata from vectors in BaseAI
func (p *BaseAIProvider) DeleteMetadata(ctx context.Context, ids []string, keys []string) error {
	if len(ids) == 0 || len(keys) == 0 {
		return nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	start := time.Now()
	p.stats.TotalOperations++

	// Prepare API request
	req := BaseAIRequest{
		Action:     "delete_metadata",
		Collection: "default",
		Options: map[string]interface{}{
			"ids":  ids,
			"keys": keys,
		},
	}

	// Call BaseAI API
	resp, cost, err := p.callAPI(ctx, req)
	if err != nil {
		p.stats.FailedOps++
		p.logger.Error("BaseAI DeleteMetadata failed: %v", err)
		return fmt.Errorf("BaseAI DeleteMetadata failed: %v", err)
	}

	p.costTracker.TotalCost += cost
	p.costTracker.Operations++

	if resp.Success {
		p.stats.SuccessfulOps++
		p.stats.AverageLatency = time.Duration(float64(p.stats.AverageLatency) + float64(time.Since(start))/2)
		p.logger.Info("Successfully deleted metadata from %d vectors in BaseAI", len(ids))
		return nil
	} else {
		p.stats.FailedOps++
		return fmt.Errorf("BaseAI API error: %s", resp.Error)
	}
}

// GetStats returns provider statistics
func (p *BaseAIProvider) GetStats(ctx context.Context) (*ProviderStats, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Copy current stats
	stats := *p.stats
	stats.LastHealthCheck = time.Now()

	return &stats, nil
}

// Optimize optimizes the BaseAI provider
func (p *BaseAIProvider) Optimize(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	start := time.Now()
	p.stats.TotalOperations++

	// Prepare API request
	req := BaseAIRequest{
		Action: "optimize",
		Options: map[string]interface{}{
			"cleanup": true,
			"compact": true,
		},
	}

	// Call BaseAI API
	resp, cost, err := p.callAPI(ctx, req)
	if err != nil {
		p.stats.FailedOps++
		p.logger.Error("BaseAI Optimize failed: %v", err)
		return fmt.Errorf("BaseAI Optimize failed: %v", err)
	}

	p.costTracker.TotalCost += cost
	p.costTracker.Operations++

	if resp.Success {
		p.stats.SuccessfulOps++
		p.stats.AverageLatency = time.Duration(float64(p.stats.AverageLatency) + float64(time.Since(start))/2)
		p.logger.Info("BaseAI optimization completed successfully")
		return nil
	} else {
		p.stats.FailedOps++
		return fmt.Errorf("BaseAI API error: %s", resp.Error)
	}
}

// Backup backs up data in BaseAI
func (p *BaseAIProvider) Backup(ctx context.Context, path string) error {
	if path == "" {
		return fmt.Errorf("backup path cannot be empty")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	start := time.Now()
	p.stats.TotalOperations++

	// Prepare API request
	req := BaseAIRequest{
		Action: "backup",
		Options: map[string]interface{}{
			"path": path,
		},
	}

	// Call BaseAI API
	resp, cost, err := p.callAPI(ctx, req)
	if err != nil {
		p.stats.FailedOps++
		p.logger.Error("BaseAI Backup failed: %v", err)
		return fmt.Errorf("BaseAI Backup failed: %v", err)
	}

	p.costTracker.TotalCost += cost
	p.costTracker.Operations++

	if resp.Success {
		p.stats.SuccessfulOps++
		p.stats.AverageLatency = time.Duration(float64(p.stats.AverageLatency) + float64(time.Since(start))/2)
		p.logger.Info("BaseAI backup completed successfully to %s", path)
		return nil
	} else {
		p.stats.FailedOps++
		return fmt.Errorf("BaseAI API error: %s", resp.Error)
	}
}

// Restore restores data in BaseAI
func (p *BaseAIProvider) Restore(ctx context.Context, path string) error {
	if path == "" {
		return fmt.Errorf("restore path cannot be empty")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	start := time.Now()
	p.stats.TotalOperations++

	// Prepare API request
	req := BaseAIRequest{
		Action: "restore",
		Options: map[string]interface{}{
			"path": path,
		},
	}

	// Call BaseAI API
	resp, cost, err := p.callAPI(ctx, req)
	if err != nil {
		p.stats.FailedOps++
		p.logger.Error("BaseAI Restore failed: %v", err)
		return fmt.Errorf("BaseAI Restore failed: %v", err)
	}

	p.costTracker.TotalCost += cost
	p.costTracker.Operations++

	if resp.Success {
		p.stats.SuccessfulOps++
		p.stats.AverageLatency = time.Duration(float64(p.stats.AverageLatency) + float64(time.Since(start))/2)
		p.logger.Info("BaseAI restore completed successfully from %s", path)
		return nil
	} else {
		p.stats.FailedOps++
		return fmt.Errorf("BaseAI API error: %s", resp.Error)
	}
}

// Initialize initializes the BaseAI provider
func (p *BaseAIProvider) Initialize(ctx context.Context, config interface{}) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Already initialized in NewBaseAIProvider
	p.stats.Status = "initialized"
	p.logger.Info("BaseAI provider initialized")
	return nil
}

// Start starts the BaseAI provider
func (p *BaseAIProvider) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Test connection to BaseAI API
	testReq := BaseAIRequest{
		Action: "health",
	}

	_, cost, err := p.callAPI(ctx, testReq)
	if err != nil {
		p.stats.Status = "error"
		p.logger.Error("BaseAI connection test failed: %v", err)
		return fmt.Errorf("BaseAI connection test failed: %v", err)
	}

	p.costTracker.TotalCost += cost
	p.costTracker.Operations++
	p.stats.Status = "active"
	p.logger.Info("BaseAI provider started successfully")

	return nil
}

// Stop stops the BaseAI provider
func (p *BaseAIProvider) Stop(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.stats.Status = "stopped"
	p.logger.Info("BaseAI provider stopped")
	return nil
}

// Health checks provider health
func (p *BaseAIProvider) Health(ctx context.Context) (*HealthStatus, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	start := time.Now()
	p.lastHealthCheck = time.Now()

	// Check connection to BaseAI API
	testReq := BaseAIRequest{
		Action: "health",
	}

	_, cost, err := p.callAPI(ctx, testReq)
	if err != nil {
		p.stats.Status = "unhealthy"
		return &HealthStatus{
			Status:    "unhealthy",
			Message:   fmt.Sprintf("BaseAI health check failed: %v", err),
			Timestamp: time.Now(),
		}, nil
	}

	p.costTracker.TotalCost += cost
	p.costTracker.Operations++
	p.stats.Status = "healthy"

	return &HealthStatus{
		Status:       "healthy",
		Message:      "BaseAI API is responding",
		Timestamp:    time.Now(),
		LastCheck:    time.Now(),
		ResponseTime: time.Since(start),
		Metrics: map[string]interface{}{
			"total_requests":    p.costTracker.Operations,
			"total_cost":        p.costTracker.TotalCost,
			"collections_count": len(p.collections),
			"vectors_count":     p.stats.TotalVectors,
		},
	}, nil
}

// Close closes the provider
func (p *BaseAIProvider) Close(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.stats.Status = "closed"
	p.logger.Info("BaseAI provider closed")
	return nil
}

// GetCostInfo returns cost information for BaseAI
func (p *BaseAIProvider) GetCostInfo() *CostInfo {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return &CostInfo{
		Currency:      "USD",
		ComputeCost:   p.costTracker.TotalCost,
		TransferCost:  0.0,
		StorageCost:   0.0,
		TotalCost:     p.costTracker.TotalCost,
		BillingPeriod: "monthly",
		FreeTierUsed:  0.0,
		FreeTierLimit: 0.0,
		Details: map[string]float64{
			"request_count": float64(p.costTracker.Operations),
		},
	}
}

// callAPI makes an API call to BaseAI
func (p *BaseAIProvider) callAPI(ctx context.Context, req BaseAIRequest) (*BaseAIResponse, float64, error) {
	start := time.Now()

	// Marshal request
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, 0.0, fmt.Errorf("failed to marshal request: %v", err)
	}

	// Create HTTP request
	url := fmt.Sprintf("%s/memory", p.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, 0.0, fmt.Errorf("failed to create request: %v", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	httpReq.Header.Set("User-Agent", "HelixCode/1.0")

	// Make request
	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, 0.0, fmt.Errorf("API request failed: %v", err)
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0.0, fmt.Errorf("failed to read response: %v", err)
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, 0.0, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var apiResp BaseAIResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, 0.0, fmt.Errorf("failed to parse response: %v", err)
	}

	// Calculate cost (BaseAI typically charges per request and tokens)
	cost := float64(0.001) // Base cost per request
	if apiResp.Meta != nil {
		cost = apiResp.Meta.Cost
	}

	p.logger.Debug("BaseAI API call completed in %v, cost: $%.6f", time.Since(start), cost)

	return &apiResp, cost, nil
}

// Helper functions for type conversion
func getStringFromMetadata(metadata map[string]interface{}, key string) string {
	if val, ok := metadata[key].(string); ok {
		return val
	}
	return ""
}

// getStringFromConfig safely extracts a string value from config
func getStringFromConfig(config *IndexConfig, key string) string {
	if config == nil {
		return ""
	}

	switch key {
	case "field":
		// For BaseAI, field is typically "vector" by default
		if config.Parameters != nil {
			if val, ok := config.Parameters["field"].(string); ok {
				return val
			}
		}
		return "vector"
	case "type":
		return config.Type
	default:
		if config.Parameters != nil {
			if val, ok := config.Parameters[key].(string); ok {
				return val
			}
		}
	}
	return ""
}

func getStringFromMap(m map[string]interface{}, key string) string {
	if val, ok := m[key].(string); ok {
		return val
	}
	return ""
}

func getFloatFromMap(m map[string]interface{}, key string) float64 {
	if val, ok := m[key].(float64); ok {
		return val
	}
	return 0.0
}

func getIntFromMap(m map[string]interface{}, key string) int {
	if val, ok := m[key].(float64); ok {
		return int(val)
	}
	return 0
}

func getFloatSliceFromMap(m map[string]interface{}, key string) []float64 {
	if val, ok := m[key].([]interface{}); ok {
		result := make([]float64, len(val))
		for i, v := range val {
			if f, ok := v.(float64); ok {
				result[i] = f
			}
		}
		return result
	}
	return []float64{}
}

func getMapFromMap(m map[string]interface{}, key string) map[string]interface{} {
	if val, ok := m[key].(map[string]interface{}); ok {
		return val
	}
	return map[string]interface{}{}
}

func getTimeFromMap(m map[string]interface{}, key string) time.Time {
	if val, ok := m[key].(string); ok {
		if t, err := time.Parse(time.RFC3339, val); err == nil {
			return t
		}
	}
	return time.Time{}
}
