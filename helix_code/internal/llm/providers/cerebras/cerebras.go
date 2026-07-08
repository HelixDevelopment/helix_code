// Package cerebras hosts the Cerebras Cloud provider implementation as a
// dedicated sub-package of internal/llm.
//
// Speed programme P5-T02 (R1 B21, R3 §8.4): internal/llm was a single
// ~30k-LOC / ~75-file package, so any 1-line edit to one provider
// recompiled the whole package. Extracting cohesive provider
// implementations into sub-packages shrinks the incremental-compile unit
// — a Cerebras edit now recompiles only this small package, not all of
// internal/llm. This is a PURE structural move: ZERO behaviour change.
//
// Cerebras's constructor is NOT referenced by internal/llm/factory.go
// (the central provider switch), so moving it out introduces NO import
// cycle: this package imports dev.helix.code/internal/llm for the shared
// Provider-interface types, and internal/llm never imports back.
package cerebras

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

	"dev.helix.code/internal/llm"
	"dev.helix.code/internal/providers/httpclient"
	"github.com/google/uuid"
)

// Compile-time assertion that *Provider satisfies the shared
// llm.Provider interface — this is the structural-move no-regression
// guard for P5-T02: if a future edit drops a method the build fails here
// rather than at a distant caller.
var _ llm.Provider = (*Provider)(nil)

// Provider implements the llm.Provider interface for Cerebras Cloud
// (https://inference.cerebras.ai/). The Cerebras Cloud Chat Completions
// API is officially OpenAI-compatible (see
// https://inference-docs.cerebras.ai/api-reference/chat-completions), so
// this provider thin-wraps the OpenAI message + response shapes already
// declared in internal/llm/openai_provider.go and reuses the round-46
// llm.MapOpenAIFinishReasonToErr helper for LLMResponse.Err wiring.
//
// Round-63 §11.4 anti-bluff close-out: this provider landed
// LLMResponse.Err coverage for the final provider in the round-46
// 17-provider deferred list (17/17 = 100% coverage). Real Cerebras SDK
// integration (proper model discovery, dedicated tokenizer, streaming
// SSE keep-alive handling) remains future work per the round-63 commit
// body; today it talks to Cerebras Cloud over the OpenAI-compat
// endpoint exactly as Qwen, Copilot, xAI, OpenRouter, and Azure do.
//
// Speed programme P5-T02: this type was named CerebrasProvider while it
// lived in package llm; in this dedicated sub-package the redundant
// "Cerebras" prefix is dropped (Go convention — package name already
// disambiguates: cerebras.Provider). The constructor likewise becomes
// cerebras.NewProvider. The only external caller of the old
// llm.NewCerebrasProvider symbol was the round-63 test file, updated in
// the same commit.
type Provider struct {
	config     llm.ProviderConfigEntry
	endpoint   string
	apiKey     string
	httpClient *http.Client
	models     []llm.ModelInfo
	lastHealth *llm.ProviderHealth
}

// NewProvider creates a new Cerebras provider. The endpoint defaults to
// https://api.cerebras.ai/v1 (the official OpenAI-compat base URL) when
// ProviderConfigEntry.Endpoint is empty. The API key is read from the
// config first, then the CEREBRAS_API_KEY environment variable. An empty
// key surfaces a configuration error so callers can fall back to a
// different provider per the CONST-039 multi-provider mandate.
func NewProvider(config llm.ProviderConfigEntry) (*Provider, error) {
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

	provider := &Provider{
		config:   config,
		endpoint: endpoint,
		apiKey:   apiKey,
		// Shared tuned HTTP/2 transport (speed programme P1-T01,
		// R1 B03 / R3 §4.7) — connection pooling only; request
		// behaviour is unchanged.
		httpClient: httpclient.NewHTTPClient(60 * time.Second),
		lastHealth: &llm.ProviderHealth{
			Status:    "unknown",
			LastCheck: time.Now(),
		},
	}

	provider.initializeModels()
	return provider, nil
}

// GetType returns the provider type.
func (cb *Provider) GetType() llm.ProviderType {
	return llm.ProviderTypeCerebras
}

// GetName returns the provider name.
func (cb *Provider) GetName() string {
	return "Cerebras"
}

// GetModels returns available models.
func (cb *Provider) GetModels() []llm.ModelInfo {
	return cb.models
}

// GetCapabilities returns provider capabilities.
func (cb *Provider) GetCapabilities() []llm.ModelCapability {
	return []llm.ModelCapability{
		llm.CapabilityTextGeneration,
		llm.CapabilityCodeGeneration,
		llm.CapabilityCodeAnalysis,
		llm.CapabilityPlanning,
		llm.CapabilityDebugging,
		llm.CapabilityRefactoring,
		llm.CapabilityTesting,
		llm.CapabilityReasoning,
	}
}

