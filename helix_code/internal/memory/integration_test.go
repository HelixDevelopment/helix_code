//go:build integration
// +build integration

package memory

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"
)

// Integration tests for memory providers using real services.
// Run with: go test -tags=integration -v ./internal/memory/...
//
// Prerequisites:
//   docker compose -f docker-compose.test.yml up -d redis-test memcached-test
//
// Environment variables:
//   REDIS_HOST=localhost REDIS_PORT=6380 REDIS_PASSWORD=test_redis_password_123
//   MEMCACHED_HOST=localhost MEMCACHED_PORT=11212

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// TestRedisIntegration tests Redis provider with real Redis server.
//
// Round-37 §11.4 anti-bluff sweep (2026-05-18): with the data path now
// wired to a real go-redis/v9 client, the parent test gates ALL
// subtests on Health(). Without this guard the previously sibling-only
// Health skip would let dependent subtests run against an unreachable
// backend and produce noisy failures unrelated to the unit under test.
// Per CONST-035 honest contract, unreachable infrastructure is a SKIP,
// not a FAIL.
func TestRedisIntegration(t *testing.T) {
	host := getEnvOrDefault("REDIS_HOST", "localhost")
	port := getEnvOrDefault("REDIS_PORT", "6380")
	password := getEnvOrDefault("REDIS_PASSWORD", "test_redis_password_123")

	config := map[string]interface{}{
		"host":     host,
		"port":     6380,
		"password": password,
		"prefix":   fmt.Sprintf("test:%d:", time.Now().UnixNano()),
	}

	provider, err := NewRedisMemoryProvider(config)
	if err != nil {
		t.Fatalf("Failed to create Redis provider: %v", err)
	}
	defer provider.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if hErr := provider.Health(ctx); hErr != nil {
		t.Skipf("Redis not available at %s:%s - skipping integration tests: %v (SKIP-OK: #infra-redis-unavailable)", host, port, hErr)
	}

	// Test Health
	t.Run("Health", func(t *testing.T) {
		if err := provider.Health(ctx); err != nil {
			t.Fatalf("Health failed: %v", err)
		}
	})

	// Test Store and Retrieve
	t.Run("StoreRetrieve", func(t *testing.T) {
		testData := map[string]interface{}{
			"name":      "integration-test",
			"timestamp": time.Now().Unix(),
			"nested": map[string]interface{}{
				"level": 1,
				"data":  "nested-value",
			},
		}

		err := provider.Store(ctx, "test-key-1", testData)
		if err != nil {
			t.Fatalf("Store failed: %v", err)
		}

		retrieved, err := provider.Retrieve(ctx, "test-key-1")
		if err != nil {
			t.Fatalf("Retrieve failed: %v", err)
		}

		retrievedMap, ok := retrieved.(map[string]interface{})
		if !ok {
			t.Fatalf("Retrieved data is not a map: %T", retrieved)
		}

		if retrievedMap["name"] != "integration-test" {
			t.Errorf("Expected name 'integration-test', got %v", retrievedMap["name"])
		}
	})

	// Test Search
	t.Run("Search", func(t *testing.T) {
		// Store multiple items
		provider.Store(ctx, "search-1", "findme")
		provider.Store(ctx, "search-2", "findme")
		provider.Store(ctx, "search-3", "notthis")

		results, err := provider.Search(ctx, "findme", 10)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		if len(results) != 2 {
			t.Errorf("Expected 2 search results, got %d", len(results))
		}
	})

	// Test Delete
	t.Run("Delete", func(t *testing.T) {
		provider.Store(ctx, "delete-key", "delete-value")

		err := provider.Delete(ctx, "delete-key")
		if err != nil {
			t.Fatalf("Delete failed: %v", err)
		}

		_, err = provider.Retrieve(ctx, "delete-key")
		if err == nil {
			t.Error("Expected error retrieving deleted key")
		}
	})

	// Test Clear
	t.Run("Clear", func(t *testing.T) {
		provider.Store(ctx, "clear-1", "value1")
		provider.Store(ctx, "clear-2", "value2")

		err := provider.Clear(ctx)
		if err != nil {
			t.Fatalf("Clear failed: %v", err)
		}

		_, err1 := provider.Retrieve(ctx, "clear-1")
		_, err2 := provider.Retrieve(ctx, "clear-2")
		if err1 == nil || err2 == nil {
			t.Error("Expected errors after clear")
		}
	})

	// Test concurrent operations
	t.Run("Concurrent", func(t *testing.T) {
		done := make(chan bool, 10)
		for i := 0; i < 10; i++ {
			go func(id int) {
				key := fmt.Sprintf("concurrent-%d", id)
				value := fmt.Sprintf("value-%d", id)

				if err := provider.Store(ctx, key, value); err != nil {
					t.Errorf("Concurrent store failed: %v", err)
					done <- false
					return
				}

				retrieved, err := provider.Retrieve(ctx, key)
				if err != nil {
					t.Errorf("Concurrent retrieve failed: %v", err)
					done <- false
					return
				}

				if retrieved != value {
					t.Errorf("Concurrent data mismatch: expected %s, got %v", value, retrieved)
					done <- false
					return
				}

				done <- true
			}(i)
		}

		for i := 0; i < 10; i++ {
			<-done
		}
	})
}

