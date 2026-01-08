package providers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========================================
// Test Setup and Helpers
// ========================================

func newTestMem0Provider(baseURL string) *Mem0Provider {
	config := map[string]interface{}{
		"base_url": baseURL,
		"api_key":  "test-api-key",
		"user_id":  "test-user",
		"agent_id": "test-agent",
		"run_id":   "test-run",
	}
	provider, _ := NewMem0Provider(config)
	return provider
}

func newTestMem0ProviderMinimal(baseURL string) *Mem0Provider {
	config := map[string]interface{}{
		"base_url": baseURL,
		"api_key":  "test-api-key",
	}
	provider, _ := NewMem0Provider(config)
	return provider
}

// ========================================
// NewMem0Provider Tests
// ========================================

func TestNewMem0Provider(t *testing.T) {
	tests := []struct {
		name     string
		config   map[string]interface{}
		expected struct {
			baseURL string
			apiKey  string
			userID  string
			agentID string
			runID   string
		}
	}{
		{
			name:   "with all config values",
			config: map[string]interface{}{
				"base_url": "https://custom.mem0.ai",
				"api_key":  "my-api-key",
				"user_id":  "user-123",
				"agent_id": "agent-456",
				"run_id":   "run-789",
			},
			expected: struct {
				baseURL string
				apiKey  string
				userID  string
				agentID string
				runID   string
			}{
				baseURL: "https://custom.mem0.ai",
				apiKey:  "my-api-key",
				userID:  "user-123",
				agentID: "agent-456",
				runID:   "run-789",
			},
		},
		{
			name:   "with default base URL",
			config: map[string]interface{}{
				"api_key": "my-api-key",
			},
			expected: struct {
				baseURL string
				apiKey  string
				userID  string
				agentID string
				runID   string
			}{
				baseURL: "https://api.mem0.ai",
				apiKey:  "my-api-key",
				userID:  "",
				agentID: "",
				runID:   "",
			},
		},
		{
			name:   "with empty config",
			config: map[string]interface{}{},
			expected: struct {
				baseURL string
				apiKey  string
				userID  string
				agentID string
				runID   string
			}{
				baseURL: "https://api.mem0.ai",
				apiKey:  "",
				userID:  "",
				agentID: "",
				runID:   "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewMem0Provider(tt.config)
			require.NoError(t, err)
			require.NotNil(t, provider)

			assert.Equal(t, tt.expected.baseURL, provider.baseURL)
			assert.Equal(t, tt.expected.apiKey, provider.apiKey)
			assert.Equal(t, tt.expected.userID, provider.userID)
			assert.Equal(t, tt.expected.agentID, provider.agentID)
			assert.Equal(t, tt.expected.runID, provider.runID)
		})
	}
}

// ========================================
// Provider Metadata Tests
// ========================================

func TestMem0Provider_GetType(t *testing.T) {
	provider := newTestMem0Provider("http://localhost")
	assert.Equal(t, "mem0", provider.GetType())
}

func TestMem0Provider_GetName(t *testing.T) {
	provider := newTestMem0Provider("http://localhost")
	assert.Equal(t, "Mem0", provider.GetName())
}

func TestMem0Provider_GetCapabilities(t *testing.T) {
	provider := newTestMem0Provider("http://localhost")
	caps := provider.GetCapabilities()

	assert.Contains(t, caps, "memory_storage")
	assert.Contains(t, caps, "memory_retrieval")
	assert.Contains(t, caps, "memory_search")
	assert.Contains(t, caps, "context_management")
	assert.Contains(t, caps, "graph_memory")
	assert.Contains(t, caps, "vector_memory")
}

func TestMem0Provider_GetConfiguration(t *testing.T) {
	config := map[string]interface{}{
		"api_key": "test-key",
		"user_id": "test-user",
	}
	provider, _ := NewMem0Provider(config)

	returnedConfig := provider.GetConfiguration()
	assert.Equal(t, config, returnedConfig)
}

func TestMem0Provider_IsCloud(t *testing.T) {
	tests := []struct {
		name     string
		baseURL  string
		expected bool
	}{
		{
			name:     "mem0.ai URL is cloud",
			baseURL:  "https://api.mem0.ai",
			expected: true,
		},
		{
			name:     "custom URL is not cloud",
			baseURL:  "http://localhost:8080",
			expected: false,
		},
		{
			name:     "self-hosted mem0 is not cloud",
			baseURL:  "https://mem0.mycompany.com",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := newTestMem0Provider(tt.baseURL)
			assert.Equal(t, tt.expected, provider.IsCloud())
		})
	}
}

