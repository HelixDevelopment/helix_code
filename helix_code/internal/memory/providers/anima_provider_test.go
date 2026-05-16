package providers

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========================================
// Initialization Tests
// ========================================

func TestNewAnimaProvider(t *testing.T) {
	config := &AnimaConfig{
		APIKey:  "test-key",
		BaseURL: "https://api.anima.ai/v1",
	}

	provider, err := NewAnimaProvider(config)
	require.NoError(t, err)
	assert.NotNil(t, provider)
	assert.Equal(t, config, provider.config)
}

func TestNewAnimaProvider_NilConfig(t *testing.T) {
	provider, err := NewAnimaProvider(nil)
	require.NoError(t, err)
	assert.NotNil(t, provider)
}

func TestAnimaProvider_Initialize(t *testing.T) {
	config := &AnimaConfig{
		APIKey:  "test-key",
		BaseURL: "https://api.anima.ai/v1",
	}

	provider, err := NewAnimaProvider(config)
	require.NoError(t, err)

	ctx := context.Background()
	err = provider.Initialize(ctx, nil)
	require.NoError(t, err)
	assert.True(t, provider.initialized)
}

func TestAnimaProvider_Initialize_WithConfig(t *testing.T) {
	provider, err := NewAnimaProvider(nil)
	require.NoError(t, err)

	ctx := context.Background()
	newConfig := &AnimaConfig{
		APIKey:  "new-key",
		BaseURL: "https://new.api.anima.ai/v1",
	}
	err = provider.Initialize(ctx, newConfig)
	require.NoError(t, err)
	assert.Equal(t, "new-key", provider.config.APIKey)
}

func TestAnimaProvider_Initialize_Idempotent(t *testing.T) {
	provider, err := NewAnimaProvider(nil)
	require.NoError(t, err)

	ctx := context.Background()
	err = provider.Initialize(ctx, nil)
	require.NoError(t, err)

	// Second init should also succeed (no-op)
	err = provider.Initialize(ctx, nil)
	require.NoError(t, err)
}

// ========================================
// Lifecycle Tests
// ========================================

func TestAnimaProvider_Start(t *testing.T) {
	provider, err := NewAnimaProvider(nil)
	require.NoError(t, err)

	ctx := context.Background()
	err = provider.Initialize(ctx, nil)
	require.NoError(t, err)

	err = provider.Start(ctx)
	require.NoError(t, err)
	assert.True(t, provider.started)
}

