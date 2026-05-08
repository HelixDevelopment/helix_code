package huggingface

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"dev.helix.code/internal/llm"
)

const BaseURL = "https://api-inference.huggingface.co/models"

type Client struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

func NewClient(apiKey string) *Client {
	return &Client{
		apiKey:  apiKey,
		baseURL: BaseURL,
		client:  &http.Client{Timeout: 60 * time.Second},
	}
}

type hfRequest struct {
	Inputs string `json:"inputs"`
}

type hfResponse struct {
	GeneratedText string `json:"generated_text"`
}

func (c *Client) Generate(ctx context.Context, req *llm.LLMRequest) (*llm.LLMResponse, error) {
	prompt := ""
	if len(req.Messages) > 0 {
		prompt = req.Messages[len(req.Messages)-1].Content
	}
	model := req.Model
	if model == "" {
		model = "mistralai/Mistral-7B-Instruct-v0.2"
	}
	body := hfRequest{Inputs: prompt}
	data, _ := json.Marshal(body)
	url := c.baseURL + "/" + model
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("huggingface request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("huggingface API error: HTTP %d", resp.StatusCode)
	}
	var results []hfResponse
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("empty response from huggingface")
	}
	return &llm.LLMResponse{Content: results[0].GeneratedText}, nil
}
