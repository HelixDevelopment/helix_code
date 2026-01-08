package providers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========================================
// Test Setup and Helpers
// ========================================

func newTestWeaviateProvider(baseURL string) *WeaviateProvider {
	config := map[string]interface{}{
		"url":            baseURL,
		"api_key":        "test-api-key",
		"class":          "TestClass",
		"batch_size":     100,
		"backup_backend": "filesystem",
	}
	provider, _ := NewWeaviateProvider(config)
	return provider.(*WeaviateProvider)
}

func newTestWeaviateProviderWithBackend(baseURL, backend string) *WeaviateProvider {
	config := map[string]interface{}{
		"url":            baseURL,
		"api_key":        "test-api-key",
		"class":          "TestClass",
		"batch_size":     100,
		"backup_backend": backend,
	}
	provider, _ := NewWeaviateProvider(config)
	return provider.(*WeaviateProvider)
}

// ========================================
// NewWeaviateProvider Tests
// ========================================

func TestNewWeaviateProvider(t *testing.T) {
	tests := []struct {
		name     string
		config   map[string]interface{}
		expected struct {
			url           string
			apiKey        string
			class         string
			batchSize     int
			backupBackend string
		}
	}{
		{
			name: "with all config values",
			config: map[string]interface{}{
				"url":            "http://custom-weaviate:8080",
				"api_key":        "my-api-key",
				"class":          "CustomClass",
				"batch_size":     200,
				"backup_backend": "s3",
			},
			expected: struct {
				url           string
				apiKey        string
				class         string
				batchSize     int
				backupBackend string
			}{
				url:           "http://custom-weaviate:8080",
				apiKey:        "my-api-key",
				class:         "CustomClass",
				batchSize:     200,
				backupBackend: "s3",
			},
		},
		{
			name:   "with default values",
			config: map[string]interface{}{},
			expected: struct {
				url           string
				apiKey        string
				class         string
				batchSize     int
				backupBackend string
			}{
				url:           "http://localhost:8080",
				apiKey:        "",
				class:         "Vector",
				batchSize:     100,
				backupBackend: "filesystem",
			},
		},
		{
			name: "with partial config",
			config: map[string]interface{}{
				"url": "http://weaviate:9090",
			},
			expected: struct {
				url           string
				apiKey        string
				class         string
				batchSize     int
				backupBackend string
			}{
				url:           "http://weaviate:9090",
				apiKey:        "",
				class:         "Vector",
				batchSize:     100,
				backupBackend: "filesystem",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewWeaviateProvider(tt.config)
			require.NoError(t, err)
			require.NotNil(t, provider)

			wp := provider.(*WeaviateProvider)
			assert.Equal(t, tt.expected.url, wp.config.URL)
			assert.Equal(t, tt.expected.apiKey, wp.config.APIKey)
			assert.Equal(t, tt.expected.class, wp.config.Class)
			assert.Equal(t, tt.expected.batchSize, wp.config.BatchSize)
			assert.Equal(t, tt.expected.backupBackend, wp.config.BackupBackend)
		})
	}
}

// ========================================
// Provider Metadata Tests
// ========================================

func TestWeaviateProvider_GetType(t *testing.T) {
	provider := newTestWeaviateProvider("http://localhost:8080")
	assert.Equal(t, "weaviate", provider.GetType())
}

func TestWeaviateProvider_GetName(t *testing.T) {
	provider := newTestWeaviateProvider("http://localhost:8080")
	assert.Equal(t, "weaviate", provider.GetName())
}

func TestWeaviateProvider_GetCapabilities(t *testing.T) {
	provider := newTestWeaviateProvider("http://localhost:8080")
	caps := provider.GetCapabilities()

	assert.Contains(t, caps, "vector_storage")
	assert.Contains(t, caps, "similarity_search")
	assert.Contains(t, caps, "metadata_filtering")
}