// TestMemcachedIntegration tests Memcached provider with real Memcached server.
//
// Round-37 §11.4 anti-bluff sweep (2026-05-18): with the data path now
// wired to a real gomemcache client, the parent test gates ALL subtests
// on Health(). The Search subtest is also updated to assert the honest
// contract — Memcached's wire protocol has no SCAN equivalent, so
// Search now ALWAYS returns ErrListNotSupportedByBackend (round 36 and
// prior shipped a fake walk over a local in-memory map and pretended
// it worked — CRITICAL fabricated-capability bluff per CONST-035).
func TestMemcachedIntegration(t *testing.T) {
	host := getEnvOrDefault("MEMCACHED_HOST", "localhost")
	port := getEnvOrDefault("MEMCACHED_PORT", "11212")

	config := map[string]interface{}{
		"host":   host,
		"port":   11212,
		"prefix": fmt.Sprintf("test:%d:", time.Now().UnixNano()),
	}

	provider, err := NewMemcachedMemoryProvider(config)
	if err != nil {
		t.Fatalf("Failed to create Memcached provider: %v", err)
	}
	defer provider.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if hErr := provider.Health(ctx); hErr != nil {
		t.Skipf("Memcached not available at %s:%s - skipping integration tests: %v (SKIP-OK: #infra-memcached-unavailable)", host, port, hErr)
	}

	// Test Health
	t.Run("Health", func(t *testing.T) {
		if err := provider.Health(ctx); err != nil {
			t.Fatalf("Health failed: %v", err)
		}
	})

	// Test Store and Retrieve
	t.Run("StoreRetrieve", func(t *testing.T) {
		testData := map[string]interface{}{
			"name":      "memcached-integration",
			"timestamp": time.Now().Unix(),
		}

		err := provider.Store(ctx, "mc-test-1", testData)
		if err != nil {
			t.Fatalf("Store failed: %v", err)
		}

		retrieved, err := provider.Retrieve(ctx, "mc-test-1")
		if err != nil {
			t.Fatalf("Retrieve failed: %v", err)
		}

		retrievedMap, ok := retrieved.(map[string]interface{})
		if !ok {
			t.Fatalf("Retrieved data is not a map: %T", retrieved)
		}

		if retrievedMap["name"] != "memcached-integration" {
			t.Errorf("Expected name 'memcached-integration', got %v", retrievedMap["name"])
		}
	})

	// Test Search: round-37 honest contract — Memcached has no SCAN
	// equivalent, so Search MUST return ErrListNotSupportedByBackend.
	// The previous "store 3, search 2" assertion certified a
	// fabricated capability.
	t.Run("Search", func(t *testing.T) {
		_, err := provider.Search(ctx, "anything", 10)
		if err == nil {
			t.Fatal("Search returned nil error — anti-bluff regression")
		}
		if !errors.Is(err, ErrListNotSupportedByBackend) {
			t.Errorf("Search err = %v, want errors.Is(err, ErrListNotSupportedByBackend)", err)
		}
	})

	// Test Delete
	t.Run("Delete", func(t *testing.T) {
		provider.Store(ctx, "mc-delete", "value")

		err := provider.Delete(ctx, "mc-delete")
		if err != nil {
			t.Fatalf("Delete failed: %v", err)
		}

		_, err = provider.Retrieve(ctx, "mc-delete")
		if err == nil {
			t.Error("Expected error for deleted key")
		}
	})

	// Test Clear
	t.Run("Clear", func(t *testing.T) {
		provider.Store(ctx, "mc-clear-1", "v1")
		provider.Store(ctx, "mc-clear-2", "v2")

		err := provider.Clear(ctx)
		if err != nil {
			t.Fatalf("Clear failed: %v", err)
		}

		_, err1 := provider.Retrieve(ctx, "mc-clear-1")
		_, err2 := provider.Retrieve(ctx, "mc-clear-2")
		if err1 == nil || err2 == nil {
			t.Error("Expected errors after clear")
		}
	})
}

