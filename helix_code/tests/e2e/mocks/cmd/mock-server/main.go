// Package main provides a mock LLM server that simulates multiple cloud providers
// for testing purposes. This allows running all LLM integration tests without
// requiring actual API keys.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// Server configuration
type Config struct {
	Port         string
	MockOpenAI   bool
	MockAnthropic bool
	MockGemini   bool
	MockAzure    bool
	MockBedrock  bool
	MockGroq     bool
	MockMistral  bool
	MockCohere   bool
	LogLevel     string
}

// Mock server state
type MockServer struct {
	config       Config
	requestCount map[string]int
	mu           sync.RWMutex
}

// OpenAI-compatible request/response structures
type ChatCompletionRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Temperature float64       `json:"temperature,omitempty"`
	Stream      bool          `json:"stream,omitempty"`
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatCompletionResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

type Choice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Anthropic-specific structures
type AnthropicRequest struct {
	Model       string              `json:"model"`
	Messages    []AnthropicMessage  `json:"messages"`
	MaxTokens   int                 `json:"max_tokens"`
	System      string              `json:"system,omitempty"`
}

type AnthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type AnthropicResponse struct {
	ID           string               `json:"id"`
	Type         string               `json:"type"`
	Role         string               `json:"role"`
	Content      []AnthropicContent   `json:"content"`
	Model        string               `json:"model"`
	StopReason   string               `json:"stop_reason"`
	StopSequence *string              `json:"stop_sequence"`
	Usage        AnthropicUsage       `json:"usage"`
}

type AnthropicContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type AnthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// Gemini-specific structures
type GeminiRequest struct {
	Contents []GeminiContent `json:"contents"`
}

type GeminiContent struct {
	Parts []GeminiPart `json:"parts"`
	Role  string       `json:"role,omitempty"`
}

type GeminiPart struct {
	Text string `json:"text"`
}

type GeminiResponse struct {
	Candidates []GeminiCandidate `json:"candidates"`
}

type GeminiCandidate struct {
	Content      GeminiContent `json:"content"`
	FinishReason string        `json:"finishReason"`
}

// Error response
type ErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code,omitempty"`
	} `json:"error"`
}

