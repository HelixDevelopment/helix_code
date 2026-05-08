package cohere

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"dev.helix.code/internal/llm"
)

const CohereBaseURL = "https://api.cohere.com/v1/chat"

type Client struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

func NewClient(apiKey string) *Client {
	return &Client{
		apiKey:  apiKey,
		baseURL: CohereBaseURL,
		client:  &http.Client{Timeout: 60 * time.Second},
	}
}

type cohereRequest struct {
	Model       string          `json:"model"`
	Message     string          `json:"message"`
	ChatHistory []cohereMessage `json:"chat_history,omitempty"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature float64         `json:"temperature,omitempty"`
}

type cohereMessage struct {
	Role    string `json:"role"`
	Message string `json:"message"`
}

type cohereResponse struct {
	Text         string `json:"text"`
	FinishReason string `json:"finish_reason"`
}

func (c *Client) Generate(ctx context.Context, req *llm.LLMRequest) (*llm.LLMResponse, error) {
	if len(req.Messages) == 0 {
		return nil, fmt.Errorf("no messages in request")
	}
	last := req.Messages[len(req.Messages)-1]
	history := make([]cohereMessage, 0, len(req.Messages)-1)
	for i := 0; i < len(req.Messages)-1; i++ {
		role := "User"
		if req.Messages[i].Role == "assistant" {
			role = "Chatbot"
		}
		history = append(history, cohereMessage{Role: role, Message: req.Messages[i].Content})
	}
	body := cohereRequest{
		Model:       req.Model,
		Message:     last.Content,
		ChatHistory: history,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
	}
	if body.Model == "" {
		body.Model = "command-r-plus"
	}
	if body.MaxTokens == 0 {
		body.MaxTokens = 4096
	}
	if body.Temperature == 0 {
		body.Temperature = 0.7
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cohere API error: HTTP %d", resp.StatusCode)
	}
	var cr cohereResponse
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &llm.LLMResponse{Content: cr.Text, FinishReason: cr.FinishReason}, nil
}
