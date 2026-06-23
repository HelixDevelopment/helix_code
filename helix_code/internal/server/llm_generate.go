package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"dev.helix.code/internal/llm"
	"github.com/gin-gonic/gin"
)

// errUnknownProvider is returned by resolveLLMProvider when the request (or
// HELIX_LLM_PROVIDER) explicitly names a provider that llm.Select cannot
// resolve. It is distinct from a construction/availability failure: the user
// supplied an invalid provider name, so the handler answers 400 (client error)
// rather than silently falling back to the local Ollama default — that silent
// fallback turned a provider typo into a misleading Ollama 404 (server
// defect #4). The named provider is wrapped so the error body can echo it.
var errUnknownProvider = errors.New("unknown provider")

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

// llmProviderResolver is the indirection the handlers call to obtain a provider
// for a request. It defaults to the real resolveLLMProvider below. It is a
// package-level var (not a hardcoded call) ONLY so unit tests can substitute a
// real-but-deterministic provider to exercise the streaming goroutine's
// channel-ownership behaviour without a live network/Ollama — production code
// never reassigns it, so the default real path is always what ships.
var llmProviderResolver = resolveLLMProvider

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

	// The name the caller actually supplied (request body field or env), used
	// for honest error reporting on the unknown-provider path.
	requested := strings.TrimSpace(sel.Flag)
	if requested == "" {
		requested = strings.TrimSpace(sel.Env)
	}

	ptype, selErr := llm.Select(sel)
	switch {
	case selErr == nil:
		// A provider was named and resolved to a known type — construct it.
		entry := llm.ProviderConfigEntry{Type: ptype, Enabled: true}
		if strings.TrimSpace(model) != "" {
			entry.Models = []string{model}
		}
		provider, cErr := llm.NewCloudProvider(ptype, entry)
		if cErr != nil {
			return nil, fmt.Errorf("failed to construct provider %q: %w", ptype, cErr)
		}
		if provider != nil {
			return provider, nil
		}
		// Defensive: NewCloudProvider returned (nil, nil) — treat as a real
		// construction failure rather than silently masking it as Ollama.
		return nil, fmt.Errorf("provider %q constructed nil without an error", ptype)

	case errors.Is(selErr, llm.ErrNoProviderConfigured):
		// No provider named anywhere — fall through to the local Ollama default
		// below (out-of-the-box behaviour for a zero-config server with Ollama).

	default:
		// A provider WAS explicitly named but llm.Select could not resolve it
		// (unknown/unsupported provider string). Do NOT silently fall back to
		// Ollama — that masks the user's typo as an unrelated Ollama 404
		// (server defect #4). Surface a clear unknown-provider error so the
		// handler can answer 400.
		return nil, fmt.Errorf("%w: %q", errUnknownProvider, requested)
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

// resolveDefaultModel fills in a request that OMITTED the model with a
// VERIFIED-AVAILABLE model from the provider's own catalog.
//
// CONST-036 / CONST-037: LLMsVerifier is the single source of truth and every
// model surfaced MUST be verified-available, so the default is sourced from
// provider.GetModels() (which, for the OpenAI-compatible cloud providers,
// refreshes LIVE from the provider's own `GET /models` on first call) — never
// a hardcoded literal. The first catalog entry is the provider's currently
// served, leading model.
//
// The historical defect this guards (server defect: empty/default-model
// Generate → upstream 400 → API 502): a Generate that omitted the model left
// llm.LLMRequest.Model == "" all the way to the wire. DeepSeek (and any
// provider that does not synthesise its own default) then rejected the empty
// model — e.g. `400: "The supported API model names are deepseek-v4-pro or
// deepseek-v4-flash, but you passed ."`. The fix never lets an empty model
// reach the provider when the catalog can supply a verified one.
//
// When the request already names a model, or the catalog is empty (offline /
// unreachable provider — that staleness is the verifier's concern, not the
// server's to mask), the model is left unchanged and the provider's own
// default-handling / honest error path takes over. This is the same behaviour
// as before for those cases — the change is strictly additive on the
// previously-broken empty-model-against-a-reachable-catalog path.
func resolveDefaultModel(provider llm.Provider, requested string) string {
	if strings.TrimSpace(requested) != "" {
		return requested
	}
	for _, m := range provider.GetModels() {
		// Prefer the verifier-facing Name; fall back to the catalog ID when a
		// provider populates only ID. Skip blank entries defensively.
		if name := strings.TrimSpace(m.Name); name != "" {
			return name
		}
		if id := strings.TrimSpace(m.ID); id != "" {
			return id
		}
	}
	// Catalog empty (offline/unreachable). Leave it empty: the provider's own
	// default-or-honest-error path handles it — we do NOT invent a model.
	return ""
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

	provider, err := llmProviderResolver(req.Provider, req.Model)
	if err != nil {
		c.JSON(providerResolveStatus(err), gin.H{"status": "error", "error": err.Error()})
		return
	}
	defer func() { _ = provider.Close() }()

	// CONST-036/037: when the request omitted the model, resolve it to a
	// verified-available model from the provider's catalog so an empty model is
	// never sent upstream (server defect: empty-model → provider 400 → API 502).
	llmReq.Model = resolveDefaultModel(provider, llmReq.Model)

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

	provider, err := llmProviderResolver(req.Provider, req.Model)
	if err != nil {
		c.JSON(providerResolveStatus(err), gin.H{"status": "error", "error": err.Error()})
		return
	}
	defer func() { _ = provider.Close() }()

	// CONST-036/037: same verified-available default-model resolution as the
	// non-streaming path — an omitted model must not reach the provider empty.
	llmReq.Model = resolveDefaultModel(provider, llmReq.Model)

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")

	ctx, cancel := context.WithTimeout(c.Request.Context(), 120*time.Second)
	defer cancel()

	// CHANNEL-OWNERSHIP CONTRACT (see llm.Provider.GenerateStream interface doc):
	// the PROVIDER is the SENDER and the SOLE closer of chunkChan — it closes the
	// channel on every return path (success, error, ctx-cancel). This consumer
	// MUST NOT close chunkChan. Closing it here too would be a double-close, which
	// panics ("close of closed channel") inside this producer goroutine; that
	// panic is NOT recoverable by gin.Recovery and crashes the whole server
	// process — a single client request could remotely kill the server
	// (server defect #5; CONST-035 / Article XI §11.9). The provider's guaranteed
	// close is what lets streamProviderToSSE observe the drain, emit the terminal
	// `data: [DONE]` frame, and return without waiting for the 120s ctx deadline.
	chunkChan := make(chan llm.LLMResponse, 100)
	errCh := make(chan error, 1)
	go func() {
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

// providerResolveStatus maps a resolveLLMProvider error to the right HTTP
// status. An explicitly-named-but-unknown provider is a client error (400 —
// the caller typed an invalid provider name); any other resolution failure
// (construction/credentials/endpoint) is a 503 the operator can act on.
func providerResolveStatus(err error) int {
	if errors.Is(err, errUnknownProvider) {
		return http.StatusBadRequest
	}
	return http.StatusServiceUnavailable
}

// envLLMProvider reads HELIX_LLM_PROVIDER. Factored out so the resolution path
// has a single, testable env touch point.
func envLLMProvider() string {
	return os.Getenv("HELIX_LLM_PROVIDER")
}
