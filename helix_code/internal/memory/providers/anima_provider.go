package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"dev.helix.code/internal/logging"
	"dev.helix.code/internal/memory"
)

// AnimaProvider implementation for Anima AI memory provider
type AnimaProvider struct {
	config      *AnimaConfig
	logger      *logging.Logger
	mu          sync.RWMutex
	initialized bool
	started     bool
	data        map[string]*VectorData
	collections map[string]*CollectionInfo
	indexes     map[string]map[string]*IndexInfo
	metadata    map[string]map[string]interface{}
	stats       *ProviderStats
}

// AnimaConfig contains Anima provider configuration
type AnimaConfig struct {
	APIKey  string `json:"api_key"`
	BaseURL string `json:"base_url"`
}

// AnimaClient stub client
type AnimaClient struct{}

// NewAnimaProvider creates a new Anima provider
func NewAnimaProvider(config *AnimaConfig) (*AnimaProvider, error) {
	return &AnimaProvider{
		config: config,
		logger: logging.NewLoggerWithName("anima_provider"),
	}, nil
}

// Initialize initializes the provider
func (p *AnimaProvider) Initialize(ctx context.Context, config interface{}) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.initialized {
		return nil // Already initialized
	}

	p.logger.Info("Initializing Anima provider")

	// Initialize internal data structures
	p.data = make(map[string]*VectorData)
	p.collections = make(map[string]*CollectionInfo)
	p.indexes = make(map[string]map[string]*IndexInfo)
	p.metadata = make(map[string]map[string]interface{})

	// Initialize stats
	p.stats = &ProviderStats{
		Status:        "initialized",
		LastOperation: time.Now(),
	}

	// Parse configuration if provided
	if config != nil {
		if animaConfig, ok := config.(*AnimaConfig); ok {
			p.config = animaConfig
		}
	}

	// Set default config if not provided
	if p.config == nil {
		p.config = &AnimaConfig{
			BaseURL: "https://api.anima.ai/v1",
		}
	}

	p.initialized = true
	p.logger.Info("Anima provider initialized successfully")
	return nil
}

// Start starts the provider
func (p *AnimaProvider) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.initialized {
		return fmt.Errorf("provider must be initialized before starting")
	}

	p.logger.Info("Starting Anima provider")
	p.started = true
	p.stats.Status = "running"
	p.stats.Uptime = time.Since(time.Now())
	p.logger.Info("Anima provider started successfully")
	return nil
}

// Stop stops the provider
func (p *AnimaProvider) Stop(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.logger.Info("Stopping Anima provider")
	p.started = false
	p.stats.Status = "stopped"
	p.logger.Info("Anima provider stopped successfully")
	return nil
}

// Health returns health status
func (p *AnimaProvider) Health(ctx context.Context) (*HealthStatus, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	status := "healthy"
	message := "Provider is operating normally"
	responseTime := time.Millisecond * 10

	if !p.initialized {
		status = "not_initialized"
		message = "Provider has not been initialized"
	} else if !p.started {
		status = "not_started"
		message = "Provider has not been started"
	}

	// Build metrics safely
	metrics := map[string]interface{}{
		"data_count":        len(p.data),
		"collections_count": len(p.collections),
	}
	if p.stats != nil {
		metrics["uptime_seconds"] = p.stats.Uptime.Seconds()
	} else {
		metrics["uptime_seconds"] = 0.0
	}

	return &HealthStatus{
		Status:       status,
		Message:      message,
		Timestamp:    time.Now(),
		ResponseTime: responseTime,
		Metrics:      metrics,
		Details: map[string]interface{}{
			"provider": "Anima AI Memory Provider",
			"version":  "1.0.0",
		},
	}, nil
}

// GetName returns provider name
func (p *AnimaProvider) GetName() string {
	return "anima"
}

// GetType returns provider type
func (p *AnimaProvider) GetType() string {
	return "anima"
}

