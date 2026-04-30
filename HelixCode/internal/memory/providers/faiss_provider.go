package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"dev.helix.code/internal/logging"
)

// SimulationNotice is a constant message explaining the simulation nature of this provider.
const SimulationNotice = "FAISSProvider is a simulated implementation that does not use the native FAISS library. " +
	"It provides compatible vector storage and search functionality using pure Go implementations. " +
	"For production use with large-scale vector search, consider using actual FAISS with CGO bindings " +
	"or an external vector database like Milvus, Pinecone, or Qdrant."

// FAISSProvider implements VectorProvider with a simulated FAISS-like interface.
//
// IMPORTANT: This is a SIMULATED provider that does NOT use the native FAISS library.
// It provides API-compatible vector storage and similarity search using pure Go implementations.
//
// Key differences from native FAISS:
//   - No GPU acceleration (GPU operations are no-ops with informational logging)
//   - No IVF/PQ/HNSW index optimizations (uses brute-force search)
//   - Suitable for development, testing, and small-scale deployments
//   - For large-scale production use, integrate native FAISS via CGO or use managed services
//
// Features provided:
//   - Vector storage with metadata
//   - Cosine similarity search (with Euclidean and dot product support)
//   - Collection management
//   - JSON-based persistence
//   - Backup and restore functionality
//   - Metadata filtering
type FAISSProvider struct {
	config      *FAISSConfig
	logger      *logging.Logger
	mu          sync.RWMutex
	initialized bool
	started     bool
	indices     map[string]*FAISSIndex
	collections map[string]*CollectionConfig
	stats       *ProviderStats
	startTime   time.Time
}

// FAISSConfig contains FAISS provider configuration.
// Note: Some options (like GPUDevice, IndexType, NList, NProbe) are accepted for API
// compatibility but have limited or no effect in this simulated implementation.
type FAISSConfig struct {
	IndexPath      string `json:"index_path"`      // Path for index files (used for persistence)
	IndexType      string `json:"index_type"`      // Index type (accepted but not used - simulation uses brute force)
	Dimension      int    `json:"dimension"`       // Vector dimension (validated on insert)
	Metric         string `json:"metric"`          // Distance metric: "cosine", "euclidean", "dot" (cosine by default)
	NList          int    `json:"nlist"`           // IVF parameter (accepted but not used in simulation)
	NProbe         int    `json:"nprobe"`          // IVF search parameter (accepted but not used in simulation)
	MemoryIndex    bool   `json:"memory_index"`    // Keep index in memory (always true in simulation)
	GPUDevice      int    `json:"gpu_device"`      // GPU device ID (GPU not available in simulation)
	StoragePath    string `json:"storage_path"`    // Path for persistent storage
	Compression    bool   `json:"compression"`     // Enable compression (not implemented in simulation)
	BatchSize      int    `json:"batch_size"`      // Batch size for operations
	MaxConnections int    `json:"max_connections"` // Max concurrent connections (informational only)
}

// FAISSIndex represents a simulated FAISS index.
// This is an in-memory index with JSON persistence, not a native FAISS index.
type FAISSIndex struct {
	name        string
	indexPath   string
	dimension   int
	metric      string
	vectorMap   map[string][]float64
	metadataMap map[string]map[string]interface{}
	timestamps  map[string]time.Time
	mu          sync.RWMutex
	initialized bool
	createdAt   time.Time
	updatedAt   time.Time
}

// indexPersistenceData is the JSON structure for persisting index data to disk.
type indexPersistenceData struct {
	Name      string                            `json:"name"`
	Dimension int                               `json:"dimension"`
	Metric    string                            `json:"metric"`
	Vectors   map[string][]float64              `json:"vectors"`
	Metadata  map[string]map[string]interface{} `json:"metadata"`
	CreatedAt time.Time                         `json:"created_at"`
	UpdatedAt time.Time                         `json:"updated_at"`
}

