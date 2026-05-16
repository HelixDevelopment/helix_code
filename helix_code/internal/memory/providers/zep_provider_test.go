package providers

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========================================
// ZepProvider Unit Tests
// ========================================

func TestNewZepProvider(t *testing.T) {
	tests := []struct {
		name    string
		config  map[string]interface{}
		wantErr bool
	}{
		{
			name:    "empty config",
			config:  map[string]interface{}{},
			wantErr: false,
		},
		{
			name: "with api_key",
			config: map[string]interface{}{
				"api_key": "test-api-key",
			},
			wantErr: false,
		},
		{
			name: "with all config options",
			config: map[string]interface{}{
				"api_key":  "test-api-key",
				"base_url": "https://api.getzep.com",
				"user_id":  "test-user",
			},
			wantErr: false,
		},
		{
			name:    "nil config",
			config:  nil,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewZepProvider(tt.config)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, provider)
			assert.NotNil(t, provider.client)
			assert.NotNil(t, provider.logger)
			assert.NotNil(t, provider.collections)
			assert.NotNil(t, provider.metadataCache)
		})
	}
}

func TestZepProvider_GetType(t *testing.T) {
	provider, err := NewZepProvider(map[string]interface{}{})
	require.NoError(t, err)

	assert.Equal(t, string(ProviderTypeZep), provider.GetType())
}

func TestZepProvider_GetName(t *testing.T) {
	provider, err := NewZepProvider(map[string]interface{}{})
	require.NoError(t, err)

	assert.Equal(t, "Zep", provider.GetName())
}

func TestZepProvider_GetCapabilities(t *testing.T) {
	provider, err := NewZepProvider(map[string]interface{}{})
	require.NoError(t, err)

	caps := provider.GetCapabilities()
	assert.NotEmpty(t, caps)
	assert.Contains(t, caps, "memory_storage")
	assert.Contains(t, caps, "memory_retrieval")
	assert.Contains(t, caps, "memory_search")
	assert.Contains(t, caps, "graph_memory")
	assert.Contains(t, caps, "knowledge_graph")
}

func TestZepProvider_GetConfiguration(t *testing.T) {
	config := map[string]interface{}{
		"api_key": "test-key",
		"user_id": "test-user",
	}
	provider, err := NewZepProvider(config)
	require.NoError(t, err)

	gotConfig := provider.GetConfiguration()
	assert.NotNil(t, gotConfig)
	configMap, ok := gotConfig.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "test-key", configMap["api_key"])
}

func TestZepProvider_IsCloud(t *testing.T) {
	tests := []struct {
		name     string
		baseURL  string
		expected bool
	}{
		{
			name:     "empty base URL (cloud)",
			baseURL:  "",
			expected: true,
		},
		{
			name:     "zep.ai URL (cloud)",
			baseURL:  "https://api.zep.ai/v2",
			expected: true,
		},
		{
			name:     "getzep.com URL (cloud)",
			baseURL:  "https://api.getzep.com/v2",
			expected: true,
		},
		{
			name:     "self-hosted URL",
			baseURL:  "http://localhost:8000",
			expected: false,
		},
		{
			name:     "custom domain",
			baseURL:  "https://zep.mycompany.com",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewZepProvider(map[string]interface{}{
				"base_url": tt.baseURL,
			})
			require.NoError(t, err)

			assert.Equal(t, tt.expected, provider.IsCloud())
		})
	}
}

func TestZepProvider_GetCostInfo(t *testing.T) {
	provider, err := NewZepProvider(map[string]interface{}{})
	require.NoError(t, err)

	costInfo := provider.GetCostInfo()
	assert.NotNil(t, costInfo)
	assert.Equal(t, "USD", costInfo.Currency)
	assert.Equal(t, "monthly", costInfo.BillingPeriod)
}

func TestZepProvider_GetStats(t *testing.T) {
	provider, err := NewZepProvider(map[string]interface{}{})
	require.NoError(t, err)

	ctx := context.Background()
	stats, err := provider.GetStats(ctx)

	require.NoError(t, err)
	assert.NotNil(t, stats)
	assert.Equal(t, "Zep", stats.Name)
	assert.Equal(t, "zep", stats.Type)
	assert.Equal(t, "active", stats.Status)
}