func TestMem0Provider_GetCostInfo(t *testing.T) {
	provider := newTestMem0Provider("http://localhost")
	costInfo := provider.GetCostInfo()

	assert.NotNil(t, costInfo)
	assert.Equal(t, "USD", costInfo.Currency)
	assert.Equal(t, "monthly", costInfo.BillingPeriod)
}

// ========================================
// Collection Management Tests (Unsupported Operations)
// ========================================

func TestMem0Provider_CreateCollection_NotSupported(t *testing.T) {
	provider := newTestMem0Provider("http://localhost")
	ctx := context.Background()

	err := provider.CreateCollection(ctx, "test-collection", &CollectionConfig{
		Name:      "test-collection",
		Dimension: 1536,
		Metric:    "cosine",
	})

	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrMem0OperationNotSupported))
	assert.Contains(t, err.Error(), "does not support traditional collections")
}

func TestMem0Provider_DeleteCollection_NotSupported(t *testing.T) {
	provider := newTestMem0Provider("http://localhost")
	ctx := context.Background()

	err := provider.DeleteCollection(ctx, "test-collection")

	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrMem0OperationNotSupported))
	assert.Contains(t, err.Error(), "does not support collection deletion")
}

func TestMem0Provider_ListCollections_WithConfiguration(t *testing.T) {
	provider := newTestMem0Provider("http://localhost")
	ctx := context.Background()

	collections, err := provider.ListCollections(ctx)

	require.NoError(t, err)
	assert.Len(t, collections, 3) // user, agent, run

	// Verify user collection
	userFound := false
	agentFound := false
	runFound := false

	for _, col := range collections {
		if col.Name == "user_test-user" {
			userFound = true
			assert.Equal(t, "active", col.Status)
			assert.Equal(t, "user_memories", col.Metadata["type"])
		}
		if col.Name == "agent_test-agent" {
			agentFound = true
			assert.Equal(t, "active", col.Status)
			assert.Equal(t, "agent_memories", col.Metadata["type"])
		}
		if col.Name == "run_test-run" {
			runFound = true
			assert.Equal(t, "active", col.Status)
			assert.Equal(t, "run_memories", col.Metadata["type"])
		}
	}

	assert.True(t, userFound, "User collection not found")
	assert.True(t, agentFound, "Agent collection not found")
	assert.True(t, runFound, "Run collection not found")
}

func TestMem0Provider_ListCollections_WithoutConfiguration(t *testing.T) {
	provider := newTestMem0ProviderMinimal("http://localhost")
	ctx := context.Background()

	collections, err := provider.ListCollections(ctx)

	require.NoError(t, err)
	assert.Len(t, collections, 1)
	assert.Equal(t, "all_memories", collections[0].Name)
	assert.Equal(t, "all_memories", collections[0].Metadata["type"])
}

func TestMem0Provider_GetCollection_Found(t *testing.T) {
	provider := newTestMem0Provider("http://localhost")
	ctx := context.Background()

	col, err := provider.GetCollection(ctx, "user_test-user")

	require.NoError(t, err)
	assert.NotNil(t, col)
	assert.Equal(t, "user_test-user", col.Name)
}

func TestMem0Provider_GetCollection_NotFound(t *testing.T) {
	provider := newTestMem0Provider("http://localhost")
	ctx := context.Background()

	col, err := provider.GetCollection(ctx, "nonexistent-collection")

	assert.Error(t, err)
	assert.Nil(t, col)
	assert.True(t, errors.Is(err, ErrMem0OperationNotSupported))
}

// ========================================
// Index Management Tests (Unsupported Operations)
// ========================================

func TestMem0Provider_CreateIndex_NotSupported(t *testing.T) {
	provider := newTestMem0Provider("http://localhost")
	ctx := context.Background()

	err := provider.CreateIndex(ctx, "collection", &IndexConfig{
		Name: "test-index",
		Type: "IVF_FLAT",
	})

	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrMem0OperationNotSupported))
	assert.Contains(t, err.Error(), "managed service")
}

func TestMem0Provider_DeleteIndex_NotSupported(t *testing.T) {
	provider := newTestMem0Provider("http://localhost")
	ctx := context.Background()

	err := provider.DeleteIndex(ctx, "collection", "index-name")

	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrMem0OperationNotSupported))
	assert.Contains(t, err.Error(), "managed service")
}

