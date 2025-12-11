package memory

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// MemoryConfig represents configuration for memory operations
type MemoryConfig struct {
	Enabled          bool          `json:"enabled"`
	Provider         string        `json:"provider"`
	MaxGenerations   int           `json:"max_generations"`
	MaxConversations int           `json:"max_conversations"`
	TTL              time.Duration `json:"ttl"`
}

// MemorySearchResult represents a search result from memory providers
type MemorySearchResult struct {
	Key   string      `json:"key"`
	Data  interface{} `json:"data"`
	Score float64     `json:"score"`
}

// MemoryProvider defines the interface for memory providers
type MemoryProvider interface {
	// Store stores data in memory
	Store(ctx context.Context, key string, data interface{}) error

	// Retrieve retrieves data from memory
	Retrieve(ctx context.Context, key string) (interface{}, error)

	// Search searches for data in memory
	Search(ctx context.Context, query string, limit int) ([]MemorySearchResult, error)

	// Delete deletes data from memory
	Delete(ctx context.Context, key string) error

	// Clear clears all data from memory
	Clear(ctx context.Context) error

	// Health checks the health of the memory provider
	Health(ctx context.Context) error

	// Name returns the name of the provider
	Name() string

	// Type returns the type of the provider
	Type() string
}

// MemoryManager manages memory operations across multiple providers
type MemoryManager struct {
	providers       map[string]MemoryProvider
	defaultProvider string
	config          *MemoryConfig
	mu              sync.RWMutex
}

// NewMemoryManager creates a new memory manager
func NewMemoryManager(config *MemoryConfig) *MemoryManager {
	return &MemoryManager{
		providers: make(map[string]MemoryProvider),
		config:    config,
	}
}

// RegisterProvider registers a memory provider
func (mm *MemoryManager) RegisterProvider(name string, provider MemoryProvider) error {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	if _, exists := mm.providers[name]; exists {
		return fmt.Errorf("provider %s already registered", name)
	}

	mm.providers[name] = provider

	// Set as default if it's the first provider
	if mm.defaultProvider == "" {
		mm.defaultProvider = name
	}

	return nil
}

// UnregisterProvider unregisters a memory provider
func (mm *MemoryManager) UnregisterProvider(name string) error {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	if _, exists := mm.providers[name]; !exists {
		return fmt.Errorf("provider %s not registered", name)
	}

	delete(mm.providers, name)

	// Reset default if it was removed
	if mm.defaultProvider == name {
		mm.defaultProvider = ""
		// Set first available as default
		for providerName := range mm.providers {
			mm.defaultProvider = providerName
			break
		}
	}

	return nil
}

// SetDefaultProvider sets the default memory provider
func (mm *MemoryManager) SetDefaultProvider(name string) error {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	if _, exists := mm.providers[name]; !exists {
		return fmt.Errorf("provider %s not registered", name)
	}

	mm.defaultProvider = name
	return nil
}

// GetProvider gets a memory provider by name
func (mm *MemoryManager) GetProvider(name string) (MemoryProvider, error) {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	provider, exists := mm.providers[name]
	if !exists {
		return nil, fmt.Errorf("provider %s not found", name)
	}

	return provider, nil
}

// GetDefaultProvider gets the default memory provider
func (mm *MemoryManager) GetDefaultProvider() (MemoryProvider, error) {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	if mm.defaultProvider == "" {
		return nil, fmt.Errorf("no default provider set")
	}

	return mm.GetProvider(mm.defaultProvider)
}

// ListProviders lists all registered providers
func (mm *MemoryManager) ListProviders() []string {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	providers := make([]string, 0, len(mm.providers))
	for name := range mm.providers {
		providers = append(providers, name)
	}

	return providers
}

// Store stores data using the default provider
func (mm *MemoryManager) Store(ctx context.Context, key string, data interface{}) error {
	provider, err := mm.GetDefaultProvider()
	if err != nil {
		return fmt.Errorf("failed to get default provider: %w", err)
	}

	return provider.Store(ctx, key, data)
}

