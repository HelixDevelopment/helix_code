package llm

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// OllamaProvider implements the LLM provider interface for Ollama
type OllamaProvider struct {
	config    OllamaConfig
	apiClient *http.Client
	models    []OllamaModel
	isRunning bool
}

// OllamaConfig holds configuration for Ollama
type OllamaConfig struct {
	BaseURL       string        `json:"base_url"`
	DefaultModel  string        `json:"default_model"`
	Timeout       time.Duration `json:"timeout"`
	KeepAlive     time.Duration `json:"keep_alive"`
	StreamEnabled bool          `json:"stream_enabled"`
}

// OllamaModel represents an Ollama model
type OllamaModel struct {
	Name       string             `json:"name"`
	ModifiedAt time.Time          `json:"modified_at"`
	Size       int64              `json:"size"`
	Digest     string             `json:"digest"`
	Details    OllamaModelDetails `json:"details"`
}

// OllamaModelDetails contains model-specific details
type OllamaModelDetails struct {
	Format            string   `json:"format"`
	Family            string   `json:"family"`
	Families          []string `json:"families"`
	ParameterSize     string   `json:"parameter_size"`
	QuantizationLevel string   `json:"quantization_level"`
}

// OllamaAPIRequest represents a request to the Ollama API
type OllamaAPIRequest struct {
	Model    string                 `json:"model"`
	Prompt   string                 `json:"prompt"`
	Messages []Message              `json:"messages"`
	Stream   bool                   `json:"stream"`
	Options  map[string]interface{} `json:"options"`
}

// OllamaAPIResponse represents a response from the Ollama API
type OllamaAPIResponse struct {
	Model              string `json:"model"`
	CreatedAt          string `json:"created_at"`
	Response           string `json:"response"`
	Done               bool   `json:"done"`
	// DoneReason is set by Ollama 0.1.30+ on the terminal frame and
	// indicates why generation stopped: "stop" (clean end-of-turn),
	// "length" (num_predict / max-tokens reached → ErrResponseTruncated),
	// "load" / "unload" (model lifecycle), "" (older Ollama versions).
	// Round-46 LLMResponse.Err wiring consumes this field; older versions
	// fall through to no Err since the truncation signal is not knowable
	// from the on-wire payload.
	DoneReason         string `json:"done_reason,omitempty"`
	Context            []int  `json:"context"`
	TotalDuration      int64  `json:"total_duration"`
	LoadDuration       int64  `json:"load_duration"`
	PromptEvalCount    int    `json:"prompt_eval_count"`
	PromptEvalDuration int64  `json:"prompt_eval_duration"`
	EvalCount          int    `json:"eval_count"`
	EvalDuration       int64  `json:"eval_duration"`
}

// mapOllamaDoneReasonToErr returns the round-46 sentinel matching an
// Ollama done_reason, or nil if the reason indicates a clean stop or
// is empty (older Ollama versions). Ollama exposes no content-filter
// hook on the wire, so ErrResponseContentBlocked is not reachable here.
// Reference: https://github.com/ollama/ollama/blob/main/docs/api.md
func mapOllamaDoneReasonToErr(reason string) error {
	switch reason {
	case "length":
		return ErrResponseTruncated
	default:
		return nil
	}
}

// NewOllamaProvider creates a new Ollama provider
func NewOllamaProvider(config OllamaConfig) (*OllamaProvider, error) {
	provider := &OllamaProvider{
		config: config,
		apiClient: &http.Client{
			Timeout: config.Timeout,
		},
		isRunning: true,
	}

	// Discover available models
	if err := provider.discoverModels(); err != nil {
		log.Printf("Warning: Failed to discover Ollama models: %v", err)
	}

	log.Printf("✅ Ollama provider initialized with %d models", len(provider.models))
	return provider, nil
}

// GetType returns the provider type
func (p *OllamaProvider) GetType() ProviderType {
	return ProviderTypeLocal
}

// GetName returns the provider name
func (p *OllamaProvider) GetName() string {
	return "ollama"
}

