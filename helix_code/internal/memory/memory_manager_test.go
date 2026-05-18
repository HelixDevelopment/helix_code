package memory

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestNewMemoryManager(t *testing.T) {
	config := &MemoryConfig{}
	manager := NewMemoryManager(config)

	if manager == nil {
		t.Fatal("NewMemoryManager returned nil")
	}

	if len(manager.ListProviders()) != 0 {
		t.Error("New manager should have no providers")
	}

	if manager.defaultProvider != "" {
		t.Error("New manager should have no default provider")
	}
}

func TestMemoryManagerRegisterProvider(t *testing.T) {
	manager := NewMemoryManager(&MemoryConfig{})
	provider, _ := NewInMemoryProvider(map[string]interface{}{})

	err := manager.RegisterProvider("test-provider", provider)
	if err != nil {
		t.Fatalf("Failed to register provider: %v", err)
	}

	providers := manager.ListProviders()
	if len(providers) != 1 {
		t.Errorf("Expected 1 provider, got %d", len(providers))
	}

	if providers[0] != "test-provider" {
		t.Errorf("Expected provider name 'test-provider', got '%s'", providers[0])
	}

	// Should be set as default
	if manager.defaultProvider != "test-provider" {
		t.Errorf("Expected default provider 'test-provider', got '%s'", manager.defaultProvider)
	}
}

func TestMemoryManagerUnregisterProvider(t *testing.T) {
	manager := NewMemoryManager(&MemoryConfig{})
	provider, _ := NewInMemoryProvider(map[string]interface{}{})

	// Register
	err := manager.RegisterProvider("test-provider", provider)
	if err != nil {
		t.Fatalf("Failed to register provider: %v", err)
	}

	// Unregister
	err = manager.UnregisterProvider("test-provider")
	if err != nil {
		t.Fatalf("Failed to unregister provider: %v", err)
	}

	providers := manager.ListProviders()
	if len(providers) != 0 {
		t.Errorf("Expected 0 providers after unregister, got %d", len(providers))
	}

	// Default should be reset
	if manager.defaultProvider != "" {
		t.Errorf("Expected no default provider after unregister, got '%s'", manager.defaultProvider)
	}
}

func TestMemoryManagerSetDefaultProvider(t *testing.T) {
	manager := NewMemoryManager(&MemoryConfig{})

	// Register multiple providers
	provider1, _ := NewInMemoryProvider(map[string]interface{}{})
	provider2, _ := NewInMemoryProvider(map[string]interface{}{})

	manager.RegisterProvider("provider1", provider1)
	manager.RegisterProvider("provider2", provider2)

	// Set default
	err := manager.SetDefaultProvider("provider2")
	if err != nil {
		t.Fatalf("Failed to set default provider: %v", err)
	}

	if manager.defaultProvider != "provider2" {
		t.Errorf("Expected default provider 'provider2', got '%s'", manager.defaultProvider)
	}
}

func TestMemoryManagerGetProvider(t *testing.T) {
	manager := NewMemoryManager(&MemoryConfig{})
	provider, _ := NewInMemoryProvider(map[string]interface{}{})

	manager.RegisterProvider("test-provider", provider)

	// Get existing provider
	retrieved, err := manager.GetProvider("test-provider")
	if err != nil {
		t.Fatalf("Failed to get provider: %v", err)
	}

	if retrieved != provider {
		t.Error("Retrieved provider is not the same as registered")
	}

	// Get non-existent provider
	_, err = manager.GetProvider("non-existent")
	if err == nil {
		t.Error("Expected error for non-existent provider")
	}
}

func TestMemoryManagerGetDefaultProvider(t *testing.T) {
	manager := NewMemoryManager(&MemoryConfig{})

	// No providers
	_, err := manager.GetDefaultProvider()
	if err == nil {
		t.Error("Expected error when no default provider set")
	}

	// Add provider
	provider, _ := NewInMemoryProvider(map[string]interface{}{})
	manager.RegisterProvider("test-provider", provider)

	// Should work now
	defaultProvider, err := manager.GetDefaultProvider()
	if err != nil {
		t.Fatalf("Failed to get default provider: %v", err)
	}

	if defaultProvider != provider {
		t.Error("Default provider is not correct")
	}
}

