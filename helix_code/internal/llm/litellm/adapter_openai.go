package litellm

import (
	"encoding/json"
	"fmt"

	"dev.helix.code/internal/llm"
)

type OpenAIAdapter struct{}

type openaiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openaiRequest struct {
	Model       string          `json:"model"`
	Messages    []openaiMessage `json:"messages"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature float64         `json:"temperature,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
}

type openaiChoice struct {
	Index   int           `json:"index"`
	Message openaiMessage `json:"message"`
	Delta   openaiMessage `json:"delta,omitempty"`
}

type openaiResponse struct {
	Choices []openaiChoice `json:"choices"`
}

func (a *OpenAIAdapter) Format() ResponseFormat {
	return FormatOpenAI
}

func (a *OpenAIAdapter) ConvertRequest(req *llm.LLMRequest) (interface{}, error) {
	if req == nil {
		return nil, fmt.Errorf("nil request")
	}
	if len(req.Messages) == 0 {
		return &openaiRequest{
			Model:       req.Model,
			Messages:    []openaiMessage{},
			MaxTokens:   req.MaxTokens,
			Temperature: req.Temperature,
		}, nil
	}
	msgs := make([]openaiMessage, len(req.Messages))
	for i, m := range req.Messages {
		if m.Role == "" {
			m.Role = "user"
		}
		msgs[i] = openaiMessage{Role: m.Role, Content: m.Content}
	}
	return &openaiRequest{
		Model:       req.Model,
		Messages:    msgs,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
	}, nil
}

func (a *OpenAIAdapter) ConvertResponse(raw interface{}) (*llm.LLMResponse, error) {
	if raw == nil {
		return nil, fmt.Errorf("nil response")
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("marshal raw: %w", err)
	}
	var resp openaiResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}
	if resp.Choices[0].Message.Content == "" {
		return nil, fmt.Errorf("empty message content")
	}
	return &llm.LLMResponse{Content: resp.Choices[0].Message.Content}, nil
}

func (a *OpenAIAdapter) ConvertStreamChunk(raw interface{}) (*LLMStreamChunk, error) {
	if raw == nil {
		return nil, nil
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var resp openaiResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	if len(resp.Choices) == 0 {
		return nil, nil
	}
	return &LLMStreamChunk{Content: resp.Choices[0].Delta.Content}, nil
}