// NewFAISSProvider creates a new simulated FAISS provider.
// This provider does NOT use native FAISS - see SimulationNotice for details.
func NewFAISSProvider(config map[string]interface{}) (VectorProvider, error) {
	faissConfig := &FAISSConfig{
		IndexPath:      "./data/faiss/index",
		IndexType:      "flat", // Default to flat (brute force) since that's what simulation does
		Dimension:      1536,
		Metric:         "cosine",
		NList:          100,
		NProbe:         10,
		MemoryIndex:    true,
		GPUDevice:      -1, // Default to CPU (-1 means no GPU)
		StoragePath:    "./data/faiss",
		Compression:    false,
		BatchSize:      1000,
		MaxConnections: 100,
	}

	// Parse configuration from map
	if config != nil {
		if v, ok := config["index_path"].(string); ok {
			faissConfig.IndexPath = v
		}
		if v, ok := config["index_type"].(string); ok {
			faissConfig.IndexType = v
		}
		if v, ok := config["dimension"].(int); ok {
			faissConfig.Dimension = v
		}
		if v, ok := config["dimension"].(float64); ok {
			faissConfig.Dimension = int(v)
		}
		if v, ok := config["metric"].(string); ok {
			faissConfig.Metric = v
		}
		if v, ok := config["nlist"].(int); ok {
			faissConfig.NList = v
		}
		if v, ok := config["nlist"].(float64); ok {
			faissConfig.NList = int(v)
		}
		if v, ok := config["nprobe"].(int); ok {
			faissConfig.NProbe = v
		}
		if v, ok := config["nprobe"].(float64); ok {
			faissConfig.NProbe = int(v)
		}
		if v, ok := config["memory_index"].(bool); ok {
			faissConfig.MemoryIndex = v
		}
		if v, ok := config["gpu_device"].(int); ok {
			faissConfig.GPUDevice = v
		}
		if v, ok := config["gpu_device"].(float64); ok {
			faissConfig.GPUDevice = int(v)
		}
		if v, ok := config["storage_path"].(string); ok {
			faissConfig.StoragePath = v
		}
		if v, ok := config["compression"].(bool); ok {
			faissConfig.Compression = v
		}
		if v, ok := config["batch_size"].(int); ok {
			faissConfig.BatchSize = v
		}
		if v, ok := config["batch_size"].(float64); ok {
			faissConfig.BatchSize = int(v)
		}
		if v, ok := config["max_connections"].(int); ok {
			faissConfig.MaxConnections = v
		}
		if v, ok := config["max_connections"].(float64); ok {
			faissConfig.MaxConnections = int(v)
		}
	}

	return &FAISSProvider{
		config:      faissConfig,
		logger:      logging.NewLoggerWithName("faiss_provider"),
		indices:     make(map[string]*FAISSIndex),
		collections: make(map[string]*CollectionConfig),
		stats: &ProviderStats{
			TotalVectors:     0,
			TotalCollections: 0,
			TotalSize:        0,
			AverageLatency:   0,
			LastOperation:    time.Now(),
			ErrorCount:       0,
			Uptime:           0,
		},
	}, nil
}

// Initialize initializes the simulated FAISS provider.
// Note: This is a simulation - no native FAISS library is loaded.
func (p *FAISSProvider) Initialize(ctx context.Context, config interface{}) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.initialized {
		return nil
	}

	p.logger.Info("Initializing simulated FAISS provider (no native FAISS library)")
	p.logger.Info("Configuration: index_type=%s dimension=%d metric=%s storage_path=%s",
		p.config.IndexType, p.config.Dimension, p.config.Metric, p.config.StoragePath)
	p.logger.Info("Note: %s", SimulationNotice)

	// Create storage directory for persistence
	if err := os.MkdirAll(p.config.StoragePath, 0755); err != nil {
		return fmt.Errorf("failed to create storage directory: %w", err)
	}

	// Load existing indices from disk if available
	if err := p.loadExistingIndices(ctx); err != nil {
		p.logger.Warn("Failed to load existing indices (starting fresh): %v", err)
	}

	p.initialized = true
	p.stats.LastOperation = time.Now()

	p.logger.Info("Simulated FAISS provider initialized successfully (loaded %d indices)", len(p.indices))
	return nil
}

// Start starts the simulated FAISS provider.
func (p *FAISSProvider) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.initialized {
		return fmt.Errorf("provider not initialized")
	}

	if p.started {
		return nil
	}

	// Log GPU configuration notice (GPU not available in simulation)
	if p.config.GPUDevice >= 0 {
		p.logger.Info("GPU device %d requested but not available in simulated FAISS provider. "+
			"Using CPU-based vector search. For GPU acceleration, integrate native FAISS via CGO.", p.config.GPUDevice)
	}

	p.started = true
	p.startTime = time.Now()
	p.stats.LastOperation = time.Now()
	p.stats.Uptime = 0

	p.logger.Info("Simulated FAISS provider started (CPU mode, brute-force search)")
	return nil
}

// Store stores vectors in the simulated FAISS index.
// Vectors are stored in-memory with metadata and can be persisted to disk.
func (p *FAISSProvider) Store(ctx context.Context, vectors []*VectorData) error {
	start := time.Now()
	defer func() {
		p.updateStats(time.Since(start))
	}()

	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.started {
		return fmt.Errorf("provider not started")
	}

	for _, vector := range vectors {
		collection := vector.Collection
		if collection == "" {
			collection = "default"
		}

		index, err := p.getOrCreateIndex(ctx, collection)
		if err != nil {
			return fmt.Errorf("failed to get or create index for collection %q: %w", collection, err)
		}

		if err := index.addVector(vector); err != nil {
			return fmt.Errorf("failed to add vector %q: %w", vector.ID, err)
		}

		p.stats.TotalVectors++
		p.stats.TotalSize += int64(len(vector.Vector) * 8) // 8 bytes per float64
	}

	p.stats.LastOperation = time.Now()
	return nil
}