// TestFilesystemIntegration tests Filesystem provider with real filesystem
func TestFilesystemIntegration(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "helixcode-fs-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	config := map[string]interface{}{
		"path": tempDir,
	}

	provider, err := NewFilesystemMemoryProvider(config)
	if err != nil {
		t.Fatalf("Failed to create Filesystem provider: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Test Health
	t.Run("Health", func(t *testing.T) {
		err := provider.Health(ctx)
		if err != nil {
			t.Fatalf("Health check failed: %v", err)
		}
	})

	// Test Store and Retrieve
	t.Run("StoreRetrieve", func(t *testing.T) {
		testData := map[string]interface{}{
			"name":      "filesystem-integration",
			"timestamp": time.Now().Unix(),
			"nested": map[string]interface{}{
				"deep": "value",
			},
		}

		err := provider.Store(ctx, "fs-test-1", testData)
		if err != nil {
			t.Fatalf("Store failed: %v", err)
		}

		retrieved, err := provider.Retrieve(ctx, "fs-test-1")
		if err != nil {
			t.Fatalf("Retrieve failed: %v", err)
		}

		retrievedMap, ok := retrieved.(map[string]interface{})
		if !ok {
			t.Fatalf("Retrieved data is not a map: %T", retrieved)
		}

		if retrievedMap["name"] != "filesystem-integration" {
			t.Errorf("Expected name 'filesystem-integration', got %v", retrievedMap["name"])
		}
	})

	// Test large data
	t.Run("LargeData", func(t *testing.T) {
		// Create a large piece of data
		largeData := make([]interface{}, 1000)
		for i := 0; i < 1000; i++ {
			largeData[i] = map[string]interface{}{
				"index": i,
				"data":  fmt.Sprintf("item-%d-with-some-extra-content-to-increase-size", i),
			}
		}

		err := provider.Store(ctx, "large-data", largeData)
		if err != nil {
			t.Fatalf("Store large data failed: %v", err)
		}

		retrieved, err := provider.Retrieve(ctx, "large-data")
		if err != nil {
			t.Fatalf("Retrieve large data failed: %v", err)
		}

		retrievedSlice, ok := retrieved.([]interface{})
		if !ok {
			t.Fatalf("Retrieved data is not a slice: %T", retrieved)
		}

		if len(retrievedSlice) != 1000 {
			t.Errorf("Expected 1000 items, got %d", len(retrievedSlice))
		}
	})

	// Test special characters in keys
	t.Run("SpecialCharacters", func(t *testing.T) {
		specialKeys := []string{
			"key/with/slashes",
			"key..with..dots",
			"key with spaces",
			"key:with:colons",
			"key@with@special!chars#",
		}

		for _, key := range specialKeys {
			err := provider.Store(ctx, key, "special-value")
			if err != nil {
				t.Errorf("Failed to store key '%s': %v", key, err)
				continue
			}

			retrieved, err := provider.Retrieve(ctx, key)
			if err != nil {
				t.Errorf("Failed to retrieve key '%s': %v", key, err)
				continue
			}

			if retrieved != "special-value" {
				t.Errorf("Value mismatch for key '%s': expected 'special-value', got %v", key, retrieved)
			}
		}
	})

	// Test Search
	t.Run("Search", func(t *testing.T) {
		provider.Store(ctx, "fs-search-1", "searchable")
		provider.Store(ctx, "fs-search-2", "searchable")
		provider.Store(ctx, "fs-search-3", "different")

		results, err := provider.Search(ctx, "searchable", 10)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		if len(results) != 2 {
			t.Errorf("Expected 2 results, got %d", len(results))
		}
	})

	// Test Delete
	t.Run("Delete", func(t *testing.T) {
		provider.Store(ctx, "fs-delete", "value")

		err := provider.Delete(ctx, "fs-delete")
		if err != nil {
			t.Fatalf("Delete failed: %v", err)
		}

		_, err = provider.Retrieve(ctx, "fs-delete")
		if err == nil {
			t.Error("Expected error for deleted key")
		}

		// Verify file is actually removed
		files, _ := os.ReadDir(tempDir)
		for _, f := range files {
			if f.Name() == "fs-delete.json" {
				t.Error("File should have been deleted")
			}
		}
	})

	// Test Clear
	t.Run("Clear", func(t *testing.T) {
		provider.Store(ctx, "fs-clear-1", "v1")
		provider.Store(ctx, "fs-clear-2", "v2")

		err := provider.Clear(ctx)
		if err != nil {
			t.Fatalf("Clear failed: %v", err)
		}

		_, err1 := provider.Retrieve(ctx, "fs-clear-1")
		_, err2 := provider.Retrieve(ctx, "fs-clear-2")
		if err1 == nil || err2 == nil {
			t.Error("Expected errors after clear")
		}
	})

	// Test concurrent file operations
	t.Run("Concurrent", func(t *testing.T) {
		done := make(chan bool, 10)
		for i := 0; i < 10; i++ {
			go func(id int) {
				key := fmt.Sprintf("fs-concurrent-%d", id)
				value := map[string]interface{}{
					"id":   id,
					"data": fmt.Sprintf("concurrent-data-%d", id),
				}

				if err := provider.Store(ctx, key, value); err != nil {
					t.Errorf("Concurrent store failed: %v", err)
					done <- false
					return
				}

				retrieved, err := provider.Retrieve(ctx, key)
				if err != nil {
					t.Errorf("Concurrent retrieve failed: %v", err)
					done <- false
					return
				}

				retrievedMap, ok := retrieved.(map[string]interface{})
				if !ok {
					t.Errorf("Concurrent data wrong type: %T", retrieved)
					done <- false
					return
				}

				// JSON unmarshaling converts int to float64
				if int(retrievedMap["id"].(float64)) != id {
					t.Errorf("Concurrent data mismatch")
					done <- false
					return
				}

				done <- true
			}(i)
		}

		for i := 0; i < 10; i++ {
			<-done
		}
	})
}

// TestDatabaseIntegration tests PostgreSQL integration
func TestDatabaseIntegration(t *testing.T) {
	host := getEnvOrDefault("POSTGRES_HOST", "localhost")
	port := getEnvOrDefault("POSTGRES_PORT", "5433")
	user := getEnvOrDefault("POSTGRES_USER", "helix_test")
	password := getEnvOrDefault("POSTGRES_PASSWORD", "test_password_secure_123")
	dbname := getEnvOrDefault("POSTGRES_DB", "helix_test")

	// Skip if no database available
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)

	t.Logf("Testing database connection: host=%s port=%s db=%s", host, port, dbname)

	// This test validates that the database connection parameters are correct
	// The actual database tests are in internal/database package
	t.Run("ConnectionParams", func(t *testing.T) {
		if host == "" || port == "" || user == "" || password == "" {
			t.Skip("Database environment variables not set")  // SKIP-OK: #legacy-untriaged
		}
		t.Logf("Database connection string format validated: %s", connStr[:50]+"...")
	})
}