func main() {
	config := loadConfig()
	server := &MockServer{
		config:       config,
		requestCount: make(map[string]int),
	}

	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("/health", server.healthHandler)
	mux.HandleFunc("/", server.rootHandler)

	// OpenAI endpoints
	if config.MockOpenAI {
		mux.HandleFunc("/v1/chat/completions", server.openAIChatHandler)
		mux.HandleFunc("/v1/models", server.openAIModelsHandler)
		mux.HandleFunc("/v1/embeddings", server.openAIEmbeddingsHandler)
		log.Println("OpenAI mock endpoints enabled")
	}

	// Anthropic endpoints
	if config.MockAnthropic {
		mux.HandleFunc("/v1/messages", server.anthropicMessagesHandler)
		log.Println("Anthropic mock endpoints enabled")
	}

	// Gemini endpoints
	if config.MockGemini {
		mux.HandleFunc("/v1beta/models/", server.geminiHandler)
		mux.HandleFunc("/v1/models/", server.geminiHandler)
		log.Println("Gemini mock endpoints enabled")
	}

	// Azure OpenAI endpoints
	if config.MockAzure {
		mux.HandleFunc("/openai/deployments/", server.azureOpenAIHandler)
		log.Println("Azure OpenAI mock endpoints enabled")
	}

	// AWS Bedrock endpoints
	if config.MockBedrock {
		mux.HandleFunc("/model/", server.bedrockHandler)
		log.Println("AWS Bedrock mock endpoints enabled")
	}

	// Groq endpoints (OpenAI compatible)
	if config.MockGroq {
		mux.HandleFunc("/openai/v1/chat/completions", server.openAIChatHandler)
		log.Println("Groq mock endpoints enabled")
	}

	// Mistral endpoints (OpenAI-compatible). Mistral's real path is
	// /v1/chat/completions, which the OpenAI block above already
	// registers — net/http.ServeMux panics on a duplicate pattern when
	// both MockOpenAI and MockMistral are enabled. Register the OpenAI
	// path for Mistral ONLY when the OpenAI block did not already claim
	// it; always expose a distinct /mistral/v1/chat/completions alias so
	// a Mistral-only base URL still resolves.
	if config.MockMistral {
		if !config.MockOpenAI {
			mux.HandleFunc("/v1/chat/completions", server.openAIChatHandler)
		}
		mux.HandleFunc("/mistral/v1/chat/completions", server.openAIChatHandler)
		log.Println("Mistral mock endpoints enabled")
	}

	// Cohere endpoints
	if config.MockCohere {
		mux.HandleFunc("/v1/generate", server.cohereGenerateHandler)
		mux.HandleFunc("/v1/chat", server.cohereChatHandler)
		log.Println("Cohere mock endpoints enabled")
	}

	addr := fmt.Sprintf(":%s", config.Port)
	log.Printf("Mock LLM Server starting on %s", addr)
	log.Printf("Enabled providers: OpenAI=%v, Anthropic=%v, Gemini=%v, Azure=%v, Bedrock=%v, Groq=%v, Mistral=%v, Cohere=%v",
		config.MockOpenAI, config.MockAnthropic, config.MockGemini, config.MockAzure,
		config.MockBedrock, config.MockGroq, config.MockMistral, config.MockCohere)

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func loadConfig() Config {
	return Config{
		Port:          getEnv("MOCK_PORT", "8090"),
		MockOpenAI:    getEnvBool("MOCK_OPENAI", true),
		MockAnthropic: getEnvBool("MOCK_ANTHROPIC", true),
		MockGemini:    getEnvBool("MOCK_GEMINI", true),
		MockAzure:     getEnvBool("MOCK_AZURE", true),
		MockBedrock:   getEnvBool("MOCK_BEDROCK", true),
		MockGroq:      getEnvBool("MOCK_GROQ", true),
		MockMistral:   getEnvBool("MOCK_MISTRAL", true),
		MockCohere:    getEnvBool("MOCK_COHERE", true),
		LogLevel:      getEnv("LOG_LEVEL", "info"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		return strings.ToLower(value) == "true" || value == "1"
	}
	return defaultValue
}

func (s *MockServer) incrementCount(provider string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requestCount[provider]++
}

func (s *MockServer) healthHandler(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	counts := make(map[string]int)
	for k, v := range s.requestCount {
		counts[k] = v
	}
	s.mu.RUnlock()

	response := map[string]interface{}{
		"status":        "healthy",
		"timestamp":     time.Now().UTC().Format(time.RFC3339),
		"request_counts": counts,
		"providers": map[string]bool{
			"openai":    s.config.MockOpenAI,
			"anthropic": s.config.MockAnthropic,
			"gemini":    s.config.MockGemini,
			"azure":     s.config.MockAzure,
			"bedrock":   s.config.MockBedrock,
			"groq":      s.config.MockGroq,
			"mistral":   s.config.MockMistral,
			"cohere":    s.config.MockCohere,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *MockServer) rootHandler(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"name":    "HelixCode Mock LLM Server",
		"version": "1.0.0",
		"endpoints": []string{
			"/health",
			"/v1/chat/completions (OpenAI)",
			"/v1/models (OpenAI)",
			"/v1/messages (Anthropic)",
			"/v1beta/models/* (Gemini)",
			"/openai/deployments/* (Azure)",
			"/model/* (Bedrock)",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// OpenAI handlers
func (s *MockServer) openAIChatHandler(w http.ResponseWriter, r *http.Request) {
	s.incrementCount("openai")

	if r.Method != http.MethodPost {
		s.sendError(w, "Method not allowed", "invalid_request_error", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.sendError(w, "Failed to read request body", "invalid_request_error", http.StatusBadRequest)
		return
	}

	var req ChatCompletionRequest
	if err := json.Unmarshal(body, &req); err != nil {
		s.sendError(w, "Invalid JSON", "invalid_request_error", http.StatusBadRequest)
		return
	}

	// Generate mock response
	responseContent := generateMockResponse(req.Messages)

	response := ChatCompletionResponse{
		ID:      fmt.Sprintf("chatcmpl-mock-%d", time.Now().UnixNano()),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   req.Model,
		Choices: []Choice{
			{
				Index: 0,
				Message: ChatMessage{
					Role:    "assistant",
					Content: responseContent,
				},
				FinishReason: "stop",
			},
		},
		Usage: Usage{
			PromptTokens:     countTokens(req.Messages),
			CompletionTokens: len(responseContent) / 4,
			TotalTokens:      countTokens(req.Messages) + len(responseContent)/4,
		},
	}

	if req.Stream {
		s.streamOpenAIResponse(w, response)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *MockServer) streamOpenAIResponse(w http.ResponseWriter, response ChatCompletionResponse) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	content := response.Choices[0].Message.Content
	words := strings.Fields(content)

	for i, word := range words {
		chunk := map[string]interface{}{
			"id":      response.ID,
			"object":  "chat.completion.chunk",
			"created": response.Created,
			"model":   response.Model,
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"delta": map[string]string{
						"content": word + " ",
					},
					"finish_reason": nil,
				},
			},
		}

		if i == len(words)-1 {
			chunk["choices"].([]map[string]interface{})[0]["finish_reason"] = "stop"
		}

		data, _ := json.Marshal(chunk)
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
		time.Sleep(50 * time.Millisecond)
	}

	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()
}

func (s *MockServer) openAIModelsHandler(w http.ResponseWriter, r *http.Request) {
	s.incrementCount("openai_models")

	response := map[string]interface{}{
		"object": "list",
		"data": []map[string]interface{}{
			{"id": "gpt-4", "object": "model", "owned_by": "openai"},
			{"id": "gpt-4-turbo", "object": "model", "owned_by": "openai"},
			{"id": "gpt-3.5-turbo", "object": "model", "owned_by": "openai"},
			{"id": "text-embedding-ada-002", "object": "model", "owned_by": "openai"},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *MockServer) openAIEmbeddingsHandler(w http.ResponseWriter, r *http.Request) {
	s.incrementCount("openai_embeddings")

	// Return mock embeddings
	embedding := make([]float64, 1536)
	for i := range embedding {
		embedding[i] = float64(i%100) / 100.0
	}

	response := map[string]interface{}{
		"object": "list",
		"data": []map[string]interface{}{
			{
				"object":    "embedding",
				"embedding": embedding,
				"index":     0,
			},
		},
		"model": "text-embedding-ada-002",
		"usage": map[string]int{
			"prompt_tokens": 10,
			"total_tokens":  10,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Anthropic handlers
func (s *MockServer) anthropicMessagesHandler(w http.ResponseWriter, r *http.Request) {
	s.incrementCount("anthropic")

	if r.Method != http.MethodPost {
		s.sendError(w, "Method not allowed", "invalid_request_error", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.sendError(w, "Failed to read request body", "invalid_request_error", http.StatusBadRequest)
		return
	}

	var req AnthropicRequest
	if err := json.Unmarshal(body, &req); err != nil {
		s.sendError(w, "Invalid JSON", "invalid_request_error", http.StatusBadRequest)
		return
	}

	// Convert messages to chat messages for response generation
	var messages []ChatMessage
	for _, m := range req.Messages {
		messages = append(messages, ChatMessage{Role: m.Role, Content: m.Content})
	}

	responseContent := generateMockResponse(messages)

	response := AnthropicResponse{
		ID:   fmt.Sprintf("msg_mock_%d", time.Now().UnixNano()),
		Type: "message",
		Role: "assistant",
		Content: []AnthropicContent{
			{Type: "text", Text: responseContent},
		},
		Model:      req.Model,
		StopReason: "end_turn",
		Usage: AnthropicUsage{
			InputTokens:  countTokensFromAnthropicMessages(req.Messages),
			OutputTokens: len(responseContent) / 4,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Gemini handlers
func (s *MockServer) geminiHandler(w http.ResponseWriter, r *http.Request) {
	s.incrementCount("gemini")

	if r.Method != http.MethodPost {
		s.sendError(w, "Method not allowed", "invalid_request_error", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.sendError(w, "Failed to read request body", "invalid_request_error", http.StatusBadRequest)
		return
	}

	var req GeminiRequest
	if err := json.Unmarshal(body, &req); err != nil {
		s.sendError(w, "Invalid JSON", "invalid_request_error", http.StatusBadRequest)
		return
	}

	// Generate response content
	var inputText string
	for _, content := range req.Contents {
		for _, part := range content.Parts {
			inputText += part.Text + " "
		}
	}

	responseContent := fmt.Sprintf("Mock Gemini response for: %s...", truncate(inputText, 50))

	response := GeminiResponse{
		Candidates: []GeminiCandidate{
			{
				Content: GeminiContent{
					Parts: []GeminiPart{{Text: responseContent}},
					Role:  "model",
				},
				FinishReason: "STOP",
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Azure OpenAI handler
func (s *MockServer) azureOpenAIHandler(w http.ResponseWriter, r *http.Request) {
	s.incrementCount("azure")
	// Azure OpenAI uses the same format as OpenAI
	s.openAIChatHandler(w, r)
}

// AWS Bedrock handler
func (s *MockServer) bedrockHandler(w http.ResponseWriter, r *http.Request) {
	s.incrementCount("bedrock")

	if r.Method != http.MethodPost {
		s.sendError(w, "Method not allowed", "invalid_request_error", http.StatusMethodNotAllowed)
		return
	}

	// Bedrock has different formats based on model
	// Simplified response for testing
	response := map[string]interface{}{
		"completion": "Mock Bedrock response for testing purposes.",
		"stop_reason": "stop",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Cohere handlers
func (s *MockServer) cohereGenerateHandler(w http.ResponseWriter, r *http.Request) {
	s.incrementCount("cohere_generate")

	response := map[string]interface{}{
		"id":          fmt.Sprintf("gen-%d", time.Now().UnixNano()),
		"generations": []map[string]interface{}{
			{"text": "Mock Cohere generate response for testing."},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *MockServer) cohereChatHandler(w http.ResponseWriter, r *http.Request) {
	s.incrementCount("cohere_chat")

	response := map[string]interface{}{
		"response_id": fmt.Sprintf("chat-%d", time.Now().UnixNano()),
		"text":        "Mock Cohere chat response for testing.",
		"generation_id": fmt.Sprintf("gen-%d", time.Now().UnixNano()),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Helper functions
func (s *MockServer) sendError(w http.ResponseWriter, message, errType string, statusCode int) {
	response := ErrorResponse{}
	response.Error.Message = message
	response.Error.Type = errType

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}

func generateMockResponse(messages []ChatMessage) string {
	if len(messages) == 0 {
		return "Hello! I'm a mock LLM response for testing purposes."
	}

	lastMessage := messages[len(messages)-1]
	content := lastMessage.Content

	// Generate contextual mock responses
	lowerContent := strings.ToLower(content)

	switch {
	case strings.Contains(lowerContent, "hello") || strings.Contains(lowerContent, "hi"):
		return "Hello! I'm a mock LLM server responding to your greeting. How can I help you today?"
	case strings.Contains(lowerContent, "code") || strings.Contains(lowerContent, "function"):
		return "Here's a mock code response:\n\n```go\nfunc example() string {\n    return \"mock response\"\n}\n```\n\nThis is a test response from the mock LLM server."
	case strings.Contains(lowerContent, "test"):
		return "This is a test response from the mock LLM server. All systems are functioning correctly for testing purposes."
	case strings.Contains(lowerContent, "error"):
		return "I understand you're asking about errors. This is a mock response to help test error handling scenarios."
	default:
		return fmt.Sprintf("Mock LLM response for: %s\n\nThis response is generated by the HelixCode Mock LLM Server for testing purposes. The actual response would come from a real LLM provider.", truncate(content, 100))
	}
}

func countTokens(messages []ChatMessage) int {
	total := 0
	for _, m := range messages {
		total += len(m.Content) / 4 // Rough approximation
	}
	return total
}

func countTokensFromAnthropicMessages(messages []AnthropicMessage) int {
	total := 0
	for _, m := range messages {
		total += len(m.Content) / 4
	}
	return total
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
