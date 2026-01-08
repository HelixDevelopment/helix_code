package providers

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========================================
// MockProvider for testing
// ========================================

type MockProvider struct {
	name           string
	providerType   string
	initialized    bool
	started        bool
	vectors        map[string]*VectorData
	collections    map[string]*CollectionInfo
	shouldFail     bool
	failOperations map[string]bool
}

func NewMockProvider(name string) *MockProvider {
	return &MockProvider{
		name:           name,
		providerType:   "mock",
		vectors:        make(map[string]*VectorData),
		collections:    make(map[string]*CollectionInfo),
		failOperations: make(map[string]bool),
	}
}

func (m *MockProvider) GetName() string               { return m.name }
func (m *MockProvider) GetType() string               { return m.providerType }
func (m *MockProvider) GetCapabilities() []string     { return []string{"store", "search", "retrieve"} }
func (m *MockProvider) GetConfiguration() interface{} { return nil }
func (m *MockProvider) IsCloud() bool                 { return false }
func (m *MockProvider) GetCostInfo() *CostInfo {
	return &CostInfo{Currency: "USD", TotalCost: 0.0}
}

func (m *MockProvider) Initialize(ctx context.Context, config interface{}) error {
	if m.shouldFail {
		return assert.AnError
	}
	m.initialized = true
	return nil
}

func (m *MockProvider) Start(ctx context.Context) error {
	if m.shouldFail {
		return assert.AnError
	}
	m.started = true
	return nil
}

func (m *MockProvider) Stop(ctx context.Context) error {
	m.started = false
	return nil
}

func (m *MockProvider) Close(ctx context.Context) error {
	return nil
}

func (m *MockProvider) Health(ctx context.Context) (*HealthStatus, error) {
	if m.failOperations["health"] {
		return nil, assert.AnError
	}
	return &HealthStatus{
		Status:    "healthy",
		Message:   "OK",
		Timestamp: time.Now(),
	}, nil
}

func (m *MockProvider) Store(ctx context.Context, vectors []*VectorData) error {
	if m.failOperations["store"] {
		return assert.AnError
	}
	for _, v := range vectors {
		m.vectors[v.ID] = v
	}
	return nil
}

func (m *MockProvider) Retrieve(ctx context.Context, ids []string) ([]*VectorData, error) {
	if m.failOperations["retrieve"] {
		return nil, assert.AnError
	}
	var result []*VectorData
	for _, id := range ids {
		if v, ok := m.vectors[id]; ok {
			result = append(result, v)
		}
	}
	return result, nil
}

func (m *MockProvider) Update(ctx context.Context, id string, vector *VectorData) error {
	m.vectors[id] = vector
	return nil
}

func (m *MockProvider) Delete(ctx context.Context, ids []string) error {
	for _, id := range ids {
		delete(m.vectors, id)
	}
	return nil
}

func (m *MockProvider) Search(ctx context.Context, query *VectorQuery) (*VectorSearchResult, error) {
	if m.failOperations["search"] {
		return nil, assert.AnError
	}
	return &VectorSearchResult{
		Results: []*VectorSearchResultItem{
			{ID: "result-1", Score: 0.95},
		},
		Total:    1,
		Duration: time.Millisecond * 10,
	}, nil
}

func (m *MockProvider) FindSimilar(ctx context.Context, embedding []float64, k int, filters map[string]interface{}) ([]*VectorSimilarityResult, error) {
	return []*VectorSimilarityResult{
		{ID: "similar-1", Score: 0.9},
	}, nil
}

func (m *MockProvider) BatchFindSimilar(ctx context.Context, queries [][]float64, k int) ([][]*VectorSimilarityResult, error) {
	return nil, nil
}

func (m *MockProvider) CreateCollection(ctx context.Context, name string, config *CollectionConfig) error {
	m.collections[name] = &CollectionInfo{Name: name, Dimension: config.Dimension}
	return nil
}

func (m *MockProvider) DeleteCollection(ctx context.Context, name string) error {
	delete(m.collections, name)
	return nil
}

func (m *MockProvider) ListCollections(ctx context.Context) ([]*CollectionInfo, error) {
	var result []*CollectionInfo
	for _, c := range m.collections {
		result = append(result, c)
	}
	return result, nil
}

func (m *MockProvider) GetCollection(ctx context.Context, name string) (*CollectionInfo, error) {
	if c, ok := m.collections[name]; ok {
		return c, nil
	}
	return nil, assert.AnError
}

func (m *MockProvider) CreateIndex(ctx context.Context, collection string, config *IndexConfig) error {
	return nil
}

func (m *MockProvider) DeleteIndex(ctx context.Context, collection, name string) error {
	return nil
}

func (m *MockProvider) ListIndexes(ctx context.Context, collection string) ([]*IndexInfo, error) {
	return []*IndexInfo{}, nil
}

func (m *MockProvider) AddMetadata(ctx context.Context, id string, metadata map[string]interface{}) error {
	if v, ok := m.vectors[id]; ok {
		if v.Metadata == nil {
			v.Metadata = make(map[string]interface{})
		}
		for k, val := range metadata {
			v.Metadata[k] = val
		}
	}
	return nil
}

func (m *MockProvider) UpdateMetadata(ctx context.Context, id string, metadata map[string]interface{}) error {
	return m.AddMetadata(ctx, id, metadata)
}