// ========================================
// Index Operation Tests
// ========================================

func TestZepProvider_CreateIndex_ReturnsError(t *testing.T) {
	provider, err := NewZepProvider(map[string]interface{}{})
	require.NoError(t, err)

	ctx := context.Background()
	err = provider.CreateIndex(ctx, "test-collection", &IndexConfig{
		Name: "test-index",
		Type: "flat",
	})

	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrZepIndexNotSupported))
	assert.Contains(t, err.Error(), "does not support manual index management")
}

func TestZepProvider_DeleteIndex_ReturnsError(t *testing.T) {
	provider, err := NewZepProvider(map[string]interface{}{})
	require.NoError(t, err)

	ctx := context.Background()
	err = provider.DeleteIndex(ctx, "test-collection", "test-index")

	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrZepIndexNotSupported))
}

func TestZepProvider_ListIndexes_ReturnsSyntheticIndex(t *testing.T) {
	provider, err := NewZepProvider(map[string]interface{}{})
	require.NoError(t, err)

	ctx := context.Background()
	indexes, err := provider.ListIndexes(ctx, "any-collection")

	require.NoError(t, err)
	require.Len(t, indexes, 1)
	assert.Equal(t, "zep_knowledge_graph", indexes[0].Name)
	assert.Equal(t, "automatic", indexes[0].Type)
	assert.Equal(t, "active", indexes[0].State)
	assert.Contains(t, indexes[0].Metadata["description"].(string), "automatically indexes")
}

// ========================================
// Metadata Operation Tests (Unit Tests without API)
// ========================================

func TestZepProvider_AddMetadata_EmptyID(t *testing.T) {
	provider, err := NewZepProvider(map[string]interface{}{})
	require.NoError(t, err)

	ctx := context.Background()
	err = provider.AddMetadata(ctx, "", map[string]interface{}{"key": "value"})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "id cannot be empty")
}

func TestZepProvider_AddMetadata_EmptyMetadata(t *testing.T) {
	provider, err := NewZepProvider(map[string]interface{}{})
	require.NoError(t, err)

	ctx := context.Background()
	err = provider.AddMetadata(ctx, "test-id", map[string]interface{}{})

	// Empty metadata should return nil (no-op)
	assert.NoError(t, err)
}

func TestZepProvider_AddMetadata_CachesNonUserMetadata(t *testing.T) {
	provider, err := NewZepProvider(map[string]interface{}{})
	require.NoError(t, err)

	ctx := context.Background()
	// Use a UUID-like ID to trigger non-user path
	id := "abc-123-def-456"
	metadata := map[string]interface{}{
		"key1": "value1",
		"key2": 42,
	}

	err = provider.AddMetadata(ctx, id, metadata)
	require.NoError(t, err)

	// Verify it was cached
	provider.metadataCacheMu.RLock()
	cached, ok := provider.metadataCache[id]
	provider.metadataCacheMu.RUnlock()

	assert.True(t, ok)
	assert.Equal(t, "value1", cached["key1"])
	assert.Equal(t, 42, cached["key2"])
}

func TestZepProvider_UpdateMetadata_EmptyID(t *testing.T) {
	provider, err := NewZepProvider(map[string]interface{}{})
	require.NoError(t, err)

	ctx := context.Background()
	err = provider.UpdateMetadata(ctx, "", map[string]interface{}{"key": "value"})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "id cannot be empty")
}

func TestZepProvider_UpdateMetadata_EmptyMetadata(t *testing.T) {
	provider, err := NewZepProvider(map[string]interface{}{})
	require.NoError(t, err)

	ctx := context.Background()
	err = provider.UpdateMetadata(ctx, "test-id", map[string]interface{}{})

	assert.NoError(t, err)
}

func TestZepProvider_UpdateMetadata_UpdatesCache(t *testing.T) {
	provider, err := NewZepProvider(map[string]interface{}{})
	require.NoError(t, err)

	ctx := context.Background()
	id := "abc-123-def-456"

	// Add initial metadata
	err = provider.AddMetadata(ctx, id, map[string]interface{}{"key1": "value1"})
	require.NoError(t, err)

	// Update metadata
	err = provider.UpdateMetadata(ctx, id, map[string]interface{}{"key1": "updated", "key2": "new"})
	require.NoError(t, err)

	// Verify updates
	provider.metadataCacheMu.RLock()
	cached := provider.metadataCache[id]
	provider.metadataCacheMu.RUnlock()

	assert.Equal(t, "updated", cached["key1"])
	assert.Equal(t, "new", cached["key2"])
}