func TestAnimaProvider_Start_NotInitialized(t *testing.T) {
	provider, err := NewAnimaProvider(nil)
	require.NoError(t, err)

	ctx := context.Background()
	err = provider.Start(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must be initialized")
}

func TestAnimaProvider_Stop(t *testing.T) {
	provider, err := NewAnimaProvider(nil)
	require.NoError(t, err)

	ctx := context.Background()
	err = provider.Initialize(ctx, nil)
	require.NoError(t, err)

	err = provider.Start(ctx)
	require.NoError(t, err)

	err = provider.Stop(ctx)
	require.NoError(t, err)
	assert.False(t, provider.started)
}

func TestAnimaProvider_Health(t *testing.T) {
	provider, err := NewAnimaProvider(nil)
	require.NoError(t, err)

	ctx := context.Background()
	err = provider.Initialize(ctx, nil)
	require.NoError(t, err)

	err = provider.Start(ctx)
	require.NoError(t, err)

	health, err := provider.Health(ctx)
	require.NoError(t, err)
	assert.Equal(t, "healthy", health.Status)
}

func TestAnimaProvider_Health_NotStarted(t *testing.T) {
	provider, err := NewAnimaProvider(nil)
	require.NoError(t, err)

	ctx := context.Background()
	health, err := provider.Health(ctx)
	require.NoError(t, err)
	assert.Equal(t, "not_initialized", health.Status)
}

func TestAnimaProvider_Health_NotInitialized(t *testing.T) {
	provider, err := NewAnimaProvider(nil)
	require.NoError(t, err)

	ctx := context.Background()
	err = provider.Initialize(ctx, nil)
	require.NoError(t, err)

	health, err := provider.Health(ctx)
	require.NoError(t, err)
	assert.Equal(t, "not_started", health.Status)
}

// ========================================
// Provider Info Tests
// ========================================

func TestAnimaProvider_GetName(t *testing.T) {
	provider, err := NewAnimaProvider(nil)
	require.NoError(t, err)
	assert.Equal(t, "anima", provider.GetName())
}

func TestAnimaProvider_GetType(t *testing.T) {
	provider, err := NewAnimaProvider(nil)
	require.NoError(t, err)
	assert.Equal(t, "anima", provider.GetType())
}

func TestAnimaProvider_GetCapabilities(t *testing.T) {
	provider, err := NewAnimaProvider(nil)
	require.NoError(t, err)
	caps := provider.GetCapabilities()

	assert.NotEmpty(t, caps)
	assert.Contains(t, caps, "vector_storage")
	assert.Contains(t, caps, "vector_search")
	assert.Contains(t, caps, "metadata_management")
}

func TestAnimaProvider_GetConfiguration(t *testing.T) {
	config := &AnimaConfig{
		APIKey:  "test-key",
		BaseURL: "https://api.anima.ai/v1",
	}
	provider, err := NewAnimaProvider(config)
	require.NoError(t, err)

	cfg := provider.GetConfiguration()
	assert.Equal(t, config, cfg)
}

func TestAnimaProvider_IsCloud(t *testing.T) {
	provider, err := NewAnimaProvider(nil)
	require.NoError(t, err)
	assert.True(t, provider.IsCloud())
}

func TestAnimaProvider_GetCostInfo(t *testing.T) {
	provider, err := NewAnimaProvider(nil)
	require.NoError(t, err)
	costInfo := provider.GetCostInfo()

	assert.Equal(t, "USD", costInfo.Currency)
	assert.Equal(t, 0.001, costInfo.ReadCost)
	assert.Equal(t, 0.002, costInfo.WriteCost)
	assert.Equal(t, 0.0001, costInfo.StorageCost)
}

// ========================================
// CRUD Operations Tests
// ========================================

func TestAnimaProvider_Store(t *testing.T) {
	provider, err := NewAnimaProvider(nil)
	require.NoError(t, err)

	ctx := context.Background()
	err = provider.Initialize(ctx, nil)
	require.NoError(t, err)

	err = provider.Start(ctx)
	require.NoError(t, err)

	data := []*VectorData{{
		ID:       "test-id",
		Vector:   []float64{0.1, 0.2, 0.3},
		Metadata: map[string]interface{}{"key": "value"},
	}}

	err = provider.Store(ctx, data)
	require.NoError(t, err)
}

func TestAnimaProvider_Store_NotStarted(t *testing.T) {
	provider, err := NewAnimaProvider(nil)
	require.NoError(t, err)

	ctx := context.Background()
	data := []*VectorData{{ID: "test-id", Vector: []float64{0.1}}}

	err = provider.Store(ctx, data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "started")
}

func TestAnimaProvider_Retrieve(t *testing.T) {
	provider, err := NewAnimaProvider(nil)
	require.NoError(t, err)

	ctx := context.Background()
	err = provider.Initialize(ctx, nil)
	require.NoError(t, err)

	err = provider.Start(ctx)
	require.NoError(t, err)

	// Store data first
	data := []*VectorData{{
		ID:       "test-id",
		Vector:   []float64{0.1, 0.2, 0.3},
		Metadata: map[string]interface{}{"content": "test content"},
	}}
	err = provider.Store(ctx, data)
	require.NoError(t, err)

	// Retrieve it
	results, err := provider.Retrieve(ctx, []string{"test-id"})
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "test-id", results[0].ID)
}