func (m *MockProvider) GetMetadata(ctx context.Context, ids []string) (map[string]map[string]interface{}, error) {
	result := make(map[string]map[string]interface{})
	for _, id := range ids {
		if v, ok := m.vectors[id]; ok {
			result[id] = v.Metadata
		}
	}
	return result, nil
}

func (m *MockProvider) DeleteMetadata(ctx context.Context, ids []string, keys []string) error {
	return nil
}

func (m *MockProvider) GetStats(ctx context.Context) (*ProviderStats, error) {
	return &ProviderStats{
		Name:         m.name,
		Type:         m.providerType,
		Status:       "active",
		TotalVectors: int64(len(m.vectors)),
	}, nil
}

func (m *MockProvider) Optimize(ctx context.Context) error {
	return nil
}

func (m *MockProvider) Backup(ctx context.Context, path string) error {
	return nil
}

func (m *MockProvider) Restore(ctx context.Context, path string) error {
	return nil
}

// ========================================
// ProviderFactory Tests
// ========================================

func TestNewProviderFactory(t *testing.T) {
	// Test with nil config
	factory := NewProviderFactory(nil)
	assert.NotNil(t, factory)
	assert.NotNil(t, factory.config)
	assert.Equal(t, int64(30), factory.config.DefaultTimeout)
	assert.True(t, factory.config.EnableValidation)
	assert.True(t, factory.config.EnableAutoConfig)
}

func TestNewProviderFactory_WithConfig(t *testing.T) {
	config := &FactoryConfig{
		DefaultTimeout:    60,
		EnableValidation:  false,
		EnableAutoConfig:  false,
		HealthCheckOnInit: false,
	}

	factory := NewProviderFactory(config)
	assert.NotNil(t, factory)
	assert.Equal(t, int64(60), factory.config.DefaultTimeout)
	assert.False(t, factory.config.EnableValidation)
}

func TestProviderFactory_GetDefaultConfiguration(t *testing.T) {
	factory := NewProviderFactory(nil)

	defaults := factory.getDefaultConfiguration(ProviderTypePinecone)
	assert.NotNil(t, defaults)
}

func TestProviderFactory_ApplyAutoConfiguration(t *testing.T) {
	factory := NewProviderFactory(&FactoryConfig{
		EnableAutoConfig: true,
		CustomConfigs: map[ProviderType]interface{}{
			ProviderTypePinecone: map[string]interface{}{
				"custom_setting": "value",
			},
		},
	})

	config := map[string]interface{}{
		"api_key": "test-key",
	}

	result := factory.applyAutoConfiguration(ProviderTypePinecone, config)
	assert.NotNil(t, result)
	assert.Equal(t, "test-key", result["api_key"])
	assert.Equal(t, "value", result["custom_setting"])
}

// ========================================
// MonitoredProvider Tests
// ========================================

func TestMonitoredProvider_RecordOperation(t *testing.T) {
	mock := NewMockProvider("test")
	monitored := &MonitoredProvider{
		provider: mock,
		metrics:  &ProviderMetrics{},
	}

	start := time.Now()
	monitored.recordOperation(start, nil)

	metrics := monitored.GetMetrics()
	assert.Equal(t, int64(1), metrics.TotalOperations)
	assert.Equal(t, int64(1), metrics.SuccessCount)
	assert.Equal(t, int64(0), metrics.FailureCount)
}

func TestMonitoredProvider_RecordOperationWithError(t *testing.T) {
	mock := NewMockProvider("test")
	monitored := &MonitoredProvider{
		provider: mock,
		metrics:  &ProviderMetrics{},
	}

	start := time.Now()
	monitored.recordOperation(start, assert.AnError)

	metrics := monitored.GetMetrics()
	assert.Equal(t, int64(1), metrics.TotalOperations)
	assert.Equal(t, int64(0), metrics.SuccessCount)
	assert.Equal(t, int64(1), metrics.FailureCount)
}

func TestMonitoredProvider_Store(t *testing.T) {
	mock := NewMockProvider("test")
	monitored := &MonitoredProvider{
		provider: mock,
		metrics:  &ProviderMetrics{},
	}

	ctx := context.Background()
	vectors := []*VectorData{{ID: "v1", Vector: []float64{0.1, 0.2}}}

	err := monitored.Store(ctx, vectors)
	require.NoError(t, err)

	metrics := monitored.GetMetrics()
	assert.Equal(t, int64(1), metrics.TotalOperations)
	assert.Equal(t, int64(1), metrics.SuccessCount)
}

func TestMonitoredProvider_Search(t *testing.T) {
	mock := NewMockProvider("test")
	monitored := &MonitoredProvider{
		provider: mock,
		metrics:  &ProviderMetrics{},
	}

	ctx := context.Background()
	query := &VectorQuery{Vector: []float64{0.1, 0.2}, TopK: 10}

	result, err := monitored.Search(ctx, query)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 1, len(result.Results))

	metrics := monitored.GetMetrics()
	assert.Equal(t, int64(1), metrics.TotalOperations)
}

func TestMonitoredProvider_Health(t *testing.T) {
	mock := NewMockProvider("test")
	monitored := &MonitoredProvider{
		provider: mock,
		metrics:  &ProviderMetrics{},
	}

	ctx := context.Background()
	health, err := monitored.Health(ctx)

	require.NoError(t, err)
	assert.Equal(t, "healthy", health.Status)
}

