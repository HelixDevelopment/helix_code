package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
)

// DeepSeekProvider implements the Provider interface for the DeepSeek
// Cloud (api.deepseek.com) OpenAI-compatible endpoint. Round-41 F12
// fast-path expansion: a user with DEEPSEEK_API_KEY can run
// `HELIX_LLM_PROVIDER=deepseek ./bin/cli` without first editing
// config.yaml or starting the HelixCode server.
type DeepSeekProvider struct {
	config     ProviderConfigEntry
	endpoint   string
	apiKey     string
	httpClient *http.Client
	models     []ModelInfo
	lastHealth *ProviderHealth
}

// NewDeepSeekProvider creates a new DeepSeek provider.
func NewDeepSeekProvider(config ProviderConfigEntry) (*DeepSeekProvider, error) {
	endpoint := config.Endpoint
	if endpoint == "" {
		endpoint = "https://api.deepseek.com/v1"
	}

	apiKey := config.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("DEEPSEEK_API_KEY")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("DeepSeek API key is required (set config.APIKey or DEEPSEEK_API_KEY env var)")
	}

	p := &DeepSeekProvider{
		config:   config,
		endpoint: endpoint,
		apiKey:   apiKey,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		lastHealth: &ProviderHealth{
			Status:    "unknown",
			LastCheck: time.Now(),
		},
	}

	p.initializeModels()

	return p, nil
}

func (dp *DeepSeekProvider) GetType() ProviderType  { return ProviderTypeDeepSeek }
func (dp *DeepSeekProvider) GetName() string        { return "DeepSeek" }
func (dp *DeepSeekProvider) GetModels() []ModelInfo { return dp.models }

func (dp *DeepSeekProvider) GetCapabilities() []ModelCapability {
	return []ModelCapability{
		CapabilityTextGeneration,
		CapabilityCodeGeneration,
		CapabilityCodeAnalysis,
		CapabilityPlanning,
		CapabilityDebugging,
		CapabilityRefactoring,
		CapabilityTesting,
	}
}

func (dp *DeepSeekProvider) Generate(ctx context.Context, request *LLMRequest) (*LLMResponse, error) {
	startTime := time.Now()

	openaiRequest, err := dp.convertToOpenAIRequest(request)
	if err != nil {
		return nil, fmt.Errorf("failed to convert request: %v", err)
	}

	response, err := dp.makeOpenAIRequest(ctx, openaiRequest)
	if err != nil {
		return nil, fmt.Errorf("DeepSeek request failed: %v", err)
	}

	return dp.convertFromOpenAIResponse(response, request.ID, time.Since(startTime)), nil
}

func (dp *DeepSeekProvider) GenerateStream(ctx context.Context, request *LLMRequest, ch chan<- LLMResponse) error {
	defer close(ch)

	openaiRequest, err := dp.convertToOpenAIRequest(request)
	if err != nil {
		return fmt.Errorf("failed to convert request: %v", err)
	}
	openaiRequest.Stream = true

	return dp.makeOpenAIStreamRequest(ctx, openaiRequest, ch, request.ID)
}

func (dp *DeepSeekProvider) IsAvailable(ctx context.Context) bool {
	health, err := dp.GetHealth(ctx)
	return err == nil && health.Status == "healthy"
}

func (dp *DeepSeekProvider) GetHealth(ctx context.Context) (*ProviderHealth, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/models", dp.endpoint), nil)
	if err != nil {
		dp.updateHealth("unhealthy", 0, dp.lastHealth.ErrorCount+1)
		return dp.lastHealth, fmt.Errorf("failed to create health check request: %v", err)
	}
	dp.setAuthHeaders(req)

	start := time.Now()
	resp, err := dp.httpClient.Do(req)
	latency := time.Since(start)
	if err != nil {
		dp.updateHealth("unhealthy", latency, dp.lastHealth.ErrorCount+1)
		return dp.lastHealth, fmt.Errorf("health check failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		dp.updateHealth("unhealthy", latency, dp.lastHealth.ErrorCount+1)
		return dp.lastHealth, fmt.Errorf("health check returned status %d", resp.StatusCode)
	}

	var modelsResponse struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&modelsResponse); err != nil {
		dp.updateHealth("degraded", latency, dp.lastHealth.ErrorCount)
		return dp.lastHealth, nil
	}

	dp.updateHealth("healthy", latency, 0)
	dp.lastHealth.ModelCount = len(modelsResponse.Data)
	return dp.lastHealth, nil
}

func (dp *DeepSeekProvider) Close() error {
	dp.httpClient.CloseIdleConnections()
	return nil
}

// GetContextWindow returns DeepSeek's published max context window.
// DeepSeek-V3 / V4 / Reasoner all advertise 128k.
func (dp *DeepSeekProvider) GetContextWindow() int { return 128_000 }

func (dp *DeepSeekProvider) CountTokens(text string) (int, error) {
	return CharBasedTokenCount(text)
}