// Retrieve retrieves data using the default provider
func (mm *MemoryManager) Retrieve(ctx context.Context, key string) (interface{}, error) {
	provider, err := mm.GetDefaultProvider()
	if err != nil {
		return nil, fmt.Errorf("failed to get default provider: %w", err)
	}

	return provider.Retrieve(ctx, key)
}

// Search searches data using the default provider
func (mm *MemoryManager) Search(ctx context.Context, query string, limit int) ([]MemorySearchResult, error) {
	provider, err := mm.GetDefaultProvider()
	if err != nil {
		return nil, fmt.Errorf("failed to get default provider: %w", err)
	}

	return provider.Search(ctx, query, limit)
}

// Delete deletes data using the default provider
func (mm *MemoryManager) Delete(ctx context.Context, key string) error {
	provider, err := mm.GetDefaultProvider()
	if err != nil {
		return fmt.Errorf("failed to get default provider: %w", err)
	}

	return provider.Delete(ctx, key)
}

// Clear clears all data using the default provider
func (mm *MemoryManager) Clear(ctx context.Context) error {
	provider, err := mm.GetDefaultProvider()
	if err != nil {
		return fmt.Errorf("failed to get default provider: %w", err)
	}

	return provider.Clear(ctx)
}

// Health checks the health of all providers
func (mm *MemoryManager) Health(ctx context.Context) map[string]error {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	health := make(map[string]error)

	for name, provider := range mm.providers {
		if err := provider.Health(ctx); err != nil {
			health[name] = err
		} else {
			health[name] = nil
		}
	}

	return health
}

// GetStatistics returns statistics for all providers
func (mm *MemoryManager) GetStatistics() map[string]interface{} {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	stats := map[string]interface{}{
		"total_providers":  len(mm.providers),
		"default_provider": mm.defaultProvider,
		"providers":        make(map[string]interface{}),
	}

	providers := make(map[string]interface{})
	for name, provider := range mm.providers {
		providers[name] = map[string]interface{}{
			"type": provider.Type(),
			"name": provider.Name(),
		}
	}

	stats["providers"] = providers
	return stats
}

// MemoryProviderFactory creates memory providers
type MemoryProviderFactory struct{}

// NewMemoryProviderFactory creates a new factory
func NewMemoryProviderFactory() *MemoryProviderFactory {
	return &MemoryProviderFactory{}
}

// CreateProvider creates a memory provider based on configuration
func (f *MemoryProviderFactory) CreateProvider(providerType string, config map[string]interface{}) (MemoryProvider, error) {
	switch providerType {
	case "redis":
		return NewRedisMemoryProvider(config)
	case "memcached":
		return NewMemcachedMemoryProvider(config)
	case "inmemory":
		return NewInMemoryProvider(config)
	case "filesystem":
		return NewFilesystemMemoryProvider(config)
	default:
		return nil, fmt.Errorf("unsupported memory provider type: %s", providerType)
	}
}

// InMemoryProvider is an in-memory implementation of MemoryProvider
type InMemoryProvider struct {
	data map[string]interface{}
	mu   sync.RWMutex
}

// NewInMemoryProvider creates a new in-memory provider
func NewInMemoryProvider(config map[string]interface{}) (*InMemoryProvider, error) {
	return &InMemoryProvider{
		data: make(map[string]interface{}),
	}, nil
}

// Store stores data in memory
func (p *InMemoryProvider) Store(ctx context.Context, key string, data interface{}) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.data[key] = data
	return nil
}

// Retrieve retrieves data from memory
func (p *InMemoryProvider) Retrieve(ctx context.Context, key string) (interface{}, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	data, exists := p.data[key]
	if !exists {
		return nil, fmt.Errorf("key not found: %s", key)
	}

	return data, nil
}