func TestMonitoredProvider_DelegateMethods(t *testing.T) {
	mock := NewMockProvider("test-provider")
	monitored := &MonitoredProvider{
		provider: mock,
		metrics:  &ProviderMetrics{},
	}

	assert.Equal(t, "test-provider", monitored.GetName())
	assert.Equal(t, "mock", monitored.GetType())
	assert.Contains(t, monitored.GetCapabilities(), "store")
	assert.False(t, monitored.IsCloud())
	assert.NotNil(t, monitored.GetCostInfo())
}

// ========================================
// ProviderChain Tests
// ========================================

func TestNewProviderChain(t *testing.T) {
	providers := []VectorProvider{
		NewMockProvider("provider1"),
		NewMockProvider("provider2"),
	}

	chain := NewProviderChain(providers)
	assert.NotNil(t, chain)
	assert.Equal(t, 2, len(chain.providers))
	assert.Equal(t, 0, chain.current)
}

func TestProviderChain_GetName(t *testing.T) {
	chain := NewProviderChain([]VectorProvider{NewMockProvider("p1")})
	assert.Equal(t, "provider_chain", chain.GetName())
}

func TestProviderChain_GetType(t *testing.T) {
	chain := NewProviderChain([]VectorProvider{NewMockProvider("p1")})
	assert.Equal(t, ProviderTypeAgnostic, chain.GetType())
}

func TestProviderChain_GetCapabilities(t *testing.T) {
	chain := NewProviderChain([]VectorProvider{
		NewMockProvider("p1"),
		NewMockProvider("p2"),
	})

	caps := chain.GetCapabilities()
	assert.NotEmpty(t, caps)
	assert.Contains(t, caps, "store")
}

func TestProviderChain_Initialize(t *testing.T) {
	chain := NewProviderChain([]VectorProvider{
		NewMockProvider("p1"),
		NewMockProvider("p2"),
	})

	ctx := context.Background()
	err := chain.Initialize(ctx, nil)
	require.NoError(t, err)
}

func TestProviderChain_Start(t *testing.T) {
	chain := NewProviderChain([]VectorProvider{
		NewMockProvider("p1"),
		NewMockProvider("p2"),
	})

	ctx := context.Background()
	err := chain.Start(ctx)
	require.NoError(t, err)
}

func TestProviderChain_Stop(t *testing.T) {
	chain := NewProviderChain([]VectorProvider{
		NewMockProvider("p1"),
	})

	ctx := context.Background()
	err := chain.Stop(ctx)
	require.NoError(t, err)
}

func TestProviderChain_Health(t *testing.T) {
	chain := NewProviderChain([]VectorProvider{
		NewMockProvider("p1"),
	})

	ctx := context.Background()
	health, err := chain.Health(ctx)

	require.NoError(t, err)
	assert.Equal(t, "healthy", health.Status)
}

func TestProviderChain_Health_NoProviders(t *testing.T) {
	chain := &ProviderChain{
		providers: []VectorProvider{},
		current:   0,
	}

	ctx := context.Background()
	_, err := chain.Health(ctx)
	assert.Error(t, err)
}

func TestProviderChain_GetConfiguration(t *testing.T) {
	chain := NewProviderChain([]VectorProvider{
		NewMockProvider("p1"),
	})

	config := chain.GetConfiguration()
	assert.Nil(t, config) // MockProvider returns nil
}

func TestProviderChain_IsCloud(t *testing.T) {
	chain := NewProviderChain([]VectorProvider{
		NewMockProvider("p1"),
	})

	assert.False(t, chain.IsCloud())
}

func TestProviderChain_GetStats(t *testing.T) {
	chain := NewProviderChain([]VectorProvider{
		NewMockProvider("p1"),
	})

	ctx := context.Background()
	stats, err := chain.GetStats(ctx)

	require.NoError(t, err)
	assert.NotNil(t, stats)
}

func TestProviderChain_Optimize(t *testing.T) {
	chain := NewProviderChain([]VectorProvider{
		NewMockProvider("p1"),
	})

	ctx := context.Background()
	err := chain.Optimize(ctx)
	require.NoError(t, err)
}

func TestProviderChain_Backup(t *testing.T) {
	chain := NewProviderChain([]VectorProvider{
		NewMockProvider("p1"),
	})

	ctx := context.Background()
	err := chain.Backup(ctx, "/tmp/backup")
	require.NoError(t, err)
}

func TestProviderChain_Restore(t *testing.T) {
	chain := NewProviderChain([]VectorProvider{
		NewMockProvider("p1"),
	})

	ctx := context.Background()
	err := chain.Restore(ctx, "/tmp/backup")
	require.NoError(t, err)
}

// ========================================
// HybridProvider Tests
// ========================================

func TestNewHybridProvider(t *testing.T) {
	providers := map[string]VectorProvider{
		"store":  NewMockProvider("store-provider"),
		"search": NewMockProvider("search-provider"),
	}

	hybrid := NewHybridProvider(HybridStrategyFailover, providers)
	assert.NotNil(t, hybrid)
	assert.Equal(t, HybridStrategyFailover, hybrid.strategy)
	assert.Equal(t, 2, len(hybrid.providers))
}

func TestHybridProvider_GetName(t *testing.T) {
	hybrid := NewHybridProvider(HybridStrategyFailover, map[string]VectorProvider{})
	assert.Equal(t, "Hybrid(failover)", hybrid.GetName())
}