func (dp *DeepSeekProvider) initializeModels() {
	dp.models = []ModelInfo{
		{
			Name:        "deepseek-chat",
			Provider:    ProviderTypeDeepSeek,
			ContextSize: 128000,
			MaxTokens:   8192,
			Description: "DeepSeek Chat - flagship general-purpose model",
		},
		{
			Name:        "deepseek-reasoner",
			Provider:    ProviderTypeDeepSeek,
			ContextSize: 128000,
			MaxTokens:   8192,
			Description: "DeepSeek Reasoner - reasoning-focused model",
		},
		{
			Name:        "deepseek-coder",
			Provider:    ProviderTypeDeepSeek,
			ContextSize: 128000,
			MaxTokens:   8192,
			Description: "DeepSeek Coder - code-specialised model",
		},
	}

	for i := range dp.models {
		EnrichModelInfo(&dp.models[i])
	}

	log.Printf("✅ DeepSeek provider initialized with %d models", len(dp.models))
}

func (dp *DeepSeekProvider) convertToOpenAIRequest(request *LLMRequest) (*OpenAIRequest, error) {
	var messages []OpenAIMessage
	for _, msg := range request.Messages {
		openaiMsg := OpenAIMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
		if msg.Name != "" {
			openaiMsg.Name = msg.Name
		}
		messages = append(messages, openaiMsg)
	}

	return &OpenAIRequest{
		Model:       request.Model,
		Messages:    messages,
		MaxTokens:   request.MaxTokens,
		Temperature: request.Temperature,
		TopP:        request.TopP,
		Stream:      request.Stream,
	}, nil
}

func (dp *DeepSeekProvider) convertFromOpenAIResponse(openaiResp *OpenAIResponse, requestID uuid.UUID, processingTime time.Duration) *LLMResponse {
	var content string
	if len(openaiResp.Choices) > 0 {
		content = openaiResp.Choices[0].Message.Content
	}

	finish := ""
	if len(openaiResp.Choices) > 0 {
		finish = openaiResp.Choices[0].FinishReason
	}

	return &LLMResponse{
		ID:        uuid.New(),
		RequestID: requestID,
		Content:   content,
		Usage: Usage{
			PromptTokens:     openaiResp.Usage.PromptTokens,
			CompletionTokens: openaiResp.Usage.CompletionTokens,
			TotalTokens:      openaiResp.Usage.TotalTokens,
		},
		FinishReason:   finish,
		ProcessingTime: processingTime,
		CreatedAt:      time.Now(),
		// Round-50 LLMResponse.Err wiring (CONST-035 / Article XI §11.9):
		// DeepSeek is OpenAI-compatible — finish_reason "length" /
		// "content_filter" indicate truncation / content block. Reuse
		// the round-46 OpenAI mapper helper (same closed mapping per
		// https://api-docs.deepseek.com/api/create-chat-completion).
		Err: mapOpenAIFinishReasonToErr(finish),
	}
}

func (dp *DeepSeekProvider) makeOpenAIRequest(ctx context.Context, request *OpenAIRequest) (*OpenAIResponse, error) {
	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/chat/completions", dp.endpoint), bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	dp.setAuthHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := dp.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("DeepSeek API returned status %d: %s", resp.StatusCode, string(body))
	}

	var response OpenAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}
	return &response, nil
}

func (dp *DeepSeekProvider) makeOpenAIStreamRequest(ctx context.Context, request *OpenAIRequest, ch chan<- LLMResponse, requestID uuid.UUID) error {
	jsonData, err := json.Marshal(request)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/chat/completions", dp.endpoint), bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	dp.setAuthHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := dp.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("DeepSeek API returned status %d: %s", resp.StatusCode, string(body))
	}

	decoder := json.NewDecoder(resp.Body)
	for decoder.More() {
		var streamResp OpenAIStreamResponse
		if err := decoder.Decode(&streamResp); err != nil {
			return err
		}

		if len(streamResp.Choices) > 0 {
			choice := streamResp.Choices[0]
			if choice.Delta.Content != "" {
				response := LLMResponse{
					ID:        uuid.New(),
					RequestID: requestID,
					Content:   choice.Delta.Content,
					CreatedAt: time.Now(),
				}

				select {
				case ch <- response:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		}

		if len(streamResp.Choices) > 0 && streamResp.Choices[0].FinishReason != "" {
			// Round-50 LLMResponse.Err wiring for the streaming path
			// (CONST-035 / Article XI §11.9): when the final frame
			// carries finish_reason="length"/"content_filter", emit a
			// terminal LLMResponse with Err populated so stream
			// consumers (notably tool_provider.go :201/:251) can
			// distinguish a clean stop from a partial-error stop.
			finishReason := streamResp.Choices[0].FinishReason
			if errSentinel := mapOpenAIFinishReasonToErr(finishReason); errSentinel != nil {
				select {
				case ch <- LLMResponse{
					ID:           uuid.New(),
					RequestID:    requestID,
					FinishReason: finishReason,
					CreatedAt:    time.Now(),
					Err:          errSentinel,
				}:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			break
		}
	}

	return nil
}

func (dp *DeepSeekProvider) setAuthHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+dp.apiKey)
}

func (dp *DeepSeekProvider) updateHealth(status string, latency time.Duration, errorCount int) {
	dp.lastHealth.Status = status
	dp.lastHealth.Latency = latency
	dp.lastHealth.ErrorCount = errorCount
	dp.lastHealth.LastCheck = time.Now()
}
