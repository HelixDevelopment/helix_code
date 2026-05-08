package litellm

import (
	"encoding/json"
	"fmt"

	"dev.helix.code/internal/llm"
)

type AnthropicAdapter struct{}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicRequest struct {
	Model       string             `json:"model"`
	Messages    []anthropicMessage `json:"messages"`
	System      string             `json:"system,omitempty"`
	MaxTokens   int                `json:"max_tokens"`
	Temperature float64            `json:"temperature,omitempty"`
}

type anthropicContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type anthropicResponse struct {
	Content []anthropicContentBlock `json:"content"`
}

func (a *AnthropicAdapter) Format() ResponseFormat {
	return FormatAnthropic
}

func (a *AnthropicAdapter) ConvertRequest(req *llm.LLMRequest) (interface{}, error) {
	if req == nil {
		return nil, fmt.Errorf("nil request")
	}
	system := ""
	msgs := make([]anthropicMessage, 0, len(req.Messages))
	for _, m := range req.Messages {
		if m.Role == "system" {
			if system != "" {
				system += "\n"
			}
			system += m.Content
		} else {
			role := m.Role
			if role == "assistant" {
				role = "assistant"
			} else if role == "" {
				role = "user"
			}
			msgs = append(msgs, anthropicMessage{Role: role, Content: m.Content})
		}
	}
	return &anthropicRequest{
		Model:       req.Model,
		Messages:    msgs,
		System:      system,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
	}, nil
}

func (a *AnthropicAdapter) ConvertResponse(raw interface{}) (*llm.LLMResponse, error) {
	if raw == nil {
		return nil, fmt.Errorf("nil response")
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("marshal raw: %w", err)
	}
	var resp anthropicResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	content := ""
	for _, block := range resp.Content {
		if block.Type == "text" {
			content += block.Text
		}
	}
	if content == "" {
		return nil, fmt.Errorf("no content in response")
	}
	return &llm.LLMResponse{Content: content}, nil
}

func (a *AnthropicAdapter) ConvertStreamChunk(raw interface{}) (*LLMStreamChunk, error) {
	if raw == nil {
		return nil, nil
	}
	data, _ := json.Marshal(raw)
	var chunk struct {
		Type string `json:"type"`
		Delta struct {
			Text string `json:"text"`
		} `json:"delta"`
	}
	if err := json.Unmarshal(data, &chunk); err != nil {
		return nil, nil
	}
	if chunk.Type == "content_block_delta" {
		return &LLMStreamChunk{Content: chunk.Delta.Text}, nil
	}
	if chunk.Type == "message_delta" && chunk.Delta.Text != "" {
		return &LLMStreamChunk{Content: chunk.Delta.Text}, nil
	}
	return nil, nil
}