// Search searches for data (simple implementation)
func (p *InMemoryProvider) Search(ctx context.Context, query string, limit int) ([]MemorySearchResult, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	results := make([]MemorySearchResult, 0, limit)
	count := 0

	for key, data := range p.data {
		if count >= limit {
			break
		}

		// Simple string matching for demo
		if fmt.Sprintf("%v", data) == query || key == query {
			results = append(results, MemorySearchResult{
				Key:   key,
				Data:  data,
				Score: 1.0,
			})
			count++
		}
	}

	return results, nil
}

// Delete deletes data from memory
func (p *InMemoryProvider) Delete(ctx context.Context, key string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.data[key]; !exists {
		return fmt.Errorf("key not found: %s", key)
	}

	delete(p.data, key)
	return nil
}

// Clear clears all data
func (p *InMemoryProvider) Clear(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.data = make(map[string]interface{})
	return nil
}

// Health checks health
func (p *InMemoryProvider) Health(ctx context.Context) error {
	return nil // In-memory is always healthy
}

// Name returns the provider name
func (p *InMemoryProvider) Name() string {
	return "in-memory"
}

// Type returns the provider type
func (p *InMemoryProvider) Type() string {
	return "inmemory"
}

// RedisMemoryProvider is a Redis implementation
type RedisMemoryProvider struct {
	config map[string]interface{}
}

// NewRedisMemoryProvider creates a new Redis provider
func NewRedisMemoryProvider(config map[string]interface{}) (*RedisMemoryProvider, error) {
	return &RedisMemoryProvider{
		config: config,
	}, nil
}

// Store stores data in Redis
func (p *RedisMemoryProvider) Store(ctx context.Context, key string, data interface{}) error {
	// Implementation would use Redis client
	return fmt.Errorf("Redis provider not fully implemented")
}

// Retrieve retrieves data from Redis
func (p *RedisMemoryProvider) Retrieve(ctx context.Context, key string) (interface{}, error) {
	// Implementation would use Redis client
	return nil, fmt.Errorf("Redis provider not fully implemented")
}

// Search searches data in Redis
func (p *RedisMemoryProvider) Search(ctx context.Context, query string, limit int) ([]MemorySearchResult, error) {
	// Implementation would use Redis search
	return nil, fmt.Errorf("Redis provider not fully implemented")
}

// Delete deletes data from Redis
func (p *RedisMemoryProvider) Delete(ctx context.Context, key string) error {
	// Implementation would use Redis client
	return fmt.Errorf("Redis provider not fully implemented")
}

// Clear clears all data from Redis
func (p *RedisMemoryProvider) Clear(ctx context.Context) error {
	// Implementation would use Redis client
	return fmt.Errorf("Redis provider not fully implemented")
}

// Health checks Redis health
func (p *RedisMemoryProvider) Health(ctx context.Context) error {
	// Implementation would check Redis connection
	return fmt.Errorf("Redis provider not fully implemented")
}

// Name returns the provider name
func (p *RedisMemoryProvider) Name() string {
	return "redis"
}

// Type returns the provider type
func (p *RedisMemoryProvider) Type() string {
	return "redis"
}

// MemcachedMemoryProvider is a Memcached implementation
type MemcachedMemoryProvider struct {
	config map[string]interface{}
}

// NewMemcachedMemoryProvider creates a new Memcached provider
func NewMemcachedMemoryProvider(config map[string]interface{}) (*MemcachedMemoryProvider, error) {
	return &MemcachedMemoryProvider{
		config: config,
	}, nil
}

// Store stores data in Memcached
func (p *MemcachedMemoryProvider) Store(ctx context.Context, key string, data interface{}) error {
	return fmt.Errorf("Memcached provider not fully implemented")
}

// Retrieve retrieves data from Memcached
func (p *MemcachedMemoryProvider) Retrieve(ctx context.Context, key string) (interface{}, error) {
	return nil, fmt.Errorf("Memcached provider not fully implemented")
}

// Search searches data in Memcached
func (p *MemcachedMemoryProvider) Search(ctx context.Context, query string, limit int) ([]MemorySearchResult, error) {
	return nil, fmt.Errorf("Memcached provider not fully implemented")
}