func TestZepProvider_GetMetadata_EmptyIDs(t *testing.T) {
	provider, err := NewZepProvider(map[string]interface{}{})
	require.NoError(t, err)

	ctx := context.Background()
	result, err := provider.GetMetadata(ctx, []string{})

	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestZepProvider_GetMetadata_FromCache(t *testing.T) {
	provider, err := NewZepProvider(map[string]interface{}{})
	require.NoError(t, err)

	ctx := context.Background()
	id := "abc-123-def-456"

	// Add metadata to cache
	provider.metadataCacheMu.Lock()
	provider.metadataCache[id] = map[string]interface{}{"key": "value"}
	provider.metadataCacheMu.Unlock()

	// Get metadata
	result, err := provider.GetMetadata(ctx, []string{id})

	require.NoError(t, err)
	assert.Contains(t, result, id)
	assert.Equal(t, "value", result[id]["key"])
}

func TestZepProvider_DeleteMetadata_EmptyInputs(t *testing.T) {
	provider, err := NewZepProvider(map[string]interface{}{})
	require.NoError(t, err)

	ctx := context.Background()

	// Empty IDs
	err = provider.DeleteMetadata(ctx, []string{}, []string{"key"})
	assert.NoError(t, err)

	// Empty keys
	err = provider.DeleteMetadata(ctx, []string{"id"}, []string{})
	assert.NoError(t, err)
}

func TestZepProvider_DeleteMetadata_CleansCache(t *testing.T) {
	provider, err := NewZepProvider(map[string]interface{}{})
	require.NoError(t, err)

	ctx := context.Background()
	id := "abc-123-def-456"

	// Add metadata to cache
	provider.metadataCacheMu.Lock()
	provider.metadataCache[id] = map[string]interface{}{
		"key1": "value1",
		"key2": "value2",
	}
	provider.metadataCacheMu.Unlock()

	// Delete one key
	err = provider.DeleteMetadata(ctx, []string{id}, []string{"key1"})
	require.NoError(t, err)

	// Verify key1 was removed
	provider.metadataCacheMu.RLock()
	cached := provider.metadataCache[id]
	provider.metadataCacheMu.RUnlock()

	assert.NotContains(t, cached, "key1")
	assert.Contains(t, cached, "key2")
}

func TestZepProvider_DeleteMetadata_RemovesEmptyCache(t *testing.T) {
	provider, err := NewZepProvider(map[string]interface{}{})
	require.NoError(t, err)

	ctx := context.Background()
	id := "abc-123-def-456"

	// Add metadata to cache
	provider.metadataCacheMu.Lock()
	provider.metadataCache[id] = map[string]interface{}{"key1": "value1"}
	provider.metadataCacheMu.Unlock()

	// Delete the only key
	err = provider.DeleteMetadata(ctx, []string{id}, []string{"key1"})
	require.NoError(t, err)

	// Verify cache entry was removed
	provider.metadataCacheMu.RLock()
	_, exists := provider.metadataCache[id]
	provider.metadataCacheMu.RUnlock()

	assert.False(t, exists)
}

// ========================================
// Collection Operation Tests (Unit Tests)
// ========================================

func TestZepProvider_CreateCollection_EmptyName(t *testing.T) {
	provider, err := NewZepProvider(map[string]interface{}{})
	require.NoError(t, err)

	ctx := context.Background()
	err = provider.CreateCollection(ctx, "", &CollectionConfig{})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "collection name (user ID) cannot be empty")
}

func TestZepProvider_DeleteCollection_EmptyName(t *testing.T) {
	provider, err := NewZepProvider(map[string]interface{}{})
	require.NoError(t, err)

	ctx := context.Background()
	err = provider.DeleteCollection(ctx, "")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "collection name (user ID) cannot be empty")
}

