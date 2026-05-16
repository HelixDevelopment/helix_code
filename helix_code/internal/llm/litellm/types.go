package litellm

import (
	"time"

	"dev.helix.code/internal/llm"
)

type ResponseFormat string

const (
	FormatOpenAI    ResponseFormat = "openai"
	FormatAnthropic ResponseFormat = "anthropic"
	FormatGoogle    ResponseFormat = "google"
)

type FormatAdapter interface {
	Format() ResponseFormat
	ConvertRequest(req *llm.LLMRequest) (interface{}, error)
	ConvertResponse(raw interface{}) (*llm.LLMResponse, error)
	ConvertStreamChunk(raw interface{}) (*LLMStreamChunk, error)
}

type LLMStreamChunk struct {
	Content string
	Done    bool
}

type UnifiedProviderConfig struct {
	Adapter      FormatAdapter
	Endpoint     string
	APIKey       string
	Timeout      time.Duration
	DefaultModel string
	MaxTokens    int
	Temperature  float64
}

type ProviderInfo struct {
	Name           string
	Format         ResponseFormat
	Endpoint       string
	AuthType       string
	DefaultModel   string
	SupportsStream bool
	MaxContext     int
	Models         []string
}