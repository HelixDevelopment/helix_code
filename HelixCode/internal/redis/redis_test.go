package redis

import (
	"context"
	"sync"
	"testing"
	"time"

	"dev.helix.code/internal/config"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Client Creation and Connection Tests
// =============================================================================

func TestNewClient_Disabled(t *testing.T) {
	cfg := &config.RedisConfig{
		Enabled: false,
		Host:    "localhost",
		Port:    6379,
	}

	client, err := NewClient(cfg)
	assert.NoError(t, err)
	assert.NotNil(t, client)
	assert.False(t, client.IsEnabled())
	assert.Nil(t, client.GetClient())
}

func TestNewClient_InvalidConfig(t *testing.T) {
	cfg := &config.RedisConfig{
		Enabled: true,
		Host:    "invalid-host",
		Port:    6379,
	}

	client, err := NewClient(cfg)
	assert.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "failed to connect to Redis")
}

func TestNewClient_InvalidPort(t *testing.T) {
	cfg := &config.RedisConfig{
		Enabled: true,
		Host:    "localhost",
		Port:    99999, // Invalid port
	}

	client, err := NewClient(cfg)
	assert.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "failed to connect to Redis")
}

func TestNewClient_EmptyHost(t *testing.T) {
	cfg := &config.RedisConfig{
		Enabled: true,
		Host:    "",
		Port:    6379,
	}

	client, err := NewClient(cfg)
	assert.Error(t, err)
	assert.Nil(t, client)
}

func TestNewClient_WithPassword(t *testing.T) {
	cfg := &config.RedisConfig{
		Enabled:  true,
		Host:     "localhost",
		Port:     6379,
		Password: "wrong-password",
	}

	// This should fail with auth error or connection error
	client, err := NewClient(cfg)
	// Either connection fails or auth fails
	if err != nil {
		assert.Nil(t, client)
	}
}

func TestNewClient_WithDatabase(t *testing.T) {
	cfg := &config.RedisConfig{
		Enabled:  true,
		Host:     "localhost",
		Port:     6379,
		Database: 15, // Valid database number
	}

	// This will fail if Redis isn't running, which is expected in CI
	_, _ = NewClient(cfg)
}

// =============================================================================
// Client Close Tests
// =============================================================================

func TestClient_Close(t *testing.T) {
	// Test Close on disabled client
	cfg := &config.RedisConfig{Enabled: false}
	client, _ := NewClient(cfg)
	assert.NoError(t, client.Close())

	// Test Close on client with nil client
	client = &Client{client: nil, config: &config.RedisConfig{Enabled: true}}
	assert.NoError(t, client.Close())
}

func TestClient_Close_MultipleTimesDisabled(t *testing.T) {
	cfg := &config.RedisConfig{Enabled: false}
	client, _ := NewClient(cfg)

	// Closing multiple times should not cause errors
	assert.NoError(t, client.Close())
	assert.NoError(t, client.Close())
	assert.NoError(t, client.Close())
}

// =============================================================================
// IsEnabled and GetClient Tests
// =============================================================================

func TestClient_IsEnabled_NilConfig(t *testing.T) {
	client := &Client{client: nil, config: nil}
	assert.False(t, client.IsEnabled())
}

func TestClient_IsEnabled_Disabled(t *testing.T) {
	client := &Client{client: nil, config: &config.RedisConfig{Enabled: false}}
	assert.False(t, client.IsEnabled())
}

func TestClient_IsEnabled_Enabled(t *testing.T) {
	client := &Client{client: nil, config: &config.RedisConfig{Enabled: true}}
	assert.True(t, client.IsEnabled())
}

func TestClient_GetClient_NilWhenDisabled(t *testing.T) {
	cfg := &config.RedisConfig{Enabled: false}
	client, _ := NewClient(cfg)
	assert.Nil(t, client.GetClient())
}

// =============================================================================
// Disabled Client Operation Tests (No-ops and Errors)
// =============================================================================