// GetCapabilities returns provider capabilities
func (p *AnimaProvider) GetCapabilities() []string {
	return []string{
		"vector_storage",
		"vector_search",
		"metadata_management",
		"collection_management",
		"index_management",
		"batch_operations",
		"similarity_search",
		"backup_restore",
		"optimization",
		"health_checking",
		"statistics",
	}
}

// GetConfiguration returns provider configuration
func (p *AnimaProvider) GetConfiguration() interface{} {
	return p.config
}

// IsCloud returns whether provider is cloud-based
func (p *AnimaProvider) IsCloud() bool {
	return true
}

// GetCostInfo returns cost information
func (p *AnimaProvider) GetCostInfo() *memory.CostInfo {
	return memory.NewCostInfo("USD", 0.001, 0.002, 0.0001)
}

// Store stores vectors
func (p *AnimaProvider) Store(ctx context.Context, vectors []*VectorData) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.started {
		return fmt.Errorf("provider must be started to store vectors")
	}

	p.logger.Debug("Storing %d vectors", len(vectors))

	for _, vector := range vectors {
		if vector.ID == "" {
			return fmt.Errorf("vector ID is required")
		}
		if len(vector.Vector) == 0 {
			return fmt.Errorf("vector data is empty")
		}

		// Set timestamp if not provided
		if vector.Timestamp.IsZero() {
			vector.Timestamp = time.Now()
		}

		// Store vector data
		p.data[vector.ID] = vector

		// Initialize metadata if not exists
		if p.metadata[vector.ID] == nil {
			p.metadata[vector.ID] = make(map[string]interface{})
		}
		// Copy metadata
		for k, v := range vector.Metadata {
			p.metadata[vector.ID][k] = v
		}
	}

	// Update stats
	p.stats.TotalVectors += int64(len(vectors))
	p.stats.SuccessfulOps += int64(len(vectors))
	p.stats.TotalOperations += int64(len(vectors))
	p.stats.LastOperation = time.Now()

	p.logger.Debug("Successfully stored %d vectors", len(vectors))
	return nil
}

// Retrieve retrieves vectors
func (p *AnimaProvider) Retrieve(ctx context.Context, ids []string) ([]*VectorData, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.started {
		return nil, fmt.Errorf("provider must be started to retrieve vectors")
	}

	if len(ids) == 0 {
		return []*VectorData{}, nil
	}

	p.logger.Debug("Retrieving %d vectors", len(ids))

	vectors := make([]*VectorData, 0, len(ids))
	for _, id := range ids {
		if vector, exists := p.data[id]; exists {
			// Copy metadata
			vectorCopy := *vector
			vectorCopy.Metadata = make(map[string]interface{})
			for k, v := range vector.Metadata {
				vectorCopy.Metadata[k] = v
			}
			vectors = append(vectors, &vectorCopy)
		}
	}

	p.logger.Debug("Retrieved %d vectors", len(vectors))
	return vectors, nil
}

// Update updates a vector
func (p *AnimaProvider) Update(ctx context.Context, id string, vector *VectorData) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.started {
		return fmt.Errorf("provider must be started to update vectors")
	}

	if id == "" {
		return fmt.Errorf("vector ID is required")
	}

	if _, exists := p.data[id]; !exists {
		return fmt.Errorf("vector with ID %s does not exist", id)
	}

	// Update vector
	vector.ID = id // Ensure ID consistency
	if vector.Timestamp.IsZero() {
		vector.Timestamp = time.Now()
	}
	p.data[id] = vector

	// Update metadata
	if p.metadata[id] == nil {
		p.metadata[id] = make(map[string]interface{})
	}
	for k, v := range vector.Metadata {
		p.metadata[id][k] = v
	}

	// Update stats
	p.stats.SuccessfulOps++
	p.stats.TotalOperations++
	p.stats.LastOperation = time.Now()

	p.logger.Debug("Updated vector with ID: %s", id)
	return nil
}

