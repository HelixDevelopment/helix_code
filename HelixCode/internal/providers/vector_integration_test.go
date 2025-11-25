package providers

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// VectorIntegration Tests
// =============================================================================

func TestNewVectorIntegration(t *testing.T) {
	config := &VectorConfig{
		DefaultProvider: "test-provider",
		Providers:       make(map[string]*SingleProviderConfigWrapper),
		LoadBalancing:   "",
		FailoverEnabled: true,
	}

	vi := NewVectorIntegration(config)

	require.NotNil(t, vi)
	assert.NotNil(t, vi.registry)
	assert.NotNil(t, vi.logger)
	assert.NotNil(t, vi.providers)
	assert.Equal(t, "test-provider", vi.config.DefaultProvider)
}

func TestNewVectorIntegration_NilConfig(t *testing.T) {
	vi := NewVectorIntegration(nil)

	require.NotNil(t, vi)
	assert.Nil(t, vi.config)
	assert.NotNil(t, vi.providers)
}

// =============================================================================
// VectorConfig Tests
// =============================================================================

func TestVectorConfig_Fields(t *testing.T) {
	config := &VectorConfig{
		DefaultProvider: "pinecone",
		LoadBalancing:   "round_robin",
		FailoverEnabled: true,
	}

	assert.Equal(t, "pinecone", config.DefaultProvider)
	assert.True(t, config.FailoverEnabled)
}

// =============================================================================
// VectorData Tests
// =============================================================================

func TestVectorData_Fields(t *testing.T) {
	now := time.Now()
	data := &VectorData{
		ID:        "vec-123",
		Embedding: []float64{0.1, 0.2, 0.3, 0.4, 0.5},
		Metadata: map[string]interface{}{
			"source": "test",
			"type":   "document",
		},
		IndexName: "test-index",
		CreatedAt: now,
	}

	assert.Equal(t, "vec-123", data.ID)
	assert.Len(t, data.Embedding, 5)
	assert.Equal(t, "test", data.Metadata["source"])
	assert.Equal(t, "test-index", data.IndexName)
	assert.Equal(t, now, data.CreatedAt)
}

func TestVectorData_EmbeddingValues(t *testing.T) {
	embedding := make([]float64, 1536)
	for i := range embedding {
		embedding[i] = float64(i) / 1536.0
	}

	data := &VectorData{
		ID:        "embed-test",
		Embedding: embedding,
	}

	assert.Len(t, data.Embedding, 1536)
	assert.InDelta(t, 0.0, data.Embedding[0], 0.001)
	assert.InDelta(t, 1535.0/1536.0, data.Embedding[1535], 0.001)
}

// =============================================================================
// VectorSearchQuery Tests
// =============================================================================

func TestVectorSearchQuery_Fields(t *testing.T) {
	query := &VectorSearchQuery{
		Embedding: []float64{0.1, 0.2, 0.3},
		IndexName: "documents",
		K:         10,
		Threshold: 0.8,
		Filters: map[string]interface{}{
			"type": "article",
		},
		Metric: "cosine",
	}

	assert.Len(t, query.Embedding, 3)
	assert.Equal(t, "documents", query.IndexName)
	assert.Equal(t, 10, query.K)
	assert.Equal(t, 0.8, query.Threshold)
	assert.Equal(t, "article", query.Filters["type"])
	assert.Equal(t, "cosine", query.Metric)
}

func TestVectorSearchQuery_DefaultValues(t *testing.T) {
	query := &VectorSearchQuery{}

	assert.Empty(t, query.Embedding)
	assert.Empty(t, query.IndexName)
	assert.Equal(t, 0, query.K)
	assert.Equal(t, 0.0, query.Threshold)
}

// =============================================================================
// VectorSearchResult Tests
// =============================================================================

func TestVectorSearchResult_Fields(t *testing.T) {
	result := &VectorSearchResult{
		ID:     "result-1",
		Vector: []float64{0.5, 0.5, 0.5},
		Metadata: map[string]interface{}{
			"title": "Test Document",
		},
		Score:    0.95,
		Distance: 0.05,
	}

	assert.Equal(t, "result-1", result.ID)
	assert.Len(t, result.Vector, 3)
	assert.Equal(t, "Test Document", result.Metadata["title"])
	assert.Equal(t, 0.95, result.Score)
	assert.Equal(t, 0.05, result.Distance)
}