func TestHybridProvider_GetType(t *testing.T) {
	hybrid := NewHybridProvider(HybridStrategyFailover, map[string]VectorProvider{})
	assert.Equal(t, "hybrid", hybrid.GetType())
}

func TestHybridProvider_GetCapabilities(t *testing.T) {
	hybrid := NewHybridProvider(HybridStrategyFailover, map[string]VectorProvider{
		"p1": NewMockProvider("p1"),
	})

	caps := hybrid.GetCapabilities()
	assert.NotEmpty(t, caps)
}

func TestHybridProvider_GetConfiguration(t *testing.T) {
	hybrid := NewHybridProvider(HybridStrategyFailover, map[string]VectorProvider{
		"p1": NewMockProvider("p1"),
	})

	config := hybrid.GetConfiguration()
	assert.NotNil(t, config)

	configMap := config.(map[string]interface{})
	assert.Equal(t, HybridStrategyFailover, configMap["strategy"])
}

func TestHybridProvider_IsCloud(t *testing.T) {
	hybrid := NewHybridProvider(HybridStrategyFailover, map[string]VectorProvider{
		"p1": NewMockProvider("p1"),
	})

	assert.False(t, hybrid.IsCloud())
}

func TestHybridProvider_SelectProvider_Failover(t *testing.T) {
	mock := NewMockProvider("p1")
	hybrid := NewHybridProvider(HybridStrategyFailover, map[string]VectorProvider{
		"p1": mock,
	})

	selected := hybrid.selectProvider("store")
	assert.Equal(t, mock, selected)
}

func TestHybridProvider_SelectProvider_RoundRobin(t *testing.T) {
	mock1 := NewMockProvider("p1")
	mock2 := NewMockProvider("p2")
	hybrid := NewHybridProvider(HybridStrategyRoundRobin, map[string]VectorProvider{
		"p1": mock1,
		"p2": mock2,
	})

	// First call
	_ = hybrid.selectProvider("store")
	// Second call should rotate
	_ = hybrid.selectProvider("store")

	assert.Equal(t, 2, hybrid.roundRobin)
}

func TestHybridProvider_SelectProvider_NoProviders(t *testing.T) {
	hybrid := NewHybridProvider(HybridStrategyFailover, map[string]VectorProvider{})

	selected := hybrid.selectProvider("store")
	assert.Nil(t, selected)
}

func TestHybridProvider_Store(t *testing.T) {
	hybrid := NewHybridProvider(HybridStrategyFailover, map[string]VectorProvider{
		"p1": NewMockProvider("p1"),
	})

	ctx := context.Background()
	vectors := []*VectorData{{ID: "v1", Vector: []float64{0.1}}}

	err := hybrid.Store(ctx, vectors)
	require.NoError(t, err)
}

func TestHybridProvider_Store_NoProvider(t *testing.T) {
	hybrid := NewHybridProvider(HybridStrategyFailover, map[string]VectorProvider{})

	ctx := context.Background()
	vectors := []*VectorData{{ID: "v1", Vector: []float64{0.1}}}

	err := hybrid.Store(ctx, vectors)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no provider available")
}

