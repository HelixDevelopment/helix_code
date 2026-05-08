package litellm

import (
	"context"
	"testing"

	"dev.helix.code/internal/llm"
)

func TestOpenAIAdapter_ConvertRequest(t *testing.T) {
	a := &OpenAIAdapter{}
	req := &llm.LLMRequest{
		Model:       "gpt-4",
		Messages:    []llm.Message{{Role: "user", Content: "hello"}},
		MaxTokens:   100,
		Temperature: 0.5,
	}
	converted, err := a.ConvertRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	openaiReq := converted.(*openaiRequest)
	if openaiReq.Model != "gpt-4" {
		t.Errorf("expected gpt-4, got %s", openaiReq.Model)
	}
	if len(openaiReq.Messages) != 1 || openaiReq.Messages[0].Content != "hello" {
		t.Errorf("unexpected messages")
	}
	if openaiReq.MaxTokens != 100 {
		t.Errorf("expected 100, got %d", openaiReq.MaxTokens)
	}
	if openaiReq.Temperature != 0.5 {
		t.Errorf("expected 0.5, got %f", openaiReq.Temperature)
	}
}

func TestOpenAIAdapter_ConvertResponse(t *testing.T) {
	a := &OpenAIAdapter{}
	raw := map[string]interface{}{
		"choices": []interface{}{
			map[string]interface{}{
				"message": map[string]interface{}{
					"role":    "assistant",
					"content": "Hello, world!",
				},
			},
		},
	}
	resp, err := a.ConvertResponse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "Hello, world!" {
		t.Errorf("expected Hello, world!, got %s", resp.Content)
	}
	if resp.FinishReason != "" {
		t.Errorf("expected empty finish reason, got %s", resp.FinishReason)
	}
}

func TestAnthropicAdapter_ConvertRequest(t *testing.T) {
	a := &AnthropicAdapter{}
	req := &llm.LLMRequest{
		Model: "claude-3-opus-20240229",
		Messages: []llm.Message{
			{Role: "system", Content: "You are helpful."},
			{Role: "user", Content: "hi"},
		},
	}
	converted, err := a.ConvertRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	anthReq := converted.(*anthropicRequest)
	if anthReq.System != "You are helpful." {
		t.Errorf("expected system prompt, got %s", anthReq.System)
	}
	if len(anthReq.Messages) != 1 || anthReq.Messages[0].Content != "hi" {
		t.Errorf("unexpected messages")
	}
}

func TestAnthropicAdapter_ConvertResponse(t *testing.T) {
	a := &AnthropicAdapter{}
	raw := map[string]interface{}{
		"content": []interface{}{
			map[string]interface{}{"type": "text", "text": "Hello!"},
			map[string]interface{}{"type": "image", "source": map[string]interface{}{"type": "base64"}},
		},
	}
	resp, err := a.ConvertResponse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "Hello!" {
		t.Errorf("expected Hello!, got %s", resp.Content)
	}
}

func TestGoogleAdapter_ConvertRequest(t *testing.T) {
	a := &GoogleAdapter{}
	req := &llm.LLMRequest{
		Messages: []llm.Message{{Role: "user", Content: "hello"}},
	}
	converted, err := a.ConvertRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	gReq := converted.(*googleRequest)
	if len(gReq.Contents) != 1 || len(gReq.Contents[0].Parts) != 1 {
		t.Errorf("unexpected structure")
	}
	if gReq.Contents[0].Parts[0].Text != "hello" {
		t.Errorf("expected hello, got %s", gReq.Contents[0].Parts[0].Text)
	}
}

func TestGoogleAdapter_ConvertResponse(t *testing.T) {
	a := &GoogleAdapter{}
	raw := map[string]interface{}{
		"candidates": []interface{}{
			map[string]interface{}{
				"content": map[string]interface{}{
					"parts": []interface{}{
						map[string]interface{}{"text": "Hi there!"},
					},
				},
			},
		},
	}
	resp, err := a.ConvertResponse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "Hi there!" {
		t.Errorf("expected Hi there!, got %s", resp.Content)
	}
}

func TestUnifiedProvider_NilRequest(t *testing.T) {
	p := NewUnifiedProvider(UnifiedProviderConfig{
		Adapter: &OpenAIAdapter{},
		Endpoint: "https://api.openai.com/v1/chat/completions",
	})
	_, err := p.Generate(context.Background(), nil)
	if err == nil {
		t.Error("expected error for nil request")
	}
}

func TestUnifiedProvider_EmptyEndpoint(t *testing.T) {
	p := NewUnifiedProvider(UnifiedProviderConfig{
		Adapter: &OpenAIAdapter{},
	})
	req := &llm.LLMRequest{
		Messages: []llm.Message{{Role: "user", Content: "test"}},
	}
	_, err := p.Generate(context.Background(), req)
	if err == nil {
		t.Error("expected error for empty endpoint")
	}
}