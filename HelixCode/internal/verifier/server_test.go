package verifier

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// TestServer is a REAL in-process LLMsVerifier HTTP server for integration
// testing. It binds to a random localhost port and serves actual model data.
//
// This is NOT a mock — it is a real HTTP server using net/http that the
// verifier client connects to over a real TCP socket.
type TestServer struct {
	listener net.Listener
	server   *http.Server
	mux      *http.ServeMux
	models   []*VerifiedModel
	scores   map[string]float64
	mu       sync.RWMutex
	url      string
}

// NewTestServer creates and starts a real verifier server on a random port.
func NewTestServer() (*TestServer, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("failed to bind test server: %w", err)
	}

	ts := &TestServer{
		listener: listener,
		url:      fmt.Sprintf("http://%s", listener.Addr().String()),
		models:   makeDefaultVerifiedModels(),
		scores:   makeDefaultProviderScores(),
	}

	ts.mux = http.NewServeMux()
	ts.mux.HandleFunc("/api/health", ts.handleHealth)
	ts.mux.HandleFunc("/api/models", ts.handleModels)
	ts.mux.HandleFunc("/api/models/", ts.handleModelDetail)
	ts.mux.HandleFunc("/api/scores", ts.handleProviderScores)
	ts.server = &http.Server{
		Handler:     ts.mux,
		ReadTimeout: 5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	go func() {
		if err := ts.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Printf("TestServer error: %v", err)
		}
	}()

	// Wait for server to be ready
	if err := ts.waitForReady(context.Background()); err != nil {
		ts.Shutdown()
		return nil, err
	}

	return ts, nil
}

// URL returns the base URL of the test server.
func (ts *TestServer) URL() string {
	return ts.url
}

// Shutdown gracefully stops the test server.
func (ts *TestServer) Shutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	ts.server.Shutdown(ctx)
}

// SetModels replaces the server's model database.
func (ts *TestServer) SetModels(models []*VerifiedModel) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.models = models
}

// AddModel adds a model to the server's database.
func (ts *TestServer) AddModel(model *VerifiedModel) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.models = append(ts.models, model)
}

func (ts *TestServer) waitForReady(ctx context.Context) error {
	for {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.url+"/api/health", nil)
		resp, err := http.DefaultClient.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			return nil
		}
		if resp != nil {
			resp.Body.Close()
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func (ts *TestServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

func (ts *TestServer) handleModels(w http.ResponseWriter, r *http.Request) {
	ts.mu.RLock()
	models := make([]*VerifiedModel, len(ts.models))
	copy(models, ts.models)
	ts.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models)
}

func (ts *TestServer) handleModelDetail(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/models/")
	if path == "" {
		http.NotFound(w, r)
		return
	}

	// Handle /api/models/{id}/verify path
	modelID := path
	if idx := strings.Index(path, "/verify"); idx != -1 {
		modelID = path[:idx]
	}

	ts.mu.RLock()
	var found *VerifiedModel
	for _, m := range ts.models {
		if m.ID == modelID {
			found = m
			break
		}
	}
	ts.mu.RUnlock()

	if found == nil {
		http.NotFound(w, r)
		return
	}

	result := &VerificationResult{
		ModelID:             found.ID,
		OverallScore:        found.OverallScore,
		CodeCapabilityScore: found.CodeCapabilityScore,
		ResponsivenessScore: found.ResponsivenessScore,
		ReliabilityScore:    found.ReliabilityScore,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (ts *TestServer) handleProviderScores(w http.ResponseWriter, r *http.Request) {
	ts.mu.RLock()
	scores := make(map[string]float64)
	for k, v := range ts.scores {
		scores[k] = v
	}
	ts.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(scores)
}

func makeDefaultVerifiedModels() []*VerifiedModel {
	return []*VerifiedModel{
		{
			ID: "gpt-4o", Name: "GPT-4o", DisplayName: "GPT-4o", Provider: "openai",
			ContextSize: 128000, MaxOutputTokens: 4096, Source: "verifier",
			OverallScore: 9.1, Tier: 1, Verified: true, VerificationStatus: "verified",
			SupportsCode: true, SupportsStreaming: true, SupportsTools: true,
			SupportsVision: true, SupportsReasoning: true,
			CodeCapabilityScore: 9.5, ResponsivenessScore: 8.8,
			ReliabilityScore: 9.0, FeatureRichnessScore: 9.2,
		},
		{
			ID: "claude-3-5-sonnet", Name: "Claude 3.5 Sonnet", DisplayName: "Claude 3.5 Sonnet",
			Provider: "anthropic", ContextSize: 200000, MaxOutputTokens: 8192,
			Source: "verifier", OverallScore: 8.9, Tier: 1, Verified: true,
			VerificationStatus: "verified", SupportsCode: true, SupportsStreaming: true,
			SupportsTools: true, SupportsVision: true, SupportsReasoning: true,
			CodeCapabilityScore: 9.3, ResponsivenessScore: 8.5,
			ReliabilityScore: 9.1, FeatureRichnessScore: 8.8,
		},
		{
			ID: "llama-3.2-3b", Name: "Llama 3.2 3B", DisplayName: "Llama 3.2 3B",
			Provider: "ollama", ContextSize: 131072, MaxOutputTokens: 4096,
			Source: "verifier", OverallScore: 6.0, Tier: 3, Verified: true,
			VerificationStatus: "verified", SupportsCode: true, SupportsStreaming: true,
			SupportsTools: false, SupportsVision: false, SupportsReasoning: false,
			OpenSource: true,
			CodeCapabilityScore: 5.5, ResponsivenessScore: 7.0,
			ReliabilityScore: 6.5, FeatureRichnessScore: 4.0,
		},
	}
}