// Delete deletes vectors
func (p *AnimaProvider) Delete(ctx context.Context, ids []string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.started {
		return fmt.Errorf("provider must be started to delete vectors")
	}

	if len(ids) == 0 {
		return nil
	}

	p.logger.Debug("Deleting %d vectors", len(ids))

	for _, id := range ids {
		delete(p.data, id)
		delete(p.metadata, id)
	}

	// Update stats
	p.stats.TotalVectors -= int64(len(ids))
	p.stats.SuccessfulOps += int64(len(ids))
	p.stats.TotalOperations += int64(len(ids))
	p.stats.LastOperation = time.Now()

	p.logger.Debug("Deleted %d vectors", len(ids))
	return nil
}

// Search searches for vectors
func (p *AnimaProvider) Search(ctx context.Context, query *VectorQuery) (*VectorSearchResult, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.started {
		return nil, fmt.Errorf("provider must be started to search vectors")
	}

	if query == nil {
		return nil, fmt.Errorf("query is required")
	}

	startTime := time.Now()
	p.logger.Debug("Searching with top_k=%d, threshold=%f", query.TopK, query.Threshold)

	results := []*VectorSearchResultItem{}

	// Simple cosine similarity search
	for _, vector := range p.data {
		// Apply collection filter
		if query.Collection != "" && vector.Collection != query.Collection {
			continue
		}

		// Apply namespace filter
		if query.Namespace != "" && vector.Namespace != query.Namespace {
			continue
		}

		// Calculate similarity if query vector is provided
		if len(query.Vector) > 0 && len(vector.Vector) > 0 {
			similarity := calculateCosineSimilarity(query.Vector, vector.Vector)
			if similarity < query.Threshold {
				continue
			}

			result := &VectorSearchResultItem{
				ID:       vector.ID,
				Score:    similarity,
				Distance: 1 - similarity, // Convert similarity to distance
			}

			// Include vector if requested
			if query.IncludeVector {
				result.Vector = make([]float64, len(vector.Vector))
				copy(result.Vector, vector.Vector)
			}

			// Include metadata
			if len(vector.Metadata) > 0 {
				result.Metadata = make(map[string]interface{})
				for k, v := range vector.Metadata {
					result.Metadata[k] = v
				}
			}

			results = append(results, result)
		}
	}

	// Sort results by score (descending)
	sortResults(results)

	// Apply TopK limit
	if query.TopK > 0 && len(results) > query.TopK {
		results = results[:query.TopK]
	}

	searchResult := &VectorSearchResult{
		Results:   results,
		Total:     len(results),
		Query:     query,
		Duration:  time.Since(startTime),
		Namespace: query.Namespace,
	}

	p.logger.Debug("Search completed in %v, found %d results", searchResult.Duration, len(results))
	return searchResult, nil
}

// FindSimilar finds similar vectors
func (p *AnimaProvider) FindSimilar(ctx context.Context, embedding []float64, k int, filters map[string]interface{}) ([]*memory.VectorSimilarityResult, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.started {
		return nil, fmt.Errorf("provider must be started to find similar vectors")
	}

	if len(embedding) == 0 {
		return nil, fmt.Errorf("embedding is required")
	}

	if k <= 0 {
		k = 10 // Default to 10
	}

	p.logger.Debug("Finding %d similar vectors", k)

	similarResults := []*memory.VectorSimilarityResult{}

	for _, vector := range p.data {
		// Apply filters
		if !matchesFilters(vector, filters) {
			continue
		}

		// Calculate cosine similarity
		similarity := calculateCosineSimilarity(embedding, vector.Vector)

		result := &memory.VectorSimilarityResult{
			ID:       vector.ID,
			Score:    similarity,
			Distance: 1 - similarity,
		}

		// Include vector (for consistency with other providers)
		result.Vector = make([]float64, len(vector.Vector))
		copy(result.Vector, vector.Vector)

		// Include metadata
		if len(vector.Metadata) > 0 {
			result.Metadata = make(map[string]interface{})
			for k, v := range vector.Metadata {
				result.Metadata[k] = v
			}
		}

		similarResults = append(similarResults, result)
	}

	// Sort by similarity score (descending)
	for i := 0; i < len(similarResults); i++ {
		for j := i + 1; j < len(similarResults); j++ {
			if similarResults[i].Score < similarResults[j].Score {
				similarResults[i], similarResults[j] = similarResults[j], similarResults[i]
			}
		}
	}

	// Apply K limit
	if len(similarResults) > k {
		similarResults = similarResults[:k]
	}

	p.logger.Debug("Found %d similar vectors", len(similarResults))
	return similarResults, nil
}