func TestZepProvider_GetCollection_EmptyName(t *testing.T) {
	provider, err := NewZepProvider(map[string]interface{}{})
	require.NoError(t, err)

	ctx := context.Background()
	_, err = provider.GetCollection(ctx, "")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "collection name (user ID) cannot be empty")
}

func TestZepProvider_GetCollection_FromCache(t *testing.T) {
	provider, err := NewZepProvider(map[string]interface{}{})
	require.NoError(t, err)

	// Add to cache
	cachedCollection := &CollectionInfo{
		Name:      "cached-user",
		Status:    "active",
		CreatedAt: time.Now(),
	}
	provider.collectionsMu.Lock()
	provider.collections["cached-user"] = cachedCollection
	provider.collectionsMu.Unlock()

	ctx := context.Background()
	result, err := provider.GetCollection(ctx, "cached-user")

	require.NoError(t, err)
	assert.Equal(t, cachedCollection, result)
}

// ========================================
// Optimize Tests
// ========================================

func TestZepProvider_Optimize_NoUsersConfigured(t *testing.T) {
	provider, err := NewZepProvider(map[string]interface{}{})
	require.NoError(t, err)

	ctx := context.Background()
	// This won't call the API since no userID is set and no collections cached
	err = provider.Optimize(ctx)

	// Should not return error even without users to warm
	assert.NoError(t, err)
}

// ========================================
// Backup/Restore Tests
// ========================================

func TestZepProvider_Backup_EmptyPath(t *testing.T) {
	provider, err := NewZepProvider(map[string]interface{}{})
	require.NoError(t, err)

	ctx := context.Background()
	err = provider.Backup(ctx, "")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "backup path cannot be empty")
}

func TestZepProvider_Backup_NoUserConfigured(t *testing.T) {
	provider, err := NewZepProvider(map[string]interface{}{})
	require.NoError(t, err)

	ctx := context.Background()
	err = provider.Backup(ctx, "/tmp/backup.json")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no user_id configured")
}

func TestZepProvider_Restore_EmptyPath(t *testing.T) {
	provider, err := NewZepProvider(map[string]interface{}{})
	require.NoError(t, err)

	ctx := context.Background()
	err = provider.Restore(ctx, "")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "restore path cannot be empty")
}

func TestZepProvider_Restore_NoBackupData(t *testing.T) {
	provider, err := NewZepProvider(map[string]interface{}{})
	require.NoError(t, err)

	ctx := context.Background()
	err = provider.Restore(ctx, "/tmp/backup.json")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "restore not supported")
}

// ========================================
// Lifecycle Tests
// ========================================

func TestZepProvider_Initialize(t *testing.T) {
	provider, err := NewZepProvider(map[string]interface{}{})
	require.NoError(t, err)

	ctx := context.Background()
	err = provider.Initialize(ctx, nil)

	assert.NoError(t, err)
}

func TestZepProvider_Start(t *testing.T) {
	provider, err := NewZepProvider(map[string]interface{}{})
	require.NoError(t, err)

	ctx := context.Background()
	err = provider.Start(ctx)

	assert.NoError(t, err)
}

func TestZepProvider_Stop(t *testing.T) {
	provider, err := NewZepProvider(map[string]interface{}{})
	require.NoError(t, err)

	ctx := context.Background()
	err = provider.Stop(ctx)

	assert.NoError(t, err)
}

func TestZepProvider_Close(t *testing.T) {
	provider, err := NewZepProvider(map[string]interface{}{})
	require.NoError(t, err)

	ctx := context.Background()
	err = provider.Close(ctx)

	assert.NoError(t, err)
}

// ========================================
// Helper Function Tests
// ========================================