// Retrieve retrieves vectors by ID from the simulated FAISS index.
func (p *FAISSProvider) Retrieve(ctx context.Context, ids []string) ([]*VectorData, error) {
	start := time.Now()
	defer func() {
		p.updateStats(time.Since(start))
	}()

	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.started {
		return nil, fmt.Errorf("provider not started")
	}

	var results []*VectorData

	// Search across all indices for the requested IDs
	for _, index := range p.indices {
		vectors := index.getVectors(ids)
		results = append(results, vectors...)
	}

	p.stats.LastOperation = time.Now()
	return results, nil
}

// Update updates a vector in the simulated FAISS index.
// Like native FAISS, this implementation deletes and re-inserts the vector.
func (p *FAISSProvider) Update(ctx context.Context, id string, vector *VectorData) error {
	// Delete existing vector first
	if err := p.Delete(ctx, []string{id}); err != nil {
		return fmt.Errorf("failed to delete existing vector %q during update: %w", id, err)
	}
	// Insert updated vector
	vector.ID = id // Ensure the ID is preserved
	return p.Store(ctx, []*VectorData{vector})
}

// Delete deletes vectors from the simulated FAISS index.
func (p *FAISSProvider) Delete(ctx context.Context, ids []string) error {
	start := time.Now()
	defer func() {
		p.updateStats(time.Since(start))
	}()

	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.started {
		return fmt.Errorf("provider not started")
	}

	deletedCount := 0
	for _, id := range ids {
		// Find and remove from all indices
		for name, index := range p.indices {
			if index.removeVector(id) {
				p.stats.TotalVectors--
				p.logger.Debug("Vector deleted: id=%s collection=%s", id, name)
				deletedCount++
				break
			}
		}
	}

	p.logger.Debug("Deleted %d vectors (requested %d)", deletedCount, len(ids))
	p.stats.LastOperation = time.Now()
	return nil
}

// Search performs vector similarity search in the simulated FAISS index.
// This uses brute-force similarity search (no IVF/HNSW optimization).
// Results are sorted by similarity score in descending order.
func (p *FAISSProvider) Search(ctx context.Context, query *VectorQuery) (*VectorSearchResult, error) {
	start := time.Now()
	defer func() {
		p.updateStats(time.Since(start))
	}()

	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.started {
		return nil, fmt.Errorf("provider not started")
	}

	collection := query.Collection
	if collection == "" {
		collection = "default"
	}

	index, exists := p.indices[collection]
	if !exists {
		return &VectorSearchResult{
			Results:  []*VectorSearchResultItem{},
			Total:    0,
			Query:    query,
			Duration: time.Since(start),
		}, nil
	}

	results, err := index.search(query, p.config.Metric)
	if err != nil {
		return nil, fmt.Errorf("search failed in collection %q: %w", collection, err)
	}

	return &VectorSearchResult{
		Results:   results,
		Total:     len(results),
		Query:     query,
		Duration:  time.Since(start),
		Namespace: query.Namespace,
	}, nil
}

// FindSimilar finds similar vectors using brute-force similarity search.
func (p *FAISSProvider) FindSimilar(ctx context.Context, embedding []float64, k int, filters map[string]interface{}) ([]*VectorSimilarityResult, error) {
	start := time.Now()
	defer func() {
		p.updateStats(time.Since(start))
	}()

	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.started {
		return nil, fmt.Errorf("provider not started")
	}

	// Extract collection from filters if present
	collection := ""
	if filters != nil {
		if c, ok := filters["collection"].(string); ok {
			collection = c
		}
	}

	query := &VectorQuery{
		Vector:     embedding,
		TopK:       k,
		Filters:    filters,
		Collection: collection,
	}

	searchResult, err := p.Search(ctx, query)
	if err != nil {
		return nil, err
	}

	results := make([]*VectorSimilarityResult, 0, len(searchResult.Results))
	for _, item := range searchResult.Results {
		results = append(results, &VectorSimilarityResult{
			ID:       item.ID,
			Vector:   item.Vector,
			Metadata: item.Metadata,
			Score:    item.Score,
			Distance: 1 - item.Score, // Convert similarity to distance
		})
	}

	return results, nil
}

// BatchFindSimilar finds similar vectors for multiple queries.
func (p *FAISSProvider) BatchFindSimilar(ctx context.Context, queries [][]float64, k int) ([][]*VectorSimilarityResult, error) {
	results := make([][]*VectorSimilarityResult, len(queries))
	for i, query := range queries {
		similar, err := p.FindSimilar(ctx, query, k, nil)
		if err != nil {
			return nil, fmt.Errorf("batch query %d failed: %w", i, err)
		}
		results[i] = similar
	}
	return results, nil
}

