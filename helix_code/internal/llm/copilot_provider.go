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
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

// CopilotProvider implements the Provider interface for GitHub Copilot
type CopilotProvider struct {
	config      ProviderConfigEntry
	endpoint    string
	githubToken string
	bearerToken string
	httpClient  *http.Client
	models      []ModelInfo
	lastHealth  *ProviderHealth
}

// CopilotTokenResponse represents the response from GitHub's token exchange endpoint
type CopilotTokenResponse struct {
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expires_at"`
}

// NewCopilotProvider creates a new GitHub Copilot provider
func NewCopilotProvider(config ProviderConfigEntry) (*CopilotProvider, error) {
	endpoint := config.Endpoint
	if endpoint == "" {
		endpoint = "https://api.githubcopilot.com"
	}

	provider := &CopilotProvider{
		config:   config,
		endpoint: endpoint,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		lastHealth: &ProviderHealth{
			Status:    "unknown",
			LastCheck: time.Now(),
		},
	}

	// Get GitHub token and exchange for Copilot bearer token
	if err := provider.initializeToken(); err != nil {
		return nil, fmt.Errorf("failed to initialize Copilot token: %v", err)
	}

	// Initialize models
	provider.initializeModels()

	return provider, nil
}

// initializeToken gets GitHub token and exchanges it for Copilot bearer token
func (cp *CopilotProvider) initializeToken() error {
	// Try to get GitHub token from multiple sources
	githubToken := cp.getGitHubToken()
	if githubToken == "" {
		return fmt.Errorf("GitHub token is required for Copilot provider. Set GITHUB_TOKEN environment variable or ensure GitHub CLI is authenticated")
	}

	cp.githubToken = githubToken

	// Exchange GitHub token for Copilot bearer token
	bearerToken, err := cp.exchangeGitHubToken(githubToken)
	if err != nil {
		return fmt.Errorf("failed to exchange GitHub token for Copilot bearer token: %v", err)
	}

	cp.bearerToken = bearerToken
	return nil
}

// getGitHubToken retrieves GitHub token from various sources
func (cp *CopilotProvider) getGitHubToken() string {
	// 1. Environment variable
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		return token
	}

	// 2. API key from config
	if cp.config.APIKey != "" {
		return cp.config.APIKey
	}

	// 3. GitHub CLI token locations
	if token := cp.loadGitHubCLIToken(); token != "" {
		return token
	}

	return ""
}

// loadGitHubCLIToken loads token from GitHub CLI standard locations
func (cp *CopilotProvider) loadGitHubCLIToken() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	// GitHub CLI config locations
	locations := []string{
		filepath.Join(homeDir, ".config", "gh", "hosts.yml"),
		filepath.Join(homeDir, ".config", "github-copilot", "hosts.json"),
	}

	for _, location := range locations {
		if token := cp.extractTokenFromFile(location); token != "" {
			return token
		}
	}

	return ""
}

// extractTokenFromFile is a not-implemented stub kept only as a named
// seam for a future YAML/JSON-aware token extractor. Today it always
// returns "" — the documented contract that callers MUST treat as
// "no token on disk, fall back to env-var / config" (round-33 §11.4
// honest-stub anchor — the previous body was the same `return ""` but
// the comment block read "For now, we'll rely on environment variables
// and config" which downstream maintainers could read as "this works
// today"; in reality the function performs zero file IO and returns
// the empty string unconditionally. CONST-035 / Article XI §11.9 /
// CONST-050(A)).
//
// When the real extractor lands it MUST:
//   1. Validate `path` (no symlink escape, mode 0600 per CONST-042),
//   2. Detect format from extension (.yaml/.yml/.json/.toml),
//   3. Parse and locate the canonical "token" key,
//   4. Return the extracted string or "" on absence/parse-failure.
func (cp *CopilotProvider) extractTokenFromFile(path string) string {
	_ = path
	return ""
}

// exchangeGitHubToken exchanges a GitHub token for a Copilot bearer token
func (cp *CopilotProvider) exchangeGitHubToken(githubToken string) (string, error) {
	req, err := http.NewRequest("GET", "https://api.github.com/copilot_internal/v2/token", nil)
	if err != nil {
		return "", fmt.Errorf("failed to create token exchange request: %w", err)
	}

	req.Header.Set("Authorization", "Token "+githubToken)
	req.Header.Set("User-Agent", "HelixCode/1.0")
	req.Header.Set("Accept", "application/vnd.github.v2+json")

	resp, err := cp.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to exchange GitHub token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("token exchange failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp CopilotTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("failed to decode token response: %w", err)
	}

	return tokenResp.Token, nil
}

// GetType returns the provider type
func (cp *CopilotProvider) GetType() ProviderType {
	return ProviderTypeCopilot
}

