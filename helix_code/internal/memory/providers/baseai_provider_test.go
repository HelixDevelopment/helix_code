package providers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========================================
// Initialization Tests
// ========================================

func TestNewBaseAIProvider(t *testing.T) {
	config := map[string]interface{}{
		"api_key":  "test-key",
		"base_url": "https://api.langbase.com/v1",
	}

	provider, err := NewBaseAIProvider(config)
	require.NoError(t, err)
	assert.NotNil(t, provider)
	assert.Equal(t, "test-key", provider.apiKey)
	assert.Equal(t, "https://api.langbase.com/v1", provider.baseURL)
}

func TestNewBaseAIProvider_MissingAPIKey(t *testing.T) {
	config := map[string]interface{}{
		"base_url": "https://api.langbase.com/v1",
	}

	provider, err := NewBaseAIProvider(config)
	assert.Error(t, err)
	assert.Nil(t, provider)
	assert.Contains(t, err.Error(), "api_key is required")
}

func TestNewBaseAIProvider_DefaultBaseURL(t *testing.T) {
	config := map[string]interface{}{
		"api_key": "test-key",
	}

	provider, err := NewBaseAIProvider(config)
	require.NoError(t, err)
	assert.NotNil(t, provider)
	assert.Equal(t, "https://api.langbase.com/v1", provider.baseURL)
}

func TestNewBaseAIProvider_TrailingSlashRemoved(t *testing.T) {
	config := map[string]interface{}{
		"api_key":  "test-key",
		"base_url": "https://api.langbase.com/v1/",
	}

	provider, err := NewBaseAIProvider(config)
	require.NoError(t, err)
	assert.Equal(t, "https://api.langbase.com/v1", provider.baseURL)
}

// ========================================
// Metadata Tests
// ========================================

func TestBaseAIProvider_GetType(t *testing.T) {
	provider := createTestBaseAIProvider(t)
	assert.Equal(t, "baseai", provider.GetType())
}

func TestBaseAIProvider_GetName(t *testing.T) {
	provider := createTestBaseAIProvider(t)
	assert.Equal(t, "BaseAI", provider.GetName())
}

func TestBaseAIProvider_GetCapabilities(t *testing.T) {
	provider := createTestBaseAIProvider(t)
	capabilities := provider.GetCapabilities()
	assert.NotEmpty(t, capabilities)
	assert.Contains(t, capabilities, "memory_storage")
	assert.Contains(t, capabilities, "memory_retrieval")
	assert.Contains(t, capabilities, "memory_search")
	assert.Contains(t, capabilities, "cost_tracking")
}

func TestBaseAIProvider_GetConfiguration(t *testing.T) {
	provider := createTestBaseAIProvider(t)
	config := provider.GetConfiguration()
	assert.NotNil(t, config)
	configMap := config.(map[string]interface{})
	assert.Equal(t, "test-key", configMap["api_key"])
}

func TestBaseAIProvider_IsCloud(t *testing.T) {
	provider := createTestBaseAIProvider(t)
	assert.True(t, provider.IsCloud())
}

func TestBaseAIProvider_GetCostInfo(t *testing.T) {
	provider := createTestBaseAIProvider(t)
	costInfo := provider.GetCostInfo()
	assert.NotNil(t, costInfo)
	assert.Equal(t, "USD", costInfo.Currency)
	assert.Equal(t, "monthly", costInfo.BillingPeriod)
}

// ========================================
// Lifecycle Tests
// ========================================

func TestBaseAIProvider_Initialize(t *testing.T) {
	provider := createTestBaseAIProvider(t)
	ctx := context.Background()

	err := provider.Initialize(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, "initialized", provider.stats.Status)
}

func TestBaseAIProvider_Start(t *testing.T) {
	server := createMockBaseAIServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(BaseAIResponse{Success: true})
	})
	defer server.Close()

	provider := createTestBaseAIProviderWithURL(t, server.URL)
	ctx := context.Background()

	err := provider.Start(ctx)
	require.NoError(t, err)
	assert.Equal(t, "active", provider.stats.Status)
}