// CreateCollection creates a new collection in the simulated FAISS provider.
func (p *FAISSProvider) CreateCollection(ctx context.Context, name string, config *CollectionConfig) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.collections[name]; exists {
		return fmt.Errorf("collection %s already exists", name)
	}

	p.collections[name] = config
	p.stats.TotalCollections++

	p.logger.Info("Collection created: name=%s dimension=%d metric=%s", name, config.Dimension, config.Metric)
	return nil
}

// DeleteCollection deletes a collection from the simulated FAISS provider.
func (p *FAISSProvider) DeleteCollection(ctx context.Context, name string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.collections[name]; !exists {
		return fmt.Errorf("collection %s not found", name)
	}

	// Remove associated index and its persistence
	if index, exists := p.indices[name]; exists {
		// Delete persisted data
		persistPath := filepath.Join(index.indexPath, "index.json")
		if err := os.Remove(persistPath); err != nil && !os.IsNotExist(err) {
			p.logger.Warn("Failed to remove persisted index file: %v", err)
		}
	}

	delete(p.collections, name)
	delete(p.indices, name)
	p.stats.TotalCollections--

	p.logger.Info("Collection deleted: name=%s", name)
	return nil
}

// ListCollections lists all collections in the simulated FAISS provider.
func (p *FAISSProvider) ListCollections(ctx context.Context) ([]*CollectionInfo, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var collections []*CollectionInfo

	for name, config := range p.collections {
		var vectorCount int64
		var createdAt, updatedAt time.Time
		if index, exists := p.indices[name]; exists {
			vectorCount = int64(index.vectorCount())
			createdAt = index.createdAt
			updatedAt = index.updatedAt
		}
		if createdAt.IsZero() {
			createdAt = time.Now()
		}
		if updatedAt.IsZero() {
			updatedAt = time.Now()
		}

		collections = append(collections, &CollectionInfo{
			Name:        name,
			Dimension:   config.Dimension,
			Metric:      config.Metric,
			VectorCount: vectorCount,
			CreatedAt:   createdAt,
			UpdatedAt:   updatedAt,
		})
	}

	return collections, nil
}

// GetCollection gets collection information.
func (p *FAISSProvider) GetCollection(ctx context.Context, name string) (*CollectionInfo, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	config, exists := p.collections[name]
	if !exists {
		return nil, fmt.Errorf("collection %s not found", name)
	}

	var vectorCount int64
	var createdAt, updatedAt time.Time
	if index, exists := p.indices[name]; exists {
		vectorCount = int64(index.vectorCount())
		createdAt = index.createdAt
		updatedAt = index.updatedAt
	}
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	if updatedAt.IsZero() {
		updatedAt = time.Now()
	}

	return &CollectionInfo{
		Name:        name,
		Dimension:   config.Dimension,
		Metric:      config.Metric,
		VectorCount: vectorCount,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}, nil
}

// CreateIndex creates an index. In this simulation, indices are automatically created.
// This method logs that IVF/HNSW optimizations are not available.
func (p *FAISSProvider) CreateIndex(ctx context.Context, collection string, config *IndexConfig) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	_, exists := p.indices[collection]
	if !exists {
		return fmt.Errorf("collection %s not found", collection)
	}

	p.logger.Info("CreateIndex called for collection %s with type %s. "+
		"Note: This simulated provider uses brute-force search; IVF/HNSW optimizations are not available.",
		collection, config.Type)
	return nil
}

// DeleteIndex deletes an index. In this simulation, this is a no-op.
func (p *FAISSProvider) DeleteIndex(ctx context.Context, collection, name string) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	_, exists := p.indices[collection]
	if !exists {
		return fmt.Errorf("collection %s not found", collection)
	}

	p.logger.Info("DeleteIndex called for collection %s, index %s. "+
		"Note: This simulated provider uses a single brute-force index per collection.", collection, name)
	return nil
}

// ListIndexes lists indexes in a collection. Returns a single "flat" index.
func (p *FAISSProvider) ListIndexes(ctx context.Context, collection string) ([]*IndexInfo, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	index, exists := p.indices[collection]
	if !exists {
		return nil, fmt.Errorf("collection %s not found", collection)
	}

	return []*IndexInfo{
		{
			Name:      "flat_simulation",
			Type:      "Flat",
			State:     "ready",
			CreatedAt: index.createdAt,
			UpdatedAt: index.updatedAt,
			Metadata: map[string]interface{}{
				"simulation": true,
				"note":       "This is a simulated brute-force index, not native FAISS",
			},
		},
	}, nil
}