func TestMem0Provider_ListIndexes(t *testing.T) {
	provider := newTestMem0Provider("http://localhost")
	ctx := context.Background()

	indexes, err := provider.ListIndexes(ctx, "collection")

	require.NoError(t, err)
	assert.Len(t, indexes, 1)
	assert.Equal(t, "mem0_managed_index", indexes[0].Name)
	assert.Equal(t, "managed", indexes[0].Type)
	assert.Equal(t, "active", indexes[0].State)
	assert.True(t, indexes[0].Metadata["managed"].(bool))
}

// ========================================
// Backup/Restore Tests (Unsupported Operations)
// ========================================

func TestMem0Provider_Backup_NotSupported(t *testing.T) {
	provider := newTestMem0Provider("http://localhost")
	ctx := context.Background()

	err := provider.Backup(ctx, "/tmp/backup")

	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrMem0OperationNotSupported))
	assert.Contains(t, err.Error(), "managed cloud service")
}

func TestMem0Provider_Restore_NotSupported(t *testing.T) {
	provider := newTestMem0Provider("http://localhost")
	ctx := context.Background()

	err := provider.Restore(ctx, "/tmp/backup")

	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrMem0OperationNotSupported))
	assert.Contains(t, err.Error(), "managed cloud service")
}

// ========================================
// Optimize Test (No-op)
// ========================================

func TestMem0Provider_Optimize(t *testing.T) {
	provider := newTestMem0Provider("http://localhost")
	ctx := context.Background()

	err := provider.Optimize(ctx)

	// Should succeed (no-op)
	require.NoError(t, err)
}

// ========================================
// GetMetadata Tests
// ========================================

func TestMem0Provider_GetMetadata_EmptyIDs(t *testing.T) {
	provider := newTestMem0Provider("http://localhost")
	ctx := context.Background()

	result, err := provider.GetMetadata(ctx, []string{})

	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestMem0Provider_GetMetadata_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && r.URL.Path == "/memories/mem-123/" {
			response := map[string]interface{}{
				"id":   "mem-123",
				"text": "test memory content",
				"metadata": map[string]interface{}{
					"category": "test",
					"priority": 1,
				},
			}
			json.NewEncoder(w).Encode(response)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	provider := newTestMem0Provider(server.URL)
	ctx := context.Background()

	result, err := provider.GetMetadata(ctx, []string{"mem-123"})

	require.NoError(t, err)
	assert.Contains(t, result, "mem-123")
	assert.Equal(t, "test", result["mem-123"]["category"])
	assert.Equal(t, float64(1), result["mem-123"]["priority"])
}

func TestMem0Provider_GetMetadata_PartialFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/memories/mem-exists/" {
			response := map[string]interface{}{
				"id": "mem-exists",
				"metadata": map[string]interface{}{
					"key": "value",
				},
			}
			json.NewEncoder(w).Encode(response)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	provider := newTestMem0Provider(server.URL)
	ctx := context.Background()

	result, err := provider.GetMetadata(ctx, []string{"mem-exists", "mem-not-found"})

	require.NoError(t, err)
	assert.Contains(t, result, "mem-exists")
	assert.Contains(t, result, "mem-not-found")
	assert.Equal(t, "value", result["mem-exists"]["key"])
	assert.Empty(t, result["mem-not-found"]) // Empty map for not found
}

// ========================================
// DeleteMetadata Tests
// ========================================

func TestMem0Provider_DeleteMetadata_EmptyInputs(t *testing.T) {
	provider := newTestMem0Provider("http://localhost")
	ctx := context.Background()

	// Empty IDs
	err := provider.DeleteMetadata(ctx, []string{}, []string{"key"})
	require.NoError(t, err)

	// Empty keys
	err = provider.DeleteMetadata(ctx, []string{"id"}, []string{})
	require.NoError(t, err)
}

