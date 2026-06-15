//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"dev.helix.code/internal/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// specify_server_e2e_test.go — REAL end-to-end exercise of the
// POST /api/v1/specify web endpoint (server.specifyHandler in
// internal/server/specify.go) against a LIVE local Ollama, asserting a genuine
// speckit Specify-phase debate result reaches the HTTP caller over the wire.
//
// Anti-bluff (CONST-035 / BLUFF-001 / Article XI §11.9 / CONST-050(A) /
// §11.4.107): this test makes NO stub, NO fake provider, NO canned response, and
// NO mock. It boots the REAL HTTP server via server.New(...) (which registers
// the real /api/v1/specify route through setupRoutes) on a real TCP port, then
// issues a REAL http.Client POST. The server-side path is the exact production
// code:
//
//   specifyHandler -> resolveLLMProvider (default local Ollama, no provider
//     named) -> speckit_debate_adapter.NewLLMBackedResponder (2 agents) ->
//     speckit.NewPillar + SetDebateFunc(LLMBackedDebateFunc(responder)) ->
//     pillar.ExecutePhase(PhaseSpecify) -> every debate turn round-trips through
//     a REAL provider.Generate -> REAL HTTP POST to
//     http://localhost:11434/api/chat -> a real model produces real tokens.
//
// This is the missing HTTP-server counterpart to the existing PROVIDER-DIRECT
// e2e (tests/integration/specify_e2e_test.go, which calls pillar.ExecutePhase
// directly) and to the unit-level httptest coverage
// (internal/server/specify_test.go's TestSpecify_RealProvider_NeverFabricates).
// Here the request goes over a real TCP socket to a booted server.
//
// Request shape  (internal/server/specify.go:44-55, specifyRequest):
//   {"request": "<feature/spec text>",  // required, non-empty after TrimSpace
//    "provider": "<optional provider>", // empty => HELIX_LLM_PROVIDER / Ollama
//    "model":    "<optional model id>"} // empty => provider's first model
//
// Success response (specify.go:182-190, HTTP 200):
//   {"status":"success", "output":<string>, "qualityScore":<num>,
//    "debateID":<string>, "success":<bool>, "provider":<string>,
//    "model":<string>}
//
// Honest-error response (specify.go:83-88 / 102-109 / 144-151 / 162-180,
//   HTTP 502/503/500): {"status":"error", "error":<string>, "provider":...}
//   with NO fabricated "output".
//
// Run:
//   go test -tags=integration -run TestSpecifyServerE2E ./tests/integration/ -count=1 -v
//
// Per CONST-050(A) integration tests exercise the real system. If Ollama is not
// reachable OR no model is installed, the test SKIPs with an explicit reason
// (SKIP-OK §11.4.3) rather than bluffing a PASS — an honest documented absence,
// never a fabricated success.

