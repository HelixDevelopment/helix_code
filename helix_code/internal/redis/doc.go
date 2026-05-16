// Package redis provides Redis client integration for HelixCode.
//
// This package implements a Redis client wrapper that provides caching,
// pub/sub messaging, and data structure operations. It supports graceful
// degradation when Redis is disabled, making it optional for deployments
// that don't require caching or real-time features.
//
// # Architecture
//
// The package wraps the go-redis client with:
//
//   - Configuration-based initialization
//   - Graceful handling when disabled
//   - Simplified API for common operations
//   - Connection health checking
//
// # Client Creation
//
// Create a Redis client from configuration:
//
//	cfg := &config.RedisConfig{
//	    Enabled:  true,
//	    Host:     "localhost",
//	    Port:     6379,
//	    Password: "",
//	    Database: 0,
//	}
//
//	client, err := redis.NewClient(cfg)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer client.Close()
//
// # Disabled Mode
//
// When Redis is disabled, operations are handled gracefully:
//
//	cfg := &config.RedisConfig{Enabled: false}
//	client, _ := redis.NewClient(cfg)
//
//	// Write operations return nil (no-op)
//	err := client.Set(ctx, "key", "value", time.Hour) // err = nil
//
//	// Read operations return appropriate errors
//	_, err = client.Get(ctx, "key") // err = "Redis is disabled"
//
// # Key-Value Operations
//
// Basic string operations:
//
//	// Set with expiration
//	err := client.Set(ctx, "session:123", userData, 24*time.Hour)
//
//	// Get value
//	value, err := client.Get(ctx, "session:123")
//
//	// Delete keys
//	err = client.Del(ctx, "session:123", "session:456")
//
//	// Check existence
//	count, err := client.Exists(ctx, "session:123")
//
//	// Set/get expiration
//	err = client.Expire(ctx, "key", 1*time.Hour)
//	ttl, err := client.TTL(ctx, "key")
//
// # Hash Operations
//
// Hash maps for structured data:
//
//	// Set hash fields
//	err := client.HSet(ctx, "user:123", "name", "John", "email", "john@example.com")
//
//	// Get single field
//	name, err := client.HGet(ctx, "user:123", "name")
//
//	// Get all fields
//	fields, err := client.HGetAll(ctx, "user:123")
//
//	// Delete fields
//	err = client.HDel(ctx, "user:123", "email")
//
// # List Operations
//
// List data structures:
//
//	// Push to list
//	err := client.LPush(ctx, "queue", "task1", "task2")
//
//	// Pop from list
//	task, err := client.RPop(ctx, "queue")
//
//	// Blocking pop with timeout
//	items, err := client.BRPop(ctx, 30*time.Second, "queue")
//
//	// Get list length
//	length, err := client.LLen(ctx, "queue")
//
// # Set Operations
//
// Unordered sets:
//
//	// Add members
//	err := client.SAdd(ctx, "tags", "go", "redis", "api")
//
//	// Get all members
//	members, err := client.SMembers(ctx, "tags")
//
//	// Remove members
//	err = client.SRem(ctx, "tags", "api")
//
// # Sorted Set Operations
//
// Ordered sets with scores:
//
//	// Add members with scores
//	err := client.ZAdd(ctx, "leaderboard",
//	    &redis.Z{Score: 100, Member: "player1"},
//	    &redis.Z{Score: 200, Member: "player2"},
//	)
//
//	// Get range
//	members, err := client.ZRange(ctx, "leaderboard", 0, 9)
//
//	// Remove members
//	err = client.ZRem(ctx, "leaderboard", "player1")
//
// # Pub/Sub Messaging
//
// Real-time messaging:
//
//	// Publish a message
//	err := client.Publish(ctx, "notifications", message)
//
//	// Subscribe to channels
//	pubsub := client.Subscribe(ctx, "notifications", "events")
//	if pubsub != nil {
//	    ch := pubsub.Channel()
//	    for msg := range ch {
//	        fmt.Printf("Received on %s: %s\n", msg.Channel, msg.Payload)
//	    }
//	}
//
// # Underlying Client Access
//
// For advanced operations, access the underlying go-redis client:
//
//	underlying := client.GetClient()
//	if underlying != nil {
//	    // Use go-redis methods directly
//	    result := underlying.Incr(ctx, "counter")
//	}
//
// # Status Checking
//
// Check if Redis is enabled:
//
//	if client.IsEnabled() {
//	    // Perform Redis operations
//	} else {
//	    // Use fallback logic
//	}
//
// # Connection Timeout
//
// The client uses a 5-second timeout for initial connection.
// If the connection fails, NewClient returns an error.
//
// # Use Cases in HelixCode
//
// The Redis client supports:
//
//   - Session caching for faster authentication
//   - Rate limiting token storage
//   - Real-time task status updates via pub/sub
//   - Temporary result caching for LLM responses
//   - Queue management for background tasks
//   - Leaderboard/scoring for worker performance
//
// # Configuration
//
// Redis configuration from config/config.yaml:
//
//	redis:
//	  enabled: true
//	  host: localhost
//	  port: 6379
//	  password: ""
//	  database: 0
//
// Environment variable override:
//
//	HELIX_REDIS_PASSWORD=secret
//
// # Thread Safety
//
// The go-redis client is thread-safe. Multiple goroutines can
// share the same Client instance for concurrent operations.
//
// # Error Handling
//
// When Redis is disabled:
//   - Write operations (Set, Del, etc.) return nil
//   - Read operations return "Redis is disabled" error
//   - Subscribe returns nil
//
// This allows code to work without special handling when Redis is unavailable.
package redis