// =============================================================================
// Round-37 §11.4 anti-bluff — real-backend round-trip integration tests
//
// These tests exercise the real go-redis/v9 and gomemcache clients wired
// in by round 37 against real containers (helix_code/docker-compose.full-test.yml
// or helix_code/docker-compose.test.yml). Per CONST-050(A)+(B) the
// integration layer MUST hit real infrastructure — no mocks.
//
// Run via:
//   make test-infra-up
//   go test -tags=integration -count=1 -race ./internal/memory/...
//   make test-infra-down
// =============================================================================

func TestRedisMemoryProvider_Set_Get_Roundtrip_Round37(t *testing.T) {
	host := getEnvOrDefault("REDIS_HOST", "localhost")
	port := getEnvOrDefault("REDIS_PORT", "6380")
	password := getEnvOrDefault("REDIS_PASSWORD", "")

	config := map[string]interface{}{
		"host":     host,
		"port":     6380,
		"password": password,
		"prefix":   fmt.Sprintf("round37:%d:", time.Now().UnixNano()),
	}
	if p := os.Getenv("REDIS_PORT"); p == "" {
		// Default mapping when running via docker-compose.full-test.yml (Redis on 6379).
		config["port"] = 6379
	}
	_ = port // referenced by Skipf below

	provider, err := NewRedisMemoryProvider(config)
	if err != nil {
		t.Fatalf("Failed to construct Redis provider: %v", err)
	}
	defer provider.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := provider.Health(ctx); err != nil {
		t.Skipf("Redis not reachable at %s:%v: %v (SKIP-OK: #infra-redis-unavailable)", host, config["port"], err)
	}

	want := map[string]interface{}{
		"phase":  "round-37",
		"answer": 42.0,
	}
	if err := provider.Store(ctx, "round37-roundtrip", want); err != nil {
		t.Fatalf("Store returned error: %v", err)
	}

	got, err := provider.Retrieve(ctx, "round37-roundtrip")
	if err != nil {
		t.Fatalf("Retrieve returned error: %v", err)
	}
	gotMap, ok := got.(map[string]interface{})
	if !ok {
		t.Fatalf("Retrieved value is not map[string]interface{}: %T", got)
	}
	if gotMap["phase"] != want["phase"] {
		t.Errorf("phase: got %v, want %v", gotMap["phase"], want["phase"])
	}
	if gotMap["answer"] != want["answer"] {
		t.Errorf("answer: got %v, want %v", gotMap["answer"], want["answer"])
	}
}