// Delete deletes data from Memcached
func (p *MemcachedMemoryProvider) Delete(ctx context.Context, key string) error {
	return fmt.Errorf("Memcached provider not fully implemented")
}

// Clear clears all data from Memcached
func (p *MemcachedMemoryProvider) Clear(ctx context.Context) error {
	return fmt.Errorf("Memcached provider not fully implemented")
}

// Health checks Memcached health
func (p *MemcachedMemoryProvider) Health(ctx context.Context) error {
	return fmt.Errorf("Memcached provider not fully implemented")
}

// Name returns the provider name
func (p *MemcachedMemoryProvider) Name() string {
	return "memcached"
}

// Type returns the provider type
func (p *MemcachedMemoryProvider) Type() string {
	return "memcached"
}

// FilesystemMemoryProvider is a filesystem-based implementation
type FilesystemMemoryProvider struct {
	config map[string]interface{}
}

// NewFilesystemMemoryProvider creates a new filesystem provider
func NewFilesystemMemoryProvider(config map[string]interface{}) (*FilesystemMemoryProvider, error) {
	return &FilesystemMemoryProvider{
		config: config,
	}, nil
}

// Store stores data in filesystem
func (p *FilesystemMemoryProvider) Store(ctx context.Context, key string, data interface{}) error {
	return fmt.Errorf("Filesystem provider not fully implemented")
}

// Retrieve retrieves data from filesystem
func (p *FilesystemMemoryProvider) Retrieve(ctx context.Context, key string) (interface{}, error) {
	return nil, fmt.Errorf("Filesystem provider not fully implemented")
}

// Search searches data in filesystem
func (p *FilesystemMemoryProvider) Search(ctx context.Context, query string, limit int) ([]MemorySearchResult, error) {
	return nil, fmt.Errorf("Filesystem provider not fully implemented")
}

// Delete deletes data from filesystem
func (p *FilesystemMemoryProvider) Delete(ctx context.Context, key string) error {
	return fmt.Errorf("Filesystem provider not fully implemented")
}

// Clear clears all data from filesystem
func (p *FilesystemMemoryProvider) Clear(ctx context.Context) error {
	return fmt.Errorf("Filesystem provider not fully implemented")
}

// Health checks filesystem health
func (p *FilesystemMemoryProvider) Health(ctx context.Context) error {
	return fmt.Errorf("Filesystem provider not fully implemented")
}

// Name returns the provider name
func (p *FilesystemMemoryProvider) Name() string {
	return "filesystem"
}

// Type returns the provider type
func (p *FilesystemMemoryProvider) Type() string {
	return "filesystem"
}

// Global memory manager instance
var globalManager *MemoryManager

// GetGlobalManager returns the global memory manager
func GetGlobalManager() *MemoryManager {
	return globalManager
}

// SetGlobalManager sets the global memory manager
func SetGlobalManager(manager *MemoryManager) {
	globalManager = manager
}

// InitializeGlobalManager initializes the global memory manager
func InitializeGlobalManager(config *MemoryConfig) {
	globalManager = NewMemoryManager(config)

	// Register default in-memory provider
	inMemory, _ := NewInMemoryProvider(map[string]interface{}{})
	globalManager.RegisterProvider("inmemory", inMemory)
}

// StoreGlobal stores data using the global manager
func StoreGlobal(ctx context.Context, key string, data interface{}) error {
	if globalManager == nil {
		return fmt.Errorf("global memory manager not initialized")
	}
	return globalManager.Store(ctx, key, data)
}

// RetrieveGlobal retrieves data using the global manager
func RetrieveGlobal(ctx context.Context, key string) (interface{}, error) {
	if globalManager == nil {
		return nil, fmt.Errorf("global memory manager not initialized")
	}
	return globalManager.Retrieve(ctx, key)
}

// SearchGlobal searches data using the global manager
func SearchGlobal(ctx context.Context, query string, limit int) ([]MemorySearchResult, error) {
	if globalManager == nil {
		return nil, fmt.Errorf("global memory manager not initialized")
	}
	return globalManager.Search(ctx, query, limit)
}
