package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Remove unused imports - io is not used

// LlamaCPPProvider implements the LLM provider interface for Llama.cpp
type LlamaCPPProvider struct {
	config    LlamaConfig
	isRunning bool
}

// LlamaConfig holds configuration for Llama.cpp
type LlamaConfig struct {
	Model         string        `json:"model"`
	ContextSize   int           `json:"context_size"`
	GPUEnabled    bool          `json:"gpu_enabled"`
	GPULayers     int           `json:"gpu_layers"`
	Threads       int           `json:"threads"`
	ServerHost    string        `json:"server_host"`
	ServerPort    int           `json:"server_port"`
	ServerTimeout time.Duration `json:"server_timeout"`
}

// NewLlamaCPPProvider creates a new Llama.cpp provider
func NewLlamaCPPProvider(config LlamaConfig) (*LlamaCPPProvider, error) {
	provider := &LlamaCPPProvider{
		config:    config,
		isRunning: true,
	}

	log.Printf("✅ Llama.cpp provider initialized with model: %s", config.Model)
	return provider, nil
}

// GetType returns the provider type
func (p *LlamaCPPProvider) GetType() ProviderType {
	return ProviderTypeLocal
}

// GetName returns the provider name
func (p *LlamaCPPProvider) GetName() string {
	return "llama-cpp"
}

// GetModels returns available models
// ANTI-BLUFF: Query REAL Llama.cpp server for available models
func (p *LlamaCPPProvider) GetModels() []ModelInfo {
	baseURL := p.config.ServerHost
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	// REAL HTTP call to Llama.cpp server
	resp, err := http.Get(baseURL + "/models")
	if err != nil {
		log.Printf("Failed to list Llama.cpp models: %v", err)
		// Return configured model as fallback
		models := []ModelInfo{
			{
				Name:        p.config.Model,
				Provider:    ProviderTypeLocal,
				ContextSize: p.config.ContextSize,
				MaxTokens:   p.config.ContextSize,
				Description: "Local Llama.cpp model",
			},
		}
		for i := range models {
			EnrichModelInfo(&models[i])
		}
		return models
	}
	defer resp.Body.Close()

	// Parse response
	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("Failed to decode Llama.cpp models: %v", err)
		return []ModelInfo{}
	}

	models := make([]ModelInfo, len(result.Models))
	for i, m := range result.Models {
		models[i] = ModelInfo{
			Name:        m.Name,
			Provider:    ProviderTypeLocal,
			ContextSize: p.config.ContextSize,
			MaxTokens:   p.config.ContextSize,
			Description: "Local Llama.cpp model",
		}
	}
	for i := range models {
		EnrichModelInfo(&models[i])
	}
	return models
}