func TestClient_Methods_Disabled(t *testing.T) {
	cfg := &config.RedisConfig{Enabled: false}
	client, _ := NewClient(cfg)
	ctx := context.Background()

	// Test methods that should be no-ops when disabled
	assert.NoError(t, client.Set(ctx, "key", "value", 0))
	assert.NoError(t, client.Del(ctx, "key"))
	assert.NoError(t, client.Expire(ctx, "key", time.Hour))
	assert.NoError(t, client.HSet(ctx, "key", "field", "value"))
	assert.NoError(t, client.HDel(ctx, "key", "field"))
	assert.NoError(t, client.Publish(ctx, "channel", "message"))
	assert.NoError(t, client.LPush(ctx, "key", "value"))
	assert.NoError(t, client.SAdd(ctx, "key", "member"))
	assert.NoError(t, client.SRem(ctx, "key", "member"))
	assert.NoError(t, client.ZAdd(ctx, "key"))
	assert.NoError(t, client.ZRem(ctx, "key", "member"))

	// Test methods that should return errors when disabled
	_, err := client.Get(ctx, "key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Redis is disabled")

	_, err = client.Exists(ctx, "key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Redis is disabled")

	_, err = client.TTL(ctx, "key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Redis is disabled")

	_, err = client.HGet(ctx, "key", "field")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Redis is disabled")

	_, err = client.HGetAll(ctx, "key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Redis is disabled")

	_, err = client.RPop(ctx, "key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Redis is disabled")

	_, err = client.BRPop(ctx, time.Second, "key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Redis is disabled")

	_, err = client.LLen(ctx, "key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Redis is disabled")

	_, err = client.SMembers(ctx, "key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Redis is disabled")

	_, err = client.ZRange(ctx, "key", 0, -1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Redis is disabled")

	// Test Subscribe (should return nil when disabled)
	pubsub := client.Subscribe(ctx, "channel")
	assert.Nil(t, pubsub)
}

func TestClient_Set_DisabledWithExpiration(t *testing.T) {
	cfg := &config.RedisConfig{Enabled: false}
	client, _ := NewClient(cfg)
	ctx := context.Background()

	// Set with various expirations should be no-op
	assert.NoError(t, client.Set(ctx, "key1", "value1", 0))
	assert.NoError(t, client.Set(ctx, "key2", "value2", time.Second))
	assert.NoError(t, client.Set(ctx, "key3", "value3", time.Hour))
	assert.NoError(t, client.Set(ctx, "key4", "value4", -1)) // Negative duration
}

func TestClient_Del_DisabledMultipleKeys(t *testing.T) {
	cfg := &config.RedisConfig{Enabled: false}
	client, _ := NewClient(cfg)
	ctx := context.Background()

	// Delete multiple keys should be no-op
	assert.NoError(t, client.Del(ctx, "key1", "key2", "key3"))
	assert.NoError(t, client.Del(ctx)) // No keys
}

func TestClient_Exists_DisabledMultipleKeys(t *testing.T) {
	cfg := &config.RedisConfig{Enabled: false}
	client, _ := NewClient(cfg)
	ctx := context.Background()

	_, err := client.Exists(ctx, "key1", "key2", "key3")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Redis is disabled")
}

// =============================================================================
// Hash Operation Tests (Disabled Client)
// =============================================================================

func TestClient_HSet_DisabledMultipleFields(t *testing.T) {
	cfg := &config.RedisConfig{Enabled: false}
	client, _ := NewClient(cfg)
	ctx := context.Background()

	// HSet with multiple field-value pairs should be no-op
	assert.NoError(t, client.HSet(ctx, "hash", "field1", "value1", "field2", "value2"))
	assert.NoError(t, client.HSet(ctx, "hash")) // No fields
}

func TestClient_HDel_DisabledMultipleFields(t *testing.T) {
	cfg := &config.RedisConfig{Enabled: false}
	client, _ := NewClient(cfg)
	ctx := context.Background()

	// HDel with multiple fields should be no-op
	assert.NoError(t, client.HDel(ctx, "hash", "field1", "field2", "field3"))
	assert.NoError(t, client.HDel(ctx, "hash")) // No fields
}

// =============================================================================
// List Operation Tests (Disabled Client)
// =============================================================================

func TestClient_LPush_DisabledMultipleValues(t *testing.T) {
	cfg := &config.RedisConfig{Enabled: false}
	client, _ := NewClient(cfg)
	ctx := context.Background()

	// LPush with multiple values should be no-op
	assert.NoError(t, client.LPush(ctx, "list", "val1", "val2", "val3"))
	assert.NoError(t, client.LPush(ctx, "list")) // No values
}

func TestClient_BRPop_DisabledMultipleKeys(t *testing.T) {
	cfg := &config.RedisConfig{Enabled: false}
	client, _ := NewClient(cfg)
	ctx := context.Background()

	_, err := client.BRPop(ctx, time.Second, "list1", "list2")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Redis is disabled")
}

// =============================================================================
// Set Operation Tests (Disabled Client)
// =============================================================================

func TestClient_SAdd_DisabledMultipleMembers(t *testing.T) {
	cfg := &config.RedisConfig{Enabled: false}
	client, _ := NewClient(cfg)
	ctx := context.Background()

	// SAdd with multiple members should be no-op
	assert.NoError(t, client.SAdd(ctx, "set", "mem1", "mem2", "mem3"))
	assert.NoError(t, client.SAdd(ctx, "set")) // No members
}

func TestClient_SRem_DisabledMultipleMembers(t *testing.T) {
	cfg := &config.RedisConfig{Enabled: false}
	client, _ := NewClient(cfg)
	ctx := context.Background()

	// SRem with multiple members should be no-op
	assert.NoError(t, client.SRem(ctx, "set", "mem1", "mem2", "mem3"))
	assert.NoError(t, client.SRem(ctx, "set")) // No members
}

// =============================================================================
// Sorted Set Operation Tests (Disabled Client)
// =============================================================================

func TestClient_ZAdd_DisabledWithMembers(t *testing.T) {
	cfg := &config.RedisConfig{Enabled: false}
	client, _ := NewClient(cfg)
	ctx := context.Background()

	// ZAdd with members should be no-op
	members := []*redis.Z{
		{Score: 1.0, Member: "one"},
		{Score: 2.0, Member: "two"},
	}
	assert.NoError(t, client.ZAdd(ctx, "zset", members...))
	assert.NoError(t, client.ZAdd(ctx, "zset")) // No members
}

func TestClient_ZRem_DisabledMultipleMembers(t *testing.T) {
	cfg := &config.RedisConfig{Enabled: false}
	client, _ := NewClient(cfg)
	ctx := context.Background()

	// ZRem with multiple members should be no-op
	assert.NoError(t, client.ZRem(ctx, "zset", "mem1", "mem2", "mem3"))
	assert.NoError(t, client.ZRem(ctx, "zset")) // No members
}

func TestClient_ZRange_DisabledVariousRanges(t *testing.T) {
	cfg := &config.RedisConfig{Enabled: false}
	client, _ := NewClient(cfg)
	ctx := context.Background()

	// ZRange with various ranges should return error
	_, err := client.ZRange(ctx, "zset", 0, -1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Redis is disabled")

	_, err = client.ZRange(ctx, "zset", 0, 10)
	assert.Error(t, err)

	_, err = client.ZRange(ctx, "zset", -5, -1)
	assert.Error(t, err)
}

// =============================================================================
// Pub/Sub Tests (Disabled Client)
// =============================================================================

func TestClient_Subscribe_DisabledMultipleChannels(t *testing.T) {
	cfg := &config.RedisConfig{Enabled: false}
	client, _ := NewClient(cfg)
	ctx := context.Background()

	// Subscribe to multiple channels should return nil when disabled
	pubsub := client.Subscribe(ctx, "channel1", "channel2", "channel3")
	assert.Nil(t, pubsub)
}

func TestClient_Publish_DisabledVariousMessages(t *testing.T) {
	cfg := &config.RedisConfig{Enabled: false}
	client, _ := NewClient(cfg)
	ctx := context.Background()

	// Publish various message types should be no-op
	assert.NoError(t, client.Publish(ctx, "channel", "string message"))
	assert.NoError(t, client.Publish(ctx, "channel", 12345))
	assert.NoError(t, client.Publish(ctx, "channel", []byte("bytes")))
	assert.NoError(t, client.Publish(ctx, "channel", nil))
}

// =============================================================================
// TTL and Expiration Tests (Disabled Client)
// =============================================================================

func TestClient_Expire_DisabledVariousDurations(t *testing.T) {
	cfg := &config.RedisConfig{Enabled: false}
	client, _ := NewClient(cfg)
	ctx := context.Background()

	// Expire with various durations should be no-op
	assert.NoError(t, client.Expire(ctx, "key", time.Second))
	assert.NoError(t, client.Expire(ctx, "key", time.Minute))
	assert.NoError(t, client.Expire(ctx, "key", time.Hour))
	assert.NoError(t, client.Expire(ctx, "key", 24*time.Hour))
	assert.NoError(t, client.Expire(ctx, "key", 0))
	assert.NoError(t, client.Expire(ctx, "key", -1)) // Negative duration
}

// =============================================================================
// Context Handling Tests
// =============================================================================

func TestClient_ContextCancellation_Disabled(t *testing.T) {
	cfg := &config.RedisConfig{Enabled: false}
	client, _ := NewClient(cfg)

	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Operations on disabled client should still work even with cancelled context
	// because they are no-ops
	assert.NoError(t, client.Set(ctx, "key", "value", 0))
	assert.NoError(t, client.Del(ctx, "key"))

	// Read operations should return "Redis is disabled" error, not context error
	_, err := client.Get(ctx, "key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Redis is disabled")
}

func TestClient_ContextTimeout_Disabled(t *testing.T) {
	cfg := &config.RedisConfig{Enabled: false}
	client, _ := NewClient(cfg)

	// Create a context with timeout that has already expired
	ctx, cancel := context.WithTimeout(context.Background(), 0)
	defer cancel()

	// Operations on disabled client should still work even with expired context
	assert.NoError(t, client.Set(ctx, "key", "value", 0))

	// Read operations should return "Redis is disabled" error
	_, err := client.Get(ctx, "key")
	assert.Contains(t, err.Error(), "Redis is disabled")
}

// =============================================================================
// Concurrent Access Tests (Disabled Client)
// =============================================================================

func TestClient_ConcurrentAccess_Disabled(t *testing.T) {
	cfg := &config.RedisConfig{Enabled: false}
	client, _ := NewClient(cfg)
	ctx := context.Background()

	var wg sync.WaitGroup
	numGoroutines := 100

	// Test concurrent Set operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			err := client.Set(ctx, "key", i, 0)
			assert.NoError(t, err)
		}(i)
	}

	// Test concurrent Get operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := client.Get(ctx, "key")
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "Redis is disabled")
		}()
	}

	// Test concurrent Del operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := client.Del(ctx, "key")
			assert.NoError(t, err)
		}()
	}

	wg.Wait()
}

