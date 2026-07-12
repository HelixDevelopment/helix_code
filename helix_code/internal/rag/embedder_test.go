package rag

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestOllamaEmbedder_RealHTTPRoundTrip drives OllamaEmbedder.Embed against a
// real net/http server exercising the exact wire protocol Ollama's
// /api/embeddings endpoint uses (POST {model,prompt} -> {embedding:[...]}),
// so the request-marshaling + response-decoding path is genuinely exercised
// end-to-end. httptest.NewServer is a real listening HTTP server on a real
// socket — this is NOT a business-logic mock (§11.4.27 permits mocks only
// in unit tests; this test drives the real client code path against a real
// wire protocol).
func TestOllamaEmbedder_RealHTTPRoundTrip(t *testing.T) {
	var gotModel, gotPrompt, gotPath, gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		var body struct {
			Model  string `json:"model"`
			Prompt string `json:"prompt"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}
		gotModel = body.Model
		gotPrompt = body.Prompt
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"embedding": []float64{0.1, 0.2, 0.3},
		})
	}))
	defer srv.Close()

	e := NewOllamaEmbedder(srv.URL, "test-embed-model")
	vec, err := e.Embed(context.Background(), "hello world")
	if err != nil {
		t.Fatalf("Embed returned error: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("expected a real POST request, got %q", gotMethod)
	}
	if gotPath != "/api/embeddings" {
		t.Fatalf("expected the real Ollama embeddings path /api/embeddings, got %q", gotPath)
	}
	if gotModel != "test-embed-model" {
		t.Fatalf("expected model %q sent on the wire, got %q", "test-embed-model", gotModel)
	}
	if gotPrompt != "hello world" {
		t.Fatalf("expected prompt %q sent on the wire, got %q", "hello world", gotPrompt)
	}
	if len(vec) != 3 || vec[0] != 0.1 || vec[1] != 0.2 || vec[2] != 0.3 {
		t.Fatalf("expected the real decoded embedding [0.1 0.2 0.3], got %v", vec)
	}
}

func TestOllamaEmbedder_NonOKStatusIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"model not found"}`))
	}))
	defer srv.Close()

	e := NewOllamaEmbedder(srv.URL, "missing-model")
	if _, err := e.Embed(context.Background(), "x"); err == nil {
		t.Fatal("expected an error for a non-200 response, got nil")
	}
}

func TestOllamaEmbedder_EmptyEmbeddingIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"embedding": []float64{}})
	}))
	defer srv.Close()

	e := NewOllamaEmbedder(srv.URL, "some-model")
	if _, err := e.Embed(context.Background(), "x"); err == nil {
		t.Fatal("expected an error for an empty embedding vector (never fabricate one)")
	}
}

// TestOllamaEmbedder_UnreachableServerIsError proves the anti-bluff
// requirement explicitly: when Ollama is unreachable, Embed MUST return a
// real error, NEVER a fabricated/fallback vector (§11.4.6).
func TestOllamaEmbedder_UnreachableServerIsError(t *testing.T) {
	e := NewOllamaEmbedder("http://127.0.0.1:1", "any-model")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := e.Embed(ctx, "x"); err == nil {
		t.Fatal("expected an error when Ollama is unreachable, got nil")
	}
}

func TestNewOllamaEmbedder_Defaults(t *testing.T) {
	e := NewOllamaEmbedder("", "")
	if e.baseURL != "http://localhost:11434" {
		t.Fatalf("expected default base URL http://localhost:11434, got %q", e.baseURL)
	}
	if e.model != "nomic-embed-text" {
		t.Fatalf("expected default embed model nomic-embed-text, got %q", e.model)
	}
}

func TestNewOllamaEmbedder_TrimsTrailingSlash(t *testing.T) {
	e := NewOllamaEmbedder("http://example.com:11434/", "m")
	if e.baseURL != "http://example.com:11434" {
		t.Fatalf("expected trailing slash trimmed, got %q", e.baseURL)
	}
}