func TestHybridProvider_Search(t *testing.T) {
	hybrid := NewHybridProvider(HybridStrategyFailover, map[string]VectorProvider{
		"p1": NewMockProvider("p1"),
	})

	ctx := context.Background()
	query := &VectorQuery{Vector: []float64{0.1}, TopK: 10}

	result, err := hybrid.Search(ctx, query)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestHybridProvider_Delete(t *testing.T) {
	hybrid := NewHybridProvider(HybridStrategyFailover, map[string]VectorProvider{
		"p1": NewMockProvider("p1"),
	})

	ctx := context.Background()
	err := hybrid.Delete(ctx, []string{"v1"})
	require.NoError(t, err)
}

func TestHybridProvider_Health(t *testing.T) {
	hybrid := NewHybridProvider(HybridStrategyFailover, map[string]VectorProvider{
		"p1": NewMockProvider("p1"),
	})

	ctx := context.Background()
	health, err := hybrid.Health(ctx)

	require.NoError(t, err)
	assert.Equal(t, "healthy", health.Status)
}

func TestHybridProvider_Health_Degraded(t *testing.T) {
	failingMock := NewMockProvider("failing")
	failingMock.failOperations["health"] = true

	hybrid := NewHybridProvider(HybridStrategyFailover, map[string]VectorProvider{
		"failing": failingMock,
	})

	ctx := context.Background()
	health, err := hybrid.Health(ctx)

	require.NoError(t, err)
	assert.Equal(t, "degraded", health.Status)
}

func TestHybridProvider_Close(t *testing.T) {
	hybrid := NewHybridProvider(HybridStrategyFailover, map[string]VectorProvider{
		"p1": NewMockProvider("p1"),
	})

	ctx := context.Background()
	err := hybrid.Close(ctx)
	require.NoError(t, err)
}

func TestHybridProvider_Initialize(t *testing.T) {
	hybrid := NewHybridProvider(HybridStrategyFailover, map[string]VectorProvider{
		"p1": NewMockProvider("p1"),
	})

	ctx := context.Background()
	err := hybrid.Initialize(ctx, nil)
	require.NoError(t, err)
}

func TestHybridProvider_Start(t *testing.T) {
	hybrid := NewHybridProvider(HybridStrategyFailover, map[string]VectorProvider{
		"p1": NewMockProvider("p1"),
	})

	ctx := context.Background()
	err := hybrid.Start(ctx)
	require.NoError(t, err)
}

func TestHybridProvider_Stop(t *testing.T) {
	hybrid := NewHybridProvider(HybridStrategyFailover, map[string]VectorProvider{
		"p1": NewMockProvider("p1"),
	})

	ctx := context.Background()
	err := hybrid.Stop(ctx)
	require.NoError(t, err)
}

func TestHybridProvider_GetCostInfo(t *testing.T) {
	hybrid := NewHybridProvider(HybridStrategyFailover, map[string]VectorProvider{
		"p1": NewMockProvider("p1"),
		"p2": NewMockProvider("p2"),
	})

	costInfo := hybrid.GetCostInfo()
	assert.NotNil(t, costInfo)
	assert.Equal(t, "USD", costInfo.Currency)
}

func TestHybridProvider_GetStats(t *testing.T) {
	hybrid := NewHybridProvider(HybridStrategyFailover, map[string]VectorProvider{
		"p1": NewMockProvider("p1"),
	})

	ctx := context.Background()
	stats, err := hybrid.GetStats(ctx)

	require.NoError(t, err)
	assert.NotNil(t, stats)
}

func TestHybridProvider_CollectionOperations(t *testing.T) {
	hybrid := NewHybridProvider(HybridStrategyFailover, map[string]VectorProvider{
		"p1": NewMockProvider("p1"),
	})

	ctx := context.Background()

	// Create collection
	err := hybrid.CreateCollection(ctx, "test", &CollectionConfig{Dimension: 128})
	require.NoError(t, err)

	// List collections
	collections, err := hybrid.ListCollections(ctx)
	require.NoError(t, err)
	assert.NotNil(t, collections)

	// Delete collection
	err = hybrid.DeleteCollection(ctx, "test")
	require.NoError(t, err)
}

func TestHybridProvider_IndexOperations(t *testing.T) {
	hybrid := NewHybridProvider(HybridStrategyFailover, map[string]VectorProvider{
		"p1": NewMockProvider("p1"),
	})

	ctx := context.Background()

	// Create index
	err := hybrid.CreateIndex(ctx, "test", &IndexConfig{Name: "idx1", Type: "flat"})
	require.NoError(t, err)

	// List indexes
	indexes, err := hybrid.ListIndexes(ctx, "test")
	require.NoError(t, err)
	assert.NotNil(t, indexes)

	// Delete index
	err = hybrid.DeleteIndex(ctx, "test", "idx1")
	require.NoError(t, err)
}

func TestHybridProvider_MetadataOperations(t *testing.T) {
	mock := NewMockProvider("p1")
	hybrid := NewHybridProvider(HybridStrategyFailover, map[string]VectorProvider{
		"p1": mock,
	})

	ctx := context.Background()

	// Store a vector first
	vectors := []*VectorData{{ID: "v1", Vector: []float64{0.1}, Metadata: make(map[string]interface{})}}
	mock.Store(ctx, vectors)

	// Add metadata
	err := hybrid.AddMetadata(ctx, "v1", map[string]interface{}{"key": "value"})
	require.NoError(t, err)

	// Get metadata
	metadata, err := hybrid.GetMetadata(ctx, []string{"v1"})
	require.NoError(t, err)
	assert.NotNil(t, metadata)

	// Update metadata
	err = hybrid.UpdateMetadata(ctx, "v1", map[string]interface{}{"key": "updated"})
	require.NoError(t, err)

	// Delete metadata
	err = hybrid.DeleteMetadata(ctx, []string{"v1"}, []string{"key"})
	require.NoError(t, err)
}

func TestHybridProvider_SimilarityOperations(t *testing.T) {
	hybrid := NewHybridProvider(HybridStrategyFailover, map[string]VectorProvider{
		"p1": NewMockProvider("p1"),
	})

	ctx := context.Background()

	// FindSimilar
	results, err := hybrid.FindSimilar(ctx, []float64{0.1, 0.2}, 5, nil)
	require.NoError(t, err)
	assert.NotNil(t, results)

	// BatchFindSimilar
	batchResults, err := hybrid.BatchFindSimilar(ctx, [][]float64{{0.1}, {0.2}}, 5)
	require.NoError(t, err)
	assert.Nil(t, batchResults) // MockProvider returns nil
}

func TestHybridProvider_Retrieve(t *testing.T) {
	mock := NewMockProvider("p1")
	hybrid := NewHybridProvider(HybridStrategyFailover, map[string]VectorProvider{
		"p1": mock,
	})

	ctx := context.Background()

	// Store first
	vectors := []*VectorData{{ID: "v1", Vector: []float64{0.1}}}
	mock.Store(ctx, vectors)

	// Retrieve
	retrieved, err := hybrid.Retrieve(ctx, []string{"v1"})
	require.NoError(t, err)
	assert.Equal(t, 1, len(retrieved))
}

func TestHybridProvider_Update(t *testing.T) {
	hybrid := NewHybridProvider(HybridStrategyFailover, map[string]VectorProvider{
		"p1": NewMockProvider("p1"),
	})

	ctx := context.Background()
	err := hybrid.Update(ctx, "v1", &VectorData{ID: "v1", Vector: []float64{0.2}})
	require.NoError(t, err)
}

func TestHybridProvider_Optimize(t *testing.T) {
	hybrid := NewHybridProvider(HybridStrategyFailover, map[string]VectorProvider{
		"p1": NewMockProvider("p1"),
	})

	ctx := context.Background()
	err := hybrid.Optimize(ctx)
	require.NoError(t, err)
}

func TestHybridProvider_BackupRestore(t *testing.T) {
	hybrid := NewHybridProvider(HybridStrategyFailover, map[string]VectorProvider{
		"p1": NewMockProvider("p1"),
	})

	ctx := context.Background()

	err := hybrid.Backup(ctx, "/tmp/backup")
	require.NoError(t, err)

	err = hybrid.Restore(ctx, "/tmp/backup")
	require.NoError(t, err)
}

// ========================================
// ProviderTestSuite Tests
// ========================================

func TestNewProviderTestSuite(t *testing.T) {
	mock := NewMockProvider("test")
	config := map[string]interface{}{"key": "value"}

	suite := NewProviderTestSuite(mock, config)
	assert.NotNil(t, suite)
	assert.Equal(t, mock, suite.provider)
	assert.Equal(t, config, suite.config)
}

func TestProviderTestSuite_GenerateTestVectors(t *testing.T) {
	mock := NewMockProvider("test")
	suite := NewProviderTestSuite(mock, nil)

	vectors := suite.generateTestVectors(10, 128)
	assert.Equal(t, 10, len(vectors))
	assert.Equal(t, 128, len(vectors[0].Vector))
}

func TestProviderTestSuite_GenerateTestVector(t *testing.T) {
	mock := NewMockProvider("test")
	suite := NewProviderTestSuite(mock, nil)

	vector := suite.generateTestVector(256)
	assert.Equal(t, 256, len(vector))
}

// ========================================
// Type Constants Tests
// ========================================

func TestProviderTypeConstants(t *testing.T) {
	assert.Equal(t, ProviderType("pinecone"), ProviderTypePinecone)
	assert.Equal(t, ProviderType("milvus"), ProviderTypeMilvus)
	assert.Equal(t, ProviderType("weaviate"), ProviderTypeWeaviate)
	assert.Equal(t, ProviderType("qdrant"), ProviderTypeQdrant)
	assert.Equal(t, ProviderType("redis"), ProviderTypeRedis)
	assert.Equal(t, ProviderType("chroma"), ProviderTypeChroma)
	assert.Equal(t, ProviderType("faiss"), ProviderTypeFAISS)
	assert.Equal(t, ProviderType("mem0"), ProviderTypeMem0)
	assert.Equal(t, ProviderType("zep"), ProviderTypeZep)
	assert.Equal(t, ProviderType("memonto"), ProviderTypeMemonto)
}

func TestLoadBalanceTypeConstants(t *testing.T) {
	assert.Equal(t, LoadBalanceType("round_robin"), LoadBalanceRoundRobin)
	assert.Equal(t, LoadBalanceType("weighted"), LoadBalanceWeighted)
	assert.Equal(t, LoadBalanceType("random"), LoadBalanceRandom)
}

func TestHybridStrategyConstants(t *testing.T) {
	assert.Equal(t, HybridStrategy("failover"), HybridStrategyFailover)
	assert.Equal(t, HybridStrategy("round_robin"), HybridStrategyRoundRobin)
	assert.Equal(t, HybridStrategy("load_balance"), HybridStrategyLoadBalance)
	assert.Equal(t, HybridStrategy("operation_based"), HybridStrategyOperationBased)
}

// ========================================
// ProviderRegistry Tests
// ========================================

func TestNewProviderRegistry(t *testing.T) {
	registry := NewProviderRegistry()
	assert.NotNil(t, registry)
	assert.NotNil(t, registry.providers)
	assert.True(t, registry.initialized)
}

func TestGetRegistry(t *testing.T) {
	registry := GetRegistry()
	assert.NotNil(t, registry)

	// Should return same instance
	registry2 := GetRegistry()
	assert.Same(t, registry, registry2)
}

func TestProviderRegistry_ListProviders(t *testing.T) {
	registry := NewProviderRegistry()

	providers := registry.ListProviders()
	assert.NotEmpty(t, providers)
}

func TestProviderRegistry_GetDefaultConfig(t *testing.T) {
	registry := NewProviderRegistry()

	tests := []struct {
		providerType ProviderType
		expectKey    string
	}{
		{ProviderTypePinecone, "environment"},
		{ProviderTypeMilvus, "host"},
		{ProviderTypeOpenAI, "model"},
		{ProviderTypeRedis, "addr"},
		{ProviderTypeChroma, "host"},
		{ProviderTypeQdrant, "host"},
		{ProviderTypeWeaviate, "url"},
		{ProviderTypeFAISS, "index_type"},
	}

	for _, tt := range tests {
		t.Run(string(tt.providerType), func(t *testing.T) {
			config := registry.GetDefaultConfig(tt.providerType)
			assert.NotNil(t, config)
			_, exists := config[tt.expectKey]
			assert.True(t, exists, "Expected key %s in config for %s", tt.expectKey, tt.providerType)
		})
	}
}

func TestProviderRegistry_GetDefaultConfig_Unknown(t *testing.T) {
	registry := NewProviderRegistry()

	config := registry.GetDefaultConfig(ProviderType("unknown"))
	assert.NotNil(t, config)
	assert.Empty(t, config)
}

func TestProviderRegistry_GetProviderStatistics(t *testing.T) {
	registry := NewProviderRegistry()

	stats := registry.GetProviderStatistics()
	assert.NotNil(t, stats)
	assert.True(t, stats.TotalProviders > 0)
	assert.True(t, stats.Initialized)
	assert.NotNil(t, stats.ProvidersByType)
}

func TestProviderRegistry_GetProviderCategory(t *testing.T) {
	registry := NewProviderRegistry()

	tests := []struct {
		providerType ProviderType
		expected     string
	}{
		{ProviderTypePinecone, "vector_database"},
		{ProviderTypeMilvus, "vector_database"},
		{ProviderTypeWeaviate, "vector_database"},
		{ProviderTypeQdrant, "vector_database"},
		{ProviderTypeFAISS, "vector_database"},
		{ProviderTypeChroma, "vector_database"},
		{ProviderTypeMemGPT, "ai_memory"},
		{ProviderTypeCharacterAI, "ai_memory"},
		{ProviderTypeAgnostic, "utility"},
	}

	for _, tt := range tests {
		t.Run(string(tt.providerType), func(t *testing.T) {
			category := registry.getProviderCategory(tt.providerType)
			assert.Equal(t, tt.expected, category)
		})
	}
}

func TestProviderRegistry_GetCompatibleProviders(t *testing.T) {
	registry := NewProviderRegistry()

	// Test with empty requirements
	compatible := registry.GetCompatibleProviders(&ProviderRequirements{})
	assert.NotEmpty(t, compatible)
}

func TestProviderRegistry_GetCompatibleProviders_WithCloudRequirement(t *testing.T) {
	registry := NewProviderRegistry()

	// Test with cloud requirement
	isCloud := true
	compatible := registry.GetCompatibleProviders(&ProviderRequirements{
		IsCloud: &isCloud,
	})
	// May or may not have cloud providers depending on what's registered
	assert.NotNil(t, compatible)
}

func TestProviderRegistry_GetCompatibleProviders_WithCapabilities(t *testing.T) {
	registry := NewProviderRegistry()

	compatible := registry.GetCompatibleProviders(&ProviderRequirements{
		Capabilities: []string{"store", "search"},
	})
	// Result may be empty slice or have items, but should run without error
	t.Logf("Found %d compatible providers", len(compatible))
}

// ========================================
// VectorData Structure Tests
// ========================================

func TestVectorData_Structure(t *testing.T) {
	now := time.Now()
	ttl := time.Hour

	data := VectorData{
		ID:         "test-id",
		Vector:     []float64{0.1, 0.2, 0.3},
		Metadata:   map[string]interface{}{"key": "value"},
		Collection: "test-collection",
		Timestamp:  now,
		TTL:        &ttl,
		Namespace:  "test-namespace",
	}

	assert.Equal(t, "test-id", data.ID)
	assert.Equal(t, 3, len(data.Vector))
	assert.Equal(t, "value", data.Metadata["key"])
	assert.Equal(t, "test-collection", data.Collection)
	assert.Equal(t, now, data.Timestamp)
	assert.Equal(t, time.Hour, *data.TTL)
	assert.Equal(t, "test-namespace", data.Namespace)
}

func TestVectorQuery_Structure(t *testing.T) {
	query := VectorQuery{
		Vector:        []float64{0.1, 0.2},
		Text:          "test query",
		Collection:    "test-collection",
		Namespace:     "test-namespace",
		TopK:          10,
		Threshold:     0.8,
		IncludeVector: true,
		Filters:       map[string]interface{}{"category": "test"},
	}

	assert.Equal(t, 2, len(query.Vector))
	assert.Equal(t, "test query", query.Text)
	assert.Equal(t, "test-collection", query.Collection)
	assert.Equal(t, 10, query.TopK)
	assert.Equal(t, 0.8, query.Threshold)
	assert.True(t, query.IncludeVector)
}

func TestVectorSearchResult_Structure(t *testing.T) {
	result := VectorSearchResult{
		Results: []*VectorSearchResultItem{
			{ID: "r1", Score: 0.95, Distance: 0.05},
			{ID: "r2", Score: 0.90, Distance: 0.10},
		},
		Total:     2,
		Duration:  time.Millisecond * 50,
		Namespace: "test-namespace",
	}

	assert.Equal(t, 2, len(result.Results))
	assert.Equal(t, 2, result.Total)
	assert.Equal(t, time.Millisecond*50, result.Duration)
	assert.Equal(t, "r1", result.Results[0].ID)
	assert.Equal(t, 0.95, result.Results[0].Score)
}

func TestCollectionConfig_Structure(t *testing.T) {
	config := CollectionConfig{
		Name:        "test-collection",
		Description: "A test collection",
		Dimension:   1536,
		Metric:      "cosine",
		Properties:  map[string]interface{}{"indexed": true},
		Replicas:    3,
		Shards:      2,
	}

	assert.Equal(t, "test-collection", config.Name)
	assert.Equal(t, "A test collection", config.Description)
	assert.Equal(t, 1536, config.Dimension)
	assert.Equal(t, "cosine", config.Metric)
	assert.Equal(t, 3, config.Replicas)
	assert.Equal(t, 2, config.Shards)
}

func TestIndexConfig_Structure(t *testing.T) {
	config := IndexConfig{
		Name:       "test-index",
		Type:       "IVF_FLAT",
		Parameters: map[string]interface{}{"nlist": 100},
		Metric:     "L2",
	}

	assert.Equal(t, "test-index", config.Name)
	assert.Equal(t, "IVF_FLAT", config.Type)
	assert.Equal(t, "L2", config.Metric)
	assert.Equal(t, 100, config.Parameters["nlist"])
}

func TestProviderStats_Structure(t *testing.T) {
	stats := ProviderStats{
		Name:             "test-provider",
		Type:             "mock",
		Status:           "active",
		TotalOperations:  1000,
		SuccessfulOps:    990,
		FailedOps:        10,
		AverageLatency:   time.Millisecond * 5,
		TotalVectors:     50000,
		TotalCollections: 5,
		TotalSize:        1024 * 1024 * 100,
		ErrorCount:       10,
		Uptime:           time.Hour * 24,
	}

	assert.Equal(t, "test-provider", stats.Name)
	assert.Equal(t, int64(1000), stats.TotalOperations)
	assert.Equal(t, int64(990), stats.SuccessfulOps)
	assert.Equal(t, int64(50000), stats.TotalVectors)
}

func TestHealthStatus_Structure(t *testing.T) {
	now := time.Now()
	health := HealthStatus{
		Status:       "healthy",
		Message:      "All systems operational",
		Timestamp:    now,
		LastCheck:    now.Add(-time.Minute),
		ResponseTime: time.Millisecond * 10,
		Metrics:      map[string]interface{}{"cpu": 50.0},
		Dependencies: map[string]string{"database": "healthy"},
		Details:      map[string]interface{}{"version": "1.0.0"},
	}

	assert.Equal(t, "healthy", health.Status)
	assert.Equal(t, "All systems operational", health.Message)
	assert.Equal(t, time.Millisecond*10, health.ResponseTime)
	assert.Equal(t, 50.0, health.Metrics["cpu"])
}

func TestCostInfo_Structure(t *testing.T) {
	costInfo := CostInfo{
		Currency:      "USD",
		ComputeCost:   10.50,
		TransferCost:  2.25,
		StorageCost:   5.00,
		TotalCost:     17.75,
		BillingPeriod: "monthly",
		FreeTierUsed:  100.0,
		FreeTierLimit: 1000.0,
		Details:       map[string]float64{"api_calls": 10.50},
	}

	assert.Equal(t, "USD", costInfo.Currency)
	assert.Equal(t, 10.50, costInfo.ComputeCost)
	assert.Equal(t, 17.75, costInfo.TotalCost)
	assert.Equal(t, "monthly", costInfo.BillingPeriod)
	assert.Equal(t, 100.0, costInfo.FreeTierUsed)
}

func TestModel_Structure(t *testing.T) {
	now := time.Now()
	model := Model{
		ID:              "model-1",
		Name:            "Test Model",
		Type:            "embedding",
		Version:         "1.0.0",
		Description:     "A test embedding model",
		Architecture:    "transformer",
		Parameters:      7000000000,
		IsActive:        true,
		CPUOptimization: true,
		GPUEnabled:      true,
		Quantization:    false,
		Caching:         true,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	assert.Equal(t, "model-1", model.ID)
	assert.Equal(t, "Test Model", model.Name)
	assert.Equal(t, int64(7000000000), model.Parameters)
	assert.True(t, model.IsActive)
	assert.True(t, model.GPUEnabled)
}

func TestEmbedding_Structure(t *testing.T) {
	now := time.Now()
	embedding := Embedding{
		ID:        "emb-1",
		ModelID:   "model-1",
		Text:      "Sample text for embedding",
		Values:    []float64{0.1, 0.2, 0.3, 0.4},
		CreatedAt: now,
		UpdatedAt: now,
		Metadata:  map[string]interface{}{"source": "test"},
	}

	assert.Equal(t, "emb-1", embedding.ID)
	assert.Equal(t, "model-1", embedding.ModelID)
	assert.Equal(t, 4, len(embedding.Values))
	assert.Equal(t, "test", embedding.Metadata["source"])
}

func TestGenerationOptions_Structure(t *testing.T) {
	options := GenerationOptions{
		MaxTokens:        1000,
		Temperature:      0.7,
		TopP:             0.9,
		FrequencyPenalty: 0.1,
		PresencePenalty:  0.1,
		Stop:             []string{".", "!"},
		Stream:           true,
	}

	assert.Equal(t, 1000, options.MaxTokens)
	assert.Equal(t, 0.7, options.Temperature)
	assert.Equal(t, 0.9, options.TopP)
	assert.True(t, options.Stream)
	assert.Equal(t, 2, len(options.Stop))
}

func TestModelPerformance_Structure(t *testing.T) {
	now := time.Now()
	perf := ModelPerformance{
		ID:                "perf-1",
		ModelID:           "model-1",
		ResponseTime:      time.Millisecond * 100,
		Throughput:        1000.0,
		CPUUtilization:    50.0,
		GPUUtilization:    80.0,
		MemoryUsage:       1024 * 1024 * 1024,
		Accuracy:          0.95,
		ErrorRate:         0.02,
		RequestsPerSecond: 50.0,
		LastUpdated:       now,
	}

	assert.Equal(t, "perf-1", perf.ID)
	assert.Equal(t, time.Millisecond*100, perf.ResponseTime)
	assert.Equal(t, 80.0, perf.GPUUtilization)
	assert.Equal(t, 0.95, perf.Accuracy)
}