// Generate generates a response using Cerebras-hosted models.
func (cb *Provider) Generate(ctx context.Context, request *llm.LLMRequest) (*llm.LLMResponse, error) {
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
func (cb *Provider) GenerateStream(ctx context.Context, request *llm.LLMRequest, ch chan<- llm.LLMResponse) error {
	defer close(ch)

	openaiRequest, err := cb.convertToOpenAIRequest(request)
	if err != nil {
		return fmt.Errorf("failed to convert request: %v", err)
	}
	openaiRequest.Stream = true

	return cb.makeOpenAIStreamRequest(ctx, openaiRequest, ch, request.ID)
}

// IsAvailable reports whether the provider is reachable + healthy.
func (cb *Provider) IsAvailable(ctx context.Context) bool {
	health, err := cb.GetHealth(ctx)
	return err == nil && health.Status == "healthy"
}

// GetHealth returns provider health by probing the /models endpoint.
func (cb *Provider) GetHealth(ctx context.Context) (*llm.ProviderHealth, error) {
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
func (cb *Provider) Close() error {
	cb.httpClient.CloseIdleConnections()
	return nil
}

// GetContextWindow returns the model's context window size in tokens.
// Default: 128_000 — Cerebras-hosted models (gemma-4-31b, gpt-oss-120b, zai-glm-4.7) all
// advertise 128k context; safe conservative value.
func (cb *Provider) GetContextWindow() int {
	return 128_000
}

// CountTokens returns an estimated token count for text. Uses char-based
// fallback (1 token ≈ 3.5 chars) — Cerebras does not currently expose a
// tokenize endpoint.
func (cb *Provider) CountTokens(text string) (int, error) {
	return llm.CharBasedTokenCount(text)
}

// Helper methods

func (cb *Provider) initializeModels() {
	// Predefined Cerebras-hosted models. Real model discovery via /models
	// happens on GetHealth; this static seed keeps the provider operable
	// before any /models round-trip and matches the round-46 pattern used
	// by Qwen and Copilot.
	cb.models = []llm.ModelInfo{
		{
			Name:        "gemma-4-31b",
			Provider:    llm.ProviderTypeCerebras,
			ContextSize: 128000,
			MaxTokens:   8192,
			Description: "Google Gemma 4 31B on Cerebras CS-3 -- frontier-class reasoning and instruction following",
		},
		{
			Name:        "gpt-oss-120b",
			Provider:    llm.ProviderTypeCerebras,
			ContextSize: 128000,
			MaxTokens:   8192,
			Description: "Cerebras GPT-OSS 120B -- open-source GPT-class model trained on CS-3",
		},
		{
			Name:        "zai-glm-4.7",
			Provider:    llm.ProviderTypeCerebras,
			ContextSize: 128000,
			MaxTokens:   8192,
			Description: "Zhipu AI GLM 4.7 on Cerebras CS-3 -- bilingual Chinese-English model",
		},
	}

	for i := range cb.models {
		llm.EnrichModelInfo(&cb.models[i])
	}

	log.Printf("Cerebras provider initialized with %d models", len(cb.models))
}

func (cb *Provider) convertToOpenAIRequest(request *llm.LLMRequest) (*llm.OpenAIRequest, error) {
	var messages []llm.OpenAIMessage
	for _, msg := range request.Messages {
		openaiMsg := llm.OpenAIMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
		if msg.Name != "" {
			openaiMsg.Name = msg.Name
		}
		messages = append(messages, openaiMsg)
	}

	return &llm.OpenAIRequest{
		Model:       request.Model,
		Messages:    messages,
		MaxTokens:   request.MaxTokens,
		Temperature: request.Temperature,
		TopP:        request.TopP,
		Stream:      request.Stream,
	}, nil
}

func (cb *Provider) convertFromOpenAIResponse(openaiResp *llm.OpenAIResponse, requestID uuid.UUID, processingTime time.Duration) *llm.LLMResponse {
	var content string
	var finishReason string

	if len(openaiResp.Choices) > 0 {
		choice := openaiResp.Choices[0]
		content = choice.Message.Content
		finishReason = choice.FinishReason
	}

	resp := &llm.LLMResponse{
		ID:        uuid.New(),
		RequestID: requestID,
		Content:   content,
		Usage: llm.Usage{
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
	// reuse llm.MapOpenAIFinishReasonToErr verbatim. If Cerebras adds a vendor-
	// specific finish_reason value, this MUST be replaced with a
	// Cerebras-specific helper in the same commit.
	resp.Err = llm.MapOpenAIFinishReasonToErr(finishReason)
	return resp
}

func (cb *Provider) makeOpenAIRequest(ctx context.Context, request *llm.OpenAIRequest) (*llm.OpenAIResponse, error) {
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

	var response llm.OpenAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	return &response, nil
}

func (cb *Provider) makeOpenAIStreamRequest(ctx context.Context, request *llm.OpenAIRequest, ch chan<- llm.LLMResponse, requestID uuid.UUID) error {
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
		var streamResp llm.OpenAIStreamResponse
		if err := decoder.Decode(&streamResp); err != nil {
			return err
		}

		if len(streamResp.Choices) > 0 {
			choice := streamResp.Choices[0]
			if choice.Delta.Content != "" {
				response := llm.LLMResponse{
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
				if errSentinel := llm.MapOpenAIFinishReasonToErr(choice.FinishReason); errSentinel != nil {
					select {
					case ch <- llm.LLMResponse{
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

func (cb *Provider) setAuthHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+cb.apiKey)
}

func (cb *Provider) updateHealth(status string, latency time.Duration, errorCount int) {
	cb.lastHealth.Status = status
	cb.lastHealth.Latency = latency
	cb.lastHealth.ErrorCount = errorCount
	cb.lastHealth.LastCheck = time.Now()
}

// Note: OpenAI API types (llm.OpenAIRequest, llm.OpenAIMessage,
// llm.OpenAIResponse, llm.OpenAIStreamResponse) are reused from
// internal/llm/openai_provider.go since Cerebras Cloud is
// OpenAI-compatible.