// GetName returns the provider name
func (cp *CopilotProvider) GetName() string {
	return "GitHub Copilot"
}

// GetModels returns available models
func (cp *CopilotProvider) GetModels() []ModelInfo {
	return cp.models
}

// GetCapabilities returns provider capabilities
func (cp *CopilotProvider) GetCapabilities() []ModelCapability {
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

// Generate generates a response using GitHub Copilot models
func (cp *CopilotProvider) Generate(ctx context.Context, request *LLMRequest) (*LLMResponse, error) {
	startTime := time.Now()

	// Convert to OpenAI-compatible format (GitHub Copilot uses OpenAI-compatible API)
	openaiRequest, err := cp.convertToOpenAIRequest(request)
	if err != nil {
		return nil, fmt.Errorf("failed to convert request: %v", err)
	}

	// Make request to GitHub Copilot API
	response, err := cp.makeOpenAIRequest(ctx, openaiRequest)
	if err != nil {
		return nil, fmt.Errorf("Copilot request failed: %v", err)
	}

	// Convert response
	llmResponse := cp.convertFromOpenAIResponse(response, request.ID, time.Since(startTime))

	return llmResponse, nil
}

// GenerateStream generates a streaming response
func (cp *CopilotProvider) GenerateStream(ctx context.Context, request *LLMRequest, ch chan<- LLMResponse) error {
	defer close(ch)

	// Convert to OpenAI-compatible format
	openaiRequest, err := cp.convertToOpenAIRequest(request)
	if err != nil {
		return fmt.Errorf("failed to convert request: %v", err)
	}

	// Enable streaming
	openaiRequest.Stream = true

	// Make streaming request
	return cp.makeOpenAIStreamRequest(ctx, openaiRequest, ch, request.ID)
}

// IsAvailable checks if the provider is available
func (cp *CopilotProvider) IsAvailable(ctx context.Context) bool {
	health, err := cp.GetHealth(ctx)
	return err == nil && health.Status == "healthy"
}

// GetHealth returns provider health status
func (cp *CopilotProvider) GetHealth(ctx context.Context) (*ProviderHealth, error) {
	// Check if we can reach the GitHub Copilot API
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/models", cp.endpoint), nil)
	if err != nil {
		cp.updateHealth("unhealthy", 0, cp.lastHealth.ErrorCount+1)
		return cp.lastHealth, fmt.Errorf("failed to create health check request: %v", err)
	}

	cp.setAuthHeaders(req)

	start := time.Now()
	resp, err := cp.httpClient.Do(req)
	latency := time.Since(start)

	if err != nil {
		cp.updateHealth("unhealthy", latency, cp.lastHealth.ErrorCount+1)
		return cp.lastHealth, fmt.Errorf("health check failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		cp.updateHealth("unhealthy", latency, cp.lastHealth.ErrorCount+1)
		return cp.lastHealth, fmt.Errorf("health check returned status %d", resp.StatusCode)
	}

	// Parse response to get model count
	var modelsResponse struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&modelsResponse); err != nil {
		cp.updateHealth("degraded", latency, cp.lastHealth.ErrorCount)
		return cp.lastHealth, nil // Still consider it available
	}

	cp.updateHealth("healthy", latency, 0)
	cp.lastHealth.ModelCount = len(modelsResponse.Data)

	return cp.lastHealth, nil
}

// Close closes the provider
func (cp *CopilotProvider) Close() error {
	cp.httpClient.CloseIdleConnections()
	return nil
}

// GetContextWindow returns the model's context window size in tokens.
// Default: 200_000 — GitHub Copilot routes to GPT-4o/Claude variants; 200k is safe.
func (cp *CopilotProvider) GetContextWindow() int {
	return 200_000
}

// CountTokens returns an estimated token count for text.
// Uses char-based fallback (1 token ≈ 3.5 chars) — Phase 3 will upgrade
// to the GitHub Copilot tokenize endpoint.
func (cp *CopilotProvider) CountTokens(text string) (int, error) {
	return CharBasedTokenCount(text)
}

// Helper methods

func (cp *CopilotProvider) initializeModels() {
	// Predefined GitHub Copilot models with their capabilities
	cp.models = []ModelInfo{
		{
			Name:        "gpt-4o",
			Provider:    ProviderTypeCopilot,
			ContextSize: 128000,
			MaxTokens:   16384,
			Description: "GitHub Copilot GPT-4o - Advanced model with strong reasoning",
		},
		{
			Name:        "gpt-4o-mini",
			Provider:    ProviderTypeCopilot,
			ContextSize: 128000,
			MaxTokens:   4096,
			Description: "GitHub Copilot GPT-4o Mini - Fast and efficient model",
		},
		{
			Name:        "gpt-3.5-turbo",
			Provider:    ProviderTypeCopilot,
			ContextSize: 16384,
			MaxTokens:   4096,
			Description: "GitHub Copilot GPT-3.5 Turbo - Classic reliable model",
		},
		{
			Name:        "claude-3.5-sonnet",
			Provider:    ProviderTypeCopilot,
			ContextSize: 90000,
			MaxTokens:   8192,
			Description: "GitHub Copilot Claude 3.5 Sonnet - Excellent reasoning and analysis",
		},
		{
			Name:        "claude-3.7-sonnet",
			Provider:    ProviderTypeCopilot,
			ContextSize: 200000,
			MaxTokens:   16384,
			Description: "GitHub Copilot Claude 3.7 Sonnet - Latest Claude model with advanced reasoning",
		},
		{
			Name:        "o1",
			Provider:    ProviderTypeCopilot,
			ContextSize: 200000,
			MaxTokens:   100000,
			Description: "GitHub Copilot o1 - Reasoning-focused model",
		},
		{
			Name:        "o3-mini",
			Provider:    ProviderTypeCopilot,
			ContextSize: 200000,
			MaxTokens:   100000,
			Description: "GitHub Copilot o3-mini - Efficient reasoning model",
		},
		{
			Name:        "gemini-2.0-flash-001",
			Provider:    ProviderTypeCopilot,
			ContextSize: 1000000,
			MaxTokens:   8192,
			Description: "GitHub Copilot Gemini 2.0 Flash - Fast and capable model",
		},
	}

	for i := range cp.models {
		EnrichModelInfo(&cp.models[i])
	}

	log.Printf("✅ GitHub Copilot provider initialized with %d models", len(cp.models))
}

func (cp *CopilotProvider) convertToOpenAIRequest(request *LLMRequest) (*OpenAIRequest, error) {
	// Convert messages to OpenAI format
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

func (cp *CopilotProvider) convertFromOpenAIResponse(openaiResp *OpenAIResponse, requestID uuid.UUID, processingTime time.Duration) *LLMResponse {
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
	// GitHub Copilot is OpenAI-compatible — it proxies GPT-4o, GPT-3.5-turbo,
	// Claude 3.5/3.7 Sonnet, o1, o3-mini, Gemini 2.0 Flash all via the
	// /chat/completions OpenAI-shape contract, normalising every backend's
	// terminator to OpenAI's finish_reason vocabulary ("stop", "length",
	// "content_filter", "tool_calls"). Reuses mapOpenAIFinishReasonToErr
	// verbatim. If Copilot stops normalising (e.g. surfaces Claude's
	// "max_tokens" raw), this MUST be replaced with a Copilot-specific
	// helper in the same commit.
	resp.Err = mapOpenAIFinishReasonToErr(finishReason)
	return resp
}

func (cp *CopilotProvider) makeOpenAIRequest(ctx context.Context, request *OpenAIRequest) (*OpenAIResponse, error) {
	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/chat/completions", cp.endpoint), bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	cp.setAuthHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := cp.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Copilot API returned status %d: %s", resp.StatusCode, string(body))
	}

	var response OpenAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	return &response, nil
}