func TestBaseAIProvider_Start_ConnectionFailed(t *testing.T) {
	config := map[string]interface{}{
		"api_key":  "test-key",
		"base_url": "http://localhost:99999", // Invalid port
	}
	provider, _ := NewBaseAIProvider(config)
	ctx := context.Background()

	err := provider.Start(ctx)
	assert.Error(t, err)
	assert.Equal(t, "error", provider.stats.Status)
}

func TestBaseAIProvider_Stop(t *testing.T) {
	provider := createTestBaseAIProvider(t)
	ctx := context.Background()

	err := provider.Stop(ctx)
	require.NoError(t, err)
	assert.Equal(t, "stopped", provider.stats.Status)
}

func TestBaseAIProvider_Close(t *testing.T) {
	provider := createTestBaseAIProvider(t)
	ctx := context.Background()

	err := provider.Close(ctx)
	require.NoError(t, err)
	assert.Equal(t, "closed", provider.stats.Status)
}

// ========================================
// Store Tests
// ========================================

func TestBaseAIProvider_Store(t *testing.T) {
	server := createMockBaseAIServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(BaseAIResponse{
			Success: true,
			Meta:    &BaseAIMeta{Cost: 0.001},
		})
	})
	defer server.Close()

	provider := createTestBaseAIProviderWithURL(t, server.URL)
	ctx := context.Background()

	data := []*VectorData{
		{
			ID:       "vec-1",
			Vector:   []float64{0.1, 0.2, 0.3},
			Metadata: map[string]interface{}{"text": "test"},
		},
	}

	err := provider.Store(ctx, data)
	require.NoError(t, err)
	assert.Equal(t, int64(1), provider.stats.TotalVectors)
	assert.Equal(t, int64(1), provider.stats.SuccessfulOps)
}

func TestBaseAIProvider_Store_Empty(t *testing.T) {
	provider := createTestBaseAIProvider(t)
	ctx := context.Background()

	err := provider.Store(ctx, []*VectorData{})
	require.NoError(t, err)
}

func TestBaseAIProvider_Store_APIError(t *testing.T) {
	server := createMockBaseAIServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(BaseAIResponse{
			Success: false,
			Error:   "storage error",
		})
	})
	defer server.Close()

	provider := createTestBaseAIProviderWithURL(t, server.URL)
	ctx := context.Background()

	data := []*VectorData{{ID: "vec-1", Vector: []float64{0.1}}}
	err := provider.Store(ctx, data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "storage error")
}

// ========================================
// Search Tests
// ========================================

func TestBaseAIProvider_Search(t *testing.T) {
	server := createMockBaseAIServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(BaseAIResponse{
			Success: true,
			Data: []interface{}{
				map[string]interface{}{
					"id":    "result-1",
					"score": 0.95,
					"metadata": map[string]interface{}{
						"text": "found content",
					},
				},
			},
		})
	})
	defer server.Close()

	provider := createTestBaseAIProviderWithURL(t, server.URL)
	ctx := context.Background()

	query := &VectorQuery{
		Text: "test query",
		TopK: 5,
	}

	result, err := provider.Search(ctx, query)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Results, 1)
	assert.Equal(t, "result-1", result.Results[0].ID)
	assert.Equal(t, 0.95, result.Results[0].Score)
}