func TestMem0Provider_DeleteMetadata_Success(t *testing.T) {
	getCount := 0
	putCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && r.URL.Path == "/memories/mem-123/" {
			getCount++
			response := map[string]interface{}{
				"id": "mem-123",
				"metadata": map[string]interface{}{
					"key1": "value1",
					"key2": "value2",
					"key3": "value3",
				},
			}
			json.NewEncoder(w).Encode(response)
			return
		}
		if r.Method == "PUT" && r.URL.Path == "/memories/mem-123/" {
			putCount++
			w.WriteHeader(http.StatusOK)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	provider := newTestMem0Provider(server.URL)
	ctx := context.Background()

	err := provider.DeleteMetadata(ctx, []string{"mem-123"}, []string{"key1", "key2"})

	require.NoError(t, err)
	assert.Equal(t, 1, getCount)
	assert.Equal(t, 1, putCount)
}

// ========================================
// Retrieve Tests
// ========================================

func TestMem0Provider_Retrieve_EmptyIDs(t *testing.T) {
	provider := newTestMem0Provider("http://localhost")
	ctx := context.Background()

	result, err := provider.Retrieve(ctx, []string{})

	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestMem0Provider_Retrieve_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && r.URL.Path == "/memories/mem-123/" {
			response := map[string]interface{}{
				"id":     "mem-123",
				"text":   "This is the memory content",
				"memory": "Processed memory text",
				"metadata": map[string]interface{}{
					"source": "test",
				},
			}
			json.NewEncoder(w).Encode(response)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	provider := newTestMem0Provider(server.URL)
	ctx := context.Background()

	result, err := provider.Retrieve(ctx, []string{"mem-123"})

	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "mem-123", result[0].ID)
	assert.Equal(t, "This is the memory content", result[0].Metadata["content"])
	assert.Equal(t, "Processed memory text", result[0].Metadata["memory"])
	assert.Equal(t, "test", result[0].Metadata["source"])
}

func TestMem0Provider_Retrieve_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	provider := newTestMem0Provider(server.URL)
	ctx := context.Background()

	result, err := provider.Retrieve(ctx, []string{"nonexistent"})

	require.NoError(t, err)
	assert.Empty(t, result) // Not found items are skipped
}

// ========================================
// Store Tests
// ========================================

func TestMem0Provider_Store_EmptyData(t *testing.T) {
	provider := newTestMem0Provider("http://localhost")
	ctx := context.Background()

	err := provider.Store(ctx, []*VectorData{})

	require.NoError(t, err)
}

func TestMem0Provider_Store_Success(t *testing.T) {
	requestReceived := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/memories/" {
			requestReceived = true

			var payload map[string]interface{}
			json.NewDecoder(r.Body).Decode(&payload)

			// Verify payload structure
			memories, ok := payload["memories"].([]interface{})
			assert.True(t, ok)
			assert.Len(t, memories, 1)

			w.WriteHeader(http.StatusCreated)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	provider := newTestMem0Provider(server.URL)
	ctx := context.Background()

	vectors := []*VectorData{
		{
			ID:     "vec-1",
			Vector: []float64{0.1, 0.2, 0.3},
			Metadata: map[string]interface{}{
				"content": "Test memory content",
			},
		},
	}

	err := provider.Store(ctx, vectors)

	require.NoError(t, err)
	assert.True(t, requestReceived)
}

// ========================================
// Search Tests
// ========================================

func TestMem0Provider_Search_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/search/" {
			response := map[string]interface{}{
				"memories": []interface{}{
					map[string]interface{}{
						"id":    "mem-1",
						"score": 0.95,
						"text":  "Matching memory",
					},
					map[string]interface{}{
						"id":    "mem-2",
						"score": 0.85,
						"text":  "Another match",
					},
				},
			}
			json.NewEncoder(w).Encode(response)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	provider := newTestMem0Provider(server.URL)
	ctx := context.Background()

	query := &VectorQuery{
		Text: "search query",
		TopK: 10,
	}

	result, err := provider.Search(ctx, query)

	require.NoError(t, err)
	assert.Len(t, result.Results, 2)
	assert.Equal(t, "mem-1", result.Results[0].ID)
	assert.Equal(t, 0.95, result.Results[0].Score)
	assert.Equal(t, "mem-2", result.Results[1].ID)
	assert.Equal(t, 0.85, result.Results[1].Score)
}

// ========================================
// Delete Tests
// ========================================

func TestMem0Provider_Delete_EmptyIDs(t *testing.T) {
	provider := newTestMem0Provider("http://localhost")
	ctx := context.Background()

	err := provider.Delete(ctx, []string{})

	require.NoError(t, err)
}

func TestMem0Provider_Delete_Success(t *testing.T) {
	deleteReceived := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" && r.URL.Path == "/memories/" {
			deleteReceived = true
			w.WriteHeader(http.StatusOK)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	provider := newTestMem0Provider(server.URL)
	ctx := context.Background()

	err := provider.Delete(ctx, []string{"mem-1", "mem-2"})

	require.NoError(t, err)
	assert.True(t, deleteReceived)
}