func TestInMemoryProviderStoreRetrieve(t *testing.T) {
	provider, _ := NewInMemoryProvider(map[string]interface{}{})
	ctx := context.Background()

	testData := map[string]interface{}{
		"name":  "test",
		"value": 42,
	}

	// Store
	err := provider.Store(ctx, "test-key", testData)
	if err != nil {
		t.Fatalf("Failed to store data: %v", err)
	}

	// Retrieve
	retrieved, err := provider.Retrieve(ctx, "test-key")
	if err != nil {
		t.Fatalf("Failed to retrieve data: %v", err)
	}

	retrievedMap, ok := retrieved.(map[string]interface{})
	if !ok {
		t.Fatal("Retrieved data is not a map")
	}

	if retrievedMap["name"] != "test" {
		t.Errorf("Expected name 'test', got %v", retrievedMap["name"])
	}

	if retrievedMap["value"] != 42 {
		t.Errorf("Expected value 42, got %v", retrievedMap["value"])
	}
}

func TestInMemoryProviderDelete(t *testing.T) {
	provider, _ := NewInMemoryProvider(map[string]interface{}{})
	ctx := context.Background()

	// Store
	err := provider.Store(ctx, "test-key", "test-value")
	if err != nil {
		t.Fatalf("Failed to store data: %v", err)
	}

	// Verify exists
	_, err = provider.Retrieve(ctx, "test-key")
	if err != nil {
		t.Fatalf("Data should exist: %v", err)
	}

	// Delete
	err = provider.Delete(ctx, "test-key")
	if err != nil {
		t.Fatalf("Failed to delete data: %v", err)
	}

	// Verify gone
	_, err = provider.Retrieve(ctx, "test-key")
	if err == nil {
		t.Error("Data should be gone after delete")
	}
}

func TestInMemoryProviderClear(t *testing.T) {
	provider, _ := NewInMemoryProvider(map[string]interface{}{})
	ctx := context.Background()

	// Store multiple items
	provider.Store(ctx, "key1", "value1")
	provider.Store(ctx, "key2", "value2")

	// Clear
	err := provider.Clear(ctx)
	if err != nil {
		t.Fatalf("Failed to clear data: %v", err)
	}

	// Verify empty
	_, err1 := provider.Retrieve(ctx, "key1")
	_, err2 := provider.Retrieve(ctx, "key2")

	if err1 == nil || err2 == nil {
		t.Error("Data should be gone after clear")
	}
}