func TestBaseAIProvider_Search_NilQuery(t *testing.T) {
	provider := createTestBaseAIProvider(t)
	ctx := context.Background()

	result, err := provider.Search(ctx, nil)
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestBaseAIProvider_Search_WithVector(t *testing.T) {
	server := createMockBaseAIServer(t, func(w http.ResponseWriter, r *http.Request) {
		var req BaseAIRequest
		json.NewDecoder(r.Body).Decode(&req)
		// Verify vector is passed
		assert.NotEmpty(t, req.Query.Vector)
		json.NewEncoder(w).Encode(BaseAIResponse{Success: true, Data: []interface{}{}})
	})
	defer server.Close()

	provider := createTestBaseAIProviderWithURL(t, server.URL)
	ctx := context.Background()

	query := &VectorQuery{
		Vector: []float64{0.1, 0.2, 0.3},
		TopK:   5,
	}

	result, err := provider.Search(ctx, query)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

// ========================================
// Retrieve Tests
// ========================================

func TestBaseAIProvider_Retrieve(t *testing.T) {
	server := createMockBaseAIServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(BaseAIResponse{
			Success: true,
			Data: []interface{}{
				map[string]interface{}{
					"id":     "vec-1",
					"vector": []interface{}{0.1, 0.2, 0.3},
				},
			},
		})
	})
	defer server.Close()

	provider := createTestBaseAIProviderWithURL(t, server.URL)
	ctx := context.Background()

	results, err := provider.Retrieve(ctx, []string{"vec-1"})
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "vec-1", results[0].ID)
}

func TestBaseAIProvider_Retrieve_Empty(t *testing.T) {
	provider := createTestBaseAIProvider(t)
	ctx := context.Background()

	results, err := provider.Retrieve(ctx, []string{})
	require.NoError(t, err)
	assert.Empty(t, results)
}

// ========================================
// Update Tests
// ========================================

func TestBaseAIProvider_Update(t *testing.T) {
	server := createMockBaseAIServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(BaseAIResponse{Success: true})
	})
	defer server.Close()

	provider := createTestBaseAIProviderWithURL(t, server.URL)
	ctx := context.Background()

	vector := &VectorData{
		ID:     "vec-1",
		Vector: []float64{0.4, 0.5, 0.6},
	}

	err := provider.Update(ctx, "vec-1", vector)
	require.NoError(t, err)
}

func TestBaseAIProvider_Update_NilVector(t *testing.T) {
	provider := createTestBaseAIProvider(t)
	ctx := context.Background()

	err := provider.Update(ctx, "vec-1", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be nil")
}

// ========================================
// Delete Tests
// ========================================

func TestBaseAIProvider_Delete(t *testing.T) {
	server := createMockBaseAIServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(BaseAIResponse{Success: true})
	})
	defer server.Close()

	provider := createTestBaseAIProviderWithURL(t, server.URL)
	// Pre-set vector count
	provider.stats.TotalVectors = 5
	ctx := context.Background()

	err := provider.Delete(ctx, []string{"vec-1", "vec-2"})
	require.NoError(t, err)
	assert.Equal(t, int64(3), provider.stats.TotalVectors)
}

func TestBaseAIProvider_Delete_Empty(t *testing.T) {
	provider := createTestBaseAIProvider(t)
	ctx := context.Background()

	err := provider.Delete(ctx, []string{})
	require.NoError(t, err)
}

// ========================================
// FindSimilar Tests
// ========================================

func TestBaseAIProvider_FindSimilar(t *testing.T) {
	server := createMockBaseAIServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(BaseAIResponse{
			Success: true,
			Data: []interface{}{
				map[string]interface{}{
					"id":    "similar-1",
					"score": 0.92,
				},
			},
		})
	})
	defer server.Close()

	provider := createTestBaseAIProviderWithURL(t, server.URL)
	ctx := context.Background()

	results, err := provider.FindSimilar(ctx, []float64{0.1, 0.2, 0.3}, 5, nil)
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "similar-1", results[0].ID)
}

func TestBaseAIProvider_FindSimilar_EmptyEmbedding(t *testing.T) {
	provider := createTestBaseAIProvider(t)
	ctx := context.Background()

	results, err := provider.FindSimilar(ctx, []float64{}, 5, nil)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestBaseAIProvider_BatchFindSimilar(t *testing.T) {
	server := createMockBaseAIServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(BaseAIResponse{
			Success: true,
			Data:    []interface{}{},
		})
	})
	defer server.Close()

	provider := createTestBaseAIProviderWithURL(t, server.URL)
	ctx := context.Background()

	queries := [][]float64{
		{0.1, 0.2, 0.3},
		{0.4, 0.5, 0.6},
	}

	results, err := provider.BatchFindSimilar(ctx, queries, 5)
	require.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestBaseAIProvider_BatchFindSimilar_Empty(t *testing.T) {
	provider := createTestBaseAIProvider(t)
	ctx := context.Background()

	results, err := provider.BatchFindSimilar(ctx, [][]float64{}, 5)
	require.NoError(t, err)
	assert.Empty(t, results)
}