func TestWeaviateProvider_GetConfiguration(t *testing.T) {
	config := map[string]interface{}{
		"url":     "http://localhost:8080",
		"api_key": "test-key",
		"class":   "TestClass",
	}
	provider, _ := NewWeaviateProvider(config)

	returnedConfig := provider.GetConfiguration()
	assert.NotNil(t, returnedConfig)
}

func TestWeaviateProvider_IsCloud(t *testing.T) {
	provider := newTestWeaviateProvider("http://localhost:8080")
	// Weaviate can be self-hosted or cloud, but IsCloud returns false by default
	assert.False(t, provider.IsCloud())
}

func TestWeaviateProvider_GetCostInfo(t *testing.T) {
	provider := newTestWeaviateProvider("http://localhost:8080")
	costInfo := provider.GetCostInfo()

	assert.NotNil(t, costInfo)
	assert.Equal(t, "USD", costInfo.Currency)
	assert.Equal(t, "monthly", costInfo.BillingPeriod)
}

// ========================================
// Backup Backend Validation Tests
// ========================================

func TestIsValidBackupBackend(t *testing.T) {
	tests := []struct {
		backend string
		valid   bool
	}{
		{"s3", true},
		{"gcs", true},
		{"azure", true},
		{"filesystem", true},
		{"invalid", false},
		{"", false},
		{"S3", false},           // Case sensitive
		{"file-system", false},  // Wrong format
	}

	for _, tt := range tests {
		t.Run(tt.backend, func(t *testing.T) {
			result := isValidBackupBackend(tt.backend)
			assert.Equal(t, tt.valid, result)
		})
	}
}

// ========================================
// Backup Tests
// ========================================

func TestWeaviateProvider_Backup_NotStarted(t *testing.T) {
	provider := newTestWeaviateProvider("http://localhost:8080")
	ctx := context.Background()

	err := provider.Backup(ctx, "test-backup")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "provider not started")
}

func TestWeaviateProvider_Backup_EmptyPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Meta endpoint for initialization
		if r.URL.Path == "/v1/meta" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{"version": "1.0.0"})
			return
		}
		// Ready endpoint for start
		if r.URL.Path == "/v1/.well-known/ready" {
			w.WriteHeader(http.StatusOK)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	provider := newTestWeaviateProvider(server.URL)
	ctx := context.Background()

	// Initialize and start
	require.NoError(t, provider.Initialize(ctx, nil))
	require.NoError(t, provider.Start(ctx))

	err := provider.Backup(ctx, "")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "backup path/id cannot be empty")
}

func TestWeaviateProvider_Backup_InvalidBackend(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/meta" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{"version": "1.0.0"})
			return
		}
		if r.URL.Path == "/v1/.well-known/ready" {
			w.WriteHeader(http.StatusOK)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	provider := newTestWeaviateProviderWithBackend(server.URL, "invalid-backend")
	ctx := context.Background()

	require.NoError(t, provider.Initialize(ctx, nil))
	require.NoError(t, provider.Start(ctx))

	err := provider.Backup(ctx, "test-backup")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid backup backend")
}