func TestInMemoryProviderSearch(t *testing.T) {
	provider, _ := NewInMemoryProvider(map[string]interface{}{})
	ctx := context.Background()

	// Store test data
	provider.Store(ctx, "user1", "John Doe")
	provider.Store(ctx, "user2", "Jane Smith")
	provider.Store(ctx, "item1", "John Doe") // Same value as user1

	// Search for "John Doe"
	results, err := provider.Search(ctx, "John Doe", 10)
	if err != nil {
		t.Fatalf("Failed to search: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}

	// Search for specific key
	results, err = provider.Search(ctx, "user1", 10)
	if err != nil {
		t.Fatalf("Failed to search: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 result for key search, got %d", len(results))
	}

	if results[0].Key != "user1" {
		t.Errorf("Expected key 'user1', got '%s'", results[0].Key)
	}
}

func TestInMemoryProviderHealth(t *testing.T) {
	provider, _ := NewInMemoryProvider(map[string]interface{}{})
	ctx := context.Background()

	err := provider.Health(ctx)
	if err != nil {
		t.Errorf("In-memory provider should always be healthy: %v", err)
	}
}

func TestInMemoryProviderNameAndType(t *testing.T) {
	provider, _ := NewInMemoryProvider(map[string]interface{}{})

	if provider.Name() != "in-memory" {
		t.Errorf("Expected name 'in-memory', got '%s'", provider.Name())
	}

	if provider.Type() != "inmemory" {
		t.Errorf("Expected type 'inmemory', got '%s'", provider.Type())
	}
}

func TestMemoryProviderFactory(t *testing.T) {
	factory := NewMemoryProviderFactory()

	// Test creating in-memory provider
	provider, err := factory.CreateProvider("inmemory", map[string]interface{}{})
	if err != nil {
		t.Fatalf("Failed to create in-memory provider: %v", err)
	}

	if provider.Type() != "inmemory" {
		t.Errorf("Expected type 'inmemory', got '%s'", provider.Type())
	}

	// Test creating Redis provider
	redisProvider, err := factory.CreateProvider("redis", map[string]interface{}{
		"host": "localhost",
		"port": 6379,
	})
	if err != nil {
		t.Fatalf("Failed to create Redis provider: %v", err)
	}

	if redisProvider.Type() != "redis" {
		t.Errorf("Expected type 'redis', got '%s'", redisProvider.Type())
	}

	// Test invalid provider type
	_, err = factory.CreateProvider("invalid", map[string]interface{}{})
	if err == nil {
		t.Error("Expected error for invalid provider type")
	}
}

func TestGlobalManager(t *testing.T) {
	// Initialize global manager
	config := &MemoryConfig{}
	InitializeGlobalManager(config)

	manager := GetGlobalManager()
	if manager == nil {
		t.Fatal("Global manager not initialized")
	}

	// Test global functions
	ctx := context.Background()

	// Store
	err := StoreGlobal(ctx, "global-test", "global-value")
	if err != nil {
		t.Fatalf("Failed to store globally: %v", err)
	}

	// Retrieve
	value, err := RetrieveGlobal(ctx, "global-test")
	if err != nil {
		t.Fatalf("Failed to retrieve globally: %v", err)
	}

	if value != "global-value" {
		t.Errorf("Expected 'global-value', got %v", value)
	}

	// Search
	results, err := SearchGlobal(ctx, "global-value", 10)
	if err != nil {
		t.Fatalf("Failed to search globally: %v", err)
	}

	if len(results) == 0 {
		t.Error("Expected at least one search result")
	}
}

func TestMemoryManagerConcurrency(t *testing.T) {
	manager := NewMemoryManager(&MemoryConfig{})
	provider, _ := NewInMemoryProvider(map[string]interface{}{})
	manager.RegisterProvider("concurrency-test", provider)

	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(id int) {
			ctx := context.Background()
			key := fmt.Sprintf("concurrent-key-%d", id)
			value := fmt.Sprintf("concurrent-value-%d", id)

			// Store
			err := manager.Store(ctx, key, value)
			if err != nil {
				t.Errorf("Concurrent store failed for %d: %v", id, err)
				done <- false
				return
			}

			// Retrieve
			retrieved, err := manager.Retrieve(ctx, key)
			if err != nil {
				t.Errorf("Concurrent retrieve failed for %d: %v", id, err)
				done <- false
				return
			}

			if retrieved != value {
				t.Errorf("Concurrent data mismatch for %d: expected %s, got %v", id, value, retrieved)
				done <- false
				return
			}

			done <- true
		}(i)
	}

	// Wait for all goroutines
	allPassed := true
	for i := 0; i < 10; i++ {
		if !<-done {
			allPassed = false
		}
	}

	if !allPassed {
		t.Error("Some concurrent operations failed")
	}
}

func TestMemoryManagerHealth(t *testing.T) {
	manager := NewMemoryManager(&MemoryConfig{})

	// No providers
	health := manager.Health(context.Background())
	if len(health) != 0 {
		t.Errorf("Expected empty health map with no providers, got %d entries", len(health))
	}

	// Add provider
	provider, _ := NewInMemoryProvider(map[string]interface{}{})
	manager.RegisterProvider("health-test", provider)

	health = manager.Health(context.Background())
	if len(health) != 1 {
		t.Errorf("Expected 1 health entry, got %d", len(health))
	}

	if health["health-test"] != nil {
		t.Errorf("Expected nil error for healthy provider, got %v", health["health-test"])
	}
}

func TestMemoryManagerStatistics(t *testing.T) {
	manager := NewMemoryManager(&MemoryConfig{})

	// Add providers
	provider1, _ := NewInMemoryProvider(map[string]interface{}{})
	provider2, _ := NewInMemoryProvider(map[string]interface{}{})

	manager.RegisterProvider("provider1", provider1)
	manager.RegisterProvider("provider2", provider2)
	manager.SetDefaultProvider("provider1")

	stats := manager.GetStatistics()

	if stats["total_providers"] != 2 {
		t.Errorf("Expected 2 total providers, got %v", stats["total_providers"])
	}

	if stats["default_provider"] != "provider1" {
		t.Errorf("Expected default provider 'provider1', got %v", stats["default_provider"])
	}

	providers, ok := stats["providers"].(map[string]interface{})
	if !ok {
		t.Fatal("Providers stats not a map")
	}

	if len(providers) != 2 {
		t.Errorf("Expected 2 provider details, got %d", len(providers))
	}
}

func BenchmarkMemoryManagerStore(b *testing.B) {
	manager := NewMemoryManager(&MemoryConfig{})
	provider, _ := NewInMemoryProvider(map[string]interface{}{})
	manager.RegisterProvider("bench", provider)

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("bench-key-%d", i)
		value := fmt.Sprintf("bench-value-%d", i)
		manager.Store(ctx, key, value)
	}
}