// AddMetadata adds metadata to a vector.
func (p *FAISSProvider) AddMetadata(ctx context.Context, id string, metadata map[string]interface{}) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for _, index := range p.indices {
		if err := index.addMetadata(id, metadata); err == nil {
			return nil
		}
	}

	return fmt.Errorf("vector with ID %s not found", id)
}

// UpdateMetadata updates vector metadata.
func (p *FAISSProvider) UpdateMetadata(ctx context.Context, id string, metadata map[string]interface{}) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for _, index := range p.indices {
		if err := index.updateMetadata(id, metadata); err == nil {
			return nil
		}
	}

	return fmt.Errorf("vector with ID %s not found", id)
}

// GetMetadata gets vector metadata.
func (p *FAISSProvider) GetMetadata(ctx context.Context, ids []string) (map[string]map[string]interface{}, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make(map[string]map[string]interface{})

	for _, index := range p.indices {
		metadata := index.getMetadata(ids)
		for id, data := range metadata {
			if _, exists := result[id]; !exists {
				result[id] = data
			}
		}
	}

	return result, nil
}

// DeleteMetadata deletes specific metadata keys from vectors.
func (p *FAISSProvider) DeleteMetadata(ctx context.Context, ids []string, keys []string) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for _, index := range p.indices {
		index.deleteMetadata(ids, keys)
	}

	return nil
}

// GetStats gets provider statistics.
func (p *FAISSProvider) GetStats(ctx context.Context) (*ProviderStats, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	uptime := p.stats.Uptime
	if p.started && !p.startTime.IsZero() {
		uptime = time.Since(p.startTime)
	}

	return &ProviderStats{
		Name:             "faiss_simulated",
		Type:             "faiss",
		Status:           p.getStatus(),
		TotalVectors:     p.stats.TotalVectors,
		TotalCollections: p.stats.TotalCollections,
		TotalSize:        p.stats.TotalSize,
		AverageLatency:   p.stats.AverageLatency,
		LastOperation:    p.stats.LastOperation,
		ErrorCount:       p.stats.ErrorCount,
		Uptime:           uptime,
		Metadata: map[string]interface{}{
			"simulation":    true,
			"gpu_available": false,
			"index_type":    "flat",
			"note":          SimulationNotice,
		},
	}, nil
}

// Optimize triggers index optimization. In this simulation, it persists data to disk.
func (p *FAISSProvider) Optimize(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.logger.Info("Optimize called - persisting all indices to disk")

	for name, index := range p.indices {
		if err := index.save(); err != nil {
			p.logger.Warn("Failed to persist index %s: %v", name, err)
		}
	}

	p.logger.Info("Optimization complete (data persisted to disk)")
	return nil
}

// Backup backs up the simulated FAISS provider to the specified path.
func (p *FAISSProvider) Backup(ctx context.Context, path string) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	backupPath := filepath.Join(path, fmt.Sprintf("faiss_backup_%s", time.Now().Format("20060102_150405")))

	if err := os.MkdirAll(backupPath, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	// Save each index to the backup path
	for name, index := range p.indices {
		indexBackupPath := filepath.Join(backupPath, name)
		if err := os.MkdirAll(indexBackupPath, 0755); err != nil {
			return fmt.Errorf("failed to create backup directory for index %s: %w", name, err)
		}

		if err := index.saveToPath(indexBackupPath); err != nil {
			return fmt.Errorf("failed to backup index %s: %w", name, err)
		}
	}

	p.logger.Info("Backup completed: path=%s indices=%d", backupPath, len(p.indices))
	return nil
}

// Restore restores the simulated FAISS provider from a backup.
func (p *FAISSProvider) Restore(ctx context.Context, path string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if err := p.loadExistingIndicesFromPath(ctx, path); err != nil {
		return fmt.Errorf("failed to restore from backup: %w", err)
	}

	p.logger.Info("Restore completed: path=%s indices=%d", path, len(p.indices))
	return nil
}

// Health checks the health of the simulated FAISS provider.
func (p *FAISSProvider) Health(ctx context.Context) (*HealthStatus, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	status := "healthy"
	message := "Simulated FAISS provider is operational"

	if !p.initialized {
		status = "not_initialized"
		message = "Provider has not been initialized"
	} else if !p.started {
		status = "not_started"
		message = "Provider is initialized but not started"
	}

	uptime := time.Duration(0)
	if p.started && !p.startTime.IsZero() {
		uptime = time.Since(p.startTime)
	}

	return &HealthStatus{
		Status:       status,
		Message:      message,
		Timestamp:    time.Now(),
		LastCheck:    time.Now(),
		ResponseTime: time.Millisecond, // Simulated response time
		Metrics: map[string]interface{}{
			"total_vectors":     p.stats.TotalVectors,
			"total_collections": p.stats.TotalCollections,
			"total_size_bytes":  p.stats.TotalSize,
			"uptime_seconds":    uptime.Seconds(),
		},
		Dependencies: map[string]string{
			"storage": "local_disk",
			"compute": "cpu",
		},
		Details: map[string]interface{}{
			"simulation":    true,
			"gpu_available": false,
			"native_faiss":  false,
		},
	}, nil
}