// ========================================
// Collection Tests
// ========================================

func TestBaseAIProvider_CreateCollection(t *testing.T) {
	server := createMockBaseAIServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(BaseAIResponse{Success: true})
	})
	defer server.Close()

	provider := createTestBaseAIProviderWithURL(t, server.URL)
	ctx := context.Background()

	config := &CollectionConfig{
		Dimension: 384,
		Metric:    "cosine",
	}

	err := provider.CreateCollection(ctx, "test-collection", config)
	require.NoError(t, err)

	// Verify collection was added
	coll, exists := provider.collections["test-collection"]
	assert.True(t, exists)
	assert.Equal(t, "test-collection", coll.Name)
}

func TestBaseAIProvider_CreateCollection_EmptyName(t *testing.T) {
	provider := createTestBaseAIProvider(t)
	ctx := context.Background()

	err := provider.CreateCollection(ctx, "", &CollectionConfig{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be empty")
}

func TestBaseAIProvider_DeleteCollection(t *testing.T) {
	server := createMockBaseAIServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(BaseAIResponse{Success: true})
	})
	defer server.Close()

	provider := createTestBaseAIProviderWithURL(t, server.URL)
	// Add a collection first
	provider.collections["to-delete"] = &CollectionInfo{Name: "to-delete"}
	provider.stats.TotalCollections = 2

	ctx := context.Background()

	err := provider.DeleteCollection(ctx, "to-delete")
	require.NoError(t, err)

	_, exists := provider.collections["to-delete"]
	assert.False(t, exists)
	assert.Equal(t, int64(1), provider.stats.TotalCollections)
}

func TestBaseAIProvider_DeleteCollection_EmptyName(t *testing.T) {
	provider := createTestBaseAIProvider(t)
	ctx := context.Background()

	err := provider.DeleteCollection(ctx, "")
	assert.Error(t, err)
}

func TestBaseAIProvider_ListCollections(t *testing.T) {
	server := createMockBaseAIServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(BaseAIResponse{
			Success: true,
			Data: []interface{}{
				map[string]interface{}{
					"name":         "collection-1",
					"vector_count": float64(100),
				},
			},
		})
	})
	defer server.Close()

	provider := createTestBaseAIProviderWithURL(t, server.URL)
	ctx := context.Background()

	collections, err := provider.ListCollections(ctx)
	require.NoError(t, err)
	assert.Len(t, collections, 1)
	assert.Equal(t, "collection-1", collections[0].Name)
}

func TestBaseAIProvider_GetCollection(t *testing.T) {
	provider := createTestBaseAIProvider(t)
	ctx := context.Background()

	// Default collection should exist
	coll, err := provider.GetCollection(ctx, "default")
	require.NoError(t, err)
	assert.NotNil(t, coll)
	assert.Equal(t, "default", coll.Name)
}

func TestBaseAIProvider_GetCollection_EmptyName(t *testing.T) {
	provider := createTestBaseAIProvider(t)
	ctx := context.Background()

	coll, err := provider.GetCollection(ctx, "")
	assert.Error(t, err)
	assert.Nil(t, coll)
}

// ========================================
// Index Tests
// ========================================

func TestBaseAIProvider_CreateIndex(t *testing.T) {
	server := createMockBaseAIServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(BaseAIResponse{Success: true})
	})
	defer server.Close()

	provider := createTestBaseAIProviderWithURL(t, server.URL)
	ctx := context.Background()

	config := &IndexConfig{
		Name:   "test-index",
		Type:   "hnsw",
		Metric: "cosine",
	}

	err := provider.CreateIndex(ctx, "default", config)
	require.NoError(t, err)
}

