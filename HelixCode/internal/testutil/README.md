# testutil Package

The `testutil` package provides comprehensive testing utilities for the HelixCode project. It offers helper functions and utilities for testing various components, including database connections, Redis clients, LLM providers, browser automation, and infrastructure availability detection.

## Table of Contents

- [Overview](#overview)
- [Infrastructure Detection](#infrastructure-detection)
- [Skip Helpers](#skip-helpers)
- [Configuration Helpers](#configuration-helpers)
- [Database and Redis Clients](#database-and-redis-clients)
- [URL Getters](#url-getters)
- [Data Cleanup](#data-cleanup)
- [Environment Variables](#environment-variables)
- [Full Test Infrastructure](#full-test-infrastructure)
- [Usage Examples](#usage-examples)
- [Best Practices](#best-practices)

## Overview

The testutil package serves as the foundation for all integration and end-to-end testing in HelixCode. It provides:

- **Infrastructure Detection**: Functions to check if external services (PostgreSQL, Redis, Ollama, etc.) are available
- **Skip Helpers**: Graceful test skipping when required infrastructure is unavailable
- **Configuration Helpers**: Pre-configured settings for test databases, Redis, SSH servers, and more
- **Client Creation**: Automatic creation of test clients with proper cleanup
- **Data Management**: Utilities for cleaning up test data between test runs

This design allows tests to run in various environments - from minimal local setups to full CI/CD pipelines with complete infrastructure.

## Infrastructure Detection

The package provides boolean functions to detect available test infrastructure. These functions check environment variables to determine service availability.

### Available Detection Functions

| Function | Description | Environment Variables Checked |
|----------|-------------|------------------------------|
| `TestInfrastructureAvailable()` | Full test infrastructure | `HELIX_TEST_INFRA` |
| `DatabaseAvailable()` | PostgreSQL database | `HELIX_TEST_DATABASE_HOST`, `HELIX_DATABASE_HOST` |
| `RedisAvailable()` | Redis cache | `HELIX_TEST_REDIS_HOST`, `HELIX_REDIS_HOST` |
| `OllamaAvailable()` | Ollama LLM server | `HELIX_TEST_OLLAMA_URL`, `OLLAMA_HOST` |
| `MockLLMAvailable()` | Mock LLM server | `HELIX_TEST_MOCK_LLM_URL` |
| `SSHServerAvailable()` | SSH test server | `HELIX_TEST_SSH_HOST` |
| `BrowserAvailable()` | Selenium/ChromeDP | `HELIX_TEST_SELENIUM_URL`, `HELIX_TEST_CHROMEDP_URL` |
| `CogneeAvailable()` | Cognee service | `HELIX_TEST_COGNEE_URL` |
| `VectorDBAvailable()` | Vector databases | `HELIX_TEST_WEAVIATE_URL`, `HELIX_TEST_CHROMADB_URL`, `HELIX_TEST_QDRANT_URL` |

### Example Usage

```go
func TestConditionalFeature(t *testing.T) {
    if testutil.DatabaseAvailable() && testutil.RedisAvailable() {
        // Run full integration test
        runFullIntegrationTest(t)
    } else if testutil.DatabaseAvailable() {
        // Run database-only test
        runDatabaseTest(t)
    } else {
        // Run in-memory test
        runInMemoryTest(t)
    }
}
```

## Skip Helpers

Skip helpers provide a clean way to skip tests when required infrastructure is unavailable. They use `t.Helper()` for proper test output and include descriptive skip messages.

### Available Skip Functions

| Function | Skips When |
|----------|------------|
| `SkipIfNoInfrastructure(t)` | Full test infrastructure unavailable |
| `SkipIfNoDatabase(t)` | PostgreSQL unavailable |
| `SkipIfNoRedis(t)` | Redis unavailable |
| `SkipIfNoOllama(t)` | Ollama unavailable |
| `SkipIfNoMockLLM(t)` | Mock LLM server unavailable |
| `SkipIfNoSSH(t)` | SSH test server unavailable |
| `SkipIfNoBrowser(t)` | Browser automation unavailable |
| `SkipIfNoCognee(t)` | Cognee service unavailable |
| `SkipIfNoVectorDB(t)` | Vector database unavailable |

### Example Usage

```go
func TestDatabaseFeature(t *testing.T) {
    testutil.SkipIfNoDatabase(t)

    // This code only runs if database is available
    db := testutil.GetTestDatabase(t)
    // ... test database operations
}

func TestRedisCache(t *testing.T) {
    testutil.SkipIfNoRedis(t)

    // This code only runs if Redis is available
    client := testutil.GetTestRedis(t)
    // ... test Redis operations
}
```

## Configuration Helpers

Configuration helpers return properly configured structs for connecting to test infrastructure.

### GetTestDatabaseConfig

Returns a `database.Config` struct with test database settings:

```go
func GetTestDatabaseConfig() database.Config
```

**Default Values:**
- Host: `localhost` (or from environment)
- Port: `5432`
- User: `helixcode`
- Password: `helixcode_test`
- DBName: `helixcode_test`
- SSLMode: `disable`

### GetTestRedisConfig

Returns a `*config.RedisConfig` struct with test Redis settings:

```go
func GetTestRedisConfig() *config.RedisConfig
```

**Default Values:**
- Enabled: `true`
- Host: `localhost` (or from environment)
- Port: `6379`
- Database: `0`

### GetSSHTestConfig

Returns SSH test server configuration as multiple return values:

```go
func GetSSHTestConfig() (host string, port int, user string, password string, keyPath string)
```

**Default Values:**
- Host: `localhost`
- Port: `2222`
- User: `helixcode`
- Password: `helixcode_test`

## Database and Redis Clients

These functions create test clients with automatic cleanup via `t.Cleanup()`.

### GetTestDatabase

Creates a test database connection with automatic schema initialization and cleanup:

```go
func GetTestDatabase(t *testing.T) *database.Database
```

**Features:**
- Automatically skips test if database unavailable
- Initializes database schema
- Registers cleanup function to close connection
- Uses `t.Helper()` for proper test output

**Example:**

```go
func TestProjectManager(t *testing.T) {
    db := testutil.GetTestDatabase(t)

    manager := project.NewDatabaseManager(db)

    proj, err := manager.CreateProject(context.Background(), "test", "description", "/path", "go")
    require.NoError(t, err)
    assert.NotEmpty(t, proj.ID)
}
```

### GetTestRedis

Creates a test Redis connection with automatic cleanup:

```go
func GetTestRedis(t *testing.T) *redis.Client
```

**Features:**
- Automatically skips test if Redis unavailable
- Registers cleanup function to close connection
- Uses `t.Helper()` for proper test output

**Example:**

```go
func TestCacheOperations(t *testing.T) {
    client := testutil.GetTestRedis(t)

    ctx := context.Background()
    err := client.Set(ctx, "test-key", "test-value", time.Minute)
    require.NoError(t, err)

    value, err := client.Get(ctx, "test-key")
    require.NoError(t, err)
    assert.Equal(t, "test-value", value)
}
```

## URL Getters

These functions return URLs for various test services with sensible defaults.

| Function | Default URL | Environment Variable |
|----------|-------------|---------------------|
| `GetOllamaURL()` | `http://localhost:11434` | `HELIX_TEST_OLLAMA_URL`, `OLLAMA_HOST` |
| `GetMockLLMURL()` | `http://localhost:8090` | `HELIX_TEST_MOCK_LLM_URL` |
| `GetSeleniumURL()` | `http://localhost:4444` | `HELIX_TEST_SELENIUM_URL` |
| `GetChromeDPURL()` | `http://localhost:9222` | `HELIX_TEST_CHROMEDP_URL` |
| `GetCogneeURL()` | `http://localhost:8000` | `HELIX_TEST_COGNEE_URL` |

### Example Usage

```go
func TestOllamaIntegration(t *testing.T) {
    testutil.SkipIfNoOllama(t)

    ollamaURL := testutil.GetOllamaURL()
    provider := ollama.NewProvider(ollamaURL)

    // ... test LLM operations
}
```

## Data Cleanup

### CleanupTestData

Truncates test tables in the correct dependency order:

```go
func CleanupTestData(t *testing.T, db *database.Database)
```

**Tables Cleaned (in order):**
1. `checkpoints`
2. `task_dependencies`
3. `tasks`
4. `sessions`
5. `workers`
6. `projects`
7. `users`

**Example:**

```go
func TestWithCleanSlate(t *testing.T) {
    db := testutil.GetTestDatabase(t)

    // Clean up any existing data
    testutil.CleanupTestData(t, db)

    // Run test with clean database
    // ...
}
```

## Environment Variables

The package reads the following environment variables:

### Infrastructure Control

| Variable | Description |
|----------|-------------|
| `HELIX_TEST_INFRA` | Set to `"true"` to enable full test infrastructure |

### Database Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `HELIX_DATABASE_HOST` | `localhost` | PostgreSQL host |
| `HELIX_DATABASE_PORT` | `5432` | PostgreSQL port |
| `HELIX_DATABASE_USER` | `helixcode` | Database user |
| `HELIX_DATABASE_PASSWORD` | `helixcode_test` | Database password |
| `HELIX_DATABASE_NAME` | `helixcode_test` | Database name |

### Redis Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `HELIX_REDIS_HOST` | `localhost` | Redis host |
| `HELIX_REDIS_PORT` | `6379` | Redis port |
| `HELIX_REDIS_PASSWORD` | (empty) | Redis password |

### Service URLs

| Variable | Default | Description |
|----------|---------|-------------|
| `HELIX_TEST_OLLAMA_URL` | `http://localhost:11434` | Ollama URL |
| `HELIX_TEST_MOCK_LLM_URL` | `http://localhost:8090` | Mock LLM server URL |
| `HELIX_TEST_SELENIUM_URL` | `http://localhost:4444` | Selenium WebDriver URL |
| `HELIX_TEST_CHROMEDP_URL` | `http://localhost:9222` | ChromeDP URL |
| `HELIX_TEST_COGNEE_URL` | `http://localhost:8000` | Cognee service URL |

### SSH Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `HELIX_TEST_SSH_HOST` | `localhost` | SSH test server host |
| `HELIX_TEST_SSH_PORT` | `2222` | SSH test server port |
| `HELIX_TEST_SSH_USER` | `helixcode` | SSH test user |
| `HELIX_TEST_SSH_PASSWORD` | `helixcode_test` | SSH test password |
| `HELIX_TEST_SSH_KEY_PATH` | (empty) | Path to SSH key file |

### Vector Databases

| Variable | Description |
|----------|-------------|
| `HELIX_TEST_WEAVIATE_URL` | Weaviate vector DB URL |
| `HELIX_TEST_CHROMADB_URL` | ChromaDB URL |
| `HELIX_TEST_QDRANT_URL` | Qdrant URL |

## Full Test Infrastructure

To run tests with full infrastructure, use the provided Docker Compose setup:

```bash
# Start all test containers
make test-infra-up

# Run tests with full infrastructure
make test-full

# Stop test containers
make test-infra-down
```

The `docker-compose.full-test.yml` provides:
- PostgreSQL database
- Redis cache
- Ollama LLM server
- Mock LLM server (supports multiple providers)
- Selenium Chrome
- ChromeDP headless browser
- SSH test servers
- Cognee memory service
- Vector databases (Weaviate, ChromaDB, Qdrant)

## Usage Examples

### Basic Test with Database

```go
package mypackage_test

import (
    "context"
    "testing"

    "dev.helix.code/internal/testutil"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestCreateUser(t *testing.T) {
    // Skip if database unavailable
    testutil.SkipIfNoDatabase(t)

    // Get database with auto-cleanup
    db := testutil.GetTestDatabase(t)

    // Clean slate for test
    testutil.CleanupTestData(t, db)

    // Create user
    ctx := context.Background()
    user, err := db.CreateUser(ctx, "testuser", "test@example.com")

    require.NoError(t, err)
    assert.Equal(t, "testuser", user.Username)
}
```

### Test with Multiple Infrastructure Dependencies

```go
func TestSessionManagement(t *testing.T) {
    // Require both database and Redis
    testutil.SkipIfNoDatabase(t)
    testutil.SkipIfNoRedis(t)

    db := testutil.GetTestDatabase(t)
    redis := testutil.GetTestRedis(t)

    sessionManager := session.NewManager(db, redis)

    ctx := context.Background()
    sess, err := sessionManager.Create(ctx, "user-123")

    require.NoError(t, err)
    assert.NotEmpty(t, sess.Token)
}
```

### Test with LLM Integration

```go
func TestLLMGeneration(t *testing.T) {
    testutil.SkipIfNoOllama(t)

    ollamaURL := testutil.GetOllamaURL()
    provider := ollama.NewProvider(ollamaURL)

    ctx := context.Background()
    response, err := provider.Generate(ctx, &llm.Request{
        Prompt: "Hello, world!",
        Model:  "llama2",
    })

    require.NoError(t, err)
    assert.NotEmpty(t, response.Content)
}
```

### Test with SSH Worker

```go
func TestSSHWorker(t *testing.T) {
    testutil.SkipIfNoSSH(t)

    host, port, user, password, keyPath := testutil.GetSSHTestConfig()

    workerConfig := worker.Config{
        Host:     host,
        Port:     port,
        User:     user,
        Password: password,
        KeyPath:  keyPath,
    }

    w, err := worker.NewSSHWorker(workerConfig)
    require.NoError(t, err)
    defer w.Close()

    // Test worker operations
    output, err := w.Execute(context.Background(), "echo 'test'")
    require.NoError(t, err)
    assert.Contains(t, output, "test")
}
```

## Best Practices

### 1. Always Use Skip Helpers First

Place skip helpers at the beginning of tests to fail fast when infrastructure is unavailable:

```go
func TestFeature(t *testing.T) {
    testutil.SkipIfNoDatabase(t)  // First line

    // Rest of test...
}
```

### 2. Use t.Cleanup for Resources

The `GetTestDatabase` and `GetTestRedis` functions automatically register cleanup. For custom resources, use `t.Cleanup`:

```go
func TestWithCustomResource(t *testing.T) {
    resource := createResource()
    t.Cleanup(func() {
        resource.Close()
    })
}
```

### 3. Clean Data Between Tests

For tests that depend on a clean database state, use `CleanupTestData`:

```go
func TestIsolated(t *testing.T) {
    db := testutil.GetTestDatabase(t)
    testutil.CleanupTestData(t, db)

    // Test runs with clean database
}
```

### 4. Use Subtests for Related Tests

Group related tests using subtests to share setup:

```go
func TestUserOperations(t *testing.T) {
    testutil.SkipIfNoDatabase(t)
    db := testutil.GetTestDatabase(t)

    t.Run("Create", func(t *testing.T) {
        // Test create
    })

    t.Run("Update", func(t *testing.T) {
        // Test update
    })

    t.Run("Delete", func(t *testing.T) {
        // Test delete
    })
}
```

### 5. Use Table-Driven Tests

Combine with table-driven tests for comprehensive coverage:

```go
func TestValidation(t *testing.T) {
    testutil.SkipIfNoDatabase(t)
    db := testutil.GetTestDatabase(t)

    tests := []struct {
        name    string
        input   string
        wantErr bool
    }{
        {"valid", "test@example.com", false},
        {"invalid", "invalid-email", true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := validate(db, tt.input)
            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

### 6. Prefer Mock LLM for Unit Tests

For unit tests, use the mock LLM server to avoid slow network calls:

```go
func TestLLMHandler(t *testing.T) {
    testutil.SkipIfNoMockLLM(t)

    mockURL := testutil.GetMockLLMURL()
    // Use mock server for fast, deterministic tests
}
```

### 7. Document Infrastructure Requirements

Add comments explaining infrastructure requirements:

```go
// TestFullWorkflow requires database, Redis, and Ollama.
// Run with: make test-infra-up && go test -v ./...
func TestFullWorkflow(t *testing.T) {
    testutil.SkipIfNoDatabase(t)
    testutil.SkipIfNoRedis(t)
    testutil.SkipIfNoOllama(t)

    // ...
}
```

### 8. Use Parallel Tests Carefully

When running parallel tests, ensure they use isolated data:

```go
func TestParallel(t *testing.T) {
    testutil.SkipIfNoDatabase(t)
    db := testutil.GetTestDatabase(t)

    t.Run("Test1", func(t *testing.T) {
        t.Parallel()
        // Use unique identifiers to avoid conflicts
    })

    t.Run("Test2", func(t *testing.T) {
        t.Parallel()
        // Use unique identifiers to avoid conflicts
    })
}
```
