package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"dev.helix.code/internal/logging"
)

// WeaviateProvider implements VectorProvider for Weaviate
type WeaviateProvider struct {
	config      *WeaviateConfig
	logger      *logging.Logger
	httpClient  *http.Client
	mu          sync.RWMutex
	initialized bool
	started     bool
}

// WeaviateConfig holds configuration for Weaviate
type WeaviateConfig struct {
	URL       string `json:"url"`
	APIKey    string `json:"api_key"`
	Class     string `json:"class"`
	BatchSize int    `json:"batch_size"`
	// Backup configuration
	BackupBackend string `json:"backup_backend"` // s3, gcs, azure, filesystem
}

// BackupBackend represents the type of backup storage backend
type BackupBackend string

const (
	// BackupBackendS3 uses AWS S3 or S3-compatible storage
	BackupBackendS3 BackupBackend = "s3"
	// BackupBackendGCS uses Google Cloud Storage
	BackupBackendGCS BackupBackend = "gcs"
	// BackupBackendAzure uses Microsoft Azure Blob Storage
	BackupBackendAzure BackupBackend = "azure"
	// BackupBackendFilesystem uses local filesystem storage
	BackupBackendFilesystem BackupBackend = "filesystem"
)

// WeaviateBackupRequest represents a request to create a Weaviate backup
type WeaviateBackupRequest struct {
	ID               string   `json:"id"`
	Include          []string `json:"include,omitempty"`
	Exclude          []string `json:"exclude,omitempty"`
	CPUPercentage    int      `json:"CPUPercentage,omitempty"`
	ChunkSize        int      `json:"ChunkSize,omitempty"`
	CompressionLevel string   `json:"CompressionLevel,omitempty"`
}

// WeaviateBackupResponse represents the response from a backup/restore operation
type WeaviateBackupResponse struct {
	ID        string   `json:"id"`
	Backend   string   `json:"backend"`
	Path      string   `json:"path"`
	Status    string   `json:"status"`
	Error     string   `json:"error,omitempty"`
	Classes   []string `json:"classes,omitempty"`
	StartTime string   `json:"startTime,omitempty"`
	EndTime   string   `json:"endTime,omitempty"`
}

// WeaviateRestoreRequest represents a request to restore a Weaviate backup
type WeaviateRestoreRequest struct {
	Include       []string `json:"include,omitempty"`
	Exclude       []string `json:"exclude,omitempty"`
	CPUPercentage int      `json:"CPUPercentage,omitempty"`
}

// Backup status constants
const (
	BackupStatusStarted   = "STARTED"
	BackupStatusSuccess   = "SUCCESS"
	BackupStatusFailed    = "FAILED"
	BackupStatusCancelled = "CANCELLED"
)

// NewWeaviateProvider creates a new Weaviate provider
func NewWeaviateProvider(config map[string]interface{}) (VectorProvider, error) {
	cfg := &WeaviateConfig{
		URL:           getStringConfig(config, "url", "http://localhost:8080"),
		APIKey:        getStringConfig(config, "api_key", ""),
		Class:         getStringConfig(config, "class", "Vector"),
		BatchSize:     getIntConfig(config, "batch_size", 100),
		BackupBackend: getStringConfig(config, "backup_backend", "filesystem"),
	}

	logger := logging.NewLoggerWithName("weaviate_provider")

	return &WeaviateProvider{
		config: cfg,
		logger: logger,
	}, nil
}

// testConnection tests the connection to Weaviate
func (p *WeaviateProvider) testConnection(ctx context.Context) error {
	url := fmt.Sprintf("%s/v1/meta", p.config.URL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	if p.config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.config.APIKey)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Weaviate meta failed with status: %d", resp.StatusCode)
	}

	return nil
}

// Initialize initializes the Weaviate provider
func (p *WeaviateProvider) Initialize(ctx context.Context, config interface{}) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.logger.Info("Initializing Weaviate provider url=%s class=%s", p.config.URL, p.config.Class)

	// Initialize HTTP client
	p.httpClient = &http.Client{
		Timeout: 30 * time.Second,
	}

	// Test connection to Weaviate
	if err := p.testConnection(ctx); err != nil {
		return fmt.Errorf("failed to connect to Weaviate: %w", err)
	}

	p.initialized = true
	p.logger.Info("Weaviate provider initialized successfully")
	return nil
}

// Start starts the Weaviate provider
func (p *WeaviateProvider) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.initialized {
		return fmt.Errorf("provider not initialized")
	}

	if p.started {
		return fmt.Errorf("provider already started")
	}

	p.logger.Info("Starting Weaviate provider")

	// Verify connection is still valid
	if err := p.testConnection(ctx); err != nil {
		return fmt.Errorf("failed to verify Weaviate connection: %w", err)
	}

	p.started = true
	p.logger.Info("Weaviate provider started successfully")
	return nil
}