func TestSafeString(t *testing.T) {
	tests := []struct {
		name     string
		input    *string
		expected string
	}{
		{
			name:     "nil pointer",
			input:    nil,
			expected: "",
		},
		{
			name:     "empty string",
			input:    stringPtr(""),
			expected: "",
		},
		{
			name:     "non-empty string",
			input:    stringPtr("hello"),
			expected: "hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := safeString(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGenerateThreadID(t *testing.T) {
	id1 := generateThreadID()
	id2 := generateThreadID()

	assert.Contains(t, id1, "thread-")
	assert.Contains(t, id2, "thread-")
	// IDs should be different (based on timestamp)
	assert.NotEqual(t, id1, id2)
}

func TestContains(t *testing.T) {
	tests := []struct {
		s        string
		substr   string
		expected bool
	}{
		{"hello world", "world", true},
		{"hello world", "hello", true},
		{"hello world", "lo wo", true},
		{"hello world", "xyz", false},
		{"", "", true},
		{"hello", "", true},
		{"", "hello", false},
	}

	for _, tt := range tests {
		result := contains(tt.s, tt.substr)
		assert.Equal(t, tt.expected, result, "contains(%q, %q) = %v, want %v", tt.s, tt.substr, result, tt.expected)
	}
}

func TestIsKnownUser(t *testing.T) {
	provider, err := NewZepProvider(map[string]interface{}{})
	require.NoError(t, err)

	// Add a user to collections
	provider.collectionsMu.Lock()
	provider.collections["known-user"] = &CollectionInfo{Name: "known-user"}
	provider.collectionsMu.Unlock()

	assert.True(t, provider.isKnownUser("known-user"))
	assert.False(t, provider.isKnownUser("unknown-user"))
}

// ========================================
// Error Variable Tests
// ========================================

func TestErrZepIndexNotSupported(t *testing.T) {
	assert.NotNil(t, ErrZepIndexNotSupported)
	assert.Contains(t, ErrZepIndexNotSupported.Error(), "does not support manual index management")
	assert.Contains(t, ErrZepIndexNotSupported.Error(), "automatically handles indexing")
}

func TestErrZepBackupNotSupported(t *testing.T) {
	assert.NotNil(t, ErrZepBackupNotSupported)
	assert.Contains(t, ErrZepBackupNotSupported.Error(), "does not support direct backup/restore")
	assert.Contains(t, ErrZepBackupNotSupported.Error(), "automatically persisted")
}

// ========================================
// Store/Retrieve Tests (Unit Tests)
// ========================================

func TestZepProvider_Store_EmptyData(t *testing.T) {
	provider, err := NewZepProvider(map[string]interface{}{})
	require.NoError(t, err)

	ctx := context.Background()
	err = provider.Store(ctx, []*VectorData{})

	// Empty data should return nil
	assert.NoError(t, err)
}

func TestZepProvider_Retrieve_LogsWarning(t *testing.T) {
	provider, err := NewZepProvider(map[string]interface{}{})
	require.NoError(t, err)

	ctx := context.Background()
	result, err := provider.Retrieve(ctx, []string{"id1", "id2"})

	// Retrieve is a stub that returns empty
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestZepProvider_Update_LogsWarning(t *testing.T) {
	provider, err := NewZepProvider(map[string]interface{}{})
	require.NoError(t, err)

	ctx := context.Background()
	err = provider.Update(ctx, "test-id", &VectorData{ID: "test-id"})

	// Update is a stub that returns nil
	assert.NoError(t, err)
}

func TestZepProvider_Delete_LogsWarning(t *testing.T) {
	provider, err := NewZepProvider(map[string]interface{}{})
	require.NoError(t, err)

	ctx := context.Background()
	err = provider.Delete(ctx, []string{"id1", "id2"})

	// Delete calls the real API - expect 401 error without credentials
	// The method logs warnings but continues, so the last error is returned
	assert.Error(t, err)
}

// ========================================
// Search Tests (Unit Tests)
// ========================================

func TestZepProvider_FindSimilar_EmptyEmbedding(t *testing.T) {
	provider, err := NewZepProvider(map[string]interface{}{})
	require.NoError(t, err)

	ctx := context.Background()
	// This will call the API, but without proper credentials will likely fail
	// The test verifies the method exists and handles the call structure
	_, err = provider.FindSimilar(ctx, []float64{}, 10, nil)

	// We expect an error due to no API credentials
	// This test validates the method is callable
	assert.Error(t, err) // Will fail due to API authentication
}

func TestZepProvider_BatchFindSimilar_EmptyQueries(t *testing.T) {
	provider, err := NewZepProvider(map[string]interface{}{})
	require.NoError(t, err)

	ctx := context.Background()
	results, err := provider.BatchFindSimilar(ctx, [][]float64{}, 10)

	require.NoError(t, err)
	assert.Empty(t, results)
}

// Helper function for tests
func stringPtr(s string) *string {
	return &s
}