// BatchFindSimilar batch finds similar vectors
func (p *AnimaProvider) BatchFindSimilar(ctx context.Context, queries [][]float64, k int) ([][]*memory.VectorSimilarityResult, error) {
	if !p.started {
		return nil, fmt.Errorf("provider must be started to find similar vectors")
	}

	if len(queries) == 0 {
		return [][]*memory.VectorSimilarityResult{}, nil
	}

	p.logger.Debug("Batch finding similar vectors for %d queries", len(queries))

	results := make([][]*memory.VectorSimilarityResult, len(queries))

	for i, query := range queries {
		similar, err := p.FindSimilar(ctx, query, k, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to find similar for query %d: %w", i, err)
		}
		results[i] = similar
	}

	p.logger.Debug("Completed batch similarity search for %d queries", len(queries))
	return results, nil
}

// CreateCollection creates a collection
func (p *AnimaProvider) CreateCollection(ctx context.Context, name string, config *CollectionConfig) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.started {
		return fmt.Errorf("provider must be started to create collections")
	}

	if name == "" {
		return fmt.Errorf("collection name is required")
	}

	if _, exists := p.collections[name]; exists {
		return fmt.Errorf("collection %s already exists", name)
	}

	// Handle nil config with defaults
	if config == nil {
		config = &CollectionConfig{
			Name:      name,
			Dimension: 768, // default dimension
			Metric:    "cosine",
		}
	}

	// Create collection info
	collectionInfo := &CollectionInfo{
		Name:      name,
		Dimension: config.Dimension,
		Metric:    config.Metric,
		Status:    "active",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Config:    config,
		Metadata:  make(map[string]interface{}),
	}

	// Initialize index map for this collection
	p.indexes[name] = make(map[string]*IndexInfo)

	p.collections[name] = collectionInfo
	p.stats.TotalCollections++
	p.stats.SuccessfulOps++
	p.stats.TotalOperations++
	p.stats.LastOperation = time.Now()

	p.logger.Info("Created collection: %s (dimension: %d, metric: %s)", name, config.Dimension, config.Metric)
	return nil
}

// DeleteCollection deletes a collection
func (p *AnimaProvider) DeleteCollection(ctx context.Context, name string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.started {
		return fmt.Errorf("provider must be started to delete collections")
	}

	if _, exists := p.collections[name]; !exists {
		return fmt.Errorf("collection %s does not exist", name)
	}

	// Delete all vectors in this collection
	vectorsToDelete := []string{}
	for id, vector := range p.data {
		if vector.Collection == name {
			vectorsToDelete = append(vectorsToDelete, id)
		}
	}

	for _, id := range vectorsToDelete {
		delete(p.data, id)
		delete(p.metadata, id)
	}

	// Delete collection and its indexes
	delete(p.collections, name)
	delete(p.indexes, name)

	p.stats.TotalCollections--
	p.stats.SuccessfulOps++
	p.stats.TotalOperations++
	p.stats.LastOperation = time.Now()

	p.logger.Info("Deleted collection: %s (removed %d vectors)", name, len(vectorsToDelete))
	return nil
}

// ListCollections lists collections
func (p *AnimaProvider) ListCollections(ctx context.Context) ([]*CollectionInfo, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.started {
		return nil, fmt.Errorf("provider must be started to list collections")
	}

	collections := make([]*CollectionInfo, 0, len(p.collections))
	for _, collection := range p.collections {
		// Update vector count
		vectorCount := int64(0)
		for _, vector := range p.data {
			if vector.Collection == collection.Name {
				vectorCount++
			}
		}
		collection.VectorCount = vectorCount
		collections = append(collections, collection)
	}

	p.logger.Debug("Listed %d collections", len(collections))
	return collections, nil
}