func TestClient_ConcurrentHashOperations_Disabled(t *testing.T) {
	cfg := &config.RedisConfig{Enabled: false}
	client, _ := NewClient(cfg)
	ctx := context.Background()

	var wg sync.WaitGroup
	numGoroutines := 50

	for i := 0; i < numGoroutines; i++ {
		wg.Add(3)

		go func(i int) {
			defer wg.Done()
			err := client.HSet(ctx, "hash", "field", i)
			assert.NoError(t, err)
		}(i)

		go func() {
			defer wg.Done()
			_, err := client.HGet(ctx, "hash", "field")
			assert.Error(t, err)
		}()

		go func() {
			defer wg.Done()
			_, err := client.HGetAll(ctx, "hash")
			assert.Error(t, err)
		}()
	}

	wg.Wait()
}

func TestClient_ConcurrentListOperations_Disabled(t *testing.T) {
	cfg := &config.RedisConfig{Enabled: false}
	client, _ := NewClient(cfg)
	ctx := context.Background()

	var wg sync.WaitGroup
	numGoroutines := 50

	for i := 0; i < numGoroutines; i++ {
		wg.Add(3)

		go func(i int) {
			defer wg.Done()
			err := client.LPush(ctx, "list", i)
			assert.NoError(t, err)
		}(i)

		go func() {
			defer wg.Done()
			_, err := client.RPop(ctx, "list")
			assert.Error(t, err)
		}()

		go func() {
			defer wg.Done()
			_, err := client.LLen(ctx, "list")
			assert.Error(t, err)
		}()
	}

	wg.Wait()
}

