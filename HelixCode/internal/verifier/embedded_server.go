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

// EmbeddedServer is a lightweight in-process LLMsVerifier HTTP server.
// It serves the constitutional fallback model list and provider scores
// when no external LLMsVerifier service is configured.
//
// This is a REAL HTTP server (not a mock) that binds to a random localhost
// port and responds to the same API endpoints as the external verifier.
type EmbeddedServer struct {
	listener net.Listener
	server   *http.Server
	mu       sync.RWMutex
	models   []*VerifiedModel
	scores   map[string]float64
	url      string
}

// NewEmbeddedServer creates and starts an embedded verifier server on a random port.
func NewEmbeddedServer() (*EmbeddedServer, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("failed to bind embedded verifier server: %w", err)
	}

	es := &EmbeddedServer{
		listener: listener,
		url:      fmt.Sprintf("http://%s", listener.Addr().String()),
		models:   FallbackModels,
		scores:   makeDefaultProviderScores(),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", es.handleHealth)
	mux.HandleFunc("/api/models", es.handleModels)
	mux.HandleFunc("/api/models/", es.handleModelDetail)
	mux.HandleFunc("/api/scores", es.handleProviderScores)

	es.server = &http.Server{
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	go func() {
		if err := es.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Printf("Embedded verifier server error: %v", err)
		}
	}()

	if err := es.waitForReady(context.Background()); err != nil {
		es.Shutdown()
		return nil, err
	}

	return es, nil
}

// URL returns the base URL of the embedded server.
func (es *EmbeddedServer) URL() string {
	return es.url
}

// Shutdown gracefully stops the embedded server.
func (es *EmbeddedServer) Shutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	es.server.Shutdown(ctx)
}

func (es *EmbeddedServer) waitForReady(ctx context.Context) error {
	for {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, es.url+"/api/health", nil)
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

func (es *EmbeddedServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy", "source": "embedded"})
}

func (es *EmbeddedServer) handleModels(w http.ResponseWriter, r *http.Request) {
	es.mu.RLock()
	models := make([]*VerifiedModel, len(es.models))
	copy(models, es.models)
	es.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models)
}

func (es *EmbeddedServer) handleModelDetail(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/models/")
	if path == "" {
		http.NotFound(w, r)
		return
	}

	modelID := path
	if idx := strings.Index(path, "/verify"); idx != -1 {
		modelID = path[:idx]
	}

	es.mu.RLock()
	var found *VerifiedModel
	for _, m := range es.models {
		if m.ID == modelID {
			found = m
			break
		}
	}
	es.mu.RUnlock()

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

func (es *EmbeddedServer) handleProviderScores(w http.ResponseWriter, r *http.Request) {
	es.mu.RLock()
	scores := make(map[string]float64)
	for k, v := range es.scores {
		scores[k] = v
	}
	es.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(scores)
}
