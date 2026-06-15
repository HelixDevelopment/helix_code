package server

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"dev.helix.code/internal/llm"
	"github.com/gin-gonic/gin"
)

// llm_generate.go — real LLM generation surface over HTTP.
//
// Anti-bluff (CONST-035 / BLUFF-001 / Article XI §11.9): these handlers make
// REAL calls to a REAL provider via the existing llm.Provider interface
// (Generate / GenerateStream). There is NO simulation, NO hardcoded canned
// response, NO print-and-sleep. Every byte returned to the caller originates
// from a provider's Generate / GenerateStream return value.
//
// Provider resolution mirrors cmd/cli/main.go exactly (no invented provider
// API): a cloud provider is selected via llm.Select (flag/env/config
// precedence) + constructed with llm.NewCloudProvider when a provider is
// named (request body `provider` field or HELIX_LLM_PROVIDER); otherwise the
// handler falls back to a local Ollama provider on the standard port — the
// same default NewCLI() and the subagent path use. Provider constructors
// resolve their own credentials from the process environment (loaded at
// server startup by secrets.LoadAPIKeys), so no key value is ever read,
// logged, or persisted here (CONST-042 / §12.1).
//
// The Server.llm field stays nil by design (see server.go New()): the
// provider is constructed PER REQUEST so a key rotated into the environment,
// or a different `provider` per call, is honoured without a server restart,
// and so a missing-key provider surfaces a real runtime auth error from the
// provider call rather than a fabricated "available" status.

// llmGenerateRequest is the JSON body accepted by POST /api/v1/llm/generate
// and POST /api/v1/llm/stream.
type llmGenerateRequest struct {
	// Prompt is the user message. Required. Either Prompt or a non-empty
	// Messages slice must be supplied.
	Prompt string `json:"prompt"`
	// Messages is an optional full chat transcript. When supplied it takes
	// precedence over Prompt (Prompt is appended as a trailing user turn if
	// both are present and non-empty).
	Messages []llm.Message `json:"messages"`
	// Model is the model id to target (e.g. "llama3.2", "claude-3-5-sonnet").
	// Optional — when empty the provider's default model is used.
	Model string `json:"model"`
	// Provider optionally names the provider to use (e.g. "anthropic",
	// "ollama"). When empty, HELIX_LLM_PROVIDER / local-Ollama default apply.
	Provider string `json:"provider"`
	// MaxTokens caps the response length. Optional (0 ⇒ provider default).
	MaxTokens int `json:"max_tokens"`
	// Temperature controls sampling. Optional (0 ⇒ provider default).
	Temperature float64 `json:"temperature"`
}

// buildLLMRequest converts the wire request into an llm.LLMRequest, applying
// the prompt/messages precedence rule. Returns an error string (empty when ok).
func (r *llmGenerateRequest) buildLLMRequest(stream bool) (*llm.LLMRequest, string) {
	messages := make([]llm.Message, 0, len(r.Messages)+1)
	messages = append(messages, r.Messages...)
	if strings.TrimSpace(r.Prompt) != "" {
		messages = append(messages, llm.Message{Role: "user", Content: r.Prompt})
	}
	if len(messages) == 0 {
		return nil, "request must include a non-empty 'prompt' or 'messages'"
	}
	return &llm.LLMRequest{
		Model:       r.Model,
		Messages:    messages,
		MaxTokens:   r.MaxTokens,
		Temperature: r.Temperature,
		Stream:      stream,
	}, ""
}

// resolveLLMProvider constructs a real llm.Provider for this request.
//
// It reuses the exact construction path cmd/cli/main.go uses:
//   - When `providerName` (or HELIX_LLM_PROVIDER) names a known provider,
//     llm.Select resolves the ProviderType and llm.NewCloudProvider builds it.
//   - Otherwise a local Ollama provider on the standard port is returned,
//     mirroring NewCLI()'s default so an out-of-the-box server with Ollama
//     running can generate with zero configuration.
//
// The provider is the caller's responsibility to Close().
func resolveLLMProvider(providerName, model string) (llm.Provider, error) {
	sel := llm.SelectorInput{
		Flag:   strings.TrimSpace(providerName),
		Env:    "", // HELIX_LLM_PROVIDER picked up below only when Flag empty
		Config: "",
	}
	// Honour HELIX_LLM_PROVIDER only when the request did not name a provider,
	// matching the flag>env precedence cmd/cli applies.
	if sel.Flag == "" {
		sel.Env = strings.TrimSpace(envLLMProvider())
	}

	ptype, selErr := llm.Select(sel)
	if selErr == nil {
		entry := llm.ProviderConfigEntry{Type: ptype, Enabled: true}
		if strings.TrimSpace(model) != "" {
			entry.Models = []string{model}
		}
		provider, cErr := llm.NewCloudProvider(ptype, entry)
		if cErr == nil && provider != nil {
			return provider, nil
		}
		// Fall through to the local default on construction failure, exactly
		// like the cmd/cli subagent path: surface nothing fake, degrade to a
		// provider that can actually run locally.
		if cErr != nil {
			return nil, fmt.Errorf("failed to construct provider %q: %w", ptype, cErr)
		}
	}

	// Default: local Ollama on the standard port (mirrors NewCLI()).
	defaultModel := strings.TrimSpace(model)
	if defaultModel == "" {
		defaultModel = "llama3.2"
	}
	provider, err := llm.NewOllamaProvider(llm.OllamaConfig{
		DefaultModel:  defaultModel,
		BaseURL:       "http://localhost:11434",
		StreamEnabled: true,
	})
	if err != nil {
		return nil, fmt.Errorf("default Ollama provider construction failed: %w", err)
	}
	return provider, nil
}