func TestBaseAIProvider_CreateIndex_EmptyCollection(t *testing.T) {
	provider := createTestBaseAIProvider(t)
	ctx := context.Background()

	err := provider.CreateIndex(ctx, "", &IndexConfig{})
	assert.Error(t, err)
}

func TestBaseAIProvider_DeleteIndex(t *testing.T) {
	server := createMockBaseAIServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(BaseAIResponse{Success: true})
	})
	defer server.Close()

	provider := createTestBaseAIProviderWithURL(t, server.URL)
	ctx := context.Background()

	err := provider.DeleteIndex(ctx, "default", "test-index")
	require.NoError(t, err)
}

func TestBaseAIProvider_DeleteIndex_EmptyParams(t *testing.T) {
	provider := createTestBaseAIProvider(t)
	ctx := context.Background()

	err := provider.DeleteIndex(ctx, "", "index")
	assert.Error(t, err)

	err = provider.DeleteIndex(ctx, "collection", "")
	assert.Error(t, err)
}

func TestBaseAIProvider_ListIndexes(t *testing.T) {
	server := createMockBaseAIServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(BaseAIResponse{
			Success: true,
			Data: []interface{}{
				map[string]interface{}{
					"name":  "index-1",
					"type":  "hnsw",
					"state": "ready",
				},
			},
		})
	})
	defer server.Close()

	provider := createTestBaseAIProviderWithURL(t, server.URL)
	ctx := context.Background()

	indexes, err := provider.ListIndexes(ctx, "default")
	require.NoError(t, err)
	assert.Len(t, indexes, 1)
	assert.Equal(t, "index-1", indexes[0].Name)
}

func TestBaseAIProvider_ListIndexes_EmptyCollection(t *testing.T) {
	provider := createTestBaseAIProvider(t)
	ctx := context.Background()

	indexes, err := provider.ListIndexes(ctx, "")
	require.NoError(t, err)
	assert.Empty(t, indexes)
}

// ========================================
// Metadata Tests
// ========================================

func TestBaseAIProvider_AddMetadata(t *testing.T) {
	server := createMockBaseAIServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(BaseAIResponse{Success: true})
	})
	defer server.Close()

	provider := createTestBaseAIProviderWithURL(t, server.URL)
	ctx := context.Background()

	err := provider.AddMetadata(ctx, "vec-1", map[string]interface{}{"key": "value"})
	require.NoError(t, err)
}

func TestBaseAIProvider_AddMetadata_EmptyParams(t *testing.T) {
	provider := createTestBaseAIProvider(t)
	ctx := context.Background()

	err := provider.AddMetadata(ctx, "", map[string]interface{}{"key": "value"})
	assert.Error(t, err)

	err = provider.AddMetadata(ctx, "id", map[string]interface{}{})
	assert.Error(t, err)
}

func TestBaseAIProvider_UpdateMetadata(t *testing.T) {
	server := createMockBaseAIServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(BaseAIResponse{Success: true})
	})
	defer server.Close()

	provider := createTestBaseAIProviderWithURL(t, server.URL)
	ctx := context.Background()

	err := provider.UpdateMetadata(ctx, "vec-1", map[string]interface{}{"key": "new-value"})
	require.NoError(t, err)
}

func TestBaseAIProvider_UpdateMetadata_EmptyParams(t *testing.T) {
	provider := createTestBaseAIProvider(t)
	ctx := context.Background()

	err := provider.UpdateMetadata(ctx, "", map[string]interface{}{"key": "value"})
	assert.Error(t, err)

	err = provider.UpdateMetadata(ctx, "id", map[string]interface{}{})
	assert.Error(t, err)
}

func TestBaseAIProvider_GetMetadata(t *testing.T) {
	server := createMockBaseAIServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(BaseAIResponse{
			Success: true,
			Data: map[string]interface{}{
				"vec-1": map[string]interface{}{"key": "value"},
			},
		})
	})
	defer server.Close()

	provider := createTestBaseAIProviderWithURL(t, server.URL)
	ctx := context.Background()

	metadata, err := provider.GetMetadata(ctx, []string{"vec-1"})
	require.NoError(t, err)
	assert.Contains(t, metadata, "vec-1")
	assert.Equal(t, "value", metadata["vec-1"]["key"])
}