// GetModels returns available models
func (p *OllamaProvider) GetModels() []ModelInfo {
	var modelInfos []ModelInfo

	for _, model := range p.models {
		modelInfos = append(modelInfos, ModelInfo{
			Name:        model.Name,
			Provider:    ProviderTypeLocal,
			ContextSize: 4096, // Default context size
			MaxTokens:   4096,
			Description: fmt.Sprintf("Ollama model: %s", model.Name),
		})
	}

	for i := range modelInfos {
		EnrichModelInfo(&modelInfos[i])
	}

	return modelInfos
}

// GetCapabilities returns model capabilities
func (p *OllamaProvider) GetCapabilities() []ModelCapability {
	return []ModelCapability{
		CapabilityTextGeneration,
		CapabilityCodeGeneration,
		CapabilityCodeAnalysis,
		CapabilityPlanning,
		CapabilityDebugging,
		CapabilityRefactoring,
		CapabilityTesting,
	}
}

// Generate generates a response using Ollama
func (p *OllamaProvider) Generate(ctx context.Context, request *LLMRequest) (*LLMResponse, error) {
	if !p.isRunning {
		return nil, ErrProviderUnavailable
	}

	// Prepare API request
	apiRequest := OllamaAPIRequest{
		Model:    p.getModelName(request.Model),
		Messages: request.Messages,
		Stream:   false,
		Options: map[string]interface{}{
			"temperature": request.Temperature,
			"top_p":       request.TopP,
			"num_predict": request.MaxTokens,
		},
	}

	// Make API call
	startTime := time.Now()
	response, err := p.makeAPIRequest(ctx, apiRequest)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}

	processingTime := time.Since(startTime)

	return &LLMResponse{
		ID:        uuid.New(),
		RequestID: request.ID,
		Content:   response.Response,
		Usage: Usage{
			PromptTokens:     response.PromptEvalCount,
			CompletionTokens: response.EvalCount,
			TotalTokens:      response.PromptEvalCount + response.EvalCount,
		},
		FinishReason:   response.DoneReason,
		ProcessingTime: processingTime,
		CreatedAt:      time.Now(),
		// Round-46 LLMResponse.Err wiring (CONST-035 / Article XI §11.9):
		// Ollama signals truncation via done_reason="length". See
		// mapOllamaDoneReasonToErr doc comment for the closed mapping.
		Err: mapOllamaDoneReasonToErr(response.DoneReason),
	}, nil
}

// GenerateStream generates a streaming response
func (p *OllamaProvider) GenerateStream(ctx context.Context, request *LLMRequest, ch chan<- LLMResponse) error {
	if !p.isRunning {
		return ErrProviderUnavailable
	}

	// Prepare streaming API request
	apiRequest := OllamaAPIRequest{
		Model:    p.getModelName(request.Model),
		Messages: request.Messages,
		Stream:   true,
		Options: map[string]interface{}{
			"temperature": request.Temperature,
			"top_p":       request.TopP,
			"num_predict": request.MaxTokens,
		},
	}

	// Make streaming request
	return p.makeStreamingRequest(ctx, apiRequest, ch)
}