func BenchmarkMemoryManagerRetrieve(b *testing.B) {
	manager := NewMemoryManager(&MemoryConfig{})
	provider, _ := NewInMemoryProvider(map[string]interface{}{})
	manager.RegisterProvider("bench", provider)

	ctx := context.Background()

	// Pre-populate
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("bench-key-%d", i)
		value := fmt.Sprintf("bench-value-%d", i)
		manager.Store(ctx, key, value)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("bench-key-%d", i%1000)
		manager.Retrieve(ctx, key)
	}
}

func BenchmarkInMemoryProviderStore(b *testing.B) {
	provider, _ := NewInMemoryProvider(map[string]interface{}{})
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key-%d", i)
		value := fmt.Sprintf("value-%d", i)
		provider.Store(ctx, key, value)
	}
}

func BenchmarkInMemoryProviderRetrieve(b *testing.B) {
	provider, _ := NewInMemoryProvider(map[string]interface{}{})
	ctx := context.Background()

	// Pre-populate
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("key-%d", i)
		value := fmt.Sprintf("value-%d", i)
		provider.Store(ctx, key, value)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key-%d", i%1000)
		provider.Retrieve(ctx, key)
	}
}

// =============================================================================
// Redis Memory Provider Tests (In-Memory Mode)
// =============================================================================

func TestRedisMemoryProvider_Creation(t *testing.T) {
	provider, err := NewRedisMemoryProvider(map[string]interface{}{
		"host": "localhost",
		"port": 6379,
	})

	if err != nil {
		t.Fatalf("Failed to create Redis provider: %v", err)
	}

	if provider.Name() != "redis" {
		t.Errorf("Expected name 'redis', got '%s'", provider.Name())
	}

	if provider.Type() != "redis" {
		t.Errorf("Expected type 'redis', got '%s'", provider.Type())
	}
}

func TestRedisMemoryProvider_StoreRetrieve(t *testing.T) {
	provider, _ := NewRedisMemoryProvider(map[string]interface{}{
		"host":   "localhost",
		"port":   6379,
		"prefix": "test",
	})
	ctx := context.Background()

	testData := map[string]interface{}{
		"name":  "test-redis",
		"value": 123,
	}

	// Store
	err := provider.Store(ctx, "redis-key", testData)
	if err != nil {
		t.Fatalf("Failed to store data: %v", err)
	}

	// Retrieve
	retrieved, err := provider.Retrieve(ctx, "redis-key")
	if err != nil {
		t.Fatalf("Failed to retrieve data: %v", err)
	}

	retrievedMap, ok := retrieved.(map[string]interface{})
	if !ok {
		t.Fatal("Retrieved data is not a map")
	}

	if retrievedMap["name"] != "test-redis" {
		t.Errorf("Expected name 'test-redis', got %v", retrievedMap["name"])
	}
}

func TestRedisMemoryProvider_Delete(t *testing.T) {
	provider, _ := NewRedisMemoryProvider(map[string]interface{}{})
	ctx := context.Background()

	// Store
	provider.Store(ctx, "delete-key", "delete-value")

	// Delete
	err := provider.Delete(ctx, "delete-key")
	if err != nil {
		t.Fatalf("Failed to delete: %v", err)
	}

	// Verify deleted
	_, err = provider.Retrieve(ctx, "delete-key")
	if err == nil {
		t.Error("Expected error for deleted key")
	}
}