// Stop stops the Weaviate provider
func (p *WeaviateProvider) Stop(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.started {
		return nil // Already stopped
	}

	p.logger.Info("Stopping Weaviate provider")

	// Close HTTP client transport if possible
	if p.httpClient != nil {
		if transport, ok := p.httpClient.Transport.(*http.Transport); ok {
			transport.CloseIdleConnections()
		}
	}

	p.started = false
	p.logger.Info("Weaviate provider stopped successfully")
	return nil
}

// GetName returns the provider name
func (p *WeaviateProvider) GetName() string {
	return "weaviate"
}

// GetType returns the provider type
func (p *WeaviateProvider) GetType() string {
	return string(ProviderTypeWeaviate)
}

// GetCapabilities returns provider capabilities
func (p *WeaviateProvider) GetCapabilities() []string {
	return []string{"vector_storage", "similarity_search", "metadata_filtering"}
}

// GetConfiguration returns the current configuration
func (p *WeaviateProvider) GetConfiguration() interface{} {
	return p.config
}

// IsCloud returns whether this is a cloud provider
func (p *WeaviateProvider) IsCloud() bool {
	return false // Weaviate can be self-hosted or cloud
}

// GetCostInfo returns cost information
func (p *WeaviateProvider) GetCostInfo() *CostInfo {
	return &CostInfo{
		Currency:      "USD",
		ComputeCost:   0.0,
		TransferCost:  0.0,
		StorageCost:   0.0,
		TotalCost:     0.0,
		BillingPeriod: "monthly",
	}
}

// Store stores vectors in Weaviate
func (p *WeaviateProvider) Store(ctx context.Context, vectors []*VectorData) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.started {
		return fmt.Errorf("provider not started")
	}

	if len(vectors) == 0 {
		return nil
	}

	p.logger.Info("Storing %d vectors in Weaviate", len(vectors))

	// Ensure class exists
	if err := p.ensureClass(ctx, vectors[0]); err != nil {
		return err
	}

	// Store vectors in batches
	batchSize := p.config.BatchSize
	for i := 0; i < len(vectors); i += batchSize {
		end := i + batchSize
		if end > len(vectors) {
			end = len(vectors)
		}

		batch := vectors[i:end]
		if err := p.storeBatch(ctx, batch); err != nil {
			return fmt.Errorf("failed to store batch %d-%d: %w", i, end, err)
		}
	}

	return nil
}

// ensureClass ensures the Weaviate class exists
func (p *WeaviateProvider) ensureClass(ctx context.Context, sampleVector *VectorData) error {
	// Check if class exists
	url := fmt.Sprintf("%s/v1/schema/classes/%s", p.config.URL, p.config.Class)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	if p.config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.config.APIKey)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		// Class exists
		return nil
	}

	if resp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to check class existence: %s", string(body))
	}

	// Create class
	classDef := map[string]interface{}{
		"class": p.config.Class,
		"properties": []map[string]interface{}{
			{
				"name":        "id",
				"dataType":    []string{"string"},
				"description": "Unique identifier",
			},
			{
				"name":        "timestamp",
				"dataType":    []string{"date"},
				"description": "Creation timestamp",
			},
		},
		"vectorizer": "none", // We'll provide vectors manually
		"vectorIndexConfig": map[string]interface{}{
			"distance": "cosine",
		},
	}

	jsonData, err := json.Marshal(classDef)
	if err != nil {
		return err
	}

	url = fmt.Sprintf("%s/v1/schema/classes", p.config.URL)
	req, err = http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	if p.config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.config.APIKey)
	}

	resp, err = p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create class: %s", string(body))
	}

	return nil
}