// GetName returns the provider name.
func (p *FAISSProvider) GetName() string {
	return "faiss"
}

// GetType returns the provider type.
func (p *FAISSProvider) GetType() string {
	return string(ProviderTypeFAISS)
}

// GetCapabilities returns provider capabilities.
// Note: Some capabilities like gpu_acceleration are listed for API compatibility but are not functional.
func (p *FAISSProvider) GetCapabilities() []string {
	return []string{
		"vector_storage",
		"vector_search",
		"metadata_filtering",
		"batch_operations",
		"collection_management",
		"backup_restore",
		"persistence",
		// Listed for API compatibility but not functional in simulation:
		// "gpu_acceleration", "ivf_indexing", "hnsw_indexing", "compression"
	}
}

// GetConfiguration returns provider configuration.
func (p *FAISSProvider) GetConfiguration() interface{} {
	return p.config
}

// IsCloud returns whether the provider is cloud-based.
func (p *FAISSProvider) IsCloud() bool {
	return false
}

// GetCostInfo returns cost information.
func (p *FAISSProvider) GetCostInfo() *CostInfo {
	return &CostInfo{
		StorageCost:   0.0, // Local storage, no direct cost
		ComputeCost:   0.0, // Local compute, no direct cost
		TransferCost:  0.0, // No data transfer costs
		TotalCost:     0.0,
		Currency:      "USD",
		BillingPeriod: "N/A",
		FreeTierUsed:  0.0,
		FreeTierLimit: 0.0,
	}
}

// Stop stops the simulated FAISS provider and persists data.
func (p *FAISSProvider) Stop(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.started {
		return nil
	}

	// Persist all indices to disk before stopping
	for name, index := range p.indices {
		if err := index.save(); err != nil {
			p.logger.Warn("Failed to save index %s: %v", name, err)
		}
	}

	p.started = false
	p.logger.Info("Simulated FAISS provider stopped")
	return nil
}

// Close closes the simulated FAISS provider.
func (p *FAISSProvider) Close(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.started && !p.initialized {
		return nil // Already closed
	}

	// Persist all indices to disk before closing
	for name, index := range p.indices {
		if err := index.save(); err != nil {
			p.logger.Warn("Failed to save index %s during close: %v", name, err)
		}
		// Clean up index resources
		index.mu.Lock()
		index.initialized = false
		index.vectorMap = nil
		index.metadataMap = nil
		index.timestamps = nil
		index.mu.Unlock()
	}

	p.started = false
	p.initialized = false
	p.indices = make(map[string]*FAISSIndex)
	p.collections = make(map[string]*CollectionConfig)

	p.logger.Info("Simulated FAISS provider closed successfully")
	return nil
}

// Helper methods

func (p *FAISSProvider) getStatus() string {
	if !p.initialized {
		return "not_initialized"
	}
	if !p.started {
		return "stopped"
	}
	return "running"
}

func (p *FAISSProvider) loadExistingIndices(ctx context.Context) error {
	return p.loadExistingIndicesFromPath(ctx, p.config.StoragePath)
}

func (p *FAISSProvider) loadExistingIndicesFromPath(ctx context.Context, storagePath string) error {
	entries, err := os.ReadDir(storagePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No existing indices
		}
		return fmt.Errorf("failed to read storage directory: %w", err)
	}

	loadedCount := 0
	for _, entry := range entries {
		if entry.IsDir() {
			indexPath := filepath.Join(storagePath, entry.Name())
			index := &FAISSIndex{
				name:        entry.Name(),
				indexPath:   indexPath,
				vectorMap:   make(map[string][]float64),
				metadataMap: make(map[string]map[string]interface{}),
				timestamps:  make(map[string]time.Time),
				initialized: false,
			}

			if err := index.load(); err != nil {
				p.logger.Warn("Failed to load index %s: %v", entry.Name(), err)
				continue
			}

			p.indices[entry.Name()] = index
			loadedCount++
		}
	}

	p.logger.Info("Loaded %d existing indices from %s", loadedCount, storagePath)
	return nil
}

func (p *FAISSProvider) getOrCreateIndex(ctx context.Context, collection string) (*FAISSIndex, error) {
	// Caller must hold p.mu (read or write lock)
	if index, exists := p.indices[collection]; exists {
		return index, nil
	}

	indexPath := filepath.Join(p.config.StoragePath, collection)
	index := &FAISSIndex{
		name:        collection,
		indexPath:   indexPath,
		dimension:   p.config.Dimension,
		metric:      p.config.Metric,
		vectorMap:   make(map[string][]float64),
		metadataMap: make(map[string]map[string]interface{}),
		timestamps:  make(map[string]time.Time),
		initialized: false,
		createdAt:   time.Now(),
		updatedAt:   time.Now(),
	}

	if err := index.initialize(p.config); err != nil {
		return nil, fmt.Errorf("failed to initialize index: %w", err)
	}

	p.indices[collection] = index
	return index, nil
}