// generateLLM handles POST /api/v1/llm/generate — a real, non-streaming
// completion. It returns the provider's actual response Content plus usage.
func (s *Server) generateLLM(c *gin.Context) {
	var req llmGenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  fmt.Sprintf("invalid request body: %v", err),
		})
		return
	}

	llmReq, validationErr := req.buildLLMRequest(false)
	if validationErr != "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": validationErr})
		return
	}

	provider, err := resolveLLMProvider(req.Provider, req.Model)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "error", "error": err.Error()})
		return
	}
	defer func() { _ = provider.Close() }()

	ctx, cancel := context.WithTimeout(c.Request.Context(), 120*time.Second)
	defer cancel()

	resp, genErr := provider.Generate(ctx, llmReq)
	if genErr != nil {
		// Real provider error (auth failure, model not found, network) —
		// surfaced honestly, never masked as success (CONST-035).
		c.JSON(http.StatusBadGateway, gin.H{
			"status":   "error",
			"error":    fmt.Sprintf("generation failed: %v", genErr),
			"provider": provider.GetName(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":   "success",
		"content":  resp.Content,
		"provider": provider.GetName(),
		"model":    llmReq.Model,
		"usage": gin.H{
			"prompt_tokens":     resp.Usage.PromptTokens,
			"completion_tokens": resp.Usage.CompletionTokens,
			"total_tokens":      resp.Usage.TotalTokens,
		},
		"finish_reason": resp.FinishReason,
	})
}

// streamLLM handles POST /api/v1/llm/stream — a real, streaming completion
// emitted as Server-Sent Events. Each chunk's Content is forwarded as it
// arrives from the provider's GenerateStream channel; a terminal `[DONE]`
// event closes the stream.
func (s *Server) streamLLM(c *gin.Context) {
	var req llmGenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  fmt.Sprintf("invalid request body: %v", err),
		})
		return
	}

	llmReq, validationErr := req.buildLLMRequest(true)
	if validationErr != "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": validationErr})
		return
	}

	provider, err := resolveLLMProvider(req.Provider, req.Model)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "error", "error": err.Error()})
		return
	}
	defer func() { _ = provider.Close() }()

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")

	ctx, cancel := context.WithTimeout(c.Request.Context(), 120*time.Second)
	defer cancel()

	chunkChan := make(chan llm.LLMResponse, 100)
	errCh := make(chan error, 1)
	go func() {
		// Close chunkChan once the provider stops producing so the consumer
		// (streamProviderToSSE) observes the channel-drain, emits the terminal
		// `data: [DONE]` frame, and returns. Without this close the consumer
		// blocks on <-chunkChan after the final real chunk and the client never
		// sees [DONE] until the 120s ctx deadline — a real hang the streaming
		// e2e (tests/integration/llm_stream_e2e_test.go) exposes (CONST-035 /
		// Article XI §11.9).
		defer close(chunkChan)
		errCh <- provider.GenerateStream(ctx, llmReq, chunkChan)
	}()

	// c.Stream pumps the provider channel to the client. Returning false from
	// the step function ends the stream. Each real chunk is forwarded as an
	// SSE `data:` frame; provider errors and EOF terminate honestly.
	streamErr := streamProviderToSSE(c, chunkChan, errCh)
	if streamErr != nil {
		// Best-effort error frame; the stream may already be partially
		// written, so we cannot change the status code here.
		fmt.Fprintf(c.Writer, "event: error\ndata: %s\n\n", streamErr.Error())
		c.Writer.(interface{ Flush() }).Flush()
	}
}

// streamProviderToSSE forwards provider chunks to the SSE writer until the
// channel closes or the provider reports an error. Returns the provider error
// (if any) once streaming completes.
func streamProviderToSSE(c *gin.Context, chunkChan <-chan llm.LLMResponse, errCh <-chan error) error {
	flusher, _ := c.Writer.(interface{ Flush() })
	for {
		select {
		case <-c.Request.Context().Done():
			return c.Request.Context().Err()
		case chunk, ok := <-chunkChan:
			if !ok {
				// Channel drained — collect the provider's terminal error.
				fmt.Fprint(c.Writer, "data: [DONE]\n\n")
				if flusher != nil {
					flusher.Flush()
				}
				if perr := <-errCh; perr != nil && perr != io.EOF {
					return fmt.Errorf("streaming generation failed: %w", perr)
				}
				return nil
			}
			if chunk.Content != "" {
				fmt.Fprintf(c.Writer, "data: %s\n\n", sseEscape(chunk.Content))
				if flusher != nil {
					flusher.Flush()
				}
			}
			if chunk.Err != nil {
				return fmt.Errorf("streaming generation failed: %w", chunk.Err)
			}
		}
	}
}

// sseEscape replaces newlines in a chunk so a multi-line token does not break
// the SSE framing (each `data:` line is a single logical field).
func sseEscape(s string) string {
	return strings.ReplaceAll(s, "\n", "\\n")
}

// envLLMProvider reads HELIX_LLM_PROVIDER. Factored out so the resolution path
// has a single, testable env touch point.
func envLLMProvider() string {
	return os.Getenv("HELIX_LLM_PROVIDER")
}
