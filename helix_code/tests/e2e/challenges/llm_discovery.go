package challenges

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// ModelInfo represents detailed information about a model
type ModelInfo struct {
	ID           string   `json:"id"`
	Name         string   `json:"name,omitempty"`
	Created      int64    `json:"created,omitempty"`
	OwnedBy      string   `json:"owned_by,omitempty"`
	Capabilities []string `json:"capabilities,omitempty"`
	Description  string   `json:"description,omitempty"`
}

// ModelCache stores discovered models with expiration
type ModelCache struct {
	Models    []ModelInfo
	Timestamp time.Time
	TTL       time.Duration
}

// ModelDiscovery handles dynamic model discovery from provider APIs
type ModelDiscovery struct {
	apiKeys *APIKeys
	cache   map[LLMProviderType]*ModelCache
	mu      sync.RWMutex
	client  *http.Client
}

// NewModelDiscovery creates a new model discovery instance
func NewModelDiscovery(apiKeys *APIKeys) *ModelDiscovery {
	return &ModelDiscovery{
		apiKeys: apiKeys,
		cache:   make(map[LLMProviderType]*ModelCache),
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// DiscoverModels fetches available models from a provider's API
func (md *ModelDiscovery) DiscoverModels(ctx context.Context, provider LLMProviderType) ([]ModelInfo, error) {
	// Check cache first
	md.mu.RLock()
	cached, exists := md.cache[provider]
	md.mu.RUnlock()

	if exists && time.Since(cached.Timestamp) < cached.TTL {
		return cached.Models, nil
	}

	// Fetch models based on provider
	var models []ModelInfo
	var err error

	switch provider {
	case ProviderOpenAI:
		models, err = md.discoverOpenAIModels(ctx)
	case ProviderXAI:
		models, err = md.discoverXAIModels(ctx)
	case ProviderDeepSeek:
		models, err = md.discoverDeepSeekModels(ctx)
	case ProviderAnthropic:
		models, err = md.discoverAnthropicModels(ctx)
	case ProviderGroq:
		models, err = md.discoverGroqModels(ctx)
	case ProviderOllama:
		models, err = md.discoverOllamaModels(ctx)
	default:
		return nil, fmt.Errorf("dynamic discovery not supported for provider: %s", provider)
	}

	if err != nil {
		return nil, err
	}

	// Cache the results (24 hour TTL)
	md.mu.Lock()
	md.cache[provider] = &ModelCache{
		Models:    models,
		Timestamp: time.Now(),
		TTL:       24 * time.Hour,
	}
	md.mu.Unlock()

	return models, nil
}

// discoverOpenAIModels fetches models from OpenAI API
func (md *ModelDiscovery) discoverOpenAIModels(ctx context.Context) ([]ModelInfo, error) {
	apiKey, err := md.apiKeys.GetAPIKey(ProviderOpenAI)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.openai.com/v1/models", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))

	resp, err := md.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("OpenAI API error (status %d): %s", resp.StatusCode, string(body))
	}

	var apiResp struct {
		Data []struct {
			ID      string `json:"id"`
			OwnedBy string `json:"owned_by"`
			Created int64  `json:"created"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, err
	}

	models := make([]ModelInfo, 0, len(apiResp.Data))
	for _, m := range apiResp.Data {
		models = append(models, ModelInfo{
			ID:      m.ID,
			OwnedBy: m.OwnedBy,
			Created: m.Created,
		})
	}

	return models, nil
}

// discoverXAIModels fetches models from xAI API (OpenAI-compatible)
func (md *ModelDiscovery) discoverXAIModels(ctx context.Context) ([]ModelInfo, error) {
	apiKey, err := md.apiKeys.GetAPIKey(ProviderXAI)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.x.ai/v1/models", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))

	resp, err := md.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("xAI API error (status %d): %s", resp.StatusCode, string(body))
	}

	var apiResp struct {
		Data []struct {
			ID      string `json:"id"`
			OwnedBy string `json:"owned_by"`
			Created int64  `json:"created"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, err
	}

	models := make([]ModelInfo, 0, len(apiResp.Data))
	for _, m := range apiResp.Data {
		models = append(models, ModelInfo{
			ID:      m.ID,
			OwnedBy: m.OwnedBy,
			Created: m.Created,
		})
	}

	return models, nil
}

// discoverDeepSeekModels fetches models from DeepSeek API (OpenAI-compatible)
func (md *ModelDiscovery) discoverDeepSeekModels(ctx context.Context) ([]ModelInfo, error) {
	apiKey, err := md.apiKeys.GetAPIKey(ProviderDeepSeek)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.deepseek.com/v1/models", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))

	resp, err := md.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("DeepSeek API error (status %d): %s", resp.StatusCode, string(body))
	}

	var apiResp struct {
		Data []struct {
			ID      string `json:"id"`
			OwnedBy string `json:"owned_by"`
			Created int64  `json:"created"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, err
	}

	models := make([]ModelInfo, 0, len(apiResp.Data))
	for _, m := range apiResp.Data {
		models = append(models, ModelInfo{
			ID:      m.ID,
			OwnedBy: m.OwnedBy,
			Created: m.Created,
		})
	}

	return models, nil
}

// discoverAnthropicModels fetches models from Anthropic API
func (md *ModelDiscovery) discoverAnthropicModels(ctx context.Context) ([]ModelInfo, error) {
	// Anthropic doesn't have a models endpoint, return hardcoded list
	// This is intentional as Anthropic's API doesn't expose model listing
	return []ModelInfo{
		{ID: "claude-3-opus-20240229", Name: "Claude 3 Opus"},
		{ID: "claude-3-sonnet-20240229", Name: "Claude 3 Sonnet"},
		{ID: "claude-3-haiku-20240307", Name: "Claude 3 Haiku"},
		{ID: "claude-2.1", Name: "Claude 2.1"},
		{ID: "claude-2.0", Name: "Claude 2.0"},
		{ID: "claude-instant-1.2", Name: "Claude Instant"},
	}, nil
}

// discoverGroqModels fetches models from Groq API (OpenAI-compatible)
func (md *ModelDiscovery) discoverGroqModels(ctx context.Context) ([]ModelInfo, error) {
	apiKey, err := md.apiKeys.GetAPIKey(ProviderGroq)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.groq.com/openai/v1/models", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))

	resp, err := md.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Groq API error (status %d): %s", resp.StatusCode, string(body))
	}

	var apiResp struct {
		Data []struct {
			ID      string `json:"id"`
			OwnedBy string `json:"owned_by"`
			Created int64  `json:"created"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, err
	}

	models := make([]ModelInfo, 0, len(apiResp.Data))
	for _, m := range apiResp.Data {
		models = append(models, ModelInfo{
			ID:      m.ID,
			OwnedBy: m.OwnedBy,
			Created: m.Created,
		})
	}

	return models, nil
}

// discoverOllamaModels fetches models from local Ollama instance
func (md *ModelDiscovery) discoverOllamaModels(ctx context.Context) ([]ModelInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "http://localhost:11434/api/tags", nil)
	if err != nil {
		return nil, err
	}

	resp, err := md.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Ollama API error (status %d): %s", resp.StatusCode, string(body))
	}

	var apiResp struct {
		Models []struct {
			Name       string `json:"name"`
			ModifiedAt string `json:"modified_at"`
			Size       int64  `json:"size"`
		} `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, err
	}

	models := make([]ModelInfo, 0, len(apiResp.Models))
	for _, m := range apiResp.Models {
		models = append(models, ModelInfo{
			ID:   m.Name,
			Name: m.Name,
		})
	}

	return models, nil
}

// GetModelIDs extracts just the model IDs from ModelInfo slice
func GetModelIDs(models []ModelInfo) []string {
	ids := make([]string, len(models))
	for i, m := range models {
		ids[i] = m.ID
	}
	return ids
}

// ClearCache clears the model cache for a specific provider or all providers
func (md *ModelDiscovery) ClearCache(provider *LLMProviderType) {
	md.mu.Lock()
	defer md.mu.Unlock()

	if provider == nil {
		// Clear all caches
		md.cache = make(map[LLMProviderType]*ModelCache)
	} else {
		// Clear specific provider cache
		delete(md.cache, *provider)
	}
}
