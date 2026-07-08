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

// CohereBaseURL is the current Cohere Chat v2 endpoint (§11.4.99). The v1/chat
// endpoint (https://api.cohere.com/v1/chat) was retired in 2025 and returns
// HTTP 404. The v2 endpoint uses a messages-array format rather than the old
// singular message+chat_history shape.
const CohereBaseURL = "https://api.cohere.com/v2/chat"

const (
	// DefaultModel is the current recommended Cohere Command model.
	// command-r-plus was retired in early 2025; command-r-08-2024 is its
	// stable replacement (§11.4.99).
	DefaultModel = "command-r-08-2024"
)

// Client implements Cohere Chat v2 API communication.
type Client struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

// NewClient creates a new Cohere API client. The base URL defaults to the
// current v2 chat endpoint. An empty apiKey will produce 401 at runtime but
// is not rejected at construction so callers can defer key resolution.
func NewClient(apiKey string) *Client {
	return &Client{
		apiKey:  apiKey,
		baseURL: CohereBaseURL,
		client:  &http.Client{Timeout: 60 * time.Second},
	}
}

// v2Request is the Cohere Chat v2 request body (POST /v2/chat).
// See https://docs.cohere.com/reference/chat
type v2Request struct {
	Model       string        `json:"model"`
	Messages    []v2Message   `json:"messages"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Temperature float64       `json:"temperature,omitempty"`
	Stream      bool          `json:"stream,omitempty"`
}

type v2Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// v2Response is the Cohere Chat v2 response body.
type v2Response struct {
	ID           string       `json:"id"`
	FinishReason string       `json:"finish_reason"`
	Message      v2MessageOut `json:"message"`
	Usage        *v2Usage     `json:"usage,omitempty"`
}

type v2MessageOut struct {
	Role    string        `json:"role"`
	Content []v2ContentPart `json:"content"`
}

type v2ContentPart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type v2Usage struct {
	BilledUnits v2BilledUnits `json:"billed_units"`
	Tokens      v2TokenUsage  `json:"tokens"`
}

type v2BilledUnits struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type v2TokenUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// Generate sends a chat completion request to the Cohere v2 API.
func (c *Client) Generate(ctx context.Context, req *llm.LLMRequest) (*llm.LLMResponse, error) {
	if len(req.Messages) == 0 {
		return nil, fmt.Errorf("no messages in request")
	}

	messages := make([]v2Message, len(req.Messages))
	for i, m := range req.Messages {
		role := m.Role
		if role == "" {
			role = "user"
		}
		messages[i] = v2Message{Role: role, Content: m.Content}
	}

	body := v2Request{
		Model:       req.Model,
		Messages:    messages,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
	}
	if body.Model == "" {
		body.Model = DefaultModel
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

	var cr v2Response
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	// Extract text from message.content[] parts.
	var content string
	for _, part := range cr.Message.Content {
		if part.Type == "text" {
			content += part.Text
		}
	}

	return &llm.LLMResponse{
		Content:      content,
		FinishReason: cr.FinishReason,
	}, nil
}