func TestRedisMemoryProvider_Get_NotFound_Round37(t *testing.T) {
	host := getEnvOrDefault("REDIS_HOST", "localhost")
	port := 6379
	if p := os.Getenv("REDIS_PORT"); p != "" {
		fmt.Sscanf(p, "%d", &port)
	}

	provider, err := NewRedisMemoryProvider(map[string]interface{}{
		"host":   host,
		"port":   port,
		"prefix": fmt.Sprintf("round37-nf:%d:", time.Now().UnixNano()),
	})
	if err != nil {
		t.Fatalf("Failed to construct Redis provider: %v", err)
	}
	defer provider.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := provider.Health(ctx); err != nil {
		t.Skipf("Redis not reachable: %v (SKIP-OK: #infra-redis-unavailable)", err)
	}

	_, err = provider.Retrieve(ctx, "this-key-was-never-set")
	if err == nil {
		t.Fatal("Retrieve of unknown key returned nil error")
	}
	if !errors.Is(err, ErrKeyNotFound) {
		t.Errorf("Retrieve unknown key: err = %v, want errors.Is(err, ErrKeyNotFound)", err)
	}
}

func TestRedisMemoryProvider_Set_TTL_Expires_Round37(t *testing.T) {
	host := getEnvOrDefault("REDIS_HOST", "localhost")
	port := 6379
	if p := os.Getenv("REDIS_PORT"); p != "" {
		fmt.Sscanf(p, "%d", &port)
	}

	provider, err := NewRedisMemoryProvider(map[string]interface{}{
		"host":   host,
		"port":   port,
		"prefix": fmt.Sprintf("round37-ttl:%d:", time.Now().UnixNano()),
	})
	if err != nil {
		t.Fatalf("Failed to construct Redis provider: %v", err)
	}
	defer provider.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := provider.Health(ctx); err != nil {
		t.Skipf("Redis not reachable: %v (SKIP-OK: #infra-redis-unavailable)", err)
	}

	ttlCtx := WithMemoryTTL(ctx, 1*time.Second)
	if err := provider.Store(ttlCtx, "ephemeral", "value-that-should-expire"); err != nil {
		t.Fatalf("Store with TTL failed: %v", err)
	}

	// Confirm present immediately
	if _, err := provider.Retrieve(ctx, "ephemeral"); err != nil {
		t.Fatalf("Retrieve immediately after Set: %v", err)
	}

	// Wait past TTL + grace
	time.Sleep(1500 * time.Millisecond)

	_, err = provider.Retrieve(ctx, "ephemeral")
	if err == nil {
		t.Fatal("Retrieve after TTL expiry returned nil error — TTL not honoured")
	}
	if !errors.Is(err, ErrKeyNotFound) {
		t.Errorf("Retrieve after TTL: err = %v, want errors.Is(err, ErrKeyNotFound)", err)
	}
}