// storeBatch stores a batch of vectors
func (p *WeaviateProvider) storeBatch(ctx context.Context, vectors []*VectorData) error {
	objects := make([]map[string]interface{}, len(vectors))

	for i, v := range vectors {
		properties := make(map[string]interface{})
		for k, val := range v.Metadata {
			properties[k] = val
		}
		properties["id"] = v.ID
		properties["timestamp"] = v.Timestamp.Format(time.RFC3339)

		objects[i] = map[string]interface{}{
			"class":      p.config.Class,
			"properties": properties,
			"vector":     v.Vector,
		}
	}

	data := map[string]interface{}{
		"objects": objects,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/v1/objects", p.config.URL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	if p.config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.config.APIKey)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Weaviate batch store failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// Retrieve retrieves vectors by IDs
func (p *WeaviateProvider) Retrieve(ctx context.Context, ids []string) ([]*VectorData, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.started {
		return nil, fmt.Errorf("provider not started")
	}

	if len(ids) == 0 {
		return []*VectorData{}, nil
	}

	p.logger.Info("Retrieving %d vectors from Weaviate", len(ids))

	vectors := make([]*VectorData, 0, len(ids))

	// Retrieve each object by ID
	for _, id := range ids {
		url := fmt.Sprintf("%s/v1/objects/%s/%s?include=vector", p.config.URL, p.config.Class, id)
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, err
		}

		if p.config.APIKey != "" {
			req.Header.Set("Authorization", "Bearer "+p.config.APIKey)
		}

		resp, err := p.httpClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusNotFound {
			// Object not found, skip
			continue
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("failed to retrieve object %s: %s", id, string(body))
		}

		var obj struct {
			ID         string                 `json:"id"`
			Properties map[string]interface{} `json:"properties"`
			Vector     []float64              `json:"vector"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&obj); err != nil {
			return nil, err
		}

		// Parse timestamp
		timestamp := time.Now()
		if ts, ok := obj.Properties["timestamp"].(string); ok {
			if parsedTime, err := time.Parse(time.RFC3339, ts); err == nil {
				timestamp = parsedTime
			}
		}

		// Remove internal properties
		metadata := make(map[string]interface{})
		for k, v := range obj.Properties {
			if k != "id" && k != "timestamp" {
				metadata[k] = v
			}
		}

		vectors = append(vectors, &VectorData{
			ID:        obj.ID,
			Vector:    obj.Vector,
			Metadata:  metadata,
			Timestamp: timestamp,
		})
	}

	return vectors, nil
}

// Update updates a vector
func (p *WeaviateProvider) Update(ctx context.Context, id string, vector *VectorData) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.started {
		return fmt.Errorf("provider not started")
	}

	p.logger.Info("Updating vector %s in Weaviate", id)

	// Prepare properties
	properties := make(map[string]interface{})
	for k, v := range vector.Metadata {
		properties[k] = v
	}
	properties["id"] = vector.ID
	properties["timestamp"] = vector.Timestamp.Format(time.RFC3339)

	// Prepare update data
	data := map[string]interface{}{
		"class":      p.config.Class,
		"properties": properties,
		"vector":     vector.Vector,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/v1/objects/%s/%s", p.config.URL, p.config.Class, id)
	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	if p.config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.config.APIKey)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to update vector: %s", string(body))
	}

	return nil
}

// Delete deletes vectors by IDs
func (p *WeaviateProvider) Delete(ctx context.Context, ids []string) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.started {
		return fmt.Errorf("provider not started")
	}

	if len(ids) == 0 {
		return nil
	}

	p.logger.Info("Deleting %d vectors from Weaviate", len(ids))

	// Delete each object individually
	for _, id := range ids {
		url := fmt.Sprintf("%s/v1/objects/%s/%s", p.config.URL, p.config.Class, id)
		req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
		if err != nil {
			return err
		}

		if p.config.APIKey != "" {
			req.Header.Set("Authorization", "Bearer "+p.config.APIKey)
		}

		resp, err := p.httpClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("failed to delete vector %s: %s", id, string(body))
		}
	}

	return nil
}

// Search performs vector similarity search
func (p *WeaviateProvider) Search(ctx context.Context, query *VectorQuery) (*VectorSearchResult, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.started {
		return nil, fmt.Errorf("provider not started")
	}

	p.logger.Info("Searching vectors in Weaviate with top_k=%d", query.TopK)

	// Build GraphQL query
	graphqlQuery := fmt.Sprintf(`
	{
	  Get {
		%s(
		  nearVector: {vector: %s}
		  limit: %d
		) {
		  id
		  _additional {
			distance
		  }
		}
	  }
	}`, p.config.Class, vectorToGraphQLString(query.Vector), query.TopK)

	data := map[string]interface{}{
		"query": graphqlQuery,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/v1/graphql", p.config.URL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	if p.config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.config.APIKey)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Weaviate GraphQL query failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Parse response
	var gqlResp struct {
		Data struct {
			Get map[string][]struct {
				ID         string `json:"id"`
				Additional struct {
					Distance float64 `json:"distance"`
				} `json:"_additional"`
			} `json:"Get"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &gqlResp); err != nil {
		return nil, err
	}

	results := []*VectorSearchResultItem{}
	if objects, ok := gqlResp.Data.Get[p.config.Class]; ok {
		for _, obj := range objects {
			item := &VectorSearchResultItem{
				ID:       obj.ID,
				Distance: obj.Additional.Distance,
				Score:    1.0 - obj.Additional.Distance, // Convert distance to similarity
				Metadata: make(map[string]interface{}),
			}
			results = append(results, item)
		}
	}

	return &VectorSearchResult{
		Results: results,
		Total:   len(results),
		Query:   query,
	}, nil
}

// vectorToGraphQLString converts a vector to GraphQL string format
func vectorToGraphQLString(vec []float64) string {
	if len(vec) == 0 {
		return "[]"
	}

	result := "["
	for i, v := range vec {
		if i > 0 {
			result += ","
		}
		result += fmt.Sprintf("%.6f", v)
	}
	result += "]"
	return result
}

// FindSimilar finds similar vectors
func (p *WeaviateProvider) FindSimilar(ctx context.Context, embedding []float64, k int, filters map[string]interface{}) ([]*VectorSimilarityResult, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.started {
		return nil, fmt.Errorf("provider not started")
	}

	p.logger.Info("Finding %d similar vectors in Weaviate", k)

	// Use the Search method and convert results
	query := &VectorQuery{
		Vector:  embedding,
		TopK:    k,
		Filters: filters,
	}

	searchResult, err := p.Search(ctx, query)
	if err != nil {
		return nil, err
	}

	// Convert to VectorSimilarityResult format
	results := make([]*VectorSimilarityResult, len(searchResult.Results))
	for i, item := range searchResult.Results {
		results[i] = &VectorSimilarityResult{
			ID:       item.ID,
			Score:    item.Score,
			Distance: item.Distance,
			Metadata: item.Metadata,
			Vector:   nil, // Not included by default for performance
		}
	}

	return results, nil
}

// BatchFindSimilar performs batch similarity search
func (p *WeaviateProvider) BatchFindSimilar(ctx context.Context, queries [][]float64, k int) ([][]*VectorSimilarityResult, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.started {
		return nil, fmt.Errorf("provider not started")
	}

	p.logger.Info("Batch finding similar vectors for %d queries in Weaviate", len(queries))

	// Process each query sequentially
	// Weaviate doesn't have native batch search, so we do multiple queries
	results := make([][]*VectorSimilarityResult, len(queries))
	for i, query := range queries {
		similarVectors, err := p.FindSimilar(ctx, query, k, nil)
		if err != nil {
			return nil, fmt.Errorf("batch query %d failed: %w", i, err)
		}
		results[i] = similarVectors
	}

	return results, nil
}

// CreateCollection creates a new collection
func (p *WeaviateProvider) CreateCollection(ctx context.Context, name string, config *CollectionConfig) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.initialized {
		return fmt.Errorf("provider not initialized")
	}

	p.logger.Info("Creating collection %s in Weaviate", name)

	// Create a Weaviate class (collection)
	classDef := map[string]interface{}{
		"class": name,
		"properties": []map[string]interface{}{
			{
				"name":        "id",
				"dataType":    []string{"string"},
				"description": "Unique identifier",
			},
			{
				"name":        "timestamp",
				"dataType":    []string{"date"},
				"description": "Creation timestamp",
			},
		},
		"vectorizer": "none",
		"vectorIndexConfig": map[string]interface{}{
			"distance": "cosine",
		},
	}

	// Add custom properties from config if provided
	if config != nil && config.Properties != nil {
		properties := classDef["properties"].([]map[string]interface{})
		for key, propType := range config.Properties {
			// Type assert propType to string
			propTypeStr, ok := propType.(string)
			if !ok {
				propTypeStr = "text" // Default fallback
			}
			properties = append(properties, map[string]interface{}{
				"name":     key,
				"dataType": []string{propTypeStr},
			})
		}
		classDef["properties"] = properties
	}

	jsonData, err := json.Marshal(classDef)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/v1/schema/classes", p.config.URL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	if p.config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.config.APIKey)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create collection: %s", string(body))
	}

	return nil
}

