package providers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========================================
// Initialization Tests
// ========================================

func TestNewChromaDBProvider(t *testing.T) {
	config := map[string]interface{}{
		"url":      "http://localhost:8000",
		"api_key":  "test-key",
		"tenant":   "test-tenant",
		"database": "test-db",
	}

	provider, err := NewChromaDBProvider(config)
	require.NoError(t, err)
	assert.NotNil(t, provider)

	chromaProvider := provider.(*ChromaDBProvider)
	assert.Equal(t, "http://localhost:8000", chromaProvider.config.URL)
	assert.Equal(t, "test-key", chromaProvider.config.APIKey)
	assert.Equal(t, "test-tenant", chromaProvider.config.Tenant)
	assert.Equal(t, "test-db", chromaProvider.config.Database)
}

func TestNewChromaDBProvider_DefaultValues(t *testing.T) {
	config := map[string]interface{}{}

	provider, err := NewChromaDBProvider(config)
	require.NoError(t, err)

	chromaProvider := provider.(*ChromaDBProvider)
	assert.Equal(t, "http://localhost:8000", chromaProvider.config.URL)
	assert.Equal(t, "default_tenant", chromaProvider.config.Tenant)
	assert.Equal(t, "default_database", chromaProvider.config.Database)
}

// ========================================
// Metadata Tests
// ========================================

func TestChromaDBProvider_GetName(t *testing.T) {
	provider := createTestChromaDBProvider(t)
	assert.Equal(t, "chromadb", provider.GetName())
}

func TestChromaDBProvider_GetType(t *testing.T) {
	provider := createTestChromaDBProvider(t)
	assert.Equal(t, "chroma", provider.GetType())
}

func TestChromaDBProvider_GetCapabilities(t *testing.T) {
	provider := createTestChromaDBProvider(t)
	capabilities := provider.GetCapabilities()
	assert.NotEmpty(t, capabilities)
	assert.Contains(t, capabilities, "vector_storage")
	assert.Contains(t, capabilities, "similarity_search")
	assert.Contains(t, capabilities, "metadata_filtering")
}

func TestChromaDBProvider_GetConfiguration(t *testing.T) {
	provider := createTestChromaDBProvider(t)
	config := provider.GetConfiguration()
	assert.NotNil(t, config)
	chromaConfig := config.(*ChromaDBConfig)
	assert.NotEmpty(t, chromaConfig.URL)
}

func TestChromaDBProvider_IsCloud(t *testing.T) {
	provider := createTestChromaDBProvider(t)
	assert.False(t, provider.IsCloud())
}

func TestChromaDBProvider_GetCostInfo(t *testing.T) {
	provider := createTestChromaDBProvider(t)
	costInfo := provider.GetCostInfo()
	assert.NotNil(t, costInfo)
	assert.Equal(t, "USD", costInfo.Currency)
	assert.Equal(t, 0.0, costInfo.TotalCost)
}

// ========================================
// Lifecycle Tests
// ========================================

func TestChromaDBProvider_Initialize(t *testing.T) {
	server := createMockChromaServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/heartbeat" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	defer server.Close()

	provider := createTestChromaDBProviderWithURL(t, server.URL)
	ctx := context.Background()

	err := provider.Initialize(ctx, nil)
	require.NoError(t, err)
	assert.True(t, provider.initialized)
}

