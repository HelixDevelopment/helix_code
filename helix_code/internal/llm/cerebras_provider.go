package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
)

// CerebrasProvider implements the Provider interface for Cerebras Cloud
// (https://inference.cerebras.ai/). The Cerebras Cloud Chat Completions
// API is officially OpenAI-compatible (see
// https://inference-docs.cerebras.ai/api-reference/chat-completions), so
// this provider thin-wraps the OpenAI message + response shapes already
// declared in openai_provider.go and reuses the round-46
// mapOpenAIFinishReasonToErr helper for LLMResponse.Err wiring.
//
// Round-63 §11.4 anti-bluff close-out: this file was created to land
// LLMResponse.Err coverage for the final provider in the round-46
// 17-provider deferred list (17/17 = 100% coverage). Real Cerebras SDK
// integration (proper model discovery, dedicated tokenizer, streaming
// SSE keep-alive handling) remains future work per the round-63 commit
// body; today it talks to Cerebras Cloud over the OpenAI-compat
// endpoint exactly as Qwen, Copilot, xAI, OpenRouter, and Azure do.
type CerebrasProvider struct {
	config     ProviderConfigEntry
	endpoint   string
	apiKey     string
	httpClient *http.Client
	models     []ModelInfo
	lastHealth *ProviderHealth
}

// NewCerebrasProvider creates a new Cerebras provider. The endpoint
// defaults to https://api.cerebras.ai/v1 (the official OpenAI-compat
// base URL) when ProviderConfigEntry.Endpoint is empty. The API key is
// read from the config first, then the CEREBRAS_API_KEY environment
// variable. An empty key surfaces a configuration error so callers can
// fall back to a different provider per the CONST-039 multi-provider
// mandate.
func NewCerebrasProvider(config ProviderConfigEntry) (*CerebrasProvider, error) {
	endpoint := config.Endpoint
	if endpoint == "" {
		endpoint = "https://api.cerebras.ai/v1"
	}

	apiKey := config.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("CEREBRAS_API_KEY")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("no API key available - configure CEREBRAS_API_KEY environment variable or provide APIKey in config")
	}

	provider := &CerebrasProvider{
		config:   config,
		endpoint: endpoint,
		apiKey:   apiKey,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		lastHealth: &ProviderHealth{
			Status:    "unknown",
			LastCheck: time.Now(),
		},
	}

	provider.initializeModels()
	return provider, nil
}

// GetType returns the provider type.
func (cb *CerebrasProvider) GetType() ProviderType {
	return ProviderTypeCerebras
}

// GetName returns the provider name.
func (cb *CerebrasProvider) GetName() string {
	return "Cerebras"
}

// GetModels returns available models.
func (cb *CerebrasProvider) GetModels() []ModelInfo {
	return cb.models
}

// GetCapabilities returns provider capabilities.
func (cb *CerebrasProvider) GetCapabilities() []ModelCapability {
	return []ModelCapability{
		CapabilityTextGeneration,
		CapabilityCodeGeneration,
		CapabilityCodeAnalysis,
		CapabilityPlanning,
		CapabilityDebugging,
		CapabilityRefactoring,
		CapabilityTesting,
		CapabilityReasoning,
	}
}

// Generate generates a response using Cerebras-hosted models.
func (cb *CerebrasProvider) Generate(ctx context.Context, request *LLMRequest) (*LLMResponse, error) {
	startTime := time.Now()

	openaiRequest, err := cb.convertToOpenAIRequest(request)
	if err != nil {
		return nil, fmt.Errorf("failed to convert request: %v", err)
	}

	response, err := cb.makeOpenAIRequest(ctx, openaiRequest)
	if err != nil {
		return nil, fmt.Errorf("Cerebras request failed: %v", err)
	}

	return cb.convertFromOpenAIResponse(response, request.ID, time.Since(startTime)), nil
}

// GenerateStream generates a streaming response.
func (cb *CerebrasProvider) GenerateStream(ctx context.Context, request *LLMRequest, ch chan<- LLMResponse) error {
	defer close(ch)

	openaiRequest, err := cb.convertToOpenAIRequest(request)
	if err != nil {
		return fmt.Errorf("failed to convert request: %v", err)
	}
	openaiRequest.Stream = true

	return cb.makeOpenAIStreamRequest(ctx, openaiRequest, ch, request.ID)
}