func (cp *CopilotProvider) makeOpenAIStreamRequest(ctx context.Context, request *OpenAIRequest, ch chan<- LLMResponse, requestID uuid.UUID) error {
	jsonData, err := json.Marshal(request)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/chat/completions", cp.endpoint), bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	cp.setAuthHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := cp.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Copilot API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Stream responses
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
			// (CONST-035 / Article XI §11.9): on a terminal finish_reason
			// of "length" or "content_filter", emit a terminal LLMResponse
			// with Err populated so downstream consumers (notably
			// tool_provider.go :201/:251) can distinguish a clean stop
			// from a partial-error stop. Reuses the round-46 OpenAI mapper
			// since Copilot normalises every upstream model's terminator
			// to the OpenAI vocabulary.
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

		if streamResp.Choices[0].FinishReason != "" {
			break
		}
	}

	return nil
}

func (cp *CopilotProvider) setAuthHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+cp.bearerToken)
	req.Header.Set("Editor-Version", "HelixCode/1.0")
	req.Header.Set("Editor-Plugin-Version", "HelixCode/1.0")
	req.Header.Set("Copilot-Integration-Id", "vscode-chat")
}

func (cp *CopilotProvider) updateHealth(status string, latency time.Duration, errorCount int) {
	cp.lastHealth.Status = status
	cp.lastHealth.Latency = latency
	cp.lastHealth.ErrorCount = errorCount
	cp.lastHealth.LastCheck = time.Now()
}

// Note: OpenAI API types are reused for GitHub Copilot compatibility
// They are declared in openai_provider.go and used here since they're in the same package
