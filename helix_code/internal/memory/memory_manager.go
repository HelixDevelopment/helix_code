package memory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"
)

// ErrRedisClientNotInitialized is returned by RedisMemoryProvider.Health
// when no real Redis client has been wired into the provider. Before
// round-31 §11.4 anti-bluff sweep (2026-05-18) Health returned nil
// unconditionally — even when no client existed — so the health endpoint
// reported OK for a backend that was never even attempted. The
// RedisMemoryProvider struct currently keeps state in a local in-memory
// map (no real client field exists), so Health now surfaces this sentinel
// to make the absence of real Redis connectivity unambiguous to operators
// and monitoring systems. Wiring a real go-redis/v9 client and replacing
// the in-memory map MUST happen before this provider is fit for the
// "redis" name; until then Health correctly fails closed.
//
// §11.4 CRITICAL: false health for dead Redis is a release blocker under
// Article XI §11.9 / CONST-035.
var ErrRedisClientNotInitialized = errors.New("redis memory provider: client has not been initialized — Health() previously returned nil regardless of backend state (§11.4 CRITICAL: false health for dead Redis); the current RedisMemoryProvider holds state in an in-memory map and does NOT yet talk to a real Redis server, so Health fails closed until a real go-redis/v9 client is wired in")

// ErrMemcachedClientNotInitialized is returned by
// MemcachedMemoryProvider.Health when no real Memcached client has been
// wired into the provider. Same story as ErrRedisClientNotInitialized:
// the previous implementation returned nil unconditionally so the health
// endpoint advertised OK regardless of backend state. Until a real
// gomemcache client is wired in and the local in-memory map is replaced,
// Health fails closed via this sentinel.
//
// §11.4 CRITICAL: false health for dead Memcached is a release blocker
// under Article XI §11.9 / CONST-035.
var ErrMemcachedClientNotInitialized = errors.New("memcached memory provider: client has not been initialized — Health() previously returned nil regardless of backend state (§11.4 CRITICAL: false health for dead Memcached); the current MemcachedMemoryProvider holds state in an in-memory map and does NOT yet talk to a real Memcached server, so Health fails closed until a real gomemcache client is wired in")

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
	host   string
	port   int
	prefix string
	data   map[string]interface{}
	mu     sync.RWMutex
}

// NewRedisMemoryProvider creates a new Redis provider
func NewRedisMemoryProvider(config map[string]interface{}) (*RedisMemoryProvider, error) {
	provider := &RedisMemoryProvider{
		config: config,
		data:   make(map[string]interface{}),
	}

	// Extract Redis connection settings from config
	if host, ok := config["host"].(string); ok {
		provider.host = host
	} else {
		provider.host = "localhost"
	}
	if port, ok := config["port"].(int); ok {
		provider.port = port
	} else {
		provider.port = 6379
	}
	if prefix, ok := config["prefix"].(string); ok {
		provider.prefix = prefix
	} else {
		provider.prefix = "helix:memory:"
	}

	return provider, nil
}

// Store stores data in Redis
func (p *RedisMemoryProvider) Store(ctx context.Context, key string, data interface{}) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Store in local cache (Redis client would be used in production)
	fullKey := p.prefix + key
	p.data[fullKey] = data
	return nil
}

// Retrieve retrieves data from Redis
func (p *RedisMemoryProvider) Retrieve(ctx context.Context, key string) (interface{}, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	fullKey := p.prefix + key
	if data, exists := p.data[fullKey]; exists {
		return data, nil
	}
	return nil, fmt.Errorf("key not found: %s", key)
}

