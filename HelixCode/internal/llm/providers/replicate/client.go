package replicate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"dev.helix.code/internal/llm"
)

const ReplicateBaseURL = "https://api.replicate.com/v1"

type Client struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

func NewClient(apiKey string) *Client {
	return &Client{
		apiKey:  apiKey,
		baseURL: ReplicateBaseURL,
		client:  &http.Client{Timeout: 120 * time.Second},
	}
}

type replicateInput struct {
	Prompt      string  `json:"prompt"`
	MaxTokens   int     `json:"max_tokens,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`
}

type replicateRequest struct {
	Input replicateInput `json:"input"`
}

type replicatePrediction struct {
	ID     string      `json:"id"`
	Status string      `json:"status"`
	Output interface{} `json:"output,omitempty"`
	Error  string      `json:"error,omitempty"`
}

func (c *Client) Generate(ctx context.Context, req *llm.LLMRequest) (*llm.LLMResponse, error) {
	prompt := ""
	if len(req.Messages) > 0 {
		prompt = req.Messages[len(req.Messages)-1].Content
	}
	model := req.Model
	if model == "" {
		model = "meta/meta-llama-3-70b-instruct"
	}
	body := replicateRequest{
		Input: replicateInput{
			Prompt:      prompt,
			MaxTokens:   req.MaxTokens,
			Temperature: req.Temperature,
		},
	}
	if body.Input.MaxTokens == 0 {
		body.Input.MaxTokens = 4096
	}
	if body.Input.Temperature == 0 {
		body.Input.Temperature = 0.7
	}
	data, _ := json.Marshal(body)
	predURL := c.baseURL + "/models/" + model + "/predictions"
	httpReq, _ := http.NewRequestWithContext(ctx, "POST", predURL, bytes.NewReader(data))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("replicate request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("replicate API error: HTTP %d", resp.StatusCode)
	}
	var pred replicatePrediction
	if err := json.NewDecoder(resp.Body).Decode(&pred); err != nil {
		return nil, fmt.Errorf("decode prediction: %w", err)
	}
	output, err := c.waitForCompletion(ctx, pred.ID)
	if err != nil {
		return nil, err
	}
	return &llm.LLMResponse{Content: output}, nil
}

func (c *Client) waitForCompletion(ctx context.Context, id string) (string, error) {
	for i := 0; i < 30; i++ {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(2 * time.Second):
		}
		req, _ := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/predictions/"+id, nil)
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		resp, err := c.client.Do(req)
		if err != nil {
			return "", err
		}
		var pred replicatePrediction
		json.NewDecoder(resp.Body).Decode(&pred)
		resp.Body.Close()
		switch pred.Status {
		case "succeeded":
			return fmt.Sprintf("%v", pred.Output), nil
		case "failed":
			return "", fmt.Errorf("replicate prediction failed: %s", pred.Error)
		}
	}
	return "", fmt.Errorf("replicate prediction timed out after 60s")
}