func TestBaseAIProvider_GetMetadata_Empty(t *testing.T) {
	provider := createTestBaseAIProvider(t)
	ctx := context.Background()

	metadata, err := provider.GetMetadata(ctx, []string{})
	require.NoError(t, err)
	assert.Empty(t, metadata)
}

func TestBaseAIProvider_DeleteMetadata(t *testing.T) {
	server := createMockBaseAIServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(BaseAIResponse{Success: true})
	})
	defer server.Close()

	provider := createTestBaseAIProviderWithURL(t, server.URL)
	ctx := context.Background()

	err := provider.DeleteMetadata(ctx, []string{"vec-1"}, []string{"key"})
	require.NoError(t, err)
}

func TestBaseAIProvider_DeleteMetadata_Empty(t *testing.T) {
	provider := createTestBaseAIProvider(t)
	ctx := context.Background()

	err := provider.DeleteMetadata(ctx, []string{}, []string{"key"})
	require.NoError(t, err)

	err = provider.DeleteMetadata(ctx, []string{"id"}, []string{})
	require.NoError(t, err)
}

// ========================================
// Stats and Management Tests
// ========================================

func TestBaseAIProvider_GetStats(t *testing.T) {
	provider := createTestBaseAIProvider(t)
	ctx := context.Background()

	stats, err := provider.GetStats(ctx)
	require.NoError(t, err)
	assert.NotNil(t, stats)
	assert.Equal(t, "BaseAI", stats.Name)
	assert.Equal(t, "baseai", stats.Type)
}

func TestBaseAIProvider_Optimize(t *testing.T) {
	server := createMockBaseAIServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(BaseAIResponse{Success: true})
	})
	defer server.Close()

	provider := createTestBaseAIProviderWithURL(t, server.URL)
	ctx := context.Background()

	err := provider.Optimize(ctx)
	require.NoError(t, err)
}

func TestBaseAIProvider_Backup(t *testing.T) {
	server := createMockBaseAIServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(BaseAIResponse{Success: true})
	})
	defer server.Close()

	provider := createTestBaseAIProviderWithURL(t, server.URL)
	ctx := context.Background()

	err := provider.Backup(ctx, "/tmp/backup")
	require.NoError(t, err)
}

func TestBaseAIProvider_Backup_EmptyPath(t *testing.T) {
	provider := createTestBaseAIProvider(t)
	ctx := context.Background()

	err := provider.Backup(ctx, "")
	assert.Error(t, err)
}

func TestBaseAIProvider_Restore(t *testing.T) {
	server := createMockBaseAIServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(BaseAIResponse{Success: true})
	})
	defer server.Close()

	provider := createTestBaseAIProviderWithURL(t, server.URL)
	ctx := context.Background()

	err := provider.Restore(ctx, "/tmp/backup")
	require.NoError(t, err)
}

func TestBaseAIProvider_Restore_EmptyPath(t *testing.T) {
	provider := createTestBaseAIProvider(t)
	ctx := context.Background()

	err := provider.Restore(ctx, "")
	assert.Error(t, err)
}

func TestBaseAIProvider_Health(t *testing.T) {
	server := createMockBaseAIServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(BaseAIResponse{Success: true})
	})
	defer server.Close()

	provider := createTestBaseAIProviderWithURL(t, server.URL)
	ctx := context.Background()

	health, err := provider.Health(ctx)
	require.NoError(t, err)
	assert.NotNil(t, health)
	assert.Equal(t, "healthy", health.Status)
	assert.NotNil(t, health.Metrics)
}

func TestBaseAIProvider_Health_Unhealthy(t *testing.T) {
	config := map[string]interface{}{
		"api_key":  "test-key",
		"base_url": "http://localhost:99999",
	}
	provider, _ := NewBaseAIProvider(config)
	ctx := context.Background()

	health, err := provider.Health(ctx)
	require.NoError(t, err)
	assert.Equal(t, "unhealthy", health.Status)
}