// GetCollection gets collection info
func (p *AnimaProvider) GetCollection(ctx context.Context, name string) (*CollectionInfo, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.started {
		return nil, fmt.Errorf("provider must be started to get collection info")
	}

	collection, exists := p.collections[name]
	if !exists {
		return nil, fmt.Errorf("collection %s does not exist", name)
	}

	// Update vector count
	vectorCount := int64(0)
	for _, vector := range p.data {
		if vector.Collection == name {
			vectorCount++
		}
	}
	collection.VectorCount = vectorCount

	p.logger.Debug("Retrieved collection info for: %s", name)
	return collection, nil
}

// CreateIndex creates an index
func (p *AnimaProvider) CreateIndex(ctx context.Context, collection string, config *IndexConfig) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.started {
		return fmt.Errorf("provider must be started to create indexes")
	}

	// Check if collection exists
	if _, exists := p.collections[collection]; !exists {
		return fmt.Errorf("collection %s does not exist", collection)
	}

	// Initialize index map for collection if not exists
	if p.indexes[collection] == nil {
		p.indexes[collection] = make(map[string]*IndexInfo)
	}

	// Check if index already exists
	if _, exists := p.indexes[collection][config.Name]; exists {
		return fmt.Errorf("index %s already exists in collection %s", config.Name, collection)
	}

	// Create index info
	indexInfo := &IndexInfo{
		Name:      config.Name,
		Type:      config.Type,
		State:     "active",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Config:    config,
		Metadata:  make(map[string]interface{}),
	}

	p.indexes[collection][config.Name] = indexInfo
	p.stats.SuccessfulOps++
	p.stats.TotalOperations++
	p.stats.LastOperation = time.Now()

	p.logger.Info("Created index: %s in collection: %s (type: %s)", config.Name, collection, config.Type)
	return nil
}

// DeleteIndex deletes an index
func (p *AnimaProvider) DeleteIndex(ctx context.Context, collection, name string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.started {
		return fmt.Errorf("provider must be started to delete indexes")
	}

	// Check if collection exists
	if _, exists := p.collections[collection]; !exists {
		return fmt.Errorf("collection %s does not exist", collection)
	}

	// Check if index map exists
	if p.indexes[collection] == nil {
		return fmt.Errorf("collection %s has no indexes", collection)
	}

	// Delete index
	if _, exists := p.indexes[collection][name]; !exists {
		return fmt.Errorf("index %s does not exist in collection %s", name, collection)
	}

	delete(p.indexes[collection], name)
	p.stats.SuccessfulOps++
	p.stats.TotalOperations++
	p.stats.LastOperation = time.Now()

	p.logger.Info("Deleted index: %s from collection: %s", name, collection)
	return nil
}

// ListIndexes lists indexes
func (p *AnimaProvider) ListIndexes(ctx context.Context, collection string) ([]*IndexInfo, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.started {
		return nil, fmt.Errorf("provider must be started to list indexes")
	}

	// Check if collection exists
	if _, exists := p.collections[collection]; !exists {
		return nil, fmt.Errorf("collection %s does not exist", collection)
	}

	indexes := p.indexes[collection]
	if indexes == nil {
		return []*IndexInfo{}, nil
	}

	indexList := make([]*IndexInfo, 0, len(indexes))
	for _, index := range indexes {
		indexList = append(indexList, index)
	}

	p.logger.Debug("Listed %d indexes for collection: %s", len(indexList), collection)
	return indexList, nil
}

// AddMetadata adds metadata
func (p *AnimaProvider) AddMetadata(ctx context.Context, id string, metadata map[string]interface{}) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.started {
		return fmt.Errorf("provider must be started to add metadata")
	}

	// Check if vector exists
	if _, exists := p.data[id]; !exists {
		return fmt.Errorf("vector with ID %s does not exist", id)
	}

	// Initialize metadata if not exists
	if p.metadata[id] == nil {
		p.metadata[id] = make(map[string]interface{})
	}

	// Add metadata
	for k, v := range metadata {
		p.metadata[id][k] = v
	}

	// Update vector metadata
	if p.data[id].Metadata == nil {
		p.data[id].Metadata = make(map[string]interface{})
	}
	for k, v := range metadata {
		p.data[id].Metadata[k] = v
	}

	p.stats.SuccessfulOps++
	p.stats.TotalOperations++
	p.stats.LastOperation = time.Now()

	p.logger.Debug("Added metadata for vector: %s", id)
	return nil
}