func TestRedisMemoryProvider_NilClient_ReturnsSentinel_Round37(t *testing.T) {
	// Even in integration mode, the nil-client invariant must hold —
	// constructing with no host/url MUST surface
	// ErrRedisClientNotInitialized on every data method.
	provider, err := NewRedisMemoryProvider(map[string]interface{}{})
	if err != nil {
		t.Fatalf("Failed to construct nil-client Redis provider: %v", err)
	}
	ctx := context.Background()

	for name, op := range map[string]func() error{
		"Store":    func() error { return provider.Store(ctx, "k", "v") },
		"Retrieve": func() error { _, e := provider.Retrieve(ctx, "k"); return e },
		"Delete":   func() error { return provider.Delete(ctx, "k") },
		"Clear":    func() error { return provider.Clear(ctx) },
		"Search":   func() error { _, e := provider.Search(ctx, "q", 10); return e },
		"Health":   func() error { return provider.Health(ctx) },
	} {
		err := op()
		if !errors.Is(err, ErrRedisClientNotInitialized) {
			t.Errorf("%s on nil-client: err = %v, want errors.Is(err, ErrRedisClientNotInitialized)", name, err)
		}
	}
}

func TestMemcachedMemoryProvider_Set_Get_Roundtrip_Round37(t *testing.T) {
	host := getEnvOrDefault("MEMCACHED_HOST", "localhost")
	port := 11211
	if p := os.Getenv("MEMCACHED_PORT"); p != "" {
		fmt.Sscanf(p, "%d", &port)
	}

	provider, err := NewMemcachedMemoryProvider(map[string]interface{}{
		"host":   host,
		"port":   port,
		"prefix": fmt.Sprintf("round37mc:%d:", time.Now().UnixNano()),
	})
	if err != nil {
		t.Fatalf("Failed to construct Memcached provider: %v", err)
	}
	defer provider.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := provider.Health(ctx); err != nil {
		t.Skipf("Memcached not reachable at %s:%d: %v (SKIP-OK: #infra-memcached-unavailable)", host, port, err)
	}

	want := map[string]interface{}{
		"phase": "round-37",
		"item":  "memcached-roundtrip",
	}
	if err := provider.Store(ctx, "mc-roundtrip", want); err != nil {
		t.Fatalf("Store returned error: %v", err)
	}

	got, err := provider.Retrieve(ctx, "mc-roundtrip")
	if err != nil {
		t.Fatalf("Retrieve returned error: %v", err)
	}
	gotMap, ok := got.(map[string]interface{})
	if !ok {
		t.Fatalf("Retrieved value is not map[string]interface{}: %T", got)
	}
	if gotMap["phase"] != want["phase"] {
		t.Errorf("phase: got %v, want %v", gotMap["phase"], want["phase"])
	}
}

