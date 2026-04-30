package verifier

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Client is the HTTP REST API client for LLMsVerifier.
// It mirrors HelixAgent's pkg/sdk/go/verifier/client.go pattern.
// Uses stdlib net/http — no external dependency needed.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	timeout    time.Duration
}

// NewClient creates a verifier API client.
// baseURL: verifier service URL (e.g., "http://localhost:8081")
// apiKey: optional bearer token for authenticated endpoints
// timeout: HTTP client timeout (0 = default 30s)
func NewClient(baseURL, apiKey string, timeout time.Duration) *Client {
	if baseURL == "" {
		baseURL = "http://localhost:8081"
	}
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return &Client{
		baseURL:    baseURL,
		apiKey:     apiKey,
		timeout:    timeout,
		httpClient: &http.Client{Timeout: timeout},
	}
}

// WithHTTPClient allows injecting a custom HTTP client (for testing).
func (c *Client) WithHTTPClient(hc *http.Client) *Client {
	c.httpClient = hc
	return c
}

// Health checks the verifier service health endpoint.
func (c *Client) Health(ctx context.Context) (*HealthResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/health", nil)
	if err != nil {
		return nil, fmt.Errorf("verifier health: create request: %w", err)
	}
	c.setAuthHeader(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("verifier health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("verifier health: HTTP %d", resp.StatusCode)
	}

	var hr HealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&hr); err != nil {
		return nil, fmt.Errorf("verifier health: decode: %w", err)
	}
	return &hr, nil
}

// GetModels fetches all verified models from the verifier.
func (c *Client) GetModels(ctx context.Context) ([]*VerifiedModel, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/models", nil)
	if err != nil {
		return nil, err
	}
	c.setAuthHeader(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch models from verifier: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("verifier models: HTTP %d", resp.StatusCode)
	}

	var models []*VerifiedModel
	if err := json.NewDecoder(resp.Body).Decode(&models); err != nil {
		return nil, fmt.Errorf("failed to decode models: %w", err)
	}
	return models, nil
}

// GetModelByID fetches a single model by ID.
func (c *Client) GetModelByID(ctx context.Context, modelID string) (*VerifiedModel, error) {
	url := fmt.Sprintf("%s/api/models/%s", c.baseURL, modelID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	c.setAuthHeader(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch model %s: %w", modelID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("model %s not found", modelID)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("verifier model get: HTTP %d", resp.StatusCode)
	}

	var model VerifiedModel
	if err := json.NewDecoder(resp.Body).Decode(&model); err != nil {
		return nil, fmt.Errorf("failed to decode model: %w", err)
	}
	return &model, nil
}

// GetProviderScores fetches all provider scores.
func (c *Client) GetProviderScores(ctx context.Context) (map[string]float64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/scores", nil)
	if err != nil {
		return nil, err
	}
	c.setAuthHeader(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch scores: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("verifier scores: HTTP %d", resp.StatusCode)
	}

	var scores map[string]float64
	if err := json.NewDecoder(resp.Body).Decode(&scores); err != nil {
		return nil, fmt.Errorf("failed to decode scores: %w", err)
	}
	return scores, nil
}

// VerifyModel triggers on-demand verification for a model.
func (c *Client) VerifyModel(ctx context.Context, modelID string) (*VerificationResult, error) {
	url := fmt.Sprintf("%s/api/models/%s/verify", c.baseURL, modelID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return nil, err
	}
	c.setAuthHeader(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to verify model %s: %w", modelID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return nil, fmt.Errorf("verifier verify: HTTP %d", resp.StatusCode)
	}

	var result VerificationResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode verification result: %w", err)
	}
	return &result, nil
}

// GetPricing fetches token pricing data.
func (c *Client) GetPricing(ctx context.Context) ([]map[string]interface{}, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/pricing", nil)
	if err != nil {
		return nil, err
	}
	c.setAuthHeader(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch pricing: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("verifier pricing: HTTP %d", resp.StatusCode)
	}

	var pricing []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&pricing); err != nil {
		return nil, fmt.Errorf("failed to decode pricing: %w", err)
	}
	return pricing, nil
}

// GetLimits fetches rate limit data.
func (c *Client) GetLimits(ctx context.Context) ([]map[string]interface{}, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/limits", nil)
	if err != nil {
		return nil, err
	}
	c.setAuthHeader(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch limits: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("verifier limits: HTTP %d", resp.StatusCode)
	}

	var limits []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&limits); err != nil {
		return nil, fmt.Errorf("failed to decode limits: %w", err)
	}
	return limits, nil
}

// setAuthHeader adds the Authorization header if API key is configured.
func (c *Client) setAuthHeader(req *http.Request) {
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "HelixCode-VerifierClient/1.0")
}
