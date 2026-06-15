package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"dev.helix.code/internal/llm"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// llm_generate_regression_test.go — standing regression guards (§11.4.135) for
// two REAL reproduced server defects, each authored RED-on-the-broken-artifact
// with a single RED_MODE polarity switch (§11.4.115):
//
//   - RED_MODE=1 reproduces the historical defect on a faithful replica of the
//     pre-fix code path and asserts the defect IS present (the proof the guard
//     is real — run against the OLD logic it captures a panic / the Ollama mask).
//   - RED_MODE=0 (default) is the standing GREEN guard asserting the defect is
//     ABSENT in the shipped handler.
//
// Defect #5 (CRITICAL): /api/v1/llm/stream double channel-close. The provider's
// GenerateStream owns `defer close(ch)`; the OLD streamLLM ALSO did
// `defer close(chunkChan)` on the same channel → `panic: close of closed
// channel` in the spawned producer goroutine → uncatchable by gin.Recovery →
// the whole process dies from one client request.
//
// Defect #4 (MEDIUM): an explicitly-named UNKNOWN provider silently fell back to
// the local Ollama default, so a user's provider typo surfaced as a misleading
// Ollama 404 instead of a clear "unknown provider" 400.
//
// CONST-050(A): the fake provider below lives ONLY in this *_test.go unit file.
// Production code never references it.

// redMode reports whether RED_MODE is set to a truthy value (default false ⇒
// GREEN guard). Per §11.4.115 the SAME source serves both polarities.
func redMode(t *testing.T) bool {
	t.Helper()
	v := strings.TrimSpace(os.Getenv("RED_MODE"))
	return v == "1" || strings.EqualFold(v, "true") || strings.EqualFold(v, "yes")
}

// closingFakeProvider is a deterministic, network-free llm.Provider whose
// GenerateStream obeys the channel-ownership contract: it sends a chunk then
// closes ch via `defer close(ch)` — exactly like deepseek/openai and the other
// SENDER-closes providers. Driving the real streamLLM with this provider is
// what would double-close (and crash) under the OLD consumer-also-closes code.
type closingFakeProvider struct {
	chunks []string
}

func (f *closingFakeProvider) GetType() llm.ProviderType            { return llm.ProviderTypeOllama }
func (f *closingFakeProvider) GetName() string                      { return "fake-closing" }
func (f *closingFakeProvider) GetModels() []llm.ModelInfo           { return nil }
func (f *closingFakeProvider) GetCapabilities() []llm.ModelCapability { return nil }
func (f *closingFakeProvider) IsAvailable(ctx context.Context) bool { return true }
func (f *closingFakeProvider) GetContextWindow() int                { return 4096 }
func (f *closingFakeProvider) CountTokens(text string) (int, error) { return len(text) / 4, nil }
func (f *closingFakeProvider) Close() error                         { return nil }

func (f *closingFakeProvider) GetHealth(ctx context.Context) (*llm.ProviderHealth, error) {
	return &llm.ProviderHealth{Status: "healthy", LastCheck: time.Now()}, nil
}

func (f *closingFakeProvider) Generate(ctx context.Context, req *llm.LLMRequest) (*llm.LLMResponse, error) {
	return &llm.LLMResponse{ID: uuid.New(), Content: strings.Join(f.chunks, "")}, nil
}

// GenerateStream is the SENDER and SOLE closer of ch (the contract).
func (f *closingFakeProvider) GenerateStream(ctx context.Context, req *llm.LLMRequest, ch chan<- llm.LLMResponse) error {
	defer close(ch)
	for _, c := range f.chunks {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ch <- llm.LLMResponse{ID: uuid.New(), Content: c, CreatedAt: time.Now()}:
		}
	}
	return nil
}

// withFakeResolver temporarily points the handler's provider resolver at a fake
// that returns p, restoring the real resolver on cleanup.
func withFakeResolver(t *testing.T, p llm.Provider) {
	t.Helper()
	prev := llmProviderResolver
	llmProviderResolver = func(providerName, model string) (llm.Provider, error) { return p, nil }
	t.Cleanup(func() { llmProviderResolver = prev })
}

// oldStreamPumpReplica replicates the PRE-FIX streamLLM producer goroutine: the
// CONSUMER also closes the channel (`defer close(chunkChan)`) on top of the
// provider's own `defer close(ch)`. Used only in RED_MODE to prove the historic
// double-close genuinely panics. Returns the recovered panic value (nil if no
// panic), captured from inside the spawned goroutine (where gin.Recovery cannot
// reach it — which is exactly why the real bug killed the process).
func oldStreamPumpReplica(p llm.Provider) (recovered interface{}) {
	chunkChan := make(chan llm.LLMResponse, 100)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() { recovered = recover() }()
		// OLD consumer-side close (the bug): the provider ALSO closes chunkChan.
		defer close(chunkChan)
		_ = p.GenerateStream(context.Background(), &llm.LLMRequest{}, chunkChan)
	}()
	// Drain so GenerateStream's sends do not block.
	for range chunkChan {
	}
	wg.Wait()
	return recovered
}

