# Database Package

The `database` package provides PostgreSQL database connectivity and operations for the HelixCode platform.

## Overview

This package handles:
- PostgreSQL connection management
- Connection pooling
- Schema initialization
- Query execution
- Transaction management
- Migration support

## Key Types

### Database

The main database wrapper:

```go
type Database struct {
    pool   *pgxpool.Pool
    config *Config
    logger *logging.Logger
}
```

### Config

Database configuration:

```go
type Config struct {
    Host            string
    Port            int
    User            string
    Password        string
    Name            string
    SSLMode         string
    MaxConns        int32
    MinConns        int32
    MaxConnLifetime time.Duration
}
```

## Usage

### Creating a Database Connection

```go
import "dev.helix.code/internal/database"

config := &database.Config{
    Host:     "localhost",
    Port:     5432,
    User:     "helixcode",
    Password: os.Getenv("HELIX_DATABASE_PASSWORD"),
    Name:     "helixcode",
    SSLMode:  "disable",
}

db, err := database.New(config)
if err != nil {
    log.Fatal(err)
}
defer db.Close()
```

### Initializing Schema

```go
// Create tables automatically
err := db.InitializeSchema(ctx)
if err != nil {
    log.Fatal(err)
}
```

### Executing Queries

```go
// Query single row
var user User
err := db.QueryRow(ctx, "SELECT * FROM users WHERE id = $1", userID).Scan(&user)

// Query multiple rows
rows, err := db.Query(ctx, "SELECT * FROM projects WHERE user_id = $1", userID)
defer rows.Close()

// Execute command
_, err := db.Exec(ctx, "UPDATE users SET name = $1 WHERE id = $2", name, userID)
```

### Transactions

```go
tx, err := db.Begin(ctx)
if err != nil {
    return err
}
defer tx.Rollback(ctx)

// Execute operations
_, err = tx.Exec(ctx, "INSERT INTO ...")
if err != nil {
    return err
}

return tx.Commit(ctx)
```

## Connection Pooling

The package uses pgxpool for efficient connection management:

```go
config := &database.Config{
    MaxConns:        25,
    MinConns:        5,
    MaxConnLifetime: 30 * time.Minute,
}
```

## Health Checks

```go
// Check database connectivity
err := db.Ping(ctx)
if err != nil {
    log.Error("Database unhealthy")
}

// Get pool statistics
stats := db.Stats()
log.Info("Active connections: %d", stats.AcquiredConns())
```

## Database Schema

The package manages these core tables:

- `users` - User accounts
- `projects` - Development projects
- `sessions` - User sessions
- `workers` - Distributed workers
- `tasks` - Task queue
- `llm_providers` - LLM provider configurations
- `notifications` - Notification records

## Configuration via Environment

| Variable | Description | Default |
|----------|-------------|---------|
| HELIX_DATABASE_HOST | Database host | localhost |
| HELIX_DATABASE_PORT | Database port | 5432 |
| HELIX_DATABASE_USER | Database user | helixcode |
| HELIX_DATABASE_PASSWORD | Database password | (required) |
| HELIX_DATABASE_NAME | Database name | helixcode |

## Testing

```bash
go test -v ./internal/database/...
```

## Notes

- Database can be disabled for testing (leave host empty)
- Schema is auto-created on startup
- Use prepared statements for repeated queries
- Always close rows after iteration
