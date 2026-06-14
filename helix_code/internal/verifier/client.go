package verifier

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
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

	// The real LLMsVerifier server emits `timestamp` as a Unix integer
	// (api/handlers.go HealthHandler), while the embedded server and the
	// HelixCode HealthResponse model it as an RFC3339 time.Time. Decode the
	// timestamp permissively (json.Number) so BOTH shapes parse, then
	// normalise into HealthResponse.
	var raw struct {
		Status    string          `json:"status"`
		Version   string          `json:"version"`
		Timestamp json.RawMessage `json:"timestamp"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("verifier health: decode: %w", err)
	}
	hr := HealthResponse{Status: raw.Status, Version: raw.Version}
	hr.Timestamp = parseFlexibleTime(raw.Timestamp)
	return &hr, nil
}

// parseFlexibleTime parses a JSON timestamp that may be either a Unix integer
// (real LLMsVerifier server) or an RFC3339 string (embedded server / time.Time).
func parseFlexibleTime(raw json.RawMessage) time.Time {
	if len(raw) == 0 {
		return time.Time{}
	}
	// Try integer (Unix seconds).
	var unix int64
	if err := json.Unmarshal(raw, &unix); err == nil {
		return time.Unix(unix, 0).UTC()
	}
	// Try RFC3339 string.
	var t time.Time
	if err := json.Unmarshal(raw, &t); err == nil {
		return t
	}
	return time.Time{}
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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read models body: %w", err)
	}
	return decodeModels(body)
}

// decodeModels parses an /api/models response that may be in EITHER shape:
//   - the embedded server's bare JSON array `[ {VerifiedModel}, ... ]`, or
//   - the real LLMsVerifier server's envelope `{"models":[...],"count":N}`
//     (api/handlers.go ListModelsHandler) whose objects use `status` for the
//     verification status (mapped into VerifiedModel.VerificationStatus).
//
// The bare-array path is attempted first so existing behaviour / tests are
// preserved; the envelope path is the new reconciliation for the real server.
func decodeModels(body []byte) ([]*VerifiedModel, error) {
	// Detect the shape by the first non-whitespace byte rather than by the
	// length of the `models` array: `[` is the legacy embedded-server bare
	// array, `{` is the real-server envelope `{"models":[...],"count":N}`.
	// Keying on `len(envelope.Models) > 0` mis-handled a legitimate "zero
	// models" envelope (`{"models":[],"count":0}` / `{"models":null}` /
	// `{"count":0}`): it fell through to the bare-array path which then tried
	// to unmarshal the whole `{...}` object as a JSON array and returned a
	// spurious decode error instead of an empty slice.
	switch firstJSONByte(body) {
	case '{':
		// Real-server envelope. Decode `models` (present-but-empty, null, or
		// absent all yield an empty slice with no error).
		var envelope struct {
			Models json.RawMessage `json:"models"`
		}
		if err := json.Unmarshal(body, &envelope); err != nil {
			return nil, fmt.Errorf("failed to decode models envelope: %w", err)
		}
		if len(envelope.Models) == 0 {
			// `models` key absent → nothing to decode.
			return []*VerifiedModel{}, nil
		}
		return unmarshalModelArray(envelope.Models)
	default:
		// Legacy embedded-server bare array (or empty body).
		return unmarshalModelArray(body)
	}
}

// firstJSONByte returns the first non-whitespace byte of body, or 0 if body is
// empty / all-whitespace. Used to discriminate a JSON object (`{`) from a JSON
// array (`[`) without a speculative full unmarshal.
func firstJSONByte(body []byte) byte {
	for _, b := range body {
		switch b {
		case ' ', '\t', '\r', '\n':
			continue
		default:
			return b
		}
	}
	return 0
}

// unmarshalModelArray decodes a JSON array of model objects, reconciling the
// real server's `status` field onto VerifiedModel.VerificationStatus (which the
// real server populates but tags differently). VerifiedModel's json tags cover
// every other field that overlaps.
func unmarshalModelArray(raw json.RawMessage) ([]*VerifiedModel, error) {
	// The real LLMsVerifier server emits `id` as the numeric DB primary key and
	// carries the string model identifier in `model_id`, whereas the embedded
	// server emits `id` as the string model identifier directly. Decode the
	// array through an alias that maps `id` to a number-or-string-tolerant raw
	// value so the numeric real-server `id` does not break VerifiedModel.ID
	// (a string). The string model identifier is then resolved from `model_id`
	// (real server) or the stringified/raw `id` (embedded server).
	type modelAlias VerifiedModel
	var rawObjs []struct {
		modelAlias
		RawID   json.RawMessage `json:"id"`
		ModelID string          `json:"model_id"`
		Status  string          `json:"status"`
	}
	if err := json.Unmarshal(raw, &rawObjs); err != nil {
		return nil, fmt.Errorf("failed to decode models: %w", err)
	}

	models := make([]*VerifiedModel, len(rawObjs))
	for i := range rawObjs {
		m := VerifiedModel(rawObjs[i].modelAlias)

		// Resolve the string model identifier into VerifiedModel.ID.
		switch {
		case rawObjs[i].ModelID != "":
			m.ID = rawObjs[i].ModelID
		case len(rawObjs[i].RawID) > 0:
			// Could be a quoted string ("gpt-4o") or a bare number (2).
			var s string
			if err := json.Unmarshal(rawObjs[i].RawID, &s); err == nil {
				m.ID = s
			} else {
				m.ID = strings.Trim(string(rawObjs[i].RawID), `"`)
			}
		}

		// Overlay the real server's `status` key (verification status) which
		// does not match VerifiedModel.VerificationStatus's tag.
		if m.VerificationStatus == "" && rawObjs[i].Status != "" {
			m.VerificationStatus = rawObjs[i].Status
		}

		models[i] = &m
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

	// The real LLMsVerifier server does NOT expose /api/scores — it serves
	// /api/providers (api/server.go). When /api/scores is absent (404/405),
	// fall back to /api/providers and derive the provider→score map there.
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusMethodNotAllowed {
		return c.getProviderScoresFromProviders(ctx)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("verifier scores: HTTP %d", resp.StatusCode)
	}

	var scores map[string]float64
	if err := json.NewDecoder(resp.Body).Decode(&scores); err != nil {
		return nil, fmt.Errorf("failed to decode scores: %w", err)
	}
	return scores, nil
}

// getProviderScoresFromProviders queries the real LLMsVerifier `/api/providers`
// endpoint and projects its `{"providers":[{name,reliability_score}],...}`
// envelope into the HelixCode provider→score map. This is the reconciliation
// for the endpoint mismatch: HelixCode historically called /api/scores, which
// the real server never served.
func (c *Client) getProviderScoresFromProviders(ctx context.Context) (map[string]float64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/providers", nil)
	if err != nil {
		return nil, err
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

	var envelope struct {
		Providers []struct {
			Name             string  `json:"name"`
			ReliabilityScore float64 `json:"reliability_score"`
			Score            float64 `json:"score"`
		} `json:"providers"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("failed to decode providers: %w", err)
	}

	scores := make(map[string]float64, len(envelope.Providers))
	for _, p := range envelope.Providers {
		if p.Name == "" {
			continue
		}
		// Prefer an explicit `score` field if present; otherwise use the
		// provider's reliability_score (the real server's provider metric).
		if p.Score != 0 {
			scores[p.Name] = p.Score
		} else {
			scores[p.Name] = p.ReliabilityScore
		}
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