func TestVectorSearchResult_ScoreDistanceRelationship(t *testing.T) {
	// Score and distance are typically inversely related
	result := &VectorSearchResult{
		Score:    0.85,
		Distance: 1 - 0.85,
	}

	assert.InDelta(t, 1.0, result.Score+result.Distance, 0.001)
}

// =============================================================================
// VectorIndexConfig Tests
// =============================================================================

func TestVectorIndexConfig_Fields(t *testing.T) {
	config := &VectorIndexConfig{
		Dimension:   1536,
		Metric:      "cosine",
		Description: "Test index for document embeddings",
	}

	assert.Equal(t, 1536, config.Dimension)
	assert.Equal(t, "cosine", config.Metric)
	assert.Equal(t, "Test index for document embeddings", config.Description)
}

func TestVectorIndexConfig_Metrics(t *testing.T) {
	metrics := []string{"cosine", "euclidean", "dot_product", "manhattan"}

	for _, metric := range metrics {
		config := &VectorIndexConfig{
			Dimension: 768,
			Metric:    metric,
		}
		assert.Equal(t, metric, config.Metric)
	}
}

// =============================================================================
// VectorIndexInfo Tests
// =============================================================================

func TestVectorIndexInfo_Fields(t *testing.T) {
	now := time.Now()
	info := &VectorIndexInfo{
		Name:        "documents",
		Description: "Document embeddings index",
		Dimension:   1536,
		Metric:      "cosine",
		VectorCount: 10000,
		Size:        1024 * 1024 * 100, // 100MB
		CreatedAt:   now.Add(-24 * time.Hour),
		UpdatedAt:   now,
	}

	assert.Equal(t, "documents", info.Name)
	assert.Equal(t, "Document embeddings index", info.Description)
	assert.Equal(t, 1536, info.Dimension)
	assert.Equal(t, "cosine", info.Metric)
	assert.Equal(t, int64(10000), info.VectorCount)
	assert.Equal(t, int64(1024*1024*100), info.Size)
	assert.True(t, info.UpdatedAt.After(info.CreatedAt))
}

// =============================================================================
// VectorStats Tests
// =============================================================================

func TestVectorStats_Fields(t *testing.T) {
	stats := &VectorStats{
		TotalVectors:   50000,
		TotalIndexes:   5,
		TotalSize:      1024 * 1024 * 500, // 500MB
		AverageLatency: 50 * time.Millisecond,
		LastOperation:  time.Now(),
		ErrorCount:     10,
		Uptime:         24 * time.Hour,
		Cost:           25.50,
	}

	assert.Equal(t, int64(50000), stats.TotalVectors)
	assert.Equal(t, int64(5), stats.TotalIndexes)
	assert.Equal(t, int64(1024*1024*500), stats.TotalSize)
	assert.Equal(t, 50*time.Millisecond, stats.AverageLatency)
	assert.Equal(t, int64(10), stats.ErrorCount)
	assert.Equal(t, 24*time.Hour, stats.Uptime)
	assert.Equal(t, 25.50, stats.Cost)
}

func TestVectorStats_ZeroValues(t *testing.T) {
	stats := &VectorStats{}

	assert.Equal(t, int64(0), stats.TotalVectors)
	assert.Equal(t, int64(0), stats.TotalIndexes)
	assert.Equal(t, int64(0), stats.TotalSize)
	assert.Equal(t, int64(0), stats.ErrorCount)
}

// =============================================================================
// VectorHealthStatus Tests
// =============================================================================

func TestVectorHealthStatus_Fields(t *testing.T) {
	status := &VectorHealthStatus{
		Status:             "healthy",
		TotalProviders:     3,
		HealthyProviders:   2,
		UnhealthyProviders: 1,
		LastCheck:          time.Now(),
	}

	assert.Equal(t, "healthy", status.Status)
	assert.Equal(t, 3, status.TotalProviders)
	assert.Equal(t, 2, status.HealthyProviders)
	assert.Equal(t, 1, status.UnhealthyProviders)
}

func TestVectorHealthStatus_AllHealthy(t *testing.T) {
	status := &VectorHealthStatus{
		Status:             "healthy",
		TotalProviders:     5,
		HealthyProviders:   5,
		UnhealthyProviders: 0,
	}

	assert.Equal(t, status.TotalProviders, status.HealthyProviders)
	assert.Equal(t, 0, status.UnhealthyProviders)
}