// Search searches data in Redis
func (p *RedisMemoryProvider) Search(ctx context.Context, query string, limit int) ([]MemorySearchResult, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	results := make([]MemorySearchResult, 0, limit)
	count := 0

	for key, data := range p.data {
		if count >= limit {
			break
		}
		// Simple string matching - check if query matches key or data value
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

// Delete deletes data from Redis
func (p *RedisMemoryProvider) Delete(ctx context.Context, key string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	fullKey := p.prefix + key
	delete(p.data, fullKey)
	return nil
}

// Clear clears all data from Redis
func (p *RedisMemoryProvider) Clear(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.data = make(map[string]interface{})
	return nil
}

// Health checks Redis health by attempting to PING the configured backend.
//
// Round-31 §11.4 anti-bluff sweep (2026-05-18): the previous implementation
// returned nil unconditionally — a placeholder labelled "In production,
// this would ping the Redis server" — so monitoring endpoints reported OK
// regardless of whether Redis was alive or even reachable. That is a
// CRITICAL false-health bluff under Article XI §11.9 / CONST-035.
//
// The current RedisMemoryProvider struct holds state in an in-memory map
// and does NOT yet embed a real go-redis/v9 client (no client field is
// declared). Until a real client is wired in (tracked in the close-out
// log under §11.4 follow-ups), Health fails closed with
// ErrRedisClientNotInitialized so operators and dashboards see the
// missing-implementation state honestly rather than a fabricated PASS.
func (p *RedisMemoryProvider) Health(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("redis memory provider: health check aborted: %w", err)
	}
	// No real Redis client is wired in yet (see ErrRedisClientNotInitialized
	// doc-comment). Return the sentinel rather than a fabricated PASS.
	return ErrRedisClientNotInitialized
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
	host   string
	port   int
	prefix string
	data   map[string]interface{}
	mu     sync.RWMutex
}

// NewMemcachedMemoryProvider creates a new Memcached provider
func NewMemcachedMemoryProvider(config map[string]interface{}) (*MemcachedMemoryProvider, error) {
	provider := &MemcachedMemoryProvider{
		config: config,
		data:   make(map[string]interface{}),
	}

	// Extract Memcached connection settings from config
	if host, ok := config["host"].(string); ok {
		provider.host = host
	} else {
		provider.host = "localhost"
	}
	if port, ok := config["port"].(int); ok {
		provider.port = port
	} else {
		provider.port = 11211
	}
	if prefix, ok := config["prefix"].(string); ok {
		provider.prefix = prefix
	} else {
		provider.prefix = "helix:memory:"
	}

	return provider, nil
}

// Store stores data in Memcached
func (p *MemcachedMemoryProvider) Store(ctx context.Context, key string, data interface{}) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	fullKey := p.prefix + key
	p.data[fullKey] = data
	return nil
}

// Retrieve retrieves data from Memcached
func (p *MemcachedMemoryProvider) Retrieve(ctx context.Context, key string) (interface{}, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	fullKey := p.prefix + key
	if data, exists := p.data[fullKey]; exists {
		return data, nil
	}
	return nil, fmt.Errorf("key not found: %s", key)
}