// ========================================
// Health Tests
// ========================================

func TestMem0Provider_Health_Healthy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health/" {
			response := map[string]interface{}{
				"status": "ok",
			}
			json.NewEncoder(w).Encode(response)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	provider := newTestMem0Provider(server.URL)
	ctx := context.Background()

	health, err := provider.Health(ctx)

	require.NoError(t, err)
	assert.Equal(t, "healthy", health.Status)
}

func TestMem0Provider_Health_Unhealthy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health/" {
			response := map[string]interface{}{
				"status": "error",
			}
			json.NewEncoder(w).Encode(response)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	provider := newTestMem0Provider(server.URL)
	ctx := context.Background()

	health, err := provider.Health(ctx)

	require.NoError(t, err)
	assert.Equal(t, "unhealthy", health.Status)
}

func TestMem0Provider_Health_ConnectionError(t *testing.T) {
	// Use invalid URL to trigger connection error
	provider := newTestMem0Provider("http://localhost:99999")
	ctx := context.Background()

	health, err := provider.Health(ctx)

	require.NoError(t, err) // Health returns status, not error
	assert.Equal(t, "unhealthy", health.Status)
	assert.NotEmpty(t, health.Message)
}

// ========================================
// GetStats Tests
// ========================================

func TestMem0Provider_GetStats_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/stats/" {
			response := map[string]interface{}{
				"total_memories":   100,
				"total_size_bytes": 1048576,
				"avg_latency_ms":   50,
			}
			json.NewEncoder(w).Encode(response)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	provider := newTestMem0Provider(server.URL)
	ctx := context.Background()

	stats, err := provider.GetStats(ctx)

	require.NoError(t, err)
	assert.Equal(t, int64(100), stats.TotalVectors)
	assert.Equal(t, int64(1048576), stats.TotalSize)
	assert.Equal(t, 50*time.Millisecond, stats.AverageLatency)
}

// ========================================
// UpdateMetadata Tests
// ========================================

func TestMem0Provider_UpdateMetadata_Success(t *testing.T) {
	updateReceived := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PUT" && r.URL.Path == "/memories/mem-123/" {
			updateReceived = true

			var payload map[string]interface{}
			json.NewDecoder(r.Body).Decode(&payload)

			assert.Equal(t, "mem-123", payload["memory_id"])
			assert.NotNil(t, payload["metadata"])

			w.WriteHeader(http.StatusOK)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	provider := newTestMem0Provider(server.URL)
	ctx := context.Background()

	err := provider.UpdateMetadata(ctx, "mem-123", map[string]interface{}{
		"key": "value",
	})

	require.NoError(t, err)
	assert.True(t, updateReceived)
}

// ========================================
// AddMetadata Tests
// ========================================

func TestMem0Provider_AddMetadata_DelegatesToUpdate(t *testing.T) {
	updateReceived := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PUT" && r.URL.Path == "/memories/mem-123/" {
			updateReceived = true
			w.WriteHeader(http.StatusOK)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	provider := newTestMem0Provider(server.URL)
	ctx := context.Background()

	err := provider.AddMetadata(ctx, "mem-123", map[string]interface{}{
		"new_key": "new_value",
	})

	require.NoError(t, err)
	assert.True(t, updateReceived)
}

// ========================================
// FindSimilar Tests
// ========================================

func TestMem0Provider_FindSimilar_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/search/" {
			response := map[string]interface{}{
				"memories": []interface{}{
					map[string]interface{}{
						"id":    "similar-1",
						"score": 0.9,
					},
				},
			}
			json.NewEncoder(w).Encode(response)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	provider := newTestMem0Provider(server.URL)
	ctx := context.Background()

	results, err := provider.FindSimilar(ctx, []float64{0.1, 0.2}, 5, nil)

	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "similar-1", results[0].ID)
	assert.Equal(t, 0.9, results[0].Score)
	assert.InDelta(t, 0.1, results[0].Distance, 0.0001) // 1.0 - score (use InDelta for float comparison)
}

// ========================================
// BatchFindSimilar Tests
// ========================================