func TestAnimaProvider_Retrieve_NotFound(t *testing.T) {
	provider, err := NewAnimaProvider(nil)
	require.NoError(t, err)

	ctx := context.Background()
	err = provider.Initialize(ctx, nil)
	require.NoError(t, err)

	err = provider.Start(ctx)
	require.NoError(t, err)

	results, err := provider.Retrieve(ctx, []string{"non-existent"})
	// Non-existent IDs should return empty results, not error
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestAnimaProvider_Update(t *testing.T) {
	provider, err := NewAnimaProvider(nil)
	require.NoError(t, err)

	ctx := context.Background()
	err = provider.Initialize(ctx, nil)
	require.NoError(t, err)

	err = provider.Start(ctx)
	require.NoError(t, err)

	// Store data first
	data := []*VectorData{{
		ID:       "test-id",
		Vector:   []float64{0.1, 0.2, 0.3},
		Metadata: map[string]interface{}{"content": "original"},
	}}
	err = provider.Store(ctx, data)
	require.NoError(t, err)

	// Update it
	updatedData := &VectorData{
		ID:       "test-id",
		Vector:   []float64{0.4, 0.5, 0.6},
		Metadata: map[string]interface{}{"content": "updated"},
	}
	err = provider.Update(ctx, "test-id", updatedData)
	require.NoError(t, err)

	// Verify update
	results, err := provider.Retrieve(ctx, []string{"test-id"})
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "updated", results[0].Metadata["content"])
}

func TestAnimaProvider_Delete(t *testing.T) {
	provider, err := NewAnimaProvider(nil)
	require.NoError(t, err)

	ctx := context.Background()
	err = provider.Initialize(ctx, nil)
	require.NoError(t, err)

	err = provider.Start(ctx)
	require.NoError(t, err)

	// Store data first
	data := []*VectorData{{ID: "test-id", Vector: []float64{0.1}}}
	err = provider.Store(ctx, data)
	require.NoError(t, err)

	// Delete it
	err = provider.Delete(ctx, []string{"test-id"})
	require.NoError(t, err)

	// Verify deleted
	results, err := provider.Retrieve(ctx, []string{"test-id"})
	require.NoError(t, err)
	assert.Empty(t, results)
}

// ========================================
// Search Tests
// ========================================

func TestAnimaProvider_Search(t *testing.T) {
	provider, err := NewAnimaProvider(nil)
	require.NoError(t, err)

	ctx := context.Background()
	err = provider.Initialize(ctx, nil)
	require.NoError(t, err)

	err = provider.Start(ctx)
	require.NoError(t, err)

	// Store some data
	for i := 0; i < 5; i++ {
		data := []*VectorData{{
			ID:       fmt.Sprintf("id-%d", i),
			Vector:   []float64{float64(i) * 0.1, float64(i) * 0.2},
			Metadata: map[string]interface{}{"text": fmt.Sprintf("content %d", i)},
		}}
		err = provider.Store(ctx, data)
		require.NoError(t, err)
	}

	// Search
	query := &VectorQuery{
		Text: "content",
		TopK: 3,
	}
	result, err := provider.Search(ctx, query)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.LessOrEqual(t, len(result.Results), 3)
}

func TestAnimaProvider_FindSimilar(t *testing.T) {
	provider, err := NewAnimaProvider(nil)
	require.NoError(t, err)

	ctx := context.Background()
	err = provider.Initialize(ctx, nil)
	require.NoError(t, err)

	err = provider.Start(ctx)
	require.NoError(t, err)

	// Store some data
	data := []*VectorData{{
		ID:     "test-id",
		Vector: []float64{0.1, 0.2, 0.3},
	}}
	err = provider.Store(ctx, data)
	require.NoError(t, err)

	// Find similar
	results, err := provider.FindSimilar(ctx, []float64{0.1, 0.2, 0.3}, 5, nil)
	require.NoError(t, err)
	assert.NotEmpty(t, results)
}