// DeleteCollection deletes a collection
func (p *WeaviateProvider) DeleteCollection(ctx context.Context, name string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.initialized {
		return fmt.Errorf("provider not initialized")
	}

	p.logger.Info("Deleting collection %s from Weaviate", name)

	url := fmt.Sprintf("%s/v1/schema/classes/%s", p.config.URL, name)
	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return err
	}

	if p.config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.config.APIKey)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete collection: %s", string(body))
	}

	return nil
}

// ListCollections lists all collections
func (p *WeaviateProvider) ListCollections(ctx context.Context) ([]*CollectionInfo, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.initialized {
		return nil, fmt.Errorf("provider not initialized")
	}

	p.logger.Info("Listing collections in Weaviate")

	url := fmt.Sprintf("%s/v1/schema", p.config.URL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	if p.config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.config.APIKey)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to list collections: %s", string(body))
	}

	var schema struct {
		Classes []struct {
			Class       string `json:"class"`
			Description string `json:"description"`
		} `json:"classes"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&schema); err != nil {
		return nil, err
	}

	collections := make([]*CollectionInfo, len(schema.Classes))
	for i, class := range schema.Classes {
		collections[i] = &CollectionInfo{
			Name:        class.Class,
			VectorCount: 0, // Weaviate doesn't provide this in schema endpoint
			Status:      "active",
			CreatedAt:   time.Now(),
			Metadata: map[string]interface{}{
				"description": class.Description,
			},
		}
	}

	return collections, nil
}

// GetCollection gets collection information
func (p *WeaviateProvider) GetCollection(ctx context.Context, name string) (*CollectionInfo, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.initialized {
		return nil, fmt.Errorf("provider not initialized")
	}

	p.logger.Info("Getting collection %s info from Weaviate", name)

	url := fmt.Sprintf("%s/v1/schema/classes/%s", p.config.URL, name)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	if p.config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.config.APIKey)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("collection %s not found", name)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get collection: %s", string(body))
	}

	var class struct {
		Class       string `json:"class"`
		Description string `json:"description"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&class); err != nil {
		return nil, err
	}

	return &CollectionInfo{
		Name:        class.Class,
		VectorCount: 0,
		Status:      "active",
		CreatedAt:   time.Now(),
		Metadata: map[string]interface{}{
			"description": class.Description,
		},
	}, nil
}