func TestChromaDBProvider_Initialize_ConnectionFailed(t *testing.T) {
	config := map[string]interface{}{
		"url": "http://localhost:99999",
	}
	vp, _ := NewChromaDBProvider(config)
	provider := vp.(*ChromaDBProvider)
	ctx := context.Background()

	err := provider.Initialize(ctx, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to connect")
}

func TestChromaDBProvider_Start(t *testing.T) {
	server := createMockChromaServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer server.Close()

	provider := createTestChromaDBProviderWithURL(t, server.URL)
	ctx := context.Background()

	// Must initialize first
	err := provider.Initialize(ctx, nil)
	require.NoError(t, err)

	err = provider.Start(ctx)
	require.NoError(t, err)
	assert.True(t, provider.started)
}

func TestChromaDBProvider_Start_NotInitialized(t *testing.T) {
	provider := createTestChromaDBProvider(t)
	ctx := context.Background()

	err := provider.Start(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestChromaDBProvider_Stop(t *testing.T) {
	server := createMockChromaServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer server.Close()

	provider := createTestChromaDBProviderWithURL(t, server.URL)
	ctx := context.Background()

	_ = provider.Initialize(ctx, nil)
	_ = provider.Start(ctx)

	err := provider.Stop(ctx)
	require.NoError(t, err)
	assert.False(t, provider.started)
}

// ========================================
// Store Tests
// ========================================

func TestChromaDBProvider_Store(t *testing.T) {
	callCount := 0
	server := createMockChromaServer(t, func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
	})
	defer server.Close()

	provider := createTestChromaDBProviderWithURL(t, server.URL)
	ctx := context.Background()

	_ = provider.Initialize(ctx, nil)
	_ = provider.Start(ctx)

	data := []*VectorData{
		{
			ID:         "vec-1",
			Vector:     []float64{0.1, 0.2, 0.3},
			Collection: "test-collection",
		},
	}

	err := provider.Store(ctx, data)
	require.NoError(t, err)
}

func TestChromaDBProvider_Store_NotStarted(t *testing.T) {
	provider := createTestChromaDBProvider(t)
	ctx := context.Background()

	data := []*VectorData{{ID: "vec-1"}}
	err := provider.Store(ctx, data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not started")
}

func TestChromaDBProvider_Store_Empty(t *testing.T) {
	server := createMockChromaServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer server.Close()

	provider := createTestChromaDBProviderWithURL(t, server.URL)
	ctx := context.Background()

	_ = provider.Initialize(ctx, nil)
	_ = provider.Start(ctx)

	err := provider.Store(ctx, []*VectorData{})
	require.NoError(t, err)
}

func TestChromaDBProvider_Store_DefaultCollection(t *testing.T) {
	// Anti-bluff (CONST-035 / §11.9): the original form set
	// usedDefaultCollection in the mock handler, discarded the Store
	// result with `_ = provider.Store(ctx, data)`, and then discarded the
	// flag with `_ = usedDefaultCollection // Used in test verification`
	// — passing even if Store never hit the mock server. The test's
	// stated invariant ("VectorData without an explicit Collection MUST
	// route to /api/v1/collections/default/add") was checked only by
	// human reader, not by assertion. Now: require Store to succeed AND
	// assert the default-collection path was actually exercised.
	usedDefaultCollection := false
	server := createMockChromaServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/collections/default/add" {
			usedDefaultCollection = true
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
	})
	defer server.Close()

	provider := createTestChromaDBProviderWithURL(t, server.URL)
	ctx := context.Background()

	require.NoError(t, provider.Initialize(ctx, nil), "Initialize must succeed against mock server")
	require.NoError(t, provider.Start(ctx), "Start must succeed against mock server")

	data := []*VectorData{
		{
			ID:     "vec-1",
			Vector: []float64{0.1, 0.2, 0.3},
			// No collection specified - should use default
		},
	}

	require.NoError(t, provider.Store(ctx, data), "Store with default-collection data must succeed")
	assert.True(t, usedDefaultCollection,
		"Store with no explicit Collection must route to /api/v1/collections/default/add")
}

// ========================================
// Search Tests
// ========================================

func TestChromaDBProvider_Search(t *testing.T) {
	server := createMockChromaServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ids":       [][]string{{"result-1", "result-2"}},
			"distances": [][]float64{{0.1, 0.2}},
			"metadatas": [][]map[string]interface{}{{{"key": "value"}, {}}},
		})
	})
	defer server.Close()

	provider := createTestChromaDBProviderWithURL(t, server.URL)
	ctx := context.Background()

	_ = provider.Initialize(ctx, nil)
	_ = provider.Start(ctx)

	query := &VectorQuery{
		Vector:     []float64{0.1, 0.2, 0.3},
		Collection: "test-collection",
		TopK:       5,
	}

	result, err := provider.Search(ctx, query)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestChromaDBProvider_Search_NotStarted(t *testing.T) {
	provider := createTestChromaDBProvider(t)
	ctx := context.Background()

	query := &VectorQuery{Vector: []float64{0.1}}
	result, err := provider.Search(ctx, query)
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestChromaDBProvider_Search_NilQuery(t *testing.T) {
	server := createMockChromaServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer server.Close()

	provider := createTestChromaDBProviderWithURL(t, server.URL)
	ctx := context.Background()

	_ = provider.Initialize(ctx, nil)
	_ = provider.Start(ctx)

	result, err := provider.Search(ctx, nil)
	assert.Error(t, err)
	assert.Nil(t, result)
}

// ========================================
// Retrieve Tests
// ========================================

func TestChromaDBProvider_Retrieve(t *testing.T) {
	server := createMockChromaServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ids":        []string{"vec-1"},
			"embeddings": [][]float64{{0.1, 0.2, 0.3}},
			"metadatas":  []map[string]interface{}{{"key": "value"}},
		})
	})
	defer server.Close()

	provider := createTestChromaDBProviderWithURL(t, server.URL)
	ctx := context.Background()

	_ = provider.Initialize(ctx, nil)
	_ = provider.Start(ctx)

	results, err := provider.Retrieve(ctx, []string{"vec-1"})
	require.NoError(t, err)
	assert.NotNil(t, results)
}