// IsAvailable checks if the provider is available
func (p *OllamaProvider) IsAvailable(ctx context.Context) bool {
	if !p.isRunning {
		return false
	}

	// Test API endpoint
	resp, err := p.apiClient.Get(p.getAPIURL("/api/tags"))
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// GetHealth returns provider health status
func (p *OllamaProvider) GetHealth(ctx context.Context) (*ProviderHealth, error) {
	if !p.isRunning {
		return &ProviderHealth{
			Status:     "unhealthy",
			LastCheck:  time.Now(),
			ErrorCount: 1,
		}, nil
	}

	// Test API endpoint
	start := time.Now()
	resp, err := p.apiClient.Get(p.getAPIURL("/api/tags"))
	latency := time.Since(start)

	if err != nil || resp.StatusCode != http.StatusOK {
		return &ProviderHealth{
			Status:     "degraded",
			Latency:    latency,
			LastCheck:  time.Now(),
			ErrorCount: 1,
			ModelCount: len(p.models),
		}, nil
	}

	return &ProviderHealth{
		Status:     "healthy",
		Latency:    latency,
		LastCheck:  time.Now(),
		ErrorCount: 0,
		ModelCount: len(p.models),
	}, nil
}

// Close stops the Ollama provider
func (p *OllamaProvider) Close() error {
	p.isRunning = false
	log.Println("✅ Ollama provider closed")
	return nil
}

// GetContextWindow returns the model's context window size in tokens.
// Default: 200_000 — local Ollama models vary; 200k is a safe upper bound
// pending a Phase 3 upgrade to query /api/show for the actual context length.
func (p *OllamaProvider) GetContextWindow() int {
	return 200_000
}

// CountTokens returns an estimated token count for text.
// Uses char-based fallback (1 token ≈ 3.5 chars) — Phase 3 will upgrade
// to query the Ollama tokenize endpoint when available.
func (p *OllamaProvider) CountTokens(text string) (int, error) {
	return CharBasedTokenCount(text)
}

// Private helper methods

func (p *OllamaProvider) discoverModels() error {
	resp, err := p.apiClient.Get(p.getAPIURL("/api/tags"))
	if err != nil {
		return fmt.Errorf("failed to fetch models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var response struct {
		Models []OllamaModel `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return fmt.Errorf("failed to decode models response: %w", err)
	}

	p.models = response.Models
	return nil
}

func (p *OllamaProvider) getModelName(requestedModel string) string {
	if requestedModel != "" {
		return requestedModel
	}

	if p.config.DefaultModel != "" {
		return p.config.DefaultModel
	}

	// Return first available model
	if len(p.models) > 0 {
		return p.models[0].Name
	}

	return "llama2" // Fallback default
}

func (p *OllamaProvider) getAPIURL(path string) string {
	baseURL := p.config.BaseURL
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}

	return strings.TrimSuffix(baseURL, "/") + path
}

func (p *OllamaProvider) makeAPIRequest(ctx context.Context, request OllamaAPIRequest) (*OllamaAPIResponse, error) {
	url := p.getAPIURL("/api/chat")

	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(requestBody)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := p.apiClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var response OllamaAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response, nil
}

func (p *OllamaProvider) makeStreamingRequest(ctx context.Context, request OllamaAPIRequest, ch chan<- LLMResponse) error {
	// Ollama streaming wire-format: NDJSON (newline-delimited JSON). Each line
	// is a complete OllamaAPIResponse; the terminal frame carries Done=true.
	// We line-iterate via bufio.Scanner and forward each chunk as a separate
	// LLMResponse so consumers see incremental output (round-33 §11.4 fix —
	// previous body only decoded ONE frame, fabricating streaming for callers
	// while silently discarding subsequent chunks; CONST-035 / Article XI §11.9).
	url := p.getAPIURL("/api/chat")

	requestBody, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(requestBody)))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/x-ndjson")

	resp, err := p.apiClient.Do(req)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	scanner := bufio.NewScanner(resp.Body)
	// Ollama chunks can carry full prompt-eval echoes; raise the buffer cap.
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var chunk OllamaAPIResponse
		if err := json.Unmarshal(line, &chunk); err != nil {
			return fmt.Errorf("failed to decode NDJSON chunk: %w", err)
		}
		// ANTI-BLUFF: Send REAL streaming chunk from Ollama (one per NDJSON line).
		// Round-46 LLMResponse.Err wiring (CONST-035 / Article XI §11.9):
		// the terminal frame (chunk.Done==true) carries done_reason; map
		// it into Err so downstream stream consumers can distinguish a
		// clean stop from truncation. FinishReason is also preserved.
		out := LLMResponse{
			ID:           uuid.New(),
			Content:      chunk.Response,
			CreatedAt:    time.Now(),
			FinishReason: chunk.DoneReason,
		}
		if chunk.Done {
			out.Err = mapOllamaDoneReasonToErr(chunk.DoneReason)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ch <- out:
		}
		if chunk.Done {
			break
		}
	}
	if err := scanner.Err(); err != nil && err != io.EOF {
		return fmt.Errorf("streaming read failed: %w", err)
	}

	return nil
}