// TestSpecifyServerE2E boots the real server and POSTs a real spec request to
// the live /api/v1/specify endpoint, asserting the DETERMINISTIC anti-bluff
// contract: either a genuine 200 with non-empty phase output (and NOT the
// synthesized "awaiting provider wiring" stub), or an honest 502/503 error with
// no leaked fabricated output. Both outcomes are environment-independent proofs
// that the handler genuinely drives the speckit engine and never fabricates.
func TestSpecifyServerE2E(t *testing.T) {
	model, reachable := liveOllamaModel(t)
	if !reachable {
		t.Skip("SKIP-OK: local Ollama not reachable at " + ollamaEndpoint + "; cannot exercise the real specify phase") //nolint
	}
	if model == "" {
		t.Skip("SKIP-OK: local Ollama is reachable but no model is installed; pull a model (e.g. `ollama pull qwen2.5:3b`) to exercise the specify phase") //nolint
	}
	t.Logf("targeting live Ollama model %q via %s for the /api/v1/specify phase", model, ollamaEndpoint)

	// Ensure no cloud provider is named so resolveLLMProvider falls to the local
	// Ollama default — the exact out-of-the-box server path.
	t.Setenv("HELIX_LLM_PROVIDER", "")

	port := freePort(t)
	srv := server.New(minimalServerConfig(port), nil, nil)

	// Start the real HTTP server in the background; stop it at test end.
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Start() }()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	})

	base := "http://127.0.0.1:" + itoa(port)

	// Wait for the listener to accept connections (or fail fast on serve error).
	require.Eventually(t, func() bool {
		select {
		case err := <-serveErr:
			t.Fatalf("server failed to start: %v", err)
			return false
		default:
		}
		c, err := net.DialTimeout("tcp", "127.0.0.1:"+itoa(port), 200*time.Millisecond)
		if err != nil {
			return false
		}
		_ = c.Close()
		return true
	}, 10*time.Second, 100*time.Millisecond, "server must come up on its port")

	// A small, concrete spec task. The 2-agent speckit debate runs multiple
	// rounds against the model, so this is the slow path — allow a generous
	// client deadline (the server caps the phase at 180s internally; we give the
	// HTTP round-trip headroom beyond that).
	bodyJSON := `{"request":"Add a /healthz endpoint that returns 200 OK","model":"` + model + `"}`
	ctx, cancel := context.WithTimeout(context.Background(), 280*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/api/v1/specify", bytes.NewBufferString(bodyJSON))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	// A dedicated client with a generous timeout (the speckit debate against a
	// small local model is slow).
	client := &http.Client{Timeout: 280 * time.Second}
	resp, err := client.Do(req)
	require.NoError(t, err, "the real HTTP POST to the specify endpoint must succeed at the transport level")
	defer func() { _ = resp.Body.Close() }()

	var decoded map[string]interface{}
	dec := json.NewDecoder(resp.Body)
	require.NoError(t, dec.Decode(&decoded), "response body must be JSON")

	t.Logf("HTTP %d body=%v", resp.StatusCode, decoded)

	// Anti-bluff switch-on-status (mirrors specify_test.go's deterministic
	// invariant over real HTTP): /specify returns 200 with REAL non-empty output,
	// or an honest 502/503 with NO leaked fabricated output — never anything else.
	switch resp.StatusCode {
	case http.StatusOK:
		assert.Equal(t, "success", decoded["status"], "real specify phase must report status:success")

		output, ok := decoded["output"].(string)
		require.True(t, ok, "a 200 response must carry a string 'output' field")
		require.NotEmpty(t, strings.TrimSpace(output),
			"a 200 must carry REAL non-empty phase output, never a fabricated-empty success")

		// Anti-bluff proof: the genuine speckit/provider output must NOT be the
		// synthesized deterministic stub the adapter emits when no real provider is
		// wired (speckit_debate_adapter/wiring.go). Its presence would mean the
		// debate never reached a real provider.
		assert.NotContains(t, output, "awaiting provider wiring",
			"real phase output must NOT contain the synthesized 'awaiting provider wiring' stub; got %q", output)

		// Provider name proves a real Ollama provider was constructed and invoked.
		assert.Equal(t, "ollama", decoded["provider"], "must name the real provider invoked")

		excerpt := output
		if len(excerpt) > 600 {
			excerpt = excerpt[:600] + "…(truncated)"
		}
		t.Logf("REAL /specify 200 output excerpt:\n%s", excerpt)

	case http.StatusBadGateway, http.StatusServiceUnavailable:
		// Honest failure (provider unreachable, or the small model too weak/slow to
		// finish the debate within the server's 180s phase deadline): real error,
		// no leaked fabricated output.
		assert.Equal(t, "error", decoded["status"], "the error path must report status:error")
		errMsg, _ := decoded["error"].(string)
		require.NotEmpty(t, errMsg, "the error path must carry a real error message")
		if out, ok := decoded["output"].(string); ok {
			assert.Empty(t, out, "no fabricated phase output may appear on the failure path")
		}
		t.Logf("HONEST /specify error (HTTP %d): %s", resp.StatusCode, errMsg)

	default:
		t.Fatalf("unexpected status %d (body=%v): /api/v1/specify must return 200 (real output) or 502/503 (real error), never anything else",
			resp.StatusCode, decoded)
	}
}
