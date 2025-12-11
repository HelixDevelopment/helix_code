package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"dev.helix.code/internal/logging"
	"dev.helix.code/internal/memory"
)

// MemontoProvider implements memory operations using Memonto.ai knowledge graph
type MemontoProvider struct {
	config          map[string]interface{}
	logger          *logging.Logger
	userID          string
	apiKey          string
	baseURL         string
	httpClient      *http.Client
	mu              sync.RWMutex
	localCache      map[string][]byte
	collections     map[string]*CollectionInfo
	stats           *ProviderStats
	costTracker     *CostTracker
	lastHealthCheck time.Time
	pythonPath      string
	memontoDir      string
}

// Memonto API structures
type MemontoRequest struct {
	Action  string                 `json:"action"`
	UserID  string                 `json:"user_id"`
	Data    interface{}            `json:"data,omitempty"`
	Options map[string]interface{} `json:"options,omitempty"`
}

type MemontoResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Meta    *MemontoMeta `json:"meta,omitempty"`
}

type MemontoMeta struct {
	Nodes    int                    `json:"nodes"`
	Edges    int                    `json:"edges"`
	Concepts []string               `json:"concepts"`
	Timing   map[string]float64     `json:"timing"`
	Cost     float64                `json:"cost"`
}

type MemontoKnowledgeGraph struct {
	Nodes map[string]*MemontoNode `json:"nodes"`
	Edges []*MemontoEdge          `json:"edges"`
}

type MemontoNode struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	Label       string                 `json:"label"`
	Properties  map[string]interface{} `json:"properties"`
	Embedding   []float64              `json:"embedding,omitempty"`
	Created     time.Time              `json:"created"`
	Updated     time.Time              `json:"updated"`
}

type MemontoEdge struct {
	ID         string                 `json:"id"`
	Source     string                 `json:"source"`
	Target     string                 `json:"target"`
	Type       string                 `json:"type"`
	Weight     float64                `json:"weight"`
	Properties map[string]interface{} `json:"properties"`
	Created    time.Time              `json:"created"`
}

type MemontoConcept struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Properties  map[string]interface{} `json:"properties"`
	Embedding   []float64              `json:"embedding"`
}

// CostTracker tracks API costs
type CostTracker struct {
	TotalCost      float64 `json:"total_cost"`
	Operations     int     `json:"operations"`
	NodesCreated   int     `json:"nodes_created"`
	EdgesCreated   int     `json:"edges_created"`
	ConceptsLearned int     `json:"concepts_learned"`
}

