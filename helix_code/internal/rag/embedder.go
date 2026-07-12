// Embedder + OllamaEmbedder — the real embedding backend for CONST-040
// §HXC-118 Phase 2/3.
//
// digital.vasic.rag (the submodule) ships retrieval/pipeline machinery but
// deliberately no concrete embedding call (§11.4.28(B): the submodule
// cannot know HelixCode's embedding provider). OllamaEmbedder is that
// missing HelixCode-side piece: a genuine HTTP client against Ollama's
// real /api/embeddings endpoint.
//
// ANTI-BLUFF (§11.4.6, CLAUDE.md Rule 2): Embed always performs a real
// network round-trip. It never returns a fabricated, hashed, random, or
// otherwise synthetic vector standing in for a real embedding — every
// failure mode (unreachable server, non-200 status, malformed response,
// empty vector) surfaces as an error instead.
package rag

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Embedder produces a real embedding vector for a piece of text. The only
// production implementation is OllamaEmbedder; test doubles are permitted
// in unit tests only (§11.4.27 / CONST-050(A)).
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float64, error)
}

// OllamaEmbedder is a real HTTP client for Ollama's embeddings endpoint:
//
//	POST {baseURL}/api/embeddings
//	  {"model": "<model>", "prompt": "<text>"}
//	  -> {"embedding": [<float64>, ...]}
//
// Reference: https://github.com/ollama/ollama/blob/main/docs/api.md#generate-embeddings
type OllamaEmbedder struct {
	baseURL    string
	model      string
	httpClient *http.Client
}

// NewOllamaEmbedder constructs a real Ollama-backed embedder. baseURL
// defaults to "http://localhost:11434" when empty; model defaults to
// "nomic-embed-text" (Ollama's canonical text-embeddings model) when
// empty. Construction performs no network I/O.
func NewOllamaEmbedder(baseURL, model string) *OllamaEmbedder {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	if model == "" {
		model = "nomic-embed-text"
	}
	return &OllamaEmbedder{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		model:   model,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type ollamaEmbeddingsRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type ollamaEmbeddingsResponse struct {
	Embedding []float64 `json:"embedding"`
}

// Embed performs a real POST to Ollama's /api/embeddings endpoint and
// returns the decoded embedding vector.
func (e *OllamaEmbedder) Embed(ctx context.Context, text string) ([]float64, error) {
	reqBody, err := json.Marshal(ollamaEmbeddingsRequest{Model: e.model, Prompt: text})
	if err != nil {
		return nil, fmt.Errorf("rag: failed to marshal embeddings request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+"/api/embeddings", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("rag: failed to create embeddings request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := e.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("rag: ollama embeddings request failed (is Ollama running at %s?): %w", e.baseURL, err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("rag: ollama embeddings API returned status %d: %s", resp.StatusCode, string(body))
	}

	var decoded ollamaEmbeddingsResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, fmt.Errorf("rag: failed to decode ollama embeddings response: %w", err)
	}
	if len(decoded.Embedding) == 0 {
		return nil, fmt.Errorf("rag: ollama returned an empty embedding for model %q (never fabricated — see §11.4.6)", e.model)
	}
	return decoded.Embedding, nil
}