func (p *FAISSProvider) updateStats(duration time.Duration) {
	p.stats.LastOperation = time.Now()

	// Update average latency (exponential moving average)
	if p.stats.AverageLatency == 0 {
		p.stats.AverageLatency = duration
	} else {
		alpha := 0.1 // Smoothing factor
		p.stats.AverageLatency = time.Duration(float64(p.stats.AverageLatency)*(1-alpha) + float64(duration)*alpha)
	}
}

// FAISSIndex helper methods

func (idx *FAISSIndex) initialize(config *FAISSConfig) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	if idx.initialized {
		return nil
	}

	// Create index directory
	if err := os.MkdirAll(idx.indexPath, 0755); err != nil {
		return fmt.Errorf("failed to create index directory: %w", err)
	}

	idx.dimension = config.Dimension
	idx.metric = config.Metric
	idx.createdAt = time.Now()
	idx.updatedAt = time.Now()
	idx.initialized = true
	return nil
}

func (idx *FAISSIndex) addVector(vector *VectorData) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	idx.vectorMap[vector.ID] = vector.Vector
	idx.metadataMap[vector.ID] = vector.Metadata
	idx.timestamps[vector.ID] = time.Now()
	idx.updatedAt = time.Now()
	return nil
}

func (idx *FAISSIndex) removeVector(id string) bool {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	if _, exists := idx.vectorMap[id]; exists {
		delete(idx.vectorMap, id)
		delete(idx.metadataMap, id)
		delete(idx.timestamps, id)
		idx.updatedAt = time.Now()
		return true
	}
	return false
}

func (idx *FAISSIndex) getVectors(ids []string) []*VectorData {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	var vectors []*VectorData

	for _, id := range ids {
		if vector, exists := idx.vectorMap[id]; exists {
			metadata := idx.metadataMap[id]
			timestamp := idx.timestamps[id]
			vectors = append(vectors, &VectorData{
				ID:         id,
				Vector:     vector,
				Metadata:   metadata,
				Collection: idx.name,
				Timestamp:  timestamp,
			})
		}
	}

	return vectors
}

// search performs brute-force similarity search across all vectors in the index.
// Results are sorted by similarity score in descending order and limited to TopK.
// Note: This is O(n) complexity. Native FAISS would use IVF/HNSW for faster search.
func (idx *FAISSIndex) search(query *VectorQuery, metric string) ([]*VectorSearchResultItem, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	// Handle empty query vector
	if len(query.Vector) == 0 {
		return []*VectorSearchResultItem{}, nil
	}

	// Collect all candidates with their scores
	candidates := make([]*VectorSearchResultItem, 0, len(idx.vectorMap))

	for id, vector := range idx.vectorMap {
		// Calculate similarity based on the configured metric
		var score float64
		var distance float64

		switch metric {
		case "euclidean", "l2":
			distance = calculateEuclideanDistance(query.Vector, vector)
			score = 1.0 / (1.0 + distance) // Convert distance to similarity
		case "dot", "inner_product":
			score = calculateDotProduct(query.Vector, vector)
			distance = -score // For dot product, higher is better
		default: // "cosine" is default
			score = calculateCosineSimilarity(query.Vector, vector)
			distance = 1.0 - score
		}

		// Apply threshold filter
		if query.Threshold > 0 && score < query.Threshold {
			continue
		}

		// Apply metadata filters if present
		if query.Filters != nil && len(query.Filters) > 0 {
			metadata := idx.metadataMap[id]
			if !faissMatchesFilters(metadata, query.Filters) {
				continue
			}
		}

		candidates = append(candidates, &VectorSearchResultItem{
			ID:       id,
			Vector:   vector,
			Score:    score,
			Distance: distance,
			Metadata: idx.metadataMap[id],
		})
	}

	// Sort by score descending (highest similarity first)
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})

	// Limit to TopK results
	topK := query.TopK
	if topK <= 0 {
		topK = 10 // Default to 10 results
	}
	if topK > len(candidates) {
		topK = len(candidates)
	}

	return candidates[:topK], nil
}

func (idx *FAISSIndex) vectorCount() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	return len(idx.vectorMap)
}

func (idx *FAISSIndex) addMetadata(id string, metadata map[string]interface{}) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	if _, exists := idx.vectorMap[id]; !exists {
		return fmt.Errorf("vector with ID %s not found", id)
	}

	if idx.metadataMap[id] == nil {
		idx.metadataMap[id] = make(map[string]interface{})
	}

	for k, v := range metadata {
		idx.metadataMap[id][k] = v
	}
	idx.updatedAt = time.Now()

	return nil
}