func TestRedisMemoryProvider_Clear(t *testing.T) {
	provider, _ := NewRedisMemoryProvider(map[string]interface{}{})
	ctx := context.Background()

	// Store multiple
	provider.Store(ctx, "key1", "value1")
	provider.Store(ctx, "key2", "value2")

	// Clear
	err := provider.Clear(ctx)
	if err != nil {
		t.Fatalf("Failed to clear: %v", err)
	}

	// Verify cleared
	_, err1 := provider.Retrieve(ctx, "key1")
	_, err2 := provider.Retrieve(ctx, "key2")

	if err1 == nil || err2 == nil {
		t.Error("Expected errors for cleared keys")
	}
}

func TestRedisMemoryProvider_Search(t *testing.T) {
	provider, _ := NewRedisMemoryProvider(map[string]interface{}{})
	ctx := context.Background()

	provider.Store(ctx, "alice", "John")
	provider.Store(ctx, "bob", "John")
	provider.Store(ctx, "charlie", "Jane")

	// Search for value "John" - should find 2 results
	results, err := provider.Search(ctx, "John", 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 results for value 'John', got %d", len(results))
	}
}

// TestRedisMemoryProvider_Health asserts the round-31 §11.4 anti-bluff
// contract: Health MUST surface ErrRedisClientNotInitialized whenever no
// real go-redis/v9 client has been wired into the provider, instead of
// the previous unconditional nil return (which fabricated PASS for dead
// Redis backends). When a real client is wired in this test must be
// updated to assert real connectivity against a real Redis container per
// CONST-050(A).
func TestRedisMemoryProvider_Health(t *testing.T) {
	provider, _ := NewRedisMemoryProvider(map[string]interface{}{})
	ctx := context.Background()

	err := provider.Health(ctx)
	if err == nil {
		t.Fatal("Health() returned nil — anti-bluff regression: a Redis provider with no real client wired in MUST NOT report healthy")
	}
	if !errors.Is(err, ErrRedisClientNotInitialized) {
		t.Errorf("Health() = %v, want error wrapping ErrRedisClientNotInitialized", err)
	}
}

// =============================================================================
// Memcached Memory Provider Tests (In-Memory Mode)
// =============================================================================

func TestMemcachedMemoryProvider_Creation(t *testing.T) {
	provider, err := NewMemcachedMemoryProvider(map[string]interface{}{
		"host": "localhost",
		"port": 11211,
	})

	if err != nil {
		t.Fatalf("Failed to create Memcached provider: %v", err)
	}

	if provider.Name() != "memcached" {
		t.Errorf("Expected name 'memcached', got '%s'", provider.Name())
	}

	if provider.Type() != "memcached" {
		t.Errorf("Expected type 'memcached', got '%s'", provider.Type())
	}
}

func TestMemcachedMemoryProvider_StoreRetrieve(t *testing.T) {
	provider, _ := NewMemcachedMemoryProvider(map[string]interface{}{})
	ctx := context.Background()

	testData := map[string]interface{}{
		"name":  "test-memcached",
		"value": 456,
	}

	// Store
	err := provider.Store(ctx, "mc-key", testData)
	if err != nil {
		t.Fatalf("Failed to store data: %v", err)
	}

	// Retrieve
	retrieved, err := provider.Retrieve(ctx, "mc-key")
	if err != nil {
		t.Fatalf("Failed to retrieve data: %v", err)
	}

	retrievedMap, ok := retrieved.(map[string]interface{})
	if !ok {
		t.Fatal("Retrieved data is not a map")
	}

	if retrievedMap["name"] != "test-memcached" {
		t.Errorf("Expected name 'test-memcached', got %v", retrievedMap["name"])
	}
}