func TestClient_ConcurrentSetOperations_Disabled(t *testing.T) {
	cfg := &config.RedisConfig{Enabled: false}
	client, _ := NewClient(cfg)
	ctx := context.Background()

	var wg sync.WaitGroup
	numGoroutines := 50

	for i := 0; i < numGoroutines; i++ {
		wg.Add(3)

		go func(i int) {
			defer wg.Done()
			err := client.SAdd(ctx, "set", i)
			assert.NoError(t, err)
		}(i)

		go func(i int) {
			defer wg.Done()
			err := client.SRem(ctx, "set", i)
			assert.NoError(t, err)
		}(i)

		go func() {
			defer wg.Done()
			_, err := client.SMembers(ctx, "set")
			assert.Error(t, err)
		}()
	}

	wg.Wait()
}

func TestClient_ConcurrentSortedSetOperations_Disabled(t *testing.T) {
	cfg := &config.RedisConfig{Enabled: false}
	client, _ := NewClient(cfg)
	ctx := context.Background()

	var wg sync.WaitGroup
	numGoroutines := 50

	for i := 0; i < numGoroutines; i++ {
		wg.Add(3)

		go func(i int) {
			defer wg.Done()
			member := &redis.Z{Score: float64(i), Member: i}
			err := client.ZAdd(ctx, "zset", member)
			assert.NoError(t, err)
		}(i)

		go func(i int) {
			defer wg.Done()
			err := client.ZRem(ctx, "zset", i)
			assert.NoError(t, err)
		}(i)

		go func() {
			defer wg.Done()
			_, err := client.ZRange(ctx, "zset", 0, -1)
			assert.Error(t, err)
		}()
	}

	wg.Wait()
}

