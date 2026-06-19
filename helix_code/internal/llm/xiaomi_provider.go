package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"time"
)

const (
	xiaomiDefaultBaseURL = "https://api.xiaomimimo.com/v1"
	xiaomiDefaultTimeout = 120 * time.Second
)

// xiaomiSeedModels is the verified offline fallback model list.
// CONST-036: the primary model list comes from GET /v1/models (live);
// this seed is used ONLY when the endpoint is unreachable.
var xiaomiSeedModels = []ModelInfo{
	{
		Name:        "mimo-v2.5-pro",
		Provider:    ProviderTypeXiaomi,
		ContextSize: 1000000,
		MaxTokens:   128000,
		Capabilities: []ModelCapability{
			CapabilityTextGeneration, CapabilityCodeGeneration,
			CapabilityReasoning, CapabilityPlanning,
		},
		SupportsTools: true,
		Description:   "Xiaomi MiMo V2.5 Pro - flagship text generation, 1M context, deep thinking, tool calling",
	},
	{
		Name:        "mimo-v2.5",
		Provider:    ProviderTypeXiaomi,
		ContextSize: 1000000,
		MaxTokens:   128000,
		Capabilities: []ModelCapability{
			CapabilityTextGeneration, CapabilityCodeGeneration,
			CapabilityVision, CapabilityReasoning,
		},
		SupportsTools:  true,
		SupportsVision: true,
		Description:    "Xiaomi MiMo V2.5 - omni-modal (text/image/video/audio), 1M context, tool calling",
	},
	{
		Name:        "mimo-v2-pro",
		Provider:    ProviderTypeXiaomi,
		ContextSize: 1000000,
		MaxTokens:   128000,
		Capabilities: []ModelCapability{
			CapabilityTextGeneration, CapabilityCodeGeneration,
			CapabilityReasoning,
		},
		SupportsTools: true,
		Description:   "Xiaomi MiMo V2 Pro - text generation, 1M context (deprecated 2026-06-30, routes to V2.5)",
	},
	{
		Name:        "mimo-v2-omni",
		Provider:    ProviderTypeXiaomi,
		ContextSize: 256000,
		MaxTokens:   128000,
		Capabilities: []ModelCapability{
			CapabilityTextGeneration, CapabilityVision, CapabilityReasoning,
		},
		SupportsTools:  true,
		SupportsVision: true,
		Description:    "Xiaomi MiMo V2 Omni - multimodal, 256K context (deprecated 2026-06-30, routes to V2.5)",
	},
	{
		Name:        "mimo-v2-flash",
		Provider:    ProviderTypeXiaomi,
		ContextSize: 256000,
		MaxTokens:   64000,
		Capabilities: []ModelCapability{
			CapabilityTextGeneration, CapabilityCodeGeneration,
			CapabilityReasoning,
		},
		SupportsTools: true,
		Description:   "Xiaomi MiMo V2 Flash - fast text generation, 256K context (deprecated 2026-06-30, routes to V2.5)",
	},
	{
		Name:        "mimo-v2.5-asr",
		Provider:    ProviderTypeXiaomi,
		ContextSize: 8000,
		MaxTokens:   2000,
		Capabilities: []ModelCapability{
			CapabilityTextGeneration,
		},
		Description: "Xiaomi MiMo V2.5 ASR - speech recognition (Chinese dialects, English, code-switch)",
	},
	{
		Name:        "mimo-v2.5-tts",
		Provider:    ProviderTypeXiaomi,
		ContextSize: 8000,
		MaxTokens:   8000,
		Capabilities: []ModelCapability{
			CapabilityTextGeneration,
		},
		Description: "Xiaomi MiMo V2.5 TTS - speech synthesis with natural language style instructions",
	},
	{
		Name:        "mimo-v2.5-tts-voiceclone",
		Provider:    ProviderTypeXiaomi,
		ContextSize: 8000,
		MaxTokens:   8000,
		Capabilities: []ModelCapability{
			CapabilityTextGeneration,
		},
		Description: "Xiaomi MiMo V2.5 TTS Voice Clone - speech synthesis with timbre cloning from reference audio",
	},
	{
		Name:        "mimo-v2.5-tts-voicedesign",
		Provider:    ProviderTypeXiaomi,
		ContextSize: 8000,
		MaxTokens:   8000,
		Capabilities: []ModelCapability{
			CapabilityTextGeneration,
		},
		Description: "Xiaomi MiMo V2.5 TTS Voice Design - speech synthesis with timbre design from text description",
	},
	{
		Name:        "mimo-v2-tts",
		Provider:    ProviderTypeXiaomi,
		ContextSize: 8000,
		MaxTokens:   8000,
		Capabilities: []ModelCapability{
			CapabilityTextGeneration,
		},
		Description: "Xiaomi MiMo V2 TTS - speech synthesis (deprecated 2026-06-30, routes to V2.5 TTS)",
	},
}

// XiaomiProvider implements the Provider interface for Xiaomi MiMo models.
// Text generation delegates to an embedded OpenAICompatibleProvider.
type XiaomiProvider struct {
	oaiProvider *OpenAICompatibleProvider
	baseURL     string
	apiKey      string
	httpClient  *http.Client
	models      []ModelInfo
}

