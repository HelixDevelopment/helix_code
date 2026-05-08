package litellm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"dev.helix.code/internal/llm"
)

type UnifiedProvider struct {
	config UnifiedProviderConfig
	client *http.Client
}

func NewUnifiedProvider(config UnifiedProviderConfig) *UnifiedProvider {
	timeout := config.Timeout
	if timeout == 0 {
		timeout = 60 * time.Second
	}
	return &UnifiedProvider{
		config: config,
		client: &http.Client{Timeout: timeout},
	}
}

func (p *UnifiedProvider) Generate(ctx context.Context, req *llm.LLMRequest) (*llm.LLMResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("nil request")
	}
	model := req.Model
	if model == "" {
		model = p.config.DefaultModel
	}
	if model == "" {
		model = "default"
	}
	localReq := &llm.LLMRequest{
		Model:       model,
		Messages:    req.Messages,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
	}
	if localReq.MaxTokens == 0 {
		localReq.MaxTokens = p.config.MaxTokens
	}
	if localReq.MaxTokens == 0 {
		localReq.MaxTokens = 4096
	}
	if localReq.Temperature == 0 {
		localReq.Temperature = p.config.Temperature
	}
	if localReq.Temperature == 0 {
		localReq.Temperature = 0.7
	}

	converted, err := p.config.Adapter.ConvertRequest(localReq)
	if err != nil {
		return nil, fmt.Errorf("convert request: %w", err)
	}
	data, err := json.Marshal(converted)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}
	endpoint := p.config.Endpoint
	if endpoint == "" {
		return nil, fmt.Errorf("empty endpoint")
	}
	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if p.config.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.config.APIKey)
	}
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error HTTP %d: %s", resp.StatusCode, string(bodyBytes))
	}
	var raw interface{}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	result, err := p.config.Adapter.ConvertResponse(raw)
	if err != nil {
		return nil, fmt.Errorf("convert response: %w", err)
	}
	if result == nil {
		return nil, fmt.Errorf("nil response from adapter")
	}
	return result, nil
}

func (p *UnifiedProvider) GenerateStream(ctx context.Context, req *llm.LLMRequest) (<-chan *LLMStreamChunk, error) {
	if req == nil {
		return nil, fmt.Errorf("nil request")
	}
	ch := make(chan *LLMStreamChunk, 100)
	go func() {
		defer close(ch)
		model := req.Model
		if model == "" {
			model = p.config.DefaultModel
		}
		if model == "" {
			model = "default"
		}
		localReq := &llm.LLMRequest{
			Model:       model,
			Messages:    req.Messages,
			MaxTokens:   req.MaxTokens,
			Temperature: req.Temperature,
		}
		if localReq.MaxTokens == 0 {
			localReq.MaxTokens = p.config.MaxTokens
		}
		if localReq.MaxTokens == 0 {
			localReq.MaxTokens = 4096
		}
		if localReq.Temperature == 0 {
			localReq.Temperature = p.config.Temperature
		}
		if localReq.Temperature == 0 {
			localReq.Temperature = 0.7
		}
		converted, err := p.config.Adapter.ConvertRequest(localReq)
		if err != nil {
			select {
			case ch <- &LLMStreamChunk{Content: fmt.Sprintf("error: %v", err), Done: true}:
			case <-ctx.Done():
			}
			return
		}
		data, _ := json.Marshal(converted)
		endpoint := p.config.Endpoint
		if endpoint == "" {
			select {
			case ch <- &LLMStreamChunk{Content: "error: empty endpoint", Done: true}:
			case <-ctx.Done():
			}
			return
		}
		httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(data))
		if err != nil {
			select {
			case ch <- &LLMStreamChunk{Content: fmt.Sprintf("error: %v", err), Done: true}:
			case <-ctx.Done():
			}
			return
		}
		httpReq.Header.Set("Content-Type", "application/json")
		if p.config.APIKey != "" {
			httpReq.Header.Set("Authorization", "Bearer "+p.config.APIKey)
		}
		resp, err := p.client.Do(httpReq)
		if err != nil {
			select {
			case ch <- &LLMStreamChunk{Content: fmt.Sprintf("error: %v", err), Done: true}:
			case <-ctx.Done():
			}
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			select {
			case ch <- &LLMStreamChunk{Content: fmt.Sprintf("error: HTTP %d", resp.StatusCode), Done: true}:
			case <-ctx.Done():
			}
			return
		}
		decoder := json.NewDecoder(resp.Body)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			var raw json.RawMessage
			if err := decoder.Decode(&raw); err != nil {
				return
			}
			chunk, err := p.config.Adapter.ConvertStreamChunk(raw)
			if err != nil {
				continue
			}
			if chunk == nil {
				continue
			}
			select {
			case ch <- chunk:
			case <-ctx.Done():
				return
			}
		}
		select {
		case ch <- &LLMStreamChunk{Done: true}:
		case <-ctx.Done():
		}
	}()
	return ch, nil
}