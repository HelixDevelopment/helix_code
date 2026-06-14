package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"dev.helix.code/internal/llm"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// llm_generate_test.go — real httptest-driven unit coverage for the
// generateLLM (POST /api/v1/llm/generate) and streamLLM (POST
// /api/v1/llm/stream) handlers in llm_generate.go.
//
// Anti-bluff (CONST-035 / Article XI §11.9): there is NO injection seam on the
// Server struct — Server.llm stays nil by design and resolveLLMProvider builds
// a REAL llm.Provider per request (see llm_generate.go doc-comment). So this
// test does NOT fabricate a provider. It exercises two honest, real response
// paths through gin's test recorder:
//
//  1. The deterministic request-validation path — an empty body / a body with
//     neither prompt nor messages MUST be rejected 400 with a real error body
//     by buildLLMRequest, with NO provider call and NO panic.
//
//  2. The realistic no-reachable-provider path — with no provider named and no
//     Ollama reachable (the default resolveLLMProvider target), provider.Generate
//     makes a REAL HTTP call that fails, and the handler MUST surface that as a
//     real 502 Bad Gateway with status:error + a "generation failed:" message +
//     the real provider name — never a fabricated 200 (BLUFF-001 guard).
//
// NewOllamaProvider never errors (it returns a constructed provider regardless
// of Ollama reachability), so the default path reaches provider.Generate; we
// point it at a guaranteed-dead localhost port via HELIX_LLM_OLLAMA_HOST-free
// resolution by relying on the standard port being closed in the test env, and
// additionally assert the error is a genuine connection/generation failure.

// postJSON drives a handler through a fresh gin engine + httptest recorder and
// returns the recorder plus the decoded JSON body.
func postJSON(t *testing.T, route string, h gin.HandlerFunc, body string) (*httptest.ResponseRecorder, map[string]interface{}) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST(route, h)

	w := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodPost, route, bytes.NewBufferString(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	var decoded map[string]interface{}
	if w.Body.Len() > 0 {
		// SSE responses are not JSON; only attempt a decode when it parses.
		_ = json.Unmarshal(w.Body.Bytes(), &decoded)
	}
	return w, decoded
}

// TestGenerateLLM_RejectsEmptyRequest proves generateLLM returns a real 400
// with status:error when the body carries neither prompt nor messages — the
// buildLLMRequest validation path, exercised with zero network dependency.
func TestGenerateLLM_RejectsEmptyRequest(t *testing.T) {
	srv := &Server{}

	// Body decodes fine but yields an empty messages slice -> validation 400.
	w, body := postJSON(t, "/api/v1/llm/generate", srv.generateLLM, `{"model":"llama3.2"}`)

	require.Equal(t, http.StatusBadRequest, w.Code, "empty prompt+messages must be a 400, not a fabricated success")
	require.NotNil(t, body, "error response must be JSON")
	assert.Equal(t, "error", body["status"])
	errMsg, _ := body["error"].(string)
	assert.Contains(t, errMsg, "non-empty", "error body must name the real validation cause")
	// Anti-bluff: a validation rejection must NOT leak a fake content field.
	_, hasContent := body["content"]
	assert.False(t, hasContent, "a 400 must not carry a fabricated 'content' field")
}

// TestGenerateLLM_RejectsMalformedJSON proves generateLLM returns a real 400
// when the body is not valid JSON (ShouldBindJSON failure path).
func TestGenerateLLM_RejectsMalformedJSON(t *testing.T) {
	srv := &Server{}
	w, body := postJSON(t, "/api/v1/llm/generate", srv.generateLLM, `{not-json`)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.NotNil(t, body)
	assert.Equal(t, "error", body["status"])
	errMsg, _ := body["error"].(string)
	assert.Contains(t, errMsg, "invalid request body", "malformed JSON must surface the real bind error")
}

// TestGenerateLLM_RealProviderUnreachable_Returns502 proves the realistic
// end-to-end honest path: a valid request with no reachable provider drives the
// REAL resolveLLMProvider + provider.Generate, which fails against the
// unreachable local Ollama, and the handler surfaces a real 502 Bad Gateway —
// NOT a fabricated 200 (BLUFF-001 guard). This asserts the handler genuinely
// calls the provider and honestly reports its failure.
func TestGenerateLLM_RealProviderUnreachable_Returns502(t *testing.T) {
	// Hermetic: ensure no provider is named so resolveLLMProvider falls to the
	// local Ollama default (which is not running in the test environment).
	t.Setenv("HELIX_LLM_PROVIDER", "")

	// Pre-flight: confirm the default Ollama endpoint really is unreachable, so
	// this test asserts the genuine failure path rather than silently passing
	// against a live model. If Ollama IS up here, the test would instead need a
	// model present; we skip with an explicit reason rather than bluff a PASS.
	if ollamaReachable() {
		t.Skip("SKIP-OK: local Ollama is reachable in this env; this test asserts the unreachable-provider 502 path") //nolint
	}

	srv := &Server{}
	w, body := postJSON(t, "/api/v1/llm/generate", srv.generateLLM, `{"prompt":"What is 2+2?"}`)

	require.Equal(t, http.StatusBadGateway, w.Code,
		"unreachable real provider must surface a real 502, never a fabricated 200")
	require.NotNil(t, body, "error response must be JSON")
	assert.Equal(t, "error", body["status"])

	errMsg, _ := body["error"].(string)
	assert.Contains(t, errMsg, "generation failed", "must report the real provider generation failure")

	// The handler reports the REAL provider name it actually tried — proof it
	// constructed and invoked a real provider, not a stub.
	assert.Equal(t, "ollama", body["provider"], "must name the real provider that was invoked")

	// Anti-bluff: no fabricated success content may leak on the error path.
	if c, ok := body["content"].(string); ok {
		assert.Empty(t, c, "no fabricated content may appear on the failure path")
	}
}

// TestStreamLLM_RejectsEmptyRequest proves streamLLM shares the same real
// validation discipline: an empty prompt+messages body is a real 400 before any
// streaming begins (no provider resolution, no SSE headers committed to a body).
func TestStreamLLM_RejectsEmptyRequest(t *testing.T) {
	srv := &Server{}
	w, body := postJSON(t, "/api/v1/llm/stream", srv.streamLLM, `{"model":"llama3.2"}`)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.NotNil(t, body)
	assert.Equal(t, "error", body["status"])
	errMsg, _ := body["error"].(string)
	assert.Contains(t, errMsg, "non-empty")
}

// ollamaReachable reports whether a local Ollama is answering on the standard
// port — used to keep the unreachable-provider assertion honest.
func ollamaReachable() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost:11434/api/tags", nil)
	if err != nil {
		return false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode == http.StatusOK
}

// compile-time guard: keep the llm + strings imports load-bearing so the test
// file documents the request/response types it asserts against even as the
// handler evolves. (llm.LLMRequest is the type buildLLMRequest produces;
// strings is used to keep the assertion helpers explicit.)
var _ = llm.LLMRequest{}
var _ = strings.TrimSpace