func TestWeaviateProvider_Backup_Success(t *testing.T) {
	var backupCallCount int32 = 0
	var statusCallCount int32 = 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Meta endpoint for initialization
		if r.URL.Path == "/v1/meta" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{"version": "1.0.0"})
			return
		}
		// Ready endpoint
		if r.URL.Path == "/v1/.well-known/ready" {
			w.WriteHeader(http.StatusOK)
			return
		}
		// Backup creation endpoint
		if r.Method == "POST" && r.URL.Path == "/v1/backups/filesystem" {
			atomic.AddInt32(&backupCallCount, 1)

			// Verify request body
			var reqBody WeaviateBackupRequest
			json.NewDecoder(r.Body).Decode(&reqBody)
			assert.Equal(t, "test-backup-123", reqBody.ID)
			assert.Contains(t, reqBody.Include, "TestClass")

			// Verify authorization header
			assert.Equal(t, "Bearer test-api-key", r.Header.Get("Authorization"))

			response := WeaviateBackupResponse{
				ID:      "test-backup-123",
				Backend: "filesystem",
				Status:  BackupStatusStarted,
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
			return
		}
		// Backup status endpoint
		if r.Method == "GET" && r.URL.Path == "/v1/backups/filesystem/test-backup-123" {
			count := atomic.AddInt32(&statusCallCount, 1)

			var response WeaviateBackupResponse
			if count >= 2 {
				// Return success after second poll
				response = WeaviateBackupResponse{
					ID:      "test-backup-123",
					Backend: "filesystem",
					Status:  BackupStatusSuccess,
				}
			} else {
				// Return started on first poll
				response = WeaviateBackupResponse{
					ID:      "test-backup-123",
					Backend: "filesystem",
					Status:  BackupStatusStarted,
				}
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	provider := newTestWeaviateProvider(server.URL)
	ctx := context.Background()

	require.NoError(t, provider.Initialize(ctx, nil))
	require.NoError(t, provider.Start(ctx))

	err := provider.Backup(ctx, "test-backup-123")

	require.NoError(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(&backupCallCount))
	assert.GreaterOrEqual(t, atomic.LoadInt32(&statusCallCount), int32(2))
}

func TestWeaviateProvider_Backup_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/meta" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{"version": "1.0.0"})
			return
		}
		if r.URL.Path == "/v1/.well-known/ready" {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method == "POST" && r.URL.Path == "/v1/backups/filesystem" {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error": "backup failed"}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	provider := newTestWeaviateProvider(server.URL)
	ctx := context.Background()

	require.NoError(t, provider.Initialize(ctx, nil))
	require.NoError(t, provider.Start(ctx))

	err := provider.Backup(ctx, "test-backup")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "backup request failed")
}

func TestWeaviateProvider_Backup_Failed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/meta" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{"version": "1.0.0"})
			return
		}
		if r.URL.Path == "/v1/.well-known/ready" {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method == "POST" && r.URL.Path == "/v1/backups/filesystem" {
			response := WeaviateBackupResponse{
				ID:      "test-backup",
				Backend: "filesystem",
				Status:  BackupStatusStarted,
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
			return
		}
		if r.Method == "GET" && r.URL.Path == "/v1/backups/filesystem/test-backup" {
			response := WeaviateBackupResponse{
				ID:      "test-backup",
				Backend: "filesystem",
				Status:  BackupStatusFailed,
				Error:   "disk space exhausted",
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	provider := newTestWeaviateProvider(server.URL)
	ctx := context.Background()

	require.NoError(t, provider.Initialize(ctx, nil))
	require.NoError(t, provider.Start(ctx))

	err := provider.Backup(ctx, "test-backup")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "backup failed")
	assert.Contains(t, err.Error(), "disk space exhausted")
}

func TestWeaviateProvider_Backup_Cancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/meta" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{"version": "1.0.0"})
			return
		}
		if r.URL.Path == "/v1/.well-known/ready" {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method == "POST" && r.URL.Path == "/v1/backups/filesystem" {
			response := WeaviateBackupResponse{
				ID:      "test-backup",
				Backend: "filesystem",
				Status:  BackupStatusStarted,
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
			return
		}
		if r.Method == "GET" && r.URL.Path == "/v1/backups/filesystem/test-backup" {
			response := WeaviateBackupResponse{
				ID:      "test-backup",
				Backend: "filesystem",
				Status:  BackupStatusCancelled,
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	provider := newTestWeaviateProvider(server.URL)
	ctx := context.Background()

	require.NoError(t, provider.Initialize(ctx, nil))
	require.NoError(t, provider.Start(ctx))

	err := provider.Backup(ctx, "test-backup")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "backup was cancelled")
}

func TestWeaviateProvider_Backup_ContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/meta" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{"version": "1.0.0"})
			return
		}
		if r.URL.Path == "/v1/.well-known/ready" {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method == "POST" && r.URL.Path == "/v1/backups/filesystem" {
			response := WeaviateBackupResponse{
				ID:      "test-backup",
				Backend: "filesystem",
				Status:  BackupStatusStarted,
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
			return
		}
		if r.Method == "GET" && r.URL.Path == "/v1/backups/filesystem/test-backup" {
			// Always return STARTED to let context cancellation happen
			time.Sleep(100 * time.Millisecond)
			response := WeaviateBackupResponse{
				ID:      "test-backup",
				Backend: "filesystem",
				Status:  BackupStatusStarted,
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	provider := newTestWeaviateProvider(server.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	require.NoError(t, provider.Initialize(ctx, nil))
	require.NoError(t, provider.Start(ctx))

	err := provider.Backup(ctx, "test-backup")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context deadline exceeded")
}

// ========================================
// Restore Tests
// ========================================

func TestWeaviateProvider_Restore_NotInitialized(t *testing.T) {
	provider := newTestWeaviateProvider("http://localhost:8080")
	ctx := context.Background()

	err := provider.Restore(ctx, "test-backup")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "provider not initialized")
}

func TestWeaviateProvider_Restore_EmptyPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/meta" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{"version": "1.0.0"})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	provider := newTestWeaviateProvider(server.URL)
	ctx := context.Background()

	require.NoError(t, provider.Initialize(ctx, nil))

	err := provider.Restore(ctx, "")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "restore backup id cannot be empty")
}

func TestWeaviateProvider_Restore_InvalidBackend(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/meta" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{"version": "1.0.0"})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	provider := newTestWeaviateProviderWithBackend(server.URL, "invalid-backend")
	ctx := context.Background()

	require.NoError(t, provider.Initialize(ctx, nil))

	err := provider.Restore(ctx, "test-backup")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid backup backend")
}

func TestWeaviateProvider_Restore_Success(t *testing.T) {
	var restoreCallCount int32 = 0
	var statusCallCount int32 = 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/meta" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{"version": "1.0.0"})
			return
		}
		// Restore endpoint
		if r.Method == "POST" && r.URL.Path == "/v1/backups/filesystem/test-restore-123/restore" {
			atomic.AddInt32(&restoreCallCount, 1)

			// Verify request body
			var reqBody WeaviateRestoreRequest
			json.NewDecoder(r.Body).Decode(&reqBody)
			assert.Contains(t, reqBody.Include, "TestClass")

			// Verify authorization header
			assert.Equal(t, "Bearer test-api-key", r.Header.Get("Authorization"))

			response := WeaviateBackupResponse{
				ID:      "test-restore-123",
				Backend: "filesystem",
				Status:  BackupStatusStarted,
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
			return
		}
		// Restore status endpoint
		if r.Method == "GET" && r.URL.Path == "/v1/backups/filesystem/test-restore-123/restore" {
			count := atomic.AddInt32(&statusCallCount, 1)

			var response WeaviateBackupResponse
			if count >= 2 {
				response = WeaviateBackupResponse{
					ID:      "test-restore-123",
					Backend: "filesystem",
					Status:  BackupStatusSuccess,
				}
			} else {
				response = WeaviateBackupResponse{
					ID:      "test-restore-123",
					Backend: "filesystem",
					Status:  BackupStatusStarted,
				}
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	provider := newTestWeaviateProvider(server.URL)
	ctx := context.Background()

	require.NoError(t, provider.Initialize(ctx, nil))

	err := provider.Restore(ctx, "test-restore-123")

	require.NoError(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(&restoreCallCount))
	assert.GreaterOrEqual(t, atomic.LoadInt32(&statusCallCount), int32(2))
}

func TestWeaviateProvider_Restore_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/meta" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{"version": "1.0.0"})
			return
		}
		if r.Method == "POST" && r.URL.Path == "/v1/backups/filesystem/test-backup/restore" {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error": "backup not found"}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	provider := newTestWeaviateProvider(server.URL)
	ctx := context.Background()

	require.NoError(t, provider.Initialize(ctx, nil))

	err := provider.Restore(ctx, "test-backup")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "restore request failed")
}

func TestWeaviateProvider_Restore_Failed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/meta" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{"version": "1.0.0"})
			return
		}
		if r.Method == "POST" && r.URL.Path == "/v1/backups/filesystem/test-backup/restore" {
			response := WeaviateBackupResponse{
				ID:      "test-backup",
				Backend: "filesystem",
				Status:  BackupStatusStarted,
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
			return
		}
		if r.Method == "GET" && r.URL.Path == "/v1/backups/filesystem/test-backup/restore" {
			response := WeaviateBackupResponse{
				ID:      "test-backup",
				Backend: "filesystem",
				Status:  BackupStatusFailed,
				Error:   "schema mismatch",
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	provider := newTestWeaviateProvider(server.URL)
	ctx := context.Background()

	require.NoError(t, provider.Initialize(ctx, nil))

	err := provider.Restore(ctx, "test-backup")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "restore failed")
	assert.Contains(t, err.Error(), "schema mismatch")
}

func TestWeaviateProvider_Restore_Cancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/meta" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{"version": "1.0.0"})
			return
		}
		if r.Method == "POST" && r.URL.Path == "/v1/backups/filesystem/test-backup/restore" {
			response := WeaviateBackupResponse{
				ID:      "test-backup",
				Backend: "filesystem",
				Status:  BackupStatusStarted,
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
			return
		}
		if r.Method == "GET" && r.URL.Path == "/v1/backups/filesystem/test-backup/restore" {
			response := WeaviateBackupResponse{
				ID:      "test-backup",
				Backend: "filesystem",
				Status:  BackupStatusCancelled,
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	provider := newTestWeaviateProvider(server.URL)
	ctx := context.Background()

	require.NoError(t, provider.Initialize(ctx, nil))

	err := provider.Restore(ctx, "test-backup")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "restore was cancelled")
}

// ========================================
// Backup Backend Configuration Tests
// ========================================

func TestWeaviateProvider_Backup_S3Backend(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/meta" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{"version": "1.0.0"})
			return
		}
		if r.URL.Path == "/v1/.well-known/ready" {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method == "POST" && r.URL.Path == "/v1/backups/s3" {
			response := WeaviateBackupResponse{
				ID:      "s3-backup",
				Backend: "s3",
				Status:  BackupStatusStarted,
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
			return
		}
		if r.Method == "GET" && r.URL.Path == "/v1/backups/s3/s3-backup" {
			response := WeaviateBackupResponse{
				ID:      "s3-backup",
				Backend: "s3",
				Status:  BackupStatusSuccess,
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	provider := newTestWeaviateProviderWithBackend(server.URL, "s3")
	ctx := context.Background()

	require.NoError(t, provider.Initialize(ctx, nil))
	require.NoError(t, provider.Start(ctx))

	err := provider.Backup(ctx, "s3-backup")
	require.NoError(t, err)
}

func TestWeaviateProvider_Backup_GCSBackend(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/meta" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{"version": "1.0.0"})
			return
		}
		if r.URL.Path == "/v1/.well-known/ready" {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method == "POST" && r.URL.Path == "/v1/backups/gcs" {
			response := WeaviateBackupResponse{
				ID:      "gcs-backup",
				Backend: "gcs",
				Status:  BackupStatusStarted,
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
			return
		}
		if r.Method == "GET" && r.URL.Path == "/v1/backups/gcs/gcs-backup" {
			response := WeaviateBackupResponse{
				ID:      "gcs-backup",
				Backend: "gcs",
				Status:  BackupStatusSuccess,
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	provider := newTestWeaviateProviderWithBackend(server.URL, "gcs")
	ctx := context.Background()

	require.NoError(t, provider.Initialize(ctx, nil))
	require.NoError(t, provider.Start(ctx))

	err := provider.Backup(ctx, "gcs-backup")
	require.NoError(t, err)
}

func TestWeaviateProvider_Backup_AzureBackend(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/meta" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{"version": "1.0.0"})
			return
		}
		if r.URL.Path == "/v1/.well-known/ready" {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method == "POST" && r.URL.Path == "/v1/backups/azure" {
			response := WeaviateBackupResponse{
				ID:      "azure-backup",
				Backend: "azure",
				Status:  BackupStatusStarted,
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
			return
		}
		if r.Method == "GET" && r.URL.Path == "/v1/backups/azure/azure-backup" {
			response := WeaviateBackupResponse{
				ID:      "azure-backup",
				Backend: "azure",
				Status:  BackupStatusSuccess,
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	provider := newTestWeaviateProviderWithBackend(server.URL, "azure")
	ctx := context.Background()

	require.NoError(t, provider.Initialize(ctx, nil))
	require.NoError(t, provider.Start(ctx))

	err := provider.Backup(ctx, "azure-backup")
	require.NoError(t, err)
}

// ========================================
// Backup Status Tests
// ========================================

func TestWeaviateProvider_GetBackupStatus_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/meta" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{"version": "1.0.0"})
			return
		}
		if r.Method == "GET" && r.URL.Path == "/v1/backups/filesystem/test-backup" {
			response := WeaviateBackupResponse{
				ID:        "test-backup",
				Backend:   "filesystem",
				Status:    BackupStatusSuccess,
				Path:      "/backups/test-backup",
				Classes:   []string{"TestClass"},
				StartTime: "2025-01-08T10:00:00Z",
				EndTime:   "2025-01-08T10:05:00Z",
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	provider := newTestWeaviateProvider(server.URL)
	ctx := context.Background()

	require.NoError(t, provider.Initialize(ctx, nil))

	status, err := provider.getBackupStatus(ctx, "filesystem", "test-backup")

	require.NoError(t, err)
	assert.Equal(t, "test-backup", status.ID)
	assert.Equal(t, "filesystem", status.Backend)
	assert.Equal(t, BackupStatusSuccess, status.Status)
	assert.Equal(t, "/backups/test-backup", status.Path)
	assert.Contains(t, status.Classes, "TestClass")
}

func TestWeaviateProvider_GetBackupStatus_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/meta" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{"version": "1.0.0"})
			return
		}
		if r.Method == "GET" && r.URL.Path == "/v1/backups/filesystem/nonexistent" {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error": "backup not found"}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	provider := newTestWeaviateProvider(server.URL)
	ctx := context.Background()

	require.NoError(t, provider.Initialize(ctx, nil))

	status, err := provider.getBackupStatus(ctx, "filesystem", "nonexistent")

	assert.Error(t, err)
	assert.Nil(t, status)
	assert.Contains(t, err.Error(), "failed to get backup status")
}

// ========================================
// Restore Status Tests
// ========================================

func TestWeaviateProvider_GetRestoreStatus_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/meta" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{"version": "1.0.0"})
			return
		}
		if r.Method == "GET" && r.URL.Path == "/v1/backups/filesystem/test-backup/restore" {
			response := WeaviateBackupResponse{
				ID:        "test-backup",
				Backend:   "filesystem",
				Status:    BackupStatusSuccess,
				Classes:   []string{"TestClass"},
				StartTime: "2025-01-08T11:00:00Z",
				EndTime:   "2025-01-08T11:10:00Z",
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	provider := newTestWeaviateProvider(server.URL)
	ctx := context.Background()

	require.NoError(t, provider.Initialize(ctx, nil))

	status, err := provider.getRestoreStatus(ctx, "filesystem", "test-backup")

	require.NoError(t, err)
	assert.Equal(t, "test-backup", status.ID)
	assert.Equal(t, "filesystem", status.Backend)
	assert.Equal(t, BackupStatusSuccess, status.Status)
}

func TestWeaviateProvider_GetRestoreStatus_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/meta" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{"version": "1.0.0"})
			return
		}
		if r.Method == "GET" && r.URL.Path == "/v1/backups/filesystem/nonexistent/restore" {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error": "restore not found"}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	provider := newTestWeaviateProvider(server.URL)
	ctx := context.Background()

	require.NoError(t, provider.Initialize(ctx, nil))

	status, err := provider.getRestoreStatus(ctx, "filesystem", "nonexistent")

	assert.Error(t, err)
	assert.Nil(t, status)
	assert.Contains(t, err.Error(), "failed to get restore status")
}

// ========================================
// Lifecycle Tests
// ========================================

func TestWeaviateProvider_Lifecycle(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/meta" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{"version": "1.0.0"})
			return
		}
		if r.URL.Path == "/v1/.well-known/ready" {
			w.WriteHeader(http.StatusOK)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	provider := newTestWeaviateProvider(server.URL)
	ctx := context.Background()

	// Initialize
	err := provider.Initialize(ctx, nil)
	require.NoError(t, err)
	assert.True(t, provider.initialized)

	// Start
	err = provider.Start(ctx)
	require.NoError(t, err)
	assert.True(t, provider.started)

	// Stop
	err = provider.Stop(ctx)
	require.NoError(t, err)
	assert.False(t, provider.started)

	// Close (resets initialized when called while started)
	// First restart so Close can do its full cleanup
	err = provider.Start(ctx)
	require.NoError(t, err)
	assert.True(t, provider.started)

	err = provider.Close(ctx)
	require.NoError(t, err)
	assert.False(t, provider.started)
	assert.False(t, provider.initialized)
}

// ========================================
// Authorization Header Tests
// ========================================

func TestWeaviateProvider_AuthorizationHeader(t *testing.T) {
	authHeaderReceived := ""

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/meta" {
			authHeaderReceived = r.Header.Get("Authorization")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{"version": "1.0.0"})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	provider := newTestWeaviateProvider(server.URL)
	ctx := context.Background()

	provider.Initialize(ctx, nil)

	assert.Equal(t, "Bearer test-api-key", authHeaderReceived)
}

func TestWeaviateProvider_NoAuthorizationWhenNoAPIKey(t *testing.T) {
	authHeaderReceived := ""

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/meta" {
			authHeaderReceived = r.Header.Get("Authorization")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{"version": "1.0.0"})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	config := map[string]interface{}{
		"url": server.URL,
	}
	provider, _ := NewWeaviateProvider(config)
	ctx := context.Background()

	provider.Initialize(ctx, nil)

	assert.Empty(t, authHeaderReceived)
}

// ========================================
// VectorProvider Interface Compliance Test
// ========================================

func TestWeaviateProvider_ImplementsVectorProvider(t *testing.T) {
	var _ VectorProvider = (*WeaviateProvider)(nil)
}

// ========================================
// Health Check Tests
// ========================================

func TestWeaviateProvider_Health_Healthy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/meta" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{"version": "1.0.0"})
			return
		}
		if r.URL.Path == "/v1/.well-known/ready" {
			w.WriteHeader(http.StatusOK)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	provider := newTestWeaviateProvider(server.URL)
	ctx := context.Background()

	require.NoError(t, provider.Initialize(ctx, nil))

	health, err := provider.Health(ctx)

	require.NoError(t, err)
	assert.Equal(t, "healthy", health.Status)
	assert.Equal(t, "Weaviate is operational", health.Message)
}

func TestWeaviateProvider_Health_Unhealthy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/meta" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{"version": "1.0.0"})
			return
		}
		if r.URL.Path == "/v1/.well-known/ready" {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"error": "not ready"}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	provider := newTestWeaviateProvider(server.URL)
	ctx := context.Background()

	require.NoError(t, provider.Initialize(ctx, nil))

	health, err := provider.Health(ctx)

	require.NoError(t, err)
	assert.Equal(t, "unhealthy", health.Status)
}

func TestWeaviateProvider_Health_NotInitialized(t *testing.T) {
	provider := newTestWeaviateProvider("http://localhost:8080")
	ctx := context.Background()

	health, err := provider.Health(ctx)

	require.NoError(t, err)
	assert.Equal(t, "not_initialized", health.Status)
	assert.Equal(t, "Provider not initialized", health.Message)
}

// ========================================
// Backup Response Types Tests
// ========================================

func TestWeaviateBackupResponse_JSON(t *testing.T) {
	response := WeaviateBackupResponse{
		ID:        "test-backup",
		Backend:   "s3",
		Path:      "s3://bucket/backup",
		Status:    BackupStatusSuccess,
		Classes:   []string{"Class1", "Class2"},
		StartTime: "2025-01-08T10:00:00Z",
		EndTime:   "2025-01-08T10:05:00Z",
	}

	jsonData, err := json.Marshal(response)
	require.NoError(t, err)

	var decoded WeaviateBackupResponse
	err = json.Unmarshal(jsonData, &decoded)
	require.NoError(t, err)

	assert.Equal(t, response.ID, decoded.ID)
	assert.Equal(t, response.Backend, decoded.Backend)
	assert.Equal(t, response.Path, decoded.Path)
	assert.Equal(t, response.Status, decoded.Status)
	assert.Equal(t, response.Classes, decoded.Classes)
}

func TestWeaviateBackupRequest_JSON(t *testing.T) {
	request := WeaviateBackupRequest{
		ID:               "my-backup",
		Include:          []string{"Class1"},
		Exclude:          nil,
		CPUPercentage:    50,
		ChunkSize:        128,
		CompressionLevel: "default",
	}

	jsonData, err := json.Marshal(request)
	require.NoError(t, err)

	var decoded WeaviateBackupRequest
	err = json.Unmarshal(jsonData, &decoded)
	require.NoError(t, err)

	assert.Equal(t, request.ID, decoded.ID)
	assert.Equal(t, request.Include, decoded.Include)
	assert.Equal(t, request.CPUPercentage, decoded.CPUPercentage)
}

func TestWeaviateRestoreRequest_JSON(t *testing.T) {
	request := WeaviateRestoreRequest{
		Include:       []string{"Class1", "Class2"},
		CPUPercentage: 75,
	}

	jsonData, err := json.Marshal(request)
	require.NoError(t, err)

	var decoded WeaviateRestoreRequest
	err = json.Unmarshal(jsonData, &decoded)
	require.NoError(t, err)

	assert.Equal(t, request.Include, decoded.Include)
	assert.Equal(t, request.CPUPercentage, decoded.CPUPercentage)
}

// ========================================
// Backup Constants Tests
// ========================================

func TestBackupStatusConstants(t *testing.T) {
	assert.Equal(t, "STARTED", BackupStatusStarted)
	assert.Equal(t, "SUCCESS", BackupStatusSuccess)
	assert.Equal(t, "FAILED", BackupStatusFailed)
	assert.Equal(t, "CANCELLED", BackupStatusCancelled)
}

func TestBackupBackendConstants(t *testing.T) {
	assert.Equal(t, BackupBackend("s3"), BackupBackendS3)
	assert.Equal(t, BackupBackend("gcs"), BackupBackendGCS)
	assert.Equal(t, BackupBackend("azure"), BackupBackendAzure)
	assert.Equal(t, BackupBackend("filesystem"), BackupBackendFilesystem)
}