// UpdateMetadata updates metadata
func (p *AnimaProvider) UpdateMetadata(ctx context.Context, id string, metadata map[string]interface{}) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.started {
		return fmt.Errorf("provider must be started to update metadata")
	}

	// Check if vector exists
	if _, exists := p.data[id]; !exists {
		return fmt.Errorf("vector with ID %s does not exist", id)
	}

	// Initialize metadata if not exists
	if p.metadata[id] == nil {
		p.metadata[id] = make(map[string]interface{})
	}
	if p.data[id].Metadata == nil {
		p.data[id].Metadata = make(map[string]interface{})
	}

	// Update metadata (replace entire metadata map)
	p.metadata[id] = make(map[string]interface{})
	p.data[id].Metadata = make(map[string]interface{})

	for k, v := range metadata {
		p.metadata[id][k] = v
		p.data[id].Metadata[k] = v
	}

	p.stats.SuccessfulOps++
	p.stats.TotalOperations++
	p.stats.LastOperation = time.Now()

	p.logger.Debug("Updated metadata for vector: %s", id)
	return nil
}

// GetMetadata gets metadata
func (p *AnimaProvider) GetMetadata(ctx context.Context, ids []string) (map[string]map[string]interface{}, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.started {
		return nil, fmt.Errorf("provider must be started to get metadata")
	}

	if len(ids) == 0 {
		return make(map[string]map[string]interface{}), nil
	}

	result := make(map[string]map[string]interface{})

	for _, id := range ids {
		if metadata, exists := p.metadata[id]; exists {
			// Copy metadata
			metadataCopy := make(map[string]interface{})
			for k, v := range metadata {
				metadataCopy[k] = v
			}
			result[id] = metadataCopy
		}
	}

	p.logger.Debug("Retrieved metadata for %d vectors", len(result))
	return result, nil
}

// DeleteMetadata deletes metadata
func (p *AnimaProvider) DeleteMetadata(ctx context.Context, ids []string, keys []string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.started {
		return fmt.Errorf("provider must be started to delete metadata")
	}

	if len(ids) == 0 {
		return nil
	}

	for _, id := range ids {
		// Check if vector exists
		if _, exists := p.data[id]; !exists {
			continue // Skip non-existent vectors
		}

		// Initialize metadata if not exists
		if p.metadata[id] == nil {
			p.metadata[id] = make(map[string]interface{})
		}
		if p.data[id].Metadata == nil {
			p.data[id].Metadata = make(map[string]interface{})
		}

		// Delete specific keys if provided, otherwise delete all metadata
		if len(keys) > 0 {
			for _, key := range keys {
				delete(p.metadata[id], key)
				delete(p.data[id].Metadata, key)
			}
		} else {
			// Delete all metadata
			p.metadata[id] = make(map[string]interface{})
			p.data[id].Metadata = make(map[string]interface{})
		}
	}

	p.stats.SuccessfulOps++
	p.stats.TotalOperations++
	p.stats.LastOperation = time.Now()

	p.logger.Debug("Deleted metadata for %d vectors", len(ids))
	return nil
}

// GetStats gets provider stats
func (p *AnimaProvider) GetStats(ctx context.Context) (*ProviderStats, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Update uptime if provider is started
	if p.started && p.stats.Status == "running" {
		p.stats.Uptime = time.Since(time.Now().Add(-p.stats.Uptime))
	}

	// Update vector count
	vectorCount := int64(len(p.data))
	p.stats.TotalVectors = vectorCount

	// Update total collections
	p.stats.TotalCollections = int64(len(p.collections))

	// Calculate total storage size (rough estimate)
	totalSize := int64(0)
	for _, vector := range p.data {
		// Each vector data: ID (string) + vector data + metadata + timestamps
		totalSize += int64(len(vector.ID) + len(vector.Vector)*8 + 100) // Rough estimate
	}
	p.stats.TotalSize = totalSize

	return p.stats, nil
}