func TestMem0Provider_BatchFindSimilar_Success(t *testing.T) {
	callCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/search/" {
			callCount++
			response := map[string]interface{}{
				"memories": []interface{}{
					map[string]interface{}{
						"id":    "similar-1",
						"score": 0.9,
					},
				},
			}
			json.NewEncoder(w).Encode(response)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	provider := newTestMem0Provider(server.URL)
	ctx := context.Background()

	queries := [][]float64{
		{0.1, 0.2},
		{0.3, 0.4},
		{0.5, 0.6},
	}

	results, err := provider.BatchFindSimilar(ctx, queries, 5)

	require.NoError(t, err)
	assert.Len(t, results, 3)
	assert.Equal(t, 3, callCount) // One call per query
}

// ========================================
// Lifecycle Tests
// ========================================

func TestMem0Provider_Lifecycle(t *testing.T) {
	provider := newTestMem0Provider("http://localhost")
	ctx := context.Background()

	// Initialize
	err := provider.Initialize(ctx, nil)
	require.NoError(t, err)

	// Start
	err = provider.Start(ctx)
	require.NoError(t, err)

	// Stop
	err = provider.Stop(ctx)
	require.NoError(t, err)

	// Close
	err = provider.Close(ctx)
	require.NoError(t, err)
}

// ========================================
// Update Tests
// ========================================

func TestMem0Provider_Update(t *testing.T) {
	updateReceived := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PUT" && r.URL.Path == "/memories/vec-1/" {
			updateReceived = true
			w.WriteHeader(http.StatusOK)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	provider := newTestMem0Provider(server.URL)
	ctx := context.Background()

	vectorData := &VectorData{
		ID:     "vec-1",
		Vector: []float64{0.1, 0.2},
		Metadata: map[string]interface{}{
			"content": "Updated content",
		},
	}

	err := provider.Update(ctx, "vec-1", vectorData)

	require.NoError(t, err)
	assert.True(t, updateReceived)
}

// ========================================
// Error Cases Tests
// ========================================

func TestMem0Provider_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "Internal server error"}`))
	}))
	defer server.Close()

	provider := newTestMem0Provider(server.URL)
	ctx := context.Background()

	_, err := provider.GetStats(ctx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "API error 500")
}

func TestMem0Provider_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{invalid json}`))
	}))
	defer server.Close()

	provider := newTestMem0Provider(server.URL)
	ctx := context.Background()

	_, err := provider.GetStats(ctx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse response")
}

// ========================================
// Authorization Header Tests
// ========================================

func TestMem0Provider_AuthorizationHeader(t *testing.T) {
	authHeaderReceived := ""

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeaderReceived = r.Header.Get("Authorization")
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
	}))
	defer server.Close()

	provider := newTestMem0Provider(server.URL)
	ctx := context.Background()

	provider.Health(ctx)

	assert.Equal(t, "Bearer test-api-key", authHeaderReceived)
}

func TestMem0Provider_NoAuthorizationWhenNoAPIKey(t *testing.T) {
	authHeaderReceived := ""

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeaderReceived = r.Header.Get("Authorization")
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
	}))
	defer server.Close()

	config := map[string]interface{}{
		"base_url": server.URL,
	}
	provider, _ := NewMem0Provider(config)
	ctx := context.Background()

	provider.Health(ctx)

	assert.Empty(t, authHeaderReceived)
}

// ========================================
// Helper Function Tests
// ========================================

func TestGetStringValue(t *testing.T) {
	data := map[string]interface{}{
		"string_key": "value",
		"int_key":    123,
		"nil_key":    nil,
	}

	assert.Equal(t, "value", getStringValue(data, "string_key"))
	assert.Equal(t, "", getStringValue(data, "int_key"))
	assert.Equal(t, "", getStringValue(data, "nil_key"))
	assert.Equal(t, "", getStringValue(data, "missing_key"))
}

func TestGetFloatValue(t *testing.T) {
	data := map[string]interface{}{
		"float_key": 1.5,
		"int_key":   100,
		"string_key": "not a number",
		"nil_key":   nil,
	}

	assert.Equal(t, 1.5, getFloatValue(data, "float_key"))
	assert.Equal(t, float64(100), getFloatValue(data, "int_key"))
	assert.Equal(t, 0.0, getFloatValue(data, "string_key"))
	assert.Equal(t, 0.0, getFloatValue(data, "nil_key"))
	assert.Equal(t, 0.0, getFloatValue(data, "missing_key"))
}

// ========================================
// VectorProvider Interface Compliance Test
// ========================================

func TestMem0Provider_ImplementsVectorProvider(t *testing.T) {
	var _ VectorProvider = (*Mem0Provider)(nil)
}