func (idx *FAISSIndex) updateMetadata(id string, metadata map[string]interface{}) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	if _, exists := idx.vectorMap[id]; !exists {
		return fmt.Errorf("vector with ID %s not found", id)
	}

	if idx.metadataMap[id] == nil {
		idx.metadataMap[id] = make(map[string]interface{})
	}

	for k, v := range metadata {
		idx.metadataMap[id][k] = v
	}
	idx.updatedAt = time.Now()

	return nil
}

func (idx *FAISSIndex) getMetadata(ids []string) map[string]map[string]interface{} {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	result := make(map[string]map[string]interface{})

	for _, id := range ids {
		if metadata, exists := idx.metadataMap[id]; exists {
			result[id] = metadata
		}
	}

	return result
}

func (idx *FAISSIndex) deleteMetadata(ids []string, keys []string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	for _, id := range ids {
		if metadata, exists := idx.metadataMap[id]; exists {
			for _, key := range keys {
				delete(metadata, key)
			}
		}
	}
	idx.updatedAt = time.Now()
}

func (idx *FAISSIndex) save() error {
	return idx.saveToPath(idx.indexPath)
}

func (idx *FAISSIndex) saveToPath(path string) error {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if err := os.MkdirAll(path, 0755); err != nil {
		return fmt.Errorf("failed to create index directory: %w", err)
	}

	data := indexPersistenceData{
		Name:      idx.name,
		Dimension: idx.dimension,
		Metric:    idx.metric,
		Vectors:   idx.vectorMap,
		Metadata:  idx.metadataMap,
		CreatedAt: idx.createdAt,
		UpdatedAt: time.Now(),
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal index data: %w", err)
	}

	persistPath := filepath.Join(path, "index.json")
	if err := os.WriteFile(persistPath, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write index file: %w", err)
	}

	return nil
}

func (idx *FAISSIndex) load() error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	persistPath := filepath.Join(idx.indexPath, "index.json")
	jsonData, err := os.ReadFile(persistPath)
	if err != nil {
		if os.IsNotExist(err) {
			// No persisted data, start fresh
			idx.initialized = true
			idx.createdAt = time.Now()
			idx.updatedAt = time.Now()
			return nil
		}
		return fmt.Errorf("failed to read index file: %w", err)
	}

	var data indexPersistenceData
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return fmt.Errorf("failed to unmarshal index data: %w", err)
	}

	idx.name = data.Name
	idx.dimension = data.Dimension
	idx.metric = data.Metric
	idx.vectorMap = data.Vectors
	idx.metadataMap = data.Metadata
	idx.createdAt = data.CreatedAt
	idx.updatedAt = data.UpdatedAt
	idx.initialized = true

	// Initialize timestamps map if not present
	if idx.timestamps == nil {
		idx.timestamps = make(map[string]time.Time)
	}

	return nil
}

// Similarity calculation functions

// calculateEuclideanDistance calculates the Euclidean (L2) distance between two vectors.
func calculateEuclideanDistance(a, b []float64) float64 {
	if len(a) != len(b) {
		return math.MaxFloat64
	}

	var sum float64
	for i := 0; i < len(a); i++ {
		diff := a[i] - b[i]
		sum += diff * diff
	}

	return math.Sqrt(sum)
}

// calculateDotProduct calculates the dot product (inner product) between two vectors.
func calculateDotProduct(a, b []float64) float64 {
	if len(a) != len(b) {
		return 0.0
	}

	var sum float64
	for i := 0; i < len(a); i++ {
		sum += a[i] * b[i]
	}

	return sum
}

// faissMatchesFilters checks if metadata matches the given filter criteria.
func faissMatchesFilters(metadata map[string]interface{}, filters map[string]interface{}) bool {
	if metadata == nil {
		return len(filters) == 0
	}

	for key, filterValue := range filters {
		metaValue, exists := metadata[key]
		if !exists {
			return false
		}

		// Simple equality check - extend for more complex filtering if needed
		if metaValue != filterValue {
			// Try type-flexible comparison
			switch fv := filterValue.(type) {
			case float64:
				if mv, ok := metaValue.(float64); ok && mv == fv {
					continue
				}
				if mv, ok := metaValue.(int); ok && float64(mv) == fv {
					continue
				}
			case int:
				if mv, ok := metaValue.(int); ok && mv == fv {
					continue
				}
				if mv, ok := metaValue.(float64); ok && int(mv) == fv {
					continue
				}
			case string:
				if mv, ok := metaValue.(string); ok && mv == fv {
					continue
				}
			}
			return false
		}
	}

	return true
}