// NewXiaomiProvider creates a new Xiaomi MiMo provider.
func NewXiaomiProvider(config ProviderConfigEntry) (*XiaomiProvider, error) {
	baseURL := config.Endpoint
	if baseURL == "" {
		baseURL = xiaomiDefaultBaseURL
	}

	timeout := xiaomiDefaultTimeout
	if val, ok := config.Parameters["timeout"].(float64); ok {
		timeout = time.Duration(val) * time.Second
	}

	httpClient := &http.Client{Timeout: timeout}

	// Create embedded OpenAI-compatible provider for text generation
	oaiConfig := OpenAICompatibleConfig{
		BaseURL:          baseURL,
		APIKey:           config.APIKey,
		DefaultModel:     "mimo-v2.5",
		Timeout:          timeout,
		MaxRetries:       3,
		StreamingSupport: true,
		ModelEndpoint:    "/models",
		ChatEndpoint:     "/chat/completions",
	}
	if len(config.Models) > 0 {
		oaiConfig.DefaultModel = config.Models[0]
	}

	oaiProvider, err := NewOpenAICompatibleProvider("xiaomi", oaiConfig)
	if err != nil {
		return nil, fmt.Errorf("create embedded OpenAI-compatible provider: %w", err)
	}

	provider := &XiaomiProvider{
		oaiProvider: oaiProvider,
		baseURL:     baseURL,
		apiKey:      config.APIKey,
		httpClient:  httpClient,
		models:      xiaomiSeedModels,
	}

	// Override seed models with live catalogue if available
	if liveModels := oaiProvider.GetModels(); len(liveModels) > 0 {
		provider.models = liveModels
		log.Printf("Xiaomi provider initialized with %d live models", len(liveModels))
	} else {
		log.Printf("Xiaomi provider using seed model list (%d models)", len(provider.models))
	}

	return provider, nil
}

// GetType returns the provider type.
func (p *XiaomiProvider) GetType() ProviderType { return ProviderTypeXiaomi }

// GetName returns the provider name.
func (p *XiaomiProvider) GetName() string { return "xiaomi" }

// GetModels returns the list of available Xiaomi MiMo models.
func (p *XiaomiProvider) GetModels() []ModelInfo { return p.models }

// GetCapabilities returns the provider-level capabilities.
func (p *XiaomiProvider) GetCapabilities() []ModelCapability {
	return []ModelCapability{
		CapabilityTextGeneration, CapabilityCodeGeneration,
		CapabilityCodeAnalysis, CapabilityReasoning,
		CapabilityPlanning, CapabilityDebugging, CapabilityVision,
	}
}

// Generate sends a non-streaming completion request via the embedded
// OpenAI-compatible provider.
func (p *XiaomiProvider) Generate(ctx context.Context, request *LLMRequest) (*LLMResponse, error) {
	return p.oaiProvider.Generate(ctx, request)
}

// GenerateStream sends a streaming completion request via the embedded
// OpenAI-compatible provider.
func (p *XiaomiProvider) GenerateStream(ctx context.Context, request *LLMRequest, ch chan<- LLMResponse) error {
	return p.oaiProvider.GenerateStream(ctx, request, ch)
}

// IsAvailable checks if the underlying provider is reachable.
func (p *XiaomiProvider) IsAvailable(ctx context.Context) bool {
	return p.oaiProvider.IsAvailable(ctx)
}

// GetHealth returns the health status of the underlying provider.
func (p *XiaomiProvider) GetHealth(ctx context.Context) (*ProviderHealth, error) {
	return p.oaiProvider.GetHealth(ctx)
}

// Close releases resources held by the provider.
func (p *XiaomiProvider) Close() error {
	return p.oaiProvider.Close()
}

// GetContextWindow returns the maximum context window across all known models.
func (p *XiaomiProvider) GetContextWindow() int {
	maxCtx := 0
	for _, m := range p.models {
		if m.ContextSize > maxCtx {
			maxCtx = m.ContextSize
		}
	}
	if maxCtx == 0 {
		maxCtx = 256000
	}
	return maxCtx
}

// CountTokens returns an estimated token count for the given text.
// Delegates to the char-based fallback (1 token ~ 3.5 chars).
func (p *XiaomiProvider) CountTokens(text string) (int, error) {
	return CharBasedTokenCount(text)
}

// TranscribeAudio performs speech-to-text using Xiaomi MiMo ASR.
// Endpoint: POST /v1/audio/transcriptions
// Model: mimo-v2.5-asr
func (p *XiaomiProvider) TranscribeAudio(ctx context.Context, audioData []byte, filename string) (string, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return "", fmt.Errorf("create form file: %w", err)
	}
	if _, err := part.Write(audioData); err != nil {
		return "", fmt.Errorf("write audio data: %w", err)
	}

	if err := writer.WriteField("model", "mimo-v2.5-asr"); err != nil {
		return "", fmt.Errorf("write model field: %w", err)
	}

	writer.Close()

	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/v1/audio/transcriptions", &buf)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("transcription request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("transcription failed (%d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	return result.Text, nil
}

// SynthesizeSpeech performs text-to-speech using Xiaomi MiMo TTS.
// Endpoint: POST /v1/audio/speech
// Models: mimo-v2.5-tts, mimo-v2.5-tts-voiceclone, mimo-v2.5-tts-voicedesign
func (p *XiaomiProvider) SynthesizeSpeech(ctx context.Context, text string, model string) ([]byte, error) {
	if model == "" {
		model = "mimo-v2.5-tts"
	}

	reqBody := map[string]interface{}{
		"model": model,
		"input": text,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/v1/audio/speech", bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tts request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("tts failed (%d): %s", resp.StatusCode, string(body))
	}

	audioData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read audio response: %w", err)
	}

	return audioData, nil
}
