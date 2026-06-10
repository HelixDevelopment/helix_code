package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// openai_compatible_catalog.go — shared live-catalog fetch for OpenAI-compatible
// providers (CONST-036 / F6-D-5). Mirrors the OpenRouter `fetchCatalog` pattern:
// query the provider's `GET /models` endpoint and reflect whatever the provider
// actually serves today, instead of carrying a stale hardcoded list.
//
// CONST-036: model metadata is sourced LIVE from the provider, never invented.
// Callers keep a small verified SEED list only as an offline fallback when the
// endpoint is unreachable (no network / revoked key) — clearly labelled, never
// presented as authoritative.

// openAICompatCatalogResponse is the standard OpenAI `/models` response shape,
// shared by OpenAI, DeepSeek, Mistral and other OpenAI-compatible gateways.
type openAICompatCatalogResponse struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

// fetchOpenAICompatibleCatalog GETs `<endpoint>/models` with a Bearer token and
// returns the live model list as []ModelInfo for the given provider type.
// Returns an error (and no models) when the endpoint is unreachable, returns a
// non-200, or yields an empty catalog — the caller then keeps its verified seed.
func fetchOpenAICompatibleCatalog(
	ctx context.Context,
	endpoint, apiKey string,
	httpClient *http.Client,
	pt ProviderType,
	defaultContext, defaultMaxTokens int,
) ([]ModelInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/models", endpoint), nil)
	if err != nil {
		return nil, fmt.Errorf("build /models request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET /models: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GET /models returned %d: %s", resp.StatusCode, string(body))
	}

	var cat openAICompatCatalogResponse
	if err := json.NewDecoder(resp.Body).Decode(&cat); err != nil {
		return nil, fmt.Errorf("decode /models response: %w", err)
	}
	if len(cat.Data) == 0 {
		return nil, fmt.Errorf("/models returned empty data array")
	}

	models := make([]ModelInfo, 0, len(cat.Data))
	for _, m := range cat.Data {
		if m.ID == "" {
			continue
		}
		mi := ModelInfo{
			Name:        m.ID,
			Provider:    pt,
			ContextSize: defaultContext,
			MaxTokens:   defaultMaxTokens,
			Description: fmt.Sprintf("%s model (live /models catalog)", pt),
		}
		models = append(models, mi)
	}
	if len(models) == 0 {
		return nil, fmt.Errorf("/models returned no usable model ids")
	}
	return models, nil
}