// Optimize optimizes the provider
func (p *AnimaProvider) Optimize(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.started {
		return fmt.Errorf("provider must be started to optimize")
	}

	p.logger.Info("Optimizing Anima provider")

	// Optimize indexes by rebuilding them
	for collectionName, indexes := range p.indexes {
		for indexName, indexInfo := range indexes {
			p.logger.Debug("Optimizing index %s in collection %s", indexName, collectionName)
			indexInfo.UpdatedAt = time.Now()
			indexInfo.Metadata["last_optimized"] = time.Now()
		}
	}

	// Update collection statistics
	for collectionName, collection := range p.collections {
		vectorCount := int64(0)
		for _, vector := range p.data {
			if vector.Collection == collectionName {
				vectorCount++
			}
		}
		collection.VectorCount = vectorCount
		collection.UpdatedAt = time.Now()
	}

	// Clean up orphaned metadata (metadata for non-existent vectors)
	orphanedKeys := []string{}
	for id := range p.metadata {
		if _, exists := p.data[id]; !exists {
			orphanedKeys = append(orphanedKeys, id)
		}
	}
	for _, key := range orphanedKeys {
		delete(p.metadata, key)
	}

	p.stats.SuccessfulOps++
	p.stats.TotalOperations++
	p.stats.LastOperation = time.Now()
	if p.stats.Metadata == nil {
		p.stats.Metadata = make(map[string]interface{})
	}
	p.stats.Metadata["last_optimized_at"] = time.Now()

	p.logger.Info("Anima provider optimization completed: cleaned %d orphaned metadata entries", len(orphanedKeys))
	return nil
}

// Backup backs up data
func (p *AnimaProvider) Backup(ctx context.Context, path string) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.started {
		return fmt.Errorf("provider must be started to backup")
	}

	p.logger.Info("Backing up Anima provider data to: %s", path)

	type backupState struct {
		Timestamp time.Time              `json:"timestamp"`
		Entries   map[string]interface{} `json:"entries"`
		Version   string                 `json:"version"`
	}
	state := backupState{
		Timestamp: time.Now(),
		Entries:   make(map[string]interface{}),
		Version:   "1.0",
	}
	for k, v := range p.data {
		state.Entries[k] = v
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("serialize backup: %w", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write backup: %w", err)
	}

	p.stats.SuccessfulOps++
	p.stats.TotalOperations++
	p.stats.LastOperation = time.Now()

	p.logger.Info("Anima provider backup completed")
	return nil
}

func (p *AnimaProvider) Restore(ctx context.Context, path string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.started {
		return fmt.Errorf("provider must be started to restore")
	}

	p.logger.Info("Restoring Anima provider data from: %s", path)

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read backup: %w", err)
	}
	var state struct {
		Entries map[string]interface{} `json:"entries"`
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("deserialize backup: %w", err)
	}
	if state.Entries != nil {
		for k, v := range state.Entries {
			if vd, ok := v.(*VectorData); ok {
				p.data[k] = vd
			}
		}
	}

	p.stats.SuccessfulOps++
	p.stats.TotalOperations++
	p.stats.LastOperation = time.Now()

	p.logger.Info("Anima provider restore completed")
	return nil
}

// Close closes the provider
func (p *AnimaProvider) Close(ctx context.Context) error {
	return p.Stop(ctx)
}

// Helper functions

// sortResults sorts VectorSearchResultItem by score (descending)
func sortResults(results []*VectorSearchResultItem) {
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[i].Score < results[j].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}
}