func TestChromaDBProvider_Retrieve_NotStarted(t *testing.T) {
	provider := createTestChromaDBProvider(t)
	ctx := context.Background()

	results, err := provider.Retrieve(ctx, []string{"vec-1"})
	assert.Error(t, err)
	assert.Nil(t, results)
}

func TestChromaDBProvider_Retrieve_Empty(t *testing.T) {
	server := createMockChromaServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer server.Close()

	provider := createTestChromaDBProviderWithURL(t, server.URL)
	ctx := context.Background()

	_ = provider.Initialize(ctx, nil)
	_ = provider.Start(ctx)

	results, err := provider.Retrieve(ctx, []string{})
	require.NoError(t, err)
	assert.Empty(t, results)
}

// ========================================
// Update Tests
// ========================================

func TestChromaDBProvider_Update(t *testing.T) {
	server := createMockChromaServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
	})
	defer server.Close()

	provider := createTestChromaDBProviderWithURL(t, server.URL)
	ctx := context.Background()

	_ = provider.Initialize(ctx, nil)
	_ = provider.Start(ctx)

	vector := &VectorData{
		ID:         "vec-1",
		Vector:     []float64{0.4, 0.5, 0.6},
		Collection: "test-collection",
	}

	err := provider.Update(ctx, "vec-1", vector)
	require.NoError(t, err)
}

func TestChromaDBProvider_Update_NotStarted(t *testing.T) {
	provider := createTestChromaDBProvider(t)
	ctx := context.Background()

	vector := &VectorData{ID: "vec-1", Vector: []float64{0.1}}
	err := provider.Update(ctx, "vec-1", vector)
	assert.Error(t, err)
}

func TestChromaDBProvider_Update_NilVector(t *testing.T) {
	server := createMockChromaServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer server.Close()

	provider := createTestChromaDBProviderWithURL(t, server.URL)
	ctx := context.Background()

	_ = provider.Initialize(ctx, nil)
	_ = provider.Start(ctx)

	err := provider.Update(ctx, "vec-1", nil)
	assert.Error(t, err)
}

// ========================================
// Delete Tests
// ========================================

func TestChromaDBProvider_Delete(t *testing.T) {
	server := createMockChromaServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
	})
	defer server.Close()

	provider := createTestChromaDBProviderWithURL(t, server.URL)
	ctx := context.Background()

	_ = provider.Initialize(ctx, nil)
	_ = provider.Start(ctx)

	err := provider.Delete(ctx, []string{"vec-1", "vec-2"})
	require.NoError(t, err)
}

func TestChromaDBProvider_Delete_NotStarted(t *testing.T) {
	provider := createTestChromaDBProvider(t)
	ctx := context.Background()

	err := provider.Delete(ctx, []string{"vec-1"})
	assert.Error(t, err)
}

