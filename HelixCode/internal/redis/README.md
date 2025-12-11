# Redis Package

The `redis` package provides Redis client and caching functionality for the HelixCode platform.

## Overview

This package handles:
- Redis connection management
- Caching operations
- Pub/Sub messaging
- Real-time state management
- Distributed locks

## Key Types

### Client

```go
type Client struct {
    client *redis.Client
    config *Config
    logger *logging.Logger
}
```

### Config

```go
type Config struct {
    Enabled     bool
    Host        string
    Port        int
    Password    string
    DB          int
    MaxRetries  int
    PoolSize    int
    DialTimeout time.Duration
}
```

## Usage

### Creating the Client

```go
import redisclient "dev.helix.code/internal/redis"

config := &redisclient.Config{
    Enabled:  true,
    Host:     "localhost",
    Port:     6379,
    Password: os.Getenv("REDIS_PASSWORD"),
    DB:       0,
    PoolSize: 10,
}

client, err := redisclient.New(config)
if err != nil {
    log.Fatal(err)
}
defer client.Close()
```

### Basic Operations

```go
// Set value
err := client.Set(ctx, "key", "value", 1*time.Hour)

// Get value
val, err := client.Get(ctx, "key")

// Delete key
err := client.Delete(ctx, "key")

// Check existence
exists, err := client.Exists(ctx, "key")
```

### Caching

```go
// Cache with TTL
err := client.Cache(ctx, "user:123", userData, 30*time.Minute)

// Get from cache
var user User
err := client.GetCached(ctx, "user:123", &user)

// Cache with fetch function
user, err := client.CacheOrFetch(ctx, "user:123", 30*time.Minute, func() (interface{}, error) {
    return db.GetUser(ctx, 123)
})

// Invalidate cache
err := client.Invalidate(ctx, "user:123")
```

### Pub/Sub

```go
// Subscribe to channel
messages := client.Subscribe(ctx, "tasks")
for msg := range messages {
    log.Info("Received: %s", msg.Payload)
}

// Publish message
err := client.Publish(ctx, "tasks", "task:123:completed")
```

### Distributed Locks

```go
// Acquire lock
lock, err := client.AcquireLock(ctx, "resource:123", 10*time.Second)
if err != nil {
    return err
}
defer lock.Release(ctx)

// Do work with lock
```

### Hash Operations

```go
// Set hash field
err := client.HSet(ctx, "user:123", "name", "John")

// Get hash field
name, err := client.HGet(ctx, "user:123", "name")

// Get all hash fields
fields, err := client.HGetAll(ctx, "user:123")
```

### List Operations

```go
// Push to list
err := client.LPush(ctx, "queue:tasks", taskID)

// Pop from list
taskID, err := client.RPop(ctx, "queue:tasks")

// Get list length
length, err := client.LLen(ctx, "queue:tasks")
```

### Sorted Sets

```go
// Add to sorted set
err := client.ZAdd(ctx, "leaderboard", score, member)

// Get top N
members, err := client.ZRevRange(ctx, "leaderboard", 0, 9)

// Get rank
rank, err := client.ZRank(ctx, "leaderboard", member)
```

## Configuration

```yaml
redis:
  enabled: true
  host: "localhost"
  port: 6379
  password: "${REDIS_PASSWORD}"
  db: 0
  pool_size: 10
  dial_timeout: 5s
  read_timeout: 3s
  write_timeout: 3s
```

## Health Check

```go
// Check Redis connectivity
err := client.Ping(ctx)
if err != nil {
    log.Error("Redis unhealthy: %v", err)
}

// Get pool stats
stats := client.PoolStats()
```

## Testing

```bash
go test -v ./internal/redis/...
```

## Notes

- Redis is optional and can be disabled
- Use connection pooling for efficiency
- Set appropriate TTLs for cache entries
- Monitor memory usage for large datasets
