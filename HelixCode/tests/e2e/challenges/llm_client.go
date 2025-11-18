package challenges

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// LLMClient handles communication with LLM providers
type LLMClient struct {
	provider LLMProviderType
	model    string
	apiKeys  *APIKeys
	client   *http.Client
}

// NewLLMClient creates a new LLM client
func NewLLMClient(provider LLMProviderType, model string, apiKeys *APIKeys) *LLMClient {
	return &LLMClient{
		provider: provider,
		model:    model,
		apiKeys:  apiKeys,
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// CompletionRequest represents a request to an LLM
type CompletionRequest struct {
	Prompt      string
	MaxTokens   int
	Temperature float64
	SystemPrompt string
}

// CompletionResponse represents a response from an LLM
type CompletionResponse struct {
	Content      string
	FinishReason string
	TokensUsed   int
}

// Complete sends a completion request to the LLM
func (c *LLMClient) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	switch c.provider {
	case ProviderXAI:
		return c.completeXAI(ctx, req)
	case ProviderOpenAI:
		return c.completeOpenAI(ctx, req)
	case ProviderAnthropic:
		return c.completeAnthropic(ctx, req)
	case ProviderGemini:
		return c.completeGemini(ctx, req)
	case ProviderGroq:
		return c.completeGroq(ctx, req)
	case ProviderMistral:
		return c.completeMistral(ctx, req)
	case ProviderOllama:
		return c.completeOllama(ctx, req)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", c.provider)
	}
}

// xAI (Grok) API implementation
func (c *LLMClient) completeXAI(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	apiKey, err := c.apiKeys.GetAPIKey(ProviderXAI)
	if err != nil {
		return nil, err
	}

	// xAI uses OpenAI-compatible API
	payload := map[string]interface{}{
		"model": c.model,
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": req.SystemPrompt,
			},
			{
				"role":    "user",
				"content": req.Prompt,
			},
		},
		"max_tokens":  req.MaxTokens,
		"temperature": req.Temperature,
		"stream":      false,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://api.x.ai/v1/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var apiResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			TotalTokens int `json:"total_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(apiResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	return &CompletionResponse{
		Content:      apiResp.Choices[0].Message.Content,
		FinishReason: apiResp.Choices[0].FinishReason,
		TokensUsed:   apiResp.Usage.TotalTokens,
	}, nil
}

// OpenAI API implementation
func (c *LLMClient) completeOpenAI(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	apiKey, err := c.apiKeys.GetAPIKey(ProviderOpenAI)
	if err != nil {
		return nil, err
	}

	payload := map[string]interface{}{
		"model": c.model,
		"messages": []map[string]string{
			{"role": "system", "content": req.SystemPrompt},
			{"role": "user", "content": req.Prompt},
		},
		"max_tokens":  req.MaxTokens,
		"temperature": req.Temperature,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OpenAI API error (status %d): %s", resp.StatusCode, string(body))
	}

	var apiResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			TotalTokens int `json:"total_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, err
	}

	if len(apiResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	return &CompletionResponse{
		Content:      apiResp.Choices[0].Message.Content,
		FinishReason: apiResp.Choices[0].FinishReason,
		TokensUsed:   apiResp.Usage.TotalTokens,
	}, nil
}

// Anthropic Claude API implementation
func (c *LLMClient) completeAnthropic(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	apiKey, err := c.apiKeys.GetAPIKey(ProviderAnthropic)
	if err != nil {
		return nil, err
	}

	payload := map[string]interface{}{
		"model": c.model,
		"messages": []map[string]string{
			{"role": "user", "content": req.Prompt},
		},
		"max_tokens": req.MaxTokens,
		"system":     req.SystemPrompt,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Anthropic API error (status %d): %s", resp.StatusCode, string(body))
	}

	var apiResp struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		StopReason string `json:"stop_reason"`
		Usage      struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, err
	}

	if len(apiResp.Content) == 0 {
		return nil, fmt.Errorf("no content in response")
	}

	return &CompletionResponse{
		Content:      apiResp.Content[0].Text,
		FinishReason: apiResp.StopReason,
		TokensUsed:   apiResp.Usage.InputTokens + apiResp.Usage.OutputTokens,
	}, nil
}

// Gemini API implementation (placeholder)
func (c *LLMClient) completeGemini(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	return nil, fmt.Errorf("Gemini provider not yet implemented")
}

// Groq API implementation
func (c *LLMClient) completeGroq(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	apiKey, err := c.apiKeys.GetAPIKey(ProviderGroq)
	if err != nil {
		return nil, err
	}

	// Groq uses OpenAI-compatible API
	payload := map[string]interface{}{
		"model": c.model,
		"messages": []map[string]string{
			{"role": "system", "content": req.SystemPrompt},
			{"role": "user", "content": req.Prompt},
		},
		"max_tokens":  req.MaxTokens,
		"temperature": req.Temperature,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://api.groq.com/openai/v1/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Groq API error (status %d): %s", resp.StatusCode, string(body))
	}

	var apiResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			TotalTokens int `json:"total_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, err
	}

	if len(apiResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	return &CompletionResponse{
		Content:      apiResp.Choices[0].Message.Content,
		FinishReason: apiResp.Choices[0].FinishReason,
		TokensUsed:   apiResp.Usage.TotalTokens,
	}, nil
}

// Mistral API implementation (placeholder)
func (c *LLMClient) completeMistral(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	return nil, fmt.Errorf("Mistral provider not yet implemented")
}

// Ollama local API implementation (for reference)
func (c *LLMClient) completeOllama(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	payload := map[string]interface{}{
		"model":  c.model,
		"prompt": fmt.Sprintf("%s\n\n%s", req.SystemPrompt, req.Prompt),
		"stream": false,
		"options": map[string]interface{}{
			"temperature": req.Temperature,
			"num_predict": req.MaxTokens,
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", "http://localhost:11434/api/generate", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Ollama API error (status %d): %s", resp.StatusCode, string(body))
	}

	var apiResp struct {
		Response string `json:"response"`
		Done     bool   `json:"done"`
	}

	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, err
	}

	return &CompletionResponse{
		Content:      apiResp.Response,
		FinishReason: "stop",
		TokensUsed:   0, // Ollama doesn't return token count
	}, nil
}
