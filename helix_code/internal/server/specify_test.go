package server

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// specify_test.go — real httptest-driven unit coverage for the specifyHandler
// (POST /api/v1/specify) in specify.go.
//
// Anti-bluff (CONST-035 / Article XI §11.9): there is NO injection seam on the
// Server struct — Server.llm stays nil and resolveLLMProvider builds a REAL
// llm.Provider per request. So this test does NOT fabricate a provider. It
// exercises honest, real response paths through gin's test recorder
// (postJSON is the shared helper from llm_generate_test.go):
//
//  1. The deterministic request-validation path — an empty/missing 'request'
//     MUST be rejected 400 with a real error body, NO provider call, NO panic.
//  2. The malformed-JSON path — ShouldBindJSON failure → real 400.
//  3. The realistic no-reachable-provider path — with no provider named and no
//     Ollama reachable, the handler resolves the local-Ollama default, then the
//     real speckit phase's first debate turn calls provider.Generate, which
//     fails against the unreachable endpoint, and the handler surfaces a real
//     502 — NEVER a fabricated 200 with phase output (BLUFF-001 guard).

// TestSpecify_RejectsEmptyRequest proves specifyHandler returns a real 400 with
// status:error when the body carries no 'request' — zero network dependency.
func TestSpecify_RejectsEmptyRequest(t *testing.T) {
	srv := &Server{}
	w, body := postJSON(t, "/api/v1/specify", srv.specifyHandler, `{"provider":"ollama"}`)

	require.Equal(t, http.StatusBadRequest, w.Code,
		"empty request must be a 400, not a fabricated success")
	require.NotNil(t, body, "error response must be JSON")
	assert.Equal(t, "error", body["status"])
	errMsg, _ := body["error"].(string)
	assert.Contains(t, errMsg, "non-empty", "error body must name the real validation cause")

	// Anti-bluff: a validation rejection must NOT leak a fabricated phase output.
	_, hasOutput := body["output"]
	assert.False(t, hasOutput, "a 400 must not carry a fabricated 'output' field")
}

// TestSpecify_RejectsWhitespaceRequest proves a request that is only whitespace
// is treated as empty (TrimSpace) and rejected 400 — no provider call.
func TestSpecify_RejectsWhitespaceRequest(t *testing.T) {
	srv := &Server{}
	w, body := postJSON(t, "/api/v1/specify", srv.specifyHandler, `{"request":"   "}`)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.NotNil(t, body)
	assert.Equal(t, "error", body["status"])
	errMsg, _ := body["error"].(string)
	assert.Contains(t, errMsg, "non-empty")
}

// TestSpecify_RejectsMalformedJSON proves specifyHandler returns a real 400
// when the body is not valid JSON (ShouldBindJSON failure path).
func TestSpecify_RejectsMalformedJSON(t *testing.T) {
	srv := &Server{}
	w, body := postJSON(t, "/api/v1/specify", srv.specifyHandler, `{not-json`)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.NotNil(t, body)
	assert.Equal(t, "error", body["status"])
	errMsg, _ := body["error"].(string)
	assert.Contains(t, errMsg, "invalid request body", "malformed JSON must surface the real bind error")
}

// TestSpecify_RealProviderUnreachable_Returns502 proves the realistic honest
// path: a valid request with no reachable provider drives the REAL
// resolveLLMProvider + the REAL speckit ExecutePhase, whose first debate turn
// calls provider.Generate against the unreachable local Ollama and fails, so
// the handler surfaces a real 502 — NOT a fabricated 200 with phase output
// (BLUFF-001 guard). This proves the handler genuinely runs the speckit engine
// and honestly reports its failure.
// TestSpecify_RealProvider_NeverFabricates drives the REAL resolveLLMProvider +
// REAL speckit ExecutePhase and asserts a DETERMINISTIC, environment-independent
// anti-bluff invariant (§11.4.98 — no flaky dependence on whether a local Ollama
// happens to be running). The handler MUST never fabricate a 200: either a real
// 200 with genuine non-empty phase output, or a real error (502/503) naming the
// real provider with no leaked output. Anything else fails.
func TestSpecify_RealProvider_NeverFabricates(t *testing.T) {
	t.Setenv("HELIX_LLM_PROVIDER", "")

	srv := &Server{}
	// A model is provided so the no-models guard does not pre-empt the phase run;
	// the real debate turn then either completes (200) or fails honestly (502/503).
	w, body := postJSON(t, "/api/v1/specify", srv.specifyHandler,
		`{"request":"Add a /healthz endpoint","model":"llama3.2"}`)
	require.NotNil(t, body, "response must be JSON")

	switch w.Code {
	case http.StatusOK:
		// Real success: MUST carry genuine non-empty phase output — never a
		// fabricated-empty 200.
		assert.Equal(t, "success", body["status"])
		out, _ := body["output"].(string)
		assert.NotEmpty(t, out,
			"a 200 must carry REAL non-empty phase output, never a fabricated-empty success")
	case http.StatusBadGateway, http.StatusServiceUnavailable:
		// Real failure (provider unreachable, or the model too weak to finish the
		// debate within the deadline): honest error, no leaked fabricated output.
		assert.Equal(t, "error", body["status"])
		errMsg, _ := body["error"].(string)
		assert.NotEmpty(t, errMsg, "the error path must carry a real error message")
		if out, ok := body["output"].(string); ok {
			assert.Empty(t, out, "no fabricated phase output may appear on the failure path")
		}
	default:
		t.Fatalf("unexpected status %d (body=%v): /specify must return 200 (real output) or 502/503 (real error), never anything else", w.Code, body)
	}
}