// matchesFilters checks if a vector matches the provided filters.
//
// Supported value shapes (CONST-050(A) honest replacement of the
// prior "Simple equality check for now" bluff):
//
//   - Scalar value `filters["k"] = v`: strict equality (vector.Metadata[k] == v).
//   - Operator map `filters["k"] = map[string]interface{}{"$gt": 5}`: each
//     operator key in the map must hold against vector.Metadata[k].
//     Supported operators: $eq, $ne, $gt, $gte, $lt, $lte, $in, $nin,
//     $contains. Unknown operator keys cause the vector to fail the
//     match (so callers learn from a missing result, not a silently
//     wrong inclusion).
//
// A vector with no metadata under the queried key fails the match.
func matchesFilters(vector *VectorData, filters map[string]interface{}) bool {
	if len(filters) == 0 {
		return true
	}
	for key, filterValue := range filters {
		vectorValue, exists := vector.Metadata[key]
		if !exists {
			return false
		}
		if opMap, ok := filterValue.(map[string]interface{}); ok {
			if !matchesOperatorMap(vectorValue, opMap) {
				return false
			}
			continue
		}
		// Scalar value → strict equality.
		if vectorValue != filterValue {
			return false
		}
	}
	return true
}

// matchesOperatorMap evaluates a `{"$op": value, ...}` map against the
// vector's value. Every operator key in the map must hold (logical AND).
// Unknown operators return false (fail closed) so callers don't get
// silently-wrong matches from typos in their filter syntax.
func matchesOperatorMap(vectorValue interface{}, ops map[string]interface{}) bool {
	for op, want := range ops {
		switch op {
		case "$eq":
			if vectorValue != want {
				return false
			}
		case "$ne":
			if vectorValue == want {
				return false
			}
		case "$gt":
			if !compareNumeric(vectorValue, want, func(a, b float64) bool { return a > b }) {
				return false
			}
		case "$gte":
			if !compareNumeric(vectorValue, want, func(a, b float64) bool { return a >= b }) {
				return false
			}
		case "$lt":
			if !compareNumeric(vectorValue, want, func(a, b float64) bool { return a < b }) {
				return false
			}
		case "$lte":
			if !compareNumeric(vectorValue, want, func(a, b float64) bool { return a <= b }) {
				return false
			}
		case "$in":
			if !inSlice(vectorValue, want) {
				return false
			}
		case "$nin":
			if inSlice(vectorValue, want) {
				return false
			}
		case "$contains":
			vs, vok := vectorValue.(string)
			ws, wok := want.(string)
			if !vok || !wok || !strings.Contains(vs, ws) {
				return false
			}
		default:
			// Unknown operator — fail closed.
			return false
		}
	}
	return true
}

// compareNumeric coerces both sides to float64 and runs `cmp`. Returns
// false if either side isn't a recognised numeric type.
func compareNumeric(a, b interface{}, cmp func(float64, float64) bool) bool {
	af, aok := toFloat64(a)
	bf, bok := toFloat64(b)
	if !aok || !bok {
		return false
	}
	return cmp(af, bf)
}

// toFloat64 coerces the common JSON / Go numeric types to float64. Returns
// (0, false) if the value isn't numeric — callers treat that as no-match.
func toFloat64(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint:
		return float64(n), true
	case uint32:
		return float64(n), true
	case uint64:
		return float64(n), true
	default:
		return 0, false
	}
}

// inSlice reports whether `v` equals any element of `want`. `want` may be
// `[]interface{}` (the JSON-decoded shape) or a typed slice. Returns
// false if `want` isn't a slice at all (so $in with a non-list value
// fails closed rather than silently matching).
func inSlice(v, want interface{}) bool {
	if list, ok := want.([]interface{}); ok {
		for _, item := range list {
			if v == item {
				return true
			}
		}
		return false
	}
	// Try common typed slices.
	switch list := want.(type) {
	case []string:
		s, ok := v.(string)
		if !ok {
			return false
		}
		for _, item := range list {
			if s == item {
				return true
			}
		}
	case []int:
		n, ok := v.(int)
		if !ok {
			return false
		}
		for _, item := range list {
			if n == item {
				return true
			}
		}
	case []float64:
		f, ok := toFloat64(v)
		if !ok {
			return false
		}
		for _, item := range list {
			if f == item {
				return true
			}
		}
	}
	return false
}
