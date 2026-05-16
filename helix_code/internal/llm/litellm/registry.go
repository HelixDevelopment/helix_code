package litellm

import (
	"context"
	"fmt"
	"sync"
)

type Registry struct {
	mu        sync.RWMutex
	providers map[string]ProviderInfo
	adapters  map[string]FormatAdapter
	instances map[string]*UnifiedProvider
}

func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]ProviderInfo),
		adapters:  make(map[string]FormatAdapter),
		instances: make(map[string]*UnifiedProvider),
	}
}

func (r *Registry) Register(name string, info ProviderInfo, adapter FormatAdapter, config UnifiedProviderConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[name] = info
	r.adapters[name] = adapter
	r.instances[name] = NewUnifiedProvider(config)
}

func (r *Registry) GetProvider(name string) (*UnifiedProvider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.instances[name]
	if !ok {
		return nil, fmt.Errorf("provider not found: %s", name)
	}
	return p, nil
}

func (r *Registry) ListProviders() []ProviderInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]ProviderInfo, 0, len(r.providers))
	for _, info := range r.providers {
		result = append(result, info)
	}
	return result
}

func (r *Registry) FromLLMsVerifier(ctx context.Context, verifierURL string) error {
	// Stub implementation - would query LLMsVerifier API in real implementation
	return nil
}