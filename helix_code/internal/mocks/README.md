# Mocks Package

The mocks package provides mock implementations of HelixCode interfaces for unit testing. These mocks use `testify/mock` for flexible behavior configuration and assertion verification.

## Overview

Mock implementations allow tests to:
- Isolate components from external dependencies
- Simulate various success and failure scenarios
- Verify method calls and arguments
- Control return values dynamically

## Available Mocks

### MockVectorProvider

A mock implementation of the `VectorProvider` interface for testing memory operations:

```go
type MockVectorProvider struct {
    mock.Mock
    // Internal state for realistic behavior
    store       map[string][]*memory.VectorData
    collections map[string]*memory.CollectionConfig
    indices     map[string]*memory.IndexInfo
    stats       *providers.ProviderStats
    healthy     bool
    initialized bool
    started     bool
}
```

## Usage

### Creating a Mock

```go
import (
    "testing"
    "github.com/stretchr/testify/mock"
    "dev.helix.code/internal/mocks"
)

func TestMyComponent(t *testing.T) {
    mockProvider := mocks.NewMockVectorProvider(t)
    // Use mockProvider in your tests
}
```

### Setting Up Expectations

```go
// Expect Store to be called with any context and vectors
mockProvider.On("Store", mock.Anything, mock.Anything).Return(nil)

// Expect specific arguments
mockProvider.On("Search", mock.Anything, &memory.SearchQuery{
    Query: "test query",
    TopK:  10,
}).Return(results, nil)

// Return an error
mockProvider.On("Delete", mock.Anything, mock.Anything).
    Return(errors.New("deletion failed"))
```

### Verifying Calls

```go
// Verify method was called
mockProvider.AssertCalled(t, "Store", mock.Anything, expectedVectors)

// Verify call count
mockProvider.AssertNumberOfCalls(t, "Store", 2)

// Verify all expectations were met
mockProvider.AssertExpectations(t)
```

## Mock Implementations

### Store

Stores vectors in internal state and updates statistics:

```go
func (m *MockVectorProvider) Store(ctx context.Context, vectors []*memory.VectorData) error
```

### Retrieve

Retrieves vectors by ID from internal state:

```go
func (m *MockVectorProvider) Retrieve(ctx context.Context, ids []string) ([]*memory.VectorData, error)
```

### Search

Searches vectors using the configured mock response:

```go
func (m *MockVectorProvider) Search(ctx context.Context, query *memory.SearchQuery) ([]*memory.SearchResult, error)
```

### Health

Returns configured health status:

```go
func (m *MockVectorProvider) Health(ctx context.Context) (*providers.HealthStatus, error)
```

## Test Patterns

### Basic Test

```go
func TestStoreVectors(t *testing.T) {
    mock := mocks.NewMockVectorProvider(t)

    // Setup expectation
    mock.On("Store", mock.Anything, mock.Anything).Return(nil)

    // Create component with mock
    service := NewVectorService(mock)

    // Execute
    err := service.AddVectors(ctx, vectors)

    // Assert
    assert.NoError(t, err)
    mock.AssertExpectations(t)
}
```

### Testing Error Handling

```go
func TestStoreVectors_Error(t *testing.T) {
    mock := mocks.NewMockVectorProvider(t)

    // Return error
    mock.On("Store", mock.Anything, mock.Anything).
        Return(errors.New("database unavailable"))

    service := NewVectorService(mock)
    err := service.AddVectors(ctx, vectors)

    assert.Error(t, err)
    assert.Contains(t, err.Error(), "database unavailable")
}
```

### Using Internal State

The mock maintains internal state for more realistic testing:

```go
func TestRetrieveStoredVectors(t *testing.T) {
    mock := mocks.NewMockVectorProvider(t)

    // Store vectors (uses internal state)
    mock.On("Store", mock.Anything, mock.Anything).Return(nil)
    mock.Store(ctx, vectors)

    // Retrieve should find them
    mock.On("Retrieve", mock.Anything, mock.Anything).Return(vectors, nil)
    result, err := mock.Retrieve(ctx, []string{"id1", "id2"})

    assert.NoError(t, err)
    assert.Len(t, result, 2)
}
```

## Dependencies

- `github.com/stretchr/testify/mock` - Mock framework
- `github.com/stretchr/testify/suite` - Test suites
- Internal packages: `llm`, `logging`, `memory`, `memory/providers`

## Best Practices

1. **Always verify expectations**: Call `AssertExpectations(t)` at the end of tests
2. **Use specific matchers when possible**: Prefer exact values over `mock.Anything`
3. **Test error paths**: Mock error returns to test error handling
4. **Reset between tests**: Create new mock instances for each test
5. **Document mock behavior**: Comment complex mock setups