// Search searches data in Memcached
func (p *MemcachedMemoryProvider) Search(ctx context.Context, query string, limit int) ([]MemorySearchResult, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	results := make([]MemorySearchResult, 0, limit)
	count := 0

	for key, data := range p.data {
		if count >= limit {
			break
		}
		// Simple string matching - check if query matches key or data value
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

// Delete deletes data from Memcached
func (p *MemcachedMemoryProvider) Delete(ctx context.Context, key string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	fullKey := p.prefix + key
	delete(p.data, fullKey)
	return nil
}

// Clear clears all data from Memcached
func (p *MemcachedMemoryProvider) Clear(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.data = make(map[string]interface{})
	return nil
}

// Health checks Memcached health by probing the configured backend.
//
// Round-31 §11.4 anti-bluff sweep (2026-05-18): the previous implementation
// returned nil unconditionally — a placeholder labelled "In production,
// this would ping the Memcached server" — so monitoring endpoints reported
// OK regardless of whether Memcached was alive or even reachable. That is
// a CRITICAL false-health bluff under Article XI §11.9 / CONST-035.
//
// The current MemcachedMemoryProvider struct holds state in an in-memory
// map and does NOT yet embed a real gomemcache client (no client field is
// declared). Until a real client is wired in (tracked in the close-out
// log under §11.4 follow-ups), Health fails closed with
// ErrMemcachedClientNotInitialized so operators and dashboards see the
// missing-implementation state honestly rather than a fabricated PASS.
func (p *MemcachedMemoryProvider) Health(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("memcached memory provider: health check aborted: %w", err)
	}
	// No real Memcached client is wired in yet (see
	// ErrMemcachedClientNotInitialized doc-comment). Return the sentinel
	// rather than a fabricated PASS.
	return ErrMemcachedClientNotInitialized
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
	config   map[string]interface{}
	basePath string
	index    map[string]string // Maps keys to file paths
	mu       sync.RWMutex
}

// NewFilesystemMemoryProvider creates a new filesystem provider
func NewFilesystemMemoryProvider(config map[string]interface{}) (*FilesystemMemoryProvider, error) {
	provider := &FilesystemMemoryProvider{
		config: config,
		index:  make(map[string]string),
	}

	// Extract filesystem settings from config
	if basePath, ok := config["base_path"].(string); ok {
		provider.basePath = basePath
	} else {
		provider.basePath = "/tmp/helix-memory"
	}

	// Create base directory if it doesn't exist
	if err := os.MkdirAll(provider.basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base directory: %w", err)
	}

	return provider, nil
}

// Store stores data in filesystem
func (p *FilesystemMemoryProvider) Store(ctx context.Context, key string, data interface{}) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Serialize data to JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to serialize data: %w", err)
	}

	// Create safe filename from key
	filename := p.keyToFilename(key)
	filepath := p.basePath + "/" + filename

	// Write to file
	if err := os.WriteFile(filepath, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	// Update index
	p.index[key] = filepath
	return nil
}

// Retrieve retrieves data from filesystem
func (p *FilesystemMemoryProvider) Retrieve(ctx context.Context, key string) (interface{}, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	filepath, exists := p.index[key]
	if !exists {
		// Try to find by filename
		filename := p.keyToFilename(key)
		filepath = p.basePath + "/" + filename
	}

	// Read file
	jsonData, err := os.ReadFile(filepath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("key not found: %s", key)
		}
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Deserialize data
	var data interface{}
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return nil, fmt.Errorf("failed to deserialize data: %w", err)
	}

	return data, nil
}

// Search searches data in filesystem
func (p *FilesystemMemoryProvider) Search(ctx context.Context, query string, limit int) ([]MemorySearchResult, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	results := make([]MemorySearchResult, 0, limit)
	count := 0

	// Search through indexed keys
	for key, filepath := range p.index {
		if count >= limit {
			break
		}

		// Read the file content
		jsonData, err := os.ReadFile(filepath)
		if err != nil {
			continue
		}

		var data interface{}
		if err := json.Unmarshal(jsonData, &data); err != nil {
			continue
		}

		// Simple string matching - check if query matches key or data value
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

// Delete deletes data from filesystem
func (p *FilesystemMemoryProvider) Delete(ctx context.Context, key string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	filepath, exists := p.index[key]
	if !exists {
		filename := p.keyToFilename(key)
		filepath = p.basePath + "/" + filename
	}

	// Remove file
	if err := os.Remove(filepath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	// Remove from index
	delete(p.index, key)
	return nil
}

// Clear clears all data from filesystem
func (p *FilesystemMemoryProvider) Clear(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Remove all files in base directory
	entries, err := os.ReadDir(p.basePath)
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			filepath := p.basePath + "/" + entry.Name()
			os.Remove(filepath)
		}
	}

	// Clear index
	p.index = make(map[string]string)
	return nil
}

// Health checks filesystem health
func (p *FilesystemMemoryProvider) Health(ctx context.Context) error {
	// Check if base directory is accessible
	_, err := os.Stat(p.basePath)
	if err != nil {
		return fmt.Errorf("filesystem health check failed: %w", err)
	}
	return nil
}

// keyToFilename converts a key to a safe filename
func (p *FilesystemMemoryProvider) keyToFilename(key string) string {
	// Simple hash-based filename to avoid special characters
	hash := 0
	for _, c := range key {
		hash = hash*31 + int(c)
	}
	if hash < 0 {
		hash = -hash
	}
	return fmt.Sprintf("%d.json", hash)
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
