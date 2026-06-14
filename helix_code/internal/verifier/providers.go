package verifier

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// providers.go — GetProviders exposes the REAL LLMsVerifier /api/providers
// envelope as structured VerifierProvider records. This is the DYNAMIC single
// source of truth (CONST-036) for each provider's base URL (api_url) + model
// list — replacing any hardcoded base URL on the primary provider-construction
// path (CONST-046: no hardcoded URL/model in the primary path).
//
// CONST-042/§12.1 no-secret-leak: this file carries provider METADATA only
// (name + public base URL + model ids + status) — never any credential.

// VerifierProvider is one provider record from /api/providers. The api_url is
// the load-bearing field: it is the provider's base URL as the verifier knows
// it, consumed by the dynamic catalogue INSTEAD of a hardcoded literal.
type VerifierProvider struct {
	Name     string   // canonical provider name, e.g. "cerebras"
	APIURL   string   // base URL from the verifier (api_url)
	Endpoint string   // chat endpoint path, when the verifier provides one
	Models   []string // model ids the verifier lists (empty when served as a count)
	IsActive bool     // is_active flag
	Status   string   // status string ("active"/"inactive"/…)
}

// GetProviders fetches the full provider records from the real LLMsVerifier
// /api/providers endpoint. It returns an error (never a silent empty slice) when
// the verifier is unreachable or returns a non-200, so callers can deterministically
// gate a degraded fallback on reachability.
func (c *Client) GetProviders(ctx context.Context) ([]VerifierProvider, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/providers", nil)
	if err != nil {
		return nil, fmt.Errorf("verifier providers: create request: %w", err)
	}
	c.setAuthHeader(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch providers: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("verifier providers: HTTP %d", resp.StatusCode)
	}

	// The real server emits `api_url` for the base URL and may emit `models`
	// either as a JSON ARRAY of model ids OR as an integer COUNT. Decode
	// `models` permissively (json.RawMessage) so an integer count does not
	// break parsing — only an array populates the Models slice.
	var envelope struct {
		Providers []struct {
			Name             string          `json:"name"`
			APIURL           string          `json:"api_url"`
			Endpoint         string          `json:"endpoint"`
			Models           json.RawMessage `json:"models"`
			IsActive         bool            `json:"is_active"`
			Status           string          `json:"status"`
			ReliabilityScore float64         `json:"reliability_score"`
		} `json:"providers"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("failed to decode providers: %w", err)
	}

	out := make([]VerifierProvider, 0, len(envelope.Providers))
	for _, p := range envelope.Providers {
		if p.Name == "" {
			continue
		}
		out = append(out, VerifierProvider{
			Name:     p.Name,
			APIURL:   p.APIURL,
			Endpoint: p.Endpoint,
			Models:   decodeProviderModels(p.Models),
			IsActive: p.IsActive,
			Status:   p.Status,
		})
	}
	return out, nil
}

// decodeProviderModels parses the `models` field of a provider record. It is an
// array of model ids on builds that list them, or an integer count on builds
// that summarise — only the array form yields a populated slice; the count form
// (and null/absent) yields an empty slice with no error.
func decodeProviderModels(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var ids []string
	if err := json.Unmarshal(raw, &ids); err == nil {
		return ids
	}
	return nil
}