func TestChromaDBProvider_Delete_Empty(t *testing.T) {
	server := createMockChromaServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer server.Close()

	provider := createTestChromaDBProviderWithURL(t, server.URL)
	ctx := context.Background()

	_ = provider.Initialize(ctx, nil)
	_ = provider.Start(ctx)

	err := provider.Delete(ctx, []string{})
	require.NoError(t, err)
}

// ========================================
// FindSimilar Tests
// ========================================

func TestChromaDBProvider_FindSimilar(t *testing.T) {
	server := createMockChromaServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ids":       [][]string{{"similar-1"}},
			"distances": [][]float64{{0.05}},
		})
	})
	defer server.Close()

	provider := createTestChromaDBProviderWithURL(t, server.URL)
	ctx := context.Background()

	_ = provider.Initialize(ctx, nil)
	_ = provider.Start(ctx)

	results, err := provider.FindSimilar(ctx, []float64{0.1, 0.2, 0.3}, 5, nil)
	require.NoError(t, err)
	assert.NotNil(t, results)
}

func TestChromaDBProvider_FindSimilar_NotStarted(t *testing.T) {
	provider := createTestChromaDBProvider(t)
	ctx := context.Background()

	results, err := provider.FindSimilar(ctx, []float64{0.1}, 5, nil)
	assert.Error(t, err)
	assert.Nil(t, results)
}

func TestChromaDBProvider_FindSimilar_EmptyEmbedding(t *testing.T) {
	server := createMockChromaServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ids":       [][]string{{}},
			"distances": [][]float64{{}},
		})
	})
	defer server.Close()

	provider := createTestChromaDBProviderWithURL(t, server.URL)
	ctx := context.Background()

	_ = provider.Initialize(ctx, nil)
	_ = provider.Start(ctx)

	results, err := provider.FindSimilar(ctx, []float64{}, 5, nil)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestChromaDBProvider_BatchFindSimilar(t *testing.T) {
	server := createMockChromaServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ids":       [][]string{{}},
			"distances": [][]float64{{}},
		})
	})
	defer server.Close()

	provider := createTestChromaDBProviderWithURL(t, server.URL)
	ctx := context.Background()

	_ = provider.Initialize(ctx, nil)
	_ = provider.Start(ctx)

	queries := [][]float64{
		{0.1, 0.2, 0.3},
		{0.4, 0.5, 0.6},
	}

	results, err := provider.BatchFindSimilar(ctx, queries, 5)
	require.NoError(t, err)
	assert.NotNil(t, results)
}

// ========================================
// Collection Tests
// ========================================

func TestChromaDBProvider_CreateCollection(t *testing.T) {
	server := createMockChromaServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":   "collection-id",
			"name": "test-collection",
		})
	})
	defer server.Close()

	provider := createTestChromaDBProviderWithURL(t, server.URL)
	ctx := context.Background()

	_ = provider.Initialize(ctx, nil)
	_ = provider.Start(ctx)

	config := &CollectionConfig{
		Dimension: 384,
		Metric:    "cosine",
	}

	err := provider.CreateCollection(ctx, "test-collection", config)
	require.NoError(t, err)
}

func TestChromaDBProvider_CreateCollection_NotStarted(t *testing.T) {
	provider := createTestChromaDBProvider(t)
	ctx := context.Background()

	err := provider.CreateCollection(ctx, "test", &CollectionConfig{})
	assert.Error(t, err)
}

func TestChromaDBProvider_DeleteCollection(t *testing.T) {
	server := createMockChromaServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer server.Close()

	provider := createTestChromaDBProviderWithURL(t, server.URL)
	ctx := context.Background()

	_ = provider.Initialize(ctx, nil)
	_ = provider.Start(ctx)

	err := provider.DeleteCollection(ctx, "test-collection")
	require.NoError(t, err)
}