// NewMemontoProvider creates a new Memonto provider instance
func NewMemontoProvider(config map[string]interface{}) (*MemontoProvider, error) {
	provider := &MemontoProvider{
		config:      config,
		logger:      logging.NewLoggerWithName("memonto_provider"),
		httpClient:  &http.Client{Timeout: 30 * time.Second},
		localCache:  make(map[string][]byte),
		collections: make(map[string]*CollectionInfo),
		stats: &ProviderStats{
			Name:             "Memonto",
			Type:             "memonto",
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
		costTracker: &CostTracker{},
	}

	// Extract configuration
	if apiKey, ok := config["api_key"].(string); ok {
		provider.apiKey = apiKey
	}

	if baseURL, ok := config["base_url"].(string); ok {
		provider.baseURL = strings.TrimSuffix(baseURL, "/")
	} else {
		provider.baseURL = "https://api.memonto.ai/v1"
	}

	if userID, ok := config["user_id"].(string); ok {
		provider.userID = userID
	} else {
		provider.userID = "default"
	}

	if pythonPath, ok := config["python_path"].(string); ok {
		provider.pythonPath = pythonPath
	} else {
		// Try to find Python
		if python, err := exec.LookPath("python3"); err == nil {
			provider.pythonPath = python
		} else if python, err := exec.LookPath("python"); err == nil {
			provider.pythonPath = python
		}
	}

	if memontoDir, ok := config["memonto_dir"].(string); ok {
		provider.memontoDir = memontoDir
	} else {
		// Default to local directory
		provider.memontoDir = filepath.Join(os.TempDir(), "memonto")
	}

	// Initialize with default collection
	defaultCollection := &CollectionInfo{
		Name:        "default",
		VectorCount: 0,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Config: &CollectionConfig{
			Dimension: 1536, // Default embedding dimension
			Metric:    "cosine",
		},
	}
	provider.collections["default"] = defaultCollection
	provider.stats.TotalCollections = 1

	return provider, nil
}

// GetType returns the provider type
func (p *MemontoProvider) GetType() string {
	return string(memory.ProviderTypeMemonto)
}

// GetName returns the provider name
func (p *MemontoProvider) GetName() string {
	return "Memonto"
}

// GetCapabilities returns provider capabilities
func (p *MemontoProvider) GetCapabilities() []string {
	return []string{
		"memory_storage",
		"memory_retrieval",
		"memory_search",
		"context_management",
		"graph_memory",
		"knowledge_graph",
		"ontology_management",
		"concept_learning",
		"relationship_mapping",
		"metadata_operations",
		"cost_tracking",
	}
}

// GetConfiguration returns provider configuration
func (p *MemontoProvider) GetConfiguration() interface{} {
	return p.config
}

// IsCloud returns whether this is a cloud provider
func (p *MemontoProvider) IsCloud() bool {
	return p.baseURL != "" && !strings.Contains(p.baseURL, "localhost")
}

// Store stores memory data in Memonto
func (p *MemontoProvider) Store(ctx context.Context, data []*VectorData) error {
	if len(data) == 0 {
		return nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	start := time.Now()
	p.stats.TotalOperations++

	// Convert to knowledge graph format
	knowledgeGraph := &MemontoKnowledgeGraph{
		Nodes: make(map[string]*MemontoNode),
		Edges: []*MemontoEdge{},
	}

	for _, item := range data {
		// Create main node
		node := &MemontoNode{
			ID:         item.ID,
			Type:       "memory",
			Label:      getStringFromMetadata(item.Metadata, "title"),
			Properties: item.Metadata,
			Embedding:  item.Vector,
			Created:    time.Now(),
			Updated:    time.Now(),
		}
		if node.Label == "" {
			node.Label = fmt.Sprintf("Memory %s", item.ID[:8])
		}
		knowledgeGraph.Nodes[item.ID] = node

		// Extract concepts and create relationships
		if content, ok := item.Metadata["content"].(string); ok {
			concepts := p.extractConcepts(content)
			for _, concept := range concepts {
				// Create concept node if not exists
				conceptID := fmt.Sprintf("concept_%s", concept)
				if _, exists := knowledgeGraph.Nodes[conceptID]; !exists {
					conceptNode := &MemontoNode{
						ID:     conceptID,
						Type:   "concept",
						Label:  concept,
						Properties: map[string]interface{}{
							"type": "concept",
							"name": concept,
						},
						Created: time.Now(),
						Updated: time.Now(),
					}
					knowledgeGraph.Nodes[conceptID] = conceptNode
				}

				// Create relationship edge
				edge := &MemontoEdge{
					ID:     fmt.Sprintf("rel_%s_%s", item.ID, conceptID),
					Source: item.ID,
					Target: conceptID,
					Type:   "mentions",
					Weight: 1.0,
					Properties: map[string]interface{}{
						"relationship_type": "mentions",
						"confidence": 0.8,
					},
					Created: time.Now(),
				}
				knowledgeGraph.Edges = append(knowledgeGraph.Edges, edge)
			}
		}
	}

	// Prepare API request
	req := MemontoRequest{
		Action: "store_knowledge",
		UserID: p.userID,
		Data:   knowledgeGraph,
		Options: map[string]interface{}{
			"extract_concepts": true,
			"create_edges":     true,
			"learn_patterns":   true,
		},
	}

	// Call Memonto API
	resp, cost, err := p.callAPI(ctx, req)
	if err != nil {
		p.stats.FailedOps++
		p.stats.Status = "error"
		p.logger.Error("Memonto Store failed: %v", err)
		return fmt.Errorf("Memonto Store failed: %v", err)
	}

	// Update local cache and stats
	if resp.Success {
		p.stats.SuccessfulOps++
		p.stats.TotalVectors += int64(len(data))
		p.stats.AverageLatency = time.Duration(float64(p.stats.AverageLatency) + float64(time.Since(start))/2)
		p.costTracker.TotalCost += cost
		p.costTracker.Operations++
		p.costTracker.NodesCreated += len(knowledgeGraph.Nodes)
		p.costTracker.EdgesCreated += len(knowledgeGraph.Edges)
		p.stats.Status = "active"

		// Update collection info
		if coll, exists := p.collections["default"]; exists {
			coll.VectorCount += int64(len(data))
			coll.UpdatedAt = time.Now()
		}

		p.logger.Info("Successfully stored %d memories to Memonto (%d nodes, %d edges)", 
			len(data), len(knowledgeGraph.Nodes), len(knowledgeGraph.Edges))
		return nil
	} else {
		p.stats.FailedOps++
		p.stats.Status = "error"
		return fmt.Errorf("Memonto API error: %s", resp.Error)
	}
}

// Search searches for memory in Memonto
func (p *MemontoProvider) Search(ctx context.Context, query *VectorQuery) (*VectorSearchResult, error) {
	if query == nil {
		return nil, fmt.Errorf("query cannot be nil")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	start := time.Now()
	p.stats.TotalOperations++

	// Prepare API request
	req := MemontoRequest{
		Action: "search_knowledge",
		UserID: p.userID,
		Data: map[string]interface{}{
			"query":      query.Text,
			"vector":     query.Vector,
			"top_k":      query.TopK,
			"filters":    query.Filters,
			"search_type": "semantic",
		},
		Options: map[string]interface{}{
			"include_concepts":   true,
			"include_relations": true,
			"similarity_threshold": 0.7,
		},
	}

	// Call Memonto API
	resp, cost, err := p.callAPI(ctx, req)
	if err != nil {
		p.stats.FailedOps++
		p.logger.Error("Memonto Search failed: %v", err)
		return &VectorSearchResult{Results: []*VectorSearchResultItem{}}, nil
	}

	p.costTracker.TotalCost += cost
	p.costTracker.Operations++

	// Convert results
	result := &VectorSearchResult{
		Results: []*VectorSearchResultItem{},
	}

	if resp.Success && resp.Data != nil {
		if data, ok := resp.Data.(map[string]interface{}); ok {
			if memories, ok := data["memories"].([]interface{}); ok {
				for _, item := range memories {
					if resultMap, ok := item.(map[string]interface{}); ok {
						resultItem := &VectorSearchResultItem{
							ID:       getStringFromMap(resultMap, "id"),
							Score:    getFloatFromMap(resultMap, "similarity"),
							Metadata: getMapFromMap(resultMap, "metadata"),
						}
						result.Results = append(result.Results, resultItem)
					}
				}
			}
		}
	}

	p.stats.SuccessfulOps++
	p.stats.AverageLatency = time.Duration(float64(p.stats.AverageLatency) + float64(time.Since(start))/2)
	p.logger.Info("Memonto Search returned %d results", len(result.Results))

	return result, nil
}

// Retrieve retrieves vectors by IDs from Memonto
func (p *MemontoProvider) Retrieve(ctx context.Context, ids []string) ([]*VectorData, error) {
	if len(ids) == 0 {
		return []*VectorData{}, nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	start := time.Now()
	p.stats.TotalOperations++

	// Prepare API request
	req := MemontoRequest{
		Action: "retrieve_memories",
		UserID: p.userID,
		Data: map[string]interface{}{
			"ids": ids,
		},
		Options: map[string]interface{}{
			"include_concepts":   true,
			"include_relations": true,
		},
	}

	// Call Memonto API
	resp, cost, err := p.callAPI(ctx, req)
	if err != nil {
		p.stats.FailedOps++
		p.logger.Error("Memonto Retrieve failed: %v", err)
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
						Vector:   getFloatSliceFromMap(resultMap, "embedding"),
						Metadata: getMapFromMap(resultMap, "properties"),
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

// Update updates a vector in Memonto
func (p *MemontoProvider) Update(ctx context.Context, id string, vector *VectorData) error {
	if vector == nil {
		return fmt.Errorf("vector data cannot be nil")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	start := time.Now()
	p.stats.TotalOperations++

	// Prepare API request
	req := MemontoRequest{
		Action: "update_memory",
		UserID: p.userID,
		Data: map[string]interface{}{
			"id":         id,
			"properties": vector.Metadata,
			"embedding":  vector.Vector,
		},
		Options: map[string]interface{}{
			"update_concepts": true,
			"relearn_patterns": true,
		},
	}

	// Call Memonto API
	resp, cost, err := p.callAPI(ctx, req)
	if err != nil {
		p.stats.FailedOps++
		p.logger.Error("Memonto Update failed: %v", err)
		return fmt.Errorf("Memonto Update failed: %v", err)
	}

	p.costTracker.TotalCost += cost
	p.costTracker.Operations++

	if resp.Success {
		p.stats.SuccessfulOps++
		p.stats.AverageLatency = time.Duration(float64(p.stats.AverageLatency) + float64(time.Since(start))/2)
		p.logger.Info("Successfully updated memory %s in Memonto", id)
		return nil
	} else {
		p.stats.FailedOps++
		return fmt.Errorf("Memonto API error: %s", resp.Error)
	}
}

// Delete deletes memory from Memonto
func (p *MemontoProvider) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	start := time.Now()
	p.stats.TotalOperations++

	// Prepare API request
	req := MemontoRequest{
		Action: "delete_memories",
		UserID: p.userID,
		Data: map[string]interface{}{
			"ids": ids,
		},
		Options: map[string]interface{}{
			"cascade_delete": true,
			"cleanup_edges":  true,
		},
	}

	// Call Memonto API
	resp, cost, err := p.callAPI(ctx, req)
	if err != nil {
		p.stats.FailedOps++
		p.logger.Error("Memonto Delete failed: %v", err)
		return fmt.Errorf("Memonto Delete failed: %v", err)
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

		p.logger.Info("Successfully deleted %d memories from Memonto", len(ids))
		return nil
	} else {
		p.stats.FailedOps++
		return fmt.Errorf("Memonto API error: %s", resp.Error)
	}
}

// FindSimilar finds similar vectors in Memonto
func (p *MemontoProvider) FindSimilar(ctx context.Context, embedding []float64, k int, filters map[string]interface{}) ([]*VectorSimilarityResult, error) {
	if len(embedding) == 0 {
		return []*VectorSimilarityResult{}, nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	start := time.Now()
	p.stats.TotalOperations++

	// Prepare API request
	req := MemontoRequest{
		Action: "find_similar",
		UserID: p.userID,
		Data: map[string]interface{}{
			"embedding": embedding,
			"top_k":     k,
			"filters":   filters,
		},
		Options: map[string]interface{}{
			"similarity_threshold": 0.7,
			"include_concepts":      true,
		},
	}

	// Call Memonto API
	resp, cost, err := p.callAPI(ctx, req)
	if err != nil {
		p.stats.FailedOps++
		p.logger.Error("Memonto FindSimilar failed: %v", err)
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
						Score:    getFloatFromMap(resultMap, "similarity"),
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

// BatchFindSimilar finds similar vectors for multiple queries in Memonto
func (p *MemontoProvider) BatchFindSimilar(ctx context.Context, queries [][]float64, k int) ([][]*VectorSimilarityResult, error) {
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

// CreateCollection creates a collection in Memonto
func (p *MemontoProvider) CreateCollection(ctx context.Context, name string, config *CollectionConfig) error {
	if name == "" {
		return fmt.Errorf("collection name cannot be empty")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	start := time.Now()
	p.stats.TotalOperations++

	// Prepare API request
	req := MemontoRequest{
		Action: "create_domain",
		UserID: p.userID,
		Data: map[string]interface{}{
			"name":        name,
			"description": fmt.Sprintf("Memory domain for %s", name),
			"config":      config,
		},
	}

	// Call Memonto API
	resp, cost, err := p.callAPI(ctx, req)
	if err != nil {
		p.stats.FailedOps++
		p.logger.Error("Memonto CreateCollection failed: %v", err)
		return fmt.Errorf("Memonto CreateCollection failed: %v", err)
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
		p.logger.Info("Successfully created collection %s in Memonto", name)
		return nil
	} else {
		p.stats.FailedOps++
		return fmt.Errorf("Memonto API error: %s", resp.Error)
	}
}

// DeleteCollection deletes a collection in Memonto
func (p *MemontoProvider) DeleteCollection(ctx context.Context, name string) error {
	if name == "" {
		return fmt.Errorf("collection name cannot be empty")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	start := time.Now()
	p.stats.TotalOperations++

	// Prepare API request
	req := MemontoRequest{
		Action: "delete_domain",
		UserID: p.userID,
		Data: map[string]interface{}{
			"name": name,
		},
		Options: map[string]interface{}{
			"cascade_delete": true,
		},
	}

	// Call Memonto API
	resp, cost, err := p.callAPI(ctx, req)
	if err != nil {
		p.stats.FailedOps++
		p.logger.Error("Memonto DeleteCollection failed: %v", err)
		return fmt.Errorf("Memonto DeleteCollection failed: %v", err)
	}

	p.costTracker.TotalCost += cost
	p.costTracker.Operations++

	if resp.Success {
		// Remove from local collections
		delete(p.collections, name)
		p.stats.TotalCollections--

		p.stats.SuccessfulOps++
		p.stats.AverageLatency = time.Duration(float64(p.stats.AverageLatency) + float64(time.Since(start))/2)
		p.logger.Info("Successfully deleted collection %s from Memonto", name)
		return nil
	} else {
		p.stats.FailedOps++
		return fmt.Errorf("Memonto API error: %s", resp.Error)
	}
}

// ListCollections lists collections in Memonto
func (p *MemontoProvider) ListCollections(ctx context.Context) ([]*CollectionInfo, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	start := time.Now()
	p.stats.TotalOperations++

	// Prepare API request
	req := MemontoRequest{
		Action: "list_domains",
		UserID: p.userID,
	}

	// Call Memonto API
	resp, cost, err := p.callAPI(ctx, req)
	if err != nil {
		p.stats.FailedOps++
		p.logger.Error("Memonto ListCollections failed: %v", err)
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
						VectorCount: int64(getIntFromMap(resultMap, "memory_count")),
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

// GetCollection gets collection info in Memonto
func (p *MemontoProvider) GetCollection(ctx context.Context, name string) (*CollectionInfo, error) {
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
	req := MemontoRequest{
		Action: "get_domain",
		UserID: p.userID,
		Data: map[string]interface{}{
			"name": name,
		},
	}

	// Call Memonto API
	resp, cost, err := p.callAPI(ctx, req)
	if err != nil {
		p.stats.FailedOps++
		p.logger.Error("Memonto GetCollection failed: %v", err)
		return nil, fmt.Errorf("Memonto GetCollection failed: %v", err)
	}

	p.costTracker.TotalCost += cost
	p.costTracker.Operations++

	if resp.Success && resp.Data != nil {
		if resultMap, ok := resp.Data.(map[string]interface{}); ok {
			collectionInfo := &CollectionInfo{
				Name:        getStringFromMap(resultMap, "name"),
				VectorCount: int64(getIntFromMap(resultMap, "memory_count")),
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

// CreateIndex creates an index in Memonto
func (p *MemontoProvider) CreateIndex(ctx context.Context, collection string, config *IndexConfig) error {
	if collection == "" {
		return fmt.Errorf("collection name cannot be empty")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	start := time.Now()
	p.stats.TotalOperations++

	// Prepare API request
	req := MemontoRequest{
		Action: "create_index",
		UserID: p.userID,
		Data: map[string]interface{}{
			"domain": collection,
			"field":  getStringFromConfigMemonto(config, "field"),
			"type":   getStringFromConfigMemonto(config, "type"),
			"metric": config.Metric,
		},
	}

	// Call Memonto API
	resp, cost, err := p.callAPI(ctx, req)
	if err != nil {
		p.stats.FailedOps++
		p.logger.Error("Memonto CreateIndex failed: %v", err)
		return fmt.Errorf("Memonto CreateIndex failed: %v", err)
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
		return fmt.Errorf("Memonto API error: %s", resp.Error)
	}
}

// DeleteIndex deletes an index in Memonto
func (p *MemontoProvider) DeleteIndex(ctx context.Context, collection, name string) error {
	if collection == "" || name == "" {
		return fmt.Errorf("collection name and index name cannot be empty")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	start := time.Now()
	p.stats.TotalOperations++

	// Prepare API request
	req := MemontoRequest{
		Action: "delete_index",
		UserID: p.userID,
		Data: map[string]interface{}{
			"domain": collection,
			"name":   name,
		},
	}

	// Call Memonto API
	resp, cost, err := p.callAPI(ctx, req)
	if err != nil {
		p.stats.FailedOps++
		p.logger.Error("Memonto DeleteIndex failed: %v", err)
		return fmt.Errorf("Memonto DeleteIndex failed: %v", err)
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
		return fmt.Errorf("Memonto API error: %s", resp.Error)
	}
}

// ListIndexes lists indexes in Memonto
func (p *MemontoProvider) ListIndexes(ctx context.Context, collection string) ([]*IndexInfo, error) {
	if collection == "" {
		return []*IndexInfo{}, nil
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	start := time.Now()
	p.stats.TotalOperations++

	// Prepare API request
	req := MemontoRequest{
		Action: "list_indexes",
		UserID: p.userID,
		Data: map[string]interface{}{
			"domain": collection,
		},
	}

	// Call Memonto API
	resp, cost, err := p.callAPI(ctx, req)
	if err != nil {
		p.stats.FailedOps++
		p.logger.Error("Memonto ListIndexes failed: %v", err)
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

// AddMetadata adds metadata to a vector in Memonto
func (p *MemontoProvider) AddMetadata(ctx context.Context, id string, metadata map[string]interface{}) error {
	if id == "" || len(metadata) == 0 {
		return fmt.Errorf("id and metadata cannot be empty")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	start := time.Now()
	p.stats.TotalOperations++

	// Prepare API request
	req := MemontoRequest{
		Action: "add_metadata",
		UserID: p.userID,
		Data: map[string]interface{}{
			"id":       id,
			"metadata": metadata,
		},
	}

	// Call Memonto API
	resp, cost, err := p.callAPI(ctx, req)
	if err != nil {
		p.stats.FailedOps++
		p.logger.Error("Memonto AddMetadata failed: %v", err)
		return fmt.Errorf("Memonto AddMetadata failed: %v", err)
	}

	p.costTracker.TotalCost += cost
	p.costTracker.Operations++

	if resp.Success {
		p.stats.SuccessfulOps++
		p.stats.AverageLatency = time.Duration(float64(p.stats.AverageLatency) + float64(time.Since(start))/2)
		p.logger.Info("Successfully added metadata to memory %s in Memonto", id)
		return nil
	} else {
		p.stats.FailedOps++
		return fmt.Errorf("Memonto API error: %s", resp.Error)
	}
}

// UpdateMetadata updates metadata for a vector in Memonto
func (p *MemontoProvider) UpdateMetadata(ctx context.Context, id string, metadata map[string]interface{}) error {
	if id == "" || len(metadata) == 0 {
		return fmt.Errorf("id and metadata cannot be empty")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	start := time.Now()
	p.stats.TotalOperations++

	// Prepare API request
	req := MemontoRequest{
		Action: "update_metadata",
		UserID: p.userID,
		Data: map[string]interface{}{
			"id":       id,
			"metadata": metadata,
		},
	}

	// Call Memonto API
	resp, cost, err := p.callAPI(ctx, req)
	if err != nil {
		p.stats.FailedOps++
		p.logger.Error("Memonto UpdateMetadata failed: %v", err)
		return fmt.Errorf("Memonto UpdateMetadata failed: %v", err)
	}

	p.costTracker.TotalCost += cost
	p.costTracker.Operations++

	if resp.Success {
		p.stats.SuccessfulOps++
		p.stats.AverageLatency = time.Duration(float64(p.stats.AverageLatency) + float64(time.Since(start))/2)
		p.logger.Info("Successfully updated metadata for memory %s in Memonto", id)
		return nil
	} else {
		p.stats.FailedOps++
		return fmt.Errorf("Memonto API error: %s", resp.Error)
	}
}

// GetMetadata gets metadata for vectors in Memonto
func (p *MemontoProvider) GetMetadata(ctx context.Context, ids []string) (map[string]map[string]interface{}, error) {
	if len(ids) == 0 {
		return map[string]map[string]interface{}{}, nil
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	start := time.Now()
	p.stats.TotalOperations++

	// Prepare API request
	req := MemontoRequest{
		Action: "get_metadata",
		UserID: p.userID,
		Data: map[string]interface{}{
			"ids": ids,
		},
	}

	// Call Memonto API
	resp, cost, err := p.callAPI(ctx, req)
	if err != nil {
		p.stats.FailedOps++
		p.logger.Error("Memonto GetMetadata failed: %v", err)
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

// DeleteMetadata deletes metadata from vectors in Memonto
func (p *MemontoProvider) DeleteMetadata(ctx context.Context, ids []string, keys []string) error {
	if len(ids) == 0 || len(keys) == 0 {
		return nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	start := time.Now()
	p.stats.TotalOperations++

	// Prepare API request
	req := MemontoRequest{
		Action: "delete_metadata",
		UserID: p.userID,
		Data: map[string]interface{}{
			"ids":  ids,
			"keys": keys,
		},
	}

	// Call Memonto API
	resp, cost, err := p.callAPI(ctx, req)
	if err != nil {
		p.stats.FailedOps++
		p.logger.Error("Memonto DeleteMetadata failed: %v", err)
		return fmt.Errorf("Memonto DeleteMetadata failed: %v", err)
	}

	p.costTracker.TotalCost += cost
	p.costTracker.Operations++

	if resp.Success {
		p.stats.SuccessfulOps++
		p.stats.AverageLatency = time.Duration(float64(p.stats.AverageLatency) + float64(time.Since(start))/2)
		p.logger.Info("Successfully deleted metadata from %d memories in Memonto", len(ids))
		return nil
	} else {
		p.stats.FailedOps++
		return fmt.Errorf("Memonto API error: %s", resp.Error)
	}
}

// GetStats returns provider statistics
func (p *MemontoProvider) GetStats(ctx context.Context) (*ProviderStats, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Copy current stats
	stats := *p.stats
	stats.LastHealthCheck = time.Now()

	return &stats, nil
}

// Optimize optimizes the Memonto provider
func (p *MemontoProvider) Optimize(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	start := time.Now()
	p.stats.TotalOperations++

	// Prepare API request
	req := MemontoRequest{
		Action: "optimize_knowledge",
		UserID: p.userID,
		Options: map[string]interface{}{
			"compact_graph": true,
			"merge_nodes":   true,
			"cleanup_edges":  true,
			"relearn_patterns": true,
		},
	}

	// Call Memonto API
	resp, cost, err := p.callAPI(ctx, req)
	if err != nil {
		p.stats.FailedOps++
		p.logger.Error("Memonto Optimize failed: %v", err)
		return fmt.Errorf("Memonto Optimize failed: %v", err)
	}

	p.costTracker.TotalCost += cost
	p.costTracker.Operations++

	if resp.Success {
		p.stats.SuccessfulOps++
		p.stats.AverageLatency = time.Duration(float64(p.stats.AverageLatency) + float64(time.Since(start))/2)
		p.logger.Info("Memonto optimization completed successfully")
		return nil
	} else {
		p.stats.FailedOps++
		return fmt.Errorf("Memonto API error: %s", resp.Error)
	}
}

// Backup backs up data in Memonto
func (p *MemontoProvider) Backup(ctx context.Context, path string) error {
	if path == "" {
		return fmt.Errorf("backup path cannot be empty")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	start := time.Now()
	p.stats.TotalOperations++

	// Prepare API request
	req := MemontoRequest{
		Action: "export_knowledge",
		UserID: p.userID,
		Options: map[string]interface{}{
			"format":      "json",
			"export_path": path,
			"include_edges": true,
			"include_concepts": true,
		},
	}

	// Call Memonto API
	resp, cost, err := p.callAPI(ctx, req)
	if err != nil {
		p.stats.FailedOps++
		p.logger.Error("Memonto Backup failed: %v", err)
		return fmt.Errorf("Memonto Backup failed: %v", err)
	}

	p.costTracker.TotalCost += cost
	p.costTracker.Operations++

	if resp.Success {
		p.stats.SuccessfulOps++
		p.stats.AverageLatency = time.Duration(float64(p.stats.AverageLatency) + float64(time.Since(start))/2)
		p.logger.Info("Memonto backup completed successfully to %s", path)
		return nil
	} else {
		p.stats.FailedOps++
		return fmt.Errorf("Memonto API error: %s", resp.Error)
	}
}

// Restore restores data in Memonto
func (p *MemontoProvider) Restore(ctx context.Context, path string) error {
	if path == "" {
		return fmt.Errorf("restore path cannot be empty")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	start := time.Now()
	p.stats.TotalOperations++

	// Prepare API request
	req := MemontoRequest{
		Action: "import_knowledge",
		UserID: p.userID,
		Options: map[string]interface{}{
			"import_path":      path,
			"merge_existing":   true,
			"create_edges":     true,
			"learn_concepts":    true,
		},
	}

	// Call Memonto API
	resp, cost, err := p.callAPI(ctx, req)
	if err != nil {
		p.stats.FailedOps++
		p.logger.Error("Memonto Restore failed: %v", err)
		return fmt.Errorf("Memonto Restore failed: %v", err)
	}

	p.costTracker.TotalCost += cost
	p.costTracker.Operations++

	if resp.Success {
		p.stats.SuccessfulOps++
		p.stats.AverageLatency = time.Duration(float64(p.stats.AverageLatency) + float64(time.Since(start))/2)
		p.logger.Info("Memonto restore completed successfully from %s", path)
		return nil
	} else {
		p.stats.FailedOps++
		return fmt.Errorf("Memonto API error: %s", resp.Error)
	}
}

// Initialize initializes the Memonto provider
func (p *MemontoProvider) Initialize(ctx context.Context, config interface{}) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Create memonto directory if not exists
	if err := os.MkdirAll(p.memontoDir, 0755); err != nil {
		p.logger.Error("Failed to create memonto directory: %v", err)
		return fmt.Errorf("failed to create memonto directory: %v", err)
	}

	p.stats.Status = "initialized"
	p.logger.Info("Memonto provider initialized")
	return nil
}

// Start starts the Memonto provider
func (p *MemontoProvider) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Test connection to Memonto API
	testReq := MemontoRequest{
		Action: "health",
		UserID: p.userID,
	}

	_, cost, err := p.callAPI(ctx, testReq)
	if err != nil {
		p.stats.Status = "error"
		p.logger.Error("Memonto connection test failed: %v", err)
		return fmt.Errorf("Memonto connection test failed: %v", err)
	}

	p.costTracker.TotalCost += cost
	p.costTracker.Operations++
	p.stats.Status = "active"
	p.logger.Info("Memonto provider started successfully")

	return nil
}

// Stop stops the Memonto provider
func (p *MemontoProvider) Stop(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.stats.Status = "stopped"
	p.logger.Info("Memonto provider stopped")
	return nil
}

// Health checks provider health
func (p *MemontoProvider) Health(ctx context.Context) (*HealthStatus, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	start := time.Now()
	p.lastHealthCheck = time.Now()

	// Check connection to Memonto API
	testReq := MemontoRequest{
		Action: "health",
		UserID: p.userID,
	}

	_, cost, err := p.callAPI(ctx, testReq)
	if err != nil {
		p.stats.Status = "unhealthy"
		return &HealthStatus{
			Status:    "unhealthy",
			Message:   fmt.Sprintf("Memonto health check failed: %v", err),
			Timestamp: time.Now(),
		}, nil
	}

	p.costTracker.TotalCost += cost
	p.costTracker.Operations++
	p.stats.Status = "healthy"

	return &HealthStatus{
		Status:       "healthy",
		Message:      "Memonto API is responding",
		Timestamp:    time.Now(),
		LastCheck:    time.Now(),
		ResponseTime: time.Since(start),
		Details: map[string]interface{}{
			"total_operations":   p.stats.TotalOperations,
			"total_cost":         p.costTracker.TotalCost,
			"nodes_created":       p.costTracker.NodesCreated,
			"edges_created":       p.costTracker.EdgesCreated,
			"concepts_learned":    p.costTracker.ConceptsLearned,
			"collections_count":   len(p.collections),
			"vectors_count":       p.stats.TotalVectors,
		},
	}, nil
}

// Close closes the provider
func (p *MemontoProvider) Close(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.stats.Status = "closed"
	p.logger.Info("Memonto provider closed")
	return nil
}

// GetCostInfo returns cost information for Memonto
func (p *MemontoProvider) GetCostInfo() *CostInfo {
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
			"operations": float64(p.costTracker.Operations),
		},
	}
}

// callAPI makes an API call to Memonto
func (p *MemontoProvider) callAPI(ctx context.Context, req MemontoRequest) (*MemontoResponse, float64, error) {
	start := time.Now()

	// Marshal request
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, 0.0, fmt.Errorf("failed to marshal request: %v", err)
	}

	// Create HTTP request
	url := fmt.Sprintf("%s/knowledge", p.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, 0.0, fmt.Errorf("failed to create request: %v", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	}
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
	var apiResp MemontoResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, 0.0, fmt.Errorf("failed to parse response: %v", err)
	}

	// Calculate cost
	cost := float64(0.001) // Base cost per operation
	if apiResp.Meta != nil {
		cost = apiResp.Meta.Cost
	}

	p.logger.Debug("Memonto API call completed in %v, cost: $%.6f", time.Since(start), cost)

	return &apiResp, cost, nil
}

// extractConcepts extracts concepts from text
func (p *MemontoProvider) extractConcepts(text string) []string {
	// Simple concept extraction - in a real implementation, this would use NLP
	words := strings.Fields(strings.ToLower(text))
	concepts := []string{}
	seen := make(map[string]bool)

	// Filter out common words and extract potential concepts
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "and": true, "or": true, "but": true,
		"in": true, "on": true, "at": true, "to": true, "for": true, "of": true,
		"with": true, "by": true, "is": true, "are": true, "was": true, "were": true,
		"be": true, "been": true, "being": true, "have": true, "has": true, "had": true,
		"do": true, "does": true, "did": true, "will": true, "would": true, "could": true,
		"should": true, "may": true, "might": true, "shall": true,
		"can": true, "cannot": true, "must": true, "i": true, "you": true, "he": true,
		"she": true, "it": true, "we": true, "they": true, "me": true, "him": true,
		"her": true, "us": true, "them": true, "my": true, "your": true, "his": true,
		"its": true, "our": true, "their": true, "this": true, "that": true, "these": true,
		"those": true, "what": true, "which": true, "who": true, "when": true, "where": true,
		"why": true, "how": true, "all": true, "both": true, "each": true, "few": true,
		"more": true, "most": true, "other": true, "some": true, "such": true, "no": true,
		"nor": true, "not": true, "only": true, "own": true, "same": true, "so": true,
		"than": true, "too": true, "very": true, "just": true, "now": true, "also": true,
	}

	for _, word := range words {
		// Clean word
		word = strings.Trim(word, ".,!?;:\"'()[]{}")
		if len(word) < 3 || stopWords[word] || seen[word] {
			continue
		}

		// Add as concept
		concepts = append(concepts, word)
		seen[word] = true

		// Limit concepts
		if len(concepts) >= 10 {
			break
		}
	}

	return concepts
}

// getStringFromConfig safely extracts a string value from config for Memonto
func getStringFromConfigMemonto(config *IndexConfig, key string) string {
	if config == nil {
		return ""
	}
	
	switch key {
	case "field":
		// For Memonto, field is typically "vector" by default
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