func TestAnimaProvider_BatchFindSimilar(t *testing.T) {
	provider, err := NewAnimaProvider(nil)
	require.NoError(t, err)

	ctx := context.Background()
	err = provider.Initialize(ctx, nil)
	require.NoError(t, err)

	err = provider.Start(ctx)
	require.NoError(t, err)

	// Store some data
	data := []*VectorData{{
		ID:     "test-id",
		Vector: []float64{0.1, 0.2, 0.3},
	}}
	err = provider.Store(ctx, data)
	require.NoError(t, err)

	// Batch find
	queries := [][]float64{
		{0.1, 0.2, 0.3},
		{0.2, 0.3, 0.4},
	}
	results, err := provider.BatchFindSimilar(ctx, queries, 5)
	require.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestAnimaProvider_BatchFindSimilar_Empty(t *testing.T) {
	provider, err := NewAnimaProvider(nil)
	require.NoError(t, err)

	ctx := context.Background()
	err = provider.Initialize(ctx, nil)
	require.NoError(t, err)

	err = provider.Start(ctx)
	require.NoError(t, err)

	results, err := provider.BatchFindSimilar(ctx, [][]float64{}, 5)
	require.NoError(t, err)
	assert.Empty(t, results)
}

// ========================================
// Collection Tests
// ========================================

func TestAnimaProvider_CreateCollection(t *testing.T) {
	provider, err := NewAnimaProvider(nil)
	require.NoError(t, err)

	ctx := context.Background()
	err = provider.Initialize(ctx, nil)
	require.NoError(t, err)

	err = provider.Start(ctx)
	require.NoError(t, err)

	err = provider.CreateCollection(ctx, "test-collection", &CollectionConfig{
		Name:        "test-collection",
		Description: "Test collection",
		Dimension:   768,
		Metric:      "cosine",
	})
	require.NoError(t, err)
}

func TestAnimaProvider_CreateCollection_AlreadyExists(t *testing.T) {
	provider, err := NewAnimaProvider(nil)
	require.NoError(t, err)

	ctx := context.Background()
	err = provider.Initialize(ctx, nil)
	require.NoError(t, err)

	err = provider.Start(ctx)
	require.NoError(t, err)

	err = provider.CreateCollection(ctx, "test-collection", nil)
	require.NoError(t, err)

	// Second creation should fail
	err = provider.CreateCollection(ctx, "test-collection", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestAnimaProvider_GetCollection(t *testing.T) {
	provider, err := NewAnimaProvider(nil)
	require.NoError(t, err)

	ctx := context.Background()
	err = provider.Initialize(ctx, nil)
	require.NoError(t, err)

	err = provider.Start(ctx)
	require.NoError(t, err)

	err = provider.CreateCollection(ctx, "test-collection", nil)
	require.NoError(t, err)

	info, err := provider.GetCollection(ctx, "test-collection")
	require.NoError(t, err)
	assert.NotNil(t, info)
	assert.Equal(t, "test-collection", info.Name)
}

func TestAnimaProvider_ListCollections(t *testing.T) {
	provider, err := NewAnimaProvider(nil)
	require.NoError(t, err)

	ctx := context.Background()
	err = provider.Initialize(ctx, nil)
	require.NoError(t, err)

	err = provider.Start(ctx)
	require.NoError(t, err)

	err = provider.CreateCollection(ctx, "collection-1", nil)
	require.NoError(t, err)

	err = provider.CreateCollection(ctx, "collection-2", nil)
	require.NoError(t, err)

	collections, err := provider.ListCollections(ctx)
	require.NoError(t, err)
	assert.Len(t, collections, 2)
}

func TestAnimaProvider_DeleteCollection(t *testing.T) {
	provider, err := NewAnimaProvider(nil)
	require.NoError(t, err)

	ctx := context.Background()
	err = provider.Initialize(ctx, nil)
	require.NoError(t, err)

	err = provider.Start(ctx)
	require.NoError(t, err)

	err = provider.CreateCollection(ctx, "test-collection", nil)
	require.NoError(t, err)

	err = provider.DeleteCollection(ctx, "test-collection")
	require.NoError(t, err)

	// Verify deleted
	_, err = provider.GetCollection(ctx, "test-collection")
	assert.Error(t, err)
}

// ========================================
// Index Tests
// ========================================

func TestAnimaProvider_CreateIndex(t *testing.T) {
	provider, err := NewAnimaProvider(nil)
	require.NoError(t, err)

	ctx := context.Background()
	err = provider.Initialize(ctx, nil)
	require.NoError(t, err)

	err = provider.Start(ctx)
	require.NoError(t, err)

	err = provider.CreateCollection(ctx, "test-collection", nil)
	require.NoError(t, err)

	err = provider.CreateIndex(ctx, "test-collection", &IndexConfig{
		Name: "test-index",
		Type: "hnsw",
	})
	require.NoError(t, err)
}

func TestAnimaProvider_ListIndexes(t *testing.T) {
	provider, err := NewAnimaProvider(nil)
	require.NoError(t, err)

	ctx := context.Background()
	err = provider.Initialize(ctx, nil)
	require.NoError(t, err)

	err = provider.Start(ctx)
	require.NoError(t, err)

	err = provider.CreateCollection(ctx, "test-collection", nil)
	require.NoError(t, err)

	err = provider.CreateIndex(ctx, "test-collection", &IndexConfig{Name: "index-1"})
	require.NoError(t, err)

	err = provider.CreateIndex(ctx, "test-collection", &IndexConfig{Name: "index-2"})
	require.NoError(t, err)

	indexes, err := provider.ListIndexes(ctx, "test-collection")
	require.NoError(t, err)
	assert.Len(t, indexes, 2)
}

func TestAnimaProvider_DeleteIndex(t *testing.T) {
	provider, err := NewAnimaProvider(nil)
	require.NoError(t, err)

	ctx := context.Background()
	err = provider.Initialize(ctx, nil)
	require.NoError(t, err)

	err = provider.Start(ctx)
	require.NoError(t, err)

	err = provider.CreateCollection(ctx, "test-collection", nil)
	require.NoError(t, err)

	err = provider.CreateIndex(ctx, "test-collection", &IndexConfig{Name: "test-index"})
	require.NoError(t, err)

	err = provider.DeleteIndex(ctx, "test-collection", "test-index")
	require.NoError(t, err)

	indexes, err := provider.ListIndexes(ctx, "test-collection")
	require.NoError(t, err)
	assert.Empty(t, indexes)
}

// ========================================
// Metadata Tests
// ========================================

func TestAnimaProvider_AddMetadata(t *testing.T) {
	provider, err := NewAnimaProvider(nil)
	require.NoError(t, err)

	ctx := context.Background()
	err = provider.Initialize(ctx, nil)
	require.NoError(t, err)

	err = provider.Start(ctx)
	require.NoError(t, err)

	// First store a vector
	data := []*VectorData{{ID: "item-1", Vector: []float64{0.1, 0.2, 0.3}}}
	err = provider.Store(ctx, data)
	require.NoError(t, err)

	err = provider.AddMetadata(ctx, "item-1", map[string]interface{}{
		"key1": "value1",
		"key2": 123,
	})
	require.NoError(t, err)
}

func TestAnimaProvider_GetMetadata(t *testing.T) {
	provider, err := NewAnimaProvider(nil)
	require.NoError(t, err)

	ctx := context.Background()
	err = provider.Initialize(ctx, nil)
	require.NoError(t, err)

	err = provider.Start(ctx)
	require.NoError(t, err)

	// First store a vector
	data := []*VectorData{{ID: "item-1", Vector: []float64{0.1, 0.2, 0.3}}}
	err = provider.Store(ctx, data)
	require.NoError(t, err)

	err = provider.AddMetadata(ctx, "item-1", map[string]interface{}{
		"key1": "value1",
	})
	require.NoError(t, err)

	meta, err := provider.GetMetadata(ctx, []string{"item-1"})
	require.NoError(t, err)
	assert.NotNil(t, meta["item-1"])
	assert.Equal(t, "value1", meta["item-1"]["key1"])
}

func TestAnimaProvider_UpdateMetadata(t *testing.T) {
	provider, err := NewAnimaProvider(nil)
	require.NoError(t, err)

	ctx := context.Background()
	err = provider.Initialize(ctx, nil)
	require.NoError(t, err)

	err = provider.Start(ctx)
	require.NoError(t, err)

	// First store a vector
	data := []*VectorData{{ID: "item-1", Vector: []float64{0.1, 0.2, 0.3}}}
	err = provider.Store(ctx, data)
	require.NoError(t, err)

	err = provider.AddMetadata(ctx, "item-1", map[string]interface{}{
		"key1": "value1",
	})
	require.NoError(t, err)

	err = provider.UpdateMetadata(ctx, "item-1", map[string]interface{}{
		"key1": "updated",
		"key2": "new",
	})
	require.NoError(t, err)

	meta, err := provider.GetMetadata(ctx, []string{"item-1"})
	require.NoError(t, err)
	assert.Equal(t, "updated", meta["item-1"]["key1"])
	assert.Equal(t, "new", meta["item-1"]["key2"])
}

func TestAnimaProvider_DeleteMetadata(t *testing.T) {
	provider, err := NewAnimaProvider(nil)
	require.NoError(t, err)

	ctx := context.Background()
	err = provider.Initialize(ctx, nil)
	require.NoError(t, err)

	err = provider.Start(ctx)
	require.NoError(t, err)

	// First store a vector
	data := []*VectorData{{ID: "item-1", Vector: []float64{0.1, 0.2, 0.3}}}
	err = provider.Store(ctx, data)
	require.NoError(t, err)

	err = provider.AddMetadata(ctx, "item-1", map[string]interface{}{
		"key1": "value1",
		"key2": "value2",
	})
	require.NoError(t, err)

	err = provider.DeleteMetadata(ctx, []string{"item-1"}, []string{"key1"})
	require.NoError(t, err)

	meta, err := provider.GetMetadata(ctx, []string{"item-1"})
	require.NoError(t, err)
	_, exists := meta["item-1"]["key1"]
	assert.False(t, exists)
	assert.Equal(t, "value2", meta["item-1"]["key2"])
}

// ========================================
// Stats Tests
// ========================================

func TestAnimaProvider_GetStats(t *testing.T) {
	provider, err := NewAnimaProvider(nil)
	require.NoError(t, err)

	ctx := context.Background()
	err = provider.Initialize(ctx, nil)
	require.NoError(t, err)

	err = provider.Start(ctx)
	require.NoError(t, err)

	stats, err := provider.GetStats(ctx)
	require.NoError(t, err)
	assert.NotNil(t, stats)
	assert.Equal(t, "running", stats.Status)
}

func TestAnimaProvider_GetStats_NotStarted(t *testing.T) {
	provider, err := NewAnimaProvider(nil)
	require.NoError(t, err)

	ctx := context.Background()
	err = provider.Initialize(ctx, nil)
	require.NoError(t, err)

	stats, err := provider.GetStats(ctx)
	require.NoError(t, err)
	assert.NotNil(t, stats)
	assert.Equal(t, "initialized", stats.Status)
}

// ========================================
// Interface Verification
// ========================================

func TestAnimaProvider_NotNil(t *testing.T) {
	provider, err := NewAnimaProvider(nil)
	require.NoError(t, err)
	assert.NotNil(t, provider)
}