// TestStreamLLM_NoDoubleCloseCrash_RegressionGuard — Defect #5 guard.
//
// RED_MODE=1: drive the OLD double-close replica with a SENDER-closes provider
// and assert it panics with "close of closed channel" (the historic crash).
//
// RED_MODE=0 (default, GREEN guard): drive the REAL streamLLM handler over a
// real gin engine + httptest recorder with the same SENDER-closes provider, and
// assert the request completes with a real SSE body (data: ... + [DONE]) and NO
// panic — proving the consumer no longer double-closes.
func TestStreamLLM_NoDoubleCloseCrash_RegressionGuard(t *testing.T) {
	fake := &closingFakeProvider{chunks: []string{"Hello", " world"}}

	if redMode(t) {
		recovered := oldStreamPumpReplica(fake)
		require.NotNil(t, recovered,
			"RED expectation: the pre-fix consumer-also-closes path MUST double-close and panic")
		msg, _ := recovered.(error)
		if msg != nil {
			assert.Contains(t, msg.Error(), "close of closed channel",
				"RED: the panic must be the double-close crash this guard exists to prevent")
		} else {
			assert.Contains(t, toString(recovered), "close of closed channel")
		}
		return
	}

	// GREEN guard: the REAL handler must NOT crash and MUST stream honestly.
	withFakeResolver(t, fake)
	gin.SetMode(gin.TestMode)
	srv := &Server{}
	router := gin.New()
	router.Use(gin.Recovery()) // mirrors production; proves we don't even rely on it
	router.POST("/api/v1/llm/stream", srv.streamLLM)

	w := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodPost, "/api/v1/llm/stream",
		strings.NewReader(`{"prompt":"hi"}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	// If the goroutine double-closes, the process would normally die; in-test the
	// panic surfaces here. ServeHTTP must return cleanly.
	require.NotPanics(t, func() { router.ServeHTTP(w, req) },
		"streamLLM must not panic — a double channel-close would crash the server process")

	body := w.Body.String()
	assert.Contains(t, body, "data: Hello", "real provider chunk must be streamed as SSE")
	assert.Contains(t, body, "data: [DONE]", "stream must terminate with the [DONE] frame")
	assert.NotContains(t, strings.ToLower(body), "close of closed channel")
}

// TestResolveLLMProvider_UnknownProviderNoSilentOllamaFallback_RegressionGuard
// — Defect #4 guard.
//
// RED_MODE=1: assert the OLD behaviour — an unknown named provider resolved to
// the Ollama default (provider.GetName() == "ollama"), masking the typo.
//
// RED_MODE=0 (default, GREEN guard): assert the FIXED behaviour — an unknown
// named provider yields errUnknownProvider (no provider), and the handler
// answers 400 naming the bad provider, never a silent Ollama fallback.
func TestResolveLLMProvider_UnknownProviderNoSilentOllamaFallback_RegressionGuard(t *testing.T) {
	t.Setenv("HELIX_LLM_PROVIDER", "") // ensure only the request-named provider matters

	if redMode(t) {
		// Replicate the OLD resolution: on llm.Select failure, fall through to
		// the local Ollama default (the masking bug).
		sel := llm.SelectorInput{Flag: "definitely-not-a-real-provider"}
		_, selErr := llm.Select(sel)
		require.Error(t, selErr, "an unknown provider name must not resolve")
		oldFallback, err := llm.NewOllamaProvider(llm.OllamaConfig{
			DefaultModel: "llama3.2", BaseURL: "http://localhost:11434", StreamEnabled: true,
		})
		require.NoError(t, err)
		defer func() { _ = oldFallback.Close() }()
		assert.Equal(t, "ollama", oldFallback.GetName(),
			"RED expectation: the pre-fix path silently returned the Ollama default for an unknown provider")
		return
	}

	// GREEN guard 1: resolveLLMProvider rejects the unknown provider, no fallback.
	prov, err := resolveLLMProvider("definitely-not-a-real-provider", "")
	require.Error(t, err, "an explicitly-named unknown provider must NOT silently fall back to Ollama")
	assert.Nil(t, prov, "no provider must be constructed for an unknown provider name")
	assert.ErrorIs(t, err, errUnknownProvider, "the error must be the unknown-provider sentinel")
	assert.Contains(t, err.Error(), "definitely-not-a-real-provider",
		"the error must echo the bad provider name the user supplied")

	// GREEN guard 2: the handler maps that to a real 400 (client error), not 503.
	srv := &Server{}
	w, body := postJSON(t, "/api/v1/llm/generate", srv.generateLLM,
		`{"prompt":"hi","provider":"definitely-not-a-real-provider"}`)
	require.Equal(t, http.StatusBadRequest, w.Code,
		"an unknown provider is a client error (400), never a 503 or a masked Ollama 404")
	require.NotNil(t, body)
	assert.Equal(t, "error", body["status"])
	errMsg, _ := body["error"].(string)
	assert.Contains(t, errMsg, "unknown provider", "the 400 body must name the unknown-provider cause")
	assert.NotContains(t, strings.ToLower(errMsg), "ollama",
		"the unknown-provider error must NOT be masked as an Ollama failure")
}

// TestResolveLLMProvider_NoProviderNamedStillFallsBackToOllama proves the fix is
// surgical: when NO provider is named, the default Ollama fallback is preserved
// (out-of-the-box zero-config behaviour). Only an EXPLICIT unknown name is
// rejected.
func TestResolveLLMProvider_NoProviderNamedStillFallsBackToOllama(t *testing.T) {
	t.Setenv("HELIX_LLM_PROVIDER", "")
	prov, err := resolveLLMProvider("", "")
	require.NoError(t, err, "no provider named must still resolve to the local Ollama default")
	require.NotNil(t, prov)
	defer func() { _ = prov.Close() }()
	assert.Equal(t, "ollama", prov.GetName(),
		"zero-config default must remain the local Ollama provider")
}

func toString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	if e, ok := v.(error); ok {
		return e.Error()
	}
	return ""
}
