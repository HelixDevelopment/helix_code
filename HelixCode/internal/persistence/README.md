# Persistence Package

The `persistence` package provides data persistence abstraction for the HelixCode platform.

## Overview

This package handles:
- Abstract storage interface
- Multiple backend support (file, database, cloud)
- Serialization/deserialization
- Caching layer
- Data versioning

## Key Types

### Store

```go
type Store interface {
    Get(ctx context.Context, key string) ([]byte, error)
    Set(ctx context.Context, key string, value []byte) error
    Delete(ctx context.Context, key string) error
    List(ctx context.Context, prefix string) ([]string, error)
}
```

### Repository

```go
type Repository[T any] struct {
    store      Store
    serializer Serializer
}
```

## Usage

### Creating a Store

```go
import "dev.helix.code/internal/persistence"

// File-based store
store := persistence.NewFileStore("/data")

// Database store
store := persistence.NewDatabaseStore(db)

// Redis store
store := persistence.NewRedisStore(redisClient)
```

### Using Repository

```go
repo := persistence.NewRepository[User](store)

// Save
err := repo.Save(ctx, "user:123", user)

// Get
user, err := repo.Get(ctx, "user:123")

// Delete
err := repo.Delete(ctx, "user:123")
```

## Configuration

```yaml
persistence:
  backend: "database"
  cache:
    enabled: true
    ttl: 5m
```

## Testing

```bash
go test -v ./internal/persistence/...
```