// IsAvailable reports whether the provider is reachable + healthy.
func (cb *CerebrasProvider) IsAvailable(ctx context.Context) bool {
	health, err := cb.GetHealth(ctx)
	return err == nil && health.Status == "healthy"
}

// GetHealth returns provider health by probing the /models endpoint.
func (cb *CerebrasProvider) GetHealth(ctx context.Context) (*ProviderHealth, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/models", cb.endpoint), nil)
	if err != nil {
		cb.updateHealth("unhealthy", 0, cb.lastHealth.ErrorCount+1)
		return cb.lastHealth, fmt.Errorf("failed to create health check request: %v", err)
	}

	cb.setAuthHeaders(req)

	start := time.Now()
	resp, err := cb.httpClient.Do(req)
	latency := time.Since(start)

	if err != nil {
		cb.updateHealth("unhealthy", latency, cb.lastHealth.ErrorCount+1)
		return cb.lastHealth, fmt.Errorf("health check failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		cb.updateHealth("unhealthy", latency, cb.lastHealth.ErrorCount+1)
		return cb.lastHealth, fmt.Errorf("health check returned status %d", resp.StatusCode)
	}

	var modelsResponse struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&modelsResponse); err != nil {
		cb.updateHealth("degraded", latency, cb.lastHealth.ErrorCount)
		return cb.lastHealth, nil
	}

	cb.updateHealth("healthy", latency, 0)
	cb.lastHealth.ModelCount = len(modelsResponse.Data)
	return cb.lastHealth, nil
}

// Close closes the provider.
func (cb *CerebrasProvider) Close() error {
	cb.httpClient.CloseIdleConnections()
	return nil
}

// GetContextWindow returns the model's context window size in tokens.
// Default: 128_000 — Cerebras-hosted Llama 3.1 70B and 405B both
// advertise 128k context; safe conservative value.
func (cb *CerebrasProvider) GetContextWindow() int {
	return 128_000
}

// CountTokens returns an estimated token count for text. Uses char-based
// fallback (1 token ≈ 3.5 chars) — Cerebras does not currently expose a
// tokenize endpoint.
func (cb *CerebrasProvider) CountTokens(text string) (int, error) {
	return CharBasedTokenCount(text)
}

// Helper methods

func (cb *CerebrasProvider) initializeModels() {
	// Predefined Cerebras-hosted models. Real model discovery via /models
	// happens on GetHealth; this static seed keeps the provider operable
	// before any /models round-trip and matches the round-46 pattern used
	// by Qwen and Copilot.
	cb.models = []ModelInfo{
		{
			Name:        "llama3.1-8b",
			Provider:    ProviderTypeCerebras,
			ContextSize: 128000,
			MaxTokens:   8192,
			Description: "Cerebras Llama 3.1 8B - Fast inference-optimised model",
		},
		{
			Name:        "llama3.1-70b",
			Provider:    ProviderTypeCerebras,
			ContextSize: 128000,
			MaxTokens:   8192,
			Description: "Cerebras Llama 3.1 70B - Balanced quality and speed",
		},
		{
			Name:        "llama-3.3-70b",
			Provider:    ProviderTypeCerebras,
			ContextSize: 128000,
			MaxTokens:   8192,
			Description: "Cerebras Llama 3.3 70B - Latest Meta model on Cerebras CS-3",
		},
	}

	for i := range cb.models {
		EnrichModelInfo(&cb.models[i])
	}

	log.Printf("Cerebras provider initialized with %d models", len(cb.models))
}

func (cb *CerebrasProvider) convertToOpenAIRequest(request *LLMRequest) (*OpenAIRequest, error) {
	var messages []OpenAIMessage
	for _, msg := range request.Messages {
		openaiMsg := OpenAIMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
		if msg.Name != "" {
			openaiMsg.Name = msg.Name
		}
		messages = append(messages, openaiMsg)
	}

	return &OpenAIRequest{
		Model:       request.Model,
		Messages:    messages,
		MaxTokens:   request.MaxTokens,
		Temperature: request.Temperature,
		TopP:        request.TopP,
		Stream:      request.Stream,
	}, nil
}