// ========================================
// Helper Function Tests
// ========================================

func TestGetStringFromMetadata(t *testing.T) {
	metadata := map[string]interface{}{
		"text": "hello",
		"num":  42,
	}

	assert.Equal(t, "hello", getStringFromMetadata(metadata, "text"))
	assert.Equal(t, "", getStringFromMetadata(metadata, "num"))
	assert.Equal(t, "", getStringFromMetadata(metadata, "missing"))
}

func TestGetStringFromConfig(t *testing.T) {
	config := &IndexConfig{
		Type: "hnsw",
		Parameters: map[string]interface{}{
			"field": "embedding",
		},
	}

	assert.Equal(t, "embedding", getStringFromConfig(config, "field"))
	assert.Equal(t, "hnsw", getStringFromConfig(config, "type"))
	assert.Equal(t, "", getStringFromConfig(nil, "field"))
}

func TestGetStringFromMap(t *testing.T) {
	m := map[string]interface{}{
		"key": "value",
		"num": 42,
	}

	assert.Equal(t, "value", getStringFromMap(m, "key"))
	assert.Equal(t, "", getStringFromMap(m, "num"))
	assert.Equal(t, "", getStringFromMap(m, "missing"))
}

func TestGetFloatFromMap(t *testing.T) {
	m := map[string]interface{}{
		"score": 0.95,
		"text":  "hello",
	}

	assert.Equal(t, 0.95, getFloatFromMap(m, "score"))
	assert.Equal(t, 0.0, getFloatFromMap(m, "text"))
	assert.Equal(t, 0.0, getFloatFromMap(m, "missing"))
}

func TestGetIntFromMap(t *testing.T) {
	m := map[string]interface{}{
		"count": float64(42),
		"text":  "hello",
	}

	assert.Equal(t, 42, getIntFromMap(m, "count"))
	assert.Equal(t, 0, getIntFromMap(m, "text"))
}

func TestGetFloatSliceFromMap(t *testing.T) {
	m := map[string]interface{}{
		"vector": []interface{}{0.1, 0.2, 0.3},
		"text":   "hello",
	}

	result := getFloatSliceFromMap(m, "vector")
	assert.Len(t, result, 3)
	assert.Equal(t, 0.1, result[0])

	assert.Empty(t, getFloatSliceFromMap(m, "text"))
	assert.Empty(t, getFloatSliceFromMap(m, "missing"))
}

func TestGetMapFromMap(t *testing.T) {
	m := map[string]interface{}{
		"metadata": map[string]interface{}{"key": "value"},
		"text":     "hello",
	}

	result := getMapFromMap(m, "metadata")
	assert.Equal(t, "value", result["key"])
	assert.Empty(t, getMapFromMap(m, "text"))
}

func TestGetTimeFromMap(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	m := map[string]interface{}{
		"created": now.Format(time.RFC3339),
		"invalid": "not a time",
	}

	result := getTimeFromMap(m, "created")
	assert.Equal(t, now.Format(time.RFC3339), result.Format(time.RFC3339))
	assert.True(t, getTimeFromMap(m, "invalid").IsZero())
	assert.True(t, getTimeFromMap(m, "missing").IsZero())
}

// ========================================
// Test Helpers
// ========================================

func createTestBaseAIProvider(t *testing.T) *BaseAIProvider {
	config := map[string]interface{}{
		"api_key":  "test-key",
		"base_url": "https://api.langbase.com/v1",
	}
	provider, err := NewBaseAIProvider(config)
	require.NoError(t, err)
	return provider
}

func createTestBaseAIProviderWithURL(t *testing.T, url string) *BaseAIProvider {
	config := map[string]interface{}{
		"api_key":  "test-key",
		"base_url": url,
	}
	provider, err := NewBaseAIProvider(config)
	require.NoError(t, err)
	return provider
}

func createMockBaseAIServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	return httptest.NewServer(handler)
}