func TestVectorHealthStatus_Degraded(t *testing.T) {
	status := &VectorHealthStatus{
		Status:             "degraded",
		TotalProviders:     5,
		HealthyProviders:   3,
		UnhealthyProviders: 2,
	}

	assert.Equal(t, "degraded", status.Status)
	assert.Equal(t, status.TotalProviders, status.HealthyProviders+status.UnhealthyProviders)
}

func TestVectorHealthStatus_Unhealthy(t *testing.T) {
	status := &VectorHealthStatus{
		Status:             "unhealthy",
		TotalProviders:     3,
		HealthyProviders:   0,
		UnhealthyProviders: 3,
	}

	assert.Equal(t, "unhealthy", status.Status)
	assert.Equal(t, 0, status.HealthyProviders)
	assert.Equal(t, 3, status.UnhealthyProviders)
}

// =============================================================================
// VectorIntegration Conversion Tests
// =============================================================================

func TestVectorIntegration_ConvertToProviderVector(t *testing.T) {
	vi := NewVectorIntegration(&VectorConfig{})
	now := time.Now()

	vector := &VectorData{
		ID:        "test-id",
		Embedding: []float64{0.1, 0.2, 0.3},
		Metadata: map[string]interface{}{
			"key": "value",
		},
		IndexName: "test-index",
		CreatedAt: now,
	}

	providerVector := vi.convertToProviderVector(vector)

	assert.Equal(t, "test-id", providerVector.ID)
	assert.Equal(t, vector.Embedding, providerVector.Vector)
	assert.Equal(t, "value", providerVector.Metadata["key"])
	assert.Equal(t, "test-index", providerVector.Collection)
	assert.Equal(t, now, providerVector.Timestamp)
}

func TestVectorIntegration_ConvertToProviderQuery(t *testing.T) {
	vi := NewVectorIntegration(&VectorConfig{})

	query := &VectorSearchQuery{
		Embedding: []float64{0.5, 0.5, 0.5},
		IndexName: "search-index",
		K:         10,
		Threshold: 0.7,
		Filters: map[string]interface{}{
			"type": "document",
		},
	}

	providerQuery := vi.convertToProviderQuery(query)

	assert.Equal(t, query.Embedding, providerQuery.Vector)
	assert.Equal(t, "search-index", providerQuery.Collection)
	assert.Equal(t, 10, providerQuery.TopK)
	assert.Equal(t, 0.7, providerQuery.Threshold)
	assert.Equal(t, "document", providerQuery.Filters["type"])
}

// =============================================================================
// Edge Cases and Boundary Tests
// =============================================================================

func TestVectorData_EmptyEmbedding(t *testing.T) {
	data := &VectorData{
		ID:        "empty-embed",
		Embedding: []float64{},
	}

	assert.Empty(t, data.Embedding)
}

func TestVectorData_LargeEmbedding(t *testing.T) {
	// OpenAI ada-002 produces 1536 dimensions
	embedding := make([]float64, 1536)
	for i := range embedding {
		embedding[i] = 0.001 * float64(i)
	}

	data := &VectorData{
		ID:        "large-embed",
		Embedding: embedding,
	}

	assert.Len(t, data.Embedding, 1536)
}

func TestVectorSearchQuery_HighK(t *testing.T) {
	query := &VectorSearchQuery{
		K: 1000,
	}

	assert.Equal(t, 1000, query.K)
}

func TestVectorSearchQuery_LowThreshold(t *testing.T) {
	query := &VectorSearchQuery{
		Threshold: 0.0,
	}

	assert.Equal(t, 0.0, query.Threshold)
}

func TestVectorSearchQuery_HighThreshold(t *testing.T) {
	query := &VectorSearchQuery{
		Threshold: 1.0,
	}

	assert.Equal(t, 1.0, query.Threshold)
}

func TestVectorIndexInfo_ZeroVectors(t *testing.T) {
	info := &VectorIndexInfo{
		Name:        "empty-index",
		VectorCount: 0,
		Size:        0,
	}

	assert.Equal(t, int64(0), info.VectorCount)
	assert.Equal(t, int64(0), info.Size)
}

func TestVectorStats_HighLatency(t *testing.T) {
	stats := &VectorStats{
		AverageLatency: 5 * time.Second,
	}

	assert.Equal(t, 5*time.Second, stats.AverageLatency)
}

// SingleProviderConfigWrapper is a placeholder type for testing
type SingleProviderConfigWrapper struct {
	Enabled bool
}