func (cb *CerebrasProvider) convertFromOpenAIResponse(openaiResp *OpenAIResponse, requestID uuid.UUID, processingTime time.Duration) *LLMResponse {
	var content string
	var finishReason string

	if len(openaiResp.Choices) > 0 {
		choice := openaiResp.Choices[0]
		content = choice.Message.Content
		finishReason = choice.FinishReason
	}

	resp := &LLMResponse{
		ID:        uuid.New(),
		RequestID: requestID,
		Content:   content,
		Usage: Usage{
			PromptTokens:     openaiResp.Usage.PromptTokens,
			CompletionTokens: openaiResp.Usage.CompletionTokens,
			TotalTokens:      openaiResp.Usage.TotalTokens,
		},
		FinishReason:   finishReason,
		ProcessingTime: processingTime,
		CreatedAt:      time.Now(),
	}

	// Round-63 LLMResponse.Err wiring (CONST-035 / CONST-050(A)+(B) / Article XI §11.9):
	// Cerebras Cloud's chat completions API advertises the SAME finish_reason
	// vocabulary as OpenAI ("stop", "length", "content_filter", "tool_calls")
	// per https://inference-docs.cerebras.ai/api-reference/chat-completions —
	// reuse mapOpenAIFinishReasonToErr verbatim. If Cerebras adds a vendor-
	// specific finish_reason value, this MUST be replaced with a
	// Cerebras-specific helper in the same commit.
	resp.Err = mapOpenAIFinishReasonToErr(finishReason)
	return resp
}

func (cb *CerebrasProvider) makeOpenAIRequest(ctx context.Context, request *OpenAIRequest) (*OpenAIResponse, error) {
	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/chat/completions", cb.endpoint), bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	cb.setAuthHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := cb.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Cerebras API returned status %d: %s", resp.StatusCode, string(body))
	}

	var response OpenAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	return &response, nil
}

func (cb *CerebrasProvider) makeOpenAIStreamRequest(ctx context.Context, request *OpenAIRequest, ch chan<- LLMResponse, requestID uuid.UUID) error {
	jsonData, err := json.Marshal(request)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/chat/completions", cb.endpoint), bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	cb.setAuthHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := cb.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Cerebras API returned status %d: %s", resp.StatusCode, string(body))
	}

	decoder := json.NewDecoder(resp.Body)
	for decoder.More() {
		var streamResp OpenAIStreamResponse
		if err := decoder.Decode(&streamResp); err != nil {
			return err
		}

		if len(streamResp.Choices) > 0 {
			choice := streamResp.Choices[0]
			if choice.Delta.Content != "" {
				response := LLMResponse{
					ID:        uuid.New(),
					RequestID: requestID,
					Content:   choice.Delta.Content,
					CreatedAt: time.Now(),
				}
				select {
				case ch <- response:
				case <-ctx.Done():
					return ctx.Err()
				}
			}

			// Round-63 LLMResponse.Err wiring for the streaming path
			// (CONST-035 / Article XI §11.9): emit a terminal Err-bearing
			// frame on the channel when the final chunk carries a
			// finish_reason of "length" or "content_filter" so downstream
			// stream consumers (notably tool_provider.go :201/:251) can
			// distinguish a clean stop from a partial-error stop.
			if choice.FinishReason != "" {
				if errSentinel := mapOpenAIFinishReasonToErr(choice.FinishReason); errSentinel != nil {
					select {
					case ch <- LLMResponse{
						ID:           uuid.New(),
						RequestID:    requestID,
						FinishReason: choice.FinishReason,
						CreatedAt:    time.Now(),
						Err:          errSentinel,
					}:
					case <-ctx.Done():
						return ctx.Err()
					}
				}
			}
		}

		if len(streamResp.Choices) > 0 && streamResp.Choices[0].FinishReason != "" {
			break
		}
	}

	return nil
}

func (cb *CerebrasProvider) setAuthHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+cb.apiKey)
}

func (cb *CerebrasProvider) updateHealth(status string, latency time.Duration, errorCount int) {
	cb.lastHealth.Status = status
	cb.lastHealth.Latency = latency
	cb.lastHealth.ErrorCount = errorCount
	cb.lastHealth.LastCheck = time.Now()
}

// Note: OpenAI API types (OpenAIRequest, OpenAIMessage, OpenAIResponse,
// OpenAIStreamResponse) are reused from openai_provider.go since
// Cerebras Cloud is OpenAI-compatible.