// CreateIndex creates an index
func (p *WeaviateProvider) CreateIndex(ctx context.Context, collection string, config *IndexConfig) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.initialized {
		return fmt.Errorf("provider not initialized")
	}

	p.logger.Info("Creating index %s in collection %s in Weaviate", config.Name, collection)

	// In Weaviate, indexes are defined at class creation time via vectorIndexConfig
	// We cannot create indexes dynamically after class creation
	// Log info and return success for compatibility
	p.logger.Info("Weaviate indexes are managed at class/collection level - skipping dynamic index creation")

	return nil
}

// DeleteIndex deletes an index
func (p *WeaviateProvider) DeleteIndex(ctx context.Context, collection, name string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.initialized {
		return fmt.Errorf("provider not initialized")
	}

	p.logger.Info("Deleting index %s from collection %s in Weaviate", name, collection)

	// Weaviate indexes are part of class definition and cannot be deleted separately
	p.logger.Info("Weaviate indexes are managed at class/collection level - skipping dynamic index deletion")

	return nil
}

// ListIndexes lists indexes in a collection
func (p *WeaviateProvider) ListIndexes(ctx context.Context, collection string) ([]*IndexInfo, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.initialized {
		return nil, fmt.Errorf("provider not initialized")
	}

	p.logger.Info("Listing indexes in collection %s in Weaviate", collection)

	// Weaviate uses HNSW index by default for vector indexing
	// Return the default index info
	return []*IndexInfo{
		{
			Name:      "hnsw_vector_index",
			Type:      "hnsw",
			State:     "ready",
			CreatedAt: time.Now(),
			Metadata: map[string]interface{}{
				"collection":   collection,
				"vector_count": 0,
			},
		},
	}, nil
}

// AddMetadata adds metadata to a vector
func (p *WeaviateProvider) AddMetadata(ctx context.Context, id string, metadata map[string]interface{}) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.started {
		return fmt.Errorf("provider not started")
	}

	p.logger.Info("Adding metadata to vector %s in Weaviate", id)

	// Retrieve existing vector
	vectors, err := p.Retrieve(ctx, []string{id})
	if err != nil {
		return err
	}
	if len(vectors) == 0 {
		return fmt.Errorf("vector %s not found", id)
	}

	// Merge new metadata with existing
	vector := vectors[0]
	if vector.Metadata == nil {
		vector.Metadata = make(map[string]interface{})
	}
	for k, v := range metadata {
		vector.Metadata[k] = v
	}

	// Update the vector
	return p.Update(ctx, id, vector)
}

// UpdateMetadata updates metadata
func (p *WeaviateProvider) UpdateMetadata(ctx context.Context, id string, metadata map[string]interface{}) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.started {
		return fmt.Errorf("provider not started")
	}

	p.logger.Info("Updating metadata for vector %s in Weaviate", id)

	// Retrieve existing vector
	vectors, err := p.Retrieve(ctx, []string{id})
	if err != nil {
		return err
	}
	if len(vectors) == 0 {
		return fmt.Errorf("vector %s not found", id)
	}

	// Replace metadata entirely
	vector := vectors[0]
	vector.Metadata = metadata

	// Update the vector
	return p.Update(ctx, id, vector)
}