func TestChromaDBProvider_ListCollections(t *testing.T) {
	server := createMockChromaServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// ChromaDB returns a list of collection names
		json.NewEncoder(w).Encode([]string{"collection-1", "collection-2"})
	})
	defer server.Close()

	provider := createTestChromaDBProviderWithURL(t, server.URL)
	ctx := context.Background()

	_ = provider.Initialize(ctx, nil)
	_ = provider.Start(ctx)

	collections, err := provider.ListCollections(ctx)
	require.NoError(t, err)
	assert.NotNil(t, collections)
	assert.Len(t, collections, 2)
}

func TestChromaDBProvider_GetCollection(t *testing.T) {
	server := createMockChromaServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"name": "test-collection",
		})
	})
	defer server.Close()

	provider := createTestChromaDBProviderWithURL(t, server.URL)
	ctx := context.Background()

	_ = provider.Initialize(ctx, nil)
	_ = provider.Start(ctx)

	collection, err := provider.GetCollection(ctx, "test-collection")
	require.NoError(t, err)
	assert.NotNil(t, collection)
}

// ========================================
// Index Tests
// ========================================

func TestChromaDBProvider_CreateIndex(t *testing.T) {
	server := createMockChromaServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer server.Close()

	provider := createTestChromaDBProviderWithURL(t, server.URL)
	ctx := context.Background()

	_ = provider.Initialize(ctx, nil)
	_ = provider.Start(ctx)

	config := &IndexConfig{
		Name: "test-index",
		Type: "hnsw",
	}

	// ChromaDB doesn't support manual index creation
	err := provider.CreateIndex(ctx, "test-collection", config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not support")
}

func TestChromaDBProvider_DeleteIndex(t *testing.T) {
	server := createMockChromaServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer server.Close()

	provider := createTestChromaDBProviderWithURL(t, server.URL)
	ctx := context.Background()

	_ = provider.Initialize(ctx, nil)
	_ = provider.Start(ctx)

	// ChromaDB doesn't support manual index deletion
	err := provider.DeleteIndex(ctx, "test-collection", "test-index")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not support")
}

func TestChromaDBProvider_ListIndexes(t *testing.T) {
	server := createMockChromaServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode([]map[string]interface{}{})
	})
	defer server.Close()

	provider := createTestChromaDBProviderWithURL(t, server.URL)
	ctx := context.Background()

	_ = provider.Initialize(ctx, nil)
	_ = provider.Start(ctx)

	// ChromaDB returns a default HNSW index even though manual indexing isn't supported
	indexes, err := provider.ListIndexes(ctx, "test-collection")
	require.NoError(t, err)
	assert.NotNil(t, indexes)
}

// ========================================
// Metadata Operations Tests
// ========================================

func TestChromaDBProvider_AddMetadata(t *testing.T) {
	server := createMockChromaServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// AddMetadata first calls GetMetadata, then Update
		// Return metadata response for GET, success for POST
		if r.Method == "POST" && r.URL.Path == "/api/v1/collections/default/get" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ids":       []string{"vec-1"},
				"metadatas": []map[string]interface{}{{"existing": "data"}},
			})
		}
	})
	defer server.Close()

	provider := createTestChromaDBProviderWithURL(t, server.URL)
	ctx := context.Background()

	_ = provider.Initialize(ctx, nil)
	_ = provider.Start(ctx)

	err := provider.AddMetadata(ctx, "vec-1", map[string]interface{}{"key": "value"})
	require.NoError(t, err)
}

func TestChromaDBProvider_UpdateMetadata(t *testing.T) {
	server := createMockChromaServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer server.Close()

	provider := createTestChromaDBProviderWithURL(t, server.URL)
	ctx := context.Background()

	_ = provider.Initialize(ctx, nil)
	_ = provider.Start(ctx)

	err := provider.UpdateMetadata(ctx, "vec-1", map[string]interface{}{"key": "new-value"})
	require.NoError(t, err)
}

func TestChromaDBProvider_GetMetadata(t *testing.T) {
	server := createMockChromaServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"metadatas": []map[string]interface{}{{"key": "value"}},
		})
	})
	defer server.Close()

	provider := createTestChromaDBProviderWithURL(t, server.URL)
	ctx := context.Background()

	_ = provider.Initialize(ctx, nil)
	_ = provider.Start(ctx)

	metadata, err := provider.GetMetadata(ctx, []string{"vec-1"})
	require.NoError(t, err)
	assert.NotNil(t, metadata)
}