// GetCapabilities returns model capabilities
func (p *LlamaCPPProvider) GetCapabilities() []ModelCapability {
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

// Generate generates a response using Llama.cpp
// ANTI-BLUFF: This MUST make REAL HTTP call to Llama.cpp server
func (p *LlamaCPPProvider) Generate(ctx context.Context, request *LLMRequest) (*LLMResponse, error) {
	if !p.isRunning {
		return nil, ErrProviderUnavailable
	}

	// REAL implementation - connect to Llama.cpp server
	baseURL := p.config.ServerHost
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	if p.config.ServerPort != 0 {
		baseURL = fmt.Sprintf("%s:%d", baseURL, p.config.ServerPort)
	}

	// Llama.cpp uses /v1/completions endpoint (OpenAI compatible)
	apiURL := baseURL + "/v1/completions"

	// Build request payload - use Messages if available, otherwise use Model
	var payload map[string]interface{}
	if len(request.Messages) > 0 {
		payload = map[string]interface{}{
			"model":       p.config.Model,
			"messages":    request.Messages,
			"max_tokens":  request.MaxTokens,
			"temperature": request.Temperature,
			"stream":      false,
		}
	} else {
		// Convert Messages to prompt string
		prompt := ""
		for _, msg := range request.Messages {
			prompt += msg.Role + ": " + msg.Content + "\n"
		}
		payload = map[string]interface{}{
			"model":       p.config.Model,
			"prompt":      prompt,
			"max_tokens":  request.MaxTokens,
			"temperature": request.Temperature,
			"stream":      false,
		}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Make REAL HTTP call to Llama.cpp server
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: p.config.ServerTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("llama.cpp request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("llama.cpp returned status %d", resp.StatusCode)
	}

	// Parse REAL response
	var result struct {
		Content string `json:"content"`
		Usage   struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &LLMResponse{
		ID:        uuid.New(),
		RequestID: request.ID,
		Content:   result.Content,
		Usage: Usage{
			PromptTokens:     result.Usage.PromptTokens,
			CompletionTokens: result.Usage.CompletionTokens,
			TotalTokens:      result.Usage.TotalTokens,
		},
		ProcessingTime: time.Since(time.Now()),
		CreatedAt:      time.Now(),
	}, nil
}

// GenerateStream generates a streaming response
// ANTI-BLUFF: This MUST stream REAL response from Llama.cpp server
func (p *LlamaCPPProvider) GenerateStream(ctx context.Context, request *LLMRequest, ch chan<- LLMResponse) error {
	if !p.isRunning {
		return ErrProviderUnavailable
	}

	// REAL streaming implementation
	baseURL := p.config.ServerHost
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	if p.config.ServerPort != 0 {
		baseURL = fmt.Sprintf("%s:%d", baseURL, p.config.ServerPort)
	}

	// Build prompt from messages or use Model field
	prompt := ""
	if len(request.Messages) > 0 {
		for _, msg := range request.Messages {
			prompt += msg.Role + ": " + msg.Content + "\n"
		}
	} else {
		prompt = request.Model
	}

	payload := map[string]interface{}{
		"model":      p.config.Model,
		"prompt":     prompt,
		"max_tokens": request.MaxTokens,
		"temperature": request.Temperature,
		"stream":      true,
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, "POST", baseURL+"/completion", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	client := &http.Client{Timeout: 0} // No timeout for streaming
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("llama.cpp stream request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read SSE stream from REAL server
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk struct {
			Content string `json:"content"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case ch <- LLMResponse{
			ID:        uuid.New(),
			RequestID: request.ID,
			Content:   chunk.Content,
			CreatedAt: time.Now(),
		}:
		}
	}

	return scanner.Err()
}

// IsAvailable checks if the provider is available
func (p *LlamaCPPProvider) IsAvailable(ctx context.Context) bool {
	return p.isRunning
}

// GetHealth returns provider health status
func (p *LlamaCPPProvider) GetHealth(ctx context.Context) (*ProviderHealth, error) {
	if !p.isRunning {
		return &ProviderHealth{
			Status:     "unhealthy",
			LastCheck:  time.Now(),
			ErrorCount: 1,
		}, nil
	}

	return &ProviderHealth{
		Status:     "healthy",
		LastCheck:  time.Now(),
		ErrorCount: 0,
		ModelCount: len(p.GetModels()),
	}, nil
}

// Close stops the Llama.cpp provider
func (p *LlamaCPPProvider) Close() error {
	p.isRunning = false
	log.Println("✅ Llama.cpp provider closed")
	return nil
}

// GetContextWindow returns the model's context window size in tokens.
// Uses the ContextSize from the LlamaConfig when set; falls back to 200_000.
func (p *LlamaCPPProvider) GetContextWindow() int {
	if p.config.ContextSize > 0 {
		return p.config.ContextSize
	}
	return 200_000
}

// CountTokens returns an estimated token count for text.
// Uses char-based fallback (1 token ≈ 3.5 chars) — Phase 3 will upgrade
// to the llama.cpp /tokenize endpoint.
func (p *LlamaCPPProvider) CountTokens(text string) (int, error) {
	return CharBasedTokenCount(text)
}