// GetMetadata gets metadata for vectors
func (p *WeaviateProvider) GetMetadata(ctx context.Context, ids []string) (map[string]map[string]interface{}, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.started {
		return nil, fmt.Errorf("provider not started")
	}

	if len(ids) == 0 {
		return map[string]map[string]interface{}{}, nil
	}

	p.logger.Info("Getting metadata for %d vectors from Weaviate", len(ids))

	// Retrieve vectors
	vectors, err := p.Retrieve(ctx, ids)
	if err != nil {
		return nil, err
	}

	// Extract metadata
	result := make(map[string]map[string]interface{})
	for _, vector := range vectors {
		result[vector.ID] = vector.Metadata
	}

	return result, nil
}

// DeleteMetadata deletes metadata
func (p *WeaviateProvider) DeleteMetadata(ctx context.Context, ids []string, keys []string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.started {
		return fmt.Errorf("provider not started")
	}

	if len(ids) == 0 || len(keys) == 0 {
		return nil
	}

	p.logger.Info("Deleting metadata keys %v for %d vectors in Weaviate", keys, len(ids))

	// Process each ID
	for _, id := range ids {
		// Retrieve existing vector
		vectors, err := p.Retrieve(ctx, []string{id})
		if err != nil {
			return err
		}
		if len(vectors) == 0 {
			continue // Skip if not found
		}

		// Remove specified keys from metadata
		vector := vectors[0]
		if vector.Metadata == nil {
			continue
		}

		for _, key := range keys {
			delete(vector.Metadata, key)
		}

		// Update the vector
		if err := p.Update(ctx, id, vector); err != nil {
			return err
		}
	}

	return nil
}

// GetStats returns provider statistics
func (p *WeaviateProvider) GetStats(ctx context.Context) (*ProviderStats, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.initialized {
		return nil, fmt.Errorf("provider not initialized")
	}

	p.logger.Info("Getting stats from Weaviate provider")

	// Get collection count
	collections, err := p.ListCollections(ctx)
	if err != nil {
		p.logger.Warn("Failed to list collections for stats: %v", err)
		collections = []*CollectionInfo{}
	}

	// Get meta information
	url := fmt.Sprintf("%s/v1/meta", p.config.URL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	if p.config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.config.APIKey)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	status := "operational"
	if resp.StatusCode != http.StatusOK {
		status = "degraded"
	}

	return &ProviderStats{
		Name:             p.GetName(),
		Type:             p.GetType(),
		Status:           status,
		TotalVectors:     0, // Weaviate doesn't provide this easily
		TotalCollections: int64(len(collections)),
		TotalSize:        0,
		LastHealthCheck:  time.Now(),
	}, nil
}

// Optimize optimizes the provider
func (p *WeaviateProvider) Optimize(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.started {
		return fmt.Errorf("provider not started")
	}

	p.logger.Info("Optimizing Weaviate provider")

	// Weaviate performs automatic optimization
	// This is a no-op for compatibility with the interface
	p.logger.Info("Weaviate performs automatic optimization - no manual action needed")

	return nil
}

// Backup creates a backup using Weaviate's native backup API.
//
// The path parameter is used as the backup ID. The backup is created using the
// configured backup backend (s3, gcs, azure, or filesystem).
//
// Weaviate Backup API Reference:
//   - POST /v1/backups/{backend} - Create a backup
//   - GET /v1/backups/{backend}/{backup_id} - Check backup status
//
// Supported backends:
//   - s3: AWS S3 or S3-compatible storage (requires BACKUP_S3_* env vars)
//   - gcs: Google Cloud Storage (requires BACKUP_GCS_* env vars)
//   - azure: Microsoft Azure Blob Storage (requires BACKUP_AZURE_* env vars)
//   - filesystem: Local filesystem (requires BACKUP_FILESYSTEM_PATH env var)
//
// Example configuration for filesystem backend:
//
//	BACKUP_FILESYSTEM_PATH=/var/weaviate/backups
//
// For more details, see: https://weaviate.io/developers/weaviate/configuration/backups
func (p *WeaviateProvider) Backup(ctx context.Context, path string) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.started {
		return fmt.Errorf("provider not started")
	}

	if path == "" {
		return fmt.Errorf("backup path/id cannot be empty")
	}

	backend := p.config.BackupBackend
	if backend == "" {
		backend = "filesystem"
	}

	// Validate backend
	if !isValidBackupBackend(backend) {
		return fmt.Errorf("invalid backup backend: %s (supported: s3, gcs, azure, filesystem)", backend)
	}

	p.logger.Info("Creating backup with id=%s backend=%s for Weaviate provider", path, backend)

	// Create backup request - include configured class if specified
	backupReq := WeaviateBackupRequest{
		ID: path,
	}

	// Include the configured class in the backup if specified
	if p.config.Class != "" {
		backupReq.Include = []string{p.config.Class}
	}

	jsonData, err := json.Marshal(backupReq)
	if err != nil {
		return fmt.Errorf("failed to marshal backup request: %w", err)
	}

	// Create backup via POST /v1/backups/{backend}
	url := fmt.Sprintf("%s/v1/backups/%s", p.config.URL, backend)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create backup request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if p.config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.config.APIKey)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute backup request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read backup response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("backup request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var backupResp WeaviateBackupResponse
	if err := json.Unmarshal(body, &backupResp); err != nil {
		return fmt.Errorf("failed to parse backup response: %w", err)
	}

	p.logger.Info("Backup initiated with id=%s status=%s", backupResp.ID, backupResp.Status)

	// Wait for backup to complete by polling status
	if err := p.waitForBackupCompletion(ctx, backend, path); err != nil {
		return err
	}

	p.logger.Info("Backup completed successfully with id=%s", path)
	return nil
}

