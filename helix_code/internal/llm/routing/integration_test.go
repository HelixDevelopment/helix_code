//go:build integration

// integration_test.go — speed-programme Phase 3, task P3-T01.
//
// Integration test for the small-model routing cascade. Per CONST-050(A),
// integration tests exercise the real system with NO mocks: this test runs
// the cascade against two REAL HTTP provider shims — a "small-model" server
// and a "frontier-model" server — over real `net/http` round-trips, with the
// real VerifierResolver picking the concrete model from a real verifier-style
// catalogue.
//
// It proves the end-to-end cascade: a confident small-model response is
// accepted from the small server; a low-confidence (truncated) small-model
// response escalates to the frontier server, and the final answer comes from
// the frontier server over a real HTTP call.
//
// Run: go test -tags=integration -run TestRoutingCascade ./internal/llm/routing/

package routing

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// chatRequest / chatResponse are minimal OpenAI-style wire structs so the
// provider shims do a real JSON round-trip.
type chatRequest struct {
	Model    string `json:"model"`
	Messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
}

type chatChoice struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
	FinishReason string `json:"finish_reason"`
}

type chatResponse struct {
	Model   string       `json:"model"`
	Choices []chatChoice `json:"choices"`
}

// newProviderShim starts a real HTTP server that returns the given content +
// finish reason. It records how many times it was called.
func newProviderShim(t *testing.T, content, finishReason string, calls *int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*calls++
		body, _ := io.ReadAll(r.Body)
		var req chatRequest
		_ = json.Unmarshal(body, &req)
		resp := chatResponse{Model: req.Model}
		ch := chatChoice{FinishReason: finishReason}
		ch.Message.Content = content
		resp.Choices = []chatChoice{ch}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

// realHTTPGen builds a GenerateFunc that makes a REAL HTTP POST to whichever
// shim corresponds to the routed tier, decodes the response, and maps the
// finish reason to a routing confidence.
func realHTTPGen(t *testing.T, smallURL, frontierURL string) GenerateFunc {
	t.Helper()
	return func(ctx context.Context, modelID string, tier ModelTier) (Result, error) {
		url := frontierURL
		if tier == TierSmall {
			url = smallURL
		}
		reqBody, _ := json.Marshal(chatRequest{Model: modelID})
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytesReader(reqBody))
		if err != nil {
			return Result{}, err
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpResp, err := http.DefaultClient.Do(httpReq)
		if err != nil {
			return Result{}, err
		}
		defer httpResp.Body.Close()
		var resp chatResponse
		if err := json.NewDecoder(httpResp.Body).Decode(&resp); err != nil {
			return Result{}, err
		}
		if len(resp.Choices) == 0 {
			return Result{Content: "", Confidence: 0.0}, nil
		}
		c := resp.Choices[0]
		conf := 1.0
		if c.FinishReason != "stop" && c.FinishReason != "" {
			conf = 0.0 // truncated / filtered → low confidence → escalate
		}
		return Result{Content: c.Message.Content, Confidence: conf}, nil
	}
}

// bytesReader is a tiny helper so this file needs no extra import.
func bytesReader(b []byte) *byteReaderImpl { return &byteReaderImpl{b: b} }

type byteReaderImpl struct {
	b   []byte
	pos int
}

func (r *byteReaderImpl) Read(p []byte) (int, error) {
	if r.pos >= len(r.b) {
		return 0, io.EOF
	}
	n := copy(p, r.b[r.pos:])
	r.pos += n
	return n, nil
}

// integrationCatalogue is a real verifier-style catalogue used by the
// VerifierResolver in this integration test — model metadata, never hardcoded
// into routing logic.
func integrationCatalogue() []TierModel {
	return []TierModel{
		{ID: "frontier-premium", VerifierTier: 1, Score: 9.4, Verified: true},
		{ID: "small-fast", VerifierTier: 3, Score: 7.1, Verified: true},
	}
}

// catalogueSource adapts a static catalogue to VerifiedModelSource for the
// integration test (this is the integration fixture, not a behavioural mock —
// it just feeds the resolver real metadata shapes).
type catalogueSource struct{ models []TierModel }

func (c catalogueSource) VerifiedModels(_ context.Context) ([]TierModel, error) {
	return c.models, nil
}

// TestRoutingCascade_ConfidentSmallModelIsAccepted runs the cascade against
// real HTTP shims: the small server returns a clean "stop" response, so the
// router accepts it without touching the frontier server.
func TestRoutingCascade_ConfidentSmallModelIsAccepted(t *testing.T) {
	smallCalls, frontierCalls := 0, 0
	small := newProviderShim(t, "fix: correct typo", "stop", &smallCalls)
	defer small.Close()
	frontier := newProviderShim(t, "frontier answer", "stop", &frontierCalls)
	defer frontier.Close()

	r, err := NewRouter(DefaultPolicy(), NewVerifierResolver(catalogueSource{integrationCatalogue()}))
	if err != nil {
		t.Fatalf("NewRouter: %v", err)
	}
	res, err := r.Route(context.Background(), TaskCommitMessage, realHTTPGen(t, small.URL, frontier.URL))
	if err != nil {
		t.Fatalf("Route: %v", err)
	}
	if res.Tier != TierSmall || res.ModelID != "small-fast" {
		t.Errorf("confident small response: tier=%s model=%s, want small/small-fast", res.Tier, res.ModelID)
	}
	if res.Content != "fix: correct typo" {
		t.Errorf("content = %q, want the small-model answer", res.Content)
	}
	if smallCalls != 1 || frontierCalls != 0 {
		t.Errorf("calls: small=%d frontier=%d, want 1/0 (no escalation)", smallCalls, frontierCalls)
	}
}

// TestRoutingCascade_LowConfidenceEscalatesToFrontier runs the cascade against
// real HTTP shims: the small server returns a TRUNCATED response (finish
// reason "length"), so the router escalates and makes a REAL HTTP call to the
// frontier server. The final answer comes from the frontier server.
func TestRoutingCascade_LowConfidenceEscalatesToFrontier(t *testing.T) {
	smallCalls, frontierCalls := 0, 0
	small := newProviderShim(t, "partial degr", "length", &smallCalls)
	defer small.Close()
	frontier := newProviderShim(t, "complete frontier-quality answer", "stop", &frontierCalls)
	defer frontier.Close()

	r, err := NewRouter(DefaultPolicy(), NewVerifierResolver(catalogueSource{integrationCatalogue()}))
	if err != nil {
		t.Fatalf("NewRouter: %v", err)
	}
	res, err := r.Route(context.Background(), TaskClassification, realHTTPGen(t, small.URL, frontier.URL))
	if err != nil {
		t.Fatalf("Route: %v", err)
	}
	if res.Tier != TierFrontier || res.ModelID != "frontier-premium" {
		t.Errorf("escalated response: tier=%s model=%s, want frontier/frontier-premium", res.Tier, res.ModelID)
	}
	if !res.Escalated {
		t.Error("low-confidence small response must be marked Escalated")
	}
	if res.Content != "complete frontier-quality answer" {
		t.Errorf("content = %q, want the frontier-model answer (no quality regression)", res.Content)
	}
	if smallCalls != 1 || frontierCalls != 1 {
		t.Errorf("calls: small=%d frontier=%d, want 1/1 (small attempt + real escalation)", smallCalls, frontierCalls)
	}

	// The routing log is the captured per-subtask model-used evidence.
	log := r.Log()
	t.Logf("routing log (anti-bluff evidence): %d entries", len(log))
	for i, e := range log {
		t.Logf("  [%d] class=%s tier=%s model=%s confidence=%.2f escalated=%v",
			i, e.Class, e.Tier, e.ModelID, e.Confidence, e.Escalated)
	}
}
