package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// F6 / D-5 (CONST-036) — provider GetModels() lists MUST be dynamic (live-fetched
// from the provider's /models catalog), not hardcoded literal slices.
//
// RED_MODE polarity switch (§11.4.115):
//   RED_MODE=1 → reproduce the gap on the pre-fix tree (assert the provider has
//                NO live-fetch capability — a fake /models server returning a
//                novel model id is NOT reflected in GetModels()). This FAILs once
//                the live-fetch is wired (proving RED reproduced on broken code).
//   RED_MODE=0 → standing GREEN guard (assert the live catalog IS reflected).
//
// In-package unit tests: each drives the provider's live-fetch against an
// httptest /models server, so no real network/key is needed (the fetch itself is
// the unit under test). These are unit-test fixtures (CONST-050(A) permits them
// here only).

// openaiModelsHandler returns a standard OpenAI {data:[{id}]} catalog.
func openaiModelsHandler(ids ...string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		out := `{"data":[`
		for i, id := range ids {
			if i > 0 {
				out += ","
			}
			out += `{"id":"` + id + `"}`
		}
		out += `]}`
		_, _ = w.Write([]byte(out))
	}
}

func hasModelNamed(models []ModelInfo, name string) bool {
	for _, m := range models {
		if m.Name == name {
			return true
		}
	}
	return false
}

// TestOpenAI_NoNetworkAtConstruction_D5 guards the no-network-at-construction
// contract (batch-2 review SHOULD-FIX #2): constructing the provider + calling
// initializeModels() must make ZERO HTTP calls; the live /models fetch is lazy
// on first GetModels(). Protects against a future refactor that fetches eagerly.
func TestOpenAI_NoNetworkAtConstruction_D5(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"gpt-4o"}]}`))
	}))
	defer srv.Close()

	p := &OpenAIProvider{
		endpoint:   srv.URL,
		apiKey:     "sk-test-realkey-123",
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
	p.initializeModels()
	if got := atomic.LoadInt32(&hits); got != 0 {
		t.Fatalf("construction made %d HTTP call(s); expected 0 (no-network-at-construction contract)", got)
	}
	_ = p.GetModels() // first call triggers the lazy live catalog fetch
	if got := atomic.LoadInt32(&hits); got < 1 {
		t.Fatalf("GetModels() made %d HTTP call(s); expected >=1 (lazy live fetch)", got)
	}
}

func TestOpenAI_DynamicModels_D5(t *testing.T) {
	novel := "gpt-d5-dynamic-probe"
	srv := httptest.NewServer(openaiModelsHandler(novel, "gpt-4o"))
	defer srv.Close()

	p := &OpenAIProvider{
		endpoint:   srv.URL,
		apiKey:     "sk-test-realkey-123",
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
	p.initializeModels()
	got := hasModelNamed(p.GetModels(), novel)
	assertDynamic(t, "OpenAI", got)
}

func TestDeepSeek_DynamicModels_D5(t *testing.T) {
	novel := "deepseek-d5-dynamic-probe"
	srv := httptest.NewServer(openaiModelsHandler(novel, "deepseek-chat"))
	defer srv.Close()

	p := &DeepSeekProvider{
		endpoint:   srv.URL,
		apiKey:     "sk-test-realkey-123",
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
	p.initializeModels()
	got := hasModelNamed(p.GetModels(), novel)
	assertDynamic(t, "DeepSeek", got)
}

func TestMistral_DynamicModels_D5(t *testing.T) {
	novel := "mistral-d5-dynamic-probe"
	srv := httptest.NewServer(openaiModelsHandler(novel, "mistral-large-latest"))
	defer srv.Close()

	p := &MistralProvider{
		endpoint:   srv.URL,
		apiKey:     "sk-test-realkey-123",
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
	p.initializeModels()
	got := hasModelNamed(p.GetModels(), novel)
	assertDynamic(t, "Mistral", got)
}

func TestAnthropic_DynamicModels_D5(t *testing.T) {
	novel := "claude-d5-dynamic-probe"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"` + novel + `","display_name":"Claude D5 Probe"},{"id":"claude-3-5-sonnet-latest","display_name":"Claude 3.5 Sonnet"}]}`))
	}))
	defer srv.Close()

	p := &AnthropicProvider{
		apiKey:     "sk-ant-test-realkey-123",
		endpoint:   srv.URL + "/v1/messages",
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
	models := p.fetchModelCatalog(context.Background())
	got := hasModelNamed(models, novel)
	assertDynamic(t, "Anthropic", got)
}

func assertDynamic(t *testing.T, provider string, liveModelPresent bool) {
	t.Helper()
	if redMode() {
		// RED: on the broken (hardcoded) tree the novel live model is absent.
		if liveModelPresent {
			t.Fatalf("RED_MODE=1: %s already reflects live catalog — defect not reproduced", provider)
		}
		t.Logf("RED reproduced: %s GetModels() is hardcoded (novel live model absent)", provider)
		return
	}
	// GREEN: live catalog reflected in GetModels().
	if !liveModelPresent {
		t.Fatalf("%s GetModels() did not reflect the live /models catalog — still hardcoded (CONST-036)", provider)
	}
}