func TestMemcachedMemoryProvider_Get_NotFound_Round37(t *testing.T) {
	host := getEnvOrDefault("MEMCACHED_HOST", "localhost")
	port := 11211
	if p := os.Getenv("MEMCACHED_PORT"); p != "" {
		fmt.Sscanf(p, "%d", &port)
	}

	provider, err := NewMemcachedMemoryProvider(map[string]interface{}{
		"host":   host,
		"port":   port,
		"prefix": fmt.Sprintf("round37mc-nf:%d:", time.Now().UnixNano()),
	})
	if err != nil {
		t.Fatalf("Failed to construct Memcached provider: %v", err)
	}
	defer provider.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := provider.Health(ctx); err != nil {
		t.Skipf("Memcached not reachable: %v (SKIP-OK: #infra-memcached-unavailable)", err)
	}

	_, err = provider.Retrieve(ctx, "this-key-was-never-set")
	if err == nil {
		t.Fatal("Retrieve of unknown key returned nil error")
	}
	if !errors.Is(err, ErrKeyNotFound) {
		t.Errorf("Retrieve unknown key: err = %v, want errors.Is(err, ErrKeyNotFound)", err)
	}
}

func TestMemcachedMemoryProvider_List_NotSupported_Round37(t *testing.T) {
	// Real client; Memcached Search MUST always surface
	// ErrListNotSupportedByBackend regardless of backend reachability,
	// because the wire protocol has no SCAN equivalent. This is the
	// honest-contract sentinel introduced by round 37.
	provider, err := NewMemcachedMemoryProvider(map[string]interface{}{
		"host": "localhost",
		"port": 11211,
	})
	if err != nil {
		t.Fatalf("Failed to construct Memcached provider: %v", err)
	}
	defer provider.Close()

	_, err = provider.Search(context.Background(), "anything", 10)
	if err == nil {
		t.Fatal("Search returned nil error — anti-bluff regression")
	}
	if !errors.Is(err, ErrListNotSupportedByBackend) {
		t.Errorf("Search err = %v, want errors.Is(err, ErrListNotSupportedByBackend)", err)
	}
}

func TestMemcachedMemoryProvider_NilClient_ReturnsSentinel_Round37(t *testing.T) {
	provider, err := NewMemcachedMemoryProvider(map[string]interface{}{})
	if err != nil {
		t.Fatalf("Failed to construct nil-client Memcached provider: %v", err)
	}
	ctx := context.Background()

	for name, op := range map[string]func() error{
		"Store":    func() error { return provider.Store(ctx, "k", "v") },
		"Retrieve": func() error { _, e := provider.Retrieve(ctx, "k"); return e },
		"Delete":   func() error { return provider.Delete(ctx, "k") },
		"Clear":    func() error { return provider.Clear(ctx) },
		"Search":   func() error { _, e := provider.Search(ctx, "q", 10); return e },
		"Health":   func() error { return provider.Health(ctx) },
	} {
		err := op()
		if !errors.Is(err, ErrMemcachedClientNotInitialized) {
			t.Errorf("%s on nil-client: err = %v, want errors.Is(err, ErrMemcachedClientNotInitialized)", name, err)
		}
	}
}

// BenchmarkRedisIntegration benchmarks Redis operations with real server
func BenchmarkRedisIntegration(b *testing.B) {
	host := getEnvOrDefault("REDIS_HOST", "localhost")

	config := map[string]interface{}{
		"host":     host,
		"port":     6380,
		"password": getEnvOrDefault("REDIS_PASSWORD", "test_redis_password_123"),
		"prefix":   fmt.Sprintf("bench:%d:", time.Now().UnixNano()),
	}

	provider, err := NewRedisMemoryProvider(config)
	if err != nil {
		b.Fatalf("Failed to create provider: %v", err)
	}

	ctx := context.Background()

	// Check if Redis is available
	if err := provider.Health(ctx); err != nil {
		b.Skipf("Redis not available: %v", err)
	}

	b.Run("Store", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("bench-key-%d", i)
			provider.Store(ctx, key, "benchmark-value")
		}
	})

	// Pre-populate for retrieve benchmark
	for i := 0; i < 1000; i++ {
		provider.Store(ctx, fmt.Sprintf("retrieve-key-%d", i), "value")
	}

	b.Run("Retrieve", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("retrieve-key-%d", i%1000)
			provider.Retrieve(ctx, key)
		}
	})
}
