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

	"dev.helix.code/internal/config"
	"dev.helix.code/internal/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// llm_generate_e2e_test.go — REAL end-to-end exercise of the
// POST /api/v1/llm/generate web endpoint (server.generateLLM) against a LIVE
// local Ollama, asserting genuine LLM output reaches the HTTP caller over the
// wire.
//
// Anti-bluff (CONST-035 / BLUFF-001 / Article XI §11.9 / CONST-050(A)): this
// test makes NO stub, NO fake provider, NO canned response, and NO mock. It
// boots the REAL HTTP server via server.New(...) (which registers the real
// /api/v1/llm/generate route through setupRoutes) on a real TCP port, then
// issues a REAL http.Client POST. The server-side path is the exact production
// code:
//
//   generateLLM -> resolveLLMProvider (default local Ollama, no provider named)
//     -> llm.NewOllamaProvider -> provider.Generate -> REAL HTTP POST to
//     http://localhost:11434/api/chat -> a real model produces real tokens.
//
// It is the positive counterpart to
// internal/server/llm_generate_test.go's TestGenerateLLM_RealProviderUnreachable_Returns502
// (which asserts the honest 502 when Ollama is DOWN). Here Ollama is UP with a
// real model, so the endpoint MUST return a real 200 whose `content` contains
// the model's actual answer to "What is 2+2?" — proving the web endpoint
// genuinely generates.
//
// Run:
//   go test -tags=integration -run TestLLMGenerateE2E ./tests/integration/ -count=1 -v
//
// Per CONST-050(A) integration tests exercise the real system. If Ollama is not
// reachable OR no model is installed, the test SKIPs with an explicit reason
// (SKIP-OK) rather than bluffing a PASS — an honest documented absence, never a
// fabricated success.

const ollamaEndpoint = "http://localhost:11434"

// ollamaTags is the minimal shape of GET /api/tags needed to discover an
// installed model name at runtime (no hardcoded model assumption beyond a
// preference order over tiny models).
type ollamaTags struct {
	Models []struct {
		Name string `json:"name"`
	} `json:"models"`
}

// liveOllamaModel returns the name of an installed model to target, preferring
// the smallest known-tiny models, else the first installed model. The bool
// reports whether Ollama is reachable at all. An empty string with reachable ==
// true means Ollama is up but has no model installed.
func liveOllamaModel(t *testing.T) (model string, reachable bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ollamaEndpoint+"/api/tags", nil)
	if err != nil {
		return "", false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", false // unreachable
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", false
	}
	var tags ollamaTags
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return "", true // reachable but tag list unreadable
	}
	if len(tags.Models) == 0 {
		return "", true // reachable, no models
	}
	prefer := []string{"qwen2.5:0.5b", "llama3.2:1b", "llama3.2", "qwen2.5:1.5b"}
	for _, p := range prefer {
		for _, m := range tags.Models {
			if m.Name == p {
				return m.Name, true
			}
		}
	}
	return tags.Models[0].Name, true
}

// freePort asks the OS for an unused TCP port so the test server never collides
// with the live HelixAgent (or anything else) on the host.
func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()
	return ln.Addr().(*net.TCPAddr).Port
}

// minimalServerConfig builds the smallest valid config that lets server.New run
// with nil db/redis (every db-dependent subsystem is guarded by `if db != nil`)
// and no verifier/QA bootstrap, so the only live route concern is /llm/generate.
func minimalServerConfig(port int) *config.Config {
	cfg := &config.Config{}
	cfg.Server.Address = "127.0.0.1"
	cfg.Server.Port = port
	cfg.Server.ReadTimeout = 30
	cfg.Server.WriteTimeout = 130 // > handler's 120s provider timeout
	cfg.Server.IdleTimeout = 30
	cfg.Logging.Level = "error"
	cfg.Verifier = nil      // no verifier bootstrap
	cfg.QA.Enabled = false  // no helix_qa engine
	return cfg
}

// TestLLMGenerateE2E boots the real server and POSTs a real request to the live
// generate endpoint, asserting a genuine 200 + non-empty content answering the
// arithmetic prompt.
func TestLLMGenerateE2E(t *testing.T) {
	model, reachable := liveOllamaModel(t)
	if !reachable {
		t.Skip("SKIP-OK: local Ollama not reachable at " + ollamaEndpoint + "; cannot exercise the real generation path") //nolint
	}
	if model == "" {
		t.Skip("SKIP-OK: local Ollama is reachable but no model is installed; pull a model (e.g. `ollama pull qwen2.5:0.5b`) to exercise generation") //nolint
	}
	t.Logf("targeting live Ollama model %q via %s", model, ollamaEndpoint)

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

	bodyJSON := `{"prompt":"What is 2+2? Answer with the number only.","model":"` + model + `"}`
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/api/v1/llm/generate", bytes.NewBufferString(bodyJSON))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err, "the real HTTP POST to the generate endpoint must succeed at the transport level")
	defer func() { _ = resp.Body.Close() }()

	var decoded map[string]interface{}
	dec := json.NewDecoder(resp.Body)
	require.NoError(t, dec.Decode(&decoded), "200 body must be JSON")

	t.Logf("HTTP %d body=%v", resp.StatusCode, decoded)

	require.Equal(t, http.StatusOK, resp.StatusCode,
		"live Ollama must yield a real 200 from the generate endpoint, body=%v", decoded)

	assert.Equal(t, "success", decoded["status"], "real generation must report status:success")

	content, ok := decoded["content"].(string)
	require.True(t, ok, "response must carry a string 'content' field")
	require.NotEmpty(t, strings.TrimSpace(content), "real model output must be non-empty")

	// Anti-bluff proof: the model's genuine answer to 2+2 must contain "4". A
	// simulated/canned response would not reliably solve arithmetic; this asserts
	// on real model output, not metadata.
	assert.Contains(t, content, "4",
		"the live model's real answer to 'What is 2+2?' must contain '4'; got %q", content)

	// Provider name proves a real Ollama provider was constructed and invoked.
	assert.Equal(t, "ollama", decoded["provider"], "must name the real provider invoked")
}

// itoa is a tiny strconv.Itoa to avoid an extra import in a single-use spot.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [20]byte
	pos := len(b)
	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(b[pos:])
}
