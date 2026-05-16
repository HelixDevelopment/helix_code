# HelixCode Testing Infrastructure

This document describes the complete testing infrastructure for HelixCode, including unit tests, integration tests, and end-to-end tests with real services.

## Test Categories

### 1. Unit Tests
Unit tests run without external dependencies and use in-memory implementations.

```bash
# Run all unit tests
go test ./...

# Run with coverage
go test -cover ./...

# Run specific package
go test -v ./internal/memory/...
```

### 2. Integration Tests
Integration tests require real services (database, Redis, Memcached, etc.) and use the `integration` build tag.

```bash
# Start test infrastructure
docker compose -f docker-compose.test.yml up -d

# Run integration tests
go test -tags=integration -v ./...

# Run specific integration tests
go test -tags=integration -v ./internal/memory/...

# Stop infrastructure
docker compose -f docker-compose.test.yml down -v
```

### 3. End-to-End Tests
E2E tests validate complete workflows using the challenge framework.

```bash
cd tests/e2e/challenges
go run cmd/runner/main.go -list
go run cmd/runner/main.go -challenge notes-project-001
```

## Test Infrastructure Services

### Docker Compose Configuration

File: `docker-compose.test.yml`

| Service | Port | Purpose |
|---------|------|---------|
| PostgreSQL | 5433 | Database integration tests |
| Redis | 6380 | Cache and memory provider tests |
| Memcached | 11212 | Memory provider tests |
| ChromaDB | 8002 | Vector storage tests |
| Qdrant | 6333/6334 | Vector database tests |
| Ollama | 11434 | Local LLM tests |
| Cognee | 8001 | Knowledge graph tests |
| Prometheus | 9091 | Monitoring tests (optional) |
| Grafana | 3001 | Dashboard tests (optional) |

### Starting the Infrastructure

```bash
# Start core services (postgres, redis, memcached)
docker compose -f docker-compose.test.yml up -d postgres-test redis-test memcached-test

# Start all services
docker compose -f docker-compose.test.yml up -d

# Start with monitoring
docker compose -f docker-compose.test.yml --profile monitoring up -d

# Verify services are healthy
docker compose -f docker-compose.test.yml ps
```

### Stopping the Infrastructure

```bash
# Stop and remove containers
docker compose -f docker-compose.test.yml down

# Stop and remove volumes (clean slate)
docker compose -f docker-compose.test.yml down -v
```

## Environment Variables

Integration tests use environment variables for service configuration:

### Database
```bash
export POSTGRES_HOST=localhost
export POSTGRES_PORT=5433
export POSTGRES_USER=helix_test
export POSTGRES_PASSWORD=test_password_secure_123
export POSTGRES_DB=helix_test
```

### Redis
```bash
export REDIS_HOST=localhost
export REDIS_PORT=6380
export REDIS_PASSWORD=test_redis_password_123
```

### Memcached
```bash
export MEMCACHED_HOST=localhost
export MEMCACHED_PORT=11212
```

### Ollama
```bash
export OLLAMA_HOST=http://localhost:11434
```

## Running Tests

### Complete Test Suite

```bash
#!/bin/bash
# run_all_tests.sh

# Start infrastructure
docker compose -f docker-compose.test.yml up -d

# Wait for services to be healthy
echo "Waiting for services to start..."
sleep 10

# Set environment variables
export POSTGRES_HOST=localhost
export POSTGRES_PORT=5433
export POSTGRES_USER=helix_test
export POSTGRES_PASSWORD=test_password_secure_123
export POSTGRES_DB=helix_test
export REDIS_HOST=localhost
export REDIS_PORT=6380
export REDIS_PASSWORD=test_redis_password_123
export MEMCACHED_HOST=localhost
export MEMCACHED_PORT=11212

# Run unit tests
echo "Running unit tests..."
go test -cover ./...

# Run integration tests
echo "Running integration tests..."
go test -tags=integration -v ./...

# Cleanup
docker compose -f docker-compose.test.yml down -v
```

### Test-Specific Commands