// waitForBackupCompletion polls the backup status until it completes or fails
func (p *WeaviateProvider) waitForBackupCompletion(ctx context.Context, backend, backupID string) error {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	timeout := time.After(10 * time.Minute)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return fmt.Errorf("backup timed out after 10 minutes")
		case <-ticker.C:
			status, err := p.getBackupStatus(ctx, backend, backupID)
			if err != nil {
				return fmt.Errorf("failed to get backup status: %w", err)
			}

			switch status.Status {
			case BackupStatusSuccess:
				return nil
			case BackupStatusFailed:
				return fmt.Errorf("backup failed: %s", status.Error)
			case BackupStatusCancelled:
				return fmt.Errorf("backup was cancelled")
			case BackupStatusStarted:
				p.logger.Info("Backup in progress id=%s", backupID)
				continue
			default:
				p.logger.Info("Backup status: %s", status.Status)
				continue
			}
		}
	}
}

// getBackupStatus retrieves the current status of a backup operation
func (p *WeaviateProvider) getBackupStatus(ctx context.Context, backend, backupID string) (*WeaviateBackupResponse, error) {
	url := fmt.Sprintf("%s/v1/backups/%s/%s", p.config.URL, backend, backupID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	if p.config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.config.APIKey)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get backup status: %s", string(body))
	}

	var statusResp WeaviateBackupResponse
	if err := json.Unmarshal(body, &statusResp); err != nil {
		return nil, err
	}

	return &statusResp, nil
}

// isValidBackupBackend checks if the given backend is valid
func isValidBackupBackend(backend string) bool {
	switch BackupBackend(backend) {
	case BackupBackendS3, BackupBackendGCS, BackupBackendAzure, BackupBackendFilesystem:
		return true
	default:
		return false
	}
}

// Restore restores from a backup using Weaviate's native backup API.
//
// The path parameter is used as the backup ID. The restore operation uses the
// configured backup backend (s3, gcs, azure, or filesystem).
//
// Weaviate Restore API Reference:
//   - POST /v1/backups/{backend}/{backup_id}/restore - Restore from backup
//   - GET /v1/backups/{backend}/{backup_id}/restore - Check restore status
//
// Supported backends:
//   - s3: AWS S3 or S3-compatible storage (requires BACKUP_S3_* env vars)
//   - gcs: Google Cloud Storage (requires BACKUP_GCS_* env vars)
//   - azure: Microsoft Azure Blob Storage (requires BACKUP_AZURE_* env vars)
//   - filesystem: Local filesystem (requires BACKUP_FILESYSTEM_PATH env var)
//
// IMPORTANT: If you are running Weaviate v1.23.12 or older, you must update to
// v1.23.13 or higher before restoring a backup to prevent data corruption.
//
// For more details, see: https://weaviate.io/developers/weaviate/configuration/backups
func (p *WeaviateProvider) Restore(ctx context.Context, path string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.initialized {
		return fmt.Errorf("provider not initialized")
	}

	if path == "" {
		return fmt.Errorf("restore backup id cannot be empty")
	}

	backend := p.config.BackupBackend
	if backend == "" {
		backend = "filesystem"
	}

	// Validate backend
	if !isValidBackupBackend(backend) {
		return fmt.Errorf("invalid backup backend: %s (supported: s3, gcs, azure, filesystem)", backend)
	}

	p.logger.Info("Restoring backup with id=%s backend=%s for Weaviate provider", path, backend)

	// Create restore request - include configured class if specified
	restoreReq := WeaviateRestoreRequest{}

	// Include the configured class in the restore if specified
	if p.config.Class != "" {
		restoreReq.Include = []string{p.config.Class}
	}

	jsonData, err := json.Marshal(restoreReq)
	if err != nil {
		return fmt.Errorf("failed to marshal restore request: %w", err)
	}

	// Create restore via POST /v1/backups/{backend}/{backup_id}/restore
	url := fmt.Sprintf("%s/v1/backups/%s/%s/restore", p.config.URL, backend, path)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create restore request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if p.config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.config.APIKey)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute restore request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read restore response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("restore request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var restoreResp WeaviateBackupResponse
	if err := json.Unmarshal(body, &restoreResp); err != nil {
		return fmt.Errorf("failed to parse restore response: %w", err)
	}

	p.logger.Info("Restore initiated with id=%s status=%s", restoreResp.ID, restoreResp.Status)

	// Wait for restore to complete by polling status
	if err := p.waitForRestoreCompletion(ctx, backend, path); err != nil {
		return err
	}

	p.logger.Info("Restore completed successfully with id=%s", path)
	return nil
}