func TestMemcachedMemoryProvider_Delete(t *testing.T) {
	provider, _ := NewMemcachedMemoryProvider(map[string]interface{}{})
	ctx := context.Background()

	provider.Store(ctx, "delete-key", "value")

	err := provider.Delete(ctx, "delete-key")
	if err != nil {
		t.Fatalf("Failed to delete: %v", err)
	}

	_, err = provider.Retrieve(ctx, "delete-key")
	if err == nil {
		t.Error("Expected error for deleted key")
	}
}

func TestMemcachedMemoryProvider_Clear(t *testing.T) {
	provider, _ := NewMemcachedMemoryProvider(map[string]interface{}{})
	ctx := context.Background()

	provider.Store(ctx, "key1", "value1")
	provider.Store(ctx, "key2", "value2")

	err := provider.Clear(ctx)
	if err != nil {
		t.Fatalf("Failed to clear: %v", err)
	}

	_, err1 := provider.Retrieve(ctx, "key1")
	_, err2 := provider.Retrieve(ctx, "key2")

	if err1 == nil || err2 == nil {
		t.Error("Expected errors for cleared keys")
	}
}

func TestMemcachedMemoryProvider_Search(t *testing.T) {
	provider, _ := NewMemcachedMemoryProvider(map[string]interface{}{})
	ctx := context.Background()

	provider.Store(ctx, "alpha", "SearchValue")
	provider.Store(ctx, "beta", "SearchValue")
	provider.Store(ctx, "gamma", "OtherValue")

	results, err := provider.Search(ctx, "SearchValue", 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 results for value 'SearchValue', got %d", len(results))
	}
}

// TestMemcachedMemoryProvider_Health asserts the round-31 §11.4 anti-bluff
// contract: Health MUST surface ErrMemcachedClientNotInitialized whenever
// no real gomemcache client has been wired into the provider, instead of
// the previous unconditional nil return (which fabricated PASS for dead
// Memcached backends). When a real client is wired in this test must be
// updated to assert real connectivity against a real Memcached container
// per CONST-050(A).
func TestMemcachedMemoryProvider_Health(t *testing.T) {
	provider, _ := NewMemcachedMemoryProvider(map[string]interface{}{})
	ctx := context.Background()

	err := provider.Health(ctx)
	if err == nil {
		t.Fatal("Health() returned nil — anti-bluff regression: a Memcached provider with no real client wired in MUST NOT report healthy")
	}
	if !errors.Is(err, ErrMemcachedClientNotInitialized) {
		t.Errorf("Health() = %v, want error wrapping ErrMemcachedClientNotInitialized", err)
	}
}

// =============================================================================
// Filesystem Memory Provider Tests
// =============================================================================

func TestFilesystemMemoryProvider_Creation(t *testing.T) {
	tempDir := t.TempDir()

	provider, err := NewFilesystemMemoryProvider(map[string]interface{}{
		"path": tempDir,
	})

	if err != nil {
		t.Fatalf("Failed to create Filesystem provider: %v", err)
	}

	if provider.Name() != "filesystem" {
		t.Errorf("Expected name 'filesystem', got '%s'", provider.Name())
	}

	if provider.Type() != "filesystem" {
		t.Errorf("Expected type 'filesystem', got '%s'", provider.Type())
	}
}

func TestFilesystemMemoryProvider_StoreRetrieve(t *testing.T) {
	tempDir := t.TempDir()
	provider, _ := NewFilesystemMemoryProvider(map[string]interface{}{
		"path": tempDir,
	})
	ctx := context.Background()

	testData := map[string]interface{}{
		"name":  "test-fs",
		"value": 789,
	}

	// Store
	err := provider.Store(ctx, "fs-key", testData)
	if err != nil {
		t.Fatalf("Failed to store data: %v", err)
	}

	// Retrieve
	retrieved, err := provider.Retrieve(ctx, "fs-key")
	if err != nil {
		t.Fatalf("Failed to retrieve data: %v", err)
	}

	retrievedMap, ok := retrieved.(map[string]interface{})
	if !ok {
		t.Fatal("Retrieved data is not a map")
	}

	if retrievedMap["name"] != "test-fs" {
		t.Errorf("Expected name 'test-fs', got %v", retrievedMap["name"])
	}
}