// =============================================================================
// Edge Case Tests
// =============================================================================

func TestClient_EmptyKey_Disabled(t *testing.T) {
	cfg := &config.RedisConfig{Enabled: false}
	client, _ := NewClient(cfg)
	ctx := context.Background()

	// Operations with empty key should still be no-ops when disabled
	assert.NoError(t, client.Set(ctx, "", "value", 0))
	assert.NoError(t, client.Del(ctx, ""))

	_, err := client.Get(ctx, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Redis is disabled")
}

func TestClient_SpecialCharactersInKey_Disabled(t *testing.T) {
	cfg := &config.RedisConfig{Enabled: false}
	client, _ := NewClient(cfg)
	ctx := context.Background()

	specialKeys := []string{
		"key:with:colons",
		"key with spaces",
		"key\twith\ttabs",
		"key\nwith\nnewlines",
		"key/with/slashes",
		"key\\with\\backslashes",
		"unicode:ключ:键",
	}

	for _, key := range specialKeys {
		assert.NoError(t, client.Set(ctx, key, "value", 0))
		_, err := client.Get(ctx, key)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Redis is disabled")
	}
}

func TestClient_LargeValue_Disabled(t *testing.T) {
	cfg := &config.RedisConfig{Enabled: false}
	client, _ := NewClient(cfg)
	ctx := context.Background()

	// Create a large value (1MB)
	largeValue := make([]byte, 1024*1024)
	for i := range largeValue {
		largeValue[i] = byte(i % 256)
	}

	assert.NoError(t, client.Set(ctx, "large_key", string(largeValue), 0))
}

func TestClient_NilValue_Disabled(t *testing.T) {
	cfg := &config.RedisConfig{Enabled: false}
	client, _ := NewClient(cfg)
	ctx := context.Background()

	// Setting nil value should be no-op when disabled
	assert.NoError(t, client.Set(ctx, "key", nil, 0))
}

// =============================================================================
// Mock Client Tests (Testing with mocked internal client)
// =============================================================================

func TestClient_CreateWithNilConfig(t *testing.T) {
	// Creating a client struct directly with nil config
	client := &Client{client: nil, config: nil}

	assert.False(t, client.IsEnabled())
	assert.Nil(t, client.GetClient())
	assert.NoError(t, client.Close())
}

func TestClient_StructFields(t *testing.T) {
	cfg := &config.RedisConfig{
		Enabled:  false,
		Host:     "localhost",
		Port:     6379,
		Password: "secret",
		Database: 1,
	}

	client, err := NewClient(cfg)
	require.NoError(t, err)

	// Verify the client is created correctly
	assert.NotNil(t, client)
	assert.False(t, client.IsEnabled())
	assert.Nil(t, client.client)
	assert.Equal(t, cfg, client.config)
}

// =============================================================================
// Integration-style Tests (Without actual Redis connection)
// =============================================================================

func TestClient_FullWorkflow_Disabled(t *testing.T) {
	cfg := &config.RedisConfig{Enabled: false}
	client, _ := NewClient(cfg)
	ctx := context.Background()

	// Simulate a typical workflow with disabled client
	// 1. Set a value
	assert.NoError(t, client.Set(ctx, "session:123", "user_data", time.Hour))

	// 2. Try to get it (should fail because disabled)
	_, err := client.Get(ctx, "session:123")
	assert.Error(t, err)

	// 3. Set expiration
	assert.NoError(t, client.Expire(ctx, "session:123", 30*time.Minute))

	// 4. Check TTL (should fail)
	_, err = client.TTL(ctx, "session:123")
	assert.Error(t, err)

	// 5. Delete
	assert.NoError(t, client.Del(ctx, "session:123"))

	// 6. Check existence (should fail)
	_, err = client.Exists(ctx, "session:123")
	assert.Error(t, err)
}

func TestClient_HashWorkflow_Disabled(t *testing.T) {
	cfg := &config.RedisConfig{Enabled: false}
	client, _ := NewClient(cfg)
	ctx := context.Background()

	// Simulate a hash workflow
	// 1. Set hash fields
	assert.NoError(t, client.HSet(ctx, "user:123", "name", "John", "email", "john@example.com"))

	// 2. Get single field (should fail)
	_, err := client.HGet(ctx, "user:123", "name")
	assert.Error(t, err)

	// 3. Get all fields (should fail)
	_, err = client.HGetAll(ctx, "user:123")
	assert.Error(t, err)

	// 4. Delete fields
	assert.NoError(t, client.HDel(ctx, "user:123", "email"))
}

func TestClient_ListWorkflow_Disabled(t *testing.T) {
	cfg := &config.RedisConfig{Enabled: false}
	client, _ := NewClient(cfg)
	ctx := context.Background()

	// Simulate a list (queue) workflow
	// 1. Push items
	assert.NoError(t, client.LPush(ctx, "queue:jobs", "job1", "job2", "job3"))

	// 2. Get length (should fail)
	_, err := client.LLen(ctx, "queue:jobs")
	assert.Error(t, err)

	// 3. Pop item (should fail)
	_, err = client.RPop(ctx, "queue:jobs")
	assert.Error(t, err)
}

func TestClient_SetWorkflow_Disabled(t *testing.T) {
	cfg := &config.RedisConfig{Enabled: false}
	client, _ := NewClient(cfg)
	ctx := context.Background()

	// Simulate a set workflow
	// 1. Add members
	assert.NoError(t, client.SAdd(ctx, "tags:article:123", "go", "redis", "testing"))

	// 2. Get members (should fail)
	_, err := client.SMembers(ctx, "tags:article:123")
	assert.Error(t, err)

	// 3. Remove member
	assert.NoError(t, client.SRem(ctx, "tags:article:123", "testing"))
}

func TestClient_SortedSetWorkflow_Disabled(t *testing.T) {
	cfg := &config.RedisConfig{Enabled: false}
	client, _ := NewClient(cfg)
	ctx := context.Background()

	// Simulate a sorted set (leaderboard) workflow
	// 1. Add members with scores
	members := []*redis.Z{
		{Score: 100, Member: "player1"},
		{Score: 200, Member: "player2"},
		{Score: 150, Member: "player3"},
	}
	assert.NoError(t, client.ZAdd(ctx, "leaderboard", members...))

	// 2. Get range (should fail)
	_, err := client.ZRange(ctx, "leaderboard", 0, -1)
	assert.Error(t, err)

	// 3. Remove member
	assert.NoError(t, client.ZRem(ctx, "leaderboard", "player1"))
}

func TestClient_PubSubWorkflow_Disabled(t *testing.T) {
	cfg := &config.RedisConfig{Enabled: false}
	client, _ := NewClient(cfg)
	ctx := context.Background()

	// Simulate a pub/sub workflow
	// 1. Subscribe (should return nil)
	pubsub := client.Subscribe(ctx, "notifications")
	assert.Nil(t, pubsub)

	// 2. Publish (should be no-op)
	assert.NoError(t, client.Publish(ctx, "notifications", "Hello, World!"))
}

// =============================================================================
// Error Message Tests
// =============================================================================

func TestClient_DisabledErrorMessages(t *testing.T) {
	cfg := &config.RedisConfig{Enabled: false}
	client, _ := NewClient(cfg)
	ctx := context.Background()

	// All read operations should return consistent error messages
	readOperations := []struct {
		name string
		fn   func() error
	}{
		{"Get", func() error { _, err := client.Get(ctx, "k"); return err }},
		{"Exists", func() error { _, err := client.Exists(ctx, "k"); return err }},
		{"TTL", func() error { _, err := client.TTL(ctx, "k"); return err }},
		{"HGet", func() error { _, err := client.HGet(ctx, "k", "f"); return err }},
		{"HGetAll", func() error { _, err := client.HGetAll(ctx, "k"); return err }},
		{"RPop", func() error { _, err := client.RPop(ctx, "k"); return err }},
		{"BRPop", func() error { _, err := client.BRPop(ctx, time.Second, "k"); return err }},
		{"LLen", func() error { _, err := client.LLen(ctx, "k"); return err }},
		{"SMembers", func() error { _, err := client.SMembers(ctx, "k"); return err }},
		{"ZRange", func() error { _, err := client.ZRange(ctx, "k", 0, -1); return err }},
	}

	for _, op := range readOperations {
		t.Run(op.name, func(t *testing.T) {
			err := op.fn()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "Redis is disabled")
		})
	}
}

// =============================================================================
// Configuration Variation Tests
// =============================================================================

func TestNewClient_VariousConfigurations(t *testing.T) {
	testCases := []struct {
		name        string
		cfg         *config.RedisConfig
		shouldError bool
	}{
		{
			name: "disabled with all fields",
			cfg: &config.RedisConfig{
				Enabled:  false,
				Host:     "redis.example.com",
				Port:     6380,
				Password: "password123",
				Database: 5,
			},
			shouldError: false,
		},
		{
			name: "disabled minimal",
			cfg: &config.RedisConfig{
				Enabled: false,
			},
			shouldError: false,
		},
		{
			name: "disabled with zero port",
			cfg: &config.RedisConfig{
				Enabled: false,
				Host:    "localhost",
				Port:    0,
			},
			shouldError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client, err := NewClient(tc.cfg)
			if tc.shouldError {
				assert.Error(t, err)
				assert.Nil(t, client)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, client)
				assert.False(t, client.IsEnabled())
			}
		})
	}
}