func TestChromaDBProvider_DeleteMetadata(t *testing.T) {
	server := createMockChromaServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// DeleteMetadata first calls GetMetadata, then Update
		if r.Method == "POST" && r.URL.Path == "/api/v1/collections/default/get" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ids":       []string{"vec-1"},
				"metadatas": []map[string]interface{}{{"key": "value", "other": "data"}},
			})
		}
	})
	defer server.Close()

	provider := createTestChromaDBProviderWithURL(t, server.URL)
	ctx := context.Background()

	_ = provider.Initialize(ctx, nil)
	_ = provider.Start(ctx)

	err := provider.DeleteMetadata(ctx, []string{"vec-1"}, []string{"key"})
	require.NoError(t, err)
}

// ========================================
// Management Tests
// ========================================

func TestChromaDBProvider_GetStats(t *testing.T) {
	server := createMockChromaServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer server.Close()

	provider := createTestChromaDBProviderWithURL(t, server.URL)
	ctx := context.Background()

	_ = provider.Initialize(ctx, nil)
	_ = provider.Start(ctx)

	stats, err := provider.GetStats(ctx)
	require.NoError(t, err)
	assert.NotNil(t, stats)
}

func TestChromaDBProvider_Optimize(t *testing.T) {
	server := createMockChromaServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer server.Close()

	provider := createTestChromaDBProviderWithURL(t, server.URL)
	ctx := context.Background()

	_ = provider.Initialize(ctx, nil)
	_ = provider.Start(ctx)

	err := provider.Optimize(ctx)
	require.NoError(t, err)
}

func TestChromaDBProvider_Backup(t *testing.T) {
	server := createMockChromaServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer server.Close()

	provider := createTestChromaDBProviderWithURL(t, server.URL)
	ctx := context.Background()

	_ = provider.Initialize(ctx, nil)
	_ = provider.Start(ctx)

	// ChromaDB doesn't support backup operations
	err := provider.Backup(ctx, "/tmp/backup")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not support")
}

func TestChromaDBProvider_Restore(t *testing.T) {
	server := createMockChromaServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer server.Close()

	provider := createTestChromaDBProviderWithURL(t, server.URL)
	ctx := context.Background()

	_ = provider.Initialize(ctx, nil)
	_ = provider.Start(ctx)

	// ChromaDB doesn't support restore operations
	err := provider.Restore(ctx, "/tmp/backup")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not support")
}

func TestChromaDBProvider_Health(t *testing.T) {
	server := createMockChromaServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer server.Close()

	provider := createTestChromaDBProviderWithURL(t, server.URL)
	ctx := context.Background()

	_ = provider.Initialize(ctx, nil)
	_ = provider.Start(ctx)

	health, err := provider.Health(ctx)
	require.NoError(t, err)
	assert.NotNil(t, health)
}

func TestChromaDBProvider_Close(t *testing.T) {
	server := createMockChromaServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer server.Close()

	provider := createTestChromaDBProviderWithURL(t, server.URL)
	ctx := context.Background()

	_ = provider.Initialize(ctx, nil)
	_ = provider.Start(ctx)

	err := provider.Close(ctx)
	require.NoError(t, err)
}

// ========================================
// Test Helpers
// ========================================

func createTestChromaDBProvider(t *testing.T) *ChromaDBProvider {
	config := map[string]interface{}{
		"url": "http://localhost:8000",
	}
	vp, err := NewChromaDBProvider(config)
	require.NoError(t, err)
	return vp.(*ChromaDBProvider)
}

func createTestChromaDBProviderWithURL(t *testing.T, url string) *ChromaDBProvider {
	config := map[string]interface{}{
		"url": url,
	}
	vp, err := NewChromaDBProvider(config)
	require.NoError(t, err)
	return vp.(*ChromaDBProvider)
}

func createMockChromaServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	return httptest.NewServer(handler)
}