func TestFilesystemMemoryProvider_Delete(t *testing.T) {
	tempDir := t.TempDir()
	provider, _ := NewFilesystemMemoryProvider(map[string]interface{}{
		"path": tempDir,
	})
	ctx := context.Background()

	provider.Store(ctx, "delete-key", "value")

	err := provider.Delete(ctx, "delete-key")
	if err != nil {
		t.Fatalf("Failed to delete: %v", err)
	}

	_, err = provider.Retrieve(ctx, "delete-key")
	if err == nil {
		t.Error("Expected error for deleted key")
	}
}

func TestFilesystemMemoryProvider_Clear(t *testing.T) {
	tempDir := t.TempDir()
	provider, _ := NewFilesystemMemoryProvider(map[string]interface{}{
		"path": tempDir,
	})
	ctx := context.Background()

	provider.Store(ctx, "key1", "value1")
	provider.Store(ctx, "key2", "value2")

	err := provider.Clear(ctx)
	if err != nil {
		t.Fatalf("Failed to clear: %v", err)
	}

	_, err1 := provider.Retrieve(ctx, "key1")
	_, err2 := provider.Retrieve(ctx, "key2")

	if err1 == nil || err2 == nil {
		t.Error("Expected errors for cleared keys")
	}
}

func TestFilesystemMemoryProvider_Search(t *testing.T) {
	tempDir := t.TempDir()
	provider, _ := NewFilesystemMemoryProvider(map[string]interface{}{
		"path": tempDir,
	})
	ctx := context.Background()

	provider.Store(ctx, "first", "SearchableContent")
	provider.Store(ctx, "second", "SearchableContent")
	provider.Store(ctx, "third", "DifferentContent")

	results, err := provider.Search(ctx, "SearchableContent", 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 results for value 'SearchableContent', got %d", len(results))
	}
}

func TestFilesystemMemoryProvider_Health(t *testing.T) {
	tempDir := t.TempDir()
	provider, _ := NewFilesystemMemoryProvider(map[string]interface{}{
		"path": tempDir,
	})
	ctx := context.Background()

	err := provider.Health(ctx)
	if err != nil {
		t.Errorf("Filesystem provider should be healthy with valid path: %v", err)
	}
}

func TestFilesystemMemoryProvider_SpecialCharactersInKey(t *testing.T) {
	tempDir := t.TempDir()
	provider, _ := NewFilesystemMemoryProvider(map[string]interface{}{
		"path": tempDir,
	})
	ctx := context.Background()

	// Keys with special characters should be handled safely
	specialKeys := []string{
		"key/with/slashes",
		"key..with..dots",
		"key with spaces",
		"key:with:colons",
	}

	for _, key := range specialKeys {
		err := provider.Store(ctx, key, "value")
		if err != nil {
			t.Errorf("Failed to store key '%s': %v", key, err)
			continue
		}

		retrieved, err := provider.Retrieve(ctx, key)
		if err != nil {
			t.Errorf("Failed to retrieve key '%s': %v", key, err)
			continue
		}

		if retrieved != "value" {
			t.Errorf("Value mismatch for key '%s': expected 'value', got %v", key, retrieved)
		}
	}
}

// =============================================================================
// Factory Tests for New Providers
// =============================================================================

func TestMemoryProviderFactory_AllProviders(t *testing.T) {
	factory := NewMemoryProviderFactory()
	tempDir := t.TempDir()

	testCases := []struct {
		providerType string
		config       map[string]interface{}
		expectedType string
	}{
		{"inmemory", map[string]interface{}{}, "inmemory"},
		{"redis", map[string]interface{}{"host": "localhost", "port": 6379}, "redis"},
		{"memcached", map[string]interface{}{"host": "localhost", "port": 11211}, "memcached"},
		{"filesystem", map[string]interface{}{"path": tempDir}, "filesystem"},
	}

	for _, tc := range testCases {
		t.Run(tc.providerType, func(t *testing.T) {
			provider, err := factory.CreateProvider(tc.providerType, tc.config)
			if err != nil {
				t.Fatalf("Failed to create %s provider: %v", tc.providerType, err)
			}

			if provider.Type() != tc.expectedType {
				t.Errorf("Expected type '%s', got '%s'", tc.expectedType, provider.Type())
			}
		})
	}
}
