package litellm

import (
	"encoding/json"
	"fmt"

	"dev.helix.code/internal/llm"
)

type GoogleAdapter struct{}

type googlePart struct {
	Text string `json:"text"`
}

type googleContent struct {
	Parts []googlePart `json:"parts"`
}

type googleRequest struct {
	Contents []googleContent `json:"contents"`
}

type googleCandidate struct {
	Content googleContent `json:"content"`
}

type googleResponse struct {
	Candidates []googleCandidate `json:"candidates"`
}

func (a *GoogleAdapter) Format() ResponseFormat {
	return FormatGoogle
}

func (a *GoogleAdapter) ConvertRequest(req *llm.LLMRequest) (interface{}, error) {
	if req == nil {
		return nil, fmt.Errorf("nil request")
	}
	parts := make([]googlePart, 0, len(req.Messages))
	for _, m := range req.Messages {
		if m.Role == "" {
			m.Role = "user"
		}
		parts = append(parts, googlePart{Text: m.Content})
	}
	return &googleRequest{
		Contents: []googleContent{{Parts: parts}},
	}, nil
}

func (a *GoogleAdapter) ConvertResponse(raw interface{}) (*llm.LLMResponse, error) {
	if raw == nil {
		return nil, fmt.Errorf("nil response")
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}
	var resp googleResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no content in response")
	}
	return &llm.LLMResponse{Content: resp.Candidates[0].Content.Parts[0].Text}, nil
}

func (a *GoogleAdapter) ConvertStreamChunk(raw interface{}) (*LLMStreamChunk, error) {
	if raw == nil {
		return nil, nil
	}
	data, _ := json.Marshal(raw)
	var resp googleResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, nil
	}
	return &LLMStreamChunk{Content: resp.Candidates[0].Content.Parts[0].Text}, nil
}