// waitForRestoreCompletion polls the restore status until it completes or fails
func (p *WeaviateProvider) waitForRestoreCompletion(ctx context.Context, backend, backupID string) error {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	timeout := time.After(10 * time.Minute)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return fmt.Errorf("restore timed out after 10 minutes")
		case <-ticker.C:
			status, err := p.getRestoreStatus(ctx, backend, backupID)
			if err != nil {
				return fmt.Errorf("failed to get restore status: %w", err)
			}

			switch status.Status {
			case BackupStatusSuccess:
				return nil
			case BackupStatusFailed:
				return fmt.Errorf("restore failed: %s", status.Error)
			case BackupStatusCancelled:
				return fmt.Errorf("restore was cancelled")
			case BackupStatusStarted:
				p.logger.Info("Restore in progress id=%s", backupID)
				continue
			default:
				p.logger.Info("Restore status: %s", status.Status)
				continue
			}
		}
	}
}

// getRestoreStatus retrieves the current status of a restore operation
func (p *WeaviateProvider) getRestoreStatus(ctx context.Context, backend, backupID string) (*WeaviateBackupResponse, error) {
	url := fmt.Sprintf("%s/v1/backups/%s/%s/restore", p.config.URL, backend, backupID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	if p.config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.config.APIKey)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get restore status: %s", string(body))
	}

	var statusResp WeaviateBackupResponse
	if err := json.Unmarshal(body, &statusResp); err != nil {
		return nil, err
	}

	return &statusResp, nil
}

// Health checks provider health
func (p *WeaviateProvider) Health(ctx context.Context) (*HealthStatus, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.initialized {
		return &HealthStatus{
			Status:       "not_initialized",
			Message:      "Provider not initialized",
			ResponseTime: 0,
			Timestamp:    time.Now(),
		}, nil
	}

	p.logger.Info("Checking health of Weaviate provider")

	startTime := time.Now()

	// Test connection via readiness endpoint
	url := fmt.Sprintf("%s/v1/.well-known/ready", p.config.URL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return &HealthStatus{
			Status:       "unhealthy",
			Message:      fmt.Sprintf("Failed to create request: %v", err),
			ResponseTime: time.Since(startTime),
			Timestamp:    time.Now(),
		}, nil
	}

	if p.config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.config.APIKey)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return &HealthStatus{
			Status:       "unhealthy",
			Message:      fmt.Sprintf("Connection failed: %v", err),
			ResponseTime: time.Since(startTime),
			Timestamp:    time.Now(),
		}, nil
	}
	defer resp.Body.Close()

	responseTime := time.Since(startTime)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return &HealthStatus{
			Status:       "unhealthy",
			Message:      fmt.Sprintf("Health check failed (status %d): %s", resp.StatusCode, string(body)),
			ResponseTime: responseTime,
			Timestamp:    time.Now(),
		}, nil
	}

	return &HealthStatus{
		Status:       "healthy",
		Message:      "Weaviate is operational",
		ResponseTime: responseTime,
		Timestamp:    time.Now(),
	}, nil
}

// Helper functions
func getStringConfig(config map[string]interface{}, key, defaultValue string) string {
	if val, ok := config[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return defaultValue
}

func getIntConfig(config map[string]interface{}, key string, defaultValue int) int {
	if val, ok := config[key]; ok {
		if num, ok := val.(int); ok {
			return num
		}
	}
	return defaultValue
}

// Close closes the Weaviate provider
func (p *WeaviateProvider) Close(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.started {
		return nil // Already stopped
	}

	if p.httpClient != nil {
		// Close idle connections
		p.httpClient.CloseIdleConnections()
	}

	p.started = false
	p.initialized = false

	p.logger.Info("Weaviate provider closed successfully")
	return nil
}