```bash
# Memory provider tests (unit)
go test -v ./internal/memory/...

# Memory provider tests (integration with real services)
go test -tags=integration -v ./internal/memory/...

# Database tests
go test -v ./internal/database/...

# API/Server tests
go test -v ./internal/server/...

# LLM integration tests
go test -v ./internal/llm/...

# Provider tests (AI integration)
go test -v ./internal/providers/...
```

## Test Coverage

### Coverage Report

```bash
# Generate coverage profile
go test -coverprofile=coverage.out ./...

# View coverage in browser
go tool cover -html=coverage.out

# Get coverage summary
go tool cover -func=coverage.out | tail -1
```

### Coverage Targets

| Package | Target | Current |
|---------|--------|---------|
| internal/memory | 100% | 67.8% |
| internal/providers | 100% | 53.4% |
| internal/llm | 100% | 42.0% |
| internal/server | 100% | 43.0% |
| internal/database | 100% | 73.8% |

## Writing Tests

### Unit Test Guidelines

1. Tests should be self-contained and not depend on external services
2. Use table-driven tests for multiple scenarios
3. Use `testify` assertions for cleaner test code
4. Mock external dependencies using interfaces

```go
func TestFeature(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
        wantErr  bool
    }{
        {"valid input", "test", "result", false},
        {"empty input", "", "", true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := Feature(tt.input)
            if tt.wantErr {
                require.Error(t, err)
                return
            }
            require.NoError(t, err)
            assert.Equal(t, tt.expected, result)
        })
    }
}
```

### Integration Test Guidelines

1. Use the `integration` build tag
2. Skip tests if services aren't available
3. Use unique prefixes to avoid test interference
4. Clean up test data after tests

```go
//go:build integration
// +build integration

func TestIntegration(t *testing.T) {
    // Check service availability
    if err := checkService(); err != nil {
        t.Skipf("Service not available: %v", err)
    }

    // Use unique prefix
    prefix := fmt.Sprintf("test:%d:", time.Now().UnixNano())

    // Run tests
    t.Run("Operation", func(t *testing.T) {
        // Test code
    })

    // Cleanup
    t.Cleanup(func() {
        // Clean up test data
    })
}
```

## Continuous Integration

### GitHub Actions Workflow

The CI pipeline runs tests automatically on push and pull requests:

1. **Unit Tests**: Run on every push
2. **Integration Tests**: Run with Docker services
3. **E2E Tests**: Run on main branch merges

See `.github/workflows/` for workflow definitions.

## Troubleshooting

### Service Connection Issues

```bash
# Check if services are running
docker compose -f docker-compose.test.yml ps

# View service logs
docker compose -f docker-compose.test.yml logs redis-test
docker compose -f docker-compose.test.yml logs postgres-test

# Test connectivity manually
redis-cli -h localhost -p 6380 -a test_redis_password_123 ping
psql -h localhost -p 5433 -U helix_test -d helix_test -c "SELECT 1"
```

### Test Isolation Issues

If tests are interfering with each other:

1. Use unique prefixes/namespaces per test
2. Run tests with `-p 1` to disable parallelism
3. Ensure cleanup runs after tests

```bash
go test -p 1 -v ./...
```

### Memory/Resource Issues

```bash
# Increase Docker resources if needed
docker system prune -f
docker volume prune -f
```

## Best Practices

1. **No Mock Data in Production**: Tests verify that no mock/fake data reaches production code paths
2. **Real Services**: Integration tests use real services via Docker
3. **Isolation**: Each test suite uses unique prefixes to prevent conflicts
4. **Cleanup**: Tests clean up after themselves
5. **Skippable**: Integration tests skip gracefully if services aren't available
6. **Documentation**: All test requirements are documented

## Related Files

- `docker-compose.test.yml` - Test infrastructure definition
- `run_tests.sh` - Unit test runner
- `run_all_tests.sh` - Complete test runner
- `run_integration_tests.sh` - Integration test runner
- `tests/e2e/` - End-to-end test framework
- `tests/e2e/challenges/` - Challenge-based E2E tests
